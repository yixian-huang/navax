// ============================================================
// API Client 204 No Content Response Handling
// ============================================================
// Regression test for DELETE /me/themes/{themeId} which returns 204.
// Ensures the client properly handles empty response bodies without throwing.

import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { themesApi } from '@/api/themes';
import { installMockApi, uninstallMockApi } from '@/api/mock-handlers';
import { request } from '@/api/client';
import type { ApiResponse, Theme } from '@/api/types';

describe('API Client 204 No Content', () => {
  beforeAll(() => {
    installMockApi();
  });

  afterAll(() => {
    uninstallMockApi();
  });

  it('should handle themesApi.uninstall (204 response) without throwing', async () => {
    // First, import a mock private theme via POST
    const importResponse = await request<ApiResponse<Theme>>('/me/themes/import', {
      method: 'POST',
      body: { githubUrl: 'https://github.com/user/theme-repo' },
    });

    expect(importResponse.code).toBe('OK');
    expect(importResponse.data).toBeDefined();
    const themeId = importResponse.data!.id;

    // Then, uninstall it via DELETE (returns 204)
    // Should not throw SyntaxError even though response has no body
    const uninstallResult = await themesApi.uninstall(themeId);

    // The result should be a resolved promise without throwing
    // Client converts 204 to { code: 'OK', data: null, meta: {} }
    expect(uninstallResult).toEqual({
      code: 'OK',
      data: null,
      meta: {},
    });
  });

  it('should handle request<null> on 204 responses', async () => {
    // Import a theme first
    const importResponse = await request<ApiResponse<Theme>>('/me/themes/import', {
      method: 'POST',
      body: { githubUrl: 'https://github.com/user/theme-repo' },
    });
    const themeId = importResponse.data!.id;

    // Call request directly expecting null result
    const result = await request('/me/themes/' + encodeURIComponent(themeId), {
      method: 'DELETE',
    });

    expect(result).toEqual({
      code: 'OK',
      data: null,
      meta: {},
    });
  });
});
