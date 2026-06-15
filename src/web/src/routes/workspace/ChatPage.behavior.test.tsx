import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const createConversation = vi.fn();
const createConversationShare = vi.fn();
const createTask = vi.fn();
const convertConversationToTask = vi.fn();
const createMessageShare = vi.fn();
const deleteMessage = vi.fn();
const exportConversationMarkdown = vi.fn();
const getConversationConfig = vi.fn();
const forkConversation = vi.fn();
const bookmarkMessage = vi.fn();
const createPersona = vi.fn();
const listConversations = vi.fn();
const listMessages = vi.fn();
const listModels = vi.fn();
const listPersonas = vi.fn();
const startTask = vi.fn();
const sendMessage = vi.fn();
const sendMessageStream = vi.fn();
const deletePersona = vi.fn();
const updateMessage = vi.fn();
const updateConversationConfig = vi.fn();
const updatePersona = vi.fn();
const listKnowledgeBases = vi.fn();
const navigate = vi.fn();
const routeState = vi.hoisted(() => ({
  conversationId: undefined as string | undefined
}));

const appContext = vi.hoisted(() => ({
  authState: {
    preferences: {
      defaultMode: 'chat' as const,
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: true
    },
    status: 'authenticated' as const,
    user: { email: 'user@example.com', id: 'u1' }
  }
}));

class MockWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;
  static instances: MockWebSocket[] = [];

  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent<string>) => void) | null = null;
  onopen: ((event: Event) => void) | null = null;
  readyState = MockWebSocket.CONNECTING;
  sentMessages: string[] = [];
  url: string;

  constructor(url: string) {
    this.url = url;
    MockWebSocket.instances.push(this);
  }

  close() {
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.({} as CloseEvent);
  }

  emit(data: unknown) {
    this.onmessage?.({ data: JSON.stringify(data) } as MessageEvent<string>);
  }

  open() {
    this.readyState = MockWebSocket.OPEN;
    this.onopen?.({} as Event);
  }

  send(data: string) {
    this.sentMessages.push(data);
  }
}

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');

  return {
    ...actual,
    useNavigate: () => navigate,
    useParams: () => ({ conversationId: routeState.conversationId })
  };
});

vi.mock('../../app/providers', () => ({
  useAppContext: () => appContext
}));

vi.mock('../../features/chat/api', async () => {
  const actual = await vi.importActual<typeof import('../../features/chat/api')>('../../features/chat/api');

  return {
    ...actual,
    createChatApi: () => ({
      createConversation,
      createConversationShare,
      createPersona,
      convertConversationToTask,
      createMessageShare,
      deletePersona,
      deleteMessage,
      exportConversationMarkdown,
      forkConversation,
      bookmarkMessage,
      getConversationConfig,
      listConversations,
      listMessages,
      listModels,
      listPersonas,
      sendMessage,
      sendMessageStream,
      updateMessage,
      updateConversationConfig,
      updatePersona
    })
  };
});

vi.mock('../../features/tasks/api', () => ({
  createTasksApi: () => ({
    createTask,
    startTask
  })
}));

vi.mock('../../features/knowledge/api', () => ({
  createKnowledgeApi: () => ({
    listKnowledgeBases
  })
}));

import { ChatPage } from './ChatPage';

function mockActiveConversation() {
  routeState.conversationId = 'conversation_1';
  listConversations.mockResolvedValue([{ id: 'conversation_1', title: 'Research thread' }]);
  listKnowledgeBases.mockResolvedValue([
    {
      documentCount: 3,
      id: 'kb_1',
      name: 'Architecture Notes'
    }
  ]);
  listMessages.mockResolvedValue([{ id: 'm1', role: 'assistant', content: 'Ready when you are.' }]);
  listModels.mockResolvedValue([{ id: 'balanced-chat', label: 'balanced-chat' }]);
  listPersonas.mockResolvedValue([]);
  getConversationConfig.mockResolvedValue({
    conversationId: 'conversation_1',
    knowledgeBaseIds: ['kb_1'],
    maxOutputTokens: 1024,
    modelId: 'balanced-chat',
    systemPromptOverride: '',
    temperature: 1,
    toolsEnabled: false
  });
}

