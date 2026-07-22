// @ts-expect-error Vitest runs in Node, while the browser tsconfig intentionally omits Node types.
import { createHash } from 'node:crypto';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import type { ComponentType, ReactNode } from 'react';
import { RouterProvider } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { releaseCapabilityProjection } from '@/generated/release-projection.generated';

type RouterProjectionResponse = {
  releaseIdentity: {
    sourceTree: string;
    contractDigest: string;
    deploymentProfile: string;
  };
  generation: number;
  projectionDigest: string;
  capabilities: Array<{
    capabilityId: string;
    disposition: 'committed' | 'conditional';
    availability: 'enabled';
    enabled: true;
  }>;
};

function routerProjectionResponse(): RouterProjectionResponse {
  const response: RouterProjectionResponse = {
    releaseIdentity: {
      sourceTree: 'a'.repeat(40),
      contractDigest: `sha256:${'b'.repeat(64)}`,
      deploymentProfile: 'monolith'
    },
    generation: 7,
    projectionDigest: '',
    capabilities: releaseCapabilityProjection
      .filter((capability) => capability.disposition !== 'excluded')
      .map((capability) => ({
        capabilityId: capability.capabilityId,
        disposition: capability.disposition,
        availability: 'enabled',
        enabled: true
      }))
  };
  const payload = {
    identity: response.releaseIdentity,
    generation: response.generation,
    capabilities: response.capabilities
  };
  response.projectionDigest = `sha256:${createHash('sha256').update(JSON.stringify(payload)).digest('hex')}`;
  return response;
}

function jsonResponse(value: unknown, status = 200) {
  return new Response(JSON.stringify(value), {
    status,
    headers: { 'Content-Type': 'application/json' }
  });
}

const chatApiMocks = vi.hoisted(() => ({
  convertConversationToTask: vi.fn((conversationId = 'conversation_router') =>
    Promise.resolve({
      draftTaskGoal: `Create a SOLO follow-up from ${conversationId}.`,
      relatedKnowledgeBaseIds: ['kb_router'],
      suggestedBudget: 20,
      suggestedExecutionMode: 'safe'
    })
  ),
  createConversation: vi.fn(() => Promise.resolve({ id: 'conversation_router_solo', title: 'SOLO continuation' })),
  getConversationConfig: vi.fn((conversationId = 'conversation_router') =>
    Promise.resolve({
      conversationId,
      knowledgeBaseIds: ['kb_router'],
      maxOutputTokens: 800,
      modelId: 'gpt-4o-mini',
      systemPromptOverride: '',
      temperature: 0.2,
      toolsEnabled: true
    })
  ),
  listConversations: vi.fn(() => Promise.resolve([{ id: 'conversation_router', title: 'Router parameter thread' }])),
  listMessages: vi.fn(() => Promise.resolve([{ id: 'message_router', role: 'assistant', content: 'Router parameter message.' }])),
  listModels: vi.fn(() => Promise.resolve([
    { capabilityId: 'relay.provider_inference', id: 'gpt-4o-mini', label: 'gpt-4o-mini' }
  ])),
  listPersonas: vi.fn(() => Promise.resolve([])),
  sendMessage: vi.fn(() => Promise.resolve([])),
  sendMessageStream: vi.fn(async (_conversationId: string, _payload: Record<string, unknown>, handlers: { onChunk: (chunk: string) => void }) => {
    handlers.onChunk('Router streamed answer.');
  }),
  updateConversationConfig: vi.fn((conversationId: string, config: Record<string, unknown>) =>
    Promise.resolve({ conversationId, ...config })
  )
}));

const taskApiMocks = vi.hoisted(() => {
  const runningTaskDetail = () => ({
    authorizationScope: 'full_access',
    budgetConsumed: 6,
    budgetLimit: 20,
    createdAt: '2026-06-09T10:00:00Z',
    currentStep: 'Review workspace context',
    events: [
      { createdAt: '2026-06-09T10:01:00Z', message: 'Task execution started', type: 'started' },
      { createdAt: '2026-06-09T10:02:00Z', message: 'Executing Review workspace context', type: 'running' }
    ],
    executionMode: 'safe',
    goal: 'Draft launch checklist',
    id: 'task_router_new',
    knowledgeBaseIds: ['kb_router'],
    startedAt: '2026-06-09T10:01:00Z',
    status: 'running',
    steps: [
      { id: 'step_router_1', status: 'completed', stepIndex: 1, title: 'Understand the goal' },
      { id: 'step_router_2', status: 'running', stepIndex: 2, title: 'Review workspace context' },
      { id: 'step_router_3', status: 'pending', stepIndex: 3, title: 'Deliver starter result' }
    ],
    title: 'Draft launch checklist',
    toolAllowList: ['browser', 'shell'],
    toolDenyList: ['email']
  });

  return {
    approveTask: vi.fn(() => Promise.resolve(runningTaskDetail())),
    cancelTask: vi.fn(() => Promise.resolve({ ...runningTaskDetail(), status: 'cancelled' })),
    createTask: vi.fn(() =>
      Promise.resolve({
        authorizationScope: 'full_access',
        budgetLimit: 20,
        executionMode: 'safe',
        goal: 'Draft launch checklist',
        id: 'task_router_new',
        knowledgeBaseIds: ['kb_router'],
        status: 'draft',
        title: 'Draft launch checklist'
      })
    ),
    getTask: vi.fn((taskId?: string) =>
      Promise.resolve(
        taskId === 'task_router_new'
          ? runningTaskDetail()
          : {
              authorizationScope: 'workspace_tools',
              budgetConsumed: 12,
              budgetLimit: 12,
              executionMode: 'standard',
              finishedAt: '2026-06-09T10:20:00Z',
              goal: 'Review launch plan',
              id: 'task_completed',
              knowledgeBaseIds: ['kb_router'],
              resultArtifacts: [{ label: 'Report', value: 'solo-result.md' }],
              resultSummary: 'Completed a starter SOLO run for: Review launch plan',
              status: 'completed',
              steps: [{ id: 'step_done', status: 'completed', stepIndex: 1, title: 'Understand the goal' }],
              title: 'Review launch plan',
              toolAllowList: ['browser'],
              toolDenyList: ['email']
            }
      )
    ),
    listTasks: vi.fn(() =>
      Promise.resolve([
        {
          authorizationScope: 'workspace_tools',
          budgetLimit: 20,
          executionMode: 'standard',
          goal: 'Watch live rollout',
          id: 'task_running',
          status: 'running',
          title: 'Watch live rollout'
        },
        {
          authorizationScope: 'workspace_tools',
          budgetLimit: 12,
          executionMode: 'standard',
          goal: 'Review launch plan',
          id: 'task_completed',
          status: 'completed',
          title: 'Review launch plan'
        },
        {
          authorizationScope: 'workspace_tools',
          budgetLimit: 8,
          executionMode: 'safe',
          goal: 'Abort risky task',
          id: 'task_cancelled',
          status: 'cancelled',
          title: 'Abort risky task'
        }
      ])
    ),
    pauseTask: vi.fn(() => Promise.resolve({ ...runningTaskDetail(), status: 'paused' })),
    resumeTask: vi.fn(() => Promise.resolve(runningTaskDetail())),
    startTask: vi.fn(() => Promise.resolve(runningTaskDetail())),
    updateTaskBudget: vi.fn(() => Promise.resolve({ ...runningTaskDetail(), budgetLimit: 30 }))
  };
});

const knowledgeApiMocks = vi.hoisted(() => {
  const retrievalResult = {
    chunkId: 'chunk_router_1',
    chunkIndex: 0,
    documentId: 'doc_router_overview',
    documentTitle: 'Router Retrieval Runbook',
    documentVersion: 'v2',
    retrievalMethod: 'hybrid_rerank',
    similarity: 0.94,
    snippet: 'Deployment rollback needs cited context.',
    source: {
      chunkId: 'chunk_router_1',
      chunkIndex: 0,
      documentId: 'doc_router_overview',
      documentTitle: 'Router Retrieval Runbook',
      documentVersion: 'v2',
      highlightPositions: [{ end: 19, start: 11 }],
      matchedSnippet: 'rollback',
      originalText: 'Deployment rollback needs cited context.',
      pageNumber: 7,
      sourceUrl: 'https://docs.example/router.pdf'
    }
  };

  return {
    getKnowledgeBase: vi.fn((knowledgeBaseId = 'kb_router') =>
      Promise.resolve({
        chunkOverlap: 75,
        chunkSize: 800,
        chunkStrategy: 'semantic',
        documentCount: 2,
        embeddingModel: 'text-embedding-3-large',
        id: knowledgeBaseId,
        name: 'Research Vault',
        rerankTopK: 8,
        rerankerModel: 'bge-reranker-large',
        retrievalMode: 'hybrid_rerank',
        updatedAt: '2026-06-09T00:00:00Z'
      })
    ),
    listKnowledgeBases: vi.fn(() =>
      Promise.resolve([
        {
          documentCount: 4,
          id: 'kb_router',
          name: 'Research Vault'
        }
      ])
    ),
    listKnowledgeDocumentChunks: vi.fn(() =>
      Promise.resolve([
        {
          charCount: 41,
          chunkId: 'chunk_router_1',
          chunkIndex: 0,
          content: 'Deployment rollback needs cited context.',
          documentVersion: 'v2',
          estimatedTokenCount: 8,
          metadata: {
            documentVersion: 'v2',
            endRune: 41,
            pageNumber: 7,
            sourceUrl: 'https://docs.example/router.pdf',
            startRune: 0
          }
        },
        {
          charCount: 38,
          chunkId: 'chunk_router_2',
          chunkIndex: 1,
          content: 'Release signoff keeps owners aligned.',
          documentVersion: 'v2',
          estimatedTokenCount: 7,
          metadata: {
            documentVersion: 'v2',
            endRune: 79,
            pageNumber: 8,
            sourceUrl: 'https://docs.example/router.pdf',
            startRune: 41
          }
        }
      ])
    ),
    listKnowledgeDocumentVersions: vi.fn(() =>
      Promise.resolve([
        {
          chunkCount: 2,
          content: 'Version 2 runbook content.',
          documentId: 'doc_router_overview',
          documentVersion: 'v2',
          title: 'Router Retrieval Runbook',
          updateStrategy: 'versioned'
        }
      ])
    ),
    listKnowledgeDocumentIngestionJobs: vi.fn(() => Promise.resolve([])),
    listKnowledgeDocuments: vi.fn(() =>
      Promise.resolve([
        {
          content: 'Deployment controls include rollback and citation checks.',
          documentVersion: 'v2',
          id: 'doc_router_overview',
          title: 'Router Retrieval Runbook',
          updateStrategy: 'versioned',
          updatedAt: '2026-06-09T00:00:00Z'
        }
      ])
    ),
    listRetrievalTestCases: vi.fn(() =>
      Promise.resolve([
        {
          expectedChunkId: 'chunk_router_1',
          expectedChunkIndex: 0,
          expectedDocumentId: 'doc_router_overview',
          expectedDocumentTitle: 'Router Retrieval Runbook',
          id: 'case_router_1',
          knowledgeBaseId: 'kb_router',
          query: 'deployment rollback'
        }
      ])
    ),
    retrieveKnowledge: vi.fn(() => Promise.resolve([retrievalResult])),
    runRetrievalTestCases: vi.fn(() =>
      Promise.resolve({
        failed: 0,
        knowledgeBaseId: 'kb_router',
        passed: 1,
        results: [
          {
            actualResult: retrievalResult,
            expectedResult: retrievalResult,
            passed: true,
            query: 'deployment rollback',
            rank: 1,
            testCaseId: 'case_router_1'
          }
        ],
        total: 1
      })
    )
  };
});

const adminApiMocks = vi.hoisted(() => ({
  approveAgent: vi.fn(() => Promise.resolve()),
  createDueMarketplacePayouts: vi.fn(() =>
    Promise.resolve({
      data: [{ id: 'payout_router_created', status: 'payout_pending', provider: 'stripe_connect' }],
      total: 1
    })
  ),
  dismissMarketplaceAbuseReport: vi.fn(() => Promise.resolve({ status: 'dismissed' })),
  enforceReviewSLA: vi.fn(() => Promise.resolve({ scanned: 3, alerted: 2 })),
  getBillingSummary: vi.fn(() =>
    Promise.resolve({
      billingSessions: { count: 1, settledAmount: 4.5 },
      paymentIntents: { count: 1, refundedAmount: 10, totalAmount: 29 },
      webhookEvents: { count: 1, failedCount: 1 },
      settlements: { count: 1, grossAmount: 50 },
      payouts: { count: 1, totalAmount: 40 }
    })
  ),
  listBillingSurface: vi.fn((surface: string) =>
    Promise.resolve({
      data:
        surface === 'sessions'
          ? [
              {
                id: 'bs_router_phase28',
                model: 'gpt-4o',
                settledAmount: 4.5,
                status: 'settled',
                createdAt: '2026-06-09T00:00:00Z'
              }
            ]
          : surface === 'payouts'
            ? [
                {
                  id: 'payout_router_pending',
                  provider: 'stripe_connect',
                  providerPayoutId: 'provider-router-existing',
                  amount: 40,
                  status: 'payout_pending',
                  createdAt: '2026-06-09T00:00:00Z'
                }
              ]
            : surface === 'topups'
              ? [
                  {
                    id: 'topup_router_refund',
                    provider: 'stripe',
                    amount: 25,
                    money: 25,
                    refundedAmount: 5,
                    status: 'paid',
                    paymentIntentId: 'pi_router_topup',
                    providerPaymentIntentId: 'pi_provider_router_topup',
                    currency: 'usd',
                    createdAt: '2026-06-09T00:00:00Z'
                  }
                ]
              : [],
      total: surface === 'sessions' || surface === 'payouts' || surface === 'topups' ? 1 : 0
    })
  ),
  listMarketplaceAbuseReports: vi.fn(() =>
    Promise.resolve({
      data: [
        {
          id: 'report_router',
          reporterOrganizationId: 'org_router',
          reporterUserId: 'user_router',
          agentId: 'agent_review_router',
          reason: 'malware',
          details: 'attempted credential exfiltration',
          status: 'open',
          createdAt: '2026-06-04T10:00:00Z',
          updatedAt: '2026-06-04T10:00:00Z'
        }
      ],
      total: 1
    })
  ),
  markMarketplacePayoutFailed: vi.fn(() => Promise.resolve({ id: 'payout_router_pending', status: 'failed', providerPayoutId: 'provider-router-failed' })),
  markMarketplacePayoutPaid: vi.fn(() => Promise.resolve({ id: 'payout_router_pending', status: 'paid_out', providerPayoutId: 'provider-router-paid' })),
  reinstateMarketplaceAgent: vi.fn(() => Promise.resolve({ status: 'approved' })),
  rejectAgent: vi.fn(() => Promise.resolve()),
  resolveMarketplaceAbuseReport: vi.fn(() => Promise.resolve({ status: 'resolved' })),
  refundTopup: vi.fn(() => Promise.resolve({ id: 'refund_router_topup', status: 'succeeded', providerRefundId: 're_router_topup' })),
  takedownMarketplaceAgent: vi.fn(() => Promise.resolve({ status: 'takedown' })),
  requestAgentChanges: vi.fn(() => Promise.resolve())
}));

const marketplaceApiMocks = vi.hoisted(() => ({
  createTemplate: vi.fn(() =>
    Promise.resolve({
      id: 'tpl_router_created',
      type: 'workflow',
      name: 'Router Template',
      templateData: { nodes: [{ id: 'start' }] },
      tags: ['router'],
      downloadsCount: 0
    })
  ),
  installAgent: vi.fn((_agentID?: string, _versionID?: string, provider?: string) =>
    Promise.resolve(
      provider === 'alipay'
        ? {
            checkoutSessionId: 'cs_marketplace_alipay_router',
            url: 'https://checkout.alipay.test/session/cs_marketplace_alipay_router'
          }
        : { checkoutSessionId: 'cs_marketplace_1', url: 'https://checkout.example/session' }
    )
  ),
  deleteAgent: vi.fn(() => Promise.resolve())
}));

const agentPlanStepsApiMocks = vi.hoisted(() => {
  const runDetailFixture = (runId = 'run_1', overrides: Record<string, unknown> = {}) => ({
    error: 'token_budget_exceeded: used 1200 tokens exceeds budget 1000',
    id: runId,
    iterationCount: 4,
    mode: 'planning',
    planSteps: [
      {
        approvalStatus: 'approved',
        id: 'step_inspect',
        index: 1,
        input: { scope: 'workspace' },
        resultContent: 'Workspace inspected.',
        runId,
        status: 'completed',
        title: 'Inspect workspace'
      },
      {
        approvalStatus: 'pending',
        id: 'step_patch',
        index: 2,
        input: { file: 'router.test.tsx' },
        runId,
        status: 'pending',
        title: 'Patch router coverage',
        toolName: 'editor'
      },
      {
        approvalStatus: 'not_required',
        error: 'tsc failed',
        id: 'step_verify',
        index: 3,
        input: { command: 'pnpm test' },
        runId,
        status: 'failed',
        title: 'Run verification'
      }
    ],
    status: 'token_budget_exceeded',
    toolCallCount: 2,
    toolRuns: [
      {
        approvalStatus: 'pending',
        arguments: { query: 'route coverage' },
        id: 'tool_search',
        riskLevel: 'medium',
        status: 'pending_approval',
        toolName: 'web_search',
        toolType: 'builtin'
      },
      {
        approvalStatus: 'approved',
        arguments: { command: 'pnpm test' },
        error: 'exit 1',
        id: 'tool_shell',
        status: 'failed',
        toolName: 'shell',
        toolType: 'builtin'
      }
    ],
    ...overrides
  });
  const journeyPlanSteps = (runId = 'run_router_agent') => [
    {
      approvalStatus: 'approved',
      id: 'step_scope',
      index: 1,
      resultContent: 'Release scope inspected.',
      runId,
      status: 'completed',
      title: 'Inspect release scope'
    },
    {
      approvalStatus: 'pending',
      dependsOn: [1],
      description: 'Patch the router coverage after the release scope is known.',
      id: 'step_patch',
      index: 2,
      input: { file: 'src/web/src/app/router.test.tsx' },
      runId,
      status: 'pending',
      title: 'Patch router coverage',
      toolName: 'editor'
    }
  ];
  const journeyDetail = (runId = 'run_router_agent', overrides: Record<string, unknown> = {}) =>
    runDetailFixture(runId, {
      error: '',
      iterationCount: 2,
      planSteps: journeyPlanSteps(runId),
      status: 'pending_approval',
      toolCallCount: 1,
      toolRuns: [
        {
          approvalStatus: 'pending',
          arguments: { query: 'agent planning route journey' },
          id: 'tool_search',
          riskLevel: 'medium',
          status: 'pending_approval',
          toolName: 'web_search',
          toolType: 'builtin'
        }
      ],
      ...overrides
    });

  return {
    adjustPlan: vi.fn((runId = 'run_1') => Promise.resolve(runDetailFixture(runId))),
    approvePlanStep: vi.fn((runId = 'run_1') =>
      Promise.resolve(
        journeyDetail(runId, {
          planSteps: journeyPlanSteps(runId).map((step) =>
            step.id === 'step_patch' ? { ...step, approvalStatus: 'approved', status: 'approved' } : step
          ),
          toolRuns: []
        })
      )
    ),
    approveToolRun: vi.fn((runId = 'run_1') =>
      Promise.resolve(
        journeyDetail(runId, {
          toolRuns: [
            {
              approvalDecisionReason: 'Route-level operator approval.',
              approvalStatus: 'approved',
              id: 'tool_search',
              resultContent: 'Search approved.',
              status: 'completed',
              toolName: 'web_search',
              toolType: 'builtin'
            }
          ]
        })
      )
    ),
    continuePlan: vi.fn((runId = 'run_1') =>
      Promise.resolve(
        journeyDetail(runId, {
          iterationCount: 5,
          planSteps: journeyPlanSteps(runId).map((step) => ({ ...step, approvalStatus: 'approved', status: 'completed' })),
          status: 'completed',
          toolRuns: []
        })
      )
    ),
    continueRunWithBudget: vi.fn((runId = 'run_1') =>
      Promise.resolve(
        runDetailFixture(runId, {
          error: '',
          status: 'completed',
          toolRuns: []
        })
      )
    ),
    createPlanStep: vi.fn((runId = 'run_1') => Promise.resolve(runDetailFixture(runId))),
    deletePlanStep: vi.fn((runId = 'run_1') => Promise.resolve(runDetailFixture(runId))),
    executePlanStep: vi.fn((runId = 'run_1') =>
      Promise.resolve(
        journeyDetail(runId, {
          planSteps: journeyPlanSteps(runId).map((step) =>
            step.id === 'step_patch'
              ? { ...step, approvalStatus: 'approved', resultContent: 'Router coverage patched.', status: 'completed' }
              : step
          ),
          toolRuns: []
        })
      )
    ),
    getRunDetail: vi.fn((runId = 'run_1') =>
      Promise.resolve(runId === 'run_router_agent' ? journeyDetail(runId) : runDetailFixture(runId))
    ),
    movePlanStep: vi.fn((runId = 'run_1') => Promise.resolve(runDetailFixture(runId))),
    rejectToolRun: vi.fn((runId = 'run_1') => Promise.resolve(runDetailFixture(runId, { status: 'failed' }))),
    retryPlanStep: vi.fn((runId = 'run_1') => Promise.resolve(runDetailFixture(runId, { error: '', status: 'running' }))),
    retryToolRun: vi.fn((runId = 'run_1') => Promise.resolve(runDetailFixture(runId, { error: '', status: 'running' }))),
    skipPlanStep: vi.fn((runId = 'run_1') => Promise.resolve(runDetailFixture(runId))),
    updatePlanStep: vi.fn((runId = 'run_1') => Promise.resolve(runDetailFixture(runId)))
  };
});

