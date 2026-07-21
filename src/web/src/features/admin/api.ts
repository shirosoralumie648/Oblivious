import {
  acknowledgeAdminObservabilityAlertOperationContract,
  applyAdminChannelModelUpdatesOperationContract,
  approveAdminMarketplaceReviewOperationContract,
  approveAdminRelayPricingCatalogImportOperationContract,
  batchUpdateAdminChannelsOperationContract,
  claimAdminMarketplaceReviewOperationContract,
  createAdminChannelOperationContract,
  createAdminObservabilityAlertProviderOperationContract,
  createAdminPlanOperationContract,
  createAdminRelayPricingCatalogImportOperationContract,
  createAdminRouteOperationContract,
  createDueAdminMarketplacePayoutsOperationContract,
  deactivateAdminPlanOperationContract,
  deleteAdminChannelOperationContract,
  deleteAdminRouteOperationContract,
  detectAdminChannelModelUpdatesOperationContract,
  disableAdminUserOperationContract,
  dismissMarketplaceAbuseReportOperationContract,
  enableAdminUserOperationContract,
  enforceAdminMarketplaceReviewSLAOperationContract,
  getAdminBillingSummaryOperationContract,
  getAdminChannelHealthOperationContract,
  getAdminChannelOperationContract,
  getAdminObservabilityAlertOperationContract,
  getAdminObservabilityAlertRoutingOperationContract,
  getAdminPlanOperationContract,
  getAdminRelayPricingSettingsOperationContract,
  getAdminRelayUsagePriceReconciliationOperationContract,
  getAdminRouteOperationContract,
  getAdminStatsOperationContract,
  getAdminUsageAnalyticsOperationContract,
  getAdminUsageRequestLogCoverageOperationContract,
  getAdminUserOperationContract,
  listAdminAPITokensOperationContract,
  listAdminAuditLogsOperationContract,
  listAdminBillingSessionsOperationContract,
  listAdminBillingWebhookEventsOperationContract,
  listAdminChannelProvidersOperationContract,
  listAdminChannelRuntimeStatsOperationContract,
  listAdminChannelsOperationContract,
  listAdminInvoicesOperationContract,
  listAdminMarketplacePayoutsOperationContract,
  listAdminMarketplaceReviewsOperationContract,
  listAdminMarketplaceSettlementsOperationContract,
  listAdminModelInventoryOperationContract,
  listAdminObservabilityAlertDeliveriesOperationContract,
  listAdminObservabilityAlertProvidersOperationContract,
  listAdminObservabilityAlertsOperationContract,
  listAdminObservabilityRecoveryActionsOperationContract,
  listAdminPaymentIntentsOperationContract,
  listAdminPlansOperationContract,
  listAdminRefundsOperationContract,
  listAdminRelayPricingCatalogImportsOperationContract,
  listAdminRelayPricingCatalogSyncRunsOperationContract,
  listAdminRoutesOperationContract,
  listAdminSubscriptionsOperationContract,
  listAdminTopupsOperationContract,
  listAdminUsageLimitSettingsOperationContract,
  listAdminUsageLogsOperationContract,
  listAdminUsersOperationContract,
  listMarketplaceAbuseReportsOperationContract,
  markAdminMarketplacePayoutFailedOperationContract,
  markAdminMarketplacePayoutPaidOperationContract,
  recordAdminTopupRefundOperationContract,
  refreshAdminChannelBalanceOperationContract,
  reinstateMarketplaceAgentOperationContract,
  rejectAdminMarketplaceReviewOperationContract,
  rejectAdminRelayPricingCatalogImportOperationContract,
  rejectMarketplaceAgentAppealOperationContract,
  requestChangesForAdminMarketplaceReviewOperationContract,
  resolveAdminObservabilityAlertOperationContract,
  resolveMarketplaceAbuseReportOperationContract,
  revokeAdminAPITokenOperationContract,
  rollbackAdminRelayPricingCatalogImportOperationContract,
  syncAdminChannelModelsOperationContract,
  syncAdminRelayPricingCatalogImportOperationContract,
  takedownMarketplaceAgentOperationContract,
  testAdminChannelOperationContract,
  testAdminObservabilityAlertProviderOperationContract,
  updateAdminChannelOperationContract,
  updateAdminObservabilityAlertProviderOperationContract,
  updateAdminObservabilityAlertRoutingOperationContract,
  updateAdminPlanOperationContract,
  updateAdminRelayPricingSettingsOperationContract,
  updateAdminRouteOperationContract,
  updateAdminUsageLimitSettingsOperationContract,
  updateAdminUserOperationContract,
  updateAdminUserQuotaOperationContract,
  type OperationContractMetadataV1
} from '@/generated/operation-contracts.generated';
import {
  jsonEnvelopeDecoder,
  jsonRequestEncoder,
  noneRequestEncoder,
  type HttpClient,
  type OperationTransportContract
} from '../../services/http/client';
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
  RelayPricingCatalogImport,
  RelayPricingCatalogImportFilter,
  RelayPricingCatalogImportRequest,
  RelayPricingCatalogRejectRequest,
  RelayPricingCatalogRollbackRequest,
  RelayPricingCatalogSyncRequest,
  RelayPricingCatalogSyncRun,
  RelayPricingCatalogSyncRunFilter,
  RelayUsagePriceReconciliationFilter,
  RelayUsagePriceReconciliationResponse,
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
  UsageRequestLogCoverageResponse,
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

