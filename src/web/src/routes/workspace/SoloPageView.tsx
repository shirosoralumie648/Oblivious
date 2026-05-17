import type { KnowledgeBaseSummary, TaskDetail, TaskSummary } from '../../types/api';

type SoloPageViewProps = {
  activeBudgetLimit: string;
  authorizationScope: string;
  budgetLimit: string;
  completedTasks: TaskSummary[];
  defaultAuthorizationScope: string;
  error: string | null;
  executionMode: string;
  goal: string;
  isLoading: boolean;
  isLoadingTaskID: string | null;
  isStarting: boolean;
  isTaskCreationView: boolean;
  knowledgeBases: KnowledgeBaseSummary[];
  recentTasks: TaskSummary[];
  returnTo: string | null;
  runningTasks: TaskSummary[];
  selectedKnowledgeBaseIDs: string[];
  startedTask: TaskDetail | null;
  startedTaskToolAllowList: string[];
  startedTaskToolDenyList: string[];
  stoppedTasks: TaskSummary[];
  taskEvents: NonNullable<TaskDetail['events']>;
  taskKnowledgeBaseNames: string[];
  taskResultArtifacts: NonNullable<TaskDetail['resultArtifacts']>;
  toolAllowListInput: string;
  toolDenyListInput: string;
  userDefaultMode: string;
  userModelStrategy: string;
  userNetworkEnabledHint: boolean;
  onApproveTask: () => void;
  onCancelTask: () => void;
  onChangeActiveBudgetLimit: (value: string) => void;
  onChangeAuthorizationScope: (value: string) => void;
  onChangeBudgetLimit: (value: string) => void;
  onChangeExecutionMode: (value: string) => void;
  onChangeGoal: (value: string) => void;
  onChangeToolAllowListInput: (value: string) => void;
  onChangeToolDenyListInput: (value: string) => void;
  onContinueInChat: () => void;
  onContinueTask: () => void;
  onExportResult: () => void;
  onNavigateBackToChat: () => void;
  onNavigateNewTask: () => void;
  onNavigateTasks: () => void;
  onOpenTask: (taskID: string) => void;
  onPauseTask: () => void;
  onResumeTask: () => void;
  onRetryTask: () => void;
  onStartSoloRun: () => void;
  onToggleKnowledgeBase: (knowledgeBaseID: string) => void;
  onUpdateBudget: () => void;
};

function renderTaskGroup(
  title: string,
  tasks: TaskSummary[],
  isLoadingTaskID: string | null,
  onOpenTask: (taskID: string) => void
) {
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
            <button disabled={isLoadingTaskID === task.id} onClick={() => onOpenTask(task.id)} type="button">
              {`Open task ${task.title}`}
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}

