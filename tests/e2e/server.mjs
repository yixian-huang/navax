// 以全新临时数据目录启动 navax 二进制，供 Playwright webServer 托管生命周期。
import { spawn } from 'node:child_process';
import { existsSync, mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const port = process.env.NAVAX_E2E_PORT || '18173';
const binary = process.env.NAVAX_BINARY || resolve(import.meta.dirname, '../../bin/navax');

if (!existsSync(binary)) {
  console.error(`未找到 navax 二进制：${binary}\n请先在仓库根目录运行 make build（或通过 NAVAX_BINARY 指定路径）。`);
  process.exit(1);
}

const dataDir = mkdtempSync(join(tmpdir(), 'navax-e2e-'));

const child = spawn(binary, [], {
  stdio: 'inherit',
  env: {
    ...process.env,
    NAVAX_ADDR: `127.0.0.1:${port}`,
    NAVAX_DATA_DIR: dataDir,
    NAVAX_SETUP_TOKEN: 'e2e-suite-setup-token-0123456789abcdef00',
    PUBLIC_BASE_URL: `http://127.0.0.1:${port}`,
    INSTANCE_NAME: 'nav.ax',
  },
});

child.on('exit', code => process.exit(code ?? 0));
for (const signal of ['SIGINT', 'SIGTERM']) {
  process.on(signal, () => child.kill(signal));
}
