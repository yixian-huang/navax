// ============================================================
// nav.ax Background media library API
// ============================================================

import { request } from '@/api/client';
import type { ApiResponse, BackgroundMedia } from '@/api/types';

export const backgroundsApi = {
  listPresets: (all = false) =>
    request<ApiResponse<BackgroundMedia[]>>(
      `/backgrounds/presets${all ? '?all=1' : ''}`,
    ),

  uploadPreset: (file: File) => {
    const body = new FormData();
    body.set('file', file);
    return request<ApiResponse<BackgroundMedia>>('/backgrounds/presets', {
      method: 'POST',
      body,
    });
  },

  deletePreset: (id: string) =>
    request<ApiResponse<{ deleted: boolean }>>(`/backgrounds/presets/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    }),

  listMine: () => request<ApiResponse<BackgroundMedia[]>>('/backgrounds/mine'),

  uploadMine: (file: File) => {
    const body = new FormData();
    body.set('file', file);
    return request<ApiResponse<BackgroundMedia>>('/backgrounds/mine', {
      method: 'POST',
      body,
    });
  },

  deleteMine: (id: string) =>
    request<ApiResponse<{ deleted: boolean }>>(`/backgrounds/mine/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    }),
};
