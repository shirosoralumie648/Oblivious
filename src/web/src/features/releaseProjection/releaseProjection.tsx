import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode
} from 'react';
import { Outlet } from 'react-router-dom';

import { useAppContext } from '@/app/providers';
import { getAppReadinessCapabilitiesOperationContract } from '@/generated/operation-contracts.generated';
import {
  releaseCapabilityProjection,
  releaseProjectionDigest,
  releaseSurfaceProjection
} from '@/generated/release-projection.generated';
import {
  createHttpClient,
  noneRequestEncoder,
  rawResponseDecoder,
  type HttpClient,
  type OperationTransportContract
} from '@/services/http/client';
import { HttpError } from '@/services/http/errors';

export type AppReleaseIdentity = {
  readonly sourceTree: string;
  readonly contractDigest: string;
  readonly deploymentProfile: string;
};

export type AppCapabilityAvailability = {
  readonly capabilityId: string;
  readonly disposition: 'committed' | 'conditional';
  readonly availability: 'enabled' | 'disabled' | 'blocked';
  readonly enabled: boolean;
};

export type AppCapabilityProjectionResponse = {
  readonly releaseIdentity: AppReleaseIdentity;
  readonly generation: number;
  readonly projectionDigest: string;
  readonly capabilities: readonly AppCapabilityAvailability[];
};

export type ReleaseProjectionStatus =
  | 'loading'
  | 'ready'
  | 'unauthenticated'
  | 'unavailable';

export type ReleaseProjectionState = {
  readonly status: ReleaseProjectionStatus;
  readonly releaseIdentity: AppReleaseIdentity | null;
  readonly generation: number | null;
  readonly projectionDigest: string | null;
  readonly generatedProjectionDigest: typeof releaseProjectionDigest;
  readonly capabilityById: Readonly<Record<string, AppCapabilityAvailability>>;
  readonly isCapabilityEnabled: (capabilityId: string) => boolean;
};

export type ReleaseProjectionApi = {
  load: () => Promise<AppCapabilityProjectionResponse>;
};

const readinessTransport: OperationTransportContract<Response> = {
  operation: getAppReadinessCapabilitiesOperationContract,
  requestEncoder: noneRequestEncoder(getAppReadinessCapabilitiesOperationContract),
  responseDecoder: rawResponseDecoder(getAppReadinessCapabilitiesOperationContract, 200)
};

const sha256Pattern = /^sha256:[0-9a-f]{64}$/;
const sourceTreePattern = /^[0-9a-f]{40}$/;
const expectedDeploymentProfile = 'monolith';
const emptyCapabilityById = Object.freeze({}) as Readonly<Record<string, AppCapabilityAvailability>>;

type GeneratedCapability = (typeof releaseCapabilityProjection)[number];

const generatedCapabilityById = (() => {
  const result = new Map<string, GeneratedCapability>();
  for (const capability of releaseCapabilityProjection) {
    if (result.has(capability.capabilityId)) {
      throw new TypeError(`Generated release projection contains duplicate capability ${capability.capabilityId}.`);
    }
    result.set(capability.capabilityId, capability);
  }
  return result;
})();

const expectedAppCapabilityIds = releaseCapabilityProjection
  .filter((capability) => capability.disposition !== 'excluded')
  .map((capability) => capability.capabilityId)
  .sort();

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function assertExactKeys(value: Record<string, unknown>, expected: readonly string[], field: string) {
  const actual = Object.keys(value).sort();
  const wanted = [...expected].sort();
  if (actual.length !== wanted.length || actual.some((key, index) => key !== wanted[index])) {
    throw new TypeError(`Invalid ${field} fields.`);
  }
}

function requireString(value: unknown, field: string): string {
  if (typeof value !== 'string' || value.trim() === '') {
    throw new TypeError(`Invalid ${field}.`);
  }
  return value;
}

function parseReleaseIdentity(value: unknown): AppReleaseIdentity {
  if (!isRecord(value)) {
    throw new TypeError('Invalid releaseIdentity.');
  }
  assertExactKeys(value, ['sourceTree', 'contractDigest', 'deploymentProfile'], 'releaseIdentity');
  const sourceTree = requireString(value.sourceTree, 'releaseIdentity.sourceTree');
  const contractDigest = requireString(value.contractDigest, 'releaseIdentity.contractDigest');
  const deploymentProfile = requireString(value.deploymentProfile, 'releaseIdentity.deploymentProfile');
  if (!sourceTreePattern.test(sourceTree) || !sha256Pattern.test(contractDigest)) {
    throw new TypeError('Release identity does not match the trusted build identity shape.');
  }
  if (deploymentProfile !== expectedDeploymentProfile) {
    throw new TypeError('Release identity deployment profile is not committed for this application projection.');
  }
  return Object.freeze({ sourceTree, contractDigest, deploymentProfile });
}

