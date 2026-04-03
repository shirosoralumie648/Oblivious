import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const createConversation = vi.fn();
const createTask = vi.fn();
const convertConversationToTask = vi.fn();
const getConversationConfig = vi.fn();
const listConversations = vi.fn();
const listMessages = vi.fn();
const listModels = vi.fn();
const startTask = vi.fn();
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
    convertConversationToTask,
    getConversationConfig,
    listConversations,
    listMessages,
    listModels,
    sendMessage,
    updateConversationConfig
  })
}));

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

describe('ChatPage', () => {
  beforeEach(() => {
    createConversation.mockReset();
    createTask.mockReset();
    convertConversationToTask.mockReset();
    getConversationConfig.mockReset();
    listConversations.mockReset();
    listMessages.mockReset();
    listModels.mockReset();
    startTask.mockReset();
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
    expect(navigate).toHaveBeenCalledWith('/solo?taskId=task_1');
  });
});
