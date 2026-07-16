// ============================================================
// nav.ax Assets API — image uploads and public instance config
// ============================================================

import { request } from '@/api/client';
import type { ApiResponse, Asset, AssetKind, PublicConfig } from '@/api/types';

export const assetsApi = {
  /** 上传图片资源（avatar / background / site-icon），返回不可变的 asset URL。 */
  upload: (kind: AssetKind, file: File) => {
    const body = new FormData();
    body.set('kind', kind);
    body.set('file', file);
    return request<ApiResponse<Asset>>('/assets', { method: 'POST', body });
  },
};

let publicConfigRequest: Promise<ApiResponse<PublicConfig>> | undefined;

/** 获取公开实例配置（含上传大小上限等），进程内缓存一次。 */
export function getPublicConfig(): Promise<ApiResponse<PublicConfig>> {
  publicConfigRequest ??= request<ApiResponse<PublicConfig>>('/public/config').catch(error => {
    publicConfigRequest = undefined;
    throw error;
  });
  return publicConfigRequest;
}
