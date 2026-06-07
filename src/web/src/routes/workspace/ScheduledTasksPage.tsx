import { useEffect, useMemo, useState } from 'react';

import {
  createScheduledTasksApi,
  type ScheduledTask,
  type ScheduledTaskTargetType
} from '../../features/scheduledTasks/scheduledTasksApi';
import { createHttpClient } from '../../services/http/client';
import type { ScheduledTaskRun } from '../../types/api';

type RunPanelState = {
  error: string | null;
  isLoading: boolean;
  isOpen: boolean;
  runs: ScheduledTaskRun[] | null;
};

function errorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim() !== '') {
    return error.message;
  }
  if (typeof error === 'string' && error.trim() !== '') {
    return error;
  }

  return fallback;
}

function targetTypeLabel(targetType: ScheduledTaskTargetType) {
  return targetType === 'workflow' ? 'Workflow' : 'Agent';
}

function formatRunTimestamp(value: string | null | undefined, emptyText: string) {
  if (!value) {
    return emptyText;
  }

  const [datePart, timePart] = value.split('T');
  if (datePart && timePart) {
    return `${datePart} ${timePart.slice(0, 5)}`;
  }

  return value;
}

function runStatusClass(status: string) {
  const normalizedStatus = status.toLowerCase();
  if (normalizedStatus === 'completed' || normalizedStatus === 'success' || normalizedStatus === 'succeeded') {
    return 'bg-emerald-50 text-emerald-800';
  }
  if (normalizedStatus === 'failed' || normalizedStatus === 'error') {
    return 'bg-red-50 text-red-800';
  }
  if (normalizedStatus === 'running' || normalizedStatus === 'started') {
    return 'bg-sky-50 text-sky-800';
  }

  return 'bg-[#eee8dc] text-[#625b4f]';
}

