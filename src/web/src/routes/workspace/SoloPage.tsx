import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';

import { useAppContext } from '../../app/providers';
import { createChatApi } from '../../features/chat/api';
import { createKnowledgeApi } from '../../features/knowledge/api';
import { createTasksApi } from '../../features/tasks/api';
import { createHttpClient } from '../../services/http/client';
import type { KnowledgeBaseSummary, TaskDetail, TaskSummary } from '../../types/api';

const defaultBudgetLimit = '10';
const defaultAuthorizationScope = 'workspace_tools';
const defaultExecutionMode = 'standard';

function taskIDFromSearch(search: string) {
  const taskID = new URLSearchParams(search).get('taskId');
  if (taskID === null) {
    return '';
  }

  return taskID.trim();
}

function downloadTaskResult(task: TaskDetail, knowledgeBaseNames: string[]) {
  const fileName = `${task.title || task.id}`.trim().replace(/\s+/g, '-').toLowerCase() || task.id;
  const content = [
    `# ${task.title}`,
    '',
    `- Goal: ${task.goal}`,
    `- Status: ${task.status}`,
    `- Execution mode: ${task.executionMode}`,
    `- Authorization scope: ${task.authorizationScope}`,
    `- Budget consumed: ${task.budgetConsumed ?? 0} / ${task.budgetLimit}`,
    `- Started at: ${task.startedAt ?? 'N/A'}`,
    `- Finished at: ${task.finishedAt ?? 'N/A'}`,
    `- Knowledge sources: ${knowledgeBaseNames.length > 0 ? knowledgeBaseNames.join(', ') : 'None'}`,
    '',
    '## Result',
    task.resultSummary || 'No result summary available.',
    '',
    '## Steps',
    ...task.steps.map((step) => `- [${step.status}] ${step.title}`)
  ].join('\n');

  const blob = new Blob([content], { type: 'text/markdown;charset=utf-8' });
  const downloadURL = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = downloadURL;
  link.download = `${fileName || 'solo-result'}.md`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(downloadURL);
}

