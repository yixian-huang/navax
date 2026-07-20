/** Themes still shipped as CSS packages. */
export const RETAINED_THEME_IDS = [
  'slate',
  'slate-dark',
  'sakura',
  'noir',
  'orbit',
  'terminal',
] as const;

export type RetainedThemeId = (typeof RETAINED_THEME_IDS)[number];

/** Culled package ids → closest retained theme. */
export const THEME_ID_ALIASES: Record<string, RetainedThemeId> = {
  kyoto: 'slate',
  terracotta: 'slate',
  mono: 'slate',
  mochi: 'sakura',
  pastelsky: 'sakura',
  cyber: 'orbit',
};

const DEFAULT_THEME: RetainedThemeId = 'slate';

export function resolveThemeId(themeId: string | null | undefined): RetainedThemeId {
  const raw = (themeId || '').trim();
  if ((RETAINED_THEME_IDS as readonly string[]).includes(raw)) {
    return raw as RetainedThemeId;
  }
  if (raw in THEME_ID_ALIASES) {
    return THEME_ID_ALIASES[raw];
  }
  return DEFAULT_THEME;
}
