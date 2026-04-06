import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const createConversation = vi.fn();
const createTask = vi.fn();
const getConversationConfig = vi.fn();
const navigate = vi.fn();
const sendMessage = vi.fn();
const updateConversationConfig = vi.fn();
const getTask = vi.fn();
const listKnowledgeBases = vi.fn();
const listTasks = vi.fn();
const pauseTask = vi.fn();
const approveTask = vi.fn();
const cancelTask = vi.fn();
const resumeTask = vi.fn();
const startTask = vi.fn();
const updateTaskBudget = vi.fn();

const appContext = vi.hoisted(() => ({
  authState: {
    preferences: {
      defaultMode: 'solo' as const,
      modelStrategy: 'quality',
      networkEnabledHint: true,
      onboardingCompleted: true
    },
    status: 'authenticated' as const,
    user: { email: 'user@example.com', id: 'u1' }
  }
}));

vi.mock('../../app/providers', () => ({
  useAppContext: () => appContext
}));

vi.mock('react-router-dom', () => ({
  useNavigate: () => navigate
}));

vi.mock('../../features/chat/api', () => ({
  createChatApi: () => ({
    createConversation,
    getConversationConfig,
    sendMessage,
    updateConversationConfig
  })
}));

vi.mock('../../features/tasks/api', () => ({
  createTasksApi: () => ({
    approveTask,
    cancelTask,
    createTask,
    getTask,
    listTasks,
    pauseTask,
    resumeTask,
    startTask,
    updateTaskBudget
  })
}));

vi.mock('../../features/knowledge/api', () => ({
  createKnowledgeApi: () => ({
    listKnowledgeBases
  })
}));

import { SoloPage } from './SoloPage';

