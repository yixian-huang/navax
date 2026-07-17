/**
 * Ad-hoc Playwright smoke: public home site-card UX over wallpaper.
 * Checks comfortable layout, list (no frost), favicon plate, description contrast.
 */
import { chromium } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';

const BASE = process.env.NAVAX_BASE_URL || 'http://localhost:3000';
const OUT_DIR = path.resolve('test-results/smoke-site-card');
fs.mkdirSync(OUT_DIR, { recursive: true });

const browser = await chromium.launch({ headless: true });
const context = await browser.newContext({
  viewport: { width: 1280, height: 900 },
  locale: 'zh-CN',
});
const page = await context.newPage();
const findings = [];
const ok = (name, pass, detail = '') => {
  findings.push({ name, pass, detail });
  console.log(`${pass ? 'PASS' : 'FAIL'}  ${name}${detail ? ` — ${detail}` : ''}`);
};

async function cardMetrics(selector) {
  return page.evaluate((sel) => {
    const cards = [...document.querySelectorAll(sel)];
    return cards.slice(0, 6).map((el, index) => {
      const cs = getComputedStyle(el);
      const fav = el.querySelector('.site-card-favicon');
      const favCs = fav ? getComputedStyle(fav) : null;
      const title = el.querySelector('.site-card-title, h3');
      const domain = el.querySelector('.site-card-domain, .site-card-list-domain');
      const desc = el.querySelector('.site-card-desc, .site-card-list-desc');
      const img = el.querySelector('img');
      const titleCs = title ? getComputedStyle(title) : null;
      const descCs = desc ? getComputedStyle(desc) : null;
      const domainCs = domain ? getComputedStyle(domain) : null;

      const parseRgb = (c) => {
        const m = String(c).match(/rgba?\(([\d.]+),\s*([\d.]+),\s*([\d.]+)(?:,\s*([\d.]+))?\)/);
        if (!m) return null;
        return { r: +m[1], g: +m[2], b: +m[3], a: m[4] === undefined ? 1 : +m[4] };
      };
      const relLuma = (rgb) => {
        if (!rgb) return null;
        const f = (v) => {
          v /= 255;
          return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
        };
        return 0.2126 * f(rgb.r) + 0.7152 * f(rgb.g) + 0.0722 * f(rgb.b);
      };
      const contrast = (fg, bg) => {
        const L1 = relLuma(fg);
        const L2 = relLuma(bg);
        if (L1 == null || L2 == null) return null;
        const hi = Math.max(L1, L2);
        const lo = Math.min(L1, L2);
        return (hi + 0.05) / (lo + 0.05);
      };

      // Approximate card/page background for contrast (backdrop not fully captured).
      const bgRgb = parseRgb(cs.backgroundColor) || parseRgb(getComputedStyle(document.body).backgroundColor);
      const descRgb = descCs ? parseRgb(descCs.color) : null;
      const titleRgb = titleCs ? parseRgb(titleCs.color) : null;

      const rect = el.getBoundingClientRect();
      const imgRect = img?.getBoundingClientRect();
      const titleRect = title?.getBoundingClientRect();
      const iconBeside =
        imgRect && titleRect
          ? Math.abs(imgRect.top - titleRect.top) < 28 && imgRect.right <= titleRect.left + 8
          : null;

      return {
        index,
        className: el.className,
        text: (title?.textContent || '').trim().slice(0, 40),
        hasDesc: Boolean(desc && (desc.textContent || '').trim()),
        descText: (desc?.textContent || '').trim().slice(0, 50),
        backdropFilter: cs.backdropFilter || cs.webkitBackdropFilter || 'none',
        backgroundColor: cs.backgroundColor,
        faviconBg: favCs?.backgroundColor || null,
        faviconFilter: favCs ? favCs.backdropFilter || favCs.webkitBackdropFilter || 'none' : null,
        titleColor: titleCs?.color || null,
        descColor: descCs?.color || null,
        domainColor: domainCs?.color || null,
        descContrastVsCardBg: contrast(descRgb, bgRgb),
        titleContrastVsCardBg: contrast(titleRgb, bgRgb),
        iconBesideTitle: iconBeside,
        width: Math.round(rect.width),
        height: Math.round(rect.height),
      };
    });
  }, selector);
}

