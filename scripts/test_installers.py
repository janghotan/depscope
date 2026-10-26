#!/usr/bin/env python3
"""install.sh 与 install.ps1 的确定性测试。

测试在本地启动一个 HTTP 服务器，模拟 GitHub Releases 与 GitHub API，
用真实归档（由测试现场打包合成文件）覆盖校验和、缺失资源与失败路径。
这些是模拟网络响应的测试：它们证明安装器的选档与校验逻辑正确，
不能证明真实发布资产的下载可用（真实端到端安装另行验证）。

运行：python3 scripts/test_installers.py
"""

from __future__ import annotations

import hashlib
import http.server
import io
import json
import os
import pathlib
import shutil
import subprocess
import sys
import ssl
import subprocess as _sp
import tarfile
import tempfile
import threading
import unittest
import zipfile

ROOT = pathlib.Path(__file__).resolve().parent.parent
INSTALL_SH = ROOT / "install.sh"
INSTALL_PS1 = ROOT / "install.ps1"

REPO = "janghotan/depscope"
TAG = "v0.1.0"
ASSET_VERSION = "0.1.0"

# 服务器状态：路径 -> (状态码, 字节内容)。测试可随时修改。
routes: dict[str, tuple[int, bytes]] = {}
fail_paths: set[str] = set()


class Handler(http.server.BaseHTTPRequestHandler):
    """返回 routes 中登记的响应；未登记的路径返回 404。"""

    protocol_version = "HTTP/1.1"

    def do_GET(self) -> None:  # noqa: N802 - http.server 要求的名称
        path = self.path.split("?", 1)[0]
        if path in fail_paths:
            self.send_error(500, "simulated server error")
            return
        status, body = routes.get(path, (404, b"not found"))
        self.send_response(status)
        self.send_header("Content-Type", "application/octet-stream")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_args) -> None:  # 静默日志
        pass


def make_cli_tarball(os_name: str, arch: str) -> bytes:
    """构造与 GoReleaser 布局一致的 CLI 归档：顶层目录 + depscope 可执行文件。"""
    top = f"depscope_cli_{ASSET_VERSION}_{os_name}_{arch}"
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w:gz") as tf:
        for name, data, mode in (
            (f"{top}/LICENSE", b"Apache License 2.0\n", 0o644),
            (f"{top}/NOTICE", b"DepScope\n", 0o644),
            (f"{top}/README.md", b"# DepScope\n", 0o644),
        ):
            info = tarfile.TarInfo(name)
            info.size = len(data)
            info.mode = mode
            tf.addfile(info, io.BytesIO(data))
        # 模拟真实 CLI：只有在 `version` 子命令下输出并返回 0，否则给出非零退出码。
        script = (
            b"#!/bin/sh\n"
            b'if [ "$1" = version ]; then printf "depscope 0.1.0\\n"; exit 0; fi\n'
            b"exit 2\n"
        )
        info = tarfile.TarInfo(f"{top}/depscope")
        info.size = len(script)
        info.mode = 0o755
        tf.addfile(info, io.BytesIO(script))
        # 归档内不应包含 API 二进制，安装器只安装 CLI。
        api = tarfile.TarInfo(f"{top}/depscope-api")
        api_info = b"#!/bin/sh\n"
        api.size = len(api_info)
        api.mode = 0o755
        tf.addfile(api, io.BytesIO(api_info))
    return buf.getvalue()


def make_cli_tarball_with_exit(os_name: str, arch: str, exit_code: int) -> bytes:
    """构造 `version` 返回指定退出码的归档，用于回归测试。"""
    top = f"depscope_cli_{ASSET_VERSION}_{os_name}_{arch}"
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w:gz") as tf:
        script = f"#!/bin/sh\nprintf 'depscope 0.1.0\\n'\nexit {exit_code}\n".encode()
        info = tarfile.TarInfo(f"{top}/depscope")
        info.size = len(script)
        info.mode = 0o755
        tf.addfile(info, io.BytesIO(script))
    return buf.getvalue()


