// ============================================================
// nav.ax Query Hooks — TanStack Query wrappers
// ============================================================

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { ApiError } from '@/api/client';
import { authApi } from '@/api/auth';
import { navigationApi } from '@/api/navigation';
import { analyticsApi } from '@/api/analytics';
import { adminApi } from '@/api/admin';
import type {
  CreateCategoryRequest,
  CreateSiteRequest,
  UpdateSiteRequest,
  PageKind,
  PageSettings,
  PublicationSettingsUpdate,
  CreateInvitationRequest,
  CreatePlatformSiteRequest,
  DirectoryCategoryInput,
  SystemSettings,
  UpdatePlatformSiteRequest,
  SubdomainRequest,
  ChangePasswordRequest,
  UpdateProfileRequest,
} from '@/api/types';

// ---- Auth ----
export function useCurrentUser() {
  return useQuery({
    queryKey: ['auth', 'session'],
    queryFn: async () => {
      const res = await authApi.getSession();
      return res.data;
    },
    staleTime: 5 * 60 * 1000,
  });
}

export function useProfile() {
  const session = useCurrentUser();
  return useQuery({
    queryKey: ['auth', 'profile'],
    queryFn: async () => (await authApi.getProfile()).data,
    enabled: Boolean(session.data?.authenticated),
  });
}

export function useUpdateProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: UpdateProfileRequest) => authApi.updateProfile(data),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['auth', 'profile'] }),
        queryClient.invalidateQueries({ queryKey: ['auth', 'session'] }),
      ]);
    },
  });
}

export function useSessions() {
  const session = useCurrentUser();
  return useQuery({
    queryKey: ['auth', 'sessions'],
    queryFn: async () => (await authApi.listSessions()).data,
    enabled: Boolean(session.data?.authenticated),
  });
}

export function useChangePassword() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: ChangePasswordRequest) => authApi.changePassword(data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['auth', 'sessions'] }),
  });
}

export function useRevokeSession() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (sessionId: string) => authApi.revokeSession(sessionId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['auth', 'sessions'] }),
  });
}

export function useLogout() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => authApi.logout(),
    onSuccess: () => queryClient.clear(),
  });
}

// ---- Navigation Page ----
export function usePageScope(): PageKind {
  const [searchParams] = useSearchParams();
  return searchParams.get('scope') === 'system' ? 'system' : 'personal';
}

export function useMyPage(scopeOverride?: PageKind) {
  const routeScope = usePageScope();
  const scope = scopeOverride ?? routeScope;
  return useQuery({
    queryKey: ['navigation', 'page', scope],
    queryFn: async () => {
      const res = await navigationApi.getCurrentPage(scope);
      return res.data;
    },
  });
}

export function useUpdatePageSettings() {
  const qc = useQueryClient();
  const scope = usePageScope();
  const { data: page } = useMyPage(scope);
  return useMutation({
    mutationFn: async (settings: PageSettings) => {
      if (!page) throw new Error('导航页尚未加载');
      return navigationApi.forPage(page.id).replaceSettings({
        ...settings,
        expectedRevision: page.draftRevision ?? 0,
      });
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['navigation', 'page', scope] }),
  });
}

export function useSavePageComposition() {
  const queryClient = useQueryClient();
  const scope = usePageScope();
  const { data: page } = useMyPage(scope);
  return useMutation({
    mutationFn: async (input: {
      categories: { id: string; siteIds: string[] }[];
      settings: PageSettings;
    }) => {
      if (!page?.settings) throw new Error('页面设置尚未加载');
      const api = navigationApi.forPage(page.id);
      const orderResponse = await api.replaceContentOrder({
        expectedRevision: page.draftRevision ?? 0,
        categories: input.categories,
      });
      return api.replaceSettings({
        ...input.settings,
        expectedRevision: orderResponse.data.draftRevision,
      });
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['navigation', 'page', scope] }),
  });
}

