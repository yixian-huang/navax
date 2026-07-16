import { useCallback, useEffect, useRef } from 'react';
import { analyticsApi } from '@/api/analytics';

function createClientEventId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

export function usePublicEventTracker(pageId?: string, snapshotId?: string) {
  const recordedPageRef = useRef('');

  useEffect(() => {
    if (!pageId) return;
    const pageKey = `${pageId}:${snapshotId ?? ''}`;
    if (recordedPageRef.current === pageKey) return;
    recordedPageRef.current = pageKey;
    void analyticsApi.recordPublicEvent({
      type: 'page_view',
      pageId,
      snapshotId,
      clientEventId: createClientEventId(),
    }).catch(() => undefined);
  }, [pageId, snapshotId]);

  return useCallback((siteId: string) => {
    if (!pageId) return;
    void analyticsApi.recordPublicEvent({
      type: 'site_click',
      pageId,
      snapshotId,
      siteId,
      clientEventId: createClientEventId(),
    }).catch(() => undefined);
  }, [pageId, snapshotId]);
}
