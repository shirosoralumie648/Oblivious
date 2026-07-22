// @ts-expect-error Vitest runs in Node, while the browser tsconfig intentionally omits Node types.
import { createHash } from 'node:crypto';
import type { ComponentProps, ReactNode } from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, expectTypeOf, it, vi } from 'vitest';

import { getAppReadinessCapabilitiesOperationContract } from '@/generated/operation-contracts.generated';
import type { HttpClient } from '@/services/http/client';

const appContext = vi.hoisted(() => ({
  authState: {
    status: 'authenticated' as 'idle' | 'loading' | 'authenticated' | 'unauthenticated',
    user: { id: 'user_1' } as { id: string } | null
  }
}));

vi.mock('@/app/providers', () => ({
  useAppContext: () => appContext
}));

import {
  createReleaseProjectionApi,
  releaseCapabilityProjection,
  releaseProjectionDigest,
  ReleaseProjectionProvider,
  useReleaseProjection,
  type AppCapabilityProjectionResponse
} from './releaseProjection';

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

it('keeps the exported runtime response readonly at the TypeScript boundary', () => {
  expectTypeOf<AppCapabilityProjectionResponse['capabilities']>().toEqualTypeOf<readonly {
    readonly capabilityId: string;
    readonly disposition: 'committed' | 'conditional';
    readonly availability: 'enabled' | 'disabled' | 'blocked';
    readonly enabled: boolean;
  }[]>();
});
