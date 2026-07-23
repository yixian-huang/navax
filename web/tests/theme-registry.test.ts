import { afterEach, describe, expect, it, vi } from 'vitest';
import { themeRegistry } from '@/themes/registry';

const HREF = (id: string) => `/api/v1/public/themes/v${id}.css`;

function makeRoot(): HTMLElement {
  const root = document.createElement('div');
  root.setAttribute('data-nx', 'page-root');
  document.body.appendChild(root);
  return root;
}

function linkFor(id: string): HTMLLinkElement | null {
  return document.head.querySelector<HTMLLinkElement>(`link[data-theme-style="${id}"]`);
}

function fire(id: string, type: 'load' | 'error'): void {
  const link = linkFor(id);
  if (!link) throw new Error(`no pending <link> for theme "${id}"`);
  link.dispatchEvent(new Event(type));
}

afterEach(() => {
  document.head.querySelectorAll('link[data-theme-style]').forEach(el => el.remove());
  document.body.innerHTML = '';
  vi.useRealTimers();
});

describe('themeRegistry.activate', () => {
  it('discards stale callbacks so a slow request cannot beat a newer choice', async () => {
    const root = makeRoot();
    const slow = themeRegistry.activate('alpha', HREF('alpha'), root);
    const fresh = themeRegistry.activate('beta', HREF('beta'), root);

    // 慢请求最终完成，但它已经被 beta 取代——不得改写 data-theme。
    expect(await slow).toBe('superseded');
    fire('beta', 'load');
    expect(await fresh).toBe('applied');
    expect(root.dataset.theme).toBe('beta');
    expect(linkFor('alpha')).toBeNull();
  });

  it('keeps the old stylesheet until the new one has loaded', async () => {
    const root = makeRoot();
    const first = themeRegistry.activate('one', HREF('one'), root);
    fire('one', 'load');
    expect(await first).toBe('applied');

    const second = themeRegistry.activate('two', HREF('two'), root);
    // 新样式表在途：旧 link 必须还在，否则中间这一帧是裸样式。
    expect(linkFor('one')).not.toBeNull();
    expect(root.dataset.theme).toBe('one');

    fire('two', 'load');
    expect(await second).toBe('applied');
    expect(linkFor('one')).toBeNull();
    expect(root.dataset.theme).toBe('two');
  });

  it('keeps the current theme when the new stylesheet errors (410 included)', async () => {
    const root = makeRoot();
    const first = themeRegistry.activate('keep', HREF('keep'), root);
    fire('keep', 'load');
    await first;

    const revoked = themeRegistry.activate('revoked', HREF('revoked'), root);
    fire('revoked', 'error');

    expect(await revoked).toBe('failed');
    expect(linkFor('revoked')).toBeNull();
    expect(root.dataset.theme).toBe('keep');
  });

  it('fails after the load timeout instead of waiting forever', async () => {
    vi.useFakeTimers();
    const root = makeRoot();
    const pending = themeRegistry.activate('hang', HREF('hang'), root);
    vi.advanceTimersByTime(5000);

    expect(await pending).toBe('failed');
    expect(linkFor('hang')).toBeNull();
  });

  it('falls back to the baseline tokens when the very first load fails', async () => {
    const root = makeRoot();
    const first = themeRegistry.activate('broken', HREF('broken'), root);
    fire('broken', 'error');

    expect(await first).toBe('failed');
    // 没有任何主题可保留：不留下指向不存在样式的 data-theme。
    expect(root.dataset.theme).toBeUndefined();
  });

  it('scopes data-theme to the given root, never to <html>', async () => {
    const root = makeRoot();
    const applied = themeRegistry.activate('scoped', HREF('scoped'), root);
    fire('scoped', 'load');
    await applied;

    expect(root.dataset.theme).toBe('scoped');
    expect(document.documentElement.hasAttribute('data-theme')).toBe(false);
    expect(themeRegistry.getActive(root)).toBe('scoped');

    themeRegistry.deactivate(root);
    expect(root.hasAttribute('data-theme')).toBe(false);
    expect(linkFor('scoped')).toBeNull();
    expect(themeRegistry.getActive(root)).toBeNull();
  });

  it('keeps separate roots independent', async () => {
    const publicRoot = makeRoot();
    const previewRoot = makeRoot();

    const a = themeRegistry.activate('rootA', HREF('rootA'), publicRoot);
    fire('rootA', 'load');
    await a;
    const b = themeRegistry.activate('rootB', HREF('rootB'), previewRoot);
    fire('rootB', 'load');
    await b;

    expect(publicRoot.dataset.theme).toBe('rootA');
    expect(previewRoot.dataset.theme).toBe('rootB');
  });
});
