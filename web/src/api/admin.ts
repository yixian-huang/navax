// ============================================================
// nav.ax Admin API Service
// ============================================================

import { request, requestAttachment } from './client';
import type {
  ApiResponse,
  AdminOverview,
  User,
  Invitation,
  PlatformSite,
  PlatformCategory,
  Theme,
  SystemSettings,
  AuditEntry,
  PaginatedResponse,
  AdminLink,
  CreateInvitationRequest,
  CreatePlatformSiteRequest,
  DirectoryCategoryInput,
  UpdatePlatformSiteRequest,
  ProviderKind,
  ProviderConfig,
  ProviderSummary,
  ProviderConfigUpdate,
  ProviderTestResult,
  UpdateState,
  UpdateSettingsPatch,
  Backup,
  RestoreToken,
  AdminSubdomainRequest,
  ContractSubdomainStatus,
  SubdomainReviewRequest,
  PasswordResetLink,
} from './types';

function asPaginated<T>(response: ApiResponse<T[] | PaginatedResponse<T>>): ApiResponse<PaginatedResponse<T>> {
  if (!Array.isArray(response.data)) {
    return response as ApiResponse<PaginatedResponse<T>>;
  }
  const page = response.meta.page ?? 1;
  const pageSize = response.meta.pageSize ?? response.data.length;
  const total = response.meta.total ?? response.data.length;
  return {
    ...response,
    data: {
      items: response.data,
      page,
      pageSize,
      total,
      totalPages: response.meta.totalPages ?? Math.ceil(total / Math.max(pageSize, 1)),
    },
  };
}

