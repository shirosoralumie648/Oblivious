export type AdminStats = {
  users: {
    totalUsers: number;
    activeUsers: number;
    newUsersToday: number;
    newUsersWeek: number;
  };
  quotas: {
    totalBalance: number;
    totalUsed: number;
    activeTopups: number;
  };
  conversations: number;
  agents: number;
  tasks: number;
  mcpServers: number;
  channelsTotal: number;
  channelsOnline: number;
  activeAgents: number;
  apiCalls24h: number;
  dailyStats?: Array<{ date: string; calls: number }>;
  modelBreakdown?: Array<{ model: string; count: number }>;
};

export type ChannelStatus = 'online' | 'degraded' | 'offline';

export type ChannelBalance = {
  amount: number;
  currency: string;
  source?: string;
};

export type ChannelHealthDetail = {
  status: ChannelStatus;
  message?: string;
  checkedAt?: string;
};

export type ChannelInfo = {
  id: string;
  organizationID: string;
  organizationId?: string;
  name: string;
  provider: string;
  baseURL: string;
  baseUrl?: string;
  models: string[];
  groups: string[];
  rpm: number;
  tpm: number;
  priority: number;
  estimatedCostPer1K?: number;
  costMultiplier?: number;
  weight?: number;
  enabled: boolean;
  status: ChannelStatus;
  latency: number | null;
  createdAt: string;
  updatedAt: string;
};

export type ChannelProviderInfo = {
  id: string;
  displayName: string;
  kind: string;
  status: string;
  defaultBaseURL: string;
  configurable?: boolean;
  installable?: boolean;
  runtimeReady?: boolean;
};

export type ChannelCreateRequest = {
  name: string;
  provider: string;
  apiKey: string;
  baseURL: string;
  baseUrl?: string;
  models: string[];
  groups: string[];
  rpmLimit: number;
  tpmLimit: number;
  priority: number;
  estimatedCostPer1K: number;
  costMultiplier: number;
  weight?: number;
};

export type ChannelUpdateRequest = Partial<ChannelCreateRequest> & {
  enabled?: boolean;
};

export type RelayPricingSettings = {
  modelMultipliers: Record<string, number>;
  groupMultipliers: Record<string, number>;
};

export type RelayPricingCatalogEntry = {
  id?: string;
  apiType: string;
  model: string;
  dimension: string;
  unitCost: number;
  markup: number;
  currency: string;
  source: string;
  effectiveFrom?: string;
  active: boolean;
};

export type RelayPricingCatalogImportRequest = {
  provider: string;
  source: string;
  sourceHash?: string;
  notes?: string;
  deactivateMissing?: boolean;
  effectiveFrom?: string;
  entries: RelayPricingCatalogEntry[];
};

export type RelayPricingCatalogSyncRequest = {
  provider: string;
  source?: string;
  sourceUrl?: string;
  sourceJson?: unknown;
  notes?: string;
  deactivateMissing?: boolean;
  effectiveFrom?: string;
  requiredModels?: string[];
  maxBytes?: number;
};

export type RelayPricingCatalogRejectRequest = {
  reason?: string;
};

export type RelayPricingCatalogRollbackRequest = {
  notes?: string;
};

export type RelayPricingCatalogDiffEntry = {
  action: string;
  key: string;
  before?: RelayPricingCatalogEntry;
  after?: RelayPricingCatalogEntry;
  reason?: string;
  applied?: boolean;
  appliedId?: string;
};

export type RelayPricingCatalogDiff = {
  added: number;
  updated: number;
  unchanged: number;
  deactivated: number;
  entries: RelayPricingCatalogDiffEntry[];
};

export type RelayPricingCatalogImport = {
  id: string;
  provider: string;
  source: string;
  sourceHash?: string;
  status: string;
  notes?: string;
  deactivateMissing: boolean;
  importedBy?: string;
  importedByEmail?: string;
  approvedBy?: string;
  approvedByEmail?: string;
  entries: RelayPricingCatalogEntry[];
  diff: RelayPricingCatalogDiff;
  createdAt: string;
  approvedAt?: string;
};

