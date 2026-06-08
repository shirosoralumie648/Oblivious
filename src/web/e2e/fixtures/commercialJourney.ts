import type { Page, Route } from '@playwright/test';

const now = '2026-05-29T00:00:00Z';

const preferences = {
  defaultMode: 'chat' as const,
  modelStrategy: 'balanced',
  networkEnabledHint: true,
  onboardingCompleted: false,
};

const commercialSession = {
  onboardingCompleted: false,
  preferences,
  session: {
    id: 'session_commercial',
    expiresAt: '2026-05-30T00:00:00Z',
  },
  user: {
    id: 'user_commercial_admin',
    email: 'commercial-admin@example.com',
    name: 'Commercial Admin',
    role: 'admin',
  },
  workspace: {
    id: 'workspace_commercial',
  },
};

const knowledgeBase = {
  id: 'kb_commercial',
  name: 'Commercial Runbook',
  documentCount: 1,
  updatedAt: now,
};

const knowledgeDocument = {
  id: 'doc_commercial_runbook',
  title: 'Commercial Deployment Runbook',
  content: 'Deploy, rollback, restore, and Relay outage procedures for the commercial journey.',
  updatedAt: now,
};

const conversation = {
  id: 'conv_commercial',
  title: 'Commercial Relay Journey',
  createdAt: now,
  updatedAt: now,
};

const baseConversationConfig = {
  conversationId: conversation.id,
  knowledgeBaseIds: [knowledgeBase.id],
  modelId: 'commercial-journey-model',
  systemPromptOverride: 'Keep commercial evidence tied to Relay billing and quota.',
  temperature: 0.2,
  maxOutputTokens: 512,
  toolsEnabled: true,
  updatedAt: now,
};

const approvalTask = {
  id: 'task_commercial_approval',
  title: 'Commercial approval boundary',
  goal: 'Verify commercial deployment with bounded tools and approval.',
  status: 'awaiting_confirmation',
  executionMode: 'standard',
  authorizationScope: 'workspace_tools',
  budgetLimit: 25,
  budgetConsumed: 3,
  knowledgeBaseIds: [knowledgeBase.id],
  toolAllowList: ['calculator', 'datetime'],
  toolDenyList: ['http_request'],
  currentStep: 'Waiting for operator approval',
  events: [
    {
      type: 'approval_required',
      message: 'Human approval is required before deployment actions continue.',
      createdAt: now,
    },
  ],
  resultArtifacts: [{ label: 'Relay evidence', value: 'all AI calls route through Relay' }],
  steps: [
    { id: 'step_scope', title: 'Confirm tenant scope', status: 'completed', stepIndex: 1, createdAt: now },
    { id: 'step_approval', title: 'Request approval', status: 'awaiting_confirmation', stepIndex: 2, createdAt: now },
  ],
  createdAt: now,
  startedAt: now,
  updatedAt: now,
};

const failedTask = {
  id: 'task_commercial_failed',
  title: 'Commercial recovery check',
  goal: 'Retry a failed commercial run without losing tenant context.',
  status: 'failed',
  executionMode: 'standard',
  authorizationScope: 'workspace_tools',
  budgetLimit: 25,
  budgetConsumed: 7,
  knowledgeBaseIds: [knowledgeBase.id],
  toolAllowList: ['calculator'],
  toolDenyList: ['http_request'],
  currentStep: 'Provider outage recovered by retry',
  events: [
    {
      type: 'provider_error',
      message: 'Relay provider failed and preserved retry context.',
      createdAt: now,
    },
  ],
  resultArtifacts: [{ label: 'Retry evidence', value: 'context preserved after failure' }],
  steps: [
    { id: 'step_retry_scope', title: 'Keep tenant scope', status: 'completed', stepIndex: 1, createdAt: now },
    { id: 'step_retry', title: 'Retry failed tool run', status: 'failed', stepIndex: 2, createdAt: now },
  ],
  createdAt: now,
  startedAt: now,
  updatedAt: now,
};

const commercialCategory = {
  id: 'cat_operations',
  name: 'Operations',
  slug: 'operations',
  displayOrder: 1,
  agentCount: 1,
};

