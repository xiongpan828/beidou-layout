import { defineConfig, devices } from '@playwright/test';

const withTrailingSlash = (value: string) => (value.endsWith('/') ? value : `${value}/`);

// 默认本地 dev（需先启动 pnpm dev:admin 与后端）；远程用 ADMIN_WEB_URL 覆盖。
const adminBase = withTrailingSlash(process.env.ADMIN_WEB_URL || 'http://127.0.0.1:3000');

export default defineConfig({
  testDir: __dirname,
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  timeout: 30_000,
  expect: { timeout: 10_000 },
  reporter: [['list'], ['html', { open: 'never', outputFolder: 'playwright-report' }]],
  use: {
    ...devices['Desktop Chrome'],
    baseURL: adminBase,
    headless: true,
    ignoreHTTPSErrors: true,
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
});