describe('SoloPage', () => {
  beforeEach(() => {
    appContext.authState.preferences = {
      defaultMode: 'solo',
      modelStrategy: 'quality',
      networkEnabledHint: true,
      onboardingCompleted: true
    };
    createConversation.mockReset();
    createTask.mockReset();
    getConversationConfig.mockReset();
    navigate.mockReset();
    sendMessage.mockReset();
    updateConversationConfig.mockReset();
    getTask.mockReset();
    listKnowledgeBases.mockReset();
    listTasks.mockReset();
    pauseTask.mockReset();
    approveTask.mockReset();
    cancelTask.mockReset();
    resumeTask.mockReset();
    startTask.mockReset();
    updateTaskBudget.mockReset();
    window.history.replaceState({}, '', '/solo');
  });

  afterEach(() => {
    createConversation.mockReset();
    createTask.mockReset();
    getConversationConfig.mockReset();
    navigate.mockReset();
    sendMessage.mockReset();
    updateConversationConfig.mockReset();
    getTask.mockReset();
    listKnowledgeBases.mockReset();
    listTasks.mockReset();
    pauseTask.mockReset();
    approveTask.mockReset();
    cancelTask.mockReset();
    resumeTask.mockReset();
    startTask.mockReset();
    updateTaskBudget.mockReset();
    window.history.replaceState({}, '', '/solo');
  });

  it('loads and renders the solo launch context with recent tasks', async () => {
    listTasks.mockResolvedValue([
      { executionMode: 'standard', goal: 'Review launch plan', id: 'task_2', status: 'completed', title: 'Review launch plan' },
      { executionMode: 'safe', goal: 'Summarize notes', id: 'task_1', status: 'draft', title: 'Summarize notes' }
    ]);
    listKnowledgeBases.mockResolvedValue([
      { documentCount: 3, id: 'kb_1', name: 'Product Docs' }
    ]);

    render(<SoloPage />);

    expect(screen.getByText('Loading solo workspace…')).toBeInTheDocument();
    expect(await screen.findByText('Model strategy: quality')).toBeInTheDocument();
    expect(screen.getByText('Web suggestions: Enabled')).toBeInTheDocument();
    expect(screen.getByText('Review launch plan')).toBeInTheDocument();
    expect(screen.getByLabelText('Use knowledge base Product Docs')).toBeInTheDocument();
  });

  it('groups tasks by status on the solo home screen', async () => {
    listTasks.mockResolvedValue([
      { executionMode: 'standard', goal: 'Watch live rollout', id: 'task_running', status: 'running', title: 'Watch live rollout' },
      { executionMode: 'standard', goal: 'Review launch plan', id: 'task_done', status: 'completed', title: 'Review launch plan' },
      { executionMode: 'safe', goal: 'Abort risky task', id: 'task_cancelled', status: 'cancelled', title: 'Abort risky task' }
    ]);
    listKnowledgeBases.mockResolvedValue([]);

    render(<SoloPage />);

    expect(await screen.findByText('Running tasks')).toBeInTheDocument();
    expect(screen.getByText('Completed tasks')).toBeInTheDocument();
    expect(screen.getByText('Stopped tasks')).toBeInTheDocument();
    expect(screen.queryByText('Recent tasks')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Open task Watch live rollout' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Open task Review launch plan' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Open task Abort risky task' })).toBeInTheDocument();
  });

  it('renders a dedicated task creation view on /solo/new', async () => {
    window.history.replaceState({}, '', '/solo/new');
    listTasks.mockResolvedValue([
      { authorizationScope: 'workspace_tools', budgetLimit: 8, executionMode: 'standard', goal: 'Review launch plan', id: 'task_2', status: 'running', title: 'Review launch plan' }
    ]);
    listKnowledgeBases.mockResolvedValue([]);

    render(<SoloPage />);

    expect(await screen.findByRole('heading', { name: 'New SOLO task' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Back to tasks' })).toBeInTheDocument();
    expect(screen.queryByText('Running tasks')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Back to tasks' }));

    expect(navigate).toHaveBeenCalledWith('/solo');
  });

  it('creates and starts a solo task, then renders the live execution state', async () => {
    listTasks.mockResolvedValue([]);
    listKnowledgeBases.mockResolvedValue([
      { documentCount: 4, id: 'kb_1', name: 'Research Vault' }
    ]);
    createTask.mockResolvedValue({
      budgetLimit: 20,
      budgetConsumed: 0,
      createdAt: '2026-04-03T10:00:00Z',
      executionMode: 'safe',
      finishedAt: undefined,
      goal: 'Draft launch checklist',
      id: 'task_new',
      knowledgeBaseIds: ['kb_1'],
      startedAt: undefined,
      status: 'draft',
      title: 'Draft launch checklist'
    });
    startTask.mockResolvedValue({
      budgetLimit: 20,
      budgetConsumed: 6,
      createdAt: '2026-04-03T10:00:00Z',
      currentStep: 'Review workspace context',
      executionMode: 'safe',
      events: [
        { createdAt: '2026-04-03T10:01:00Z', message: 'Task execution started', type: 'started' },
        { createdAt: '2026-04-03T10:02:00Z', message: 'Executing Review workspace context', type: 'running' }
      ],
      finishedAt: undefined,
      goal: 'Draft launch checklist',
      id: 'task_new',
      knowledgeBaseIds: ['kb_1'],
      startedAt: '2026-04-03T10:01:00Z',
      status: 'running',
      steps: [
        { id: 'step_1', status: 'completed', stepIndex: 1, title: 'Understand the goal' },
        { id: 'step_2', status: 'running', stepIndex: 2, title: 'Review workspace context' },
        { id: 'step_3', status: 'pending', stepIndex: 3, title: 'Deliver starter result' }
      ],
      title: 'Draft launch checklist'
    });

    render(<SoloPage />);

    await screen.findByRole('button', { name: 'Start solo run' });
    fireEvent.change(screen.getByLabelText('Task goal'), { target: { value: 'Draft launch checklist' } });
    fireEvent.click(screen.getByLabelText('Use knowledge base Research Vault'));
    fireEvent.change(screen.getByLabelText('Execution mode'), { target: { value: 'safe' } });
    fireEvent.change(screen.getByLabelText('Budget limit'), { target: { value: '20' } });
    fireEvent.click(screen.getByRole('button', { name: 'Start solo run' }));

    await waitFor(() => {
      expect(createTask).toHaveBeenCalledWith({
        authorizationScope: 'workspace_tools',
        budgetLimit: 20,
        executionMode: 'safe',
        goal: 'Draft launch checklist',
        knowledgeBaseIds: ['kb_1']
      });
    });
    await waitFor(() => {
      expect(startTask).toHaveBeenCalledWith('task_new');
    });
    expect(screen.getByText('Status: running')).toBeInTheDocument();
    expect(screen.getByText('Budget consumed: 6 / 20')).toBeInTheDocument();
    expect(screen.getByText('Started at: 2026-04-03T10:01:00Z')).toBeInTheDocument();
    expect(screen.getByText('Current knowledge sources')).toBeInTheDocument();
    expect(screen.getByText('Research Vault')).toBeInTheDocument();
    expect(screen.getByText('Current step: Review workspace context')).toBeInTheDocument();
    expect(screen.getByText('Executing Review workspace context')).toBeInTheDocument();
    expect(screen.getByText('Understand the goal')).toBeInTheDocument();
    expect(screen.getByText('Review workspace context')).toBeInTheDocument();
    expect(screen.getByText('Deliver starter result')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Pause run' })).toBeInTheDocument();
  });

  it('creates a solo task with tool boundaries and shows enabled tools during execution', async () => {
    listTasks.mockResolvedValue([]);
    listKnowledgeBases.mockResolvedValue([]);
    createTask.mockResolvedValue({
      authorizationScope: 'workspace_tools',
      budgetLimit: 12,
      budgetConsumed: 0,
      createdAt: '2026-04-03T10:00:00Z',
      executionMode: 'standard',
      goal: 'Investigate deployment blockers',
      id: 'task_tools',
      status: 'draft',
      title: 'Investigate deployment blockers'
    });
    startTask.mockResolvedValue({
      authorizationScope: 'workspace_tools',
      budgetLimit: 12,
      budgetConsumed: 3,
      createdAt: '2026-04-03T10:00:00Z',
      executionMode: 'standard',
      goal: 'Investigate deployment blockers',
      id: 'task_tools',
      knowledgeBaseIds: [],
      startedAt: '2026-04-03T10:01:00Z',
      status: 'running',
      steps: [
        { id: 'step_1', status: 'completed', stepIndex: 1, title: 'Understand the goal' },
        { id: 'step_2', status: 'running', stepIndex: 2, title: 'Review workspace context' }
      ],
      title: 'Investigate deployment blockers',
      toolAllowList: ['browser', 'shell'],
      toolDenyList: ['email']
    });

    render(<SoloPage />);

    await screen.findByRole('button', { name: 'Start solo run' });
    fireEvent.change(screen.getByLabelText('Task goal'), { target: { value: 'Investigate deployment blockers' } });
    fireEvent.change(screen.getByLabelText('Allowed tools'), { target: { value: ' browser, shell, browser ' } });
    fireEvent.change(screen.getByLabelText('Blocked tools'), { target: { value: ' email ' } });
    fireEvent.click(screen.getByRole('button', { name: 'Start solo run' }));

    await waitFor(() => {
      expect(createTask).toHaveBeenCalledWith({
        authorizationScope: 'workspace_tools',
        budgetLimit: 10,
        executionMode: 'standard',
        goal: 'Investigate deployment blockers',
        knowledgeBaseIds: [],
        toolAllowList: ['browser', 'shell'],
        toolDenyList: ['email']
      });
    });
    expect(await screen.findByRole('heading', { name: 'Current enabled tools' })).toBeInTheDocument();
    expect(screen.getByText('browser')).toBeInTheDocument();
    expect(screen.getByText('shell')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Blocked tools' })).toBeInTheDocument();
    expect(screen.getByText('email')).toBeInTheDocument();
  });

  it('waits for approval before continuing a safe solo task', async () => {
    listTasks.mockResolvedValue([]);
    listKnowledgeBases.mockResolvedValue([]);
    createTask.mockResolvedValue({
      authorizationScope: 'full_access',
      budgetLimit: 20,
      budgetConsumed: 0,
      createdAt: '2026-04-03T10:00:00Z',
      executionMode: 'safe',
      goal: 'Draft vendor outreach plan',
      id: 'task_safe',
      status: 'draft',
      title: 'Draft vendor outreach plan'
    });
    startTask.mockResolvedValue({
      authorizationScope: 'full_access',
      budgetLimit: 20,
      budgetConsumed: 0,
      createdAt: '2026-04-03T10:00:00Z',
      executionMode: 'safe',
      goal: 'Draft vendor outreach plan',
      id: 'task_safe',
      knowledgeBaseIds: [],
      startedAt: '2026-04-03T10:01:00Z',
      status: 'awaiting_confirmation',
      steps: [
        { id: 'step_1', status: 'completed', stepIndex: 1, title: 'Understand the goal' },
        { id: 'step_2', status: 'awaiting_confirmation', stepIndex: 2, title: 'Confirm execution boundary' },
        { id: 'step_3', status: 'pending', stepIndex: 3, title: 'Deliver starter result' }
      ],
      title: 'Draft vendor outreach plan'
    });
    approveTask.mockResolvedValue({
      authorizationScope: 'full_access',
      budgetLimit: 20,
      budgetConsumed: 5,
      createdAt: '2026-04-03T10:00:00Z',
      executionMode: 'safe',
      goal: 'Draft vendor outreach plan',
      id: 'task_safe',
      knowledgeBaseIds: [],
      startedAt: '2026-04-03T10:01:00Z',
      status: 'running',
      steps: [
        { id: 'step_1', status: 'completed', stepIndex: 1, title: 'Understand the goal' },
        { id: 'step_2', status: 'completed', stepIndex: 2, title: 'Confirm execution boundary' },
        { id: 'step_3', status: 'running', stepIndex: 3, title: 'Deliver starter result' }
      ],
      title: 'Draft vendor outreach plan'
    });

    render(<SoloPage />);

    await screen.findByRole('button', { name: 'Start solo run' });
    fireEvent.change(screen.getByLabelText('Task goal'), { target: { value: 'Draft vendor outreach plan' } });
    fireEvent.change(screen.getByLabelText('Execution mode'), { target: { value: 'safe' } });
    fireEvent.change(screen.getByLabelText('Authorization scope'), { target: { value: 'full_access' } });
    fireEvent.change(screen.getByLabelText('Budget limit'), { target: { value: '20' } });
    fireEvent.click(screen.getByRole('button', { name: 'Start solo run' }));

    await waitFor(() => {
      expect(createTask).toHaveBeenCalledWith({
        authorizationScope: 'full_access',
        budgetLimit: 20,
        executionMode: 'safe',
        goal: 'Draft vendor outreach plan',
        knowledgeBaseIds: []
      });
    });
    await screen.findByText('Status: awaiting_confirmation');
    expect(screen.getByText('Authorization scope: full_access')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Approve plan' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Approve plan' }));

    await waitFor(() => {
      expect(approveTask).toHaveBeenCalledWith('task_safe');
    });
    expect(screen.getByText('Status: running')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Pause run' })).toBeInTheDocument();
  });

  it('loads an existing recent task detail when selected', async () => {
    listTasks.mockResolvedValue([
      { budgetLimit: 12, executionMode: 'standard', goal: 'Review launch plan', id: 'task_2', status: 'completed', title: 'Review launch plan' }
    ]);
    listKnowledgeBases.mockResolvedValue([]);
    getTask.mockResolvedValue({
      budgetLimit: 12,
      executionMode: 'standard',
      goal: 'Review launch plan',
      id: 'task_2',
      knowledgeBaseIds: ['kb_2'],
      resultSummary: 'Completed a starter SOLO run for: Review launch plan',
      status: 'completed',
      steps: [
        { id: 'step_1', status: 'completed', stepIndex: 1, title: 'Understand the goal' }
      ],
      title: 'Review launch plan'
    });

    render(<SoloPage />);

    await screen.findByText('Review launch plan');
    fireEvent.click(screen.getByRole('button', { name: 'Open task Review launch plan' }));

    await waitFor(() => {
      expect(getTask).toHaveBeenCalledWith('task_2');
    });
    expect(screen.getByText('Completed a starter SOLO run for: Review launch plan')).toBeInTheDocument();
    expect(screen.getByText('Understand the goal')).toBeInTheDocument();
  });

  it('opens the task from the taskId query param on initial load', async () => {
    window.history.replaceState({}, '', '/solo?taskId=task_2');
    listTasks.mockResolvedValue([
      { budgetLimit: 12, executionMode: 'standard', goal: 'Review launch plan', id: 'task_2', status: 'completed', title: 'Review launch plan' }
    ]);
    listKnowledgeBases.mockResolvedValue([]);
    getTask.mockResolvedValue({
      budgetLimit: 12,
      executionMode: 'standard',
      goal: 'Review launch plan',
      id: 'task_2',
      knowledgeBaseIds: ['kb_2'],
      resultSummary: 'Completed a starter SOLO run for: Review launch plan',
      status: 'completed',
      steps: [
        { id: 'step_1', status: 'completed', stepIndex: 1, title: 'Understand the goal' }
      ],
      title: 'Review launch plan'
    });

    render(<SoloPage />);

    await waitFor(() => {
      expect(getTask).toHaveBeenCalledWith('task_2');
    });
    expect(screen.getByText('Completed a starter SOLO run for: Review launch plan')).toBeInTheDocument();
    expect(screen.getByText('Understand the goal')).toBeInTheDocument();
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
    fireEvent.click(screen.getByRole('button', { name: 'Back to chat' }));

    expect(navigate).toHaveBeenCalledWith('/chat/conversation_1');
  });

  it('pauses and resumes a running task from the execution view', async () => {
    listTasks.mockResolvedValue([]);
    listKnowledgeBases.mockResolvedValue([]);
    startTask.mockResolvedValue({
      budgetLimit: 8,
      currentStep: 'Review workspace context',
      executionMode: 'standard',
      events: [
        { createdAt: '2026-04-03T10:01:00Z', message: 'Executing Review workspace context', type: 'running' }
      ],
      goal: 'Review launch plan',
      id: 'task_1',
      knowledgeBaseIds: [],
      status: 'running',
      steps: [
        { id: 'step_1', status: 'completed', stepIndex: 1, title: 'Understand the goal' },
        { id: 'step_2', status: 'running', stepIndex: 2, title: 'Review workspace context' }
      ],
      title: 'Review launch plan'
    });
    createTask.mockResolvedValue({
      budgetLimit: 8,
      executionMode: 'standard',
      goal: 'Review launch plan',
      id: 'task_1',
      knowledgeBaseIds: [],
      status: 'draft',
      title: 'Review launch plan'
    });
    pauseTask.mockResolvedValue({
      budgetLimit: 8,
      currentStep: 'Review workspace context',
      executionMode: 'standard',
      events: [
        { createdAt: '2026-04-03T10:02:00Z', message: 'Execution paused at Review workspace context', type: 'paused' }
      ],
      goal: 'Review launch plan',
      id: 'task_1',
      knowledgeBaseIds: [],
      status: 'paused',
      steps: [
        { id: 'step_1', status: 'completed', stepIndex: 1, title: 'Understand the goal' },
        { id: 'step_2', status: 'paused', stepIndex: 2, title: 'Review workspace context' },
        { id: 'step_3', status: 'pending', stepIndex: 3, title: 'Deliver runtime result' }
      ],
      title: 'Review launch plan'
    });
    resumeTask.mockResolvedValue({
      budgetLimit: 8,
      currentStep: 'Review workspace context',
      executionMode: 'standard',
      events: [
        { createdAt: '2026-04-03T10:03:00Z', message: 'Execution resumed', type: 'resumed' },
        { createdAt: '2026-04-03T10:04:00Z', message: 'Executing Review workspace context', type: 'running' }
      ],
      goal: 'Review launch plan',
      id: 'task_1',
      knowledgeBaseIds: [],
      status: 'running',
      steps: [
        { id: 'step_1', status: 'completed', stepIndex: 1, title: 'Understand the goal' },
        { id: 'step_2', status: 'running', stepIndex: 2, title: 'Review workspace context' },
        { id: 'step_3', status: 'pending', stepIndex: 3, title: 'Deliver runtime result' }
      ],
      title: 'Review launch plan'
    });

    render(<SoloPage />);

    await screen.findByRole('button', { name: 'Start solo run' });
    fireEvent.change(screen.getByLabelText('Task goal'), { target: { value: 'Review launch plan' } });
    fireEvent.click(screen.getByRole('button', { name: 'Start solo run' }));
    await screen.findByText('Status: running');

    fireEvent.click(screen.getByRole('button', { name: 'Pause run' }));
    await waitFor(() => {
      expect(pauseTask).toHaveBeenCalledWith('task_1');
    });
    expect(screen.getByText('Status: paused')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Resume run' }));
    await waitFor(() => {
      expect(resumeTask).toHaveBeenCalledWith('task_1');
    });
    expect(screen.getByText('Status: running')).toBeInTheDocument();
    expect(screen.getByText('Execution resumed')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Continue run' })).toBeInTheDocument();
  });

  it('continues a running task to a completed runtime result', async () => {
    listTasks.mockResolvedValue([]);
    listKnowledgeBases.mockResolvedValue([]);
    createTask.mockResolvedValue({
      budgetLimit: 12,
      executionMode: 'standard',
      goal: 'Review launch plan',
      id: 'task_runtime',
      knowledgeBaseIds: [],
      status: 'draft',
      title: 'Review launch plan'
    });
    startTask
      .mockResolvedValueOnce({
        budgetLimit: 12,
        budgetConsumed: 8,
        currentStep: 'Deliver runtime result',
        executionMode: 'standard',
        events: [
          { createdAt: '2026-04-03T10:01:00Z', message: 'Executing Deliver runtime result', type: 'running' }
        ],
        goal: 'Review launch plan',
        id: 'task_runtime',
        knowledgeBaseIds: ['kb_2'],
        status: 'running',
        steps: [
          { id: 'step_1', status: 'completed', stepIndex: 1, title: 'Understand the goal' },
          { id: 'step_2', status: 'completed', stepIndex: 2, title: 'Review workspace context' },
          { id: 'step_3', status: 'running', stepIndex: 3, title: 'Deliver runtime result' }
        ],
        title: 'Review launch plan'
      })
      .mockResolvedValueOnce({
        budgetLimit: 12,
        budgetConsumed: 12,
        executionMode: 'standard',
        events: [
          { createdAt: '2026-04-03T10:05:00Z', message: 'Runtime execution completed', type: 'completed' }
        ],
        goal: 'Review launch plan',
        id: 'task_runtime',
        knowledgeBaseIds: ['kb_2'],
        resultArtifacts: [
          { label: 'Completed steps', value: '3 / 3' },
          { label: 'Budget usage', value: '12 / 12' },
          { label: 'Knowledge sources', value: '1' }
        ],
        resultSummary: 'Runtime result for "Review launch plan"\nCompleted steps: 3 / 3',
        status: 'completed',
        steps: [
          { id: 'step_1', status: 'completed', stepIndex: 1, title: 'Understand the goal' },
          { id: 'step_2', status: 'completed', stepIndex: 2, title: 'Review workspace context' },
          { id: 'step_3', status: 'completed', stepIndex: 3, title: 'Deliver runtime result' }
        ],
        title: 'Review launch plan'
      });

    render(<SoloPage />);

    await screen.findByRole('button', { name: 'Start solo run' });
    fireEvent.change(screen.getByLabelText('Task goal'), { target: { value: 'Review launch plan' } });
    fireEvent.click(screen.getByRole('button', { name: 'Start solo run' }));

    await screen.findByText('Status: running');
    fireEvent.click(screen.getByRole('button', { name: 'Continue run' }));

    await waitFor(() => {
      expect(startTask).toHaveBeenNthCalledWith(2, 'task_runtime');
    });
    expect(screen.getByText('Status: completed')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Result artifacts' })).toBeInTheDocument();
    expect(screen.getByText('Completed steps')).toBeInTheDocument();
    expect(screen.getByText('3 / 3')).toBeInTheDocument();
    expect(screen.getByText('Runtime execution completed')).toBeInTheDocument();
  });

  it('updates budget from the execution view', async () => {
    listTasks.mockResolvedValue([
      { authorizationScope: 'workspace_tools', budgetLimit: 8, executionMode: 'standard', goal: 'Review launch plan', id: 'task_2', status: 'running', title: 'Review launch plan' }
    ]);
    listKnowledgeBases.mockResolvedValue([]);
    getTask.mockResolvedValue({
      authorizationScope: 'workspace_tools',
      budgetConsumed: 4,
      budgetLimit: 8,
      executionMode: 'standard',
      goal: 'Review launch plan',
      id: 'task_2',
      knowledgeBaseIds: [],
      status: 'running',
      steps: [
        { id: 'step_1', status: 'completed', stepIndex: 1, title: 'Understand the goal' },
        { id: 'step_2', status: 'running', stepIndex: 2, title: 'Review workspace context' }
      ],
      title: 'Review launch plan'
    });
    updateTaskBudget.mockResolvedValue({
      authorizationScope: 'workspace_tools',
      budgetConsumed: 4,
      budgetLimit: 15,
      executionMode: 'standard',
      goal: 'Review launch plan',
      id: 'task_2',
      knowledgeBaseIds: [],
      status: 'running',
      steps: [
        { id: 'step_1', status: 'completed', stepIndex: 1, title: 'Understand the goal' },
        { id: 'step_2', status: 'running', stepIndex: 2, title: 'Review workspace context' }
      ],
      title: 'Review launch plan'
    });

    render(<SoloPage />);

    await screen.findByText('Review launch plan');
    fireEvent.click(screen.getByRole('button', { name: 'Open task Review launch plan' }));
    await screen.findByText('Status: running');

    fireEvent.change(screen.getByLabelText('Active budget limit'), { target: { value: '15' } });
    fireEvent.click(screen.getByRole('button', { name: 'Update budget' }));

    await waitFor(() => {
      expect(updateTaskBudget).toHaveBeenCalledWith('task_2', { budgetLimit: 15 });
    });
    expect(screen.getByText('Budget consumed: 4 / 15')).toBeInTheDocument();
  });

  it('retries a completed task from the result view', async () => {
    listTasks.mockResolvedValue([
      { budgetLimit: 12, executionMode: 'standard', goal: 'Review launch plan', id: 'task_2', status: 'completed', title: 'Review launch plan' }
    ]);
    listKnowledgeBases.mockResolvedValue([]);
    getTask.mockResolvedValue({
      budgetLimit: 12,
      executionMode: 'standard',
      goal: 'Review launch plan',
      id: 'task_2',
      knowledgeBaseIds: ['kb_2'],
      resultSummary: 'Completed a starter SOLO run for: Review launch plan',
      status: 'completed',
      steps: [
        { id: 'step_1', status: 'completed', stepIndex: 1, title: 'Understand the goal' }
      ],
      title: 'Review launch plan'
    });
    startTask.mockResolvedValue({
      budgetLimit: 12,
      executionMode: 'standard',
      goal: 'Review launch plan',
      id: 'task_2',
      knowledgeBaseIds: ['kb_2'],
      status: 'running',
      steps: [
        { id: 'step_1', status: 'completed', stepIndex: 1, title: 'Understand the goal' },
        { id: 'step_2', status: 'running', stepIndex: 2, title: 'Review workspace context' },
        { id: 'step_3', status: 'pending', stepIndex: 3, title: 'Deliver starter result' }
      ],
      title: 'Review launch plan'
    });

    render(<SoloPage />);

    await screen.findByText('Review launch plan');
    fireEvent.click(screen.getByRole('button', { name: 'Open task Review launch plan' }));
    await screen.findByText('Status: completed');

    fireEvent.click(screen.getByRole('button', { name: 'Retry run' }));

    await waitFor(() => {
      expect(startTask).toHaveBeenCalledWith('task_2');
    });
    expect(screen.getByText('Status: running')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Pause run' })).toBeInTheDocument();
  });

  it('continues a completed solo result in chat', async () => {
    listTasks.mockResolvedValue([
      { budgetLimit: 12, executionMode: 'standard', goal: 'Review launch plan', id: 'task_2', status: 'completed', title: 'Review launch plan' }
    ]);
    listKnowledgeBases.mockResolvedValue([]);
    getTask.mockResolvedValue({
      budgetLimit: 12,
      executionMode: 'standard',
      goal: 'Review launch plan',
      id: 'task_2',
      knowledgeBaseIds: ['kb_2'],
      resultSummary: 'Completed a starter SOLO run for: Review launch plan',
      status: 'completed',
      steps: [
        { id: 'step_1', status: 'completed', stepIndex: 1, title: 'Understand the goal' }
      ],
      title: 'Review launch plan'
    });
    createConversation.mockResolvedValue({
      id: 'conversation_2',
      title: 'Review launch plan'
    });
    getConversationConfig.mockResolvedValue({
      conversationId: 'conversation_2',
      knowledgeBaseIds: [],
      maxOutputTokens: 1024,
      modelId: 'balanced-chat',
      systemPromptOverride: '',
      temperature: 1,
      toolsEnabled: false
    });
    updateConversationConfig.mockResolvedValue({
      conversationId: 'conversation_2',
      knowledgeBaseIds: ['kb_2'],
      maxOutputTokens: 1024,
      modelId: 'balanced-chat',
      systemPromptOverride: '',
      temperature: 1,
      toolsEnabled: false
    });
    sendMessage.mockResolvedValue([
      { content: 'Continue from this SOLO result.', id: 'message_1', role: 'user' },
      { content: 'What should we do next?', id: 'message_2', role: 'assistant' }
    ]);

    render(<SoloPage />);

    await screen.findByText('Review launch plan');
    fireEvent.click(screen.getByRole('button', { name: 'Open task Review launch plan' }));
    await screen.findByText('Status: completed');

    fireEvent.click(screen.getByRole('button', { name: 'Continue in Chat' }));

    await waitFor(() => {
      expect(createConversation).toHaveBeenCalledWith({ title: 'Review launch plan' });
    });
    await waitFor(() => {
      expect(getConversationConfig).toHaveBeenCalledWith('conversation_2');
    });
    await waitFor(() => {
      expect(updateConversationConfig).toHaveBeenCalledWith('conversation_2', {
        knowledgeBaseIds: ['kb_2'],
        maxOutputTokens: 1024,
        modelId: 'balanced-chat',
        systemPromptOverride: '',
        temperature: 1,
        toolsEnabled: false
      });
    });
    await waitFor(() => {
      expect(sendMessage).toHaveBeenCalledWith(
        'conversation_2',
        {
          content:
            'Continue from this SOLO result.\nGoal: Review launch plan\nResult: Completed a starter SOLO run for: Review launch plan'
        }
      );
    });
    expect(navigate).toHaveBeenCalledWith('/chat/conversation_2');
  });

  it('exports a completed solo result', async () => {
    const createObjectURL = vi.fn(() => 'blob:solo-result');
    const revokeObjectURL = vi.fn();
    const originalCreateObjectURL = URL.createObjectURL;
    const originalRevokeObjectURL = URL.revokeObjectURL;
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: createObjectURL
    });
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      value: revokeObjectURL
    });

    listTasks.mockResolvedValue([
      { budgetConsumed: 12, budgetLimit: 12, executionMode: 'standard', goal: 'Review launch plan', id: 'task_2', status: 'completed', title: 'Review launch plan' }
    ]);
    listKnowledgeBases.mockResolvedValue([]);
    getTask.mockResolvedValue({
      budgetConsumed: 12,
      budgetLimit: 12,
      executionMode: 'standard',
      finishedAt: '2026-04-03T18:30:00Z',
      goal: 'Review launch plan',
      id: 'task_2',
      knowledgeBaseIds: ['kb_2'],
      resultSummary: 'Completed a starter SOLO run for: Review launch plan',
      startedAt: '2026-04-03T18:00:00Z',
      status: 'completed',
      steps: [
        { id: 'step_1', status: 'completed', stepIndex: 1, title: 'Understand the goal' }
      ],
      title: 'Review launch plan'
    });

    render(<SoloPage />);

    await screen.findByText('Review launch plan');
    fireEvent.click(screen.getByRole('button', { name: 'Open task Review launch plan' }));
    await screen.findByText('Status: completed');

    fireEvent.click(screen.getByRole('button', { name: 'Export result' }));

    await waitFor(() => {
      expect(createObjectURL).toHaveBeenCalledTimes(1);
    });
    const exportCalls = createObjectURL.mock.calls as unknown[][];
    const exportedBlob = exportCalls[0]?.[0];
    expect(exportedBlob).toBeInstanceOf(Blob);
    expect(clickSpy).toHaveBeenCalledTimes(1);
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:solo-result');

    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: originalCreateObjectURL
    });
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      value: originalRevokeObjectURL
    });
    clickSpy.mockRestore();
  });
});