def make_cli_zip(zip_os: str, arch: str) -> bytes:
    """构造 Windows 风格 CLI ZIP。"""
    top = f"depscope_cli_{ASSET_VERSION}_{zip_os}_{arch}"
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w", zipfile.ZIP_DEFLATED) as zf:
        zf.writestr(f"{top}/LICENSE", "Apache License 2.0\n")
        zf.writestr(f"{top}/NOTICE", "DepScope\n")
        zf.writestr(f"{top}/depscope.exe", "fake-windows-exe")
    return buf.getvalue()


def build_release_routes(base: str, *, include_linux_amd64: bool = True) -> None:
    """按 GoReleaser 的真实命名登记发布资产与 checksums.txt。"""
    assets: dict[str, bytes] = {}
    for os_name in ("linux", "darwin"):
        for arch in ("amd64", "arm64"):
            name = f"depscope_cli_{ASSET_VERSION}_{os_name}_{arch}.tar.gz"
            assets[name] = make_cli_tarball(os_name, arch)
    for arch in ("amd64", "arm64"):
        name = f"depscope_cli_{ASSET_VERSION}_windows_{arch}.zip"
        assets[name] = make_cli_zip("windows", arch)
    # API 归档也登记在 checksums.txt 中，用于验证安装器只挑选 CLI 资产。
    for os_name in ("linux", "darwin"):
        assets[f"depscope_api_{ASSET_VERSION}_{os_name}_amd64.tar.gz"] = b"api-archive-placeholder"

    if not include_linux_amd64:
        assets.pop(f"depscope_cli_{ASSET_VERSION}_linux_amd64.tar.gz", None)

    checksum_lines = []
    for name, data in sorted(assets.items()):
        digest = hashlib.sha256(data).hexdigest()
        checksum_lines.append(f"{digest}  {name}")
    checksums = ("\n".join(checksum_lines) + "\n").encode()

    routes.clear()
    routes[f"/releases/download/{TAG}/checksums.txt"] = (200, checksums)
    for name, data in assets.items():
        routes[f"/releases/download/{TAG}/{name}"] = (200, data)
    api_body = json.dumps({"tag_name": TAG, "prerelease": False}).encode()
    routes[f"/api/repos/{REPO}/releases/latest"] = (200, api_body)
    routes["/releases/download/latest"] = (302, b"")


def run_installer(base: str, install_dir: pathlib.Path, *extra: str,
                  env_overrides: dict[str, str] | None = None,
                  ca_path: pathlib.Path | None = None) -> subprocess.CompletedProcess:
    """在受控环境中运行 install.sh；ca_path 用于信任测试服务器的自签名证书。"""
    env = {
        "PATH": os.environ.get("PATH", "/usr/bin:/bin"),
        "HOME": os.environ.get("HOME", "/root"),
        "DEPSCOPE_API_BASE": f"{base}/api/repos/{REPO}",
        "DEPSCOPE_RELEASE_BASE": f"{base}/releases",
        "DEPSCOPE_INSTALL_DIR": str(install_dir),
        "TMPDIR": os.environ.get("TMPDIR", "/tmp"),
    }
    if ca_path is not None:
        env["DEPSCOPE_CA_BUNDLE"] = str(ca_path)
    if env_overrides:
        env.update(env_overrides)
    return subprocess.run(
        ["sh", str(INSTALL_SH), *extra],
        capture_output=True, text=True, env=env, timeout=120,
    )


PWSH = os.environ.get("DEPSCOPE_TEST_PWSH") or shutil.which("pwsh") or shutil.which("powershell")


