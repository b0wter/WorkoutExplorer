import { defineConfig } from '@playwright/test';

// WX_BASE is set by run.sh (own server on a free port). Chromium and WebKit; tests that need one engine skip the other.
export default defineConfig({
  testDir: '.',
  testMatch: '**/*.pw.mjs',
  outputDir: '../../../build/ui-test/results',
  globalSetup: './setup.mjs',
  workers: 4,
  retries: 1,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  reporter: [['list']],
  use: { baseURL: process.env.WX_BASE, locale: 'en-US', viewport: { width: 1440, height: 900 } },
  projects: [
    { name: 'chromium', use: { browserName: 'chromium' } },
    { name: 'webkit', use: { browserName: 'webkit' } },
  ],
});
