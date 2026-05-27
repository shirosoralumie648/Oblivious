# Relay Route Table

This table is the v05 Relay Authority Gate route policy ledger. It mirrors `src/server/internal/relay/handler/router.go` and `src/server/internal/relay/handler/policy.go`.

Class meanings:
- `CommercialSupportedBilled`: production-callable commercial endpoint with Phase 15 billing/refund semantics.
- `DisabledInProduction`: registered for compatibility, but production rejects the endpoint with `endpoint_disabled_in_production` before handler or provider execution.
- `InternalAdminOnly`: reserved class required by the commercial gate; no current `/v1/*` route uses it.

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
| `POST` | `/v1/chat/completions` | chat | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_usage | Chat streaming remains rejected until tested streaming settlement exists | Phase 16 evidence |
| `POST` | `/v1/responses` | responses | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_usage | Responses streaming returns `streaming_settlement_not_supported` | Phase 16 evidence |
| `GET` | `/v1/realtime` | realtime | Native stream | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | Realtime settlement and client-abort billing are not defined | Future commercial support |
| `POST` | `/v1/embeddings` | embeddings | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_usage |  | Phase 16 evidence |
| `POST` | `/v1/images/generations` | images_generations | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_estimate | Phase 15 settles the request estimate when provider usage is absent | Phase 16 evidence |
| `POST` | `/v1/images/edits` | images_edits | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_estimate | Phase 15 settles the request estimate when provider usage is absent | Phase 16 evidence |
| `POST` | `/v1/images/variations` | images_variations | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_estimate | Phase 15 settles the request estimate when provider usage is absent | Phase 16 evidence |
| `POST` | `/v1/videos` | videos | Native | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | Video billing and provider behavior are not verified | Future commercial support |
| `POST` | `/v1/audio/speech` | audio_speech | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_estimate | Phase 15 settles the request estimate when provider usage is absent | Phase 16 evidence |
| `POST` | `/v1/audio/transcriptions` | audio_transcriptions | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_estimate | Phase 15 settles the request estimate when provider usage is absent | Phase 16 evidence |
| `POST` | `/v1/audio/translations` | audio_translations | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_estimate | Phase 15 settles the request estimate when provider usage is absent | Phase 16 evidence |
| `POST` | `/v1/moderations` | moderations | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_estimate | Phase 15 settles the request estimate when provider usage is absent | Phase 16 evidence |
| `POST` | `/v1/completions` | completions | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | preauthorize_then_settle_usage |  | Phase 16 evidence |
| `POST` | `/v1/batch` | batch | Native | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | Async batch settlement and audit are not defined | Future commercial support |
| `GET` | `/v1/batches` | batch | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | Batch passthrough lacks commercial audit and settlement | Future commercial support |
| `GET` | `/v1/batches/:id` | batch | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | Batch passthrough lacks commercial audit and settlement | Future commercial support |
| `POST` | `/v1/files` | files | File proxy | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | File mapping persistence and storage billing are not implemented | Future commercial support |
| `GET` | `/v1/files` | files | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | File passthrough lacks tenant file ownership and audit | Future commercial support |
| `GET` | `/v1/files/:id` | files | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | File passthrough lacks tenant file ownership and audit | Future commercial support |
| `DELETE` | `/v1/files/:id` | files | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | File passthrough lacks tenant file ownership and audit | Future commercial support |
| `GET` | `/v1/files/:id/content` | files | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | production_disabled | File passthrough lacks tenant file ownership and audit | Future commercial support |
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

## Phase 13-14 Evidence Contract

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
- streaming/realtime, file, batch, and async endpoints are either explicitly rejected by the handler or production-disabled before provider dispatch.

v05 Relay Authority Gate closeout evidence remains Phase 16 work.
