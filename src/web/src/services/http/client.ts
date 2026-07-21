import type {
  OperationContractMetadataV1,
  SchemaIdentityV1
} from '@/generated/operation-contracts.generated';

import { HttpError } from './errors';
import { unwrapEnvelope } from './envelope';

export type RequestEncoder = {
  readonly id: 'none' | 'json' | 'form-data' | 'raw';
  readonly mediaType: string | null;
  readonly schemaIdentity: SchemaIdentityV1;
};

export type ResponseDecoder<T> = {
  readonly id: 'json-envelope' | 'text' | 'raw-response' | 'none';
  readonly status: number;
  readonly mediaType: string | null;
  readonly schemaIdentity: SchemaIdentityV1;
  readonly decode: (response: Response) => Promise<T>;
};

export type OperationTransportContract<T> = {
  readonly operation: OperationContractMetadataV1;
  readonly requestEncoder: RequestEncoder;
  readonly responseDecoder: ResponseDecoder<T>;
};

export type HttpClient = {
  request: <T>(path: string, init?: RequestInit, contract?: OperationTransportContract<T>) => Promise<T>;
  get: <T>(path: string, init?: RequestInit, contract?: OperationTransportContract<T>) => Promise<T>;
  post: <T>(path: string, body?: unknown, init?: RequestInit, contract?: OperationTransportContract<T>) => Promise<T>;
  put: <T>(path: string, body?: unknown, init?: RequestInit, contract?: OperationTransportContract<T>) => Promise<T>;
  delete: <T>(path: string, init?: RequestInit, contract?: OperationTransportContract<T>) => Promise<T>;
};

export type HttpClientOptions = {
  baseUrl?: string;
  fetchFn?: typeof fetch;
};

