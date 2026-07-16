// ============================================================
// nav.ax Theme Manifest — bridge between external theme format
// and the internal ThemePackage runtime representation.
//
// ThemeManifest is the format for installable third-party themes
// (e.g. loaded from a CDN or uploaded as JSON).
//
// Use manifestToPackage() to convert a manifest into a runnable
// ThemePackage that can be registered with the ThemeRegistry.
// ============================================================

import type { ThemePackage, ThemeMeta } from '@/themes/types';

export interface ThemeManifest {
  /** Unique theme ID (becomes data-theme attribute) */
  id: string;
  /** Display name */
  name: string;
  /** Short subtitle in preferred language */
  subtitle: string;
  /** Longer description for the picker UI */
  description: string;
  /** Semantic version */
  version: string;
  /** Theme author / publisher */
  author: string;
  /** Light, dark, or both */
  mode: 'light' | 'dark' | 'both';
  /** Three hex swatches for the picker preview */
  swatches: [string, string, string];
  /** Serious or cute — determines grouping in picker */
  vibe: 'serious' | 'cute';
  /** The complete CSS for this theme — will be injected as <style> */
  css: string;
  /** Optional: URL to a preview image */
  preview?: string;
  /** License identifier */
  license?: string;
  /** Author's homepage */
  homepage?: string;
}

/**
 * Convert an external ThemeManifest into an internal ThemePackage.
 * Validates required fields; returns null if invalid.
 */
export function manifestToPackage(manifest: ThemeManifest): ThemePackage | null {
  if (!manifest.id || !manifest.name || !manifest.css) {
    console.warn('[ThemeManifest] Invalid manifest: missing id, name, or css.', manifest);
    return null;
  }

  const meta: ThemeMeta = {
    name: manifest.name,
    subtitle: manifest.subtitle || '',
    description: manifest.description || '',
    swatches: manifest.swatches || ['#ccc', '#888', '#444'],
    vibe: manifest.vibe || 'serious',
  };

  return {
    id: manifest.id,
    meta,
    css: manifest.css,
  };
}
