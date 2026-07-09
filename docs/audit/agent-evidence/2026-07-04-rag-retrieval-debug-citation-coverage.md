# RAG retrieval debug citation coverage API

Date: 2026-07-04

## Scope

Closed a local commercial RAG observability gap where retrieval results carried citation metadata but lacked a dedicated API surface for debugging citation coverage, retrieval options, and returned provenance. This is repository-local API/contract evidence; it does not replace target Postgres/Qdrant/live provider proof.

## Change

- `src/server/internal/knowledge/types_enhanced.go`
  - Added `KnowledgeRetrievalCitationCoverage`.
  - Added `KnowledgeRetrievalDebugReport` with query, options, result count, citation coverage, and typed retrieval results.
- `src/server/internal/http/knowledge_handler.go`
  - Added `retrieveKnowledgeDebug`.
  - Preserves the normal retrieval path while returning citation coverage metrics.
- `src/server/internal/http/routes_knowledge.go`
  - Added `POST /api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieve/debug`.
- `src/server/internal/http/routes_knowledge_alias.go`
  - Added alias `POST /api/v1/knowledge-bases/{knowledgeBaseId}/retrieve/debug`.
- `docs/api/openapi.yaml`, `docs/api/route-surface-manifest.json`, and `scripts/verify_openapi_contract.py`
  - Added named schemas, canonical and alias paths, route-surface manifest entries, and contract checks.

## Verification

- RED: `cd src/server && go test ./internal/http -run TestKnowledgeHandlerRetrieveDebugReturnsCitationCoverage -count=1 -v`
  - Failed before implementation with missing `retrieveKnowledgeDebug` and missing `KnowledgeRetrievalDebugReport` symbols.
- GREEN:
  - `cd src/server && go test ./internal/http -run 'TestKnowledgeHandlerRetrieveDebugReturnsCitationCoverage|TestRegisterKnowledgeAliasRoutesDispatchesDocumentsAndRetrieve|TestRouteSurfaceKnowledgeRoutesRequireSessionWithoutDatabase|TestRouteSurfaceKnowledgeRoutesDispatchWithCSRFWithoutDatabase|TestRouteSurfaceKnowledgeAliasRoutesRequireSessionWithoutDatabase|TestRouteSurfaceKnowledgeAliasRoutesDispatchWithCSRFWithoutDatabase|TestRouteSurfaceManifestRoutesAreRegisteredWithoutDatabase' -count=1 -v`
  - Passed.
  - `bash scripts/verify-openapi-contract.sh`
  - Passed.

## Remaining boundary

This debug endpoint proves local route, handler, and API contract behavior. Target-runtime retrieval-debug proof still needs a deployed environment with real Postgres/Qdrant retrieval evidence and captured response artifacts.
