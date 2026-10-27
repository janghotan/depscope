# Model Context Protocol (MCP) Integration

DepScope provides an official Model Context Protocol (MCP) server integration using the official Go MCP SDK (`github.com/modelcontextprotocol/go-sdk`) and `stdio` transport.

This integration allows MCP-compatible AI coding assistants and agent environments (such as Claude Desktop, Cursor, Roo Code, Windsurf, or custom agent runtimes) to invoke DepScope's dependency security scanning and risk assessment capabilities locally.

---

## Capabilities & Exposed Tools

The DepScope MCP server exposes four specialized tools, strictly mapped to existing security engine and project scanner features:

### 1. `scan_project`
Scans a permitted project directory for dependency security issues using supported package manifests and lockfiles.

- **Input Parameters**:
  - `path` (string, required): Filesystem path to the project directory. Must be within permitted directories.
- **Supported Formats**:
  - npm `package.json` with `package-lock.json` (lockfileVersion 2 or 3).
  - Python `requirements.txt` with exact `==` or `===` pins.
  - *Unsupported formats (e.g. yarn.lock, poetry.lock, pnpm-lock.yaml) are recorded as limitations and incomplete coverage, never silently guessed.*
- **Output**:
  - `scan_id`: Unique session identifier for caching and subsequent queries.
  - `status`: Overall scan status (`clean`, `findings`, `incomplete`, `failed`).
  - `summary`: High-level counts of confirmed findings, heuristic warnings, and severity breakdown.
  - `formats`: Breakdown of detected manifests, resolved package count, unresolved package count, and resolution completeness.
  - `findings`: Array of findings with affected package, severity, dependency relationship (`direct` or `transitive`), scopes (`production`, `development`), and risk scores.
  - `limitations`: Coverage limitations and unsupported formats encountered.
  - `sources`: Consulted data sources and timestamps.

### 2. `scan_package`
Checks an individual package and version using DepScope's security engine, querying OSV.dev and package registries.

- **Input Parameters**:
  - `package` (string, required): Package specifier (e.g., `lodash`, `@scope/pkg@1.2.3`, `npm:lodash@4.17.21`, or `pypi:requests==2.31.0`).
  - `version` (string, optional): Specific version to check. If omitted, the latest version is resolved.
  - `ecosystem` (string, optional): Target ecosystem (`npm` or `pypi`). Defaults to `npm`.
- **Output**:
  - `scan_id`: Unique session identifier for querying details.
  - `target`: Canonical package identity and resolved version.
  - `status`: `clean`, `findings`, `incomplete`, or `failed`.
  - `summary`: Severity counts and confirmed findings count.
  - `findings`: Security findings and heuristic warnings.
  - `sources`: Data sources consulted.

### 3. `get_finding_details`
Returns comprehensive finding evidence, upstream advisory identifiers, advisory URLs, and full dependency context.

- **Input Parameters**:
  - `finding_id` (string, required): The unique finding ID (e.g. `GHSA-...`, `CVE-...`, or project finding ID).
  - `scan_id` (string, optional): The scan ID in which the finding occurred. If omitted, recent scans are searched.
- **Output**:
  - `finding_id`, `kind`, `title`, `description`, `severity`.
  - `package`: Canonical ecosystem and package name.
  - `risk`: Risk assessment score and explanation.
  - `evidence`: Advisory references and dependency path evidence.
  - `sources`: Detailed source metadata with URLs.
  - `dependency_context`: Dependency relationship (`direct` or `transitive`), scopes (`production`, `development`, `optional`), resolution completeness, and representative dependency paths.

### 4. `get_scan_summary`
Returns structured scan totals, severity counts, and dependency resolution completeness.

- **Input Parameters**:
  - `scan_id` (string, optional): Previous scan session ID.
  - `path` (string, optional): Project path to scan and summarize if `scan_id` is not provided.
  - `package` (string, optional): Package specifier to scan and summarize if `scan_id` is not provided.
- **Output**:
  - `totals`: Confirmed findings, heuristic warnings, highest severity, counts per severity level (`critical`, `high`, `medium`, `low`, `unknown`).
  - `resolution`: `complete`, `partial`, or `unknown`.
  - `has_unresolved_dependencies`: Boolean flag indicating incomplete dependency resolution.
  - `message`: Descriptive summary explaining the security posture.

---

## Security Boundary & Guardrails

The DepScope MCP server is designed around strict defensive principles:

1. **Read-Only Inspection**:
   - The server **never executes** package code, build scripts, npm lifecycle hooks (`postinstall`, etc.), or external commands.
   - It only reads and parses project manifest and lockfile texts.
2. **Filesystem Path Restriction**:
   - By default, project directory scanning is restricted to the server process's current working directory.
   - Additional directories must be explicitly allowlisted using `--allowed-path <directory>`.
   - Path traversal attempts (`../`) and unauthorized symlink traversals outside allowed directories are strictly blocked with an access denial error.
3. **No False Safety Assurances**:
   - A scan result with zero findings is reported as `clean`, but the server explicitly notes that consulted sources have no known advisories and does not guarantee complete absence of vulnerabilities.
   - Incomplete resolutions, missing lockfiles, or unpinned dependencies are explicitly reported as `incomplete` or `partial`.
4. **Clean Protocol Stream**:
   - The MCP stdio transport runs over `stdin` and `stdout`.
   - All server diagnostics, startup messages, and operational logs are strictly written to `stderr`.

---

## Setup & Client Configuration

You can launch the MCP server through either the unified `depscope` CLI or the dedicated `depscope-mcp` binary:

```bash
# Using depscope CLI
depscope mcp --allowed-path /path/to/my-project

# Using depscope-mcp binary
depscope-mcp --allowed-path /path/to/my-project
```

### Claude Desktop Configuration

Add the server to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "depscope": {
      "command": "depscope",
      "args": [
        "mcp",
        "--allowed-path",
        "/home/user/projects"
      ]
    }
  }
}
```

Or using the dedicated binary:

```json
{
  "mcpServers": {
    "depscope": {
      "command": "/usr/local/bin/depscope-mcp",
      "args": [
        "--allowed-path",
        "/home/user/projects"
      ]
    }
  }
}
```

### Cursor / Roo Code / Generic MCP Client

```json
{
  "depscope": {
    "command": "depscope",
    "args": ["mcp", "--allowed-path", "."]
  }
}
```

---

## Limitations

- **Ecosystems**: Only `npm` and `PyPI` are supported in this release.
- **Lockfile Support**:
  - npm requires `package-lock.json` with `lockfileVersion` 2 or 3.
  - Python requires `requirements.txt` with exact `==` or `===` version pins.
  - Transitive resolution for unpinned Python dependencies, `yarn.lock`, `pnpm-lock.yaml`, `poetry.lock`, and `uv.lock` are not supported and are flagged as unsupported.
- **Transport**: Only local `stdio` transport is supported. Network HTTP/SSE transports and write-capable remediation tools are intentionally excluded.