export type RelayPricingCatalogImportFilter = {
  provider?: string;
  source?: string;
  status?: string;
  limit?: number;
  offset?: number;
};

export type RelayPricingCatalogSyncRun = {
  id: string;
  job: string;
  provider?: string;
  source?: string;
  sourceRef?: string;
  sourceHash?: string;
  status: string;
  importId?: string;
  entryCount: number;
  missingRequiredModels?: string[];
  error?: string;
  startedAt: string;
  finishedAt?: string;
};

export type RelayPricingCatalogSyncRunFilter = {
  job?: string;
  provider?: string;
  source?: string;
  status?: string;
  limit?: number;
  offset?: number;
};

export type UsageLimitSettings = {
  organizationId?: string;
  organizationID?: string;
  userId?: string;
  userID?: string;
  quotaMode?: 'organization' | 'user' | string;
  maxConcurrentRequests: number;
  windowSeconds: number;
  maxTokensPerWindow: number;
  maxTokensPerRequest?: number;
  updatedAt?: string;
};

export type ChannelTestResult = {
  success: boolean;
  latency: number;
  latencyMs?: number;
  provider?: string;
  models?: string[];
  balance?: ChannelBalance | null;
  balanceError?: string;
  health?: ChannelHealthDetail | null;
  error?: string;
};

export type ChannelModelSyncResult = {
  channel: ChannelInfo;
  testResult: ChannelTestResult;
};

export type ChannelModelUpdatePreview = {
  id: string;
  currentModels: string[];
  upstreamModels: string[];
  added: string[];
  removed: string[];
  unchanged: string[];
  testResult?: ChannelTestResult;
};

export type ChannelModelUpdateApplyRequest = {
  mode?: 'merge' | 'replace';
};

export type ChannelModelUpdateApplyResult = {
  channel?: ChannelInfo;
  preview?: ChannelModelUpdatePreview;
  mode: 'merge' | 'replace';
  appliedModels: string[];
};

export type ChannelBalanceRefreshResult = {
  id: string;
  status: ChannelStatus;
  balance?: ChannelBalance | null;
  balanceError?: string;
  channelHealth?: ChannelHealthDetail | null;
  testResult: ChannelTestResult;
  checkedAt?: string;
};

export type ChannelHealth = {
  id?: string;
  status: ChannelStatus;
  latency: number;
  models?: string[];
  balance?: ChannelBalance | null;
  balanceError?: string;
  health?: ChannelHealthDetail | null;
  error?: string;
  checkedAt?: string;
};

export type ChannelRuntimeStats = {
  channelID: string;
  channelId?: string;
  rpmCurrent: number;
  tpmCurrent: number;
  totalRequests: number;
  successCount: number;
  failureCount: number;
  avgLatencyMs: number;
  rateLimitedUntil?: string;
  affinityConversationCount?: number;
};

export type ModelInventoryChannel = {
  id: string;
  name: string;
  provider: string;
  groups: string[];
  enabled: boolean;
  priority: number;
  estimatedCostPer1K: number;
  costMultiplier: number;
};

export type ModelInventoryEntry = {
  model: string;
  providers: string[];
  groups: string[];
  channelCount: number;
  enabledChannelCount: number;
  disabledChannelCount: number;
  minEstimatedCostPer1K: number;
  maxEstimatedCostPer1K: number;
  avgCostMultiplier: number;
  requestCount: number;
  totalCost: number;
  totalChannelCost: number;
  channels: ModelInventoryChannel[];
};

export type ModelInventoryFilter = {
  provider?: string;
  group?: string;
  status?: string;
  search?: string;
  sort?: string;
  limit?: number;
  offset?: number;
};

export type RouteChannel = {
  channelID: string;
  channelName?: string;
  weight: number;
  priority: number;
  enabled: boolean;
};

