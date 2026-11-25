import { defineConfig, devices } from '@playwright/test';

// WX_BASE is set by run.sh (own server on a free port). Projects: three desktop engines, two emulated phones,
// a small tablet and Firefox at phone width (Firefox has no isMobile or touch emulation, viewport only).
// A layout-specific test restricts itself with test.skip(({ isMobile }) => ...) or by project name.
const desktop = { viewport: { width: 1440, height: 900 } };

export default defineConfig({
  testDir: '.',
  testMatch: '**/*.pw.mjs',
  outputDir: '../../../build/ui-test/results',
  globalSetup: './setup.mjs',
  workers: 4,
  retries: 1,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  // html and json feed report.sh (unattended run); traces and screenshots only for failures.
  reporter: [['list'], ['json', { outputFile: '../../../build/ui-test/report.json' }], ['html', { outputFolder: '../../../build/ui-test/html', open: 'never' }]],
  use: { baseURL: process.env.WX_BASE, locale: 'en-US', trace: 'retain-on-failure', screenshot: 'only-on-failure' },
  projects: [
    { name: 'chromium', use: { browserName: 'chromium', ...desktop } },
    { name: 'webkit', use: { browserName: 'webkit', ...desktop } },
    { name: 'firefox', use: { browserName: 'firefox', ...desktop } },
    { name: 'iPhone', use: { ...devices['iPhone 13'], locale: 'en-US' } },
    { name: 'Android', use: { ...devices['Pixel 7'], locale: 'en-US' } },
    { name: 'iPad', use: { ...devices['iPad Mini'], locale: 'en-US' } },
    { name: 'firefox-phone', use: { browserName: 'firefox', viewport: { width: 390, height: 844 } } },
  ],
});