export function SoloPageView({
  activeBudgetLimit,
  authorizationScope,
  budgetLimit,
  completedTasks,
  defaultAuthorizationScope,
  error,
  executionMode,
  goal,
  isLoading,
  isLoadingTaskID,
  isStarting,
  isTaskCreationView,
  knowledgeBases,
  recentTasks,
  returnTo,
  runningTasks,
  selectedKnowledgeBaseIDs,
  startedTask,
  startedTaskToolAllowList,
  startedTaskToolDenyList,
  stoppedTasks,
  taskEvents,
  taskKnowledgeBaseNames,
  taskResultArtifacts,
  toolAllowListInput,
  toolDenyListInput,
  userDefaultMode,
  userModelStrategy,
  userNetworkEnabledHint,
  onApproveTask,
  onCancelTask,
  onChangeActiveBudgetLimit,
  onChangeAuthorizationScope,
  onChangeBudgetLimit,
  onChangeExecutionMode,
  onChangeGoal,
  onChangeToolAllowListInput,
  onChangeToolDenyListInput,
  onContinueInChat,
  onContinueTask,
  onExportResult,
  onNavigateBackToChat,
  onNavigateNewTask,
  onNavigateTasks,
  onOpenTask,
  onPauseTask,
  onResumeTask,
  onRetryTask,
  onStartSoloRun,
  onToggleKnowledgeBase,
  onUpdateBudget
}: SoloPageViewProps) {
  return (
    <section>
      <h1>{isTaskCreationView ? 'New SOLO task' : 'SOLO'}</h1>
      <p>
        {isTaskCreationView
          ? 'Define the task boundary before handing execution over to SOLO.'
          : 'Launch a focused autonomous run with a clear goal, bounded execution mode, and selected workspace knowledge.'}
      </p>
      {isLoading ? <p>Loading solo workspace…</p> : null}
      {error ? <p>{error}</p> : null}
      <p>Default mode: {userDefaultMode}</p>
      <p>Model strategy: {userModelStrategy}</p>
      <p>Web suggestions: {userNetworkEnabledHint ? 'Enabled' : 'Disabled'}</p>
      {isTaskCreationView ? (
        <button onClick={onNavigateTasks} type="button">
          Back to tasks
        </button>
      ) : (
        <button onClick={onNavigateNewTask} type="button">
          New task
        </button>
      )}
      {returnTo ? (
        <button onClick={onNavigateBackToChat} type="button">
          Back to chat
        </button>
      ) : null}

      <label>
        Task goal
        <textarea onChange={(event) => onChangeGoal(event.target.value)} rows={4} value={goal} />
      </label>

      <label>
        Execution mode
        <select onChange={(event) => onChangeExecutionMode(event.target.value)} value={executionMode}>
          <option value="safe">safe</option>
          <option value="standard">standard</option>
          <option value="auto">auto</option>
        </select>
      </label>

      <label>
        Authorization scope
        <select onChange={(event) => onChangeAuthorizationScope(event.target.value)} value={authorizationScope}>
          <option value="knowledge_only">knowledge_only</option>
          <option value="workspace_tools">workspace_tools</option>
          <option value="full_access">full_access</option>
        </select>
      </label>

      <label>
        Budget limit
        <input onChange={(event) => onChangeBudgetLimit(event.target.value)} type="number" value={budgetLimit} />
      </label>

      <label>
        Allowed tools
        <input
          onChange={(event) => onChangeToolAllowListInput(event.target.value)}
          placeholder="browser, shell"
          type="text"
          value={toolAllowListInput}
        />
      </label>

      <label>
        Blocked tools
        <input
          onChange={(event) => onChangeToolDenyListInput(event.target.value)}
          placeholder="email, file_delete"
          type="text"
          value={toolDenyListInput}
        />
      </label>

      <section>
        <h2>Knowledge sources</h2>
        {knowledgeBases.length === 0 ? <p>No knowledge bases linked yet.</p> : null}
        {knowledgeBases.map((knowledgeBase) => (
          <label key={knowledgeBase.id}>
            <input
              checked={selectedKnowledgeBaseIDs.includes(knowledgeBase.id)}
              onChange={() => onToggleKnowledgeBase(knowledgeBase.id)}
              type="checkbox"
            />
            {`Use knowledge base ${knowledgeBase.name}`}
          </label>
        ))}
      </section>

      <button disabled={isStarting} onClick={onStartSoloRun} type="button">
        Start solo run
      </button>

      {!isTaskCreationView ? renderTaskGroup('Running tasks', runningTasks, isLoadingTaskID, onOpenTask) : null}
      {!isTaskCreationView ? renderTaskGroup('Completed tasks', completedTasks, isLoadingTaskID, onOpenTask) : null}
      {!isTaskCreationView ? renderTaskGroup('Stopped tasks', stoppedTasks, isLoadingTaskID, onOpenTask) : null}

      {startedTask ? (
        <section>
          <h2>{startedTask.status === 'completed' ? 'Latest result' : 'Execution view'}</h2>
          <p>{`Status: ${startedTask.status}`}</p>
          <p>{`Execution mode: ${startedTask.executionMode}`}</p>
          <p>{`Authorization scope: ${startedTask.authorizationScope ?? defaultAuthorizationScope}`}</p>
          <p>{`Budget consumed: ${startedTask.budgetConsumed ?? 0} / ${startedTask.budgetLimit}`}</p>
          {startedTask.status !== 'completed' && startedTask.status !== 'cancelled' ? (
            <div>
              <label>
                Active budget limit
                <input onChange={(event) => onChangeActiveBudgetLimit(event.target.value)} type="number" value={activeBudgetLimit} />
              </label>
              <button disabled={isLoadingTaskID === startedTask.id} onClick={onUpdateBudget} type="button">
                Update budget
              </button>
            </div>
          ) : null}
          {startedTask.startedAt ? <p>{`Started at: ${startedTask.startedAt}`}</p> : null}
          {startedTask.finishedAt ? <p>{`Finished at: ${startedTask.finishedAt}`}</p> : null}
          {startedTask.currentStep ? <p>{`Current step: ${startedTask.currentStep}`}</p> : null}
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
          <section>
            <h3>Current enabled tools</h3>
            {startedTaskToolAllowList.length === 0 ? (
              <p>
                {startedTask.authorizationScope === 'knowledge_only'
                  ? 'No tools enabled for this task.'
                  : 'Using the default tool access for this authorization scope.'}
              </p>
            ) : (
              <ul>
                {startedTaskToolAllowList.map((toolName) => (
                  <li key={toolName}>{toolName}</li>
                ))}
              </ul>
            )}
          </section>
          <section>
            <h3>Blocked tools</h3>
            {startedTaskToolDenyList.length === 0 ? (
              <p>No blocked tools configured for this task.</p>
            ) : (
              <ul>
                {startedTaskToolDenyList.map((toolName) => (
                  <li key={toolName}>{toolName}</li>
                ))}
              </ul>
            )}
          </section>
          {taskEvents.length > 0 ? (
            <section>
              <h3>Execution timeline</h3>
              <ul>
                {taskEvents.map((event) => (
                  <li key={`${event.type}-${event.createdAt ?? event.message}`}>
                    <strong>{event.type}</strong>
                    <span>{` ${event.message}`}</span>
                  </li>
                ))}
              </ul>
            </section>
          ) : null}
          {startedTask.resultSummary ? (
            <p>{startedTask.resultSummary}</p>
          ) : startedTask.status === 'awaiting_confirmation' ? (
            <p>SOLO is waiting for your approval before continuing beyond the current execution boundary.</p>
          ) : (
            <p>SOLO is still working through the current plan.</p>
          )}
          {taskResultArtifacts.length > 0 ? (
            <section>
              <h3>Result artifacts</h3>
              <ul>
                {taskResultArtifacts.map((artifact) => (
                  <li key={`${artifact.label}-${artifact.value}`}>
                    <strong>{artifact.label}</strong>
                    <span>{` ${artifact.value}`}</span>
                  </li>
                ))}
              </ul>
            </section>
          ) : null}
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
              <button disabled={isLoadingTaskID === startedTask.id} onClick={onContinueTask} type="button">
                Continue run
              </button>
              <button onClick={onPauseTask} type="button">
                Pause run
              </button>
              <button onClick={onCancelTask} type="button">
                Cancel run
              </button>
            </div>
          ) : null}
          {startedTask.status === 'paused' ? (
            <div>
              <button onClick={onResumeTask} type="button">
                Resume run
              </button>
              <button onClick={onCancelTask} type="button">
                Cancel run
              </button>
            </div>
          ) : null}
          {startedTask.status === 'awaiting_confirmation' ? (
            <div>
              <button disabled={isLoadingTaskID === startedTask.id} onClick={onApproveTask} type="button">
                Approve plan
              </button>
              <button onClick={onCancelTask} type="button">
                Cancel run
              </button>
            </div>
          ) : null}
          {startedTask.status === 'completed' || startedTask.status === 'cancelled' ? (
            <div>
              <button disabled={isLoadingTaskID === startedTask.id} onClick={onRetryTask} type="button">
                Retry run
              </button>
              <button disabled={isLoadingTaskID === startedTask.id} onClick={onContinueInChat} type="button">
                Continue in Chat
              </button>
              <button onClick={onExportResult} type="button">
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