export type RouteChannelInput = {
  channelID: string;
  weight: number;
  priority: number;
  enabled: boolean;
};

export type RouteStrategy = 'adaptive' | 'weighted' | 'priority' | 'cost_aware';

export type RouteInfo = {
  id: string;
  model: string;
  strategy: RouteStrategy | string;
  channels: RouteChannel[];
  createdAt: string;
  modelPattern?: string;
  targetChannelId?: string;
  targetChannelName?: string;
  priority?: number;
  enabled?: boolean;
};

export type RouteCreateRequest = {
  model: string;
  strategy: RouteStrategy;
  channels: RouteChannelInput[];
};

export type RouteUpdateRequest = Partial<RouteCreateRequest>;

export type PlanInfo = {
  id: string;
  name: string;
  description: string;
  quotaAmount: number;
  tokenQuota: number;
  price: number;
  modelAccess: string[];
  agentLimit: number;
  maxTokensPerRequest: number;
  durationDays?: number | null;
  isActive: boolean;
  isPublic: boolean;
  sortOrder: number;
  subscriberCount?: number;
  createdAt: string;
  updatedAt?: string;
};

export type PlanCreateRequest = {
  name: string;
  description: string;
  quotaAmount: number;
  tokenQuota: number;
  price: number;
  modelAccess: string[];
  agentLimit: number;
  maxTokensPerRequest: number;
  durationDays?: number | null;
  isPublic: boolean;
  sortOrder: number;
};

export type PlanUpdateRequest = Partial<PlanCreateRequest> & {
  isActive?: boolean;
};

export type UserUsageStats = {
  totalTokens: number;
  totalAPICalls: number;
  totalCost: number;
};

export type UserDetail = {
  id: string;
  email: string;
  name: string;
  role: string;
  planID?: string | null;
  planId?: string | null;
  planName?: string | null;
  quotaBalance?: number;
  status: 'active' | 'disabled';
  lastLoginAt: string | null;
  createdAt: string;
  usageStats?: UserUsageStats | null;
  totalTokens?: number;
  totalApiCalls?: number;
  totalCost?: number;
};

export type UserUpdateRequest = {
  role?: string;
  planID?: string | null;
  planId?: string | null;
  status?: 'active' | 'disabled';
};

export type UserQuotaUpdateRequest = {
  balance: number;
};

export type AuditEntry = {
  id: string;
  actorID: string;
  actorId?: string;
  actorEmail: string;
  action: string;
  resourceType: string;
  resourceID?: string;
  resourceId?: string;
  changes?: string;
  ipAddress?: string;
  createdAt: string;
};

export type UsageLogEntry = {
  id: string;
  organizationId?: string;
  userId: string;
  apiTokenId?: string;
  requestId?: string;
  apiType?: string;
  featureType?: string;
  quotaMode?: string;
  model: string;
  channelId?: string;
  provider?: string;
  status?: string;
  statusCode?: number;
  errorCode?: string;
  latencyMs?: number;
  cost: number;
  channelCost: number;
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  createdAt: string;
};

export type UsageLogFilter = {
  organizationID?: string;
  organizationId?: string;
  userID?: string;
  userId?: string;
  apiTokenID?: string;
  apiTokenId?: string;
  requestID?: string;
  requestId?: string;
  apiType?: string;
  featureType?: string;
  quotaMode?: string;
  model?: string;
  channelID?: string;
  channelId?: string;
  provider?: string;
  status?: string;
  limit?: number;
  offset?: number;
};

export type UsageAnalyticsBucket = {
  dimension: 'model' | 'feature' | 'user' | 'time' | 'channel' | 'provider' | string;
  key: string;
  requestCount: number;
  totalTokens: number;
  totalCost: number;
  startedAt?: string;
};

export type UsageAnalyticsCrossDimensionBucket = UsageAnalyticsBucket & {
  dimension: 'model_time' | 'user_feature' | 'feature_time' | string;
  primary?: string;
  secondary?: string;
};

