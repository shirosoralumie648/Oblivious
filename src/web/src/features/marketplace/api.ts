import type { HttpClient } from '../../services/http/client';

export type MarketplaceAgent = {
  id: string;
  ownerID: string;
  ownerId?: string;
  ownerName?: string;
  name: string;
  description: string;
  iconURL?: string;
  iconUrl?: string;
  categoryID?: string;
  categoryId?: string;
  categorySlug?: string;
  categoryName?: string;
  tags: string[];
  tools: string;
  exampleConversations: string;
  systemPrompt?: string;
  visibility: 'public' | 'private' | 'unlisted';
  status: string;
  reviewReason?: string;
  pricingType: 'free' | 'one_time' | 'subscription';
  pricingAmount: number;
  currentVersion?: string;
  version?: string;
  latestVersion?: string;
  installCount: number;
  ratingAvg: number;
  rating?: number;
  ratingCount: number;
  recommendation?: {
    score: number;
    reason: string;
  };
  paymentProviders?: PaymentProvider[];
  createdAt: string;
  updatedAt: string;
};

export type PaymentProvider = {
  name: string;
};

export type AgentTool = { name: string; description: string; inputSchema: Record<string, unknown> };
export type AgentExample = { userMessage: string; assistantMessage: string };
export type AgentReview = { id: string; agentID: string; userID: string; userName?: string; rating: number; body?: string; text?: string; createdAt: string; updatedAt?: string };
export type AgentVersion = { id?: string; agentID?: string; version: string; changelog?: string; status?: string; createdAt: string };
export type Category = { id: string; name: string; slug: string; displayOrder?: number; agentCount: number };
export type AgentInstall = { id: string; agentID: string; agentId?: string; agentName?: string; userID?: string; version?: string; installedAt: string };
export type MarketplaceCheckoutResponse = {
  checkoutSessionId?: string;
  checkoutSessionID?: string;
  url?: string;
  checkoutUrl?: string;
  checkoutURL?: string;
};
export type MarketplaceInstallResult = AgentInstall | MarketplaceCheckoutResponse;
export type SettlementCycle = 'weekly' | 'monthly' | 'quarterly';

export type MarketplaceSettlementPreferences = {
  cycle: SettlementCycle;
  label?: string;
  description?: string;
  payoutBusinessDays: number;
  processingFeePercent: number;
  minimumPayoutAmount: number;
  effectiveFrom: string;
};

export type MarketplaceRevenueTier = {
  currentTier: string;
  label: string;
  monthlySalesAmount: number;
  platformFeeAmount: number;
  publisherNetAmount: number;
  platformFeePercent: number;
  publisherSharePercent: number;
  effectivePlatformFeePercent: number;
  nextTierAt?: number;
  salesToNextTier?: number;
  estimatedPublisherNetAtNextTier?: number;
  estimatedPublisherNetIncreaseAtNextTier?: number;
};

export type PublisherAgentStats = {
  agentID: string;
  agentName: string;
  installCount: number;
  activeUsers: number;
  apiCallCount: number;
};

export type PublisherStats = {
  totalAgents: number;
  totalInstalls: number;
  activeUsers: number;
  totalAPICalls: number;
  grossRevenue: number;
  platformFees: number;
  netRevenue: number;
  refundedAmount: number;
  pendingSettlementAmount: number;
  availableAmount: number;
  payoutPendingAmount: number;
  paidOutAmount: number;
  revenueTier?: MarketplaceRevenueTier;
  perAgentStats?: PublisherAgentStats[];
};

export type AgentPublishRequest = {
  name: string;
  description: string;
  iconURL?: string;
  iconUrl?: string;
  categoryID: string;
  tags: string[];
  tools: string;
  exampleConversations: string;
  systemPrompt?: string;
  visibility: 'public' | 'private' | 'unlisted';
  pricingType: 'free' | 'one_time' | 'subscription';
  pricingAmount: number;
  version: string;
  changelog?: string;
};

export type AutomatedReviewFinding = {
  type: string;
  severity: string;
  field?: string;
  message: string;
  evidence?: string;
};

export type AutomatedReviewResult = {
  agentID?: string;
  decision?: string;
  scanner?: string;
  findings: AutomatedReviewFinding[];
  createdAt?: string;
};

