// ============================================================
// Mock ↔ OpenAPI 契约守卫
// ------------------------------------------------------------
// 开发态 mock（VITE_ENABLE_API_MOCKS=true）把内部状态投影为契约响应，
// 供前端 normalize* 消费。若投影与 api/openapi.yaml 漂移，dev 下会静默出错。
// 本测试装载 mock，请求前端真实调用的契约端点，并用 openapi 响应 schema 校验，
// 从而在 CI 中拦截 mock/契约漂移。真实后端 ↔ openapi 的一致性由 Go 契约测试保证。
// ============================================================

import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { beforeAll, afterAll, describe, expect, it } from 'vitest';
import { parse as parseYaml } from 'yaml';
import Ajv2020, { type ValidateFunction } from 'ajv/dist/2020';
import addFormats from 'ajv-formats';
import { installMockApi, uninstallMockApi } from '@/api/mock-handlers';

// vitest 以 web/ 为工作目录，契约位于仓库根的 api/openapi.yaml。
const specPath = resolve(process.cwd(), '../api/openapi.yaml');
const spec = parseYaml(readFileSync(specPath, 'utf8')) as {
  paths: Record<string, Record<string, { responses: Record<string, { $ref?: string }> }>>;
  components: { schemas: Record<string, { minLength?: number; maxLength?: number }> };
};

// mock 使用可读的占位 ID（如 usr_001、cat_001）；不透明 ID 的 8–64 位长度是后端
// 生成约束，与前端消费的契约「形状」无关。守卫聚焦形状（字段/类型/枚举/嵌套），
// 故此处放宽 Id 的长度约束，其余（type、format、enum、required）保持严格。
delete spec.components.schemas.Id.minLength;
delete spec.components.schemas.Id.maxLength;

const ajv = new Ajv2020({ strict: false, validateSchema: false, allErrors: true });
addFormats(ajv);
ajv.addSchema(spec, 'openapi');

// JSON Pointer 段转义：/ → ~1，~ → ~0。
const jp = (segment: string) => segment.replace(/~/g, '~0').replace(/\//g, '~1');

// 定位某端点某状态码 application/json 响应体的 schema，返回编译后的校验器。
// 响应通常经 components/responses 间接引用，需先解引用再指向其 schema 节点。
function responseValidator(path: string, method: string, status: string): ValidateFunction {
  const response = spec.paths[path]?.[method]?.responses?.[status];
  if (!response) throw new Error(`openapi 缺少 ${method.toUpperCase()} ${path} 的 ${status} 响应`);
  const pointer = response.$ref
    ? `openapi#/components/responses/${jp(response.$ref.split('/').pop()!)}/content/${jp('application/json')}/schema`
    : `openapi#/paths/${jp(path)}/${method}/responses/${status}/content/${jp('application/json')}/schema`;
  const validate = ajv.getSchema(pointer);
  if (!validate) throw new Error(`无法编译 schema：${pointer}`);
  return validate as ValidateFunction;
}

// 前端真实调用的契约端点 → 期望响应。覆盖含投影逻辑的读端点与资源上传。
type Case = { name: string; path: string; method: string; status: string; url: string; init?: RequestInit };
const cases: Case[] = [
  { name: '当前草稿页', path: '/api/v1/pages/current', method: 'get', status: '200', url: '/api/v1/pages/current?scope=personal' },
  { name: '公开首页快照', path: '/api/v1/public/home', method: 'get', status: '200', url: '/api/v1/public/home' },
  { name: '公开实例配置', path: '/api/v1/public/config', method: 'get', status: '200', url: '/api/v1/public/config' },
  { name: '主题列表', path: '/api/v1/themes', method: 'get', status: '200', url: '/api/v1/themes' },
  { name: '公共目录', path: '/api/v1/public/directory', method: 'get', status: '200', url: '/api/v1/public/directory' },
  { name: '发现页', path: '/api/v1/public/discover', method: 'get', status: '200', url: '/api/v1/public/discover' },
  { name: '子域名（无申请为 null）', path: '/api/v1/me/subdomain', method: 'get', status: '200', url: '/api/v1/me/subdomain' },
  { name: '资源上传', path: '/api/v1/assets', method: 'post', status: '201', url: '/api/v1/assets', init: { method: 'POST' } },
];

beforeAll(() => {
  // 保证非 /admin 路径，mock 以普通用户身份投影个人页。
  window.history.pushState({}, '', '/');
  installMockApi();
});

afterAll(() => uninstallMockApi());

describe('mock 响应符合 OpenAPI 契约', () => {
  it.each(cases)('$name', async ({ path, method, status, url, init }) => {
    const response = await window.fetch(url, init);
    expect(response.status, `${method.toUpperCase()} ${url} 状态码`).toBe(Number(status));

    const body = await response.json();
    const validate = responseValidator(path, method, status);
    const valid = validate(body);
    expect(valid, JSON.stringify(validate.errors, null, 2)).toBe(true);
  });
});
