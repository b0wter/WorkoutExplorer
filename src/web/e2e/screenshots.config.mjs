import { defineConfig, devices } from '@playwright/test';

// README screenshots (make screenshots), not part of the suite: only *.shot.mjs, one try, output to docs/screenshots.
export default defineConfig({
  testDir: '.',
  testMatch: '**/*.shot.mjs',
  outputDir: '../../../build/ui-test/results',
  globalSetup: './setup.mjs',
  workers: 1,
  retries: 0,
  timeout: 90_000,
  reporter: [['list']],
  use: { baseURL: process.env.WX_BASE, locale: 'en-US' },
  projects: [
    { name: 'desktop', use: { browserName: 'chromium', viewport: { width: 1440, height: 1400 } } },
    { name: 'phone', use: { ...devices['iPhone 13'], locale: 'en-US', deviceScaleFactor: 2 } },
    { name: 'tablet', use: { browserName: 'chromium', viewport: { width: 820, height: 1180 }, hasTouch: true } },
  ],
});
