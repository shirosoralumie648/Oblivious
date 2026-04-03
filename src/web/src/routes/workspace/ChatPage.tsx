import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';

import { useAppContext } from '../../app/providers';
import { createChatApi } from '../../features/chat/api';
import { createKnowledgeApi } from '../../features/knowledge/api';
import { createTasksApi } from '../../features/tasks/api';
import { createHttpClient } from '../../services/http/client';
import type {
  ChatMessage,
  ConversationConfig,
  ConversationTaskDraft,
  ConversationSummary,
  KnowledgeBaseSummary,
  MessageOverrides,
  ModelOption
} from '../../types/api';

const emptyMessageOverrides: MessageOverrides = {
  maxOutputTokens: undefined,
  modelId: undefined,
  systemPromptOverride: undefined,
  temperature: undefined,
  toolsEnabled: undefined
};
const defaultSoloAuthorizationScope = 'workspace_tools';

export function ChatPage() {
  const { authState } = useAppContext();
  const { conversationId } = useParams<{ conversationId?: string }>();
  const navigate = useNavigate();
  const chatApi = useMemo(() => createChatApi(createHttpClient()), []);
  const knowledgeApi = useMemo(() => createKnowledgeApi(createHttpClient()), []);
  const tasksApi = useMemo(() => createTasksApi(createHttpClient()), []);
  const [availableModels, setAvailableModels] = useState<ModelOption[]>([]);
  const [availableKnowledgeBases, setAvailableKnowledgeBases] = useState<KnowledgeBaseSummary[]>([]);
  const [configError, setConfigError] = useState<string | null>(null);
  const [conversationConfig, setConversationConfig] = useState<ConversationConfig | null>(null);
  const [conversations, setConversations] = useState<ConversationSummary[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [isCreatingConversation, setIsCreatingConversation] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [isLoadingConfig, setIsLoadingConfig] = useState(false);
  const [isLoadingMessages, setIsLoadingMessages] = useState(false);
  const [isLoadingModels, setIsLoadingModels] = useState(false);
  const [isLoadingKnowledgeBases, setIsLoadingKnowledgeBases] = useState(false);
  const [isSavingConfig, setIsSavingConfig] = useState(false);
  const [isSending, setIsSending] = useState(false);
  const [isPreparingSoloTask, setIsPreparingSoloTask] = useState(false);
  const [isStartingSoloTask, setIsStartingSoloTask] = useState(false);
  const [messageInput, setMessageInput] = useState('');
  const [messageOverrides, setMessageOverrides] = useState<MessageOverrides>(emptyMessageOverrides);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [soloTaskBudgetLimit, setSoloTaskBudgetLimit] = useState('20');
  const [soloTaskDraft, setSoloTaskDraft] = useState<ConversationTaskDraft | null>(null);
  const [soloTaskAuthorizationScope, setSoloTaskAuthorizationScope] = useState(defaultSoloAuthorizationScope);
  const [soloTaskExecutionMode, setSoloTaskExecutionMode] = useState('standard');
  const [soloTaskGoal, setSoloTaskGoal] = useState('');
  const [soloTaskKnowledgeBaseIDs, setSoloTaskKnowledgeBaseIDs] = useState<string[]>([]);
  const [showMessageOverrides, setShowMessageOverrides] = useState(false);

  const currentConversationId = conversationId ?? null;
  const canSendMessage = !isSending && messageInput.trim() !== '' && currentConversationId !== null;
  const selectedConversation = conversations.find((conversation) => conversation.id === currentConversationId) ?? null;

  useEffect(() => {
    setSoloTaskBudgetLimit('20');
    setSoloTaskDraft(null);
    setSoloTaskAuthorizationScope(defaultSoloAuthorizationScope);
    setSoloTaskExecutionMode('standard');
    setSoloTaskGoal('');
    setSoloTaskKnowledgeBaseIDs([]);
  }, [currentConversationId]);

  useEffect(() => {
    let cancelled = false;

    const loadConversations = async () => {
      if (authState.status !== 'authenticated') {
        setConversations([]);
        setMessages([]);
        setError(null);
        setIsLoading(false);
        return;
      }

      setIsLoading(true);
      setError(null);

      try {
        const nextConversations = await chatApi.listConversations();

        if (cancelled) {
          return;
        }

        setConversations(nextConversations);

        if (nextConversations.length === 0) {
          setMessages([]);
          setConversationConfig(null);
          return;
        }

        const hasSelectedConversation = currentConversationId
          ? nextConversations.some((conversation) => conversation.id === currentConversationId)
          : false;

        if (!hasSelectedConversation) {
          navigate(`/chat/${nextConversations[0].id}`, { replace: true });
        }
      } catch {
        if (!cancelled) {
          setError('Unable to load conversations.');
          setConversations([]);
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    };

    void loadConversations();

    return () => {
      cancelled = true;
    };
  }, [authState.status, chatApi, currentConversationId, navigate]);

  useEffect(() => {
    let cancelled = false;

    const loadModels = async () => {
      if (authState.status !== 'authenticated') {
        setAvailableModels([]);
        return;
      }

      setIsLoadingModels(true);

      try {
        const nextModels = await chatApi.listModels();
        if (!cancelled) {
          setAvailableModels(nextModels);
        }
      } catch {
        if (!cancelled) {
          setAvailableModels([]);
          setConfigError('Unable to load available models.');
        }
      } finally {
        if (!cancelled) {
          setIsLoadingModels(false);
        }
      }
    };

    void loadModels();

    return () => {
      cancelled = true;
    };
  }, [authState.status, chatApi]);

  useEffect(() => {
    let cancelled = false;

    const loadKnowledgeBases = async () => {
      if (authState.status !== 'authenticated') {
        setAvailableKnowledgeBases([]);
        return;
      }

      setIsLoadingKnowledgeBases(true);

      try {
        const nextKnowledgeBases = await knowledgeApi.listKnowledgeBases();
        if (!cancelled) {
          setAvailableKnowledgeBases(nextKnowledgeBases);
        }
      } catch {
        if (!cancelled) {
          setAvailableKnowledgeBases([]);
          setConfigError('Unable to load knowledge bases.');
        }
      } finally {
        if (!cancelled) {
          setIsLoadingKnowledgeBases(false);
        }
      }
    };

    void loadKnowledgeBases();

    return () => {
      cancelled = true;
    };
  }, [authState.status, knowledgeApi]);

  useEffect(() => {
    let cancelled = false;

    const loadMessages = async () => {
      if (!currentConversationId) {
        setMessages([]);
        return;
      }

      setIsLoadingMessages(true);

      try {
        const nextMessages = await chatApi.listMessages(currentConversationId);

        if (!cancelled) {
          setMessages(nextMessages);
        }
      } catch {
        if (!cancelled) {
          setError('Unable to load messages.');
          setMessages([]);
        }
      } finally {
        if (!cancelled) {
          setIsLoadingMessages(false);
        }
      }
    };

    void loadMessages();

    return () => {
      cancelled = true;
    };
  }, [chatApi, currentConversationId]);

  useEffect(() => {
    let cancelled = false;

    const loadConversationConfig = async () => {
      if (!currentConversationId) {
        setConversationConfig(null);
        setConfigError(null);
        return;
      }

      setIsLoadingConfig(true);
      setConfigError(null);

      try {
        const nextConfig = await chatApi.getConversationConfig(currentConversationId);
        if (!cancelled) {
          setConversationConfig({
            ...nextConfig,
            knowledgeBaseIds: nextConfig.knowledgeBaseIds ?? []
          });
        }
      } catch {
        if (!cancelled) {
          setConversationConfig(null);
          setConfigError('Unable to load conversation settings.');
        }
      } finally {
        if (!cancelled) {
          setIsLoadingConfig(false);
        }
      }
    };

    void loadConversationConfig();

    return () => {
      cancelled = true;
    };
  }, [chatApi, currentConversationId]);

  const handleCreateConversation = async () => {
    setIsCreatingConversation(true);

    try {
      const conversation = await chatApi.createConversation();
      setConversations((current) => [conversation, ...current]);
      setMessages([]);
      setError(null);
      navigate(`/chat/${conversation.id}`);
    } catch {
      setError('Unable to create conversation.');
    } finally {
      setIsCreatingConversation(false);
    }
  };

  const handleUpdateConfig = async (patch: Partial<ConversationConfig>) => {
    if (!currentConversationId || !conversationConfig) {
      return;
    }

    const nextConfig = {
      ...conversationConfig,
      ...patch
    };

    setIsSavingConfig(true);
    setConfigError(null);

    try {
      const savedConfig = await chatApi.updateConversationConfig(currentConversationId, {
        knowledgeBaseIds: nextConfig.knowledgeBaseIds ?? [],
        maxOutputTokens: nextConfig.maxOutputTokens,
        modelId: nextConfig.modelId,
        systemPromptOverride: nextConfig.systemPromptOverride ?? '',
        temperature: nextConfig.temperature,
        toolsEnabled: nextConfig.toolsEnabled
      });
      setConversationConfig({
        ...savedConfig,
        knowledgeBaseIds: savedConfig.knowledgeBaseIds ?? []
      });
    } catch {
      setConfigError('Unable to save conversation settings.');
    } finally {
      setIsSavingConfig(false);
    }
  };

  const handleSendMessage = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!canSendMessage || !currentConversationId) {
      return;
    }

    setIsSending(true);
    setError(null);

    try {
      const overrides = Object.fromEntries(
        Object.entries(messageOverrides).filter(([, value]) => value !== undefined && value !== '')
      ) as MessageOverrides;
      const nextMessages = await chatApi.sendMessage(currentConversationId, {
        content: messageInput.trim(),
        overrides: Object.keys(overrides).length > 0 ? overrides : undefined
      });
      setMessages(nextMessages);
      setMessageInput('');
      setMessageOverrides(emptyMessageOverrides);
      setShowMessageOverrides(false);
    } catch {
      setError('Unable to send message.');
    } finally {
      setIsSending(false);
    }
  };

  const handlePrepareSoloTask = async () => {
    if (!currentConversationId) {
      return;
    }

    setIsPreparingSoloTask(true);
    setError(null);

    try {
      const nextDraft = await chatApi.convertConversationToTask(currentConversationId);
      setSoloTaskDraft(nextDraft);
      setSoloTaskGoal(nextDraft.draftTaskGoal);
      setSoloTaskExecutionMode(nextDraft.suggestedExecutionMode);
      setSoloTaskBudgetLimit(String(nextDraft.suggestedBudget));
      setSoloTaskKnowledgeBaseIDs(nextDraft.relatedKnowledgeBaseIds ?? []);
    } catch {
      setError('Unable to prepare a SOLO task from this conversation.');
    } finally {
      setIsPreparingSoloTask(false);
    }
  };

  const handleStartSoloTask = async () => {
    const trimmedGoal = soloTaskGoal.trim();
    if (trimmedGoal === '') {
      setError('SOLO task goal is required.');
      return;
    }

    setIsStartingSoloTask(true);
    setError(null);

    try {
      const parsedBudgetLimit = Number.parseInt(soloTaskBudgetLimit, 10);
      const createdTask = await tasksApi.createTask({
        authorizationScope: soloTaskAuthorizationScope,
        budgetLimit: Number.isNaN(parsedBudgetLimit) ? 0 : parsedBudgetLimit,
        executionMode: soloTaskExecutionMode,
        goal: trimmedGoal,
        knowledgeBaseIds: soloTaskKnowledgeBaseIDs
      });
      await tasksApi.startTask(createdTask.id);
      navigate(`/solo?taskId=${createdTask.id}`);
    } catch {
      setError('Unable to start the SOLO task.');
    } finally {
      setIsStartingSoloTask(false);
    }
  };

  if (authState.status === 'loading' || isLoading) {
    return <p>Loading conversations…</p>;
  }

  return (
    <section>
      <header>
        <div>
          <h1>Chat</h1>
          <p>{selectedConversation ? selectedConversation.title : 'Start a conversation to begin.'}</p>
        </div>
        <button disabled={isCreatingConversation} onClick={() => void handleCreateConversation()} type="button">
          {isCreatingConversation ? 'Creating…' : 'New conversation'}
        </button>
      </header>
      {error ? <p>{error}</p> : null}
      <div>
        <aside>
          <h2>Conversations</h2>
          {conversations.length === 0 ? (
            <p>No conversations yet. Create one to start chatting.</p>
          ) : (
            <ul>
              {conversations.map((conversation) => {
                const isSelected = conversation.id === currentConversationId;

                return (
                  <li key={conversation.id}>
                    <button
                      aria-current={isSelected ? 'page' : undefined}
                      onClick={() => navigate(`/chat/${conversation.id}`)}
                      type="button"
                    >
                      {isSelected ? '• ' : ''}
                      {conversation.title}
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </aside>
        <div>
          {currentConversationId ? (
            <>
              <section>
                <h2>Conversation settings</h2>
                {isLoadingConfig ? <p>Loading conversation settings…</p> : null}
                {isLoadingModels ? <p>Loading available models…</p> : null}
                {isLoadingKnowledgeBases ? <p>Loading knowledge bases…</p> : null}
                {configError ? <p>{configError}</p> : null}
                <label>
                  Model
                  <select
                    disabled={isLoadingConfig || isLoadingModels || isSavingConfig || conversationConfig === null}
                    onChange={(event) => void handleUpdateConfig({ modelId: event.target.value })}
                    value={conversationConfig?.modelId ?? ''}
                  >
                    {availableModels.map((model) => (
                      <option key={model.id} value={model.id}>
                        {model.label}
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  System prompt override
                  <textarea
                    disabled={isLoadingConfig || isSavingConfig || conversationConfig === null}
                    onBlur={(event) => void handleUpdateConfig({ systemPromptOverride: event.target.value })}
                    defaultValue={conversationConfig?.systemPromptOverride ?? ''}
                  />
                </label>
                <label>
                  Temperature
                  <input
                    defaultValue={conversationConfig?.temperature ?? 1}
                    disabled={isLoadingConfig || isSavingConfig || conversationConfig === null}
                    max="2"
                    min="0"
                    onBlur={(event) => void handleUpdateConfig({ temperature: Number(event.target.value) })}
                    step="0.1"
                    type="number"
                  />
                </label>
                <label>
                  Max output tokens
                  <input
                    defaultValue={conversationConfig?.maxOutputTokens ?? 1024}
                    disabled={isLoadingConfig || isSavingConfig || conversationConfig === null}
                    min="1"
                    onBlur={(event) => void handleUpdateConfig({ maxOutputTokens: Number(event.target.value) })}
                    type="number"
                  />
                </label>
                <label>
                  <input
                    checked={conversationConfig?.toolsEnabled ?? false}
                    disabled={isLoadingConfig || isSavingConfig || conversationConfig === null}
                    onChange={(event) => void handleUpdateConfig({ toolsEnabled: event.target.checked })}
                    type="checkbox"
                  />
                  Tools enabled
                </label>
                <fieldset disabled={isLoadingConfig || isLoadingKnowledgeBases || isSavingConfig || conversationConfig === null}>
                  <legend>Knowledge bases</legend>
                  {availableKnowledgeBases.length === 0 ? (
                    <p>No knowledge bases available yet.</p>
                  ) : (
                    availableKnowledgeBases.map((knowledgeBase) => {
                      const checked = conversationConfig?.knowledgeBaseIds.includes(knowledgeBase.id) ?? false;

                      return (
                        <label key={knowledgeBase.id}>
                          <input
                            checked={checked}
                            onChange={(event) => {
                              const currentKnowledgeBaseIds = conversationConfig?.knowledgeBaseIds ?? [];
                              const nextKnowledgeBaseIds = event.target.checked
                                ? currentKnowledgeBaseIds.includes(knowledgeBase.id)
                                  ? currentKnowledgeBaseIds
                                  : [...currentKnowledgeBaseIds, knowledgeBase.id]
                                : currentKnowledgeBaseIds.filter((currentKnowledgeBaseId) => currentKnowledgeBaseId !== knowledgeBase.id);

                              void handleUpdateConfig({ knowledgeBaseIds: nextKnowledgeBaseIds });
                            }}
                            type="checkbox"
                          />
                          {`Use knowledge base ${knowledgeBase.name}`}
                        </label>
                      );
                    })
                  )}
                </fieldset>
              </section>
              <section>
                <h2>SOLO handoff</h2>
                <button
                  disabled={isPreparingSoloTask || isStartingSoloTask}
                  onClick={() => void handlePrepareSoloTask()}
                  type="button"
                >
                  {isPreparingSoloTask ? 'Preparing SOLO handoff…' : 'Hand off to SOLO'}
                </button>
                {soloTaskDraft ? (
                  <section>
                    <h3>Convert to SOLO task</h3>
                    <label>
                      SOLO task goal
                      <textarea onChange={(event) => setSoloTaskGoal(event.target.value)} rows={4} value={soloTaskGoal} />
                    </label>
                    <label>
                      Execution mode
                      <select
                        onChange={(event) => setSoloTaskExecutionMode(event.target.value)}
                        value={soloTaskExecutionMode}
                      >
                        <option value="safe">safe</option>
                        <option value="standard">standard</option>
                        <option value="auto">auto</option>
                      </select>
                    </label>
                    <label>
                      Authorization scope for SOLO
                      <select
                        onChange={(event) => setSoloTaskAuthorizationScope(event.target.value)}
                        value={soloTaskAuthorizationScope}
                      >
                        <option value="knowledge_only">knowledge_only</option>
                        <option value="workspace_tools">workspace_tools</option>
                        <option value="full_access">full_access</option>
                      </select>
                    </label>
                    <fieldset>
                      <legend>Knowledge sources for SOLO</legend>
                      {availableKnowledgeBases.length === 0 ? (
                        <p>No knowledge bases available yet.</p>
                      ) : (
                        availableKnowledgeBases.map((knowledgeBase) => (
                          <label key={knowledgeBase.id}>
                            <input
                              checked={soloTaskKnowledgeBaseIDs.includes(knowledgeBase.id)}
                              onChange={(event) => {
                                setSoloTaskKnowledgeBaseIDs((current) =>
                                  event.target.checked
                                    ? current.includes(knowledgeBase.id)
                                      ? current
                                      : [...current, knowledgeBase.id]
                                    : current.filter((currentKnowledgeBaseID) => currentKnowledgeBaseID !== knowledgeBase.id)
                                );
                              }}
                              type="checkbox"
                            />
                            {`Use knowledge base ${knowledgeBase.name} in SOLO`}
                          </label>
                        ))
                      )}
                    </fieldset>
                    <label>
                      Budget limit
                      <input
                        onChange={(event) => setSoloTaskBudgetLimit(event.target.value)}
                        type="number"
                        value={soloTaskBudgetLimit}
                      />
                    </label>
                    <button disabled={isStartingSoloTask} onClick={() => void handleStartSoloTask()} type="button">
                      {isStartingSoloTask ? 'Starting SOLO…' : 'Start in SOLO'}
                    </button>
                  </section>
                ) : null}
              </section>
              {isLoadingMessages ? (
                <p>Loading messages…</p>
              ) : messages.length === 0 ? (
                <p>No messages yet. Send the first message to start this chat.</p>
              ) : (
                <ul>
                  {messages.map((message) => (
                    <li key={message.id}>
                      <strong>{message.role}:</strong> {message.content}
                    </li>
                  ))}
                </ul>
              )}
              <form onSubmit={handleSendMessage}>
                <label>
                  Message
                  <input
                    disabled={isSending || currentConversationId === null}
                    onChange={(event) => setMessageInput(event.target.value)}
                    placeholder="Send a message"
                    value={messageInput}
                  />
                </label>
                <button onClick={() => setShowMessageOverrides((current) => !current)} type="button">
                  {showMessageOverrides ? 'Hide per-message overrides' : 'Use per-message overrides'}
                </button>
                {showMessageOverrides ? (
                  <section>
                    <h2>Message overrides</h2>
                    <label>
                      Model override
                      <select
                        onChange={(event) => setMessageOverrides((current) => ({ ...current, modelId: event.target.value || undefined }))}
                        value={messageOverrides.modelId ?? ''}
                      >
                        <option value="">Use conversation default</option>
                        {availableModels.map((model) => (
                          <option key={model.id} value={model.id}>
                            {model.label}
                          </option>
                        ))}
                      </select>
                    </label>
                    <label>
                      System prompt override
                      <textarea
                        onChange={(event) => setMessageOverrides((current) => ({ ...current, systemPromptOverride: event.target.value || undefined }))}
                        value={messageOverrides.systemPromptOverride ?? ''}
                      />
                    </label>
                    <label>
                      Temperature override
                      <input
                        max="2"
                        min="0"
                        onChange={(event) => setMessageOverrides((current) => ({
                          ...current,
                          temperature: event.target.value === '' ? undefined : Number(event.target.value)
                        }))}
                        step="0.1"
                        type="number"
                        value={messageOverrides.temperature ?? ''}
                      />
                    </label>
                    <label>
                      Max output tokens override
                      <input
                        min="1"
                        onChange={(event) => setMessageOverrides((current) => ({
                          ...current,
                          maxOutputTokens: event.target.value === '' ? undefined : Number(event.target.value)
                        }))}
                        type="number"
                        value={messageOverrides.maxOutputTokens ?? ''}
                      />
                    </label>
                    <label>
                      <input
                        checked={messageOverrides.toolsEnabled ?? false}
                        onChange={(event) => setMessageOverrides((current) => ({ ...current, toolsEnabled: event.target.checked }))}
                        type="checkbox"
                      />
                      Tools enabled for this message
                    </label>
                  </section>
                ) : null}
                <button disabled={!canSendMessage} type="submit">
                  {isSending ? 'Sending…' : 'Send'}
                </button>
              </form>
            </>
          ) : (
            <div>
              <p>Create a conversation to start chatting.</p>
              <button disabled={isCreatingConversation} onClick={() => void handleCreateConversation()} type="button">
                {isCreatingConversation ? 'Creating…' : 'Create your first conversation'}
              </button>
            </div>
          )}
        </div>
      </div>
    </section>
  );
}
