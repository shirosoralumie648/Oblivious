import type { HttpClient } from '../../services/http/client';
import type { BillingSummary, ConsoleAccessSummary, ConsoleModelSummary, UsageSummary } from '../../types/api';

export type ConsoleApi = {
  getAccess: () => Promise<ConsoleAccessSummary>;
  getBilling: () => Promise<BillingSummary>;
  getModels: () => Promise<ConsoleModelSummary[]>;
  getUsage: () => Promise<UsageSummary>;
};

export function createConsoleApi(client: HttpClient): ConsoleApi {
  return {
    getAccess: () => client.get<ConsoleAccessSummary>('/api/v1/console/access'),
    getBilling: () => client.get<BillingSummary>('/api/v1/console/billing'),
    getModels: () => client.get<ConsoleModelSummary[]>('/api/v1/console/models'),
    getUsage: () => client.get<UsageSummary>('/api/v1/console/usage')
  };
}
