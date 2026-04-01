import { describe, expect, it } from 'vitest';

import { createAuthStore } from './store';

describe('auth store', () => {
  it('starts idle and stores authenticated user', () => {
    const store = createAuthStore();

    expect(store.getState().status).toBe('idle');
    expect(store.getState().user).toBeNull();

    store.setAuthenticatedUser({ id: 'u1', email: 'user@example.com' });

    expect(store.getState().status).toBe('authenticated');
    expect(store.getState().user).toEqual({
      id: 'u1',
      email: 'user@example.com'
    });
  });
});