export type AutomatedReviewRejectionError = Error & {
  code?: string;
  data?: {
    automatedReview?: AutomatedReviewResult;
  };
};

export type MarketplaceTemplate = {
  id: string;
  organizationId?: string;
  type: 'agent' | 'workflow' | 'plugin';
  name: string;
  description?: string;
  templateData: unknown;
  category?: string;
  tags: string[];
  downloadsCount: number;
  ratingAvg?: number;
  createdAt?: string;
  updatedAt?: string;
};

export type TemplateCreateRequest = {
  type: 'agent' | 'workflow' | 'plugin';
  name: string;
  description?: string;
  templateData: unknown;
  category?: string;
  tags: string[];
};

export type TemplateSearchParams = {
  query?: string;
  q?: string;
  type?: 'agent' | 'workflow' | 'plugin';
  category?: string;
  tags?: string | string[];
  limit?: number;
  offset?: number;
};

export type TemplateInstall = {
  id: string;
  templateID: string;
  templateId?: string;
  organizationId?: string;
  userID?: string;
  type: 'agent' | 'workflow' | 'plugin';
  name: string;
  templateData: unknown;
  installedAt?: string;
};

export type MarketplaceHomeData = {
  featured: MarketplaceAgent[];
  popular: MarketplaceAgent[];
  topRated: MarketplaceAgent[];
  recent: MarketplaceAgent[];
  categories: Category[];
};

export type CuratedMarketplaceSections = Pick<MarketplaceHomeData, 'popular' | 'topRated' | 'recent'>;

export type MarketplaceSearchParams = {
  query?: string;
  q?: string;
  categorySlug?: string;
  category?: string;
  tags?: string | string[];
  minRating?: number;
  maxRating?: number;
  pricingType?: 'free' | 'one_time' | 'subscription';
  priceFilter?: 'all' | 'free' | 'paid';
  sort?: 'relevance' | 'rating' | 'installs' | 'newest' | 'popular' | 'recommended';
  limit?: number;
  offset?: number;
};

export type MarketplaceApi = {
  getFeatured: () => Promise<MarketplaceAgent[]>;
  getCuratedSections: () => Promise<CuratedMarketplaceSections>;
  getCategories: () => Promise<Category[]>;
  getAgent: (id: string) => Promise<MarketplaceAgent>;
  publishAgent: (input: AgentPublishRequest) => Promise<MarketplaceAgent>;
  updateAgent: (id: string, input: AgentPublishRequest) => Promise<MarketplaceAgent>;
  deleteAgent: (id: string) => Promise<void>;
  searchAgents: (params: MarketplaceSearchParams) => Promise<{ agents: MarketplaceAgent[]; total: number }>;
  installAgent: (agentId: string, versionId?: string, paymentProvider?: string) => Promise<MarketplaceInstallResult>;
  uninstallAgent: (agentId: string) => Promise<void>;
  getInstalledAgents: () => Promise<AgentInstall[]>;
  getMyAgents: (limit?: number, offset?: number) => Promise<MarketplaceAgent[]>;
  getPublisherStats: () => Promise<PublisherStats>;
  getSettlementPreferences: () => Promise<MarketplaceSettlementPreferences>;
  updateSettlementPreferences: (cycle: SettlementCycle) => Promise<MarketplaceSettlementPreferences>;
  getReviews: (agentId: string, limit?: number, offset?: number) => Promise<AgentReview[]>;
  submitReview: (agentId: string, input: { rating: number; body?: string; text?: string }) => Promise<AgentReview>;
  getVersions: (agentId: string) => Promise<AgentVersion[]>;
  listTemplates: (params?: TemplateSearchParams) => Promise<{ templates: MarketplaceTemplate[]; total: number }>;
  getTemplate: (id: string) => Promise<MarketplaceTemplate>;
  createTemplate: (input: TemplateCreateRequest) => Promise<MarketplaceTemplate>;
  installTemplate: (templateId: string) => Promise<TemplateInstall>;
};

