# M2-A Knowledge Beta Closeout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把当前 Knowledge CRUD 与 retrieval 链路收口为可依赖的 Beta 子模块，在不改变公开接口 shape 的前提下，完成结果质量优化、页面用户流稳定化、回归补齐与文档追平。

**Architecture:** 先在后端 `knowledge` store 内部收敛 retrieval 排序与 snippet 生成逻辑，再在前端 `KnowledgePage` 中把 CRUD、详情、编辑器与 retrieval 的状态切换收稳，最后同步更新契约文档与历史阶段判断。整个子项目保持 `/api/v1/app/knowledge-bases/...` 的 path 与 response shape 不变，避免把 `M2-A` 演变成 `M3` 级能力重构。

**Tech Stack:** Go `net/http` + `database/sql`, React 18 + React Router, Vitest + Testing Library, Markdown architecture / progress docs, Bash root quality gates

---

## File Structure

- Create: `src/server/internal/knowledge/store_test.go`
  - 直接验证 retrieval 排序辅助逻辑与 snippet 生成逻辑，不依赖数据库集成环境
- Modify: `src/server/internal/knowledge/store.go`
  - 收敛 retrieval 排序、snippet 选取与空结果行为
- Modify: `src/server/internal/knowledge/service.go`
  - 统一 retrieval 查询的输入规范化，保持公开接口不变
- Modify: `src/server/internal/knowledge/service_test.go`
  - 覆盖查询规范化、空结果与 retrieval 结果返回语义
- Modify: `src/server/internal/http/knowledge_handler_test.go`
  - 覆盖 retrieval 请求 trim、空结果响应与 handler 层稳定语义
- Modify: `src/web/src/routes/workspace/KnowledgePage.tsx`
  - 收敛 Knowledge Beta 页面状态流，提升 retrieval 结果展示、空结果反馈和 CRUD/retrieval 切换稳定性
- Modify: `src/web/src/routes/workspace/KnowledgePage.test.tsx`
  - 补齐 retrieval 成功、空结果、CRUD 后检索状态重置与回跳场景
- Modify: `docs/architecture/current-system-contracts.md`
  - 把 Knowledge 能力边界从“可运行 MVP”更新到“Beta 可依赖”
- Modify: `docs/reports/2026-04-04-progress-plan.md`
  - 只更新与 Knowledge / `M2` / retrieval 边界直接相关的历史判断
- Modify: `docs/reports/2026-04-06-execution-progress-review.md`
  - 同上，追平 `Task 6`、`Task 10`、`M2/M3` 与 Knowledge Beta 的现实状态

## Task 1: Harden Backend Retrieval Ranking And Snippet Logic

**Files:**
- Create: `src/server/internal/knowledge/store_test.go`
- Modify: `src/server/internal/knowledge/store.go`
- Modify: `src/server/internal/knowledge/service.go`
- Modify: `src/server/internal/knowledge/service_test.go`

- [ ] **Step 1: Write the failing backend tests for ranking, snippets, and query normalization**

