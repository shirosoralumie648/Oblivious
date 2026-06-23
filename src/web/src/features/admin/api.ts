import type { HttpClient } from '../../services/http/client';
import type {
  AdminStats,
  APITokenEntry,
  APITokenFilter,
  AuditEntry,
  BillingFilter,
  BillingInspectionRecord,
  BillingSummary,
  BillingSurface,
  ChannelBalanceRefreshResult,
  ChannelCreateRequest,
  ChannelHealth,
  ChannelModelUpdateApplyRequest,
  ChannelModelUpdateApplyResult,
  ChannelModelUpdatePreview,
  ChannelInfo,
  ChannelModelSyncResult,
  ChannelProviderInfo,
  ChannelRuntimeStats,
  ChannelTestResult,
  ChannelUpdateRequest,
  MarketplaceAbuseReport,
  ModelInventoryEntry,
  ModelInventoryFilter,
  PaginatedResponse,
  PlanCreateRequest,
  PlanInfo,
  PlanUpdateRequest,
  PublishedAgent,
  RelayPricingSettings,
  RouteCreateRequest,
  RouteInfo,
  RouteUpdateRequest,
  ReviewSLAEnforcementResult,
  TopupRefundRequest,
  UsageAnalyticsFilter,
  UsageAnalyticsResponse,
  UsageLimitSettings,
  UsageLogEntry,
  UsageLogFilter,
  UserDetail,
  UserQuotaUpdateRequest,
  UserUpdateRequest,
} from '../../types/admin';

type QueryValue = string | number | boolean | undefined | null;

type ChannelListPayload = {
  channels?: ChannelInfo[];
  data?: ChannelInfo[];
  total?: number;
};

type ChannelProviderListPayload = {
  providers?: ChannelProviderInfo[];
  data?: ChannelProviderInfo[];
};