function isTransparentBg(bg) {
  if (!bg) return true;
  if (bg === 'transparent' || bg === 'rgba(0, 0, 0, 0)') return true;
  const m = bg.match(/rgba?\(([\d.]+),\s*([\d.]+),\s*([\d.]+)(?:,\s*([\d.]+))?\)/);
  if (m && (m[4] === undefined ? 1 : +m[4]) < 0.05) return true;
  return false;
}

function hasBlur(filter) {
  return Boolean(filter && filter !== 'none' && /blur\(/i.test(filter));
}

try {
  console.log('>> open', BASE + '/');
  await page.goto(BASE + '/', { waitUntil: 'networkidle', timeout: 45000 });
  await page.waitForTimeout(1500);

  const shell = await page.evaluate(() => {
    const root = document.querySelector('[data-wallpaper]');
    return {
      wallpaper: root?.getAttribute('data-wallpaper') || null,
      tone: root?.getAttribute('data-wallpaper-tone') || null,
      hasCards: document.querySelectorAll('.material-card').length,
      hasList: document.querySelectorAll('.site-card-list').length,
    };
  });
  console.log('>> shell', shell);
  ok('wallpaper active', shell.wallpaper === 'true', `tone=${shell.tone}`);

  // --- Comfortable density ---
  // Click density switcher if present to force comfortable
  const densityBtn = page.locator('button[aria-label*="舒适"], button:has-text("舒适")').first();
  if (await densityBtn.count()) {
    await densityBtn.click().catch(() => {});
    await page.waitForTimeout(400);
  }

  await page.screenshot({ path: path.join(OUT_DIR, 'comfortable.png'), fullPage: true });
  const comfortable = await cardMetrics('.material-card.site-card-comfortable, a.material-card');
  // Prefer comfortable class if available
  const comfortCards = comfortable.filter(c => String(c.className).includes('site-card-comfortable'));
  const cardsC = comfortCards.length ? comfortCards : comfortable;
  console.log('>> comfortable sample', JSON.stringify(cardsC.slice(0, 2), null, 2));

  ok(
    'comfortable cards present',
    cardsC.length > 0,
    `n=${cardsC.length}`,
  );

  const sideBySide = cardsC.filter(c => c.iconBesideTitle === true);
  ok(
    'comfortable: icon beside title (not own row)',
    sideBySide.length >= Math.min(2, cardsC.length),
    `ok=${sideBySide.length}/${cardsC.length}`,
  );

  const favPlate = cardsC.filter(c => c.faviconBg && !isTransparentBg(c.faviconBg));
  ok(
    'comfortable: favicon no extra plate',
    favPlate.length === 0,
    favPlate.length ? `non-transparent=${JSON.stringify(favPlate.map(f => f.faviconBg))}` : 'transparent',
  );

  const withDesc = cardsC.filter(c => c.hasDesc);
  ok('comfortable: descriptions rendered', withDesc.length > 0, `withDesc=${withDesc.length}`);

  // On frosted white card, description should be relatively dark (low oklch L or rgb luma)
  const descColors = withDesc.map(c => c.descColor);
  const darkInk = withDesc.filter(c => {
    const oklch = String(c.descColor).match(/oklch\(\s*([\d.]+)/i);
    if (oklch) return +oklch[1] < 0.55;
    const m = String(c.descColor).match(/rgba?\(([\d.]+),\s*([\d.]+),\s*([\d.]+)/);
    if (!m) return false;
    const luma = (0.2126 * +m[1] + 0.7152 * +m[2] + 0.0722 * +m[3]) / 255;
    return luma < 0.55;
  });
  ok(
    'comfortable: description uses dark ink on frosted panel',
    darkInk.length >= Math.min(1, withDesc.length),
    `darkInk=${darkInk.length}/${withDesc.length} samples=${descColors.slice(0, 2).join(' | ')}`,
  );

  // Card frost expected for comfortable
  const cardBlur = cardsC.filter(c => hasBlur(c.backdropFilter));
  ok(
    'comfortable: cards keep soft frost (expected)',
    cardBlur.length > 0 || cardsC.some(c => !isTransparentBg(c.backgroundColor)),
    `blurred=${cardBlur.length}`,
  );

  // --- List density ---
  const listBtn = page.locator('button[aria-label*="列表"], button:has-text("列表")').first();
  // DensitySwitcher may use icons only
  const densityButtons = page.locator('[class*="Density"], [aria-label*="密度"], button[title*="列表"]');
  // Try clicking all density toggle candidates that mention list
  const listToggle = page.locator('button').filter({ hasText: /列表|List/i });
  if (await listToggle.count()) {
    await listToggle.first().click();
  } else {
    // Fallback: density switcher icons — third button often list
    const switcher = page.locator('button[aria-label*="紧凑"], button[aria-label*="舒适"], button[aria-label*="列表"]');
    const n = await switcher.count();
    for (let i = 0; i < n; i++) {
      const label = await switcher.nth(i).getAttribute('aria-label');
      if (label && /列表|list/i.test(label)) {
        await switcher.nth(i).click();
        break;
      }
    }
    // Also try data attributes / title
    const byTitle = page.locator('button[title*="列表"], button[aria-label="list"], button[data-density="list"]');
    if (await byTitle.count()) await byTitle.first().click();
  }
  await page.waitForTimeout(600);

  // Force list via evaluate if UI control missed
  const listCount = await page.locator('.site-card-list').count();
  if (listCount === 0) {
    // click density icons in toolbar — inspect structure
    const densityInfo = await page.evaluate(() => {
      const buttons = [...document.querySelectorAll('button')].filter(b => {
        const t = `${b.getAttribute('aria-label') || ''} ${b.getAttribute('title') || ''} ${b.textContent || ''}`;
        return /密度|紧凑|舒适|列表|compact|comfortable|list|density/i.test(t);
      });
      return buttons.map(b => ({
        label: b.getAttribute('aria-label'),
        title: b.getAttribute('title'),
        text: (b.textContent || '').trim().slice(0, 20),
      }));
    });
    console.log('>> density controls', densityInfo);
    for (const btn of densityInfo) {
      if (/列表|list/i.test(`${btn.label} ${btn.title} ${btn.text}`)) {
        await page.locator(`button[aria-label="${btn.label}"]`).first().click().catch(() => {});
      }
    }
    await page.waitForTimeout(500);
  }

  // Last resort: set density in React is hard; click by role from DensitySwitcher source
  if ((await page.locator('.site-card-list').count()) === 0) {
    await page.evaluate(() => {
      // Dispatch click on buttons containing density icons near SiteGrid
      const candidates = [...document.querySelectorAll('button')];
      const hit = candidates.find(b => (b.getAttribute('aria-label') || '').includes('列表'));
      hit?.click();
    });
    await page.waitForTimeout(500);
  }

  await page.screenshot({ path: path.join(OUT_DIR, 'list.png'), fullPage: true });
  const listCards = await cardMetrics('.site-card-list');
  console.log('>> list sample', JSON.stringify(listCards.slice(0, 2), null, 2));
  ok('list cards present', listCards.length > 0, `n=${listCards.length}`);

  const panelMetrics = await page.evaluate(() => {
    const panel = document.querySelector('.site-card-list-panel');
    if (!panel) return null;
    const cs = getComputedStyle(panel);
    return {
      backdropFilter: cs.backdropFilter || cs.webkitBackdropFilter || 'none',
      backgroundColor: cs.backgroundColor,
      className: panel.className,
    };
  });
  console.log('>> list panel', panelMetrics);
  ok(
    'list panel is not material-card',
    panelMetrics && !String(panelMetrics.className).includes('material-card'),
    panelMetrics?.className || 'missing panel',
  );
  ok(
    'list panel: no backdrop blur',
    panelMetrics && !hasBlur(panelMetrics.backdropFilter),
    panelMetrics?.backdropFilter || 'n/a',
  );
  ok(
    'list panel: transparent on wallpaper',
    panelMetrics && isTransparentBg(panelMetrics.backgroundColor),
    panelMetrics?.backgroundColor || 'n/a',
  );

  const listBlur = listCards.filter(c => hasBlur(c.backdropFilter));
  ok(
    'list rows: no backdrop blur / frosted glass',
    listBlur.length === 0,
    listBlur.length ? `blurred=${listBlur.length} filter=${listBlur[0]?.backdropFilter}` : 'none',
  );

  // Titles should use light ink on dark wallpaper tone
  const tone = shell.tone;
  if (tone === 'dark' && listCards.length) {
    const lightTitles = listCards.filter(c => {
      const oklch = String(c.titleColor).match(/oklch\(\s*([\d.]+)/i);
      return oklch && +oklch[1] > 0.7;
    });
    ok(
      'list: title uses light ink on dark wallpaper',
      lightTitles.length >= Math.min(1, listCards.length),
      `light=${lightTitles.length}/${listCards.length} sample=${listCards[0]?.titleColor}`,
    );
  }

  // List background should be transparent or very low alpha (hover may raise slightly — check unhovered)
  const listOpaque = listCards.filter(c => {
    const m = String(c.backgroundColor).match(/rgba?\(([\d.]+),\s*([\d.]+),\s*([\d.]+)(?:,\s*([\d.]+))?\)/);
    if (!m) return !isTransparentBg(c.backgroundColor);
    const a = m[4] === undefined ? 1 : +m[4];
    return a > 0.2; // >20% opaque plate is unwanted for list
  });
  ok(
    'list: no solid frosted plate background',
    listOpaque.length === 0,
    listOpaque.length ? `opaque=${listOpaque.map(c => c.backgroundColor).join('; ')}` : 'transparent/low-alpha',
  );

  const listFavPlate = listCards.filter(c => c.faviconBg && !isTransparentBg(c.faviconBg));
  ok(
    'list: favicon no extra plate',
    listFavPlate.length === 0,
    listFavPlate.length ? listFavPlate.map(c => c.faviconBg).join('; ') : 'ok',
  );

  const listDesc = listCards.filter(c => c.hasDesc);
  ok('list: descriptions shown when present', listDesc.length > 0 || listCards.length === 0, `withDesc=${listDesc.length}`);

  // Summary
  const failed = findings.filter(f => !f.pass);
  const report = {
    base: BASE,
    shell,
    comfortable: cardsC.slice(0, 3),
    list: listCards.slice(0, 3),
    findings,
    failed: failed.length,
    screenshots: {
      comfortable: path.join(OUT_DIR, 'comfortable.png'),
      list: path.join(OUT_DIR, 'list.png'),
    },
  };
  fs.writeFileSync(path.join(OUT_DIR, 'report.json'), JSON.stringify(report, null, 2));
  console.log('\n== SUMMARY ==');
  console.log(`passed ${findings.length - failed.length}/${findings.length}`);
  if (failed.length) {
    console.log('failures:');
    for (const f of failed) console.log(' -', f.name, f.detail);
  }
  console.log('report', path.join(OUT_DIR, 'report.json'));
  process.exitCode = failed.length ? 1 : 0;
} catch (e) {
  console.error('ERROR', e);
  await page.screenshot({ path: path.join(OUT_DIR, 'error.png'), fullPage: true }).catch(() => {});
  process.exitCode = 1;
} finally {
  await browser.close();
}