```go
// src/server/internal/knowledge/store_test.go
package knowledge

import (
	"database/sql"
	"strings"
	"testing"
)

func TestScoreKnowledgeCandidatePrefersTitleMatchesOverChunkAndBody(t *testing.T) {
	terms := buildKnowledgeQueryTerms("deployment")

	titleScore := scoreKnowledgeCandidate("Deployment Runbook", "general notes", sql.NullString{}, terms)
	chunkScore := scoreKnowledgeCandidate("Runbook", "general notes", sql.NullString{String: "deployment checklist", Valid: true}, terms)
	bodyScore := scoreKnowledgeCandidate("Runbook", "deployment lives in the body", sql.NullString{}, terms)

	if !(titleScore > chunkScore && chunkScore > bodyScore) {
		t.Fatalf("expected title > chunk > body, got title=%d chunk=%d body=%d", titleScore, chunkScore, bodyScore)
	}
}

func TestBuildKnowledgeSnippetCentersTheMatchedTerm(t *testing.T) {
	content := "alpha beta gamma delta epsilon zeta eta theta iota kappa lambda deployment controls are documented after this section with rollback notes"
	snippet := buildKnowledgeSnippet(content, "deployment controls")

	if snippet == "" {
		t.Fatal("expected non-empty snippet")
	}
	if snippet == content {
		t.Fatalf("expected centered snippet, got full content %q", snippet)
	}
	if !strings.Contains(strings.ToLower(snippet), "deployment controls") {
		t.Fatalf("expected snippet to contain query, got %q", snippet)
	}
	if !strings.HasPrefix(snippet, "...") {
		t.Fatalf("expected leading ellipsis for centered snippet, got %q", snippet)
	}
}

func TestChooseKnowledgeSnippetSourcePrefersChunkWhenChunkHasMoreTermHits(t *testing.T) {
	terms := buildKnowledgeQueryTerms("deployment rollback")

	source := chooseKnowledgeSnippetSource(
		"General architecture notes without the query terms together.",
		sql.NullString{String: "Deployment rollback steps are documented in this chunk.", Valid: true},
		terms,
	)

	if source != "Deployment rollback steps are documented in this chunk." {
		t.Fatalf("expected chunk source, got %q", source)
	}
}
```

```go
// src/server/internal/knowledge/service_test.go
func TestRetrieveNormalizesKnowledgeQueryBeforeCallingStore(t *testing.T) {
	store := &fakeStore{
		retrievalResults: []KnowledgeRetrievalResult{},
	}
	service := NewService(store)

	if _, err := service.Retrieve(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "kb_7", "  deployment   rollback  "); err != nil {
		t.Fatalf("retrieve knowledge: %v", err)
	}

	if store.retrievalQuery != "deployment rollback" {
		t.Fatalf("expected normalized query %q, got %q", "deployment rollback", store.retrievalQuery)
	}
}
```

- [ ] **Step 2: Run the backend Knowledge tests to verify they fail**

Run:

```bash
cd src/server
go test ./internal/knowledge -run 'TestScoreKnowledgeCandidatePrefersTitleMatchesOverChunkAndBody|TestBuildKnowledgeSnippetCentersTheMatchedTerm|TestChooseKnowledgeSnippetSourcePrefersChunkWhenChunkHasMoreTermHits|TestRetrieveNormalizesKnowledgeQueryBeforeCallingStore' -v
```

Expected: FAIL because the helper functions do not exist yet and `Service.Retrieve` still passes the raw query through.

- [ ] **Step 3: Implement retrieval helper functions and query normalization**

```go
// src/server/internal/knowledge/service.go
import (
	"context"
	"database/sql"
	"strings"
	"time"

	"oblivious/server/internal/auth"
)

func normalizeKnowledgeQuery(query string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(query)), " ")
}

func (s *Service) Retrieve(ctx context.Context, session auth.Session, knowledgeBaseID, query string) ([]KnowledgeRetrievalResult, error) {
	return s.store.RetrieveKnowledge(ctx, session.WorkspaceID, knowledgeBaseID, normalizeKnowledgeQuery(query))
}
```

```go
// src/server/internal/knowledge/store.go
import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"
	"unicode"

	"oblivious/server/internal/auth"
)

type knowledgeRetrievalCandidate struct {
	documentID    string
	documentTitle string
	documentBody  string
	chunkContent  sql.NullString
	chunkIndex    int
	updatedAt     time.Time
}

func buildKnowledgeQueryTerms(query string) []string {
	normalized := normalizeKnowledgeQuery(query)
	if normalized == "" {
		return nil
	}

	return strings.Fields(strings.ToLower(normalized))
}

func countKnowledgeTermHits(content string, terms []string) int {
	lowerContent := strings.ToLower(content)
	hits := 0
	for _, term := range terms {
		if term != "" && strings.Contains(lowerContent, term) {
			hits++
		}
	}
	return hits
}

func scoreKnowledgeCandidate(title, body string, chunk sql.NullString, terms []string) int {
	titleHits := countKnowledgeTermHits(title, terms)
	bodyHits := countKnowledgeTermHits(body, terms)
	chunkHits := 0
	if chunk.Valid {
		chunkHits = countKnowledgeTermHits(chunk.String, terms)
	}

	score := titleHits*100 + chunkHits*25 + bodyHits*10
	if titleHits == len(terms) && len(terms) > 0 {
		score += 50
	}
	return score
}

func chooseKnowledgeSnippetSource(body string, chunk sql.NullString, terms []string) string {
	if chunk.Valid && countKnowledgeTermHits(chunk.String, terms) >= countKnowledgeTermHits(body, terms) {
		return chunk.String
	}
	return body
}
```

