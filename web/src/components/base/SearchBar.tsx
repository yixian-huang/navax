/* eslint-disable react-refresh/only-export-components */
// ============================================================
// nav.ax SearchBar — Refined Neutral / Material
// A floating material layer with a soft focus ring.
// ============================================================

import { useState, useEffect, useRef, useCallback } from 'react';
import { cn } from '@/lib/utils';

export type SearchEngine = 'google' | 'bing' | 'duckduckgo' | 'baidu';

const engines: { key: SearchEngine; label: string; url: string }[] = [
  { key: 'google', label: 'Google', url: 'https://www.google.com/search?q=' },
  { key: 'bing', label: 'Bing', url: 'https://www.bing.com/search?q=' },
  { key: 'duckduckgo', label: 'DuckDuckGo', url: 'https://duckduckgo.com/?q=' },
  { key: 'baidu', label: '百度', url: 'https://www.baidu.com/s?wd=' },
];

// ---- 搜索历史 ----
const HISTORY_KEY = 'inav_search_history';
const MAX_HISTORY = 10;

function loadHistory(): string[] {
  try {
    const raw = window.localStorage.getItem(HISTORY_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed.filter((h: unknown): h is string => typeof h === 'string' && h.trim().length > 0) : [];
  } catch {
    return [];
  }
}

function saveHistory(items: string[]) {
  try {
    window.localStorage.setItem(HISTORY_KEY, JSON.stringify(items.slice(0, MAX_HISTORY)));
  } catch {
    // quota exceeded — silently drop
  }
}

function addToHistory(query: string, existing: string[]): string[] {
  const q = query.trim();
  if (!q) return existing;
  const deduped = existing.filter(h => h !== q);
  return [q, ...deduped].slice(0, MAX_HISTORY);
}

interface SearchBarProps {
  value: string;
  onChange: (value: string) => void;
  onSearch: (query: string, engine: SearchEngine) => void;
  engine: SearchEngine;
  onEngineChange: (engine: SearchEngine) => void;
  placeholder?: string;
  size?: 'lg' | 'md' | 'sm';
  showEngineSelector?: boolean;
  showHint?: boolean;
  autoFocus?: boolean;
  /** 动态智能建议词，输入框空闲时轮播展示 */
  suggestions?: string[];
}

export default function SearchBar({
  value,
  onChange,
  onSearch,
  engine,
  onEngineChange,
  placeholder = '搜索或输入网址...',
  size = 'lg',
  showEngineSelector = false,
  showHint = false,
  autoFocus = false,
  suggestions,
}: SearchBarProps) {
  const [engineOpen, setEngineOpen] = useState(false);
  const [focused, setFocused] = useState(false);
  const [suggestIdx, setSuggestIdx] = useState(0);
  const [suggestVisible, setSuggestVisible] = useState(true);
  const [history, setHistory] = useState<string[]>(loadHistory);
  const [historyOpen, setHistoryOpen] = useState(false);
  const blurTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const currentEngine = engines.find(e => e.key === engine) || engines[0];

  // 写入历史 + 通知上层执行搜索
  const commitSearch = useCallback((q: string, eng: SearchEngine) => {
    setHistory(prev => {
      const next = addToHistory(q, prev);
      saveHistory(next);
      return next;
    });
    onSearch(q, eng);
  }, [onSearch]);

  // 空闲时轮播智能建议：淡出 → 切换 → 淡入
  const hasSuggestions = !!suggestions && suggestions.length > 0;
  const idle = hasSuggestions && !focused && !value.trim();
  const idleRef = useRef(idle);
  idleRef.current = idle;

  useEffect(() => {
    if (!hasSuggestions) return;
    const timer = setInterval(() => {
      if (!idleRef.current) return;
      setSuggestVisible(false);
      window.setTimeout(() => {
        setSuggestIdx(i => (i + 1) % (suggestions?.length || 1));
        setSuggestVisible(true);
      }, 400);
    }, 3600);
    return () => clearInterval(timer);
  }, [hasSuggestions, suggestions]);

  const activePlaceholder = idle && suggestions
    ? suggestions[suggestIdx % suggestions.length]
    : placeholder;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!value.trim()) return;
    setHistoryOpen(false);
    commitSearch(value.trim(), engine);
  };

  const handleHistoryPick = (q: string) => {
    onChange(q);
    setHistoryOpen(false);
    commitSearch(q, engine);
  };

  const handleClearHistory = () => {
    setHistory([]);
    try { window.localStorage.setItem(HISTORY_KEY, JSON.stringify([])); } catch { /* ignore */ }
    setHistoryOpen(false);
  };

  const handleFocus = () => {
    setFocused(true);
    if (blurTimer.current) { clearTimeout(blurTimer.current); blurTimer.current = null; }
    if (!value.trim() && history.length > 0) setHistoryOpen(true);
  };

  const handleBlur = () => {
    setFocused(false);
    blurTimer.current = setTimeout(() => setHistoryOpen(false), 140);
  };

  // 键盘导航：上下选历史项、回车确认、Esc关闭
  const [historyIdx, setHistoryIdx] = useState(-1);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!historyOpen || history.length === 0) return;
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setHistoryIdx(i => Math.min(i + 1, history.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setHistoryIdx(i => Math.max(i - 1, -1));
    } else if (e.key === 'Escape') {
      setHistoryOpen(false);
    } else if (e.key === 'Enter' && historyIdx >= 0) {
      e.preventDefault();
      handleHistoryPick(history[historyIdx]);
    }
  };

  // 重置键盘选中
  useEffect(() => { setHistoryIdx(-1); }, [historyOpen]);

  const sizeClasses = {
    lg: 'h-16 text-base',
    md: 'h-12 text-sm',
    sm: 'h-10 text-sm',
  };

  const dropdownOpen = engineOpen || historyOpen;

  return (
    <form
      data-nx="search-box"
      onSubmit={handleSubmit}
      // When open, lift above following page sections. Parent wrappers that use
      // rise-in (transform) also need their own z-index — see SearchSection.
      className={cn('relative', dropdownOpen ? 'z-30' : 'z-0')}
    >
      {/* Anchor only the bar so absolute menus sit under the input, not under the hint. */}
      <div className="relative">
        <div
          className={cn(
            // Do NOT use overflow-hidden — it clips absolute engine/history menus.
            // search-bar-surface: solid by default; frosted under [data-wallpaper] (see index.css).
            'search-bar-surface flex items-center bg-background-50 transition-all duration-300',
            size === 'sm' ? 'rounded-xl' : 'rounded-2xl',
          )}
          style={{
            boxShadow: focused
              ? '0 0 0 2px oklch(var(--primary-400) / 0.35), 0 8px 24px -4px oklch(var(--primary-900) / 0.12), 0 20px 48px -12px oklch(var(--primary-900) / 0.10)'
              : 'var(--elevation-raised)',
          }}
        >
          <div className="flex items-center justify-center w-14 md:w-16 flex-shrink-0">
            <i className={cn(
              'ri-search-line text-lg transition-colors duration-200',
              focused ? 'text-primary-500' : 'text-foreground-300'
            )} />
          </div>

          <input
            data-nx="search-input"
            type="text"
            value={value}
            onChange={e => onChange(e.target.value)}
            onFocus={handleFocus}
            onBlur={handleBlur}
            onKeyDown={handleKeyDown}
            placeholder={activePlaceholder}
            className={cn(
              'flex-1 pr-4 bg-transparent text-foreground-900 focus:outline-none transition-all duration-300',
              idle && !suggestVisible ? 'placeholder:opacity-0' : 'placeholder:opacity-100',
              'placeholder:text-foreground-300',
              sizeClasses[size]
            )}
            autoFocus={autoFocus}
            aria-label="搜索"
            role="combobox"
            aria-expanded={historyOpen}
            aria-haspopup="listbox"
          />

          {showEngineSelector && (
            <div className="relative flex items-center pl-2 pr-2.5 flex-shrink-0">
              <button
                type="button"
                onClick={() => setEngineOpen(!engineOpen)}
                className="flex items-center gap-1.5 h-8 px-3 rounded-lg text-xs font-medium text-foreground-500 bg-background-200/60 hover:bg-background-200 hover:text-foreground-700 transition-all duration-200 whitespace-nowrap"
                aria-expanded={engineOpen}
                aria-haspopup="listbox"
                aria-label={`当前搜索引擎：${currentEngine.label}`}
              >
                {currentEngine.label}
                <i className={cn('ri-arrow-down-s-line text-sm transition-transform duration-200', engineOpen && 'rotate-180')} />
              </button>
            </div>
          )}
        </div>

        {/* Engine menu — sibling of the bar so it is never clipped by bar styles */}
        {showEngineSelector && engineOpen && (
          <>
            <div className="fixed inset-0 z-40" onClick={() => setEngineOpen(false)} aria-hidden />
            <div
              className="absolute top-full right-2 mt-2 z-50 w-40 bg-background-50 rounded-xl shadow-overlay border border-background-200/70 py-1.5 dropdown-enter"
              role="listbox"
              aria-label="选择搜索引擎"
            >
              {engines.map(en => (
                <button
                  key={en.key}
                  type="button"
                  role="option"
                  aria-selected={engine === en.key}
                  onClick={() => {
                    onEngineChange(en.key);
                    setEngineOpen(false);
                  }}
                  className={cn(
                    'w-full flex items-center gap-2.5 px-3.5 py-2 text-sm transition-colors duration-100 whitespace-nowrap',
                    engine === en.key
                      ? 'text-primary-600 font-medium bg-background-100'
                      : 'text-foreground-500 hover:text-foreground-800 hover:bg-background-100'
                  )}
                >
                  <span className={cn(
                    'w-1.5 h-1.5 rounded-full',
                    engine === en.key ? 'bg-accent-500' : 'bg-secondary-100'
                  )} />
                  {en.label}
                </button>
              ))}
            </div>
          </>
        )}

        {/* 搜索历史下拉 */}
        {historyOpen && (
          <>
            <div className="fixed inset-0 z-40" onClick={() => setHistoryOpen(false)} aria-hidden />
            <div
              className="absolute left-0 right-0 top-full mt-2 z-50 bg-background-50 rounded-xl shadow-overlay border border-background-200/70 py-1.5 dropdown-enter"
              role="listbox"
              aria-label="最近搜索"
            >
              <div className="flex items-center justify-between px-3.5 pb-1.5">
                <span className="text-[10px] text-foreground-300 tracking-wide uppercase">最近搜索</span>
                <button
                  type="button"
                  onMouseDown={e => { e.preventDefault(); handleClearHistory(); }}
                  className="text-[10px] text-foreground-300 hover:text-foreground-500 transition-colors duration-150 whitespace-nowrap cursor-pointer"
                >
                  清除记录
                </button>
              </div>
              {history.map((h, idx) => (
                <button
                  key={`${h}-${idx}`}
                  type="button"
                  role="option"
                  aria-selected={idx === historyIdx}
                  onMouseDown={e => { e.preventDefault(); handleHistoryPick(h); }}
                  className={cn(
                    'w-full flex items-center gap-3 px-3.5 py-2 text-sm transition-colors duration-100 whitespace-nowrap text-left cursor-pointer',
                    idx === historyIdx
                      ? 'bg-background-100 text-primary-600'
                      : 'text-foreground-500 hover:text-foreground-800 hover:bg-background-100'
                  )}
                >
                  <i className="ri-time-line text-[11px] text-foreground-300 flex-shrink-0" />
                  <span className="truncate">{h}</span>
                </button>
              ))}
              {size !== 'sm' && (
                <div className="pt-1.5 px-3.5 mt-0.5">
                  <p className="text-[10px] text-foreground-300 leading-relaxed">
                    <i className="ri-arrow-up-line text-[9px] mr-0.5" /><i className="ri-arrow-down-line text-[9px] mr-1" /> 方向键选择 · 回车确认 · Esc 关闭
                  </p>
                </div>
              )}
            </div>
          </>
        )}
      </div>

      {showHint && (
        <p className="mt-3 text-[11px] text-foreground-300 pl-1 tracking-wide">
          回车搜索 · 支持切换搜索引擎 · 粘贴网址可直接访问
        </p>
      )}
    </form>
  );
}

export { engines };
