# Mainline Engineering Governance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 锁定当前版本基线，按 ROADMAP 先完成工程化治理主线：冻结唯一执行基线、清理历史文档权威漂移、收敛能力表述、拆解后端装配点与前端大页面，为后续 Knowledge 与 SOLO 深化提供稳定底座。

**Architecture:** 先做认知与文档治理，再做结构治理，最后为能力深化建立决策入口。主线只覆盖 `src/server`、`src/web`、`config`、`scripts`、`.github/workflows` 与主线文档；`new-api` 和 `lobehub` 保持 reference 角色。所有变更围绕当前代码现实，不反向迎合旧报告。

**Tech Stack:** Go (`net/http`, `database/sql`), React + TypeScript + React Router, pnpm, Vitest, shell scripts, Markdown 文档治理

---

## File Structure

### Governance / Docs
- Modify: `README.md`
  - 明确唯一主线、reference repos、基线文档入口。
- Modify: `docs/architecture/current-system-contracts.md`
  - 对齐 CURRENT_STATUS 与 ROADMAP 的权威层级、能力边界与 DoD 门面。
- Modify: `CURRENT_STATUS.md`
  - 修正当前文件末尾误拼入 `ROADMAP.md` 的结构错误，保持文档单一职责。
- Modify: `ROADMAP.md`
  - 如实施过程中发现里程碑排序需微调，仅做最小更新。
- Create: `docs/superpowers/plans/2026-04-08-mainline-engineering-governance-implementation.md`
  - 本实施计划。
- Modify or archive intent in: `docs/reports/2026-04-04-codebase-analysis.md`, `docs/reports/2026-04-04-progress-plan.md`, `docs/reports/2026-04-04-todo-tracker.md`, `docs/reports/2026-04-05-technical-audit.md`, `docs/reports/2026-04-05-todo-tracker.md`
  - 统一加上“历史材料/非现状依据”标识，避免继续误导。

### Backend structure
- Modify: `src/server/internal/http/router.go`
  - 将集中注册逻辑拆为模块级 register 函数调用。
- Create: `src/server/internal/http/routes_auth.go`
- Create: `src/server/internal/http/routes_preferences.go`
- Create: `src/server/internal/http/routes_chat.go`
- Create: `src/server/internal/http/routes_knowledge.go`
- Create: `src/server/internal/http/routes_tasks.go`
- Create: `src/server/internal/http/routes_console.go`
  - 每个文件负责一个 bounded route surface。
- Test: `src/server/internal/http/server_test.go`
  - 保持现有 server/router 行为不回退。
- Test: `src/server/internal/http/chat_handler_test.go`
- Test: `src/server/internal/http/knowledge_handler_test.go`
- Test: `src/server/internal/http/task_handler_test.go`

### Frontend structure
- Modify: `src/web/src/routes/workspace/KnowledgePage.tsx`
- Modify: `src/web/src/routes/workspace/SoloPage.tsx`
  - 将页面层从“吞动作、吞状态、吞格式化”收敛为容器。
- Create: `src/web/src/routes/workspace/knowledge/useKnowledgePageState.ts`
- Create: `src/web/src/routes/workspace/knowledge/KnowledgePageView.tsx`
- Create: `src/web/src/routes/workspace/solo/useSoloPageState.ts`
- Create: `src/web/src/routes/workspace/solo/SoloPageView.tsx`
  - 拆出页面状态与展示层，降低页面主文件复杂度。
- Test: `src/web/src/routes/workspace/KnowledgePage.test.tsx`
- Test: `src/web/src/routes/workspace/SoloPage.test.tsx`

### Quality / validation
- Modify if needed: `scripts/check.sh`
- Modify if needed: `scripts/test.sh`
  - 只在里程碑 DoD 需要显式新增验证时做最小补充。

---

### Task 1: 锁定唯一执行基线并修正文档结构错误

**Files:**
- Modify: `CURRENT_STATUS.md`
- Modify: `README.md`
- Modify: `docs/architecture/current-system-contracts.md`
- Test: `CURRENT_STATUS.md`

- [ ] **Step 1: 修正 `CURRENT_STATUS.md` 尾部误拼接内容的失败检查**