def run_ps1(base: str, install_dir: pathlib.Path, *extra: str,
            env_overrides: dict[str, str] | None = None,
            ca_path: pathlib.Path | None = None) -> subprocess.CompletedProcess:
    """在受控环境中用 PowerShell 运行 install.ps1。

    install.ps1 的发布与 API 地址默认指向官方 GitHub，测试通过
    DEPSCOPE_RELEASE_BASE 与 DEPSCOPE_API_BASE 覆盖为本地模拟服务器。

    注意：PowerShell 使用 .NET 的 TLS 实现，在 Linux/macOS 上读取 SSL_CERT_FILE，
    在 Windows 上读取系统证书存储。这里分别处理，证书校验始终保持开启。
    """
    work_home = pathlib.Path(tempfile.mkdtemp(prefix="depscope-pshome-"))
    ca_dir = work_home / ".local" / "share" / "ca-certificates"
    ca_dir.mkdir(parents=True)
    if ca_path is not None:
        shutil.copy(ca_path, ca_dir / "depscope-test.crt")
        shutil.copy(ca_path, ca_dir / "depscope-test.pem")

    # .NET 在 Windows 上读取系统证书存储而不是 SSL_CERT_FILE，因此测试前把
    # 测试 CA 导入当前用户 Root 存储，运行结束后移除；证书校验始终开启。
    if ca_path is not None and os.name == "nt":
        # 使用 .NET X509Store 直接写入，避免 Import-Certificate 在无交互会话中
        # 要求 UI 确认（"UI is not allowed in this operation"）。
        import_script = (
            "$store = New-Object System.Security.Cryptography.X509Certificates.X509Store('Root','CurrentUser'); "
            "$store.Open('ReadWrite'); "
            f"$cert = New-Object System.Security.Cryptography.X509Certificates.X509Certificate2('{ca_path}'); "
            "$store.Add($cert); $store.Close(); Write-Output 'imported'"
        )
        import_result = subprocess.run(
            [str(PWSH), "-NoProfile", "-NonInteractive", "-Command", import_script],
            capture_output=True, text=True,
        )
        if import_result.returncode != 0:
            shutil.rmtree(work_home, ignore_errors=True)
            raise RuntimeError(
                f"could not import the test CA into the user root store: {import_result.stderr}"
            )

    # 参数通过 -File 传递，避免命令字符串中的引号被当成参数名的一部分。
    env = {
        "PATH": os.environ.get("PATH", "/usr/bin:/bin"),
        "HOME": str(work_home),
        "XDG_CACHE_HOME": str(work_home / "cache"),
        "XDG_DATA_HOME": str(work_home / "data"),
        "XDG_CONFIG_HOME": str(work_home / "config"),
        "SSL_CERT_FILE": str(ca_dir / "depscope-test.pem"),
        "SSL_CERT_DIR": str(ca_dir),
        "DEPSCOPE_INSTALL_DIR": str(install_dir),
        "DEPSCOPE_RELEASE_BASE": f"{base}/releases",
        "DEPSCOPE_API_BASE": f"{base}/api/repos/{REPO}",
    }
    if env_overrides:
        env.update(env_overrides)
    result = subprocess.run(
        [str(PWSH), "-NoProfile", "-NonInteractive", "-File", str(INSTALL_PS1), *extra],
        capture_output=True, text=True, env=env, timeout=180,
    )
    if ca_path is not None and os.name == "nt":
        # 测试结束把临时信任的 CA 从当前用户 Root 存储中移除。
        remove_script = (
            "$store = New-Object System.Security.Cryptography.X509Certificates.X509Store('Root','CurrentUser'); "
            "$store.Open('ReadWrite'); "
            "foreach ($c in $store.Certificates) { "
            "if ($c.Subject -like '*DepScope Test CA*') { $store.Remove($c) } }; "
            "$store.Close()"
        )
        subprocess.run(
            [str(PWSH), "-NoProfile", "-NonInteractive", "-Command", remove_script],
            capture_output=True, text=True,
        )
    shutil.rmtree(work_home, ignore_errors=True)
    return result


