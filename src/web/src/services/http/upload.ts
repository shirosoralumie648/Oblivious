import type { OperationContractMetadataV1 } from '@/generated/operation-contracts.generated';

import {
  createHttpClient,
  validateOperationTransportContract,
  type OperationTransportContract
} from './client';

export async function uploadFile(
  path: string,
  file: File,
  operation: OperationContractMetadataV1,
  contract: OperationTransportContract<Response>,
  fieldName = 'file',
  fetchFn: typeof fetch = fetch
): Promise<Response> {
  validateOperationTransportContract(path, 'POST', operation, contract);
  const formData = new FormData();
  formData.append(fieldName, file);

  return createHttpClient({ fetchFn }).request<Response>(
    path,
    {
      body: formData,
      method: 'POST'
    },
    contract
  );
}
