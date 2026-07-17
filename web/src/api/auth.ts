// ============================================================
// nav.ax Auth API Service
// ============================================================

import { request } from './client';
import type {
  ApiResponse,
  AuthSession,
  LoginRequest,
  InviteRegisterRequest,
  UserProfile,
  UpdateProfileRequest,
  ChangePasswordRequest,
  SessionInfo,
  BootstrapStatus,
  BootstrapRequest,
} from './types';

export const authApi = {
  getBootstrapStatus: () =>
    request<ApiResponse<BootstrapStatus>>('/bootstrap/status'),

  bootstrap: (setupToken: string, data: BootstrapRequest) =>
    request<ApiResponse<AuthSession>>('/bootstrap', {
      method: 'POST',
      headers: { 'X-Setup-Token': setupToken },
      body: data,
    }),

  getSession: () =>
    request<ApiResponse<AuthSession>>('/auth/session'),

  login: (data: LoginRequest) =>
    request<ApiResponse<AuthSession>>('/auth/login', { method: 'POST', body: data }),

  logout: () =>
    request<ApiResponse<null>>('/auth/logout', { method: 'POST' }),

  forgotPassword: (email: string) =>
    request<ApiResponse<{ message: string }>>('/auth/password/forgot', { method: 'POST', body: { email } }),

  resetPassword: (token: string, password: string) =>
    request<ApiResponse<{ message: string }>>('/auth/password/reset', { method: 'POST', body: { token, password } }),

  registerViaInvite: (token: string, data: InviteRegisterRequest) =>
    request<ApiResponse<AuthSession>>(`/auth/invitations/${encodeURIComponent(token)}/register`, { method: 'POST', body: data }),

  registerOpen: (data: InviteRegisterRequest) =>
    request<ApiResponse<AuthSession>>('/auth/register', { method: 'POST', body: data }),

  validateInviteToken: (token: string) =>
    request<ApiResponse<{ valid: true; inviterName: string; expiresAt: string }>>(`/auth/invitations/${encodeURIComponent(token)}`),

  getProfile: () =>
    request<ApiResponse<UserProfile>>('/me/profile'),

  updateProfile: (data: UpdateProfileRequest) =>
    request<ApiResponse<UserProfile>>('/me/profile', { method: 'PATCH', body: data }),

  changePassword: (data: ChangePasswordRequest) =>
    request<ApiResponse<null>>('/me/password', { method: 'PATCH', body: data }),

  listSessions: () =>
    request<ApiResponse<SessionInfo[]>>('/me/sessions'),

  revokeSession: (sessionId: string) =>
    request<ApiResponse<null>>(`/me/sessions/${encodeURIComponent(sessionId)}`, { method: 'DELETE' }),
};