class ServerCase(unittest.TestCase):
    """为每个测试启动独立的模拟发布服务器。"""

    @classmethod
    def setUpClass(cls) -> None:
        # 安装器强制 HTTPS 并校验证书，因此测试服务器也必须使用受信任的证书。
        cls.cert_dir = pathlib.Path(tempfile.mkdtemp(prefix="depscope-certs-"))
        cls.ca_path = cls.cert_dir / "ca.pem"
        _sp.run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1",
                 "-keyout", str(cls.cert_dir / "ca.key"), "-out", str(cls.ca_path),
                 "-subj", "/CN=DepScope Test CA", "-addext", "basicConstraints=critical,CA:TRUE",
                 "-addext", "keyUsage=critical,keyCertSign,cRLSign"], check=True, capture_output=True)
        _sp.run(["openssl", "req", "-newkey", "rsa:2048", "-nodes",
                 "-keyout", str(cls.cert_dir / "server.key"), "-out", str(cls.cert_dir / "server.csr"),
                 "-subj", "/CN=127.0.0.1"], check=True, capture_output=True)
        ext = cls.cert_dir / "ext.cnf"
        ext.write_text("subjectAltName=IP:127.0.0.1\nbasicConstraints=CA:FALSE\n")
        _sp.run(["openssl", "x509", "-req", "-days", "1", "-in", str(cls.cert_dir / "server.csr"),
                 "-CA", str(cls.ca_path), "-CAkey", str(cls.cert_dir / "ca.key"),
                 "-CAcreateserial", "-out", str(cls.cert_dir / "server.pem"), "-extfile", str(ext)],
                check=True, capture_output=True)

        cls.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        ctx.load_cert_chain(str(cls.cert_dir / "server.pem"), str(cls.cert_dir / "server.key"))
        cls.server.socket = ctx.wrap_socket(cls.server.socket, server_side=True)
        cls.base = f"https://127.0.0.1:{cls.server.server_address[1]}"
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()

    @classmethod
    def tearDownClass(cls) -> None:
        cls.server.shutdown()
        cls.server.server_close()
        shutil.rmtree(cls.cert_dir, ignore_errors=True)

    def setUp(self) -> None:
        routes.clear()
        fail_paths.clear()
        build_release_routes(self.base)
        self.tmp = pathlib.Path(tempfile.mkdtemp(prefix="depscope-test-"))
        self.install_dir = self.tmp / "bin"

    def tearDown(self) -> None:
        shutil.rmtree(self.tmp, ignore_errors=True)