const agentsApiMocks = vi.hoisted(() => ({
  createAgent: vi.fn((agent: unknown) => Promise.resolve(agent)),
  createRun: vi.fn(() =>
    Promise.resolve({
      id: 'run_router_agent',
      planSteps: [],
      status: 'pending_approval',
      toolRuns: []
    })
  ),
  deleteAgent: vi.fn(() => Promise.resolve()),
  getAgent: vi.fn(() =>
    Promise.resolve({
      config: { approvalMode: 'tiered', defaultExecutionMode: 'planning' },
      id: 'agent_1',
      isPublic: false,
      model: 'gpt-4o-mini',
      name: 'Research Agent',
      tools: []
    })
  ),
  getAgentTools: vi.fn(() =>
    Promise.resolve([
      {
        capabilityId: 'mcp.network_execution',
        description: 'Search the web with tenant policy controls.',
        inputSchema: { type: 'object' },
        name: 'web_search',
        requiresApproval: true,
        riskLevel: 'medium',
        toolType: 'builtin'
      }
    ])
  ),
  listAgents: vi.fn(() =>
    Promise.resolve([
      {
        config: {
          approvalMode: 'tiered',
          defaultExecutionMode: 'planning',
          longTermMemoryExtractionPolicy: 'deterministic',
          longTermMemoryUpdatePolicy: 'exact_refresh',
          longTermMemoryWritePolicy: 'interaction_and_explicit',
          maxIterations: 8,
          tokenBudget: 30000
        },
        description: 'Router-level research automation.',
        id: 'agent_1',
        isPublic: false,
        model: 'gpt-4o-mini',
        name: 'Research Agent',
        tools: []
      }
    ])
  ),
  updateAgent: vi.fn((agent: unknown) => Promise.resolve(agent))
}));

const swrApiMocks = vi.hoisted(() => ({
  getAdminStats: vi.fn(() =>
    Promise.resolve({
      users: { totalUsers: 24, activeUsers: 12, newUsersToday: 2, newUsersWeek: 5 },
      quotas: { totalBalance: 100, totalUsed: 25, activeTopups: 3 },
      conversations: 8,
      agents: 11,
      tasks: 4,
      mcpServers: 2,
      channelsTotal: 4,
      channelsOnline: 3,
      activeAgents: 7,
      apiCalls24h: 128,
      dailyStats: [],
      modelBreakdown: []
    })
  )
}));

vi.mock('@xyflow/react', () => {
  const passthrough = ({ children }: { children?: ReactNode }) => <>{children}</>;

  return {
    Background: () => <div aria-hidden="true" data-xyflow-background="true" />,
    Controls: () => <div aria-hidden="true" data-xyflow-controls="true" />,
    MarkerType: { ArrowClosed: 'arrowclosed' },
    MiniMap: () => <div aria-hidden="true" data-xyflow-minimap="true" />,
    ReactFlowProvider: passthrough,
    ReactFlow: ({
      children,
      nodeTypes,
      nodes,
      snapToGrid,
    }: {
      children?: ReactNode;
      nodeTypes?: Record<string, ComponentType<any>>;
      nodes?: Array<any>;
      snapToGrid?: boolean;
    }) => (
      <div aria-label="React Flow router mock" data-snap-to-grid={snapToGrid ? 'true' : 'false'}>
        {(nodes ?? []).map((node) => {
          const NodeComponent = nodeTypes?.[node.type];
          return (
            <div data-node-id={node.id} key={node.id}>
              {NodeComponent ? <NodeComponent data={node.data} selected={node.selected} /> : null}
            </div>
          );
        })}
        {children}
      </div>
    ),
  };
});

vi.mock('../features/console/api', () => ({
  createConsoleApi: () => ({
    getAccess: () =>
      Promise.resolve({
        defaultMode: 'chat',
        modelStrategy: 'balanced',
        networkEnabledHint: true,
        onboardingCompleted: true,
        sessionExpiresAt: '2026-04-03T00:00:00Z',
        sessionId: 'session_1',
        userEmail: 'user@example.com',
        userId: 'user_1',
        workspaceId: 'workspace_1'
      }),
    createApiToken: () =>
      Promise.resolve({
        rawToken: 'obv_router_secret',
        token: {
          id: 'tok_router_created',
          modelLimits: ['gpt-4o-mini'],
          modelLimitsEnabled: true,
          name: 'Router key',
          status: 'active',
          tokenPrefix: 'obv_router_created',
          usedQuota: 0,
          createdAt: '2026-06-09T00:00:00Z'
        }
      }),
    getBilling: () =>
      Promise.resolve({
        period: '30d',
        requests: 5,
        inputTokens: 120,
        outputTokens: 80,
        estimatedCostUsd: 0.0004,
        balanceUsd: 42.5,
        creditLimitUsd: 100,
        currentSpendUsd: 0.0004,
        paymentProviders: [{ name: 'stripe' }, { name: 'alipay' }],
        nextInvoice: {
          amountUsd: 0.0004,
          dueAt: '2026-04-30T00:00:00Z',
          id: 'invoice_next',
          status: 'draft'
        }
      }),
    listInvoices: () =>
      Promise.resolve([
        {
          amountUsd: 0.0004,
          dueAt: '2026-04-30T00:00:00Z',
          hostedInvoiceUrl: 'https://billing.stripe.test/invoices/invoice_paid',
          id: 'invoice_paid',
          invoicePdf: 'https://billing.stripe.test/invoices/invoice_paid.pdf',
          status: 'paid'
        }
      ]),
    listPackages: () =>
      Promise.resolve([
        {
          agentLimit: 25,
          createdAt: '2026-06-01T00:00:00Z',
          durationDays: 30,
          id: 'pkg_router_pro',
          isActive: true,
          isPublic: true,
          maxTokensPerRequest: 32000,
          modelAccess: ['gpt-4.1'],
          name: 'Router Pro',
          price: 29,
          quotaAmount: 150,
          sortOrder: 10,
          tokenQuota: 1500000,
          updatedAt: '2026-06-01T00:00:00Z'
        }
      ]),
    getModels: () =>
      Promise.resolve([{ id: 'balanced-chat', label: 'balanced-chat', requests: 2 }]),
    getUsage: () =>
      Promise.resolve({
        period: '7d',
        requests: 5,
        byModel: [{ key: 'gpt-4o', requestCount: 3, totalTokens: 300, totalCost: 0.09 }],
        byFeature: [{ key: 'workflow', requestCount: 2, totalTokens: 180, totalCost: 0.04 }],
        byUser: [{ key: 'user_router', requestCount: 2, totalTokens: 240, totalCost: 0.12 }],
        timeSeries: [{ bucket: '2026-06-09', requestCount: 3, totalTokens: 280, totalCost: 0.09 }],
        recent: [
          {
            id: 'usage_router_console',
            apiTokenId: 'tok_router_1',
            requestId: 'req_router_console',
            apiType: 'chat',
            model: 'gpt-4o',
            channelId: 'ch_router_1',
            provider: 'openai',
            status: 'success',
            statusCode: 200,
            latencyMs: 42,
            cost: 0.004,
            promptTokens: 100,
            completionTokens: 50,
            totalTokens: 150,
            createdAt: '2026-06-09T00:00:00Z'
          }
        ]
      }),
    listApiTokenUsage: () =>
      Promise.resolve([
        {
          id: 'usage_router_1',
          apiTokenId: 'tok_router_1',
          requestId: 'req_router_1',
          apiType: 'chat',
          model: 'gpt-4o',
          channelId: 'ch_router_1',
          provider: 'openai',
          status: 'success',
          statusCode: 200,
          latencyMs: 42,
          cost: 0.004,
          promptTokens: 100,
          completionTokens: 50,
          totalTokens: 150,
          createdAt: '2026-06-09T00:00:00Z'
        }
      ]),
    listApiTokens: () =>
      Promise.resolve([
        {
          id: 'tok_router_1',
          modelLimits: ['gpt-4o'],
          modelLimitsEnabled: true,
          name: 'Router gateway key',
          status: 'active',
          tokenPrefix: 'obv_router',
          usedQuota: 2.5,
          createdAt: '2026-06-09T00:00:00Z'
        }
      ]),
    revokeApiToken: () => Promise.resolve()
  })
}));

vi.mock('../features/notifications/notificationsApi', () => ({
  createNotificationsApi: () => ({
    deleteNotification: () => Promise.resolve({ status: 'deleted' }),
    getUnreadCount: () => Promise.resolve({ count: 1 }),
    listNotifications: () =>
      Promise.resolve([
        {
          category: 'system',
          createdAt: '2026-06-06T08:00:00Z',
          id: 'notif_1',
          isRead: false,
          message: 'Database connection failed',
          title: 'Database down',
          type: 'critical',
          userId: 'user_1'
        }
      ]),
    markAllRead: () => Promise.resolve({ status: 'ok' }),
    markRead: () => Promise.resolve({ id: 'notif_1', isRead: true })
  })
}));

vi.mock('../app/providers', () => ({
  useAppContext: () => ({
    authState: {
      status: 'authenticated',
      user: { id: 'admin_1', email: 'admin@example.com', role: 'admin' },
      preferences: {
        defaultMode: 'chat',
        modelStrategy: 'balanced',
        networkEnabledHint: false,
        onboardingCompleted: false
      }
    },
    bootstrapAuth: () => Promise.resolve(),
    updatePreferences: (preferences: unknown) => Promise.resolve(preferences)
  })
}));

vi.mock('../features/chat/api', () => ({
  createChatApi: () => chatApiMocks,
  createConversationRealtimeSocket: () => ({
    close: vi.fn(),
    sendTyping: vi.fn()
  })
}));

vi.mock('../features/knowledge/api', () => ({
  createKnowledgeApi: () => knowledgeApiMocks
}));

vi.mock('../features/tasks/api', () => ({
  createTasksApi: () => taskApiMocks
}));

vi.mock('../lib/swr', async (importOriginal) => {
  const original = await importOriginal<typeof import('../lib/swr')>();
  return {
    ...original,
    fetcher: (key: Parameters<typeof original.fetcher>[0]) =>
      key[0] === '/api/v1/admin/stats' ? swrApiMocks.getAdminStats() : original.fetcher(key)
  };
});

