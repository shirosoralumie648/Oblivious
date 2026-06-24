import type { Page, Route } from '@playwright/test';

const now = '2026-06-17T08:00:00Z';

const adminSession = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_admin_channels',
    expiresAt: '2026-06-18T08:00:00Z',
  },
  user: {
    id: 'user_admin_channels',
    email: 'admin-channels@example.com',
    name: 'Channels Admin',
    role: 'admin',
  },
  workspace: {
    id: 'workspace_admin_channels',
  },
};

const openAIChannel = {
  id: 'channel_browser_openai',
  organizationID: 'org_browser_channels',
  name: 'OpenAI Browser Primary',
  provider: 'openai',
  baseURL: 'https://api.openai.browser.test/v1',
  models: ['gpt-4o', 'gpt-4.1-mini', 'o3-mini'],
  groups: ['default', 'enterprise'],
  rpm: 120,
  tpm: 120000,
  priority: 1,
  estimatedCostPer1K: 0.005,
  costMultiplier: 1,
  weight: 100,
  enabled: true,
  status: 'online',
  latency: 112,
  createdAt: now,
  updatedAt: now,
};

const createChannelPayload = {
  name: 'Browser OpenRouter',
  provider: 'openrouter',
  apiKey: 'sk-browser-openrouter',
  baseURL: 'https://openrouter.browser.test/api/v1',
  models: ['gpt-4o', 'gpt-4.1-mini'],
  groups: ['default', 'enterprise'],
  rpmLimit: 240,
  tpmLimit: 240000,
  priority: 2,
  estimatedCostPer1K: 0.0042,
  costMultiplier: 1.15,
  weight: 80,
};

const updateChannelPayload = {
  name: 'Browser OpenRouter Updated',
  provider: 'openrouter',
  baseURL: 'https://openrouter.browser.test/api/v2',
  models: ['gpt-4o', 'gpt-4.1-mini', 'gpt-4.1'],
  groups: ['default', 'enterprise', 'qa'],
  rpmLimit: 360,
  tpmLimit: 360000,
  priority: 1,
  estimatedCostPer1K: 0.0037,
  costMultiplier: 1.05,
  weight: 90,
};

function channelFromPayload(id: string, payload: typeof createChannelPayload | typeof updateChannelPayload) {
  return {
    id,
    organizationID: 'org_browser_channels',
    name: payload.name,
    provider: payload.provider,
    baseURL: payload.baseURL,
    models: [...payload.models],
    groups: [...payload.groups],
    rpm: payload.rpmLimit,
    tpm: payload.tpmLimit,
    priority: payload.priority,
    estimatedCostPer1K: payload.estimatedCostPer1K,
    costMultiplier: payload.costMultiplier,
    weight: payload.weight,
    enabled: true,
    status: 'online',
    latency: 93,
    createdAt: now,
    updatedAt: now,
  };
}

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

function stringArrayMatches(value: unknown, expected: string[]) {
  return Array.isArray(value) && value.length === expected.length && value.every((item, index) => item === expected[index]);
}

function channelCreatePayloadMatches(payload: Record<string, unknown>) {
  return (
    payload.name === createChannelPayload.name &&
    payload.provider === createChannelPayload.provider &&
    payload.apiKey === createChannelPayload.apiKey &&
    payload.baseURL === createChannelPayload.baseURL &&
    stringArrayMatches(payload.models, createChannelPayload.models) &&
    stringArrayMatches(payload.groups, createChannelPayload.groups) &&
    payload.rpmLimit === createChannelPayload.rpmLimit &&
    payload.tpmLimit === createChannelPayload.tpmLimit &&
    payload.priority === createChannelPayload.priority &&
    payload.estimatedCostPer1K === createChannelPayload.estimatedCostPer1K &&
    payload.costMultiplier === createChannelPayload.costMultiplier &&
    payload.weight === createChannelPayload.weight
  );
}

function channelUpdatePayloadMatches(payload: Record<string, unknown>) {
  return (
    !('apiKey' in payload) &&
    payload.name === updateChannelPayload.name &&
    payload.provider === updateChannelPayload.provider &&
    payload.baseURL === updateChannelPayload.baseURL &&
    stringArrayMatches(payload.models, updateChannelPayload.models) &&
    stringArrayMatches(payload.groups, updateChannelPayload.groups) &&
    payload.rpmLimit === updateChannelPayload.rpmLimit &&
    payload.tpmLimit === updateChannelPayload.tpmLimit &&
    payload.priority === updateChannelPayload.priority &&
    payload.estimatedCostPer1K === updateChannelPayload.estimatedCostPer1K &&
    payload.costMultiplier === updateChannelPayload.costMultiplier &&
    payload.weight === updateChannelPayload.weight
  );
}

function modelUpdatePreview(channelId: string) {
  return {
    id: channelId,
    currentModels: ['gpt-4o', 'gpt-4.1-mini', 'gpt-4.1'],
    upstreamModels: ['gpt-4o', 'gpt-4.1-mini', 'gpt-4.1', 'gpt-4.1-nano'],
    added: ['gpt-4.1-nano'],
    removed: [],
    unchanged: ['gpt-4o', 'gpt-4.1-mini', 'gpt-4.1'],
    testResult: {
      success: true,
      latency: 93,
      latencyMs: 93,
      provider: 'openrouter',
      models: ['gpt-4o', 'gpt-4.1-mini', 'gpt-4.1', 'gpt-4.1-nano'],
      balance: { amount: 48.75, currency: 'USD', source: 'fixture' },
      health: { status: 'online', message: 'Browser fixture online', checkedAt: now },
    },
  };
}

