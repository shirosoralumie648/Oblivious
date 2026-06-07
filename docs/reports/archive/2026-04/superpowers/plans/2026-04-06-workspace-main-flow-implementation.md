# Workspace Main Flow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 打通 `onboarding -> chat -> knowledge -> solo -> settings` 的工作区主链路，并把 onboarding 从“强制阻断”改为“可跳过但仍可引导”。

**Architecture:** 本轮只修改前端主线 `src/web`，不新增后端 API。实现顺序按“鉴权与落点规则 -> onboarding/settings -> chat 主入口 -> knowledge/solo 回跳 -> 文档与回归”推进，把跨页跳转逻辑收敛到小型 helper 与 query 参数，而不是分散在多个页面里。

**Tech Stack:** React 18, React Router 6, Vite 5, Vitest, Testing Library, existing `AppContext`, `AuthApi`, `ChatApi`, `KnowledgeApi`, `TasksApi`

---

## File Structure

- Create: `src/web/src/features/auth/workspaceLanding.ts`
  - 统一已认证用户的默认落点决策
- Create: `src/web/src/features/auth/workspaceLanding.test.ts`
  - 覆盖 onboarding 未完成、chat 默认模式、solo 默认模式
- Create: `src/web/src/features/auth/ProtectedRoute.test.tsx`
  - 锁定“鉴权继续有效，但不再强制 onboarding”的行为
- Modify: `src/web/src/features/auth/ProtectedRoute.tsx`
  - 放松 onboarding gating，只保留登录门禁
- Modify: `src/web/src/features/layouts/WorkspaceLayout.tsx`
  - 调整导航顺序，保证 Chat 第一优先级
- Modify: `src/web/src/features/layouts/WorkspaceLayout.test.tsx`
  - 锁定新的导航顺序
- Modify: `src/web/src/app/router.test.tsx`
  - 保持现有工作区/console/marketing smoke tests 绿灯
- Modify: `src/web/src/routes/workspace/OnboardingPage.tsx`
  - 实现继续到 Chat / SOLO 和 Skip 行为
- Modify: `src/web/src/routes/workspace/OnboardingPage.test.tsx`
  - 覆盖 continue/skip 行为
- Modify: `src/web/src/routes/workspace/SettingsPage.tsx`
  - 增加返回 Chat CTA 与 onboarding 状态文案
- Modify: `src/web/src/routes/workspace/SettingsPage.test.tsx`
  - 覆盖 return CTA 与保存后仍留在页内
- Modify: `src/web/src/routes/workspace/ChatPage.tsx`
  - 实现 `/chat` 空状态、首会话创建、消息收发、knowledge CTA、setup 提示
- Modify: `src/web/src/routes/workspace/ChatPage.behavior.test.tsx`
  - 扩展主路径行为覆盖
- Modify: `src/web/src/routes/workspace/ChatPage.test.tsx`
  - 维持路由烟测
- Modify: `src/web/src/routes/workspace/KnowledgePage.tsx`
  - 增加 `returnTo` 回跳入口
- Modify: `src/web/src/routes/workspace/KnowledgePage.test.tsx`
  - 覆盖来自 Chat 的返回入口
- Modify: `src/web/src/routes/workspace/SoloPage.tsx`
  - 识别 `taskId` 与 `returnTo`，显示回到 Chat 的入口
- Modify: `src/web/src/routes/workspace/SoloPage.test.tsx`
  - 覆盖带 query 的 task detail 与 back-to-chat
- Modify: `docs/architecture/current-system-contracts.md`
  - 更新工作区主流转与 onboarding 行为描述

## Task 1: Relax Onboarding Gating And Centralize Landing Decisions

**Files:**
- Create: `src/web/src/features/auth/workspaceLanding.ts`
- Create: `src/web/src/features/auth/workspaceLanding.test.ts`
- Create: `src/web/src/features/auth/ProtectedRoute.test.tsx`
- Modify: `src/web/src/features/auth/ProtectedRoute.tsx`

- [ ] **Step 1: Write the failing landing helper tests**

