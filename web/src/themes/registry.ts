// ============================================================
// ThemeRegistry — singleton that manages theme packages.
//
// Responsibilities:
//   1. Register / unregister theme packages
//   2. Activate a theme (inject CSS + set data-theme on <html>)
//   3. Deactivate current theme (remove CSS + attribute)
//   4. List available themes for the picker UI
//
// Usage:
//   import { themeRegistry } from '@/themes/registry';
//   themeRegistry.register(myTheme);
//   themeRegistry.activate('slate');
// ============================================================

import type { ThemePackage } from './types';

class ThemeRegistry {
  private packages = new Map<string, ThemePackage>();
  private activeId: string | null = null;
  private styleEl: HTMLStyleElement | null = null;

  /** Register a single theme package */
  register(pkg: ThemePackage): void {
    if (this.packages.has(pkg.id)) {
      console.warn(`[ThemeRegistry] Theme "${pkg.id}" is already registered — overwriting.`);
    }
    this.packages.set(pkg.id, pkg);
  }

  /** Register multiple theme packages at once */
  registerAll(pkgs: ThemePackage[]): void {
    pkgs.forEach(p => this.register(p));
  }

  /**
   * Activate a theme by ID.
   * - Removes any previously injected <style> tag
   * - Injects the new theme's CSS
   * - Sets data-theme attribute on <html>
   */
  activate(id: string): void {
    if (this.activeId === id) return;

    const pkg = this.packages.get(id);
    if (!pkg) {
      console.warn(`[ThemeRegistry] Theme "${id}" not found. Available: ${this.listIds().join(', ')}`);
      return;
    }

    // Remove old style element
    this.removeStyle();

    // Inject new style
    if (typeof document !== 'undefined') {
      this.styleEl = document.createElement('style');
      this.styleEl.setAttribute('data-theme-style', id);
      this.styleEl.textContent = pkg.css;
      document.head.appendChild(this.styleEl);

      // Set data-theme on <html> so CSS selectors like
      // [data-theme="xxx"] body::before work correctly
      document.documentElement.setAttribute('data-theme', id);
    }

    this.activeId = id;
  }

  /** Deactivate current theme — removes CSS and attribute */
  deactivate(): void {
    this.removeStyle();
    if (typeof document !== 'undefined') {
      document.documentElement.removeAttribute('data-theme');
    }
    this.activeId = null;
  }

  /** Get the currently active theme ID */
  getActive(): string | null {
    return this.activeId;
  }

  /** List all registered theme packages */
  list(): ThemePackage[] {
    return Array.from(this.packages.values());
  }

  /** Get a specific theme package by ID */
  get(id: string): ThemePackage | undefined {
    return this.packages.get(id);
  }

  /** Check if a theme ID is registered */
  has(id: string): boolean {
    return this.packages.has(id);
  }

  /** Unregister a theme. If it's active, deactivates first. */
  remove(id: string): void {
    if (this.activeId === id) {
      this.deactivate();
    }
    this.packages.delete(id);
  }

  /** List all registered theme IDs (for debugging) */
  private listIds(): string[] {
    return Array.from(this.packages.keys());
  }

  /** Remove injected style element and data-theme attribute */
  private removeStyle(): void {
    if (this.styleEl) {
      this.styleEl.remove();
      this.styleEl = null;
    }
  }
}

/** Singleton instance — import this everywhere */
export const themeRegistry = new ThemeRegistry();