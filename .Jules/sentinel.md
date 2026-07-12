## 2025-02-20 - [Security] Fix implicit tls.Conn.Handshake via tls.Dialer inside http.Transport

**Learning:** There was a GO-2026-5856 vulnerability associated with `crypto/tls` in Go `1.25.0` to `1.26.4` which leaked Encrypted Client Hello details.

**Action:** Update the Go toolchain/modules to patch security vulnerabilities like GO-2026-5856 in `crypto/tls`.

## 2025-02-20 - [Typing/Validation] Fix Missing Types and Unhandled Fields in Web Channel Settings

**Learning:** When using typescript interfaces with properties that are required for conditionally filtering out unconfigurable/unsupported items, be aware that you might be making a type error if those properties don't exist in the type definition itself. Similarly, mock HTTP assertions should not assume the order or fields of struct serialization if there is potential for it to be structurally different (eg `[]KnowledgeIngestionJob` instead of `KnowledgeDocument`).

**Action:**
1. Added missing boolean properties (`configurable`, `installable`, `runtimeReady`) to `ChannelProviderInfo`.
2. Changed the way the mock HTTP assertion checks upload payload details in `TestRegisterKnowledgeAliasRoutesDispatchesDocumentUpload` so that it doesn't assume that a single `requestedDoc` struct was populated when instead an ingestion job list was created.