```ts
import { describe, expect, it } from 'vitest';

import { resolveWorkspaceLandingPath } from './workspaceLanding';

describe('resolveWorkspaceLandingPath', () => {
  it('routes incomplete onboarding users to onboarding', () => {
    expect(
      resolveWorkspaceLandingPath({
        defaultMode: 'chat',
        modelStrategy: 'balanced',
        networkEnabledHint: false,
        onboardingCompleted: false
      })
    ).toBe('/onboarding');
  });

  it('routes completed chat users to /chat', () => {
    expect(
      resolveWorkspaceLandingPath({
        defaultMode: 'chat',
        modelStrategy: 'balanced',
        networkEnabledHint: false,
        onboardingCompleted: true
      })
    ).toBe('/chat');
  });

  it('routes completed solo users to /solo/new', () => {
    expect(
      resolveWorkspaceLandingPath({
        defaultMode: 'solo',
        modelStrategy: 'quality',
        networkEnabledHint: true,
        onboardingCompleted: true
      })
    ).toBe('/solo/new');
  });
});
```

- [ ] **Step 2: Write the failing ProtectedRoute behavior tests**

```tsx
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';

const appContext = vi.hoisted(() => ({
  authState: {
    preferences: {
      defaultMode: 'chat' as const,
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: false
    },
    status: 'authenticated' as const,
    user: { email: 'user@example.com', id: 'u1' }
  }
}));

vi.mock('../../app/providers', () => ({
  useAppContext: () => appContext
}));

import { ProtectedRoute } from './ProtectedRoute';

describe('ProtectedRoute', () => {
  it('allows authenticated users with incomplete onboarding to remain on /chat', () => {
    render(
      <MemoryRouter initialEntries={['/chat']}>
        <Routes>
          <Route element={<ProtectedRoute />}>
            <Route element={<div>chat shell</div>} path="/chat" />
            <Route element={<div>onboarding shell</div>} path="/onboarding" />
          </Route>
        </Routes>
      </MemoryRouter>
    );

    expect(screen.getByText('chat shell')).toBeInTheDocument();
  });

  it('redirects completed onboarding users away from /onboarding using the default mode', () => {
    appContext.authState.preferences = {
      defaultMode: 'solo',
      modelStrategy: 'quality',
      networkEnabledHint: true,
      onboardingCompleted: true
    };

    render(
      <MemoryRouter initialEntries={['/onboarding']}>
        <Routes>
          <Route element={<ProtectedRoute />}>
            <Route element={<div>solo shell</div>} path="/solo/new" />
            <Route element={<div>onboarding shell</div>} path="/onboarding" />
          </Route>
        </Routes>
      </MemoryRouter>
    );

    expect(screen.getByText('solo shell')).toBeInTheDocument();
  });
});
```

- [ ] **Step 3: Run the auth flow tests to verify they fail**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/features/auth/workspaceLanding.test.ts \
  src/features/auth/ProtectedRoute.test.tsx
```

Expected: FAIL because `workspaceLanding.ts` does not exist and `ProtectedRoute` still forces `/onboarding`.

- [ ] **Step 4: Implement the landing helper**

```ts
import type { UserPreferences } from '../../types/api';

export function resolveWorkspaceLandingPath(preferences: UserPreferences | null | undefined) {
  if (!preferences || !preferences.onboardingCompleted) {
    return '/onboarding';
  }

  return preferences.defaultMode === 'solo' ? '/solo/new' : '/chat';
}
```

- [ ] **Step 5: Update ProtectedRoute to stop forcing onboarding**

```tsx
import { Navigate, Outlet, useLocation } from 'react-router-dom';

import { useAppContext } from '../../app/providers';
import { resolveWorkspaceLandingPath } from './workspaceLanding';

