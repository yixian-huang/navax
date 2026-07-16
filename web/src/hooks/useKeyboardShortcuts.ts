// ============================================================
// nav.ax Keyboard Shortcuts — global keyboard shortcuts hook
// Ctrl+K / Cmd+K → focus search
// Esc → close modals / clear search
// ============================================================
import { useEffect, useCallback } from 'react';

interface ShortcutHandlers {
  onFocusSearch?: () => void;
  onEscape?: () => void;
}

export function useKeyboardShortcuts(handlers: ShortcutHandlers) {
  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      const mod = e.metaKey || e.ctrlKey;
      const activeEl = document.activeElement;
      const isInput = activeEl instanceof HTMLInputElement || activeEl instanceof HTMLTextAreaElement || (activeEl as HTMLElement)?.isContentEditable;

      // Ctrl+K / Cmd+K → focus search
      if (mod && e.key === 'k') {
        e.preventDefault();
        if (handlers.onFocusSearch) {
          handlers.onFocusSearch();
          return;
        }
        // Default: find any search input and focus it
        const searchInput = document.querySelector<HTMLInputElement>(
          'input[type="search"], input[placeholder*="搜索"], input[placeholder*="search"], [data-search-input]',
        );
        if (searchInput) {
          searchInput.focus();
          searchInput.select();
        }
      }

      // Escape (only when not in input — let inputs handle their own escape)
      if (e.key === 'Escape' && !isInput) {
        if (handlers.onEscape) {
          handlers.onEscape();
        }
      }
    },
    [handlers],
  );

  useEffect(() => {
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [handleKeyDown]);
}
