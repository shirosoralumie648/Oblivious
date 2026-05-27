# Relay Route Table

This table is the v05 Relay Authority Gate route policy ledger. It mirrors `src/server/internal/relay/handler/router.go` and `src/server/internal/relay/handler/policy.go`.

Class meanings:
- `CommercialSupportedBilled`: production-callable commercial endpoint; v05 Phase 15 must prove final settlement/refund behavior.
- `DisabledInProduction`: registered for compatibility, but production rejects the endpoint with `endpoint_disabled_in_production` before handler or provider execution.
- `InternalAdminOnly`: reserved class required by the commercial gate; no current `/v1/*` route uses it.

Phase 14 policy meanings:
- `trusted_internal_identity`: production requests must carry `X-Oblivious-Internal-Auth`, `X-Oblivious-Internal-User-ID`, and `X-Oblivious-Internal-Organization-ID` before handler/provider execution.
- `global_relay_token_bucket`: supported calls enter the existing Relay token-bucket guard before channel/provider selection.
- `relay_route_policy_decision`: the route wrapper emits an injectable audit event for allowed and rejected policy decisions.
- `not_applicable`: the route is production-disabled before commercial auth/rate-limit behavior can apply.

| Method | Path | API type | Strategy | Commercial class | Production status | Auth policy | Tenant identity | Rate-limit policy | Audit policy | Billing policy | Disabled reason | Future owner |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `POST` | `/v1/chat/completions` | chat | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | Phase 15 settlement |  | Phase 15 settlement |
| `POST` | `/v1/responses` | responses | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | Phase 15 settlement |  | Phase 15 settlement |
| `GET` | `/v1/realtime` | realtime | Native stream | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | Realtime settlement and client-abort billing are not defined | Phase 15 |
| `POST` | `/v1/embeddings` | embeddings | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | Phase 15 settlement |  | Phase 15 settlement |
| `POST` | `/v1/images/generations` | images_generations | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | Phase 15 settlement |  | Phase 15 settlement |
| `POST` | `/v1/images/edits` | images_edits | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | Phase 15 settlement |  | Phase 15 settlement |
| `POST` | `/v1/images/variations` | images_variations | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | Phase 15 settlement |  | Phase 15 settlement |
| `POST` | `/v1/videos` | videos | Native | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | Video billing and provider behavior are not verified | Phase 15 |
| `POST` | `/v1/audio/speech` | audio_speech | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | Phase 15 settlement |  | Phase 15 settlement |
| `POST` | `/v1/audio/transcriptions` | audio_transcriptions | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | Phase 15 settlement |  | Phase 15 settlement |
| `POST` | `/v1/audio/translations` | audio_translations | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | Phase 15 settlement |  | Phase 15 settlement |
| `POST` | `/v1/moderations` | moderations | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | Phase 15 settlement |  | Phase 15 settlement |
| `POST` | `/v1/completions` | completions | Native | CommercialSupportedBilled | Enabled | trusted_internal_identity | Required | global_relay_token_bucket | relay_route_policy_decision | Phase 15 settlement |  | Phase 15 settlement |
| `POST` | `/v1/batch` | batch | Native | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | Async batch settlement and audit are not defined | Phase 15 |
| `GET` | `/v1/batches` | batch | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | Batch passthrough lacks commercial audit and settlement | Phase 15 |
| `GET` | `/v1/batches/:id` | batch | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | Batch passthrough lacks commercial audit and settlement | Phase 15 |
| `POST` | `/v1/files` | files | File proxy | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | File mapping persistence and storage billing are not implemented | Phase 15 |
| `GET` | `/v1/files` | files | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | File passthrough lacks tenant file ownership and audit | Phase 15 |
| `GET` | `/v1/files/:id` | files | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | File passthrough lacks tenant file ownership and audit | Phase 15 |
| `DELETE` | `/v1/files/:id` | files | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | File passthrough lacks tenant file ownership and audit | Phase 15 |
| `GET` | `/v1/files/:id/content` | files | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | File passthrough lacks tenant file ownership and audit | Phase 15 |
| `POST` | `/v1/fine_tuning/jobs` | fine_tuning | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | Fine-tuning job lifecycle and training-token billing are not implemented | Phase 15 |
| `GET` | `/v1/fine_tuning/jobs` | fine_tuning | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | Fine-tuning job lifecycle and audit are not implemented | Phase 15 |
| `GET` | `/v1/fine_tuning/jobs/:id` | fine_tuning | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | Fine-tuning job lifecycle and audit are not implemented | Phase 15 |
| `POST` | `/v1/fine_tuning/jobs/:id/cancel` | fine_tuning | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | Fine-tuning cancel billing and audit are not implemented | Phase 15 |
| `GET` | `/v1/fine_tuning/jobs/:id/events` | fine_tuning | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | Fine-tuning event streaming audit is not implemented | Phase 15 |
| `POST` | `/v1/assistants` | assistants | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | Assistants lifecycle billing and governance are not implemented | Phase 15 |
| `GET` | `/v1/assistants` | assistants | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | Assistants lifecycle billing and governance are not implemented | Phase 15 |
| `GET` | `/v1/assistants/:id` | assistants | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | Assistants lifecycle billing and governance are not implemented | Phase 15 |
| `POST` | `/v1/threads` | threads | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | Threads lifecycle billing and audit are not implemented | Phase 15 |
| `GET` | `/v1/threads/:id` | threads | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | Threads lifecycle billing and audit are not implemented | Phase 15 |
| `POST` | `/v1/threads/:id/runs` | runs | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | Runs lifecycle billing and tool-call audit are not implemented | Phase 15 |
| `GET` | `/v1/threads/:id/runs/:rid` | runs | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | Runs lifecycle billing and tool-call audit are not implemented | Phase 15 |
| `POST` | `/v1/threads/:id/runs/:rid/submit` | runs | Passthrough | DisabledInProduction | Disabled | not_applicable | Not required | not_applicable | relay_route_policy_decision | Production disabled | Run submit-tool-output billing and audit are not implemented | Phase 15 |

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

Final settlement/refund behavior remains v05 Phase 15 work. v05 closeout evidence remains Phase 16 work.
