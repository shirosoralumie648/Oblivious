# Relay Route Table

This table is the v05 Relay Authority Gate route policy ledger. It mirrors `src/server/internal/relay/handler/router.go` and `src/server/internal/relay/handler/policy.go`.

Phase 16 closes the v05 route-table evidence by tying this ledger to `.planning/phases/16-relay-authority-evidence-and-v05-closeout/16-VERIFICATION.md`. v05 completion means the Relay Authority Gate is proven for route classification, production fail-closed behavior, provider-bypass checks, supported-route auth/rate-limit/audit, and settlement/refund policy. It does not complete v06 money movement, v07 production operations, or v08 product completeness.

Class meanings:
- `CommercialSupportedBilled`: production-callable commercial endpoint with Phase 15 billing/refund semantics.
- `DisabledInProduction`: registered for compatibility, but production rejects the endpoint with `endpoint_disabled_in_production` before handler or provider execution.
- `InternalAdminOnly`: reserved class required by the commercial gate; no current `/v1/*` route uses it.
- Relay Batch routes remain disabled by default in production. `RELAY_BATCH_COMMERCIAL_LIFECYCLE_ENABLED=true` only promotes them after durable polling, settlement/refund, audit, usage capture, and target evidence have been deployed.
- Relay Realtime remains disabled by default in production. `RELAY_REALTIME_COMMERCIAL_LIFECYCLE_ENABLED=true` only promotes it after production prebill configuration, abort settlement, request-log linkage, and target evidence have been deployed. The guarded handler path now covers missing-model fail-fast, origin rejection before billing/upstream dial, API-token query-model authorization input, upstream `response.done` usage capture, and missing-usage fail-closed behavior, but local handler coverage is not sufficient final-readiness evidence by itself.

Phase 14 policy meanings:
- `trusted_internal_identity`: production requests must carry `X-Oblivious-Internal-Auth`, `X-Oblivious-Internal-User-ID`, and `X-Oblivious-Internal-Organization-ID` before handler/provider execution.
- `global_relay_token_bucket`: supported calls enter the existing Relay token-bucket guard before channel/provider selection.
- `relay_route_policy_decision`: the route wrapper emits an injectable audit event for allowed and rejected policy decisions.
- `not_applicable`: the route is production-disabled before commercial auth/rate-limit behavior can apply.

Phase 15 billing policy meanings:
- `preauthorize_then_settle_usage`: pre-authorize estimated quota before provider dispatch, settle provider-reported usage exactly once per organization-scoped idempotency key, and refund provider errors, nil responses, or missing required usage.
- `preauthorize_then_settle_estimate`: pre-authorize estimated quota before provider dispatch, settle the request estimate exactly once on 2xx provider success, and refund provider errors or nil responses.
- `production_disabled`: no commercial charge path; production rejects before handler/provider execution.

