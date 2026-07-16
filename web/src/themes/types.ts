// ============================================================
// Theme Package Type — the atomic unit of the theme system.
// Each package is self-contained: metadata + complete CSS.
// ============================================================

export interface ThemeMeta {
  name: string;
  subtitle: string;
  description: string;
  swatches: [string, string, string];
  vibe: 'serious' | 'cute';
}

export interface ThemePackage {
  /** Unique theme ID, used as data-theme attribute value */
  id: string;
  /** Display metadata for the picker UI */
  meta: ThemeMeta;
  /** Complete CSS for this theme — includes CSS custom properties
   *  (tokens) AND any visual override rules. Injected as a single
   *  <style> tag when the theme is activated. */
  css: string;
}