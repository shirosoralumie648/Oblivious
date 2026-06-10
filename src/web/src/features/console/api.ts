import type { HttpClient } from '../../services/http/client';
import type { AccessSummary, BillingPaymentProviderSummary, BillingSummary as BaseBillingSummary, CreatedRelayApiToken, CreateRelayApiTokenRequest, ModelSummary, PackageOption, RelayApiToken, RelayApiTokenUsageItem, UsageSummary } from '../../types/api';

export type ConsoleBillingInvoiceSummary = {
  id: string;
  status: string;
  amountUsd: number;
  dueAt: string;
  hostedInvoiceUrl?: string;
  invoicePdf?: string;
};

export type ConsoleBillingSummary = BaseBillingSummary & {
  balanceUsd: number;
  creditLimitUsd: number;
  currentSpendUsd: number;
  nextInvoice?: ConsoleBillingInvoiceSummary;
  paymentProviders?: BillingPaymentProviderSummary[];
};

export type BillingCheckoutProvider = BillingPaymentProviderSummary['name'];

export type BillingCheckoutRequest = {
  amount?: number;
  kind: 'subscription' | 'topup';
  packageId?: string;
  provider?: BillingCheckoutProvider;
};

export type BillingCheckoutSession = {
  checkoutSessionId: string;
  url: string;
};

export type ConsoleApi = {
  createApiToken: (request: CreateRelayApiTokenRequest) => Promise<CreatedRelayApiToken>;
  createBillingCheckout: (request: BillingCheckoutRequest) => Promise<BillingCheckoutSession>;
  getAccess: () => Promise<AccessSummary>;
  getBilling: () => Promise<ConsoleBillingSummary>;
  getModels: () => Promise<ModelSummary[]>;
  getUsage: () => Promise<UsageSummary>;
  listPackages: () => Promise<PackageOption[]>;
  listInvoices: () => Promise<ConsoleBillingInvoiceSummary[]>;
  listApiTokens: () => Promise<RelayApiToken[]>;
  listApiTokenUsage: (tokenId: string) => Promise<RelayApiTokenUsageItem[]>;
  revokeApiToken: (tokenId: string) => Promise<{ status: string }>;
};

export function createConsoleApi(client: HttpClient): ConsoleApi {
  return {
    createApiToken: (request) => client.post<CreatedRelayApiToken>('/api/v1/console/api-tokens', request),
    createBillingCheckout: (request) => client.post<BillingCheckoutSession>('/api/v1/billing/checkout', request),
    getAccess: () => client.get<AccessSummary>('/api/v1/console/access'),
    getBilling: () => client.get<ConsoleBillingSummary>('/api/v1/console/billing'),
    getModels: () => client.get<ModelSummary[]>('/api/v1/console/models'),
    getUsage: () => client.get<UsageSummary>('/api/v1/console/usage'),
    listPackages: () => client.get<PackageOption[]>('/api/v1/app/packages'),
    listInvoices: () => client.get<ConsoleBillingInvoiceSummary[]>('/api/v1/console/invoices'),
    listApiTokens: () => client.get<RelayApiToken[]>('/api/v1/console/api-tokens'),
    listApiTokenUsage: (tokenId) => client.get<RelayApiTokenUsageItem[]>(`/api/v1/console/api-tokens/${tokenId}/usage`),
    revokeApiToken: (tokenId) => client.delete<{ status: string }>(`/api/v1/console/api-tokens/${tokenId}`)
  };
}