type AgentListPayload = { agents?: MarketplaceAgent[]; data?: MarketplaceAgent[]; total?: number };
type CategoryListPayload = { categories?: Category[]; data?: Category[]; total?: number };
type AgentDetailPayload = { agent?: MarketplaceAgent; versions?: AgentVersion[]; paymentProviders?: PaymentProvider[] };
type InstallListPayload = { installs?: AgentInstall[]; data?: AgentInstall[]; total?: number };
type ReviewListPayload = { reviews?: AgentReview[]; data?: AgentReview[]; total?: number };
type VersionListPayload = { versions?: AgentVersion[]; data?: AgentVersion[]; total?: number };
type TemplateListPayload = { templates?: MarketplaceTemplate[]; data?: MarketplaceTemplate[]; total?: number };
type TemplateDetailPayload = { template?: MarketplaceTemplate };

function buildQuery(params?: Record<string, string | number | boolean | string[] | undefined | null>) {
  const query = new URLSearchParams();

  Object.entries(params ?? {}).forEach(([key, value]) => {
    if (Array.isArray(value)) {
      if (value.length > 0) {
        query.set(key, value.join(','));
      }
      return;
    }
    if (value !== undefined && value !== null && value !== '') {
      query.set(key, String(value));
    }
  });

  const serialized = query.toString();
  return serialized ? `?${serialized}` : '';
}

function publishPayload(input: Partial<AgentPublishRequest> & Record<string, unknown>): Partial<AgentPublishRequest> {
  const payload: Record<string, unknown> = {
    ...input,
    iconURL: input.iconURL ?? input.iconUrl,
  };
  delete payload.iconUrl;
  delete payload.categoryId;
  delete payload.categorySlug;
  return payload as Partial<AgentPublishRequest>;
}

function searchParams(params: MarketplaceSearchParams) {
  const pricingType = params.pricingType ?? (params.priceFilter === 'free' ? 'free' : undefined);
  return {
    ...params,
    query: params.query ?? params.q,
    categorySlug: params.categorySlug ?? params.category,
    tags: Array.isArray(params.tags) ? params.tags.join(',') : params.tags,
    pricingType,
    priceFilter: undefined,
    q: undefined,
    category: undefined,
  };
}

