// ============================================================
// ThemeRegistry — atomic theme switching against a theme root.
//
// The registry no longer owns CSS. It owns the <link> lifecycle:
// which stylesheet is currently applied to which theme root, and
// how to move from one to the next without a flash of unstyled or
// half-styled content.
//
// Usage:
//   import { themeRegistry } from '@/themes/registry';
//   await themeRegistry.activate('slate', '/api/v1/public/themes/v….css', rootEl);
//   themeRegistry.deactivate(rootEl);
// ============================================================

/**
 * `applied` 样式已生效；`failed` 加载失败或超时，当前主题不变；
 * `superseded` 这次切换在落地前被更新的选择取代（不是错误，调用方不该报错）。
 */
export type ThemeSwitchResult = 'applied' | 'failed' | 'superseded';

/** 加载超时上限。超过它就认定失败，而不是让页面无限期停在旧主题上等一个可能永不到达的响应。 */
const LOAD_TIMEOUT_MS = 5000;

/**
 * 模块级自增序号，全局单调。每次切换捕获一个值，`load`/`error` 回调回来时先比对：
 * 对不上说明用户已经选了别的主题，这个回调必须丢弃——否则一个慢请求会在它终于
 * 完成时覆盖掉用户更新的选择（快速连点主题时必然发生）。
 */
let switchSeq = 0;

interface PendingSwitch {
  link: HTMLLinkElement;
  timer: ReturnType<typeof setTimeout>;
  settle: (result: ThemeSwitchResult) => void;
}

interface RootState {
  /** 该 root 上最后一次发起的切换序号。 */
  latestSeq: number;
  appliedLink: HTMLLinkElement | null;
  appliedId: string | null;
  appliedHref: string | null;
  pending: PendingSwitch | null;
}

class ThemeRegistry {
  // 状态按主题根分别记录：同一页可以有多个主题根（公开页 vs. /app/themes 的预览容器），
  // 它们的当前主题互不相干。WeakMap 让根元素卸载后状态自动回收。
  private states = new WeakMap<HTMLElement, RootState>();

  /**
   * 把 `id` 对应的样式表挂到 `root` 上。
   *
   * 切换是原子的：新样式表**加载成功之后**才移除旧的并改写 `data-theme`，
   * 因此中间不存在"旧的已经没了、新的还没到"的裸样式窗口。
   */
  activate(id: string, cssHref: string, root: HTMLElement): Promise<ThemeSwitchResult> {
    if (typeof document === 'undefined') return Promise.resolve('failed');

    const state = this.stateFor(root);

    // 序号先递增再做任何判断：即使这次是重复点击（no-op），也必须让在途回调作废，
    // 否则"切到 A → 切回当前主题"时，A 的慢响应回来还会把 data-theme 改成 A。
    const seq = ++switchSeq;
    state.latestSeq = seq;
    this.abandonPending(state);

    if (state.appliedId === id && state.appliedHref === cssHref) {
      // 幂等：root 重挂后属性可能丢了，补回来即可，不必重新下载样式表。
      root.dataset.theme = id;
      return Promise.resolve('applied');
    }

    const link = document.createElement('link');
    link.rel = 'stylesheet';
    link.href = cssHref;
    // E2E 与调试靠这个属性定位当前主题样式表。
    link.dataset.themeStyle = id;

    let settle!: (result: ThemeSwitchResult) => void;
    const done = new Promise<ThemeSwitchResult>(resolve => { settle = resolve; });

    const finish = (loaded: boolean) => {
      // 过期回调：直接丢弃，连 DOM 都不要碰。对应的 link 在被取代时已经移除。
      if (seq !== state.latestSeq) return;
      const pending = state.pending;
      if (!pending || pending.link !== link) return;

      state.pending = null;
      clearTimeout(pending.timer);
      link.removeEventListener('load', onLoad);
      link.removeEventListener('error', onError);

      if (loaded) {
        // 先加载后替换：到这一刻新样式表已经可用，移除旧 link 不会露出无主题状态。
        state.appliedLink?.remove();
        state.appliedLink = link;
        state.appliedId = id;
        state.appliedHref = cssHref;
        root.dataset.theme = id;
        pending.settle('applied');
        return;
      }

      // 失败（含被撤销版本的 410，浏览器同样触发 error）与超时走同一条路：
      // 丢掉这张失败的样式表，当前主题原样保留，由调用方决定怎么提示用户。
      link.remove();
      if (!state.appliedId) {
        // 首次加载就失败：没有"当前主题"可保留，显式回落到主 CSS 里的基线令牌，
        // 免得留下一个指向不存在样式的 data-theme。
        delete root.dataset.theme;
      }
      pending.settle('failed');
    };

    const onLoad = () => finish(true);
    const onError = () => finish(false);

    link.addEventListener('load', onLoad);
    link.addEventListener('error', onError);

    state.pending = {
      link,
      timer: setTimeout(() => finish(false), LOAD_TIMEOUT_MS),
      settle,
    };

    document.head.appendChild(link);
    return done;
  }

  /** 撤下 `root` 上的主题：移除样式表与 `data-theme`，并作废在途切换。 */
  deactivate(root: HTMLElement): void {
    const state = this.states.get(root);
    if (!state) return;

    state.latestSeq = ++switchSeq;
    this.abandonPending(state);
    state.appliedLink?.remove();
    state.appliedLink = null;
    state.appliedId = null;
    state.appliedHref = null;
    delete root.dataset.theme;
    this.states.delete(root);
  }

  /** 当前在 `root` 上生效的主题 ID（尚未成功加载的切换不算）。 */
  getActive(root: HTMLElement): string | null {
    return this.states.get(root)?.appliedId ?? null;
  }

  private stateFor(root: HTMLElement): RootState {
    const existing = this.states.get(root);
    if (existing) return existing;
    const created: RootState = {
      latestSeq: 0,
      appliedLink: null,
      appliedId: null,
      appliedHref: null,
      pending: null,
    };
    this.states.set(root, created);
    return created;
  }

  /**
   * 放弃在途切换。link 必须从文档里摘掉——留着的话它一旦加载完成就会连同旧主题
   * 一起生效，出现两套样式叠加；同时立刻结算 Promise，避免调用方永远 await 下去。
   */
  private abandonPending(state: RootState): void {
    const pending = state.pending;
    if (!pending) return;
    state.pending = null;
    clearTimeout(pending.timer);
    pending.link.remove();
    pending.settle('superseded');
  }
}

/** Singleton instance — import this everywhere */
export const themeRegistry = new ThemeRegistry();