vi.mock('../features/admin/api', () => ({
  createAdminApi: () => ({
    getStats: () =>
      Promise.resolve({
        users: { totalUsers: 24, activeUsers: 12, newUsersToday: 2, newUsersWeek: 5 },
        quotas: { totalBalance: 100, totalUsed: 25, activeTopups: 3 },
        conversations: 8,
        agents: 11,
        tasks: 4,
        mcpServers: 2,
        channelsTotal: 4,
        channelsOnline: 3,
        activeAgents: 7,
        apiCalls24h: 128
      }),
    listPlans: () =>
      Promise.resolve([
        {
          id: 'plan_pro',
          name: 'Pro',
          description: 'Production plan',
          quotaAmount: 500,
          tokenQuota: 1000000,
          price: 29,
          modelAccess: ['gpt-4o', 'claude-3.5'],
          agentLimit: 10,
          maxTokensPerRequest: 32000,
          durationDays: 30,
          isActive: true,
          isPublic: true,
          sortOrder: 1,
          subscriberCount: 42,
          createdAt: '2026-01-01T00:00:00Z',
          updatedAt: '2026-01-02T00:00:00Z'
        }
      ]),
    createPlan: (input: Record<string, unknown>) =>
      Promise.resolve({
        id: 'plan_router_created',
        createdAt: '2026-01-03T00:00:00Z',
        updatedAt: '2026-01-03T00:00:00Z',
        ...input
      }),
    updatePlan: (id: string, input: Record<string, unknown>) =>
      Promise.resolve({
        id,
        createdAt: '2026-01-01T00:00:00Z',
        updatedAt: '2026-01-03T00:00:00Z',
        ...input
      }),
    deactivatePlan: () => Promise.resolve(),
    listUsers: () =>
      Promise.resolve({
        data: [
          {
            id: 'user_1',
            email: 'admin@example.com',
            name: 'Admin User',
            role: 'admin',
            planID: 'plan_pro',
            planName: 'Pro',
            status: 'active',
            lastLoginAt: '2026-01-01T00:00:00Z',
            createdAt: '2025-01-01T00:00:00Z',
            usageStats: { totalTokens: 1000, totalAPICalls: 20, totalCost: 1.5 }
          }
        ],
        total: 1
      }),
    updateUser: (id: string, input: Record<string, unknown>) => Promise.resolve({ id, ...input }),
    disableUser: () => Promise.resolve(),
    enableUser: () => Promise.resolve(),
    listAuditLogs: () =>
      Promise.resolve({
        data: [
          {
            id: 'audit_1',
            actorID: 'user_1',
            actorEmail: 'admin@example.com',
            action: 'channel.create',
            resourceType: 'channel',
            resourceID: 'ch_1',
            changes: '{"name":"OpenAI"}',
            ipAddress: '127.0.0.1',
            createdAt: '2026-01-01T00:00:00Z'
          }
        ],
        total: 1
      }),
    getBillingSummary: adminApiMocks.getBillingSummary,
    listBillingSurface: adminApiMocks.listBillingSurface,
    getRelayPricingSettings: () =>
      Promise.resolve({
        groupMultipliers: { enterprise: 0.85 },
        modelMultipliers: { 'gpt-4o': 1.2, 'gpt-4o-mini': 0.7 }
      }),
    updateRelayPricingSettings: (settings: unknown) => Promise.resolve(settings),
    getUsageLimitSettings: () =>
      Promise.resolve([
        {
          enabled: true,
          id: 'limit_router_org_hour',
          limitType: 'request_tokens',
          limitValue: 250,
          maxConcurrentRequests: 5,
          maxTokensPerRequest: 250,
          maxTokensPerWindow: 1000,
          organizationId: 'org_router',
          period: 'hour',
          quotaMode: 'organization',
          scopeId: 'org_router',
          scopeType: 'organization',
          windowSeconds: 3600
        }
      ]),
    updateUsageLimitSettings: (settings: Record<string, unknown>) =>
      Promise.resolve({
        ...settings,
        id: typeof settings.id === 'string' && settings.id !== '' ? settings.id : 'limit_router_saved'
      }),
    listUsageLogs: (filter?: { status?: string; limit?: number }) => {
      if (filter?.status === 'error') {
        return Promise.resolve({
          data: [
            {
              id: 'usage_router_limited',
              organizationId: 'org_router',
              userId: 'user_router',
              requestId: 'req_settings_limited',
              apiType: 'chat',
              featureType: 'workspace_chat',
              model: 'gpt-4o',
              status: 'error',
              statusCode: 429,
              errorCode: 'relay_rate_limited',
              cost: 0,
              channelCost: 0,
              promptTokens: 0,
              completionTokens: 0,
              totalTokens: 0,
              createdAt: '2026-06-09T00:00:00Z'
            }
          ],
          total: 1
        });
      }
      if (filter?.status === 'success' && filter.limit === 1) {
        return Promise.resolve({
          data: [
            {
              id: 'usage_router_recovered',
              organizationId: 'org_router',
              userId: 'user_router',
              requestId: 'req_settings_recovered',
              apiType: 'chat',
              featureType: 'workspace_chat',
              model: 'gpt-4o',
              status: 'success',
              statusCode: 200,
              cost: 0.01,
              channelCost: 0.004,
              promptTokens: 20,
              completionTokens: 10,
              totalTokens: 30,
              createdAt: '2026-06-09T00:05:00Z'
            }
          ],
          total: 1
        });
      }
      return Promise.resolve({
        data: [
          {
            id: 'usage_admin_router',
            organizationId: 'org_router',
            userId: 'user_router',
            apiTokenId: 'tok_router_1',
            requestId: 'req_admin_router',
            apiType: 'chat',
            featureType: 'workspace_chat',
            quotaMode: 'relay_billing',
            model: 'gpt-4o',
            channelId: 'ch_router_1',
            provider: 'openai',
            status: 'success',
            statusCode: 200,
            latencyMs: 42,
            cost: 0.42,
            channelCost: 0.21,
            promptTokens: 100,
            completionTokens: 20,
            totalTokens: 120,
            createdAt: '2026-06-09T00:00:00Z'
          }
        ],
        total: 1
      });
    },
    getUsageAnalytics: () =>
      Promise.resolve({
        byModel: [{ dimension: 'model', key: 'gpt-4o', requestCount: 3, totalTokens: 150, totalCost: 0.0012 }],
        byFeature: [{ dimension: 'feature', key: 'chat', requestCount: 2, totalTokens: 120, totalCost: 0.0009 }],
        byUser: [{ dimension: 'user', key: 'user_router', requestCount: 4, totalTokens: 200, totalCost: 0.0015 }],
        byTime: [{ dimension: 'time', key: '2026-06-09T00:00:00Z', requestCount: 5, totalTokens: 300, totalCost: 0.002 }],
        byChannel: [{ dimension: 'channel', key: 'ch_router_1', requestCount: 6, totalTokens: 360, totalCost: 0.0025 }],
        byProvider: [{ dimension: 'provider', key: 'openai', requestCount: 7, totalTokens: 420, totalCost: 0.003 }],
        crossDimensions: [
          {
            dimension: 'model_time',
            key: 'gpt-4o / 2026-06-09T00:00:00Z',
            primary: 'gpt-4o',
            secondary: '2026-06-09T00:00:00Z',
            requestCount: 9,
            totalTokens: 900,
            totalCost: 0.009
          }
        ]
      }),
    listChannels: () =>
      Promise.resolve({
        data: [
          {
            id: 'ch_router_1',
            name: 'OpenAI Primary',
            provider: 'openai',
            baseURL: 'https://api.openai.com/v1',
            models: ['gpt-4o', 'gpt-4o-mini', 'text-embedding-3-small'],
            groups: ['default', 'vip'],
            rpm: 120,
            tpm: 120000,
            priority: 1,
            estimatedCostPer1K: 0.02,
            costMultiplier: 1.1,
            weight: 80,
            enabled: true,
            status: 'online',
            latency: 118,
            createdAt: '2026-06-01T00:00:00Z',
            updatedAt: '2026-06-09T00:00:00Z'
          }
        ],
        total: 1
      }),
    listChannelProviders: () =>
      Promise.resolve([
        {
          id: 'openai',
          displayName: 'OpenAI',
          kind: 'openai_compatible',
          status: 'supported',
          defaultBaseURL: 'https://api.openai.com/v1'
        },
        {
          id: 'anthropic',
          displayName: 'Claude',
          kind: 'native',
          status: 'supported',
          defaultBaseURL: 'https://api.anthropic.com'
        }
      ]),
    listChannelStats: () =>
      Promise.resolve([
        {
          channelID: 'ch_router_1',
          channelId: 'ch_router_1',
          totalRequests: 240,
          successCount: 230,
          failureCount: 10,
          rpmCurrent: 24,
          tpmCurrent: 4800,
          avgLatencyMs: 132,
          affinityConversationCount: 7
        }
      ]),
    getChannelHealth: () =>
      Promise.resolve({
        id: 'ch_router_1',
        status: 'online',
        latency: 118,
        models: ['gpt-4o', 'gpt-4o-mini'],
        balance: { amount: 12.5, currency: 'USD', source: 'openai_credit_grants' }
      }),
    testChannel: () =>
      Promise.resolve({
        success: true,
        latency: 118,
        provider: 'openai',
        models: ['gpt-4o-mini', 'text-embedding-3-small'],
        balance: { amount: 12.5, currency: 'USD', source: 'openai_credit_grants' },
        health: { status: 'online', checkedAt: '2026-06-09T00:00:00Z' }
      }),
    syncChannelModels: () =>
      Promise.resolve({
        channel: {
          id: 'ch_router_1',
          name: 'OpenAI Primary',
          provider: 'openai',
          baseURL: 'https://api.openai.com/v1',
          models: ['gpt-4o', 'gpt-4o-mini', 'text-embedding-3-small'],
          groups: ['default', 'vip'],
          rpm: 120,
          tpm: 120000,
          priority: 1,
          estimatedCostPer1K: 0.02,
          costMultiplier: 1.1,
          weight: 80,
          enabled: true,
          status: 'online',
          latency: 118,
          createdAt: '2026-06-01T00:00:00Z',
          updatedAt: '2026-06-09T00:00:00Z'
        },
        testResult: {
          success: true,
          latency: 121,
          provider: 'openai',
          models: ['gpt-4o-mini', 'text-embedding-3-small'],
          balance: { amount: 12.5, currency: 'USD' },
          health: { status: 'online' }
        }
      }),
    detectChannelModelUpdates: () =>
      Promise.resolve({
        id: 'ch_router_1',
        currentModels: ['gpt-4o'],
        upstreamModels: ['gpt-4o', 'gpt-4o-mini', 'text-embedding-3-small'],
        added: ['gpt-4o-mini', 'text-embedding-3-small'],
        removed: [],
        unchanged: ['gpt-4o'],
        testResult: {
          success: true,
          latency: 121,
          provider: 'openai',
          models: ['gpt-4o-mini', 'text-embedding-3-small'],
          balance: { amount: 12.5, currency: 'USD' },
          health: { status: 'online' }
        }
      }),
    applyChannelModelUpdates: () =>
      Promise.resolve({
        mode: 'merge',
        appliedModels: ['gpt-4o', 'gpt-4o-mini', 'text-embedding-3-small'],
        preview: {
          id: 'ch_router_1',
          currentModels: ['gpt-4o', 'gpt-4o-mini', 'text-embedding-3-small'],
          upstreamModels: ['gpt-4o', 'gpt-4o-mini', 'text-embedding-3-small'],
          added: [],
          removed: [],
          unchanged: ['gpt-4o', 'gpt-4o-mini', 'text-embedding-3-small']
        }
      }),
    refreshChannelBalance: () =>
      Promise.resolve({
        id: 'ch_router_1',
        status: 'online',
        balance: { amount: 12.5, currency: 'USD', source: 'openai_credit_grants' },
        channelHealth: { status: 'online', checkedAt: '2026-06-09T00:00:00Z' },
        testResult: {
          success: true,
          latency: 118,
          provider: 'openai',
          models: ['gpt-4o-mini'],
          balance: { amount: 12.5, currency: 'USD' },
          health: { status: 'online' }
        },
        checkedAt: '2026-06-09T00:00:00Z'
      }),
    batchUpdateChannels: () => Promise.resolve(),
    createChannel: (input: Record<string, unknown>) => Promise.resolve({ id: 'ch_router_created', ...input }),
    updateChannel: (id: string, input: Record<string, unknown>) => Promise.resolve({ id, ...input }),
    deleteChannel: () => Promise.resolve(),
    listRoutes: () =>
      Promise.resolve([
        {
          id: 'route_router_gpt4',
          model: 'gpt-4o*',
          strategy: 'weighted',
          channels: [
            {
              channelID: 'ch_router_1',
              channelName: 'OpenAI Primary',
              priority: 1,
              weight: 80,
              enabled: true
            }
          ],
          createdAt: '2026-06-01T00:00:00Z'
        }
      ]),
    createRoute: (input: Record<string, unknown>) => Promise.resolve({ id: 'route_router_created', ...input }),
    updateRoute: (id: string, input: Record<string, unknown>) => Promise.resolve({ id, ...input }),
    deleteRoute: () => Promise.resolve(),
    listObservabilityAlerts: () =>
      Promise.resolve([
        {
          key: 'relay-backlog',
          title: 'Relay backlog',
          severity: 'critical',
          status: 'open',
          summary: 'Relay queue exceeded recovery threshold',
          component: 'relay',
          lastOccurredAt: '2026-06-05T08:30:00Z',
          occurrenceCount: 3
        }
      ]),
    getObservabilityAlertRoutingRules: () =>
      Promise.resolve({
        debug: [],
        info: ['email'],
        warning: ['email', 'im'],
        critical: ['email', 'im', 'sms', 'third_party']
      }),
    updateObservabilityAlertRoutingRules: (rules: unknown) => Promise.resolve(rules),
    listObservabilityAlertProviders: () =>
      Promise.resolve([
        {
          id: 'alert_provider_slack_ops',
          kind: 'slack_webhook',
          channel: 'im',
          name: 'Slack Ops',
          status: 'active',
          config: {
            webhook_url: '********',
            default_channel: '#ops'
          },
          createdAt: '2026-06-05T08:00:00Z'
        }
      ]),
    createObservabilityAlertProvider: (input: Record<string, unknown>) =>
      Promise.resolve({
        id: 'alert_provider_created',
        channel: 'im',
        ...input
      }),
    updateObservabilityAlertProvider: (id: string, input: Record<string, unknown>) =>
      Promise.resolve({
        id,
        channel: 'im',
        ...input
      }),
    testObservabilityAlertProvider: () =>
      Promise.resolve({
        providerId: 'alert_provider_slack_ops',
        kind: 'slack_webhook',
        channel: 'im',
        ok: true,
        message: 'Provider test delivered.',
        testedAt: '2026-06-05T08:35:00Z'
      }),
    acknowledgeObservabilityAlert: () =>
      Promise.resolve({
        key: 'relay-backlog',
        title: 'Relay backlog',
        severity: 'critical',
        status: 'acknowledged',
        component: 'relay'
      }),
    resolveObservabilityAlert: () =>
      Promise.resolve({
        key: 'relay-backlog',
        title: 'Relay backlog',
        severity: 'critical',
        status: 'resolved',
        component: 'relay'
      }),
    listObservabilityAlertDeliveries: () =>
      Promise.resolve([
        {
          alertKey: 'relay-backlog',
          channel: 'email',
          providerId: 'smtp_primary',
          providerKind: 'smtp',
          delivered: true,
          attemptedAt: '2026-06-05T08:31:00Z'
        },
        {
          alertKey: 'relay-backlog',
          channel: 'im',
          providerId: 'alert_provider_slack_ops',
          providerKind: 'slack_webhook',
          delivered: false,
          error: 'im webhook failed',
          attemptedAt: '2026-06-05T08:32:00Z'
        }
      ]),
    listObservabilityRecoveryActions: () =>
      Promise.resolve([
        {
          id: 'restart-relay:relay-backlog:1',
          policyName: 'restart-relay',
          alertKey: 'relay-backlog',
          severity: 'critical',
          component: 'relay',
          type: 'restart',
          status: 'recorded',
          reason: 'Relay backlog',
          createdAt: '2026-06-05T09:00:00Z'
        }
      ]),
    listReviews: () =>
      Promise.resolve({
        data: [
          {
            id: 'agent_review_router',
            name: 'Research Agent',
            description: 'Helps with research',
            ownerID: 'owner_1',
            ownerName: 'Publisher',
            status: 'pending_review',
            visibility: 'public',
            pricingType: 'one_time',
            pricingAmount: 19,
            categoryID: 'cat_1',
            categoryName: 'Productivity',
            tags: ['research'],
            ratingAvg: 4.5,
            ratingCount: 8,
            installCount: 120,
            reviewSLA: {
              submittedAt: '2026-06-02T13:00:00Z',
              manualDeadlineAt: '2026-06-05T13:00:00Z',
              manualSlaHours: 72,
              manualSlaStatus: 'due_soon',
              minutesUntilDeadline: 60,
              automatedReviewDeadlineAt: '2026-06-02T13:05:00Z',
              automatedReviewSlaMinutes: 5,
              automatedReviewSlaStatus: 'overdue',
              vipPublisher: true,
              publisherTier: 'vip',
              publisherTierSource: 'organization_metadata'
            },
            createdAt: '2026-01-01T00:00:00Z',
            updatedAt: '2026-01-02T00:00:00Z'
          }
        ],
        total: 1
      }),
    listAPITokens: () =>
      Promise.resolve({
        data: [
          {
            createdAt: '2026-05-31T10:00:00Z',
            id: 'tok_admin_router',
            lastUsedAt: '2026-06-01T10:00:00Z',
            modelLimits: ['gpt-4o', 'claude-3-5-sonnet'],
            modelLimitsEnabled: true,
            name: 'Production key',
            organizationId: 'org_router',
            quotaLimit: 50,
            requestCount: 12,
            status: 'active',
            tokenPrefix: 'sk-oblv',
            totalCost: 1.23,
            usedQuota: 12.5,
            userEmail: 'user@example.com',
            userGroup: 'vip',
            userId: 'user_router'
          }
        ],
        total: 1
      }),
    listModelInventory: () =>
      Promise.resolve({
        data: [
          {
            avgCostMultiplier: 1.2,
            channelCount: 2,
            channels: [
              {
                costMultiplier: 1.1,
                enabled: true,
                estimatedCostPer1K: 0.02,
                groups: ['default', 'vip'],
                id: 'ch_router_primary',
                name: 'OpenAI primary',
                priority: 10,
                provider: 'openai'
              }
            ],
            disabledChannelCount: 1,
            enabledChannelCount: 1,
            groups: ['default', 'vip'],
            maxEstimatedCostPer1K: 0.05,
            minEstimatedCostPer1K: 0.02,
            model: 'gpt-4o',
            providers: ['openai', 'azure'],
            requestCount: 30,
            totalChannelCost: 0.61,
            totalCost: 1.23
          }
        ],
        total: 1
      }),
    approveAgent: adminApiMocks.approveAgent,
    createDueMarketplacePayouts: adminApiMocks.createDueMarketplacePayouts,
    dismissMarketplaceAbuseReport: adminApiMocks.dismissMarketplaceAbuseReport,
    enforceReviewSLA: adminApiMocks.enforceReviewSLA,
    listMarketplaceAbuseReports: adminApiMocks.listMarketplaceAbuseReports,
    markMarketplacePayoutFailed: adminApiMocks.markMarketplacePayoutFailed,
    markMarketplacePayoutPaid: adminApiMocks.markMarketplacePayoutPaid,
    reinstateMarketplaceAgent: adminApiMocks.reinstateMarketplaceAgent,
    rejectAgent: adminApiMocks.rejectAgent,
    resolveMarketplaceAbuseReport: adminApiMocks.resolveMarketplaceAbuseReport,
    refundTopup: adminApiMocks.refundTopup,
    takedownMarketplaceAgent: adminApiMocks.takedownMarketplaceAgent,
    requestAgentChanges: adminApiMocks.requestAgentChanges,
    revokeAPIToken: () => Promise.resolve()
  })
}));

vi.mock('../features/marketplace/api', () => {
  const marketplaceAgent = {
    id: 'agent_1',
    ownerID: 'owner_1',
    ownerName: 'Publisher',
    name: 'Research Agent',
    description: 'Helps with research workflows',
    categoryID: 'cat_1',
    categoryName: 'Productivity',
    categorySlug: 'productivity',
    tags: ['research', 'writing'],
    tools: '[{"name":"search"}]',
    exampleConversations: 'User asks for a market scan.',
    visibility: 'public',
    status: 'approved',
    pricingType: 'one_time',
    pricingAmount: 19,
    currentVersion: '1.0.0',
    installCount: 120,
    ratingAvg: 4.5,
    ratingCount: 8,
    paymentProviders: [{ name: 'stripe' }, { name: 'alipay' }],
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-02T00:00:00Z'
  };

  return {
    createMarketplaceApi: () => ({
      getCategories: () => Promise.resolve([{ id: 'cat_1', name: 'Productivity', slug: 'productivity', agentCount: 1 }]),
      searchAgents: () => Promise.resolve({ agents: [marketplaceAgent], total: 1 }),
      listTemplates: () =>
        Promise.resolve({
          templates: [
            {
              id: 'tpl_1',
              type: 'workflow',
              name: 'Lead Intake Template',
              description: 'Reusable workflow template for lead qualification.',
              templateData: { nodes: [{ id: 'start' }] },
              category: 'Sales',
              tags: ['crm', 'lead'],
              downloadsCount: 12,
              ratingAvg: 4.7,
              createdAt: '2026-01-06T00:00:00Z'
            }
          ],
          total: 1
        }),
      installTemplate: () =>
        Promise.resolve({
          id: 'tpl_install_1',
          templateID: 'tpl_1',
          type: 'workflow',
          name: 'Lead Intake Template',
          templateData: { nodes: [{ id: 'start' }] },
          installedAt: '2026-01-07T00:00:00Z'
        }),
      createTemplate: marketplaceApiMocks.createTemplate,
      getCuratedSections: () =>
        Promise.resolve({
          popular: [{ ...marketplaceAgent, id: 'agent_popular', name: 'Popular Ops Agent', installCount: 420 }],
          topRated: [{ ...marketplaceAgent, id: 'agent_top_rated', name: 'Top Rated QA Agent', ratingAvg: 4.9 }],
          recent: [{ ...marketplaceAgent, id: 'agent_recent', name: 'New Arrival Agent', createdAt: '2026-01-08T00:00:00Z' }]
        }),
      getAgent: () => Promise.resolve(marketplaceAgent),
      getReviews: () =>
        Promise.resolve([
          {
            id: 'review_1',
            agentID: 'agent_1',
            userID: 'user_1',
            rating: 5,
            body: 'Great for launch research.',
            createdAt: '2026-01-03T00:00:00Z'
          }
        ]),
      getVersions: () => Promise.resolve([{ id: 'ver_1', version: '1.0.0', createdAt: '2026-01-01T00:00:00Z' }]),
      installAgent: marketplaceApiMocks.installAgent,
      uninstallAgent: () => Promise.resolve(),
      deleteAgent: marketplaceApiMocks.deleteAgent,
      getMyAgents: () => Promise.resolve([marketplaceAgent]),
      getInstalledAgents: () =>
        Promise.resolve([
          {
            id: 'install_1',
            agentID: 'agent_1',
            agentName: 'Research Agent',
            version: '1.0.0',
            installedAt: '2026-01-04T00:00:00Z'
          }
        ]),
      getSettlementPreferences: () =>
        Promise.resolve({
          cycle: 'monthly',
          label: 'Monthly',
          payoutBusinessDays: 5,
          processingFeePercent: 1,
          minimumPayoutAmount: 100,
          effectiveFrom: 'next_settlement_cycle'
        }),
      updateSettlementPreferences: () =>
        Promise.resolve({
          cycle: 'weekly',
          label: 'Weekly',
          payoutBusinessDays: 3,
          processingFeePercent: 2,
          minimumPayoutAmount: 100,
          effectiveFrom: 'next_settlement_cycle'
        }),
      getPublisherStats: () =>
        Promise.resolve({
          totalAgents: 1,
          totalInstalls: 120,
          activeUsers: 64,
          totalAPICalls: 900,
          grossRevenue: 15000,
          platformFees: 2850,
          netRevenue: 12150,
          refundedAmount: 350,
          pendingSettlementAmount: 1200,
          availableAmount: 10950,
          payoutPendingAmount: 640,
          paidOutAmount: 8000,
          revenueTier: {
            currentTier: 'tier_3',
            label: 'Tier 3',
            monthlySalesAmount: 15000,
            platformFeeAmount: 2850,
            publisherNetAmount: 12150,
            platformFeePercent: 15,
            publisherSharePercent: 85,
            effectivePlatformFeePercent: 19,
            nextTierAt: 100000,
            salesToNextTier: 85000,
            estimatedPublisherNetIncreaseAtNextTier: 72250
          },
          perAgentStats: [
            {
              agentID: 'agent_1',
              agentName: 'Research Agent',
              installCount: 120,
              activeUsers: 64,
              apiCallCount: 900
            }
          ]
        }),
      publishAgent: () =>
        Promise.resolve({
          id: 'agent_published_router',
          name: 'Published Router Agent',
          status: 'pending_review'
        }),
      submitReview: () => Promise.resolve({ id: 'review_2', agentID: 'agent_1', userID: 'user_1', rating: 5, body: 'Useful' })
    }),
    getMarketplaceCheckoutUrl: (value: { url?: string; checkoutUrl?: string; checkoutURL?: string }) =>
      value.url ?? value.checkoutUrl ?? value.checkoutURL ?? '',
    isMarketplaceCheckoutResponse: (value: { checkoutSessionId?: string; checkoutSessionID?: string }) =>
      Boolean(value.checkoutSessionId ?? value.checkoutSessionID)
  };
});

vi.mock('../features/scheduledTasks/scheduledTasksApi', () => ({
  createScheduledTasksApi: () => ({
    createScheduledTask: () =>
      Promise.resolve({
        cronExpression: '0 9 * * 1',
        enabled: true,
        id: 'schedule_2',
        name: 'Created workflow schedule',
        targetId: 'workflow_2',
        targetType: 'workflow'
      }),
    listRuns: () =>
      Promise.resolve([
        {
          createdAt: '2026-06-09T09:00:00Z',
          finishedAt: '2026-06-09T09:02:00Z',
          id: 'run_schedule_router_list',
          scheduledTaskId: 'schedule_1',
          startedAt: '2026-06-09T09:00:00Z',
          status: 'completed',
          updatedAt: '2026-06-09T09:02:00Z'
        }
      ]),
    listScheduledTasks: () =>
      Promise.resolve([
        {
          cronExpression: '0 9 * * 1',
          enabled: true,
          id: 'schedule_1',
          lastRunAt: '2026-06-02T09:00:00Z',
          name: 'Weekly workflow schedule',
          nextRunAt: '2026-06-09T09:00:00Z',
          targetId: 'workflow_1',
          targetType: 'workflow'
        }
      ]),
    runScheduledTaskNow: () =>
      Promise.resolve({
        createdAt: '2026-06-09T10:00:00Z',
        finishedAt: null,
        id: 'run_schedule_router_now',
        scheduledTaskId: 'schedule_1',
        startedAt: '2026-06-09T10:00:00Z',
        status: 'running',
        updatedAt: '2026-06-09T10:00:00Z'
      }),
    updateScheduledTaskEnabled: () =>
      Promise.resolve({
        cronExpression: '0 9 * * 1',
        enabled: false,
        id: 'schedule_1',
        lastRunAt: '2026-06-02T09:00:00Z',
        name: 'Weekly workflow schedule',
        nextRunAt: null,
        targetId: 'workflow_1',
        targetType: 'workflow'
      })
  })
}));

