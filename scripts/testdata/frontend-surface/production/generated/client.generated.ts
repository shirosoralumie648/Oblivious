export type SchemaIdentityV1 = { readonly kind: 'none'; readonly value: null };
export type OperationContractMetadataV1 = {
  readonly method: string;
  readonly normalizedPath: string;
  readonly operationId: string;
  readonly capabilityId: string;
  readonly request: { readonly mediaType: string | null; readonly schemaIdentity: SchemaIdentityV1 };
  readonly successResponses: readonly { readonly status: string; readonly mediaType: string | null; readonly schemaIdentity: SchemaIdentityV1 }[];
};

export const listUsersOperationContract: OperationContractMetadataV1 = {
  method: 'GET', normalizedPath: '/fixture/users', operationId: 'listUsers', capabilityId: 'fixture.users',
  request: { mediaType: null, schemaIdentity: { kind: 'none', value: null } }, successResponses: [{ status: '200', mediaType: 'application/json', schemaIdentity: { kind: 'none', value: null } }]
};
export const uploadOperationContract: OperationContractMetadataV1 = {
  method: 'POST', normalizedPath: '/fixture/upload', operationId: 'uploadFixture', capabilityId: 'fixture.upload',
  request: { mediaType: 'multipart/form-data', schemaIdentity: { kind: 'none', value: null } }, successResponses: [{ status: '200', mediaType: 'application/json', schemaIdentity: { kind: 'none', value: null } }]
};
export const streamOperationContract: OperationContractMetadataV1 = {
  method: 'POST', normalizedPath: '/fixture/stream', operationId: 'streamFixture', capabilityId: 'fixture.stream',
  request: { mediaType: 'application/json', schemaIdentity: { kind: 'none', value: null } }, successResponses: [{ status: '200', mediaType: 'text/event-stream', schemaIdentity: { kind: 'none', value: null } }]
};
export const socketOperationContract: OperationContractMetadataV1 = {
  method: 'GET', normalizedPath: '/fixture/socket', operationId: 'socketFixture', capabilityId: 'fixture.socket',
  request: { mediaType: null, schemaIdentity: { kind: 'none', value: null } }, successResponses: [{ status: '101', mediaType: null, schemaIdentity: { kind: 'none', value: null } }]
};
