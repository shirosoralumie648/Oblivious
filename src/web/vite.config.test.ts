// @vitest-environment node

import { describe, expect, it } from 'vitest';

import config from './vite.config';

describe('vite API proxy', () => {
  it('forwards app and relay API requests in dev and preview', () => {
    expect(config).toMatchObject({
      preview: {
        proxy: {
          '/api': expect.objectContaining({ target: 'http://127.0.0.1:8080' }),
          '/v1': expect.objectContaining({ target: 'http://127.0.0.1:8080' }),
        },
      },
      server: {
        proxy: {
          '/api': expect.objectContaining({ target: 'http://127.0.0.1:8080' }),
          '/v1': expect.objectContaining({ target: 'http://127.0.0.1:8080' }),
        },
      },
    });
  });
});
