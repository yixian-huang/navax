import PublicNavigationView, { HomeEmpty404 } from '@/components/feature/PublicNavigationView';
import { useCurrentUser, useSystemPage } from '@/hooks/useQueries';

export default function HomePage() {
  const { data: page, isLoading, error, refetch } = useSystemPage();
  const { data: authSession } = useCurrentUser();

  const displayName = authSession?.user?.username || '朋友';
  const host = typeof window !== 'undefined' ? window.location.hostname.toLowerCase() : '';
  const isSubdomainHost = host.split('.').length > 2
    || (host.endsWith('.localhost') && host !== 'localhost');

  // Personal subdomain home: offer share chrome without changing layout.
  const isPersonalPublished = page?.kind === 'personal';
  const share = isPersonalPublished && page
    ? {
        title: page.title,
        url: typeof window !== 'undefined' ? window.location.href : '',
        ownerName: page.ownerName || page.title,
        subdomain: page.subdomain || undefined,
      }
    : null;

  return (
    <PublicNavigationView
      page={page}
      isLoading={isLoading}
      error={error}
      onRetry={() => refetch()}
      displayName={isPersonalPublished ? (page?.ownerName || page?.title || displayName) : displayName}
      showBrowserGuide={!isSubdomainHost}
      share={share}
      empty404={(error as { status?: number } | null)?.status === 404 ? (
        <HomeEmpty404
          isSubdomainHost={isSubdomainHost}
          isLoggedIn={Boolean(authSession?.user)}
          isAdmin={authSession?.user?.role === 'admin'}
        />
      ) : undefined}
    />
  );
}
