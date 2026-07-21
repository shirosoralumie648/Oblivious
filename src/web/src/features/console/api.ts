import {
  createBillingCheckoutOperationContract,
  createConsoleAPITokenOperationContract,
  getAccessOperationContract,
  getBillingOperationContract,
  getConsoleModelsOperationContract,
  getUsageOperationContract,
  listConsoleAPITokensOperationContract,
  listConsoleAPITokenUsageOperationContract,
  listConsoleBillingInvoicesOperationContract,
  listPackagesOperationContract,
  revokeConsoleAPITokenOperationContract,
  type OperationContractMetadataV1
} from '@/generated/operation-contracts.generated';
import {
  jsonEnvelopeDecoder,
  jsonRequestEncoder,
  noneRequestEncoder,
  type HttpClient,
  type OperationTransportContract
} from '../../services/http/client';
import type { AccessSummary, BillingPaymentProviderSummary, BillingSummary as BaseBillingSummary, ConsoleApiTokenUsageItem, CreatedRelayApiToken, CreateRelayApiTokenRequest, ModelSummary, PackageOption, RelayApiToken, UsageSummary } from '../../types/api';

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
  listApiTokenUsage: (tokenId: string) => Promise<ConsoleApiTokenUsageItem[]>;
  revokeApiToken: (tokenId: string) => Promise<{ status: string }>;
};

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

const createApiTokenTransport = jsonTransport<CreatedRelayApiToken>(createConsoleAPITokenOperationContract, 201);
const createBillingCheckoutTransport = jsonTransport<BillingCheckoutSession>(createBillingCheckoutOperationContract, 201);
const getAccessTransport = jsonTransport<AccessSummary>(getAccessOperationContract);
const getBillingTransport = jsonTransport<ConsoleBillingSummary>(getBillingOperationContract);
const getModelsTransport = jsonTransport<ModelSummary[]>(getConsoleModelsOperationContract);
const getUsageTransport = jsonTransport<UsageSummary>(getUsageOperationContract);
const listPackagesTransport = jsonTransport<PackageOption[]>(listPackagesOperationContract);
const listInvoicesTransport = jsonTransport<ConsoleBillingInvoiceSummary[]>(listConsoleBillingInvoicesOperationContract);
const listApiTokensTransport = jsonTransport<RelayApiToken[]>(listConsoleAPITokensOperationContract);
const listApiTokenUsageTransport = jsonTransport<ConsoleApiTokenUsageItem[]>(listConsoleAPITokenUsageOperationContract);
const revokeApiTokenTransport = jsonTransport<{ status: string }>(revokeConsoleAPITokenOperationContract);

export function createConsoleApi(client: HttpClient): ConsoleApi {
  return {
    createApiToken: (request) =>
      client.post<CreatedRelayApiToken>('/api/v1/console/api-tokens', request, undefined, createApiTokenTransport),
    createBillingCheckout: (request) =>
      client.post<BillingCheckoutSession>('/api/v1/billing/checkout', request, undefined, createBillingCheckoutTransport),
    getAccess: () => client.get<AccessSummary>('/api/v1/console/access', undefined, getAccessTransport),
    getBilling: () => client.get<ConsoleBillingSummary>('/api/v1/console/billing', undefined, getBillingTransport),
    getModels: () => client.get<ModelSummary[]>('/api/v1/console/models', undefined, getModelsTransport),
    getUsage: () => client.get<UsageSummary>('/api/v1/console/usage', undefined, getUsageTransport),
    listPackages: () => client.get<PackageOption[]>('/api/v1/app/packages', undefined, listPackagesTransport),
    listInvoices: () =>
      client.get<ConsoleBillingInvoiceSummary[]>('/api/v1/console/invoices', undefined, listInvoicesTransport),
    listApiTokens: () =>
      client.get<RelayApiToken[]>('/api/v1/console/api-tokens', undefined, listApiTokensTransport),
    listApiTokenUsage: (tokenId) =>
      client.get<ConsoleApiTokenUsageItem[]>(
        `/api/v1/console/api-tokens/${tokenId}/usage`,
        undefined,
        listApiTokenUsageTransport
      ),
    revokeApiToken: (tokenId) =>
      client.delete<{ status: string }>(
        `/api/v1/console/api-tokens/${tokenId}`,
        undefined,
        revokeApiTokenTransport
      )
  };
}
