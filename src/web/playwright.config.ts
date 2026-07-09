import { defineConfig, devices } from '@playwright/test';

const chromiumExecutablePath = process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH;
const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? 'http://127.0.0.1:4173';
const previewURL = new URL(baseURL);
const previewPort = previewURL.port || (previewURL.protocol === 'https:' ? '443' : '80');
const useManagedWebServer = process.env.PLAYWRIGHT_SKIP_WEB_SERVER !== 'true';
const packageRunner = process.env.PLAYWRIGHT_WEB_SERVER_PACKAGE_RUNNER ?? 'corepack pnpm';
const testTimeout = Number.parseInt(process.env.PLAYWRIGHT_TEST_TIMEOUT_MS ?? '60000', 10);
const webServerCommand =
  process.env.PLAYWRIGHT_WEB_SERVER_COMMAND ??
  `${packageRunner} build && ${packageRunner} preview --host ${previewURL.hostname} --port ${previewPort}`;

const webServer = useManagedWebServer
  ? {
      command: webServerCommand,
      url: baseURL,
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
    }
  : undefined;

export default defineConfig({
  testDir: './e2e',
  timeout: Number.isFinite(testTimeout) ? testTimeout : 60_000,
  fullyParallel: false,
  reporter: process.env.CI ? [['github'], ['list']] : 'list',
  use: {
    baseURL,
    reducedMotion: 'reduce',
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
