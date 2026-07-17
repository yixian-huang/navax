import { defineConfig, devices } from '@playwright/test';

// E2E 测试针对内嵌前端的真实 navax 二进制运行（先执行 make build）。
// server.mjs 会以全新临时数据目录启动服务，保证每次运行从未初始化状态开始。
const PORT = process.env.NAVAX_E2E_PORT || '18173';
const BASE_URL = `http://127.0.0.1:${PORT}`;

export default defineConfig({
  testDir: './specs',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 30_000,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: BASE_URL,
    locale: 'zh-CN',
    trace: 'retain-on-failure',
  },
  webServer: {
    command: 'node server.mjs',
    url: `${BASE_URL}/readyz`,
    reuseExistingServer: false,
    timeout: 30_000,
    // navax 的 slog JSON（含启动失败原因）走 stdout，默认会被丢弃。
    stdout: 'pipe',
  },
  projects: [
    { name: 'setup', testMatch: /global\.setup\.ts/ },
    {
      name: 'chromium',
      testMatch: /.*\.spec\.ts/,
      use: { ...devices['Desktop Chrome'] },
      dependencies: ['setup'],
    },
  ],
});
