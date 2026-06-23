import type { Page, Route } from '@playwright/test';

const now = '2026-06-17T03:10:00Z';

const session = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_publishing_browser',
    expiresAt: '2026-06-18T03:10:00Z',
  },
  user: {
    id: 'user_publishing_operator',
    email: 'publishing-operator@example.com',
    name: 'Publishing Operator',
    role: 'admin',
  },
  workspace: {
    id: 'workspace_publishing_browser',
  },
};

const opsWebhook = {
  id: 'channel_ops_webhook',
  organization_id: 'org_publishing_browser',
  name: 'Ops Webhook',
  type: 'webhook',
  status: 'degraded',
  config: {
    secret: '********',
    url: 'https://hooks.example/ops',
  },
  created_at: now,
  updated_at: now,
};

const fallbackSlack = {
  id: 'channel_fallback_slack',
  organization_id: 'org_publishing_browser',
  name: 'Fallback Slack',
  type: 'slack',
  status: 'active',
  config: {
    botToken: '********',
    signingSecret: '********',
    url: 'https://hooks.slack.com/services/fallback',
  },
  created_at: now,
  updated_at: now,
};

const longMobileWebhook = {
  id: 'channel_provider_research_incident_webhook_mobile_long',
  organization_id: 'org_publishing_browser',
  name: 'ProviderResearchClusterPublishingWebhookIncidentChannelWithoutSpaces20260624',
  type: 'webhook',
  status: 'degraded',
  config: {
    secret: '********',
    url: 'https://hooks.example/providerresearchclusterpublishingwebhookincidentchannelwithoutspaces20260624',
  },
  created_at: now,
  updated_at: now,
};

const createdWebhook = {
  id: 'channel_browser_created',
  organization_id: 'org_publishing_browser',
  name: 'Browser Webhook',
  type: 'webhook',
  status: 'active',
  config: {
    secret: 'browser-secret',
    url: 'https://hooks.example/browser',
  },
  created_at: now,
  updated_at: now,
};

const updatedOpsWebhook = {
  ...opsWebhook,
  name: 'Ops Webhook Edited',
  status: 'active',
  config: {
    secret: '********',
    url: 'https://hooks.example/ops-edited',
    endpointUrl: 'https://hooks.example/ops-edited',
    webhookUrl: 'https://hooks.example/ops-edited',
    webhook_url: 'https://hooks.example/ops-edited',
  },
};

const deletedOpsWebhook = {
  ...updatedOpsWebhook,
  status: 'disabled',
};

const recentMessages = [
  {
    id: 'channel_message_recent_browser',
    direction: 'inbound',
    status: 'recorded',
    retry_count: 0,
    raw_message: { text: 'publish browser inbound' },
    transformed_message: { role: 'user', content: [{ type: 'text', text: 'publish browser inbound' }] },
    transform_success: true,
    failure_reason: '',
    created_at: now,
  },
];

const failedMessages = [
  {
    id: 'channel_message_failed_browser',
    direction: 'outbound',
    status: 'retry_pending',
    retry_count: 2,
    next_retry_at: '2026-06-17T03:15:00Z',
    raw_message: { text: 'publish browser retry me' },
    transformed_message: { role: 'assistant', content: [{ type: 'text', text: 'publish browser retry me' }] },
    transform_success: false,
    transform_error: 'delivery failed',
    failure_reason: 'upstream 503',
    created_at: now,
  },
  {
    id: 'channel_message_failed_mobile_provider_research_cluster_without_spaces_20260624',
    direction: 'outbound',
    status: 'retry_pending',
    retry_count: 4,
    next_retry_at: '2026-06-17T03:20:00Z',
    raw_message: { text: 'providerresearchclusterpublishingdeliveryincidentwithoutspaces20260624' },
    transformed_message: {
      role: 'assistant',
      content: [{ type: 'text', text: 'providerresearchclusterpublishingdeliveryincidentwithoutspaces20260624' }],
    },
    transform_success: false,
    transform_error: 'providerresearchclusterpublishingdeliveryincidentwithoutspaces20260624',
    failure_reason: 'providerresearchclusterpublishingdeliveryincidentwithoutspaces20260624',
    created_at: now,
  },
];

const recoveredMessages = [
  ...recentMessages,
  {
    id: 'channel_message_recovered_browser',
    direction: 'outbound',
    status: 'recorded',
    retry_count: 3,
    raw_message: { text: 'publish browser retry me' },
    transformed_message: { role: 'assistant', content: [{ type: 'text', text: 'publish browser retry me' }] },
    transform_success: true,
    failure_reason: '',
    created_at: now,
  },
];

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

async function fulfillError(route: Route, message: string, status = 422) {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify({
      ok: false,
      data: null,
      error: { code: 'fixture_contract_mismatch', message },
    }),
  });
}

async function fulfillNotFound(route: Route) {
  await route.fulfill({
    status: 404,
    contentType: 'application/json',
    body: JSON.stringify({
      ok: false,
      data: null,
      error: { code: 'not_found', message: 'publishing channels fixture route not found' },
    }),
  });
}