```bash
python - <<'PY'
from pathlib import Path
text = Path('/home/shirosora/code_storage/Oblivious/CURRENT_STATUS.md').read_text()
assert '\n# ROADMAP.md\n' not in text, 'CURRENT_STATUS.md still contains pasted ROADMAP content'
PY
```

Expected: FAIL with `CURRENT_STATUS.md still contains pasted ROADMAP content`

- [ ] **Step 2: 运行检查确认失败**

Run: `python - <<'PY' ... PY`
Expected: 退出非 0，并输出 `CURRENT_STATUS.md still contains pasted ROADMAP content`

- [ ] **Step 3: 最小修改 `CURRENT_STATUS.md`，删除误拼接的 ROADMAP 段落**

```md
## 10. 总结与下一步建议

### 10.1 总结

这个项目已经不再是“有没有东西”的问题，而是“有没有把真相说清楚”的问题。

- 主线已经明确：`src/server` + `src/web`
- 核心闭环已经形成：Auth / Chat / Knowledge / SOLO / Console
- 当前最严重的问题不是功能缺失，而是：
  1. 历史文档仍在制造旧真相
  2. SOLO 与 Knowledge 的能力深度被高估
  3. 路由装配与大页面开始暴露 Vibe Coding 的结构后遗症

### 10.2 下一步建议

1. **立即冻结文档权威层级**：CURRENT_STATUS.md 与 ROADMAP.md 成为新的执行基线；旧报告降级为历史材料。
2. **先做工程化对齐，不急着加新功能**：拆装配点、拆大页面、收敛能力边界表述。
3. **把 SOLO 和 Knowledge 的“能力真相”写死到路线图里**：避免再用大词掩盖浅实现。
4. **将 Project Health Score 目标定义为两步走**：先从 71 拉到 78-82（可稳定治理），再评估是否进入更深能力迭代。
```

- [ ] **Step 4: 在 `README.md` 增加唯一执行基线入口**

```md
## Execution Baseline

The current engineering baseline is defined by:
- `CURRENT_STATUS.md`
- `ROADMAP.md`
- `docs/architecture/current-system-contracts.md`

Historical files under `docs/reports/` are reference material only. They are not the source of truth for current project state.
```

- [ ] **Step 5: 在 `docs/architecture/current-system-contracts.md` 增加权威层级说明**