export function ProtectedRoute() {
  const location = useLocation();
  const { authState } = useAppContext();

  if (authState.status === 'idle') {
    return <Outlet />;
  }

  if (authState.status === 'loading') {
    return <main>Loading session…</main>;
  }

  if (authState.status === 'unauthenticated') {
    const redirectPath = `${location.pathname}${location.search}${location.hash}`;
    return <Navigate replace state={{ from: redirectPath }} to="/login" />;
  }

  if (location.pathname === '/onboarding' && authState.preferences?.onboardingCompleted) {
    return <Navigate replace to={resolveWorkspaceLandingPath(authState.preferences)} />;
  }

  return <Outlet />;
}
```

- [ ] **Step 6: Re-run the auth flow tests to verify they pass**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/features/auth/workspaceLanding.test.ts \
  src/features/auth/ProtectedRoute.test.tsx
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add \
  src/web/src/features/auth/workspaceLanding.ts \
  src/web/src/features/auth/workspaceLanding.test.ts \
  src/web/src/features/auth/ProtectedRoute.tsx \
  src/web/src/features/auth/ProtectedRoute.test.tsx
git commit -m "feat(web): relax onboarding gate for workspace flow"
```

## Task 2: Finish Onboarding And Settings Handoff UX

**Files:**
- Modify: `src/web/src/routes/workspace/OnboardingPage.tsx`
- Modify: `src/web/src/routes/workspace/OnboardingPage.test.tsx`
- Modify: `src/web/src/routes/workspace/SettingsPage.tsx`
- Modify: `src/web/src/routes/workspace/SettingsPage.test.tsx`

- [ ] **Step 1: Write the failing onboarding navigation tests**

```tsx
it('saves preferences and routes to /chat after choosing chat', async () => {
  render(
    <MemoryRouter>
      <OnboardingPage />
    </MemoryRouter>
  );

  fireEvent.click(screen.getByRole('button', { name: 'Start with Chat' }));
  fireEvent.click(screen.getByRole('button', { name: 'Continue to workspace' }));

  await waitFor(() => {
    expect(updatePreferences).toHaveBeenCalledWith({
      defaultMode: 'chat',
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: true
    });
  });
  expect(navigate).toHaveBeenCalledWith('/chat');
});

it('saves preferences and routes to /solo/new after choosing solo', async () => {
  render(
    <MemoryRouter>
      <OnboardingPage />
    </MemoryRouter>
  );

  fireEvent.click(screen.getByRole('button', { name: 'Start with SOLO' }));
  fireEvent.click(screen.getByRole('button', { name: 'Continue to workspace' }));

  await waitFor(() => {
    expect(updatePreferences).toHaveBeenCalledWith({
      defaultMode: 'solo',
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: true
    });
  });
  expect(navigate).toHaveBeenCalledWith('/solo/new');
});

it('routes to /chat without saving when the user skips onboarding', () => {
  render(
    <MemoryRouter>
      <OnboardingPage />
    </MemoryRouter>
  );

  fireEvent.click(screen.getByRole('button', { name: 'Skip for now' }));

  expect(updatePreferences).not.toHaveBeenCalled();
  expect(navigate).toHaveBeenCalledWith('/chat');
});
```

- [ ] **Step 2: Write the failing settings return CTA test**

```tsx
it('offers a return path back to chat', () => {
  render(<SettingsPage />);

  expect(screen.getByRole('button', { name: 'Return to chat' })).toBeInTheDocument();
});
```

- [ ] **Step 3: Run the onboarding/settings tests to verify they fail**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/routes/workspace/OnboardingPage.test.tsx \
  src/routes/workspace/SettingsPage.test.tsx
```

Expected: FAIL because the pages do not navigate after continue/skip and Settings has no return CTA.

- [ ] **Step 4: Implement onboarding continue and skip behavior**

```tsx
import { useNavigate } from 'react-router-dom';

export function OnboardingPage() {
  const navigate = useNavigate();
  const { updatePreferences } = useAppContext();

  const handleContinue = async () => {
    if (defaultMode === null) {
      return;
    }

    await updatePreferences({
      defaultMode,
      modelStrategy,
      networkEnabledHint: false,
      onboardingCompleted: true
    });

    navigate(defaultMode === 'solo' ? '/solo/new' : '/chat');
  };

  const handleSkip = () => {
    navigate('/chat');
  };
}
```

- [ ] **Step 5: Implement the settings return CTA**

```tsx
import { useNavigate } from 'react-router-dom';

