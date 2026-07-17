import { useEffect, useState } from 'react';
import { analyzeWallpaperTone, type WallpaperTone } from '@/lib/wallpaperTone';

/**
 * Sample wallpaper + opacity → 'light' | 'dark' for free-floating text contrast.
 * Defaults to 'light' (dark ink) until analysis finishes.
 * Re-runs when opacity changes so lowering image strength switches ink correctly.
 */
export function useWallpaperTone(
  backgroundUrl: string | undefined,
  /** Wallpaper image strength 0–1 (same as appearance.background.opacity). */
  backgroundOpacity: number = 1,
): WallpaperTone | null {
  const [tone, setTone] = useState<WallpaperTone | null>(backgroundUrl ? 'light' : null);

  useEffect(() => {
    if (!backgroundUrl) {
      setTone(null);
      return;
    }

    let cancelled = false;
    const alpha = Math.min(1, Math.max(0, backgroundOpacity));
    // Optimistic default: dark ink (light tone) — correct for low opacity + most presets.
    setTone('light');

    void analyzeWallpaperTone(backgroundUrl, alpha).then(next => {
      if (!cancelled) setTone(next);
    });

    return () => {
      cancelled = true;
    };
  }, [backgroundUrl, backgroundOpacity]);

  return tone;
}
