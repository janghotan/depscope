#!/bin/sh
# DepScope CLI 安装脚本（POSIX sh，适用于 Linux 与 macOS）。
#
# 用法：
#   curl -fsSL https://raw.githubusercontent.com/janghotan/depscope/main/install.sh | sh
#   sh install.sh --version v0.1.0 --dir "$HOME/bin"
#
# 安装流程：确定平台 → 下载 checksums.txt 与对应归档 → 校验 SHA-256 →
# 仅解压 CLI 归档中的 depscope 可执行文件 → 原子替换目标文件。
# 校验失败时不会解压或执行任何下载内容。
set -eu

REPO="janghotan/depscope"
BIN_NAME="depscope"
DEFAULT_DIR="${HOME:-}/.local/bin"

# 可通过环境变量覆盖的可选设置。
INSTALL_DIR="${DEPSCOPE_INSTALL_DIR:-$DEFAULT_DIR}"
RELEASE_BASE="${DEPSCOPE_RELEASE_BASE:-https://github.com/$REPO/releases}"
API_BASE="${DEPSCOPE_API_BASE:-https://api.github.com/repos/$REPO}"
REQUESTED_VERSION=""

TMP_DIR=""

# 输出安装进度提示。
info() {
	printf 'depscope: %s\n' "$*" >&2
}

# 输出错误并以非零状态退出。
fail() {
	printf 'depscope: error: %s\n' "$*" >&2
	exit 1
}

# 退出时清理临时目录。
cleanup() {
	if [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ]; then
		rm -rf "$TMP_DIR"
	fi
}
trap cleanup EXIT HUP INT TERM

# 打印用法说明。
usage() {
	cat <<'EOF'
Install the DepScope CLI from GitHub Releases.

Usage:
  install.sh [options]

Options:
  --dir <path>        Install directory (default: $HOME/.local/bin)
  --version <tag>     Release tag to install, for example v0.1.0
                      (default: the latest published CLI release)
  -h, --help          Show this help

Environment:
  DEPSCOPE_INSTALL_DIR   Same as --dir
  DEPSCOPE_VERSION       Same as --version
  DEPSCOPE_RELEASE_BASE  Override the release download base URL
                         (default: https://github.com/janghotan/depscope/releases)
  DEPSCOPE_API_BASE      Override the GitHub API base URL
                         (default: https://api.github.com/repos/janghotan/depscope)
  DEPSCOPE_CA_BUNDLE     Extra CA bundle trusted for downloads; used by the
                         test suite. TLS certificate verification is never
                         disabled, and plain HTTP is always refused.

Supported platforms: Linux and macOS on amd64 and arm64.

The archive is verified against the release checksums.txt before it is
extracted. An existing depscope binary is replaced only after the new
binary has been verified and staged next to it.
EOF
}

# 解析命令行参数；未知参数直接失败。
parse_args() {
	while [ $# -gt 0 ]; do
		case "$1" in
		--dir)
			[ $# -ge 2 ] || fail "--dir requires a path"
			INSTALL_DIR="$2"
			shift 2
			;;
		--dir=*)
			INSTALL_DIR="${1#--dir=}"
			shift
			;;
		--version)
			[ $# -ge 2 ] || fail "--version requires a tag"
			REQUESTED_VERSION="$2"
			shift 2
			;;
		--version=*)
			REQUESTED_VERSION="${1#--version=}"
			shift
			;;
		-h | --help)
			usage
			exit 0
			;;
		*)
			fail "unknown argument: $1 (run with --help)"
			;;
		esac
	done
	# 未通过命令行指定版本时，回退到 DEPSCOPE_VERSION。
	if [ -z "$REQUESTED_VERSION" ] && [ -n "${DEPSCOPE_VERSION:-}" ]; then
		REQUESTED_VERSION="$DEPSCOPE_VERSION"
	fi
}