export function SoloPage() {
  const { authState } = useAppContext();
  const navigate = useNavigate();
  const httpClient = useMemo(() => createHttpClient(), []);
  const chatApi = useMemo(() => createChatApi(httpClient), [httpClient]);
  const knowledgeApi = useMemo(() => createKnowledgeApi(httpClient), [httpClient]);
  const tasksApi = useMemo(() => createTasksApi(httpClient), [httpClient]);
  const [budgetLimit, setBudgetLimit] = useState(defaultBudgetLimit);
  const [authorizationScope, setAuthorizationScope] = useState(defaultAuthorizationScope);
  const [error, setError] = useState<string | null>(null);
  const [executionMode, setExecutionMode] = useState(defaultExecutionMode);
  const [goal, setGoal] = useState('');
  const [isLoading, setIsLoading] = useState(true);
  const [isLoadingTaskID, setIsLoadingTaskID] = useState<string | null>(null);
  const [isStarting, setIsStarting] = useState(false);
  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBaseSummary[]>([]);
  const [recentTasks, setRecentTasks] = useState<TaskSummary[]>([]);
  const [selectedKnowledgeBaseIDs, setSelectedKnowledgeBaseIDs] = useState<string[]>([]);
  const [startedTask, setStartedTask] = useState<TaskDetail | null>(null);

  function applyTaskDetail(detail: TaskDetail) {
    setStartedTask(detail);
    setRecentTasks((current) => [detail, ...current.filter((task) => task.id !== detail.id)]);
  }

  useEffect(() => {
    let cancelled = false;

    const loadSoloContext = async () => {
      setIsLoading(true);
      setError(null);

      try {
        const [tasks, bases] = await Promise.all([tasksApi.listTasks(), knowledgeApi.listKnowledgeBases()]);
        if (!cancelled) {
          setRecentTasks(tasks);
          setKnowledgeBases(bases);
        }
      } catch {
        if (!cancelled) {
          setRecentTasks([]);
          setKnowledgeBases([]);
          setError('Unable to load solo workspace.');
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    };

    void loadSoloContext();

    return () => {
      cancelled = true;
    };
  }, [knowledgeApi, tasksApi]);

  useEffect(() => {
    const taskID = taskIDFromSearch(window.location.search);
    if (taskID === '') {
      return;
    }

    let cancelled = false;

    const loadTaskFromQuery = async () => {
      setIsLoadingTaskID(taskID);
      setError(null);

      try {
        const detail = await tasksApi.getTask(taskID);
        if (!cancelled) {
          applyTaskDetail(detail);
        }
      } catch {
        if (!cancelled) {
          setError('Unable to load task detail.');
        }
      } finally {
        if (!cancelled) {
          setIsLoadingTaskID(null);
        }
      }
    };

    void loadTaskFromQuery();

    return () => {
      cancelled = true;
    };
  }, [tasksApi]);

  const toggleKnowledgeBase = (knowledgeBaseID: string) => {
    setSelectedKnowledgeBaseIDs((current) =>
      current.includes(knowledgeBaseID)
        ? current.filter((currentID) => currentID !== knowledgeBaseID)
        : [...current, knowledgeBaseID]
    );
  };

  const taskKnowledgeBaseNames =
    startedTask?.knowledgeBaseIds.map((knowledgeBaseID) => {
      const matchedKnowledgeBase = knowledgeBases.find((knowledgeBase) => knowledgeBase.id === knowledgeBaseID);
      return matchedKnowledgeBase?.name ?? knowledgeBaseID;
    }) ?? [];
  const runningTasks = recentTasks.filter(
    (task) => task.status === 'running' || task.status === 'paused' || task.status === 'awaiting_confirmation'
  );
  const completedTasks = recentTasks.filter((task) => task.status === 'completed');
  const stoppedTasks = recentTasks.filter(
    (task) => task.status !== 'running' && task.status !== 'paused' && task.status !== 'awaiting_confirmation' && task.status !== 'completed'
  );

  const handleStartSoloRun = async () => {
    const trimmedGoal = goal.trim();
    if (trimmedGoal === '') {
      setError('Task goal is required.');
      return;
    }

    setIsStarting(true);
    setError(null);

    try {
      const parsedBudgetLimit = Number.parseInt(budgetLimit, 10);
      const createdTask = await tasksApi.createTask({
        authorizationScope,
        budgetLimit: Number.isNaN(parsedBudgetLimit) ? 0 : parsedBudgetLimit,
        executionMode,
        goal: trimmedGoal,
        knowledgeBaseIds: selectedKnowledgeBaseIDs
      });
      const detail = await tasksApi.startTask(createdTask.id);
      applyTaskDetail(detail);
    } catch {
      setError('Unable to start a solo run.');
    } finally {
      setIsStarting(false);
    }
  };

  const handleOpenTask = async (taskID: string) => {
    setIsLoadingTaskID(taskID);
    setError(null);

    try {
      const detail = await tasksApi.getTask(taskID);
      applyTaskDetail(detail);
    } catch {
      setError('Unable to load task detail.');
    } finally {
      setIsLoadingTaskID(null);
    }
  };

  const handlePauseTask = async () => {
    if (!startedTask) {
      return;
    }

    setError(null);
    try {
      const detail = await tasksApi.pauseTask(startedTask.id);
      applyTaskDetail(detail);
    } catch {
      setError('Unable to pause task.');
    }
  };

  const handleApproveTask = async () => {
    if (!startedTask) {
      return;
    }

    setIsLoadingTaskID(startedTask.id);
    setError(null);

    try {
      const detail = await tasksApi.approveTask(startedTask.id);
      applyTaskDetail(detail);
    } catch {
      setError('Unable to approve task plan.');
    } finally {
      setIsLoadingTaskID(null);
    }
  };

  const handleResumeTask = async () => {
    if (!startedTask) {
      return;
    }

    setError(null);
    try {
      const detail = await tasksApi.resumeTask(startedTask.id);
      applyTaskDetail(detail);
    } catch {
      setError('Unable to resume task.');
    }
  };

  const handleCancelTask = async () => {
    if (!startedTask) {
      return;
    }

    setError(null);
    try {
      const detail = await tasksApi.cancelTask(startedTask.id);
      applyTaskDetail(detail);
    } catch {
      setError('Unable to cancel task.');
    }
  };

  const handleRetryTask = async () => {
    if (!startedTask) {
      return;
    }

    setIsLoadingTaskID(startedTask.id);
    setError(null);

    try {
      const detail = await tasksApi.startTask(startedTask.id);
      applyTaskDetail(detail);
    } catch {
      setError('Unable to retry task.');
    } finally {
      setIsLoadingTaskID(null);
    }
  };

  const handleContinueInChat = async () => {
    if (!startedTask) {
      return;
    }

    setIsLoadingTaskID(startedTask.id);
    setError(null);

    try {
      const conversation = await chatApi.createConversation({ title: startedTask.title });
      const conversationConfig = await chatApi.getConversationConfig(conversation.id);
      await chatApi.updateConversationConfig(conversation.id, {
        knowledgeBaseIds: startedTask.knowledgeBaseIds,
        maxOutputTokens: conversationConfig.maxOutputTokens,
        modelId: conversationConfig.modelId,
        systemPromptOverride: conversationConfig.systemPromptOverride ?? '',
        temperature: conversationConfig.temperature,
        toolsEnabled: conversationConfig.toolsEnabled
      });
      await chatApi.sendMessage(conversation.id, {
        content: `Continue from this SOLO result.\nGoal: ${startedTask.goal}\nResult: ${
          startedTask.resultSummary || 'No result summary available.'
        }`
      });
      navigate(`/chat/${conversation.id}`);
    } catch {
      setError('Unable to continue this task in chat.');
    } finally {
      setIsLoadingTaskID(null);
    }
  };

  const handleExportResult = () => {
    if (!startedTask) {
      return;
    }

    downloadTaskResult(startedTask, taskKnowledgeBaseNames);
  };

  const renderTaskGroup = (title: string, tasks: TaskSummary[]) => {
    if (tasks.length === 0) {
      return null;
    }

    return (
      <section>
        <h2>{title}</h2>
        <ul>
          {tasks.map((task) => (
            <li key={task.id}>
              <strong>{task.title}</strong>
              <span>{task.status}</span>
              <button disabled={isLoadingTaskID === task.id} onClick={() => void handleOpenTask(task.id)} type="button">
                {`Open task ${task.title}`}
              </button>
            </li>
          ))}
        </ul>
      </section>
    );
  };

  return (
    <section>
      <h1>SOLO</h1>
      <p>Launch a focused autonomous run with a clear goal, bounded execution mode, and selected workspace knowledge.</p>
      {isLoading ? <p>Loading solo workspace…</p> : null}
      {error ? <p>{error}</p> : null}
      <p>Default mode: {authState.preferences?.defaultMode ?? 'chat'}</p>
      <p>Model strategy: {authState.preferences?.modelStrategy ?? 'balanced'}</p>
      <p>Web suggestions: {authState.preferences?.networkEnabledHint ? 'Enabled' : 'Disabled'}</p>

      <label>
        Task goal
        <textarea onChange={(event) => setGoal(event.target.value)} rows={4} value={goal} />
      </label>

      <label>
        Execution mode
        <select onChange={(event) => setExecutionMode(event.target.value)} value={executionMode}>
          <option value="safe">safe</option>
          <option value="standard">standard</option>
          <option value="auto">auto</option>
        </select>
      </label>

      <label>
        Authorization scope
        <select onChange={(event) => setAuthorizationScope(event.target.value)} value={authorizationScope}>
          <option value="knowledge_only">knowledge_only</option>
          <option value="workspace_tools">workspace_tools</option>
          <option value="full_access">full_access</option>
        </select>
      </label>

      <label>
        Budget limit
        <input onChange={(event) => setBudgetLimit(event.target.value)} type="number" value={budgetLimit} />
      </label>

      <section>
        <h2>Knowledge sources</h2>
        {knowledgeBases.length === 0 ? <p>No knowledge bases linked yet.</p> : null}
        {knowledgeBases.map((knowledgeBase) => (
          <label key={knowledgeBase.id}>
            <input
              checked={selectedKnowledgeBaseIDs.includes(knowledgeBase.id)}
              onChange={() => toggleKnowledgeBase(knowledgeBase.id)}
              type="checkbox"
            />
            {`Use knowledge base ${knowledgeBase.name}`}
          </label>
        ))}
      </section>

      <button disabled={isStarting} onClick={() => void handleStartSoloRun()} type="button">
        Start solo run
      </button>

      {renderTaskGroup('Running tasks', runningTasks)}
      {renderTaskGroup('Completed tasks', completedTasks)}
      {renderTaskGroup('Stopped tasks', stoppedTasks)}

      {startedTask ? (
        <section>
          <h2>{startedTask.status === 'completed' ? 'Latest result' : 'Execution view'}</h2>
          <p>{`Status: ${startedTask.status}`}</p>
          <p>{`Execution mode: ${startedTask.executionMode}`}</p>
          <p>{`Authorization scope: ${startedTask.authorizationScope}`}</p>
          <p>{`Budget consumed: ${startedTask.budgetConsumed ?? 0} / ${startedTask.budgetLimit}`}</p>
          {startedTask.startedAt ? <p>{`Started at: ${startedTask.startedAt}`}</p> : null}
          {startedTask.finishedAt ? <p>{`Finished at: ${startedTask.finishedAt}`}</p> : null}
          <section>
            <h3>Current knowledge sources</h3>
            {taskKnowledgeBaseNames.length === 0 ? (
              <p>No knowledge sources enabled for this task.</p>
            ) : (
              <ul>
                {taskKnowledgeBaseNames.map((knowledgeBaseName) => (
                  <li key={knowledgeBaseName}>{knowledgeBaseName}</li>
                ))}
              </ul>
            )}
          </section>
          {startedTask.resultSummary ? (
            <p>{startedTask.resultSummary}</p>
          ) : startedTask.status === 'awaiting_confirmation' ? (
            <p>SOLO is waiting for your approval before continuing beyond the current execution boundary.</p>
          ) : (
            <p>SOLO is still working through the current plan.</p>
          )}
          <ol>
            {startedTask.steps.map((step) => (
              <li key={step.id}>
                <span>{step.title}</span>
                <span>{` ${step.status}`}</span>
              </li>
            ))}
          </ol>
          {startedTask.status === 'running' ? (
            <div>
              <button onClick={() => void handlePauseTask()} type="button">
                Pause run
              </button>
              <button onClick={() => void handleCancelTask()} type="button">
                Cancel run
              </button>
            </div>
          ) : null}
          {startedTask.status === 'paused' ? (
            <div>
              <button onClick={() => void handleResumeTask()} type="button">
                Resume run
              </button>
              <button onClick={() => void handleCancelTask()} type="button">
                Cancel run
              </button>
            </div>
          ) : null}
          {startedTask.status === 'awaiting_confirmation' ? (
            <div>
              <button disabled={isLoadingTaskID === startedTask.id} onClick={() => void handleApproveTask()} type="button">
                Approve plan
              </button>
              <button onClick={() => void handleCancelTask()} type="button">
                Cancel run
              </button>
            </div>
          ) : null}
          {startedTask.status === 'completed' || startedTask.status === 'cancelled' ? (
            <div>
              <button disabled={isLoadingTaskID === startedTask.id} onClick={() => void handleRetryTask()} type="button">
                Retry run
              </button>
              <button disabled={isLoadingTaskID === startedTask.id} onClick={() => void handleContinueInChat()} type="button">
                Continue in Chat
              </button>
              <button onClick={() => handleExportResult()} type="button">
                Export result
              </button>
            </div>
          ) : null}
        </section>
      ) : !isLoading && recentTasks.length === 0 ? (
        <p>No solo tasks yet. Start a solo run to create your first task.</p>
      ) : null}
    </section>
  );
}