describe('ChatPage', () => {
  beforeEach(() => {
    appContext.authState.preferences = {
      defaultMode: 'chat',
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: true
    };
    createConversation.mockReset();
    createConversationShare.mockReset();
    createPersona.mockReset();
    createTask.mockReset();
    convertConversationToTask.mockReset();
    createMessageShare.mockReset();
    deletePersona.mockReset();
    deleteMessage.mockReset();
    exportConversationMarkdown.mockReset();
    getConversationConfig.mockReset();
    forkConversation.mockReset();
    bookmarkMessage.mockReset();
    listConversations.mockReset();
    listMessages.mockReset();
    listModels.mockReset();
    listPersonas.mockReset();
    listPersonas.mockResolvedValue([]);
    startTask.mockReset();
    sendMessage.mockReset();
    sendMessageStream.mockReset();
    updateMessage.mockReset();
    updateConversationConfig.mockReset();
    updatePersona.mockReset();
    listKnowledgeBases.mockReset();
    navigate.mockReset();
    MockWebSocket.instances = [];
    vi.stubGlobal('WebSocket', MockWebSocket);
    routeState.conversationId = undefined;
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('shows the shared conversation rail on /chat and creates the first conversation', async () => {
    routeState.conversationId = undefined;
    listConversations.mockResolvedValue([]);
    listKnowledgeBases.mockResolvedValue([]);
    listModels.mockResolvedValue([{ id: 'balanced-chat', label: 'balanced-chat' }]);
    createConversation.mockResolvedValue({ id: 'conversation_1', title: 'New conversation' });

    render(<ChatPage />);

    expect(await screen.findByRole('navigation', { name: 'Conversation rail' })).toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: 'Conversation rail' })).toHaveClass('md:w-[200px]', 'lg:w-[300px]');
    expect(screen.getByRole('button', { name: 'Conversations' })).toHaveAttribute('aria-expanded', 'false');
    expect(await screen.findByText('No conversations yet. Start a workspace thread to begin.')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Create first conversation' }));

    await waitFor(() => {
      expect(createConversation).toHaveBeenCalledWith({ title: 'New conversation' });
    });
    expect(navigate).toHaveBeenCalledWith('/chat/conversation_1');
  });

  it('opens and closes the mobile conversation rail from the chat shell', async () => {
    routeState.conversationId = undefined;
    listConversations.mockResolvedValue([{ id: 'conversation_1', title: 'Research thread' }]);
    listKnowledgeBases.mockResolvedValue([]);
    listModels.mockResolvedValue([{ id: 'balanced-chat', label: 'balanced-chat' }]);

    render(<ChatPage />);

    const toggle = await screen.findByRole('button', { name: 'Conversations' });
    const rail = screen.getByRole('navigation', { name: 'Conversation rail' });
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(rail).toHaveClass('hidden');

    fireEvent.click(toggle);

    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(rail).toHaveClass('fixed');
    expect(screen.getByRole('link', { name: 'Open conversation Research thread' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Close conversations' }));

    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(rail).toHaveClass('hidden');
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
    sendMessageStream.mockImplementation(async (_conversationId, _payload, handlers) => {
      handlers.onChunk('Drafted');
      handlers.onChunk(' summary.');
    });
    listMessages
      .mockResolvedValueOnce([{ id: 'm1', role: 'assistant', content: 'Ready when you are.' }])
      .mockResolvedValueOnce([
        { id: 'm1', role: 'assistant', content: 'Ready when you are.' },
        { id: 'm2', role: 'user', content: 'Draft a rollout summary.' },
        { id: 'm3', role: 'assistant', content: 'Drafted summary.' }
      ]);

    render(<ChatPage />);

    await screen.findByText('Ready when you are.');
    fireEvent.change(screen.getByLabelText('Message draft'), { target: { value: 'Draft a rollout summary.' } });
    fireEvent.click(screen.getByRole('button', { name: 'Send message' }));

    await waitFor(() => {
      expect(sendMessageStream).toHaveBeenCalledWith(
        'conversation_1',
        { content: 'Draft a rollout summary.' },
        expect.objectContaining({ onChunk: expect.any(Function) })
      );
    });
    expect(await screen.findByText('Drafted summary.')).toBeInTheDocument();
    expect(listMessages).toHaveBeenLastCalledWith('conversation_1');
  });

  it('joins the active realtime conversation and sends typing presence', async () => {
    mockActiveConversation();

    render(<ChatPage />);

    await screen.findByText('Ready when you are.');
    expect(MockWebSocket.instances).toHaveLength(1);
    const socket = MockWebSocket.instances[0];
    expect(socket.url).toMatch('/api/v1/ws');

    act(() => socket.open());

    expect(socket.sentMessages.map((message) => JSON.parse(message))).toContainEqual({
      conversationId: 'conversation_1',
      type: 'chat_join'
    });

    fireEvent.change(screen.getByLabelText('Message draft'), { target: { value: 'Co-edit this answer.' } });

    await waitFor(() => {
      expect(socket.sentMessages.map((message) => JSON.parse(message))).toContainEqual({
        conversationId: 'conversation_1',
        isTyping: true,
        type: 'chat_typing'
      });
    });
  });

  it('applies realtime sync, update, delete, and typing events for the active conversation', async () => {
    mockActiveConversation();

    render(<ChatPage />);

    await screen.findByText('Ready when you are.');
    const socket = MockWebSocket.instances[0];
    act(() => socket.open());

    act(() => {
      socket.emit({
        category: 'chat',
        payload: {
          conversationId: 'conversation_1',
          messages: [
            { id: 'm1', role: 'assistant', content: 'Ready when you are.' },
            { id: 'm2', role: 'user', content: 'Realtime draft.' }
          ]
        },
        type: 'chat_messages_synced'
      });
    });

    expect(await screen.findByText('Realtime draft.')).toBeInTheDocument();

    act(() => {
      socket.emit({
        category: 'chat',
        payload: {
          conversationId: 'conversation_1',
          isTyping: true,
          userId: 'user_2'
        },
        type: 'chat_typing'
      });
    });

    expect(await screen.findByRole('status')).toHaveTextContent('A collaborator is typing...');

    act(() => {
      socket.emit({
        category: 'chat',
        payload: {
          conversationId: 'conversation_1',
          message: { id: 'm2', role: 'user', content: 'Realtime draft updated.' },
          messageId: 'm2'
        },
        type: 'chat_message_updated'
      });
    });

    expect(await screen.findByText('Realtime draft updated.')).toBeInTheDocument();

    act(() => {
      socket.emit({
        category: 'chat',
        payload: {
          conversationId: 'conversation_1',
          messageId: 'm2'
        },
        type: 'chat_message_deleted'
      });
    });

    await waitFor(() => {
      expect(screen.queryByText('Realtime draft updated.')).not.toBeInTheDocument();
    });
  });

  it('previews selected image attachments and sends attachment metadata with the stream payload', async () => {
    mockActiveConversation();
    sendMessageStream.mockImplementation(async (_conversationId, _payload, handlers) => {
      handlers.onChunk('Reviewed attachment.');
    });
    listMessages
      .mockResolvedValueOnce([{ id: 'm1', role: 'assistant', content: 'Ready when you are.' }])
      .mockResolvedValueOnce([
        { id: 'm1', role: 'assistant', content: 'Ready when you are.' },
        {
          attachments: [
            {
              contentType: 'image/png',
              id: 'attachment-diagram.png-2048',
              name: 'diagram.png',
              sizeBytes: 2048,
              type: 'image'
            }
          ],
          id: 'm2',
          role: 'user',
          content: 'Review this.'
        },
        { id: 'm3', role: 'assistant', content: 'Reviewed attachment.' }
      ]);

    render(<ChatPage />);

    await screen.findByText('Ready when you are.');
    const attachmentInput = screen.getByLabelText('Attach images/files');
    const imageFile = new File(['x'.repeat(2048)], 'diagram.png', { type: 'image/png' });
    fireEvent.change(attachmentInput, { target: { files: [imageFile] } });

    expect(await screen.findByText('diagram.png')).toBeInTheDocument();
    expect(screen.getByText('2 KB')).toBeInTheDocument();
    expect(screen.getByText('image/png')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Message draft'), { target: { value: 'Review this.' } });
    fireEvent.click(screen.getByRole('button', { name: 'Send message' }));

    await waitFor(() => {
      expect(sendMessageStream).toHaveBeenCalledWith(
        'conversation_1',
        {
          attachments: [
            {
              contentType: 'image/png',
              id: 'attachment-diagram.png-2048',
              name: 'diagram.png',
              sizeBytes: 2048,
              type: 'image'
            }
          ],
          content: 'Review this.'
        },
        expect.objectContaining({ onChunk: expect.any(Function) })
      );
    });
  });

  it('renders fenced code blocks in transcript messages instead of plain text', async () => {
    routeState.conversationId = 'conversation_1';
    listConversations.mockResolvedValue([{ id: 'conversation_1', title: 'Research thread' }]);
    listKnowledgeBases.mockResolvedValue([]);
    listMessages.mockResolvedValue([
      {
        id: 'm1',
        role: 'assistant',
        content: 'Use this helper:\n\n```ts\nconst answer: number = 42;\n```\n\nThen continue.'
      }
    ]);
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

    const { container } = render(<ChatPage />);

    expect(await screen.findByText('Use this helper:')).toBeInTheDocument();
    expect(await screen.findByText('Then continue.')).toBeInTheDocument();
    expect(await screen.findByText('ts')).toBeInTheDocument();
    expect(container.querySelector('pre code')).toHaveTextContent('const answer: number = 42;');
  });

  it('renders assistant knowledge citation cards with source details', async () => {
    routeState.conversationId = 'conversation_1';
    listConversations.mockResolvedValue([{ id: 'conversation_1', title: 'Research thread' }]);
    listKnowledgeBases.mockResolvedValue([]);
    listMessages.mockResolvedValue([
	      {
	        id: 'm1',
	        role: 'assistant',
	        content: 'The rollout depends on the published release gates.',
	        knowledgeCitations: [
	          {
	            chunkId: 'chunk_7',
	            chunkIndex: 6,
	            documentVersion: 'v4',
	            highlightPositions: [
	              { start: 0, end: 13 },
	              { start: 41, end: 56 }
	            ],
	            documentTitle: 'Release Gate Runbook',
	            knowledgeBaseId: 'kb_1',
	            knowledgeBaseName: 'Architecture Notes',
	            pageNumber: 12,
	            score: 0.87,
	            snippet: 'Every release must pass smoke tests, rollback drills, and owner approval.',
	            sourceUrl: 'https://docs.example.test/release-gates'
	          }
        ]
      }
    ]);
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

    expect(await screen.findByText('The rollout depends on the published release gates.')).toBeInTheDocument();
    expect(await screen.findByText('Release Gate Runbook')).toBeInTheDocument();
	    expect(screen.getByText('Architecture Notes')).toBeInTheDocument();
	    expect(screen.getByText('Every release must pass smoke tests, rollback drills, and owner approval.')).toBeInTheDocument();
	    expect(screen.getByText('Page 12')).toBeInTheDocument();
	    expect(screen.getByText('Version v4')).toBeInTheDocument();
	    expect(screen.getByText('Chunk 7')).toBeInTheDocument();
	    expect(screen.getByText('Highlights 0-13, 41-56')).toBeInTheDocument();
	    expect(screen.getByText('Score 0.87')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open citation source for Release Gate Runbook' })).toHaveAttribute(
      'href',
      'https://docs.example.test/release-gates'
    );
  });

  it('copies the raw text from a fenced code block and shows a copied state', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, {
      clipboard: {
        writeText
      }
    });
    routeState.conversationId = 'conversation_1';
    listConversations.mockResolvedValue([{ id: 'conversation_1', title: 'Research thread' }]);
    listKnowledgeBases.mockResolvedValue([]);
    listMessages.mockResolvedValue([
      {
        id: 'm1',
        role: 'assistant',
        content: 'Use this helper:\n\n```ts\nconst answer = 42;\nconsole.log(answer);\n```\n\nThen continue.'
      }
    ]);
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

    fireEvent.click(await screen.findByRole('button', { name: 'Copy code block ts' }));

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith('const answer = 42;\nconsole.log(answer);');
    });
    expect(await screen.findByRole('button', { name: 'Copied code block ts' })).toBeInTheDocument();
  });

  it('exports the active conversation as markdown', async () => {
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
    exportConversationMarkdown.mockResolvedValue('# Research thread\n');
    const createObjectURL = vi.fn(() => 'blob:conversation-markdown');
    const originalCreateObjectURL = URL.createObjectURL;
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: createObjectURL
    });

    try {
      render(<ChatPage />);

      await screen.findByText('Ready when you are.');
      fireEvent.click(screen.getByRole('button', { name: 'Export Markdown' }));

      await waitFor(() => {
        expect(exportConversationMarkdown).toHaveBeenCalledWith('conversation_1');
      });
      expect(createObjectURL).toHaveBeenCalledTimes(1);
      expect(await screen.findByRole('link', { name: 'Download Markdown export' })).toHaveAttribute('href', 'blob:conversation-markdown');
    } finally {
      Object.defineProperty(URL, 'createObjectURL', {
        configurable: true,
        value: originalCreateObjectURL
      });
    }
  });

  it('shares the active conversation with an expiration', async () => {
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
    createConversationShare.mockResolvedValue({
      id: 'share_conversation_1',
      url: 'https://share.example.test/conversation_1'
    });

    render(<ChatPage />);

    await screen.findByText('Ready when you are.');
    fireEvent.change(screen.getByLabelText('Conversation share expiration'), {
      target: { value: '2026-06-05T12:00:00Z' }
    });
    fireEvent.click(screen.getByRole('button', { name: 'Share conversation' }));

    await waitFor(() => {
      expect(createConversationShare).toHaveBeenCalledWith('conversation_1', {
        expiresAt: '2026-06-05T12:00:00Z'
      });
    });
    expect(await screen.findByText('https://share.example.test/conversation_1')).toBeInTheDocument();
  });

  it('shares a conversation range from a transcript message', async () => {
    routeState.conversationId = 'conversation_1';
    listConversations.mockResolvedValue([{ id: 'conversation_1', title: 'Research thread' }]);
    listKnowledgeBases.mockResolvedValue([]);
    listMessages.mockResolvedValue([
      { id: 'm1', role: 'user', content: 'Start from here.' },
      { id: 'm2', role: 'assistant', content: 'Range answer.' }
    ]);
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
    createConversationShare.mockResolvedValue({
      id: 'share_conversation_range_1',
      url: 'https://share.example.test/range_1'
    });

    render(<ChatPage />);

    await screen.findByText('Range answer.');
    fireEvent.change(screen.getByLabelText('Conversation share expiration'), {
      target: { value: '2026-06-05T12:00:00Z' }
    });
    fireEvent.click(screen.getByRole('button', { name: 'Share conversation from message m1' }));

    await waitFor(() => {
      expect(createConversationShare).toHaveBeenCalledWith('conversation_1', {
        expiresAt: '2026-06-05T12:00:00Z',
        startMessageId: 'm1'
      });
    });
    expect(await screen.findByText('https://share.example.test/range_1')).toBeInTheDocument();
  });

  it('saves model settings and sends messages with conversation overrides', async () => {
    routeState.conversationId = 'conversation_1';
    listConversations.mockResolvedValue([{ id: 'conversation_1', title: 'Research thread' }]);
    listKnowledgeBases.mockResolvedValue([]);
    listMessages.mockResolvedValue([{ id: 'm1', role: 'assistant', content: 'Ready when you are.' }]);
    listModels.mockResolvedValue([
      { id: 'balanced-chat', label: 'Balanced chat' },
      { id: 'quality-chat', label: 'Quality chat' }
    ]);
    getConversationConfig.mockResolvedValue({
      conversationId: 'conversation_1',
      knowledgeBaseIds: [],
      maxOutputTokens: 1024,
      modelId: 'balanced-chat',
      systemPromptOverride: '',
      temperature: 1,
      toolsEnabled: false
    });
    updateConversationConfig.mockResolvedValue({
      conversationId: 'conversation_1',
      knowledgeBaseIds: [],
      maxOutputTokens: 2048,
      modelId: 'quality-chat',
      systemPromptOverride: 'Prefer concise rollout notes.',
      temperature: 0.4,
      toolsEnabled: true
    });
    sendMessageStream.mockImplementation(async (_conversationId, _payload, handlers) => {
      handlers.onChunk('Concise response.');
    });
    listMessages
      .mockResolvedValueOnce([{ id: 'm1', role: 'assistant', content: 'Ready when you are.' }])
      .mockResolvedValueOnce([
        { id: 'm1', role: 'assistant', content: 'Ready when you are.' },
        { id: 'm2', role: 'user', content: 'Summarize the launch risk.' },
        { id: 'm3', role: 'assistant', content: 'Concise response.' }
      ]);

    render(<ChatPage />);

    await screen.findByLabelText('Conversation model');
    fireEvent.change(screen.getByLabelText('Conversation model'), { target: { value: 'quality-chat' } });
    fireEvent.change(screen.getByLabelText('Temperature'), { target: { value: '0.4' } });
    fireEvent.change(screen.getByLabelText('Max output tokens'), { target: { value: '2048' } });
    fireEvent.change(screen.getByLabelText('System prompt override'), { target: { value: 'Prefer concise rollout notes.' } });
    fireEvent.click(screen.getByLabelText('Enable tools for this conversation'));
    fireEvent.click(screen.getByRole('button', { name: 'Save conversation settings' }));

    await waitFor(() => {
      expect(updateConversationConfig).toHaveBeenCalledWith('conversation_1', {
        knowledgeBaseIds: [],
        maxOutputTokens: 2048,
        modelId: 'quality-chat',
        personaId: '',
        systemPromptOverride: 'Prefer concise rollout notes.',
        temperature: 0.4,
        toolsEnabled: true
      });
    });

    fireEvent.change(screen.getByLabelText('Message draft'), { target: { value: 'Summarize the launch risk.' } });
    fireEvent.click(screen.getByRole('button', { name: 'Send message' }));

    await waitFor(() => {
      expect(sendMessageStream).toHaveBeenCalledWith(
        'conversation_1',
        {
          content: 'Summarize the launch risk.',
          overrides: {
            maxOutputTokens: 2048,
            modelId: 'quality-chat',
            systemPromptOverride: 'Prefer concise rollout notes.',
            temperature: 0.4,
            toolsEnabled: true
          }
        },
        expect.objectContaining({ onChunk: expect.any(Function) })
      );
    });
  });

  it('preserves saved conversation overrides when sending attachments', async () => {
    routeState.conversationId = 'conversation_1';
    listConversations.mockResolvedValue([{ id: 'conversation_1', title: 'Research thread' }]);
    listKnowledgeBases.mockResolvedValue([]);
    listMessages.mockResolvedValue([{ id: 'm1', role: 'assistant', content: 'Ready when you are.' }]);
    listModels.mockResolvedValue([
      { id: 'balanced-chat', label: 'Balanced chat' },
      { id: 'quality-chat', label: 'Quality chat' }
    ]);
    getConversationConfig.mockResolvedValue({
      conversationId: 'conversation_1',
      knowledgeBaseIds: [],
      maxOutputTokens: 1024,
      modelId: 'balanced-chat',
      systemPromptOverride: '',
      temperature: 1,
      toolsEnabled: false
    });
    updateConversationConfig.mockResolvedValue({
      conversationId: 'conversation_1',
      knowledgeBaseIds: [],
      maxOutputTokens: 2048,
      modelId: 'quality-chat',
      systemPromptOverride: 'Prefer concise rollout notes.',
      temperature: 0.4,
      toolsEnabled: true
    });
    sendMessageStream.mockImplementation(async (_conversationId, _payload, handlers) => {
      handlers.onChunk('Concise response.');
    });
    listMessages
      .mockResolvedValueOnce([{ id: 'm1', role: 'assistant', content: 'Ready when you are.' }])
      .mockResolvedValueOnce([
        { id: 'm1', role: 'assistant', content: 'Ready when you are.' },
        {
          attachments: [
            {
              contentType: 'application/pdf',
              id: 'attachment-brief.pdf-4096',
              name: 'brief.pdf',
              sizeBytes: 4096,
              type: 'file'
            }
          ],
          id: 'm2',
          role: 'user',
          content: ''
        },
        { id: 'm3', role: 'assistant', content: 'Concise response.' }
      ]);

    render(<ChatPage />);

    await screen.findByLabelText('Conversation model');
    fireEvent.change(screen.getByLabelText('Conversation model'), { target: { value: 'quality-chat' } });
    fireEvent.change(screen.getByLabelText('Temperature'), { target: { value: '0.4' } });
    fireEvent.change(screen.getByLabelText('Max output tokens'), { target: { value: '2048' } });
    fireEvent.change(screen.getByLabelText('System prompt override'), { target: { value: 'Prefer concise rollout notes.' } });
    fireEvent.click(screen.getByLabelText('Enable tools for this conversation'));
    fireEvent.click(screen.getByRole('button', { name: 'Save conversation settings' }));

    await waitFor(() => {
      expect(updateConversationConfig).toHaveBeenCalled();
    });

    const attachmentInput = screen.getByLabelText('Attach images/files');
    const pdfFile = new File(['x'.repeat(4096)], 'brief.pdf', { type: 'application/pdf' });
    fireEvent.change(attachmentInput, { target: { files: [pdfFile] } });
    fireEvent.click(screen.getByRole('button', { name: 'Send message' }));

    await waitFor(() => {
      expect(sendMessageStream).toHaveBeenCalledWith(
        'conversation_1',
        {
          attachments: [
            {
              contentType: 'application/pdf',
              id: 'attachment-brief.pdf-4096',
              name: 'brief.pdf',
              sizeBytes: 4096,
              type: 'file'
            }
          ],
          content: '',
          overrides: {
            maxOutputTokens: 2048,
            modelId: 'quality-chat',
            systemPromptOverride: 'Prefer concise rollout notes.',
            temperature: 0.4,
            toolsEnabled: true
          }
        },
        expect.objectContaining({ onChunk: expect.any(Function) })
      );
    });
  });

  it('saves persona selection with conversation settings', async () => {
    routeState.conversationId = 'conversation_1';
    listConversations.mockResolvedValue([{ id: 'conversation_1', title: 'Research thread' }]);
    listKnowledgeBases.mockResolvedValue([]);
    listMessages.mockResolvedValue([{ id: 'm1', role: 'assistant', content: 'Ready when you are.' }]);
    listModels.mockResolvedValue([{ id: 'balanced-chat', label: 'Balanced chat' }]);
    listPersonas.mockResolvedValue([
      {
        constraints: 'Call out rollout risk.',
        id: 'persona_launch',
        name: 'Launch reviewer',
        role: 'Launch reviewer',
        style: 'Direct',
        tone: 'Precise'
      }
    ]);
    getConversationConfig.mockResolvedValue({
      conversationId: 'conversation_1',
      knowledgeBaseIds: [],
      maxOutputTokens: 1024,
      modelId: 'balanced-chat',
      personaId: '',
      systemPromptOverride: '',
      temperature: 1,
      toolsEnabled: false
    });
    updateConversationConfig.mockResolvedValue({
      conversationId: 'conversation_1',
      knowledgeBaseIds: [],
      maxOutputTokens: 1024,
      modelId: 'balanced-chat',
      personaId: 'persona_launch',
      systemPromptOverride: '',
      temperature: 1,
      toolsEnabled: false
    });

    render(<ChatPage />);

    await screen.findByLabelText('Conversation persona');
    expect(screen.getByRole('option', { name: 'Launch reviewer' })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Conversation persona'), { target: { value: 'persona_launch' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save conversation settings' }));

    await waitFor(() => {
      expect(updateConversationConfig).toHaveBeenCalledWith('conversation_1', {
        knowledgeBaseIds: [],
        maxOutputTokens: 1024,
        modelId: 'balanced-chat',
        personaId: 'persona_launch',
        systemPromptOverride: '',
        temperature: 1,
        toolsEnabled: false
      });
    });
  });

  it('creates a persona from conversation settings and selects it locally', async () => {
    routeState.conversationId = 'conversation_1';
    listConversations.mockResolvedValue([{ id: 'conversation_1', title: 'Research thread' }]);
    listKnowledgeBases.mockResolvedValue([]);
    listMessages.mockResolvedValue([{ id: 'm1', role: 'assistant', content: 'Ready when you are.' }]);
    listModels.mockResolvedValue([{ id: 'balanced-chat', label: 'Balanced chat' }]);
    listPersonas.mockResolvedValue([]);
    getConversationConfig.mockResolvedValue({
      conversationId: 'conversation_1',
      knowledgeBaseIds: [],
      maxOutputTokens: 1024,
      modelId: 'balanced-chat',
      personaId: '',
      systemPromptOverride: '',
      temperature: 1,
      toolsEnabled: false
    });
    createPersona.mockResolvedValue({
      constraints: 'Call out rollout risk.',
      id: 'persona_launch',
      name: 'Launch reviewer',
      openingMessage: 'Ready to review.',
      role: 'Reviewer',
      style: 'Direct',
      suggestedQuestions: ['What is risky?', 'What changed?'],
      tone: 'Precise'
    });

    render(<ChatPage />);

    await screen.findByLabelText('Persona name');
    fireEvent.change(screen.getByLabelText('Persona name'), { target: { value: 'Launch reviewer' } });
    fireEvent.change(screen.getByLabelText('Persona role'), { target: { value: 'Reviewer' } });
    fireEvent.change(screen.getByLabelText('Persona style'), { target: { value: 'Direct' } });
    fireEvent.change(screen.getByLabelText('Persona tone'), { target: { value: 'Precise' } });
    fireEvent.change(screen.getByLabelText('Persona constraints'), { target: { value: 'Call out rollout risk.' } });
    fireEvent.change(screen.getByLabelText('Persona opening message'), { target: { value: 'Ready to review.' } });
    fireEvent.change(screen.getByLabelText('Persona suggested questions'), {
      target: { value: 'What is risky?\n\nWhat changed?' }
    });
    fireEvent.click(screen.getByRole('button', { name: 'Create persona' }));

    await waitFor(() => {
      expect(createPersona).toHaveBeenCalledWith({
        constraints: 'Call out rollout risk.',
        name: 'Launch reviewer',
        openingMessage: 'Ready to review.',
        role: 'Reviewer',
        style: 'Direct',
        suggestedQuestions: ['What is risky?', 'What changed?'],
        tone: 'Precise'
      });
    });
    expect(await screen.findByRole('option', { name: 'Launch reviewer' })).toBeInTheDocument();
    expect(screen.getByLabelText('Conversation persona')).toHaveValue('persona_launch');
  });

  it('edits a persona from conversation settings', async () => {
    routeState.conversationId = 'conversation_1';
    listConversations.mockResolvedValue([{ id: 'conversation_1', title: 'Research thread' }]);
    listKnowledgeBases.mockResolvedValue([]);
    listMessages.mockResolvedValue([{ id: 'm1', role: 'assistant', content: 'Ready when you are.' }]);
    listModels.mockResolvedValue([{ id: 'balanced-chat', label: 'Balanced chat' }]);
    listPersonas.mockResolvedValue([
      {
        constraints: 'Call out rollout risk.',
        id: 'persona_launch',
        name: 'Launch reviewer',
        openingMessage: 'Ready to review.',
        role: 'Reviewer',
        style: 'Direct',
        suggestedQuestions: ['What changed?'],
        tone: 'Precise'
      }
    ]);
    getConversationConfig.mockResolvedValue({
      conversationId: 'conversation_1',
      knowledgeBaseIds: [],
      maxOutputTokens: 1024,
      modelId: 'balanced-chat',
      personaId: '',
      systemPromptOverride: '',
      temperature: 1,
      toolsEnabled: false
    });
    updatePersona.mockResolvedValue({
      constraints: 'Focus on release blockers.',
      id: 'persona_launch',
      name: 'Launch critic',
      openingMessage: 'Ready to review.',
      role: 'Reviewer',
      style: 'Direct',
      suggestedQuestions: ['What changed?', 'What can slip?'],
      tone: 'Precise'
    });

    render(<ChatPage />);

    await screen.findByRole('button', { name: 'Edit persona persona_launch' });
    fireEvent.click(screen.getByRole('button', { name: 'Edit persona persona_launch' }));
    expect(screen.getByLabelText('Persona suggested questions')).toHaveValue('What changed?');
    fireEvent.change(screen.getByLabelText('Persona name'), { target: { value: 'Launch critic' } });
    fireEvent.change(screen.getByLabelText('Persona constraints'), { target: { value: 'Focus on release blockers.' } });
    fireEvent.change(screen.getByLabelText('Persona suggested questions'), {
      target: { value: 'What changed?\nWhat can slip?' }
    });
    fireEvent.click(screen.getByRole('button', { name: 'Update persona' }));

    await waitFor(() => {
      expect(updatePersona).toHaveBeenCalledWith('persona_launch', {
        constraints: 'Focus on release blockers.',
        name: 'Launch critic',
        openingMessage: 'Ready to review.',
        role: 'Reviewer',
        style: 'Direct',
        suggestedQuestions: ['What changed?', 'What can slip?'],
        tone: 'Precise'
      });
    });
    expect(await screen.findByRole('option', { name: 'Launch critic' })).toBeInTheDocument();
  });

  it('deletes a selected persona and clears conversation settings', async () => {
    routeState.conversationId = 'conversation_1';
    listConversations.mockResolvedValue([{ id: 'conversation_1', title: 'Research thread' }]);
    listKnowledgeBases.mockResolvedValue([]);
    listMessages.mockResolvedValue([{ id: 'm1', role: 'assistant', content: 'Ready when you are.' }]);
    listModels.mockResolvedValue([{ id: 'balanced-chat', label: 'Balanced chat' }]);
    listPersonas.mockResolvedValue([
      {
        constraints: 'Call out rollout risk.',
        id: 'persona_launch',
        name: 'Launch reviewer',
        role: 'Reviewer',
        style: 'Direct',
        tone: 'Precise'
      }
    ]);
    getConversationConfig.mockResolvedValue({
      conversationId: 'conversation_1',
      knowledgeBaseIds: [],
      maxOutputTokens: 1024,
      modelId: 'balanced-chat',
      personaId: 'persona_launch',
      systemPromptOverride: '',
      temperature: 1,
      toolsEnabled: false
    });
    deletePersona.mockResolvedValue(undefined);
    updateConversationConfig.mockResolvedValue({
      conversationId: 'conversation_1',
      knowledgeBaseIds: [],
      maxOutputTokens: 1024,
      modelId: 'balanced-chat',
      personaId: '',
      systemPromptOverride: '',
      temperature: 1,
      toolsEnabled: false
    });

    render(<ChatPage />);

    await screen.findByRole('button', { name: 'Delete persona persona_launch' });
    expect(screen.getByLabelText('Conversation persona')).toHaveValue('persona_launch');
    fireEvent.click(screen.getByRole('button', { name: 'Delete persona persona_launch' }));

    await waitFor(() => {
      expect(deletePersona).toHaveBeenCalledWith('persona_launch');
    });
    await waitFor(() => {
      expect(updateConversationConfig).toHaveBeenCalledWith('conversation_1', {
        knowledgeBaseIds: [],
        maxOutputTokens: 1024,
        modelId: 'balanced-chat',
        personaId: '',
        systemPromptOverride: '',
        temperature: 1,
        toolsEnabled: false
      });
    });
    expect(screen.getByLabelText('Conversation persona')).toHaveValue('');
    expect(screen.queryByRole('option', { name: 'Launch reviewer' })).not.toBeInTheDocument();
  });

  it('renders a conversation rail for the active conversation and routes between conversations', async () => {
    routeState.conversationId = 'conversation_1';
    listConversations.mockResolvedValue([
      { id: 'conversation_1', title: 'Research thread' },
      { id: 'conversation_2', title: 'Launch checklist' },
      { id: 'conversation_3', title: 'Support triage' }
    ]);
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
    createConversation.mockResolvedValue({ id: 'conversation_new', title: 'New conversation' });

    render(<ChatPage />);

    const rail = await screen.findByRole('navigation', { name: 'Conversation rail' });
    expect(rail).toHaveTextContent('Research thread');
    expect(rail).toHaveTextContent('Launch checklist');
    expect(rail).toHaveTextContent('Support triage');

    fireEvent.click(screen.getByRole('link', { name: 'Open conversation Launch checklist' }));
    expect(navigate).toHaveBeenCalledWith('/chat/conversation_2');

    fireEvent.click(screen.getByRole('button', { name: 'New conversation' }));
    await waitFor(() => {
      expect(createConversation).toHaveBeenCalledWith({ title: 'New conversation' });
    });
    expect(navigate).toHaveBeenCalledWith('/chat/conversation_new');
  });

  it('filters the active conversation rail by search text', async () => {
    routeState.conversationId = 'conversation_1';
    listConversations.mockResolvedValue([
      { id: 'conversation_1', title: 'Research thread' },
      { id: 'conversation_2', title: 'Launch checklist' },
      { id: 'conversation_3', title: 'Support triage' }
    ]);
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

    render(<ChatPage />);

    await screen.findByRole('navigation', { name: 'Conversation rail' });
    fireEvent.change(screen.getByLabelText('Search conversations'), { target: { value: 'launch' } });

    expect(screen.getByRole('link', { name: 'Open conversation Launch checklist' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Open conversation Research thread' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Open conversation Support triage' })).not.toBeInTheDocument();
  });

  it('filters the active conversation rail to conversations with bookmarked messages', async () => {
    routeState.conversationId = 'conversation_1';
    listConversations.mockResolvedValue([
      { hasBookmarkedMessages: true, id: 'conversation_1', title: 'Research thread' },
      { hasBookmarkedMessages: false, id: 'conversation_2', title: 'Launch checklist' }
    ]);
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

    render(<ChatPage />);

    await screen.findByRole('navigation', { name: 'Conversation rail' });
    fireEvent.click(screen.getByRole('button', { name: 'Starred conversations' }));
    expect(screen.getByRole('link', { name: 'Open conversation Research thread' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Open conversation Launch checklist' })).not.toBeInTheDocument();
  });

  it('filters the active conversation rail to archived conversations', async () => {
    routeState.conversationId = 'conversation_1';
    listConversations.mockResolvedValue([
      { id: 'conversation_1', title: 'Research thread' },
      { archivedAt: '2026-06-04T10:00:00Z', id: 'conversation_2', title: 'Launch checklist' }
    ]);
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

    render(<ChatPage />);

    await screen.findByRole('navigation', { name: 'Conversation rail' });

    fireEvent.click(screen.getByRole('button', { name: 'Archived conversations' }));
    expect(screen.getByRole('link', { name: 'Open conversation Launch checklist' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Open conversation Research thread' })).not.toBeInTheDocument();
  });

  it('forks the conversation from a selected message and opens the branch', async () => {
    routeState.conversationId = 'conversation_1';
    listConversations.mockResolvedValue([{ id: 'conversation_1', title: 'Research thread' }]);
    listKnowledgeBases.mockResolvedValue([]);
    listMessages.mockResolvedValue([
      { id: 'm1', role: 'user', content: 'Explore the launch plan.' },
      { id: 'm2', role: 'assistant', content: 'Here is the first path.' },
      { id: 'm3', role: 'user', content: 'Actually try another direction.' }
    ]);
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
    forkConversation.mockResolvedValue({ id: 'conversation_fork', title: 'Research thread branch' });

    render(<ChatPage />);

    await screen.findByText('Here is the first path.');
    fireEvent.click(screen.getByRole('button', { name: 'Fork conversation from message m2' }));

    await waitFor(() => {
      expect(forkConversation).toHaveBeenCalledWith('conversation_1', {
        messageId: 'm2',
        title: 'Branch from Here is the first path.'
      });
    });
    expect(navigate).toHaveBeenCalledWith('/chat/conversation_fork');
  });

  it('renders message actions for copy, retry, and branch workflows', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, {
      clipboard: {
        writeText
      }
    });
    routeState.conversationId = 'conversation_1';
    listConversations.mockResolvedValue([{ id: 'conversation_1', title: 'Research thread' }]);
    listKnowledgeBases.mockResolvedValue([]);
    listMessages.mockResolvedValue([
      { id: 'm1', role: 'user', content: 'Draft the launch checklist.' },
      { id: 'm2', role: 'assistant', content: 'Here is the checklist.' }
    ]);
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
      { id: 'm1', role: 'user', content: 'Draft the launch checklist.' },
      { id: 'm2', role: 'assistant', content: 'Here is the checklist.' },
      { id: 'm3', role: 'assistant', content: 'Retried checklist.' }
    ]);
    forkConversation.mockResolvedValue({ id: 'conversation_fork', title: 'Checklist branch' });

    render(<ChatPage />);

    await screen.findByText('Here is the checklist.');
    fireEvent.click(screen.getByRole('button', { name: 'Copy message m2' }));
    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith('Here is the checklist.');
    });

    fireEvent.click(screen.getByRole('button', { name: 'Retry from message m1' }));
    await waitFor(() => {
      expect(sendMessage).toHaveBeenCalledWith('conversation_1', {
        content: 'Draft the launch checklist.',
        overrides: {
          maxOutputTokens: 1024,
          modelId: 'balanced-chat',
          systemPromptOverride: '',
          temperature: 1,
          toolsEnabled: false
        }
      });
    });

    fireEvent.click(screen.getByRole('button', { name: 'Branch from message m2' }));
    await waitFor(() => {
      expect(forkConversation).toHaveBeenCalledWith('conversation_1', {
        messageId: 'm2',
        title: 'Branch from Here is the checklist.'
      });
    });
  });

  it('regenerates an assistant response from the assistant message action', async () => {
    routeState.conversationId = 'conversation_1';
    listConversations.mockResolvedValue([{ id: 'conversation_1', title: 'Research thread' }]);
    listKnowledgeBases.mockResolvedValue([]);
    listMessages.mockResolvedValue([
      { id: 'm1', role: 'user', content: 'Draft the launch checklist.' },
      { id: 'm2', role: 'assistant', content: 'Here is the checklist.' }
    ]);
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
      { id: 'm1', role: 'user', content: 'Draft the launch checklist.' },
      { id: 'm2', role: 'assistant', content: 'Here is the checklist.' },
      { id: 'm3', role: 'assistant', content: 'Regenerated checklist.' }
    ]);

    render(<ChatPage />);

    await screen.findByText('Here is the checklist.');
    fireEvent.click(screen.getByRole('button', { name: 'Regenerate response for message m2' }));

    await waitFor(() => {
      expect(sendMessage).toHaveBeenCalledWith('conversation_1', {
        content: 'Draft the launch checklist.',
        overrides: {
          maxOutputTokens: 1024,
          modelId: 'balanced-chat',
          systemPromptOverride: '',
          temperature: 1,
          toolsEnabled: false
        }
      });
    });
    expect(await screen.findByText('Regenerated checklist.')).toBeInTheDocument();
  });

  it('shows an action error when regenerating an assistant response without a prior user message', async () => {
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

    render(<ChatPage />);

    await screen.findByText('Ready when you are.');
    fireEvent.click(screen.getByRole('button', { name: 'Regenerate response for message m1' }));

    expect(await screen.findByText('No user message is available to regenerate this response.')).toBeInTheDocument();
    expect(sendMessage).not.toHaveBeenCalled();
  });

  it('edits, deletes, bookmarks, and shares messages from the transcript actions', async () => {
    routeState.conversationId = 'conversation_1';
    listConversations.mockResolvedValue([{ id: 'conversation_1', title: 'Research thread' }]);
    listKnowledgeBases.mockResolvedValue([]);
    listMessages.mockResolvedValue([
      { id: 'm1', role: 'user', content: 'Draft the launch checklist.' },
      { id: 'm2', role: 'assistant', content: 'Here is the checklist.', bookmarked: false }
    ]);
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
    updateMessage.mockResolvedValue({ id: 'm2', role: 'assistant', content: 'Here is the revised checklist.', bookmarked: false });
    bookmarkMessage.mockResolvedValue({ id: 'm2', role: 'assistant', content: 'Here is the revised checklist.', bookmarked: true });
    createMessageShare.mockResolvedValue({ id: 'share_1', url: 'https://share.example.test/share_1' });
    deleteMessage.mockResolvedValue(undefined);

    render(<ChatPage />);

    await screen.findByText('Here is the checklist.');
    fireEvent.click(screen.getByRole('button', { name: 'Edit message m2' }));
    fireEvent.change(screen.getByLabelText('Edit message m2 content'), {
      target: { value: 'Here is the revised checklist.' }
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save edit for message m2' }));

    await waitFor(() => {
      expect(updateMessage).toHaveBeenCalledWith('conversation_1', 'm2', { content: 'Here is the revised checklist.' });
    });
    expect(await screen.findByText('Here is the revised checklist.')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Bookmark message m2' }));
    await waitFor(() => {
      expect(bookmarkMessage).toHaveBeenCalledWith('conversation_1', 'm2', { bookmarked: true });
    });
    expect(await screen.findByText('Bookmarked')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Share expiration for m2'), {
      target: { value: '2026-06-05T12:00:00Z' }
    });
    fireEvent.click(screen.getByRole('button', { name: 'Share message m2' }));
    await waitFor(() => {
      expect(createMessageShare).toHaveBeenCalledWith('conversation_1', 'm2', {
        expiresAt: '2026-06-05T12:00:00Z'
      });
    });
    expect(await screen.findByText('https://share.example.test/share_1')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Delete message m2' }));
    await waitFor(() => {
      expect(deleteMessage).toHaveBeenCalledWith('conversation_1', 'm2');
    });
    expect(screen.queryByText('Here is the revised checklist.')).not.toBeInTheDocument();
  });

  it('shows a retryable chat workspace error when initial loading fails', async () => {
    routeState.conversationId = 'conversation_1';
    listConversations.mockRejectedValue(new Error('tenant quota ledger unavailable'));
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

    expect(await screen.findByText('Unable to load chat workspace.')).toBeInTheDocument();
    expect(screen.getByText('tenant quota ledger unavailable')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Retry chat workspace' })).toBeInTheDocument();
  });

  it('shows quota and Relay errors when message send fails', async () => {
    mockActiveConversation();
    sendMessageStream.mockRejectedValue(new Error('quota preauthorization failed through Relay'));

    render(<ChatPage />);

    await screen.findByText('Ready when you are.');
    fireEvent.change(screen.getByLabelText('Message draft'), { target: { value: 'Draft a rollout summary.' } });
    fireEvent.click(screen.getByRole('button', { name: 'Send message' }));

    expect(await screen.findByText('Unable to send message.')).toBeInTheDocument();
    expect(await screen.findByText('quota preauthorization failed through Relay')).toBeInTheDocument();
    expect(screen.getByLabelText('Message draft')).toHaveValue('Draft a rollout summary.');
  });

  it('shows SOLO handoff errors without opening a fake handoff', async () => {
    mockActiveConversation();
    convertConversationToTask.mockRejectedValue(new Error('conversation cannot be converted under current Relay policy'));

    render(<ChatPage />);

    await screen.findByRole('button', { name: 'Hand off to SOLO' });
    fireEvent.click(screen.getByRole('button', { name: 'Hand off to SOLO' }));

    expect(await screen.findByText('Unable to prepare SOLO handoff.')).toBeInTheDocument();
    expect(screen.getByText('conversation cannot be converted under current Relay policy')).toBeInTheDocument();
    expect(screen.queryByLabelText('SOLO task goal')).not.toBeInTheDocument();
  });

  it('renders one Convert to SOLO task heading', async () => {
    mockActiveConversation();
    convertConversationToTask.mockResolvedValue({
      draftTaskGoal: 'Draft a launch checklist from this thread.',
      relatedKnowledgeBaseIds: ['kb_1'],
      suggestedBudget: 20,
      suggestedExecutionMode: 'standard'
    });

    render(<ChatPage />);

    await screen.findByRole('button', { name: 'Hand off to SOLO' });
    fireEvent.click(screen.getByRole('button', { name: 'Hand off to SOLO' }));

    await screen.findByLabelText('SOLO task goal');
    expect(screen.getAllByRole('heading', { name: 'Convert to SOLO task' })).toHaveLength(1);
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

  it('loads knowledge base bindings in conversation settings and saves selected knowledge bases', async () => {
    routeState.conversationId = 'conversation_1';
    listConversations.mockResolvedValue([
      {
        id: 'conversation_1',
        title: 'Research thread'
      }
    ]);
    listMessages.mockResolvedValue([]);
    listModels.mockResolvedValue([
      { id: 'balanced-chat', label: 'balanced-chat' },
      { id: 'quality-chat', label: 'quality-chat' }
    ]);
    getConversationConfig.mockResolvedValue({
      conversationId: 'conversation_1',
      knowledgeBaseIds: ['kb_1'],
      maxOutputTokens: 1024,
      modelId: 'balanced-chat',
      systemPromptOverride: '',
      temperature: 1,
      toolsEnabled: false
    });
    listKnowledgeBases.mockResolvedValue([
      {
        documentCount: 3,
        id: 'kb_1',
        name: 'Architecture Notes'
      },
      {
        documentCount: 5,
        id: 'kb_2',
        name: 'Runbooks'
      }
    ]);
    updateConversationConfig.mockResolvedValue({
      conversationId: 'conversation_1',
      knowledgeBaseIds: ['kb_1', 'kb_2'],
      maxOutputTokens: 1024,
      modelId: 'balanced-chat',
      systemPromptOverride: '',
      temperature: 1,
      toolsEnabled: false
    });

    render(<ChatPage />);

    expect(await screen.findByLabelText('Use knowledge base Architecture Notes')).toBeChecked();
    expect(screen.getByLabelText('Use knowledge base Runbooks')).not.toBeChecked();

    fireEvent.click(screen.getByLabelText('Use knowledge base Runbooks'));

    await waitFor(() => {
      expect(updateConversationConfig).toHaveBeenCalledWith('conversation_1', {
        knowledgeBaseIds: ['kb_1', 'kb_2'],
        maxOutputTokens: 1024,
        modelId: 'balanced-chat',
        personaId: '',
        systemPromptOverride: '',
        temperature: 1,
        toolsEnabled: false
      });
    });
  });

  it('converts the current conversation into a solo task and routes into the solo workspace', async () => {
    routeState.conversationId = 'conversation_1';
    listConversations.mockResolvedValue([
      {
        id: 'conversation_1',
        title: 'Research thread'
      }
    ]);
    listMessages.mockResolvedValue([
      { content: 'Draft a launch checklist from this thread.', id: 'message_1', role: 'user' }
    ]);
    listModels.mockResolvedValue([{ id: 'balanced-chat', label: 'balanced-chat' }]);
    getConversationConfig.mockResolvedValue({
      conversationId: 'conversation_1',
      knowledgeBaseIds: ['kb_1'],
      maxOutputTokens: 1024,
      modelId: 'balanced-chat',
      systemPromptOverride: '',
      temperature: 1,
      toolsEnabled: false
    });
    listKnowledgeBases.mockResolvedValue([
      {
        documentCount: 3,
        id: 'kb_1',
        name: 'Architecture Notes'
      },
      {
        documentCount: 2,
        id: 'kb_2',
        name: 'Runbooks'
      }
    ]);
    convertConversationToTask.mockResolvedValue({
      draftTaskGoal: 'Draft a launch checklist from this thread.',
      relatedKnowledgeBaseIds: ['kb_1'],
      suggestedBudget: 20,
      suggestedExecutionMode: 'standard'
    });
    createTask.mockResolvedValue({
      budgetLimit: 20,
      executionMode: 'standard',
      goal: 'Draft a launch checklist from this thread.',
      id: 'task_1',
      knowledgeBaseIds: ['kb_1'],
      status: 'draft',
      title: 'Draft a launch checklist from this thread.'
    });
    startTask.mockResolvedValue({
      budgetLimit: 20,
      executionMode: 'standard',
      goal: 'Draft a launch checklist from this thread.',
      id: 'task_1',
      knowledgeBaseIds: ['kb_1'],
      status: 'running',
      steps: [
        { id: 'step_1', status: 'completed', stepIndex: 1, title: 'Understand the goal' },
        { id: 'step_2', status: 'running', stepIndex: 2, title: 'Review workspace context' }
      ],
      title: 'Draft a launch checklist from this thread.'
    });

    render(<ChatPage />);

    await screen.findByRole('button', { name: 'Hand off to SOLO' });
    fireEvent.click(screen.getByRole('button', { name: 'Hand off to SOLO' }));

    expect(await screen.findByText('Convert to SOLO task')).toBeInTheDocument();
    expect(screen.getByLabelText('SOLO task goal')).toHaveValue('Draft a launch checklist from this thread.');
    expect(screen.getByLabelText('Authorization scope for SOLO')).toHaveValue('workspace_tools');
    expect(screen.getByLabelText('Use knowledge base Architecture Notes in SOLO')).toBeChecked();
    expect(screen.getByLabelText('Use knowledge base Runbooks in SOLO')).not.toBeChecked();
    fireEvent.change(screen.getByLabelText('Authorization scope for SOLO'), { target: { value: 'full_access' } });
    fireEvent.change(screen.getByLabelText('Allowed tools for SOLO'), { target: { value: ' browser, shell ' } });
    fireEvent.change(screen.getByLabelText('Blocked tools for SOLO'), { target: { value: ' email ' } });
    fireEvent.click(screen.getByLabelText('Use knowledge base Runbooks in SOLO'));
    fireEvent.click(screen.getByRole('button', { name: 'Start in SOLO' }));

    await waitFor(() => {
      expect(createTask).toHaveBeenCalledWith({
        authorizationScope: 'full_access',
        budgetLimit: 20,
        executionMode: 'standard',
        goal: 'Draft a launch checklist from this thread.',
        knowledgeBaseIds: ['kb_1', 'kb_2'],
        toolAllowList: ['browser', 'shell'],
        toolDenyList: ['email']
      });
    });
    await waitFor(() => {
      expect(startTask).toHaveBeenCalledWith('task_1');
    });
    expect(navigate).toHaveBeenCalledWith('/solo?taskId=task_1&returnTo=%2Fchat%2Fconversation_1');
  });
});
