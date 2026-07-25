import { request } from './client';
import type { ApiResponse, Theme } from './types';

export const themesApi = {
  importZip: (file: File) => {
    const body = new FormData();
    body.set('file', file);
    return request<ApiResponse<Theme>>('/me/themes/import', { method: 'POST', body });
  },
  importGitHub: (githubUrl: string, ref?: string) =>
    request<ApiResponse<Theme>>('/me/themes/import', {
      method: 'POST',
      body: ref ? { githubUrl, ref } : { githubUrl },
    }),
  uninstall: (themeId: string) =>
    request<ApiResponse<null>>(`/me/themes/${encodeURIComponent(themeId)}`, { method: 'DELETE' }),
};