export function SettingsPage() {
  const navigate = useNavigate();

  return (
    <section>
      <h1>Settings</h1>
      <p>{preferences.onboardingCompleted ? 'Onboarding complete' : 'Onboarding pending'}</p>
      <button onClick={() => void handleSave()} type="button">
        Save preferences
      </button>
      <button onClick={() => navigate('/chat')} type="button">
        Return to chat
      </button>
      {savedMessage ? <p>{savedMessage}</p> : null}
    </section>
  );
}
```

- [ ] **Step 6: Re-run the onboarding/settings tests to verify they pass**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/routes/workspace/OnboardingPage.test.tsx \
  src/routes/workspace/SettingsPage.test.tsx
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add \
  src/web/src/routes/workspace/OnboardingPage.tsx \
  src/web/src/routes/workspace/OnboardingPage.test.tsx \
  src/web/src/routes/workspace/SettingsPage.tsx \
  src/web/src/routes/workspace/SettingsPage.test.tsx
git commit -m "feat(web): add onboarding and settings handoff flow"
```

## Task 3: Turn Chat Into The Workspace Hub

**Files:**
- Modify: `src/web/src/routes/workspace/ChatPage.tsx`
- Modify: `src/web/src/routes/workspace/ChatPage.behavior.test.tsx`
- Modify: `src/web/src/routes/workspace/ChatPage.test.tsx`

- [ ] **Step 1: Write the failing chat hub tests**

```tsx
it('shows an empty state on /chat and creates the first conversation', async () => {
  routeState.conversationId = undefined;
  listConversations.mockResolvedValue([]);
  listKnowledgeBases.mockResolvedValue([]);
  listModels.mockResolvedValue([{ id: 'balanced-chat', label: 'balanced-chat' }]);
  createConversation.mockResolvedValue({ id: 'conversation_1', title: 'New conversation' });

  render(<ChatPage />);

  expect(await screen.findByText('No conversations yet. Start a workspace thread to begin.')).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Create first conversation' }));

  await waitFor(() => {
    expect(createConversation).toHaveBeenCalledWith({ title: 'New conversation' });
  });
  expect(navigate).toHaveBeenCalledWith('/chat/conversation_1');
});

it('sends a message inside the active conversation', async () => {
  routeState.conversationId = 'conversation_1';
  listConversations.mockResolvedValue([{ id: 'conversation_1', title: 'Research thread' }]);
  listKnowledgeBases.mockResolvedValue([]);
  listMessages.mockResolvedValue([{ id: 'm1', role: 'assistant', content: 'Ready when you are.' }]);
  listModels.mockResolvedValue([{ id: 'balanced-chat', label: 'balanced-chat' }]);
  getConversationConfig.mockResolvedValue({
    conversationId: 'conversation_1',
    knowledgeBaseIds: [],
    maxOutputTokens: 1024,
    modelId: 'balanced-chat',
    systemPromptOverride: '',
    temperature: 1,
    toolsEnabled: false
  });
  sendMessage.mockResolvedValue([
    { id: 'm1', role: 'assistant', content: 'Ready when you are.' },
    { id: 'm2', role: 'user', content: 'Draft a rollout summary.' }
  ]);

  render(<ChatPage />);

  await screen.findByText('Ready when you are.');
  fireEvent.change(screen.getByLabelText('Message draft'), { target: { value: 'Draft a rollout summary.' } });
  fireEvent.click(screen.getByRole('button', { name: 'Send message' }));

  await waitFor(() => {
    expect(sendMessage).toHaveBeenCalledWith('conversation_1', { content: 'Draft a rollout summary.' });
  });
});

it('shows a create-knowledge-base CTA when the active conversation has no knowledge bases available', async () => {
  routeState.conversationId = 'conversation_1';
  listConversations.mockResolvedValue([{ id: 'conversation_1', title: 'Research thread' }]);
  listKnowledgeBases.mockResolvedValue([]);
  listMessages.mockResolvedValue([]);
  listModels.mockResolvedValue([{ id: 'balanced-chat', label: 'balanced-chat' }]);
  getConversationConfig.mockResolvedValue({
    conversationId: 'conversation_1',
    knowledgeBaseIds: [],
    maxOutputTokens: 1024,
    modelId: 'balanced-chat',
    systemPromptOverride: '',
    temperature: 1,
    toolsEnabled: false
  });

  render(<ChatPage />);

  expect(await screen.findByRole('button', { name: 'Create knowledge base' })).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Create knowledge base' }));

  expect(navigate).toHaveBeenCalledWith('/knowledge?returnTo=%2Fchat%2Fconversation_1');
});

it('shows a setup reminder for users who skipped onboarding', async () => {
  appContext.authState.preferences = {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: false,
    onboardingCompleted: false
  };
  routeState.conversationId = 'conversation_1';
  listConversations.mockResolvedValue([{ id: 'conversation_1', title: 'Research thread' }]);
  listKnowledgeBases.mockResolvedValue([]);
  listMessages.mockResolvedValue([]);
  listModels.mockResolvedValue([{ id: 'balanced-chat', label: 'balanced-chat' }]);
  getConversationConfig.mockResolvedValue({
    conversationId: 'conversation_1',
    knowledgeBaseIds: [],
    maxOutputTokens: 1024,
    modelId: 'balanced-chat',
    systemPromptOverride: '',
    temperature: 1,
    toolsEnabled: false
  });

  render(<ChatPage />);

  expect(await screen.findByText('Finish setup to lock in your default workspace preferences.')).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Complete setup' }));

  expect(navigate).toHaveBeenCalledWith('/onboarding');
});
```