type ParsedMediaType = {
  base: string;
  parameters: ReadonlyMap<string, string>;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function parseMediaType(value: string): ParsedMediaType | null {
  const [rawBase, ...rawParameters] = value.split(';');
  const base = rawBase?.trim().toLowerCase();
  if (!base || !base.includes('/')) {
    return null;
  }

  const parameters = new Map<string, string>();
  for (const rawParameter of rawParameters) {
    const separator = rawParameter.indexOf('=');
    if (separator <= 0) {
      return null;
    }
    const name = rawParameter.slice(0, separator).trim().toLowerCase();
    const parameterValue = rawParameter.slice(separator + 1).trim().replace(/^"|"$/g, '');
    if (!name || !parameterValue || parameters.has(name)) {
      return null;
    }
    parameters.set(name, parameterValue);
  }
  return { base, parameters };
}

function isJSONMediaType(value: string): boolean {
  const parsed = parseMediaType(value);
  return parsed !== null && (parsed.base === 'application/json' || parsed.base.endsWith('+json'));
}

function mediaTypeMatches(expected: string | null, actual: string | null): boolean {
  if (expected === null) {
    return actual === null || actual.trim() === '';
  }
  if (actual === null || actual.trim() === '') {
    return false;
  }
  const expectedMedia = parseMediaType(expected);
  const actualMedia = parseMediaType(actual);
  if (!expectedMedia || !actualMedia || expectedMedia.base !== actualMedia.base) {
    return false;
  }
  return [...actualMedia.parameters.keys()].every((name) => name === 'charset');
}

function selectedSuccess(
  operation: OperationContractMetadataV1,
  status: number
): OperationContractMetadataV1['successResponses'][number] {
  const matches = operation.successResponses.filter((response) => response.status === String(status));
  if (matches.length !== 1) {
    throw new TypeError(`Operation ${operation.operationId} does not declare one success response for status ${status}.`);
  }
  return matches[0];
}

export function noneRequestEncoder(operation: OperationContractMetadataV1): RequestEncoder {
  return {
    id: 'none',
    mediaType: operation.request.mediaType,
    schemaIdentity: operation.request.schemaIdentity
  };
}

export function jsonRequestEncoder(operation: OperationContractMetadataV1): RequestEncoder {
  return {
    id: 'json',
    mediaType: operation.request.mediaType,
    schemaIdentity: operation.request.schemaIdentity
  };
}

export function formDataRequestEncoder(operation: OperationContractMetadataV1): RequestEncoder {
  return {
    id: 'form-data',
    mediaType: operation.request.mediaType,
    schemaIdentity: operation.request.schemaIdentity
  };
}

export function rawRequestEncoder(operation: OperationContractMetadataV1): RequestEncoder {
  return {
    id: 'raw',
    mediaType: operation.request.mediaType,
    schemaIdentity: operation.request.schemaIdentity
  };
}

export function jsonEnvelopeDecoder<T>(operation: OperationContractMetadataV1, status: number): ResponseDecoder<T> {
  const success = selectedSuccess(operation, status);
  return {
    id: 'json-envelope',
    status,
    mediaType: success.mediaType,
    schemaIdentity: success.schemaIdentity,
    decode: async (response) => unwrapEnvelope<T>(await response.json())
  };
}

export function textResponseDecoder(operation: OperationContractMetadataV1, status: number): ResponseDecoder<string> {
  const success = selectedSuccess(operation, status);
  return {
    id: 'text',
    status,
    mediaType: success.mediaType,
    schemaIdentity: success.schemaIdentity,
    decode: (response) => response.text()
  };
}

export function noneResponseDecoder(operation: OperationContractMetadataV1, status: number): ResponseDecoder<void> {
  const success = selectedSuccess(operation, status);
  return {
    id: 'none',
    status,
    mediaType: success.mediaType,
    schemaIdentity: success.schemaIdentity,
    decode: async () => undefined
  };
}

export function rawResponseDecoder(operation: OperationContractMetadataV1, status: number): ResponseDecoder<Response> {
  const success = selectedSuccess(operation, status);
  return {
    id: 'raw-response',
    status,
    mediaType: success.mediaType,
    schemaIdentity: success.schemaIdentity,
    decode: async (response) => response
  };
}

function normalizedPathMatches(path: string, normalizedPath: string): boolean {
  let pathname: string;
  try {
    pathname = new URL(path, 'http://oblivious.invalid').pathname;
  } catch {
    return false;
  }
  const actualSegments = pathname.split('/').filter(Boolean);
  const expectedSegments = normalizedPath.split('/').filter(Boolean);
  return expectedSegments.length === actualSegments.length && expectedSegments.every((segment, index) => {
    if (segment.startsWith('{') && segment.endsWith('}')) {
      return actualSegments[index].length > 0;
    }
    return segment === actualSegments[index];
  });
}

function validateRequestEncoder(operation: OperationContractMetadataV1, encoder: RequestEncoder): void {
  if (encoder.schemaIdentity !== operation.request.schemaIdentity || encoder.mediaType !== operation.request.mediaType) {
    throw new TypeError(`Request encoder identity does not match operation ${operation.operationId}.`);
  }
  if (encoder.id === 'none' && encoder.mediaType !== null) {
    throw new TypeError('The none request encoder requires a null media type.');
  }
  if (encoder.id === 'json' && (encoder.mediaType === null || !isJSONMediaType(encoder.mediaType))) {
    throw new TypeError('The JSON request encoder requires a JSON-compatible media type.');
  }
  if (encoder.id === 'form-data' && parseMediaType(encoder.mediaType ?? '')?.base !== 'multipart/form-data') {
    throw new TypeError('The form-data request encoder requires multipart/form-data.');
  }
  if (encoder.id === 'raw' && encoder.mediaType === null) {
    throw new TypeError('The raw request encoder requires a declared media type.');
  }
}

function validateResponseDecoder<T>(operation: OperationContractMetadataV1, decoder: ResponseDecoder<T>): void {
  const success = selectedSuccess(operation, decoder.status);
  if (
    typeof decoder.decode !== 'function'
    || decoder.schemaIdentity !== success.schemaIdentity
    || decoder.mediaType !== success.mediaType
  ) {
    throw new TypeError(`Response decoder identity does not match operation ${operation.operationId}.`);
  }
  if (decoder.id === 'json-envelope' && (decoder.mediaType === null || !isJSONMediaType(decoder.mediaType))) {
    throw new TypeError('The JSON response decoder requires a JSON-compatible media type.');
  }
  if (decoder.id === 'text' && !parseMediaType(decoder.mediaType ?? '')?.base.startsWith('text/')) {
    throw new TypeError('The text response decoder requires a text media type.');
  }
  if (decoder.id === 'none' && (decoder.status !== 204 || decoder.mediaType !== null)) {
    throw new TypeError('The none response decoder requires status 204 with no media type.');
  }
  if (decoder.id === 'raw-response' && decoder.mediaType === null) {
    throw new TypeError('The raw response decoder requires a declared media type.');
  }
}

function validateTransportContract<T>(
  path: string,
  method: string,
  contract: OperationTransportContract<T>
): void {
  if (contract.operation.method !== method.toUpperCase()) {
    throw new TypeError(`Operation ${contract.operation.operationId} does not match request method ${method}.`);
  }
  if (!normalizedPathMatches(path, contract.operation.normalizedPath)) {
    throw new TypeError(`Operation ${contract.operation.operationId} does not match request path.`);
  }
  validateRequestEncoder(contract.operation, contract.requestEncoder);
  validateResponseDecoder(contract.operation, contract.responseDecoder);
}

function requestHeaders(
  init: RequestInit,
  accept: string | null,
  encoder: RequestEncoder | null,
  hasBody: boolean
): Record<string, string> {
  const headers = new Headers(init.headers);
  const callerAccept = headers.get('accept');
  if (accept === null && callerAccept !== null) {
    throw new TypeError('Caller Accept header contradicts a response with no declared media type.');
  }
  if (accept !== null && callerAccept !== null) {
    const accepted = callerAccept.split(',').some((value) => parseMediaType(value.trim())?.base === parseMediaType(accept)?.base);
    if (!accepted) {
      throw new TypeError(`Caller Accept header contradicts declared response media type ${accept}.`);
    }
  }
  if (accept !== null) {
    headers.set('Accept', accept);
  }

  if (hasBody && encoder !== null && encoder.id !== 'form-data') {
    const declaredContentType = encoder.mediaType ?? 'application/json';
    const callerContentType = headers.get('content-type');
    if (callerContentType !== null && !mediaTypeMatches(declaredContentType, callerContentType)) {
      throw new TypeError(`Caller Content-Type header contradicts declared request media type ${declaredContentType}.`);
    }
    headers.set('Content-Type', declaredContentType);
  }

  const result: Record<string, string> = {};
  headers.forEach((value, name) => {
    const key = name === 'accept' ? 'Accept' : name === 'content-type' ? 'Content-Type' : name;
    result[key] = value;
  });
  return result;
}

function encodeBody(body: unknown, encoder: RequestEncoder | null): BodyInit | null | undefined {
  if (body === undefined) {
    return undefined;
  }
  if (encoder === null && typeof FormData !== 'undefined' && body instanceof FormData) {
    return body;
  }
  if (encoder?.id === 'none') {
    throw new TypeError('The none request encoder cannot send a body.');
  }
  if (encoder?.id === 'form-data') {
    if (typeof FormData === 'undefined' || !(body instanceof FormData)) {
      throw new TypeError('The form-data request encoder requires FormData.');
    }
    return body;
  }
  if (encoder?.id === 'raw') {
    return body as BodyInit;
  }
  if (encoder?.id === 'json' || encoder === null) {
    return typeof body === 'string' && encoder === null ? body : JSON.stringify(body);
  }
  return body as BodyInit;
}

async function parseErrorResponse(response: Response): Promise<HttpError> {
  let message = response.statusText || 'HTTP request failed';
  let code: string | undefined;
  let data: unknown;
  const contentType = response.headers.get('content-type');

  if (contentType !== null && isJSONMediaType(contentType)) {
    try {
      const payload = await response.json();
      if (isRecord(payload) && 'data' in payload) {
        data = payload.data;
      }
      if (isRecord(payload) && 'error' in payload) {
        const error = payload.error;
        if (isRecord(error) && typeof error.message === 'string') {
          message = error.message;
        }
        if (isRecord(error) && typeof error.code === 'string') {
          code = error.code;
        }
      }
    } catch {
      // Malformed and non-JSON response bodies are never exposed through public errors.
    }
  }
  return new HttpError(response.status, message, { code, data });
}

export function createHttpClient(options: HttpClientOptions = {}): HttpClient {
  const baseUrl = options.baseUrl ?? '';
  const fetchFn = options.fetchFn ?? fetch;

  const dispatch = async <T>(
    path: string,
    init: RequestInit,
    body: unknown,
    contract?: OperationTransportContract<T>
  ): Promise<T> => {
    const method = (init.method ?? 'GET').toUpperCase();
    if (contract) {
      validateTransportContract(path, method, contract);
    }
    const encodedBody = encodeBody(body, contract?.requestEncoder ?? null);
    const accept = contract ? contract.responseDecoder.mediaType : 'application/json';
    const response = await fetchFn(`${baseUrl}${path}`, {
      ...init,
      body: encodedBody,
      headers: requestHeaders(init, accept, contract?.requestEncoder ?? null, encodedBody !== undefined && encodedBody !== null)
    });

    if (!response.ok) {
      throw await parseErrorResponse(response);
    }

    if (contract) {
      if (response.status !== contract.responseDecoder.status) {
        throw new TypeError(`Response status ${response.status} does not match declared status ${contract.responseDecoder.status}.`);
      }
      if (!mediaTypeMatches(contract.responseDecoder.mediaType, response.headers.get('content-type'))) {
        throw new TypeError(`Response media type does not match operation ${contract.operation.operationId}.`);
      }
      return contract.responseDecoder.decode(response);
    }

    if (response.status === 204) {
      return undefined as T;
    }
    if (!isJSONMediaType(response.headers.get('content-type') ?? '')) {
      throw new TypeError('Successful JSON response is missing a JSON-compatible Content-Type.');
    }
    return unwrapEnvelope<T>(await response.json());
  };

  const request = <T>(path: string, init: RequestInit = {}, contract?: OperationTransportContract<T>) =>
    dispatch<T>(path, init, init.body, contract);

  return {
    request,
    get: (path, init = {}, contract) => dispatch(path, { ...init, method: 'GET' }, undefined, contract),
    post: (path, body, init = {}, contract) => dispatch(path, { ...init, method: 'POST' }, body, contract),
    put: (path, body, init = {}, contract) => dispatch(path, { ...init, method: 'PUT' }, body, contract),
    delete: (path, init = {}, contract) => dispatch(path, { ...init, method: 'DELETE' }, undefined, contract)
  };
}