```md
## 0. Authority

当前主线权威层级如下：

1. 代码与运行脚本
2. `CURRENT_STATUS.md`
3. `ROADMAP.md`
4. 本文档 `docs/architecture/current-system-contracts.md`
5. `docs/reports/*` 历史材料

若历史报告与当前代码或上述基线文档冲突，以代码与基线文档为准。
```

- [ ] **Step 6: 运行文档结构检查**

Run: `python - <<'PY'
from pathlib import Path
text = Path('/home/shirosora/code_storage/Oblivious/CURRENT_STATUS.md').read_text()
assert '\n# ROADMAP.md\n' not in text
assert '## 10. 总结与下一步建议' in text
print('ok')
PY`
Expected: `ok`

- [ ] **Step 7: 提交本任务**

```bash
git add CURRENT_STATUS.md README.md docs/architecture/current-system-contracts.md
git commit -m "docs: lock mainline execution baseline"
```

### Task 2: 降级历史报告权威并防止继续误导

**Files:**
- Modify: `docs/reports/2026-04-04-codebase-analysis.md`
- Modify: `docs/reports/2026-04-04-progress-plan.md`
- Modify: `docs/reports/2026-04-04-todo-tracker.md`
- Modify: `docs/reports/2026-04-05-technical-audit.md`
- Modify: `docs/reports/2026-04-05-todo-tracker.md`
- Test: `docs/reports/*.md`

- [ ] **Step 1: 为历史报告写失败检查，确认它们尚未声明“非现状依据”**

```bash
python - <<'PY'
from pathlib import Path
files = [
    'docs/reports/2026-04-04-codebase-analysis.md',
    'docs/reports/2026-04-04-progress-plan.md',
    'docs/reports/2026-04-04-todo-tracker.md',
    'docs/reports/2026-04-05-technical-audit.md',
    'docs/reports/2026-04-05-todo-tracker.md',
]
missing = []
for file in files:
    text = Path('/home/shirosora/code_storage/Oblivious', file).read_text()
    if '历史材料，非当前现状依据' not in text:
        missing.append(file)
assert not missing, f'missing historical-warning header: {missing}'
PY
```

Expected: FAIL with missing file list

- [ ] **Step 2: 运行检查确认失败**

Run: `python - <<'PY' ... PY`
Expected: FAIL with `missing historical-warning header`

- [ ] **Step 3: 在每份历史报告开头插入统一警示块**

```md
> **状态声明**
>
> 本文件属于历史材料，非当前现状依据。
> 当前项目状态、执行基线与路线图请以以下文件为准：
> - `CURRENT_STATUS.md`
> - `ROADMAP.md`
> - `docs/architecture/current-system-contracts.md`
```

- [ ] **Step 4: 在 `2026-04-05-technical-audit.md` 与 `2026-04-04-progress-plan.md` 加显式过时说明**

```md
> **过时说明**
>
> 本文中的部分判断已被后续代码与基线文档覆盖，保留仅用于追溯阶段性认知，不得直接转化为当前 backlog。
```

- [ ] **Step 5: 运行历史材料头部检查**

Run: `python - <<'PY'
from pathlib import Path
files = [
    'docs/reports/2026-04-04-codebase-analysis.md',
    'docs/reports/2026-04-04-progress-plan.md',
    'docs/reports/2026-04-04-todo-tracker.md',
    'docs/reports/2026-04-05-technical-audit.md',
    'docs/reports/2026-04-05-todo-tracker.md',
]
for file in files:
    text = Path('/home/shirosora/code_storage/Oblivious', file).read_text()
    assert '历史材料，非当前现状依据' in text, file
print('ok')
PY`
Expected: `ok`

- [ ] **Step 6: 提交本任务**

```bash
git add docs/reports/2026-04-04-codebase-analysis.md docs/reports/2026-04-04-progress-plan.md docs/reports/2026-04-04-todo-tracker.md docs/reports/2026-04-05-technical-audit.md docs/reports/2026-04-05-todo-tracker.md
git commit -m "docs: demote stale audit reports"
```

### Task 3: 将 root 验证入口固化为里程碑 DoD 门面

**Files:**
- Modify: `README.md`
- Modify: `ROADMAP.md`
- Modify: `scripts/check.sh`
- Modify: `scripts/test.sh`
- Test: `scripts/check.sh`
- Test: `scripts/test.sh`

- [ ] **Step 1: 写失败检查，确认 README 尚未包含统一 DoD 章节**

```bash
python - <<'PY'
from pathlib import Path
text = Path('/home/shirosora/code_storage/Oblivious/README.md').read_text()
assert '## Milestone DoD Commands' in text, 'README missing milestone DoD section'
PY
```

Expected: FAIL with `README missing milestone DoD section`

- [ ] **Step 2: 运行检查确认失败**

Run: `python - <<'PY' ... PY`
Expected: FAIL

- [ ] **Step 3: 在 README 中加入统一 DoD 章节**

```md
## Milestone DoD Commands

Use these commands as the default milestone verification surface:

```bash
bash scripts/check.sh
bash scripts/test.sh
```

Module-specific checks may be added by a milestone, but they do not replace the root verification surface.
```

- [ ] **Step 4: 在 ROADMAP 中把 root scripts 写成硬性默认门面**

```md
### Default DoD Surface

Unless a milestone explicitly adds extra checks, the default Definition of Done verification surface is:

```bash
bash scripts/check.sh
bash scripts/test.sh
```
```

- [ ] **Step 5: 在 `scripts/check.sh` 和 `scripts/test.sh` 头部补充稳定语义注释**

```bash
# Mainline default static/build verification surface.
# Milestones may add extra checks, but should not replace this entrypoint.
```

```bash
# Mainline default automated test verification surface.
# Milestones may add extra checks, but should not replace this entrypoint.
```

- [ ] **Step 6: 运行门面文本检查**

Run: `python - <<'PY'
from pathlib import Path
readme = Path('/home/shirosora/code_storage/Oblivious/README.md').read_text()
roadmap = Path('/home/shirosora/code_storage/Oblivious/ROADMAP.md').read_text()
assert '## Milestone DoD Commands' in readme
assert 'bash scripts/check.sh' in readme
assert 'bash scripts/test.sh' in readme
assert 'Default DoD Surface' in roadmap
print('ok')
PY`
Expected: `ok`

- [ ] **Step 7: 提交本任务**

```bash
git add README.md ROADMAP.md scripts/check.sh scripts/test.sh
git commit -m "docs: standardize milestone verification surface"
```

### Task 4: 拆分后端路由装配，清理 `router.go` 单点膨胀

**Files:**
- Modify: `src/server/internal/http/router.go`
- Create: `src/server/internal/http/routes_auth.go`
- Create: `src/server/internal/http/routes_preferences.go`
- Create: `src/server/internal/http/routes_chat.go`
- Create: `src/server/internal/http/routes_knowledge.go`
- Create: `src/server/internal/http/routes_tasks.go`
- Create: `src/server/internal/http/routes_console.go`
- Test: `src/server/internal/http/chat_handler_test.go`
- Test: `src/server/internal/http/knowledge_handler_test.go`
- Test: `src/server/internal/http/task_handler_test.go`

- [ ] **Step 1: 写一个失败检查，约束 `router.go` 只保留组装，不再内联全部注册逻辑**

```bash
python - <<'PY'
from pathlib import Path
text = Path('/home/shirosora/code_storage/Oblivious/src/server/internal/http/router.go').read_text()
assert 'registerAuthRoutes(' in text, 'router.go missing registerAuthRoutes extraction'
assert 'registerKnowledgeRoutes(' in text, 'router.go missing registerKnowledgeRoutes extraction'
assert text.count('mux.Handle(') < 8, 'router.go still contains too many direct route registrations'
PY
```

Expected: FAIL because extraction does not exist yet

- [ ] **Step 2: 运行检查确认失败**

Run: `python - <<'PY' ... PY`
Expected: FAIL with missing register function assertion

- [ ] **Step 3: 在 `router.go` 中保留装配骨架，改为调用模块级注册函数**

```go
func NewRouter(cfg config.Config, database *sql.DB) stdhttp.Handler {
	mux := stdhttp.NewServeMux()
	mux.HandleFunc("/healthz", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		writeJSON(w, stdhttp.StatusOK, map[string]string{"status": "ok"})
	})

	authService := auth.NewService(auth.NewSQLStore(database))
	authMiddleware := newAuthMiddleware(cfg, authService)
	preferencesService := userprefs.NewService(userprefs.NewSQLStore(database))
	authHandler := newAuthHandler(authService, authMiddleware, preferencesService)
	replyGenerator := chat.NewHTTPReplyGenerator(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.ModelDefaultName, time.Duration(cfg.LLMTimeoutMS)*time.Millisecond)
	chatHandler := newChatHandler(chat.NewService(chat.NewSQLStore(database), replyGenerator, cfg.ModelDefaultName, usage.NewSQLRecorder(database)))
	consoleHandler := newConsoleHandler(console.NewService(console.NewSQLStore(database)), preferencesService)
	knowledgeHandler := newKnowledgeHandler(knowledge.NewService(knowledge.NewSQLStore(database)))
	preferencesHandler := newPreferencesHandler(preferencesService)
	taskHandler := newTaskHandler(task.NewService(task.NewSQLStore(database)))

	registerAuthRoutes(mux, authMiddleware, authHandler)
	registerPreferenceRoutes(mux, authMiddleware, preferencesHandler)
	registerChatRoutes(mux, authMiddleware, chatHandler)
	registerKnowledgeRoutes(mux, authMiddleware, knowledgeHandler)
	registerTaskRoutes(mux, authMiddleware, taskHandler)
	registerConsoleRoutes(mux, authMiddleware, consoleHandler)

	return applyMiddleware(mux, withRecover, withRequestID, withLogging, withCORS(cfg.CORSAllowedOrigins))
}
```

- [ ] **Step 4: 在 `routes_auth.go` 中提取 auth 注册**

```go
package http

import stdhttp "net/http"

func registerAuthRoutes(mux *stdhttp.ServeMux, authMiddleware authMiddleware, authHandler authHandler) {
	mux.HandleFunc("/api/v1/auth/login", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		authHandler.login(w, r)
	})

	mux.HandleFunc("/api/v1/auth/register", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		authHandler.register(w, r)
	})

	mux.Handle("/api/v1/auth/me", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		authHandler.me(w, r)
	})))

	mux.Handle("/api/v1/auth/logout", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		authHandler.logout(w, r)
	})))
}
```

- [ ] **Step 5: 在各 `routes_*.go` 中按原逻辑逐块迁移注册代码**

```go
func registerKnowledgeRoutes(mux *stdhttp.ServeMux, authMiddleware authMiddleware, knowledgeHandler knowledgeHandler) {
	mux.Handle("/api/v1/app/knowledge-bases", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			knowledgeHandler.listKnowledgeBases(w, r)
		case stdhttp.MethodPost:
			knowledgeHandler.createKnowledgeBase(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))

	// 保持原 `knowledge-bases/` 路径分发逻辑不变，完整迁移原代码块。
}
```

```go
func registerTaskRoutes(mux *stdhttp.ServeMux, authMiddleware authMiddleware, taskHandler taskHandler) {
	// 完整迁移原 `/api/v1/app/tasks` 与 `/api/v1/app/tasks/` 注册逻辑。
}
```

```go
func registerConsoleRoutes(mux *stdhttp.ServeMux, authMiddleware authMiddleware, consoleHandler consoleHandler) {
	// 完整迁移原 `/api/v1/console/*` 注册逻辑。
}
```

- [ ] **Step 6: 运行后端 HTTP 定向测试**

Run: `go test ./src/server/internal/http -run 'Test(ChatHandler|KnowledgeHandler|TaskHandler)'`
Expected: PASS

- [ ] **Step 7: 运行提取检查确认通过**

Run: `python - <<'PY'
from pathlib import Path
text = Path('/home/shirosora/code_storage/Oblivious/src/server/internal/http/router.go').read_text()
assert 'registerAuthRoutes(' in text
assert 'registerKnowledgeRoutes(' in text
assert text.count('mux.Handle(') < 8
print('ok')
PY`
Expected: `ok`

- [ ] **Step 8: 提交本任务**

```bash
git add src/server/internal/http/router.go src/server/internal/http/routes_auth.go src/server/internal/http/routes_preferences.go src/server/internal/http/routes_chat.go src/server/internal/http/routes_knowledge.go src/server/internal/http/routes_tasks.go src/server/internal/http/routes_console.go
git commit -m "refactor: split mainline HTTP route registration"
```

### Task 5: 拆分 Knowledge 页面，停止页面吞逻辑

**Files:**
- Modify: `src/web/src/routes/workspace/KnowledgePage.tsx`
- Create: `src/web/src/routes/workspace/knowledge/useKnowledgePageState.ts`
- Create: `src/web/src/routes/workspace/knowledge/KnowledgePageView.tsx`
- Test: `src/web/src/routes/workspace/KnowledgePage.test.tsx`

- [ ] **Step 1: 写失败检查，要求 `KnowledgePage.tsx` 只做容器拼装**

```bash
python - <<'PY'
from pathlib import Path
text = Path('/home/shirosora/code_storage/Oblivious/src/web/src/routes/workspace/KnowledgePage.tsx').read_text()
assert 'useKnowledgePageState' in text, 'KnowledgePage missing extracted state hook'
assert 'KnowledgePageView' in text, 'KnowledgePage missing extracted view'
assert 'handleRetrieveKnowledge' not in text, 'KnowledgePage still owns heavy page actions'
PY
```

Expected: FAIL because extraction does not exist yet

- [ ] **Step 2: 运行检查确认失败**

Run: `python - <<'PY' ... PY`
Expected: FAIL

- [ ] **Step 3: 抽取 `useKnowledgePageState.ts`，迁移页面状态与动作**

```ts
export function useKnowledgePageState() {
  const navigate = useNavigate();
  const { knowledgeBaseId } = useParams<{ knowledgeBaseId?: string }>();
  const { authState } = useAppContext();
  const returnTo = new URLSearchParams(window.location.search).get('returnTo');
  const knowledgeApi = useMemo(() => createKnowledgeApi(createHttpClient()), []);

  const [error, setError] = useState<string | null>(null);
  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBaseSummary[]>([]);
  const [knowledgeDocuments, setKnowledgeDocuments] = useState<KnowledgeDocumentSummary[]>([]);
  const [retrievalResults, setRetrievalResults] = useState<KnowledgeRetrievalResult[]>([]);

  // 将现有 load / create / update / delete / retrieve 动作完整迁移进 hook。

  return {
    authState,
    error,
    knowledgeBaseId,
    knowledgeBases,
    knowledgeDocuments,
    retrievalResults,
    returnTo,
    // 返回现有页面需要的全部字段和 handler
  };
}
```

- [ ] **Step 4: 新建 `KnowledgePageView.tsx`，承接渲染结构**

```tsx
export function KnowledgePageView(props: {
  authState: ReturnType<typeof useKnowledgePageState>['authState'];
  error: string | null;
  // 继续列出 KnowledgePage 实际用到的 props
}) {
  return (
    <>
      {/* 将现有 JSX 原样迁移到展示组件，避免行为变化 */}
    </>
  );
}
```

- [ ] **Step 5: 将 `KnowledgePage.tsx` 收敛为容器**

```tsx
import { KnowledgePageView } from './knowledge/KnowledgePageView';
import { useKnowledgePageState } from './knowledge/useKnowledgePageState';