type RelayPricingCatalogImportListPayload = {
  imports?: RelayPricingCatalogImport[];
  data?: RelayPricingCatalogImport[];
  total?: number;
};

type RelayPricingCatalogSyncRunListPayload = {
  runs?: RelayPricingCatalogSyncRun[];
  data?: RelayPricingCatalogSyncRun[];
  total?: number;
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
  listRelayPricingCatalogImports: (params?: RelayPricingCatalogImportFilter) => Promise<PaginatedResponse<RelayPricingCatalogImport>>;
  createRelayPricingCatalogImport: (input: RelayPricingCatalogImportRequest) => Promise<RelayPricingCatalogImport>;
  syncRelayPricingCatalogImport: (input: RelayPricingCatalogSyncRequest) => Promise<RelayPricingCatalogImport>;
  approveRelayPricingCatalogImport: (id: string) => Promise<RelayPricingCatalogImport>;
  rejectRelayPricingCatalogImport: (id: string, input: RelayPricingCatalogRejectRequest | string) => Promise<RelayPricingCatalogImport>;
  rollbackRelayPricingCatalogImport: (id: string, input?: RelayPricingCatalogRollbackRequest) => Promise<RelayPricingCatalogImport>;
  listRelayPricingCatalogSyncRuns: (params?: RelayPricingCatalogSyncRunFilter) => Promise<PaginatedResponse<RelayPricingCatalogSyncRun>>;
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
  getRelayUsagePriceReconciliation: (params?: RelayUsagePriceReconciliationFilter) => Promise<RelayUsagePriceReconciliationResponse>;
  getUsageRequestLogCoverage: (params?: UsageLogFilter) => Promise<UsageRequestLogCoverageResponse>;
  listAPITokens: (params?: APITokenFilter) => Promise<PaginatedResponse<APITokenEntry>>;
  revokeAPIToken: (id: string) => Promise<void>;
  listReviews: (params?: { status?: string; limit?: number; offset?: number }) => Promise<PaginatedResponse<PublishedAgent>>;
  enforceReviewSLA: (params?: { limit?: number; offset?: number }) => Promise<ReviewSLAEnforcementResult>;
  claimReview: (id: string) => Promise<void>;
  approveAgent: (id: string) => Promise<void>;
  rejectAgent: (id: string, reason: string) => Promise<void>;
  requestAgentChanges: (id: string, reason: string) => Promise<void>;
  listMarketplaceAbuseReports: (params?: { status?: string; limit?: number; offset?: number }) => Promise<PaginatedResponse<MarketplaceAbuseReport>>;
  resolveMarketplaceAbuseReport: (id: string, resolution: string) => Promise<{ status: 'resolved' | 'dismissed' | string }>;
  dismissMarketplaceAbuseReport: (id: string, resolution: string) => Promise<{ status: 'resolved' | 'dismissed' | string }>;
  takedownMarketplaceAgent: (id: string, reason: string) => Promise<{ status: string }>;
  reinstateMarketplaceAgent: (id: string, reason: string) => Promise<{ status: string }>;
  rejectMarketplaceAgentAppeal: (id: string, reason: string) => Promise<{ status: string }>;
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

function jsonTransport<T>(
  operation: OperationContractMetadataV1,
  status = 200
): OperationTransportContract<T> {
  return {
    operation,
    requestEncoder: operation.request.mediaType === null
      ? noneRequestEncoder(operation)
      : jsonRequestEncoder(operation),
    responseDecoder: jsonEnvelopeDecoder<T>(operation, status)
  };
}

export function createAdminApi(client: HttpClient): AdminApi {
  const apiPrefix = '/api/v1/admin';
  const relayPricingCatalogPrefix = `${apiPrefix}/pricing/relay-catalog`;
  const observabilityAlertsPrefix = `${apiPrefix}/observability/alerts`;
  const observabilityAlertProvidersPrefix = `${apiPrefix}/observability/alert-providers`;

  return {
    getStats: () => client.get<AdminStats>(`${apiPrefix}/stats`, undefined, jsonTransport(getAdminStatsOperationContract)),
    getRelayPricingSettings: () =>
      client.get<RelayPricingSettings>(
        `${apiPrefix}/settings/relay-pricing`,
        undefined,
        jsonTransport(getAdminRelayPricingSettingsOperationContract)
      ),
    updateRelayPricingSettings: (input) =>
      client.put<RelayPricingSettings>(
        `${apiPrefix}/settings/relay-pricing`,
        input,
        undefined,
        jsonTransport(updateAdminRelayPricingSettingsOperationContract)
      ),
    listRelayPricingCatalogImports: async (params) => {
      const payload = await client.get<RelayPricingCatalogImportListPayload>(
        `${relayPricingCatalogPrefix}/imports${buildQuery(params)}`,
        undefined,
        jsonTransport(listAdminRelayPricingCatalogImportsOperationContract)
      );
      return collection(payload.imports ?? payload.data, payload.total);
    },
    createRelayPricingCatalogImport: (input) =>
      client.post<RelayPricingCatalogImport>(
        `${relayPricingCatalogPrefix}/imports`,
        input,
        undefined,
        jsonTransport(createAdminRelayPricingCatalogImportOperationContract, 201)
      ),
    syncRelayPricingCatalogImport: (input) =>
      client.post<RelayPricingCatalogImport>(
        `${relayPricingCatalogPrefix}/sync`,
        input,
        undefined,
        jsonTransport(syncAdminRelayPricingCatalogImportOperationContract, 201)
      ),
    approveRelayPricingCatalogImport: (id) =>
      client.post<RelayPricingCatalogImport>(
        `${relayPricingCatalogPrefix}/imports/${encodeURIComponent(id)}/approve`,
        undefined,
        undefined,
        jsonTransport(approveAdminRelayPricingCatalogImportOperationContract)
      ),
    rejectRelayPricingCatalogImport: (id, input) =>
      client.post<RelayPricingCatalogImport>(
        `${relayPricingCatalogPrefix}/imports/${encodeURIComponent(id)}/reject`,
        typeof input === 'string' ? { reason: input } : input,
        undefined,
        jsonTransport(rejectAdminRelayPricingCatalogImportOperationContract)
      ),
    rollbackRelayPricingCatalogImport: (id, input = {}) =>
      client.post<RelayPricingCatalogImport>(
        `${relayPricingCatalogPrefix}/imports/${encodeURIComponent(id)}/rollback`,
        input,
        undefined,
        jsonTransport(rollbackAdminRelayPricingCatalogImportOperationContract, 201)
      ),
    listRelayPricingCatalogSyncRuns: async (params) => {
      const payload = await client.get<RelayPricingCatalogSyncRunListPayload>(
        `${relayPricingCatalogPrefix}/sync-runs${buildQuery(params)}`,
        undefined,
        jsonTransport(listAdminRelayPricingCatalogSyncRunsOperationContract)
      );
      return collection(payload.runs ?? payload.data, payload.total);
    },
    getUsageLimitSettings: async () => {
      const payload = await client.get<UsageLimitSettingsPayload>(
        `${apiPrefix}/settings/usage-limits`,
        undefined,
        jsonTransport(listAdminUsageLimitSettingsOperationContract)
      );
      return payload.usageLimits ?? payload.data ?? [];
    },
    updateUsageLimitSettings: (input) =>
      client.put<UsageLimitSettings>(
        `${apiPrefix}/settings/usage-limits`,
        input,
        undefined,
        jsonTransport(updateAdminUsageLimitSettingsOperationContract)
      ),

    listChannels: async (params) => {
      const payload = await client.get<ChannelListPayload>(
        `${apiPrefix}/channels${buildQuery(params)}`,
        undefined,
        jsonTransport(listAdminChannelsOperationContract)
      );
      return collection(payload.channels ?? payload.data, payload.total);
    },
    listChannelProviders: async () => {
      const payload = await client.get<ChannelProviderListPayload>(
        `${apiPrefix}/channel-providers`,
        undefined,
        jsonTransport(listAdminChannelProvidersOperationContract)
      );
      return payload.providers ?? payload.data ?? [];
    },
    listChannelStats: async () => {
      const payload = await client.get<ChannelRuntimeStatsPayload>(
        `${apiPrefix}/channels/stats`,
        undefined,
        jsonTransport(listAdminChannelRuntimeStatsOperationContract)
      );
      return payload.stats ?? payload.data ?? [];
    },
    listModelInventory: async (params) => {
      const payload = await client.get<ModelInventoryListPayload>(
        `${apiPrefix}/models${buildQuery(params)}`,
        undefined,
        jsonTransport(listAdminModelInventoryOperationContract)
      );
      return collection(payload.models ?? payload.data, payload.total);
    },
    getChannel: (id) =>
      client.get<ChannelInfo>(`${apiPrefix}/channels/${id}`, undefined, jsonTransport(getAdminChannelOperationContract)),
    createChannel: (input) =>
      client.post<ChannelInfo>(
        `${apiPrefix}/channels`,
        input,
        undefined,
        jsonTransport(createAdminChannelOperationContract, 201)
      ),
    updateChannel: (id, input) =>
      client.put<ChannelInfo>(
        `${apiPrefix}/channels/${id}`,
        input,
        undefined,
        jsonTransport(updateAdminChannelOperationContract)
      ),
    deleteChannel: async (id) => {
      await client.delete<{ status: string }>(
        `${apiPrefix}/channels/${id}`,
        undefined,
        jsonTransport(deleteAdminChannelOperationContract)
      );
    },
    testChannel: (id) =>
      client.post<ChannelTestResult>(
        `${apiPrefix}/channels/${id}/test`,
        undefined,
        undefined,
        jsonTransport(testAdminChannelOperationContract)
      ),
    syncChannelModels: (id) =>
      client.post<ChannelModelSyncResult>(
        `${apiPrefix}/channels/${id}/sync-models`,
        undefined,
        undefined,
        jsonTransport(syncAdminChannelModelsOperationContract)
      ),
    detectChannelModelUpdates: (id) =>
      client.post<ChannelModelUpdatePreview>(
        `${apiPrefix}/channels/${id}/model-updates/detect`,
        undefined,
        undefined,
        jsonTransport(detectAdminChannelModelUpdatesOperationContract)
      ),
    applyChannelModelUpdates: (id, input = { mode: 'merge' }) =>
      client.post<ChannelModelUpdateApplyResult>(
        `${apiPrefix}/channels/${id}/model-updates/apply`,
        input,
        undefined,
        jsonTransport(applyAdminChannelModelUpdatesOperationContract)
      ),
    refreshChannelBalance: (id) =>
      client.post<ChannelBalanceRefreshResult>(
        `${apiPrefix}/channels/${id}/refresh-balance`,
        undefined,
        undefined,
        jsonTransport(refreshAdminChannelBalanceOperationContract)
      ),
    getChannelHealth: (id) =>
      client.get<ChannelHealth>(
        `${apiPrefix}/channels/${id}/health`,
        undefined,
        jsonTransport(getAdminChannelHealthOperationContract)
      ),
    batchUpdateChannels: async (ids, action) => {
      await client.post<{ status: string }>(
        `${apiPrefix}/channels/batch`,
        { ids, action: toBatchAction(action) },
        undefined,
        jsonTransport(batchUpdateAdminChannelsOperationContract)
      );
    },

    listRoutes: async () => {
      const payload = await client.get<RouteListPayload>(
        `${apiPrefix}/routes`,
        undefined,
        jsonTransport(listAdminRoutesOperationContract)
      );
      return payload.routes ?? payload.data ?? [];
    },
    getRoute: (id) =>
      client.get<RouteInfo>(`${apiPrefix}/routes/${id}`, undefined, jsonTransport(getAdminRouteOperationContract)),
    createRoute: (input) =>
      client.post<RouteInfo>(
        `${apiPrefix}/routes`,
        input,
        undefined,
        jsonTransport(createAdminRouteOperationContract, 201)
      ),
    updateRoute: (id, input) =>
      client.put<RouteInfo>(
        `${apiPrefix}/routes/${id}`,
        input,
        undefined,
        jsonTransport(updateAdminRouteOperationContract)
      ),
    deleteRoute: async (id) => {
      await client.delete<{ status: string }>(
        `${apiPrefix}/routes/${id}`,
        undefined,
        jsonTransport(deleteAdminRouteOperationContract)
      );
    },

    listPlans: async (params) => {
      const payload = await client.get<PlanListPayload>(
        `${apiPrefix}/plans${buildQuery(params)}`,
        undefined,
        jsonTransport(listAdminPlansOperationContract)
      );
      return payload.plans ?? payload.data ?? [];
    },
    getPlan: (id) =>
      client.get<PlanInfo>(`${apiPrefix}/plans/${id}`, undefined, jsonTransport(getAdminPlanOperationContract)),
    createPlan: (input) =>
      client.post<PlanInfo>(
        `${apiPrefix}/plans`,
        input,
        undefined,
        jsonTransport(createAdminPlanOperationContract, 201)
      ),
    updatePlan: (id, input) =>
      client.put<PlanInfo>(
        `${apiPrefix}/plans/${id}`,
        input,
        undefined,
        jsonTransport(updateAdminPlanOperationContract)
      ),
    deactivatePlan: async (id) => {
      await client.delete<{ status: string }>(
        `${apiPrefix}/plans/${id}`,
        undefined,
        jsonTransport(deactivateAdminPlanOperationContract)
      );
    },

    listUsers: async (params) => {
      const queryParams = { ...params, planID: params?.planID ?? params?.planId };
      delete queryParams.planId;
      const payload = await client.get<UserListPayload>(
        `${apiPrefix}/users${buildQuery(queryParams)}`,
        undefined,
        jsonTransport(listAdminUsersOperationContract)
      );
      return collection(payload.users ?? payload.data, payload.total);
    },
    getUser: (id) =>
      client.get<UserDetail>(`${apiPrefix}/users/${id}`, undefined, jsonTransport(getAdminUserOperationContract)),
    updateUser: (id, input) => {
      const body = { ...input, planID: input.planID ?? input.planId };
      delete body.planId;
      return client.put<UserDetail>(
        `${apiPrefix}/users/${id}`,
        body,
        undefined,
        jsonTransport(updateAdminUserOperationContract)
      );
    },
    updateUserQuota: (id, input) =>
      client.request<UserDetail>(`${apiPrefix}/users/${id}`, {
        method: 'PATCH',
        body: JSON.stringify({ balance: input.balance }),
      }, jsonTransport(updateAdminUserQuotaOperationContract)),
    disableUser: async (id) => {
      await client.post<{ status: string }>(
        `${apiPrefix}/users/${id}/disable`,
        undefined,
        undefined,
        jsonTransport(disableAdminUserOperationContract)
      );
    },
    enableUser: async (id) => {
      await client.post<{ status: string }>(
        `${apiPrefix}/users/${id}/enable`,
        undefined,
        undefined,
        jsonTransport(enableAdminUserOperationContract)
      );
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
      const payload = await client.get<AuditListPayload>(
        `${apiPrefix}/audit-logs${buildQuery(queryParams)}`,
        undefined,
        jsonTransport(listAdminAuditLogsOperationContract)
      );
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
      const payload = await client.get<UsageLogListPayload>(
        `${apiPrefix}/usage-logs${buildQuery(queryParams)}`,
        undefined,
        jsonTransport(listAdminUsageLogsOperationContract)
      );
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
      return client.get<UsageAnalyticsResponse>(
        `${apiPrefix}/usage-analytics${buildQuery(queryParams)}`,
        undefined,
        jsonTransport(getAdminUsageAnalyticsOperationContract)
      );
    },

    getRelayUsagePriceReconciliation: (params) => {
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
        from: params?.from,
        to: params?.to,
        limit: params?.limit,
        offset: params?.offset,
      };
      return client.get<RelayUsagePriceReconciliationResponse>(
        `${apiPrefix}/billing/reconciliation/relay-usage-prices${buildQuery(queryParams)}`,
        undefined,
        jsonTransport(getAdminRelayUsagePriceReconciliationOperationContract)
      );
    },

    getUsageRequestLogCoverage: (params) => {
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
      return client.get<UsageRequestLogCoverageResponse>(
        `${apiPrefix}/billing/reconciliation/usage-request-logs${buildQuery(queryParams)}`,
        undefined,
        jsonTransport(getAdminUsageRequestLogCoverageOperationContract)
      );
    },

    listAPITokens: async (params) => {
      const queryParams = {
        ...params,
        organizationID: params?.organizationID ?? params?.organizationId,
        userID: params?.userID ?? params?.userId,
      };
      delete queryParams.organizationId;
      delete queryParams.userId;
      const payload = await client.get<APITokenListPayload>(
        `${apiPrefix}/api-tokens${buildQuery(queryParams)}`,
        undefined,
        jsonTransport(listAdminAPITokensOperationContract)
      );
      return collection(payload.apiTokens ?? payload.data, payload.total);
    },
    revokeAPIToken: async (id) => {
      await client.post<{ status: string }>(
        `${apiPrefix}/api-tokens/${id}/revoke`,
        undefined,
        undefined,
        jsonTransport(revokeAdminAPITokenOperationContract)
      );
    },

    listReviews: async (params) => {
      const payload = await client.get<ReviewListPayload>(
        `${apiPrefix}/reviews${buildQuery(params)}`,
        undefined,
        jsonTransport(listAdminMarketplaceReviewsOperationContract)
      );
      return collection(payload.reviews ?? payload.data, payload.total);
    },
    enforceReviewSLA: (params) =>
      client.post<ReviewSLAEnforcementResult>(
        `${apiPrefix}/reviews/sla/enforce${buildQuery(params)}`,
        undefined,
        undefined,
        jsonTransport(enforceAdminMarketplaceReviewSLAOperationContract)
      ),
    claimReview: async (id) => {
      await client.post<{ status: string }>(
        `${apiPrefix}/reviews/${id}/claim`,
        undefined,
        undefined,
        jsonTransport(claimAdminMarketplaceReviewOperationContract)
      );
    },
    approveAgent: async (id) => {
      await client.post<{ status: string }>(
        `${apiPrefix}/reviews/${id}/approve`,
        undefined,
        undefined,
        jsonTransport(approveAdminMarketplaceReviewOperationContract)
      );
    },
    rejectAgent: async (id, reason) => {
      await client.post<{ status: string }>(
        `${apiPrefix}/reviews/${id}/reject`,
        { reason },
        undefined,
        jsonTransport(rejectAdminMarketplaceReviewOperationContract)
      );
    },
    requestAgentChanges: async (id, reason) => {
      await client.post<{ status: string }>(
        `${apiPrefix}/reviews/${id}/needs-changes`,
        { reason },
        undefined,
        jsonTransport(requestChangesForAdminMarketplaceReviewOperationContract)
      );
    },
    listMarketplaceAbuseReports: async (params) => {
      const payload = await client.get<MarketplaceAbuseReportsPayload>(
        `${apiPrefix}/marketplace/abuse-reports${buildQuery(params)}`,
        undefined,
        jsonTransport(listMarketplaceAbuseReportsOperationContract)
      );
      return collection(payload.reports ?? payload.data, payload.total);
    },
    resolveMarketplaceAbuseReport: (id, resolution) =>
      client.post<{ status: string }>(
        `${apiPrefix}/marketplace/abuse-reports/${id}/resolve`,
        { resolution },
        undefined,
        jsonTransport(resolveMarketplaceAbuseReportOperationContract)
      ),
    dismissMarketplaceAbuseReport: (id, resolution) =>
      client.post<{ status: string }>(
        `${apiPrefix}/marketplace/abuse-reports/${id}/dismiss`,
        { resolution },
        undefined,
        jsonTransport(dismissMarketplaceAbuseReportOperationContract)
      ),
    takedownMarketplaceAgent: (id, reason) =>
      client.post<{ status: string }>(
        `${apiPrefix}/marketplace/agents/${id}/takedown`,
        { reason },
        undefined,
        jsonTransport(takedownMarketplaceAgentOperationContract)
      ),
    reinstateMarketplaceAgent: (id, reason) =>
      client.post<{ status: string }>(
        `${apiPrefix}/marketplace/agents/${id}/reinstate`,
        { reason },
        undefined,
        jsonTransport(reinstateMarketplaceAgentOperationContract)
      ),
    rejectMarketplaceAgentAppeal: (id, reason) =>
      client.post<{ status: string }>(
        `${apiPrefix}/marketplace/agents/${id}/reject-appeal`,
        { reason },
        undefined,
        jsonTransport(rejectMarketplaceAgentAppealOperationContract)
      ),

    getBillingSummary: (params) => {
      const queryParams = {
        ...params,
        organizationID: params?.organizationID ?? params?.organizationId,
        userID: params?.userID ?? params?.userId,
      };
      delete queryParams.organizationId;
      delete queryParams.userId;
      return client.get<BillingSummary>(
        `${apiPrefix}/billing/summary${buildQuery(queryParams)}`,
        undefined,
        jsonTransport(getAdminBillingSummaryOperationContract)
      );
    },
    listBillingSurface: async (surface, params) => {
      const queryParams = {
        ...params,
        organizationID: params?.organizationID ?? params?.organizationId,
        userID: params?.userID ?? params?.userId,
      };
      delete queryParams.organizationId;
      delete queryParams.userId;
      const path = `${apiPrefix}/billing/${billingSurfacePaths[surface]}${buildQuery(queryParams)}`;
      let payload: BillingListPayload;
      switch (surface) {
        case 'sessions':
          payload = await client.get<BillingListPayload>(path, undefined, jsonTransport(listAdminBillingSessionsOperationContract));
          break;
        case 'paymentIntents':
          payload = await client.get<BillingListPayload>(path, undefined, jsonTransport(listAdminPaymentIntentsOperationContract));
          break;
        case 'webhookEvents':
          payload = await client.get<BillingListPayload>(path, undefined, jsonTransport(listAdminBillingWebhookEventsOperationContract));
          break;
        case 'subscriptions':
          payload = await client.get<BillingListPayload>(path, undefined, jsonTransport(listAdminSubscriptionsOperationContract));
          break;
        case 'topups':
          payload = await client.get<BillingListPayload>(path, undefined, jsonTransport(listAdminTopupsOperationContract));
          break;
        case 'invoices':
          payload = await client.get<BillingListPayload>(path, undefined, jsonTransport(listAdminInvoicesOperationContract));
          break;
        case 'refunds':
          payload = await client.get<BillingListPayload>(path, undefined, jsonTransport(listAdminRefundsOperationContract));
          break;
        case 'settlements':
          payload = await client.get<BillingListPayload>(path, undefined, jsonTransport(listAdminMarketplaceSettlementsOperationContract));
          break;
        case 'payouts':
          payload = await client.get<BillingListPayload>(path, undefined, jsonTransport(listAdminMarketplacePayoutsOperationContract));
          break;
      }
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
      return client.post<BillingInspectionRecord>(
        `${apiPrefix}/billing/topups/${topupId}/refund`,
        body,
        undefined,
        jsonTransport(recordAdminTopupRefundOperationContract)
      );
    },
    createDueMarketplacePayouts: async () => {
      const payload = await client.post<BillingListPayload>(
        `${apiPrefix}/billing/payouts/create-due`,
        undefined,
        undefined,
        jsonTransport(createDueAdminMarketplacePayoutsOperationContract)
      );
      return collection(payload.payouts ?? payload.data, payload.total);
    },
    markMarketplacePayoutPaid: (payoutId, providerPayoutId) =>
      client.post<BillingInspectionRecord>(
        `${apiPrefix}/billing/payouts/${payoutId}/paid`,
        { providerPayoutID: providerPayoutId },
        undefined,
        jsonTransport(markAdminMarketplacePayoutPaidOperationContract)
      ),
    markMarketplacePayoutFailed: (payoutId, providerPayoutId, reason) =>
      client.post<BillingInspectionRecord>(
        `${apiPrefix}/billing/payouts/${payoutId}/failed`,
        { providerPayoutID: providerPayoutId, reason },
        undefined,
        jsonTransport(markAdminMarketplacePayoutFailedOperationContract)
      ),

    listObservabilityAlerts: async (params) => {
      const payload = await client.get<ObservabilityAlertsPayload>(
        `${observabilityAlertsPrefix}${buildQuery(params)}`,
        undefined,
        jsonTransport(listAdminObservabilityAlertsOperationContract)
      );
      return collectionPayload(payload, ['alerts']);
    },
    getObservabilityAlert: (key) =>
      client.get<AdminObservabilityAlertState>(
        `${observabilityAlertsPrefix}/${encodeURIComponent(key)}`,
        undefined,
        jsonTransport(getAdminObservabilityAlertOperationContract)
      ),
    acknowledgeObservabilityAlert: (key) =>
      client.post<AdminObservabilityAlertState>(
        `${observabilityAlertsPrefix}/${encodeURIComponent(key)}/acknowledge`,
        undefined,
        undefined,
        jsonTransport(acknowledgeAdminObservabilityAlertOperationContract)
      ),
    resolveObservabilityAlert: (key) =>
      client.post<AdminObservabilityAlertState>(
        `${observabilityAlertsPrefix}/${encodeURIComponent(key)}/resolve`,
        undefined,
        undefined,
        jsonTransport(resolveAdminObservabilityAlertOperationContract)
      ),
    listObservabilityAlertDeliveries: async (key, params) => {
      const payload = await client.get<ObservabilityDeliveryPayload>(
        `${observabilityAlertsPrefix}/${encodeURIComponent(key)}/deliveries${buildQuery(params)}`,
        undefined,
        jsonTransport(listAdminObservabilityAlertDeliveriesOperationContract)
      );
      return collectionPayload(payload, ['attempts', 'deliveries']);
    },
    listObservabilityRecoveryActions: async (params) => {
      const payload = await client.get<ObservabilityRecoveryActionsPayload>(
        `${apiPrefix}/observability/recovery-actions${buildQuery(params)}`,
        undefined,
        jsonTransport(listAdminObservabilityRecoveryActionsOperationContract)
      );
      return collectionPayload(payload, ['actions']);
    },
    getObservabilityAlertRoutingRules: () =>
      client.get<AdminObservabilityAlertRoutingRules>(
        `${apiPrefix}/observability/alert-routing`,
        undefined,
        jsonTransport(getAdminObservabilityAlertRoutingOperationContract)
      ),
    updateObservabilityAlertRoutingRules: (rules) =>
      client.put<AdminObservabilityAlertRoutingRules>(
        `${apiPrefix}/observability/alert-routing`,
        { rules },
        undefined,
        jsonTransport(updateAdminObservabilityAlertRoutingOperationContract)
      ),
    listObservabilityAlertProviders: async () => {
      const payload = await client.get<ObservabilityAlertProvidersPayload>(
        observabilityAlertProvidersPrefix,
        undefined,
        jsonTransport(listAdminObservabilityAlertProvidersOperationContract)
      );
      return collectionPayload(payload, ['providers']);
    },
    createObservabilityAlertProvider: (input) =>
      client.post<AdminObservabilityAlertProvider>(
        observabilityAlertProvidersPrefix,
        input,
        undefined,
        jsonTransport(createAdminObservabilityAlertProviderOperationContract, 201)
      ),
    updateObservabilityAlertProvider: (id, input) =>
      client.put<AdminObservabilityAlertProvider>(
        `${observabilityAlertProvidersPrefix}/${encodeURIComponent(id)}`,
        input,
        undefined,
        jsonTransport(updateAdminObservabilityAlertProviderOperationContract)
      ),
    testObservabilityAlertProvider: (id) =>
      client.post<AdminObservabilityAlertProviderTestResult>(
        `${observabilityAlertProvidersPrefix}/${encodeURIComponent(id)}/test`,
        undefined,
        undefined,
        jsonTransport(testAdminObservabilityAlertProviderOperationContract)
      ),
  };
}