type ChannelRuntimeStatsPayload = {
  stats?: ChannelRuntimeStats[];
  data?: ChannelRuntimeStats[];
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

type UsageLogListPayload = {
  usageLogs?: UsageLogEntry[];
  data?: UsageLogEntry[];
  total?: number;
};

type ModelInventoryListPayload = {
  models?: ModelInventoryEntry[];
  data?: ModelInventoryEntry[];
  total?: number;
};

type APITokenListPayload = {
  apiTokens?: APITokenEntry[];
  data?: APITokenEntry[];
  total?: number;
};

type UsageLimitSettingsPayload = {
  usageLimits?: UsageLimitSettings[];
  data?: UsageLimitSettings[];
};

export type AdminObservabilityAlertState = {
  id?: string;
  key?: string;
  Key?: string;
  name?: string;
  title?: string;
  Title?: string;
  severity?: string;
  Severity?: string;
  originalSeverity?: string;
  OriginalSeverity?: string;
  status?: string;
  Status?: string;
  summary?: string;
  message?: string;
  Message?: string;
  source?: string;
  component?: string;
  Component?: string;
  openedAt?: string;
  OpenedAt?: string;
  lastTriggeredAt?: string;
  lastOccurredAt?: string;
  LastOccurredAt?: string;
  occurrenceCount?: number;
  OccurrenceCount?: number;
};

export type AdminObservabilityAlertDeliveryAttempt = {
  id?: string;
  ID?: string;
  alertKey?: string;
  AlertKey?: string;
  channel?: string;
  Channel?: string;
  providerId?: string;
  providerID?: string;
  ProviderID?: string;
  providerKind?: string;
  ProviderKind?: string;
  delivered?: boolean;
  Delivered?: boolean;
  error?: string;
  Error?: string;
  attemptedAt?: string;
  AttemptedAt?: string;
};

export type AdminObservabilityRecoveryAction = {
  id?: string;
  ID?: string;
  policyName?: string;
  PolicyName?: string;
  alertKey?: string;
  AlertKey?: string;
  severity?: string;
  Severity?: string;
  component?: string;
  Component?: string;
  type?: string;
  Type?: string;
  status?: string;
  Status?: string;
  reason?: string;
  Reason?: string;
  createdAt?: string;
  CreatedAt?: string;
};

export type AdminObservabilityAlertDeliveryChannel = 'email' | 'im' | 'sms' | 'third_party' | 'phone' | 'in_app';
export type AdminObservabilityAlertRoutingRules = Record<string, AdminObservabilityAlertDeliveryChannel[]>;

export type AdminObservabilityAlertProviderKind =
  | 'smtp'
  | 'slack_webhook'
  | 'feishu_webhook'
  | 'dingtalk_webhook'
  | 'wecom_webhook'
  | 'twilio_sms'
  | 'aliyun_sms'
  | 'phone'
  | 'pagerduty'
  | 'opsgenie'
  | 'aliyun_monitor'
  | 'tencent_cloud_monitor';
export type AdminObservabilityAlertProviderStatus = 'active' | 'disabled';
export type AdminObservabilityAlertProviderConfig = Record<string, string | number | boolean | null | undefined>;

export type AdminObservabilityAlertProvider = {
  id: string;
  kind: AdminObservabilityAlertProviderKind;
  channel: AdminObservabilityAlertDeliveryChannel;
  name: string;
  status: AdminObservabilityAlertProviderStatus;
  config: AdminObservabilityAlertProviderConfig;
  createdAt?: string;
  updatedAt?: string;
};

export type AdminObservabilityAlertProviderRequest = {
  kind: AdminObservabilityAlertProviderKind;
  name: string;
  status: AdminObservabilityAlertProviderStatus;
  config: AdminObservabilityAlertProviderConfig;
};

export type AdminObservabilityAlertProviderTestResult = {
  providerId: string;
  kind: AdminObservabilityAlertProviderKind;
  channel: AdminObservabilityAlertDeliveryChannel;
  ok: boolean;
  message: string;
  testedAt: string;
};

export type AdminObservabilityAlertFilter = {
  severity?: string;
  status?: string;
  component?: string;
  keyPrefix?: string;
  limit?: number;
  offset?: number;
};

export type AdminObservabilityAlertHistoryFilter = {
  limit?: number;
  offset?: number;
};

export type AdminObservabilityRecoveryActionFilter = {
  alertKey?: string;
  policyName?: string;
  component?: string;
  type?: string;
  limit?: number;
  offset?: number;
};

type ObservabilityAlertsPayload =
  | AdminObservabilityAlertState[]
  | {
      alerts?: AdminObservabilityAlertState[];
      data?: AdminObservabilityAlertState[];
    };

type ObservabilityDeliveryPayload =
  | AdminObservabilityAlertDeliveryAttempt[]
  | {
      attempts?: AdminObservabilityAlertDeliveryAttempt[];
      deliveries?: AdminObservabilityAlertDeliveryAttempt[];
      data?: AdminObservabilityAlertDeliveryAttempt[];
    };

type ObservabilityRecoveryActionsPayload =
  | AdminObservabilityRecoveryAction[]
  | {
      actions?: AdminObservabilityRecoveryAction[];
      data?: AdminObservabilityRecoveryAction[];
    };

type ObservabilityAlertProvidersPayload =
  | AdminObservabilityAlertProvider[]
  | {
      providers?: AdminObservabilityAlertProvider[];
      data?: AdminObservabilityAlertProvider[];
    };

type ReviewListPayload = {
  reviews?: PublishedAgent[];
  data?: PublishedAgent[];
  total?: number;
};

type MarketplaceAbuseReportsPayload = {
  reports?: MarketplaceAbuseReport[];
  data?: MarketplaceAbuseReport[];
  total?: number;
};

type BillingListPayload = {
  sessions?: BillingInspectionRecord[];
  paymentIntents?: BillingInspectionRecord[];
  webhookEvents?: BillingInspectionRecord[];
  subscriptions?: BillingInspectionRecord[];
  topups?: BillingInspectionRecord[];
  invoices?: BillingInspectionRecord[];
  refunds?: BillingInspectionRecord[];
  settlements?: BillingInspectionRecord[];
  payouts?: BillingInspectionRecord[];
  data?: BillingInspectionRecord[];
  total?: number;
};

type BillingListCollectionKey = Exclude<keyof BillingListPayload, 'total'>;

const billingSurfacePaths: Record<BillingSurface, string> = {
  sessions: 'sessions',
  paymentIntents: 'payment-intents',
  webhookEvents: 'webhook-events',
  subscriptions: 'subscriptions',
  topups: 'topups',
  invoices: 'invoices',
  refunds: 'refunds',
  settlements: 'settlements',
  payouts: 'payouts',
};

const billingSurfaceKeys: Record<BillingSurface, BillingListCollectionKey> = {
  sessions: 'sessions',
  paymentIntents: 'paymentIntents',
  webhookEvents: 'webhookEvents',
  subscriptions: 'subscriptions',
  topups: 'topups',
  invoices: 'invoices',
  refunds: 'refunds',
  settlements: 'settlements',
  payouts: 'payouts',
};

export type AdminApi = {
  getStats: () => Promise<AdminStats>;
  getRelayPricingSettings: () => Promise<RelayPricingSettings>;
  updateRelayPricingSettings: (input: RelayPricingSettings) => Promise<RelayPricingSettings>;
  getUsageLimitSettings: () => Promise<UsageLimitSettings[]>;
  updateUsageLimitSettings: (input: UsageLimitSettings) => Promise<UsageLimitSettings>;
  listChannels: (params?: {
    provider?: string;
    status?: string;
    search?: string;
    sort?: string;
    limit?: number;
    offset?: number;
  }) => Promise<PaginatedResponse<ChannelInfo>>;
  listChannelProviders: () => Promise<ChannelProviderInfo[]>;
  listChannelStats: () => Promise<ChannelRuntimeStats[]>;
  listModelInventory: (params?: ModelInventoryFilter) => Promise<PaginatedResponse<ModelInventoryEntry>>;
  getChannel: (id: string) => Promise<ChannelInfo>;
  createChannel: (input: ChannelCreateRequest) => Promise<ChannelInfo>;
  updateChannel: (id: string, input: ChannelUpdateRequest) => Promise<ChannelInfo>;
  deleteChannel: (id: string) => Promise<void>;
  testChannel: (id: string) => Promise<ChannelTestResult>;
  syncChannelModels: (id: string) => Promise<ChannelModelSyncResult>;
  detectChannelModelUpdates: (id: string) => Promise<ChannelModelUpdatePreview>;
  applyChannelModelUpdates: (id: string, input?: ChannelModelUpdateApplyRequest) => Promise<ChannelModelUpdateApplyResult>;
  refreshChannelBalance: (id: string) => Promise<ChannelBalanceRefreshResult>;
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
  updateUserQuota: (id: string, input: UserQuotaUpdateRequest) => Promise<UserDetail>;
  disableUser: (id: string) => Promise<void>;
  enableUser: (id: string) => Promise<void>;
  listAuditLogs: (params?: {
    organizationID?: string;
    organizationId?: string;
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
  listUsageLogs: (params?: UsageLogFilter) => Promise<PaginatedResponse<UsageLogEntry>>;
  getUsageAnalytics: (params?: UsageAnalyticsFilter) => Promise<UsageAnalyticsResponse>;
  listAPITokens: (params?: APITokenFilter) => Promise<PaginatedResponse<APITokenEntry>>;
  revokeAPIToken: (id: string) => Promise<void>;
  listReviews: (params?: { status?: string; limit?: number; offset?: number }) => Promise<PaginatedResponse<PublishedAgent>>;
  enforceReviewSLA: (params?: { limit?: number; offset?: number }) => Promise<ReviewSLAEnforcementResult>;
  approveAgent: (id: string) => Promise<void>;
  rejectAgent: (id: string, reason: string) => Promise<void>;
  requestAgentChanges: (id: string, reason: string) => Promise<void>;
  listMarketplaceAbuseReports: (params?: { status?: string; limit?: number; offset?: number }) => Promise<PaginatedResponse<MarketplaceAbuseReport>>;
  resolveMarketplaceAbuseReport: (id: string, resolution: string) => Promise<{ status: 'resolved' | 'dismissed' | string }>;
  dismissMarketplaceAbuseReport: (id: string, resolution: string) => Promise<{ status: 'resolved' | 'dismissed' | string }>;
  takedownMarketplaceAgent: (id: string, reason: string) => Promise<{ status: string }>;
  reinstateMarketplaceAgent: (id: string, reason: string) => Promise<{ status: string }>;
  getBillingSummary: (params?: BillingFilter) => Promise<BillingSummary>;
  listBillingSurface: (surface: BillingSurface, params?: BillingFilter) => Promise<PaginatedResponse<BillingInspectionRecord>>;
  refundTopup: (topupId: string, input: TopupRefundRequest) => Promise<BillingInspectionRecord>;
  createDueMarketplacePayouts: () => Promise<PaginatedResponse<BillingInspectionRecord>>;
  markMarketplacePayoutPaid: (payoutId: string, providerPayoutId: string) => Promise<BillingInspectionRecord>;
  markMarketplacePayoutFailed: (payoutId: string, providerPayoutId: string, reason: string) => Promise<BillingInspectionRecord>;
  listObservabilityAlerts: (params?: AdminObservabilityAlertFilter) => Promise<AdminObservabilityAlertState[]>;
  getObservabilityAlert: (key: string) => Promise<AdminObservabilityAlertState>;
  acknowledgeObservabilityAlert: (key: string) => Promise<AdminObservabilityAlertState>;
  resolveObservabilityAlert: (key: string) => Promise<AdminObservabilityAlertState>;
  listObservabilityAlertDeliveries: (
    key: string,
    params?: AdminObservabilityAlertHistoryFilter
  ) => Promise<AdminObservabilityAlertDeliveryAttempt[]>;
  listObservabilityRecoveryActions: (params?: AdminObservabilityRecoveryActionFilter) => Promise<AdminObservabilityRecoveryAction[]>;
  getObservabilityAlertRoutingRules: () => Promise<AdminObservabilityAlertRoutingRules>;
  updateObservabilityAlertRoutingRules: (rules: AdminObservabilityAlertRoutingRules) => Promise<AdminObservabilityAlertRoutingRules>;
  listObservabilityAlertProviders: () => Promise<AdminObservabilityAlertProvider[]>;
  createObservabilityAlertProvider: (input: AdminObservabilityAlertProviderRequest) => Promise<AdminObservabilityAlertProvider>;
  updateObservabilityAlertProvider: (id: string, input: AdminObservabilityAlertProviderRequest) => Promise<AdminObservabilityAlertProvider>;
  testObservabilityAlertProvider: (id: string) => Promise<AdminObservabilityAlertProviderTestResult>;
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

type CollectionPayload<T, K extends string> = T[] | ({ data?: T[] } & Partial<Record<K, T[]>>);

function collectionPayload<T, K extends string>(payload: CollectionPayload<T, K>, keys: K[]): T[] {
  if (Array.isArray(payload)) {
    return payload;
  }
  for (const key of keys) {
    const value = payload[key];
    if (Array.isArray(value)) {
      return value;
    }
  }
  return payload.data ?? [];
}

export function createAdminApi(client: HttpClient): AdminApi {
  const apiPrefix = '/api/v1/admin';
  const observabilityAlertsPrefix = `${apiPrefix}/observability/alerts`;
  const observabilityAlertProvidersPrefix = `${apiPrefix}/observability/alert-providers`;

  return {
    getStats: () => client.get<AdminStats>(`${apiPrefix}/stats`),
    getRelayPricingSettings: () => client.get<RelayPricingSettings>(`${apiPrefix}/settings/relay-pricing`),
    updateRelayPricingSettings: (input) => client.put<RelayPricingSettings>(`${apiPrefix}/settings/relay-pricing`, input),
    getUsageLimitSettings: async () => {
      const payload = await client.get<UsageLimitSettingsPayload>(`${apiPrefix}/settings/usage-limits`);
      return payload.usageLimits ?? payload.data ?? [];
    },
    updateUsageLimitSettings: (input) => client.put<UsageLimitSettings>(`${apiPrefix}/settings/usage-limits`, input),

    listChannels: async (params) => {
      const payload = await client.get<ChannelListPayload>(`${apiPrefix}/channels${buildQuery(params)}`);
      return collection(payload.channels ?? payload.data, payload.total);
    },
    listChannelProviders: async () => {
      const payload = await client.get<ChannelProviderListPayload>(`${apiPrefix}/channel-providers`);
      return payload.providers ?? payload.data ?? [];
    },
    listChannelStats: async () => {
      const payload = await client.get<ChannelRuntimeStatsPayload>(`${apiPrefix}/channels/stats`);
      return payload.stats ?? payload.data ?? [];
    },
    listModelInventory: async (params) => {
      const payload = await client.get<ModelInventoryListPayload>(`${apiPrefix}/models${buildQuery(params)}`);
      return collection(payload.models ?? payload.data, payload.total);
    },
    getChannel: (id) => client.get<ChannelInfo>(`${apiPrefix}/channels/${id}`),
    createChannel: (input) => client.post<ChannelInfo>(`${apiPrefix}/channels`, input),
    updateChannel: (id, input) => client.put<ChannelInfo>(`${apiPrefix}/channels/${id}`, input),
    deleteChannel: async (id) => {
      await client.delete<{ status: string }>(`${apiPrefix}/channels/${id}`);
    },
    testChannel: (id) => client.post<ChannelTestResult>(`${apiPrefix}/channels/${id}/test`),
    syncChannelModels: (id) => client.post<ChannelModelSyncResult>(`${apiPrefix}/channels/${id}/sync-models`),
    detectChannelModelUpdates: (id) => client.post<ChannelModelUpdatePreview>(`${apiPrefix}/channels/${id}/model-updates/detect`),
    applyChannelModelUpdates: (id, input = { mode: 'merge' }) => client.post<ChannelModelUpdateApplyResult>(`${apiPrefix}/channels/${id}/model-updates/apply`, input),
    refreshChannelBalance: (id) => client.post<ChannelBalanceRefreshResult>(`${apiPrefix}/channels/${id}/refresh-balance`),
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
    updateUserQuota: (id, input) =>
      client.request<UserDetail>(`${apiPrefix}/users/${id}`, {
        method: 'PATCH',
        body: JSON.stringify({ balance: input.balance }),
      }),
    disableUser: async (id) => {
      await client.post<{ status: string }>(`${apiPrefix}/users/${id}/disable`);
    },
    enableUser: async (id) => {
      await client.post<{ status: string }>(`${apiPrefix}/users/${id}/enable`);
    },

    listAuditLogs: async (params) => {
      const queryParams = {
        organizationID: params?.organizationID ?? params?.organizationId,
        actorID: params?.actorID ?? params?.actorId,
        action: params?.action,
        resourceType: params?.resourceType,
        resourceID: params?.resourceID ?? params?.resourceId,
        startDate: params?.startDate,
        endDate: params?.endDate,
        limit: params?.limit,
        offset: params?.offset,
      };
      const payload = await client.get<AuditListPayload>(`${apiPrefix}/audit-logs${buildQuery(queryParams)}`);
      return collection(payload.entries ?? payload.auditLogs ?? payload.data, payload.total);
    },

    listUsageLogs: async (params) => {
      const queryParams = {
        organizationID: params?.organizationID ?? params?.organizationId,
        userID: params?.userID ?? params?.userId,
        apiTokenID: params?.apiTokenID ?? params?.apiTokenId,
        requestID: params?.requestID ?? params?.requestId,
        apiType: params?.apiType,
        featureType: params?.featureType,
        quotaMode: params?.quotaMode,
        model: params?.model,
        channelID: params?.channelID ?? params?.channelId,
        provider: params?.provider,
        status: params?.status,
        limit: params?.limit,
        offset: params?.offset,
      };
      const payload = await client.get<UsageLogListPayload>(`${apiPrefix}/usage-logs${buildQuery(queryParams)}`);
      return collection(payload.usageLogs ?? payload.data, payload.total);
    },

    getUsageAnalytics: (params) => {
      const queryParams = {
        organizationID: params?.organizationID ?? params?.organizationId,
        userID: params?.userID ?? params?.userId,
        apiType: params?.apiType,
        model: params?.model,
        channelID: params?.channelID ?? params?.channelId,
        provider: params?.provider,
        status: params?.status,
        granularity: params?.granularity,
        featureType: params?.featureType,
        quotaMode: params?.quotaMode,
        from: params?.from,
        to: params?.to,
        limit: params?.limit,
      };
      return client.get<UsageAnalyticsResponse>(`${apiPrefix}/usage-analytics${buildQuery(queryParams)}`);
    },

    listAPITokens: async (params) => {
      const queryParams = {
        ...params,
        organizationID: params?.organizationID ?? params?.organizationId,
        userID: params?.userID ?? params?.userId,
      };
      delete queryParams.organizationId;
      delete queryParams.userId;
      const payload = await client.get<APITokenListPayload>(`${apiPrefix}/api-tokens${buildQuery(queryParams)}`);
      return collection(payload.apiTokens ?? payload.data, payload.total);
    },
    revokeAPIToken: async (id) => {
      await client.post<{ status: string }>(`${apiPrefix}/api-tokens/${id}/revoke`);
    },

    listReviews: async (params) => {
      const payload = await client.get<ReviewListPayload>(`${apiPrefix}/reviews${buildQuery(params)}`);
      return collection(payload.reviews ?? payload.data, payload.total);
    },
    enforceReviewSLA: (params) =>
      client.post<ReviewSLAEnforcementResult>(`${apiPrefix}/reviews/sla/enforce${buildQuery(params)}`),
    approveAgent: async (id) => {
      await client.post<{ status: string }>(`${apiPrefix}/reviews/${id}/approve`);
    },
    rejectAgent: async (id, reason) => {
      await client.post<{ status: string }>(`${apiPrefix}/reviews/${id}/reject`, { reason });
    },
    requestAgentChanges: async (id, reason) => {
      await client.post<{ status: string }>(`${apiPrefix}/reviews/${id}/needs-changes`, { reason });
    },
    listMarketplaceAbuseReports: async (params) => {
      const payload = await client.get<MarketplaceAbuseReportsPayload>(`${apiPrefix}/marketplace/abuse-reports${buildQuery(params)}`);
      return collection(payload.reports ?? payload.data, payload.total);
    },
    resolveMarketplaceAbuseReport: (id, resolution) =>
      client.post<{ status: string }>(`${apiPrefix}/marketplace/abuse-reports/${id}/resolve`, { resolution }),
    dismissMarketplaceAbuseReport: (id, resolution) =>
      client.post<{ status: string }>(`${apiPrefix}/marketplace/abuse-reports/${id}/dismiss`, { resolution }),
    takedownMarketplaceAgent: (id, reason) =>
      client.post<{ status: string }>(`${apiPrefix}/marketplace/agents/${id}/takedown`, { reason }),
    reinstateMarketplaceAgent: (id, reason) =>
      client.post<{ status: string }>(`${apiPrefix}/marketplace/agents/${id}/reinstate`, { reason }),

    getBillingSummary: (params) => {
      const queryParams = {
        ...params,
        organizationID: params?.organizationID ?? params?.organizationId,
        userID: params?.userID ?? params?.userId,
      };
      delete queryParams.organizationId;
      delete queryParams.userId;
      return client.get<BillingSummary>(`${apiPrefix}/billing/summary${buildQuery(queryParams)}`);
    },
    listBillingSurface: async (surface, params) => {
      const queryParams = {
        ...params,
        organizationID: params?.organizationID ?? params?.organizationId,
        userID: params?.userID ?? params?.userId,
      };
      delete queryParams.organizationId;
      delete queryParams.userId;
      const payload = await client.get<BillingListPayload>(`${apiPrefix}/billing/${billingSurfacePaths[surface]}${buildQuery(queryParams)}`);
      return collection(payload[billingSurfaceKeys[surface]] ?? payload.data, payload.total);
    },
    refundTopup: (topupId, input) => {
      const body = {
        ...input,
        providerRefundID: input.providerRefundID ?? input.providerRefundId,
        providerChargeID: input.providerChargeID ?? input.providerChargeId,
        providerPaymentIntentID: input.providerPaymentIntentID ?? input.providerPaymentIntentId,
      };
      delete body.providerRefundId;
      delete body.providerChargeId;
      delete body.providerPaymentIntentId;
      return client.post<BillingInspectionRecord>(`${apiPrefix}/billing/topups/${topupId}/refund`, body);
    },
    createDueMarketplacePayouts: async () => {
      const payload = await client.post<BillingListPayload>(`${apiPrefix}/billing/payouts/create-due`);
      return collection(payload.payouts ?? payload.data, payload.total);
    },
    markMarketplacePayoutPaid: (payoutId, providerPayoutId) =>
      client.post<BillingInspectionRecord>(`${apiPrefix}/billing/payouts/${payoutId}/paid`, { providerPayoutID: providerPayoutId }),
    markMarketplacePayoutFailed: (payoutId, providerPayoutId, reason) =>
      client.post<BillingInspectionRecord>(`${apiPrefix}/billing/payouts/${payoutId}/failed`, { providerPayoutID: providerPayoutId, reason }),

    listObservabilityAlerts: async (params) => {
      const payload = await client.get<ObservabilityAlertsPayload>(`${observabilityAlertsPrefix}${buildQuery(params)}`);
      return collectionPayload(payload, ['alerts']);
    },
    getObservabilityAlert: (key) => client.get<AdminObservabilityAlertState>(`${observabilityAlertsPrefix}/${encodeURIComponent(key)}`),
    acknowledgeObservabilityAlert: (key) => client.post<AdminObservabilityAlertState>(`${observabilityAlertsPrefix}/${encodeURIComponent(key)}/acknowledge`),
    resolveObservabilityAlert: (key) => client.post<AdminObservabilityAlertState>(`${observabilityAlertsPrefix}/${encodeURIComponent(key)}/resolve`),
    listObservabilityAlertDeliveries: async (key, params) => {
      const payload = await client.get<ObservabilityDeliveryPayload>(`${observabilityAlertsPrefix}/${encodeURIComponent(key)}/deliveries${buildQuery(params)}`);
      return collectionPayload(payload, ['attempts', 'deliveries']);
    },
    listObservabilityRecoveryActions: async (params) => {
      const payload = await client.get<ObservabilityRecoveryActionsPayload>(`${apiPrefix}/observability/recovery-actions${buildQuery(params)}`);
      return collectionPayload(payload, ['actions']);
    },
    getObservabilityAlertRoutingRules: () => client.get<AdminObservabilityAlertRoutingRules>(`${apiPrefix}/observability/alert-routing`),
    updateObservabilityAlertRoutingRules: (rules) =>
      client.put<AdminObservabilityAlertRoutingRules>(`${apiPrefix}/observability/alert-routing`, { rules }),
    listObservabilityAlertProviders: async () => {
      const payload = await client.get<ObservabilityAlertProvidersPayload>(observabilityAlertProvidersPrefix);
      return collectionPayload(payload, ['providers']);
    },
    createObservabilityAlertProvider: (input) => client.post<AdminObservabilityAlertProvider>(observabilityAlertProvidersPrefix, input),
    updateObservabilityAlertProvider: (id, input) =>
      client.put<AdminObservabilityAlertProvider>(`${observabilityAlertProvidersPrefix}/${encodeURIComponent(id)}`, input),
    testObservabilityAlertProvider: (id) =>
      client.post<AdminObservabilityAlertProviderTestResult>(`${observabilityAlertProvidersPrefix}/${encodeURIComponent(id)}/test`),
  };
}
