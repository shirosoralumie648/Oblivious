declare const fetch: (path: string) => Promise<unknown>;
fetch('/excluded/test-only-call');
export const excluded = true;
