import { useCallback, useEffect, useMemo, useState } from 'react';
import { useLocation, useParams } from 'react-router-dom';

import {
  createAgentPlanStepsApi,
  type AgentPlanStep,
  type AgentToolRun,
  type MoveAgentPlanStepDirection
} from '../../features/agents/planStepsApi';
import { createHttpClient } from '../../services/http/client';

function errorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim() !== '') {
    return error.message;
  }
  if (typeof error === 'string' && error.trim() !== '') {
    return error;
  }
  return fallback;
}

function statePlanSteps(state: unknown): AgentPlanStep[] {
  if (typeof state !== 'object' || state === null || !('planSteps' in state)) {
    return [];
  }

  const planSteps = (state as { planSteps?: unknown }).planSteps;
  return Array.isArray(planSteps) ? planSteps as AgentPlanStep[] : [];
}

function readableJSON(value: unknown) {
  if (value === undefined || value === null) {
    return '';
  }

  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function canApprove(step: AgentPlanStep) {
  return step.status === 'pending';
}

function canExecute(step: AgentPlanStep) {
  return step.status === 'approved' || (step.status === 'pending' && step.approvalStatus === 'not_required');
}

function canEdit(step: AgentPlanStep) {
  return step.status === 'pending' || step.status === 'approved';
}

function canMovePlanStep(planSteps: AgentPlanStep[], index: number, direction: MoveAgentPlanStepDirection) {
  const step = planSteps[index];
  const target = planSteps[direction === 'up' ? index - 1 : index + 1];
  return Boolean(step && target && canEdit(step) && canEdit(target));
}

function canApproveToolRun(toolRun: AgentToolRun) {
  return toolRun.status === 'pending_approval' && toolRun.approvalStatus === 'pending';
}

function canRejectToolRun(toolRun: AgentToolRun) {
  return toolRun.status === 'pending_approval' && toolRun.approvalStatus === 'pending';
}

function canRetryToolRun(toolRun: AgentToolRun) {
  return toolRun.status === 'failed';
}

export function AgentPlanStepsPage() {
  const { runId = '' } = useParams();
  const location = useLocation();
  const api = useMemo(() => createAgentPlanStepsApi(createHttpClient()), []);
  const [error, setError] = useState<string | null>(null);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [editingStepId, setEditingStepId] = useState<string | null>(null);
  const [editInput, setEditInput] = useState('');
  const [editTitle, setEditTitle] = useState('');
  const [editToolName, setEditToolName] = useState('');
  const [operatingStepId, setOperatingStepId] = useState<string | null>(null);
  const [operatingToolRunId, setOperatingToolRunId] = useState<string | null>(null);
  const [planSteps, setPlanSteps] = useState<AgentPlanStep[]>(() => statePlanSteps(location.state));
  const [runError, setRunError] = useState<string | null>(null);
  const [runIterationCount, setRunIterationCount] = useState<number | null>(null);
  const [runMode, setRunMode] = useState<string | null>(null);
  const [runStatus, setRunStatus] = useState<string | null>(null);
  const [runToolCallCount, setRunToolCallCount] = useState<number | null>(null);
  const [toolRuns, setToolRuns] = useState<AgentToolRun[]>([]);

  const refreshRunDetail = useCallback(async () => {
    if (!runId) {
      setError('Run ID is required.');
      return;
    }

    setIsRefreshing(true);
    setError(null);

    try {
      const detail = await api.getRunDetail(runId);
      setPlanSteps(detail.planSteps);
      setRunError(detail.error || null);
      setRunIterationCount(typeof detail.iterationCount === 'number' ? detail.iterationCount : null);
      setRunMode(detail.mode || null);
      setRunStatus(detail.status || null);
      setRunToolCallCount(typeof detail.toolCallCount === 'number' ? detail.toolCallCount : null);
      setToolRuns(detail.toolRuns ?? []);
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to load plan steps.'));
    } finally {
      setIsRefreshing(false);
    }
  }, [api, runId]);

  const startEditingStep = (step: AgentPlanStep) => {
    setEditingStepId(step.id);
    setEditTitle(step.title);
    setEditToolName(step.toolName ?? '');
    setEditInput(readableJSON(step.input));
    setError(null);
  };

  const cancelEditingStep = () => {
    setEditingStepId(null);
    setEditTitle('');
    setEditToolName('');
    setEditInput('');
  };

  const savePlanStep = async (step: AgentPlanStep) => {
    if (!runId) {
      setError('Run ID is required.');
      return;
    }

    const title = editTitle.trim();
    if (!title) {
      setError('Plan step title is required.');
      return;
    }

    let parsedInput: Record<string, unknown> | undefined;
    if (editInput.trim() !== '') {
      try {
        const parsed = JSON.parse(editInput) as unknown;
        if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
          setError('Plan step input must be a JSON object.');
          return;
        }
        parsedInput = parsed as Record<string, unknown>;
      } catch {
        setError('Plan step input must be valid JSON.');
        return;
      }
    } else {
      parsedInput = {};
    }

    setOperatingStepId(step.id);
    setError(null);

    try {
      const refreshed = await api.updatePlanStep(runId, step.id, {
        input: parsedInput,
        title,
        toolName: editToolName.trim()
      });
      setPlanSteps(refreshed);
      cancelEditingStep();
      void refreshRunDetail();
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to update plan step.'));
    } finally {
      setOperatingStepId(null);
    }
  };

  useEffect(() => {
    void refreshRunDetail();
  }, [refreshRunDetail]);

  const updatePlanStep = async (step: AgentPlanStep, action: 'approve' | 'execute') => {
    if (!runId) {
      setError('Run ID is required.');
      return;
    }

    setOperatingStepId(step.id);
    setError(null);

    try {
      const refreshed =
        action === 'approve'
          ? await api.approvePlanStep(runId, step.id)
          : await api.executePlanStep(runId, step.id);
      setPlanSteps(refreshed);
      void refreshRunDetail();
    } catch (caughtError) {
      setError(errorMessage(caughtError, `Unable to ${action} plan step.`));
    } finally {
      setOperatingStepId(null);
    }
  };

  const movePlanStep = async (step: AgentPlanStep, direction: MoveAgentPlanStepDirection) => {
    if (!runId) {
      setError('Run ID is required.');
      return;
    }

    setOperatingStepId(step.id);
    setError(null);

    try {
      const refreshed = await api.movePlanStep(runId, step.id, direction);
      setPlanSteps(refreshed);
      void refreshRunDetail();
    } catch (caughtError) {
      setError(errorMessage(caughtError, `Unable to move plan step ${direction}.`));
    } finally {
      setOperatingStepId(null);
    }
  };

  const updateToolRun = async (toolRun: AgentToolRun, action: 'approve' | 'reject' | 'retry') => {
    if (!runId) {
      setError('Run ID is required.');
      return;
    }

    setOperatingToolRunId(toolRun.id);
    setError(null);

    try {
      const refreshed =
        action === 'approve'
          ? await api.approveToolRun(runId, toolRun.id, 'Approved from Agent Plan Steps')
          : action === 'reject'
            ? await api.rejectToolRun(runId, toolRun.id, 'Rejected from Agent Plan Steps')
          : await api.retryToolRun(runId, toolRun.id);
      setToolRuns(refreshed);
    } catch (caughtError) {
      setError(errorMessage(caughtError, `Unable to ${action} tool run.`));
    } finally {
      setOperatingToolRunId(null);
    }
  };

  return (
    <section className="mx-auto max-w-5xl space-y-6">
      <header className="space-y-2">
        <p className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">Agent run</p>
        <h1 className="font-heading text-3xl font-semibold text-[#181611]">Agent Plan Steps</h1>
        {runId ? <p className="text-sm text-[#625b4f]">Run {runId}</p> : null}
        {runStatus ? <p className="text-sm font-medium text-[#3f3a31]">Status: {runStatus}</p> : null}
        <dl
          aria-label="Agent run execution controls"
          className="grid gap-2 text-sm text-[#3f3a31] sm:grid-cols-3"
        >
          {runMode ? (
            <div className="rounded-lg border border-[#d7d2c4] bg-white px-3 py-2">
              <dt className="text-xs font-semibold uppercase text-[#6d6658]">Mode </dt>
              <dd className="font-medium">{runMode}</dd>
            </div>
          ) : null}
          {runIterationCount !== null ? (
            <div className="rounded-lg border border-[#d7d2c4] bg-white px-3 py-2">
              <dt className="text-xs font-semibold uppercase text-[#6d6658]">Iterations </dt>
              <dd className="font-medium">{runIterationCount}</dd>
            </div>
          ) : null}
          {runToolCallCount !== null ? (
            <div className="rounded-lg border border-[#d7d2c4] bg-white px-3 py-2">
              <dt className="text-xs font-semibold uppercase text-[#6d6658]">Tool calls </dt>
              <dd className="font-medium">{runToolCallCount}</dd>
            </div>
          ) : null}
          {runError ? (
            <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 sm:col-span-3">
              <dt className="text-xs font-semibold uppercase text-red-700">Stop reason </dt>
              <dd className="font-medium text-red-800">{runError}</dd>
            </div>
          ) : null}
        </dl>
        <button
          className="min-h-10 rounded-lg border border-[#181611] px-3 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-50"
          disabled={isRefreshing}
          onClick={() => void refreshRunDetail()}
          type="button"
        >
          {isRefreshing ? 'Refreshing plan steps' : 'Refresh plan steps'}
        </button>
      </header>

      {error ? (
        <p className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800" role="alert">
          {error}
        </p>
      ) : null}

      <section aria-label="Tool approval queue" className="space-y-3">
        <div className="flex flex-col gap-1">
          <p className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">Runtime controls</p>
          <h2 className="font-heading text-2xl font-semibold text-[#181611]">Tool Approval Queue</h2>
        </div>
        {toolRuns.length === 0 ? <p className="text-sm text-[#625b4f]">No tool runs need operator action.</p> : null}
        {toolRuns.map((toolRun) => {
          const toolArguments = readableJSON(toolRun.arguments);
          return (
            <article
              aria-label={`Tool run ${toolRun.toolName}`}
              className="rounded-lg border border-[#d7d2c4] bg-white p-4"
              key={toolRun.id}
            >
              <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div className="min-w-0 space-y-2">
                  <div className="flex flex-wrap items-center gap-2 text-xs font-semibold uppercase text-[#6d6658]">
                    <span>{toolRun.status}</span>
                    {toolRun.approvalStatus ? <span>Approval: {toolRun.approvalStatus}</span> : null}
                    {toolRun.riskLevel ? <span>Risk: {toolRun.riskLevel}</span> : null}
                    {toolRun.toolType ? <span>{toolRun.toolType}</span> : null}
                  </div>
                  <h3 className="text-base font-semibold text-[#181611]">{toolRun.toolName}</h3>
                  {toolArguments ? (
                    <pre className="max-h-48 overflow-auto rounded-lg bg-[#f6f1e6] p-3 text-xs leading-5 text-[#3f3a31]">
                      {toolArguments}
                    </pre>
                  ) : null}
                  {toolRun.resultContent ? <p className="text-sm leading-6 text-[#2f5f3a]">{toolRun.resultContent}</p> : null}
                  {toolRun.error ? <p className="text-sm leading-6 text-red-700">{toolRun.error}</p> : null}
                </div>

                <div className="flex shrink-0 flex-wrap gap-2">
                  <button
                    aria-label={`Approve tool ${toolRun.toolName}`}
                    className="min-h-10 rounded-lg border border-[#181611] px-3 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-50"
                    disabled={!canApproveToolRun(toolRun) || operatingToolRunId === toolRun.id}
                    onClick={() => void updateToolRun(toolRun, 'approve')}
                    type="button"
                  >
                    Approve tool
                    <span className="sr-only"> {toolRun.toolName}</span>
                  </button>
                  <button
                    aria-label={`Reject tool ${toolRun.toolName}`}
                    className="min-h-10 rounded-lg border border-red-700 px-3 text-sm font-semibold text-red-700 disabled:cursor-not-allowed disabled:opacity-50"
                    disabled={!canRejectToolRun(toolRun) || operatingToolRunId === toolRun.id}
                    onClick={() => void updateToolRun(toolRun, 'reject')}
                    type="button"
                  >
                    Reject tool
                    <span className="sr-only"> {toolRun.toolName}</span>
                  </button>
                  <button
                    aria-label={`Retry tool ${toolRun.toolName}`}
                    className="min-h-10 rounded-lg bg-[#181611] px-3 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
                    disabled={!canRetryToolRun(toolRun) || operatingToolRunId === toolRun.id}
                    onClick={() => void updateToolRun(toolRun, 'retry')}
                    type="button"
                  >
                    Retry tool
                    <span className="sr-only"> {toolRun.toolName}</span>
                  </button>
                </div>
              </div>
            </article>
          );
        })}
      </section>

      <section aria-label="Plan steps" className="space-y-3">
        {planSteps.length === 0 ? <p className="text-sm text-[#625b4f]">No plan steps to show yet.</p> : null}
        {planSteps.map((step, stepPosition) => {
          const isEditing = editingStepId === step.id;
          const stepInput = readableJSON(step.input);
          const canMoveUp = canMovePlanStep(planSteps, stepPosition, 'up');
          const canMoveDown = canMovePlanStep(planSteps, stepPosition, 'down');
          return (
            <article
              aria-label={`Plan step ${step.title}`}
              className="rounded-lg border border-[#d7d2c4] bg-white p-4"
              key={step.id}
            >
              <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2 text-xs font-semibold uppercase text-[#6d6658]">
                    {typeof step.index === 'number' ? <span>Step {step.index}</span> : null}
                    <span>{step.status}</span>
                    {step.approvalStatus ? <span>Approval: {step.approvalStatus}</span> : null}
                    {step.toolName ? <span>Tool: {step.toolName}</span> : null}
                  </div>
                  {isEditing ? (
                    <div className="mt-3 grid gap-3">
                      <label className="grid gap-1 text-sm font-medium text-[#3f3a31]">
                        Title
                        <input
                          className="min-h-10 rounded-lg border border-[#d7d2c4] px-3 text-sm"
                          onChange={(event) => setEditTitle(event.target.value)}
                          value={editTitle}
                        />
                      </label>
                      <label className="grid gap-1 text-sm font-medium text-[#3f3a31]">
                        Tool
                        <input
                          className="min-h-10 rounded-lg border border-[#d7d2c4] px-3 text-sm"
                          onChange={(event) => setEditToolName(event.target.value)}
                          placeholder="Optional built-in tool"
                          value={editToolName}
                        />
                      </label>
                      <label className="grid gap-1 text-sm font-medium text-[#3f3a31]">
                        Input
                        <textarea
                          className="min-h-28 rounded-lg border border-[#d7d2c4] px-3 py-2 font-mono text-xs leading-5"
                          onChange={(event) => setEditInput(event.target.value)}
                          value={editInput}
                        />
                      </label>
                    </div>
                  ) : (
                    <>
                      <h2 className="mt-2 text-base font-semibold text-[#181611]">{step.title}</h2>
                      {stepInput ? (
                        <pre className="mt-2 max-h-40 overflow-auto rounded-lg bg-[#f6f1e6] p-3 text-xs leading-5 text-[#3f3a31]">
                          {stepInput}
                        </pre>
                      ) : null}
                      {step.resultContent ? <p className="mt-2 text-sm leading-6 text-[#2f5f3a]">{step.resultContent}</p> : null}
                      {step.error ? <p className="mt-2 text-sm leading-6 text-red-700">{step.error}</p> : null}
                    </>
                  )}
                </div>

                <div className="flex shrink-0 flex-wrap gap-2">
                  {isEditing ? (
                    <>
                      <button
                        aria-label={`Save ${step.title}`}
                        className="min-h-10 rounded-lg bg-[#181611] px-3 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
                        disabled={operatingStepId === step.id}
                        onClick={() => void savePlanStep(step)}
                        type="button"
                      >
                        Save
                        <span className="sr-only"> {step.title}</span>
                      </button>
                      <button
                        aria-label={`Cancel editing ${step.title}`}
                        className="min-h-10 rounded-lg border border-[#181611] px-3 text-sm font-semibold"
                        onClick={cancelEditingStep}
                        type="button"
                      >
                        Cancel
                        <span className="sr-only"> {step.title}</span>
                      </button>
                    </>
                  ) : (
                    <>
                      <button
                        aria-label={`Edit ${step.title}`}
                        className="min-h-10 rounded-lg border border-[#181611] px-3 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-50"
                        disabled={!canEdit(step) || operatingStepId === step.id}
                        onClick={() => startEditingStep(step)}
                        type="button"
                      >
                        Edit
                        <span className="sr-only"> {step.title}</span>
                      </button>
                      <button
                        aria-label={`Move up ${step.title}`}
                        className="min-h-10 rounded-lg border border-[#181611] px-3 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-50"
                        disabled={!canMoveUp || operatingStepId === step.id}
                        onClick={() => void movePlanStep(step, 'up')}
                        type="button"
                      >
                        Move up
                        <span className="sr-only"> {step.title}</span>
                      </button>
                      <button
                        aria-label={`Move down ${step.title}`}
                        className="min-h-10 rounded-lg border border-[#181611] px-3 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-50"
                        disabled={!canMoveDown || operatingStepId === step.id}
                        onClick={() => void movePlanStep(step, 'down')}
                        type="button"
                      >
                        Move down
                        <span className="sr-only"> {step.title}</span>
                      </button>
                      <button
                        aria-label={`Approve ${step.title}`}
                        className="min-h-10 rounded-lg border border-[#181611] px-3 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-50"
                        disabled={!canApprove(step) || operatingStepId === step.id}
                        onClick={() => void updatePlanStep(step, 'approve')}
                        type="button"
                      >
                        Approve
                        <span className="sr-only"> {step.title}</span>
                      </button>
                      <button
                        aria-label={`Execute ${step.title}`}
                        className="min-h-10 rounded-lg bg-[#181611] px-3 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
                        disabled={!canExecute(step) || operatingStepId === step.id}
                        onClick={() => void updatePlanStep(step, 'execute')}
                        type="button"
                      >
                        Execute
                        <span className="sr-only"> {step.title}</span>
                      </button>
                    </>
                  )}
                </div>
              </div>
            </article>
          );
        })}
      </section>
    </section>
  );
}
