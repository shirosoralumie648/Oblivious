import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
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
const rollbackWorkflow = vi.fn();
const resumeExecution = vi.fn();
const resolvePausedFailure = vi.fn();
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
    pauseExecution,
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

describe('WorkflowsPage', () => {
  beforeEach(() => {
    cancelExecution.mockReset();
    checkWorkflowResourceLimits.mockReset();
    createWorkflowBranch.mockReset();
    createWorkflow.mockReset();
    deleteWorkflow.mockReset();
    executeWorkflow.mockReset();
    getExecution.mockReset();
    getExecutionDebugSnapshot.mockReset();
    listExecutions.mockReset();
    listWorkflowVersions.mockReset();
    listWorkflows.mockReset();
    matchConversationTriggers.mockReset();
    matchSemanticTriggers.mockReset();
    pauseExecution.mockReset();
    rollbackWorkflow.mockReset();
    resumeExecution.mockReset();
    resolvePausedFailure.mockReset();
    listScheduledTasks.mockReset();
    listScheduledTasks.mockResolvedValue([]);
    runScheduledTaskNow.mockReset();
    testNode.mockReset();
    triggerWorkflowWebhook.mockReset();
    updateWorkflow.mockReset();
  });

  it('loads and renders workflows', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        version: 1,
      },
    ]);

    render(<WorkflowsPage />);

    expect(screen.getByText('Loading workflows...')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Workflows' })).toBeInTheDocument();
    expect(screen.getByText('Incident triage')).toBeInTheDocument();
    expect(screen.getByText('Status: draft')).toBeInTheDocument();
  });

  it('creates a simple draft workflow with editable variables', async () => {
    listWorkflows.mockResolvedValue([]);
    createWorkflow.mockResolvedValue({
      definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
      id: 'workflow_2',
      name: 'Manual review',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 1,
    });

    render(<WorkflowsPage />);

    await screen.findByText('No workflows yet. Create a draft workflow to start.');
    fireEvent.change(screen.getByLabelText('Workflow name'), { target: { value: 'Manual review' } });
    fireEvent.change(screen.getByLabelText('Workflow variables JSON'), { target: { value: '{ "owner": "ops" }' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create draft workflow' }));

    await waitFor(() => {
      expect(createWorkflow).toHaveBeenCalledWith({
        definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
        description: 'Draft workflow created from the workspace.',
        name: 'Manual review',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
    expect(screen.getByText('Manual review')).toBeInTheDocument();
    expect(screen.getByLabelText('Variables JSON for Manual review')).toHaveValue('{\n  "owner": "ops"\n}');
  });

  it('loads workflow variables and saves edited variables through the update payload', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { assignee: 'oncall', priority: 'high' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { assignee: 'ops', priority: 'critical' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    expect(screen.getByLabelText('Variables JSON for Incident triage')).toHaveValue(
      '{\n  "assignee": "oncall",\n  "priority": "high"\n}'
    );

    fireEvent.change(screen.getByLabelText('Variables JSON for Incident triage'), {
      target: { value: '{ "assignee": "ops", "priority": "critical" }' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save variables for Incident triage' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { assignee: 'ops', priority: 'critical' },
      });
    });
    expect(screen.getByText('Version: 2')).toBeInTheDocument();
    expect(screen.getByLabelText('Variables JSON for Incident triage')).toHaveValue(
      '{\n  "assignee": "ops",\n  "priority": "critical"\n}'
    );
  });

  it('renders and saves workflow trigger configuration through the workflow definition', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            conversation: { conversationId: 'conversation_incident' },
            schedule: { cron: '0 * * * *', timezone: 'Asia/Shanghai' },
            semantic: [{ id: 'urgent-ticket', keywords: ['incident', 'sev1'], semanticThreshold: 0.85 }],
            webhook: { id: 'github', path: '/api/v1/workflows/webhooks/org_1/workflow_1', secret: 'top-secret' },
          },
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        nodes: [{ id: 'manual-start', type: 'manual' }],
        triggers: {
          conversation: { conversationId: 'conversation_incident' },
          schedule: [{ cronExpression: '*/15 * * * *', enabled: true }],
          semantic: [{ id: 'urgent-ticket', keywords: ['incident', 'sev1'], semanticThreshold: 0.9 }],
          webhook: { id: 'github', path: '/api/v1/workflows/webhooks/org_1/workflow_1' },
        },
      },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    const triggersPanel = screen.getByLabelText('Triggers Incident triage');
    expect(within(triggersPanel).getByText('Conversation: conversation_incident')).toBeInTheDocument();
    expect(within(triggersPanel).getByText('Schedule: 0 * * * * (Asia/Shanghai)')).toBeInTheDocument();
    expect(within(triggersPanel).getByText('Semantic: urgent-ticket incident, sev1 threshold 0.85')).toBeInTheDocument();
    expect(
      within(triggersPanel).getByText('Webhook: github /api/v1/workflows/webhooks/org_1/workflow_1 secret configured')
    ).toBeInTheDocument();
    expect(screen.getByLabelText('Triggers JSON for Incident triage')).toHaveValue(
      '{\n' +
        '  "conversation": {\n' +
        '    "conversationId": "conversation_incident"\n' +
        '  },\n' +
        '  "schedule": {\n' +
        '    "cron": "0 * * * *",\n' +
        '    "timezone": "Asia/Shanghai"\n' +
        '  },\n' +
        '  "semantic": [\n' +
        '    {\n' +
        '      "id": "urgent-ticket",\n' +
        '      "keywords": [\n' +
        '        "incident",\n' +
        '        "sev1"\n' +
        '      ],\n' +
        '      "semanticThreshold": 0.85\n' +
        '    }\n' +
        '  ],\n' +
        '  "webhook": {\n' +
        '    "id": "github",\n' +
        '    "path": "/api/v1/workflows/webhooks/org_1/workflow_1",\n' +
        '    "secret": "top-secret"\n' +
        '  }\n' +
        '}'
    );

    fireEvent.change(screen.getByLabelText('Triggers JSON for Incident triage'), {
      target: {
        value:
          '{ "conversation": { "conversationId": "conversation_incident" }, "schedule": { "cron": "*/15 * * * *", "timezone": "UTC" }, "semantic": [{ "id": "urgent-ticket", "keywords": ["incident", "sev1"], "semanticThreshold": 0.9 }], "webhook": { "id": "github", "path": "/api/v1/workflows/webhooks/org_1/workflow_1" } }',
      },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save triggers for Incident triage' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            conversation: { conversationId: 'conversation_incident' },
            schedule: [{ cronExpression: '*/15 * * * *', enabled: true }],
            semantic: [{ id: 'urgent-ticket', keywords: ['incident', 'sev1'], semanticThreshold: 0.9 }],
            webhook: { id: 'github', path: '/api/v1/workflows/webhooks/org_1/workflow_1' },
          },
        },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
    expect(screen.getByText('Version: 2')).toBeInTheDocument();
    expect(screen.getByLabelText('Triggers JSON for Incident triage')).toHaveValue(
      '{\n' +
        '  "conversation": {\n' +
        '    "conversationId": "conversation_incident"\n' +
        '  },\n' +
        '  "schedule": [\n' +
        '    {\n' +
        '      "cronExpression": "*/15 * * * *",\n' +
        '      "enabled": true\n' +
        '    }\n' +
        '  ],\n' +
        '  "semantic": [\n' +
        '    {\n' +
        '      "id": "urgent-ticket",\n' +
        '      "keywords": [\n' +
        '        "incident",\n' +
        '        "sev1"\n' +
        '      ],\n' +
        '      "semanticThreshold": 0.9\n' +
        '    }\n' +
        '  ],\n' +
        '  "webhook": {\n' +
        '    "id": "github",\n' +
        '    "path": "/api/v1/workflows/webhooks/org_1/workflow_1"\n' +
        '  }\n' +
        '}'
    );
  });

  it('edits schedule trigger fields without requiring raw trigger JSON', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            conversation: { conversationId: 'conversation_incident' },
            schedule: [{ cronExpression: '0 * * * *', id: 'hourly-triage' }],
          },
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        nodes: [{ id: 'manual-start', type: 'manual' }],
        triggers: {
          conversation: { conversationId: 'conversation_incident' },
          schedule: [{ cronExpression: '*/15 * * * *', enabled: false, id: 'quarter-hour-triage' }],
        },
      },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    expect(screen.getByLabelText('Schedule trigger ID for Incident triage')).toHaveValue('hourly-triage');
    expect(screen.getByLabelText('Schedule cron for Incident triage')).toHaveValue('0 * * * *');
    expect(screen.getByLabelText('Schedule enabled for Incident triage')).toBeChecked();

    fireEvent.change(screen.getByLabelText('Schedule trigger ID for Incident triage'), {
      target: { value: 'quarter-hour-triage' },
    });
    fireEvent.change(screen.getByLabelText('Schedule cron for Incident triage'), {
      target: { value: '*/15 * * * *' },
    });
    fireEvent.click(screen.getByLabelText('Schedule enabled for Incident triage'));
    fireEvent.click(screen.getByRole('button', { name: 'Save triggers for Incident triage' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            conversation: { conversationId: 'conversation_incident' },
            schedule: [{ cronExpression: '*/15 * * * *', enabled: false, id: 'quarter-hour-triage' }],
          },
        },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
    expect(screen.getByText('Version: 2')).toBeInTheDocument();
    expect(screen.getByLabelText('Triggers JSON for Incident triage')).toHaveValue(
      '{\n' +
        '  "conversation": {\n' +
        '    "conversationId": "conversation_incident"\n' +
        '  },\n' +
        '  "schedule": [\n' +
        '    {\n' +
        '      "cronExpression": "*/15 * * * *",\n' +
        '      "enabled": false,\n' +
        '      "id": "quarter-hour-triage"\n' +
        '    }\n' +
        '  ]\n' +
        '}'
    );
  });

  it('preserves additional schedule triggers when editing the first schedule trigger', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            schedule: [
              { cronExpression: '0 * * * *', id: 'hourly-triage' },
              { cronExpression: '0 9 * * 1', enabled: true, id: 'weekly-report' },
            ],
          },
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        nodes: [{ id: 'manual-start', type: 'manual' }],
        triggers: {
          schedule: [
            { cronExpression: '*/20 * * * *', enabled: true, id: 'twenty-minute-triage' },
            { cronExpression: '0 9 * * 1', enabled: true, id: 'weekly-report' },
          ],
        },
      },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.change(screen.getByLabelText('Schedule trigger ID for Incident triage'), {
      target: { value: 'twenty-minute-triage' },
    });
    fireEvent.change(screen.getByLabelText('Schedule cron for Incident triage'), {
      target: { value: '*/20 * * * *' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save triggers for Incident triage' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            schedule: [
              { cronExpression: '*/20 * * * *', enabled: true, id: 'twenty-minute-triage' },
              { cronExpression: '0 9 * * 1', enabled: true, id: 'weekly-report' },
            ],
          },
        },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
  });

  it('keeps structured schedule edits when raw trigger JSON is edited afterwards', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            conversation: { conversationId: 'conversation_incident' },
            schedule: [{ cronExpression: '0 * * * *', enabled: true, id: 'hourly-triage' }],
          },
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        nodes: [{ id: 'manual-start', type: 'manual' }],
        triggers: {
          conversation: { conversationId: 'conversation_escalation' },
          schedule: [{ cronExpression: '*/10 * * * *', enabled: true, id: 'ten-minute-triage' }],
        },
      },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.change(screen.getByLabelText('Schedule trigger ID for Incident triage'), {
      target: { value: 'ten-minute-triage' },
    });
    fireEvent.change(screen.getByLabelText('Schedule cron for Incident triage'), {
      target: { value: '*/10 * * * *' },
    });
    fireEvent.change(screen.getByLabelText('Triggers JSON for Incident triage'), {
      target: {
        value:
          '{ "conversation": { "conversationId": "conversation_escalation" }, "schedule": [{ "cronExpression": "0 * * * *", "enabled": true, "id": "hourly-triage" }] }',
      },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save triggers for Incident triage' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            conversation: { conversationId: 'conversation_escalation' },
            schedule: [{ cronExpression: '*/10 * * * *', enabled: true, id: 'ten-minute-triage' }],
          },
        },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
  });

  it('clears a disabled schedule trigger when id and cron are empty', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            schedule: [{ cronExpression: '0 * * * *', enabled: true, id: 'hourly-triage' }],
          },
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        nodes: [{ id: 'manual-start', type: 'manual' }],
        triggers: {},
      },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.click(screen.getByLabelText('Schedule enabled for Incident triage'));
    fireEvent.change(screen.getByLabelText('Schedule trigger ID for Incident triage'), {
      target: { value: '' },
    });
    fireEvent.change(screen.getByLabelText('Schedule cron for Incident triage'), {
      target: { value: '' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save triggers for Incident triage' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {},
        },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
  });

  it('edits conversation trigger fields without requiring raw trigger JSON', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            conversation: [{ conversationId: 'conversation_incident', id: 'conversation-main' }],
          },
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        nodes: [{ id: 'manual-start', type: 'manual' }],
        triggers: {
          conversation: [{ conversationId: 'conversation_escalation', id: 'conversation-escalation' }],
        },
      },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    expect(screen.getByLabelText('Conversation trigger ID for Incident triage')).toHaveValue('conversation-main');
    expect(screen.getByLabelText('Conversation ID for Incident triage')).toHaveValue('conversation_incident');

    fireEvent.change(screen.getByLabelText('Conversation trigger ID for Incident triage'), {
      target: { value: 'conversation-escalation' },
    });
    fireEvent.change(screen.getByLabelText('Conversation ID for Incident triage'), {
      target: { value: 'conversation_escalation' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save triggers for Incident triage' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            conversation: [{ conversationId: 'conversation_escalation', id: 'conversation-escalation' }],
          },
        },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
    expect(screen.getByText('Version: 2')).toBeInTheDocument();
    expect(screen.getByLabelText('Triggers JSON for Incident triage')).toHaveValue(
      '{\n' +
        '  "conversation": [\n' +
        '    {\n' +
        '      "conversationId": "conversation_escalation",\n' +
        '      "id": "conversation-escalation"\n' +
        '    }\n' +
        '  ]\n' +
        '}'
    );
  });

  it('edits semantic trigger fields without requiring raw trigger JSON', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            semantic: [{ id: 'urgent-ticket', keywords: ['incident', 'sev1'], semanticThreshold: 0.85 }],
          },
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        nodes: [{ id: 'manual-start', type: 'manual' }],
        triggers: {
          semantic: [{ id: 'escalation-ticket', keywords: ['outage', 'sev0'], semanticThreshold: 0.92 }],
        },
      },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    expect(screen.getByLabelText('Semantic trigger ID for Incident triage')).toHaveValue('urgent-ticket');
    expect(screen.getByLabelText('Semantic keywords for Incident triage')).toHaveValue('incident, sev1');
    expect(screen.getByLabelText('Semantic threshold for Incident triage')).toHaveValue('0.85');

    fireEvent.change(screen.getByLabelText('Semantic trigger ID for Incident triage'), {
      target: { value: 'escalation-ticket' },
    });
    fireEvent.change(screen.getByLabelText('Semantic keywords for Incident triage'), {
      target: { value: 'outage, sev0' },
    });
    fireEvent.change(screen.getByLabelText('Semantic threshold for Incident triage'), {
      target: { value: '0.92' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save triggers for Incident triage' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            semantic: [{ id: 'escalation-ticket', keywords: ['outage', 'sev0'], semanticThreshold: 0.92 }],
          },
        },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
    expect(screen.getByText('Version: 2')).toBeInTheDocument();
    expect(screen.getByLabelText('Triggers JSON for Incident triage')).toHaveValue(
      '{\n' +
        '  "semantic": [\n' +
        '    {\n' +
        '      "id": "escalation-ticket",\n' +
        '      "keywords": [\n' +
        '        "outage",\n' +
        '        "sev0"\n' +
        '      ],\n' +
        '      "semanticThreshold": 0.92\n' +
        '    }\n' +
        '  ]\n' +
        '}'
    );
  });

  it('preserves additional semantic triggers when editing the first semantic trigger', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            semantic: [
              { id: 'urgent-ticket', keywords: ['incident'], semanticThreshold: 0.85 },
              { id: 'billing-ticket', keywords: ['invoice'], semanticThreshold: 0.75 },
            ],
          },
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        nodes: [{ id: 'manual-start', type: 'manual' }],
        triggers: {
          semantic: [
            { id: 'sev-ticket', keywords: ['sev0'], semanticThreshold: 0.93 },
            { id: 'billing-ticket', keywords: ['invoice'], semanticThreshold: 0.75 },
          ],
        },
      },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.change(screen.getByLabelText('Semantic trigger ID for Incident triage'), {
      target: { value: 'sev-ticket' },
    });
    fireEvent.change(screen.getByLabelText('Semantic keywords for Incident triage'), {
      target: { value: 'sev0' },
    });
    fireEvent.change(screen.getByLabelText('Semantic threshold for Incident triage'), {
      target: { value: '0.93' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save triggers for Incident triage' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            semantic: [
              { id: 'sev-ticket', keywords: ['sev0'], semanticThreshold: 0.93 },
              { id: 'billing-ticket', keywords: ['invoice'], semanticThreshold: 0.75 },
            ],
          },
        },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
  });

  it('rejects semantic trigger thresholds that are not numbers', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            semantic: [{ id: 'urgent-ticket', keywords: ['incident'], semanticThreshold: 0.85 }],
          },
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.change(screen.getByLabelText('Semantic threshold for Incident triage'), {
      target: { value: 'not-a-number' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save triggers for Incident triage' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Semantic threshold must be a number.');
    expect(updateWorkflow).not.toHaveBeenCalled();
  });

  it('edits webhook trigger fields without requiring raw trigger JSON', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            webhook: { id: 'github', path: '/api/v1/workflows/webhooks/org_1/workflow_1', secret: 'top-secret' },
          },
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        nodes: [{ id: 'manual-start', type: 'manual' }],
        triggers: {
          webhook: [{ id: 'linear', path: '/api/v1/workflows/webhooks/org_1/workflow_1', secret: 'new-secret' }],
        },
      },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    expect(screen.getByLabelText('Webhook trigger ID for Incident triage')).toHaveValue('github');
    expect(screen.getByLabelText('Webhook path for Incident triage')).toHaveValue('/api/v1/workflows/webhooks/org_1/workflow_1');
    expect(screen.getByLabelText('Webhook secret for Incident triage')).toHaveValue('top-secret');

    fireEvent.change(screen.getByLabelText('Webhook trigger ID for Incident triage'), {
      target: { value: 'linear' },
    });
    fireEvent.change(screen.getByLabelText('Webhook secret for Incident triage'), {
      target: { value: 'new-secret' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save triggers for Incident triage' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            webhook: [{ id: 'linear', path: '/api/v1/workflows/webhooks/org_1/workflow_1', secret: 'new-secret' }],
          },
        },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
  });

  it('checks semantic trigger matches for a sample message', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            semantic: [{ id: 'urgent-alerts', keywords: ['urgent outage'], semanticThreshold: 0.85 }],
          },
        },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'published',
        version: 2,
      },
    ]);
    matchSemanticTriggers.mockResolvedValue([
      {
        keyword: 'urgent outage',
        matchMethod: 'embedding',
        score: 0.91,
        semanticThreshold: 0.85,
        triggerId: 'urgent-alerts',
        workflowId: 'workflow_1',
        workflowName: 'Incident triage',
        workflowVersion: 2,
      },
    ]);

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.change(screen.getByLabelText('Semantic match message'), {
      target: { value: 'urgent outage in production' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Check semantic matches' }));

    await waitFor(() => {
      expect(matchSemanticTriggers).toHaveBeenCalledWith({ message: 'urgent outage in production' });
    });
    expect(screen.getByLabelText('Semantic trigger match results')).toHaveTextContent('Incident triage');
    expect(screen.getByText('urgent-alerts | urgent outage | score 0.91 | threshold 0.85 | embedding')).toBeInTheDocument();
  });

  it('checks conversation trigger matches for a conversation ID', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            conversation: [{ conversationId: 'conversation_incident', id: 'conversation-main' }],
          },
        },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'published',
        version: 2,
      },
    ]);
    matchConversationTriggers.mockResolvedValue([
      {
        conversationId: 'conversation_incident',
        triggerId: 'conversation-main',
        workflowId: 'workflow_1',
        workflowName: 'Incident triage',
        workflowVersion: 2,
      },
    ]);

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.change(screen.getByLabelText('Conversation match ID'), {
      target: { value: 'conversation_incident' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Check conversation matches' }));

    await waitFor(() => {
      expect(matchConversationTriggers).toHaveBeenCalledWith({ conversationId: 'conversation_incident' });
    });
    expect(screen.getByLabelText('Conversation trigger match results')).toHaveTextContent('Incident triage');
    expect(screen.getByText('conversation-main | conversation_incident')).toBeInTheDocument();
  });

  it('preserves additional webhook triggers when editing the first webhook trigger', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            webhook: [
              { id: 'github', path: '/api/v1/workflows/webhooks/org_1/workflow_1', secret: 'top-secret' },
              { id: 'stripe', path: '/api/v1/workflows/webhooks/org_1/workflow_1', secret: 'stripe-secret' },
            ],
          },
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        nodes: [{ id: 'manual-start', type: 'manual' }],
        triggers: {
          webhook: [
            { id: 'linear', path: '/api/v1/workflows/webhooks/org_1/workflow_1', secret: 'linear-secret' },
            { id: 'stripe', path: '/api/v1/workflows/webhooks/org_1/workflow_1', secret: 'stripe-secret' },
          ],
        },
      },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.change(screen.getByLabelText('Webhook trigger ID for Incident triage'), {
      target: { value: 'linear' },
    });
    fireEvent.change(screen.getByLabelText('Webhook secret for Incident triage'), {
      target: { value: 'linear-secret' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save triggers for Incident triage' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            webhook: [
              { id: 'linear', path: '/api/v1/workflows/webhooks/org_1/workflow_1', secret: 'linear-secret' },
              { id: 'stripe', path: '/api/v1/workflows/webhooks/org_1/workflow_1', secret: 'stripe-secret' },
            ],
          },
        },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
  });

  it('executes a selected workflow and preserves the list when execution fails', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        version: 1,
      },
    ]);
    executeWorkflow.mockRejectedValue(new Error('execution failed'));

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.click(screen.getByRole('button', { name: 'Run Incident triage' }));

    await waitFor(() => {
      expect(executeWorkflow).toHaveBeenCalledWith('workflow_1', { executionMode: 'auto', input: {} });
    });
    expect(screen.getByText('Incident triage')).toBeInTheDocument();
    expect(screen.getByText('Unable to execute workflow. The workflow list was preserved.')).toBeInTheDocument();
  });

  it('publishes and archives a workflow from the lifecycle controls', async () => {
    const definition = { nodes: [{ id: 'manual-start', type: 'manual' }] };
    listWorkflows.mockResolvedValue([
      {
        definition,
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition,
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'published',
      variables: { owner: 'ops' },
      version: 2,
    });
    deleteWorkflow.mockResolvedValue({
      definition,
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'archived',
      variables: { owner: 'ops' },
      version: 3,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.click(screen.getByRole('button', { name: 'Publish Incident triage' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition,
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'published',
        variables: { owner: 'ops' },
      });
    });
    expect(screen.getByText('Status: published')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Archive Incident triage' }));

    await waitFor(() => {
      expect(deleteWorkflow).toHaveBeenCalledWith('workflow_1');
    });
    expect(screen.getByText('Status: archived')).toBeInTheDocument();
  });

  it('shows synced scheduled task metadata and runs it after publishing a scheduled workflow', async () => {
    const definition = {
      nodes: [{ id: 'manual-start', type: 'manual' }],
      triggers: {
        schedule: [{ cronExpression: '0 9 * * 1', enabled: true, id: 'daily-report' }],
      },
    };
    listWorkflows.mockResolvedValue([
      {
        definition,
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    listScheduledTasks
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([
        {
          cronExpression: '0 9 * * 1',
          enabled: true,
          id: 'sched_1',
          nextRunAt: '2026-06-05T09:00:00Z',
          targetId: 'workflow_1',
          targetType: 'workflow',
          workflowTriggerId: 'daily-report',
        },
        {
          cronExpression: '* * * * *',
          enabled: true,
          id: 'sched_other',
          nextRunAt: '2026-06-05T10:00:00Z',
          targetId: 'workflow_other',
          targetType: 'workflow',
          workflowTriggerId: 'other',
        },
      ]);
    updateWorkflow.mockResolvedValue({
      definition,
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'published',
      variables: { owner: 'ops' },
      version: 2,
    });
    runScheduledTaskNow.mockResolvedValue({
      id: 'schedrun_1',
      scheduledTaskId: 'sched_1',
      status: 'running',
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.click(screen.getByRole('button', { name: 'Publish Incident triage' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition,
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'published',
        variables: { owner: 'ops' },
      });
    });
    await waitFor(() => {
      expect(listScheduledTasks).toHaveBeenCalledTimes(2);
    });

    const scheduledTasks = screen.getByLabelText('Scheduled tasks for Incident triage');
    expect(within(scheduledTasks).getByText('sched_1')).toBeInTheDocument();
    expect(within(scheduledTasks).getByText('daily-report')).toBeInTheDocument();
    expect(within(scheduledTasks).getByText('0 9 * * 1')).toBeInTheDocument();
    expect(within(scheduledTasks).getByText('Next: 2026-06-05T09:00:00Z')).toBeInTheDocument();
    expect(within(scheduledTasks).queryByText('sched_other')).not.toBeInTheDocument();

    fireEvent.click(within(scheduledTasks).getByRole('button', { name: 'Run scheduled task sched_1 now' }));

    await waitFor(() => {
      expect(runScheduledTaskNow).toHaveBeenCalledWith('sched_1');
    });
    expect(screen.getByText('Scheduled task run schedrun_1 status: running.')).toBeInTheDocument();
    expect(within(scheduledTasks).getByText('Recent runs')).toBeInTheDocument();
    expect(within(scheduledTasks).getByText('schedrun_1')).toBeInTheDocument();
    expect(within(scheduledTasks).getByText('running')).toBeInTheDocument();
  });

  it('executes a workflow with edited manual input JSON', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        version: 1,
      },
    ]);
    executeWorkflow.mockResolvedValue({
      id: 'wexec_1',
      input: { priority: 'critical', ticket: 'INC-1' },
      status: 'running',
      workflowId: 'workflow_1',
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.change(screen.getByLabelText('Run input JSON for Incident triage'), {
      target: { value: '{ "ticket": "INC-1", "priority": "critical" }' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Run Incident triage' }));

    await waitFor(() => {
      expect(executeWorkflow).toHaveBeenCalledWith('workflow_1', {
        executionMode: 'auto',
        input: { priority: 'critical', ticket: 'INC-1' },
      });
    });
    expect(screen.getByText('Execution wexec_1 status: Running.')).toBeInTheDocument();
  });

  it('does not execute a workflow when manual input JSON is invalid', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        version: 1,
      },
    ]);

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.change(screen.getByLabelText('Run input JSON for Incident triage'), {
      target: { value: '[' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Run Incident triage' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Run input JSON must be a JSON object.');
    expect(executeWorkflow).not.toHaveBeenCalled();
  });

  it('triggers a webhook run with edited payload and prepends the returned execution', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: { webhook: { id: 'github' } },
        },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'published',
        version: 2,
      },
    ]);
    triggerWorkflowWebhook.mockResolvedValue({
      id: 'wexec_webhook',
      input: { action: 'opened', source: 'github' },
      status: 'running',
      workflowId: 'workflow_1',
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.change(screen.getByLabelText('Webhook payload JSON for Incident triage'), {
      target: { value: '{ "source": "github", "action": "opened" }' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Trigger webhook for Incident triage' }));

    await waitFor(() => {
      expect(triggerWorkflowWebhook).toHaveBeenCalledWith('workflow_1', {
        action: 'opened',
        source: 'github',
      });
    });
    expect(screen.getByText('Execution wexec_webhook status: Running.')).toBeInTheDocument();
    expect(screen.getByText('wexec_webhook')).toBeInTheDocument();
    expect(screen.getByLabelText('Workflow execution wexec_webhook status')).toHaveTextContent('Running');
  });

  it('shows public signed webhook helper details from the configured trigger', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: {
            webhook: {
              id: 'github',
              path: '/api/v1/workflows/webhooks/org_1/workflow_1',
              secret: 'top-secret',
            },
          },
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        organizationId: 'org_1',
        status: 'published',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    const helper = screen.getByLabelText('Signed webhook helper for Incident triage');

    expect(within(helper).getByText('Public signed webhook')).toBeInTheDocument();
    expect(within(helper).getByText('/api/v1/workflows/webhooks/org_1/workflow_1')).toBeInTheDocument();
    expect(within(helper).getByText('X-Oblivious-Timestamp')).toBeInTheDocument();
    expect(within(helper).getByText('X-Oblivious-Signature')).toBeInTheDocument();
    expect(within(helper).getByText('HMAC-SHA256(timestamp + "." + raw_body, webhook secret)')).toBeInTheDocument();
    expect(within(helper).getByText(/openssl dgst -sha256 -hmac "\$WEBHOOK_SECRET"/)).toBeInTheDocument();
    expect(
      within(helper).getByText(/curl -X POST "\$APP_ORIGIN\/api\/v1\/workflows\/webhooks\/org_1\/workflow_1"/)
    ).toBeInTheDocument();
  });

  it('shows signed webhook setup gaps when path and secret are missing', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          triggers: { webhook: { id: 'github' } },
        },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        version: 1,
      },
    ]);

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    const helper = screen.getByLabelText('Signed webhook helper for Incident triage');

    expect(within(helper).getByText('Webhook path is not configured.')).toBeInTheDocument();
    expect(within(helper).getByText('Webhook secret is required before public signed calls work.')).toBeInTheDocument();
    expect(within(helper).getByText('/api/v1/workflows/webhooks/{organization_id}/workflow_1')).toBeInTheDocument();
  });

  it('saves workflow resource policy fields into the workflow definition', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          totalTokens: 999,
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        nodes: [{ id: 'manual-start', type: 'manual' }],
        max_concurrent_executions: 2,
        concurrency_overflow: 'reject',
        max_execution_duration_seconds: 900,
        max_tokens_budget: 12000,
        max_node_executions: 500,
      },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.change(screen.getByLabelText('Max concurrent executions for Incident triage'), {
      target: { value: '2' },
    });
    fireEvent.change(screen.getByLabelText('Concurrency overflow for Incident triage'), {
      target: { value: 'reject' },
    });
    fireEvent.change(screen.getByLabelText('Max execution duration seconds for Incident triage'), {
      target: { value: '900' },
    });
    fireEvent.change(screen.getByLabelText('Max tokens budget for Incident triage'), {
      target: { value: '12000' },
    });
    fireEvent.change(screen.getByLabelText('Max node executions for Incident triage'), {
      target: { value: '500' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save resource policy for Incident triage' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
          max_concurrent_executions: 2,
          concurrency_overflow: 'reject',
          max_execution_duration_seconds: 900,
          max_tokens_budget: 12000,
          max_node_executions: 500,
        },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });

    const payload = updateWorkflow.mock.calls[0]?.[1] as { definition: Record<string, unknown> };
    expect(payload.definition).not.toHaveProperty('maxConcurrentExecutions');
    expect(payload.definition).not.toHaveProperty('totalTokens');
    expect(payload.definition).not.toHaveProperty('nodeExecutionCount');
    expect(payload.definition).not.toHaveProperty('workflow');
    expect(payload.definition).not.toHaveProperty('limits');
    expect(screen.getByText('Version: 2')).toBeInTheDocument();
  });

  it('tests a workflow node and controls recent executions from the debug area', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [
            { id: 'manual-start', type: 'manual' },
            { id: 'notify', type: 'http' },
          ],
        },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        version: 1,
      },
    ]);
    listExecutions.mockResolvedValue([
      {
        id: 'wexec_1',
        output: { routed: true },
        status: 'running',
        workflowId: 'workflow_1',
      },
    ]);
    testNode.mockResolvedValue({
      nodeId: 'notify',
      output: { ok: true },
      status: 'succeeded',
      workflowId: 'workflow_1',
    });
    pauseExecution.mockResolvedValue({ id: 'wexec_1', status: 'paused', workflowId: 'workflow_1' });
    resumeExecution.mockResolvedValue({ id: 'wexec_1', status: 'running', workflowId: 'workflow_1' });
    cancelExecution.mockResolvedValue({ id: 'wexec_1', status: 'cancelled', workflowId: 'workflow_1' });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.change(screen.getByLabelText('Node ID'), { target: { value: 'notify' } });
    fireEvent.change(screen.getByLabelText('Node input JSON'), { target: { value: '{ "ticket": "INC-1" }' } });
    fireEvent.click(screen.getByRole('button', { name: 'Test node' }));

    await waitFor(() => {
      expect(testNode).toHaveBeenCalledWith('workflow_1', {
        input: { ticket: 'INC-1' },
        nodeId: 'notify',
      });
    });
    expect(screen.getByText('Node notify returned succeeded')).toBeInTheDocument();
    expect(screen.getByText(/"ok": true/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Load executions' }));

    await waitFor(() => {
      expect(listExecutions).toHaveBeenCalledWith('workflow_1');
    });
    expect(screen.getByText('wexec_1')).toBeInTheDocument();
    expect(screen.getByLabelText('Workflow execution wexec_1 status')).toHaveTextContent('Running');
    expect(screen.getByText(/"routed": true/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Pause wexec_1' }));
    await waitFor(() => {
      expect(pauseExecution).toHaveBeenCalledWith('workflow_1', 'wexec_1');
    });
    expect(screen.getByLabelText('Workflow execution wexec_1 status')).toHaveTextContent('Paused');

    fireEvent.click(screen.getByRole('button', { name: 'Resume wexec_1' }));
    await waitFor(() => {
      expect(resumeExecution).toHaveBeenCalledWith('workflow_1', 'wexec_1');
    });
    expect(screen.getByLabelText('Workflow execution wexec_1 status')).toHaveTextContent('Running');

    fireEvent.click(screen.getByRole('button', { name: 'Cancel wexec_1' }));
    await waitFor(() => {
      expect(cancelExecution).toHaveBeenCalledWith('workflow_1', 'wexec_1');
    });
    expect(screen.getByLabelText('Workflow execution wexec_1 status')).toHaveTextContent('Cancelled');
  });

  it('displays failed node test results inside the workflow debug area', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [
            { id: 'manual-start', type: 'manual' },
            { id: 'notify', type: 'http' },
          ],
        },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        version: 1,
      },
    ]);
    testNode.mockResolvedValue({
      durationMs: 12,
      error: { message: 'upstream timeout' },
      input: { ticket: 'INC-1' },
      nodeId: 'notify',
      output: { statusCode: 500 },
      status: 'failed',
      trace: [{ nodeId: 'notify', status: 'failed' }],
      workflowId: 'workflow_1',
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.change(screen.getByLabelText('Node ID'), { target: { value: 'notify' } });
    fireEvent.change(screen.getByLabelText('Node input JSON'), { target: { value: '{ "ticket": "INC-1" }' } });
    fireEvent.click(screen.getByRole('button', { name: 'Test node' }));

    const debugArea = await screen.findByLabelText('Debug Incident triage');
    await waitFor(() => {
      expect(testNode).toHaveBeenCalledWith('workflow_1', {
        input: { ticket: 'INC-1' },
        nodeId: 'notify',
      });
    });
    expect(within(debugArea).getByText('Node notify returned failed')).toBeInTheDocument();
    expect(within(debugArea).getByText(/"message": "upstream timeout"/)).toBeInTheDocument();
    expect(within(debugArea).getByText(/"statusCode": 500/)).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('renders a visual workflow preview and opens node input details when a node is selected', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [
            { id: 'manual-start', input: { priority: 'high' }, status: 'ready', type: 'manual' },
            { id: 'classify-ticket', input: { model: 'triage-v1' }, status: 'draft', type: 'ai' },
            { id: 'notify-team', input: { channel: '#incidents' }, type: 'http' },
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
    expect(visualEditor).toBeInTheDocument();
    expect(within(visualEditor).getByRole('button', { name: 'Node 1 manual-start manual ready' })).toBeInTheDocument();
    expect(within(visualEditor).getByRole('button', { name: 'Node 2 classify-ticket ai draft' })).toBeInTheDocument();
    expect(within(visualEditor).getByRole('button', { name: 'Node 3 notify-team http unknown' })).toBeInTheDocument();
    expect(within(visualEditor).getByText('manual-start -> classify-ticket -> notify-team')).toBeInTheDocument();

    fireEvent.click(within(visualEditor).getByRole('button', { name: 'Node 2 classify-ticket ai draft' }));

    expect(within(visualEditor).getByText('Selected node: classify-ticket')).toBeInTheDocument();
    expect(within(visualEditor).getByText('Type: ai')).toBeInTheDocument();
    expect(within(visualEditor).getByText('Status: draft')).toBeInTheDocument();
    expect(within(visualEditor).getByText(/"model": "triage-v1"/)).toBeInTheDocument();
  });

  it('renders workflow canvas palette snap controls and auto-arranges nodes on a 20px grid', async () => {
    const definition = {
      edges: [
        { source: 'manual-start', target: 'classify-ticket' },
        { source: 'classify-ticket', target: 'notify-team' },
      ],
      nodes: [
        { id: 'manual-start', position: { x: 13, y: 17 }, type: 'manual' },
        { id: 'classify-ticket', type: 'llm' },
        { id: 'notify-team', position: { x: 323, y: 91 }, type: 'http' },
      ],
    };
    listWorkflows.mockResolvedValue([
      {
        definition,
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        ...definition,
        nodes: [
          { id: 'manual-start', position: { x: 80, y: 80 }, type: 'manual' },
          { id: 'classify-ticket', position: { x: 320, y: 80 }, type: 'llm' },
          { id: 'notify-team', position: { x: 560, y: 80 }, type: 'http' },
        ],
      },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    const visualEditor = screen.getByLabelText('Visual editor for Incident triage');
    const palette = within(visualEditor).getByLabelText('Node palette for Incident triage');
    const canvas = within(visualEditor).getByLabelText('React Flow canvas for Incident triage');

    expect(palette).toHaveTextContent('LLM');
    expect(palette).toHaveTextContent('Knowledge');
    expect(palette).toHaveTextContent('HTTP');
    expect(palette).toHaveTextContent('Agent');
    expect(within(palette).getAllByRole('button')).toHaveLength(22);
    expect(within(visualEditor).getByLabelText('Snap to grid for Incident triage')).toBeChecked();
    expect(within(visualEditor).getByText('Grid: 20px')).toBeInTheDocument();
    expect(within(canvas).getByRole('button', { name: 'Canvas node 1 manual-start manual at 20 20' })).toBeInTheDocument();
    expect(within(canvas).getByRole('button', { name: 'Canvas node 2 classify-ticket llm at 320 80' })).toBeInTheDocument();
    expect(within(canvas).getByLabelText('Canvas edge manual-start to classify-ticket')).toBeInTheDocument();

    fireEvent.click(within(visualEditor).getByRole('button', { name: 'Auto arrange nodes for Incident triage' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: {
          edges: [
            { source: 'manual-start', target: 'classify-ticket' },
            { source: 'classify-ticket', target: 'notify-team' },
          ],
          nodes: [
            { id: 'manual-start', position: { x: 80, y: 80 }, type: 'manual' },
            { id: 'classify-ticket', position: { x: 320, y: 80 }, type: 'llm' },
            { id: 'notify-team', position: { x: 560, y: 80 }, type: 'http' },
          ],
        },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
    const payload = updateWorkflow.mock.calls[0]?.[1] as { definition: { nodes: Array<{ position: { x: number; y: number } }> } };
    for (const node of payload.definition.nodes) {
      expect(node.position.x % 20).toBe(0);
      expect(node.position.y % 20).toBe(0);
    }
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
    expect(within(canvas).getByText('manual-start')).toBeInTheDocument();
    expect(within(canvas).getByText('classify-ticket')).toBeInTheDocument();
    expect(within(visualEditor).getByLabelText('Node palette for Incident triage')).toBeInTheDocument();
    expect(within(visualEditor).getAllByRole('button', { name: /Add .* node template to Incident triage/ })).toHaveLength(
      22
    );
  });

  it('loads executable debug templates when selecting workflow runtime nodes', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [
            { id: 'run-code', type: 'code' },
            { id: 'call-tool', type: 'tool' },
            { id: 'query-db', type: 'database' },
            { id: 'open-page', type: 'rpa' },
          ],
        },
        id: 'workflow_runtime',
        name: 'Runtime contract',
        status: 'draft',
        version: 1,
      },
    ]);
    testNode.mockResolvedValue({
      nodeId: 'call-tool',
      output: { content: 'ok' },
      status: 'succeeded',
      workflowId: 'workflow_runtime',
    });

    render(<WorkflowsPage />);

    await screen.findByText('Runtime contract');
    const visualEditor = screen.getByLabelText('Visual editor for Runtime contract');

    fireEvent.click(within(visualEditor).getByRole('button', { name: 'Node 1 run-code code unknown' }));
    expect(screen.getByLabelText('Node input JSON')).toHaveValue(
      '{\n' +
        '  "language": "javascript",\n' +
        '  "code": "return { result: inputs.value };",\n' +
        '  "inputs": {\n' +
        '    "value": "sample"\n' +
        '  },\n' +
        '  "timeoutMs": 30000\n' +
        '}'
    );

    fireEvent.click(within(visualEditor).getByRole('button', { name: 'Node 2 call-tool tool unknown' }));
    expect(screen.getByLabelText('Node input JSON')).toHaveValue(
      '{\n' +
        '  "toolName": "web_search",\n' +
        '  "toolType": "builtin",\n' +
        '  "arguments": {\n' +
        '    "query": "Oblivious workflow runtime"\n' +
        '  }\n' +
        '}'
    );

    fireEvent.click(within(visualEditor).getByRole('button', { name: 'Node 3 query-db database unknown' }));
    expect(screen.getByLabelText('Node input JSON')).toHaveValue(
      '{\n' +
        '  "connectionId": "platform",\n' +
        '  "query": "SELECT id, name FROM workflows WHERE organization_id = $1",\n' +
        '  "parameters": [\n' +
        '    "{{org.id}}"\n' +
        '  ],\n' +
        '  "limit": 20,\n' +
        '  "readOnly": true\n' +
        '}'
    );

    fireEvent.click(within(visualEditor).getByRole('button', { name: 'Node 4 open-page rpa unknown' }));
    expect(screen.getByLabelText('Node input JSON')).toHaveValue(
      '{\n' +
        '  "targetUrl": "https://example.com",\n' +
        '  "browserMode": "headless",\n' +
        '  "screenshot": true,\n' +
        '  "timeoutMs": 60000,\n' +
        '  "steps": [\n' +
        '    {\n' +
        '      "action": "goto",\n' +
        '      "value": "https://example.com"\n' +
        '    },\n' +
        '    {\n' +
        '      "action": "extract",\n' +
        '      "selector": "body"\n' +
        '    }\n' +
        '  ]\n' +
        '}'
    );

    fireEvent.click(screen.getByRole('button', { name: 'Test node' }));

    await waitFor(() => {
      expect(testNode).toHaveBeenCalledWith('workflow_runtime', {
        input: {
          browserMode: 'headless',
          screenshot: true,
          steps: [
            { action: 'goto', value: 'https://example.com' },
            { action: 'extract', selector: 'body' },
          ],
          targetUrl: 'https://example.com',
          timeoutMs: 60000,
        },
        nodeId: 'open-page',
      });
    });
  });

  it('saves selected runtime node debug JSON into the workflow definition input', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          edges: [{ source: 'run-code', target: 'call-tool' }],
          nodes: [
            { id: 'run-code', type: 'code' },
            { id: 'call-tool', type: 'tool' },
          ],
        },
        description: 'Runtime workflow',
        id: 'workflow_runtime',
        name: 'Runtime contract',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        edges: [{ source: 'run-code', target: 'call-tool' }],
        nodes: [
          {
            failurePolicy: { strategy: 'auto_retry' },
            id: 'run-code',
            input: {
              code: 'return { result: inputs.count + 1 };',
              inputs: { count: 41 },
              language: 'javascript',
              timeoutMs: 15000,
            },
            type: 'code',
          },
          { id: 'call-tool', type: 'tool' },
        ],
      },
      description: 'Runtime workflow',
      id: 'workflow_runtime',
      name: 'Runtime contract',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Runtime contract');
    const visualEditor = screen.getByLabelText('Visual editor for Runtime contract');
    fireEvent.click(within(visualEditor).getByRole('button', { name: 'Node 1 run-code code unknown' }));
    fireEvent.change(screen.getByLabelText('Node input JSON'), {
      target: {
        value:
          '{ "language": "javascript", "code": "return { result: inputs.count + 1 };", "inputs": { "count": 41 }, "timeoutMs": 15000 }',
      },
    });
    fireEvent.change(screen.getByLabelText('Failure strategy for run-code'), {
      target: { value: 'auto_retry' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save node run-code configuration' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_runtime', {
        definition: {
          edges: [{ source: 'run-code', target: 'call-tool' }],
          nodes: [
            {
              failurePolicy: { strategy: 'auto_retry' },
              id: 'run-code',
              input: {
                code: 'return { result: inputs.count + 1 };',
                inputs: { count: 41 },
                language: 'javascript',
                timeoutMs: 15000,
              },
              type: 'code',
            },
            { id: 'call-tool', type: 'tool' },
          ],
        },
        description: 'Runtime workflow',
        name: 'Runtime contract',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
    expect(screen.getByText('Version: 2')).toBeInTheDocument();
    expect(within(visualEditor).getByText(/"count": 41/)).toBeInTheDocument();
  });

  it('saves a selected visual node failure strategy into the workflow definition', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          edges: [{ source: 'manual-start', target: 'classify-ticket' }],
          nodes: [
            { id: 'manual-start', type: 'manual' },
            { id: 'classify-ticket', input: { model: 'triage-v1' }, type: 'ai' },
          ],
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        edges: [{ source: 'manual-start', target: 'classify-ticket' }],
        nodes: [
          { id: 'manual-start', type: 'manual' },
          {
            failurePolicy: { strategy: 'skip_on_failure' },
            id: 'classify-ticket',
            input: { model: 'triage-v1' },
            type: 'ai',
          },
        ],
      },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');

    const visualEditor = screen.getByLabelText('Visual editor for Incident triage');
    fireEvent.click(within(visualEditor).getByRole('button', { name: 'Node 2 classify-ticket ai unknown' }));
    fireEvent.change(screen.getByLabelText('Failure strategy for classify-ticket'), {
      target: { value: 'skip_on_failure' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save node classify-ticket configuration' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: {
          edges: [{ source: 'manual-start', target: 'classify-ticket' }],
          nodes: [
            { id: 'manual-start', type: 'manual' },
            {
              failurePolicy: { strategy: 'skip_on_failure' },
              id: 'classify-ticket',
              input: { model: 'triage-v1' },
              type: 'ai',
            },
          ],
        },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
    expect(within(visualEditor).getByText('Failure strategy: skip_on_failure')).toBeInTheDocument();
    expect(screen.getByText('Version: 2')).toBeInTheDocument();
  });

  it('saves selected node auto retry limits into the workflow definition', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          edges: [{ source: 'manual-start', target: 'classify-ticket' }],
          nodes: [
            { id: 'manual-start', type: 'manual' },
            {
              failurePolicy: { maxRetries: 1, retryDelays: ['500ms'], strategy: 'auto_retry' },
              id: 'classify-ticket',
              input: { model: 'triage-v1' },
              type: 'ai',
            },
          ],
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        edges: [{ source: 'manual-start', target: 'classify-ticket' }],
        nodes: [
          { id: 'manual-start', type: 'manual' },
          {
            failurePolicy: { maxRetries: 4, retryDelays: ['1s', '5s'], strategy: 'auto_retry' },
            id: 'classify-ticket',
            input: { model: 'triage-v1' },
            type: 'ai',
          },
        ],
      },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');

    const visualEditor = screen.getByLabelText('Visual editor for Incident triage');
    fireEvent.click(within(visualEditor).getByRole('button', { name: 'Node 2 classify-ticket ai unknown' }));
    expect(screen.getByLabelText('Max retries for classify-ticket')).toHaveValue(1);
    expect(screen.getByLabelText('Retry delays for classify-ticket')).toHaveValue('500ms');

    fireEvent.change(screen.getByLabelText('Max retries for classify-ticket'), {
      target: { value: '4' },
    });
    fireEvent.change(screen.getByLabelText('Retry delays for classify-ticket'), {
      target: { value: '1s, 5s' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save node classify-ticket configuration' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: {
          edges: [{ source: 'manual-start', target: 'classify-ticket' }],
          nodes: [
            { id: 'manual-start', type: 'manual' },
            {
              failurePolicy: { maxRetries: 4, retryDelays: ['1s', '5s'], strategy: 'auto_retry' },
              id: 'classify-ticket',
              input: { model: 'triage-v1' },
              type: 'ai',
            },
          ],
        },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
    expect(screen.getByText('Version: 2')).toBeInTheDocument();
  });

  it('clears selected node auto retry limits from the workflow definition', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          edges: [{ source: 'manual-start', target: 'classify-ticket' }],
          nodes: [
            { id: 'manual-start', type: 'manual' },
            {
              failurePolicy: { maxRetries: 4, retryDelays: ['1s', '5s'], strategy: 'auto_retry' },
              id: 'classify-ticket',
              input: { model: 'triage-v1' },
              type: 'ai',
            },
          ],
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        edges: [{ source: 'manual-start', target: 'classify-ticket' }],
        nodes: [
          { id: 'manual-start', type: 'manual' },
          {
            failurePolicy: { strategy: 'auto_retry' },
            id: 'classify-ticket',
            input: { model: 'triage-v1' },
            type: 'ai',
          },
        ],
      },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');

    const visualEditor = screen.getByLabelText('Visual editor for Incident triage');
    fireEvent.click(within(visualEditor).getByRole('button', { name: 'Node 2 classify-ticket ai unknown' }));
    fireEvent.change(screen.getByLabelText('Max retries for classify-ticket'), {
      target: { value: '' },
    });
    fireEvent.change(screen.getByLabelText('Retry delays for classify-ticket'), {
      target: { value: '' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save node classify-ticket configuration' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: {
          edges: [{ source: 'manual-start', target: 'classify-ticket' }],
          nodes: [
            { id: 'manual-start', type: 'manual' },
            {
              failurePolicy: { strategy: 'auto_retry' },
              id: 'classify-ticket',
              input: { model: 'triage-v1' },
              type: 'ai',
            },
          ],
        },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
    expect(screen.getByText('Version: 2')).toBeInTheDocument();
  });

  it('saves selected node failure branch target into the workflow definition', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          edges: [
            { source: 'classify-ticket', target: 'escalate' },
            { source: 'classify-ticket', target: 'manual-review' },
          ],
          nodes: [
            { id: 'classify-ticket', input: { model: 'triage-v1' }, type: 'ai' },
            { id: 'escalate', type: 'http' },
            { id: 'manual-review', type: 'manual' },
          ],
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        edges: [
          { source: 'classify-ticket', target: 'escalate' },
          { source: 'classify-ticket', target: 'manual-review' },
        ],
        nodes: [
          {
            failurePolicy: { failureBranchNodeId: 'manual-review', strategy: 'failure_branch' },
            id: 'classify-ticket',
            input: { model: 'triage-v1' },
            type: 'ai',
          },
          { id: 'escalate', type: 'http' },
          { id: 'manual-review', type: 'manual' },
        ],
      },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');

    const visualEditor = screen.getByLabelText('Visual editor for Incident triage');
    fireEvent.click(within(visualEditor).getByRole('button', { name: 'Node 1 classify-ticket ai unknown' }));
    fireEvent.change(screen.getByLabelText('Failure strategy for classify-ticket'), {
      target: { value: 'failure_branch' },
    });
    fireEvent.change(screen.getByLabelText('Failure branch node for classify-ticket'), {
      target: { value: 'manual-review' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save node classify-ticket configuration' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: {
          edges: [
            { source: 'classify-ticket', target: 'escalate' },
            { source: 'classify-ticket', target: 'manual-review' },
          ],
          nodes: [
            {
              failurePolicy: { failureBranchNodeId: 'manual-review', strategy: 'failure_branch' },
              id: 'classify-ticket',
              input: { model: 'triage-v1' },
              type: 'ai',
            },
            { id: 'escalate', type: 'http' },
            { id: 'manual-review', type: 'manual' },
          ],
        },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
    expect(screen.getByText('Version: 2')).toBeInTheDocument();
  });

  it('adds workflow nodes and edges with structured definition controls', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [{ id: 'manual-start', type: 'manual' }],
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        edges: [{ source: 'manual-start', target: 'notify-team' }],
        nodes: [
          { id: 'manual-start', type: 'manual' },
          { id: 'notify-team', input: { method: 'POST', url: 'https://hooks.example.test/incidents' }, type: 'http' },
        ],
      },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');

    const visualEditor = screen.getByLabelText('Visual editor for Incident triage');
    fireEvent.change(screen.getByLabelText('New node ID for Incident triage'), {
      target: { value: 'notify-team' },
    });
    fireEvent.change(screen.getByLabelText('New node type for Incident triage'), {
      target: { value: 'http' },
    });
    fireEvent.change(screen.getByLabelText('New node input JSON for Incident triage'), {
      target: { value: '{ "method": "POST", "url": "https://hooks.example.test/incidents" }' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add node to Incident triage' }));

    expect(within(visualEditor).getByRole('button', { name: 'Node 2 notify-team http unknown' })).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('New edge source for Incident triage'), {
      target: { value: 'manual-start' },
    });
    fireEvent.change(screen.getByLabelText('New edge target for Incident triage'), {
      target: { value: 'notify-team' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add edge to Incident triage' }));

    expect(within(visualEditor).getByText('manual-start -> notify-team')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Save definition for Incident triage' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: {
          edges: [{ source: 'manual-start', target: 'notify-team' }],
          nodes: [
            { id: 'manual-start', type: 'manual' },
            { id: 'notify-team', input: { method: 'POST', url: 'https://hooks.example.test/incidents' }, type: 'http' },
          ],
        },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
    expect(screen.getByText('Version: 2')).toBeInTheDocument();
  });

  it('adds conditional workflow edges with branch metadata', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [
            { id: 'check-priority', type: 'condition' },
            { id: 'escalate', type: 'http' },
            { id: 'archive', type: 'manual' },
          ],
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        edges: [{ branch: 'true', source: 'check-priority', target: 'escalate' }],
        nodes: [
          { id: 'check-priority', type: 'condition' },
          { id: 'escalate', type: 'http' },
          { id: 'archive', type: 'manual' },
        ],
      },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    const visualEditor = screen.getByLabelText('Visual editor for Incident triage');

    fireEvent.change(screen.getByLabelText('New edge source for Incident triage'), {
      target: { value: 'check-priority' },
    });
    fireEvent.change(screen.getByLabelText('New edge target for Incident triage'), {
      target: { value: 'escalate' },
    });
    fireEvent.change(screen.getByLabelText('New edge branch for Incident triage'), {
      target: { value: 'true' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add edge to Incident triage' }));

    expect(within(visualEditor).getByText('check-priority -> escalate [true]')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Save definition for Incident triage' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: {
          edges: [{ branch: 'true', source: 'check-priority', target: 'escalate' }],
          nodes: [
            { id: 'check-priority', type: 'condition' },
            { id: 'escalate', type: 'http' },
            { id: 'archive', type: 'manual' },
          ],
        },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
    expect(screen.getByText('Version: 2')).toBeInTheDocument();
  });

  it('removes workflow definition edges before saving', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          edges: [
            { branch: 'true', source: 'check-priority', target: 'escalate' },
            { branch: 'false', source: 'check-priority', target: 'archive' },
          ],
          nodes: [
            { id: 'check-priority', type: 'condition' },
            { id: 'escalate', type: 'http' },
            { id: 'archive', type: 'manual' },
          ],
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        edges: [{ branch: 'true', source: 'check-priority', target: 'escalate' }],
        nodes: [
          { id: 'check-priority', type: 'condition' },
          { id: 'escalate', type: 'http' },
          { id: 'archive', type: 'manual' },
        ],
      },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    const visualEditor = screen.getByLabelText('Visual editor for Incident triage');

    expect(within(visualEditor).getByText('check-priority -> escalate [true]')).toBeInTheDocument();
    expect(within(visualEditor).getByText('check-priority -> archive [false]')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Remove edge check-priority to archive from Incident triage' }));

    expect(within(visualEditor).queryByText('check-priority -> archive [false]')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Save definition for Incident triage' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: {
          edges: [{ branch: 'true', source: 'check-priority', target: 'escalate' }],
          nodes: [
            { id: 'check-priority', type: 'condition' },
            { id: 'escalate', type: 'http' },
            { id: 'archive', type: 'manual' },
          ],
        },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
    expect(screen.getByText('Version: 2')).toBeInTheDocument();
  });

  it('removes workflow definition nodes and connected edges before saving', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          edges: [
            { branch: 'true', source: 'check-priority', target: 'escalate' },
            { branch: 'false', source: 'check-priority', target: 'archive' },
            { source: 'archive', target: 'close-ticket' },
          ],
          nodes: [
            { id: 'check-priority', type: 'condition' },
            { id: 'escalate', type: 'http' },
            { id: 'archive', type: 'manual' },
            { id: 'close-ticket', type: 'http' },
          ],
        },
        description: 'Existing workflow',
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
        version: 1,
      },
    ]);
    updateWorkflow.mockResolvedValue({
      definition: {
        edges: [{ branch: 'true', source: 'check-priority', target: 'escalate' }],
        nodes: [
          { id: 'check-priority', type: 'condition' },
          { id: 'escalate', type: 'http' },
          { id: 'close-ticket', type: 'http' },
        ],
      },
      description: 'Existing workflow',
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      variables: { owner: 'ops' },
      version: 2,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    const visualEditor = screen.getByLabelText('Visual editor for Incident triage');

    fireEvent.click(within(visualEditor).getByRole('button', { name: 'Node 3 archive manual unknown' }));
    fireEvent.click(screen.getByRole('button', { name: 'Remove node archive from Incident triage' }));

    expect(within(visualEditor).queryByRole('button', { name: 'Node 3 archive manual unknown' })).not.toBeInTheDocument();
    expect(within(visualEditor).queryByText('check-priority -> archive [false]')).not.toBeInTheDocument();
    expect(within(visualEditor).queryByText('archive -> close-ticket')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Save definition for Incident triage' }));

    await waitFor(() => {
      expect(updateWorkflow).toHaveBeenCalledWith('workflow_1', {
        definition: {
          edges: [{ branch: 'true', source: 'check-priority', target: 'escalate' }],
          nodes: [
            { id: 'check-priority', type: 'condition' },
            { id: 'escalate', type: 'http' },
            { id: 'close-ticket', type: 'http' },
          ],
        },
        description: 'Existing workflow',
        name: 'Incident triage',
        status: 'draft',
        variables: { owner: 'ops' },
      });
    });
    expect(screen.getByText('Version: 2')).toBeInTheDocument();
  });

  it('renders an empty visual preview state when a workflow has no nodes', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: { nodes: [] },
        id: 'workflow_empty',
        name: 'Empty workflow',
        status: 'draft',
        version: 1,
      },
    ]);

    render(<WorkflowsPage />);

    await screen.findByText('Empty workflow');

    expect(screen.getByLabelText('Visual editor for Empty workflow')).toBeInTheDocument();
    expect(screen.getByText('No nodes in this workflow definition yet.')).toBeInTheDocument();
  });

  it('renders definition edges and shows selected node connections', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          edges: [
            { id: 'edge_manual_classify', source: 'manual-start', target: 'classify-ticket' },
            { source: 'classify-ticket', target: 'notify-team' },
            { source: 'missing-node', target: 'notify-team' },
          ],
          nodes: [
            { id: 'manual-start', type: 'manual' },
            { id: 'classify-ticket', type: 'ai' },
            { id: 'notify-team', type: 'http' },
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
    expect(within(visualEditor).getByText('Edges: 2 active, 1 invalid')).toBeInTheDocument();
    expect(within(visualEditor).getByText('manual-start -> classify-ticket')).toBeInTheDocument();
    expect(within(visualEditor).getByText('classify-ticket -> notify-team')).toBeInTheDocument();
    expect(within(visualEditor).getByText('Invalid edge: missing-node -> notify-team')).toBeInTheDocument();

    fireEvent.click(within(visualEditor).getByRole('button', { name: 'Node 2 classify-ticket ai unknown' }));

    expect(within(visualEditor).getByText('Incoming: manual-start')).toBeInTheDocument();
    expect(within(visualEditor).getByText('Outgoing: notify-team')).toBeInTheDocument();
  });

  it('highlights visual workflow nodes from the latest loaded execution status', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [
            { id: 'manual-start', status: 'ready', type: 'manual' },
            { id: 'classify-ticket', status: 'draft', type: 'ai' },
            { id: 'notify-team', type: 'http' },
          ],
        },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        version: 1,
      },
    ]);
    listExecutions.mockResolvedValue([
      {
        id: 'wexec_1',
        nodeExecutions: [
          { durationMs: 40, nodeId: 'manual-start', status: 'succeeded' },
          { durationMs: 480, nodeId: 'classify-ticket', status: 'failed' },
          { durationMs: 120, nodeId: 'notify-team', status: 'running' },
        ],
        status: 'running',
        workflowId: 'workflow_1',
      },
    ]);

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.click(screen.getByRole('button', { name: 'Load executions' }));

    await waitFor(() => {
      expect(listExecutions).toHaveBeenCalledWith('workflow_1');
    });

    const visualEditor = screen.getByLabelText('Visual editor for Incident triage');
    expect(
      within(visualEditor).getByRole('button', { name: 'Node 1 manual-start manual succeeded 40ms' })
    ).toBeInTheDocument();
    expect(
      within(visualEditor).getByRole('button', { name: 'Node 2 classify-ticket ai failed 480ms' })
    ).toBeInTheDocument();
    expect(
      within(visualEditor).getByRole('button', { name: 'Node 3 notify-team http running 120ms' })
    ).toBeInTheDocument();

    fireEvent.click(within(visualEditor).getByRole('button', { name: 'Node 2 classify-ticket ai failed 480ms' }));

    expect(within(visualEditor).getByText('Status: failed')).toBeInTheDocument();
    expect(within(visualEditor).getByText('Duration: 480ms')).toBeInTheDocument();
  });

  it('renders queued and terminal workflow execution statuses with readable labels', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        version: 1,
      },
    ]);
    listExecutions.mockResolvedValue([
      { id: 'wexec_queued', status: 'queued', workflowId: 'workflow_1' },
      { id: 'wexec_timeout', status: 'timeout', workflowId: 'workflow_1' },
      { id: 'wexec_max_iterations', status: 'max_iterations', workflowId: 'workflow_1' },
    ]);

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.click(screen.getByRole('button', { name: 'Load executions' }));

    await waitFor(() => {
      expect(listExecutions).toHaveBeenCalledWith('workflow_1');
    });

    expect(screen.getByLabelText('Workflow execution wexec_queued status')).toHaveTextContent('Queued');
    expect(screen.getByLabelText('Workflow execution wexec_timeout status')).toHaveTextContent('Timed out');
    expect(screen.getByLabelText('Workflow execution wexec_max_iterations status')).toHaveTextContent('Max iterations');
  });

  it('summarizes workflow debug and performance signals from node executions', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [
            { id: 'manual-start', type: 'manual' },
            { id: 'classify-ticket', type: 'ai' },
            { id: 'notify-team', type: 'http' },
          ],
        },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        version: 1,
      },
    ]);
    listExecutions.mockResolvedValue([
      {
        id: 'wexec_1',
        nodeExecutions: [
          {
            costUsd: 0,
            durationMs: 40,
            nodeId: 'manual-start',
            output: { ticket: 'INC-1' },
            status: 'succeeded',
            tokens: 0,
          },
          {
            costUsd: 0.042,
            durationMs: 480,
            nodeId: 'classify-ticket',
            output: { severity: 'critical' },
            status: 'failed',
            tokens: 900,
          },
          {
            costUsd: 0.008,
            durationMs: 120,
            nodeId: 'notify-team',
            output: { channel: '#incidents' },
            status: 'retrying',
            tokens: 100,
          },
        ],
        status: 'failed',
        workflowId: 'workflow_1',
      },
    ]);

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.click(screen.getByRole('button', { name: 'Load executions' }));

    await waitFor(() => {
      expect(listExecutions).toHaveBeenCalledWith('workflow_1');
    });

    const summary = screen.getByLabelText('Debug and performance summary for wexec_1');
    expect(within(summary).getByText('Call chain: manual-start -> classify-ticket -> notify-team')).toBeInTheDocument();
    expect(within(summary).getByText('Total duration: 640ms')).toBeInTheDocument();
    expect(within(summary).getByText('Failed nodes: 1')).toBeInTheDocument();
    expect(within(summary).getByText('Retrying nodes: 1')).toBeInTheDocument();
    expect(within(summary).getByText('Slowest node: classify-ticket (480ms)')).toBeInTheDocument();
    expect(within(summary).getByText('classify-ticket | failed | 480ms | 900 tokens | $0.042')).toBeInTheDocument();
    expect(within(summary).getByText(/"severity": "critical"/)).toBeInTheDocument();
  });

  it('loads execution debug snapshot from a recent execution and renders variables, call chain, outputs, errors, and performance', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [
            { id: 'manual-start', type: 'manual' },
            { id: 'classify-ticket', type: 'ai' },
            { id: 'notify-team', type: 'http' },
          ],
        },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        version: 1,
      },
    ]);
    listExecutions.mockResolvedValue([
      {
        id: 'wexec_1',
        status: 'failed',
        workflowId: 'workflow_1',
      },
    ]);
    getExecutionDebugSnapshot.mockResolvedValue({
      executionId: 'wexec_1',
      workflowId: 'workflow_1',
      status: 'failed',
      variableSnapshot: {
        context: { runMode: 'debug', traceId: 'trace-123' },
        input: { ticket: 'INC-1' },
        nodeOutputs: {
          'classify-ticket': { severity: 'critical' },
        },
      },
      trace: [
        {
          durationMs: 40,
          input: { ticket: 'INC-1' },
          nodeId: 'manual-start',
          output: { ticket: 'INC-1' },
          status: 'succeeded',
        },
        {
          durationMs: 480,
          error: { message: 'model unavailable' },
          input: { ticket: 'INC-1' },
          nodeId: 'classify-ticket',
          output: { severity: 'critical' },
          status: 'failed',
        },
        {
          context: { skippedBecause: 'upstream failed' },
          nodeId: 'notify-team',
          status: 'pending',
        },
      ],
      outputs: {
        'classify-ticket': { severity: 'critical' },
        execution: { routed: false },
      },
      performance: {
        bottleneckNodeId: 'classify-ticket',
        nodeDurationsMs: {
          'classify-ticket': 480,
          'manual-start': 40,
        },
        totalDurationMs: 520,
      },
      logs: [
        {
          level: 'info',
          message: 'Node manual-start succeeded in 40ms',
          nodeId: 'manual-start',
          timestamp: '2026-06-04T09:00:00Z',
        },
        {
          level: 'error',
          message: 'Node classify-ticket failed in 480ms: model unavailable',
          nodeId: 'classify-ticket',
          timestamp: '2026-06-04T09:00:01Z',
        },
      ],
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.click(screen.getByRole('button', { name: 'Load executions' }));

    await waitFor(() => {
      expect(listExecutions).toHaveBeenCalledWith('workflow_1');
    });
    fireEvent.click(screen.getByRole('button', { name: 'View details for wexec_1' }));

    await waitFor(() => {
      expect(getExecutionDebugSnapshot).toHaveBeenCalledWith('workflow_1', 'wexec_1');
    });

    const details = screen.getByLabelText('Execution debug details for wexec_1');
    expect(within(details).getByRole('heading', { name: 'Execution debug details' })).toBeInTheDocument();
    expect(within(details).getByText('wexec_1 | Failed')).toBeInTheDocument();
    expect(within(details).getByText('Variables')).toBeInTheDocument();
    expect(within(details).getByText(/"runMode": "debug"/)).toBeInTheDocument();
    expect(within(details).getAllByText(/"ticket": "INC-1"/).length).toBeGreaterThanOrEqual(1);
    expect(within(details).getByText('Call chain')).toBeInTheDocument();
    expect(within(details).getByText('manual-start -> classify-ticket -> notify-team')).toBeInTheDocument();
    expect(within(details).getByText('Outputs')).toBeInTheDocument();
    const outputsPanel = within(details).getByRole('heading', { name: 'Outputs' }).closest('section');
    expect(outputsPanel).not.toBeNull();
    expect(within(outputsPanel as HTMLElement).getByText(/"severity": "critical"/)).toBeInTheDocument();
    expect(within(details).getByText(/"routed": false/)).toBeInTheDocument();
    expect(within(details).getByText('Errors')).toBeInTheDocument();
    expect(within(details).getByText(/"message": "model unavailable"/)).toBeInTheDocument();
    expect(within(details).getByText('Logs')).toBeInTheDocument();
    expect(within(details).getByText('info | manual-start | Node manual-start succeeded in 40ms')).toBeInTheDocument();
    expect(
      within(details).getByText('error | classify-ticket | Node classify-ticket failed in 480ms: model unavailable')
    ).toBeInTheDocument();
    expect(within(details).getByText('Performance')).toBeInTheDocument();
    expect(within(details).getByText('Total duration: 520ms')).toBeInTheDocument();
    expect(within(details).getByText('Bottleneck: classify-ticket (480ms)')).toBeInTheDocument();
  });

  it('preserves loaded executions when debug snapshot loading fails', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        version: 1,
      },
    ]);
    listExecutions.mockResolvedValue([{ id: 'wexec_1', status: 'failed', workflowId: 'workflow_1' }]);
    getExecutionDebugSnapshot.mockRejectedValue(new Error());

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.click(screen.getByRole('button', { name: 'Load executions' }));

    await waitFor(() => {
      expect(listExecutions).toHaveBeenCalledWith('workflow_1');
    });
    expect(screen.getByText('wexec_1')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'View details for wexec_1' }));

    await waitFor(() => {
      expect(getExecutionDebugSnapshot).toHaveBeenCalledWith('workflow_1', 'wexec_1');
    });
    expect(screen.getByText('wexec_1')).toBeInTheDocument();
    expect(screen.getByText('Unable to load workflow execution debug snapshot.')).toBeInTheDocument();
  });

  it('checks resource limits for a loaded execution and updates the visible execution status', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        version: 1,
      },
    ]);
    listExecutions.mockResolvedValue([{ id: 'wexec_1', status: 'running', workflowId: 'workflow_1' }]);
    checkWorkflowResourceLimits.mockResolvedValue({
      id: 'wexec_1',
      status: 'max_iterations',
      workflowId: 'workflow_1',
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.click(screen.getByRole('button', { name: 'Load executions' }));

    await waitFor(() => {
      expect(listExecutions).toHaveBeenCalledWith('workflow_1');
    });

    fireEvent.change(screen.getByLabelText('Total tokens for wexec_1'), { target: { value: '2048' } });
    fireEvent.change(screen.getByLabelText('Node executions for wexec_1'), { target: { value: '1001' } });
    fireEvent.click(screen.getByRole('button', { name: 'Check resources for wexec_1' }));

    await waitFor(() => {
      expect(checkWorkflowResourceLimits).toHaveBeenCalledWith('workflow_1', 'wexec_1', {
        nodeExecutionCount: 1001,
        totalTokens: 2048,
      });
    });
    expect(screen.getByLabelText('Workflow execution wexec_1 status')).toHaveTextContent('Max iterations');
  });

  it('resolves paused workflow failures with retry, skip, edited input retry, and terminate decisions', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [
            { id: 'manual-start', type: 'manual' },
            { failurePolicy: { strategy: 'pause_on_failure' }, id: 'classify-ticket', type: 'ai' },
          ],
        },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        version: 1,
      },
    ]);
    listExecutions.mockResolvedValue([
      {
        id: 'wexec_1',
        nodeExecutions: [
          { nodeId: 'manual-start', status: 'succeeded' },
          {
            error: { message: 'model unavailable' },
            input: { priority: 'high' },
            nodeId: 'classify-ticket',
            status: 'failed',
          },
        ],
        status: 'paused',
        workflowId: 'workflow_1',
      },
    ]);
    resolvePausedFailure
      .mockResolvedValueOnce({
        id: 'wexec_1',
        nodeExecutions: [{ attempt: 2, nodeId: 'classify-ticket', status: 'pending' }],
        status: 'running',
        workflowId: 'workflow_1',
      })
      .mockResolvedValueOnce({
        id: 'wexec_1',
        nodeExecutions: [{ nodeId: 'classify-ticket', status: 'skipped' }],
        status: 'running',
        workflowId: 'workflow_1',
      })
      .mockResolvedValueOnce({
        id: 'wexec_1',
        nodeExecutions: [{ attempt: 3, input: { priority: 'urgent' }, nodeId: 'classify-ticket', status: 'pending' }],
        status: 'running',
        workflowId: 'workflow_1',
      })
      .mockResolvedValueOnce({
        id: 'wexec_1',
        nodeExecutions: [{ nodeId: 'classify-ticket', status: 'failed' }],
        status: 'failed',
        workflowId: 'workflow_1',
      });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.click(screen.getByRole('button', { name: 'Load executions' }));

    await waitFor(() => {
      expect(listExecutions).toHaveBeenCalledWith('workflow_1');
    });
    const decisionPanel = screen.getByLabelText('Paused failure decisions for wexec_1');
    expect(within(decisionPanel).getByText('Paused on failed node classify-ticket')).toBeInTheDocument();
    expect(within(decisionPanel).getByText(/"message": "model unavailable"/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Retry failed node classify-ticket for wexec_1' }));
    await waitFor(() => {
      expect(resolvePausedFailure).toHaveBeenNthCalledWith(1, 'workflow_1', 'wexec_1', {
        action: 'retry',
        nodeId: 'classify-ticket',
    });
  });

    expect(screen.getByLabelText('Workflow execution wexec_1 status')).toHaveTextContent('Running');

    fireEvent.click(screen.getByRole('button', { name: 'Load executions' }));
    await waitFor(() => expect(listExecutions).toHaveBeenCalledTimes(2));
    fireEvent.click(screen.getByRole('button', { name: 'Skip failed node classify-ticket for wexec_1' }));
    await waitFor(() => {
      expect(resolvePausedFailure).toHaveBeenNthCalledWith(2, 'workflow_1', 'wexec_1', {
        action: 'continue',
        nodeId: 'classify-ticket',
      });
    });

    fireEvent.click(screen.getByRole('button', { name: 'Load executions' }));
    await waitFor(() => expect(listExecutions).toHaveBeenCalledTimes(3));
    fireEvent.change(screen.getByLabelText('Edited retry input for classify-ticket in wexec_1'), {
      target: { value: '{ "priority": "urgent" }' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Retry classify-ticket with edited input for wexec_1' }));
    await waitFor(() => {
      expect(resolvePausedFailure).toHaveBeenNthCalledWith(3, 'workflow_1', 'wexec_1', {
        action: 'retry',
        input: { priority: 'urgent' },
        nodeId: 'classify-ticket',
      });
    });

    fireEvent.click(screen.getByRole('button', { name: 'Load executions' }));
    await waitFor(() => expect(listExecutions).toHaveBeenCalledTimes(4));
    fireEvent.click(screen.getByRole('button', { name: 'Terminate workflow wexec_1 after classify-ticket failure' }));
    await waitFor(() => {
      expect(resolvePausedFailure).toHaveBeenNthCalledWith(4, 'workflow_1', 'wexec_1', {
        action: 'fail',
        nodeId: 'classify-ticket',
      });
    });
    expect(screen.getByLabelText('Workflow execution wexec_1 status')).toHaveTextContent('Failed');
  });

  it('branches paused workflow failures to a selected next node', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [
            { id: 'manual-start', type: 'manual' },
            { failurePolicy: { strategy: 'pause_on_failure' }, id: 'classify-ticket', type: 'ai' },
            { id: 'manual-review', type: 'manual' },
            { id: 'notify-team', type: 'agent' },
          ],
        },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        version: 1,
      },
    ]);
    listExecutions.mockResolvedValue([
      {
        id: 'wexec_1',
        nodeExecutions: [
          { nodeId: 'manual-start', status: 'succeeded' },
          {
            error: { message: 'model unavailable' },
            input: { priority: 'high' },
            nodeId: 'classify-ticket',
            status: 'failed',
          },
        ],
        status: 'paused',
        workflowId: 'workflow_1',
      },
    ]);
    resolvePausedFailure.mockResolvedValue({
      id: 'wexec_1',
      nodeExecutions: [{ nodeId: 'manual-review', status: 'pending' }],
      status: 'running',
      workflowId: 'workflow_1',
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.click(screen.getByRole('button', { name: 'Load executions' }));

    await waitFor(() => {
      expect(listExecutions).toHaveBeenCalledWith('workflow_1');
    });
    fireEvent.change(screen.getByLabelText('Failure branch target for classify-ticket in wexec_1'), {
      target: { value: 'manual-review' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Branch from failed node classify-ticket for wexec_1' }));

    await waitFor(() => {
      expect(resolvePausedFailure).toHaveBeenCalledWith('workflow_1', 'wexec_1', {
        action: 'branch',
        nextNodeId: 'manual-review',
        nodeId: 'classify-ticket',
      });
    });
    expect(screen.getByLabelText('Workflow execution wexec_1 status')).toHaveTextContent('Running');
  });

  it('submits paused user input payloads through resume execution', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [
            { id: 'manual-start', type: 'manual' },
            { id: 'approval', type: 'user_input' },
            { id: 'notify', type: 'agent' },
          ],
        },
        id: 'workflow_1',
        name: 'Human approval',
        status: 'published',
        version: 1,
      },
    ]);
    listExecutions.mockResolvedValue([
      {
        id: 'wexec_1',
        nodeExecutions: [
          { nodeId: 'manual-start', status: 'succeeded' },
          {
            context: { waitReason: 'user_input_required' },
            input: { prompt: 'Approve INC-21?', required: ['approved', 'approver'] },
            nodeId: 'approval',
            nodeType: 'user_input',
            status: 'pending',
          },
        ],
        status: 'paused',
        workflowId: 'workflow_1',
      },
    ]);
    resumeExecution.mockResolvedValue({
      id: 'wexec_1',
      nodeExecutions: [
        { nodeId: 'manual-start', status: 'succeeded' },
        { nodeId: 'approval', output: { approved: true, approver: 'ops' }, status: 'succeeded' },
      ],
      status: 'running',
      workflowId: 'workflow_1',
    });

    render(<WorkflowsPage />);

    await screen.findByText('Human approval');
    fireEvent.click(screen.getByRole('button', { name: 'Load executions' }));

    await waitFor(() => {
      expect(listExecutions).toHaveBeenCalledWith('workflow_1');
    });
    const inputPanel = screen.getByLabelText('Paused input for wexec_1');
    expect(within(inputPanel).getByText('Paused on user_input_required at approval')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Resume input JSON for approval in wexec_1'), {
      target: { value: '{ "approved": true, "approver": "ops" }' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Submit input for approval in wexec_1' }));

    await waitFor(() => {
      expect(resumeExecution).toHaveBeenCalledWith('workflow_1', 'wexec_1', {
        input: { approved: true, approver: 'ops' },
        nodeId: 'approval',
      });
    });
    expect(screen.getByLabelText('Workflow execution wexec_1 status')).toHaveTextContent('Running');
  });

  it('submits paused agent tool approvals through resume execution', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: {
          nodes: [
            { id: 'call-agent', type: 'agent' },
            { id: 'done', type: 'end' },
          ],
        },
        id: 'workflow_1',
        name: 'Agent approval',
        status: 'published',
        version: 1,
      },
    ]);
    listExecutions.mockResolvedValue([
      {
        id: 'wexec_1',
        nodeExecutions: [
          {
            context: { waitReason: 'agent_approval_required' },
            input: { agentId: 'agent_1', conversationId: 'conv_1' },
            nodeId: 'call-agent',
            nodeType: 'agent',
            output: {
              runId: 'run_pending',
              status: 'pending_approval',
              toolRuns: [
                {
                  approvalStatus: 'pending',
                  id: 'tool_run_pending',
                  status: 'pending_approval',
                  toolName: 'delete_file',
                },
              ],
            },
            status: 'pending',
          },
        ],
        status: 'paused',
        workflowId: 'workflow_1',
      },
    ]);
    resumeExecution.mockResolvedValue({
      id: 'wexec_1',
      nodeExecutions: [
        { nodeId: 'call-agent', output: { runId: 'run_pending', status: 'completed' }, status: 'succeeded' },
      ],
      status: 'running',
      workflowId: 'workflow_1',
    });

    render(<WorkflowsPage />);

    await screen.findByText('Agent approval');
    fireEvent.click(screen.getByRole('button', { name: 'Load executions' }));

    const inputPanel = await screen.findByLabelText('Paused input for wexec_1');
    expect(within(inputPanel).getByText('Paused on agent_approval_required at call-agent')).toBeInTheDocument();
    expect(screen.getByLabelText('Resume input JSON for call-agent in wexec_1')).toHaveValue(
      '{\n  "runId": "run_pending",\n  "toolRunId": "tool_run_pending",\n  "approvalReason": "approved"\n}'
    );
    fireEvent.click(screen.getByRole('button', { name: 'Submit input for call-agent in wexec_1' }));

    await waitFor(() => {
      expect(resumeExecution).toHaveBeenCalledWith('workflow_1', 'wexec_1', {
        input: {
          approvalReason: 'approved',
          runId: 'run_pending',
          toolRunId: 'tool_run_pending',
        },
        nodeId: 'call-agent',
      });
    });
  });

  it('loads workflow version history and rolls back to a selected version', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'published',
        version: 3,
      },
    ]);
    listWorkflowVersions.mockResolvedValue([
      {
        definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        version: 1,
      },
      {
        definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'published',
        version: 3,
      },
    ]);
    rollbackWorkflow.mockResolvedValue({
      definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
      id: 'workflow_1',
      name: 'Incident triage',
      status: 'draft',
      version: 4,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    expect(screen.getByText('Version: 3')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Load versions for Incident triage' }));

    await waitFor(() => {
      expect(listWorkflowVersions).toHaveBeenCalledWith('workflow_1');
    });
    expect(screen.getByLabelText('Workflow version 1 status')).toHaveTextContent('draft');
    expect(screen.getByLabelText('Workflow version 3 status')).toHaveTextContent('published');

    fireEvent.click(screen.getByRole('button', { name: 'Rollback Incident triage to version 1' }));

    await waitFor(() => {
      expect(rollbackWorkflow).toHaveBeenCalledWith('workflow_1', { version: 1 });
    });
    expect(screen.getByText('Version: 4')).toBeInTheDocument();
    expect(screen.getByText('Status: draft')).toBeInTheDocument();
  });

  it('creates a branch from a loaded workflow version and refreshes version history', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'published',
        version: 2,
      },
    ]);
    listWorkflowVersions
      .mockResolvedValueOnce([
        {
          definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
          id: 'workflow_1',
          name: 'Incident triage',
          status: 'draft',
          version: 1,
        },
        {
          definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
          id: 'workflow_1',
          name: 'Incident triage',
          status: 'published',
          version: 2,
        },
      ])
      .mockResolvedValueOnce([
        {
          definition: { nodes: [{ id: 'manual-start', type: 'manual' }] },
          id: 'workflow_1',
          name: 'Incident triage',
          status: 'published',
          version: 2,
        },
      ]);
    createWorkflowBranch.mockResolvedValue({
      definition: {
        branch: {
          experimentKey: 'routing-copy-v2',
          sourceVersion: 2,
          sourceWorkflowId: 'workflow_1',
          trafficPercent: 25,
        },
        nodes: [{ id: 'manual-start', type: 'manual' }],
      },
      id: 'workflow_branch',
      name: 'Incident triage branch',
      status: 'draft',
      version: 1,
    });

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.click(screen.getByRole('button', { name: 'Load versions for Incident triage' }));

    await waitFor(() => {
      expect(listWorkflowVersions).toHaveBeenCalledWith('workflow_1');
    });
    fireEvent.click(screen.getByRole('button', { name: 'Create branch from Incident triage version 2' }));
    fireEvent.change(screen.getByLabelText('Branch name for Incident triage version 2'), {
      target: { value: 'Incident triage branch' },
    });
    fireEvent.change(screen.getByLabelText('Branch description for Incident triage version 2'), {
      target: { value: 'Experiment branch' },
    });
    fireEvent.change(screen.getByLabelText('Experiment key for Incident triage version 2'), {
      target: { value: 'routing-copy-v2' },
    });
    fireEvent.change(screen.getByLabelText('Traffic percent for Incident triage version 2'), {
      target: { value: '25' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Submit branch for Incident triage version 2' }));

    await waitFor(() => {
      expect(createWorkflowBranch).toHaveBeenCalledWith('workflow_1', {
        description: 'Experiment branch',
        experimentKey: 'routing-copy-v2',
        name: 'Incident triage branch',
        trafficPercent: 25,
        version: 2,
      });
    });
    expect(listWorkflowVersions).toHaveBeenCalledTimes(2);
    expect(screen.getByText('Incident triage branch')).toBeInTheDocument();
  });

  it('shows version definition node summaries before rollback', async () => {
    listWorkflows.mockResolvedValue([
      {
        definition: { nodes: [{ id: 'broken-start', type: 'manual' }] },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'published',
        version: 2,
      },
    ]);
    listWorkflowVersions.mockResolvedValue([
      {
        definition: { nodes: [{ id: 'stable-start', type: 'manual' }] },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'draft',
        version: 1,
      },
      {
        definition: { nodes: [{ id: 'broken-start', type: 'manual' }] },
        id: 'workflow_1',
        name: 'Incident triage',
        status: 'published',
        version: 2,
      },
    ]);

    render(<WorkflowsPage />);

    await screen.findByText('Incident triage');
    fireEvent.click(screen.getByRole('button', { name: 'Load versions for Incident triage' }));

    await waitFor(() => {
      expect(listWorkflowVersions).toHaveBeenCalledWith('workflow_1');
    });
    expect(screen.getByText('stable-start')).toBeInTheDocument();
    expect(screen.getAllByText('broken-start').length).toBeGreaterThanOrEqual(1);
  });
});
