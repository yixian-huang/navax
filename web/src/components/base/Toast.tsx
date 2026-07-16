/* eslint-disable react-refresh/only-export-components */
// ============================================================
// nav.ax Toast — design-system native, no hardcoded colors
// ============================================================

import { useState, useEffect, createContext, useContext, useCallback } from 'react';
import { X, CheckCircle, AlertCircle, Info, AlertTriangle } from 'lucide-react';
import { cn } from '@/lib/utils';

type ToastType = 'success' | 'error' | 'info' | 'warning';

interface Toast {
  id: string;
  type: ToastType;
  message: string;
}

interface ToastContextValue {
  toast: (type: ToastType, message: string) => void;
}

const ToastContext = createContext<ToastContextValue>({ toast: () => {} });

export function useToast() {
  return useContext(ToastContext);
}

const icons: Record<ToastType, typeof CheckCircle> = {
  success: CheckCircle,
  error: AlertCircle,
  info: Info,
  warning: AlertTriangle,
};

const styles: Record<ToastType, string> = {
  success: 'bg-accent-50/95 border-accent-200/70 text-accent-800',
  error: 'bg-red-50/95 border-red-200/70 text-red-800',
  info: 'bg-primary-50/95 border-primary-200/70 text-primary-800',
  warning: 'bg-secondary-50/95 border-secondary-200/70 text-secondary-800',
};

function ToastItem({ toast, onDismiss }: { toast: Toast; onDismiss: (id: string) => void }) {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const enterTimer = requestAnimationFrame(() => setVisible(true));
    const dismissTimer = setTimeout(() => setVisible(false), 3800);
    const removeTimer = setTimeout(() => onDismiss(toast.id), 4200);
    return () => {
      cancelAnimationFrame(enterTimer);
      clearTimeout(dismissTimer);
      clearTimeout(removeTimer);
    };
  }, [toast.id, onDismiss]);

  const Icon = icons[toast.type];

  return (
    <div
      className={cn(
        'flex items-center gap-3 px-4 py-3 rounded-xl border shadow-overlay backdrop-blur-sm min-w-[300px] max-w-[400px] transition-all duration-300 ease-out',
        styles[toast.type],
        visible ? 'opacity-100 translate-y-0 scale-100' : 'opacity-0 translate-y-2 scale-95',
      )}
    >
      <Icon className="w-4 h-4 flex-shrink-0" />
      <span className="text-[13px] flex-1 leading-snug">{toast.message}</span>
      <button
        onClick={() => setVisible(false)}
        className="flex-shrink-0 hover:opacity-70 transition-opacity duration-150 cursor-pointer"
      >
        <X className="w-3.5 h-3.5" />
      </button>
    </div>
  );
}

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const toast = useCallback((type: ToastType, message: string) => {
    const id = Date.now().toString() + Math.random().toString(36).slice(2);
    setToasts(prev => [...prev, { id, type, message }]);
  }, []);

  const dismiss = useCallback((id: string) => {
    setToasts(prev => prev.filter(t => t.id !== id));
  }, []);

  return (
    <ToastContext.Provider value={{ toast }}>
      {children}
      <div className="fixed bottom-6 right-6 z-[100] flex flex-col gap-2.5 pointer-events-none">
        {toasts.map(t => (
          <div key={t.id} className="pointer-events-auto">
            <ToastItem toast={t} onDismiss={dismiss} />
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}
