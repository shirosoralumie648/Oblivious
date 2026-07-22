export type SchemaIdentityV1 = { readonly kind: 'none'; readonly value: null };
export type OperationContractMetadataV1 = {
  readonly method: string;
  readonly normalizedPath: string;
  readonly operationId: string;
  readonly capabilityId: string;
  readonly request: { readonly mediaType: string | null; readonly schemaIdentity: SchemaIdentityV1 };
  readonly successResponses: readonly { readonly status: string; readonly mediaType: string | null; readonly schemaIdentity: SchemaIdentityV1 }[];
};

export type BrowserEventIdentityV1 = {
  readonly direction: 'client' | 'server';
  readonly kind: 'message' | 'event';
  readonly schemaIdentity: SchemaIdentityV1;
};

export const listUsersOperationContract: OperationContractMetadataV1 = {
  method: 'GET', normalizedPath: '/fixture/users', operationId: 'listUsers', capabilityId: 'fixture.users',
  request: { mediaType: null, schemaIdentity: { kind: 'none', value: null } }, successResponses: [{ status: '200', mediaType: 'application/json', schemaIdentity: { kind: 'none', value: null } }]
};
export const getAppReadinessCapabilitiesOperationContract: OperationContractMetadataV1 = {
  method: 'GET', normalizedPath: '/fixture/app-projection', operationId: 'getAppReadinessCapabilities', capabilityId: 'fixture.users',
  request: { mediaType: null, schemaIdentity: { kind: 'none', value: null } }, successResponses: [{ status: '200', mediaType: 'application/json', schemaIdentity: { kind: 'none', value: null } }]
};
export const uploadOperationContract: OperationContractMetadataV1 = {
  method: 'POST', normalizedPath: '/fixture/upload', operationId: 'uploadFixture', capabilityId: 'fixture.upload',
  request: { mediaType: 'multipart/form-data', schemaIdentity: { kind: 'none', value: null } }, successResponses: [{ status: '200', mediaType: 'application/json', schemaIdentity: { kind: 'none', value: null } }]
};
export const rawOperationContract: OperationContractMetadataV1 = {
  method: 'POST', normalizedPath: '/fixture/raw', operationId: 'rawFixture', capabilityId: 'fixture.raw',
  request: { mediaType: 'application/octet-stream', schemaIdentity: { kind: 'none', value: null } }, successResponses: [{ status: '200', mediaType: 'application/octet-stream', schemaIdentity: { kind: 'none', value: null } }]
};
export const textOperationContract: OperationContractMetadataV1 = {
  method: 'GET', normalizedPath: '/fixture/text', operationId: 'textFixture', capabilityId: 'fixture.text',
  request: { mediaType: null, schemaIdentity: { kind: 'none', value: null } }, successResponses: [{ status: '200', mediaType: 'text/plain', schemaIdentity: { kind: 'none', value: null } }]
};
export const noContentOperationContract: OperationContractMetadataV1 = {
  method: 'DELETE', normalizedPath: '/fixture/items/{itemId}', operationId: 'deleteFixture', capabilityId: 'fixture.delete',
  request: { mediaType: null, schemaIdentity: { kind: 'none', value: null } }, successResponses: [{ status: '204', mediaType: null, schemaIdentity: { kind: 'none', value: null } }]
};
export const streamOperationContract: OperationContractMetadataV1 = {
  method: 'POST', normalizedPath: '/fixture/stream', operationId: 'streamFixture', capabilityId: 'fixture.stream',
  request: { mediaType: 'application/json', schemaIdentity: { kind: 'none', value: null } }, successResponses: [{ status: '200', mediaType: 'text/event-stream', schemaIdentity: { kind: 'none', value: null } }]
};
export const eventSourceOperationContract: OperationContractMetadataV1 = {
  method: 'GET', normalizedPath: '/fixture/events', operationId: 'eventSourceFixture', capabilityId: 'fixture.events',
  request: { mediaType: null, schemaIdentity: { kind: 'none', value: null } }, successResponses: [{ status: '200', mediaType: 'text/event-stream', schemaIdentity: { kind: 'none', value: null } }]
};
export const socketOperationContract: OperationContractMetadataV1 = {
  method: 'GET', normalizedPath: '/fixture/socket', operationId: 'socketFixture', capabilityId: 'fixture.socket',
  request: { mediaType: null, schemaIdentity: { kind: 'none', value: null } }, successResponses: [{ status: '101', mediaType: null, schemaIdentity: { kind: 'none', value: null } }]
};

export const browserEventContracts = [
  {
    operationId: 'eventSourceFixture',
    transport: 'sse',
    events: [{ direction: 'server', kind: 'event', schemaIdentity: { kind: 'none', value: null } }]
  },
  {
    operationId: 'socketFixture',
    transport: 'websocket',
    events: [
      { direction: 'client', kind: 'message', schemaIdentity: { kind: 'none', value: null } },
      { direction: 'server', kind: 'message', schemaIdentity: { kind: 'none', value: null } }
    ]
  },
  {
    operationId: 'streamFixture',
    transport: 'sse',
    events: [{ direction: 'server', kind: 'event', schemaIdentity: { kind: 'none', value: null } }]
  }
] as const satisfies readonly {
  readonly operationId: string;
  readonly transport: 'sse' | 'websocket';
  readonly events: readonly BrowserEventIdentityV1[];
}[];
