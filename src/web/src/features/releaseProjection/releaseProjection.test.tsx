// @ts-expect-error Vitest runs in Node, while the browser tsconfig intentionally omits Node types.
import { createHash } from 'node:crypto';
import type { ComponentProps, ReactNode } from 'react';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter, Route, RouterProvider, Routes } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, expectTypeOf, it, vi } from 'vitest';

import { getAppReadinessCapabilitiesOperationContract } from '@/generated/operation-contracts.generated';
import type { HttpClient } from '@/services/http/client';

const appContext = vi.hoisted(() => ({
  authState: {
    status: 'authenticated' as 'idle' | 'loading' | 'authenticated' | 'unauthenticated',
    user: { id: 'user_1' } as { id: string } | null
  }
}));

const selectorApiMocks = vi.hoisted(() => ({
  getAgentTools: vi.fn(),
  getConversationConfig: vi.fn(),
  listAgents: vi.fn(),
  listConversations: vi.fn(),
  listKnowledgeBases: vi.fn(),
  listMessages: vi.fn(),
  listModels: vi.fn(),
  listPersonas: vi.fn(),
  updateAgent: vi.fn(),
  updateConversationConfig: vi.fn()
}));

vi.mock('@/app/providers', () => ({
  useAppContext: () => appContext
}));

vi.mock('@/features/chat/api', () => ({
  createChatApi: () => ({
    getConversationConfig: selectorApiMocks.getConversationConfig,
    listConversations: selectorApiMocks.listConversations,
    listMessages: selectorApiMocks.listMessages,
    listModels: selectorApiMocks.listModels,
    listPersonas: selectorApiMocks.listPersonas,
    updateConversationConfig: selectorApiMocks.updateConversationConfig
  }),
  createConversationRealtimeSocket: () => ({
    close: vi.fn(),
    sendTyping: vi.fn()
  })
}));

vi.mock('@/features/knowledge/api', () => ({
  createKnowledgeApi: () => ({
    listKnowledgeBases: selectorApiMocks.listKnowledgeBases
  })
}));

vi.mock('@/features/tasks/api', () => ({
  createTasksApi: () => ({})
}));

vi.mock('@/features/agents/agentsApi', () => ({
  createAgentsApi: () => ({
    getAgentTools: selectorApiMocks.getAgentTools,
    listAgents: selectorApiMocks.listAgents,
    updateAgent: selectorApiMocks.updateAgent
  })
}));

import {
  createReleaseProjectionApi,
  releaseCapabilityProjection,
  releaseProjectionDigest,
  ReleaseProjectionProvider,
  useReleaseProjection,
  type AppCapabilityProjectionResponse
} from './releaseProjection';
import { createAppRouter } from '@/app/router';
import { AdminSidebar } from '@/features/layouts/AdminSidebar';
import { ConsoleLayout } from '@/features/layouts/ConsoleLayout';
import { WorkspaceLayout } from '@/features/layouts/WorkspaceLayout';
import { HomePage } from '@/routes/marketing/HomePage';
import { AgentsPage } from '@/routes/workspace/AgentsPage';
import { ChatPage } from '@/routes/workspace/ChatPage';

const baseIdentity = {
  sourceTree: 'a'.repeat(40),
  contractDigest: `sha256:${'b'.repeat(64)}`,
  deploymentProfile: 'monolith'
};

type MutableProjectionResponse = {
  releaseIdentity: Record<string, unknown>;
  generation: number;
  projectionDigest: string;
  capabilities: Array<Record<string, unknown>>;
  [key: string]: unknown;
};

function runtimeDigest(response: Pick<MutableProjectionResponse, 'releaseIdentity' | 'generation' | 'capabilities'>) {
  const payload = {
    identity: {
      sourceTree: response.releaseIdentity.sourceTree,
      contractDigest: response.releaseIdentity.contractDigest,
      deploymentProfile: response.releaseIdentity.deploymentProfile
    },
    generation: response.generation,
    capabilities: response.capabilities.map((capability) => ({
      capabilityId: capability.capabilityId,
      disposition: capability.disposition,
      availability: capability.availability,
      enabled: capability.enabled
    }))
  };
  return `sha256:${createHash('sha256').update(JSON.stringify(payload)).digest('hex')}`;
}

