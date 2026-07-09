# Final Release Proof Hardening

## Scope

This hardening pass closes release-evidence gaps that allowed repository-local or RC evidence to look too close to final commercial readiness.

## Contract Changes

- Final target evidence defaults to `environment.class=production`; staging and preproduction require `--allow-non-production-target` and are not full commercial release proof.
- Final Relay evidence requires `relayRealtime.mode=commercial_lifecycle_enabled` and `relayBatch.mode=commercial_lifecycle_enabled`; disabled lifecycle evidence requires `--allow-disabled-commercial-lifecycle` and is only fallback or negative evidence.
- Strict verifier proof JSON must include `commit`, `runId`, `targetEvidenceSha256`, and `artifactBundleSha256`; the verifier and artifact body checks require the manifest and strict-verifier artifact body to repeat the same values.
- `scripts/compute-target-release-digests.sh` now computes canonical `targetEvidenceSha256` and `artifactBundleSha256` values after artifact collection, normalizing strict-verifier self-referential fields and refreshing the manifest plus strict-verifier artifact body with `--write`.
- `verify-target-release-evidence.sh` recomputes canonical digest values when artifact bodies are supplied and rejects stale or hand-filled strict verifier digests.
- Artifact body `collectionSource.url` and `platformProofSource.url` hosts must match the manifest `environment.baseUrl` host, except for explicit local fixture validation.
- Request-log artifact body `sloProofSource.url` must follow the same final target source rules as the main collection URL and ClickHouse platform proof URL.
- Strict preflight rejects target evidence manifests and artifact body directories that live inside the repository, so final proof must come from an external, untracked release working directory.
- Runtime release-evidence endpoints accept paired RFC3339 `from`/`to` query parameters so target proof can be collected for the same release window instead of aggregating historical database state.
- `collect-target-release-artifacts.sh` URL examples now point RAG, Relay, Marketplace, and microservice database proof collection at `/api/v1/admin/release-evidence/*` with explicit window parameters.
- OpenAPI and the route-surface manifest now document the release-evidence and latency SLO proof window contracts, and `verify-openapi-contract.sh` fails if those endpoints lose required `from`/`to`, `400`, `scope`, or sample-query coverage.
- Final artifact-body verification now requires runtime proof URLs for RAG, Relay, Marketplace, provider runtime config, microservice database, and latency SLO to use their expected Admin endpoint paths; windowed proofs must include `from` and `to`.
- `scripts/assemble-target-release-evidence.sh --artifact-dir ... --validate` now forwards downloaded artifact bodies into strict target evidence verification, so stale manifest SHA-256 values, body lineage drift, collection-source drift, and canonical digest mismatches can fail during assembly instead of only during the final commercial verifier.
- Realtime abort settlement release evidence now requires explicit `realtime_usage_missing` terminal usage records; generic `upstream_error` records no longer satisfy the abort proof.
- Relay streaming abort handling preserves coded Realtime post-upgrade errors into usage records and request-log billing metadata, so the target proof can distinguish true no-cost Realtime aborts from ordinary upstream failures.
- Relay Batch `usageAudit` target proof now requires allowed `relay.route_decision` request-log evidence coverage in addition to Batch usage records. The runtime endpoint, Batch collector, assembler, final verifier, fixtures, and quality gate all require positive `requestLogAuditRecords` covering `settlementRecords + refundRecords`.
- Provider live rail proof now requires `providerEnvironment=live` and concrete checkout/refund/payout/reconciliation references. The collector, assembler, final verifier, artifact-body fixtures, and quality gate reject pass/count-only provider proof that lacks live provider operation references.
- Deployment and Kubernetes proof now require production-bound concrete references. Deployment collector, assembler, final verifier, artifact-body fixtures, and quality gate reject pass-only deployment proof without `targetEnvironment=production` and deploy/backup/migration references. Kubernetes proof also requires `clusterRef`, `namespace`, and validation/rollout/failover references.
- Request-log latency SLO proof now rejects placeholder/sample/fake or secret-like `lastDeliveryId` and `lastRecordId` values in the collector and target verifier.
- Provider runtime config proof now requires `providers[]` details for Stripe, Alipay, and WeChat Pay in addition to summary counts. The collector, assembler, target verifier, fixtures, and quality gate reject proof that omits a required provider detail or concrete evidence ID.
- Microservice database proof now requires `services[]` details for relay, chat, workflow, rag, agent, billing, marketplace, admin, channel, task, and observability in addition to summary counts. The collector, assembler, target verifier, fixtures, and quality gate reject proof that omits a required service detail or concrete evidence ID.
- `.github/workflows/release-evidence.yml` adds a manual target evidence bundle verification workflow for GitHub Actions artifacts from another run; it refreshes canonical digests and runs target-evidence-only preflight without carrying production secrets.
- `verify-target-release-evidence.sh` now treats the default invocation as final target evidence validation and requires `OBLIVIOUS_TARGET_ARTIFACT_DIR`; pure manifest linting must be explicit with `--manifest-only` and is never commercial readiness proof.
- `verify-commercial-completion.sh` now aggregates missing strict final release inputs before heavy docs, web, Go, DB, deploy, Kubernetes, backup, or target evidence work begins. A no-skip run reports the missing `TEST_DATABASE_URL`, Kubernetes secret file, target evidence manifest, and artifact body directory together instead of stopping at the first absent variable; after target evidence preflight, it also fails fast when neither `COMMERCIAL_COMPLETION_DB_SERVER_SUITE_DATABASE_URL` nor a reachable Docker daemon is available for the isolated DB-backed server suite.

