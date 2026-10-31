import * as child_process from "child_process";
import * as fs from "fs";
import * as net from "net";
import * as path from "path";
import * as vscode from "vscode";

// 保存当前运行的 API 子进程句柄与基地址。
let apiProcess: child_process.ChildProcess | null = null;
let apiBaseUrl: string | null = null;
let outputChannel: vscode.OutputChannel | null = null;

/**
 * 获取或创建专用的 DepScope API 输出通道。
 */
export function getOutputChannel(): vscode.OutputChannel {
  if (!outputChannel) {
    outputChannel = vscode.window.createOutputChannel("DepScope API");
  }
  return outputChannel;
}

/**
 * 检查当前工作区根目录是否包含 package.json 或 requirements.txt。
 */
export function hasWorkspaceManifests(): boolean {
  const folders = vscode.workspace.workspaceFolders;
  if (!folders || folders.length === 0) {
    return false;
  }
  for (const folder of folders) {
    const rootPath = folder.uri.fsPath;
    const pkgJson = path.join(rootPath, "package.json");
    const reqTxt = path.join(rootPath, "requirements.txt");
    if (fs.existsSync(pkgJson) || fs.existsSync(reqTxt)) {
      return true;
    }
  }
  return false;
}

/**
 * 解析 depscope-api 二进制路径：
 * 1. 优先检查各工作区根目录下的 node_modules/.bin/depscope-api；
 * 2. 依次在系统 PATH 中寻找 depscope-api 可执行文件；
 * 3. 检查工作区下的 ./bin/depscope-api 本地编译产物；
 * 4. 若未找到，提示用户并在控制台报错。
 */
export function resolveBinaryPath(): string | null {
  const isWindows = process.platform === "win32";
  const binaryNames = isWindows
    ? ["depscope-api.cmd", "depscope-api.exe", "depscope-api"]
    : ["depscope-api"];

  const folders = vscode.workspace.workspaceFolders || [];

  // 1. 检查工作区 node_modules/.bin
  for (const folder of folders) {
    const binDir = path.join(folder.uri.fsPath, "node_modules", ".bin");
    for (const name of binaryNames) {
      const candidate = path.join(binDir, name);
      if (fs.existsSync(candidate)) {
        return candidate;
      }
    }
  }

  // 2. 检查系统 PATH
  const envPath = process.env.PATH || "";
  const pathDirs = envPath.split(path.delimiter).filter(Boolean);
  for (const dir of pathDirs) {
    for (const name of binaryNames) {
      const candidate = path.join(dir, name);
      try {
        if (fs.existsSync(candidate)) {
          const stat = fs.statSync(candidate);
          if (stat.isFile()) {
            return candidate;
          }
        }
      } catch {
        // 忽略无权限或不存在的目录
      }
    }
  }

  // 3. 检查工作区本地 bin 目录或根目录构建文件
  for (const folder of folders) {
    const localCandidates = [
      path.join(folder.uri.fsPath, "bin", isWindows ? "depscope-api.exe" : "depscope-api"),
      path.join(folder.uri.fsPath, isWindows ? "depscope-api.exe" : "depscope-api"),
    ];
    for (const candidate of localCandidates) {
      if (fs.existsSync(candidate)) {
        return candidate;
      }
    }
  }

  return null;
}

/**
 * 寻找本地一个可用的随机 TCP 端口。
 */
export async function findFreePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.unref();
    server.on("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      if (!address || typeof address === "string") {
        server.close(() => reject(new Error("Unable to retrieve port from server address")));
        return;
      }
      const port = address.port;
      server.close((err) => {
        if (err) {
          reject(err);
        } else {
          resolve(port);
        }
      });
    });
  });
}

/**
 * 启动后台 depscope-api 进程，监听显式分配的空闲端口并返回基地址。
 */
export async function startApiProcess(): Promise<string> {
  if (apiBaseUrl && apiProcess && !apiProcess.killed) {
    return apiBaseUrl;
  }

  const binaryPath = resolveBinaryPath();
  if (!binaryPath) {
    const msg =
      "DepScope API binary (depscope-api) not found. Please install depscope-api or ensure it is in your PATH / bin.";
    void vscode.window.showErrorMessage(msg);
    throw new Error(msg);
  }

  const port = await findFreePort();
  const out = getOutputChannel();
  out.appendLine(`[DepScope] Found depscope-api binary at: ${binaryPath}`);
  out.appendLine(`[DepScope] Allocated free port: ${port}`);

  const cwd =
    vscode.workspace.workspaceFolders && vscode.workspace.workspaceFolders[0]
      ? vscode.workspace.workspaceFolders[0].uri.fsPath
      : process.cwd();

  const env = {
    ...process.env,
    DEPSCOPE_API_ADDR: `127.0.0.1:${port}`,
  };

  out.appendLine(`[DepScope] Spawning ${binaryPath} with DEPSCOPE_API_ADDR=127.0.0.1:${port} in ${cwd}`);

  return new Promise<string>((resolve, reject) => {
    let settled = false;
    let stderrBuffer = "";

    const timeout = setTimeout(() => {
      if (!settled) {
        settled = true;
        reject(
          new Error(
            `Timeout waiting for DepScope API to start on port ${port}. Logs: ${stderrBuffer}`
          )
        );
      }
    }, 15000);

    const child = child_process.spawn(binaryPath, [], {
      cwd,
      env,
      stdio: ["ignore", "pipe", "pipe"],
    });

    apiProcess = child;

    const handleOutput = (chunk: Buffer | string) => {
      const text = chunk.toString();
      stderrBuffer += text;
      out.append(text);

      if (!settled) {
        if (
          text.includes(`127.0.0.1:${port}`) ||
          text.includes("DepScope API listening")
        ) {
          settled = true;
          clearTimeout(timeout);
          apiBaseUrl = `http://127.0.0.1:${port}`;
          out.appendLine(`[DepScope] API process confirmed listening at ${apiBaseUrl}`);
          resolve(apiBaseUrl);
        }
      }
    };

    child.stdout?.on("data", handleOutput);
    child.stderr?.on("data", handleOutput);

    child.on("error", (err) => {
      out.appendLine(`[DepScope] Process error: ${err.message}`);
      if (!settled) {
        settled = true;
        clearTimeout(timeout);
        reject(err);
      }
    });

    child.on("exit", (code, signal) => {
      out.appendLine(`[DepScope] Process exited with code ${code}, signal ${signal}`);
      apiProcess = null;
      apiBaseUrl = null;
      if (!settled) {
        settled = true;
        clearTimeout(timeout);
        reject(
          new Error(
            `DepScope API process exited prematurely with code ${code}${
              stderrBuffer ? `: ${stderrBuffer}` : ""
            }`
          )
        );
      }
    });
  });
}

/**
 * 停止当前 API 子进程并清理句柄。
 */
export function stopApiProcess(): void {
  if (apiProcess && !apiProcess.killed) {
    if (outputChannel) {
      outputChannel.appendLine("[DepScope] Stopping API process...");
    }
    try {
      apiProcess.kill("SIGTERM");
    } catch {
      try {
        apiProcess.kill("SIGKILL");
      } catch {
        // 忽略强杀错误
      }
    }
    apiProcess = null;
  }
  apiBaseUrl = null;
}

/**
 * 获取当前 API 服务的基地址。若未启动则返回 null。
 */
export function getApiBaseUrl(): string | null {
  return apiBaseUrl;
}

/**
 * 查询 API 进程是否处于活动状态。
 */
export function isApiRunning(): boolean {
  return apiProcess !== null && !apiProcess.killed && apiBaseUrl !== null;
}
