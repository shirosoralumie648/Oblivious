import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';

import { createKnowledgeApi } from '../../features/knowledge/api';
import { createChatApi } from '../../features/chat/api';
import { createTasksApi } from '../../features/tasks/api';
import { createHttpClient } from '../../services/http/client';
import type {
  ConversationConfig,
  ConvertConversationToTaskResponse,
  KnowledgeBaseSummary,
  TaskSummary,
  UpdateConversationConfigRequest
} from '../../types/api';

function parseToolList(value: string) {
  return value
    .split(',')
    .map((entry) => entry.trim())
    .filter((entry, index, values) => entry !== '' && values.indexOf(entry) === index);
}

export function ChatPage() {
  const { conversationId } = useParams<{ conversationId?: string }>();
  const navigate = useNavigate();
  const httpClient = useMemo(() => createHttpClient(), []);
  const chatApi = useMemo(() => createChatApi(httpClient), [httpClient]);
  const knowledgeApi = useMemo(() => createKnowledgeApi(httpClient), [httpClient]);
  const tasksApi = useMemo(() => createTasksApi(httpClient), [httpClient]);
  const [conversationConfig, setConversationConfig] = useState<ConversationConfig | null>(null);
  const [handoffDraft, setHandoffDraft] = useState<ConvertConversationToTaskResponse | null>(null);
  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBaseSummary[]>([]);
  const [authorizationScope, setAuthorizationScope] = useState('workspace_tools');
  const [allowedTools, setAllowedTools] = useState('');
  const [blockedTools, setBlockedTools] = useState('');
  const [selectedKnowledgeBaseIds, setSelectedKnowledgeBaseIds] = useState<string[]>([]);

  useEffect(() => {
    let cancelled = false;

    const loadChatWorkspace = async () => {
      try {
        if (conversationId) {
          const [, , nextKnowledgeBases, , nextConversationConfig] = await Promise.all([
            chatApi.listConversations(),
            chatApi.listModels(),
            knowledgeApi.listKnowledgeBases(),
            chatApi.listMessages(conversationId),
            chatApi.getConversationConfig(conversationId)
          ]);

          if (cancelled) {
            return;
          }

          setConversationConfig(nextConversationConfig);
          setSelectedKnowledgeBaseIds(nextConversationConfig.knowledgeBaseIds);
          setKnowledgeBases(nextKnowledgeBases);
          return;
        }

        const [, , nextKnowledgeBases] = await Promise.all([
          chatApi.listConversations(),
          chatApi.listModels(),
          knowledgeApi.listKnowledgeBases()
        ]);
        if (cancelled) {
          return;
        }

        setKnowledgeBases(nextKnowledgeBases);
      } catch {
        if (!cancelled) {
          setConversationConfig(null);
          setKnowledgeBases([]);
        }
      }
    };

    void loadChatWorkspace();

    return () => {
      cancelled = true;
    };
  }, [chatApi, conversationId, knowledgeApi]);

  const updateKnowledgeBinding = async (knowledgeBaseId: string) => {
    if (!conversationId || conversationConfig === null) {
      return;
    }

    const nextKnowledgeBaseIds = conversationConfig.knowledgeBaseIds.includes(knowledgeBaseId)
      ? conversationConfig.knowledgeBaseIds.filter((currentId) => currentId !== knowledgeBaseId)
      : [...conversationConfig.knowledgeBaseIds, knowledgeBaseId];
    const nextConfig: UpdateConversationConfigRequest = {
      knowledgeBaseIds: nextKnowledgeBaseIds,
      maxOutputTokens: conversationConfig.maxOutputTokens,
      modelId: conversationConfig.modelId,
      systemPromptOverride: conversationConfig.systemPromptOverride,
      temperature: conversationConfig.temperature,
      toolsEnabled: conversationConfig.toolsEnabled
    };
    const savedConfig = await chatApi.updateConversationConfig(conversationId, nextConfig);

    setConversationConfig(savedConfig);
  };

  const openSoloHandoff = async () => {
    if (!conversationId) {
      return;
    }

    const draft = await chatApi.convertConversationToTask(conversationId);
    setAuthorizationScope('workspace_tools');
    setAllowedTools('');
    setBlockedTools('');
    setSelectedKnowledgeBaseIds(draft.relatedKnowledgeBaseIds);
    setHandoffDraft(draft);
  };

  const startInSolo = async () => {
    if (handoffDraft === null) {
      return;
    }

    const createTaskPayload: Parameters<typeof tasksApi.createTask>[0] = {
      authorizationScope,
      budgetLimit: handoffDraft.suggestedBudget,
      executionMode: handoffDraft.suggestedExecutionMode,
      goal: handoffDraft.draftTaskGoal,
      knowledgeBaseIds: selectedKnowledgeBaseIds
    };
    const toolAllowList = parseToolList(allowedTools);
    const toolDenyList = parseToolList(blockedTools);

    if (toolAllowList.length > 0) {
      createTaskPayload.toolAllowList = toolAllowList;
    }
    if (toolDenyList.length > 0) {
      createTaskPayload.toolDenyList = toolDenyList;
    }

    const createdTask: TaskSummary = await tasksApi.createTask(createTaskPayload);
    await tasksApi.startTask(createdTask.id);
    navigate(`/solo?taskId=${createdTask.id}`);
  };

  const toggleSoloKnowledgeBase = (knowledgeBaseId: string) => {
    setSelectedKnowledgeBaseIds((current) =>
      current.includes(knowledgeBaseId)
        ? current.filter((currentId) => currentId !== knowledgeBaseId)
        : [...current, knowledgeBaseId]
    );
  };

  return (
    <section>
      <h1>Chat workspace</h1>
      {conversationConfig !== null ? (
        <section>
          <h2>Conversation settings</h2>
          {knowledgeBases.map((knowledgeBase) => (
            <label key={knowledgeBase.id}>
              <input
                checked={conversationConfig.knowledgeBaseIds.includes(knowledgeBase.id)}
                onChange={() => void updateKnowledgeBinding(knowledgeBase.id)}
                type="checkbox"
              />
              {`Use knowledge base ${knowledgeBase.name}`}
            </label>
          ))}
        </section>
      ) : null}
      {conversationId ? (
        <button onClick={() => void openSoloHandoff()} type="button">
          Hand off to SOLO
        </button>
      ) : null}
      {handoffDraft !== null ? (
        <section>
          <h2>Convert to SOLO task</h2>
          <label>
            SOLO task goal
            <textarea readOnly value={handoffDraft.draftTaskGoal} />
          </label>
          <label>
            Authorization scope for SOLO
            <select onChange={(event) => setAuthorizationScope(event.target.value)} value={authorizationScope}>
              <option value="knowledge_only">knowledge_only</option>
              <option value="workspace_tools">workspace_tools</option>
              <option value="full_access">full_access</option>
            </select>
          </label>
          <label>
            Allowed tools for SOLO
            <input onChange={(event) => setAllowedTools(event.target.value)} type="text" value={allowedTools} />
          </label>
          <label>
            Blocked tools for SOLO
            <input onChange={(event) => setBlockedTools(event.target.value)} type="text" value={blockedTools} />
          </label>
          {knowledgeBases.map((knowledgeBase) => (
            <label key={knowledgeBase.id}>
              <input
                checked={selectedKnowledgeBaseIds.includes(knowledgeBase.id)}
                onChange={() => toggleSoloKnowledgeBase(knowledgeBase.id)}
                type="checkbox"
              />
              {`Use knowledge base ${knowledgeBase.name} in SOLO`}
            </label>
          ))}
          <button onClick={() => void startInSolo()} type="button">
            Start in SOLO
          </button>
        </section>
      ) : null}
    </section>
  );
}