function createPayloadMatches(payload: Record<string, unknown>) {
  const config = payload.config as Record<string, unknown> | undefined;
  return (
    payload.name === createdWebhook.name &&
    payload.type === createdWebhook.type &&
    config?.url === createdWebhook.config.url &&
    config?.secret === createdWebhook.config.secret
  );
}

function updatePayloadMatches(payload: Record<string, unknown>) {
  const config = payload.config as Record<string, unknown> | undefined;
  return (
    payload.name === updatedOpsWebhook.name &&
    payload.type === updatedOpsWebhook.type &&
    payload.status === updatedOpsWebhook.status &&
    config?.secret === '********' &&
    config?.url === updatedOpsWebhook.config.url &&
    config?.endpointUrl === updatedOpsWebhook.config.endpointUrl &&
    config?.webhookUrl === updatedOpsWebhook.config.webhookUrl &&
    config?.webhook_url === updatedOpsWebhook.config.webhook_url
  );
}

function sendPayloadMatches(payload: Record<string, unknown>) {
  const message = payload.message as Record<string, unknown> | undefined;
  return (
    message?.conversation_id === 'conversation_browser_publishing' &&
    message?.role === 'assistant' &&
    message?.text === 'Delivery recovered'
  );
}

function retryPayloadMatches(payload: Record<string, unknown>) {
  return payload.fallback_channel_id === fallbackSlack.id && payload.force === true && payload.limit === 5;
}

export async function registerPublishingChannelsRoutes(page: Page): Promise<void> {
  let createdVisible = false;
  let opsUpdated = false;
  let opsDeleted = false;
  let retriedWithFallback = false;

  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    const method = request.method();

    if (method === 'GET' && pathname === '/api/v1/auth/me') {
      await fulfillJSON(route, session);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/channels') {
      await fulfillJSON(route, [
        ...(createdVisible ? [createdWebhook] : []),
        opsDeleted ? deletedOpsWebhook : opsUpdated ? updatedOpsWebhook : opsWebhook,
        longMobileWebhook,
        fallbackSlack,
      ]);
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/channels') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!createPayloadMatches(payload)) {
        await fulfillError(route, 'publishing channel create payload did not match browser form values');
        return;
      }
      createdVisible = true;
      await fulfillJSON(route, createdWebhook, 201);
      return;
    }

    if (method === 'PUT' && pathname === `/api/v1/channels/${opsWebhook.id}`) {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!updatePayloadMatches(payload)) {
        await fulfillError(route, 'publishing channel update payload did not preserve redacted secret markers and edited endpoint');
        return;
      }
      opsUpdated = true;
      await fulfillJSON(route, updatedOpsWebhook);
      return;
    }

    if (method === 'DELETE' && pathname === `/api/v1/channels/${opsWebhook.id}`) {
      if (!opsUpdated) {
        await fulfillError(route, 'browser tried to delete the channel before saving edited config');
        return;
      }
      opsDeleted = true;
      await fulfillJSON(route, deletedOpsWebhook);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/channels/${opsWebhook.id}/send`) {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!sendPayloadMatches(payload)) {
        await fulfillError(route, 'publishing channel send payload did not match browser form values');
        return;
      }
      await fulfillJSON(route, {
        id: 'channel_message_send_browser',
        direction: 'outbound',
        status: 'recorded',
        retry_count: 0,
        raw_message: { text: 'Delivery recovered' },
        transformed_message: { role: 'assistant', content: [{ type: 'text', text: 'Delivery recovered' }] },
        transform_success: true,
        created_at: now,
      });
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/channels/${opsWebhook.id}/retry-failed-messages`) {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!retryPayloadMatches(payload)) {
        await fulfillError(route, 'publishing retry payload did not force fallback channel with expected limit');
        return;
      }
      retriedWithFallback = true;
      await fulfillJSON(route, {
        claimed: 1,
        succeeded: 1,
        failed: 0,
        permanent_failures: 0,
      });
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/channels/${opsWebhook.id}/messages`) {
      await fulfillJSON(route, retriedWithFallback ? recoveredMessages : recentMessages);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/channels/${opsWebhook.id}/failed-messages`) {
      await fulfillJSON(route, retriedWithFallback ? [] : failedMessages);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/channels/${createdWebhook.id}/messages`) {
      await fulfillJSON(route, []);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/channels/${createdWebhook.id}/failed-messages`) {
      await fulfillJSON(route, []);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/channels/${fallbackSlack.id}/messages`) {
      await fulfillJSON(route, []);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/channels/${fallbackSlack.id}/failed-messages`) {
      await fulfillJSON(route, []);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/channels/${longMobileWebhook.id}/messages`) {
      await fulfillJSON(route, []);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/channels/${longMobileWebhook.id}/failed-messages`) {
      await fulfillJSON(route, failedMessages);
      return;
    }

    await fulfillNotFound(route);
  });
}