export function KnowledgePage() {
  const state = useKnowledgePageState();
  return <KnowledgePageView {...state} />;
}
```

- [ ] **Step 6: 运行 Knowledge 页面测试**

Run: `pnpm --dir /home/shirosora/code_storage/Oblivious/src/web test -- --run src/routes/workspace/KnowledgePage.test.tsx`
Expected: PASS

- [ ] **Step 7: 运行容器收敛检查**

Run: `python - <<'PY'
from pathlib import Path
text = Path('/home/shirosora/code_storage/Oblivious/src/web/src/routes/workspace/KnowledgePage.tsx').read_text()
assert 'useKnowledgePageState' in text
assert 'KnowledgePageView' in text
assert 'handleRetrieveKnowledge' not in text
print('ok')
PY`
Expected: `ok`

- [ ] **Step 8: 提交本任务**

```bash
git add src/web/src/routes/workspace/KnowledgePage.tsx src/web/src/routes/workspace/knowledge/useKnowledgePageState.ts src/web/src/routes/workspace/knowledge/KnowledgePageView.tsx src/web/src/routes/workspace/KnowledgePage.test.tsx
git commit -m "refactor: split knowledge page container and view"
```

### Task 6: 拆分 Solo 页面，停止页面吞动作与导出逻辑

**Files:**
- Modify: `src/web/src/routes/workspace/SoloPage.tsx`
- Create: `src/web/src/routes/workspace/solo/useSoloPageState.ts`
- Create: `src/web/src/routes/workspace/solo/SoloPageView.tsx`
- Test: `src/web/src/routes/workspace/SoloPage.test.tsx`

- [ ] **Step 1: 写失败检查，要求 `SoloPage.tsx` 不再直接包含 `downloadTaskResult` 与重型 handler**

```bash
python - <<'PY'
from pathlib import Path
text = Path('/home/shirosora/code_storage/Oblivious/src/web/src/routes/workspace/SoloPage.tsx').read_text()
assert 'useSoloPageState' in text, 'SoloPage missing extracted state hook'
assert 'SoloPageView' in text, 'SoloPage missing extracted view'
assert 'downloadTaskResult' not in text, 'SoloPage still owns export helper'
assert 'handleStartSoloRun' not in text, 'SoloPage still owns heavy actions'
PY
```

Expected: FAIL

- [ ] **Step 2: 运行检查确认失败**

Run: `python - <<'PY' ... PY`
Expected: FAIL

- [ ] **Step 3: 抽取 `useSoloPageState.ts`，迁移现有状态与任务动作**

```ts
export function useSoloPageState() {
  const { authState } = useAppContext();
  const navigate = useNavigate();
  const isTaskCreationView = window.location.pathname === '/solo/new';
  const returnTo = new URLSearchParams(window.location.search).get('returnTo');
  const httpClient = useMemo(() => createHttpClient(), []);
  const chatApi = useMemo(() => createChatApi(httpClient), [httpClient]);
  const knowledgeApi = useMemo(() => createKnowledgeApi(httpClient), [httpClient]);
  const tasksApi = useMemo(() => createTasksApi(httpClient), [httpClient]);

  const [goal, setGoal] = useState('');
  const [recentTasks, setRecentTasks] = useState<TaskSummary[]>([]);
  const [startedTask, setStartedTask] = useState<TaskDetail | null>(null);

  // 将当前页面的 load / create / pause / approve / resume / export 逻辑完整迁移到 hook。

  return {
    authState,
    goal,
    recentTasks,
    startedTask,
    returnTo,
    // 返回现有页面需要的全部字段和 handler
  };
}
```

- [ ] **Step 4: 在 hook 内迁移导出逻辑而不是保留在页面顶层**

```ts
function downloadTaskResult(task: TaskDetail, knowledgeBaseNames: string[]) {
  const toolRules = normalizeToolRules(task.toolAllowList, task.toolDenyList);
  const fileName = `${task.title || task.id}`.trim().replace(/\s+/g, '-').toLowerCase() || task.id;
  const content = [
    `# ${task.title}`,
    '',
    `- Goal: ${task.goal}`,
    `- Status: ${task.status}`,
    `- Execution mode: ${task.executionMode}`,
    `- Authorization scope: ${task.authorizationScope}`,
  ].join('\n');

  const blob = new Blob([content], { type: 'text/markdown;charset=utf-8' });
  const downloadURL = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = downloadURL;
  link.download = `${fileName || 'solo-result'}.md`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(downloadURL);
}
```

- [ ] **Step 5: 新建 `SoloPageView.tsx` 承接展示层**

```tsx
export function SoloPageView(props: ReturnType<typeof useSoloPageState>) {
  return (
    <>
      {/* 将现有 SoloPage JSX 原样迁移到展示组件，避免行为变化 */}
    </>
  );
}
```

- [ ] **Step 6: 将 `SoloPage.tsx` 收敛为容器**

```tsx
import { SoloPageView } from './solo/SoloPageView';
import { useSoloPageState } from './solo/useSoloPageState';

