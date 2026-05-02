import type { HttpClient } from '../../services/http/client';
import type {
  AdminStats,
  AuditEntry,
  ChannelCreateRequest,
  ChannelHealth,
  ChannelInfo,
  ChannelTestResult,
  ChannelUpdateRequest,
  PaginatedResponse,
  PlanCreateRequest,
  PlanInfo,
  PlanUpdateRequest,
  PublishedAgent,
  RouteCreateRequest,
  RouteInfo,
  RouteUpdateRequest,
  UserDetail,
  UserUpdateRequest,
} from '../../types/admin';

type QueryValue = string | number | boolean | undefined | null;

type ChannelListPayload = {
  channels?: ChannelInfo[];
  data?: ChannelInfo[];
  total?: number;
};

type RouteListPayload = {
  routes?: RouteInfo[];
  data?: RouteInfo[];
  total?: number;
};

type PlanListPayload = {
  plans?: PlanInfo[];
  data?: PlanInfo[];
  total?: number;
};

type UserListPayload = {
  users?: UserDetail[];
  data?: UserDetail[];
  total?: number;
};

type AuditListPayload = {
  entries?: AuditEntry[];
  auditLogs?: AuditEntry[];
  data?: AuditEntry[];
  total?: number;
};

type ReviewListPayload = {
  reviews?: PublishedAgent[];
  data?: PublishedAgent[];
  total?: number;
};

export type AdminApi = {
  getStats: () => Promise<AdminStats>;
  listChannels: (params?: {
    provider?: string;
    status?: string;
    search?: string;
    sort?: string;
    limit?: number;
    offset?: number;
  }) => Promise<PaginatedResponse<ChannelInfo>>;
  getChannel: (id: string) => Promise<ChannelInfo>;
  createChannel: (input: ChannelCreateRequest) => Promise<ChannelInfo>;
  updateChannel: (id: string, input: ChannelUpdateRequest) => Promise<ChannelInfo>;
  deleteChannel: (id: string) => Promise<void>;
  testChannel: (id: string) => Promise<ChannelTestResult>;
  getChannelHealth: (id: string) => Promise<ChannelHealth>;
  batchUpdateChannels: (ids: string[], action: 'enable' | 'disable' | boolean) => Promise<void>;
  listRoutes: () => Promise<RouteInfo[]>;
  getRoute: (id: string) => Promise<RouteInfo>;
  createRoute: (input: RouteCreateRequest) => Promise<RouteInfo>;
  updateRoute: (id: string, input: RouteUpdateRequest) => Promise<RouteInfo>;
  deleteRoute: (id: string) => Promise<void>;
  listPlans: (params?: { isPublic?: boolean; status?: string; search?: string; limit?: number; offset?: number }) => Promise<PlanInfo[]>;
  getPlan: (id: string) => Promise<PlanInfo>;
  createPlan: (input: PlanCreateRequest) => Promise<PlanInfo>;
  updatePlan: (id: string, input: PlanUpdateRequest) => Promise<PlanInfo>;
  deactivatePlan: (id: string) => Promise<void>;
  listUsers: (params?: {
    search?: string;
    role?: string;
    planID?: string;
    planId?: string;
    status?: string;
    sort?: string;
    limit?: number;
    offset?: number;
  }) => Promise<PaginatedResponse<UserDetail>>;
  getUser: (id: string) => Promise<UserDetail>;
  updateUser: (id: string, input: UserUpdateRequest) => Promise<UserDetail>;
  disableUser: (id: string) => Promise<void>;
  enableUser: (id: string) => Promise<void>;
  listAuditLogs: (params?: {
    actorID?: string;
    actorId?: string;
    action?: string;
    resourceType?: string;
    resourceID?: string;
    resourceId?: string;
    startDate?: string;
    endDate?: string;
    limit?: number;
    offset?: number;
  }) => Promise<PaginatedResponse<AuditEntry>>;
  listReviews: (params?: { status?: string; limit?: number; offset?: number }) => Promise<PaginatedResponse<PublishedAgent>>;
  approveAgent: (id: string) => Promise<void>;
  rejectAgent: (id: string, reason: string) => Promise<void>;
};

function buildQuery(params?: Record<string, QueryValue>) {
  const query = new URLSearchParams();

  Object.entries(params ?? {}).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') {
      query.set(key, String(value));
    }
  });

  const serialized = query.toString();
  return serialized ? `?${serialized}` : '';
}

function collection<T>(items: T[] | undefined, total: number | undefined): PaginatedResponse<T> {
  return {
    data: items ?? [],
    total: total ?? items?.length ?? 0,
  };
}

