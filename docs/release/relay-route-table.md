# Relay Route Table

This table is the v05 Relay Authority Gate route policy ledger. It mirrors `src/server/internal/relay/handler/router.go` and `src/server/internal/relay/handler/policy.go`.

Class meanings:
- `CommercialSupportedBilled`: production-callable commercial endpoint; v05 Phase 15 must prove final settlement/refund behavior.
- `DisabledInProduction`: registered for compatibility, but production rejects the endpoint with `endpoint_disabled_in_production` before handler or provider execution.
- `InternalAdminOnly`: reserved class required by the commercial gate; no current `/v1/*` route uses it.

| Method | Path | API type | Strategy | Commercial class | Production status | Disabled reason | Future owner |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `POST` | `/v1/chat/completions` | chat | Native | CommercialSupportedBilled | Enabled |  | Phase 15 settlement |
| `POST` | `/v1/responses` | responses | Native | CommercialSupportedBilled | Enabled |  | Phase 15 settlement |
| `GET` | `/v1/realtime` | realtime | Native stream | DisabledInProduction | Disabled | Realtime settlement and client-abort billing are not defined | Phase 15 |
| `POST` | `/v1/embeddings` | embeddings | Native | CommercialSupportedBilled | Enabled |  | Phase 15 settlement |
| `POST` | `/v1/images/generations` | images_generations | Native | CommercialSupportedBilled | Enabled |  | Phase 15 settlement |
| `POST` | `/v1/images/edits` | images_edits | Native | CommercialSupportedBilled | Enabled |  | Phase 15 settlement |
| `POST` | `/v1/images/variations` | images_variations | Native | CommercialSupportedBilled | Enabled |  | Phase 15 settlement |
| `POST` | `/v1/videos` | videos | Native | DisabledInProduction | Disabled | Video billing and provider behavior are not verified | Phase 15 |
| `POST` | `/v1/audio/speech` | audio_speech | Native | CommercialSupportedBilled | Enabled |  | Phase 15 settlement |
| `POST` | `/v1/audio/transcriptions` | audio_transcriptions | Native | CommercialSupportedBilled | Enabled |  | Phase 15 settlement |
| `POST` | `/v1/audio/translations` | audio_translations | Native | CommercialSupportedBilled | Enabled |  | Phase 15 settlement |
| `POST` | `/v1/moderations` | moderations | Native | CommercialSupportedBilled | Enabled |  | Phase 15 settlement |
| `POST` | `/v1/completions` | completions | Native | CommercialSupportedBilled | Enabled |  | Phase 15 settlement |
| `POST` | `/v1/batch` | batch | Native | DisabledInProduction | Disabled | Async batch settlement and audit are not defined | Phase 15 |
| `GET` | `/v1/batches` | batch | Passthrough | DisabledInProduction | Disabled | Batch passthrough lacks commercial audit and settlement | Phase 15 |
| `GET` | `/v1/batches/:id` | batch | Passthrough | DisabledInProduction | Disabled | Batch passthrough lacks commercial audit and settlement | Phase 15 |
| `POST` | `/v1/files` | files | File proxy | DisabledInProduction | Disabled | File mapping persistence and storage billing are not implemented | Phase 15 |
| `GET` | `/v1/files` | files | Passthrough | DisabledInProduction | Disabled | File passthrough lacks tenant file ownership and audit | Phase 15 |
| `GET` | `/v1/files/:id` | files | Passthrough | DisabledInProduction | Disabled | File passthrough lacks tenant file ownership and audit | Phase 15 |
| `DELETE` | `/v1/files/:id` | files | Passthrough | DisabledInProduction | Disabled | File passthrough lacks tenant file ownership and audit | Phase 15 |
| `GET` | `/v1/files/:id/content` | files | Passthrough | DisabledInProduction | Disabled | File passthrough lacks tenant file ownership and audit | Phase 15 |
| `POST` | `/v1/fine_tuning/jobs` | fine_tuning | Passthrough | DisabledInProduction | Disabled | Fine-tuning job lifecycle and training-token billing are not implemented | Phase 15 |
| `GET` | `/v1/fine_tuning/jobs` | fine_tuning | Passthrough | DisabledInProduction | Disabled | Fine-tuning job lifecycle and audit are not implemented | Phase 15 |
| `GET` | `/v1/fine_tuning/jobs/:id` | fine_tuning | Passthrough | DisabledInProduction | Disabled | Fine-tuning job lifecycle and audit are not implemented | Phase 15 |
| `POST` | `/v1/fine_tuning/jobs/:id/cancel` | fine_tuning | Passthrough | DisabledInProduction | Disabled | Fine-tuning cancel billing and audit are not implemented | Phase 15 |
| `GET` | `/v1/fine_tuning/jobs/:id/events` | fine_tuning | Passthrough | DisabledInProduction | Disabled | Fine-tuning event streaming audit is not implemented | Phase 15 |
| `POST` | `/v1/assistants` | assistants | Passthrough | DisabledInProduction | Disabled | Assistants lifecycle billing and governance are not implemented | Phase 15 |
| `GET` | `/v1/assistants` | assistants | Passthrough | DisabledInProduction | Disabled | Assistants lifecycle billing and governance are not implemented | Phase 15 |
| `GET` | `/v1/assistants/:id` | assistants | Passthrough | DisabledInProduction | Disabled | Assistants lifecycle billing and governance are not implemented | Phase 15 |
| `POST` | `/v1/threads` | threads | Passthrough | DisabledInProduction | Disabled | Threads lifecycle billing and audit are not implemented | Phase 15 |
| `GET` | `/v1/threads/:id` | threads | Passthrough | DisabledInProduction | Disabled | Threads lifecycle billing and audit are not implemented | Phase 15 |
| `POST` | `/v1/threads/:id/runs` | runs | Passthrough | DisabledInProduction | Disabled | Runs lifecycle billing and tool-call audit are not implemented | Phase 15 |
| `GET` | `/v1/threads/:id/runs/:rid` | runs | Passthrough | DisabledInProduction | Disabled | Runs lifecycle billing and tool-call audit are not implemented | Phase 15 |
| `POST` | `/v1/threads/:id/runs/:rid/submit` | runs | Passthrough | DisabledInProduction | Disabled | Run submit-tool-output billing and audit are not implemented | Phase 15 |

## Phase 13 Evidence Contract

Phase 13 proves `RELAY-08` and `RELAY-09` only:
- all currently registered `/v1/*` routes have a commercial policy;
- production-disabled routes return `endpoint_disabled_in_production` before handler/provider execution;
- supported routes remain callable for the later Phase 15 settlement work.

Provider-bypass checks, endpoint auth/rate-limit/audit semantics, and final settlement/refund evidence remain v05 Phase 14-16 work.