export async function registerAdminChannelsRoutes(page: Page): Promise<void> {
  let channels = [openAIChannel];
  let createdChannel: ReturnType<typeof channelFromPayload> | null = null;

  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    const method = request.method();

    if (method === 'GET' && pathname === '/api/v1/auth/me') {
      await fulfillJSON(route, adminSession);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/channel-providers') {
      await fulfillJSON(route, {
        providers: [
          {
            id: 'openai',
            displayName: 'OpenAI',
            kind: 'openai_compatible',
            status: 'supported',
            defaultBaseURL: 'https://api.openai.com/v1',
          },
          {
            id: 'openrouter',
            displayName: 'OpenRouter',
            kind: 'openai_compatible',
            status: 'supported',
            defaultBaseURL: 'https://openrouter.ai/api/v1',
          },
        ],
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/channels') {
      if (url.searchParams.get('sort') !== 'name:asc' || url.searchParams.get('limit') !== '50') {
        await fulfillError(route, 'admin channels list query did not preserve sort=name:asc and limit=50');
        return;
      }
      await fulfillJSON(route, { channels, total: channels.length });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/channels/stats') {
      await fulfillJSON(route, {
        stats: [
          {
            channelID: openAIChannel.id,
            rpmCurrent: 17,
            tpmCurrent: 2048,
            totalRequests: 456,
            successCount: 448,
            failureCount: 8,
            avgLatencyMs: 112,
            affinityConversationCount: 5,
          },
        ],
      });
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/admin/channels') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!channelCreatePayloadMatches(payload)) {
        await fulfillError(route, 'channel create payload did not preserve arrays, numeric coercion, and raw apiKey');
        return;
      }
      createdChannel = channelFromPayload('channel_browser_openrouter', createChannelPayload);
      channels = [openAIChannel, createdChannel];
      await fulfillJSON(route, createdChannel, 201);
      return;
    }

    if (method === 'PUT' && pathname === '/api/v1/admin/channels/channel_browser_openrouter') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!channelUpdatePayloadMatches(payload)) {
        await fulfillError(route, 'channel update payload did not omit blank apiKey or preserve normalized fields');
        return;
      }
      createdChannel = channelFromPayload('channel_browser_openrouter', updateChannelPayload);
      channels = channels.map((channel) => (channel.id === createdChannel?.id ? createdChannel : channel));
      await fulfillJSON(route, createdChannel);
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/admin/channels/channel_browser_openrouter/test') {
      await fulfillJSON(route, {
        success: true,
        latency: 93,
        latencyMs: 93,
        provider: 'openrouter',
        models: ['gpt-4o', 'gpt-4.1-mini', 'gpt-4.1'],
        balance: { amount: 48.75, currency: 'USD', source: 'fixture' },
        health: { status: 'online', message: 'Browser fixture online', checkedAt: now },
      });
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/admin/channels/channel_browser_openrouter/model-updates/detect') {
      await fulfillJSON(route, modelUpdatePreview('channel_browser_openrouter'));
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/admin/channels/channel_browser_openrouter/model-updates/apply') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (payload.mode !== 'merge') {
        await fulfillError(route, 'channel model update apply did not send mode=merge');
        return;
      }
      const appliedModels = ['gpt-4o', 'gpt-4.1-mini', 'gpt-4.1', 'gpt-4.1-nano'];
      if (createdChannel) {
        createdChannel = { ...createdChannel, models: appliedModels, updatedAt: now };
        channels = channels.map((channel) => (channel.id === createdChannel?.id ? createdChannel : channel));
      }
      await fulfillJSON(route, {
        channel: createdChannel,
        preview: modelUpdatePreview('channel_browser_openrouter'),
        mode: 'merge',
        appliedModels,
      });
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/admin/channels/batch') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      const ids = payload.ids;
      if (!stringArrayMatches(ids, ['channel_browser_openai']) || payload.action !== 'disable') {
        await fulfillError(route, 'channel batch payload did not send the selected OpenAI channel disable action');
        return;
      }
      channels = channels.map((channel) =>
        channel.id === 'channel_browser_openai' ? { ...channel, enabled: false, status: 'offline' } : channel
      );
      await fulfillJSON(route, { status: 'ok' });
      return;
    }

    if (method === 'DELETE' && pathname === '/api/v1/admin/channels/channel_browser_openrouter') {
      if (request.postData() !== null) {
        await fulfillError(route, 'channel delete request should not send a JSON body');
        return;
      }
      if (createdChannel?.name !== updateChannelPayload.name) {
        await fulfillError(route, 'browser tried to delete the channel before saving edited config');
        return;
      }
      createdChannel = null;
      channels = channels.filter((channel) => channel.id !== 'channel_browser_openrouter');
      await fulfillJSON(route, { status: 'deleted' });
      return;
    }

    await fulfillError(route, `fixture route not implemented for ${method} ${pathname}`, 404);
  });
}
