import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const approvePlanStep = vi.fn();
const approveToolRun = vi.fn();
const executePlanStep = vi.fn();
const getRunDetail = vi.fn();
const movePlanStep = vi.fn();
const rejectToolRun = vi.fn();
const retryToolRun = vi.fn();
const updatePlanStep = vi.fn();

vi.mock('../../features/agents/planStepsApi', () => ({
  createAgentPlanStepsApi: () => ({
    approvePlanStep,
    approveToolRun,
    executePlanStep,
    getRunDetail,
    movePlanStep,
    rejectToolRun,
    retryToolRun,
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

describe('AgentPlanStepsPage', () => {
  beforeEach(() => {
    approvePlanStep.mockReset();
    approveToolRun.mockReset();
    executePlanStep.mockReset();
    getRunDetail.mockReset();
    movePlanStep.mockReset();
    rejectToolRun.mockReset();
    retryToolRun.mockReset();
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

    approvePlanStep.mockResolvedValueOnce([
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
    ]);

    fireEvent.click(screen.getByRole('button', { name: 'Approve Inspect workspace' }));

    await waitFor(() => expect(approvePlanStep).toHaveBeenCalledWith('run_1', 'step_1'));
    await waitFor(() => {
      expect(within(screen.getByLabelText('Plan step Inspect workspace')).getAllByText('approved').length).toBeGreaterThan(0);
    });

    executePlanStep.mockResolvedValueOnce([
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
    ]);

    const firstStep = screen.getByLabelText('Plan step Inspect workspace');
    fireEvent.click(within(firstStep).getByRole('button', { name: 'Execute Inspect workspace' }));

    await waitFor(() => expect(executePlanStep).toHaveBeenCalledWith('run_1', 'step_1'));
    expect(await screen.findByText('Workspace inspected.')).toBeInTheDocument();
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
    updatePlanStep.mockResolvedValueOnce([
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
    ]);

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
    movePlanStep.mockResolvedValueOnce([
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
    ]);

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
    approveToolRun.mockResolvedValueOnce([
      {
        approvalStatus: 'approved',
        id: 'tool_run_1',
        riskLevel: 'dangerous',
        runId: 'run_1',
        status: 'running',
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
    ]);
    rejectToolRun.mockResolvedValueOnce([
      {
        approvalStatus: 'approved',
        id: 'tool_run_1',
        riskLevel: 'dangerous',
        runId: 'run_1',
        status: 'running',
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
    ]);
    retryToolRun.mockResolvedValueOnce([
      {
        approvalStatus: 'approved',
        id: 'tool_run_1',
        riskLevel: 'dangerous',
        runId: 'run_1',
        status: 'running',
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
    ]);

    renderDirectPage();

    expect(await screen.findByRole('heading', { name: 'Tool Approval Queue' })).toBeInTheDocument();
    expect(screen.getByLabelText('Tool run write_file')).toHaveTextContent('dangerous');
    expect(screen.getByLabelText('Tool run write_file')).toHaveTextContent('"path": "src/server/main.go"');
    expect(screen.getByLabelText('Tool run web_search')).toHaveTextContent('search endpoint timed out');
    expect(screen.getByLabelText('Tool run delete_file')).toHaveTextContent('"command": "rm -rf /tmp/build"');

    fireEvent.click(screen.getByRole('button', { name: 'Approve tool write_file' }));

    await waitFor(() => expect(approveToolRun).toHaveBeenCalledWith('run_1', 'tool_run_1', 'Approved from Agent Plan Steps'));
    await waitFor(() => {
      expect(within(screen.getByLabelText('Tool run write_file')).getByText('running')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Reject tool delete_file' }));

    await waitFor(() => expect(rejectToolRun).toHaveBeenCalledWith('run_1', 'tool_run_3', 'Rejected from Agent Plan Steps'));
    await waitFor(() => {
      expect(within(screen.getByLabelText('Tool run delete_file')).getByText('rejected')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Retry tool web_search' }));

    await waitFor(() => expect(retryToolRun).toHaveBeenCalledWith('run_1', 'tool_run_2'));
    await waitFor(() => {
      expect(within(screen.getByLabelText('Tool run web_search')).getByText('running')).toBeInTheDocument();
    });
  });
});