- [ ] **Step 2: Run the chat-focused tests to verify they fail**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/routes/workspace/ChatPage.behavior.test.tsx \
  src/routes/workspace/ChatPage.test.tsx
```

Expected: FAIL because `ChatPage` does not yet render empty state, message composer, or no-knowledge CTA.

- [ ] **Step 3: Implement `/chat` empty state and first-conversation creation**

```tsx
const [conversations, setConversations] = useState<ConversationSummary[]>([]);

const handleCreateConversation = async () => {
  const conversation = await chatApi.createConversation({ title: 'New conversation' });
  setConversations((current) => [conversation, ...current]);
  navigate(`/chat/${conversation.id}`);
};

if (!conversationId) {
  return (
    <section>
      <h1>Chat workspace</h1>
      {conversations.length === 0 ? (
        <>
          <p>No conversations yet. Start a workspace thread to begin.</p>
          <button onClick={() => void handleCreateConversation()} type="button">
            Create first conversation
          </button>
        </>
      ) : (
        <>
          <h2>Recent conversations</h2>
          {conversations.map((conversation) => (
            <button key={conversation.id} onClick={() => navigate(`/chat/${conversation.id}`)} type="button">
              {conversation.title}
            </button>
          ))}
        </>
      )}
    </section>
  );
}
```

- [ ] **Step 4: Implement conversation message rendering, send, and setup reminder**

```tsx
const { authState } = useAppContext();
const [messageDraft, setMessageDraft] = useState('');
const [messages, setMessages] = useState<ConversationMessage[]>([]);

const handleSendMessage = async () => {
  const trimmedContent = messageDraft.trim();
  if (!conversationId || trimmedContent === '') {
    return;
  }

  const nextMessages = await chatApi.sendMessage(conversationId, { content: trimmedContent });
  setMessages(nextMessages);
  setMessageDraft('');
};

{!authState.preferences?.onboardingCompleted ? (
  <section>
    <p>Finish setup to lock in your default workspace preferences.</p>
    <button onClick={() => navigate('/onboarding')} type="button">
      Complete setup
    </button>
  </section>
) : null}

<label>
  Message draft
  <textarea onChange={(event) => setMessageDraft(event.target.value)} value={messageDraft} />
</label>
<button onClick={() => void handleSendMessage()} type="button">
  Send message