export function SoloPage() {
  const state = useSoloPageState();
  return <SoloPageView {...state} />;
}
```

- [ ] **Step 7: 运行 Solo 页面测试**

Run: `pnpm --dir /home/shirosora/code_storage/Oblivious/src/web test -- --run src/routes/workspace/SoloPage.test.tsx`
Expected: PASS

- [ ] **Step 8: 运行容器收敛检查**

Run: `python - <<'PY'
from pathlib import Path
text = Path('/home/shirosora/code_storage/Oblivious/src/web/src/routes/workspace/SoloPage.tsx').read_text()
assert 'useSoloPageState' in text
assert 'SoloPageView' in text
assert 'downloadTaskResult' not in text
assert 'handleStartSoloRun' not in text
print('ok')
PY`
Expected: `ok`

- [ ] **Step 9: 提交本任务**

```bash
git add src/web/src/routes/workspace/SoloPage.tsx src/web/src/routes/workspace/solo/useSoloPageState.ts src/web/src/routes/workspace/solo/SoloPageView.tsx src/web/src/routes/workspace/SoloPage.test.tsx
git commit -m "refactor: split solo page container and view"
```

### Task 7: 为 Knowledge 与 SOLO 的后续深化建立路线决策框架

**Files:**
- Modify: `ROADMAP.md`
- Modify: `CURRENT_STATUS.md`
- Create: `docs/architecture/knowledge-evolution-decision.md`
- Create: `docs/architecture/solo-runtime-decision.md`
- Test: `docs/architecture/knowledge-evolution-decision.md`
- Test: `docs/architecture/solo-runtime-decision.md`

- [ ] **Step 1: 写失败检查，要求新增两个决策文档**

```bash
python - <<'PY'
from pathlib import Path
root = Path('/home/shirosora/code_storage/Oblivious/docs/architecture')
assert (root / 'knowledge-evolution-decision.md').exists(), 'missing knowledge decision doc'
assert (root / 'solo-runtime-decision.md').exists(), 'missing solo decision doc'
PY
```

Expected: FAIL with missing decision doc

- [ ] **Step 2: 运行检查确认失败**

Run: `python - <<'PY' ... PY`
Expected: FAIL

- [ ] **Step 3: 创建 Knowledge 路线决策文档**

```md
# Knowledge Evolution Decision

