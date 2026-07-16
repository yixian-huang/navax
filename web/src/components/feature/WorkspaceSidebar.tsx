import { type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { X, type LucideIcon } from 'lucide-react';
import { cn } from '@/lib/utils';

export interface SidebarNavItem {
  path: string;
  icon: LucideIcon;
  label: string;
}

type SidebarVariant = 'admin' | 'app';

interface WorkspaceSidebarProps {
  open: boolean;
  onClose: () => void;
  navItems: SidebarNavItem[];
  quickLinks?: SidebarNavItem[];
  currentPath: string;
  variant?: SidebarVariant;
  badge?: string;
  brandHref?: string;
  bottomContent?: ReactNode;
}

const variantStyles: Record<SidebarVariant, { width: string; logoH: string; iconSize: string; itemH: string; itemGap: string; itemPx: string; itemRadius: string; itemTextSize: string }> = {
  admin: {
    width: 'w-52',
    logoH: 'h-12',
    iconSize: 'w-3.5 h-3.5',
    itemH: 'h-8',
    itemGap: 'gap-2.5',
    itemPx: 'px-2.5',
    itemRadius: 'rounded-md',
    itemTextSize: 'text-sm',
  },
  app: {
    width: 'w-56',
    logoH: 'h-14',
    iconSize: 'w-4 h-4',
    itemH: 'h-9',
    itemGap: 'gap-3',
    itemPx: 'px-3',
    itemRadius: 'rounded-lg',
    itemTextSize: 'text-sm',
  },
};

export default function WorkspaceSidebar({
  open,
  onClose,
  navItems,
  quickLinks = [],
  currentPath,
  variant = 'admin',
  badge,
  brandHref = '/',
  bottomContent,
}: WorkspaceSidebarProps) {
  const v = variantStyles[variant];

  return (
    <>
      {/* Mobile overlay */}
      {open && (
        <div className="fixed inset-0 bg-black/20 z-40 lg:hidden" onClick={onClose} />
      )}

      <aside className={cn(
        'fixed lg:sticky top-0 left-0 z-50 h-screen bg-background-50 border-r border-background-200/70 flex flex-col transition-transform duration-200',
        'lg:translate-x-0',
        v.width,
        open ? 'translate-x-0' : '-translate-x-full'
      )}>
        {/* Logo */}
        <div className={cn('flex items-center px-4 border-b border-background-200/70', v.logoH)}>
          <Link to={brandHref} className="flex items-center gap-1.5">
            <span className="text-lg font-bold font-heading text-foreground-950">nav.ax</span>
          </Link>
          {badge && (
            <span className="ml-2 text-[10px] bg-accent-100 text-accent-700 px-1.5 py-0 rounded font-medium">{badge}</span>
          )}
          <button
            className="lg:hidden ml-auto w-7 h-7 flex items-center justify-center text-foreground-500"
            onClick={onClose}
            aria-label="关闭菜单"
          >
            <X className={cn(variant === 'admin' ? 'w-4 h-4' : 'w-5 h-5')} />
          </button>
        </div>

        {/* Navigation */}
        <nav className="flex-1 overflow-y-auto py-2 px-2 space-y-0.5">
          {navItems.map(item => {
            const itemPathname = item.path.split('?')[0];
            const isActive = itemPathname === '/app' || itemPathname === '/admin'
              ? currentPath === itemPathname
              : currentPath.startsWith(itemPathname);
            return (
              <Link
                key={item.path}
                to={item.path}
                onClick={onClose}
                className={cn(
                  'flex items-center transition-all duration-200 whitespace-nowrap',
                  v.itemH,
                  v.itemGap,
                  v.itemPx,
                  v.itemRadius,
                  v.itemTextSize,
                  isActive
                    ? 'bg-primary-100 text-primary-700 font-medium shadow-[inset_0_1px_0_rgba(0,0,0,0.02)]'
                    : 'text-foreground-600 hover:bg-background-100 hover:text-foreground-900 hover:translate-x-0.5'
                )}
              >
                <item.icon className={cn(v.iconSize, 'flex-shrink-0')} />
                {item.label}
              </Link>
            );
          })}
          {quickLinks.length > 0 && (
            <div className="pt-2 mt-2 border-t border-background-200/70 space-y-0.5">
              {quickLinks.map(item => (
                <Link
                  key={item.path}
                  to={item.path}
                  onClick={onClose}
                  className={cn(
                    'flex items-center transition-all duration-200 whitespace-nowrap text-foreground-500 hover:bg-background-100 hover:text-foreground-800',
                    v.itemH,
                    v.itemGap,
                    v.itemPx,
                    v.itemRadius,
                    v.itemTextSize,
                  )}
                >
                  <item.icon className={cn(v.iconSize, 'flex-shrink-0')} />
                  {item.label}
                </Link>
              ))}
            </div>
          )}
        </nav>

        {/* Bottom */}
        {bottomContent}
      </aside>
    </>
  );
}
