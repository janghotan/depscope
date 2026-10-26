# Release engineering

## Current release state

The repository contains CI, cross-platform Go build configuration, and a GoReleaser package definition. No release has been published.

Status of the two former release gates (repository preparation, 2026-10-26):

1. **License file:** **Resolved.** `LICENSE` carries the Apache License 2.0 with only the appendix copyright line set to `Copyright 2026 Jangho Tan`; `NOTICE` repeats that attribution. The license terms themselves are unmodified. The tagged-release workflow only checks that a `LICENSE` file exists; it does not inspect the copyright line.
2. **Go module path:** `go.mod` uses `github.com/janghotan/depscope`, derived from the authenticated GitHub account and the intended repository name `depscope`. Changing the owner or repository name requires updating `go.mod` and every import.

The repository is published at `https://github.com/janghotan/depscope` with the local history pushed as the default branch `main`. There is still no website host, API host, signing identity, or deployment credential configured. Binary releases do not deploy either service. Publishing a release remains blocked until a version is chosen and the maintainer explicitly authorizes the tag.

GoReleaser's `check` command also expects an SCM origin from which it can determine the release owner and repository. Before the repository was published there was no remote, and `goreleaser check` failed with `configuration is invalid: scm releases: no remote configured to list refs from`. With `origin` pointing at `https://github.com/janghotan/depscope`, it now reports `1 configuration file(s) validated`. Never substitute a guessed remote URL. `goreleaser release --snapshot --clean` builds the local package configuration without publishing and does not need the remote.

## Release targets and artifacts

GoReleaser builds `depscope` and `depscope-api` for Linux, macOS, and Windows on `amd64` and `arm64`, with CGO disabled. It creates separate CLI and API archives: `.tar.gz` for Unix targets, `.zip` for Windows, plus a `checksums.txt` file containing SHA-256 checksums. Build paths are trimmed, module timestamps use the source commit timestamp, and packaged file timestamps use a fixed value for reproducible artifacts.

Each archive contains the `depscope` or `depscope-api` binary, `README.md`, `LICENSE`, `NOTICE`, `docs/*.md`, and `schemas/scan-report.v1.schema.json`.

## Installers

`install.sh` (POSIX sh, Linux and macOS) and `install.ps1` (Windows PowerShell) install the CLI from the latest published release. They are part of the release contract: if an asset is renamed, the installers must change in the same commit.

| Rule | Detail |
| --- | --- |
| Asset selection | `depscope_cli_<version>_<os>_<arch>.tar.gz`, or `.zip` on Windows. `<version>` is the tag without its leading `v`, matching GoReleaser's default `Version`. |
| Checksum order | `checksums.txt` is fetched first; the archive must be listed before it is downloaded. The SHA-256 is verified before extraction or execution. |
| Transport | HTTPS only. Plain HTTP is refused, and certificate verification cannot be disabled. `DEPSCOPE_CA_BUNDLE` (Unix) only adds a trusted CA for the test suite. |
| Installation | Only the `depscope` binary; `depscope-api` is never installed. The existing binary is replaced only after the new one is verified and staged. |
| Privileges | Never root, never `sudo`, never administrator. |
| Rollback | There is none: the previous binary is overwritten in place. Re-run the installer with `--version`/`-Version` to move between releases. |

Automated coverage lives in `scripts/test_installers.py`. It serves a local HTTPS release built from real archives and covers OS/architecture detection, asset and checksum selection, checksum mismatch, missing assets, HTTP errors, unsupported targets, missing tools, existing-install replacement, and temporary-file cleanup. Those tests exercise simulated network responses; they do not prove that the published assets download. Verify a real install after every release. `NOTICE` is included because Apache License 2.0 section 4(d) asks a distributor that ships a `NOTICE` file to carry its attribution notices along; omitting it from an archive would make that obligation unmet.

CI cross-compiles both command entry points for all six OS/architecture combinations. This proves the binaries compile for those targets; it is not runtime testing on each platform. Users should verify the archive checksum before extracting or running the binaries.

## Local package validation

Install Go 1.26.10 and GoReleaser v2.18.2, then run from the repository root.

Go 1.26.10 builds the project, but `go install github.com/goreleaser/goreleaser/v2@v2.18.2` needs Go 1.27 or newer for its own module. `go install` therefore selects a newer toolchain automatically; this does not change the `go 1.26.10` directive in `go.mod`.

```sh
goreleaser check
goreleaser release --snapshot --clean
```

The snapshot command builds local artifacts under `dist/` and does not create a GitHub release. Inspect both binaries and the checksum file. To verify an archive checksum, run `sha256sum --check checksums.txt --ignore-missing` on Linux or `shasum -a 256 --check checksums.txt` on macOS.

## Tagged release procedure

After the copyright notices, repository owner and name, repository host, and release contents are approved:

1. Run the complete checks in [CONTRIBUTING.md](../CONTRIBUTING.md) and resolve every release-blocking security finding.
2. Run `goreleaser check` and `goreleaser release --snapshot --clean`; inspect the archives and checksums.
3. Review and approve the version and release notes.
4. Push a signed `vMAJOR.MINOR.PATCH` tag only after explicit maintainer approval. The `.github/workflows/release.yml` workflow then rechecks that `LICENSE` exists and that `go.mod` does not use the old provisional path, then publishes GoReleaser artifacts using the repository-scoped GitHub token.
5. Verify the published checksum file and assets from a clean download. Record any platform runtime checks separately from cross-compilation results.

The workflow is configured but must not be triggered until the copyright notices are completed and release publication is explicitly authorized.

## Website and API deployment

No deployment target or hosting account is configured. The website and API can be run locally using the commands in [the README](../README.md), but public operation needs a separate review of authentication, network isolation, CORS, TLS termination, rate limits, source terms, monitoring, and incident handling. No hosted service is implied by a tagged binary release.