- [ ] **Step 4: Route retrieval through the new ranking helpers**

```go
// src/server/internal/knowledge/store.go
func (s *SQLStore) RetrieveKnowledge(ctx context.Context, workspaceID, knowledgeBaseID, query string) ([]KnowledgeRetrievalResult, error) {
	normalizedQuery := normalizeKnowledgeQuery(query)
	if normalizedQuery == "" {
		return []KnowledgeRetrievalResult{}, nil
	}

	terms := buildKnowledgeQueryTerms(normalizedQuery)
	pattern := "%" + escapeLikePattern(normalizedQuery) + "%"
	rows, err := s.db.QueryContext(ctx, `
		SELECT d.id, d.title, d.content, c.content, COALESCE(c.chunk_index, -1), d.updated_at
		FROM knowledge_documents d
		JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
		LEFT JOIN knowledge_document_chunks c ON c.document_id = d.id
		WHERE kb.workspace_id = $1 AND d.knowledge_base_id = $2 AND (
			d.title ILIKE $3 ESCAPE '\'
			OR d.content ILIKE $3 ESCAPE '\'
			OR c.content ILIKE $3 ESCAPE '\'
		)
	`, workspaceID, knowledgeBaseID, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates := []knowledgeRetrievalCandidate{}
	for rows.Next() {
		var candidate knowledgeRetrievalCandidate
		if err := rows.Scan(
			&candidate.documentID,
			&candidate.documentTitle,
			&candidate.documentBody,
			&candidate.chunkContent,
			&candidate.chunkIndex,
			&candidate.updatedAt,
		); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		leftScore := scoreKnowledgeCandidate(candidates[i].documentTitle, candidates[i].documentBody, candidates[i].chunkContent, terms)
		rightScore := scoreKnowledgeCandidate(candidates[j].documentTitle, candidates[j].documentBody, candidates[j].chunkContent, terms)
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		if !candidates[i].updatedAt.Equal(candidates[j].updatedAt) {
			return candidates[i].updatedAt.After(candidates[j].updatedAt)
		}
		if candidates[i].documentTitle != candidates[j].documentTitle {
			return candidates[i].documentTitle < candidates[j].documentTitle
		}
		return candidates[i].chunkIndex < candidates[j].chunkIndex
	})

	results := make([]KnowledgeRetrievalResult, 0, knowledgeRetrievalLimit)
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		source := chooseKnowledgeSnippetSource(candidate.documentBody, candidate.chunkContent, terms)
		snippet := buildKnowledgeSnippet(source, normalizedQuery)
		if snippet == "" {
			continue
		}

		resultKey := candidate.documentID + "|" + snippet
		if _, exists := seen[resultKey]; exists {
			continue
		}
		seen[resultKey] = struct{}{}

		results = append(results, KnowledgeRetrievalResult{
			DocumentID:    candidate.documentID,
			DocumentTitle: candidate.documentTitle,
			Snippet:       snippet,
		})
		if len(results) == knowledgeRetrievalLimit {
			break
		}
	}

	return results, nil
}
```

- [ ] **Step 5: Re-run the backend Knowledge tests**

Run:

