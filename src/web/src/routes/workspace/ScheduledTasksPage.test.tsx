import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const createScheduledTask = vi.fn();
const listScheduledTasks = vi.fn();
const listRuns = vi.fn();
const runScheduledTaskNow = vi.fn();
const updateScheduledTaskEnabled = vi.fn();

vi.mock('../../features/scheduledTasks/scheduledTasksApi', () => ({
  createScheduledTasksApi: () => ({
    createScheduledTask,
    listScheduledTasks,
    listRuns,
    runScheduledTaskNow,
    updateScheduledTaskEnabled
  })
}));

import { ScheduledTasksPage } from './ScheduledTasksPage';

describe('ScheduledTasksPage', () => {
  beforeEach(() => {
    createScheduledTask.mockReset();
    listScheduledTasks.mockReset();
    listRuns.mockReset();
    runScheduledTaskNow.mockReset();
    updateScheduledTaskEnabled.mockReset();
  });

  it('loads scheduled tasks with next and last run timestamps', async () => {
    listScheduledTasks.mockResolvedValue([
      {
        cronExpression: '0 9 * * 1',
        enabled: true,
        id: 'schedule_1',
        lastRunAt: '2026-06-04T09:00:00Z',
        name: 'Weekly workflow',
        nextRunAt: '2026-06-05T09:00:00Z',
        targetId: 'workflow_1',
        targetType: 'workflow'
      }
    ]);

    render(<ScheduledTasksPage />);

    expect(screen.getByText('Loading scheduled tasks...')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Scheduled Tasks' })).toBeInTheDocument();
    const taskList = screen.getByLabelText('Scheduled task list');
    expect(within(taskList).getByText('Weekly workflow')).toBeInTheDocument();
    expect(within(taskList).getByText('workflow_1')).toBeInTheDocument();
    expect(within(taskList).getByText('0 9 * * 1')).toBeInTheDocument();
    expect(within(taskList).getByText('Enabled')).toBeInTheDocument();
    expect(within(taskList).getByText('Next: 2026-06-05 09:00')).toBeInTheDocument();
    expect(within(taskList).getByText('Last: 2026-06-04 09:00')).toBeInTheDocument();
  });

  it('creates a scheduled task and prepends it to the list', async () => {
    listScheduledTasks.mockResolvedValue([]);
    createScheduledTask.mockResolvedValue({
      cronExpression: '*/15 * * * *',
      enabled: false,
      id: 'schedule_2',
      name: 'Agent pulse',
      targetId: 'agent_1',
      targetType: 'agent'
    });

    render(<ScheduledTasksPage />);

    await screen.findByText('No scheduled tasks yet.');
    fireEvent.change(screen.getByLabelText('Schedule name'), { target: { value: ' Agent pulse ' } });
    fireEvent.change(screen.getByLabelText('Target type'), { target: { value: 'agent' } });
    fireEvent.change(screen.getByLabelText('Target ID'), { target: { value: ' agent_1 ' } });
    fireEvent.change(screen.getByLabelText('Cron expression'), { target: { value: ' */15 * * * * ' } });
    fireEvent.click(screen.getByLabelText('Enabled'));
    fireEvent.click(screen.getByRole('button', { name: 'Create schedule' }));

    await waitFor(() => {
      expect(createScheduledTask).toHaveBeenCalledWith({
        cronExpression: '*/15 * * * *',
        enabled: false,
        name: 'Agent pulse',
        targetId: 'agent_1',
        targetType: 'agent'
      });
    });
    expect(screen.getByText('Agent pulse')).toBeInTheDocument();
    expect(screen.getByText('agent_1')).toBeInTheDocument();
    expect(screen.getByText('*/15 * * * *')).toBeInTheDocument();
    expect(screen.getByText('Disabled')).toBeInTheDocument();
    expect(screen.getByLabelText('Schedule name')).toHaveValue('');
    expect(screen.getByLabelText('Target ID')).toHaveValue('');
  });

  it('loads and renders completed and failed runs for the selected task', async () => {
    listScheduledTasks.mockResolvedValue([
      {
        cronExpression: '0 9 * * 1',
        enabled: true,
        id: 'schedule_1',
        targetId: 'workflow_1',
        targetType: 'workflow'
      },
      {
        cronExpression: '*/30 * * * *',
        enabled: true,
        id: 'schedule_2',
        targetId: 'agent_1',
        targetType: 'agent'
      }
    ]);
    listRuns.mockResolvedValue([
      {
        createdAt: '2026-06-04T01:00:00Z',
        finishedAt: '2026-06-04T01:02:00Z',
        id: 'run_completed',
        scheduledTaskId: 'schedule_2',
        startedAt: '2026-06-04T01:00:00Z',
        status: 'completed',
        updatedAt: '2026-06-04T01:02:00Z'
      },
      {
        createdAt: '2026-06-04T02:00:00Z',
        error: 'worker timed out',
        finishedAt: '2026-06-04T02:01:00Z',
        id: 'run_failed',
        scheduledTaskId: 'schedule_2',
        startedAt: '2026-06-04T02:00:00Z',
        status: 'failed',
        updatedAt: '2026-06-04T02:01:00Z'
      }
    ]);

    render(<ScheduledTasksPage />);

    await screen.findByText('agent_1');
    const taskCards = screen.getAllByRole('listitem');
    fireEvent.click(within(taskCards[1]).getByRole('button', { name: 'Show recent runs for agent_1' }));

    await waitFor(() => {
      expect(listRuns).toHaveBeenCalledWith('schedule_2');
    });
    expect(within(taskCards[1]).getByText('completed')).toBeInTheDocument();
    expect(within(taskCards[1]).getByText('failed')).toBeInTheDocument();
    expect(within(taskCards[1]).getByText('Started: 2026-06-04 01:00')).toBeInTheDocument();
    expect(within(taskCards[1]).getByText('Finished: 2026-06-04 02:01')).toBeInTheDocument();
    expect(within(taskCards[1]).getByText('Error: worker timed out')).toBeInTheDocument();
  });

  it('shows clear empty feedback when a task has no recent runs', async () => {
    listScheduledTasks.mockResolvedValue([
      {
        cronExpression: '0 9 * * 1',
        enabled: true,
        id: 'schedule_1',
        targetId: 'workflow_1',
        targetType: 'workflow'
      }
    ]);
    listRuns.mockResolvedValue([]);

    render(<ScheduledTasksPage />);

    await screen.findByText('workflow_1');
    fireEvent.click(screen.getByRole('button', { name: 'Show recent runs for workflow_1' }));

    expect(await screen.findByText('No runs recorded for this scheduled task.')).toBeInTheDocument();
  });

  it('enables and disables a scheduled task from the task row', async () => {
    listScheduledTasks.mockResolvedValue([
      {
        cronExpression: '0 9 * * 1',
        enabled: true,
        id: 'schedule_1',
        nextRunAt: '2026-06-05T09:00:00Z',
        targetId: 'workflow_1',
        targetType: 'workflow'
      }
    ]);
    updateScheduledTaskEnabled.mockResolvedValueOnce({
      cronExpression: '0 9 * * 1',
      enabled: false,
      id: 'schedule_1',
      nextRunAt: null,
      targetId: 'workflow_1',
      targetType: 'workflow'
    });

    render(<ScheduledTasksPage />);

    await screen.findByText('workflow_1');
    fireEvent.click(screen.getByRole('button', { name: 'Disable workflow_1 schedule' }));

    await waitFor(() => {
      expect(updateScheduledTaskEnabled).toHaveBeenCalledWith('schedule_1', false);
    });
    expect(screen.getByText('Disabled')).toBeInTheDocument();
    expect(screen.getByText('Next: Not scheduled')).toBeInTheDocument();
  });

  it('runs a scheduled task immediately and opens recent runs with the new run', async () => {
    listScheduledTasks.mockResolvedValue([
      {
        cronExpression: '0 9 * * 1',
        enabled: true,
        id: 'schedule_1',
        targetId: 'workflow_1',
        targetType: 'workflow'
      }
    ]);
    runScheduledTaskNow.mockResolvedValue({
      createdAt: '2026-06-05T09:05:00Z',
      id: 'schedrun_manual',
      scheduledTaskId: 'schedule_1',
      startedAt: '2026-06-05T09:05:00Z',
      status: 'running',
      updatedAt: '2026-06-05T09:05:00Z'
    });

    render(<ScheduledTasksPage />);

    await screen.findByText('workflow_1');
    fireEvent.click(screen.getByRole('button', { name: 'Run workflow_1 schedule now' }));

    await waitFor(() => {
      expect(runScheduledTaskNow).toHaveBeenCalledWith('schedule_1');
    });
    expect(screen.getByText('running')).toBeInTheDocument();
    expect(screen.getByText('schedrun_manual')).toBeInTheDocument();
    expect(screen.getByText('Started: 2026-06-05 09:05')).toBeInTheDocument();
  });
});