function parseCapability(value: unknown): AppCapabilityAvailability {
  if (!isRecord(value)) {
    throw new TypeError('Invalid capability projection row.');
  }
  assertExactKeys(value, ['capabilityId', 'disposition', 'availability', 'enabled'], 'capability projection');
  const capabilityId = requireString(value.capabilityId, 'capabilityId');
  const generated = generatedCapabilityById.get(capabilityId);
  if (!generated || generated.disposition === 'excluded') {
    throw new TypeError(`Capability ${capabilityId} is unknown or excluded.`);
  }
  if (value.disposition !== 'committed' && value.disposition !== 'conditional') {
    throw new TypeError(`Capability ${capabilityId} has an invalid disposition.`);
  }
  if (value.disposition !== generated.disposition) {
    throw new TypeError(`Capability ${capabilityId} does not match the generated release disposition.`);
  }
  if (value.availability !== 'enabled' && value.availability !== 'disabled' && value.availability !== 'blocked') {
    throw new TypeError(`Capability ${capabilityId} has an invalid availability.`);
  }
  if (typeof value.enabled !== 'boolean' || value.enabled !== (value.availability === 'enabled')) {
    throw new TypeError(`Capability ${capabilityId} has inconsistent enabled state.`);
  }
  return Object.freeze({
    capabilityId,
    disposition: value.disposition,
    availability: value.availability,
    enabled: value.enabled
  });
}

