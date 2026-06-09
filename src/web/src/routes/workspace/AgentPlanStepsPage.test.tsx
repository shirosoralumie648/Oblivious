import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const approvePlanStep = vi.fn();
const approveToolRun = vi.fn();
const continueRunWithBudget = vi.fn();
const createPlanStep = vi.fn();
const deletePlanStep = vi.fn();
const executePlanStep = vi.fn();
const getRunDetail = vi.fn();
const movePlanStep = vi.fn();
const rejectToolRun = vi.fn();
const retryPlanStep = vi.fn();
const retryToolRun = vi.fn();
const skipPlanStep = vi.fn();
const updatePlanStep = vi.fn();

vi.mock('../../features/agents/planStepsApi', () => ({
  createAgentPlanStepsApi: () => ({
    approvePlanStep,
    approveToolRun,
    continueRunWithBudget,
    createPlanStep,
    deletePlanStep,
    executePlanStep,
    getRunDetail,
    movePlanStep,
    rejectToolRun,
    retryPlanStep,
    retryToolRun,
    skipPlanStep,
    updatePlanStep
  })
}));

import { AgentPlanStepsPage } from './AgentPlanStepsPage';
import { routerFuture } from '../../app/routerFuture';

function renderPage(planSteps: unknown[] = []) {
  render(
    <MemoryRouter
      future={routerFuture}
      initialEntries={[
        {
          pathname: '/agent-runs/run_1/plan-steps',
          state: { planSteps }
        }
      ]}
    >
      <Routes>
        <Route path="/agent-runs/:runId/plan-steps" element={<AgentPlanStepsPage />} />
      </Routes>
    </MemoryRouter>
  );
}

function renderDirectPage() {
  render(
    <MemoryRouter
      future={routerFuture}
      initialEntries={[
        {
          pathname: '/agent-runs/run_1/plan-steps'
        }
      ]}
    >
      <Routes>
        <Route path="/agent-runs/:runId/plan-steps" element={<AgentPlanStepsPage />} />
      </Routes>
    </MemoryRouter>
  );
}

function runDetail(
  planSteps: unknown[],
  {
    error,
    iterationCount = 2,
    mode = 'planning',
    status = 'planning',
    toolCallCount = 0,
    toolRuns = []
  }: {
    error?: string;
    iterationCount?: number;
    mode?: string;
    status?: string;
    toolCallCount?: number;
    toolRuns?: unknown[];
  } = {}
) {
  return {
    ...(error !== undefined ? { error } : {}),
    id: 'run_1',
    iterationCount,
    mode,
    planSteps,
    status,
    toolCallCount,
    toolRuns
  };
}

