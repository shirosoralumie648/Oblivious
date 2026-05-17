---
phase: 07-frontend-e2e-and-deployment-gate-alignment
plan: 01
status: passed
completed: 2026-05-17
requirements: [DEPLOY-02]
---

# 07-01 Summary: Frontend API and Workspace Contract Alignment

## Outcome

Passed. Frontend shared API types now cover the Phase 06 app route groups assigned to DEPLOY-02, and active workspace route errors use the Phase 07 retry/session wording.

## Files Changed

- `src/web/src/types/api.ts`
- `src/web/src/routes/workspace/KnowledgePage.tsx`
- `src/web/src/routes/workspace/SoloPage.tsx`
- `src/web/src/routes/workspace/KnowledgePageView.tsx`
- `src/web/src/routes/workspace/SoloPageView.tsx`

## Contract Work

- Added exported shared types for agents, agent conversations/messages/tools, memory documents/chunks/search, MCP servers/tools, notifications, quota, packages, and quota top-up requests.
- Kept active workspace page data flow on `createHttpClient`, `createKnowledgeApi`, `createTasksApi`, and `createChatApi`.
- Updated active Knowledge and SOLO error states to include retry and backend-session recovery guidance.
- Preserved required CTA copy: `Create knowledge base`, `Search knowledge`, and `Start solo run`.

## Marketplace Page Decision

`src/web/src/routes/workspace/MarketplacePage.tsx` was excluded from the active staged set for this plan. The active `/marketplace` router entries point to `src/web/src/routes/marketplace/*`, while the workspace-local marketplace page is legacy cleanup debt with a command-based MCP install model that does not match the current backend `/api/v1/app/mcp-servers` URL/auth-token contract.

## Verification

```bash
COREPACK_HOME=.tmp/corepack pnpm install --frozen-lockfile
```

Passed after installing the missing local dependencies needed for Vitest.

```bash
COREPACK_HOME=.tmp/corepack pnpm --dir src/web test -- src/services/http/client.test.ts src/features/auth/ProtectedRoute.test.tsx src/routes/workspace/KnowledgePage.test.tsx src/routes/workspace/SoloPage.test.tsx src/features/layouts/WorkspaceLayout.test.tsx
```

Passed. Vitest ran the web suite: 32 test files, 110 tests.

```bash
rg -n "AgentSummary|MemoryDocumentSummary|McpServer|Notification|QuotaSnapshot|PackageOption" src/web/src/types/api.ts
```

Passed. All required exported type names are present.

```bash
rg -n "Create knowledge base|Search knowledge|Start solo run|Install Server" src/web/src/routes/workspace
```

Passed for the active workspace CTA copy. `Install Server` remains excluded with the legacy workspace marketplace page decision above.

```bash
COREPACK_HOME=.tmp/corepack pnpm --dir src/web build
```

Passed. TypeScript and Vite built successfully.

```bash
git status --short -- src/web/dist src/web/test-results src/web/playwright-report .tmp node_modules src/web/node_modules
```

Passed. No generated or cache artifacts were staged for this plan.
