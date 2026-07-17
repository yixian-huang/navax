import { useMemo } from 'react';
import type { NavigationPage, PageKind, Publication } from '@/api/types';
import { useMyPage, usePageScope } from '@/hooks/useQueries';
import {
  derivePublishUiState,
  type PublishSurface,
  type PublishUiState,
} from '@/lib/publish-state';

export interface UsePublishUiStateResult {
  state: PublishUiState;
  pageId: string | undefined;
  slug: string;
  scope: PageKind;
  isLoading: boolean;
  isError: boolean;
  publication: Publication | undefined;
  page: NavigationPage | undefined;
  refetch: ReturnType<typeof useMyPage>['refetch'];
}

export function usePublishUiState(
  surface: PublishSurface = 'toolbar',
): UsePublishUiStateResult {
  const scope = usePageScope();
  const query = useMyPage(scope);
  const publication = query.data?.publication;

  const state = useMemo(
    () =>
      derivePublishUiState(publication, query.data?.draftUpdatedAt ?? null, { surface }),
    [publication, query.data?.draftUpdatedAt, surface],
  );

  return {
    state,
    pageId: query.data?.id,
    slug: publication?.slug ?? '',
    scope,
    isLoading: query.isLoading,
    isError: query.isError,
    publication,
    page: query.data,
    refetch: query.refetch,
  };
}