export const adminApi = {
  // Overview
  getOverview: () =>
    request<ApiResponse<AdminOverview>>('/admin/overview'),

  // Users
  getUsers: (params?: { search?: string; status?: string; page?: number; pageSize?: number }) =>
    request<ApiResponse<User[] | PaginatedResponse<User>>>('/admin/users', { params }).then(asPaginated),

  getUserById: (id: string) =>
    request<ApiResponse<User>>(`/admin/users/${id}`),

  disableUser: (id: string) =>
    request<ApiResponse<User>>(`/admin/users/${id}`, { method: 'PATCH', body: { status: 'disabled' } }),

  enableUser: (id: string) =>
    request<ApiResponse<User>>(`/admin/users/${id}`, { method: 'PATCH', body: { status: 'active' } }),

  revokeUserSessions: (id: string) =>
    request<ApiResponse<null>>(`/admin/users/${id}/sessions`, { method: 'DELETE' }),

  resetUserPassword: (id: string) =>
    request<ApiResponse<PasswordResetLink>>(`/admin/users/${id}/password-reset`, { method: 'POST' }),

  // Invitations
  getInvitations: (params?: { page?: number; pageSize?: number }) =>
    request<ApiResponse<Invitation[] | PaginatedResponse<Invitation>>>('/admin/invitations', { params }).then(asPaginated),

  createInvitation: (data: CreateInvitationRequest) =>
    request<ApiResponse<Invitation>>('/admin/invitations', { method: 'POST', body: data }),

  revokeInvitation: (id: string) =>
    request<ApiResponse<Invitation>>(`/admin/invitations/${id}`, { method: 'DELETE' }),

  // Directory
  getDirectorySites: (params?: { categoryId?: string; search?: string; page?: number; pageSize?: number }) =>
    request<ApiResponse<PlatformSite[] | PaginatedResponse<PlatformSite>>>('/admin/directory/sites', { params }).then(asPaginated),

  createDirectorySite: (data: CreatePlatformSiteRequest) =>
    request<ApiResponse<PlatformSite>>('/admin/directory/sites', { method: 'POST', body: data }),

  updateDirectorySite: (id: string, data: UpdatePlatformSiteRequest) =>
    request<ApiResponse<PlatformSite>>(`/admin/directory/sites/${id}`, { method: 'PATCH', body: data }),

  deleteDirectorySite: (id: string) =>
    request<ApiResponse<null>>(`/admin/directory/sites/${id}`, { method: 'DELETE' }),

  updateDirectorySiteState: (id: string, enabled: boolean) =>
    request<ApiResponse<PlatformSite>>(`/admin/directory/sites/${id}`, { method: 'PATCH', body: { enabled } }),

  // Platform Categories
  getDirectoryCategories: () =>
    request<ApiResponse<PlatformCategory[]>>('/admin/directory/categories'),

  createDirectoryCategory: (data: DirectoryCategoryInput) =>
    request<ApiResponse<PlatformCategory>>('/admin/directory/categories', { method: 'POST', body: data }),

  updateDirectoryCategory: (id: string, data: DirectoryCategoryInput) =>
    request<ApiResponse<PlatformCategory>>(`/admin/directory/categories/${id}`, { method: 'PATCH', body: data }),

  deleteDirectoryCategory: (id: string) =>
    request<ApiResponse<null>>(`/admin/directory/categories/${id}`, { method: 'DELETE' }),

  // Themes
  getPlatformThemes: () =>
    request<ApiResponse<Theme[]>>('/admin/themes'),

  setDefaultTheme: (themeId: string) =>
    request<ApiResponse<Theme>>(`/admin/themes/${themeId}`, { method: 'PATCH', body: { default: true } }),

  toggleTheme: (themeId: string, enabled: boolean) =>
    request<ApiResponse<Theme>>(`/admin/themes/${themeId}`, { method: 'PATCH', body: { enabled } }),

  updateThemeState: (themeId: string, data: { enabled?: boolean; default?: boolean }) =>
    request<ApiResponse<Theme>>(`/admin/themes/${themeId}`, { method: 'PATCH', body: data }),

  // Settings
  getSystemSettings: () =>
    request<ApiResponse<SystemSettings>>('/admin/settings'),

  updateSystemSettings: (data: Partial<SystemSettings>) =>
    request<ApiResponse<SystemSettings>>('/admin/settings', { method: 'PATCH', body: data }),

  // Audit
  getAuditLogs: (params?: { page?: number; pageSize?: number; action?: string }) =>
    request<ApiResponse<AuditEntry[] | PaginatedResponse<AuditEntry>>>('/admin/audit', { params }).then(asPaginated),

  // All Links (cross-user link management)
  getAllLinks: (params?: { search?: string; ownerId?: string; page?: number; pageSize?: number }) =>
    request<ApiResponse<AdminLink[] | PaginatedResponse<AdminLink>>>('/admin/links', { params }).then(asPaginated),

  deleteLink: (id: string, reason = '管理员删除') =>
    request<ApiResponse<null>>(`/admin/links/${id}`, { method: 'DELETE', body: { reason } }),

  // Operations: service providers
  getProviders: () =>
    request<ApiResponse<ProviderSummary[]>>('/admin/providers'),

  getProviderConfig: (kind: ProviderKind) =>
    request<ApiResponse<ProviderConfig>>(`/admin/providers/${kind}`),

  updateProviderConfig: (kind: ProviderKind, data: ProviderConfigUpdate) =>
    request<ApiResponse<ProviderConfig>>(`/admin/providers/${kind}`, { method: 'PATCH', body: data }),

  testProviderConfig: (kind: ProviderKind, recipient?: string) =>
    request<ApiResponse<ProviderTestResult>>(`/admin/providers/${kind}/test`, {
      method: 'POST',
      body: recipient ? { recipient } : {},
    }),

  // Operations: updates
  getUpdateState: () =>
    request<ApiResponse<UpdateState>>('/admin/update'),

  updateUpdateSettings: (data: UpdateSettingsPatch) =>
    request<ApiResponse<UpdateState>>('/admin/update', { method: 'PATCH', body: data }),

  checkForUpdates: () =>
    request<ApiResponse<UpdateState>>('/admin/update/check', { method: 'POST' }),

  applyUpdate: (version: string, idempotencyKey: string) =>
    request<ApiResponse<UpdateState>>('/admin/update/apply', {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey },
      body: { version, confirmation: 'APPLY_UPDATE' },
    }),

  // Operations: backups and restore
  getBackups: () =>
    request<ApiResponse<Backup[]>>('/admin/backups'),

  createBackup: () =>
    request<ApiResponse<Backup>>('/admin/backups', { method: 'POST' }),

  downloadBackup: (backupId: string) =>
    requestAttachment(`/admin/backups/${backupId}`),

  createRestoreToken: (backupId: string, password: string) =>
    request<ApiResponse<RestoreToken>>(`/admin/backups/${backupId}/restore-token`, {
      method: 'POST', body: { password },
    }),

  restoreBackup: (backupId: string, restoreToken: string) =>
    request<ApiResponse<null>>(`/admin/backups/${backupId}/restore`, {
      method: 'POST', body: { restoreToken, confirmation: 'RESTORE_BACKUP' },
    }),

  // Operations: subdomain review
  getSubdomainRequests: (params?: { status?: ContractSubdomainStatus; page?: number; pageSize?: number }) =>
    request<ApiResponse<AdminSubdomainRequest[] | PaginatedResponse<AdminSubdomainRequest>>>('/admin/subdomains', { params }).then(asPaginated),

  reviewSubdomainRequest: (requestId: string, data: SubdomainReviewRequest) =>
    request<ApiResponse<AdminSubdomainRequest>>(`/admin/subdomains/${requestId}`, { method: 'PATCH', body: data }),
};
