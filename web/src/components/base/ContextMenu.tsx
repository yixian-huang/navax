// ============================================================
// nav.ax ContextMenu — right-click context menu for site cards
// Uses design-system tokens, backdrop-blur, refined animation.
// ============================================================
import { useState, useEffect, useCallback, useRef } from 'react';
import { createPortal } from 'react-dom';
import { ExternalLink, Copy, Pencil, Trash2 } from 'lucide-react';
import { cn } from '@/lib/utils';

export interface ContextMenuAction {
  id: string;
  label: string;
  icon: typeof ExternalLink;
  onClick: () => void;
  destructive?: boolean;
}

interface ContextMenuState {
  x: number;
  y: number;
  actions: ContextMenuAction[];
}

interface ContextMenuResponse {
  handleContextMenu: (e: React.MouseEvent, actions: ContextMenuAction[]) => void;
  closeMenu: () => void;
  portal: React.ReactPortal | null;
}

export function useContextMenu(): ContextMenuResponse {
  const [menuState, setMenuState] = useState<ContextMenuState | null>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const [adjustX, setAdjustX] = useState(0);
  const [adjustY, setAdjustY] = useState(0);

  const handleContextMenu = useCallback(
    (e: React.MouseEvent, actions: ContextMenuAction[]) => {
      e.preventDefault();
      e.stopPropagation();
      setMenuState({ x: e.clientX, y: e.clientY, actions });
      setAdjustX(0);
      setAdjustY(0);
    },
    [],
  );

  const closeMenu = useCallback(() => setMenuState(null), []);

  useEffect(() => {
    if (!menuState) return;

    const handleClick = () => setMenuState(null);
    const handleEsc = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setMenuState(null);
    };

    document.addEventListener('click', handleClick, true);
    document.addEventListener('keydown', handleEsc, true);

    const raf = requestAnimationFrame(() => {
      if (menuRef.current) {
        const rect = menuRef.current.getBoundingClientRect();
        const vw = window.innerWidth;
        const vh = window.innerHeight;
        let dx = 0;
        let dy = 0;

        if (menuState.x + rect.width > vw - 8) dx = vw - rect.width - 8 - menuState.x;
        if (menuState.y + rect.height > vh - 8) dy = vh - rect.height - 8 - menuState.y;

        setAdjustX(dx);
        setAdjustY(dy);
      }
    });

    return () => {
      document.removeEventListener('click', handleClick, true);
      document.removeEventListener('keydown', handleEsc, true);
      cancelAnimationFrame(raf);
    };
  }, [menuState]);

  const x = (menuState?.x ?? 0) + adjustX;
  const y = (menuState?.y ?? 0) + adjustY;

  return {
    handleContextMenu,
    closeMenu,
    portal: menuState
      ? createPortal(
          <div
            ref={menuRef}
            className="fixed z-[100] min-w-[164px] bg-background-50/95 backdrop-blur-md rounded-xl border border-background-200/70 py-1.5 shadow-overlay animate-in"
            style={{ left: x, top: y }}
          >
            {menuState.actions.map(action => {
              const Icon = action.icon;
              return (
                <button
                  key={action.id}
                  onClick={(e) => {
                    e.stopPropagation();
                    action.onClick();
                    closeMenu();
                  }}
                  className={cn(
                    'w-full flex items-center gap-2.5 px-3.5 py-2 text-[13px] transition-colors duration-100 whitespace-nowrap text-left cursor-pointer',
                    action.destructive
                      ? 'text-red-600 hover:bg-red-50/70'
                      : 'text-foreground-700 hover:bg-background-100',
                  )}
                >
                  <Icon className="w-3.5 h-3.5 flex-shrink-0 opacity-70" />
                  {action.label}
                </button>
              );
            })}
          </div>,
          document.body,
        )
      : null,
  };
}

export function createSiteContextActions(
  site: { id: string; title: string; url: string },
  callbacks: {
    onOpen?: () => void;
    onCopyLink?: () => void;
    onEdit?: () => void;
    onDelete?: () => void;
  },
): ContextMenuAction[] {
  const actions: ContextMenuAction[] = [];
  if (callbacks.onOpen) {
    actions.push({ id: 'open', label: '在新标签页打开', icon: ExternalLink, onClick: callbacks.onOpen });
  }
  if (callbacks.onCopyLink) {
    actions.push({ id: 'copy', label: '复制链接', icon: Copy, onClick: callbacks.onCopyLink });
  }
  if (callbacks.onEdit) {
    actions.push({ id: 'edit', label: '编辑站点', icon: Pencil, onClick: callbacks.onEdit });
  }
  if (callbacks.onDelete) {
    actions.push({ id: 'delete', label: '删除站点', icon: Trash2, onClick: callbacks.onDelete, destructive: true });
  }
  return actions;
}