// ---- Categories ----
export function useCategories() {
  const scope = usePageScope();
  const { data: page } = useMyPage(scope);
  return useQuery({
    queryKey: ['navigation', 'categories', scope, page?.id],
    queryFn: async () => {
      if (!page) return [];
      return page.categories;
    },
    enabled: Boolean(page),
  });
}

export function useCreateCategory() {
  const qc = useQueryClient();
  const scope = usePageScope();
  const { data: page } = useMyPage(scope);
  return useMutation({
    mutationFn: (data: CreateCategoryRequest) => {
      if (!page) throw new Error('导航页尚未加载');
      return navigationApi.forPage(page.id).createCategory(data);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['navigation', 'page', scope] }),
  });
}

export function useUpdateCategory() {
  const qc = useQueryClient();
  const scope = usePageScope();
  const { data: page } = useMyPage(scope);
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<CreateCategoryRequest> }) => {
      if (!page) throw new Error('导航页尚未加载');
      return navigationApi.forPage(page.id).updateCategory(id, data);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['navigation', 'page', scope] }),
  });
}

export function useDeleteCategory() {
  const qc = useQueryClient();
  const scope = usePageScope();
  const { data: page } = useMyPage(scope);
  return useMutation({
    mutationFn: (id: string) => {
      if (!page) throw new Error('导航页尚未加载');
      return navigationApi.forPage(page.id).deleteCategory(id);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['navigation', 'page', scope] }),
  });
}

// ---- Sites ----
export function useSites(categoryId?: string) {
  const scope = usePageScope();
  const { data: page } = useMyPage(scope);
  return useQuery({
    queryKey: ['navigation', 'sites', scope, page?.id, categoryId],
    queryFn: async () => {
      if (!page) return [];
      return categoryId
        ? page.categories.find(category => category.id === categoryId)?.sites ?? []
        : page.categories.flatMap(category => category.sites);
    },
    enabled: Boolean(page),
  });
}

export function useCreateSite() {
  const qc = useQueryClient();
  const scope = usePageScope();
  const { data: page } = useMyPage(scope);
  return useMutation({
    mutationFn: (data: CreateSiteRequest) => {
      if (!page) throw new Error('导航页尚未加载');
      return navigationApi.forPage(page.id).createSite(data);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['navigation', 'page', scope] }),
  });
}

export function useUpdateSite() {
  const qc = useQueryClient();
  const scope = usePageScope();
  const { data: page } = useMyPage(scope);
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateSiteRequest }) => {
      if (!page) throw new Error('导航页尚未加载');
      return navigationApi.forPage(page.id).updateSite(id, data);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['navigation', 'page', scope] }),
  });
}

export function useDeleteSite() {
  const qc = useQueryClient();
  const scope = usePageScope();
  const { data: page } = useMyPage(scope);
  return useMutation({
    mutationFn: (id: string) => {
      if (!page) throw new Error('导航页尚未加载');
      return navigationApi.forPage(page.id).deleteSite(id);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['navigation', 'page', scope] }),
  });
}

// ---- Publish ----
export function usePublishSettings() {
  const scope = usePageScope();
  const { data: page } = useMyPage(scope);
  return useQuery({
    queryKey: ['navigation', 'publication', scope, page?.id],
    queryFn: async () => {
      if (!page) throw new Error('导航页尚未加载');
      const res = await navigationApi.forPage(page.id).getPublication();
      return res.data;
    },
    enabled: Boolean(page),
  });
}

export function useUpdatePublication() {
  const qc = useQueryClient();
  const scope = usePageScope();
  const { data: page } = useMyPage(scope);
  return useMutation({
    mutationFn: (settings: PublicationSettingsUpdate) => {
      if (!page) throw new Error('导航页尚未加载');
      return navigationApi.forPage(page.id).replacePublication(settings);
    },
    onSuccess: async () => {
      await Promise.all([
        qc.invalidateQueries({ queryKey: ['navigation', 'page', scope] }),
        qc.invalidateQueries({ queryKey: ['navigation', 'publication', scope] }),
      ]);
    },
  });
}

