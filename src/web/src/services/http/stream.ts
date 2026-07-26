import type { OperationContractMetadataV1 } from '@/generated/operation-contracts.generated';

import {
  createHttpClient,
  validateOperationTransportContract,
  type OperationTransportContract
} from './client';

export async function streamText(
  path: string,
  onChunk: (chunk: string) => void,
  operation: OperationContractMetadataV1,
  contract: OperationTransportContract<Response>,
  fetchFn: typeof fetch = fetch,
  init: RequestInit = {}
): Promise<void> {
  validateOperationTransportContract(path, init.method ?? 'GET', operation, contract);
  const response = await createHttpClient({ fetchFn }).request<Response>(path, init, contract);

  if (!response.body) {
    throw new Error('Unable to open stream');
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  while (true) {
    const { done, value } = await reader.read();

    if (done) {
      break;
    }

    buffer += decoder.decode(value, { stream: true });
    buffer = emitBufferedChunks(buffer, onChunk);
  }

  buffer += decoder.decode();
  emitBufferedChunks(buffer, onChunk, true);
}

function emitBufferedChunks(buffer: string, onChunk: (chunk: string) => void, flush = false) {
  let remaining = buffer;

  while (true) {
    // Locate the next blank-line frame boundary.  SSE streams use either LF
    // (\n\n, 2-byte delimiter) or CRLF (\r\n\r\n, 4-byte delimiter).  Neither
    // sequence is a substring of the other, so both can be searched independently
    // and the earlier one consumed with its exact length.
    const crlfBoundary = remaining.indexOf('\r\n\r\n');
    const lfBoundary = remaining.indexOf('\n\n');

    let boundary: number;
    let delimLen: number;

    if (crlfBoundary === -1 && lfBoundary === -1) {
      break;
    } else if (crlfBoundary !== -1 && (lfBoundary === -1 || crlfBoundary < lfBoundary)) {
      boundary = crlfBoundary;
      delimLen = 4;
    } else {
      boundary = lfBoundary;
      delimLen = 2;
    }

    const frame = remaining.slice(0, boundary);
    remaining = remaining.slice(boundary + delimLen);
    emitFrame(frame, onChunk);
  }

  if (flush && remaining.trim() !== '') {
    emitFrame(remaining, onChunk);
    return '';
  }

  return remaining;
}

function emitFrame(frame: string, onChunk: (chunk: string) => void) {
  const lines = frame.split(/\r?\n/);
  const eventType = lines
    .find((line) => line.startsWith('event:'))
    ?.slice(6)
    .trim();
  const dataLines = lines
    .filter((line) => line.startsWith('data:'))
    .map((line) => {
      const value = line.slice(5);
      return value.startsWith(' ') ? value.slice(1) : value;
    });

  if (dataLines.length === 0) {
    if (frame.trim() !== '') {
      onChunk(frame);
    }
    return;
  }

  const data = dataLines.join('\n');
  if (data === '[DONE]') {
    return;
  }
  const normalizedData = data.replace(/\\n/g, '\n');
  if (eventType === 'error') {
    throw new Error(normalizedData || 'Stream returned an error');
  }
  onChunk(normalizedData);
}
