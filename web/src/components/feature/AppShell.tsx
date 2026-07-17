// ============================================================
// nav.ax App Shell — layout wrapper for /app/* routes
// ============================================================

import { useState, useMemo } from 'react';
import { Link, Navigate, useLocation, Outlet, useSearchParams } from 'react-router-dom';
import {
  LayoutDashboard, Link2, Puzzle, Palette, Settings, Globe, ArrowLeft, Menu,
  ChevronRight, Home, Shield, Sparkles, ChevronDown, ExternalLink, BookOpen, BarChart3
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { useCurrentUser } from '@/hooks/useQueries';
import { LoadingSkeleton } from '@/components/base/SharedUI';
import WorkspaceSidebar, { type SidebarNavItem } from '@/components/feature/WorkspaceSidebar';
import PublishStatusControl from '@/components/feature/PublishStatusControl';
import { useKeyboardShortcuts } from '@/hooks/useKeyboardShortcuts';

const navItems: SidebarNavItem[] = [
  { path: '/app', icon: LayoutDashboard, label: '概览' },
  { path: '/app/links', icon: Link2, label: '导航编辑' },
  { path: '/app/analytics', icon: BarChart3, label: '访问统计' },
  { path: '/app/widgets', icon: Puzzle, label: '首页信息' },
  { path: '/app/themes', icon: Palette, label: '主题设置' },
  { path: '/app/publish', icon: Globe, label: '发布 & 域名' },
  { path: '/app/import-export', icon: BookOpen, label: '导入导出' },
  { path: '/app/settings', icon: Settings, label: '账号设置' },
];

const breadcrumbLabels: Record<string, string> = {
  '/app': '工作台',
  '/app/links': '导航编辑',
  '/app/analytics': '访问统计',
  '/app/widgets': '首页信息',
  '/app/themes': '主题设置',
  '/app/publish': '发布 & 域名',
  '/app/import-export': '导入导出',
  '/app/settings': '账号设置',
};

function AppSidebarBottom({ admin }: { admin: boolean }) {
  return (
    <div className="border-t border-background-200/70 p-2 space-y-0.5">
      {admin && (
        <Link
          to="/admin"
          className="flex items-center gap-2.5 h-8 px-2.5 rounded-md text-sm text-foreground-500 hover:text-foreground-700 hover:bg-background-100 transition-colors duration-150 whitespace-nowrap"
        >
          <Shield className="w-3.5 h-3.5 text-accent-500 flex-shrink-0" />
          管理后台
        </Link>
      )}
      <Link
        to="/"
        className="flex items-center gap-2.5 h-8 px-2.5 rounded-md text-sm text-foreground-400 hover:text-foreground-700 transition-colors duration-150 whitespace-nowrap"
      >
        <ArrowLeft className="w-3.5 h-3.5 flex-shrink-0" />
        退出工作台
      </Link>
    </div>
  );
}

function EditModeToggle({ scope, onToggle }: { scope: 'system' | 'personal'; onToggle: () => void }) {
  return (
    <button
      onClick={onToggle}
      className="flex items-center gap-1.5 h-7 px-2.5 rounded-md bg-accent-100 text-accent-700 text-xs font-medium hover:bg-accent-200 transition-colors duration-150 whitespace-nowrap"
      title={scope === 'system' ? '点击切换到我的导航' : '点击切换到主站管理'}
    >
      {scope === 'system' ? (
        <>
          <Globe className="w-3 h-3" />
          管理主站
          <ChevronDown className="w-3 h-3 opacity-60" />
        </>
      ) : (
        <>
          <Sparkles className="w-3 h-3" />
          我的导航
          <ChevronDown className="w-3 h-3 opacity-60" />
        </>
      )}
    </button>
  );
}

export default function AppShell() {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const { data: authUser, isLoading, error, status } = useCurrentUser();

  const user = authUser?.user;
  const isAdminUser = user?.role === 'admin';
  const hasExplicitScope = searchParams.has('scope');
  const scope = searchParams.get('scope') === 'system' ? 'system' : 'personal';

  const scopedNavItems = useMemo(() => navItems.map(item => ({
    ...item,
    path: `${item.path}?scope=${scope}`,
  })), [scope]);

  // Compute visible quick links — must be before any early return (rules of hooks)
  const quickLinkLabel = useMemo(() => {
    if (isAdminUser && scope === 'system') {
      return '访问 nav.ax 主站';
    }
    return '访问我的主页';
  }, [isAdminUser, scope]);

  const visibleQuickLinks = useMemo(() => [
    { path: '/', icon: Home, label: quickLinkLabel },
  ], [quickLinkLabel]);

  const currentPageLabel = breadcrumbLabels[location.pathname] || '';

  // Keyboard shortcuts
  useKeyboardShortcuts({
    onFocusSearch: () => {
      const searchInput = document.querySelector<HTMLInputElement>(
        'input[type="search"], input[placeholder*="搜索"], input[data-search-input]',
      );
      if (searchInput) { searchInput.focus(); searchInput.select(); }
    },
  });

  // Auth guard
  if (status === 'pending' || isLoading) {
    return (
      <div className="min-h-screen bg-background-50 flex items-center justify-center">
        <div className="w-full max-w-sm space-y-4">
          <div className="skeleton h-6 w-32 mx-auto" />
          <LoadingSkeleton count={4} />
        </div>
      </div>
    );
  }

  if (error || !authUser || !authUser.user) {
    return <Navigate to="/login" replace />;
  }

  if (isAdminUser && !hasExplicitScope) {
    return <Navigate to={`${location.pathname}?scope=system`} replace />;
  }

  const toggleScope = () => {
    const next = new URLSearchParams(searchParams);
    next.set('scope', scope === 'system' ? 'personal' : 'system');
    setSearchParams(next, { replace: true });
  };

  return (
    <div className="min-h-screen bg-background-50 flex" data-workspace>
      <WorkspaceSidebar
        open={sidebarOpen}
        onClose={() => setSidebarOpen(false)}
        navItems={scopedNavItems}
        currentPath={location.pathname}
        quickLinks={visibleQuickLinks}
        variant="app"
        badge="工作台"
        bottomContent={<AppSidebarBottom admin={isAdminUser} />}
      />

      {/* Main */}
      <div className="flex-1 min-w-0 flex flex-col">
        <header className="sticky top-0 z-30 h-12 bg-background-50/80 backdrop-blur-md border-b border-background-200/70 flex items-center px-4 md:px-6 gap-3">
          <button
            className="lg:hidden w-8 h-8 flex items-center justify-center rounded-md text-foreground-600 hover:bg-background-100"
            onClick={() => setSidebarOpen(true)}
            aria-label="打开菜单"
          >
            <Menu className="w-4 h-4" />
          </button>

          {/* Breadcrumb */}
          <div className="hidden sm:flex items-center gap-1.5 text-sm">
            <span className="text-foreground-400">工作台</span>
            <ChevronRight className="w-3.5 h-3.5 text-foreground-300" />
            <span className="text-foreground-700 font-medium">{currentPageLabel}</span>
          </div>

          <div className="flex-1" />

          {/* Admin edit mode toggle */}
          {isAdminUser && <EditModeToggle scope={scope} onToggle={toggleScope} />}

          {/* Actions */}
          <PublishStatusControl />
          <Link
            to={`/app/settings?scope=${scope}`}
            className="flex items-center gap-2 text-xs text-foreground-500 hover:text-foreground-700 transition-colors duration-150"
          >
            {user.avatarUrl ? (
              <img
                src={user.avatarUrl}
                alt=""
                className="w-6 h-6 rounded-full object-cover bg-background-200"
              />
            ) : (
              <span
                aria-hidden
                className="w-6 h-6 rounded-full bg-primary-100 text-primary-700 text-[11px] font-semibold inline-flex items-center justify-center"
              >
                {(user.username || '?').slice(0, 1).toUpperCase()}
              </span>
            )}
            <span className="hidden sm:inline">{user.username}</span>
          </Link>
        </header>

        <div className="flex-1 p-4 md:p-5">
          <Outlet />
        </div>
      </div>
    </div>
  );
}