## Current Truth
- 当前能力是文本检索 Beta，不是完整 RAG。
- 当前主线已具备 CRUD + retrieval 闭环。

## Option A
- 继续优化文本检索 Beta

## Option B
- 升级为 embedding / indexing / async ingestion 的 RAG 路线

## Decision Fields
- Chosen Option:
- Decision Owner:
- Effective Baseline Commit:
- Impacted Files:
- New DoD:
```

- [ ] **Step 4: 创建 SOLO 路线决策文档**

```md
# SOLO Runtime Decision

## Current Truth
- 当前能力是受限 runtime MVP，不是真实 agent orchestration。
- 当前主线已具备任务创建、审批、预算、导出闭环。

## Option A
- 继续作为任务编排 MVP

## Option B
- 升级为真实执行器与事件流模型

## Decision Fields
- Chosen Option:
- Decision Owner:
- Effective Baseline Commit:
- Impacted Files:
- New DoD:
```

- [ ] **Step 5: 在 `ROADMAP.md` 的 Phase 3 中直接引用决策文档路径**

```md
- `docs/architecture/knowledge-evolution-decision.md`
- `docs/architecture/solo-runtime-decision.md`
```

- [ ] **Step 6: 在 `CURRENT_STATUS.md` 总结章节加入“后续能力深化必须先完成路线选择”说明**

```md
5. **任何 Knowledge / SOLO 深化开发都必须先完成对应决策文档的路线选择。**
```

- [ ] **Step 7: 运行决策文档存在性检查**

Run: `python - <<'PY'
from pathlib import Path
root = Path('/home/shirosora/code_storage/Oblivious/docs/architecture')
for name in ['knowledge-evolution-decision.md', 'solo-runtime-decision.md']:
    text = (root / name).read_text()
    assert '## Current Truth' in text
    assert '## Option A' in text
    assert '## Option B' in text