## Verification

- `python -m py_compile scripts\collect_strict_verifier_evidence.py scripts\assemble_target_release_evidence.py scripts\verify_target_release_evidence.py scripts\target_release_fixture_mutations.py`
- `bash -n scripts/collect-strict-verifier-evidence-fixtures.sh scripts/assemble-target-release-evidence-fixtures.sh scripts/verify-target-release-evidence-fixtures.sh scripts/collect-target-release-artifacts-fixtures.sh scripts/verify-quality-gates.sh scripts/verify-target-release-evidence.sh`
- `bash scripts/collect-strict-verifier-evidence-fixtures.sh`
- `bash scripts/assemble-target-release-evidence-fixtures.sh`
- `bash scripts/verify-target-release-evidence-fixtures.sh`
- `bash scripts/collect-target-release-artifacts-fixtures.sh`
- `bash scripts/compute-target-release-digests-fixtures.sh`
- `bash scripts/verify-commercial-preflight-fixtures.sh`
- `bash scripts/verify-quality-gates.sh`
- `python -B -m py_compile scripts\collect_relay_batch_evidence.py scripts\verify_target_release_evidence.py scripts\target_release_fixture_mutations.py scripts\assemble_target_release_evidence.py`
- `bash scripts/collect-relay-batch-evidence-fixtures.sh`
- `python -B -m py_compile scripts\collect_provider_live_rail_evidence.py scripts\assemble_target_release_evidence.py scripts\verify_target_release_evidence.py scripts\target_release_fixture_mutations.py`
- `bash scripts/collect-provider-live-rail-evidence-fixtures.sh`
- `bash scripts/collect-deployment-evidence-fixtures.sh`
- `bash scripts/collect-kubernetes-evidence-fixtures.sh`
- `bash scripts/collect-request-log-observability-evidence-fixtures.sh`
- `python -B -m py_compile scripts\collect_microservice_database_evidence.py scripts\assemble_target_release_evidence.py scripts\verify_target_release_evidence.py scripts\target_release_fixture_mutations.py`
- `bash scripts/collect-microservice-database-evidence-fixtures.sh`
- `bash scripts/verify-target-release-evidence-fixtures.sh`
- `go test ./internal/http -run 'ReleaseEvidenceRelayBatch|ReleaseEvidenceRelayRealtime|ReleaseEvidenceRoutesRejectInvalidWindowScope' -count=1`
- `go test ./internal/http -run 'ReleaseEvidenceRelayRealtime|RelayStartupError|RelayPoolConfigurationError|BuildRelayConfigWiresCommercialLifecycleFlags|StartRelayBatchPollingWorkerIfEnabled' -count=1`
- `go test ./internal/relay -run 'TestRouterRouteWithBillingFinalizesStreamingAbortUsageAndRequestLogScope|TestRouterRouteWithBillingRecordsTerminalUpstreamError|TestRouterRouteWithBillingRetries' -count=1`
- `go test ./internal/relay/handler -run 'TestRealtime' -count=1`

## Admin Provider Catalog Hardening

- Admin channel provider responses now expose explicit `configurable`, `installable`, and `runtimeReady` flags.
- Planned providers remain visible only as catalog metadata and are not offered in the Admin Channels provider filter or create/edit form.
- Admin channel creation and Relay adapter construction both reject planned providers, including planned OpenAI-compatible providers with an explicit `baseURL`.

## Admin Provider Catalog Verification

- `go test ./internal/admin ./internal/http ./internal/relay/channel`
- `vitest run AdminChannelsPage.test.tsx`
- `tsc --noEmit`

## Marketplace Review Scanner Hardening