vi.mock('../features/workflows/workflowsApi', () => {
  const workflowFixture = () => ({
    definition: {
      concurrency_overflow: 'queue',
      edges: [
        { id: 'edge-start-classify', source: 'manual-start', target: 'classify' },
        { branch: 'true', id: 'edge-classify-notify', source: 'classify', target: 'notify-team' }
      ],
      max_concurrent_executions: 2,
      max_execution_duration_seconds: 600,
      max_node_executions: 40,
      max_tokens_budget: 12000,
      nodes: [
        { id: 'manual-start', input: { ticketId: 'INC-42' }, position: { x: 80, y: 80 }, type: 'manual' },
        {
          failurePolicy: { maxRetries: 2, retryDelays: ['1s', '5s'], strategy: 'pause_on_failure' },
          id: 'classify',
          input: { model: 'gpt-4o-mini' },
          position: { x: 320, y: 80 },
          type: 'llm'
        },
        {
          failurePolicy: { failureBranchNodeId: 'manual-start', strategy: 'failure_branch' },
          id: 'notify-team',
          input: { channel: '#ops' },
          position: { x: 560, y: 80 },
          type: 'notification'
        }
      ],
      triggers: {
        conversation: [{ conversationId: 'conversation_router', id: 'conversation-main' }],
        schedule: [{ cronExpression: '*/15 * * * *', enabled: true, id: 'quarter-hour-triage' }],
        semantic: [{ id: 'urgent-ticket', keywords: ['incident', 'sev1'], semanticThreshold: 0.86 }],
        webhook: {
          id: 'github',
          path: '/api/v1/workflows/webhooks/org_router/workflow_1',
          secret: '********'
        }
      }
    },
    description: 'Route-level incident workflow.',
    id: 'workflow_1',
    name: 'Incident triage workflow',
    organizationId: 'org_router',
    status: 'draft',
    variables: { owner: 'ops' },
    version: 3
  });

  const executionFixture = () => ({
    id: 'exec_router_paused',
    input: { ticketId: 'INC-42' },
    nodeExecutions: [
      {
        completedAt: '2026-06-09T10:00:01Z',
        durationMs: 1200,
        nodeId: 'manual-start',
        nodeType: 'manual',
        output: { ticketId: 'INC-42' },
        status: 'succeeded'
      },
      {
        attempt: 2,
        durationMs: 3400,
        error: { message: 'provider timeout' },
        input: { severity: 'sev1' },
        nodeId: 'classify',
        nodeType: 'llm',
        status: 'failed'
      },
      {
        context: { waitReason: 'approval_required' },
        input: { channel: '#ops' },
        nodeId: 'notify-team',
        nodeType: 'approval',
        status: 'pending'
      }
    ],
    output: { summary: 'needs operator review' },
    status: 'paused',
    workflowId: 'workflow_1'
  });

  return {
    createWorkflowsApi: () => ({
      cancelExecution: () => Promise.resolve({ ...executionFixture(), status: 'cancelled' }),
      checkWorkflowResourceLimits: () => Promise.resolve(executionFixture()),
      createWorkflow: () => Promise.resolve(workflowFixture()),
      createWorkflowBranch: () =>
        Promise.resolve({
          ...workflowFixture(),
          definition: {
            ...workflowFixture().definition,
            branch: { sourceWorkflowId: 'workflow_1' }
          },
          id: 'workflow_1_branch_canary',
          name: 'Incident triage workflow v2 branch',
          version: 4
        }),
      deleteWorkflow: () => Promise.resolve({ ...workflowFixture(), status: 'archived' }),
      executeWorkflow: () => Promise.resolve({ ...executionFixture(), id: 'exec_router_run', status: 'running' }),
      getExecution: () => Promise.resolve(executionFixture()),
      getExecutionDebugSnapshot: () =>
        Promise.resolve({
          executionId: 'exec_router_paused',
          logs: [{ level: 'warn', message: 'Provider timeout', nodeId: 'classify', timestamp: '2026-06-09T10:00:03Z' }],
          outputs: { 'manual-start': { ticketId: 'INC-42' } },
          performance: { bottleneckNodeId: 'classify', nodeDurationsMs: { classify: 3400 }, totalDurationMs: 4600 },
          status: 'paused',
          trace: [
            { durationMs: 1200, nodeId: 'manual-start', nodeType: 'manual', status: 'succeeded' },
            {
              durationMs: 3400,
              error: { message: 'provider timeout' },
              nodeId: 'classify',
              nodeType: 'llm',
              status: 'failed'
            }
          ],
          variableSnapshot: {
            context: { owner: 'ops' },
            input: { ticketId: 'INC-42' },
            nodeOutputs: { 'manual-start': { ticketId: 'INC-42' } }
          },
          workflowId: 'workflow_1'
        }),
      listExecutions: () => Promise.resolve([executionFixture()]),
      listWorkflowVersions: () =>
        Promise.resolve([
          workflowFixture(),
          { ...workflowFixture(), definition: { ...workflowFixture().definition, nodes: [{ id: 'manual-start', type: 'manual' }] }, version: 2 },
          {
            ...workflowFixture(),
            definition: {
              ...workflowFixture().definition,
              branch: { sourceWorkflowId: 'workflow_1' }
            },
            id: 'workflow_1_branch_canary',
            name: 'Incident triage workflow v2 branch',
            status: 'draft',
            version: 4
          }
        ]),
      listWorkflows: () => Promise.resolve([workflowFixture()]),
      matchConversationTriggers: () =>
        Promise.resolve([
          {
            conversationId: 'conversation_router',
            triggerId: 'conversation-main',
            workflowId: 'workflow_1',
            workflowName: 'Incident triage workflow',
            workflowVersion: 3
          }
        ]),
      matchSemanticTriggers: () =>
        Promise.resolve([
          {
            keyword: 'sev1',
            matchMethod: 'semantic',
            score: 0.91,
            semanticThreshold: 0.86,
            triggerId: 'urgent-ticket',
            workflowId: 'workflow_1',
            workflowName: 'Incident triage workflow',
            workflowVersion: 3
          }
        ]),
      mergeWorkflowBranch: () => Promise.resolve({ ...workflowFixture(), version: 5 }),
      pauseExecution: () => Promise.resolve(executionFixture()),
      publishWorkflowBranch: () => Promise.resolve({ ...workflowFixture(), status: 'published', version: 5 }),
      resolvePausedFailure: () => Promise.resolve(executionFixture()),
      resumeExecution: () => Promise.resolve({ ...executionFixture(), status: 'running' }),
      rollbackWorkflow: () => Promise.resolve({ ...workflowFixture(), version: 2 }),
      testNode: () =>
        Promise.resolve({
          durationMs: 42,
          input: { ticketId: 'INC-42' },
          nodeId: 'classify',
          output: { severity: 'sev1' },
          status: 'succeeded',
          workflowId: 'workflow_1'
        }),
      triggerWorkflowWebhook: () =>
        Promise.resolve({ ...executionFixture(), id: 'exec_router_webhook', status: 'queued' }),
      updateWorkflow: () => Promise.resolve({ ...workflowFixture(), version: 4 })
    })
  };
});

vi.mock('../features/agents/memoriesApi', () => ({
  createAgentMemoriesApi: () => ({
    createMemory: () =>
      Promise.resolve({
        agentId: 'agent_1',
        content: 'Remember launch checklist owner.',
        id: 'memory_created_router',
        importance: 4,
        metadata: { managedBy: 'workspace' },
        type: 'user_managed'
      }),
    deleteMemory: () => Promise.resolve(),
    exportMemories: () =>
      Promise.resolve({
        data: [
          {
            agentId: 'agent_1',
            content: 'Router export memory.',
            id: 'memory_export_router',
            importance: 5,
            metadata: { source: 'router-test' },
            type: 'user_managed'
          }
        ],
        total: 1
      }),
    importMemories: () =>
      Promise.resolve([
        {
          agentId: 'agent_1',
          content: 'Imported router memory.',
          id: 'memory_import_router',
          importance: 3,
          metadata: { imported: true },
          type: 'user_managed'
        }
      ]),
    searchMemories: () =>
      Promise.resolve({
        data: [
          {
            agentId: 'agent_1',
            content: 'Router search memory.',
            id: 'memory_search_router',
            importance: 5,
            metadata: { source: 'router-test' },
            type: 'user_managed'
          }
        ],
        total: 1
      }),
    updateMemory: () =>
      Promise.resolve({
        agentId: 'agent_1',
        content: 'Updated router memory.',
        id: 'memory_search_router',
        importance: 4,
        metadata: { source: 'router-test' },
        type: 'user_managed'
      })
  })
}));

vi.mock('../features/publishingChannels/publishingChannelsApi', () => ({
  createPublishingChannelsApi: () => ({
    createChannel: () =>
      Promise.resolve({
        config: { endpointUrl: 'https://hooks.example/ops' },
        id: 'channel_1',
        name: 'Ops Webhook',
        status: 'active',
        type: 'webhook'
      }),
    deleteChannel: () =>
      Promise.resolve({
        config: { endpointUrl: 'https://hooks.example/ops-renamed', secret: '********' },
        id: 'channel_1',
        name: 'Ops Webhook Renamed',
        status: 'disabled',
        type: 'webhook'
      }),
    listChannelMessages: () =>
      Promise.resolve([
        {
          created_at: '2026-06-09T00:00:00Z',
          direction: 'outbound',
          id: 'log_router_recent',
          status: 'delivered',
          transform_success: true
        }
      ]),
    listChannels: () =>
      Promise.resolve([
        {
          config: { endpointUrl: 'https://hooks.example/ops' },
          id: 'channel_1',
          name: 'Ops Webhook',
          status: 'active',
          type: 'webhook'
        }
      ]),
    listFailedChannelMessages: () =>
      Promise.resolve([
        {
          created_at: '2026-06-09T00:01:00Z',
          failure_reason: 'adapter timeout',
          id: 'log_router_failed',
          next_retry_at: '2026-06-09T00:05:00Z',
          retry_count: 2,
          status: 'failed',
          transform_success: true
        }
      ]),
    retryFailedChannelMessages: () => Promise.resolve({ claimed: 1, failed: 0, permanentFailures: 0, succeeded: 1 }),
    sendChannelMessage: () => Promise.resolve({ id: 'log_1', status: 'recorded', transform_success: true }),
    testChannel: () => Promise.resolve({ message: 'channel adapter is available', status: 'success' }),
    updateChannel: () =>
      Promise.resolve({
        config: { endpointUrl: 'https://hooks.example/ops-renamed', secret: '********' },
        id: 'channel_1',
        name: 'Ops Webhook Renamed',
        status: 'active',
        type: 'webhook'
      }),
    updateChannelStatus: (channel: unknown) => Promise.resolve(channel)
  })
}));

vi.mock('../features/agents/planStepsApi', () => ({
  createAgentPlanStepsApi: () => agentPlanStepsApiMocks
}));

vi.mock('../features/agents/agentsApi', () => ({
  createAgentsApi: () => agentsApiMocks
}));

vi.mock('../features/mcp/mcpServersApi', () => ({
  createMcpServersApi: () => ({
    addServer: () =>
      Promise.resolve({
        id: 'mcp_remote',
        name: 'Remote MCP',
        status: 'disconnected',
        url: 'https://mcp.example/sse'
      }),
    connectServer: (serverId: string) =>
      Promise.resolve({
        id: serverId,
        name: 'Remote MCP',
        status: 'connected',
        url: 'https://mcp.example/sse'
      }),
    disconnectServer: () => Promise.resolve({ status: 'disconnected' }),
    executeTool: () => Promise.resolve({ content: '{"ok":true}', isError: false }),
    getServerStatus: () => Promise.resolve({ status: 'connected' }),
    listLocalServers: () =>
      Promise.resolve([
        {
          description: 'Tenant-safe local MCP tools exposed by this server',
          id: 'local_builtin_safe',
          name: 'Oblivious Safe Builtins',
          toolCount: 2
        }
      ]),
    listServerTools: () =>
      Promise.resolve([
        {
          description: 'Search tenant-safe docs',
          inputSchema: { type: 'object' },
          name: 'search_docs'
        }
      ]),
    listServers: () =>
      Promise.resolve([
        {
          hasAuthToken: true,
          id: 'mcp_remote',
          name: 'Remote MCP',
          status: 'disconnected',
          url: 'https://mcp.example/sse'
        }
      ])
  })
}));

import { createAppRouter } from './router';
import { routerFuture } from './routerFuture';

