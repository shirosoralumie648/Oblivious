import type { ApiEnvelope } from '../../types/api';

import { HttpError } from './errors';

function isApiEnvelope<T>(payload: unknown): payload is ApiEnvelope<T> {
  if (typeof payload !== 'object' || payload === null) {
    return false;
  }

  return 'ok' in payload && 'data' in payload && 'error' in payload;
}

export function unwrapEnvelope<T>(payload: T | ApiEnvelope<T>): T {
  if (!isApiEnvelope<T>(payload)) {
    return payload as T;
  }

  if (!payload.ok) {
    throw new HttpError(500, payload.error?.message ?? 'HTTP request failed');
  }

  if (payload.data === null) {
    return undefined as T;
  }

  return payload.data;
}