function projectionDigestPayload(response: AppCapabilityProjectionResponse) {
  return {
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
}

async function sha256Digest(value: unknown): Promise<string> {
  if (!globalThis.crypto?.subtle) {
    throw new TypeError('Web Crypto is unavailable for release projection validation.');
  }
  const bytes = new TextEncoder().encode(JSON.stringify(value));
  const digest = await globalThis.crypto.subtle.digest('SHA-256', bytes);
  return `sha256:${Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join('')}`;
}

async function parseProjectionResponse(value: unknown): Promise<AppCapabilityProjectionResponse> {
  if (!isRecord(value)) {
    throw new TypeError('Invalid app capability projection response.');
  }
  assertExactKeys(value, ['releaseIdentity', 'generation', 'projectionDigest', 'capabilities'], 'app capability projection');
  if (!Number.isSafeInteger(value.generation) || (value.generation as number) <= 0) {
    throw new TypeError('Invalid release projection generation.');
  }
  const projectionDigest = requireString(value.projectionDigest, 'projectionDigest');
  if (!sha256Pattern.test(projectionDigest)) {
    throw new TypeError('Invalid release projection digest.');
  }
  if (!Array.isArray(value.capabilities)) {
    throw new TypeError('Invalid release projection capabilities.');
  }

  const capabilities = value.capabilities.map(parseCapability);
  const capabilityIds = capabilities.map((capability) => capability.capabilityId);
  if (
    new Set(capabilityIds).size !== capabilityIds.length
    || capabilityIds.some((capabilityId, index) => capabilityId !== expectedAppCapabilityIds[index])
    || capabilityIds.length !== expectedAppCapabilityIds.length
  ) {
    throw new TypeError('Release projection capability inventory does not exactly match the generated projection.');
  }

  const response = Object.freeze({
    releaseIdentity: parseReleaseIdentity(value.releaseIdentity),
    generation: value.generation as number,
    projectionDigest,
    capabilities: Object.freeze(capabilities)
  });
  const computedDigest = await sha256Digest(projectionDigestPayload(response));
  if (computedDigest !== response.projectionDigest) {
    throw new TypeError('Release projection digest does not match the authenticated runtime response.');
  }
  return response;
}

function identitiesEqual(left: AppReleaseIdentity, right: AppReleaseIdentity) {
  return left.sourceTree === right.sourceTree
    && left.contractDigest === right.contractDigest
    && left.deploymentProfile === right.deploymentProfile;
}

export function createReleaseProjectionApi(client: HttpClient): ReleaseProjectionApi {
  let latestGeneration = 0;
  let pinnedIdentity: AppReleaseIdentity | null = null;

  return {
    load: async () => {
      const rawResponse = await client.get<Response>(
        '/api/v1/app/readiness/capabilities',
        undefined,
        readinessTransport
      );
      const response = await parseProjectionResponse(await rawResponse.json());
      if (pinnedIdentity !== null && !identitiesEqual(pinnedIdentity, response.releaseIdentity)) {
        throw new TypeError('Release projection identity changed during the authenticated session.');
      }
      if (response.generation < latestGeneration) {
        throw new TypeError('Release projection generation regressed.');
      }
      pinnedIdentity = response.releaseIdentity;
      latestGeneration = response.generation;
      return response;
    }
  };
}

function stateFromResponse(response: AppCapabilityProjectionResponse): ReleaseProjectionState {
  const capabilityById = Object.freeze(Object.fromEntries(
    response.capabilities.map((capability) => [capability.capabilityId, capability])
  ));
  return Object.freeze({
    status: 'ready' as const,
    releaseIdentity: response.releaseIdentity,
    generation: response.generation,
    projectionDigest: response.projectionDigest,
    generatedProjectionDigest: releaseProjectionDigest,
    capabilityById,
    isCapabilityEnabled: (capabilityId: string) => capabilityById[capabilityId]?.enabled === true
  });
}

function closedState(status: Exclude<ReleaseProjectionStatus, 'ready'>): ReleaseProjectionState {
  return Object.freeze({
    status,
    releaseIdentity: null,
    generation: null,
    projectionDigest: null,
    generatedProjectionDigest: releaseProjectionDigest,
    capabilityById: emptyCapabilityById,
    isCapabilityEnabled: () => false
  });
}

const ReleaseProjectionContext = createContext<ReleaseProjectionState>(closedState('unavailable'));

export function ReleaseProjectionProvider({ children }: { children: ReactNode }) {
  const { authState } = useAppContext();
  const apiRef = useRef<ReleaseProjectionApi>();
  const [state, setState] = useState<ReleaseProjectionState>(() => closedState('loading'));

  if (apiRef.current === undefined) {
    apiRef.current = createReleaseProjectionApi(createHttpClient());
  }

  useEffect(() => {
    let cancelled = false;
    if (authState.status === 'idle' || authState.status === 'loading') {
      setState(closedState('loading'));
      return () => {
        cancelled = true;
      };
    }
    if (authState.status !== 'authenticated' || authState.user === null) {
      setState(closedState('unauthenticated'));
      return () => {
        cancelled = true;
      };
    }

    setState(closedState('loading'));
    const api = apiRef.current!;
    void api.load()
      .then((response) => {
        if (!cancelled) {
          setState(stateFromResponse(response));
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setState(closedState(error instanceof HttpError && error.status === 401 ? 'unauthenticated' : 'unavailable'));
        }
      });
    return () => {
      cancelled = true;
    };
  }, [authState.status, authState.user?.id]);

  return <ReleaseProjectionContext.Provider value={state}>{children}</ReleaseProjectionContext.Provider>;
}

export function ReleaseProjectionBoundary() {
  return (
    <ReleaseProjectionProvider>
      <Outlet />
    </ReleaseProjectionProvider>
  );
}

export function ReleaseProjectionRoute({
  capabilityId,
  children
}: {
  capabilityId?: string;
  children: ReactNode;
}) {
  const projection = useReleaseProjection();
  const generated = capabilityId === undefined ? null : getGeneratedReleaseCapability(capabilityId);

  if (capabilityId === undefined) {
    return <>{children}</>;
  }
  if (generated === null || generated.navigationDisposition === 'hidden') {
    return <main role="status">This surface is currently unavailable.</main>;
  }
  if (generated.disposition === 'conditional' && (projection.status !== 'ready' || !projection.isCapabilityEnabled(capabilityId))) {
    return <main role="status">This surface is currently unavailable.</main>;
  }
  return <>{children}</>;
}

export function isGeneratedNavigationVisible(capabilityId: string) {
  return getGeneratedReleaseCapability(capabilityId)?.navigationDisposition === 'visible';
}

export function useReleaseProjection() {
  return useContext(ReleaseProjectionContext);
}

export function useReleaseCapability(capabilityId: string) {
  const projection = useReleaseProjection();
  return useMemo(() => ({
    capability: projection.capabilityById[capabilityId] ?? null,
    enabled: projection.isCapabilityEnabled(capabilityId),
    status: projection.status
  }), [capabilityId, projection]);
}

export function getGeneratedReleaseCapability(capabilityId: string) {
  return generatedCapabilityById.get(capabilityId) ?? null;
}

export { releaseCapabilityProjection, releaseProjectionDigest, releaseSurfaceProjection };
