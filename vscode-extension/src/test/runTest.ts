import * as path from "path";
import { runTests } from "@vscode/test-electron";

async function main() {
  try {
    const extensionDevelopmentPath = path.resolve(__dirname, "../../");
    const extensionTestsPath = path.resolve(__dirname, "./suite/index");
    const testWorkspace = path.resolve(
      __dirname,
      "../../../testdata/fixtures/projects/npm-context"
    );

    // 运行测试并启动 Extension Development Host，直接使用系统已安装的 VS Code
    await runTests({
      vscodeExecutablePath: "/usr/bin/code",
      extensionDevelopmentPath,
      extensionTestsPath,
      launchArgs: [testWorkspace, "--disable-extensions"],
    });
  } catch (err) {
    console.error("Failed to run tests:", err);
    process.exit(1);
  }
}

main();