</button>
```

- [ ] **Step 5: Implement no-knowledge CTA and preserve Chat-originated SOLO handoff**

```tsx
const chatReturnPath = `/chat/${conversationId}`;

{knowledgeBases.length === 0 ? (
  <button
    onClick={() => navigate(`/knowledge?returnTo=${encodeURIComponent(chatReturnPath)}`)}
    type="button"
  >
    Create knowledge base
  </button>
) : null}

const startInSolo = async () => {
  if (handoffDraft === null) {
    return;
  }

  const createdTask = await tasksApi.createTask(createTaskPayload);
  await tasksApi.startTask(createdTask.id);
  navigate(`/solo?taskId=${createdTask.id}&returnTo=${encodeURIComponent(chatReturnPath)}`);
};
```

- [ ] **Step 6: Re-run the chat-focused tests to verify they pass**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/routes/workspace/ChatPage.behavior.test.tsx \
  src/routes/workspace/ChatPage.test.tsx
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add \
  src/web/src/routes/workspace/ChatPage.tsx \
  src/web/src/routes/workspace/ChatPage.behavior.test.tsx \
  src/web/src/routes/workspace/ChatPage.test.tsx
git commit -m "feat(web): make chat the workspace hub"
```

## Task 4: Add Knowledge And SOLO Return Flows

**Files:**
- Modify: `src/web/src/features/layouts/WorkspaceLayout.tsx`
- Modify: `src/web/src/features/layouts/WorkspaceLayout.test.tsx`
- Modify: `src/web/src/routes/workspace/KnowledgePage.tsx`
- Modify: `src/web/src/routes/workspace/KnowledgePage.test.tsx`
- Modify: `src/web/src/routes/workspace/SoloPage.tsx`
- Modify: `src/web/src/routes/workspace/SoloPage.test.tsx`

- [ ] **Step 1: Write the failing return-flow tests**

```tsx
it('renders workspace navigation in chat-first order', () => {
  render(
    <MemoryRouter>
      <WorkspaceLayout />
    </MemoryRouter>
  );

  const links = screen.getAllByRole('link').map((link) => link.textContent);
  expect(links).toEqual(['Chat', 'Knowledge', 'SOLO', 'Settings', 'Console']);
});

it('shows a back-to-chat action when returnTo is present on the knowledge route', async () => {
  window.history.replaceState({}, '', '/knowledge/kb_9?returnTo=%2Fchat%2Fconversation_1');
  routeState.knowledgeBaseId = 'kb_9';
  getKnowledgeBase.mockResolvedValue({
    documentCount: 1,
    id: 'kb_9',
    name: 'Architecture Notes',
    updatedAt: '2026-04-03T11:30:00Z'
  });
  listKnowledgeDocuments.mockResolvedValue([]);

  render(<KnowledgePage />);

  expect(await screen.findByRole('button', { name: 'Back to chat' })).toBeInTheDocument();
});

it('shows a back-to-chat action when returnTo is present on the solo route', async () => {
  window.history.replaceState({}, '', '/solo?taskId=task_1&returnTo=%2Fchat%2Fconversation_1');
  listTasks.mockResolvedValue([]);
  listKnowledgeBases.mockResolvedValue([]);
  getTask.mockResolvedValue({
    authorizationScope: 'workspace_tools',
    budgetLimit: 12,
    executionMode: 'standard',
    goal: 'Investigate blockers',
    id: 'task_1',
    knowledgeBaseIds: [],
    status: 'running',
    steps: [{ id: 'step_1', status: 'running', stepIndex: 1, title: 'Review workspace context' }],
    title: 'Investigate blockers'
  });

  render(<SoloPage />);

  expect(await screen.findByRole('button', { name: 'Back to chat' })).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the return-flow tests to verify they fail**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/routes/workspace/KnowledgePage.test.tsx \
  src/routes/workspace/SoloPage.test.tsx \
  src/features/layouts/WorkspaceLayout.test.tsx
```

Expected: FAIL because the pages do not yet read `returnTo` and the workspace nav order still puts SOLO before Knowledge.

