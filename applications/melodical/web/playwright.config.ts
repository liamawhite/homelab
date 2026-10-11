import { defineConfig, devices } from '@playwright/test';
export default defineConfig({
  testDir: 'e2e',
  forbidOnly: !!process.env.CI,
  fullyParallel: false,
  workers: 1,
  timeout: 30000,
  use: {
    baseURL: 'http://127.0.0.1:8091',
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
  projects: [
    { name: 'desktop', use: { ...devices['Desktop Chrome'] } },
    {
      name: 'tablet',
      use: { ...devices['iPad Mini'], defaultBrowserType: 'chromium' },
    },
    {
      name: 'phone',
      use: { ...devices['iPhone 13'], defaultBrowserType: 'chromium' },
    },
  ],
  webServer: {
    command: '../bin/melodical',
    url: 'http://127.0.0.1:8091/api/health',
    env: {
      MELODICAL_ADDR: '127.0.0.1:8091',
      MELODICAL_DB: '../.tmp/browser-tests.db',
    },
    reuseExistingServer: false,
  },
});
