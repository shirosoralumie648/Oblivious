import { RiAddLine, RiCloseLine, RiMenuLine } from '@remixicon/react';
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
  KnowledgeCitation,
  MessageAttachment,
  PersonaRequest,
  PersonaSummary,
  TaskSummary,
  UpdateConversationConfigRequest
} from '../../types/api';

function parseToolList(value: string) {
  return value
    .split(',')
    .map((entry) => entry.trim())
    .filter((entry, index, values) => entry !== '' && values.indexOf(entry) === index);
}

type ActionError = {
  message?: string;
  title: string;
};

type ConversationRailFilter = 'all' | 'starred' | 'archived';

type PersonaDraft = {
  constraints: string;
  name: string;
  openingMessage: string;
  role: string;
  style: string;
  suggestedQuestionsText: string;
  tone: string;
};

type MessageContentBlock =
  | {
      content: string;
      language: string;
      type: 'code';
    }
  | {
      content: string;
      type: 'heading';
    }
  | {
      content: string;
      type: 'paragraph';
    }
  | {
      items: string[];
      type: 'list';
    };

type InlineMarkdownToken =
  | {
      content: string;
      type: 'code' | 'strong' | 'text';
    };

function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim() !== '') {
    return error.message;
  }
  if (typeof error === 'string' && error.trim() !== '') {
    return error;
  }

  return fallback;
}

function toActionError(title: string, error: unknown): ActionError {
  return {
    message: getErrorMessage(error, 'The action failed. Try again or contact support if the issue continues.'),
    title
  };
}

function forkTitleForMessage(message: ConversationMessage) {
  const normalizedContent = message.content.trim().replace(/\s+/g, ' ');
  const titleSource = normalizedContent === '' ? message.id : normalizedContent;
  const shortened = titleSource.length > 80 ? `${titleSource.slice(0, 77)}...` : titleSource;
  return `Branch from ${shortened}`;
}

function formatAttachmentSize(sizeBytes: number) {
  if (sizeBytes < 1024) {
    return `${sizeBytes} B`;
  }

  const sizeKilobytes = sizeBytes / 1024;
  if (sizeKilobytes < 1024) {
    return `${Number.isInteger(sizeKilobytes) ? sizeKilobytes : sizeKilobytes.toFixed(1)} KB`;
  }

  const sizeMegabytes = sizeKilobytes / 1024;
  return `${Number.isInteger(sizeMegabytes) ? sizeMegabytes : sizeMegabytes.toFixed(1)} MB`;
}

function attachmentFromFile(file: File): MessageAttachment {
  return {
    contentType: file.type || 'application/octet-stream',
    id: `attachment-${file.name}-${file.size}`,
    name: file.name,
    sizeBytes: file.size,
    type: file.type.startsWith('image/') ? 'image' : 'file'
  };
}

const emptyPersonaDraft: PersonaDraft = {
  constraints: '',
  name: '',
  openingMessage: '',
  role: '',
  style: '',
  suggestedQuestionsText: '',
  tone: ''
};

function personaDraftFromPersona(persona: PersonaSummary): PersonaDraft {
  return {
    constraints: persona.constraints ?? '',
    name: persona.name,
    openingMessage: persona.openingMessage ?? '',
    role: persona.role ?? '',
    style: persona.style ?? '',
    suggestedQuestionsText: (persona.suggestedQuestions ?? []).join('\n'),
    tone: persona.tone ?? ''
  };
}

function personaPayloadFromDraft(draft: PersonaDraft): PersonaRequest {
  return {
    constraints: draft.constraints.trim(),
    name: draft.name.trim(),
    openingMessage: draft.openingMessage.trim(),
    role: draft.role.trim(),
    style: draft.style.trim(),
    suggestedQuestions: draft.suggestedQuestionsText
      .split('\n')
      .map((question) => question.trim())
      .filter(Boolean),
    tone: draft.tone.trim()
  };
}

function parseInlineMarkdown(content: string): InlineMarkdownToken[] {
  const tokens: InlineMarkdownToken[] = [];
  const inlinePattern = /(`[^`]+`|\*\*[^*]+\*\*)/g;
  let cursor = 0;
  let match: RegExpExecArray | null;

  while ((match = inlinePattern.exec(content)) !== null) {
    if (match.index > cursor) {
      tokens.push({ content: content.slice(cursor, match.index), type: 'text' });
    }

    const marker = match[0];
    if (marker.startsWith('`')) {
      tokens.push({ content: marker.slice(1, -1), type: 'code' });
    } else {
      tokens.push({ content: marker.slice(2, -2), type: 'strong' });
    }
    cursor = match.index + marker.length;
  }

  if (cursor < content.length) {
    tokens.push({ content: content.slice(cursor), type: 'text' });
  }

  return tokens.length > 0 ? tokens : [{ content, type: 'text' }];
}

function parseMarkdownBlocks(content: string): MessageContentBlock[] {
  const blocks: MessageContentBlock[] = [];
  const markdownLines: string[] = [];
  const lines = content.replace(/\r\n/g, '\n').split('\n');
  let codeLines: string[] = [];
  let codeLanguage = '';
  let isInCodeBlock = false;

  const pushMarkdownBlocks = () => {
    let paragraphLines: string[] = [];
    let listItems: string[] = [];

    const pushParagraph = () => {
      if (paragraphLines.length === 0) {
        return;
      }
      blocks.push({ content: paragraphLines.join(' '), type: 'paragraph' });
      paragraphLines = [];
    };
    const pushList = () => {
      if (listItems.length === 0) {
        return;
      }
      blocks.push({ items: listItems, type: 'list' });
      listItems = [];
    };

    markdownLines.forEach((line) => {
      const trimmedLine = line.trim();
      if (trimmedLine === '') {
        pushParagraph();
        pushList();
        return;
      }

      const headingMatch = /^(#{1,3})\s+(.+)$/.exec(trimmedLine);
      if (headingMatch) {
        pushParagraph();
        pushList();
        blocks.push({ content: headingMatch[2], type: 'heading' });
        return;
      }

      const listMatch = /^[-*]\s+(.+)$/.exec(trimmedLine);
      if (listMatch) {
        pushParagraph();
        listItems.push(listMatch[1]);
        return;
      }

      pushList();
      paragraphLines.push(trimmedLine);
    });

    pushParagraph();
    pushList();
    markdownLines.length = 0;
  };

  lines.forEach((line) => {
    const fenceMatch = /^```([A-Za-z0-9_-]+)?\s*$/.exec(line.trim());
    if (fenceMatch && !isInCodeBlock) {
      pushMarkdownBlocks();
      isInCodeBlock = true;
      codeLanguage = fenceMatch[1] ?? '';
      codeLines = [];
      return;
    }

    if (/^```\s*$/.test(line.trim()) && isInCodeBlock) {
      blocks.push({
        content: codeLines.join('\n').replace(/\n$/, ''),
        language: codeLanguage,
        type: 'code'
      });
      isInCodeBlock = false;
      codeLanguage = '';
      codeLines = [];
      return;
    }

    if (isInCodeBlock) {
      codeLines.push(line);
      return;
    }

    markdownLines.push(line);
  });

  if (isInCodeBlock) {
    blocks.push({
      content: codeLines.join('\n').replace(/\n$/, ''),
      language: codeLanguage,
      type: 'code'
    });
  }
  pushMarkdownBlocks();

  return blocks.length > 0 ? blocks : [{ content, type: 'paragraph' }];
}