export function usePublish() {
  const qc = useQueryClient();
  const scope = usePageScope();
  const { data: page } = useMyPage(scope);
  return useMutation({
    mutationFn: () => {
      if (!page) throw new Error('导航页尚未加载');
      return navigationApi.forPage(page.id).publish(page.draftRevision ?? 0);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['navigation', 'page', scope] }),
  });
}

export function useUnpublish() {
  const qc = useQueryClient();
  const scope = usePageScope();
  const { data: page } = useMyPage(scope);
  return useMutation({
    mutationFn: () => {
      if (!page) throw new Error('导航页尚未加载');
      return navigationApi.forPage(page.id).unpublish();
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['navigation', 'page', scope] }),
  });
}

// ---- Public Page ----
export function usePublicPage(slug: string) {
  return useQuery({
    queryKey: ['public', 'page', slug],
    queryFn: async () => {
      const res = await navigationApi.getPublicPage(slug);
      return res.data;
    },
    enabled: !!slug,
  });
}

// ---- System Page (nav.ax homepage) ----
export function useSystemPage() {
  return useQuery({
    queryKey: ['public', 'page', 'nav'],
    queryFn: async () => {
      const res = await navigationApi.getPublicPage('nav');
      return res.data;
    },
    // 契约规定主站未发布时返回 404，属稳定状态，重试只会拖慢空状态展示。
    retry: (failureCount, error) =>
      !(error instanceof ApiError && error.status === 404) && failureCount < 3,
  });
}

// ---- Themes ----
export function useThemes() {
  return useQuery({
    queryKey: ['navigation', 'themes'],
    queryFn: async () => {
      const res = await navigationApi.getThemes();
      return res.data;
    },
  });
}

export function usePlatformDirectory(
  params?: { category?: string; search?: string; page?: number; pageSize?: number },
  enabled = true,
) {
  return useQuery({
    queryKey: ['public', 'directory', params],
    queryFn: async () => (await navigationApi.getPlatformSites(params)).data,
    enabled,
  });
}

export function useDiscoverPages(params?: {
  search?: string;
  tag?: string;
  sort?: 'latest' | 'popular' | 'featured';
  page?: number;
  pageSize?: number;
}) {
  return useQuery({
    queryKey: ['public', 'discover', params],
    queryFn: async () => (await navigationApi.discoverPages(params)).data,
  });
}

// ---- Admin ----
export function useAdminOverview() {
  return useQuery({
    queryKey: ['admin', 'overview'],
    queryFn: async () => {
      const res = await adminApi.getOverview();
      return res.data;
    },
  });
}

export function useAdminUsers(params?: { search?: string; status?: string; page?: number; pageSize?: number }) {
  return useQuery({
    queryKey: ['admin', 'users', params],
    queryFn: async () => {
      const res = await adminApi.getUsers(params);
      return res.data;
    },
  });
}

export function useAdminInvitations(params?: { page?: number; pageSize?: number }) {
  return useQuery({
    queryKey: ['admin', 'invitations', params],
    queryFn: async () => {
      const res = await adminApi.getInvitations(params);
      return res.data;
    },
  });
}

export function useAdminDirectorySites(params?: { categoryId?: string; search?: string; page?: number; pageSize?: number }) {
  return useQuery({
    queryKey: ['admin', 'directory', 'sites', params],
    queryFn: async () => {
      const res = await adminApi.getDirectorySites(params);
      return res.data;
    },
  });
}

export function useAdminDirectoryCategories() {
  return useQuery({
    queryKey: ['admin', 'directory', 'categories'],
    queryFn: async () => {
      const res = await adminApi.getDirectoryCategories();
      return res.data;
    },
  });
}