function projectionResponse(
  mutate?: (response: MutableProjectionResponse) => void,
  options: { recomputeDigest?: boolean } = {}
): MutableProjectionResponse {
  const response: MutableProjectionResponse = {
    releaseIdentity: { ...baseIdentity },
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
  mutate?.(response);
  if (options.recomputeDigest !== false) {
    response.projectionDigest = runtimeDigest(response);
  }
  return response;
}

function jsonResponse(value: unknown, status = 200) {
  return new Response(JSON.stringify(value), {
    status,
    headers: { 'Content-Type': 'application/json' }
  });
}

function clientReturning(...responses: Response[]) {
  const get = vi.fn();
  for (const response of responses) {
    get.mockResolvedValueOnce(response);
  }
  return {
    client: { get } as unknown as HttpClient,
    get
  };
}

function ProjectionProbe({ capabilityId = 'mcp.network_execution' }: { capabilityId?: string }) {
  const projection = useReleaseProjection();
  return (
    <div>
      <span data-testid="projection-status">{projection.status}</span>
      <span data-testid="projection-generation">{projection.generation ?? 'none'}</span>
      <span data-testid="capability-enabled">{String(projection.isCapabilityEnabled(capabilityId))}</span>
    </div>
  );
}

describe('createReleaseProjectionApi', () => {
  it('uses the exact generated operation symbol and accepts a valid identity-bound runtime digest', async () => {
    const response = projectionResponse();
    const { client, get } = clientReturning(jsonResponse(response));

    const loaded = await createReleaseProjectionApi(client).load();

    expect(loaded).toEqual(response);
    expect(response.projectionDigest).not.toBe(releaseProjectionDigest);
    expect(get).toHaveBeenCalledTimes(1);
    expect(get.mock.calls[0]?.[0]).toBe('/api/v1/app/readiness/capabilities');
    expect(get.mock.calls[0]?.[1]).toBeUndefined();
    expect(get.mock.calls[0]?.[2]?.operation).toBe(getAppReadinessCapabilitiesOperationContract);
  });

  it.each([
    ['duplicate capability', (response: MutableProjectionResponse) => response.capabilities.push({ ...response.capabilities[0] })],
    ['unknown capability', (response: MutableProjectionResponse) => { response.capabilities[0].capabilityId = 'caller.unknown'; }],
    ['excluded capability', (response: MutableProjectionResponse) => { response.capabilities[0].capabilityId = 'sandbox.code_execution'; }],
    ['missing capability', (response: MutableProjectionResponse) => { response.capabilities.pop(); }],
    ['unsorted inventory', (response: MutableProjectionResponse) => { response.capabilities.reverse(); }]
  ])('rejects %s instead of publishing a partial generated join', async (_name, mutate) => {
    const { client } = clientReturning(jsonResponse(projectionResponse(mutate)));
    await expect(createReleaseProjectionApi(client).load()).rejects.toThrow(/capability|projection/i);
  });

  it.each([
    ['unknown availability', (response: MutableProjectionResponse) => { response.capabilities[0].availability = 'unknown'; }],
    ['inconsistent enabled flag', (response: MutableProjectionResponse) => { response.capabilities[0].enabled = false; }],
    ['generated disposition mismatch', (response: MutableProjectionResponse) => { response.capabilities[0].disposition = 'conditional'; }],
    ['Admin inventory response field', (response: MutableProjectionResponse) => { response.checkedAt = '2026-07-22T00:00:00Z'; }],
    ['Admin inventory capability field', (response: MutableProjectionResponse) => { response.capabilities[0].reasonCode = 'secret_probe'; }]
  ])('rejects %s without an Admin or permissive fallback', async (_name, mutate) => {
    const { client } = clientReturning(jsonResponse(projectionResponse(mutate)));
    await expect(createReleaseProjectionApi(client).load()).rejects.toThrow(/invalid|capability|disposition|enabled/i);
  });

  it.each([
    ['wrong source tree', (response: MutableProjectionResponse) => { response.releaseIdentity.sourceTree = 'not-a-tree'; }],
    ['wrong contract digest', (response: MutableProjectionResponse) => { response.releaseIdentity.contractDigest = `sha256:${'X'.repeat(64)}`; }],
    ['wrong deployment profile', (response: MutableProjectionResponse) => { response.releaseIdentity.deploymentProfile = 'microservices'; }]
  ])('rejects %s before publishing availability', async (_name, mutate) => {
    const { client } = clientReturning(jsonResponse(projectionResponse(mutate)));
    await expect(createReleaseProjectionApi(client).load()).rejects.toThrow(/identity|profile/i);
  });

  it('rejects a digest mutation even when the response fields otherwise match', async () => {
    const response = projectionResponse(undefined, { recomputeDigest: false });
    response.projectionDigest = `sha256:${'0'.repeat(64)}`;
    const { client } = clientReturning(jsonResponse(response));
    await expect(createReleaseProjectionApi(client).load()).rejects.toThrow(/digest/i);
  });

  it('rejects generation regression and release identity drift across one authenticated session', async () => {
    const baseline = projectionResponse();
    const regressed = projectionResponse((response) => { response.generation = 6; });
    const drifted = projectionResponse((response) => { response.releaseIdentity.sourceTree = 'c'.repeat(40); });
    const generationClient = clientReturning(jsonResponse(baseline), jsonResponse(regressed));
    const identityClient = clientReturning(jsonResponse(baseline), jsonResponse(drifted));
    const generationApi = createReleaseProjectionApi(generationClient.client);
    const identityApi = createReleaseProjectionApi(identityClient.client);

    await expect(generationApi.load()).resolves.toMatchObject({ generation: 7 });
    await expect(generationApi.load()).rejects.toThrow(/regressed/i);
    await expect(identityApi.load()).resolves.toMatchObject({ generation: 7 });
    await expect(identityApi.load()).rejects.toThrow(/identity changed/i);
  });
});

describe('ReleaseProjectionProvider', () => {
  beforeEach(() => {
    appContext.authState = {
      status: 'authenticated',
      user: { id: 'user_1' }
    };
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('accepts children only and publishes enabled availability from the authenticated endpoint', async () => {
    type ProviderProps = ComponentProps<typeof ReleaseProjectionProvider>;
    expectTypeOf<ProviderProps>().toEqualTypeOf<{ children: ReactNode }>();
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(projectionResponse())));

    render(
      <ReleaseProjectionProvider>
        <ProjectionProbe />
      </ReleaseProjectionProvider>
    );

    expect(screen.getByTestId('projection-status')).toHaveTextContent('loading');
    await waitFor(() => expect(screen.getByTestId('projection-status')).toHaveTextContent('ready'));
    expect(screen.getByTestId('projection-generation')).toHaveTextContent('7');
    expect(screen.getByTestId('capability-enabled')).toHaveTextContent('true');
    expect(fetch).toHaveBeenCalledWith('/api/v1/app/readiness/capabilities', expect.objectContaining({ method: 'GET' }));
  });

  it.each([
    ['loading', 'loading'],
    ['idle', 'loading'],
    ['unauthenticated', 'unauthenticated']
  ] as const)('fails closed while auth is %s', async (authStatus, projectionStatus) => {
    appContext.authState = {
      status: authStatus,
      user: authStatus === 'unauthenticated' ? null : { id: 'user_1' }
    };
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);

    render(
      <ReleaseProjectionProvider>
        <ProjectionProbe />
      </ReleaseProjectionProvider>
    );

    expect(screen.getByTestId('projection-status')).toHaveTextContent(projectionStatus);
    expect(screen.getByTestId('capability-enabled')).toHaveTextContent('false');
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('maps 401 and malformed runtime responses to closed provider states', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ error: { code: 'unauthorized', message: 'unauthorized' } }, 401))
      .mockResolvedValueOnce(jsonResponse(projectionResponse((response) => {
        response.capabilities[0].availability = 'unknown';
      })));
    vi.stubGlobal('fetch', fetchMock);

    const first = render(
      <ReleaseProjectionProvider>
        <ProjectionProbe />
      </ReleaseProjectionProvider>
    );
    await waitFor(() => expect(screen.getByTestId('projection-status')).toHaveTextContent('unauthenticated'));
    expect(screen.getByTestId('capability-enabled')).toHaveTextContent('false');
    first.unmount();

    render(
      <ReleaseProjectionProvider>
        <ProjectionProbe />
      </ReleaseProjectionProvider>
    );
    await waitFor(() => expect(screen.getByTestId('projection-status')).toHaveTextContent('unavailable'));
    expect(screen.getByTestId('capability-enabled')).toHaveTextContent('false');
  });
});

