import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';

import { useAppContext } from '../../app/providers';
import { createKnowledgeApi } from '../../features/knowledge/api';
import { createChatApi } from '../../features/chat/api';
import { createTasksApi } from '../../features/tasks/api';
import { createHttpClient } from '../../services/http/client';
import type {
  ConversationConfig,
  ConversationMessage,
  ConversationSummary,
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
  const { authState } = useAppContext();
  const httpClient = useMemo(() => createHttpClient(), []);
  const chatApi = useMemo(() => createChatApi(httpClient), [httpClient]);
  const knowledgeApi = useMemo(() => createKnowledgeApi(httpClient), [httpClient]);
  const tasksApi = useMemo(() => createTasksApi(httpClient), [httpClient]);
  const [conversations, setConversations] = useState<ConversationSummary[]>([]);
  const [conversationConfig, setConversationConfig] = useState<ConversationConfig | null>(null);
  const [handoffDraft, setHandoffDraft] = useState<ConvertConversationToTaskResponse | null>(null);
  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBaseSummary[]>([]);
  const [messageDraft, setMessageDraft] = useState('');
  const [messages, setMessages] = useState<ConversationMessage[]>([]);
  const [authorizationScope, setAuthorizationScope] = useState('workspace_tools');
  const [allowedTools, setAllowedTools] = useState('');
  const [blockedTools, setBlockedTools] = useState('');
  const [selectedKnowledgeBaseIds, setSelectedKnowledgeBaseIds] = useState<string[]>([]);
  const chatReturnPath = conversationId ? `/chat/${conversationId}` : '/chat';

  useEffect(() => {
    let cancelled = false;

    const loadChatWorkspace = async () => {
      try {
        if (conversationId) {
          const [nextConversations, , nextKnowledgeBases, nextMessages, nextConversationConfig] = await Promise.all([
            chatApi.listConversations(),
            chatApi.listModels(),
            knowledgeApi.listKnowledgeBases(),
            chatApi.listMessages(conversationId),
            chatApi.getConversationConfig(conversationId)
          ]);

          if (cancelled) {
            return;
          }

          setConversations(nextConversations);
          setConversationConfig(nextConversationConfig);
          setHandoffDraft(null);
          setKnowledgeBases(nextKnowledgeBases);
          setMessages(nextMessages);
          setMessageDraft('');
          setSelectedKnowledgeBaseIds(nextConversationConfig.knowledgeBaseIds);
          return;
        }

        const [nextConversations, , nextKnowledgeBases] = await Promise.all([
          chatApi.listConversations(),
          chatApi.listModels(),
          knowledgeApi.listKnowledgeBases()
        ]);
        if (cancelled) {
          return;
        }

        setConversations(nextConversations);
        setConversationConfig(null);
        setHandoffDraft(null);
        setKnowledgeBases(nextKnowledgeBases);
        setMessages([]);
        setMessageDraft('');
        setSelectedKnowledgeBaseIds([]);
      } catch {
        if (!cancelled) {
          setConversations([]);
          setConversationConfig(null);
          setHandoffDraft(null);
          setKnowledgeBases([]);
          setMessages([]);
          setMessageDraft('');
          setSelectedKnowledgeBaseIds([]);
        }
      }
    };

    void loadChatWorkspace();

    return () => {
      cancelled = true;
    };
  }, [chatApi, conversationId, knowledgeApi]);

  const handleCreateConversation = async () => {
    const conversation = await chatApi.createConversation({ title: 'New conversation' });
    setConversations((current) => [conversation, ...current]);
    navigate(`/chat/${conversation.id}`);
  };

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
    setSelectedKnowledgeBaseIds(savedConfig.knowledgeBaseIds);
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
    navigate(`/solo?taskId=${createdTask.id}&returnTo=${encodeURIComponent(chatReturnPath)}`);
  };

  const toggleSoloKnowledgeBase = (knowledgeBaseId: string) => {
    setSelectedKnowledgeBaseIds((current) =>
      current.includes(knowledgeBaseId)
        ? current.filter((currentId) => currentId !== knowledgeBaseId)
        : [...current, knowledgeBaseId]
    );
  };

  const handleSendMessage = async () => {
    const trimmedContent = messageDraft.trim();

    if (!conversationId || trimmedContent === '') {
      return;
    }

    const nextMessages = await chatApi.sendMessage(conversationId, { content: trimmedContent });
    setMessages(nextMessages);
    setMessageDraft('');
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
          <section>
            <h2>Recent conversations</h2>
            {conversations.map((conversation) => (
              <button key={conversation.id} onClick={() => navigate(`/chat/${conversation.id}`)} type="button">
                {conversation.title}
              </button>
            ))}
          </section>
        )}
      </section>
    );
  }

  return (
    <section>
      <h1>Chat workspace</h1>
      {!authState.preferences?.onboardingCompleted ? (
        <section>
          <p>Finish setup to lock in your default workspace preferences.</p>
          <button onClick={() => navigate('/onboarding')} type="button">
            Complete setup
          </button>
        </section>
      ) : null}
      <section>
        <h2>Conversation transcript</h2>
        {messages.length > 0 ? (
          <ul>
            {messages.map((message) => (
              <li key={message.id}>{message.content}</li>
            ))}
          </ul>
        ) : (
          <p>No messages yet.</p>
        )}
      </section>
      <label>
        Message draft
        <textarea onChange={(event) => setMessageDraft(event.target.value)} value={messageDraft} />
      </label>
      <button onClick={() => void handleSendMessage()} type="button">
        Send message
      </button>
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
      {conversationConfig !== null && knowledgeBases.length === 0 ? (
        <button onClick={() => navigate(`/knowledge?returnTo=${encodeURIComponent(chatReturnPath)}`)} type="button">
          Create knowledge base
        </button>
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
