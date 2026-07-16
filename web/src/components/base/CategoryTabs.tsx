// ============================================================
// nav.ax CategoryTabs — text-only with animated underline
// No pill backgrounds, no borders. Just typography + indicator.
// ============================================================

import { useRef, useEffect } from 'react';
import { cn } from '@/lib/utils';
import type { Category } from '@/api/types';
import IconRenderer from '@/components/base/IconRenderer';

interface CategoryTabsProps {
  categories: Category[];
  activeId: string;
  onChange: (id: string) => void;
  showAll?: boolean;
  allLabel?: string;
}

export default function CategoryTabs({
  categories,
  activeId,
  onChange,
  showAll = false,
  allLabel = '全部',
}: CategoryTabsProps) {
  const tabsRef = useRef<HTMLDivElement>(null);
  const indicatorRef = useRef<HTMLDivElement>(null);

  // Animate indicator to active tab position
  useEffect(() => {
    const container = tabsRef.current;
    const indicator = indicatorRef.current;
    if (!container || !indicator) return;

    const activeBtn = container.querySelector('[data-active="true"]') as HTMLElement;
    if (!activeBtn) return;

    const containerRect = container.getBoundingClientRect();
    const tabRect = activeBtn.getBoundingClientRect();

    indicator.style.width = `${tabRect.width}px`;
    indicator.style.transform = `translateX(${tabRect.left - containerRect.left}px)`;
  }, [activeId]);

  const handleKeyDown = (e: React.KeyboardEvent, index: number) => {
    const buttons = tabsRef.current?.querySelectorAll('button');
    if (!buttons) return;

    let nextIndex = index;
    if (e.key === 'ArrowRight') {
      e.preventDefault();
      nextIndex = (index + 1) % buttons.length;
    } else if (e.key === 'ArrowLeft') {
      e.preventDefault();
      nextIndex = (index - 1 + buttons.length) % buttons.length;
    } else return;

    (buttons[nextIndex] as HTMLButtonElement).focus();
  };

  let idx = 0;

  return (
    <div className="relative">
      <div
        ref={tabsRef}
        className="flex items-center gap-5 md:gap-6 overflow-x-auto pb-3 scrollbar-none"
        role="tablist"
        aria-label="分类筛选"
      >
        {showAll && (
          <button
            role="tab"
            aria-selected={activeId === ''}
            tabIndex={activeId === '' ? 0 : -1}
            data-active={activeId === ''}
            onClick={() => onChange('')}
            onKeyDown={(e) => handleKeyDown(e, idx)}
            className={cn(
              'flex-shrink-0 text-sm font-medium transition-colors duration-200 whitespace-nowrap focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-400/50 pb-1',
              activeId === '' ? 'text-primary-500' : 'text-foreground-400 hover:text-foreground-600'
            )}
          >
            {allLabel}
          </button>
        )}
        {categories.map((cat, i) => {
          const tabIdx = showAll ? i + 1 : i;
          const isActive = cat.id === activeId;
          return (
            <button
              key={cat.id}
              role="tab"
              aria-selected={isActive}
              tabIndex={isActive ? 0 : -1}
              data-active={isActive}
              onClick={() => onChange(cat.id)}
              onKeyDown={(e) => handleKeyDown(e, tabIdx)}
              className={cn(
                'flex-shrink-0 flex items-center gap-2 text-sm font-medium transition-colors duration-200 whitespace-nowrap focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-400/50 pb-1',
                isActive ? 'text-primary-500' : 'text-foreground-400 hover:text-foreground-600'
              )}
            >
              <IconRenderer icon={cat.icon} className="text-base" />
              {cat.name}
              <span className={cn(
                'text-xs ml-0.5',
                isActive ? 'text-primary-400/60' : 'text-foreground-200'
              )}>
                {cat.sites.length}
              </span>
            </button>
          );
        })}
      </div>

      {/* Animated underline indicator */}
      <div className="relative h-[2px]">
        <div className="absolute inset-0 bg-secondary-100/25 rounded-full" />
        <div
          ref={indicatorRef}
          className="absolute top-0 left-0 h-[2px] bg-primary-500 rounded-full transition-all duration-300 ease-out"
          style={{ width: 0 }}
        />
      </div>
    </div>
  );
}