function templateSearchParams(params?: TemplateSearchParams) {
  return {
    ...params,
    query: params?.query ?? params?.q,
    tags: Array.isArray(params?.tags) ? params.tags.join(',') : params?.tags,
    q: undefined,
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function isAutomatedReviewFinding(value: unknown): value is AutomatedReviewFinding {
  if (!isRecord(value)) {
    return false;
  }
  return typeof value.type === 'string' && typeof value.severity === 'string' && typeof value.message === 'string';
}

export function getAutomatedReviewRejection(error: unknown): AutomatedReviewResult | null {
  if (!isRecord(error) || error.code !== 'automated_review_rejected' || !isRecord(error.data)) {
    return null;
  }

  const automatedReview = error.data.automatedReview;
  if (!isRecord(automatedReview) || !Array.isArray(automatedReview.findings)) {
    return null;
  }

  const findings = automatedReview.findings.filter(isAutomatedReviewFinding);
  if (findings.length === 0) {
    return null;
  }

  return {
    agentID: typeof automatedReview.agentID === 'string' ? automatedReview.agentID : undefined,
    decision: typeof automatedReview.decision === 'string' ? automatedReview.decision : undefined,
    scanner: typeof automatedReview.scanner === 'string' ? automatedReview.scanner : undefined,
    findings,
    createdAt: typeof automatedReview.createdAt === 'string' ? automatedReview.createdAt : undefined,
  };
}

export function isAutomatedReviewRejection(error: unknown): error is AutomatedReviewRejectionError {
  return getAutomatedReviewRejection(error) !== null;
}

export function getMarketplaceCheckoutUrl(result: MarketplaceInstallResult): string | null {
  if (!isRecord(result)) {
    return null;
  }

  const payload: Record<string, unknown> = result;
  const checkoutUrl = payload.url ?? payload.checkoutUrl ?? payload.checkoutURL;
  return typeof checkoutUrl === 'string' && checkoutUrl.trim() !== '' ? checkoutUrl : null;
}

export function isMarketplaceCheckoutResponse(result: MarketplaceInstallResult): result is MarketplaceCheckoutResponse {
  return getMarketplaceCheckoutUrl(result) !== null;
}

export function createMarketplaceApi(client: HttpClient): MarketplaceApi {
  const apiPrefix = '/api/v1/marketplace';

  return {
    getFeatured: async () => {
      const payload = await client.get<AgentListPayload>(`${apiPrefix}/featured`);
      return payload.agents ?? payload.data ?? [];
    },
    getCuratedSections: () => client.get<CuratedMarketplaceSections>(`${apiPrefix}/curated`),
    getCategories: async () => {
      const payload = await client.get<CategoryListPayload>(`${apiPrefix}/categories`);
      return payload.categories ?? payload.data ?? [];
    },

    getAgent: async (id) => {
      const payload = await client.get<MarketplaceAgent | AgentDetailPayload>(`${apiPrefix}/agents/${id}`);
      if ('agent' in payload && payload.agent) {
        return {
          ...payload.agent,
          paymentProviders: payload.paymentProviders,
        };
      }
      return payload as MarketplaceAgent;
    },
    publishAgent: (input) => client.post<MarketplaceAgent>(`${apiPrefix}/agents`, publishPayload(input)),
    updateAgent: (id, input) => client.put<MarketplaceAgent>(`${apiPrefix}/agents/${id}`, publishPayload(input)),
    deleteAgent: async (id) => {
      await client.delete<{ status: string }>(`${apiPrefix}/agents/${id}`);
    },

    searchAgents: async (params) => {
      const payload = await client.get<AgentListPayload>(`${apiPrefix}/search${buildQuery(searchParams(params))}`);
      return { agents: payload.agents ?? payload.data ?? [], total: payload.total ?? payload.agents?.length ?? payload.data?.length ?? 0 };
    },

    installAgent: (agentId, versionId, paymentProvider) =>
      client.post<MarketplaceInstallResult>(`${apiPrefix}/agents/${agentId}/install${buildQuery({ versionID: versionId, provider: paymentProvider })}`),
    uninstallAgent: async (agentId) => {
      await client.delete<{ status: string }>(`${apiPrefix}/installs/${agentId}`);
    },
    getInstalledAgents: async () => {
      const payload = await client.get<InstallListPayload>(`${apiPrefix}/installs`);
      return payload.installs ?? payload.data ?? [];
    },

    getMyAgents: async (limit, offset) => {
      const payload = await client.get<AgentListPayload>(`${apiPrefix}/my-agents${buildQuery({ limit, offset })}`);
      return payload.agents ?? payload.data ?? [];
    },
    getPublisherStats: () => client.get<PublisherStats>(`${apiPrefix}/publisher/stats`),
    getSettlementPreferences: () => client.get<MarketplaceSettlementPreferences>(`${apiPrefix}/publisher/settlement-preferences`),
    updateSettlementPreferences: (cycle) =>
      client.put<MarketplaceSettlementPreferences>(`${apiPrefix}/publisher/settlement-preferences`, { cycle }),

    getReviews: async (agentId, limit, offset) => {
      const payload = await client.get<ReviewListPayload>(`${apiPrefix}/agents/${agentId}/reviews${buildQuery({ limit, offset })}`);
      return payload.reviews ?? payload.data ?? [];
    },
    submitReview: (agentId, input) => client.post<AgentReview>(`${apiPrefix}/agents/${agentId}/reviews`, {
      rating: input.rating,
      body: input.body ?? input.text ?? '',
    }),

    getVersions: async (agentId) => {
      const payload = await client.get<VersionListPayload>(`${apiPrefix}/agents/${agentId}/versions`);
      return payload.versions ?? payload.data ?? [];
    },

    listTemplates: async (params = {}) => {
      const payload = await client.get<TemplateListPayload>(`${apiPrefix}/templates${buildQuery(templateSearchParams(params))}`);
      return { templates: payload.templates ?? payload.data ?? [], total: payload.total ?? payload.templates?.length ?? payload.data?.length ?? 0 };
    },
    getTemplate: async (id) => {
      const payload = await client.get<MarketplaceTemplate | TemplateDetailPayload>(`${apiPrefix}/templates/${id}`);
      return 'template' in payload && payload.template ? payload.template : payload as MarketplaceTemplate;
    },
    createTemplate: (input) => client.post<MarketplaceTemplate>(`${apiPrefix}/templates`, input),
    installTemplate: (templateId) => client.post<TemplateInstall>(`${apiPrefix}/templates/${templateId}/install`),
  };
}
