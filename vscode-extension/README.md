# DepScope VS Code Extension

The `depscope-vscode` extension integrates the DepScope dependency security scanner directly into VS Code. It provides in-editor dependency inspection, vulnerability detection, and supply-chain risk analysis for npm and PyPI projects.

[![Visual Studio Marketplace Version](https://img.shields.io/visual-studio-marketplace/v/JanghoTan.depscope-vscode)](https://marketplace.visualstudio.com/items?itemName=JanghoTan.depscope-vscode)

## Installation

Install the extension directly from the Visual Studio Marketplace:

- **VS Code Marketplace**: [Install DepScope from Marketplace](https://marketplace.visualstudio.com/items?itemName=JanghoTan.depscope-vscode)
- **Command Line**:
  ```sh
  code --install-extension JanghoTan.depscope-vscode
  ```

## Capabilities

- **Scan Wizard** — A self-contained graphical wizard to detect workspace dependency manifests (`package.json`, `requirements.txt`), select targets, and observe real-time scanning progress without using the Command Palette.
- **Categorized Tree View** — An Activity Bar sidebar view with 4 strict groups:
  1. **Confirmed vulnerabilities** — OSV-matched known CVE / GHSA advisories.
  2. **Heuristic signals** — npm lifecycle scripts, typosquatting similarity warnings.
  3. **Incomplete coverage** — Dependencies with missing, unavailable, or failed coverage checks. In accordance with DepScope principles, incomplete coverage is never misclassified as clean.
  4. **Clean** — Packages where every check succeeded with zero findings.
- **Finding Detail Webviews** — Rich vulnerability reports displaying advisory IDs with links, severity badges, risk rationale, remediation guidance, and collapsible reference links.
- **In-Editor Diagnostics** — Direct integration with VS Code's Problems panel, highlighting vulnerable dependency declaration lines in `package.json` and `requirements.txt`.
- **Status Bar Integration** — Displays real-time scan status, vulnerability counts, and incomplete coverage alerts with click-to-open wizard access.

## Development

### Prerequisites

The extension acts as a client to the local `depscope-api` service. It does not bundle the Go binary.

Before running the extension, compile `depscope-api` and ensure it is available either on your system `PATH`, inside your workspace `./bin/` folder, or in `node_modules/.bin/`:

```sh
# From the repository root
go build -trimpath -o bin/depscope-api ./cmd/depscope-api
```

### Build and Run Locally

1. Install Node.js (v20+ recommended) and install extension dependencies:

```sh
cd vscode-extension
npm install
```

2. Compile the TypeScript sources:

```sh
npm run compile
```

3. Launch the Extension Development Host:
   - Open the `vscode-extension` folder or repository root in VS Code.
   - Press `F5` (or select **Run > Start Debugging**) to start a new Extension Development Host window.

### Running Tests

To run the automated integration test suite in a headless VS Code instance:

```sh
npm test
```

## Usage Walkthrough

1. Open any workspace containing `package.json` or `requirements.txt`.
2. Click the **DepScope** icon in the Activity Bar sidebar.
3. If no scan has been performed, click the **Scan Workspace** welcome button or the `$(play)` toolbar icon.
4. In Step 1 of the Scan Wizard, confirm which manifests to inspect, then click **Scan Workspace**.
5. Once completed, explore the 4-tier tree view and click any package leaf to review detailed findings in the side Webview panel.
6. Open your `package.json` or `requirements.txt` to inspect squiggly problem markers on vulnerable dependencies.
