import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: 'e2e',
  timeout: 60_000,
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  use: {
    baseURL: 'http://127.0.0.1:4173',
    headless: true,
    channel: 'chromium',
    locale: 'zh-CN',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  // Serve the production build: the spec-32 E2E seam asserts behavior against
  // built output, and /api is stubbed at the network layer (e2e/fixtures.ts),
  // so neither the dev proxy nor a real apiserver is involved.
  webServer: {
    command: 'pnpm run build && pnpm run preview --host 127.0.0.1 --port 4173 --strictPort',
    url: 'http://127.0.0.1:4173/login',
    reuseExistingServer: !process.env.CI,
    timeout: 180_000,
  },
});
