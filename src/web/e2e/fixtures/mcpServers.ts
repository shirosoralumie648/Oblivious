import type { Page, Route } from '@playwright/test';

const now = '2026-06-15T14:00:00Z';

const session = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_mcp_servers',
    expiresAt: '2026-06-16T14:00:00Z',
  },
  user: {
    id: 'user_mcp_operator',
    email: 'mcp-operator@example.com',
    name: 'MCP Operator',
    role: 'admin',
  },
  workspace: {
    id: 'workspace_mcp_servers',
  },
};

const localServers = [
  {
    description: 'Tenant-safe local MCP tools exposed by this server',
    id: 'local_search',
    name: 'Local Search Tools',
    toolCount: 2,
  },
  {
    description: 'Read-only release diagnostics for local operators',
    id: 'local_release',
    name: 'Local Release Diagnostics',
    toolCount: 1,
  },
];

const researchServer = {
  id: 'mcp_research',
  organizationId: 'org_mcp_servers',
  userId: session.user.id,
  name: 'Research tools',
  url: 'https://mcp.example/sse',
  hasAuthToken: false,
  status: 'disconnected',
  createdAt: now,
  updatedAt: now,
};

const mobileLongServer = {
  id: 'mcp_mobile_long',
  organizationId: 'org_mcp_servers',
  userId: session.user.id,
  name: 'ProviderResearchClusterMobileServerWithoutBreaks20260624',
  url: 'https://mcp.example/providerresearchclustermobileserverwithoutbreaks20260624/sse',
  hasAuthToken: false,
  status: 'disconnected',
  createdAt: now,
  updatedAt: now,
};

const connectedResearchServer = {
  ...researchServer,
  status: 'connected',
  lastConnectedAt: '2026-06-15T14:05:00Z',
  updatedAt: '2026-06-15T14:05:00Z',
};

const createdServer = {
  id: 'mcp_internal',
  organizationId: 'org_mcp_servers',
  userId: session.user.id,
  name: 'Internal MCP',
  url: 'https://mcp.internal/sse',
  hasAuthToken: true,
  status: 'disconnected',
  createdAt: '2026-06-15T14:10:00Z',
  updatedAt: '2026-06-15T14:10:00Z',
};

const searchTool = {
  name: 'search_docs',
  description: 'Search indexed workspace documents.',
  inputSchema: {
    type: 'object',
    properties: {
      query: { type: 'string' },
    },
    required: ['query'],
  },
};

const mobileLongTool = {
  name: 'provider_research_cluster_mobile_policy_tool_without_breaks_20260624',
  description: 'Validates mobile containment for provider research cluster policy evidence without spaces.',
  inputSchema: {
    type: 'object',
    properties: {
      evidenceId: {
        type: 'string',
      },
      query: {
        type: 'string',
      },
    },
    required: ['query'],
  },
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
      error: { code: 'not_found', message: 'MCP servers fixture route not found' },
    }),
  });
}

function addPayloadMatches(payload: Record<string, unknown>) {
  return (
    payload.name === createdServer.name &&
    payload.url === createdServer.url &&
    payload.authToken === 'secret-token'
  );
}

function executePayloadMatches(payload: Record<string, unknown>) {
  const args = payload.args as Record<string, unknown> | undefined;
  return payload.toolName === searchTool.name && args?.query === 'fusion';
}

function mobileLongExecutePayloadMatches(payload: Record<string, unknown>) {
  const args = payload.args as Record<string, unknown> | undefined;
  return (
    payload.toolName === mobileLongTool.name &&
    args?.query === 'mobile containment evidence'
  );
}

export async function registerMcpServersRoutes(page: Page): Promise<void> {
  let createdVisible = false;
  let createdDeleted = false;
  let researchConnected = false;
  let researchDisconnected = false;
  let toolsListed = false;

  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    const method = request.method();

    if (method === 'GET' && pathname === '/api/v1/auth/me') {
      await fulfillJSON(route, session);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/app/mcp-local-servers') {
      await fulfillJSON(route, localServers);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/app/mcp-servers') {
      const currentResearchServer = researchConnected && !researchDisconnected ? connectedResearchServer : researchServer;
      await fulfillJSON(route, [
        currentResearchServer,
        mobileLongServer,
        ...(createdVisible && !createdDeleted ? [createdServer] : []),
      ]);
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/app/mcp-servers') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!addPayloadMatches(payload)) {
        await fulfillError(route, 'MCP add payload did not match browser form selections');
        return;
      }
      createdVisible = true;
      createdDeleted = false;
      await fulfillJSON(route, createdServer, 201);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/app/mcp-servers/${researchServer.id}/connect`) {
      researchConnected = true;
      researchDisconnected = false;
      await fulfillJSON(route, connectedResearchServer);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/app/mcp-servers/${researchServer.id}/disconnect`) {
      if (!researchConnected) {
        await fulfillError(route, 'browser tried to disconnect MCP before connecting it');
        return;
      }
      researchDisconnected = true;
      await fulfillJSON(route, { status: 'disconnected' });
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/app/mcp-servers/${researchServer.id}/status`) {
      if (!researchDisconnected) {
        await fulfillError(route, 'browser requested MCP diagnostics before disconnect proof');
        return;
      }
      await fulfillJSON(route, { status: 'connected' });
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/app/mcp-servers/${researchServer.id}/tools`) {
      toolsListed = true;
      await fulfillJSON(route, [searchTool]);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/app/mcp-servers/${mobileLongServer.id}/tools`) {
      await fulfillJSON(route, [mobileLongTool]);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/app/mcp-servers/${researchServer.id}/execute`) {
      if (!toolsListed) {
        await fulfillError(route, 'browser tried to execute MCP tool before listing tools');
        return;
      }
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!executePayloadMatches(payload)) {
        await fulfillError(route, 'MCP tool execute payload did not match browser form selections');
        return;
      }
      await fulfillJSON(route, { content: 'Found fusion design details.' });
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/app/mcp-servers/${mobileLongServer.id}/execute`) {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!mobileLongExecutePayloadMatches(payload)) {
        await fulfillError(route, 'MCP long tool execute payload did not match browser form selections');
        return;
      }
      await fulfillJSON(route, {
        content:
          'provider_research_cluster_mobile_policy_tool_without_breaks_20260624_mobile_evidence_' +
          'providerresearchclustermobilecontainmentwithoutbreaks20260624'
      });
      return;
    }

    if (method === 'DELETE' && pathname === `/api/v1/app/mcp-servers/${createdServer.id}`) {
      if (!createdVisible) {
        await fulfillError(route, 'browser tried to delete an MCP server before creating it');
        return;
      }
      createdDeleted = true;
      await fulfillJSON(route, { status: 'deleted' });
      return;
    }

    await fulfillNotFound(route);
  });
}
