import type { Visibility } from '@/api/types';

export type PublishUiStateId =
  | 'never_published'
  | 'published_with_draft'
  | 'published_current';

export type PublishPrimaryAction = 'publish' | 'publish_update' | 'none';

export type PublishSurface = 'publish_page' | 'toolbar' | 'banner' | 'overview' | 'preview';

export interface PublicationLike {
  published: boolean;
  hasUnpublishedChanges: boolean;
  publishedAt: string | null;
  visibility: Visibility;
}

export interface PublishUiState {
  id: PublishUiStateId;
  shortLabel: string;
  primaryAction: PublishPrimaryAction;
  primaryLabel: string;
  primaryDisabled: boolean;
  showUnpublish: boolean;
  /** When true, clicking primary should navigate to publish page visibility section instead of calling API */
  requiresVisibilityFix: boolean;
  blockReason: string | null;
  publishedAt: string | null;
  draftUpdatedAt: string | null;
}

export function derivePublishUiState(
  publication: PublicationLike | null | undefined,
  draftUpdatedAt: string | null = null,
  options: { surface?: PublishSurface } = {},
): PublishUiState {
  const surface = options.surface ?? 'toolbar';
  const published = publication?.published ?? false;
  const hasDraft = publication?.hasUnpublishedChanges ?? false;
  const visibility = publication?.visibility ?? 'unlisted';
  const publishedAt = publication?.publishedAt ?? null;

  let id: PublishUiStateId;
  if (!published) id = 'never_published';
  else if (hasDraft) id = 'published_with_draft';
  else id = 'published_current';

  const base: Omit<PublishUiState, 'primaryDisabled' | 'requiresVisibilityFix' | 'blockReason'> = {
    id,
    shortLabel:
      id === 'never_published' ? '未发布' : id === 'published_with_draft' ? '有草稿未上线' : '已是最新',
    primaryAction:
      id === 'never_published' ? 'publish' : id === 'published_with_draft' ? 'publish_update' : 'none',
    primaryLabel:
      id === 'never_published' ? '发布' : id === 'published_with_draft' ? '发布更新' : '已是最新',
    showUnpublish: published,
    publishedAt,
    draftUpdatedAt,
  };

  const isPrivate = visibility === 'private';
  const tryingToPublish = base.primaryAction === 'publish' || base.primaryAction === 'publish_update';

  if (base.primaryAction === 'none') {
    return {
      ...base,
      primaryDisabled: true,
      requiresVisibilityFix: false,
      blockReason: null,
    };
  }

  if (isPrivate && tryingToPublish && surface === 'publish_page') {
    return {
      ...base,
      primaryDisabled: true,
      requiresVisibilityFix: true,
      blockReason: '请先将可见性改为「知道链接即可访问」或「公开展示」并保存设置',
    };
  }

  return {
    ...base,
    primaryDisabled: false,
    requiresVisibilityFix: isPrivate && tryingToPublish,
    blockReason: null,
  };
}

/** Toast after a draft-mutating save. Optional `override` replaces the whole message (e.g. themed prefix that already includes draft language). */
export function draftSaveToastMessage(
  publication: PublicationLike | null | undefined,
  override?: string,
): string {
  if (override) return override;
  if (publication?.published) return '已保存到草稿 · 访客仍看线上版';
  return '已保存到草稿';
}

export function publishSuccessToastMessage(stateId: PublishUiStateId): string {
  return stateId === 'published_with_draft' || stateId === 'published_current'
    ? '更新已发布'
    : '发布成功';
}
