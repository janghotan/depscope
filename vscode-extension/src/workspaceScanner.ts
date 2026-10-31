import * as fs from "fs";
import * as path from "path";
import * as vscode from "vscode";
import { scanPackage, ScanPackageRequest } from "./apiClient";
import { Ecosystem, ScanReport } from "./generated/report-types";

/**
 * 工作区发现的清单文件信息。
 */
export interface DiscoveredManifest {
  path: string;
  relativePath: string;
  type: "npm" | "pypi";
  dependencyCount: number;
}

/**
 * 工作区发现的依赖项声明。
 */
export interface DiscoveredDependency {
  ecosystem: Ecosystem;
  name: string;
  version?: string;
  sourceFile: string;
}

/**
 * 获取工作区根目录所有检测到的清单文件。
 */
export async function getDiscoveredManifests(): Promise<DiscoveredManifest[]> {
  const manifests: DiscoveredManifest[] = [];
  const packageJsonUris = await vscode.workspace.findFiles("**/package.json", "**/node_modules/**");
  const requirementsUris = await vscode.workspace.findFiles(
    "**/requirements*.txt",
    "**/{node_modules,.venv,venv,env}/**"
  );

  const rootFolder = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath || "";

  for (const uri of packageJsonUris) {
    try {
      const content = await fs.promises.readFile(uri.fsPath, "utf8");
      const pkgJson = JSON.parse(content) as {
        dependencies?: Record<string, string>;
        devDependencies?: Record<string, string>;
      };
      const deps = { ...(pkgJson.dependencies || {}), ...(pkgJson.devDependencies || {}) };
      const rel = rootFolder ? path.relative(rootFolder, uri.fsPath) : path.basename(uri.fsPath);
      manifests.push({
        path: uri.fsPath,
        relativePath: rel,
        type: "npm",
        dependencyCount: Object.keys(deps).length,
      });
    } catch {
      // 忽略无法读取的清单文件
    }
  }

  for (const uri of requirementsUris) {
    try {
      const content = await fs.promises.readFile(uri.fsPath, "utf8");
      const lines = content.split(/\r?\n/).filter((l) => {
        const t = l.trim();
        return t && !t.startsWith("#") && !t.startsWith("-");
      });
      const rel = rootFolder ? path.relative(rootFolder, uri.fsPath) : path.basename(uri.fsPath);
      manifests.push({
        path: uri.fsPath,
        relativePath: rel,
        type: "pypi",
        dependencyCount: lines.length,
      });
    } catch {
      // 忽略无法读取的清单文件
    }
  }

  return manifests;
}

/**
 * 从单个 package.json 提取依赖项。
 */
async function extractDependenciesFromPackageJson(filePath: string): Promise<DiscoveredDependency[]> {
  const results: DiscoveredDependency[] = [];
  try {
    const content = await fs.promises.readFile(filePath, "utf8");
    const pkgJson = JSON.parse(content) as {
      dependencies?: Record<string, string>;
      devDependencies?: Record<string, string>;
    };

    const deps = { ...(pkgJson.dependencies || {}), ...(pkgJson.devDependencies || {}) };
    if (Object.keys(deps).length === 0) {
      return results;
    }

    const lockPath = path.join(path.dirname(filePath), "package-lock.json");
    let lockPackages: Record<string, { version?: string }> | null = null;
    let lockDependencies: Record<string, { version?: string }> | null = null;

    if (fs.existsSync(lockPath)) {
      try {
        const lockRaw = await fs.promises.readFile(lockPath, "utf8");
        const parsed = JSON.parse(lockRaw) as {
          packages?: Record<string, { version?: string }>;
          dependencies?: Record<string, { version?: string }>;
        };
        lockPackages = parsed.packages || null;
        lockDependencies = parsed.dependencies || null;
      } catch {
        // 容忍损坏或格式不兼容的 lock 文件
      }
    }

    for (const [name, spec] of Object.entries(deps)) {
      let resolvedVersion: string | undefined = undefined;

      if (lockPackages) {
        const key = `node_modules/${name}`;
        if (lockPackages[key]?.version) {
          resolvedVersion = lockPackages[key].version;
        }
      }
      if (!resolvedVersion && lockDependencies && lockDependencies[name]?.version) {
        resolvedVersion = lockDependencies[name].version;
      }

      if (!resolvedVersion && typeof spec === "string") {
        const cleaned = spec.replace(/^[\^~=v\s]+/, "").trim();
        if (/^\d+\.\d+/.test(cleaned)) {
          resolvedVersion = cleaned.split(/\s+/)[0];
        }
      }

      results.push({
        ecosystem: "npm",
        name,
        version: resolvedVersion,
        sourceFile: filePath,
      });
    }
  } catch {
    // 忽略异常
  }
  return results;
}