# 检查必需的下载与校验命令是否可用。
require_commands() {
	missing=""
	for cmd in curl tar mktemp; do
		command -v "$cmd" >/dev/null 2>&1 || missing="$missing $cmd"
	done
	if command -v sha256sum >/dev/null 2>&1; then
		SHA256_CMD="sha256sum"
	elif command -v shasum >/dev/null 2>&1; then
		SHA256_CMD="shasum -a 256"
	elif command -v openssl >/dev/null 2>&1; then
		SHA256_CMD="openssl"
	else
		missing="$missing sha256sum|shasum|openssl"
	fi
	if [ -n "$missing" ]; then
		fail "missing required command(s):${missing}"
	fi
}

# 识别操作系统；仅接受 Linux 与 Darwin。
detect_os() {
	uname_s="$(uname -s 2>/dev/null || true)"
	case "$uname_s" in
	Linux) OS="linux" ;;
	Darwin) OS="darwin" ;;
	*) fail "unsupported operating system '$uname_s'; DepScope provides binaries for Linux and macOS only" ;;
	esac
}

# 识别 CPU 架构并映射到 GoReleaser 使用的架构名。
detect_arch() {
	uname_m="$(uname -m 2>/dev/null || true)"
	case "$uname_m" in
	x86_64 | amd64) ARCH="amd64" ;;
	aarch64 | arm64) ARCH="arm64" ;;
	*) fail "unsupported architecture '$uname_m'; DepScope provides amd64 and arm64 binaries" ;;
	esac
}

# 统一的下载入口：只允许 HTTPS，并始终校验 TLS 证书。
# DEPSCOPE_CA_BUNDLE 用于在测试中信任本地自签名 CA；它只增加受信证书，
# 不会关闭证书校验，也不存在跳过校验的开关。
fetch() {
	url="$1"
	shift
	if [ -n "${DEPSCOPE_CA_BUNDLE:-}" ]; then
		curl -fsSL --proto '=https' --tlsv1.2 --cacert "$DEPSCOPE_CA_BUNDLE" "$@" "$url"
	else
		curl -fsSL --proto '=https' --tlsv1.2 "$@" "$url"
	fi
}

# 从 GitHub API 查询最新已发布（非预发布）版本标签。
resolve_latest_tag() {
	api_json="$(fetch "$API_BASE/releases/latest" \
		-H 'Accept: application/vnd.github+json')" ||
		fail "could not resolve the latest release from $API_BASE/releases/latest; pass --version <tag> instead"

	tag="$(printf '%s' "$api_json" | tr ',' '\n' | grep '"tag_name"' | head -n 1 |
		sed -e 's/.*"tag_name"[[:space:]]*:[[:space:]]*"//' -e 's/".*$//')"
	[ -n "$tag" ] || fail "the latest release response did not contain a tag_name field"
	VERSION="$tag"
}

# 校验版本标签格式，避免拼出任意 URL 路径。
validate_version() {
	case "$VERSION" in
	'' | *[!A-Za-z0-9._-]*) fail "invalid version tag '$VERSION'" ;;
	esac
}

# 计算给定文件的 SHA-256 摘要，只输出十六进制字符串。
sha256_of() {
	case "$SHA256_CMD" in
	openssl) openssl dgst -sha256 "$1" | sed 's/^.*= *//' ;;
	*) $SHA256_CMD "$1" ;;
	esac | awk '{print $1}'
}

# 从 checksums.txt 中查找目标文件的期望摘要；缺失则失败。
expected_checksum() {
	awk -v want="$1" '{ name = $2; sub(/^\*/, "", name); if (name == want) { print $1; exit } }' "$2"
}