describe('AgentPlanStepsPage', () => {
  beforeEach(() => {
    approvePlanStep.mockReset();
    approveToolRun.mockReset();
    continueRunWithBudget.mockReset();
    createPlanStep.mockReset();
    deletePlanStep.mockReset();
    executePlanStep.mockReset();
    getRunDetail.mockReset();
    movePlanStep.mockReset();
    rejectToolRun.mockReset();
    retryPlanStep.mockReset();
    retryToolRun.mockReset();
    skipPlanStep.mockReset();
    updatePlanStep.mockReset();
  });

  it('loads run detail on direct entry and refreshes status with plan steps', async () => {
    getRunDetail
      .mockResolvedValueOnce({
        id: 'run_1',
        iterationCount: 2,
        mode: 'planning',
        planSteps: [
          {
            approvalStatus: 'pending',
            id: 'step_1',
            index: 1,
            runId: 'run_1',
            status: 'pending',
            title: 'Inspect workspace'
          }
        ],
        status: 'planning',
        toolCallCount: 1
      })
      .mockResolvedValueOnce({
        error: 'tool loop exceeded max iterations',
        id: 'run_1',
        iterationCount: 10,
        mode: 'planning',
        planSteps: [
          {
            approvalStatus: 'approved',
            id: 'step_1',
            index: 1,
            runId: 'run_1',
            status: 'approved',
            title: 'Inspect workspace'
          },
          {
            approvalStatus: 'not_required',
            id: 'step_2',
            index: 2,
            runId: 'run_1',
            status: 'pending',
            title: 'Patch web page'
          }
        ],
        status: 'max_iterations_reached',
        toolCallCount: 7
      });

    renderDirectPage();

    expect(await screen.findByText('Status: planning')).toBeInTheDocument();
    expect(screen.getByLabelText('Agent run execution controls')).toHaveTextContent('Mode planning');
    expect(screen.getByLabelText('Agent run execution controls')).toHaveTextContent('Iterations 2');
    expect(screen.getByLabelText('Agent run execution controls')).toHaveTextContent('Tool calls 1');
    expect(screen.getByRole('heading', { name: 'Inspect workspace' })).toBeInTheDocument();
    expect(getRunDetail).toHaveBeenCalledWith('run_1');

    fireEvent.click(screen.getByRole('button', { name: 'Refresh plan steps' }));

    expect(await screen.findByText('Status: max_iterations_reached')).toBeInTheDocument();
    expect(screen.getByLabelText('Agent run execution controls')).toHaveTextContent('Iterations 10');
    expect(screen.getByLabelText('Agent run execution controls')).toHaveTextContent('Tool calls 7');
    expect(screen.getByLabelText('Agent run execution controls')).toHaveTextContent('Stop reason tool loop exceeded max iterations');
    expect(screen.getByRole('heading', { name: 'Patch web page' })).toBeInTheDocument();
    expect(getRunDetail).toHaveBeenCalledTimes(2);
  });

  it('continues token-budget-stopped runs with an increased budget', async () => {
    getRunDetail.mockResolvedValueOnce({
      error: 'token_budget_exceeded: used 1200 tokens exceeds budget 1000',
      id: 'run_1',
      iterationCount: 2,
      mode: 'react',
      planSteps: [],
      status: 'token_budget_exceeded',
      toolCallCount: 1,
      toolRuns: []
    });
    continueRunWithBudget.mockResolvedValueOnce({
      id: 'run_1',
      iterationCount: 3,
      mode: 'react',
      planSteps: [],
      status: 'completed',
      toolCallCount: 1,
      toolRuns: []
    });

    renderDirectPage();

    expect(await screen.findByText('Status: token_budget_exceeded')).toBeInTheDocument();
    expect(screen.getByLabelText('Agent run execution controls')).toHaveTextContent('Stop reason token_budget_exceeded: used 1200 tokens exceeds budget 1000');
    fireEvent.change(screen.getByLabelText('Increased token budget'), { target: { value: '2500' } });
    fireEvent.click(screen.getByRole('button', { name: 'Continue with budget' }));

    await waitFor(() => expect(continueRunWithBudget).toHaveBeenCalledWith('run_1', 2500));
    expect(await screen.findByText('Status: completed')).toBeInTheDocument();
    expect(screen.getByLabelText('Agent run execution controls')).toHaveTextContent('Iterations 3');
    expect(screen.queryByRole('button', { name: 'Continue with budget' })).not.toBeInTheDocument();
  });

  it('renders plan steps from navigation state and refreshes them after approve and execute actions', async () => {
    renderPage([
      {
        approvalStatus: 'pending',
        id: 'step_1',
        index: 1,
        runId: 'run_1',
        status: 'pending',
        title: 'Inspect workspace'
      },
      {
        approvalStatus: 'approved',
        id: 'step_2',
        index: 2,
        runId: 'run_1',
        status: 'approved',
        title: 'Patch web page'
      }
    ]);

    expect(screen.getByRole('heading', { name: 'Agent Plan Steps' })).toBeInTheDocument();
    expect(screen.getByText('Run run_1')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Inspect workspace' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Patch web page' })).toBeInTheDocument();

    approvePlanStep.mockResolvedValueOnce(runDetail([
      {
        approvalStatus: 'approved',
        id: 'step_1',
        index: 1,
        runId: 'run_1',
        status: 'approved',
        title: 'Inspect workspace'
      },
      {
        approvalStatus: 'approved',
        id: 'step_2',
        index: 2,
        runId: 'run_1',
        status: 'approved',
        title: 'Patch web page'
      }
    ], {
      status: 'pending_approval',
      toolCallCount: 1,
      toolRuns: [
        {
          approvalStatus: 'pending',
          id: 'tool_run_1',
          riskLevel: 'dangerous',
          runId: 'run_1',
          status: 'pending_approval',
          toolName: 'write_file',
          toolType: 'builtin'
        }
      ]
    }));

    fireEvent.click(screen.getByRole('button', { name: 'Approve Inspect workspace' }));

    await waitFor(() => expect(approvePlanStep).toHaveBeenCalledWith('run_1', 'step_1'));
    await waitFor(() => {
      expect(within(screen.getByLabelText('Plan step Inspect workspace')).getAllByText('approved').length).toBeGreaterThan(0);
    });
    expect(screen.getByText('Status: pending_approval')).toBeInTheDocument();
    expect(screen.getByLabelText('Agent run execution controls')).toHaveTextContent('Tool calls 1');
    expect(screen.getByLabelText('Tool run write_file')).toHaveTextContent('dangerous');
    expect(getRunDetail).toHaveBeenCalledTimes(1);

    executePlanStep.mockResolvedValueOnce(runDetail([
      {
        approvalStatus: 'approved',
        id: 'step_1',
        index: 1,
        resultContent: 'Workspace inspected.',
        runId: 'run_1',
        status: 'completed',
        title: 'Inspect workspace'
      },
      {
        approvalStatus: 'approved',
        id: 'step_2',
        index: 2,
        runId: 'run_1',
        status: 'approved',
        title: 'Patch web page'
      }
    ], {
      error: 'model_stop',
      iterationCount: 3,
      status: 'completed',
      toolCallCount: 1,
      toolRuns: []
    }));

    const firstStep = screen.getByLabelText('Plan step Inspect workspace');
    fireEvent.click(within(firstStep).getByRole('button', { name: 'Execute Inspect workspace' }));

    await waitFor(() => expect(executePlanStep).toHaveBeenCalledWith('run_1', 'step_1'));
    expect(await screen.findByText('Workspace inspected.')).toBeInTheDocument();
    expect(screen.getByText('Status: completed')).toBeInTheDocument();
    expect(screen.getByLabelText('Agent run execution controls')).toHaveTextContent('Iterations 3');
    expect(screen.getByLabelText('Agent run execution controls')).toHaveTextContent('Stop reason model_stop');
    expect(screen.queryByLabelText('Tool run write_file')).not.toBeInTheDocument();
    expect(getRunDetail).toHaveBeenCalledTimes(1);
  });

  it('surfaces operation failures without dropping current plan steps', async () => {
    approvePlanStep.mockRejectedValueOnce(new Error('Approval failed'));

    renderPage([
      {
        id: 'step_1',
        index: 1,
        runId: 'run_1',
        status: 'pending',
        title: 'Inspect workspace'
      }
    ]);

    fireEvent.click(screen.getByRole('button', { name: 'Approve Inspect workspace' }));

    expect(await screen.findByText('Approval failed')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Inspect workspace' })).toBeInTheDocument();
  });

  it('skips pending, approved, or failed plan steps from the planning page', async () => {
    renderPage([
      {
        approvalStatus: 'pending',
        id: 'step_1',
        index: 1,
        runId: 'run_1',
        status: 'pending',
        title: 'Optional discovery'
      },
      {
        approvalStatus: 'not_required',
        id: 'step_2',
        index: 2,
        runId: 'run_1',
        status: 'approved',
        title: 'Approved verification'
      },
      {
        approvalStatus: 'not_required',
        id: 'step_3',
        index: 3,
        runId: 'run_1',
        status: 'failed',
        title: 'Failed verification'
      },
      {
        approvalStatus: 'not_required',
        id: 'step_4',
        index: 4,
        runId: 'run_1',
        status: 'completed',
        title: 'Completed implementation'
      }
    ]);
    skipPlanStep.mockResolvedValueOnce(runDetail([
      {
        approvalStatus: 'pending',
        id: 'step_1',
        index: 1,
        runId: 'run_1',
        status: 'skipped',
        title: 'Optional discovery'
      },
      {
        approvalStatus: 'not_required',
        id: 'step_2',
        index: 2,
        runId: 'run_1',
        status: 'approved',
        title: 'Approved verification'
      },
      {
        approvalStatus: 'not_required',
        id: 'step_3',
        index: 3,
        runId: 'run_1',
        status: 'failed',
        title: 'Failed verification'
      },
      {
        approvalStatus: 'not_required',
        id: 'step_4',
        index: 4,
        runId: 'run_1',
        status: 'completed',
        title: 'Completed implementation'
      }
    ]));

    expect(screen.getByRole('button', { name: 'Skip Approved verification' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Skip Failed verification' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Skip Completed implementation' })).toBeDisabled();

    fireEvent.click(screen.getByRole('button', { name: 'Skip Optional discovery' }));

    await waitFor(() => expect(skipPlanStep).toHaveBeenCalledWith('run_1', 'step_1'));
    await waitFor(() => {
      expect(within(screen.getByLabelText('Plan step Optional discovery')).getAllByText('skipped').length).toBeGreaterThan(0);
    });
  });

  it('disables execution until prior plan steps are completed or skipped', async () => {
    const planSteps = [
      {
        approvalStatus: 'not_required',
        id: 'step_1',
        index: 1,
        runId: 'run_1',
        status: 'pending',
        title: 'Gather requirements'
      },
      {
        approvalStatus: 'approved',
        id: 'step_2',
        index: 2,
        runId: 'run_1',
        status: 'approved',
        title: 'Implement patch'
      },
      {
        approvalStatus: 'not_required',
        id: 'step_3',
        index: 3,
        runId: 'run_1',
        status: 'pending',
        title: 'Run checks'
      }
    ];
    getRunDetail.mockResolvedValueOnce(runDetail(planSteps));

    renderPage(planSteps);

    expect(await screen.findByText('Status: planning')).toBeInTheDocument();

    expect(screen.getByRole('button', { name: 'Execute Gather requirements' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Execute Implement patch' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Execute Run checks' })).toBeDisabled();
  });

  it('retries failed plan steps from the planning page', async () => {
    renderPage([
      {
        approvalStatus: 'approved',
        error: 'old failure',
        id: 'step_1',
        index: 1,
        runId: 'run_1',
        status: 'failed',
        title: 'Verify patch'
      },
      {
        approvalStatus: 'not_required',
        id: 'step_2',
        index: 2,
        runId: 'run_1',
        status: 'completed',
        title: 'Completed implementation'
      }
    ]);
    retryPlanStep.mockResolvedValueOnce(runDetail([
      {
        approvalStatus: 'approved',
        id: 'step_1',
        index: 1,
        resultContent: 'retry passed',
        runId: 'run_1',
        status: 'completed',
        title: 'Verify patch'
      },
      {
        approvalStatus: 'not_required',
        id: 'step_2',
        index: 2,
        runId: 'run_1',
        status: 'completed',
        title: 'Completed implementation'
      }
    ], { status: 'running' }));

    expect(screen.getByRole('button', { name: 'Retry Verify patch' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Retry Completed implementation' })).toBeDisabled();

    fireEvent.click(screen.getByRole('button', { name: 'Retry Verify patch' }));

    await waitFor(() => expect(retryPlanStep).toHaveBeenCalledWith('run_1', 'step_1'));
    expect(await screen.findByText('retry passed')).toBeInTheDocument();
    await waitFor(() => {
      expect(within(screen.getByLabelText('Plan step Verify patch')).getAllByText('completed').length).toBeGreaterThan(0);
    });
  });

  it('disables failed-step retry until prior plan steps are completed or skipped', async () => {
    const planSteps = [
      {
        approvalStatus: 'not_required',
        id: 'step_1',
        index: 1,
        runId: 'run_1',
        status: 'skipped',
        title: 'Optional discovery'
      },
      {
        approvalStatus: 'not_required',
        id: 'step_2',
        index: 2,
        runId: 'run_1',
        status: 'pending',
        title: 'Still pending'
      },
      {
        approvalStatus: 'approved',
        error: 'old failure',
        id: 'step_3',
        index: 3,
        runId: 'run_1',
        status: 'failed',
        title: 'Retry target'
      }
    ];
    getRunDetail.mockResolvedValueOnce(runDetail(planSteps));

    renderPage(planSteps);

    expect(await screen.findByText('Status: planning')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Retry Retry target' })).toBeDisabled();
  });

  it('allows failed-step retry when prior plan steps are completed or skipped', async () => {
    const planSteps = [
      {
        approvalStatus: 'not_required',
        id: 'step_1',
        index: 1,
        runId: 'run_1',
        status: 'skipped',
        title: 'Optional discovery'
      },
      {
        approvalStatus: 'not_required',
        id: 'step_2',
        index: 2,
        runId: 'run_1',
        status: 'completed',
        title: 'Completed prerequisite'
      },
      {
        approvalStatus: 'approved',
        error: 'old failure',
        id: 'step_3',
        index: 3,
        runId: 'run_1',
        status: 'failed',
        title: 'Retry target'
      }
    ];
    getRunDetail.mockResolvedValueOnce(runDetail(planSteps));

    renderPage(planSteps);

    expect(await screen.findByText('Status: planning')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Retry Retry target' })).toBeEnabled();
  });

  it('edits a pending or approved plan step and refreshes the draft', async () => {
    renderPage([
      {
        approvalStatus: 'approved',
        id: 'step_1',
        index: 1,
        input: { path: 'old.go' },
        runId: 'run_1',
        status: 'approved',
        title: 'Patch original file',
        toolName: 'write_file'
      }
    ]);
    updatePlanStep.mockResolvedValueOnce(runDetail([
      {
        approvalStatus: 'pending',
        id: 'step_1',
        index: 1,
        input: { path: 'new.go' },
        runId: 'run_1',
        status: 'pending',
        title: 'Read safer file',
        toolName: 'read_file'
      }
    ]));

    fireEvent.click(screen.getByRole('button', { name: 'Edit Patch original file' }));
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Read safer file' } });
    fireEvent.change(screen.getByLabelText('Tool'), { target: { value: 'read_file' } });
    fireEvent.change(screen.getByLabelText('Input'), { target: { value: '{\"path\":\"new.go\"}' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save Patch original file' }));

    await waitFor(() => expect(updatePlanStep).toHaveBeenCalledWith('run_1', 'step_1', {
      input: { path: 'new.go' },
      title: 'Read safer file',
      toolName: 'read_file'
    }));
    expect(await screen.findByRole('heading', { name: 'Read safer file' })).toBeInTheDocument();
    expect(screen.getByLabelText('Plan step Read safer file')).toHaveTextContent('pending');
    expect(screen.getByLabelText('Plan step Read safer file')).toHaveTextContent('Approval: pending');
    expect(screen.getByLabelText('Plan step Read safer file')).toHaveTextContent('"path": "new.go"');
  });

  it('moves pending plan steps while keeping completed steps fixed', async () => {
    renderPage([
      {
        approvalStatus: 'not_required',
        id: 'step_1',
        index: 1,
        resultContent: 'Requirements gathered.',
        runId: 'run_1',
        status: 'completed',
        title: 'Gather requirements'
      },
      {
        approvalStatus: 'approved',
        id: 'step_2',
        index: 2,
        runId: 'run_1',
        status: 'approved',
        title: 'Draft patch'
      },
      {
        approvalStatus: 'pending',
        id: 'step_3',
        index: 3,
        runId: 'run_1',
        status: 'pending',
        title: 'Verify patch'
      }
    ]);
    movePlanStep.mockResolvedValueOnce(runDetail([
      {
        approvalStatus: 'pending',
        id: 'step_1',
        index: 1,
        resultContent: 'Requirements gathered.',
        runId: 'run_1',
        status: 'completed',
        title: 'Gather requirements'
      },
      {
        approvalStatus: 'pending',
        id: 'step_3',
        index: 2,
        runId: 'run_1',
        status: 'pending',
        title: 'Verify patch'
      },
      {
        approvalStatus: 'pending',
        id: 'step_2',
        index: 3,
        runId: 'run_1',
        status: 'pending',
        title: 'Draft patch'
      }
    ]));

    expect(screen.getByRole('button', { name: 'Move up Draft patch' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Move down Draft patch' })).toBeEnabled();

    fireEvent.click(screen.getByRole('button', { name: 'Move up Verify patch' }));

    await waitFor(() => expect(movePlanStep).toHaveBeenCalledWith('run_1', 'step_3', 'up'));
    const planArticles = await screen.findAllByLabelText(/^Plan step /);
    expect(planArticles.map((article) => within(article).getByRole('heading').textContent)).toEqual([
      'Gather requirements',
      'Verify patch',
      'Draft patch'
    ]);
    expect(screen.getByLabelText('Plan step Draft patch')).toHaveTextContent('Approval: pending');
  });

  it('adds and deletes draft plan steps from the planning page', async () => {
    renderPage([
      {
        approvalStatus: 'pending',
        id: 'step_1',
        index: 1,
        runId: 'run_1',
        status: 'pending',
        title: 'Draft patch'
      },
      {
        approvalStatus: 'approved',
        id: 'step_2',
        index: 2,
        runId: 'run_1',
        status: 'approved',
        title: 'Verify patch'
      }
    ]);
    createPlanStep.mockResolvedValueOnce(runDetail([
      {
        approvalStatus: 'pending',
        id: 'step_1',
        index: 1,
        runId: 'run_1',
        status: 'pending',
        title: 'Draft patch'
      },
      {
        approvalStatus: 'pending',
        id: 'step_new',
        index: 2,
        input: { command: 'go test ./internal/agent' },
        runId: 'run_1',
        status: 'pending',
        title: 'Run checks',
        toolName: 'execute_code'
      },
      {
        approvalStatus: 'pending',
        id: 'step_2',
        index: 3,
        runId: 'run_1',
        status: 'pending',
        title: 'Verify patch'
      }
    ]));
    deletePlanStep.mockResolvedValueOnce(runDetail([
      {
        approvalStatus: 'pending',
        id: 'step_1',
        index: 1,
        runId: 'run_1',
        status: 'pending',
        title: 'Draft patch'
      },
      {
        approvalStatus: 'pending',
        id: 'step_2',
        index: 2,
        runId: 'run_1',
        status: 'pending',
        title: 'Verify patch'
      }
    ]));

    fireEvent.click(screen.getByRole('button', { name: 'Insert after Draft patch' }));
    fireEvent.change(screen.getByLabelText('New step title'), { target: { value: 'Run checks' } });
    fireEvent.change(screen.getByLabelText('New step tool'), { target: { value: 'execute_code' } });
    fireEvent.change(screen.getByLabelText('New step input'), { target: { value: '{\"command\":\"go test ./internal/agent\"}' } });
    fireEvent.click(screen.getByRole('button', { name: 'Add plan step' }));

    await waitFor(() => expect(createPlanStep).toHaveBeenCalledWith('run_1', {
      afterPlanStepId: 'step_1',
      input: { command: 'go test ./internal/agent' },
      title: 'Run checks',
      toolName: 'execute_code'
    }));
    expect(await screen.findByRole('heading', { name: 'Run checks' })).toBeInTheDocument();
    expect(screen.getByLabelText('Plan step Verify patch')).toHaveTextContent('Approval: pending');

    fireEvent.click(screen.getByRole('button', { name: 'Delete Run checks' }));

    await waitFor(() => expect(deletePlanStep).toHaveBeenCalledWith('run_1', 'step_new'));
    await waitFor(() => expect(screen.queryByRole('heading', { name: 'Run checks' })).not.toBeInTheDocument());
    expect(screen.getByLabelText('Plan step Verify patch')).toHaveTextContent('Step 2');
  });

  it('opens the add-plan-step form without an anchor', async () => {
    renderPage([]);
    createPlanStep.mockResolvedValueOnce(runDetail([
      {
        approvalStatus: 'pending',
        id: 'step_new',
        index: 1,
        runId: 'run_1',
        status: 'pending',
        title: 'Draft first step'
      }
    ]));

    fireEvent.click(screen.getByRole('button', { name: 'Add plan step' }));
    fireEvent.change(screen.getByLabelText('New step title'), { target: { value: 'Draft first step' } });
    fireEvent.click(screen.getByRole('button', { name: 'Add plan step' }));

    await waitFor(() => expect(createPlanStep).toHaveBeenCalledWith('run_1', {
      input: {},
      title: 'Draft first step',
      toolName: ''
    }));
    expect(await screen.findByRole('heading', { name: 'Draft first step' })).toBeInTheDocument();
  });

  it('rejects invalid JSON when editing a plan step input', async () => {
    renderPage([
      {
        id: 'step_1',
        index: 1,
        runId: 'run_1',
        status: 'pending',
        title: 'Inspect workspace'
      }
    ]);

    fireEvent.click(screen.getByRole('button', { name: 'Edit Inspect workspace' }));
    fireEvent.change(screen.getByLabelText('Input'), { target: { value: '{not-json}' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save Inspect workspace' }));

    expect(await screen.findByText('Plan step input must be valid JSON.')).toBeInTheDocument();
    expect(updatePlanStep).not.toHaveBeenCalled();
  });

  it('renders tool approval queue and refreshes tool runs after approve, reject, and retry actions', async () => {
    getRunDetail.mockResolvedValue({
      id: 'run_1',
      planSteps: [],
      status: 'requires_tool_approval',
      toolRuns: [
        {
          approvalStatus: 'pending',
          arguments: { path: 'src/server/main.go' },
          id: 'tool_run_1',
          riskLevel: 'dangerous',
          runId: 'run_1',
          status: 'pending_approval',
          toolName: 'write_file',
          toolType: 'builtin'
        },
        {
          approvalStatus: 'not_required',
          error: 'search endpoint timed out',
          id: 'tool_run_2',
          riskLevel: 'medium',
          runId: 'run_1',
          status: 'failed',
          toolName: 'web_search',
          toolType: 'mcp'
        },
        {
          approvalStatus: 'pending',
          arguments: { command: 'rm -rf /tmp/build' },
          id: 'tool_run_3',
          riskLevel: 'dangerous',
          runId: 'run_1',
          status: 'pending_approval',
          toolName: 'delete_file',
          toolType: 'builtin'
        }
      ]
    });
    approveToolRun.mockResolvedValueOnce({
      id: 'run_1',
      iterationCount: 3,
      mode: 'planning',
      planSteps: [
        {
          approvalStatus: 'not_required',
          id: 'step_done',
          index: 1,
          resultContent: 'Tool approval resumed the run.',
          runId: 'run_1',
          status: 'completed',
          title: 'Resume after tool approval'
        }
      ],
      status: 'completed',
      toolCallCount: 3,
      toolRuns: [
        {
          approvalStatus: 'approved',
          id: 'tool_run_1',
          riskLevel: 'dangerous',
          runId: 'run_1',
          status: 'completed',
          toolName: 'write_file',
          toolType: 'builtin'
        },
        {
          approvalStatus: 'not_required',
          error: 'search endpoint timed out',
          id: 'tool_run_2',
          riskLevel: 'medium',
          runId: 'run_1',
          status: 'failed',
          toolName: 'web_search',
          toolType: 'mcp'
        },
        {
          approvalStatus: 'pending',
          id: 'tool_run_3',
          riskLevel: 'dangerous',
          runId: 'run_1',
          status: 'pending_approval',
          toolName: 'delete_file',
          toolType: 'builtin'
        }
      ]
    });
    rejectToolRun.mockResolvedValueOnce({
      error: 'Rejected by operator',
      id: 'run_1',
      iterationCount: 4,
      mode: 'planning',
      planSteps: [
        {
          approvalStatus: 'not_required',
          id: 'step_done',
          index: 1,
          resultContent: 'Tool approval resumed the run.',
          runId: 'run_1',
          status: 'completed',
          title: 'Resume after tool approval'
        }
      ],
      status: 'failed',
      toolCallCount: 3,
      toolRuns: [
        {
          approvalStatus: 'approved',
          id: 'tool_run_1',
          riskLevel: 'dangerous',
          runId: 'run_1',
          status: 'completed',
          toolName: 'write_file',
          toolType: 'builtin'
        },
        {
          approvalStatus: 'not_required',
          error: 'search endpoint timed out',
          id: 'tool_run_2',
          riskLevel: 'medium',
          runId: 'run_1',
          status: 'failed',
          toolName: 'web_search',
          toolType: 'mcp'
        },
        {
          approvalStatus: 'rejected',
          id: 'tool_run_3',
          riskLevel: 'dangerous',
          runId: 'run_1',
          status: 'rejected',
          toolName: 'delete_file',
          toolType: 'builtin'
        }
      ]
    });
    retryToolRun.mockResolvedValueOnce({
      id: 'run_1',
      iterationCount: 5,
      mode: 'planning',
      planSteps: [
        {
          approvalStatus: 'not_required',
          id: 'step_verify',
          index: 1,
          resultContent: 'Retry resumed search and verified the result.',
          runId: 'run_1',
          status: 'completed',
          title: 'Verify retried search'
        }
      ],
      status: 'running',
      toolCallCount: 4,
      toolRuns: [
        {
          approvalStatus: 'approved',
          id: 'tool_run_1',
          riskLevel: 'dangerous',
          runId: 'run_1',
          status: 'completed',
          toolName: 'write_file',
          toolType: 'builtin'
        },
        {
          approvalStatus: 'not_required',
          id: 'tool_run_2',
          riskLevel: 'medium',
          runId: 'run_1',
          status: 'running',
          toolName: 'web_search',
          toolType: 'mcp'
        }
      ]
    });

    renderDirectPage();

    expect(await screen.findByRole('heading', { name: 'Tool Approval Queue' })).toBeInTheDocument();
    expect(screen.getByLabelText('Tool run write_file')).toHaveTextContent('dangerous');
    expect(screen.getByLabelText('Tool run write_file')).toHaveTextContent('"path": "src/server/main.go"');
    expect(screen.getByLabelText('Tool run web_search')).toHaveTextContent('search endpoint timed out');
    expect(screen.getByLabelText('Tool run delete_file')).toHaveTextContent('"command": "rm -rf /tmp/build"');

    fireEvent.change(screen.getByLabelText('Operator decision reason for write_file'), {
      target: { value: 'Reviewed scoped file write and path is inside the repository.' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Approve tool write_file' }));

    await waitFor(() =>
      expect(approveToolRun).toHaveBeenCalledWith(
        'run_1',
        'tool_run_1',
        'Reviewed scoped file write and path is inside the repository.'
      )
    );
    await waitFor(() => {
      expect(screen.getByText('Status: completed')).toBeInTheDocument();
      expect(screen.getByLabelText('Agent run execution controls')).toHaveTextContent('Iterations 3');
      expect(screen.getByRole('heading', { name: 'Resume after tool approval' })).toBeInTheDocument();
      expect(within(screen.getByLabelText('Tool run write_file')).getByText('completed')).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText('Operator decision reason for delete_file'), {
      target: { value: 'Rejecting destructive command until a safer cleanup plan is provided.' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Reject tool delete_file' }));

    await waitFor(() =>
      expect(rejectToolRun).toHaveBeenCalledWith(
        'run_1',
        'tool_run_3',
        'Rejecting destructive command until a safer cleanup plan is provided.'
      )
    );
    await waitFor(() => {
      expect(screen.getByText('Status: failed')).toBeInTheDocument();
      expect(screen.getByLabelText('Agent run execution controls')).toHaveTextContent('Stop reason Rejected by operator');
      expect(within(screen.getByLabelText('Tool run delete_file')).getByText('rejected')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Retry tool web_search' }));

    await waitFor(() => expect(retryToolRun).toHaveBeenCalledWith('run_1', 'tool_run_2'));
    await waitFor(() => {
      expect(screen.getByText('Status: running')).toBeInTheDocument();
      expect(screen.getByRole('heading', { name: 'Verify retried search' })).toBeInTheDocument();
      expect(within(screen.getByLabelText('Tool run web_search')).getByText('running')).toBeInTheDocument();
    });
  });
});
