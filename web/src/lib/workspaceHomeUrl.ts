import type { Publication, SubdomainInfo } from '@/api/types';

/**
 * Resolve the URL to open when leaving the app workspace for "my homepage".
 * Priority: custom CNAME → approved subdomain → published /u/{slug} → root.
 */
export function resolveWorkspaceHomeUrl(options: {
  scope: 'system' | 'personal';
  subdomain?: SubdomainInfo | null;
  publication?: Publication | null;
}): string {
  if (options.scope === 'system') {
    return '/';
  }

  const sub = options.subdomain;
  if (sub && sub.status === 'approved') {
    const cname = (sub.customDomain || '').trim().replace(/^https?:\/\//i, '').replace(/\/+$/, '');
    if (cname) return `https://${cname}`;
    const full = (sub.fullDomain || '').trim().replace(/^https?:\/\//i, '').replace(/\/+$/, '');
    if (full) return `https://${full}`;
  }

  const slug = (options.publication?.slug || '').trim();
  if (slug) return `/u/${encodeURIComponent(slug)}`;

  return '/';
}

export function isExternalHomeUrl(url: string): boolean {
  return /^https?:\/\//i.test(url);
}