main() {
	parse_args "$@"
	require_commands
	detect_os
	detect_arch

	case "$OS" in
	linux | darwin) ;;
	*) fail "unsupported platform $OS" ;;
	esac

	TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/depscope-install.XXXXXX")" ||
		fail "could not create a temporary directory"
	TMP_DIR="$(cd "$TMP_DIR" && pwd -P)"

	if [ -n "$REQUESTED_VERSION" ]; then
		VERSION="$REQUESTED_VERSION"
	else
		resolve_latest_tag
	fi
	validate_version

	# GoReleaser 去掉标签前缀 v 作为归档版本；文件名版本不含前导 v。
	asset_version="${VERSION#v}"
	archive="depscope_cli_${asset_version}_${OS}_${ARCH}.tar.gz"
	release_url="$RELEASE_BASE/download/${VERSION}"

	info "installing DepScope $VERSION for $OS/$ARCH"

	# 1. 下载校验清单并确认目标归档已登记在清单中。
	fetch "$release_url/checksums.txt" -o "$TMP_DIR/checksums.txt" ||
		fail "could not download checksums.txt for $VERSION from $release_url"

	expected="$(expected_checksum "$archive" "$TMP_DIR/checksums.txt")"
	[ -n "$expected" ] ||
		fail "checksums.txt for $VERSION does not list $archive; this release has no CLI binary for $OS/$ARCH"

	# 2. 下载归档；失败即中止，不解压任何内容。
	fetch "$release_url/$archive" -o "$TMP_DIR/$archive" ||
		fail "could not download $archive from $release_url"

	# 3. 在任何解压或执行之前校验 SHA-256。
	actual="$(sha256_of "$TMP_DIR/$archive")"
	if [ "$actual" != "$expected" ]; then
		fail "checksum verification failed for $archive (expected $expected, got $actual)"
	fi
	info "checksum verified: $archive"

	# 4. 只解压归档中的 depscope 可执行文件。
	mkdir -p "$TMP_DIR/extract" || fail "could not create extraction directory"
	if ! tar -xzf "$TMP_DIR/$archive" -C "$TMP_DIR/extract" >/dev/null 2>&1; then
		fail "could not extract $archive"
	fi

	staged="$(find "$TMP_DIR/extract" -type f -name "$BIN_NAME" -print 2>/dev/null | head -n 1)"
	[ -n "$staged" ] || fail "archive $archive does not contain the $BIN_NAME executable"
	[ -f "$staged" ] || fail "$BIN_NAME in $archive is not a regular file"

	# 5. 确认解压出的文件确实是可执行文件，再安装到目标目录。
	[ -s "$staged" ] || fail "$BIN_NAME in $archive is empty"

	# 用 if 明确区分“chmod 失败”与“版本命令失败”，不使用 A && B || C 的写法。
	reported_version=""
	if chmod +x "$staged"; then
		if reported_version="$("$staged" version 2>/dev/null)"; then
			:
		else
			reported_version=""
		fi
	fi
	case "$reported_version" in
	"$BIN_NAME "*) ;;
	*) fail "the extracted $BIN_NAME did not report a version; refusing to install it" ;;
	esac

	mkdir -p "$INSTALL_DIR" || fail "could not create install directory $INSTALL_DIR"
	target="$INSTALL_DIR/$BIN_NAME"

	# 6. 在目标目录内暂存新二进制，再用 mv 原子替换；已有文件仅在此刻被替换。
	stage_target="$INSTALL_DIR/.$BIN_NAME.new.$$"
	if ! cp "$staged" "$stage_target" 2>/dev/null; then
		rm -f "$stage_target" 2>/dev/null || true
		fail "could not write to $INSTALL_DIR"
	fi
	chmod 755 "$stage_target" || {
		rm -f "$stage_target" 2>/dev/null || true
		fail "could not set permissions on $stage_target"
	}

	replaced=""
	if [ -e "$target" ]; then
		replaced="yes"
	fi
	mv -f "$stage_target" "$target" || {
		rm -f "$stage_target" 2>/dev/null || true
		fail "could not install $BIN_NAME to $target"
	}

	printf '\n'
	printf 'DepScope %s installed\n' "$reported_version"
	printf '  binary:  %s\n' "$target"
	if [ -n "$replaced" ]; then
		printf '  note:    replaced the previous %s at this path\n' "$BIN_NAME"
	fi
	printf '\n'

	case ":${PATH:-}:" in
	*":$INSTALL_DIR:"*)
		printf 'Run "%s version" to confirm the installation.\n' "$BIN_NAME"
		;;
	*)
		printf '%s is not on your PATH. Add it to your shell profile:\n' "$INSTALL_DIR"
		# shellcheck disable=SC2016 # 这里需要打印字面量 $PATH，而不是展开当前值。
		printf '\n  export PATH="%s:$PATH"\n\n' "$INSTALL_DIR"
		printf 'Then open a new shell and run "%s version".\n' "$BIN_NAME"
		;;
	esac
}

main "$@"
