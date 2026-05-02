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
  createdAt: string;
  updatedAt: string;
};

export type AgentTool = { name: string; description: string; inputSchema: Record<string, unknown> };
export type AgentExample = { userMessage: string; assistantMessage: string };
export type AgentReview = { id: string; agentID: string; userID: string; userName?: string; rating: number; body?: string; text?: string; createdAt: string; updatedAt?: string };
export type AgentVersion = { id?: string; agentID?: string; version: string; changelog?: string; status?: string; createdAt: string };
export type Category = { id: string; name: string; slug: string; displayOrder?: number; agentCount: number };
export type AgentInstall = { id: string; agentID: string; agentId?: string; agentName?: string; userID?: string; version?: string; installedAt: string };

export type AgentPublishRequest = {
  name: string;
  description: string;
  iconURL?: string;
  iconUrl?: string;
  categoryID?: string;
  categoryId?: string;
  categorySlug?: string;
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

export type MarketplaceHomeData = {
  featured: MarketplaceAgent[];
  popular: MarketplaceAgent[];
  topRated: MarketplaceAgent[];
  recent: MarketplaceAgent[];
  categories: Category[];
};

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
  getCuratedSections: () => Promise<{ popular: MarketplaceAgent[]; topRated: MarketplaceAgent[]; recent: MarketplaceAgent[] }>;
  getCategories: () => Promise<Category[]>;
  getAgent: (id: string) => Promise<MarketplaceAgent>;
  publishAgent: (input: AgentPublishRequest) => Promise<MarketplaceAgent>;
  updateAgent: (id: string, input: Partial<AgentPublishRequest>) => Promise<MarketplaceAgent>;
  deleteAgent: (id: string) => Promise<void>;
  searchAgents: (params: MarketplaceSearchParams) => Promise<{ agents: MarketplaceAgent[]; total: number }>;
  installAgent: (agentId: string, versionId?: string) => Promise<AgentInstall>;
  uninstallAgent: (agentId: string) => Promise<void>;
  getInstalledAgents: () => Promise<AgentInstall[]>;
  getMyAgents: (limit?: number, offset?: number) => Promise<MarketplaceAgent[]>;
  getReviews: (agentId: string, limit?: number, offset?: number) => Promise<AgentReview[]>;
  submitReview: (agentId: string, input: { rating: number; body?: string; text?: string }) => Promise<AgentReview>;
  getVersions: (agentId: string) => Promise<AgentVersion[]>;
};

type AgentListPayload = { agents?: MarketplaceAgent[]; data?: MarketplaceAgent[]; total?: number };
type CategoryListPayload = { categories?: Category[]; data?: Category[]; total?: number };
type AgentDetailPayload = { agent?: MarketplaceAgent; versions?: AgentVersion[] };
type InstallListPayload = { installs?: AgentInstall[]; data?: AgentInstall[]; total?: number };
type ReviewListPayload = { reviews?: AgentReview[]; data?: AgentReview[]; total?: number };
type VersionListPayload = { versions?: AgentVersion[]; data?: AgentVersion[]; total?: number };

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

function publishPayload(input: Partial<AgentPublishRequest>) {
  const payload = {
    ...input,
    iconURL: input.iconURL ?? input.iconUrl,
    categoryID: input.categoryID ?? input.categoryId ?? input.categorySlug,
  };
  delete payload.iconUrl;
  delete payload.categoryId;
  delete payload.categorySlug;
  return payload;
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

export function createMarketplaceApi(client: HttpClient): MarketplaceApi {
  const apiPrefix = '/api/v1/marketplace';

  return {
    getFeatured: async () => {
      const payload = await client.get<AgentListPayload>(`${apiPrefix}/featured`);
      return payload.agents ?? payload.data ?? [];
    },
    getCuratedSections: () => client.get<{ popular: MarketplaceAgent[]; topRated: MarketplaceAgent[]; recent: MarketplaceAgent[] }>(`${apiPrefix}/curated`),
    getCategories: async () => {
      const payload = await client.get<CategoryListPayload>(`${apiPrefix}/categories`);
      return payload.categories ?? payload.data ?? [];
    },

    getAgent: async (id) => {
      const payload = await client.get<MarketplaceAgent | AgentDetailPayload>(`${apiPrefix}/agents/${id}`);
      return 'agent' in payload && payload.agent ? payload.agent : payload as MarketplaceAgent;
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

    installAgent: (agentId, versionId) =>
      client.post<AgentInstall>(`${apiPrefix}/agents/${agentId}/install${buildQuery({ versionID: versionId })}`),
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
  };
}