print('ok')
PY`
Expected: `ok`

- [ ] **Step 8: 提交本任务**

```bash
git add ROADMAP.md CURRENT_STATUS.md docs/architecture/knowledge-evolution-decision.md docs/architecture/solo-runtime-decision.md
git commit -m "docs: add mainline capability decision records"
```

## Self-Review

### Spec coverage
- 锁定版本基线：Task 1 完成。
- 根据 roadmap 创建任务：本计划覆盖 Phase 0、Phase 1、Phase 2 的立即执行项，并为 Phase 3 建立决策入口。
- 自动推进模式：该计划本身即为自动推进的任务拆解基础。
- 直到路线图全部完成：当前 plan 覆盖了“先做什么”与“后续如何继续”的执行主轴，但 Phase 3/4 的具体实现取决于 Task 7 产生的路线选择；这是必要前提，不是遗漏。

### Placeholder scan
- 已避免 `TODO`、`TBD`、`implement later`。
- 某些“完整迁移原逻辑”说明是为了避免在计划里重写数百行既有代码，但已明确精确目标文件与保序迁移范围。

### Type consistency
- `registerAuthRoutes` / `registerKnowledgeRoutes` / `registerTaskRoutes` 命名在任务内保持一致。
- `useKnowledgePageState` / `KnowledgePageView` 与 `useSoloPageState` / `SoloPageView` 命名一致。
- Knowledge 与 SOLO 决策文档文件名已在 Task 7 前后一致。

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-08-mainline-engineering-governance-implementation.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?