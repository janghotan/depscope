# 变更说明

## What changed

<!-- 简述用户可见的变更。如果是维护者内部的清理，也请说明。 -->

## Scope and risk impact

- [ ] This change does not alter scan status semantics, risk rules, or the report contract.
- [ ] This change alters scan status semantics, risk rules, or the report contract, and `schemas/`, Go types, and website types were updated together.

## Checks actually run

Paste the exact commands you executed and their result. Delete lines you did not run.

```sh
gofmt -l .
go vet ./...
go build -trimpath ./...
go test -count=1 ./...
go mod verify
```

```sh
cd web
pnpm install --frozen-lockfile
pnpm check:contract
pnpm lint
pnpm typecheck
pnpm test
pnpm build
pnpm exec playwright install chromium
pnpm test:e2e
```

## Notes for reviewers

<!-- 有截图请附上网站或终端截图；说明已知限制、后续工作或未完成的事项。 -->

- [ ] New or changed source-code comments and docstrings are in Simplified Chinese.
- [ ] The change does not install, import, or execute a scanned package, and does not run lifecycle scripts.
- [ ] Missing, skipped, failed, or unavailable checks still surface as `incomplete`, never as `clean`.
- [ ] No credentials, `.env` files, generated build outputs, or machine-specific artifacts are included.
- [ ] Documentation was updated where behavior changed.
