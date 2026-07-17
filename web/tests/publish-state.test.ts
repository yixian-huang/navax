import { describe, expect, it } from 'vitest';
import {
  derivePublishUiState,
  draftSaveToastMessage,
  type PublicationLike,
} from '../src/lib/publish-state';

function pub(partial: Partial<PublicationLike>): PublicationLike {
  return {
    published: false,
    hasUnpublishedChanges: false,
    publishedAt: null,
    visibility: 'unlisted',
    ...partial,
  };
}

describe('derivePublishUiState', () => {
  it('maps never published', () => {
    const s = derivePublishUiState(pub({ published: false }), '2026-07-17T10:00:00Z');
    expect(s.id).toBe('never_published');
    expect(s.shortLabel).toBe('未发布');
    expect(s.primaryAction).toBe('publish');
    expect(s.primaryLabel).toBe('发布');
    expect(s.primaryDisabled).toBe(false);
    expect(s.showUnpublish).toBe(false);
  });

  it('maps published with draft changes', () => {
    const s = derivePublishUiState(
      pub({
        published: true,
        hasUnpublishedChanges: true,
        publishedAt: '2026-07-16T08:00:00Z',
      }),
      '2026-07-17T10:00:00Z',
    );
    expect(s.id).toBe('published_with_draft');
    expect(s.shortLabel).toBe('有草稿未上线');
    expect(s.primaryAction).toBe('publish_update');
    expect(s.primaryLabel).toBe('发布更新');
    expect(s.primaryDisabled).toBe(false);
    expect(s.showUnpublish).toBe(true);
  });

  it('maps published current', () => {
    const s = derivePublishUiState(
      pub({
        published: true,
        hasUnpublishedChanges: false,
        publishedAt: '2026-07-17T09:00:00Z',
      }),
      '2026-07-17T09:00:00Z',
    );
    expect(s.id).toBe('published_current');
    expect(s.shortLabel).toBe('已是最新');
    expect(s.primaryAction).toBe('none');
    expect(s.primaryLabel).toBe('已是最新');
    expect(s.primaryDisabled).toBe(true);
    expect(s.showUnpublish).toBe(true);
  });

  it('disables publish on publish page when visibility is private', () => {
    const s = derivePublishUiState(
      pub({ published: false, visibility: 'private' }),
      null,
      { surface: 'publish_page' },
    );
    expect(s.primaryDisabled).toBe(true);
    expect(s.blockReason).toMatch(/可见性/);
  });

  it('does not disable top-bar primary when private (navigates instead)', () => {
    const s = derivePublishUiState(
      pub({ published: false, visibility: 'private' }),
      null,
      { surface: 'toolbar' },
    );
    expect(s.primaryAction).toBe('publish');
    expect(s.primaryDisabled).toBe(false);
    expect(s.requiresVisibilityFix).toBe(true);
  });
});

describe('draftSaveToastMessage', () => {
  it('uses plain draft message when never published', () => {
    expect(draftSaveToastMessage(pub({ published: false }))).toBe('已保存到草稿');
  });

  it('warns visitors still see live when published', () => {
    expect(
      draftSaveToastMessage(pub({ published: true, hasUnpublishedChanges: true })),
    ).toBe('已保存到草稿 · 访客仍看线上版');
  });

  it('allows themed prefix', () => {
    expect(
      draftSaveToastMessage(pub({ published: false }), '主题已写入草稿：「石板」'),
    ).toBe('主题已写入草稿：「石板」');
  });
});
