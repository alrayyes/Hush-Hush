# Tasks

## 1. Fix the stale documentation

- [x] 1.1 Rewrite `cmd/hush-hush/web/.gitignore`'s comment to match the
      root `.gitignore`/`embed.go`/`CONTRIBUTING.md`'s explanation -
      drop the false "not ignored" claim, the closed-#208 reference, and
      the "run `bun run build` before committing" instruction; verify by
      reading it alongside the other three for consistency.
- [x] 1.2 Fix `cmd/hush-hush/web/README.md`'s pointer to the comment so
      it no longer sends readers to a wrong explanation.

## 2. Replace the placeholder

- [x] 2.1 Replace `cmd/hush-hush/web/build/index.html`'s current
      content (real build output from #468) with a static, minimal
      placeholder documenting what it's for; verify with
      `go build ./... && go vet ./... && go test ./...` and
      `golangci-lint run` all passing unchanged.

## 3. Verification

- [x] 3.1 Confirm no other file references the specific hashed asset
      filenames the old placeholder happened to contain (`grep -rn` for
      `_app/immutable` outside `build/`), so removing them doesn't break
      anything relying on their exact names.
