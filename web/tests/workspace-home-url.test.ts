import { describe, expect, it } from 'vitest';
import { isExternalHomeUrl, resolveWorkspaceHomeUrl } from '@/lib/workspaceHomeUrl';
import type { Publication, SubdomainInfo } from '@/api/types';

const published = (slug: string): Publication => ({
  visibility: 'public',
  slug,
  showAuthor: true,
  published: true,
  canonicalUrl: null,
  snapshotId: 'snap',
  publishedRevision: 1,
  publishedAt: '2026-07-01T00:00:00Z',
  hasUnpublishedChanges: false,
});

const sub = (partial: Partial<SubdomainInfo> & Pick<SubdomainInfo, 'status' | 'fullDomain'>): SubdomainInfo => ({
  id: 'sub_1',
  userId: 'usr_1',
  label: 'alice',
  appliedAt: '2026-07-01T00:00:00Z',
  ...partial,
});

describe('resolveWorkspaceHomeUrl', () => {
  it('returns root for system scope', () => {
    expect(resolveWorkspaceHomeUrl({
      scope: 'system',
      subdomain: sub({ status: 'approved', fullDomain: 'alice.nav.ax', customDomain: 'links.example.com' }),
      publication: published('alice'),
    })).toBe('/');
  });

  it('prefers custom CNAME over subdomain', () => {
    expect(resolveWorkspaceHomeUrl({
      scope: 'personal',
      subdomain: sub({ status: 'approved', fullDomain: 'alice.nav.ax', customDomain: 'links.example.com' }),
      publication: published('alice'),
    })).toBe('https://links.example.com');
  });

  it('uses approved subdomain when no CNAME', () => {
    expect(resolveWorkspaceHomeUrl({
      scope: 'personal',
      subdomain: sub({ status: 'approved', fullDomain: 'alice.nav.ax' }),
      publication: published('alice'),
    })).toBe('https://alice.nav.ax');
  });

  it('ignores pending subdomain and falls back to slug', () => {
    expect(resolveWorkspaceHomeUrl({
      scope: 'personal',
      subdomain: sub({ status: 'pending', fullDomain: 'ab.nav.ax' }),
      publication: published('alice'),
    })).toBe('/u/alice');
  });

  it('falls back to root without publication slug', () => {
    expect(resolveWorkspaceHomeUrl({
      scope: 'personal',
      subdomain: null,
      publication: null,
    })).toBe('/');
  });
});

describe('isExternalHomeUrl', () => {
  it('detects absolute urls', () => {
    expect(isExternalHomeUrl('https://alice.nav.ax')).toBe(true);
    expect(isExternalHomeUrl('/u/alice')).toBe(false);
  });
});
