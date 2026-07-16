import { defineConfig, mergeConfig } from 'vitest/config';
import viteConfig from './vite.config';

// 复用生产 vite 配置（@ 别名、auto-import、react 插件），
// 只叠加测试运行所需项：jsdom 环境（mock 依赖 window/fetch）与用例目录。
export default mergeConfig(
  viteConfig,
  defineConfig({
    test: {
      environment: 'jsdom',
      include: ['tests/**/*.test.ts'],
    },
  }),
);