class UnixInstallerTests(ServerCase):
    """覆盖 install.sh 的选档、校验、失败与安全行为。"""

    def test_installs_verified_binary(self) -> None:
        """正常路径：通过环境变量指定目录，安装成功且版本可读。"""
        result = run_installer(self.base, self.install_dir, ca_path=self.ca_path)
        self.assertEqual(result.returncode, 0, result.stderr)
        target = self.install_dir / "depscope"
        self.assertTrue(target.is_file(), "depscope binary was not installed")
        self.assertTrue(os.access(target, os.X_OK), "installed binary is not executable")
        out = subprocess.run([str(target), "version"], capture_output=True, text=True)
        self.assertIn("depscope 0.1.0", out.stdout)
        self.assertIn("installed", result.stdout)

    def test_does_not_install_api_binary(self) -> None:
        """只安装 CLI；归档中的 depscope-api 不得落到安装目录。"""
        result = run_installer(self.base, self.install_dir, ca_path=self.ca_path)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse((self.install_dir / "depscope-api").exists())
        self.assertEqual(sorted(p.name for p in self.install_dir.iterdir()), ["depscope"])

    def test_dir_argument_and_latest_resolution(self) -> None:
        """--dir 参数生效，并通过 API 解析最新标签。"""
        alt = self.tmp / "custom-bin"
        result = run_installer(self.base, self.install_dir, "--dir", str(alt), ca_path=self.ca_path)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue((alt / "depscope").is_file())

    def test_checksum_mismatch_fails_closed(self) -> None:
        """校验和不匹配：非零退出，且不产生任何安装文件。"""
        good = routes[f"/releases/download/{TAG}/depscope_cli_{ASSET_VERSION}_linux_amd64.tar.gz"]
        tampered = good[1] + b"tampered"
        routes[f"/releases/download/{TAG}/depscope_cli_{ASSET_VERSION}_linux_amd64.tar.gz"] = (200, tampered)
        result = run_installer(self.base, self.install_dir, ca_path=self.ca_path)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("checksum verification failed", result.stderr)
        self.assertFalse((self.install_dir / "depscope").exists())

    def test_missing_checksums_file_fails(self) -> None:
        """checksums.txt 缺失：fail closed。"""
        del routes[f"/releases/download/{TAG}/checksums.txt"]
        result = run_installer(self.base, self.install_dir, ca_path=self.ca_path)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("could not download checksums.txt", result.stderr)
        self.assertFalse((self.install_dir / "depscope").exists())

    def test_archive_not_listed_in_checksums(self) -> None:
        """checksums.txt 中没有对应资产（例如缺少该平台构建）：fail closed。"""
        build_release_routes(self.base, include_linux_amd64=False)
        result = run_installer(self.base, self.install_dir, ca_path=self.ca_path)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("does not list", result.stderr)
        self.assertFalse((self.install_dir / "depscope").exists())

    def test_malformed_checksums_file(self) -> None:
        """checksums.txt 格式错误：fail closed，不安装。"""
        routes[f"/releases/download/{TAG}/checksums.txt"] = (200, b"this is not a checksum file\n")
        result = run_installer(self.base, self.install_dir, ca_path=self.ca_path)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("does not list", result.stderr)
        self.assertFalse((self.install_dir / "depscope").exists())

    def test_http_error_on_archive(self) -> None:
        """归档请求返回 500：fail closed。"""
        target = f"/releases/download/{TAG}/depscope_cli_{ASSET_VERSION}_linux_amd64.tar.gz"
        fail_paths.add(target)
        result = run_installer(self.base, self.install_dir, ca_path=self.ca_path)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("could not download", result.stderr)
        self.assertFalse((self.install_dir / "depscope").exists())

    def test_missing_release_api(self) -> None:
        """最新版本 API 不可用且未指定版本：给出明确错误。"""
        del routes[f"/api/repos/{REPO}/releases/latest"]
        result = run_installer(self.base, self.install_dir, ca_path=self.ca_path)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("could not resolve the latest release", result.stderr)

    def test_unsupported_os_rejected(self) -> None:
        """不支持的操作系统：在下载任何文件前失败。"""
        fake_uname = self.tmp / "uname"
        fake_uname.write_text("#!/bin/sh\ncase \"$1\" in -s) echo FreeBSD;; -m) echo x86_64;; esac\n")
        fake_uname.chmod(0o755)
        result = run_installer(
            self.base, self.install_dir,
            env_overrides={"PATH": f"{self.tmp}:{os.environ.get('PATH', '')}"},
            ca_path=self.ca_path,
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("unsupported operating system", result.stderr)

    def test_unsupported_arch_rejected(self) -> None:
        """不支持的架构（如 32 位 x86）：在下载前失败。"""
        fake_uname = self.tmp / "uname"
        fake_uname.write_text("#!/bin/sh\ncase \"$1\" in -s) echo Linux;; -m) echo i686;; esac\n")
        fake_uname.chmod(0o755)
        result = run_installer(
            self.base, self.install_dir,
            env_overrides={"PATH": f"{self.tmp}:{os.environ.get('PATH', '')}"},
            ca_path=self.ca_path,
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("unsupported architecture", result.stderr)

    def test_arm64_detection_selects_arm64_asset(self) -> None:
        """aarch64 与 arm64 都映射到 arm64 资产。"""
        fake_uname = self.tmp / "uname"
        fake_uname.write_text("#!/bin/sh\ncase \"$1\" in -s) echo Linux;; -m) echo aarch64;; esac\n")
        fake_uname.chmod(0o755)
        result = run_installer(
            self.base, self.install_dir,
            env_overrides={"PATH": f"{self.tmp}:{os.environ.get('PATH', '')}"},
            ca_path=self.ca_path,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("linux/arm64", result.stderr)

    def test_missing_tool_fails_clearly(self) -> None:
        """缺少 curl/tar 时给出明确错误（PATH 中只保留基本系统工具）。"""
        isolated = self.tmp / "minbin"
        isolated.mkdir()
        # 只提供 sh 与 mktemp，模拟尚未安装 curl、tar、校验工具的环境。
        for tool in ("sh", "mktemp"):
            src = shutil.which(tool)
            if src:
                (isolated / tool).symlink_to(src)
        result = run_installer(
            self.base, self.install_dir,
            env_overrides={"PATH": str(isolated)},
            ca_path=self.ca_path,
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("missing required command", result.stderr)
        self.assertIn("curl", result.stderr)
        self.assertFalse((self.install_dir / "depscope").exists())

    def test_binary_with_failing_version_is_rejected(self) -> None:
        """解压出的程序在 `version` 下返回非零退出码时拒绝安装。

        回归测试：早期版本用 `A && B || true` 取值，会把失败当成空字符串放行。
        """
        key = f"/releases/download/{TAG}/depscope_cli_{ASSET_VERSION}_linux_amd64.tar.gz"
        routes[key] = (200, make_cli_tarball_with_exit("linux", "amd64", exit_code=3))
        # 同时更新 checksum，使校验通过，从而单独验证版本检查这一步。
        entry = f"/releases/download/{TAG}/checksums.txt"
        body = routes[entry][1].decode()
        lines = []
        for line in body.splitlines():
            if line.endswith(f"  depscope_cli_{ASSET_VERSION}_linux_amd64.tar.gz"):
                digest = hashlib.sha256(routes[key][1]).hexdigest()
                line = f"{digest}  depscope_cli_{ASSET_VERSION}_linux_amd64.tar.gz"
            lines.append(line)
        routes[entry] = (200, ("\n".join(lines) + "\n").encode())

        result = run_installer(self.base, self.install_dir, ca_path=self.ca_path)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("did not report a version", result.stderr)
        self.assertFalse((self.install_dir / "depscope").exists())

    def test_existing_install_is_replaced_and_temp_cleaned(self) -> None:
        """已安装的旧版本被安全替换，且不残留临时文件。"""
        self.install_dir.mkdir(parents=True)
        old = self.install_dir / "depscope"
        old.write_text("#!/bin/sh\necho old\n")
        old.chmod(0o755)
        result = run_installer(self.base, self.install_dir, ca_path=self.ca_path)
        self.assertEqual(result.returncode, 0, result.stderr)
        out = subprocess.run([str(old), "version"], capture_output=True, text=True)
        self.assertIn("depscope 0.1.0", out.stdout)
        self.assertIn("replaced the previous", result.stdout)
        leftovers = [p.name for p in self.install_dir.iterdir() if p.name != "depscope"]
        self.assertEqual(leftovers, [], f"unexpected leftover files: {leftovers}")

    def test_temporary_directory_is_removed(self) -> None:
        """退出后不留下 mktemp 目录。"""
        tmp_root = self.tmp / "tmpdir"
        tmp_root.mkdir()
        result = run_installer(self.base, self.install_dir, env_overrides={"TMPDIR": str(tmp_root)}, ca_path=self.ca_path)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(list(tmp_root.iterdir()), [], "temporary files were not cleaned up")

    def test_help_and_unknown_argument(self) -> None:
        """--help 成功退出；未知参数失败。"""
        help_result = run_installer(self.base, self.install_dir, "--help", ca_path=self.ca_path)
        self.assertEqual(help_result.returncode, 0)
        self.assertIn("Install the DepScope CLI", help_result.stdout)
        bad = run_installer(self.base, self.install_dir, "--bogus", ca_path=self.ca_path)
        self.assertNotEqual(bad.returncode, 0)
        self.assertIn("unknown argument", bad.stderr)

    def test_explicit_version_flag(self) -> None:
        """--version 绕过 API 解析，直接使用给定标签。"""
        del routes[f"/api/repos/{REPO}/releases/latest"]
        result = run_installer(self.base, self.install_dir, "--version", TAG, ca_path=self.ca_path)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue((self.install_dir / "depscope").is_file())

    def test_invalid_version_rejected(self) -> None:
        """版本标签含路径穿越字符时被拒绝。"""
        result = run_installer(self.base, self.install_dir, "--version", "../../evil", ca_path=self.ca_path)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("invalid version tag", result.stderr)


@unittest.skipIf(PWSH is None, "pwsh/powershell not available on this machine")
class WindowsInstallerTests(ServerCase):
    """覆盖 install.ps1 的选档、校验与失败行为（在 Linux 上通过 pwsh 运行）。

    这些测试验证脚本逻辑；Windows 可执行文件本身无法在 Linux 上运行，
    因此不验证 depscope.exe 的实际执行结果。
    """

    def run_ps(self, *extra: str, env_overrides: dict[str, str] | None = None):
        env = {"PROCESSOR_ARCHITECTURE": "AMD64", "OS": "Windows_NT"}
        if env_overrides:
            env.update(env_overrides)
        return run_ps1(self.base, self.install_dir, *extra, env_overrides=env, ca_path=self.ca_path)

    def test_help(self) -> None:
        """-Help 打印用法并成功退出。"""
        result = self.run_ps("-Help")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("Install the DepScope CLI", result.stdout)

    def test_unsupported_arch_rejected(self) -> None:
        """不支持的处理器架构在下载前失败。"""
        result = self.run_ps(env_overrides={"PROCESSOR_ARCHITECTURE": "x86"})
        self.assertNotEqual(result.returncode, 0)
        combined = result.stdout + result.stderr
        self.assertIn("unsupported processor architecture", combined)

    def test_invalid_version_rejected(self) -> None:
        """版本标签含非法字符时被拒绝。"""
        result = self.run_ps("-Version", "../../evil")
        self.assertNotEqual(result.returncode, 0)
        combined = result.stdout + result.stderr
        self.assertIn("invalid version tag", combined)

    def test_end_to_end_install_from_mock_release(self) -> None:
        """完整路径：下载 ZIP、校验、解压并安装 depscope.exe。"""
        result = self.run_ps()
        combined = result.stdout + result.stderr
        self.assertEqual(result.returncode, 0, combined)
        target = self.install_dir / "depscope.exe"
        self.assertTrue(target.is_file(), f"depscope.exe not installed. output: {combined}")
        self.assertIn("checksum verified", combined)
        self.assertEqual(sorted(p.name for p in self.install_dir.iterdir()), ["depscope.exe"])

    def test_checksum_mismatch_fails_closed(self) -> None:
        """校验和不匹配时失败，且不安装任何文件。"""
        key = f"/releases/download/{TAG}/depscope_cli_{ASSET_VERSION}_windows_amd64.zip"
        good = routes[key][1]
        routes[key] = (200, good + b"tampered")
        result = self.run_ps()
        combined = result.stdout + result.stderr
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("checksum verification failed", combined)
        self.assertFalse((self.install_dir / "depscope.exe").exists())

    def test_missing_checksums_entry_fails_closed(self) -> None:
        """checksums.txt 未登记该资产时失败。"""
        key = f"/releases/download/{TAG}/checksums.txt"
        body = routes[key][1].replace(f"  depscope_cli_{ASSET_VERSION}_windows_amd64.zip".encode(), b"")
        routes[key] = (200, b"\n".join(l for l in body.split(b"\n") if l.strip()) + b"\n")
        result = self.run_ps()
        combined = result.stdout + result.stderr
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("does not list", combined)
        self.assertFalse((self.install_dir / "depscope.exe").exists())

    def test_replaces_existing_binary(self) -> None:
        """已存在的 depscope.exe 在验证后被替换，且无临时文件残留。"""
        self.install_dir.mkdir(parents=True)
        (self.install_dir / "depscope.exe").write_text("old")
        result = self.run_ps()
        combined = result.stdout + result.stderr
        self.assertEqual(result.returncode, 0, combined)
        self.assertIn("replaced the previous", combined)
        self.assertEqual(sorted(p.name for p in self.install_dir.iterdir()), ["depscope.exe"])

    def test_artifact_selection_matches_release_names(self) -> None:
        """选定资产名与 GoReleaser 命名一致（ZIP + windows/arch）。"""
        script = INSTALL_PS1.read_text()
        self.assertIn('depscope_cli_${assetVersion}_windows_${arch}.zip', script)
        self.assertIn('"$ReleaseBase/download/$Version"', script)


class PortableSyntaxTests(unittest.TestCase):
    """静态检查：脚本语法与安全约束。"""

    def test_install_sh_is_posix_syntax(self) -> None:
        result = subprocess.run(["sh", "-n", str(INSTALL_SH)], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_install_sh_has_no_bashisms(self) -> None:
        if shutil.which("checkbashisms") is None:
            self.skipTest("checkbashisms not installed")
        result = subprocess.run(["checkbashisms", str(INSTALL_SH)], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_shellcheck_when_available(self) -> None:
        if shutil.which("shellcheck") is None:
            self.skipTest("shellcheck not installed")
        result = subprocess.run(["shellcheck", "-s", "sh", str(INSTALL_SH)], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout)

    def test_installer_never_evaluates_or_sudos(self) -> None:
        text = INSTALL_SH.read_text()
        for forbidden in ("sudo", "eval ", "sh -c", "bash -c"):
            self.assertNotIn(forbidden, text, f"installer must not contain {forbidden!r}")

    def test_checksum_check_precedes_extraction(self) -> None:
        """校验必须出现在解压之前，防止执行未验证内容。"""
        text = INSTALL_SH.read_text()
        verify_at = text.index("checksum verification failed")
        extract_at = text.index("could not extract")
        self.assertLess(verify_at, extract_at, "checksum verification must precede extraction")


if __name__ == "__main__":
    unittest.main(verbosity=2)