describe('app router', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      const rawUrl = typeof input === 'string' ? input : input instanceof Request ? input.url : input.toString();
      const pathname = new URL(rawUrl, 'http://localhost').pathname;
      if (pathname === '/api/v1/app/readiness/capabilities') {
        return jsonResponse(routerProjectionResponse());
      }
      return jsonResponse({ ok: false, data: null, error: { code: 'not_found', message: `Unhandled test request: ${pathname}` } }, 404);
    }));
  });

  afterEach(() => {
    Object.values(chatApiMocks).forEach((mock) => mock.mockClear());
    Object.values(knowledgeApiMocks).forEach((mock) => mock.mockClear());
    Object.values(taskApiMocks).forEach((mock) => mock.mockClear());
    Object.values(adminApiMocks).forEach((mock) => mock.mockClear());
    Object.values(marketplaceApiMocks).forEach((mock) => mock.mockClear());
    Object.values(agentPlanStepsApiMocks).forEach((mock) => mock.mockClear());
    Object.values(agentsApiMocks).forEach((mock) => mock.mockClear());
    Object.values(swrApiMocks).forEach((mock) => mock.mockClear());
    vi.unstubAllGlobals();
    window.history.replaceState({}, '', '/');
  });

  it('renders home content on /', async () => {
    const router = createAppRouter(['/']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Oblivious')).toBeInTheDocument();
    expect(await screen.findByText('AI workspace framework')).toBeInTheDocument();
  });

  it('passes chat conversation route params into the chat workspace API', async () => {
    const router = createAppRouter(['/chat/conversation_router']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Router parameter message.')).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/chat/conversation_router');
    expect(chatApiMocks.listMessages).toHaveBeenCalledWith('conversation_router');
    expect(chatApiMocks.getConversationConfig).toHaveBeenCalledWith('conversation_router');
  });

  it('executes chat conversation settings, message streaming, and SOLO handoff inside the workspace shell', async () => {
    chatApiMocks.listMessages
      .mockResolvedValueOnce([{ id: 'message_router', role: 'assistant', content: 'Router parameter message.' }])
      .mockResolvedValueOnce([
        { id: 'message_router', role: 'assistant', content: 'Router parameter message.' },
        { id: 'message_user_router', role: 'user', content: 'Summarize the launch risk.' },
        { id: 'message_assistant_router', role: 'assistant', content: 'Router streamed final answer.' }
      ]);
    const router = createAppRouter(['/chat/conversation_router']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(within(workspaceNavigation).getByRole('link', { name: 'Chat' })).toHaveAttribute('aria-current', 'page');
    expect(await screen.findByRole('heading', { name: 'Chat workspace' })).toBeInTheDocument();
    expect(await screen.findByText('Router parameter message.')).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/chat/conversation_router');

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Save conversation settings' })).toBeEnabled();
    });

    fireEvent.change(screen.getByLabelText('Temperature'), { target: { value: '0.6' } });
    fireEvent.change(screen.getByLabelText('Max output tokens'), { target: { value: '1200' } });
    fireEvent.change(screen.getByLabelText('System prompt override'), { target: { value: 'Prefer release-risk bullets.' } });
    fireEvent.click(screen.getByLabelText('Enable tools for this conversation'));
    fireEvent.click(screen.getByRole('button', { name: 'Save conversation settings' }));

    await waitFor(() => {
      expect(chatApiMocks.updateConversationConfig).toHaveBeenCalledWith('conversation_router', {
        knowledgeBaseIds: ['kb_router'],
        maxOutputTokens: 1200,
        modelId: 'gpt-4o-mini',
        personaId: '',
        systemPromptOverride: 'Prefer release-risk bullets.',
        temperature: 0.6,
        toolsEnabled: false
      });
    });

    fireEvent.change(screen.getByLabelText('Message draft'), { target: { value: 'Summarize the launch risk.' } });
    fireEvent.click(screen.getByRole('button', { name: 'Send message' }));

    await waitFor(() => {
      expect(chatApiMocks.sendMessageStream).toHaveBeenCalledWith(
        'conversation_router',
        {
          content: 'Summarize the launch risk.',
          overrides: {
            maxOutputTokens: 1200,
            modelId: 'gpt-4o-mini',
            systemPromptOverride: 'Prefer release-risk bullets.',
            temperature: 0.6,
            toolsEnabled: false
          }
        },
        expect.objectContaining({ onChunk: expect.any(Function) })
      );
    });
    expect(await screen.findByText('Router streamed final answer.')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Hand off to SOLO' }));

    expect(await screen.findByRole('heading', { name: 'Convert to SOLO task' })).toBeInTheDocument();
    expect(screen.getByLabelText('SOLO task goal')).toHaveValue('Create a SOLO follow-up from conversation_router.');
    expect(screen.getByLabelText('Use knowledge base Research Vault in SOLO')).toBeChecked();
    fireEvent.change(screen.getByLabelText('Authorization scope for SOLO'), { target: { value: 'full_access' } });
    fireEvent.change(screen.getByLabelText('Allowed tools for SOLO'), { target: { value: 'browser, shell' } });
    fireEvent.change(screen.getByLabelText('Blocked tools for SOLO'), { target: { value: 'email' } });
    fireEvent.click(screen.getByRole('button', { name: 'Start in SOLO' }));

    await waitFor(() => {
      expect(taskApiMocks.createTask).toHaveBeenCalledWith({
        authorizationScope: 'full_access',
        budgetLimit: 20,
        executionMode: 'safe',
        goal: 'Create a SOLO follow-up from conversation_router.',
        knowledgeBaseIds: ['kb_router'],
        toolAllowList: ['browser', 'shell'],
        toolDenyList: ['email']
      });
    });
    await waitFor(() => {
      expect(taskApiMocks.startTask).toHaveBeenCalledWith('task_router_new');
    });
    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/solo');
      expect(router.state.location.search).toBe('?taskId=task_router_new&returnTo=%2Fchat%2Fconversation_router');
    });
  });

  it('renders knowledge route inside the workspace shell', async () => {
    const router = createAppRouter(['/knowledge']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Knowledge' })).toBeInTheDocument();
  });

  it('keeps knowledge detail route-level RAG, document, chunk, and evaluation controls reachable', async () => {
    const router = createAppRouter(['/knowledge/kb_router']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(within(workspaceNavigation).getByRole('link', { name: 'Knowledge' })).toHaveAttribute('href', '/knowledge');
    expect(await screen.findByRole('heading', { name: 'Research Vault' })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/knowledge/kb_router');

    await waitFor(() => {
      expect(knowledgeApiMocks.getKnowledgeBase).toHaveBeenCalledWith('kb_router');
      expect(knowledgeApiMocks.listKnowledgeDocuments).toHaveBeenCalledWith('kb_router');
      expect(knowledgeApiMocks.listRetrievalTestCases).toHaveBeenCalledWith('kb_router');
    });

    expect(screen.getByText('Knowledge base ID: kb_router')).toBeInTheDocument();
    expect(screen.getByText('Retrieval strategy: hybrid_rerank')).toBeInTheDocument();
    expect(screen.getByText('Chunking: semantic · 800 chars · 75 overlap')).toBeInTheDocument();
    expect(screen.getByText('Reranking: bge-reranker-large · top 8')).toBeInTheDocument();
    expect(screen.getByLabelText('Default retrieval strategy')).toHaveValue('hybrid_rerank');
    expect(screen.getByLabelText('Chunking strategy')).toHaveValue('semantic');
    expect(screen.getByLabelText('Chunk size')).toHaveValue(800);
    expect(screen.getByLabelText('Chunk overlap')).toHaveValue(75);
    expect(screen.getByLabelText('Embedding model')).toHaveValue('text-embedding-3-large');
    expect(screen.getByLabelText('Rerank top K')).toHaveValue(8);

    expect(screen.getByText('Router Retrieval Runbook')).toBeInTheDocument();
    expect(screen.getByText('Deployment controls include rollback and citation checks.')).toBeInTheDocument();
    expect(screen.getByText('Version: v2')).toBeInTheDocument();
    expect(screen.getByText('Update strategy: versioned')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'View chunks for Router Retrieval Runbook' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'View version history for Router Retrieval Runbook' })).toBeEnabled();

    expect(screen.getByLabelText('Retrieval test set')).toBeInTheDocument();
    expect(screen.getByText('Saved cases: 1')).toBeInTheDocument();
    expect(screen.getByText('deployment rollback')).toBeInTheDocument();
    expect(screen.getByText('Expected: Router Retrieval Runbook · chunk 1 · chunk_router_1')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Retrieval query'), { target: { value: 'deployment rollback' } });
    fireEvent.change(screen.getByLabelText('Retrieval mode'), { target: { value: 'hybrid' } });
    fireEvent.click(screen.getByRole('button', { name: 'Search knowledge' }));

    await waitFor(() => {
      expect(knowledgeApiMocks.retrieveKnowledge).toHaveBeenCalledWith('kb_router', {
        mode: 'hybrid',
        query: 'deployment rollback'
      });
    });
    expect(await screen.findByText('Deployment rollback needs cited context.')).toBeInTheDocument();
    expect(screen.getByText('Score: 94%')).toBeInTheDocument();
    expect(screen.getByText('Method: hybrid_rerank')).toBeInTheDocument();
    expect(screen.getByText('Source: Router Retrieval Runbook · chunk 1 · chunk_router_1')).toBeInTheDocument();
    expect(screen.getByText('Page: 7')).toBeInTheDocument();
    expect(screen.getByText('Original text: Deployment rollback needs cited context.')).toBeInTheDocument();
    expect(screen.getByLabelText('Similarity score for chunk_router_1')).toHaveValue(0.94);
    expect(screen.getByLabelText('Highlight positions for chunk_router_1')).toHaveTextContent('Highlight: 11-19');

    fireEvent.click(screen.getByRole('button', { name: 'View chunk chunk_router_1' }));

    await waitFor(() => {
      expect(knowledgeApiMocks.listKnowledgeDocumentChunks).toHaveBeenCalledWith(
        'kb_router',
        'doc_router_overview'
      );
    });
    expect(await screen.findByRole('heading', { name: 'Chunks for Router Retrieval Runbook' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Chunk 1 chunk_router_1 selected' })).toHaveAttribute(
      'aria-pressed',
      'true'
    );
    expect(screen.getByLabelText('Original document preview')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open PDF page 7' })).toHaveAttribute(
      'href',
      'https://docs.example/router.pdf#page=7'
    );
    expect(screen.getByLabelText('Chunk content editor')).toHaveValue('Deployment rollback needs cited context.');
    fireEvent.change(screen.getByLabelText('Chunk split position'), { target: { value: '20' } });
    expect(screen.getByRole('button', { name: 'Split chunk' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Merge with previous chunk' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Merge with next chunk' })).toBeEnabled();

    fireEvent.click(screen.getByRole('button', { name: 'View version history for Router Retrieval Runbook' }));

    await waitFor(() => {
      expect(knowledgeApiMocks.listKnowledgeDocumentVersions).toHaveBeenCalledWith(
        'kb_router',
        'doc_router_overview'
      );
    });
    expect(await screen.findByRole('heading', { name: 'Version history for Router Retrieval Runbook' })).toBeInTheDocument();
    expect(screen.getByText('Version v2 · chunks 2')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Restore version v2 for Router Retrieval Runbook' })).toBeEnabled();

    fireEvent.click(screen.getByRole('button', { name: 'Run retrieval tests' }));

    await waitFor(() => {
      expect(knowledgeApiMocks.runRetrievalTestCases).toHaveBeenCalledWith('kb_router', {
        mode: 'hybrid'
      });
    });
    expect(await screen.findByText('Evaluation: 1 passed / 0 failed / 1 total')).toBeInTheDocument();
    expect(screen.getByText('case_router_1 passed at rank 1')).toBeInTheDocument();
  });

  it('renders memories route inside the workspace shell', async () => {
    const router = createAppRouter(['/memories']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Agent Memories' })).toBeInTheDocument();
  });

  it('keeps agent memories route-level CRUD and import-export controls reachable', async () => {
    const router = createAppRouter(['/memories']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(within(workspaceNavigation).getByRole('link', { name: 'Agents' })).toHaveAttribute('href', '/agents');
    expect(await screen.findByRole('heading', { name: 'Agent Memories' })).toBeInTheDocument();
    expect(await screen.findByLabelText('Optional agent ID')).toBeInTheDocument();
    expect(screen.getByLabelText('Memory content')).toBeInTheDocument();
    expect(screen.getByLabelText('Memory importance')).toHaveValue('3');
    expect(screen.getByRole('button', { name: 'Create memory' })).toBeDisabled();
    expect(screen.getByLabelText('Search query')).toBeInTheDocument();
    expect(screen.getByLabelText('Memory type')).toHaveValue('');
    expect(screen.getByLabelText('Result limit')).toHaveValue(10);
    expect(screen.getByRole('button', { name: 'Export memories' })).toBeDisabled();
    expect(screen.getByLabelText('Import memories JSON')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Search query'), { target: { value: 'router' } });
    fireEvent.click(screen.getByRole('button', { name: 'Search memories' }));

    expect(await screen.findByText('Router search memory.')).toBeInTheDocument();
    expect(screen.getByText('Agent: agent_1')).toBeInTheDocument();
    expect(screen.getByLabelText('Importance 5 of 5')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Edit memory' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Delete memory' })).toBeInTheDocument();
  });

  it('renders agent plan steps route inside the workspace shell', async () => {
    const router = createAppRouter(['/agent-runs/run_1/plan-steps']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Agent Plan Steps' })).toBeInTheDocument();
    expect(await screen.findByText('Run run_1')).toBeInTheDocument();
  });

  it('keeps agent plan steps route-level run, approval, and editing controls reachable', async () => {
    const router = createAppRouter(['/agent-runs/run_1/plan-steps']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(within(workspaceNavigation).getByRole('link', { name: 'Agents' })).toHaveAttribute('href', '/agents');
    expect(await screen.findByRole('heading', { name: 'Agent Plan Steps' })).toBeInTheDocument();
    expect(await screen.findByText('Run run_1')).toBeInTheDocument();
    expect(await screen.findByText('Status: token_budget_exceeded')).toBeInTheDocument();
    expect(screen.getByLabelText('Agent run execution controls')).toHaveTextContent('Mode planning');
    expect(screen.getByLabelText('Agent run execution controls')).toHaveTextContent('Iterations 4');
    expect(screen.getByLabelText('Agent run execution controls')).toHaveTextContent('Tool calls 2');
    expect(screen.getByLabelText('Agent run execution controls')).toHaveTextContent(
      'Stop reason token_budget_exceeded: used 1200 tokens exceeds budget 1000'
    );
    expect(screen.getByRole('button', { name: 'Refresh plan steps' })).toBeEnabled();
    expect(screen.getByLabelText('Increased token budget')).toHaveValue(2500);
    expect(screen.getByRole('button', { name: 'Continue with budget' })).toBeEnabled();

    expect(screen.getByRole('heading', { name: 'Tool Approval Queue' })).toBeInTheDocument();
    const pendingToolRun = screen.getByLabelText('Tool run web_search');
    expect(within(pendingToolRun).getByText('pending_approval')).toBeInTheDocument();
    expect(within(pendingToolRun).getByText('Approval: pending')).toBeInTheDocument();
    expect(within(pendingToolRun).getByText('Risk: medium')).toBeInTheDocument();
    expect(screen.getByLabelText('Operator decision reason for web_search')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Approve tool web_search' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Reject tool web_search' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Retry tool web_search' })).toBeDisabled();
    const failedToolRun = screen.getByLabelText('Tool run shell');
    expect(within(failedToolRun).getByText('failed')).toBeInTheDocument();
    expect(within(failedToolRun).getByText('exit 1')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Retry tool shell' })).toBeEnabled();

    expect(screen.getByLabelText('Plan steps')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Inspect workspace' })).toBeInTheDocument();
    expect(screen.getByText('Workspace inspected.')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Patch router coverage' })).toBeInTheDocument();
    expect(screen.getByText('Tool: editor')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Edit Patch router coverage' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Insert after Patch router coverage' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Approve Patch router coverage' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Execute Patch router coverage' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Skip Patch router coverage' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Delete Patch router coverage' })).toBeEnabled();
    expect(screen.getByRole('heading', { name: 'Run verification' })).toBeInTheDocument();
    expect(screen.getByText('tsc failed')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Retry Run verification' })).toBeDisabled();

    fireEvent.click(screen.getByRole('button', { name: 'Add plan step' }));

    expect(screen.getByLabelText('New step title')).toBeInTheDocument();
    expect(screen.getByLabelText('New step tool')).toBeInTheDocument();
    expect(screen.getByLabelText('New step input')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cancel new step' })).toBeInTheDocument();
  });

  it('renders agents route inside the workspace shell', async () => {
    const router = createAppRouter(['/agents']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Agents' })).toBeInTheDocument();
  });

  it('keeps agents route-level policy, run, and tool catalog controls reachable', async () => {
    const router = createAppRouter(['/agents']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(within(workspaceNavigation).getByRole('link', { name: 'Agents' })).toHaveAttribute('href', '/agents');
    expect(await screen.findByRole('heading', { name: 'Agents' })).toBeInTheDocument();
    expect(await screen.findByRole('button', { name: 'Research Agent' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getAllByText('gpt-4o-mini').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('Router-level research automation.')).toBeInTheDocument();
    expect(screen.getByLabelText('Approval mode')).toHaveValue('tiered');
    expect(screen.getByRole('heading', { name: 'Execution limits' })).toBeInTheDocument();
    expect(screen.getByLabelText('Default execution mode')).toHaveValue('planning');
    expect(screen.getByLabelText('Max iterations')).toHaveValue(8);
    expect(screen.getByLabelText('Token budget')).toHaveValue(30000);
    expect(screen.getByLabelText('Long-term memory writes')).toHaveValue('interaction_and_explicit');
    expect(screen.getByLabelText('Long-term memory extraction')).toHaveValue('deterministic');
    expect(screen.getByLabelText('Long-term memory update')).toHaveValue('exact_refresh');
    expect(screen.getByRole('heading', { name: 'Start run' })).toBeInTheDocument();
    expect(screen.getByLabelText('Run conversation ID')).toBeInTheDocument();
    expect(screen.getByLabelText('Run mode')).toHaveValue('planning');
    expect(screen.getByLabelText('Run goal')).toBeInTheDocument();
    expect(screen.getByLabelText('Run max iterations')).toHaveValue(8);
    expect(screen.getByLabelText('Run token budget')).toHaveValue(30000);
    expect(screen.getByRole('button', { name: 'Start run' })).toBeDisabled();
    expect(screen.getByRole('heading', { name: 'Tool approval policy' })).toBeInTheDocument();
    expect(screen.getByText('No tools enabled for this agent.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add custom API tool' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Available tool catalog' })).toBeInTheDocument();
    expect(screen.getByText('Tool definitions load on demand for the selected agent.')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Load tool catalog' }));
    expect(await screen.findByText('web_search')).toBeInTheDocument();
    expect(screen.getByText('builtin / medium / approval required')).toBeInTheDocument();
    expect(screen.getByText('Search the web with tenant policy controls.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Enable tool web_search' })).toBeEnabled();

    fireEvent.change(screen.getByLabelText('Run conversation ID'), { target: { value: 'conv_router_agent' } });
    fireEvent.change(screen.getByLabelText('Run goal'), { target: { value: 'Plan a release readiness review.' } });
    fireEvent.click(screen.getByRole('button', { name: 'Start run' }));

    expect(await screen.findByRole('link', { name: 'Open run plan steps' })).toHaveAttribute(
      'href',
      '/agent-runs/run_router_agent/plan-steps'
    );
  });

  it('executes an agent planning journey from /agents into plan-step operations inside the workspace shell', async () => {
    const router = createAppRouter(['/agents']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(within(workspaceNavigation).getByRole('link', { name: 'Agents' })).toHaveAttribute('href', '/agents');
    expect(await screen.findByRole('heading', { name: 'Agents' })).toBeInTheDocument();
    expect(await screen.findByRole('button', { name: 'Research Agent' })).toHaveAttribute('aria-pressed', 'true');

    fireEvent.change(screen.getByLabelText('Run conversation ID'), { target: { value: 'conv_router_agent' } });
    fireEvent.change(screen.getByLabelText('Run goal'), { target: { value: 'Plan a release readiness review.' } });
    fireEvent.click(screen.getByRole('button', { name: 'Start run' }));

    await waitFor(() => {
      expect(agentsApiMocks.createRun).toHaveBeenCalledWith({
        agentId: 'agent_1',
        conversationId: 'conv_router_agent',
        input: 'Plan a release readiness review.',
        maxIterations: 8,
        mode: 'planning',
        tokenBudget: 30000
      });
    });

    fireEvent.click(await screen.findByRole('link', { name: 'Open run plan steps' }));

    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/agent-runs/run_router_agent/plan-steps');
    });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Agent Plan Steps' })).toBeInTheDocument();
    expect(await screen.findByText('Run run_router_agent')).toBeInTheDocument();
    expect(await screen.findByText('Status: pending_approval')).toBeInTheDocument();
    await waitFor(() => {
      expect(agentPlanStepsApiMocks.getRunDetail).toHaveBeenCalledWith('run_router_agent');
    });

    expect(screen.getByLabelText('Agent run execution controls')).toHaveTextContent('Mode planning');
    expect(screen.getByLabelText('Agent run execution controls')).toHaveTextContent('Iterations 2');
    expect(screen.getByRole('heading', { name: 'Tool Approval Queue' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Inspect release scope' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Patch router coverage' })).toBeInTheDocument();
    expect(screen.getByText('Patch the router coverage after the release scope is known.')).toBeInTheDocument();
    expect(screen.getByText('Depends on: 1')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Operator decision reason for web_search'), {
      target: { value: 'Route-level operator approval.' }
    });
    fireEvent.click(screen.getByRole('button', { name: 'Approve tool web_search' }));

    await waitFor(() => {
      expect(agentPlanStepsApiMocks.approveToolRun).toHaveBeenCalledWith(
        'run_router_agent',
        'tool_search',
        'Route-level operator approval.'
      );
    });
    expect(await screen.findByText('Search approved.')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Approve Patch router coverage' }));

    await waitFor(() => {
      expect(agentPlanStepsApiMocks.approvePlanStep).toHaveBeenCalledWith('run_router_agent', 'step_patch');
    });
    expect(await screen.findByRole('button', { name: 'Execute Patch router coverage' })).toBeEnabled();

    fireEvent.click(screen.getByRole('button', { name: 'Execute Patch router coverage' }));

    await waitFor(() => {
      expect(agentPlanStepsApiMocks.executePlanStep).toHaveBeenCalledWith('run_router_agent', 'step_patch');
    });
    expect(await screen.findByText('Router coverage patched.')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Continue plan' }));

    await waitFor(() => {
      expect(agentPlanStepsApiMocks.continuePlan).toHaveBeenCalledWith('run_router_agent');
    });
    expect(await screen.findByText('Status: completed')).toBeInTheDocument();
    expect(screen.getByLabelText('Agent run execution controls')).toHaveTextContent('Iterations 5');
  });

  it('renders MCP servers route inside the workspace shell', async () => {
    const router = createAppRouter(['/mcp-servers']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'MCP Servers & Tools' })).toBeInTheDocument();
    expect(await screen.findByText('Oblivious Safe Builtins')).toBeInTheDocument();
  });

  it('keeps MCP route-level server and tool controls reachable', async () => {
    const router = createAppRouter(['/mcp-servers']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(within(workspaceNavigation).getByRole('link', { name: 'Agents' })).toHaveAttribute('href', '/agents');
    expect(await screen.findByRole('heading', { name: 'MCP Servers & Tools' })).toBeInTheDocument();
    expect(await screen.findByText('Oblivious Safe Builtins')).toBeInTheDocument();
    expect(await screen.findByLabelText('Local MCP servers')).toBeInTheDocument();
    expect(await screen.findByLabelText('Server name')).toBeInTheDocument();
    expect(screen.getByLabelText('Endpoint URL')).toBeInTheDocument();
    expect(screen.getByLabelText('Auth token')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add MCP server' })).toBeDisabled();
    expect(await screen.findByText('Remote MCP')).toBeInTheDocument();
    expect(screen.getByText('Auth token configured')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Connect' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Disconnect' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Diagnose' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'List tools' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Delete Remote MCP' })).toBeInTheDocument();
  });

  it('keeps settings route-level preferences and MCP controls reachable', async () => {
    const router = createAppRouter(['/settings']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(within(workspaceNavigation).getByRole('link', { name: 'Settings' })).toHaveAttribute('href', '/settings');
    expect(await screen.findByRole('heading', { name: 'Settings' })).toBeInTheDocument();
    expect(screen.getByLabelText('Default mode')).toHaveValue('chat');
    expect(screen.getByLabelText('Model strategy')).toHaveValue('balanced');
    expect(screen.getByLabelText('Enable web suggestions')).not.toBeChecked();
    expect(screen.getByText('Onboarding pending')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save preferences' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Return to chat' })).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Default mode'), { target: { value: 'solo' } });
    fireEvent.change(screen.getByLabelText('Model strategy'), { target: { value: 'cost' } });
    fireEvent.click(screen.getByLabelText('Enable web suggestions'));
    fireEvent.click(screen.getByRole('button', { name: 'Save preferences' }));

    expect(await screen.findByText('Preferences saved.')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'MCP Servers' })).toBeInTheDocument();
    expect(await screen.findByLabelText('Local MCP servers')).toBeInTheDocument();
    expect(await screen.findByText('Oblivious Safe Builtins')).toBeInTheDocument();
    expect(screen.getByLabelText('Server name')).toBeInTheDocument();
    expect(screen.getByLabelText('Endpoint URL')).toBeInTheDocument();
    expect(screen.getByLabelText('Auth token')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add MCP server' })).toBeDisabled();
    expect(await screen.findByText('Remote MCP')).toBeInTheDocument();
    expect(screen.getByText('Auth token configured')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Connect' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Disconnect' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Diagnose' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'List tools' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Delete Remote MCP' })).toBeInTheDocument();
  });

  it('renders onboarding inside the workspace shell', async () => {
    const router = createAppRouter(['/onboarding']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Onboarding' })).toBeInTheDocument();
  });

  it('routes onboarding solo selection into the real solo creation route', async () => {
    const router = createAppRouter(['/onboarding']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('heading', { name: 'Onboarding' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Start with SOLO' }));
    fireEvent.click(screen.getByRole('button', { name: 'Continue to workspace' }));

    expect(await screen.findByRole('heading', { name: 'New SOLO task' })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/solo/new');
    expect(screen.getByText('Define the task boundary before handing execution over to SOLO.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Back to tasks' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Running tasks' })).not.toBeInTheDocument();
  });

  it('renders solo route inside the workspace shell', async () => {
    const router = createAppRouter(['/solo']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'SOLO' })).toBeInTheDocument();
  });

  it('keeps solo route-level task launch and existing task controls reachable', async () => {
    const router = createAppRouter(['/solo']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(within(workspaceNavigation).getByRole('link', { name: 'SOLO' })).toHaveAttribute('href', '/solo');
    expect(await screen.findByRole('heading', { name: 'SOLO' })).toBeInTheDocument();
    expect(screen.getByText('Launch a focused autonomous run with a clear goal, bounded execution mode, and selected workspace knowledge.')).toBeInTheDocument();
    expect(await screen.findByText('Default mode: chat')).toBeInTheDocument();
    expect(screen.getByText('Model strategy: balanced')).toBeInTheDocument();
    expect(screen.getByText('Web suggestions: Disabled')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'New task' })).toBeInTheDocument();
    expect(screen.getByLabelText('Task goal')).toBeInTheDocument();
    expect(screen.getByLabelText('Execution mode')).toHaveValue('standard');
    expect(screen.getByLabelText('Authorization scope')).toHaveValue('workspace_tools');
    expect(screen.getByLabelText('Budget limit')).toHaveValue(10);
    expect(screen.getByLabelText('Allowed tools')).toBeInTheDocument();
    expect(screen.getByLabelText('Blocked tools')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Knowledge sources' })).toBeInTheDocument();
    expect(screen.getByLabelText('Use knowledge base Research Vault')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Running tasks' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Completed tasks' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Stopped tasks' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Open task Watch live rollout' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Open task Review launch plan' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Open task Abort risky task' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Open task Review launch plan' }));

    expect(await screen.findByRole('heading', { name: 'Latest result' })).toBeInTheDocument();
    expect(screen.getByText('Completed a starter SOLO run for: Review launch plan')).toBeInTheDocument();
    expect(screen.getByText('Report')).toBeInTheDocument();
    expect(screen.getByText('solo-result.md')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Retry run' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Continue in Chat' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Export result' })).toBeInTheDocument();
  });

  it('keeps solo new route-level task creation and live execution controls reachable', async () => {
    window.history.replaceState({}, '', '/solo/new');
    const router = createAppRouter(['/solo/new']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(within(workspaceNavigation).getByRole('link', { name: 'SOLO' })).toHaveAttribute('href', '/solo');
    expect(await screen.findByRole('heading', { name: 'New SOLO task' })).toBeInTheDocument();
    expect(screen.getByText('Define the task boundary before handing execution over to SOLO.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Back to tasks' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Running tasks' })).not.toBeInTheDocument();

    await screen.findByLabelText('Use knowledge base Research Vault');
    fireEvent.change(screen.getByLabelText('Task goal'), { target: { value: 'Draft launch checklist' } });
    fireEvent.change(screen.getByLabelText('Execution mode'), { target: { value: 'safe' } });
    fireEvent.change(screen.getByLabelText('Authorization scope'), { target: { value: 'full_access' } });
    fireEvent.change(screen.getByLabelText('Budget limit'), { target: { value: '20' } });
    fireEvent.change(screen.getByLabelText('Allowed tools'), { target: { value: 'browser, shell' } });
    fireEvent.change(screen.getByLabelText('Blocked tools'), { target: { value: 'email' } });
    fireEvent.click(screen.getByLabelText('Use knowledge base Research Vault'));
    fireEvent.click(screen.getByRole('button', { name: 'Start solo run' }));

    expect(await screen.findByRole('heading', { name: 'Execution view' })).toBeInTheDocument();
    expect(screen.getByText('Status: running')).toBeInTheDocument();
    expect(screen.getByText('Execution mode: safe')).toBeInTheDocument();
    expect(screen.getByText('Authorization scope: full_access')).toBeInTheDocument();
    expect(screen.getByText('Budget consumed: 6 / 20')).toBeInTheDocument();
    expect(screen.getByText('Current step: Review workspace context')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Current knowledge sources' })).toBeInTheDocument();
    expect(screen.getByText('Research Vault')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Current enabled tools' })).toBeInTheDocument();
    expect(screen.getByText('browser')).toBeInTheDocument();
    expect(screen.getByText('shell')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Blocked tools' })).toBeInTheDocument();
    expect(screen.getByText('email')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Execution timeline' })).toBeInTheDocument();
    expect(screen.getByText('Task execution started')).toBeInTheDocument();
    expect(screen.getByText('Executing Review workspace context')).toBeInTheDocument();
    expect(screen.getByText('Understand the goal')).toBeInTheDocument();
    expect(screen.getByText('Deliver starter result')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Continue run' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Pause run' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cancel run' })).toBeInTheDocument();
    expect(screen.getByLabelText('Active budget limit')).toHaveValue(20);
    expect(screen.getByRole('button', { name: 'Update budget' })).toBeEnabled();
  });

  it('renders workflows route inside the workspace shell', async () => {
    const router = createAppRouter(['/workflows']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Workflows' })).toBeInTheDocument();
  });

  it('keeps workflows route-level trigger, editor, branch, and execution controls reachable', async () => {
    const router = createAppRouter(['/workflows']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(within(workspaceNavigation).getByRole('link', { name: 'Scheduled Tasks' })).toHaveAttribute(
      'href',
      '/scheduled-tasks'
    );
    expect(await screen.findByRole('heading', { name: 'Workflows' })).toBeInTheDocument();
    expect(screen.getByLabelText('Create workflow')).toBeInTheDocument();
    expect(screen.getByLabelText('Workflow name')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Create draft workflow' })).toBeDisabled();
    expect(screen.getByLabelText('Conversation trigger matching')).toBeInTheDocument();
    expect(screen.getByLabelText('Semantic trigger matching')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Conversation match ID'), { target: { value: 'conversation_router' } });
    fireEvent.click(screen.getByRole('button', { name: 'Check conversation matches' }));
    expect(await screen.findByLabelText('Conversation trigger match results')).toBeInTheDocument();
    expect(await screen.findByText('conversation-main | conversation_router')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Semantic match message'), { target: { value: 'sev1 incident' } });
    fireEvent.click(screen.getByRole('button', { name: 'Check semantic matches' }));
    expect(await screen.findByLabelText('Semantic trigger match results')).toBeInTheDocument();
    expect(await screen.findByText('urgent-ticket | sev1 | score 0.91 | threshold 0.86 | semantic')).toBeInTheDocument();

    expect(await screen.findByText('Incident triage workflow')).toBeInTheDocument();
    expect(screen.getByText('Route-level incident workflow.')).toBeInTheDocument();
    expect(screen.getByText('Status: draft')).toBeInTheDocument();
    expect(screen.getByText('Version: 3')).toBeInTheDocument();
    expect(screen.getByText('Nodes: 3')).toBeInTheDocument();
    expect(screen.getByLabelText('Run input JSON for Incident triage workflow')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Run Incident triage workflow' })).toBeEnabled();
    expect(screen.getByLabelText('Webhook payload JSON for Incident triage workflow')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Trigger webhook for Incident triage workflow' })).toBeEnabled();
    expect(screen.getByLabelText('Variables Incident triage workflow')).toBeInTheDocument();
    expect(screen.getByLabelText('Triggers Incident triage workflow')).toBeInTheDocument();
    expect(screen.getByText('Conversation: conversation_router')).toBeInTheDocument();
    expect(screen.getByText('Schedule: */15 * * * *')).toBeInTheDocument();
    expect(screen.getByText('Semantic: urgent-ticket incident, sev1 threshold 0.86')).toBeInTheDocument();
    expect(
      screen.getByText('Webhook: github /api/v1/workflows/webhooks/org_router/workflow_1 secret configured')
    ).toBeInTheDocument();
    expect(screen.getByLabelText('Conversation trigger ID for Incident triage workflow')).toHaveValue(
      'conversation-main'
    );
    expect(screen.getByLabelText('Schedule cron for Incident triage workflow')).toHaveValue('*/15 * * * *');
    expect(screen.getByLabelText('Semantic threshold for Incident triage workflow')).toHaveValue('0.86');
    expect(screen.getByLabelText('Signed webhook helper for Incident triage workflow')).toBeInTheDocument();
    expect(screen.getByText('/api/v1/workflows/webhooks/org_router/workflow_1')).toBeInTheDocument();
    expect(screen.getByLabelText('Scheduled tasks for Incident triage workflow')).toBeInTheDocument();
    expect(screen.getByText('schedule_1')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Run scheduled task schedule_1 now' })).toBeEnabled();

    expect(screen.getByLabelText('Visual editor for Incident triage workflow')).toBeInTheDocument();
    expect(screen.getByLabelText('Node palette for Incident triage workflow')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add LLM node template to Incident triage workflow' })).toBeInTheDocument();
    expect(screen.getByLabelText('React Flow canvas for Incident triage workflow')).toBeInTheDocument();
    expect(screen.getByLabelText('React Flow router mock')).toHaveAttribute('data-snap-to-grid', 'true');
    expect(screen.getByLabelText('Canvas node 2 classify llm at 320 80')).toBeInTheDocument();
    expect(screen.getByLabelText('Canvas edge classify to notify-team branch true')).toBeInTheDocument();
    expect(screen.getByLabelText('Node sequence for Incident triage workflow')).toBeInTheDocument();
    expect(screen.getByLabelText('Edges for Incident triage workflow')).toBeInTheDocument();
    expect(screen.getByLabelText('Resource policy Incident triage workflow')).toBeInTheDocument();
    expect(screen.getByLabelText('Max concurrent executions for Incident triage workflow')).toHaveValue(2);
    expect(screen.getByLabelText('Max tokens budget for Incident triage workflow')).toHaveValue(12000);
    expect(screen.getByLabelText('Add node to Incident triage workflow')).toBeInTheDocument();
    expect(screen.getByLabelText('Add edge to Incident triage workflow')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Load versions for Incident triage workflow' }));
    expect(await screen.findByText('Version 2')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Rollback Incident triage workflow to version 2' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Create branch from Incident triage workflow version 2' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Publish branch Incident triage workflow v2 branch' })).toBeEnabled();
    expect(
      screen.getByRole('button', { name: 'Merge branch Incident triage workflow v2 branch into Incident triage workflow' })
    ).toBeEnabled();

    fireEvent.click(screen.getByRole('button', { name: 'Create branch from Incident triage workflow version 2' }));
    expect(await screen.findByLabelText('Branch Incident triage workflow version 2')).toBeInTheDocument();
    expect(screen.getByLabelText('Branch name for Incident triage workflow version 2')).toHaveValue(
      'Incident triage workflow v2 branch'
    );
    expect(screen.getByRole('button', { name: 'Submit branch for Incident triage workflow version 2' })).toBeEnabled();

    expect(screen.getByLabelText('Debug Incident triage workflow')).toBeInTheDocument();
    expect(screen.getByText('Known nodes: manual-start, classify, notify-team')).toBeInTheDocument();
    expect(screen.getByLabelText('Node ID')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Test node' })).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Node ID'), { target: { value: 'classify' } });
    fireEvent.click(screen.getByRole('button', { name: 'Test node' }));
    expect(await screen.findByText('Node classify returned succeeded')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Load executions' }));
    expect(await screen.findByText('exec_router_paused')).toBeInTheDocument();
    expect(screen.getByLabelText('Workflow execution exec_router_paused status')).toHaveTextContent('Paused');
    expect(screen.getByRole('button', { name: 'Pause exec_router_paused' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Resume exec_router_paused' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Cancel exec_router_paused' })).toBeEnabled();
    expect(screen.getByLabelText('Debug and performance summary for exec_router_paused')).toBeInTheDocument();
    expect(screen.getByLabelText('Paused input for exec_router_paused')).toBeInTheDocument();
    expect(screen.getByLabelText('Paused failure decisions for exec_router_paused')).toBeInTheDocument();
    expect(screen.getByLabelText('Resource limits for exec_router_paused')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'View details for exec_router_paused' }));
    const executionDebugDetails = await screen.findByLabelText('Execution debug details for exec_router_paused');
    expect(within(executionDebugDetails).getByText('manual-start -> classify')).toBeInTheDocument();
    expect(within(executionDebugDetails).getByText('Bottleneck: classify (3400ms)')).toBeInTheDocument();
  }, 15_000);

  it('renders scheduled tasks route inside the workspace shell', async () => {
    const router = createAppRouter(['/scheduled-tasks']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Scheduled Tasks' })).toBeInTheDocument();
  });

  it('keeps scheduled tasks route-level creation and run controls reachable', async () => {
    const router = createAppRouter(['/scheduled-tasks']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(within(workspaceNavigation).getByRole('link', { name: 'Workflows' })).toHaveAttribute('href', '/workflows');
    expect(await screen.findByRole('heading', { name: 'Scheduled Tasks' })).toBeInTheDocument();
    expect(await screen.findByLabelText('Create scheduled task')).toBeInTheDocument();
    expect(screen.getByLabelText('Schedule name')).toBeInTheDocument();
    expect(screen.getByLabelText('Target type')).toHaveValue('workflow');
    expect(screen.getByLabelText('Target ID')).toBeInTheDocument();
    expect(screen.getByLabelText('Cron expression')).toBeInTheDocument();
    expect(screen.getByLabelText('Enabled')).toBeChecked();
    expect(screen.getByRole('button', { name: 'Create schedule' })).toBeDisabled();
    expect(await screen.findByLabelText('Scheduled task list')).toBeInTheDocument();
    expect(await screen.findByText('Weekly workflow schedule')).toBeInTheDocument();
    expect(screen.getByText('workflow_1')).toBeInTheDocument();
    expect(screen.getByText('0 9 * * 1')).toBeInTheDocument();
    expect(screen.getByText('Next: 2026-06-09 09:00')).toBeInTheDocument();
    expect(screen.getByText('Last: 2026-06-02 09:00')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Disable workflow_1 schedule' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Run workflow_1 schedule now' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Show recent runs for workflow_1' })).toHaveAttribute(
      'aria-expanded',
      'false'
    );

    fireEvent.click(screen.getByRole('button', { name: 'Show recent runs for workflow_1' }));

    expect(await screen.findByLabelText('Recent runs for workflow_1')).toBeInTheDocument();
    expect(await screen.findByText('run_schedule_router_list')).toBeInTheDocument();
    expect(screen.getByText('Started: 2026-06-09 09:00')).toBeInTheDocument();
    expect(screen.getByText('Finished: 2026-06-09 09:02')).toBeInTheDocument();
    expect(screen.getByText('Error: None')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Refresh recent runs for workflow_1' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Hide recent runs for workflow_1' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Run workflow_1 schedule now' }));

    expect(await screen.findByText('run_schedule_router_now')).toBeInTheDocument();
    expect(screen.getByText('Started: 2026-06-09 10:00')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Disable workflow_1 schedule' }));

    expect(await screen.findByText('Disabled')).toBeInTheDocument();
  });

  it('renders publishing channels route inside the workspace shell', async () => {
    const router = createAppRouter(['/publishing']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Publishing Channels' })).toBeInTheDocument();
  });

  it('keeps publishing route-level delivery and failed-queue recovery controls reachable', async () => {
    const router = createAppRouter(['/publishing']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(within(workspaceNavigation).getByRole('link', { name: 'Workflows' })).toHaveAttribute('href', '/workflows');
    expect(await screen.findByRole('heading', { name: 'Publishing Channels' })).toBeInTheDocument();
    expect(await screen.findByLabelText('Create publishing channel')).toBeInTheDocument();
    expect(screen.getByLabelText('Channel name')).toBeInTheDocument();
    expect(screen.getByLabelText('Channel type')).toHaveValue('webhook');
    expect(screen.getByLabelText('Endpoint URL')).toBeInTheDocument();
    expect(screen.getByLabelText('Shared secret')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Create channel' })).toBeDisabled();
    expect(await screen.findByLabelText('Publishing channel send test')).toBeInTheDocument();
    expect(screen.getByLabelText('Channel')).toHaveValue('channel_1');
    expect(screen.getByLabelText('Conversation ID')).toBeInTheDocument();
    expect(screen.getByLabelText('Message text')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Send message' })).toBeDisabled();
    expect(await screen.findByLabelText('Publishing channel message visibility')).toBeInTheDocument();
    expect(await screen.findByText('1 recent / 1 failed')).toBeInTheDocument();
    expect(screen.getAllByText('log_router_recent').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('log_router_failed').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('adapter timeout')).toBeInTheDocument();
    expect(screen.getByLabelText('Failed retry queue controls')).toBeInTheDocument();
    expect(screen.getByLabelText('Fallback channel')).toHaveValue('');
    expect(screen.getByLabelText('Retry limit')).toHaveValue(null);
    expect(screen.getByRole('button', { name: 'Retry failed messages' })).toBeEnabled();
    expect(screen.getByLabelText('Publishing channel list')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Test channel Ops Webhook' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Disable Ops Webhook' })).toBeInTheDocument();
  });

  it('keeps publishing route-level channel edit and delete workflows reachable', async () => {
    const router = createAppRouter(['/publishing']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const list = await screen.findByLabelText('Publishing channel list');
    expect(await within(list).findByRole('heading', { name: 'Ops Webhook' })).toBeInTheDocument();

    fireEvent.click(within(list).getByRole('button', { name: 'Edit Ops Webhook' }));
    const editForm = await screen.findByLabelText('Edit Ops Webhook');
    fireEvent.change(within(editForm).getByLabelText('Channel name'), { target: { value: 'Ops Webhook Renamed' } });
    fireEvent.change(within(editForm).getByLabelText('Endpoint URL'), { target: { value: 'https://hooks.example/ops-renamed' } });
    fireEvent.click(within(editForm).getByRole('button', { name: 'Save channel' }));

    expect(await within(list).findByRole('heading', { name: 'Ops Webhook Renamed' })).toBeInTheDocument();
    expect(await screen.findByText('Channel updated.')).toBeInTheDocument();

    fireEvent.click(within(list).getByRole('button', { name: 'Delete Ops Webhook Renamed' }));

    expect(await screen.findByText('Channel disabled.')).toBeInTheDocument();
    expect(within(list).getByText('Disabled')).toBeInTheDocument();
  });

  it('renders billing route inside the console shell', async () => {
    const router = createAppRouter(['/console/billing']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Console')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Billing' })).toBeInTheDocument();
  });

  it('keeps console billing route-level landmarks and top-up controls reachable', async () => {
    const router = createAppRouter(['/console/billing']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const consoleNavigation = await screen.findByRole('navigation', { name: 'Console navigation' });
    expect(document.querySelector('[data-gsap-scope="console"]')).toBeInTheDocument();
    expect(within(consoleNavigation).getByRole('link', { name: 'Usage' })).toHaveAttribute('href', '/console/usage');
    expect(await screen.findByRole('heading', { name: 'Billing' })).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Back to overview' })).toHaveAttribute('href', '/console');
    expect(await screen.findByRole('link', { name: 'Open usage' })).toHaveAttribute('href', '/console/usage');
    expect(await screen.findByRole('heading', { name: 'Subscription packages' })).toBeInTheDocument();
    expect(screen.getByLabelText('Package')).toHaveValue('pkg_router_pro');
    expect(screen.getByRole('option', { name: 'Router Pro - $29.00 - 30 days' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Start subscription checkout' })).toBeEnabled();
    expect(await screen.findByLabelText('Top-up amount USD')).toHaveValue(25);
    expect(await screen.findByLabelText('Payment provider')).toHaveValue('stripe');
    expect(screen.getAllByRole('option', { name: 'Alipay' })).toHaveLength(2);
    expect(screen.getByRole('button', { name: 'Start top-up checkout' })).toBeEnabled();
    expect(screen.getByText('Invoice history')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'View invoice' })).toHaveAttribute(
      'href',
      'https://billing.stripe.test/invoices/invoice_paid'
    );
    expect(screen.getByRole('link', { name: 'Download PDF' })).toHaveAttribute(
      'href',
      'https://billing.stripe.test/invoices/invoice_paid.pdf'
    );
  });

  it('keeps console home route-level KPI and drill-down controls reachable', async () => {
    const router = createAppRouter(['/console']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const consoleNavigation = await screen.findByRole('navigation', { name: 'Console navigation' });
    expect(document.querySelector('[data-gsap-scope="console"]')).toBeInTheDocument();
    expect(document.querySelector('[data-gsap-scope="console-home"]')).toBeInTheDocument();
    expect(within(consoleNavigation).getByRole('link', { name: 'Overview' })).toHaveAttribute('href', '/console');
    expect(within(consoleNavigation).getByRole('link', { name: 'Notifications' })).toHaveAttribute(
      'href',
      '/console/notifications'
    );
    expect(await screen.findByRole('heading', { name: 'Console Home' })).toBeInTheDocument();
    expect(await screen.findByText('Current workspace scope: workspace_1')).toBeInTheDocument();
    expect(await screen.findByRole('region', { name: 'Key performance indicators' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Estimated cost' })).toHaveAttribute('href', '/console/billing');
    expect(screen.getByText('$0.0004')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Requests' })).toHaveAttribute('href', '/console/usage');
    expect(screen.getByRole('link', { name: 'Top model' })).toHaveAttribute('href', '/console/models');
    expect(screen.getByText('balanced-chat')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Access posture' })).toHaveAttribute('href', '/console/access');
    expect(screen.getByText('Session session_1')).toBeInTheDocument();
    expect(screen.getByRole('region', { name: 'Cost and usage focus' })).toBeInTheDocument();
    expect(screen.getByText('Billing requests: 5')).toBeInTheDocument();
    expect(screen.getByText('Usage requests: 5')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open billing drill-down' })).toHaveAttribute('href', '/console/billing');
    expect(screen.getByRole('link', { name: 'Open usage drill-down' })).toHaveAttribute('href', '/console/usage');
    expect(screen.getByRole('region', { name: 'Supporting summaries' })).toBeInTheDocument();
    expect(screen.getByText('Active user: user@example.com')).toBeInTheDocument();
    expect(screen.getByText('Network access hint enabled')).toBeInTheDocument();
  });

  it('renders notifications route inside the console shell', async () => {
    const router = createAppRouter(['/console/notifications']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Console')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Notifications' })).toBeInTheDocument();
  });

  it('keeps console notifications route-level alert review controls reachable', async () => {
    const router = createAppRouter(['/console/notifications']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const consoleNavigation = await screen.findByRole('navigation', { name: 'Console navigation' });
    expect(document.querySelector('[data-gsap-scope="console"]')).toBeInTheDocument();
    expect(within(consoleNavigation).getByRole('link', { name: 'Notifications' })).toHaveAttribute(
      'href',
      '/console/notifications'
    );
    expect(within(consoleNavigation).getByRole('link', { name: 'Billing' })).toHaveAttribute('href', '/console/billing');
    expect(await screen.findByRole('heading', { name: 'Notifications' })).toBeInTheDocument();
    expect(screen.getByText('Review in-app alerts routed from workspace and system events.')).toBeInTheDocument();
    expect(await screen.findByText('1 total')).toBeInTheDocument();
    expect(screen.getByText('1 unread')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Database down' })).toBeInTheDocument();
    expect(screen.getByText('Database connection failed')).toBeInTheDocument();
    expect(screen.getByText('critical')).toBeInTheDocument();
    expect(screen.getByText('Unread')).toBeInTheDocument();
    expect(screen.getByText('system')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Mark Database down as read' })).toBeEnabled();
  });

  it('keeps console models route-level summary evidence reachable', async () => {
    const router = createAppRouter(['/console/models']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const consoleNavigation = await screen.findByRole('navigation', { name: 'Console navigation' });
    expect(document.querySelector('[data-gsap-scope="console"]')).toBeInTheDocument();
    expect(within(consoleNavigation).getByRole('link', { name: 'Models' })).toHaveAttribute('href', '/console/models');
    expect(within(consoleNavigation).getByRole('link', { name: 'Access' })).toHaveAttribute('href', '/console/access');
    expect(await screen.findByRole('heading', { name: 'Models' })).toBeInTheDocument();
    expect(await screen.findByText('Review the current workspace model mix and relative request volume.')).toBeInTheDocument();
    expect(screen.getAllByText('Current workspace scope').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Default mode: chat').length).toBeGreaterThanOrEqual(1);
    expect(await screen.findByRole('navigation', { name: 'Models sibling navigation' })).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Open access' })).toHaveAttribute('href', '/console/access');
    expect(await screen.findByText('balanced-chat')).toBeInTheDocument();
    expect(screen.getByText('Requests: 2')).toBeInTheDocument();
  });

  it('keeps console access route-level API token controls reachable', async () => {
    const router = createAppRouter(['/console/access']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const consoleNavigation = await screen.findByRole('navigation', { name: 'Console navigation' });
    expect(document.querySelector('[data-gsap-scope="console"]')).toBeInTheDocument();
    expect(within(consoleNavigation).getByRole('link', { name: 'Access' })).toHaveAttribute('href', '/console/access');
    expect(await screen.findByRole('heading', { name: 'Access' })).toBeInTheDocument();
    expect(await screen.findByRole('navigation', { name: 'Access sibling navigation' })).toBeInTheDocument();
    expect(await screen.findByText('API tokens')).toBeInTheDocument();
    expect(await screen.findByText('Router gateway key')).toBeInTheDocument();
    expect(await screen.findByLabelText('Token name')).toBeInTheDocument();
    expect(screen.getByLabelText('Allowed models')).toHaveValue('gpt-4o,gpt-4o-mini');
    expect(screen.getByRole('button', { name: 'Create API token' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'View usage for Router gateway key' })).toBeInTheDocument();
  });

  it('keeps console usage route-level analytics and recent relay evidence reachable', async () => {
    const router = createAppRouter(['/console/usage']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const consoleNavigation = await screen.findByRole('navigation', { name: 'Console navigation' });
    expect(document.querySelector('[data-gsap-scope="console"]')).toBeInTheDocument();
    expect(within(consoleNavigation).getByRole('link', { name: 'Usage' })).toHaveAttribute('href', '/console/usage');
    expect(await screen.findByRole('heading', { name: 'Usage' })).toBeInTheDocument();
    expect(await screen.findByRole('navigation', { name: 'Usage sibling navigation' })).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'By model' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'By feature' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Top users' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Daily trend' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Recent relay requests' })).toBeInTheDocument();
    expect(screen.getAllByText('gpt-4o').length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText('workflow')).toBeInTheDocument();
    expect(screen.getByText('user_router')).toBeInTheDocument();
    expect(screen.getByText('req_router_console')).toBeInTheDocument();
    expect(screen.queryByText('openai / ch_router_1')).not.toBeInTheDocument();
  });

  it('keeps admin dashboard route-level metrics and commercial operation links reachable', async () => {
    const router = createAppRouter(['/admin']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const adminNavigation = await screen.findByRole('complementary', { name: 'Admin navigation' });
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
    expect(within(adminNavigation).getByRole('link', { name: 'Dashboard' })).toHaveAttribute('href', '/admin');
    expect(await screen.findByRole('heading', { name: 'Dashboard' })).toBeInTheDocument();
    expect(screen.getAllByText('Channels').length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText('Total Users')).toBeInTheDocument();
    expect(screen.getByText('API Calls (24h)')).toBeInTheDocument();
    expect(screen.getByText('Active Agents')).toBeInTheDocument();
    expect(screen.getByText('API Call Volume (7 days)')).toBeInTheDocument();
    expect(screen.getByText('Channel Uptime')).toBeInTheDocument();

    const commercialOperations = screen.getByRole('region', { name: 'Commercial operations' });
    expect(within(commercialOperations).getByRole('link', { name: /Channels/ })).toHaveAttribute('href', '/admin/channels');
    expect(within(commercialOperations).getByRole('link', { name: /Routes/ })).toHaveAttribute('href', '/admin/routes');
    expect(within(commercialOperations).getByRole('link', { name: /Plans/ })).toHaveAttribute('href', '/admin/plans');
    expect(within(commercialOperations).getByRole('link', { name: /Billing/ })).toHaveAttribute('href', '/admin/billing');
    expect(within(commercialOperations).getByRole('link', { name: /Users/ })).toHaveAttribute('href', '/admin/users');
    expect(within(commercialOperations).getByRole('link', { name: /Audit Log/ })).toHaveAttribute('href', '/admin/audit-log');
    expect(within(commercialOperations).getByRole('link', { name: /Alerts/ })).toHaveAttribute('href', '/admin/alerts');
    expect(within(commercialOperations).getByRole('link', { name: /Review Queue/ })).toHaveAttribute(
      'href',
      '/admin/reviews'
    );
  });

  it('renders admin billing route inside the admin shell', async () => {
    const router = createAppRouter(['/admin/billing']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('complementary', { name: 'Admin navigation' })).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Billing' })).toBeInTheDocument();
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
  });

  it('keeps admin billing route-level recovery controls and filters reachable', async () => {
    const router = createAppRouter(['/admin/billing']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('complementary', { name: 'Admin navigation' })).toBeInTheDocument();
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Billing' })).toBeInTheDocument();
    expect(await screen.findByText('bs_router_phase28')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Review failed webhooks' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Payment Intents' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Payouts' })).toBeInTheDocument();
    expect(screen.getByLabelText('Organization ID filter')).toBeInTheDocument();
    expect(screen.getByLabelText('Provider filter')).toBeInTheDocument();
  });

  it('executes admin billing payout operator actions inside the admin shell', async () => {
    const router = createAppRouter(['/admin/billing']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('heading', { name: 'Billing' })).toBeInTheDocument();
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('tab', { name: 'Payouts' }));
    expect(await screen.findByText('payout_router_pending')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Create due payouts' }));
    await waitFor(() => expect(adminApiMocks.createDueMarketplacePayouts).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(adminApiMocks.listBillingSurface).toHaveBeenLastCalledWith('payouts', expect.objectContaining({ limit: 50 })));

    fireEvent.click(screen.getByRole('button', { name: 'Mark payout payout_router_pending paid' }));
    expect(await screen.findByRole('heading', { name: 'Payout confirmation' })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Provider payout ID'), { target: { value: 'provider-router-paid' } });
    fireEvent.click(screen.getByRole('button', { name: 'Confirm paid payout' }));
    await waitFor(() => expect(adminApiMocks.markMarketplacePayoutPaid).toHaveBeenCalledWith('payout_router_pending', 'provider-router-paid'));

    fireEvent.click(await screen.findByRole('button', { name: 'Mark payout payout_router_pending failed' }));
    expect(await screen.findByRole('heading', { name: 'Payout failure' })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Provider payout ID'), { target: { value: 'provider-router-failed' } });
    fireEvent.change(screen.getByLabelText('Failure reason'), { target: { value: 'bank account closed' } });
    fireEvent.click(screen.getByRole('button', { name: 'Confirm failed payout' }));
    await waitFor(() =>
      expect(adminApiMocks.markMarketplacePayoutFailed).toHaveBeenCalledWith(
        'payout_router_pending',
        'provider-router-failed',
        'bank account closed'
      )
    );
  });

  it('executes admin billing top-up refund recovery inside the admin shell', async () => {
    const router = createAppRouter(['/admin/billing']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('heading', { name: 'Billing' })).toBeInTheDocument();
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('tab', { name: 'Top-ups' }));
    expect(await screen.findByText('topup_router_refund')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Record refund for top-up topup_router_refund' }));

    expect(await screen.findByRole('heading', { name: 'Top-up refund' })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Provider refund ID'), { target: { value: 're_router_topup' } });
    fireEvent.change(screen.getByLabelText('Provider charge ID'), { target: { value: 'ch_router_topup' } });
    fireEvent.change(screen.getByLabelText('Refund amount'), { target: { value: '12.5' } });
    fireEvent.change(screen.getByLabelText('Reason'), { target: { value: 'duplicate provider capture' } });
    fireEvent.click(screen.getByRole('button', { name: 'Confirm top-up refund' }));

    await waitFor(() =>
      expect(adminApiMocks.refundTopup).toHaveBeenCalledWith('topup_router_refund', {
        provider: 'stripe',
        providerRefundID: 're_router_topup',
        providerChargeID: 'ch_router_topup',
        providerPaymentIntentID: 'pi_provider_router_topup',
        amount: 12.5,
        currency: 'usd',
        reason: 'duplicate provider capture'
      })
    );
    await waitFor(() => expect(adminApiMocks.listBillingSurface).toHaveBeenLastCalledWith('topups', expect.objectContaining({ limit: 50 })));
  });

  it('keeps admin channels route-level provider, runtime, and operator controls reachable', async () => {
    const router = createAppRouter(['/admin/channels']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const adminNavigation = await screen.findByRole('complementary', { name: 'Admin navigation' });
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
    expect(within(adminNavigation).getByRole('link', { name: 'Channels' })).toHaveAttribute('href', '/admin/channels');
    expect(await screen.findByRole('heading', { name: 'Channels' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add Channel' })).toBeEnabled();
    expect(screen.getByPlaceholderText('Search channels...')).toBeInTheDocument();
    expect(screen.getByLabelText('Provider filter')).toHaveValue('all');
    expect(await screen.findByRole('option', { name: 'OpenAI' })).toBeInTheDocument();
    expect(screen.getAllByText('OpenAI Primary').length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText('openai')).toBeInTheDocument();
    expect(screen.getByText('gpt-4o')).toBeInTheDocument();
    expect(screen.getByText('gpt-4o-mini')).toBeInTheDocument();
    expect(screen.getByText('0.0200')).toBeInTheDocument();
    expect(screen.getByText('24 RPM')).toBeInTheDocument();
    expect(screen.getByText('4,800 TPM')).toBeInTheDocument();
    expect(screen.getAllByText('132ms avg').length).toBeGreaterThanOrEqual(2);
    const runtimeDiagnostics = await screen.findByLabelText('Runtime diagnostics');
    expect(within(runtimeDiagnostics).getByText('240 requests')).toBeInTheDocument();
    expect(within(runtimeDiagnostics).getByText('230 ok / 10 failed')).toBeInTheDocument();
    expect(within(runtimeDiagnostics).getByText('7 active')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Edit channel OpenAI Primary' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Delete channel OpenAI Primary' })).toBeEnabled();
    fireEvent.click(screen.getByRole('button', { name: 'Test connection for OpenAI Primary' }));
    const channelDiagnostics = await screen.findByLabelText('OpenAI Primary diagnostics');
    expect(within(channelDiagnostics).getByText('USD 12.50')).toBeInTheDocument();
    expect(within(channelDiagnostics).getByText('Health online')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Detect model updates for OpenAI Primary' }));
    const modelUpdates = await screen.findByLabelText('Model updates for OpenAI Primary');
    expect(within(modelUpdates).getByText(/3\s+upstream models checked/)).toBeInTheDocument();
    expect(within(modelUpdates).getByText('Added 2')).toBeInTheDocument();
    expect(within(modelUpdates).getByText('gpt-4o-mini')).toBeInTheDocument();
    expect(within(modelUpdates).getByRole('button', { name: 'Apply model updates for OpenAI Primary' })).toBeEnabled();
  });

  it('keeps admin routes route-level weighted strategy and target controls reachable', async () => {
    const router = createAppRouter(['/admin/routes']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const adminNavigation = await screen.findByRole('complementary', { name: 'Admin navigation' });
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
    expect(within(adminNavigation).getByRole('link', { name: 'Model Routes' })).toHaveAttribute('href', '/admin/routes');
    expect(await screen.findByRole('heading', { name: 'Model Routes' })).toBeInTheDocument();
    expect(await screen.findByText('gpt-4o*')).toBeInTheDocument();
    expect(screen.getByText('weighted')).toBeInTheDocument();
    expect(screen.getByText('OpenAI Primary')).toBeInTheDocument();
    expect(screen.getByText('p1 w80')).toBeInTheDocument();
    expect(screen.getByText('Active')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Edit route gpt-4o*' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Delete route gpt-4o*' })).toBeEnabled();
    fireEvent.click(screen.getByRole('button', { name: 'Add Route' }));
    expect(await screen.findByRole('heading', { name: 'Add Route' })).toBeInTheDocument();
    expect(screen.getByLabelText('Model Pattern')).toBeInTheDocument();
    expect(screen.getByLabelText('Strategy')).toHaveTextContent('Adaptive');
    expect(screen.getByLabelText('Strategy')).toHaveTextContent('Weighted');
    expect(screen.getByLabelText('Strategy')).toHaveTextContent('Priority');
    expect(screen.getByLabelText('Strategy')).toHaveTextContent('Cost aware');
    expect(screen.getByLabelText('Target Channel 1')).toHaveTextContent('OpenAI Primary');
    expect(screen.getByLabelText('Priority 1')).toHaveValue(1);
    expect(screen.getByLabelText('Weight 1')).toHaveValue(100);
    expect(screen.getByText('Enabled 1')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Create Route' })).toBeEnabled();
  });

  it('keeps admin plans route-level quota, access, and request-cap controls reachable', async () => {
    const router = createAppRouter(['/admin/plans']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const adminNavigation = await screen.findByRole('complementary', { name: 'Admin navigation' });
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
    expect(within(adminNavigation).getByRole('link', { name: 'Plans' })).toHaveAttribute('href', '/admin/plans');
    expect(await screen.findByRole('heading', { name: 'Plans' })).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Search plans...')).toBeInTheDocument();
    expect(screen.getByLabelText('Plan status filter')).toHaveValue('all');
    expect(screen.getByLabelText('Plan visibility filter')).toHaveValue('all');
    expect(screen.getByRole('button', { name: 'Add Plan' })).toBeEnabled();

    const planRow = (await screen.findByText('$29.00')).closest('tr');
    expect(planRow).not.toBeNull();
    expect(within(planRow as HTMLElement).getByText('Pro')).toBeInTheDocument();
    expect(within(planRow as HTMLElement).getByText('500 credits / 1,000,000 tokens')).toBeInTheDocument();
    expect(within(planRow as HTMLElement).getByText('32,000 tokens')).toBeInTheDocument();
    expect(within(planRow as HTMLElement).getByText('gpt-4o')).toBeInTheDocument();
    expect(within(planRow as HTMLElement).getByText('claude-3.5')).toBeInTheDocument();
    expect(within(planRow as HTMLElement).getByText('42')).toBeInTheDocument();
    expect(within(planRow as HTMLElement).getByText('Public')).toBeInTheDocument();
    expect(within(planRow as HTMLElement).getByText('Active')).toBeInTheDocument();
    expect(within(planRow as HTMLElement).getByRole('button', { name: 'Edit plan Pro' })).toBeEnabled();
    expect(within(planRow as HTMLElement).getByRole('button', { name: 'Deactivate plan Pro' })).toBeEnabled();

    fireEvent.click(screen.getByRole('button', { name: 'Add Plan' }));
    expect(await screen.findByRole('heading', { name: 'Add Plan' })).toBeInTheDocument();
    expect(screen.getByLabelText('Name')).toBeInTheDocument();
    expect(screen.getByLabelText('Description')).toBeInTheDocument();
    expect(screen.getByLabelText('Price')).toBeInTheDocument();
    expect(screen.getByLabelText('Quota Amount')).toBeInTheDocument();
    expect(screen.getByLabelText('Token Quota')).toBeInTheDocument();
    expect(screen.getByLabelText('Agent Limit')).toBeInTheDocument();
    expect(screen.getByLabelText('Request Token Cap')).toBeInTheDocument();
    expect(screen.getByLabelText('Model Access')).toBeInTheDocument();
    expect(screen.getByLabelText('Sort Order')).toBeInTheDocument();
    expect(screen.getByLabelText('Public plan')).toBeChecked();
    expect(screen.getByRole('button', { name: 'Create Plan' })).toBeEnabled();
  });

  it('keeps admin users route-level role, status, plan, and usage controls reachable', async () => {
    const router = createAppRouter(['/admin/users']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const adminNavigation = await screen.findByRole('complementary', { name: 'Admin navigation' });
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
    expect(within(adminNavigation).getByRole('link', { name: 'Users' })).toHaveAttribute('href', '/admin/users');
    expect(await screen.findByRole('heading', { name: 'Users' })).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Search users...')).toBeInTheDocument();
    expect(screen.getByLabelText('Role filter')).toHaveValue('all');
    expect(screen.getByLabelText('User status filter')).toHaveValue('all');

    const userRow = (await screen.findByText('admin@example.com')).closest('tr');
    expect(userRow).not.toBeNull();
    expect(within(userRow as HTMLElement).getByText('Admin User')).toBeInTheDocument();
    expect(within(userRow as HTMLElement).getByText('admin')).toBeInTheDocument();
    expect(within(userRow as HTMLElement).getByText('Pro')).toBeInTheDocument();
    expect(within(userRow as HTMLElement).getByText('Active')).toBeInTheDocument();
    expect(within(userRow as HTMLElement).getByText('1,000 tokens / 20 calls / $1.50')).toBeInTheDocument();
    expect(within(userRow as HTMLElement).getByRole('button', { name: 'Edit user admin@example.com' })).toBeEnabled();
    expect(within(userRow as HTMLElement).getByRole('button', { name: 'Disable user admin@example.com' })).toBeEnabled();

    fireEvent.click(within(userRow as HTMLElement).getByRole('button', { name: 'Edit user admin@example.com' }));
    expect(await screen.findByRole('heading', { name: 'Edit User: admin@example.com' })).toBeInTheDocument();
    expect(screen.getByLabelText('Role')).toHaveValue('admin');
    expect(screen.getByLabelText('Plan ID')).toHaveValue('plan_pro');
    expect(screen.getByLabelText('Status')).toHaveValue('active');
    expect(screen.getByRole('button', { name: 'Save User' })).toBeEnabled();
  });

  it('keeps admin audit-log route-level governance filters and table evidence reachable', async () => {
    const router = createAppRouter(['/admin/audit-log']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const adminNavigation = await screen.findByRole('complementary', { name: 'Admin navigation' });
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
    expect(within(adminNavigation).getByRole('link', { name: 'Audit Log' })).toHaveAttribute(
      'href',
      '/admin/audit-log'
    );
    expect(await screen.findByRole('heading', { name: 'Audit Log' })).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Search actor...')).toBeInTheDocument();
    expect(screen.getByLabelText('Action filter')).toBeInTheDocument();
    expect(screen.getByLabelText('Resource type filter')).toBeInTheDocument();
    expect(screen.getByLabelText('Start date')).toBeInTheDocument();
    expect(screen.getByLabelText('End date')).toBeInTheDocument();

    const auditRow = (await screen.findByText('channel.create')).closest('tr');
    expect(auditRow).not.toBeNull();
    expect(within(auditRow as HTMLElement).getByText('admin@example.com')).toBeInTheDocument();
    expect(within(auditRow as HTMLElement).getByText('channel / ch_1')).toBeInTheDocument();
    expect(within(auditRow as HTMLElement).getByText('name')).toBeInTheDocument();
    expect(within(auditRow as HTMLElement).getByText('127.0.0.1')).toBeInTheDocument();
  });

  it('keeps marketplace agent detail route-level install and review controls reachable', async () => {
    const router = createAppRouter(['/marketplace/agents/agent_1']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    const marketplaceLink = within(workspaceNavigation).getByRole('link', { name: 'Marketplace' });
    expect(marketplaceLink).toHaveAttribute('href', '/marketplace');
    expect(marketplaceLink).toHaveAttribute('aria-current', 'page');
    expect(await screen.findByRole('heading', { name: 'Research Agent' })).toBeInTheDocument();
    expect(await screen.findByText('Paid installs create a checkout-backed marketplace order before workspace installation.')).toBeInTheDocument();
    expect(await screen.findByLabelText('Agent version')).toHaveValue('ver_1');
    expect(await screen.findByLabelText('Payment provider')).toHaveValue('stripe');
    expect(screen.getByRole('option', { name: 'Alipay' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Install Agent' })).toBeEnabled();
    expect(screen.getByText('Reviews')).toBeInTheDocument();
    expect(screen.getByLabelText('Review text')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Submit Review' })).toBeEnabled();
  });

  it('executes marketplace paid-install Alipay checkout flow inside the workspace shell', async () => {
    const router = createAppRouter(['/marketplace/agents/agent_1']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(within(workspaceNavigation).getByRole('link', { name: 'Marketplace' })).toHaveAttribute('aria-current', 'page');
    expect(await screen.findByRole('heading', { name: 'Research Agent' })).toBeInTheDocument();

    fireEvent.change(await screen.findByLabelText('Payment provider'), { target: { value: 'alipay' } });
    fireEvent.click(screen.getByRole('button', { name: 'Install Agent' }));

    await waitFor(() =>
      expect(marketplaceApiMocks.installAgent).toHaveBeenCalledWith('agent_1', 'ver_1', 'alipay')
    );
    expect(await screen.findByRole('link', { name: 'Continue Alipay checkout' })).toHaveAttribute(
      'href',
      'https://checkout.alipay.test/session/cs_marketplace_alipay_router'
    );
  });

  it('keeps marketplace home route-level browse, curation, and template controls reachable', async () => {
    const router = createAppRouter(['/marketplace']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Marketplace' })).toHaveAttribute('href', '/marketplace');
    expect(await screen.findByRole('heading', { name: 'Agent Marketplace' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'My Agents' })).toHaveAttribute('href', '/marketplace/my-agents');
    expect(screen.getByRole('link', { name: 'Publish Agent' })).toHaveAttribute('href', '/marketplace/publish');
    expect(screen.getByPlaceholderText('Search agents...')).toBeInTheDocument();
    expect(screen.getByLabelText('Marketplace sort')).toHaveValue('recommended');
    expect(screen.getByText('Filters')).toBeInTheDocument();
    expect(screen.getAllByText('Productivity').length).toBeGreaterThanOrEqual(1);
    expect(await screen.findByRole('heading', { name: 'Popular' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Top rated' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'New arrivals' })).toBeInTheDocument();
    expect(screen.getByText('Popular Ops Agent')).toBeInTheDocument();
    expect(screen.getByText('Top Rated QA Agent')).toBeInTheDocument();
    expect(screen.getByText('New Arrival Agent')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Templates' })).toBeInTheDocument();
    expect(screen.getByText('Lead Intake Template')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Use Lead Intake Template' }));
    expect(await screen.findByText('Template ready to use.')).toBeInTheDocument();
    expect(await screen.findByText('Research Agent')).toBeInTheDocument();
    expect(screen.getAllByRole('link', { name: 'View Agent' })[0]).toHaveAttribute(
      'href',
      '/marketplace/agents/agent_popular'
    );
  });

  it('keeps marketplace my-agents route-level settlement and inventory controls reachable', async () => {
    const router = createAppRouter(['/marketplace/my-agents']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'My Agents' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Publish Agent' })).toHaveAttribute('href', '/marketplace/publish');
    expect(screen.getByRole('heading', { name: 'Template Factory' })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Template name'), { target: { value: 'Router Template' } });
    fireEvent.change(screen.getByLabelText('Template type'), { target: { value: 'workflow' } });
    fireEvent.change(screen.getByLabelText('Template JSON'), { target: { value: '{"nodes":[{"id":"start"}]}' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create Template' }));
    await waitFor(() =>
      expect(marketplaceApiMocks.createTemplate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'Router Template',
          templateData: { nodes: [{ id: 'start' }] },
          type: 'workflow'
        })
      )
    );
    expect(await screen.findByText('Template created: Router Template.')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Settlement Cycle' })).toBeInTheDocument();
    expect(screen.getByText('Current cycle: Monthly')).toBeInTheDocument();
    expect(screen.getByText('5 business days')).toBeInTheDocument();
    expect(screen.getByText('1% processing fee')).toBeInTheDocument();
    expect(screen.getByText('$100 minimum payout')).toBeInTheDocument();
    expect(screen.getByText('Tier 3')).toBeInTheDocument();
    expect(screen.getByText('15% current platform fee')).toBeInTheDocument();
    expect(screen.getByText('$85,000 to next tier')).toBeInTheDocument();
    expect(screen.getByText('$72,250 projected net increase')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Settlement Summary' })).toBeInTheDocument();
    expect(screen.getByText('Gross revenue')).toBeInTheDocument();
    expect(screen.getByText('$15,000')).toBeInTheDocument();
    expect(screen.getByText('Available payout')).toBeInTheDocument();
    expect(screen.getByText('$10,950')).toBeInTheDocument();
    expect(screen.getByText('Pending settlement')).toBeInTheDocument();
    expect(screen.getByText('$1,200')).toBeInTheDocument();
    expect(screen.getByText('Refunded')).toBeInTheDocument();
    expect(screen.getByText('$350')).toBeInTheDocument();
    expect(screen.getByText('Payout pending')).toBeInTheDocument();
    expect(screen.getByText('$640')).toBeInTheDocument();
    expect(screen.getByText('Paid out')).toBeInTheDocument();
    expect(screen.getByText('$8,000')).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Active Users' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'API Calls' })).toBeInTheDocument();
    expect(screen.getByText('64')).toBeInTheDocument();
    expect(screen.getByText('900')).toBeInTheDocument();
    expect(screen.getByLabelText('Settlement cycle')).toHaveValue('monthly');
    fireEvent.change(screen.getByLabelText('Settlement cycle'), { target: { value: 'weekly' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save Settlement Cycle' }));
    expect(await screen.findByText('2% processing fee')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Published Agents' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Installed Agents' })).toBeInTheDocument();
    expect(await screen.findAllByText('Research Agent')).toHaveLength(3);
    expect(screen.getByText('1.0.0')).toBeInTheDocument();
    expect(screen.getAllByText('120')).toHaveLength(2);
    expect(screen.getByRole('link', { name: 'Open agent Research Agent' })).toHaveAttribute(
      'href',
      '/marketplace/agents/agent_1'
    );
    fireEvent.click(screen.getByRole('button', { name: 'Delete agent Research Agent' }));
    const deleteDialog = await screen.findByRole('dialog', { name: 'Delete Agent' });
    expect(within(deleteDialog).getByText('Delete Research Agent from the marketplace publisher inventory.')).toBeInTheDocument();
    fireEvent.click(within(deleteDialog).getByRole('button', { name: 'Delete Agent' }));
    await waitFor(() => expect(marketplaceApiMocks.deleteAgent).toHaveBeenCalledWith('agent_1'));
    expect(screen.getByRole('button', { name: 'Uninstall Research Agent' })).toBeEnabled();
  });

  it('keeps marketplace publish route-level submission controls reachable', async () => {
    const router = createAppRouter(['/marketplace/publish']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Publish Agent' })).toBeInTheDocument();
    expect(await screen.findByText('Submit an agent for marketplace review.')).toBeInTheDocument();
    expect(await screen.findByLabelText('Name')).toBeInTheDocument();
    expect(await screen.findByLabelText('Category')).toHaveTextContent('Productivity');
    expect(screen.getByLabelText('Pricing')).toHaveValue('free');
    expect(screen.getByLabelText('Version')).toHaveValue('1.0.0');
    expect(screen.getByRole('button', { name: 'Publish Agent' })).toBeEnabled();
  });

  it('keeps admin review route-level SLA and decision controls reachable', async () => {
    const router = createAppRouter(['/admin/reviews']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('complementary', { name: 'Admin navigation' })).toBeInTheDocument();
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Review Queue' })).toBeInTheDocument();
    expect(await screen.findByLabelText('Review status filter')).toHaveValue('pending_review');
    expect(await screen.findByText('Research Agent')).toBeInTheDocument();
    expect(screen.getByText('Manual SLA: Due soon by 2026-06-05 13:00 UTC')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Approve agent Research Agent' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reject agent Research Agent' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Request changes for agent Research Agent' })).toBeInTheDocument();
  });

  it('keeps admin review route-level marketplace governance actions reachable', async () => {
    const router = createAppRouter(['/admin/reviews']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('heading', { name: 'Review Queue' })).toBeInTheDocument();
    fireEvent.click(await screen.findByRole('button', { name: 'Enforce SLA' }));
    await waitFor(() => expect(adminApiMocks.enforceReviewSLA).toHaveBeenCalledWith({ limit: 100 }));
    expect(await screen.findByText('Review SLA scan complete: 3 scanned, 2 alerted.')).toBeInTheDocument();

    expect(await screen.findByRole('heading', { name: 'Marketplace Abuse Reports' })).toBeInTheDocument();
    expect(await screen.findByText('agent_review_router')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Resolve abuse report report_router' }));
    fireEvent.change(await screen.findByLabelText('Resolution'), { target: { value: 'agent removed' } });
    fireEvent.click(screen.getByRole('button', { name: 'Resolve Report' }));
    await waitFor(() => expect(adminApiMocks.resolveMarketplaceAbuseReport).toHaveBeenCalledWith('report_router', 'agent removed'));
    expect(await screen.findByText('Abuse report resolved.')).toBeInTheDocument();

    expect(screen.getByRole('heading', { name: 'Marketplace Governance' })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Agent ID'), { target: { value: 'agent_review_router' } });
    fireEvent.change(screen.getByLabelText('Reason'), { target: { value: 'policy violation' } });
    fireEvent.click(screen.getByRole('button', { name: 'Apply Governance' }));
    await waitFor(() => expect(adminApiMocks.takedownMarketplaceAgent).toHaveBeenCalledWith('agent_review_router', 'policy violation'));
    expect(await screen.findByText('Marketplace agent taken down.')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Governance action'), { target: { value: 'reinstate' } });
    fireEvent.change(screen.getByLabelText('Reason'), { target: { value: 'appeal accepted' } });
    fireEvent.click(screen.getByRole('button', { name: 'Apply Governance' }));
    await waitFor(() => expect(adminApiMocks.reinstateMarketplaceAgent).toHaveBeenCalledWith('agent_review_router', 'appeal accepted'));
    expect(await screen.findByText('Marketplace agent reinstated.')).toBeInTheDocument();
  });

  it('requests admin review changes through the real admin router flow', async () => {
    const router = createAppRouter(['/admin/reviews']);

    render(<RouterProvider future={routerFuture} router={router} />);

    fireEvent.click(await screen.findByRole('button', { name: 'Request changes for agent Research Agent' }));
    expect(await screen.findByText('Request Changes: Research Agent')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Request Changes' }));
    expect(await screen.findByText('A change request reason is required.')).toBeInTheDocument();
    expect(adminApiMocks.requestAgentChanges).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText('Change Request Reason'), {
      target: { value: 'Add screenshots and clarify pricing.' }
    });
    fireEvent.click(screen.getByRole('button', { name: 'Request Changes' }));

    await waitFor(() =>
      expect(adminApiMocks.requestAgentChanges).toHaveBeenCalledWith(
        'agent_review_router',
        'Add screenshots and clarify pricing.'
      )
    );
  });

  it('renders admin usage logs route inside the admin shell', async () => {
    const router = createAppRouter(['/admin/usage-logs']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('complementary', { name: 'Admin navigation' })).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Usage Logs' })).toBeInTheDocument();
  });

  it('keeps admin usage logs route-level filters, analytics, and table evidence reachable', async () => {
    const router = createAppRouter(['/admin/usage-logs']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('complementary', { name: 'Admin navigation' })).toBeInTheDocument();
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Usage Logs' })).toBeInTheDocument();
    expect(await screen.findByLabelText('Organization ID filter')).toBeInTheDocument();
    expect(screen.getByLabelText('API token ID filter')).toBeInTheDocument();
    expect(screen.getByLabelText('Analytics granularity filter')).toHaveValue('day');
    expect(await screen.findByRole('heading', { name: 'Usage Analytics' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'By model' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Cross dimensions' })).toBeInTheDocument();
    expect(screen.getByText('req_admin_router')).toBeInTheDocument();
    expect(screen.getByText('tok_router_1')).toBeInTheDocument();
    expect(screen.getAllByText('gpt-4o').length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText('openai / ch_router_1')).toBeInTheDocument();
    expect(screen.getByLabelText('Success')).toBeInTheDocument();
  });

  it('keeps admin alerts route-level routing, provider, recovery, and delivery controls reachable', async () => {
    const router = createAppRouter(['/admin/alerts']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const adminNavigation = await screen.findByRole('complementary', { name: 'Admin navigation' });
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
    expect(within(adminNavigation).getByRole('link', { name: 'Alerts' })).toHaveAttribute('href', '/admin/alerts');
    expect(await screen.findByRole('heading', { name: 'Alerts' })).toBeInTheDocument();
    expect(await screen.findByLabelText('Alert filters')).toBeInTheDocument();
    expect(screen.getByLabelText('Severity')).toBeInTheDocument();
    expect(screen.getByLabelText('Status')).toBeInTheDocument();
    expect(screen.getByLabelText('Component')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Apply alert filters' })).toBeEnabled();
    expect(await screen.findByLabelText('Notification routing')).toBeInTheDocument();
    expect(screen.getByText('Critical alerts')).toBeInTheDocument();
    expect(screen.getByText('email + im + sms + third party')).toBeInTheDocument();
    expect(screen.getByLabelText('Route critical alerts to sms')).toBeChecked();
    expect(screen.getByRole('button', { name: 'Save notification routing' })).toBeEnabled();
    const providers = await screen.findByLabelText('Alert notification providers');
    expect(within(providers).getByText('Slack Ops')).toBeInTheDocument();
    expect(within(providers).getByText('alert_provider_slack_ops')).toBeInTheDocument();
    expect(within(providers).getAllByText('slack webhook').length).toBeGreaterThanOrEqual(2);
    expect(within(providers).getByRole('button', { name: 'Test provider Slack Ops' })).toBeEnabled();
    fireEvent.click(within(providers).getByRole('button', { name: 'Test provider Slack Ops' }));
    expect(await screen.findByText('Slack Ops: Provider test delivered.')).toBeInTheDocument();
    const recoveryActions = await screen.findByLabelText('Recovery actions');
    expect(within(recoveryActions).getByText('restart-relay')).toBeInTheDocument();
    expect(within(recoveryActions).getByText('relay-backlog')).toBeInTheDocument();
    expect(within(recoveryActions).getByText('restart')).toBeInTheDocument();
    expect(within(recoveryActions).getByText('recorded')).toBeInTheDocument();
    const alertList = await screen.findByLabelText('Alert list');
    expect(within(alertList).getByRole('heading', { name: 'Relay backlog' })).toBeInTheDocument();
    expect(within(alertList).getByText('Relay queue exceeded recovery threshold')).toBeInTheDocument();
    expect(within(alertList).getByText('critical')).toBeInTheDocument();
    expect(within(alertList).getByText('open')).toBeInTheDocument();
    expect(within(alertList).getByText('relay')).toBeInTheDocument();
    expect(within(alertList).getByRole('button', { name: 'Acknowledge Relay backlog' })).toBeEnabled();
    expect(within(alertList).getByRole('button', { name: 'Resolve Relay backlog' })).toBeEnabled();
    fireEvent.click(within(alertList).getByRole('button', { name: 'View deliveries Relay backlog' }));
    const deliveryHistory = await screen.findByLabelText('Notification delivery history');
    expect(within(deliveryHistory).getByText('email')).toBeInTheDocument();
    expect(within(deliveryHistory).getByText('delivered')).toBeInTheDocument();
    expect(within(deliveryHistory).getByText('im')).toBeInTheDocument();
    expect(within(deliveryHistory).getByText('failed')).toBeInTheDocument();
    expect(within(deliveryHistory).getByText('alert_provider_slack_ops')).toBeInTheDocument();
    expect(within(deliveryHistory).getByText('slack_webhook')).toBeInTheDocument();
    expect(within(deliveryHistory).getByText('im webhook failed')).toBeInTheDocument();
  });

  it('renders admin API tokens route inside the admin shell', async () => {
    const router = createAppRouter(['/admin/api-tokens']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('complementary', { name: 'Admin navigation' })).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'API Tokens' })).toBeInTheDocument();
  });

  it('keeps admin API tokens route-level filters and revoke controls reachable', async () => {
    const router = createAppRouter(['/admin/api-tokens']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const adminNavigation = await screen.findByRole('complementary', { name: 'Admin navigation' });
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
    expect(within(adminNavigation).getByRole('link', { name: 'API Tokens' })).toHaveAttribute(
      'href',
      '/admin/api-tokens'
    );
    expect(await screen.findByRole('heading', { name: 'API Tokens' })).toBeInTheDocument();
    expect(await screen.findByText('1 relay keys matched')).toBeInTheDocument();
    expect(screen.getByLabelText('Organization ID filter')).toBeInTheDocument();
    expect(screen.getByLabelText('User ID filter')).toBeInTheDocument();
    expect(screen.getByLabelText('Status filter')).toBeInTheDocument();
    expect(screen.getByLabelText('User group filter')).toBeInTheDocument();
    expect(screen.getByLabelText('Search tokens')).toBeInTheDocument();
    expect(screen.getByLabelText('Model filter')).toBeInTheDocument();
    expect(await screen.findByText('Production key')).toBeInTheDocument();
    expect(screen.getByText('sk-oblv')).toBeInTheDocument();
    expect(screen.getByText('user@example.com')).toBeInTheDocument();
    expect(screen.getByText('vip')).toBeInTheDocument();
    expect(screen.getByText('Active')).toBeInTheDocument();
    expect(screen.getByText('gpt-4o, claude-3-5-sonnet')).toBeInTheDocument();
    expect(screen.getByText('$12.5000 / $50.0000')).toBeInTheDocument();
    expect(screen.getByText('$1.2300')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Revoke Production key' })).toBeEnabled();
  });

  it('renders admin models route inside the admin shell', async () => {
    const router = createAppRouter(['/admin/models']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('complementary', { name: 'Admin navigation' })).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Models' })).toBeInTheDocument();
  });

  it('keeps admin models route-level filters, ranking, and cost evidence reachable', async () => {
    const router = createAppRouter(['/admin/models']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const adminNavigation = await screen.findByRole('complementary', { name: 'Admin navigation' });
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
    expect(within(adminNavigation).getByRole('link', { name: 'Models' })).toHaveAttribute('href', '/admin/models');
    expect(await screen.findByRole('heading', { name: 'Models' })).toBeInTheDocument();
    expect(await screen.findByText('1 relay models matched')).toBeInTheDocument();
    expect(screen.getByLabelText('Provider filter')).toBeInTheDocument();
    expect(screen.getByLabelText('Group filter')).toBeInTheDocument();
    expect(screen.getByLabelText('Status filter')).toBeInTheDocument();
    expect(screen.getByLabelText('Search models')).toBeInTheDocument();
    expect(await screen.findByText('gpt-4o')).toBeInTheDocument();
    expect(screen.getByText('openai')).toBeInTheDocument();
    expect(screen.getByText('azure')).toBeInTheDocument();
    expect(screen.getByText('default')).toBeInTheDocument();
    expect(screen.getByText('vip')).toBeInTheDocument();
    expect(screen.getByText('OpenAI primary')).toBeInTheDocument();
    expect(screen.getByText('1 / 2 enabled')).toBeInTheDocument();
    expect(screen.getByText('$0.0200 - $0.0500')).toBeInTheDocument();
    expect(screen.getByText('1.20x')).toBeInTheDocument();
    expect(screen.getByText('$0.6100')).toBeInTheDocument();
    expect(screen.getByText('$0.6200')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sort by Requests' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sort by Spend' })).toBeInTheDocument();
  });

  it('renders admin settings route inside the admin shell', async () => {
    const router = createAppRouter(['/admin/settings']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('complementary', { name: 'Admin navigation' })).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Settings' })).toBeInTheDocument();
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
  });

  it('keeps admin settings route-level pricing and usage-limit controls reachable', async () => {
    const router = createAppRouter(['/admin/settings']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const adminNavigation = await screen.findByRole('complementary', { name: 'Admin navigation' });
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
    expect(within(adminNavigation).getByRole('link', { name: 'Usage Logs' })).toHaveAttribute(
      'href',
      '/admin/usage-logs'
    );
    expect(await screen.findByRole('heading', { name: 'Settings' })).toBeInTheDocument();
    expect(await screen.findByText('Relay pricing')).toBeInTheDocument();
    expect(await screen.findByLabelText('Model multipliers JSON')).toHaveValue(
      '{\n  "gpt-4o": 1.2,\n  "gpt-4o-mini": 0.7\n}'
    );
    expect(screen.getByLabelText('Group multipliers JSON')).toHaveValue('{\n  "enterprise": 0.85\n}');
    expect(screen.getByRole('button', { name: 'Refresh' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Save Settings' })).toBeEnabled();

    fireEvent.change(screen.getByLabelText('Model multipliers JSON'), {
      target: { value: '{ "gpt-4o": 1.25, "gpt-4o-mini": 0.7 }' }
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save Settings' }));
    expect(await screen.findByText('Settings saved.')).toBeInTheDocument();

    expect(await screen.findByRole('heading', { name: 'Usage limits' })).toBeInTheDocument();
    expect(screen.getByText('organization org_router')).toBeInTheDocument();
    expect(screen.getByText('Mode: organization')).toBeInTheDocument();
    expect(screen.getAllByText('request_tokens').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('Recovered')).toBeInTheDocument();
    expect(screen.getByText('1 recent hit - relay_rate_limited')).toBeInTheDocument();
    expect(screen.getByText('Recovery: req_settings_recovered')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Edit organization org_router request_tokens hour' })).toBeEnabled();
    expect(screen.getByRole('heading', { name: 'Edit limit' })).toBeInTheDocument();
    expect(screen.getByLabelText('Scope type')).toHaveValue('organization');
    expect(screen.getByLabelText('Scope ID')).toHaveValue('');
    expect(screen.getByLabelText('Limit type')).toHaveValue('tokens');
    expect(screen.getByLabelText('Period')).toHaveValue('minute');
    expect(screen.getByLabelText('Limit value')).toHaveValue(1000);
    expect(screen.getByLabelText('Enabled')).toBeChecked();

    fireEvent.click(screen.getByRole('button', { name: 'Edit organization org_router request_tokens hour' }));

    expect(screen.getByLabelText('Scope type')).toHaveValue('organization');
    expect(screen.getByLabelText('Scope ID')).toHaveValue('org_router');
    expect(screen.getByLabelText('Limit type')).toHaveValue('request_tokens');
    expect(screen.getByLabelText('Period')).toHaveValue('hour');
    expect(screen.getByLabelText('Limit value')).toHaveValue(250);
    expect(screen.getByLabelText('Enabled')).toBeChecked();
    expect(screen.getByRole('button', { name: 'Save Usage Limit' })).toBeEnabled();

    fireEvent.change(screen.getByLabelText('Limit value'), { target: { value: '300' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save Usage Limit' }));

    expect(await screen.findByText('Usage limit saved.')).toBeInTheDocument();
  });
});
