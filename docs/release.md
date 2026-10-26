# Release engineering

## Current release state

The repository contains CI, cross-platform Go build configuration, and a GoReleaser package definition. No release has been published. The tagged-release workflow deliberately stops unless both of these maintainer decisions are complete:

1. Select and add an approved `LICENSE` file. No license is inferred or supplied by this repository.
2. Replace the provisional `module depscope` path in `go.mod` with the canonical module path after the repository's permanent hosting location is selected.

The repository has no remote URL, default branch, website host, API host, signing identity, or deployment credentials configured. Binary releases do not deploy either service.

GoReleaser's `check` command also expects an SCM origin from which it can determine the release owner and repository. In the current checkout, `goreleaser check` reports `no remote configured to list refs from`; do not substitute a guessed URL. `goreleaser release --snapshot --clean` validates and builds the local package configuration without needing a remote. Rerun `goreleaser check` after the maintainer configures the actual repository remote.

## Release targets and artifacts

GoReleaser builds `depscope` and `depscope-api` for Linux, macOS, and Windows on `amd64` and `arm64`, with CGO disabled. It creates separate CLI and API archives: `.tar.gz` for Unix targets, `.zip` for Windows, plus a `checksums.txt` file containing SHA-256 checksums. Build paths are trimmed, module timestamps use the source commit timestamp, and packaged file timestamps use a fixed value for reproducible artifacts.

CI cross-compiles both command entry points for all six OS/architecture combinations. This proves the binaries compile for those targets; it is not runtime testing on each platform. Users should verify the archive checksum before extracting or running the binaries.

## Local package validation

Install Go 1.26.10 and GoReleaser v2.18.2, then run from the repository root:

```sh
goreleaser check
goreleaser release --snapshot --clean
```

The snapshot command builds local artifacts under `dist/` and does not create a GitHub release. Inspect both binaries and the checksum file. To verify an archive checksum, run `sha256sum --check checksums.txt --ignore-missing` on Linux or `shasum -a 256 --check checksums.txt` on macOS.

## Tagged release procedure

After the license, canonical module path, repository host, and release contents are approved:

1. Run the complete checks in [CONTRIBUTING.md](../CONTRIBUTING.md) and resolve every release-blocking security finding.
2. Run `goreleaser check` and `goreleaser release --snapshot --clean`; inspect the archives and checksums.
3. Review and approve the version and release notes.
4. Push a signed `vMAJOR.MINOR.PATCH` tag only after explicit maintainer approval. The `.github/workflows/release.yml` workflow then rechecks the license and module path and publishes GoReleaser artifacts using the repository-scoped GitHub token.
5. Verify the published checksum file and assets from a clean download. Record any platform runtime checks separately from cross-compilation results.

The workflow is configured but must not be triggered until the blocking decisions are resolved and release publication is explicitly authorized.

## Website and API deployment

No deployment target or hosting account is configured. The website and API can be run locally using the commands in [the README](../README.md), but public operation needs a separate review of authentication, network isolation, CORS, TLS termination, rate limits, source terms, monitoring, and incident handling. No hosted service is implied by a tagged binary release.