export function useAdminThemes() {
  return useQuery({
    queryKey: ['admin', 'themes'],
    queryFn: async () => {
      const res = await adminApi.getPlatformThemes();
      return res.data;
    },
  });
}

export function useAdminSettings() {
  return useQuery({
    queryKey: ['admin', 'settings'],
    queryFn: async () => {
      const res = await adminApi.getSystemSettings();
      return res.data;
    },
  });
}

export function useAdminAudit(params?: { page?: number; pageSize?: number; action?: string }) {
  return useQuery({
    queryKey: ['admin', 'audit', params],
    queryFn: async () => {
      const res = await adminApi.getAuditLogs(params);
      return res.data;
    },
  });
}

// ---- Admin mutations ----
export function useDisableUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => adminApi.disableUser(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'users'] }),
  });
}

export function useEnableUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => adminApi.enableUser(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'users'] }),
  });
}

export function useRevokeUserSessions() {
  return useMutation({
    mutationFn: (id: string) => adminApi.revokeUserSessions(id),
  });
}

export function useResetUserPassword() {
  return useMutation({
    mutationFn: (id: string) => adminApi.resetUserPassword(id),
  });
}

export function useCreateInvitation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateInvitationRequest) => adminApi.createInvitation(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'invitations'] }),
  });
}

export function useRevokeInvitation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => adminApi.revokeInvitation(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'invitations'] }),
  });
}

export function useCreateDirectorySite() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreatePlatformSiteRequest) => adminApi.createDirectorySite(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'directory'] }),
  });
}

export function useToggleDirectorySite() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) => adminApi.updateDirectorySiteState(id, enabled),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'directory'] }),
  });
}

export function useUpdateDirectorySite() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdatePlatformSiteRequest }) => adminApi.updateDirectorySite(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'directory'] }),
  });
}

export function useDeleteDirectorySite() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => adminApi.deleteDirectorySite(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'directory'] }),
  });
}

export function useCreateDirectoryCategory() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: DirectoryCategoryInput) => adminApi.createDirectoryCategory(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'directory'] }),
  });
}

export function useUpdateDirectoryCategory() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: DirectoryCategoryInput }) => adminApi.updateDirectoryCategory(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'directory'] }),
  });
}

export function useDeleteDirectoryCategory() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => adminApi.deleteDirectoryCategory(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'directory'] }),
  });
}

export function useUpdateAdminSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: SystemSettings) => adminApi.updateSystemSettings(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'settings'] }),
  });
}

export function useUpdateAdminThemeState() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ themeId, data }: { themeId: string; data: { enabled?: boolean; default?: boolean } }) =>
      adminApi.updateThemeState(themeId, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'themes'] }),
  });
}

// ---- Admin: All Links ----
export function useAdminLinks(params?: { search?: string; ownerId?: string; page?: number; pageSize?: number }) {
  return useQuery({
    queryKey: ['admin', 'links', params],
    queryFn: async () => {
      const res = await adminApi.getAllLinks(params);
      return res.data;
    },
  });
}

export function useDeleteLink() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => adminApi.deleteLink(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'links'] }),
  });
}

// ---- Analytics ----
export function useAnalytics(params?: { days?: number; siteLimit?: number }) {
  return useQuery({
    queryKey: ['analytics', params],
    queryFn: async () => {
      const res = await analyticsApi.getAnalytics(params);
      return res.data;
    },
    refetchInterval: 60 * 1000,
  });
}

// ---- Subdomain ----
export function useSubdomain() {
  return useQuery({
    queryKey: ['navigation', 'subdomain'],
    queryFn: async () => {
      const res = await navigationApi.getSubdomain();
      return res.data;
    },
  });
}

export function useApplySubdomain() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: SubdomainRequest) => navigationApi.applySubdomain(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['navigation', 'subdomain'] }),
  });
}

export function useCancelSubdomainApplication() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => navigationApi.cancelSubdomainApplication(),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['navigation', 'subdomain'] }),
  });
}