export type UsageAnalyticsResponse = {
  byModel: UsageAnalyticsBucket[];
  byFeature: UsageAnalyticsBucket[];
  byUser: UsageAnalyticsBucket[];
  byTime: UsageAnalyticsBucket[];
  byChannel: UsageAnalyticsBucket[];
  byProvider: UsageAnalyticsBucket[];
  crossDimensions?: UsageAnalyticsCrossDimensionBucket[];
};

export type UsageAnalyticsFilter = {
  organizationID?: string;
  organizationId?: string;
  userID?: string;
  userId?: string;
  apiType?: string;
  model?: string;
  channelID?: string;
  channelId?: string;
  provider?: string;
  status?: string;
  granularity?: 'second' | 'minute' | 'hour' | 'day' | 'week' | 'month';
  featureType?: string;
  quotaMode?: string;
  from?: string;
  to?: string;
  limit?: number;
};

export type RelayUsagePriceReconciliationIssue = {
  id: string;
  organizationId?: string;
  userId: string;
  apiTokenId?: string;
  requestId?: string;
  apiType?: string;
  featureType?: string;
  quotaMode?: string;
  model: string;
  channelId?: string;
  provider?: string;
  status?: string;
  cost: number;
  snapshotTotalCost: number;
  deltaCost: number;
  priceCurrency?: string;
  priceSource?: string;
  issue: 'missing_snapshot' | 'cost_mismatch' | string;
  createdAt: string;
};

export type RelayUsagePriceReconciliationResponse = {
  checkedRecords: number;
  matchedRecords: number;
  missingSnapshotRecords: number;
  mismatchedRecords: number;
  ledgerTotalCost: number;
  snapshotTotalCost: number;
  deltaCost: number;
  issues: RelayUsagePriceReconciliationIssue[];
  limit: number;
  offset: number;
};

export type RelayUsagePriceReconciliationFilter = UsageLogFilter & {
  from?: string;
  to?: string;
};

export type UsageRequestLogCoverageIssue = {
  id: string;
  requestId?: string;
  model?: string;
  issue: 'missing_request_id' | 'missing_request_log' | string;
  createdAt?: string;
};

export type UsageRequestLogCoverageResponse = {
  checkedRecords: number;
  usageRowsWithRequestId: number;
  usageRowsMissingRequestId: number;
  matchedRequestLogRecords: number;
  missingRequestLogRecords: number;
  issues: UsageRequestLogCoverageIssue[];
  limit: number;
  offset: number;
};

export type APITokenEntry = {
  id: string;
  organizationId: string;
  userId: string;
  userEmail: string;
  name: string;
  tokenPrefix: string;
  status: string;
  userGroup?: string;
  modelLimitsEnabled: boolean;
  modelLimits: string[];
  quotaLimit?: number | null;
  usedQuota: number;
  requestCount: number;
  totalCost: number;
  expiresAt?: string | null;
  lastUsedAt?: string | null;
  createdAt: string;
  revokedAt?: string | null;
};

export type APITokenFilter = {
  organizationID?: string;
  organizationId?: string;
  userID?: string;
  userId?: string;
  status?: string;
  userGroup?: string;
  search?: string;
  model?: string;
  limit?: number;
  offset?: number;
};

export type ReviewSLA = {
  submittedAt?: string;
  automatedReviewDeadlineAt: string;
  automatedReviewSlaMinutes: number;
  automatedReviewSlaStatus: string;
  manualDeadlineAt: string;
  manualSlaHours: number;
  manualSlaStatus: string;
  minutesUntilDeadline: number;
  vipPublisher: boolean;
  publisherTier: string;
  publisherTierSource: string;
};

export type ReviewSLAEnforcementResult = {
  scanned: number;
  alerted: number;
};