export function ScheduledTasksPage() {
  const scheduledTasksApi = useMemo(() => createScheduledTasksApi(createHttpClient()), []);
  const [cronExpression, setCronExpression] = useState('');
  const [enabled, setEnabled] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isCreating, setIsCreating] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [scheduleName, setScheduleName] = useState('');
  const [runPanels, setRunPanels] = useState<Record<string, RunPanelState>>({});
  const [taskActions, setTaskActions] = useState<Record<string, 'status' | 'run' | null>>({});
  const [targetId, setTargetId] = useState('');
  const [targetType, setTargetType] = useState<ScheduledTaskTargetType>('workflow');
  const [tasks, setTasks] = useState<ScheduledTask[]>([]);

  useEffect(() => {
    let cancelled = false;

    const loadScheduledTasks = async () => {
      setIsLoading(true);
      setError(null);

      try {
        const nextTasks = await scheduledTasksApi.listScheduledTasks();
        if (!cancelled) {
          setTasks(nextTasks);
        }
      } catch (caughtError) {
        if (!cancelled) {
          setError(errorMessage(caughtError, 'Unable to load scheduled tasks. Retry the request or check the backend session.'));
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    };

    void loadScheduledTasks();

    return () => {
      cancelled = true;
    };
  }, [scheduledTasksApi]);

  const createSchedule = async () => {
    const trimmedName = scheduleName.trim();
    const trimmedTargetId = targetId.trim();
    const trimmedCronExpression = cronExpression.trim();
    if (trimmedName === '' || trimmedTargetId === '' || trimmedCronExpression === '') {
      return;
    }

    setIsCreating(true);
    setError(null);

    try {
      const createdTask = await scheduledTasksApi.createScheduledTask({
        cronExpression: trimmedCronExpression,
        enabled,
        name: trimmedName,
        targetId: trimmedTargetId,
        targetType
      });
      setTasks((current) => [createdTask, ...current.filter((task) => task.id !== createdTask.id)]);
      setCronExpression('');
      setEnabled(true);
      setScheduleName('');
      setTargetId('');
      setTargetType('workflow');
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to create scheduled task. Retry the request or check the backend session.'));
    } finally {
      setIsCreating(false);
    }
  };

  const loadTaskRuns = async (taskId: string) => {
    setRunPanels((current) => {
      const currentPanel = current[taskId] ?? { error: null, isLoading: false, isOpen: false, runs: null };
      return {
        ...current,
        [taskId]: {
          ...currentPanel,
          error: null,
          isLoading: true,
          isOpen: true
        }
      };
    });

    try {
      const nextRuns = await scheduledTasksApi.listRuns(taskId);
      setRunPanels((current) => ({
        ...current,
        [taskId]: {
          error: null,
          isLoading: false,
          isOpen: true,
          runs: nextRuns
        }
      }));
    } catch (caughtError) {
      setRunPanels((current) => {
        const currentPanel = current[taskId] ?? { error: null, isLoading: false, isOpen: true, runs: null };
        return {
          ...current,
          [taskId]: {
            ...currentPanel,
            error: errorMessage(caughtError, 'Unable to load scheduled task runs. Retry the request or check the backend session.'),
            isLoading: false,
            isOpen: true
          }
        };
      });
    }
  };

  const hideTaskRuns = (taskId: string) => {
    setRunPanels((current) => {
      const currentPanel = current[taskId];
      if (!currentPanel) {
        return current;
      }

      return {
        ...current,
        [taskId]: {
          ...currentPanel,
          isOpen: false
        }
      };
    });
  };

  const setTaskAction = (taskId: string, action: 'status' | 'run' | null) => {
    setTaskActions((current) => ({
      ...current,
      [taskId]: action
    }));
  };

  const updateTaskEnabled = async (task: ScheduledTask) => {
    setTaskAction(task.id, 'status');
    setError(null);

    try {
      const updatedTask = await scheduledTasksApi.updateScheduledTaskEnabled(task.id, !task.enabled);
      setTasks((current) => current.map((currentTask) => (currentTask.id === updatedTask.id ? updatedTask : currentTask)));
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to update scheduled task status. Retry the request or check the backend session.'));
    } finally {
      setTaskAction(task.id, null);
    }
  };

  const runTaskNow = async (task: ScheduledTask) => {
    setTaskAction(task.id, 'run');
    setError(null);

    try {
      const run = await scheduledTasksApi.runScheduledTaskNow(task.id);
      setRunPanels((current) => {
        const currentPanel = current[task.id] ?? { error: null, isLoading: false, isOpen: false, runs: null };
        return {
          ...current,
          [task.id]: {
            error: null,
            isLoading: false,
            isOpen: true,
            runs: [run, ...(currentPanel.runs ?? []).filter((currentRun) => currentRun.id !== run.id)]
          }
        };
      });
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to run scheduled task. Retry the request or check the backend session.'));
    } finally {
      setTaskAction(task.id, null);
    }
  };

  return (
    <section className="mx-auto max-w-6xl space-y-6">
      <header className="space-y-2">
        <p className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">Automation schedule</p>
        <h1 className="font-heading text-3xl font-semibold text-[#181611]">Scheduled Tasks</h1>
        <p className="max-w-3xl text-sm leading-6 text-[#625b4f]">
          Create workflow or agent schedules backed by the authenticated scheduled task API.
        </p>
      </header>

      {error ? (
        <p className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800" role="alert">
          {error}
        </p>
      ) : null}

      <section className="rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] p-5" aria-label="Create scheduled task">
        <h2 className="text-base font-semibold">Create schedule</h2>
        <form
          className="mt-4 grid gap-4 md:grid-cols-[minmax(0,1fr)_180px_minmax(0,1fr)_minmax(0,1fr)_auto]"
          onSubmit={(event) => {
            event.preventDefault();
            void createSchedule();
          }}
        >
          <label className="block text-sm font-medium">
            Schedule name
            <input
              className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
              onChange={(event) => setScheduleName(event.target.value)}
              placeholder="Daily digest"
              type="text"
              value={scheduleName}
            />
          </label>
          <label className="block text-sm font-medium">
            Target type
            <select
              className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
              onChange={(event) => setTargetType(event.target.value as ScheduledTaskTargetType)}
              value={targetType}
            >
              <option value="workflow">workflow</option>
              <option value="agent">agent</option>
            </select>
          </label>
          <label className="block text-sm font-medium">
            Target ID
            <input
              className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
              onChange={(event) => setTargetId(event.target.value)}
              placeholder="workflow_..."
              type="text"
              value={targetId}
            />
          </label>
          <label className="block text-sm font-medium">
            Cron expression
            <input
              className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-sm"
              onChange={(event) => setCronExpression(event.target.value)}
              placeholder="0 9 * * 1"
              type="text"
              value={cronExpression}
            />
          </label>
          <div className="flex flex-col justify-end gap-3">
            <label className="flex min-h-10 items-center gap-2 text-sm font-medium">
              <input
                checked={enabled}
                className="size-4 rounded border-[#d7d2c4]"
                onChange={() => setEnabled((current) => !current)}
                type="checkbox"
              />
              Enabled
            </label>
            <button
              className="min-h-11 rounded-lg bg-[#181611] px-4 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
              disabled={isCreating || scheduleName.trim() === '' || targetId.trim() === '' || cronExpression.trim() === ''}
              type="submit"
            >
              {isCreating ? 'Creating...' : 'Create schedule'}
            </button>
          </div>
        </form>
      </section>

      <section className="rounded-lg border border-[#d7d2c4] bg-white p-5" aria-label="Scheduled task list">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="text-base font-semibold">Task list</h2>
          <p className="text-sm text-[#625b4f]">{tasks.length === 1 ? '1 scheduled task' : `${tasks.length} scheduled tasks`}</p>
        </div>

        {isLoading ? <p className="mt-5 text-sm text-[#625b4f]">Loading scheduled tasks...</p> : null}
        {!isLoading && tasks.length === 0 ? <p className="mt-5 text-sm text-[#625b4f]">No scheduled tasks yet.</p> : null}

        {tasks.length > 0 ? (
          <ul className="mt-5 space-y-3">
            {tasks.map((task) => {
              const runPanel = runPanels[task.id];
              const runsRegionId = `scheduled-task-runs-${task.id}`;
              const currentAction = taskActions[task.id];

              return (
                <li className="rounded-lg border border-[#e4dfd2] bg-[#fbfaf7] p-4" key={task.id}>
                  <article className="grid gap-3 md:grid-cols-[minmax(0,1fr)_140px_minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)_120px_auto] md:items-center">
                    <div>
                      <p className="text-xs font-semibold uppercase text-[#6d6658]">Name</p>
                      <p className="mt-1 text-sm font-semibold text-[#181611]">{task.name}</p>
                    </div>
                    <div>
                      <p className="text-xs font-semibold uppercase text-[#6d6658]">Target</p>
                      <p className="mt-1 text-sm font-semibold text-[#181611]">{targetTypeLabel(task.targetType)}</p>
                    </div>
                    <div>
                      <p className="text-xs font-semibold uppercase text-[#6d6658]">Target ID</p>
                      <p className="mt-1 break-all font-mono text-sm text-[#181611]">{task.targetId}</p>
                    </div>
                    <div>
                      <p className="text-xs font-semibold uppercase text-[#6d6658]">Cron</p>
                      <p className="mt-1 font-mono text-sm text-[#181611]">{task.cronExpression}</p>
                    </div>
                    <div>
                      <p className="text-xs font-semibold uppercase text-[#6d6658]">Run window</p>
                      <p className="mt-1 text-sm text-[#181611]">Next: {formatRunTimestamp(task.nextRunAt, 'Not scheduled')}</p>
                      <p className="mt-1 text-sm text-[#625b4f]">Last: {formatRunTimestamp(task.lastRunAt, 'Never')}</p>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                      <p
                        className={`inline-flex w-fit rounded-lg px-3 py-1 text-sm font-semibold ${
                          task.enabled ? 'bg-emerald-50 text-emerald-800' : 'bg-[#eee8dc] text-[#625b4f]'
                        }`}
                      >
                        {task.enabled ? 'Enabled' : 'Disabled'}
                      </p>
                      <button
                        aria-label={`${task.enabled ? 'Disable' : 'Enable'} ${task.targetId} schedule`}
                        className="min-h-10 rounded-lg border border-[#d7d2c4] bg-white px-3 text-sm font-semibold text-[#181611] disabled:cursor-not-allowed disabled:opacity-60"
                        disabled={Boolean(currentAction)}
                        onClick={() => {
                          void updateTaskEnabled(task);
                        }}
                        type="button"
                      >
                        {currentAction === 'status' ? 'Saving...' : task.enabled ? 'Disable' : 'Enable'}
                      </button>
                      <button
                        aria-label={`Run ${task.targetId} schedule now`}
                        className="min-h-10 rounded-lg bg-[#181611] px-3 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-60"
                        disabled={Boolean(currentAction)}
                        onClick={() => {
                          void runTaskNow(task);
                        }}
                        type="button"
                      >
                        {currentAction === 'run' ? 'Running...' : 'Run now'}
                      </button>
                      <button
                        aria-controls={runsRegionId}
                        aria-expanded={Boolean(runPanel?.isOpen)}
                        aria-label={`Show recent runs for ${task.targetId}`}
                        className="min-h-10 rounded-lg border border-[#d7d2c4] bg-white px-3 text-sm font-semibold text-[#181611] disabled:cursor-not-allowed disabled:opacity-60"
                        disabled={runPanel?.isLoading}
                        onClick={() => {
                          void loadTaskRuns(task.id);
                        }}
                        type="button"
                      >
                        {runPanel?.isLoading ? 'Loading...' : runPanel?.isOpen ? 'Runs open' : 'Runs'}
                      </button>
                    </div>
                  </article>

                  {runPanel?.isOpen ? (
                    <section
                      aria-label={`Recent runs for ${task.targetId}`}
                      className="mt-4 border-t border-[#e4dfd2] pt-4"
                      id={runsRegionId}
                    >
                      <div className="flex flex-wrap items-center justify-between gap-3">
                        <div>
                          <h3 className="text-sm font-semibold text-[#181611]">Recent runs</h3>
                          <p className="mt-1 text-xs text-[#625b4f]">Status, timing, and error details for this schedule.</p>
                        </div>
                        <div className="flex items-center gap-2">
                          <button
                            aria-label={`Refresh recent runs for ${task.targetId}`}
                            className="min-h-9 rounded-lg border border-[#d7d2c4] bg-white px-3 text-xs font-semibold text-[#181611] disabled:cursor-not-allowed disabled:opacity-60"
                            disabled={runPanel.isLoading}
                            onClick={() => {
                              void loadTaskRuns(task.id);
                            }}
                            type="button"
                          >
                            Refresh
                          </button>
                          <button
                            aria-label={`Hide recent runs for ${task.targetId}`}
                            className="min-h-9 rounded-lg border border-transparent px-3 text-xs font-semibold text-[#625b4f]"
                            onClick={() => hideTaskRuns(task.id)}
                            type="button"
                          >
                            Hide
                          </button>
                        </div>
                      </div>

                      {runPanel.isLoading ? <p className="mt-3 text-sm text-[#625b4f]">Loading recent runs...</p> : null}
                      {runPanel.error ? (
                        <p className="mt-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-800" role="alert">
                          {runPanel.error}
                        </p>
                      ) : null}
                      {!runPanel.isLoading && !runPanel.error && runPanel.runs?.length === 0 ? (
                        <p className="mt-3 text-sm text-[#625b4f]">No runs recorded for this scheduled task.</p>
                      ) : null}
                      {!runPanel.isLoading && !runPanel.error && runPanel.runs && runPanel.runs.length > 0 ? (
                        <ol className="mt-3 divide-y divide-[#e4dfd2]">
                          {runPanel.runs.map((run) => (
                            <li className="grid gap-2 py-3 md:grid-cols-[140px_minmax(0,1fr)_minmax(0,1fr)] md:items-start" key={run.id}>
                              <div className="flex flex-wrap items-center gap-2">
                                <span className={`inline-flex rounded-lg px-2.5 py-1 text-xs font-semibold ${runStatusClass(run.status)}`}>
                                  {run.status}
                                </span>
                                <span className="font-mono text-xs text-[#625b4f]">{run.id}</span>
                              </div>
                              <div className="space-y-1 text-sm text-[#181611]">
                                <p>Started: {formatRunTimestamp(run.startedAt, 'Not started')}</p>
                                <p>Finished: {formatRunTimestamp(run.finishedAt, 'Not finished')}</p>
                              </div>
                              <p className="break-words text-sm text-[#625b4f]">Error: {run.error?.trim() ? run.error : 'None'}</p>
                            </li>
                          ))}
                        </ol>
                      ) : null}
                    </section>
                  ) : null}
                </li>
              );
            })}
          </ul>
        ) : null}
      </section>
    </section>
  );
}
