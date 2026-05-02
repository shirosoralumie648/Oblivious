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
};

export type ChannelStatus = 'online' | 'degraded' | 'offline';

export type ChannelInfo = {
  id: string;
  name: string;
  provider: string;
  baseURL: string;
  baseUrl?: string;
  models: string[];
  rpm: number;
  tpm: number;
  priority: number;
  weight?: number;
  enabled: boolean;
  status: ChannelStatus;
  latency: number | null;
  createdAt: string;
  updatedAt: string;
};

export type ChannelCreateRequest = {
  name: string;
  provider: string;
  apiKey: string;
  baseURL: string;
  baseUrl?: string;
  models: string[];
  rpmLimit: number;
  tpmLimit: number;
  priority: number;
  weight?: number;
};

export type ChannelUpdateRequest = Partial<ChannelCreateRequest> & {
  enabled?: boolean;
};

export type ChannelTestResult = {
  success: boolean;
  latency: number;
  latencyMs?: number;
  error?: string;
};

export type ChannelHealth = {
  id?: string;
  status: ChannelStatus;
  latency: number;
  error?: string;
  checkedAt?: string;
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

export type RouteInfo = {
  id: string;
  model: string;
  strategy: string;
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
  strategy: string;
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

export type PublishedAgent = {
  id: string;
  name: string;
  description: string;
  iconURL?: string;
  iconUrl?: string;
  ownerID: string;
  ownerId?: string;
  ownerName: string;
  status: 'draft' | 'pending_review' | 'pending' | 'approved' | 'rejected';
  reviewReason?: string;
  rejectionReason?: string | null;
  visibility: 'public' | 'private' | 'unlisted';
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
};

export type PaginatedResponse<T> = {
  data: T[];
  total: number;
};
