import { defineConfig, devices } from '@playwright/test';

const chromiumExecutablePath = process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH;
const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? 'http://127.0.0.1:4173';
const previewURL = new URL(baseURL);
const previewPort = previewURL.port || (previewURL.protocol === 'https:' ? '443' : '80');
const useManagedWebServer = process.env.PLAYWRIGHT_SKIP_WEB_SERVER !== 'true';

const webServer = useManagedWebServer
  ? {
      command: `pnpm build && pnpm preview --host ${previewURL.hostname} --port ${previewPort}`,
      url: baseURL,
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
    }
  : undefined;

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  reporter: process.env.CI ? [['github'], ['list']] : 'list',
  use: {
    baseURL,
    trace: 'retain-on-failure',
  },
  ...(webServer ? { webServer } : {}),
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        launchOptions: chromiumExecutablePath ? { executablePath: chromiumExecutablePath } : undefined,
      },
    },
  ],
});
