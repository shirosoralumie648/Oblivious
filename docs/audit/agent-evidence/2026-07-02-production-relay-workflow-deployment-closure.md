# Production Relay And Workflow Deployment Closure Evidence - 2026-07-02

## Claim

The previously identified production deployment gaps for standalone Relay startup and Workflow service database selection are closed in the current worktree and guarded by deployment contract checks.

## Current State

- `deploy/kubernetes/relay-deployment.yaml` sets `APP_ENV=production`, `RELAY_ENABLED=true`, `OBLIVIOUS_DB_MODE=microservices`, and reads both `DATABASE_URL` and `DB_URL_RELAY` from `oblivious-secrets`.
- `src/server/cmd/relay/main.go` opens the relay database, applies migrations, loads channels from the database, wires API-token auth, quota, usage logging, pricing, rate limits, and health alert state.
- `deploy/kubernetes/workflow-deployment.yaml` sets `OBLIVIOUS_DB_MODE=microservices` and reads `DB_URL_WORKFLOW` from `oblivious-secrets`.
- `src/server/pkg/config.LoadWorkflowConfig` uses `DB_URL_WORKFLOW` when `OBLIVIOUS_DB_MODE` is not `monolith`.
- Older audit evidence that described standalone `cmd/relay` as intentionally non-production has been marked as historical and superseded by the deployment operations contract.

## Contract Guards

- `scripts/verify_deployment_operations_contract.py` checks the Relay deployment env, Relay command production wiring, Workflow deployment ports, and Workflow microservice DB env.
- `src/server/pkg/config/service_database_test.go` verifies that service config loaders, including Relay and Workflow, use service-specific database URLs in `microservices` mode.

## Verification

```bash
bash scripts/verify-deployment-operations-contract.sh
```

Result: passed.

```bash
cd src/server
export PATH="/c/Program Files/Go/bin:$PATH"
go test ./pkg/config -run "Test.*Database|Test.*Workflow|Test.*Relay|TestLoadWorkflow" -count=1 -v
```

Result: passed.

```bash
bash scripts/check.sh docs
```

Result: passed.

## Residual Risk

This is static and unit-level release evidence. Final release still needs live Kubernetes rollout evidence proving the Relay and Workflow pods start against externally provisioned production-class databases.
