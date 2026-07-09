# Production Session Security Evidence - 2026-07-01

## Scope

- Fail closed on weak production session configuration.
- Prevent production boot with insecure session cookies.
- Make deployment examples reflect the production constraints.

## Implementation

- `src/server/internal/config/config.go`
  - `APP_ENV=production` now rejects default session secrets such as `change-me` and `test-secret`.
  - `APP_ENV=production` now requires `SESSION_SECRET` to be at least 32 characters.
  - `APP_ENV=production` now rejects `SESSION_COOKIE_SECURE=false`.

- `src/server/internal/config/config_test.go`
  - Added coverage for default/short production session secrets.
  - Added coverage for insecure production session cookies.
  - Updated other production config failure tests to use a strong secret and secure cookies so they keep asserting their intended failures.

- `config/.env.example`
  - Documents that `SESSION_SECRET=change-me` is development-only.
  - Documents that production requires `SESSION_COOKIE_SECURE=true`.

- `deploy/kubernetes/secret.example.yaml`
  - Replaces the weak `SESSION_SECRET: REPLACE_ME` placeholder with a 32+ character random-value requirement.

## Verification

- Passed:
  - `git diff --check -- src/server/internal/config/config.go src/server/internal/config/config_test.go config/.env.example deploy/kubernetes/secret.example.yaml`

- Blocked by local toolchain:
  - `gofmt -w src\server\internal\config\config.go src\server\internal\config\config_test.go`
    - Failed because `gofmt` is not on PATH.
  - `go test ./internal/config -run "TestLoadRejectsProductionWeakSessionSecret|TestLoadRejectsProductionInsecureSessionCookie|TestLoadRejectsProductionRelayWithoutRequestLogSink|TestLoadRejectsProductionWithoutRelay|TestLoadRejectsProductionQdrantWithoutRAGIndexWorker" -count=1 -v`
    - Failed because `go` is not on PATH.

## Residual Risk

- Secret strength is length/default-name validation, not entropy validation.
- Final release still requires the full config suite to run with Go installed.