```bash
cd src/server
go test ./internal/knowledge -run 'TestScoreKnowledgeCandidatePrefersTitleMatchesOverChunkAndBody|TestBuildKnowledgeSnippetCentersTheMatchedTerm|TestChooseKnowledgeSnippetSourcePrefersChunkWhenChunkHasMoreTermHits|TestRetrieveNormalizesKnowledgeQueryBeforeCallingStore' -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add \
  src/server/internal/knowledge/store.go \
  src/server/internal/knowledge/store_test.go \
  src/server/internal/knowledge/service.go \
  src/server/internal/knowledge/service_test.go
git commit -m "feat(knowledge): improve retrieval ranking and snippets"
```

## Task 2: Lock Retrieval API Semantics And Regression Paths

**Files:**
- Modify: `src/server/internal/http/knowledge_handler_test.go`

- [ ] **Step 1: Write the failing handler tests for trimmed query and empty retrieval results**

```go
// src/server/internal/http/knowledge_handler_test.go
func TestKnowledgeHandlerRetrieveTrimsAndNormalizesQuery(t *testing.T) {
	store := &knowledgeFakeStore{
		retrievalResults: []knowledge.KnowledgeRetrievalResult{},
	}
	handler := newKnowledgeHandler(knowledge.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/retrieve", strings.NewReader(`{"query":"  deployment   rollback  "}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.retrieveKnowledge(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if store.retrievalQuery != "deployment rollback" {
		t.Fatalf("expected normalized retrieval query, got %q", store.retrievalQuery)
	}
}

