import type { OperationContractMetadataV1 } from '@/generated/operation-contracts.generated';
import { createHttpClient, validateOperationTransportContract, type OperationTransportContract } from '@/services/http/client';
import type { SWRConfiguration } from 'swr';

const client = createHttpClient();

export type SWRTransportKey<T> = readonly [
  url: string,
  operation: OperationContractMetadataV1,
  contract: OperationTransportContract<T>
];

export const fetcher = <T>([url, operation, contract]: SWRTransportKey<T>) => {
  validateOperationTransportContract(url, 'GET', operation, contract);
  return client.get<T>(url, undefined, contract);
};

export const swrConfig: SWRConfiguration = {
  fetcher,
  revalidateOnFocus: false,
  dedupingInterval: 2000
};