const commercialAgent = {
  id: 'agent_commercial_operator',
  ownerID: 'publisher_commercial',
  ownerName: 'Commercial Publisher',
  name: 'Commercial Operator',
  description: 'Runs checkout-backed operational workflows with settlement and governance boundaries.',
  iconURL: '',
  categoryID: commercialCategory.id,
  categorySlug: commercialCategory.slug,
  categoryName: commercialCategory.name,
  tags: ['commercial', 'settlement', 'relay'],
  tools: JSON.stringify({ tools: [{ name: 'datetime', type: 'builtin' }] }),
  exampleConversations: JSON.stringify([{ userMessage: 'Check readiness', assistantMessage: 'Relay and billing evidence are linked.' }]),
  systemPrompt: 'Operate within commercial settlement boundaries.',
  visibility: 'public',
  status: 'approved',
  pricingType: 'one_time',
  pricingAmount: 50,
  currentVersion: '1.0.0',
  installCount: 18,
  ratingAvg: 4.9,
  rating: 4.9,
  ratingCount: 8,
  createdAt: now,
  updatedAt: now,
};

const submittedAgent = {
  ...commercialAgent,
  id: 'agent_submitted_commercial',
  name: 'Commercial Audit Drafter',
  status: 'pending_review',
  pricingAmount: 75,
  installCount: 0,
  ratingAvg: 0,
  rating: 0,
  ratingCount: 0,
};

const installedAgent = {
  id: 'install_commercial_operator',
  agentID: commercialAgent.id,
  agentName: commercialAgent.name,
  userID: commercialSession.user.id,
  version: '1.0.0',
  installedAt: now,
};

const billingSummary = {
  billingSessions: { count: 1, preAuthorizedAmount: 9, settledAmount: 8.5 },
  paymentIntents: { count: 3, totalAmount: 154, refundedAmount: 10 },
  webhookEvents: { count: 4, failedCount: 0 },
  subscriptions: { count: 1, activeCount: 1 },
  topups: { count: 1, paidAmount: 25, refundedAmount: 0 },
  invoices: { count: 1, amountDue: 29, amountPaid: 29 },
  refunds: { count: 1, totalAmount: 10 },
  settlements: { count: 1, grossAmount: 50, platformFeeAmount: 10, publisherNetAmount: 40, refundedAmount: 10 },
  payouts: { count: 1, totalAmount: 40 },
};

const billingRows = {
  sessions: [
    {
      id: 'bs_commercial_session',
      organizationId: 'organization_commercial',
      userId: commercialSession.user.id,
      status: 'settled',
      model: 'commercial-journey-model',
      apiType: 'chat.completions',
      preAuthorizedAmount: 9,
      settledAmount: 8.5,
      createdAt: now,
    },
  ],
  paymentIntents: [
    {
      id: 'pi_commercial_subscription',
      organizationId: 'organization_commercial',
      userId: commercialSession.user.id,
      provider: 'stripe',
      kind: 'subscription',
      status: 'completed',
      amount: 29,
      currency: 'usd',
      createdAt: now,
    },
  ],
  webhookEvents: [
    {
      id: 'swe_commercial_checkout',
      eventId: 'evt_commercial_checkout',
      eventType: 'checkout.session.completed',
      status: 'processed',
      provider: 'stripe',
      createdAt: now,
    },
  ],
  subscriptions: [
    {
      id: 'sub_commercial',
      packageId: 'plan_commercial',
      status: 'active',
      providerSubscriptionId: 'sub_provider_commercial',
      createdAt: now,
    },
  ],
  topups: [
    {
      id: 'topup_commercial',
      status: 'paid',
      amount: 25,
      money: 25,
      paymentIntentId: 'pi_commercial_topup',
      createdAt: now,
    },
  ],
  invoices: [
    {
      id: 'inv_commercial',
      status: 'paid',
      amountDue: 29,
      amountPaid: 29,
      providerInvoiceId: 'in_commercial',
      createdAt: now,
    },
  ],
  refunds: [
    {
      id: 'refund_commercial',
      status: 'succeeded',
      amount: 10,
      providerRefundId: 're_commercial',
      createdAt: now,
    },
  ],
  settlements: [
    {
      id: 'settlement_commercial',
      publisherOrganizationId: 'publisher_org_commercial',
      publisherUserId: 'publisher_commercial',
      status: 'payout_pending',
      agentId: commercialAgent.id,
      grossAmount: 50,
      platformFeeAmount: 10,
      publisherNetAmount: 40,
      refundedAmount: 10,
      createdAt: now,
    },
  ],
  payouts: [
    {
      id: 'payout_commercial',
      publisherOrganizationId: 'publisher_org_commercial',
      publisherUserId: 'publisher_commercial',
      status: 'pending',
      amount: 40,
      providerPayoutId: 'po_commercial',
      createdAt: now,
    },
  ],
};

function envelope(data: unknown) {
  return {
    ok: true,
    data,
    error: null,
  };
}

async function fulfillJSON(route: Route, data: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(envelope(data)),
  });
}

async function fulfillNotFound(route: Route) {
  await route.fulfill({
    status: 404,
    contentType: 'application/json',
    body: JSON.stringify({
      ok: false,
      data: null,
      error: { code: 'not_found', message: 'fixture route not found' },
    }),
  });
}