function renderInlineMarkdown(content: string) {
  return parseInlineMarkdown(content).map((token, index) => {
    if (token.type === 'code') {
      return (
        <code className="rounded bg-[#f6f1e6] px-1 py-0.5 font-mono text-[0.95em]" key={`${token.type}-${index}`}>
          {token.content}
        </code>
      );
    }
    if (token.type === 'strong') {
      return <strong key={`${token.type}-${index}`}>{token.content}</strong>;
    }

    return token.content;
  });
}

type CodeBlockFigureProps = {
  content: string;
  language: string;
};

function CodeBlockFigure({ content, language }: CodeBlockFigureProps) {
  const [isCopied, setIsCopied] = useState(false);
  const languageLabel = language || 'code';

  useEffect(() => {
    if (!isCopied) {
      return undefined;
    }

    const resetTimer = window.setTimeout(() => setIsCopied(false), 1600);
    return () => window.clearTimeout(resetTimer);
  }, [isCopied]);

  const copyCode = async () => {
    if (!navigator.clipboard) {
      return;
    }
    await navigator.clipboard.writeText(content);
    setIsCopied(true);
  };

  return (
    <figure className="overflow-hidden rounded-lg border border-[#d7d2c4] bg-[#181611]">
      <figcaption className="flex items-center justify-between gap-3 border-b border-white/10 px-3 py-2 font-mono text-xs font-semibold uppercase text-[#d9d2c3]">
        <span>{languageLabel}</span>
        <button
          aria-label={`${isCopied ? 'Copied' : 'Copy'} code block ${languageLabel}`}
          className="rounded border border-white/15 px-2 py-1 text-[11px] font-semibold normal-case text-[#f7f4ea] hover:bg-white/10"
          onClick={() => void copyCode()}
          type="button"
        >
          {isCopied ? 'Copied' : 'Copy'}
        </button>
      </figcaption>
      <pre className="max-h-80 overflow-auto p-3 text-sm leading-6 text-[#f7f4ea]">
        <code className={language ? `language-${language}` : undefined}>{content}</code>
      </pre>
    </figure>
  );
}

function renderMessageContent(content: string) {
  return (
    <div className="space-y-3">
      {parseMarkdownBlocks(content).map((block, index) => {
        if (block.type === 'code') {
          return <CodeBlockFigure content={block.content} key={`code-${index}`} language={block.language} />;
        }
        if (block.type === 'heading') {
          return (
            <h3 className="text-base font-semibold text-[#181611]" key={`heading-${index}`}>
              {renderInlineMarkdown(block.content)}
            </h3>
          );
        }
        if (block.type === 'list') {
          return (
            <ul className="list-disc space-y-1 pl-5" key={`list-${index}`}>
              {block.items.map((item, itemIndex) => (
                <li key={`${item}-${itemIndex}`}>{renderInlineMarkdown(item)}</li>
              ))}
            </ul>
          );
        }

        return <p key={`paragraph-${index}`}>{renderInlineMarkdown(block.content)}</p>;
      })}
    </div>
  );
}

function formatCitationScore(score: number) {
  return Number.isInteger(score) ? String(score) : score.toFixed(2).replace(/0$/, '').replace(/\.0$/, '');
}

function citationMetadataItems(citation: KnowledgeCitation) {
  const items: string[] = [];
  if (citation.pageNumber !== undefined && citation.pageNumber > 0) {
    items.push(`Page ${citation.pageNumber}`);
  }
  if (citation.documentVersion?.trim()) {
    items.push(`Version ${citation.documentVersion.trim()}`);
  }
  if (citation.chunkIndex !== undefined && citation.chunkIndex >= 0) {
    items.push(`Chunk ${citation.chunkIndex + 1}`);
  } else if (citation.chunkId?.trim()) {
    items.push(`Chunk ${citation.chunkId.trim()}`);
  }
  if (citation.highlightPositions && citation.highlightPositions.length > 0) {
    const highlights = citation.highlightPositions
      .map((position) => `${position.start}-${position.end}`)
      .join(', ');
    items.push(`Highlights ${highlights}`);
  }
  return items;
}

type KnowledgeCitationListProps = {
  citations: KnowledgeCitation[];
  messageId: string;
};

