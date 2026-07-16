/* eslint-disable react-refresh/only-export-components */
// ============================================================
// nav.ax SaveStatus — auto-save tracking context
// ============================================================

import { createContext, useContext, useState, useCallback, useRef } from 'react';

export type SavePhase = 'idle' | 'saving' | 'saved' | 'error';

interface SaveStatusState {
  phase: SavePhase;
  lastSavedAt: Date | null;
  errorMessage: string | null;
}

interface SaveStatusContextValue {
  status: SaveStatusState;
  markSaving: () => void;
  markSaved: () => void;
  markError: (message: string) => void;
  clearStatus: () => void;
}

const SaveStatusContext = createContext<SaveStatusContextValue>({
  status: { phase: 'idle', lastSavedAt: null, errorMessage: null },
  markSaving: () => {},
  markSaved: () => {},
  markError: () => {},
  clearStatus: () => {},
});

export function useSaveStatus() {
  return useContext(SaveStatusContext);
}

export function SaveStatusProvider({ children }: { children: React.ReactNode }) {
  const [status, setStatus] = useState<SaveStatusState>({
    phase: 'idle',
    lastSavedAt: null,
    errorMessage: null,
  });
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const markSaving = useCallback(() => {
    setStatus({ phase: 'saving', lastSavedAt: null, errorMessage: null });
  }, []);

  const markSaved = useCallback(() => {
    setStatus({ phase: 'saved', lastSavedAt: new Date(), errorMessage: null });
    // Auto-clear "saved" indicator after 3s
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      setStatus(prev => prev.phase === 'saved' ? { ...prev, phase: 'idle' } : prev);
    }, 3000);
  }, []);

  const markError = useCallback((message: string) => {
    setStatus({ phase: 'error', lastSavedAt: null, errorMessage: message });
  }, []);

  const clearStatus = useCallback(() => {
    setStatus({ phase: 'idle', lastSavedAt: null, errorMessage: null });
  }, []);

  return (
    <SaveStatusContext.Provider value={{ status, markSaving, markSaved, markError, clearStatus }}>
      {children}
    </SaveStatusContext.Provider>
  );
}

// --- SaveStatusBar component ---
export function SaveStatusBar() {
  const { status } = useSaveStatus();
  const { phase, errorMessage, lastSavedAt } = status;

  if (phase === 'idle') return null;

  return (
    <div className="h-8 flex items-center px-4 text-xs font-medium gap-2">
      {phase === 'saving' && (
        <>
          <div className="w-3 h-3 rounded-full border-2 border-primary-400 border-t-transparent animate-spin" />
          <span className="text-foreground-500">自动保存中...</span>
        </>
      )}
      {phase === 'saved' && (
        <>
          <i className="ri-check-line text-accent-500 text-sm" />
          <span className="text-accent-700">
            已保存 {lastSavedAt ? lastSavedAt.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' }) : ''}
          </span>
        </>
      )}
      {phase === 'error' && (
        <>
          <i className="ri-error-warning-line text-red-500 text-sm" />
          <span className="text-red-600">{errorMessage || '保存失败'}</span>
          <button
            onClick={() => window.location.reload()}
            className="ml-2 underline hover:text-red-700 whitespace-nowrap"
          >
            重试
          </button>
        </>
      )}
    </div>
  );
}
