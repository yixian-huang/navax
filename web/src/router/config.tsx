import { Navigate } from "react-router-dom";
import { lazy } from "react";
import type { RouteObject } from "react-router-dom";

const NotFound = lazy(() => import("@/pages/NotFound"));
const Home = lazy(() => import("@/pages/home/page"));
const LoginPage = lazy(() => import("@/pages/login/page"));
const SetupPage = lazy(() => import("@/pages/setup/page"));
const InvitePage = lazy(() => import("@/pages/invite/page"));
const PublicSharePage = lazy(() => import("@/pages/public-share/page"));
const DiscoverPage = lazy(() => import("@/pages/discover/page"));
const AppShell = lazy(() => import("@/components/feature/AppShell"));
const AdminShell = lazy(() => import("@/components/feature/AdminShell"));
const AppOverview = lazy(() => import("@/pages/app/overview/page"));
const LinksPage = lazy(() => import("@/pages/app/links/page"));
const AnalyticsPage = lazy(() => import("@/pages/app/analytics/page"));
const WidgetsPage = lazy(() => import("@/pages/app/widgets/page"));
const ThemesPage = lazy(() => import("@/pages/app/themes/page"));
const PublishPage = lazy(() => import("@/pages/app/publish/page"));
const ImportExportPage = lazy(() => import("@/pages/app/import-export/page"));
const SettingsPage = lazy(() => import("@/pages/app/settings/page"));
const AdminOverview = lazy(() => import("@/pages/admin/overview/page"));
const AdminUsersPage = lazy(() => import("@/pages/admin/users/page"));
const AdminInvitationsPage = lazy(() => import("@/pages/admin/invitations/page"));
const AdminDirectoryPage = lazy(() => import("@/pages/admin/directory/page"));
const AdminCategoriesPage = lazy(() => import("@/pages/admin/categories/page"));
const AdminThemesPage = lazy(() => import("@/pages/admin/themes/page"));
const AdminSettingsPage = lazy(() => import("@/pages/admin/settings/page"));
const AdminAuditPage = lazy(() => import("@/pages/admin/audit/page"));
const AdminLinksPage = lazy(() => import("@/pages/admin/links/page"));
const AdminOperationsPage = lazy(() => import("@/pages/admin/operations/page"));

const routes: RouteObject[] = [
  // Public
  {
    path: "/",
    element: <Home />,
  },
  {
    path: "/login",
    element: <LoginPage />,
  },
  {
    path: "/setup",
    element: <SetupPage />,
  },
  {
    path: "/invite/:token",
    element: <InvitePage />,
  },
  {
    path: "/u/:slug",
    element: <PublicSharePage />,
  },
  {
    path: "/discover",
    element: <DiscoverPage />,
  },

  // App (user dashboard)
  {
    path: "/app",
    element: <AppShell />,
    children: [
      { index: true, element: <AppOverview /> },
      { path: "links", element: <LinksPage /> },
      { path: "analytics", element: <AnalyticsPage /> },
      { path: "layout", element: <Navigate to="/app/links" replace /> },
      { path: "widgets", element: <WidgetsPage /> },
      { path: "themes", element: <ThemesPage /> },
      { path: "publish", element: <PublishPage /> },
      { path: "import-export", element: <ImportExportPage /> },
      { path: "settings", element: <SettingsPage /> },
      { path: "domain", element: <Navigate to="/app/publish" replace /> },
    ],
  },

  // Admin
  {
    path: "/admin",
    element: <AdminShell />,
    children: [
      { index: true, element: <AdminOverview /> },
      { path: "users", element: <AdminUsersPage /> },
      { path: "invitations", element: <AdminInvitationsPage /> },
      { path: "directory", element: <AdminDirectoryPage /> },
      { path: "links", element: <AdminLinksPage /> },
      { path: "categories", element: <AdminCategoriesPage /> },
      { path: "themes", element: <AdminThemesPage /> },
      { path: "settings", element: <AdminSettingsPage /> },
      { path: "operations", element: <AdminOperationsPage /> },
      { path: "audit", element: <AdminAuditPage /> },
    ],
  },

  // 404
  {
    path: "*",
    element: <NotFound />,
  },
];

export default routes;
