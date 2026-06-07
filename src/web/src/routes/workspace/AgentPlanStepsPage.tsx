import { useCallback, useEffect, useMemo, useState } from 'react';
import { useLocation, useParams } from 'react-router-dom';

import { createAgentPlanStepsApi, type AgentPlanStep, type AgentToolRun } from '../../features/agents/planStepsApi';
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
  const [operatingStepId, setOperatingStepId] = useState<string | null>(null);
  const [operatingToolRunId, setOperatingToolRunId] = useState<string | null>(null);
  const [planSteps, setPlanSteps] = useState<AgentPlanStep[]>(() => statePlanSteps(location.state));
  const [runStatus, setRunStatus] = useState<string | null>(null);
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
      setRunStatus(detail.status || null);
      setToolRuns(detail.toolRuns ?? []);
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to load plan steps.'));
    } finally {
      setIsRefreshing(false);
    }
  }, [api, runId]);

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
                    className="min-h-10 rounded-lg border border-[#181611] px-3 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-50"
                    disabled={!canApproveToolRun(toolRun) || operatingToolRunId === toolRun.id}
                    onClick={() => void updateToolRun(toolRun, 'approve')}
                    type="button"
                  >
                    Approve tool
                    <span className="sr-only"> {toolRun.toolName}</span>
                  </button>
                  <button
                    className="min-h-10 rounded-lg border border-red-700 px-3 text-sm font-semibold text-red-700 disabled:cursor-not-allowed disabled:opacity-50"
                    disabled={!canRejectToolRun(toolRun) || operatingToolRunId === toolRun.id}
                    onClick={() => void updateToolRun(toolRun, 'reject')}
                    type="button"
                  >
                    Reject tool
                    <span className="sr-only"> {toolRun.toolName}</span>
                  </button>
                  <button
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
        {planSteps.map((step) => (
          <article
            aria-label={`Plan step ${step.title}`}
            className="rounded-lg border border-[#d7d2c4] bg-white p-4"
            key={step.id}
          >
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2 text-xs font-semibold uppercase text-[#6d6658]">
                  {typeof step.index === 'number' ? <span>Step {step.index}</span> : null}
                  <span>{step.status}</span>
                  {step.approvalStatus ? <span>Approval: {step.approvalStatus}</span> : null}
                </div>
                <h2 className="mt-2 text-base font-semibold text-[#181611]">{step.title}</h2>
                {step.resultContent ? <p className="mt-2 text-sm leading-6 text-[#2f5f3a]">{step.resultContent}</p> : null}
                {step.error ? <p className="mt-2 text-sm leading-6 text-red-700">{step.error}</p> : null}
              </div>

              <div className="flex shrink-0 flex-wrap gap-2">
                <button
                  className="min-h-10 rounded-lg border border-[#181611] px-3 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-50"
                  disabled={!canApprove(step) || operatingStepId === step.id}
                  onClick={() => void updatePlanStep(step, 'approve')}
                  type="button"
                >
                  Approve
                  <span className="sr-only"> {step.title}</span>
                </button>
                <button
                  className="min-h-10 rounded-lg bg-[#181611] px-3 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
                  disabled={!canExecute(step) || operatingStepId === step.id}
                  onClick={() => void updatePlanStep(step, 'execute')}
                  type="button"
                >
                  Execute
                  <span className="sr-only"> {step.title}</span>
                </button>
              </div>
            </div>
          </article>
        ))}
      </section>
    </section>
  );
}
