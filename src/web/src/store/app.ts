export type AppState = {
  isBusy: boolean;
};

export type AppStore = {
  getState: () => AppState;
  setBusy: (isBusy: boolean) => void;
};

export function createAppStore(initialState: AppState = { isBusy: false }): AppStore {
  let state = initialState;

  return {
    getState: () => state,
    setBusy: (isBusy) => {
      state = { ...state, isBusy };
    }
  };
}