func TestKnowledgeHandlerRetrieveReturnsEmptyListWhenNoMatchExists(t *testing.T) {
	store := &knowledgeFakeStore{
		retrievalResults: []knowledge.KnowledgeRetrievalResult{},
	}
	handler := newKnowledgeHandler(knowledge.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/retrieve", strings.NewReader(`{"query":"deployment"}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.retrieveKnowledge(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	var response struct {
		Data []knowledge.KnowledgeRetrievalResult `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 0 {
		t.Fatalf("expected empty retrieval results, got %d", len(response.Data))
	}
}
```

- [ ] **Step 2: Run the handler regression tests to verify failure**

Run:

```bash
cd src/server
go test ./internal/http -run 'TestKnowledgeHandlerRetrieveTrimsAndNormalizesQuery|TestKnowledgeHandlerRetrieveReturnsEmptyListWhenNoMatchExists' -v
```

Expected: FAIL because the new handler tests do not exist yet, or query normalization is not yet visible at handler level.

- [ ] **Step 3: Re-run the handler regression tests after adding them**

Run:

```bash
cd src/server
go test ./internal/http -run 'TestKnowledgeHandlerRetrieveTrimsAndNormalizesQuery|TestKnowledgeHandlerRetrieveReturnsEmptyListWhenNoMatchExists' -v
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add src/server/internal/http/knowledge_handler_test.go
git commit -m "test(knowledge): lock retrieval handler semantics"
```

## Task 3: Close Out KnowledgePage Retrieval UX And State Flow

**Files:**
- Modify: `src/web/src/routes/workspace/KnowledgePage.tsx`
- Modify: `src/web/src/routes/workspace/KnowledgePage.test.tsx`

- [ ] **Step 1: Write the failing front-end tests for empty-result feedback and retrieval-state reset**

```tsx
// src/web/src/routes/workspace/KnowledgePage.test.tsx
it('shows query-specific empty feedback when retrieval returns no snippets', async () => {
  routeState.knowledgeBaseId = 'kb_9';
  getKnowledgeBase.mockResolvedValue({
    documentCount: 1,
    id: 'kb_9',
    name: 'Architecture Notes',
    updatedAt: '2026-04-03T11:30:00Z'
  });
  listKnowledgeDocuments.mockResolvedValue([]);
  retrieveKnowledge.mockResolvedValue([]);

  render(<KnowledgePage />);

  await screen.findByRole('heading', { name: 'Architecture Notes' });
  fireEvent.change(screen.getByLabelText('Retrieval query'), { target: { value: 'deployment rollback' } });
  fireEvent.click(screen.getByRole('button', { name: 'Search knowledge' }));

  expect(await screen.findByText('No matching snippets found for “deployment rollback”.')).toBeInTheDocument();
});

it('clears stale retrieval results after saving a document', async () => {
  routeState.knowledgeBaseId = 'kb_9';
  getKnowledgeBase.mockResolvedValue({
    documentCount: 1,
    id: 'kb_9',
    name: 'Architecture Notes',
    updatedAt: '2026-04-03T11:30:00Z'
  });
  listKnowledgeDocuments.mockResolvedValue([
    {
      content: 'System boundaries',
      id: 'doc_1',
      title: 'Overview',
      updatedAt: '2026-04-03T11:45:00Z'
    }
  ]);
  retrieveKnowledge.mockResolvedValue([
    {
      documentId: 'doc_1',
      documentTitle: 'Overview',
      snippet: 'System boundaries include deployment controls.'
    }
  ]);
  updateKnowledgeDocument.mockResolvedValue({
    content: 'Updated boundaries',
    id: 'doc_1',
    title: 'Overview v2',
    updatedAt: '2026-04-03T12:15:00Z'
  });

  render(<KnowledgePage />);

  await screen.findByRole('heading', { name: 'Architecture Notes' });
  fireEvent.change(screen.getByLabelText('Retrieval query'), { target: { value: 'deployment' } });
  fireEvent.click(screen.getByRole('button', { name: 'Search knowledge' }));
  expect(await screen.findByText('System boundaries include deployment controls.')).toBeInTheDocument();

  fireEvent.click(screen.getByRole('button', { name: 'Edit document Overview' }));
  fireEvent.change(screen.getByLabelText('Document title'), { target: { value: 'Overview v2' } });
  fireEvent.change(screen.getByLabelText('Document content'), { target: { value: 'Updated boundaries' } });
  fireEvent.click(screen.getByRole('button', { name: 'Save document' }));

  await waitFor(() => {
    expect(updateKnowledgeDocument).toHaveBeenCalledWith('kb_9', 'doc_1', {
      content: 'Updated boundaries',
      title: 'Overview v2'
    });
  });

  expect(screen.queryByText('System boundaries include deployment controls.')).not.toBeInTheDocument();
});
```

- [ ] **Step 2: Run the Knowledge page tests to verify they fail**

Run:

```bash
pnpm --dir src/web exec vitest run src/routes/workspace/KnowledgePage.test.tsx
```

Expected: FAIL because the current page only shows a generic empty result message and does not clear stale retrieval output after document mutation.

- [ ] **Step 3: Implement the retrieval UX and state reset improvements**

```tsx
// src/web/src/routes/workspace/KnowledgePage.tsx
const [lastRetrievedQuery, setLastRetrievedQuery] = useState('');

const resetKnowledgeRetrieval = () => {
  setHasRetrievedKnowledge(false);
  setLastRetrievedQuery('');
  setRetrievalQuery('');
  setRetrievalResults([]);
};

const handleRetrieveKnowledge = async () => {
  if (!knowledgeBaseId) {
    return;
  }

  const trimmedQuery = retrievalQuery.trim();
  if (trimmedQuery === '') {
    return;
  }

  setIsRetrievingKnowledge(true);
  setError(null);

  try {
    const nextResults = await knowledgeApi.retrieveKnowledge(knowledgeBaseId, { query: trimmedQuery });
    setHasRetrievedKnowledge(true);
    setLastRetrievedQuery(trimmedQuery);
    setRetrievalResults(nextResults);
  } catch {
    setError('Unable to retrieve knowledge.');
  } finally {
    setIsRetrievingKnowledge(false);
  }
};

// inside create/update/delete document success branches
resetKnowledgeRetrieval()

// inside selectedKnowledgeBase detail view
{hasRetrievedKnowledge ? <h2>Matched snippets</h2> : null}
{hasRetrievedKnowledge && retrievalResults.length === 0 ? (
  <p>{`No matching snippets found for “${lastRetrievedQuery}”.`}</p>
) : null}
{retrievalResults.length > 0 ? (
  <ul>
    {retrievalResults.map((result) => (
      <li key={`${result.documentId}-${result.snippet}`}>
        <strong>{result.documentTitle}</strong>
        <p>{result.snippet}</p>
      </li>
    ))}
  </ul>
) : null}
```

- [ ] **Step 4: Re-run the Knowledge page tests**

Run:

```bash
pnpm --dir src/web exec vitest run src/routes/workspace/KnowledgePage.test.tsx
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add \
  src/web/src/routes/workspace/KnowledgePage.tsx \
  src/web/src/routes/workspace/KnowledgePage.test.tsx
git commit -m "feat(knowledge): close out beta retrieval flow"
```

## Task 4: Update Knowledge Beta Documentation And Historical M2/M3 Reporting

**Files:**
- Modify: `docs/architecture/current-system-contracts.md`
- Modify: `docs/reports/2026-04-04-progress-plan.md`
- Modify: `docs/reports/2026-04-06-execution-progress-review.md`

- [ ] **Step 1: Write the failing documentation consistency checks**

Run:

```bash
rg -n --fixed-strings "当前提供基于 `ILIKE` 的 retrieval MVP，不包含向量检索" docs/architecture/current-system-contracts.md
rg -n --fixed-strings "Knowledge 页面完成列表/详情/文档 CRUD" docs/reports/2026-04-04-progress-plan.md
rg -n --fixed-strings "Task 10 | Deliver Knowledge Retrieval MVP | 未启动 | 0% | 仅有 CRUD，没有 retrieval |" docs/reports/2026-04-06-execution-progress-review.md
```

Expected: all three commands return matches, proving the docs still describe pre-closeout Knowledge state.

- [ ] **Step 2: Update the current system contract for Knowledge Beta**

````md
<!-- docs/architecture/current-system-contracts.md -->
说明：

- 当前支持知识库/文档 CRUD
- 当前在文档创建与更新时做最小 chunking
- 当前 retrieval 已进入 Knowledge Beta：维持现有 `/retrieve` 接口 shape，但结果排序、snippet 质量、空结果反馈和页面回归均按 Beta 标准收口
- 当前 retrieval 仍基于文本匹配，不包含向量检索、embedding 或异步 ingestion pipeline
````

- [ ] **Step 3: Update the roadmap report for Knowledge Beta and M3 boundary**

````md
<!-- docs/reports/2026-04-04-progress-plan.md -->
#### 具体工作

- Chat 页面从占位升级为可用界面
- Settings / Onboarding 实现测试目标中的交互
- Console 首页、Usage、Models、Billing、Access 页面接入真实数据
- Knowledge 页面完成列表/详情/文档 CRUD，并把当前 retrieval 一并收口到 Beta 可依赖状态
- SOLO 页面完成创建、启动、暂停、恢复、预算更新、结果桥接聊天

#### 里程碑交付物

- M2：工作区 Beta
- Chat UI
- Knowledge CRUD + retrieval Beta
- SOLO starter UI
- Settings / Onboarding
- Console 页面

#### 具体工作

- Knowledge：在现有 `/retrieve` 接口之外，才允许继续推进 ingestion、indexing 与更重的检索能力
- Chat：streaming、provider abstraction、错误降级
````

- [ ] **Step 4: Update the execution review to reflect Knowledge’s new Beta status**

```md
| M2 工作区 Beta | 2026-05-15 | 进行中 | 25% | Knowledge 已完成首个 Beta 子项目收口，其余 Chat / SOLO / Console 闭环仍待推进 |
| M3 能力 Beta | 2026-06-05 | 未开始 | 5% | advanced retrieval、SOLO runtime、Chat streaming 仍未展开 |
```

```md
| Task 6 | Finish Knowledge Workspace CRUD Integration | 已完成 Beta 收口 | 100% | Knowledge CRUD、retrieval 结果质量、页面回归与文档说明已进入 Beta 可依赖状态 |
| Task 10 | Deliver Knowledge Retrieval MVP | 已完成 M2-A 范围 | 40% | 当前文本 retrieval 已稳定进入 Beta，但更重的 ingestion / indexing / M3 检索能力尚未展开 |
```

- [ ] **Step 5: Re-run the documentation checks**

Run:

```bash
rg -n --fixed-strings "Knowledge CRUD + retrieval Beta" docs/reports/2026-04-04-progress-plan.md
rg -n --fixed-strings "当前 retrieval 已进入 Knowledge Beta" docs/architecture/current-system-contracts.md
rg -n --fixed-strings "Task 10 | Deliver Knowledge Retrieval MVP | 已完成 M2-A 范围 | 40%" docs/reports/2026-04-06-execution-progress-review.md
rg -n --fixed-strings "Task 10 | Deliver Knowledge Retrieval MVP | 未启动 | 0% | 仅有 CRUD，没有 retrieval |" docs/reports/2026-04-06-execution-progress-review.md
```

Expected:

- the first three `rg` commands return matches
- the last `rg` command returns no matches

- [ ] **Step 6: Commit**

```bash
git add \
  docs/architecture/current-system-contracts.md \
  docs/reports/2026-04-04-progress-plan.md \
  docs/reports/2026-04-06-execution-progress-review.md
git commit -m "docs(knowledge): close out M2-A beta scope"
```

## Task 5: Run Final M2-A Verification

**Files:**
- Verify: `src/server/internal/knowledge/store.go`
- Verify: `src/server/internal/knowledge/store_test.go`
- Verify: `src/server/internal/knowledge/service.go`
- Verify: `src/server/internal/knowledge/service_test.go`
- Verify: `src/server/internal/http/knowledge_handler_test.go`
- Verify: `src/web/src/routes/workspace/KnowledgePage.tsx`
- Verify: `src/web/src/routes/workspace/KnowledgePage.test.tsx`
- Verify: `docs/architecture/current-system-contracts.md`
- Verify: `docs/reports/2026-04-04-progress-plan.md`
- Verify: `docs/reports/2026-04-06-execution-progress-review.md`

- [ ] **Step 1: Verify the backend Knowledge slice**

Run:

```bash
cd src/server
go test ./internal/knowledge ./internal/http -run Knowledge -v
```

Expected: PASS

- [ ] **Step 2: Verify the front-end Knowledge slice**

Run:

```bash
pnpm --dir src/web exec vitest run src/routes/workspace/KnowledgePage.test.tsx
```

Expected: PASS

- [ ] **Step 3: Verify the root quality gates**

Run:

```bash
bash scripts/check.sh
bash scripts/test.sh
```

Expected: PASS. If `TEST_DATABASE_URL` is not set, accept the explicit integration-test skip message.

- [ ] **Step 4: Verify diff hygiene**

Run:

```bash
git diff --check
git status --short -- \
  src/server/internal/knowledge/store.go \
  src/server/internal/knowledge/store_test.go \
  src/server/internal/knowledge/service.go \
  src/server/internal/knowledge/service_test.go \
  src/server/internal/http/knowledge_handler_test.go \
  src/web/src/routes/workspace/KnowledgePage.tsx \
  src/web/src/routes/workspace/KnowledgePage.test.tsx \
  docs/architecture/current-system-contracts.md \
  docs/reports/2026-04-04-progress-plan.md \
  docs/reports/2026-04-06-execution-progress-review.md
```

Expected:

- `git diff --check` returns no whitespace / conflict-marker errors
- scoped `git status --short -- ...` only shows the intended M2-A implementation files

- [ ] **Step 5: Commit**

```bash
git add \
  src/server/internal/knowledge/store.go \
  src/server/internal/knowledge/store_test.go \
  src/server/internal/knowledge/service.go \
  src/server/internal/knowledge/service_test.go \
  src/server/internal/http/knowledge_handler_test.go \
  src/web/src/routes/workspace/KnowledgePage.tsx \
  src/web/src/routes/workspace/KnowledgePage.test.tsx \
  docs/architecture/current-system-contracts.md \
  docs/reports/2026-04-04-progress-plan.md \
  docs/reports/2026-04-06-execution-progress-review.md
git commit -m "feat(knowledge): close out M2-A knowledge beta"
```