| Method | Path | API type | Strategy | Commercial class | Production status | Auth policy | Tenant identity | Rate-limit policy | Audit policy | Billing policy | Disabled reason | Future owner |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `POST` | `/v1/chat/completions` | chat | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_usage | Streaming proxies through the billing router with provider response settlement | v08 streaming/product completeness |
| `POST` | `/v1/responses` | responses | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_usage | Streaming proxies through the billing router with provider response settlement | v08 streaming/product completeness |
| `GET` | `/v1/realtime` | realtime | Native stream | DisabledInProduction by default; CommercialSupportedBilled only when `RELAY_REALTIME_COMMERCIAL_LIFECYCLE_ENABLED=true` | Disabled by default | trusted_internal_identity when enabled | Required when enabled | global_relay_token_bucket when enabled | relay_route_policy_decision | production_disabled by default; preauthorize_then_settle_usage when enabled | Realtime production prebill, abort settlement, request-log linkage, and target proof must be deployed and proven before enabling | Future commercial support |
| `POST` | `/v1/embeddings` | embeddings | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_usage |  | v05 closed |
| `POST` | `/v1/images/generations` | images_generations | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_estimate | Phase 15 settles the request estimate when provider usage is absent | v05 closed |
| `POST` | `/v1/images/edits` | images_edits | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_estimate | Phase 15 settles the request estimate when provider usage is absent | v05 closed |
| `POST` | `/v1/images/variations` | images_variations | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_estimate | Phase 15 settles the request estimate when provider usage is absent | v05 closed |
| `POST` | `/v1/videos` | videos | Native | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | Video billing and provider behavior are not verified | Future commercial support |
| `POST` | `/v1/audio/speech` | audio_speech | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_estimate | Phase 15 settles the request estimate when provider usage is absent | v05 closed |
| `POST` | `/v1/audio/transcriptions` | audio_transcriptions | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_estimate | Phase 15 settles the request estimate when provider usage is absent | v05 closed |
| `POST` | `/v1/audio/translations` | audio_translations | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_estimate | Phase 15 settles the request estimate when provider usage is absent | v05 closed |
| `POST` | `/v1/moderations` | moderations | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_estimate | Phase 15 settles the request estimate when provider usage is absent | v05 closed |
| `POST` | `/v1/completions` | completions | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_usage |  | v05 closed |
| `POST` | `/v1/batch` | batch | Native | DisabledInProduction by default; CommercialSupportedBilled only when `RELAY_BATCH_COMMERCIAL_LIFECYCLE_ENABLED=true` | Disabled by default | trusted_internal_identity when enabled | Required when enabled | global_relay_token_bucket when enabled | relay_route_policy_decision | production_disabled by default; preauthorize_then_settle_usage when enabled | Batch prebill, polling, settlement, refund, audit, and usage capture must be deployed and proven before enabling | Future commercial support |
| `GET` | `/v1/batches` | batch | Passthrough | DisabledInProduction by default; CommercialSupportedBilled only when `RELAY_BATCH_COMMERCIAL_LIFECYCLE_ENABLED=true` | Disabled by default | trusted_internal_identity when enabled | Required when enabled | global_relay_token_bucket when enabled | relay_route_policy_decision | production_disabled by default; preauthorize_then_settle_usage when enabled | Batch polling, settlement/refund, audit, and usage capture must be deployed and proven before enabling | Future commercial support |
| `GET` | `/v1/batches/:id` | batch | Passthrough | DisabledInProduction by default; CommercialSupportedBilled only when `RELAY_BATCH_COMMERCIAL_LIFECYCLE_ENABLED=true` | Disabled by default | trusted_internal_identity when enabled | Required when enabled | global_relay_token_bucket when enabled | relay_route_policy_decision | production_disabled by default; preauthorize_then_settle_usage when enabled | Batch polling, settlement/refund, audit, and usage capture must be deployed and proven before enabling | Future commercial support |
| `POST` | `/v1/files` | files | File proxy | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_estimate | Requires configured tenant file mapping store; settles the upload estimate through Relay billing router | v08 file lifecycle hardening |
| `GET` | `/v1/files` | files | Passthrough | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_estimate | Requires tenant-scoped file mapping list; filters upstream provider files to current tenant mappings and rewrites IDs to local file IDs | v08 file lifecycle hardening |
| `GET` | `/v1/files/:id` | files | Passthrough | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_estimate | Requires tenant-scoped local-to-provider file mapping before upstream dispatch | v08 file lifecycle hardening |
| `DELETE` | `/v1/files/:id` | files | Passthrough | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_estimate | Requires tenant-scoped local-to-provider file mapping before upstream dispatch | v08 file lifecycle hardening |
| `GET` | `/v1/files/:id/content` | files | Passthrough | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_estimate | Requires tenant-scoped local-to-provider file mapping before upstream dispatch | v08 file lifecycle hardening |
| `POST` | `/v1/fine_tuning/jobs` | fine_tuning | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | Fine-tuning job lifecycle and training-token billing are not implemented | Future commercial support |
| `GET` | `/v1/fine_tuning/jobs` | fine_tuning | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | Fine-tuning job lifecycle and audit are not implemented | Future commercial support |
| `GET` | `/v1/fine_tuning/jobs/:id` | fine_tuning | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | Fine-tuning job lifecycle and audit are not implemented | Future commercial support |
| `POST` | `/v1/fine_tuning/jobs/:id/cancel` | fine_tuning | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | Fine-tuning cancel billing and audit are not implemented | Future commercial support |
| `GET` | `/v1/fine_tuning/jobs/:id/events` | fine_tuning | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | Fine-tuning event streaming audit is not implemented | Future commercial support |
| `POST` | `/v1/assistants` | assistants | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | Assistants lifecycle billing and governance are not implemented | Future commercial support |
| `GET` | `/v1/assistants` | assistants | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | Assistants lifecycle billing and governance are not implemented | Future commercial support |
| `GET` | `/v1/assistants/:id` | assistants | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | Assistants lifecycle billing and governance are not implemented | Future commercial support |
| `POST` | `/v1/threads` | threads | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | Threads lifecycle billing and audit are not implemented | Future commercial support |
| `GET` | `/v1/threads/:id` | threads | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | Threads lifecycle billing and audit are not implemented | Future commercial support |
| `POST` | `/v1/threads/:id/runs` | runs | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | Runs lifecycle billing and tool-call audit are not implemented | Future commercial support |
| `GET` | `/v1/threads/:id/runs/:rid` | runs | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | Runs lifecycle billing and tool-call audit are not implemented | Future commercial support |
| `POST` | `/v1/threads/:id/runs/:rid/submit` | runs | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | Run submit-tool-output billing and audit are not implemented | Future commercial support |

## Phase 13-16 Evidence Contract

Phase 13 proves `RELAY-08` and `RELAY-09`:
- all currently registered `/v1/*` routes have a commercial policy;
- production-disabled routes return `endpoint_disabled_in_production` before handler/provider execution.

Phase 14 proves `RELAY-10` and `RELAY-11`:
- `scripts/verify-relay-security.sh` and `bash scripts/check.sh relay-security` fail on app-service direct-provider bypass patterns;
- CI runs the Relay security boundary check;
- production-supported routes require trusted internal user and organization identity before handler/provider execution;
- route policy decisions emit audit events for allowed and rejected production requests;
- Chat, Agent, and Knowledge embedding app paths keep using Relay metadata and trusted internal headers.

Phase 15 proves `BILL-01` and `BILL-02` at the billing lifecycle and route policy level:
- supported calls preauthorize before provider dispatch;
- successful calls settle exactly once per organization-scoped idempotency key;
- provider errors, nil responses, missing required usage, and refund/settlement failures return explicit billing errors;
- streaming/realtime, batch, list-files, and async endpoints are either explicitly rejected by the handler or production-disabled before provider dispatch; upload and tenant-mapped file routes dispatch through `RouteWithBilling`.

Phase 16 closes `DOC-04` with Relay Authority Gate closeout evidence:
- `.planning/phases/16-relay-authority-evidence-and-v05-closeout/16-VERIFICATION.md` records exact commands, environment class, DB migration status, passed checks, skipped checks, and residual v06-v08 work;
- `.planning/phases/16-relay-authority-evidence-and-v05-closeout/16-01-SUMMARY.md` records v05 closeout without claiming final commercial readiness.