/**
 * 从单个 requirements.txt 提取依赖项。
 */
async function extractDependenciesFromRequirementsTxt(filePath: string): Promise<DiscoveredDependency[]> {
  const results: DiscoveredDependency[] = [];
  try {
    const content = await fs.promises.readFile(filePath, "utf8");
    const lines = content.split(/\r?\n/);

    for (const rawLine of lines) {
      const line = rawLine.trim();
      if (!line || line.startsWith("#") || line.startsWith("-")) {
        continue;
      }

      const match = line.match(/^([a-zA-Z0-9_\-\.]+)\s*(?:==|===)\s*([a-zA-Z0-9_\-\.\+]+)/);
      if (match) {
        const rawName = match[1];
        const version = match[2];
        const name = rawName.toLowerCase().replace(/_/g, "-");

        results.push({
          ecosystem: "pypi",
          name,
          version,
          sourceFile: filePath,
        });
      }
    }
  } catch {
    // 忽略异常
  }
  return results;
}

/**
 * 扫描指定清单文件（若未指定则扫描全部）。
 *
 * @param baseUrl 本地 API 服务的基地址
 * @param manifestPaths 选定的清单文件绝对路径数组（为空则扫描全部）
 * @param progress 进度回调
 * @param token 取消令牌
 * @returns 扫描生成的报告列表
 */
export async function scanWorkspaceDependencies(
  baseUrl: string,
  manifestPaths?: string[],
  progress?: (completed: number, total: number, currentPackage: string) => void,
  token?: vscode.CancellationToken
): Promise<ScanReport[]> {
  let allDeps: DiscoveredDependency[] = [];

  if (manifestPaths && manifestPaths.length > 0) {
    for (const mPath of manifestPaths) {
      if (path.basename(mPath).toLowerCase().endsWith(".json")) {
        const deps = await extractDependenciesFromPackageJson(mPath);
        allDeps.push(...deps);
      } else {
        const deps = await extractDependenciesFromRequirementsTxt(mPath);
        allDeps.push(...deps);
      }
    }
  } else {
    const manifests = await getDiscoveredManifests();
    for (const m of manifests) {
      if (m.type === "npm") {
        allDeps.push(...(await extractDependenciesFromPackageJson(m.path)));
      } else {
        allDeps.push(...(await extractDependenciesFromRequirementsTxt(m.path)));
      }
    }
  }

  // 去重 (ecosystem:name)
  const uniqueDepsMap = new Map<string, DiscoveredDependency>();
  for (const dep of allDeps) {
    const key = `${dep.ecosystem}:${dep.name}`;
    if (!uniqueDepsMap.has(key)) {
      uniqueDepsMap.set(key, dep);
    }
  }
  const uniqueDeps = Array.from(uniqueDepsMap.values());

  if (uniqueDeps.length === 0) {
    return [];
  }

  const reports: ScanReport[] = [];
  const concurrency = 4;
  let currentIndex = 0;
  let completed = 0;

  const worker = async () => {
    while (currentIndex < uniqueDeps.length) {
      if (token?.isCancellationRequested) {
        break;
      }
      const dep = uniqueDeps[currentIndex++];
      try {
        const req: ScanPackageRequest = {
          ecosystem: dep.ecosystem,
          name: dep.name,
          version: dep.version,
        };
        const report = await scanPackage(baseUrl, req);
        reports.push(report);
      } catch (err) {
        console.warn(`Failed to scan ${dep.ecosystem}:${dep.name}@${dep.version}:`, err);
      } finally {
        completed++;
        progress?.(completed, uniqueDeps.length, dep.name);
      }
    }
  };

  const pool = Array.from({ length: Math.min(concurrency, uniqueDeps.length) }, () => worker());
  await Promise.all(pool);

  return reports;
}
