import { useState, useMemo } from 'react';
import { Link, useLocation, Outlet, Navigate } from 'react-router-dom';
import {
  LayoutDashboard, Users, Link2, FolderOpen, Palette,
  Settings, FileSearch, Shield, Menu, Home, ChevronRight, ArrowLeft, Globe, Wrench, Star
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { useCurrentUser } from '@/hooks/useQueries';
import { LoadingSkeleton } from '@/components/base/SharedUI';
import WorkspaceSidebar, { type SidebarNavItem } from '@/components/feature/WorkspaceSidebar';

const navItems: SidebarNavItem[] = [
  { path: '/admin', icon: LayoutDashboard, label: '运营概览' },
  { path: '/admin/users', icon: Users, label: '用户管理' },
  { path: '/admin/invitations', icon: Link2, label: '邀请管理' },
  { path: '/admin/discover', icon: Star, label: '发现页运营' },
  { path: '/admin/links', icon: Globe, label: '链接管理' },
  { path: '/admin/categories', icon: FolderOpen, label: '公共分类' },
  { path: '/admin/themes', icon: Palette, label: '主题管理' },
  { path: '/admin/settings', icon: Settings, label: '系统配置' },
  { path: '/admin/operations', icon: Wrench, label: '运维中心' },
  { path: '/admin/audit', icon: FileSearch, label: '操作审计' },
];

const breadcrumbLabels: Record<string, string> = {
  '/admin': '运营概览',
  '/admin/users': '用户管理',
  '/admin/invitations': '邀请管理',
  '/admin/discover': '发现页运营',
  '/admin/links': '链接管理',
  '/admin/categories': '公共分类',
  '/admin/themes': '主题管理',
  '/admin/settings': '系统配置',
  '/admin/operations': '运维中心',
  '/admin/audit': '操作审计',
};

function AdminSidebarBottom() {
  return (
    <div className="border-t border-background-200/70 p-2 space-y-0.5">
      <Link to="/app" className="flex items-center gap-2.5 h-8 px-2.5 rounded-md text-sm text-foreground-500 hover:text-foreground-700 hover:bg-background-100 transition-colors duration-150 whitespace-nowrap">
        <ArrowLeft className="w-3.5 h-3.5 flex-shrink-0" />
        返回 App
      </Link>
      <Link to="/" className="flex items-center gap-2.5 h-8 px-2.5 rounded-md text-sm text-foreground-400 hover:text-foreground-700 transition-colors duration-150 whitespace-nowrap">
        <Home className="w-3.5 h-3.5 flex-shrink-0" />
        返回首页
      </Link>
    </div>
  );
}

export default function AdminShell() {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const location = useLocation();
  const { data: authUser, isLoading, error, status } = useCurrentUser();

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

  if (error || !authUser || !authUser.user || authUser.user.role !== 'admin') {
    return <Navigate to="/login" replace />;
  }

  const currentPageLabel = breadcrumbLabels[location.pathname] || '';
  const adminUser = authUser.user;

  return (
    <div className="min-h-screen bg-background-50 flex" data-workspace>
      <WorkspaceSidebar
        open={sidebarOpen}
        onClose={() => setSidebarOpen(false)}
        navItems={navItems}
        currentPath={location.pathname}
        variant="admin"
        badge="管理端"
        bottomContent={<AdminSidebarBottom />}
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
            <span className="text-foreground-400">管理</span>
            <ChevronRight className="w-3.5 h-3.5 text-foreground-300" />
            <span className="text-foreground-700 font-medium">{currentPageLabel}</span>
          </div>

          <div className="flex-1" />

          {/* Admin indicator */}
          <div className="flex items-center gap-1.5 text-xs text-foreground-400">
            <Shield className="w-3.5 h-3.5 text-accent-500" />
            <span className="hidden sm:inline">管理员模式</span>
            <span className="font-medium text-foreground-600 hidden sm:inline">{adminUser?.username}</span>
          </div>
        </header>

        <div className="flex-1 p-4 md:p-5">
          <Outlet />
        </div>
      </div>
    </div>
  );
}