function toBatchAction(action: 'enable' | 'disable' | boolean) {
  if (typeof action === 'boolean') {
    return action ? 'enable' : 'disable';
  }
  return action;
}

export function createAdminApi(client: HttpClient): AdminApi {
  const apiPrefix = '/api/v1/admin';

  return {
    getStats: () => client.get<AdminStats>(`${apiPrefix}/stats`),

    listChannels: async (params) => {
      const payload = await client.get<ChannelListPayload>(`${apiPrefix}/channels${buildQuery(params)}`);
      return collection(payload.channels ?? payload.data, payload.total);
    },
    getChannel: (id) => client.get<ChannelInfo>(`${apiPrefix}/channels/${id}`),
    createChannel: (input) => client.post<ChannelInfo>(`${apiPrefix}/channels`, input),
    updateChannel: (id, input) => client.put<ChannelInfo>(`${apiPrefix}/channels/${id}`, input),
    deleteChannel: async (id) => {
      await client.delete<{ status: string }>(`${apiPrefix}/channels/${id}`);
    },
    testChannel: (id) => client.post<ChannelTestResult>(`${apiPrefix}/channels/${id}/test`),
    getChannelHealth: (id) => client.get<ChannelHealth>(`${apiPrefix}/channels/${id}/health`),
    batchUpdateChannels: async (ids, action) => {
      await client.post<{ status: string }>(`${apiPrefix}/channels/batch`, { ids, action: toBatchAction(action) });
    },

    listRoutes: async () => {
      const payload = await client.get<RouteListPayload>(`${apiPrefix}/routes`);
      return payload.routes ?? payload.data ?? [];
    },
    getRoute: (id) => client.get<RouteInfo>(`${apiPrefix}/routes/${id}`),
    createRoute: (input) => client.post<RouteInfo>(`${apiPrefix}/routes`, input),
    updateRoute: (id, input) => client.put<RouteInfo>(`${apiPrefix}/routes/${id}`, input),
    deleteRoute: async (id) => {
      await client.delete<{ status: string }>(`${apiPrefix}/routes/${id}`);
    },

    listPlans: async (params) => {
      const payload = await client.get<PlanListPayload>(`${apiPrefix}/plans${buildQuery(params)}`);
      return payload.plans ?? payload.data ?? [];
    },
    getPlan: (id) => client.get<PlanInfo>(`${apiPrefix}/plans/${id}`),
    createPlan: (input) => client.post<PlanInfo>(`${apiPrefix}/plans`, input),
    updatePlan: (id, input) => client.put<PlanInfo>(`${apiPrefix}/plans/${id}`, input),
    deactivatePlan: async (id) => {
      await client.delete<{ status: string }>(`${apiPrefix}/plans/${id}`);
    },

    listUsers: async (params) => {
      const queryParams = { ...params, planID: params?.planID ?? params?.planId };
      delete queryParams.planId;
      const payload = await client.get<UserListPayload>(`${apiPrefix}/users${buildQuery(queryParams)}`);
      return collection(payload.users ?? payload.data, payload.total);
    },
    getUser: (id) => client.get<UserDetail>(`${apiPrefix}/users/${id}`),
    updateUser: (id, input) => {
      const body = { ...input, planID: input.planID ?? input.planId };
      delete body.planId;
      return client.put<UserDetail>(`${apiPrefix}/users/${id}`, body);
    },
    disableUser: async (id) => {
      await client.post<{ status: string }>(`${apiPrefix}/users/${id}/disable`);
    },
    enableUser: async (id) => {
      await client.post<{ status: string }>(`${apiPrefix}/users/${id}/enable`);
    },

    listAuditLogs: async (params) => {
      const queryParams = {
        ...params,
        actorID: params?.actorID ?? params?.actorId,
        resourceID: params?.resourceID ?? params?.resourceId,
      };
      delete queryParams.actorId;
      delete queryParams.resourceId;
      const payload = await client.get<AuditListPayload>(`${apiPrefix}/audit-logs${buildQuery(queryParams)}`);
      return collection(payload.entries ?? payload.auditLogs ?? payload.data, payload.total);
    },

    listReviews: async (params) => {
      const payload = await client.get<ReviewListPayload>(`${apiPrefix}/reviews${buildQuery(params)}`);
      return collection(payload.reviews ?? payload.data, payload.total);
    },
    approveAgent: async (id) => {
      await client.post<{ status: string }>(`${apiPrefix}/reviews/${id}/approve`);
    },
    rejectAgent: async (id, reason) => {
      await client.post<{ status: string }>(`${apiPrefix}/reviews/${id}/reject`, { reason });
    },
  };
}
