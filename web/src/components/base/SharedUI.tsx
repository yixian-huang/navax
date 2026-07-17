// ============================================================
// nav.ax Shared UI Components
// ============================================================

import { AlertTriangle, Inbox, RotateCw } from 'lucide-react';
import { cn } from '@/lib/utils';

// ---- Loading Skeleton ----
export function LoadingSkeleton({ className, count = 3 }: { className?: string; count?: number }) {
  return (
    <div className={cn('space-y-3', className)}>
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="skeleton h-16 w-full" />
      ))}
    </div>
  );
}

export function CardSkeleton({ className, count = 6 }: { className?: string; count?: number }) {
  return (
    <div className={cn('grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-3', className)}>
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="skeleton h-24 w-full rounded-lg" />
      ))}
    </div>
  );
}

// ---- Empty State ----
export function EmptyState({
  icon: Icon,
  iconClass,
  title,
  description,
  action,
}: {
  icon?: React.ComponentType<{ className?: string }>;
  iconClass?: string;
  title: string;
  description?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center py-16 px-4 text-center">
      <div className="w-16 h-16 rounded-full bg-background-100 flex items-center justify-center mb-4">
        {Icon ? (
          <Icon className="w-8 h-8 text-foreground-300" />
        ) : iconClass ? (
          <i className={cn(iconClass, 'text-2xl text-foreground-300')} />
        ) : (
          <Inbox className="w-8 h-8 text-foreground-300" />
        )}
      </div>
      <h3 className="text-lg font-semibold text-foreground-700 mb-1">{title}</h3>
      {description && <p className="text-sm text-foreground-400 max-w-sm mb-4">{description}</p>}
      {action}
    </div>
  );
}

// ---- Error State ----
export function ErrorState({
  message = '加载失败',
  onRetry,
}: {
  message?: string;
  onRetry?: () => void;
}) {
  return (
    <div className="flex flex-col items-center justify-center py-16 px-4 text-center">
      <div className="w-16 h-16 rounded-full bg-background-100 flex items-center justify-center mb-4">
        <AlertTriangle className="w-8 h-8 text-primary-400" />
      </div>
      <h3 className="text-lg font-semibold text-foreground-700 mb-1">出错了</h3>
      <p className="text-sm text-foreground-400 mb-4">{message}</p>
      {onRetry && (
        <button
          onClick={onRetry}
          className="inline-flex items-center gap-2 h-9 px-4 rounded-lg bg-background-100 border border-background-200 text-sm text-foreground-600 hover:bg-background-200 transition-colors duration-150"
        >
          <RotateCw className="w-4 h-4" />
          重试
        </button>
      )}
    </div>
  );
}

// ---- Confirm Dialog ----
export function ConfirmDialog({
  open,
  onClose,
  onConfirm,
  title,
  description,
  confirmLabel = '确认',
  danger = false,
}: {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  description: string;
  confirmLabel?: string;
  danger?: boolean;
}) {
  if (!open) return null;

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center">
      <div className="absolute inset-0 bg-black/30" onClick={onClose} />
      <div className="relative bg-background-50 rounded-xl shadow-overlay p-6 w-full max-w-sm mx-4">
        <h3 className="text-lg font-semibold text-foreground-900 mb-2">{title}</h3>
        <p className="text-sm text-foreground-500 mb-6">{description}</p>
        <div className="flex items-center justify-end gap-3">
          <button
            onClick={onClose}
            className="h-9 px-4 rounded-lg text-sm text-foreground-600 hover:bg-background-100 transition-colors duration-150 whitespace-nowrap"
          >
            取消
          </button>
          <button
            onClick={() => { onConfirm(); onClose(); }}
            className={cn(
              'h-9 px-4 rounded-lg text-sm font-medium transition-colors duration-150 whitespace-nowrap',
              danger ? 'bg-red-500 hover:bg-red-600 text-background-50' : 'bg-primary-500 hover:bg-primary-600 text-background-50 dark:text-foreground-950'
            )}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}

// ---- Badge ----
export function Badge({
  children,
  variant = 'default',
  className,
}: {
  children: React.ReactNode;
  variant?: 'default' | 'success' | 'warning' | 'danger';
  className?: string;
}) {
  const variants = {
    default: 'bg-background-100 text-foreground-600',
    success: 'bg-accent-50 text-accent-700',
    warning: 'bg-primary-50 text-primary-600',
    danger: 'bg-foreground-100 text-foreground-700',
  };

  return (
    <span className={cn('inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium', variants[variant], className)}>
      {children}
    </span>
  );
}

// ---- Publish Status Badge ----
export function PublishStatusBadge({
  hasUnpublishedChanges,
  publishedAt,
  published,
}: {
  hasUnpublishedChanges: boolean;
  publishedAt: string | null;
  /** Optional explicit flag; falls back to Boolean(publishedAt) for backward compatibility */
  published?: boolean;
}) {
  const isPublished = published ?? Boolean(publishedAt);
  if (!isPublished) {
    return <Badge variant="default">未发布</Badge>;
  }
  if (hasUnpublishedChanges) {
    return <Badge variant="warning">有草稿未上线</Badge>;
  }
  return <Badge variant="success">已是最新</Badge>;
}