describe('release projection product exposure', () => {
  beforeEach(() => {
    appContext.authState = {
      status: 'authenticated',
      user: { id: 'user_1' }
    };
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('hides disabled conditional workspace navigation and restores it only when the current response enables it', async () => {
    const disabled = projectionResponse((response) => {
      const capability = response.capabilities.find((item) => item.capabilityId === 'mcp.custom_execution')!;
      capability.availability = 'disabled';
      capability.enabled = false;
    });
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(disabled));
    vi.stubGlobal('fetch', fetchMock);

    const first = render(
      <ReleaseProjectionProvider>
        <MemoryRouter initialEntries={['/chat']} >
          <Routes>
            <Route element={<WorkspaceLayout />}>
              <Route path="/chat" element={<main>Chat route</main>} />
            </Route>
          </Routes>
        </MemoryRouter>
      </ReleaseProjectionProvider>
    );

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    expect(screen.getByRole('link', { name: 'Chat' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'MCP Servers' })).not.toBeInTheDocument();
    first.unmount();

    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(projectionResponse())));
    render(
      <ReleaseProjectionProvider>
        <MemoryRouter initialEntries={['/chat']} >
          <Routes>
            <Route element={<WorkspaceLayout />}>
              <Route path="/chat" element={<main>Chat route</main>} />
            </Route>
          </Routes>
        </MemoryRouter>
      </ReleaseProjectionProvider>
    );
    await waitFor(() => expect(screen.getByRole('link', { name: 'MCP Servers' })).toBeInTheDocument());
  });

  it('rejects a disabled conditional direct URL at the router boundary before mounting page content', async () => {
    const disabled = projectionResponse((response) => {
      const capability = response.capabilities.find((item) => item.capabilityId === 'mcp.custom_execution')!;
      capability.availability = 'blocked';
      capability.enabled = false;
    });
    const fetchMock = vi.fn(async () => jsonResponse(disabled));
    vi.stubGlobal('fetch', fetchMock);
    const router = createAppRouter(['/mcp-servers']);

    render(<RouterProvider  router={router} />);

    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith('/api/v1/app/readiness/capabilities', expect.any(Object)));
    expect(screen.getByRole('status')).toHaveTextContent('currently unavailable');
    expect(screen.queryByRole('heading', { name: /mcp/i })).not.toBeInTheDocument();
  });

  it('keeps committed console and admin navigation visible through generated dispositions', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(projectionResponse())));
    const consoleRender = render(
      <ReleaseProjectionProvider>
        <MemoryRouter initialEntries={['/console']} >
          <ConsoleLayout />
        </MemoryRouter>
      </ReleaseProjectionProvider>
    );
    await waitFor(() => expect(screen.getByRole('link', { name: 'Billing' })).toBeInTheDocument());
    expect(screen.getByRole('link', { name: 'Models' })).toBeInTheDocument();
    consoleRender.unmount();

    render(
      <ReleaseProjectionProvider>
        <MemoryRouter initialEntries={['/admin']} >
          <AdminSidebar />
        </MemoryRouter>
      </ReleaseProjectionProvider>
    );
    await waitFor(() => expect(screen.getByRole('link', { name: 'Dashboard' })).toBeInTheDocument());
    expect(screen.getByRole('link', { name: 'Billing' })).toBeInTheDocument();
  });

  it('renders only generated committed public marketing links without authenticated or Admin inventory input', () => {
    appContext.authState = { status: 'unauthenticated', user: null };
    render(
      <MemoryRouter >
        <HomePage />
      </MemoryRouter>
    );

    expect(screen.getByRole('link', { name: 'Sign in' })).toHaveAttribute('href', '/login');
    expect(screen.getByText('Relay chat').closest('a')).toHaveAttribute('href', '/chat');
    expect(screen.getByText('Marketplace').closest('a')).toHaveAttribute('href', '/marketplace');
  });
});