export type PublishedAgent = {
  id: string;
  name: string;
  description: string;
  iconURL?: string;
  iconUrl?: string;
  ownerID: string;
  ownerId?: string;
  ownerName: string;
  status: 'draft' | 'pending_review' | 'pending' | 'approved' | 'rejected' | 'needs_changes' | 'takedown';
  reviewReason?: string;
  rejectionReason?: string | null;
  visibility: 'public' | 'private' | 'unlisted';
  pricingType?: 'free' | 'one_time' | 'subscription';
  pricingAmount?: number;
  categoryID?: string;
  categoryId?: string;
  categoryName?: string;
  tags: string[];
  ratingAvg?: number;
  rating?: number;
  ratingCount: number;
  installCount: number;
  createdAt: string;
  updatedAt: string;
  publisherReviewTier?: string;
  reviewSLA?: ReviewSLA;
};

export type MarketplaceAbuseReport = {
  id: string;
  reporterOrganizationId: string;
  reporterUserId: string;
  agentId: string;
  reason: string;
  details?: string;
  status: 'open' | 'resolved' | 'dismissed';
  resolution?: string;
  reviewerUserId?: string;
  createdAt: string;
  updatedAt: string;
};

export type PaginatedResponse<T> = {
  data: T[];
  total: number;
};

export type BillingSummaryMetric = {
  count: number;
  totalAmount?: number;
  preAuthorizedAmount?: number;
  settledAmount?: number;
  refundedAmount?: number;
  paidAmount?: number;
  amountDue?: number;
  amountPaid?: number;
  grossAmount?: number;
  platformFeeAmount?: number;
  publisherNetAmount?: number;
  failedCount?: number;
  activeCount?: number;
};

export type BillingSummary = {
  billingSessions?: BillingSummaryMetric;
  paymentIntents?: BillingSummaryMetric;
  webhookEvents?: BillingSummaryMetric;
  subscriptions?: BillingSummaryMetric;
  topups?: BillingSummaryMetric;
  invoices?: BillingSummaryMetric;
  refunds?: BillingSummaryMetric;
  settlements?: BillingSummaryMetric;
  payouts?: BillingSummaryMetric;
};

export type BillingSurface =
  | 'sessions'
  | 'paymentIntents'
  | 'webhookEvents'
  | 'subscriptions'
  | 'topups'
  | 'invoices'
  | 'refunds'
  | 'settlements'
  | 'payouts';

export type BillingFilter = {
  organizationID?: string;
  organizationId?: string;
  userID?: string;
  userId?: string;
  status?: string;
  kind?: string;
  provider?: string;
  limit?: number;
  offset?: number;
};

export type BillingInspectionRecord = {
  id: string;
  organizationId?: string;
  userId?: string;
  publisherOrganizationId?: string;
  publisherUserId?: string;
  provider?: string;
  kind?: string;
  status?: string;
  model?: string;
  apiType?: string;
  eventId?: string;
  eventType?: string;
  packageId?: string;
  paymentIntentId?: string;
  providerChargeId?: string;
  topupOrderId?: string;
  providerPaymentIntentId?: string;
  providerCheckoutSessionId?: string;
  providerSubscriptionId?: string;
  providerInvoiceId?: string;
  providerRefundId?: string;
  providerPayoutId?: string;
  agentId?: string;
  amount?: number;
  money?: number;
  totalAmount?: number;
  preAuthorizedAmount?: number;
  settledAmount?: number;
  refundedAmount?: number;
  amountDue?: number;
  amountPaid?: number;
  grossAmount?: number;
  platformFeeAmount?: number;
  publisherNetAmount?: number;
  currency?: string;
  createdAt?: string;
  updatedAt?: string;
  receivedAt?: string;
  processedAt?: string | null;
  error?: string;
  reason?: string;
};

export type TopupRefundRequest = {
  provider?: string;
  providerRefundID?: string;
  providerRefundId?: string;
  providerChargeID?: string;
  providerChargeId?: string;
  providerPaymentIntentID?: string;
  providerPaymentIntentId?: string;
  amount: number;
  currency?: string;
  reason?: string;
};

export type MarketplacePayoutFailedRequest = {
  providerPayoutID?: string;
  providerPayoutId?: string;
  reason: string;
};
