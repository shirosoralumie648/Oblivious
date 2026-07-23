import { render, screen, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const cancelExecution = vi.fn();
const checkWorkflowResourceLimits = vi.fn();
const createWorkflowBranch = vi.fn();
const createWorkflow = vi.fn();
const deleteWorkflow = vi.fn();
const executeWorkflow = vi.fn();
const getExecution = vi.fn();
const getExecutionDebugSnapshot = vi.fn();
const listExecutions = vi.fn();
const listWorkflowVersions = vi.fn();
const listWorkflows = vi.fn();
const matchConversationTriggers = vi.fn();
const matchSemanticTriggers = vi.fn();
const pauseExecution = vi.fn();
const publishWorkflowBranch = vi.fn();
const rollbackWorkflow = vi.fn();
const resumeExecution = vi.fn();
const resolvePausedFailure = vi.fn();
const mergeWorkflowBranch = vi.fn();
const listScheduledTasks = vi.fn();
const runScheduledTaskNow = vi.fn();
const testNode = vi.fn();
const triggerWorkflowWebhook = vi.fn();
const updateWorkflow = vi.fn();

vi.mock('../../features/workflows/workflowsApi', () => ({
  createWorkflowsApi: () => ({
    cancelExecution,
    checkWorkflowResourceLimits,
    createWorkflowBranch,
    createWorkflow,
    deleteWorkflow,
    executeWorkflow,
    getExecution,
    getExecutionDebugSnapshot,
    listExecutions,
    listWorkflowVersions,
    listWorkflows,
    matchConversationTriggers,
    matchSemanticTriggers,
    mergeWorkflowBranch,
    pauseExecution,
    publishWorkflowBranch,
    rollbackWorkflow,
    resumeExecution,
    resolvePausedFailure,
    testNode,
    triggerWorkflowWebhook,
    updateWorkflow,
  }),
}));

vi.mock('../../features/scheduledTasks/scheduledTasksApi', () => ({
  createScheduledTasksApi: () => ({
    listScheduledTasks,
    runScheduledTaskNow,
  }),
}));

import { WorkflowsPage } from './WorkflowsPage';

// This suite intentionally does NOT mock `@xyflow/react`: it verifies that the
// workflow editor mounts the real React Flow canvas. Deterministic interaction
// tests that rely on the mocked canvas live in WorkflowsPage.test.tsx.
describe('WorkflowsPage real React Flow canvas', () => {
  beforeEach(() => {
    listWorkflows.mockReset();
    listScheduledTasks.mockReset();
    listScheduledTasks.mockResolvedValue([]);
  });

  it('renders the workflow editor on a real React Flow canvas', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          edges: [{ source: 'manual-start', target: 'classify-ticket' }],
          nodes: [
            { id: 'manual-start', position: { x: 80, y: 80 }, type: 'manual' },
            { id: 'classify-ticket', position: { x: 320, y: 80 }, type: 'llm' },
          ],
        },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        version: 1,
      },
    ]);

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    const visualEditor = screen.getByLabelText('Visual editor for Incident triage');
    const canvas = within(visualEditor).getByLabelText('React Flow canvas for Incident triage');

    expect(canvas.querySelector('.react-flow')).toBeInTheDocument();
    expect(within(canvas).getAllByText('manual-start')[0]).toBeInTheDocument();
    expect(within(canvas).getAllByText('classify-ticket')[0]).toBeInTheDocument();
    expect(within(visualEditor).getByLabelText('Node palette for Incident triage')).toBeInTheDocument();
    expect(within(visualEditor).getAllByRole('button', { name: /Add .* node template to Incident triage/ })).toHaveLength(
      22
    );
  });
});