describe('release projection catalog selectors', () => {
  beforeEach(() => {
    appContext.authState = {
      status: 'authenticated',
      user: { id: 'user_1' }
    };
    Object.values(selectorApiMocks).forEach((mock) => mock.mockReset());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('keeps only exact enabled model capability identities selectable and omits capabilityId from settings', async () => {
    const response = projectionResponse((current) => {
      const blocked = current.capabilities.find((item) => item.capabilityId === 'relay.provider_inference')!;
      blocked.availability = 'blocked';
      blocked.enabled = false;
    });
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(response)));
    selectorApiMocks.listConversations.mockResolvedValue([{ id: 'conversation_1', title: 'Projection test' }]);
    selectorApiMocks.listKnowledgeBases.mockResolvedValue([]);
    selectorApiMocks.listMessages.mockResolvedValue([]);
    selectorApiMocks.listModels.mockResolvedValue([
      { capabilityId: 'chat.conversation_use', id: 'enabled-chat', label: 'Enabled chat' },
      { capabilityId: 'relay.provider_inference', id: 'blocked-chat', label: 'Blocked chat' },
      { capabilityId: 'caller.unknown', id: 'unknown-chat', label: 'Unknown chat' },
      { id: 'missing-chat', label: 'Missing capability' }
    ]);
    selectorApiMocks.listPersonas.mockResolvedValue([]);
    selectorApiMocks.getConversationConfig.mockResolvedValue({
      conversationId: 'conversation_1',
      knowledgeBaseIds: [],
      maxOutputTokens: 1024,
      modelId: 'historical-chat',
      systemPromptOverride: '',
      temperature: 1,
      toolsEnabled: false
    });
    selectorApiMocks.updateConversationConfig.mockImplementation(async (conversationId, payload) => ({
      conversationId,
      ...payload
    }));

    render(
      <ReleaseProjectionProvider>
        <MemoryRouter initialEntries={['/chat/conversation_1']} >
          <Routes>
            <Route path="/chat/:conversationId" element={<ChatPage />} />
          </Routes>
        </MemoryRouter>
      </ReleaseProjectionProvider>
    );

    const modelSelect = await screen.findByLabelText('Conversation model');
    await waitFor(() => expect(within(modelSelect).getByRole('option', { name: 'Enabled chat' })).toBeInTheDocument());
    expect(within(modelSelect).queryByRole('option', { name: 'Blocked chat' })).not.toBeInTheDocument();
    expect(within(modelSelect).queryByRole('option', { name: 'Unknown chat' })).not.toBeInTheDocument();
    expect(within(modelSelect).queryByRole('option', { name: 'Missing capability' })).not.toBeInTheDocument();
    expect(within(modelSelect).queryByRole('option', { name: 'historical-chat' })).not.toBeInTheDocument();

    fireEvent.change(modelSelect, { target: { value: 'enabled-chat' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save conversation settings' }));

    await waitFor(() => expect(selectorApiMocks.updateConversationConfig).toHaveBeenCalledTimes(1));
    const settingsPayload = selectorApiMocks.updateConversationConfig.mock.calls[0]?.[1];
    expect(settingsPayload).toMatchObject({ modelId: 'enabled-chat' });
    expect(settingsPayload).not.toHaveProperty('capabilityId');
  });

  it('keeps only exact enabled tool capability identities actionable and omits capabilityId from updates', async () => {
    const response = projectionResponse((current) => {
      const blocked = current.capabilities.find((item) => item.capabilityId === 'mcp.tool_execution')!;
      blocked.availability = 'disabled';
      blocked.enabled = false;
    });
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(response)));
    const agent = {
      config: { approvalMode: 'tiered' },
      description: 'Projection test agent.',
      id: 'agent_1',
      isPublic: false,
      model: 'gpt-4o-mini',
      name: 'Projection Agent',
      systemPrompt: '',
      tools: []
    };
    selectorApiMocks.listAgents.mockResolvedValue([agent]);
    selectorApiMocks.getAgentTools.mockResolvedValue([
      { capabilityId: 'mcp.network_execution', name: 'enabled_lookup', toolType: 'mcp' },
      { capabilityId: 'mcp.tool_execution', name: 'blocked_tool', toolType: 'builtin' },
      { capabilityId: 'caller.unknown', name: 'unknown_tool', toolType: 'builtin' },
      { name: 'missing_capability', toolType: 'builtin' }
    ]);
    selectorApiMocks.updateAgent.mockImplementation(async (_agentId, payload) => ({
      ...agent,
      ...payload
    }));

    render(
      <ReleaseProjectionProvider>
        <MemoryRouter >
          <AgentsPage />
        </MemoryRouter>
      </ReleaseProjectionProvider>
    );

    expect(await screen.findByRole('button', { name: 'Projection Agent' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Load tool catalog' }));

    const enableButton = await screen.findByRole('button', { name: 'Enable tool enabled_lookup' });
    expect(screen.queryByRole('button', { name: 'Enable tool blocked_tool' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Enable tool unknown_tool' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Enable tool missing_capability' })).not.toBeInTheDocument();

    fireEvent.click(enableButton);

    await waitFor(() => expect(selectorApiMocks.updateAgent).toHaveBeenCalledTimes(1));
    const updatePayload = selectorApiMocks.updateAgent.mock.calls[0]?.[1];
    expect(updatePayload.tools).toEqual([
      expect.objectContaining({ enabled: true, name: 'enabled_lookup', type: 'mcp' })
    ]);
    expect(updatePayload.tools[0]).not.toHaveProperty('capabilityId');
  });
});

it('keeps the exported runtime response readonly at the TypeScript boundary', () => {
  expectTypeOf<AppCapabilityProjectionResponse['capabilities']>().toEqualTypeOf<readonly {
    readonly capabilityId: string;
    readonly disposition: 'committed' | 'conditional';
    readonly availability: 'enabled' | 'disabled' | 'blocked';
    readonly enabled: boolean;
  }[]>();
});