function KnowledgeCitationList({ citations, messageId }: KnowledgeCitationListProps) {
  if (citations.length === 0) {
    return null;
  }

  return (
    <ul aria-label={`Knowledge citations for message ${messageId}`} className="mt-3 space-y-2">
      {citations.map((citation, index) => {
        const citationTitle = citation.documentTitle?.trim() || citation.knowledgeBaseName?.trim() || `Citation ${index + 1}`;
        const citationKey = `${citation.knowledgeBaseId ?? messageId}-${citation.documentTitle ?? index}-${citation.snippet}`;
        const metadataItems = citationMetadataItems(citation);

        return (
          <li className="rounded-md border border-[#d7d2c4] bg-[#fbfaf6] p-3 text-sm text-[#4d463a]" key={citationKey}>
            <div className="flex flex-wrap items-start justify-between gap-2">
              <div>
                <p className="font-semibold text-[#2f2a21]">{citationTitle}</p>
                {citation.knowledgeBaseName ? <p className="text-xs font-medium text-[#625b4f]">{citation.knowledgeBaseName}</p> : null}
              </div>
              {typeof citation.score === 'number' ? (
                <span className="rounded border border-[#d7d2c4] px-2 py-0.5 text-xs font-semibold text-[#625b4f]">
                  {`Score ${formatCitationScore(citation.score)}`}
                </span>
              ) : null}
            </div>
            {metadataItems.length > 0 ? (
              <div aria-label={`Citation metadata for ${citationTitle}`} className="mt-3 flex flex-wrap gap-1.5">
                {metadataItems.map((item) => (
                  <span className="rounded border border-[#d7d2c4] bg-white px-2 py-0.5 text-xs font-medium text-[#625b4f]" key={item}>
                    {item}
                  </span>
                ))}
              </div>
            ) : null}
            <p className="mt-2 leading-6">{citation.snippet}</p>
            {citation.sourceUrl ? (
              <a
                aria-label={`Open citation source for ${citationTitle}`}
                className="mt-2 inline-flex text-sm font-semibold text-[#1a614f] underline-offset-4 hover:underline"
                href={citation.sourceUrl}
                rel="noreferrer"
                target="_blank"
              >
                Source
              </a>
            ) : null}
          </li>
        );
      })}
    </ul>
  );
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
  const [messageAttachments, setMessageAttachments] = useState<MessageAttachment[]>([]);
  const [messageDraft, setMessageDraft] = useState('');
  const [messages, setMessages] = useState<ConversationMessage[]>([]);
  const [editingMessageDraft, setEditingMessageDraft] = useState('');
  const [editingMessageId, setEditingMessageId] = useState<string | null>(null);
  const [conversationShareExpiration, setConversationShareExpiration] = useState('');
  const [conversationShareUrl, setConversationShareUrl] = useState('');
  const [messageShareExpirations, setMessageShareExpirations] = useState<Record<string, string>>({});
  const [messageShares, setMessageShares] = useState<Record<string, string>>({});
  const [modelOptions, setModelOptions] = useState<Array<{ id: string; label: string }>>([]);
  const [personaOptions, setPersonaOptions] = useState<PersonaSummary[]>([]);
  const [personaDraft, setPersonaDraft] = useState<PersonaDraft>(emptyPersonaDraft);
  const [editingPersonaId, setEditingPersonaId] = useState<string | null>(null);
  const [conversationSearch, setConversationSearch] = useState('');
  const [conversationFilter, setConversationFilter] = useState<ConversationRailFilter>('all');
  const [isConversationRailOpen, setIsConversationRailOpen] = useState(false);
  const [authorizationScope, setAuthorizationScope] = useState('workspace_tools');
  const [allowedTools, setAllowedTools] = useState('');
  const [blockedTools, setBlockedTools] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<ActionError | null>(null);
  const [markdownExportUrl, setMarkdownExportUrl] = useState('');
  const [isCreatingConversation, setIsCreatingConversation] = useState(false);
  const [isExportingMarkdown, setIsExportingMarkdown] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [isSavingPersona, setIsSavingPersona] = useState(false);
  const [deletingPersonaId, setDeletingPersonaId] = useState<string | null>(null);
  const [isPreparingHandoff, setIsPreparingHandoff] = useState(false);
  const [isSending, setIsSending] = useState(false);
  const [isStartingSolo, setIsStartingSolo] = useState(false);
  const [isUpdatingConversationSettings, setIsUpdatingConversationSettings] = useState(false);
  const [isUpdatingKnowledgeBinding, setIsUpdatingKnowledgeBinding] = useState(false);
  const [reloadToken, setReloadToken] = useState(0);
  const [settingsSavedForSend, setSettingsSavedForSend] = useState(false);
  const [selectedKnowledgeBaseIds, setSelectedKnowledgeBaseIds] = useState<string[]>([]);
  const chatReturnPath = conversationId ? `/chat/${conversationId}` : '/chat';
  const normalizedConversationSearch = conversationSearch.trim().toLowerCase();
  const filteredConversations = useMemo(() => {
    const visibleByFilter = conversations.filter((conversation) => {
      const isArchived = Boolean(conversation.archivedAt);
      if (conversationFilter === 'archived') {
        return isArchived;
      }
      if (isArchived) {
        return false;
      }
      return conversationFilter === 'starred' ? conversation.hasBookmarkedMessages : true;
    });
    if (normalizedConversationSearch === '') {
      return visibleByFilter;
    }

    return visibleByFilter.filter((conversation) => conversation.title.toLowerCase().includes(normalizedConversationSearch));
  }, [conversationFilter, conversations, normalizedConversationSearch]);

  useEffect(() => {
    let cancelled = false;

    const loadChatWorkspace = async () => {
      setIsLoading(true);
      setError(null);
      try {
        if (conversationId) {
          const [nextConversations, nextModels, nextPersonas, nextKnowledgeBases, nextMessages, nextConversationConfig] = await Promise.all([
            chatApi.listConversations(),
            chatApi.listModels(),
            chatApi.listPersonas(),
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
          setEditingMessageDraft('');
          setEditingMessageId(null);
          setConversationShareExpiration('');
          setConversationShareUrl('');
          setMessageShareExpirations({});
          setMessageShares({});
          setModelOptions(nextModels);
          setPersonaOptions(nextPersonas);
          setPersonaDraft(emptyPersonaDraft);
          setEditingPersonaId(null);
          setMessageDraft('');
          setMessageAttachments([]);
          setMarkdownExportUrl('');
          setSettingsSavedForSend(false);
          setSelectedKnowledgeBaseIds(nextConversationConfig.knowledgeBaseIds);
          setError(null);
          return;
        }

        const [nextConversations, nextModels, nextPersonas, nextKnowledgeBases] = await Promise.all([
          chatApi.listConversations(),
          chatApi.listModels(),
          chatApi.listPersonas(),
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
        setEditingMessageDraft('');
        setEditingMessageId(null);
        setConversationShareExpiration('');
        setConversationShareUrl('');
        setMessageShareExpirations({});
        setMessageShares({});
        setModelOptions(nextModels);
        setPersonaOptions(nextPersonas);
        setPersonaDraft(emptyPersonaDraft);
        setEditingPersonaId(null);
        setMessageDraft('');
        setMessageAttachments([]);
        setMarkdownExportUrl('');
        setSettingsSavedForSend(false);
        setSelectedKnowledgeBaseIds([]);
        setError(null);
      } catch (caughtError) {
        if (!cancelled) {
          setConversations([]);
          setConversationConfig(null);
          setHandoffDraft(null);
          setKnowledgeBases([]);
          setMessages([]);
          setEditingMessageDraft('');
          setEditingMessageId(null);
          setConversationShareExpiration('');
          setConversationShareUrl('');
          setMessageShareExpirations({});
          setMessageShares({});
          setModelOptions([]);
          setPersonaOptions([]);
          setPersonaDraft(emptyPersonaDraft);
          setEditingPersonaId(null);
          setMessageDraft('');
          setMessageAttachments([]);
          setMarkdownExportUrl('');
          setSettingsSavedForSend(false);
          setSelectedKnowledgeBaseIds([]);
          setError(getErrorMessage(caughtError, 'The chat workspace could not be loaded.'));
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    };

    void loadChatWorkspace();

    return () => {
      cancelled = true;
    };
  }, [chatApi, conversationId, knowledgeApi, reloadToken]);

  const handleCreateConversation = async () => {
    setActionError(null);
    setIsCreatingConversation(true);
    try {
      const conversation = await chatApi.createConversation({ title: 'New conversation' });
      setConversations((current) => [conversation, ...current]);
      setIsConversationRailOpen(false);
      navigate(`/chat/${conversation.id}`);
    } catch (caughtError) {
      setActionError(toActionError('Unable to create conversation.', caughtError));
    } finally {
      setIsCreatingConversation(false);
    }
  };

  const openConversation = (nextConversationId: string) => {
    setIsConversationRailOpen(false);
    navigate(`/chat/${nextConversationId}`);
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
      personaId: conversationConfig.personaId ?? '',
      systemPromptOverride: conversationConfig.systemPromptOverride,
      temperature: conversationConfig.temperature,
      toolsEnabled: conversationConfig.toolsEnabled
    };
    setActionError(null);
    setIsUpdatingKnowledgeBinding(true);
    try {
      const savedConfig = await chatApi.updateConversationConfig(conversationId, nextConfig);

      setConversationConfig(savedConfig);
      setSelectedKnowledgeBaseIds(savedConfig.knowledgeBaseIds);
    } catch (caughtError) {
      setActionError(toActionError('Unable to update knowledge binding.', caughtError));
    } finally {
      setIsUpdatingKnowledgeBinding(false);
    }
  };

  const updateConversationConfigDraft = (patch: Partial<UpdateConversationConfigRequest>) => {
    setConversationConfig((current) => {
      if (current === null) {
        return current;
      }

      return {
        ...current,
        ...patch
      };
    });
  };

  const updatePersonaDraft = (patch: Partial<PersonaDraft>) => {
    setPersonaDraft((current) => ({
      ...current,
      ...patch
    }));
  };

  const startEditingPersona = (persona: PersonaSummary) => {
    setActionError(null);
    setEditingPersonaId(persona.id);
    setPersonaDraft(personaDraftFromPersona(persona));
  };

  const cancelPersonaEditing = () => {
    setActionError(null);
    setEditingPersonaId(null);
    setPersonaDraft(emptyPersonaDraft);
  };

  const conversationOverrides = () => {
    if (conversationConfig === null) {
      return undefined;
    }

    return {
      maxOutputTokens: conversationConfig.maxOutputTokens,
      modelId: conversationConfig.modelId,
      systemPromptOverride: conversationConfig.systemPromptOverride,
      temperature: conversationConfig.temperature,
      toolsEnabled: conversationConfig.toolsEnabled
    };
  };

  const conversationConfigRequest = (
    config: ConversationConfig,
    patch: Partial<UpdateConversationConfigRequest> = {}
  ): UpdateConversationConfigRequest => ({
    knowledgeBaseIds: config.knowledgeBaseIds,
    maxOutputTokens: config.maxOutputTokens,
    modelId: config.modelId,
    personaId: config.personaId ?? '',
    systemPromptOverride: config.systemPromptOverride,
    temperature: config.temperature,
    toolsEnabled: config.toolsEnabled,
    ...patch
  });

  const saveConversationSettings = async () => {
    if (!conversationId || conversationConfig === null) {
      return;
    }

    const nextConfig = conversationConfigRequest(conversationConfig);
    setActionError(null);
    setIsUpdatingConversationSettings(true);
    try {
      const savedConfig = await chatApi.updateConversationConfig(conversationId, nextConfig);
      setConversationConfig(savedConfig);
      setSelectedKnowledgeBaseIds(savedConfig.knowledgeBaseIds);
      setSettingsSavedForSend(true);
    } catch (caughtError) {
      setActionError(toActionError('Unable to save conversation settings.', caughtError));
    } finally {
      setIsUpdatingConversationSettings(false);
    }
  };

  const savePersona = async () => {
    const payload = personaPayloadFromDraft(personaDraft);

    if (payload.name === '') {
      setActionError({ title: 'Persona name is required.' });
      return;
    }

    setActionError(null);
    setIsSavingPersona(true);
    try {
      if (editingPersonaId !== null) {
        const updatedPersona = await chatApi.updatePersona(editingPersonaId, payload);
        setPersonaOptions((currentPersonas) =>
          currentPersonas.map((persona) => (persona.id === updatedPersona.id ? updatedPersona : persona))
        );
        setEditingPersonaId(null);
        setPersonaDraft(emptyPersonaDraft);
        return;
      }

      const createdPersona = await chatApi.createPersona(payload);
      setPersonaOptions((currentPersonas) => [createdPersona, ...currentPersonas]);
      setConversationConfig((currentConfig) =>
        currentConfig === null
          ? currentConfig
          : {
              ...currentConfig,
              personaId: createdPersona.id
            }
      );
      setPersonaDraft(emptyPersonaDraft);
    } catch (caughtError) {
      setActionError(toActionError(editingPersonaId ? 'Unable to update persona.' : 'Unable to create persona.', caughtError));
    } finally {
      setIsSavingPersona(false);
    }
  };

  const deletePersona = async (persona: PersonaSummary) => {
    setActionError(null);
    setDeletingPersonaId(persona.id);
    try {
      await chatApi.deletePersona(persona.id);
      setPersonaOptions((currentPersonas) => currentPersonas.filter((currentPersona) => currentPersona.id !== persona.id));
      if (editingPersonaId === persona.id) {
        setEditingPersonaId(null);
        setPersonaDraft(emptyPersonaDraft);
      }
      if (conversationId && conversationConfig?.personaId === persona.id) {
        const savedConfig = await chatApi.updateConversationConfig(conversationId, conversationConfigRequest(conversationConfig, { personaId: '' }));
        setConversationConfig(savedConfig);
        setSelectedKnowledgeBaseIds(savedConfig.knowledgeBaseIds);
      } else {
        setConversationConfig((currentConfig) =>
          currentConfig?.personaId === persona.id
            ? {
                ...currentConfig,
                personaId: ''
              }
            : currentConfig
        );
      }
    } catch (caughtError) {
      setActionError(toActionError('Unable to delete persona.', caughtError));
    } finally {
      setDeletingPersonaId(null);
    }
  };

  const openSoloHandoff = async () => {
    if (!conversationId) {
      return;
    }

    setActionError(null);
    setIsPreparingHandoff(true);
    try {
      const draft = await chatApi.convertConversationToTask(conversationId);
      setAuthorizationScope('workspace_tools');
      setAllowedTools('');
      setBlockedTools('');
      setSelectedKnowledgeBaseIds(draft.relatedKnowledgeBaseIds);
      setHandoffDraft(draft);
    } catch (caughtError) {
      setHandoffDraft(null);
      setActionError(toActionError('Unable to prepare SOLO handoff.', caughtError));
    } finally {
      setIsPreparingHandoff(false);
    }
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

    setActionError(null);
    setIsStartingSolo(true);
    try {
      const createdTask: TaskSummary = await tasksApi.createTask(createTaskPayload);
      await tasksApi.startTask(createdTask.id);
      navigate(`/solo?taskId=${createdTask.id}&returnTo=${encodeURIComponent(chatReturnPath)}`);
    } catch (caughtError) {
      setActionError(toActionError('Unable to start SOLO task.', caughtError));
    } finally {
      setIsStartingSolo(false);
    }
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
    const attachments = messageAttachments;

    if (!conversationId || (trimmedContent === '' && attachments.length === 0)) {
      return;
    }

    const userMessageId = `pending-user-${Date.now()}`;
    const assistantMessageId = `pending-assistant-${Date.now()}`;
    const optimisticUserMessage: ConversationMessage = {
      attachments: attachments.length > 0 ? attachments : undefined,
      content: trimmedContent,
      id: userMessageId,
      role: 'user'
    };
    const optimisticAssistantMessage: ConversationMessage = {
      content: '',
      id: assistantMessageId,
      role: 'assistant'
    };
    const payload = {
      ...(attachments.length > 0 ? { attachments } : {}),
      content: trimmedContent,
      ...(settingsSavedForSend ? { overrides: conversationOverrides() } : {})
    };

    setActionError(null);
    setIsSending(true);
    setMessages((currentMessages) => [...currentMessages, optimisticUserMessage, optimisticAssistantMessage]);
    try {
      await chatApi.sendMessageStream(conversationId, payload, {
        onChunk: (chunk) => {
          setMessages((currentMessages) =>
            currentMessages.map((currentMessage) =>
              currentMessage.id === assistantMessageId
                ? { ...currentMessage, content: `${currentMessage.content}${chunk}` }
                : currentMessage
            )
          );
        }
      });
      const nextMessages = await chatApi.listMessages(conversationId);
      setMessages(nextMessages);
      setMessageDraft('');
      setMessageAttachments([]);
    } catch (caughtError) {
      setMessages((currentMessages) =>
        currentMessages.filter(
          (currentMessage) => currentMessage.id !== userMessageId && currentMessage.id !== assistantMessageId
        )
      );
      setActionError(toActionError('Unable to send message.', caughtError));
    } finally {
      setIsSending(false);
    }
  };

  const updateMessageAttachments = (fileList: FileList | null) => {
    setMessageAttachments(Array.from(fileList ?? []).map(attachmentFromFile));
  };

  const handleRetryFromMessage = async (message: ConversationMessage) => {
    if (!conversationId || message.content.trim() === '') {
      return;
    }

    setActionError(null);
    setIsSending(true);
    try {
      const nextMessages = await chatApi.sendMessage(conversationId, {
        content: message.content,
        overrides: conversationOverrides()
      });
      setMessages(nextMessages);
    } catch (caughtError) {
      setActionError(toActionError('Unable to retry message.', caughtError));
    } finally {
      setIsSending(false);
    }
  };

  const handleRegenerateAssistantMessage = async (message: ConversationMessage) => {
    const messageIndex = messages.findIndex((currentMessage) => currentMessage.id === message.id);
    const previousUserMessage =
      messageIndex >= 0
        ? [...messages.slice(0, messageIndex)].reverse().find((currentMessage) => currentMessage.role === 'user')
        : undefined;

    if (!previousUserMessage) {
      setActionError({ title: 'No user message is available to regenerate this response.' });
      return;
    }

    await handleRetryFromMessage(previousUserMessage);
  };

  const copyMessage = async (message: ConversationMessage) => {
    setActionError(null);
    try {
      await navigator.clipboard?.writeText(message.content);
    } catch (caughtError) {
      setActionError(toActionError('Unable to copy message.', caughtError));
    }
  };

  const replaceMessage = (nextMessage: ConversationMessage) => {
    setMessages((currentMessages) =>
      currentMessages.map((currentMessage) => (currentMessage.id === nextMessage.id ? nextMessage : currentMessage))
    );
  };

  const startEditingMessage = (message: ConversationMessage) => {
    setActionError(null);
    setEditingMessageDraft(message.content);
    setEditingMessageId(message.id);
  };

  const cancelEditingMessage = () => {
    setEditingMessageDraft('');
    setEditingMessageId(null);
  };

  const saveMessageEdit = async (message: ConversationMessage) => {
    const trimmedContent = editingMessageDraft.trim();

    if (!conversationId || trimmedContent === '') {
      return;
    }

    setActionError(null);
    try {
      const updatedMessage = await chatApi.updateMessage(conversationId, message.id, { content: trimmedContent });
      replaceMessage(updatedMessage);
      cancelEditingMessage();
    } catch (caughtError) {
      setActionError(toActionError('Unable to edit message.', caughtError));
    }
  };

  const deleteMessage = async (message: ConversationMessage) => {
    if (!conversationId) {
      return;
    }

    setActionError(null);
    try {
      await chatApi.deleteMessage(conversationId, message.id);
      setMessages((currentMessages) => currentMessages.filter((currentMessage) => currentMessage.id !== message.id));
      setMessageShares((currentShares) => {
        const { [message.id]: _removedShare, ...nextShares } = currentShares;
        return nextShares;
      });
      setMessageShareExpirations((currentExpirations) => {
        const { [message.id]: _removedExpiration, ...nextExpirations } = currentExpirations;
        return nextExpirations;
      });
      if (editingMessageId === message.id) {
        cancelEditingMessage();
      }
    } catch (caughtError) {
      setActionError(toActionError('Unable to delete message.', caughtError));
    }
  };

  const toggleMessageBookmark = async (message: ConversationMessage) => {
    if (!conversationId) {
      return;
    }

    const nextBookmarked = !message.bookmarked;
    setActionError(null);
    try {
      const updatedMessage = await chatApi.bookmarkMessage(conversationId, message.id, { bookmarked: nextBookmarked });
      replaceMessage(updatedMessage);
    } catch (caughtError) {
      setActionError(toActionError('Unable to update message bookmark.', caughtError));
    }
  };

  const exportMarkdown = async () => {
    if (!conversationId) {
      return;
    }

    setActionError(null);
    setIsExportingMarkdown(true);
    try {
      const markdown = await chatApi.exportConversationMarkdown(conversationId);
      const blob = new Blob([markdown], { type: 'text/markdown;charset=utf-8' });
      setMarkdownExportUrl(URL.createObjectURL(blob));
    } catch (caughtError) {
      setActionError(toActionError('Unable to export conversation.', caughtError));
    } finally {
      setIsExportingMarkdown(false);
    }
  };

  const shareMessage = async (message: ConversationMessage) => {
    if (!conversationId) {
      return;
    }

    setActionError(null);
    try {
      const expiresAt = messageShareExpirations[message.id]?.trim();
      const share = await chatApi.createMessageShare(
        conversationId,
        message.id,
        expiresAt ? { expiresAt } : undefined
      );
      setMessageShares((currentShares) => ({
        ...currentShares,
        [message.id]: share.url ?? share.id ?? `Share created for message ${message.id}`
      }));
    } catch (caughtError) {
      setActionError(toActionError('Unable to share message.', caughtError));
    }
  };

  const shareConversation = async () => {
    if (!conversationId) {
      return;
    }

    setActionError(null);
    try {
      const expiresAt = conversationShareExpiration.trim();
      const share = await chatApi.createConversationShare(conversationId, expiresAt ? { expiresAt } : undefined);
      setConversationShareUrl(share.url ?? share.id ?? `Share created for conversation ${conversationId}`);
    } catch (caughtError) {
      setActionError(toActionError('Unable to share conversation.', caughtError));
    }
  };

  const shareConversationFromMessage = async (message: ConversationMessage) => {
    if (!conversationId) {
      return;
    }

    setActionError(null);
    try {
      const expiresAt = conversationShareExpiration.trim();
      const share = await chatApi.createConversationShare(conversationId, {
        ...(expiresAt ? { expiresAt } : {}),
        startMessageId: message.id
      });
      setConversationShareUrl(share.url ?? share.id ?? `Share created from message ${message.id}`);
    } catch (caughtError) {
      setActionError(toActionError('Unable to share conversation range.', caughtError));
    }
  };

  const handleForkFromMessage = async (message: ConversationMessage) => {
    if (!conversationId) {
      return;
    }

    setActionError(null);
    try {
      const fork = await chatApi.forkConversation(conversationId, {
        messageId: message.id,
        title: forkTitleForMessage(message)
      });
      navigate(`/chat/${fork.id}`);
    } catch (caughtError) {
      setActionError(toActionError('Unable to fork conversation.', caughtError));
    }
  };

  const workspaceStatus = (
    <>
      {isLoading ? <p role="status">Loading chat workspace...</p> : null}
      {error !== null ? (
        <section aria-label="Chat workspace error" role="alert">
          <p>Unable to load chat workspace.</p>
          <p>{error}</p>
          <button disabled={isLoading} onClick={() => setReloadToken((current) => current + 1)} type="button">
            Retry chat workspace
          </button>
        </section>
      ) : null}
      {actionError !== null ? (
        <section aria-label="Chat action error" role="alert">
          <p>{actionError.title}</p>
          {actionError.message ? <p>{actionError.message}</p> : null}
        </section>
      ) : null}
    </>
  );

  const conversationRailEmptyState =
    conversationFilter === 'starred'
      ? 'No starred conversations yet.'
      : conversationFilter === 'archived'
        ? 'No archived conversations yet.'
        : normalizedConversationSearch === ''
          ? 'No conversations yet.'
          : 'No conversations match your search.';

  const conversationRail = (
    <nav
      aria-label="Conversation rail"
      className={`${
        isConversationRailOpen ? 'fixed inset-y-0 left-0 z-30 block w-[min(20rem,86vw)] shadow-xl md:static md:shadow-none' : 'hidden'
      } min-h-full shrink-0 overflow-y-auto border-r border-[#d7d2c4] bg-[#f6f1e8] px-3 py-4 md:block md:w-[200px] lg:w-[300px]`}
    >
      <div className="flex items-center justify-between gap-3">
        <h2 className="text-sm font-semibold text-[#2f2a21]">Conversations</h2>
        <div className="flex items-center gap-2">
          <button
            aria-label="New conversation"
            className="inline-flex min-h-9 min-w-9 items-center justify-center rounded-md border border-[#1a614f] text-[#1a614f] disabled:opacity-60"
            disabled={isCreatingConversation}
            onClick={() => void handleCreateConversation()}
            title="New conversation"
            type="button"
          >
            <RiAddLine className="size-4" aria-hidden="true" />
          </button>
          <button
            aria-label="Close conversations"
            className="inline-flex min-h-9 min-w-9 items-center justify-center rounded-md border border-[#d7d2c4] text-[#625b4f] md:hidden"
            onClick={() => setIsConversationRailOpen(false)}
            title="Close conversations"
            type="button"
          >
            <RiCloseLine className="size-4" aria-hidden="true" />
          </button>
        </div>
      </div>
      <label className="mt-4 block text-sm font-medium text-[#4d463a]">
        Search conversations
        <input
          className="mt-2 w-full rounded-md border border-[#d7d2c4] bg-white px-3 py-2 text-sm text-[#2f2a21] outline-none focus:border-[#1a614f]"
          onChange={(event) => setConversationSearch(event.target.value)}
          type="search"
          value={conversationSearch}
        />
      </label>
      <div aria-label="Conversation filters" className="mt-3 grid grid-cols-3 rounded-md border border-[#d7d2c4] bg-white p-1">
        {[
          { label: 'All', value: 'all' },
          { label: 'Starred', value: 'starred' },
          { label: 'Archived', value: 'archived' }
        ].map((filter) => (
          <button
            aria-pressed={conversationFilter === filter.value}
            className={`min-h-8 rounded px-2 text-xs font-semibold ${
              conversationFilter === filter.value ? 'bg-[#1a614f] text-white' : 'text-[#625b4f] hover:bg-[#f6f1e8]'
            }`}
            key={filter.value}
            onClick={() => setConversationFilter(filter.value as ConversationRailFilter)}
            type="button"
          >
            {filter.value === 'starred'
              ? 'Starred conversations'
              : filter.value === 'archived'
                ? 'Archived conversations'
                : 'All conversations'}
          </button>
        ))}
      </div>
      <div className="mt-4">
        {filteredConversations.length > 0 ? (
          <ul className="space-y-1">
            {filteredConversations.map((conversation) => (
              <li key={conversation.id}>
                <a
                  aria-current={conversation.id === conversationId ? 'page' : undefined}
                  aria-label={`Open conversation ${conversation.title}`}
                  className={`block min-h-10 truncate rounded-md px-3 py-2 text-sm font-medium ${
                    conversation.id === conversationId
                      ? 'bg-[#e1ece7] text-[#174739]'
                      : 'text-[#3b352c] hover:bg-white'
                  }`}
                  href={`/chat/${conversation.id}`}
                  onClick={(event) => {
                    event.preventDefault();
                    openConversation(conversation.id);
                  }}
                >
                  {conversation.title}
                </a>
              </li>
            ))}
          </ul>
        ) : (
          <p className="text-sm leading-6 text-[#625b4f]">{conversationRailEmptyState}</p>
        )}
      </div>
    </nav>
  );

  return (
    <section className="min-h-full">
      <div className="flex min-h-[calc(100vh-8rem)]">
        {conversationRail}
        <main className="min-w-0 flex-1 px-6 py-5">
          <button
            aria-expanded={isConversationRailOpen}
            aria-label="Conversations"
            className="mb-4 inline-flex min-h-10 items-center gap-2 rounded-md border border-[#d7d2c4] px-3 text-sm font-semibold text-[#2f2a21] md:hidden"
            onClick={() => setIsConversationRailOpen((current) => !current)}
            type="button"
          >
            <RiMenuLine className="size-4" aria-hidden="true" />
            Conversations
          </button>
          <h1>Chat workspace</h1>
          {workspaceStatus}
          {!authState.preferences?.onboardingCompleted ? (
            <section>
              <p>Finish setup to lock in your default workspace preferences.</p>
              <button onClick={() => navigate('/onboarding')} type="button">
                Complete setup
              </button>
            </section>
          ) : null}
          {conversationId ? (
            <>
              <section>
                <h2>Conversation transcript</h2>
                <div>
                  <button disabled={isExportingMarkdown} onClick={() => void exportMarkdown()} type="button">
                    Export Markdown
                  </button>
                  <label>
                    Conversation share expiration
                    <input
                      onChange={(event) => setConversationShareExpiration(event.target.value)}
                      placeholder="2026-06-05T12:00:00Z"
                      type="text"
                      value={conversationShareExpiration}
                    />
                  </label>
                  <button onClick={() => void shareConversation()} type="button">
                    Share conversation
                  </button>
                  {conversationShareUrl ? <p>{conversationShareUrl}</p> : null}
                  {markdownExportUrl ? (
                    <a download={`${conversationId}.md`} href={markdownExportUrl}>
                      Download Markdown export
                    </a>
                  ) : null}
                </div>
                {messages.length > 0 ? (
                  <ul>
                    {messages.map((message) => (
                      <li key={message.id}>
                        {editingMessageId === message.id ? (
                          <div>
                            <label>
                              {`Edit message ${message.id} content`}
                              <textarea
                                onChange={(event) => setEditingMessageDraft(event.target.value)}
                                value={editingMessageDraft}
                              />
                            </label>
                            <button onClick={() => void saveMessageEdit(message)} type="button">
                              {`Save edit for message ${message.id}`}
                            </button>
                            <button onClick={cancelEditingMessage} type="button">
                              {`Cancel edit for message ${message.id}`}
                            </button>
                          </div>
                        ) : (
                          <>
                            {message.content.trim() !== '' ? renderMessageContent(message.content) : null}
                            {message.role === 'assistant' && message.knowledgeCitations ? (
                              <KnowledgeCitationList citations={message.knowledgeCitations} messageId={message.id} />
                            ) : null}
                            {message.attachments && message.attachments.length > 0 ? (
                              <ul aria-label={`Attachments for message ${message.id}`}>
                                {message.attachments.map((attachment) => (
                                  <li key={attachment.id}>
                                    <span>{attachment.name}</span>
                                    <span>{formatAttachmentSize(attachment.sizeBytes)}</span>
                                    <span>{attachment.contentType}</span>
                                  </li>
                                ))}
                              </ul>
                            ) : null}
                          </>
                        )}
                        {message.bookmarked ? <p>Bookmarked</p> : null}
                        {messageShares[message.id] ? <p>{messageShares[message.id]}</p> : null}
                        <div aria-label={`Actions for message ${message.id}`}>
                          <button onClick={() => void copyMessage(message)} type="button">
                            {`Copy message ${message.id}`}
                          </button>
                          <button onClick={() => startEditingMessage(message)} type="button">
                            {`Edit message ${message.id}`}
                          </button>
                          <button onClick={() => void deleteMessage(message)} type="button">
                            {`Delete message ${message.id}`}
                          </button>
                          <button onClick={() => void toggleMessageBookmark(message)} type="button">
                            {message.bookmarked ? `Unbookmark message ${message.id}` : `Bookmark message ${message.id}`}
                          </button>
                          <label>
                            {`Share expiration for ${message.id}`}
                            <input
                              onChange={(event) => {
                                setMessageShareExpirations((currentExpirations) => ({
                                  ...currentExpirations,
                                  [message.id]: event.target.value
                                }));
                              }}
                              placeholder="2026-06-05T12:00:00Z"
                              type="text"
                              value={messageShareExpirations[message.id] ?? ''}
                            />
                          </label>
                          <button onClick={() => void shareMessage(message)} type="button">
                            {`Share message ${message.id}`}
                          </button>
                          <button onClick={() => void shareConversationFromMessage(message)} type="button">
                            {`Share conversation from message ${message.id}`}
                          </button>
                          {message.role === 'user' ? (
                            <button disabled={isSending} onClick={() => void handleRetryFromMessage(message)} type="button">
                              {`Retry from message ${message.id}`}
                            </button>
                          ) : null}
                          {message.role === 'assistant' ? (
                            <button
                              disabled={isSending}
                              onClick={() => void handleRegenerateAssistantMessage(message)}
                              type="button"
                            >
                              {`Regenerate response for message ${message.id}`}
                            </button>
                          ) : null}
                          <button onClick={() => void handleForkFromMessage(message)} type="button">
                            {`Branch from message ${message.id}`}
                          </button>
                          <button onClick={() => void handleForkFromMessage(message)} type="button">
                            {`Fork conversation from message ${message.id}`}
                          </button>
                        </div>
                      </li>
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
              <label>
                Attach images/files
                <input
                  multiple
                  onChange={(event) => updateMessageAttachments(event.target.files)}
                  type="file"
                />
              </label>
              {messageAttachments.length > 0 ? (
                <ul aria-label="Selected attachments">
                  {messageAttachments.map((attachment) => (
                    <li key={attachment.id}>
                      <span>{attachment.name}</span>
                      <span>{formatAttachmentSize(attachment.sizeBytes)}</span>
                      <span>{attachment.contentType}</span>
                    </li>
                  ))}
                </ul>
              ) : null}
              <button disabled={isSending} onClick={() => void handleSendMessage()} type="button">
                Send message
              </button>
            </>
          ) : (
            <section>
              <h2>Conversation transcript</h2>
              {conversations.length === 0 ? (
                <>
                  <p>No conversations yet. Start a workspace thread to begin.</p>
                  <button disabled={isCreatingConversation} onClick={() => void handleCreateConversation()} type="button">
                    Create first conversation
                  </button>
                </>
              ) : (
                <>
                  <p>No active conversation selected.</p>
                  <button disabled={isCreatingConversation} onClick={() => void handleCreateConversation()} type="button">
                    New conversation
                  </button>
                </>
              )}
            </section>
          )}
          {conversationConfig !== null ? (
            <section>
              <h2>Conversation settings</h2>
              <label>
                Conversation model
                <select
                  onChange={(event) => updateConversationConfigDraft({ modelId: event.target.value })}
                  value={conversationConfig.modelId}
                >
                  {modelOptions.length > 0 ? (
                    modelOptions.map((model) => (
                      <option key={model.id} value={model.id}>
                        {model.label}
                      </option>
                    ))
                  ) : (
                    <option value={conversationConfig.modelId}>{conversationConfig.modelId}</option>
                  )}
                </select>
              </label>
              <label>
                Conversation persona
                <select
                  onChange={(event) => updateConversationConfigDraft({ personaId: event.target.value })}
                  value={conversationConfig.personaId ?? ''}
                >
                  <option value="">No persona</option>
                  {personaOptions.map((persona) => (
                    <option key={persona.id} value={persona.id}>
                      {persona.name || persona.role || persona.id}
                    </option>
                  ))}
                </select>
              </label>
              <section aria-label="Persona manager">
                <h3>Personas</h3>
                {personaOptions.length > 0 ? (
                  <ul aria-label="Available personas">
                    {personaOptions.map((persona) => (
                      <li key={persona.id}>
                        <span>{persona.name || persona.role || persona.id}</span>
                        {persona.role ? <span>{persona.role}</span> : null}
                        <button onClick={() => startEditingPersona(persona)} type="button">
                          {`Edit persona ${persona.id}`}
                        </button>
                        <button
                          disabled={deletingPersonaId === persona.id}
                          onClick={() => void deletePersona(persona)}
                          type="button"
                        >
                          {`Delete persona ${persona.id}`}
                        </button>
                      </li>
                    ))}
                  </ul>
                ) : (
                  <p>No personas configured.</p>
                )}
                <label>
                  Persona name
                  <input
                    onChange={(event) => updatePersonaDraft({ name: event.target.value })}
                    type="text"
                    value={personaDraft.name}
                  />
                </label>
                <label>
                  Persona role
                  <input
                    onChange={(event) => updatePersonaDraft({ role: event.target.value })}
                    type="text"
                    value={personaDraft.role}
                  />
                </label>
                <label>
                  Persona style
                  <input
                    onChange={(event) => updatePersonaDraft({ style: event.target.value })}
                    type="text"
                    value={personaDraft.style}
                  />
                </label>
                <label>
                  Persona tone
                  <input
                    onChange={(event) => updatePersonaDraft({ tone: event.target.value })}
                    type="text"
                    value={personaDraft.tone}
                  />
                </label>
                <label>
                  Persona constraints
                  <textarea
                    onChange={(event) => updatePersonaDraft({ constraints: event.target.value })}
                    value={personaDraft.constraints}
                  />
                </label>
                <label>
                  Persona opening message
                  <textarea
                    onChange={(event) => updatePersonaDraft({ openingMessage: event.target.value })}
                    value={personaDraft.openingMessage}
                  />
                </label>
                <label>
                  Persona suggested questions
                  <textarea
                    onChange={(event) => updatePersonaDraft({ suggestedQuestionsText: event.target.value })}
                    value={personaDraft.suggestedQuestionsText}
                  />
                </label>
                <button disabled={isSavingPersona || personaDraft.name.trim() === ''} onClick={() => void savePersona()} type="button">
                  {editingPersonaId ? 'Update persona' : 'Create persona'}
                </button>
                {editingPersonaId ? (
                  <button onClick={cancelPersonaEditing} type="button">
                    Cancel persona editing
                  </button>
                ) : null}
              </section>
              <label>
                Temperature
                <input
                  max="2"
                  min="0"
                  onChange={(event) => updateConversationConfigDraft({ temperature: Number(event.target.value) })}
                  step="0.1"
                  type="number"
                  value={conversationConfig.temperature}
                />
              </label>
              <label>
                Max output tokens
                <input
                  min="1"
                  onChange={(event) => updateConversationConfigDraft({ maxOutputTokens: Number(event.target.value) })}
                  type="number"
                  value={conversationConfig.maxOutputTokens}
                />
              </label>
              <label>
                System prompt override
                <textarea
                  onChange={(event) => updateConversationConfigDraft({ systemPromptOverride: event.target.value })}
                  value={conversationConfig.systemPromptOverride}
                />
              </label>
              <label>
                <input
                  checked={conversationConfig.toolsEnabled}
                  onChange={(event) => updateConversationConfigDraft({ toolsEnabled: event.target.checked })}
                  type="checkbox"
                />
                Enable tools for this conversation
              </label>
              {knowledgeBases.map((knowledgeBase) => (
                <label key={knowledgeBase.id}>
                  <input
                    checked={conversationConfig.knowledgeBaseIds.includes(knowledgeBase.id)}
                    disabled={isUpdatingKnowledgeBinding}
                    onChange={() => void updateKnowledgeBinding(knowledgeBase.id)}
                    type="checkbox"
                  />
                  {`Use knowledge base ${knowledgeBase.name}`}
                </label>
              ))}
              <button disabled={isUpdatingConversationSettings} onClick={() => void saveConversationSettings()} type="button">
                Save conversation settings
              </button>
            </section>
          ) : null}
          {conversationConfig !== null && knowledgeBases.length === 0 ? (
            <button onClick={() => navigate(`/knowledge?returnTo=${encodeURIComponent(chatReturnPath)}`)} type="button">
              Create knowledge base
            </button>
          ) : null}
          {conversationId ? (
            <button disabled={isPreparingHandoff} onClick={() => void openSoloHandoff()} type="button">
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
              <button disabled={isStartingSolo} onClick={() => void startInSolo()} type="button">
                Start in SOLO
              </button>
            </section>
          ) : null}
        </main>
      </div>
    </section>
  );
}
