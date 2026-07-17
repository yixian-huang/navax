// ============================================================
// nav.ax Link preview API — server-side metadata fetch
// ============================================================

import { request } from '@/api/client';
import type { ApiResponse } from '@/api/types';

export interface LinkPreview {
  url: string;
  title: string;
  description: string;
  faviconUrl: string;
  siteName?: string | null;
}

export const linkPreviewApi = {
  preview: (url: string) =>
    request<ApiResponse<LinkPreview>>('/link-preview', {
      method: 'POST',
      body: { url },
    }),
};
