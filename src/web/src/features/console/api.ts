import type { HttpClient } from '../../services/http/client';
import type { UsageSummary } from '../../types/api';

export type ConsoleApi = {
  getUsage: () => Promise<UsageSummary>;
};

export function createConsoleApi(client: HttpClient): ConsoleApi {
  return {
    getUsage: () => client.get<UsageSummary>('/api/v1/console/usage')
  };
}