function taskDetail(id: string) {
  if (id === approvalTask.id) {
    return approvalTask;
  }
  return failedTask;
}

export async function registerCommercialJourneyRoutes(page: Page): Promise<void> {
  let conversationCreated = false;
  let publishedAgentSubmitted = false;
  let messages = [] as Array<{ id: string; role: string; content: string; createdAt: string }>;

  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    const method = request.method();

    if (method === 'GET' && pathname === '/api/v1/auth/me') {
      await fulfillJSON(route, commercialSession);
      return;
    }

    if (method === 'PUT' && pathname === '/api/v1/app/me/preferences') {
      const body = await request.postDataJSON();
      Object.assign(preferences, body, { onboardingCompleted: true });
      commercialSession.onboardingCompleted = true;
      await fulfillJSON(route, preferences);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/app/models') {
      await fulfillJSON(route, [{ id: 'commercial-journey-model', label: 'Commercial Relay Model' }]);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/app/personas') {
      await fulfillJSON(route, []);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/app/conversations') {
      await fulfillJSON(route, conversationCreated ? [conversation] : []);
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/app/conversations') {
      conversationCreated = true;
      await fulfillJSON(route, conversation);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/app/conversations/${conversation.id}/messages`) {
      await fulfillJSON(route, messages);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/app/conversations/${conversation.id}/config`) {
      await fulfillJSON(route, baseConversationConfig);
      return;
    }

    if (method === 'PUT' && pathname === `/api/v1/app/conversations/${conversation.id}/config`) {
      await fulfillJSON(route, baseConversationConfig);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/app/conversations/${conversation.id}/messages`) {
      messages = [
        {
          id: 'msg_user_commercial',
          role: 'user',
          content: 'Prove the commercial Relay journey.',
          createdAt: now,
        },
        {
          id: 'msg_assistant_commercial',
          role: 'assistant',
          content: 'Relay settled this chat with quota, billing, and monitoring metadata attached.',
          createdAt: now,
        },
      ];
      await fulfillJSON(route, messages);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/app/conversations/${conversation.id}/messages/stream`) {
      messages = [
        {
          id: 'msg_user_commercial',
          role: 'user',
          content: 'Prove the commercial Relay journey.',
          createdAt: now,
        },
        {
          id: 'msg_assistant_commercial',
          role: 'assistant',
          content: 'Relay settled this chat with quota, billing, and monitoring metadata attached.',
          createdAt: now,
        },
      ];
      await route.fulfill({
        status: 200,
        contentType: 'text/event-stream',
        body: 'data: [DONE]\n\n',
      });
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/app/conversations/${conversation.id}/convert-to-task`) {
      await fulfillJSON(route, {
        draftTaskGoal: approvalTask.goal,
        relatedKnowledgeBaseIds: [knowledgeBase.id],
        suggestedBudget: 25,
        suggestedExecutionMode: 'standard',
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/app/knowledge-bases') {
      await fulfillJSON(route, [knowledgeBase]);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/app/knowledge-bases/${knowledgeBase.id}`) {
      await fulfillJSON(route, knowledgeBase);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/app/knowledge-bases/${knowledgeBase.id}/documents`) {
      await fulfillJSON(route, [knowledgeDocument]);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/app/knowledge-bases/${knowledgeBase.id}/retrieval-test-cases`) {
      await fulfillJSON(route, []);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/app/knowledge-bases/${knowledgeBase.id}/retrieve`) {
      await fulfillJSON(route, [
        {
          chunkId: 'chunk_commercial_0',
          chunkIndex: 0,
          documentId: knowledgeDocument.id,
          documentTitle: knowledgeDocument.title,
          retrievalMethod: 'embedding_rag',
          similarity: 0.91,
          snippet: 'Commercial deployment rollback restore runbook evidence with source citations.',
          source: {
            chunkId: 'chunk_commercial_0',
            chunkIndex: 0,
            documentId: knowledgeDocument.id,
            documentTitle: knowledgeDocument.title,
          },
        },
      ]);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/app/tasks') {
      await fulfillJSON(route, [approvalTask, failedTask]);
      return;
    }

    if (method === 'GET' && pathname.startsWith('/api/v1/app/tasks/')) {
      await fulfillJSON(route, taskDetail(pathname.split('/').pop() ?? ''));
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/app/tasks/${approvalTask.id}/approve`) {
      await fulfillJSON(route, {
        ...approvalTask,
        status: 'running',
        currentStep: 'Operator approval recorded',
        events: [...approvalTask.events, { type: 'approval_recorded', message: 'Commercial operator approved continuation.', createdAt: now }],
        steps: approvalTask.steps.map((step) => step.id === 'step_approval' ? { ...step, status: 'running' } : step),
      });
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/app/tasks/${failedTask.id}/start`) {
      await fulfillJSON(route, {
        ...failedTask,
        status: 'running',
        currentStep: 'Retrying preserved commercial context',
        events: [...failedTask.events, { type: 'retry_started', message: 'Retry resumed with tenant and budget context.', createdAt: now }],
        steps: failedTask.steps.map((step) => step.id === 'step_retry' ? { ...step, status: 'running' } : step),
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/categories') {
      await fulfillJSON(route, { categories: [commercialCategory], total: 1 });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/search') {
      await fulfillJSON(route, { agents: [commercialAgent], total: 1 });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/templates') {
      await fulfillJSON(route, { templates: [], total: 0 });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/featured') {
      await fulfillJSON(route, { agents: [commercialAgent], total: 1 });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/curated') {
      await fulfillJSON(route, { popular: [commercialAgent], topRated: [commercialAgent], recent: [commercialAgent] });
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/marketplace/agents/${commercialAgent.id}`) {
      await fulfillJSON(route, { agent: commercialAgent, versions: [{ id: 'version_commercial_1', agentID: commercialAgent.id, version: '1.0.0', status: 'approved', createdAt: now }] });
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/marketplace/agents/${commercialAgent.id}/versions`) {
      await fulfillJSON(route, { versions: [{ id: 'version_commercial_1', agentID: commercialAgent.id, version: '1.0.0', status: 'approved', createdAt: now }], total: 1 });
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/marketplace/agents/${commercialAgent.id}/reviews`) {
      await fulfillJSON(route, { reviews: [{ id: 'review_commercial', agentID: commercialAgent.id, userID: commercialSession.user.id, userName: 'Commercial Admin', rating: 5, body: 'Settlement and governance boundaries are visible.', createdAt: now }], total: 1 });
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/marketplace/agents/${commercialAgent.id}/install`) {
      await fulfillJSON(route, installedAgent, 201);
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/marketplace/agents') {
      publishedAgentSubmitted = true;
      await fulfillJSON(route, submittedAgent, 201);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/my-agents') {
      await fulfillJSON(route, { agents: publishedAgentSubmitted ? [submittedAgent, commercialAgent] : [commercialAgent], total: publishedAgentSubmitted ? 2 : 1 });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/publisher/stats') {
      await fulfillJSON(route, {
        totalAgents: publishedAgentSubmitted ? 2 : 1,
        totalInstalls: 1,
        activeUsers: 1,
        totalAPICalls: 420,
        grossRevenue: 50,
        platformFees: 10,
        netRevenue: 40,
        refundedAmount: 10,
        pendingSettlementAmount: 40,
        availableAmount: 40,
        payoutPendingAmount: 40,
        paidOutAmount: 0,
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/publisher/settlement-preferences') {
      await fulfillJSON(route, {
        cycle: 'monthly',
        label: 'Monthly',
        payoutBusinessDays: 5,
        processingFeePercent: 1,
        minimumPayoutAmount: 100,
        effectiveFrom: 'next_settlement_cycle',
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/installs') {
      await fulfillJSON(route, { installs: [installedAgent], total: 1 });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/stats') {
      await fulfillJSON(route, {
        users: { totalUsers: 64, activeUsers: 61, newUsersToday: 4, newUsersWeek: 15 },
        quotas: { totalBalance: 250000, totalUsed: 125000, activeTopups: 3 },
        conversations: 33,
        agents: 12,
        tasks: 18,
        mcpServers: 2,
        channelsTotal: 2,
        channelsOnline: 2,
        activeAgents: 9,
        apiCalls24h: 4210,
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/reviews') {
      await fulfillJSON(route, { reviews: [commercialAgent], total: 1 });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/billing/summary') {
      await fulfillJSON(route, billingSummary);
      return;
    }

    const billingSurface = pathname.replace('/api/v1/admin/billing/', '');
    const billingMap: Record<string, keyof typeof billingRows> = {
      sessions: 'sessions',
      'payment-intents': 'paymentIntents',
      'webhook-events': 'webhookEvents',
      subscriptions: 'subscriptions',
      topups: 'topups',
      invoices: 'invoices',
      refunds: 'refunds',
      settlements: 'settlements',
      payouts: 'payouts',
    };
    if (method === 'GET' && billingSurface in billingMap) {
      const key = billingMap[billingSurface];
      await fulfillJSON(route, { [key]: billingRows[key], total: billingRows[key].length });
      return;
    }

    await fulfillNotFound(route);
  });
}
