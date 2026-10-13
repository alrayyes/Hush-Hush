# Tasks

## 1. Server: bootstrap status endpoint

- [ ] 1.1 Add `handleAuthStatus` (reusing the `loadAdminUser` +
      credential-count check `registrationIsAuthorized` already uses) and
      register `GET /auth/status` in `internal/api/server.go`; verify with
      a new Go test asserting `{"bootstrapped": false}` with no admin
      account and `{"bootstrapped": true}` after one is created.
- [ ] 1.2 Add `/auth/status` to `api/openapi.yaml` (path, response schema,
      no security requirement) and verify `bun run lint:api` passes.

## 2. Client: status check and login page toggle

- [ ] 2.1 Add a `getAuthStatus()` call to `cmd/hush-hush/web/src/lib/api.ts`
      and verify with a unit test that it calls `GET /auth/status` and
      returns the parsed boolean.
- [ ] 2.2 Update `cmd/hush-hush/web/src/routes/login/+page.svelte` to fetch
      status on load and render exactly one primary action (register or
      log in) once resolved, an inline error with retry on failure, and
      neither while pending; verify with a component test covering all
      three states and `bun run svelte-autofixer`-clean markup.

## 3. Verification

- [ ] 3.1 `go build ./... && go vet ./... && go test ./...` and
      `golangci-lint run` pass.
- [ ] 3.2 `bun run test`, `bun run check`, `bun run lint`, and
      `bun run format:check` pass in `cmd/hush-hush/web`.
- [ ] 3.3 Manually verified against the running binary: a fresh database
      shows only "Register passkey" and completes bootstrap; a database
      with an admin account shows only "Log in with a passkey".