- The deterministic Marketplace scanner now uses a structured policy manifest with stable rule IDs.
- Automated findings include `ruleId` so governance events can be traced back to the exact policy rule that produced the decision.
- The policy checksum is computed from the rule manifest instead of a free-form rule string.

## Marketplace Review Scanner Verification

- `go test ./internal/marketplace`

## Secret Audit Proof Hardening

- Target secret-audit proof now requires `checkedAt` and a `summary` object in addition to `result`, `scope`, and `findings`.
- The release evidence collector and assembler reject secret-audit proofs unless `totalRecordsScanned > 0` and `plaintextRecords`, `invalidProtectedRecords`, and `rotationRequiredRecords` are all zero.
- The final manifest carries the secret-audit checked timestamp and summary so the proof is more than an empty findings array.

## Secret Audit Proof Verification

- `python -m py_compile scripts\collect_secret_audit_evidence.py scripts\assemble_target_release_evidence.py`
- `bash -n scripts/collect-secret-audit-evidence-fixtures.sh scripts/assemble-target-release-evidence-fixtures.sh scripts/collect-secret-audit-evidence.sh`
- `bash scripts/collect-secret-audit-evidence-fixtures.sh`
- `bash scripts/assemble-target-release-evidence-fixtures.sh`

## Request Log Observability Proof Hardening

- Request-log observability evidence now requires structured latency SLO proof in addition to the existing flat `latencySLO*=pass` summary fields.
- The collector, assembler, and final verifier reject proofs without an ISO-8601 SLO window, positive triggered alert count, positive configured/delivered alert counts, zero failed alert deliveries, non-empty alert channels, and persisted recovery audit records with zero failed actions.
- The final target evidence manifest carries `latencySLOWindow`, `latencySLOTriggeredAlerts`, `alertDelivery`, and `recoveryAudit`, and artifact body validation verifies the collector output matches those manifest fields.
- `/api/v1/admin/observability/latency-slo-proof` now aggregates runtime alert states, delivery attempts, and recovery actions into the same proof shape.
- `collect-request-log-observability-evidence.sh` supports `--slo-url` so target proof can be fetched from the Admin runtime instead of relying only on a hand-built local JSON file.
- `collect-target-release-artifacts.sh` also accepts `--slo-url`, and final verification validates the SLO proof source URL against the target manifest base URL.
- Alert delivery now records missing-sink failures in delivery history, preventing `failedDeliveries=0` undercounting when routing is configured but a channel has no sink.

## Request Log Observability Proof Verification

- `python -m py_compile scripts\collect_request_log_observability_evidence.py scripts\assemble_target_release_evidence.py scripts\verify_target_release_evidence.py scripts\target_release_fixture_mutations.py`
- `bash -n scripts/collect-request-log-observability-evidence-fixtures.sh scripts/assemble-target-release-evidence-fixtures.sh scripts/verify-target-release-evidence-fixtures.sh scripts/collect-request-log-observability-evidence.sh`
- `bash scripts/collect-request-log-observability-evidence-fixtures.sh`
- `bash scripts/assemble-target-release-evidence-fixtures.sh`
- `bash scripts/verify-target-release-evidence-fixtures.sh`
- `go test ./internal/observability ./internal/http -run 'TestAlertDeliveryDispatcherRecordsMissingSinkAttempt|TestObservabilityLatencySLOProofAggregatesRuntimeDeliveryAndRecovery' -count=1`

## Workflow Telemetry Proof Hardening

- Workflow telemetry evidence already required `totalExecutions`, `successfulExecutions`, and `failedExecutions`; the assembler now carries those counts into the final target manifest instead of reducing the proof to only `successRate` and `window`.
- The final verifier rejects manifests where workflow telemetry counts are missing, do not add up, or do not match the reported success rate.
- Artifact body validation now also rejects workflow telemetry bodies whose execution counts diverge from the manifest.

## Workflow Telemetry Proof Verification

- `python -m py_compile scripts\assemble_target_release_evidence.py scripts\verify_target_release_evidence.py scripts\target_release_fixture_mutations.py`
- `bash -n scripts/collect-workflow-telemetry-evidence-fixtures.sh scripts/assemble-target-release-evidence-fixtures.sh scripts/verify-target-release-evidence-fixtures.sh`
- `bash scripts/collect-workflow-telemetry-evidence-fixtures.sh`
- `bash scripts/assemble-target-release-evidence-fixtures.sh`
- `bash scripts/verify-target-release-evidence-fixtures.sh`

## Boundary

This is verifier and documentation hardening. It does not provide the real production target manifest, downloaded artifact bodies, Kubernetes proof, provider live rails, or final no-skip strict verifier run required for a complete commercial release claim.
