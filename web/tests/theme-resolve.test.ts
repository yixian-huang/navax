import { describe, expect, it } from 'vitest';
import { resolveThemeId, THEME_ID_ALIASES, RETAINED_THEME_IDS } from '@/lib/themeResolve';

describe('resolveThemeId', () => {
  it('keeps retained ids', () => {
    for (const id of RETAINED_THEME_IDS) {
      expect(resolveThemeId(id)).toBe(id);
    }
  });

  it('maps culled ids', () => {
    expect(resolveThemeId('kyoto')).toBe('slate');
    expect(resolveThemeId('terracotta')).toBe('slate');
    expect(resolveThemeId('mono')).toBe('slate');
    expect(resolveThemeId('mochi')).toBe('sakura');
    expect(resolveThemeId('pastelsky')).toBe('sakura');
    expect(resolveThemeId('cyber')).toBe('orbit');
  });

  it('falls back to slate for unknown', () => {
    expect(resolveThemeId('nope')).toBe('slate');
    expect(resolveThemeId('')).toBe('slate');
  });

  it('alias table matches design', () => {
    expect(THEME_ID_ALIASES).toEqual({
      kyoto: 'slate',
      terracotta: 'slate',
      mono: 'slate',
      mochi: 'sakura',
      pastelsky: 'sakura',
      cyber: 'orbit',
    });
  });
});