- [ ] **Step 3: Update the workspace navigation order**

```tsx
<nav aria-label="Workspace navigation">
  <Link to="/chat">Chat</Link>
  <Link to="/knowledge">Knowledge</Link>
  <Link to="/solo">SOLO</Link>
  <Link to="/settings">Settings</Link>
  <Link to="/console">Console</Link>
</nav>
```

- [ ] **Step 4: Implement knowledge return CTA**

```tsx
const returnTo = new URLSearchParams(window.location.search).get('returnTo');

{returnTo ? (
  <button onClick={() => navigate(returnTo)} type="button">
    Back to chat
  </button>
) : (
  <button onClick={() => navigate('/knowledge')} type="button">
    Back to knowledge bases
  </button>
)}
```

- [ ] **Step 5: Implement SOLO return CTA and keep `taskId` as the primary detail selector**

```tsx
const returnTo = new URLSearchParams(window.location.search).get('returnTo');

{returnTo ? (
  <button onClick={() => navigate(returnTo)} type="button">
    Back to chat
  </button>
) : null}
```

- [ ] **Step 6: Re-run the return-flow tests to verify they pass**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/routes/workspace/KnowledgePage.test.tsx \
  src/routes/workspace/SoloPage.test.tsx \
  src/features/layouts/WorkspaceLayout.test.tsx
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add \
  src/web/src/features/layouts/WorkspaceLayout.tsx \
  src/web/src/features/layouts/WorkspaceLayout.test.tsx \
  src/web/src/routes/workspace/KnowledgePage.tsx \
  src/web/src/routes/workspace/KnowledgePage.test.tsx \
  src/web/src/routes/workspace/SoloPage.tsx \
  src/web/src/routes/workspace/SoloPage.test.tsx
git commit -m "feat(web): add workspace return paths"
```

## Task 5: Update Contracts And Run Final Web Regression

**Files:**
- Modify: `docs/architecture/current-system-contracts.md`
- Modify: `src/web/src/app/router.test.tsx`

- [ ] **Step 1: Update the contract doc for the new main flow**

```md
### 7.1 当前已注册路由

| Workspace | `/onboarding` | 已接入，允许跳过但仍作为首次引导页 |
| Workspace | `/chat` | 已接入，作为默认主入口与会话空状态页 |
| Workspace | `/chat/:conversationId` | 已接入，支持消息、知识库绑定与 SOLO handoff |
| Workspace | `/knowledge` | 已接入，支持 `returnTo` 回到 Chat |
| Workspace | `/solo` | 已接入，支持 `taskId` 与 Chat-originated return flow |
| Workspace | `/settings` | 已接入，作为长期偏好页 |
```

- [ ] **Step 2: Extend the router smoke tests for the stabilized workspace flow**

```tsx
it('renders onboarding inside the workspace shell', () => {
  const router = createAppRouter(['/onboarding']);

  render(<RouterProvider router={router} />);

  expect(screen.getByText('Workspace')).toBeInTheDocument();
  expect(screen.getByRole('heading', { name: 'Onboarding' })).toBeInTheDocument();
});
```

- [ ] **Step 3: Run the focused regression suite**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/features/auth/workspaceLanding.test.ts \
  src/features/auth/ProtectedRoute.test.tsx \
  src/app/router.test.tsx \
  src/routes/workspace/OnboardingPage.test.tsx \
  src/routes/workspace/ChatPage.behavior.test.tsx \
  src/routes/workspace/ChatPage.test.tsx \
  src/routes/workspace/KnowledgePage.test.tsx \
  src/routes/workspace/SoloPage.test.tsx \
  src/routes/workspace/SettingsPage.test.tsx \
  src/features/layouts/WorkspaceLayout.test.tsx
```

Expected: PASS

- [ ] **Step 4: Run the full web verification**

Run:

```bash
pnpm --dir src/web test
pnpm --dir src/web build
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add \
  docs/architecture/current-system-contracts.md \
  src/web/src/app/router.test.tsx
git commit -m "docs: record workspace main flow behavior"
```
