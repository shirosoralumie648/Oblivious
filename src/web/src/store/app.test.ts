import { describe, expect, it } from 'vitest';
import { createAppStore } from './app';

describe('app store', () => {
  it('initializes with default state (isBusy: false)', () => {
    const store = createAppStore();
    expect(store.getState()).toEqual({ isBusy: false });
  });

  it('initializes with provided state', () => {
    const store = createAppStore({ isBusy: true });
    expect(store.getState()).toEqual({ isBusy: true });
  });

  it('updates state when setBusy is called', () => {
    const store = createAppStore();

    store.setBusy(true);
    expect(store.getState()).toEqual({ isBusy: true });

    store.setBusy(false);
    expect(store.getState()).toEqual({ isBusy: false });
  });
});
