// ============================================================
// Wallpaper tone analysis — pick free-floating text contrast
// from average image luminance (canvas sample), blended with
// page base color by the configured wallpaper opacity.
// ============================================================

/** light = bright surface → dark ink; dark = dim surface → light ink */
export type WallpaperTone = 'light' | 'dark';

const SAMPLE = 40;
/** Relative luminance threshold (0–1). At/above → treat as light surface. */
const LIGHT_THRESHOLD = 0.52;
/**
 * Assumed page base luminance under the photo (theme background-100-ish).
 * When wallpaper opacity drops, more of this base shows through.
 */
const DEFAULT_BASE_LUMINANCE = 0.94;

/**
 * Average relative luminance of an image URL (0–1).
 * Returns null when the canvas is tainted (CORS) or load fails.
 */
export async function sampleImageLuminance(url: string): Promise<number | null> {
  if (!url || typeof document === 'undefined') return null;

  try {
    const img = await loadImage(url);
    const canvas = document.createElement('canvas');
    canvas.width = SAMPLE;
    canvas.height = SAMPLE;
    const ctx = canvas.getContext('2d', { willReadFrequently: true });
    if (!ctx) return null;

    // object-cover center crop approximation
    const scale = Math.max(SAMPLE / img.naturalWidth, SAMPLE / img.naturalHeight);
    const w = img.naturalWidth * scale;
    const h = img.naturalHeight * scale;
    const dx = (SAMPLE - w) / 2;
    const dy = (SAMPLE - h) / 2;
    ctx.drawImage(img, dx, dy, w, h);

    let data: ImageData;
    try {
      data = ctx.getImageData(0, 0, SAMPLE, SAMPLE);
    } catch {
      return null;
    }

    let sum = 0;
    let weight = 0;
    const cx = (SAMPLE - 1) / 2;
    const cy = (SAMPLE - 1) / 2;
    const pixels = data.data;
    for (let y = 0; y < SAMPLE; y++) {
      for (let x = 0; x < SAMPLE; x++) {
        const i = (y * SAMPLE + x) * 4;
        const r = pixels[i] / 255;
        const g = pixels[i + 1] / 255;
        const b = pixels[i + 2] / 255;
        const L = 0.2126 * r + 0.7152 * g + 0.0722 * b;
        const dist = Math.hypot(x - cx, y - cy) / (SAMPLE * 0.5);
        const wgt = 1.35 - Math.min(1, dist);
        sum += L * wgt;
        weight += wgt;
      }
    }
    if (weight <= 0) return null;
    return sum / weight;
  } catch {
    return null;
  }
}

/**
 * Blend photo luminance with the page base by wallpaper opacity.
 * Low opacity ⇒ base dominates ⇒ usually "light" surface ⇒ dark ink.
 */
export function effectiveSurfaceLuminance(
  imageLuminance: number | null,
  imageOpacity: number,
  baseLuminance: number = DEFAULT_BASE_LUMINANCE,
): number {
  const alpha = Math.min(1, Math.max(0, imageOpacity));
  const photo = imageLuminance == null || Number.isNaN(imageLuminance)
    ? baseLuminance
    : imageLuminance;
  return photo * alpha + baseLuminance * (1 - alpha);
}

export function toneFromLuminance(luminance: number | null, fallback: WallpaperTone = 'light'): WallpaperTone {
  if (luminance == null || Number.isNaN(luminance)) return fallback;
  return luminance >= LIGHT_THRESHOLD ? 'light' : 'dark';
}

export async function analyzeWallpaperTone(
  url: string,
  imageOpacity: number = 1,
  baseLuminance: number = DEFAULT_BASE_LUMINANCE,
): Promise<WallpaperTone> {
  const imageL = await sampleImageLuminance(url);
  const surfaceL = effectiveSurfaceLuminance(imageL, imageOpacity, baseLuminance);
  // Prefer dark ink when unknown — most bases and presets are light.
  return toneFromLuminance(surfaceL, 'light');
}

function loadImage(url: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.crossOrigin = 'anonymous';
    img.referrerPolicy = 'no-referrer';
    img.decoding = 'async';
    img.onload = () => resolve(img);
    img.onerror = () => reject(new Error('wallpaper image load failed'));
    img.src = url;
  });
}
