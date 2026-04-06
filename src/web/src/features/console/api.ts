import type { HttpClient } from '../../services/http/client';
import type { AccessSummary, BillingSummary, ModelSummary, UsageSummary } from '../../types/api';

export type ConsoleApi = {
  getAccess: () => Promise<AccessSummary>;
  getBilling: () => Promise<BillingSummary>;
  getModels: () => Promise<ModelSummary[]>;
  getUsage: () => Promise<UsageSummary>;
};

export function createConsoleApi(client: HttpClient): ConsoleApi {
  return {
    getAccess: () => client.get<AccessSummary>('/api/v1/console/access'),
    getBilling: () => client.get<BillingSummary>('/api/v1/console/billing'),
    getModels: () => client.get<ModelSummary[]>('/api/v1/console/models'),
    getUsage: () => client.get<UsageSummary>('/api/v1/console/usage')
  };
}
