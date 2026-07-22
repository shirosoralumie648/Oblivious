import {
  listUsersOperationContract,
  socketOperationContract,
  streamOperationContract,
  uploadOperationContract
} from './generated/client.generated';

type Transport = { operation: unknown; requestEncoder: unknown; responseDecoder: unknown };
type HttpClient = { get: (...args: unknown[]) => Promise<unknown>; post: (...args: unknown[]) => Promise<unknown> };
declare const client: HttpClient;
declare function noneRequestEncoder(operation: unknown): unknown;
declare function jsonRequestEncoder(operation: unknown): unknown;
declare function formDataRequestEncoder(operation: unknown): unknown;
declare function jsonEnvelopeDecoder<T>(operation: unknown, status: number): unknown;
declare function rawResponseDecoder(operation: unknown, status: number): unknown;
declare function streamText(path: string, onChunk: (chunk: string) => void, operation: unknown, contract: Transport): Promise<void>;
declare function uploadFile(path: string, file: File, operation: unknown, contract: Transport): Promise<Response>;
declare function useSWR<T>(key: readonly unknown[], fetcher: unknown): unknown;
declare function fetchFn(path: string, init: RequestInit, contract: Transport): Promise<Response>;
declare class EventSource { constructor(path: string); }
declare class WebSocket { constructor(path: string); }

const listTransport: Transport = {
  operation: listUsersOperationContract,
  requestEncoder: noneRequestEncoder(listUsersOperationContract),
  responseDecoder: jsonEnvelopeDecoder(listUsersOperationContract, 200)
};
const uploadTransport: Transport = {
  operation: uploadOperationContract,
  requestEncoder: formDataRequestEncoder(uploadOperationContract),
  responseDecoder: jsonEnvelopeDecoder(uploadOperationContract, 200)
};
const streamTransport: Transport = {
  operation: streamOperationContract,
  requestEncoder: jsonRequestEncoder(streamOperationContract),
  responseDecoder: rawResponseDecoder(streamOperationContract, 200)
};
const socketTransport: Transport = {
  operation: socketOperationContract,
  requestEncoder: noneRequestEncoder(socketOperationContract),
  responseDecoder: rawResponseDecoder(socketOperationContract, 101)
};

client.get('/fixture/users', undefined, listTransport);
client.get('/fixture/users', undefined, listTransport);
streamText('/fixture/stream', () => undefined, streamOperationContract, streamTransport);
uploadFile('/fixture/upload', new File([], 'fixture.txt'), uploadOperationContract, uploadTransport);
fetchFn('/fixture/users', { method: 'GET' }, listTransport);
useSWR([' /fixture/users', listUsersOperationContract, listTransport], () => undefined);
new EventSource(socketOperationContract.normalizedPath);
new WebSocket(socketOperationContract.normalizedPath);

export const fixtureExposure = { path: '/fixture/users', capabilityId: 'fixture.users' };
export const fixtureSelector = { capabilityId: 'fixture.users' };
