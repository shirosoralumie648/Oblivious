import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const createConversation = vi.fn();
const getConversationConfig = vi.fn();
const listConversations = vi.fn();
const listMessages = vi.fn();
const listModels = vi.fn();
const sendMessage = vi.fn();
const updateConversationConfig = vi.fn();
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

vi.mock('../../features/chat/api', () => ({
  createChatApi: () => ({
    createConversation,
    getConversationConfig,
    listConversations,
    listMessages,
    listModels,
    sendMessage,
    updateConversationConfig
  })
}));

vi.mock('../../features/knowledge/api', () => ({
  createKnowledgeApi: () => ({
    listKnowledgeBases
  })
}));

import { ChatPage } from './ChatPage';

describe('ChatPage', () => {
  beforeEach(() => {
    createConversation.mockReset();
    getConversationConfig.mockReset();
    listConversations.mockReset();
    listMessages.mockReset();
    listModels.mockReset();
    sendMessage.mockReset();
    updateConversationConfig.mockReset();
    listKnowledgeBases.mockReset();
    navigate.mockReset();
    routeState.conversationId = undefined;
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
        systemPromptOverride: '',
        temperature: 1,
        toolsEnabled: false
      });
    });
  });
});
