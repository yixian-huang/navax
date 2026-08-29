// ============================================================
// nav.ax ThemeImportDialog — import a private theme from GitHub or a zip
// ============================================================

import { useCallback, useEffect, useRef, useState } from 'react';
import { X, Loader2, FileArchive, Import } from 'lucide-react';
import { cn } from '@/lib/utils';
import { FormField, FormInput } from '@/components/base/FormField';
import { useToast } from '@/components/base/Toast';
import { themesApi } from '@/api/themes';
import { ApiError } from '@/api/client';
import type { ThemeValidationResult } from '@/api/types';

const THEME_API_URL = 'https://github.com/yixian-huang/navax/blob/main/docs/theme-api.md';
const STARTER_URL = 'https://github.com/yixian-huang/navax/tree/main/examples/theme-starter';

interface ThemeImportDialogProps {
  open: boolean;
  onClose: () => void;
  onImported: () => void;
}

type ImportTab = 'github' | 'zip';

export function ThemeImportDialog({ open, onClose, onImported }: ThemeImportDialogProps) {
  const [tab, setTab] = useState<ImportTab>('github');
  const [githubUrl, setGithubUrl] = useState('');
  const [githubRef, setGithubRef] = useState('');
  const [file, setFile] = useState<File | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [validating, setValidating] = useState(false);
  const [validation, setValidation] = useState<ThemeValidationResult | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const validateAbortRef = useRef<AbortController | null>(null);
  const { toast } = useToast();

  const reset = useCallback(() => {
    validateAbortRef.current?.abort();
    setTab('github');
    setGithubUrl('');
    setGithubRef('');
    setFile(null);
    setValidating(false);
    setValidation(null);
  }, []);

  const handleClose = useCallback(() => {
    if (submitting) return;
    reset();
    onClose();
  }, [submitting, reset, onClose]);

  // Esc 关闭，与点击遮罩层/关闭按钮同一路径（submitting 时同样按兵不动）。
  useEffect(() => {
    if (!open) return;
    const handleEsc = (e: KeyboardEvent) => {
      if (e.key === 'Escape') handleClose();
    };
    document.addEventListener('keydown', handleEsc, true);
    return () => document.removeEventListener('keydown', handleEsc, true);
  }, [open, handleClose]);

  useEffect(() => () => {
    validateAbortRef.current?.abort();
  }, []);

  const runValidate = useCallback(async (zip: File) => {
    validateAbortRef.current?.abort();
    const controller = new AbortController();
    validateAbortRef.current = controller;
    setValidating(true);
    setValidation(null);
    try {
      const response = await themesApi.validate(zip, controller.signal);
      if (controller.signal.aborted) return;
      setValidation(response.data);
    } catch (cause) {
      if (controller.signal.aborted) return;
      if (cause instanceof DOMException && cause.name === 'AbortError') return;
      const message = cause instanceof ApiError
        ? (cause.detail || cause.message)
        : '校验失败';
      setValidation({ valid: false, errors: [{ stage: 'archive', path: '', message }] });
    } finally {
      if (!controller.signal.aborted) setValidating(false);
    }
  }, []);

  const handleZipChosen = useCallback((next: File | null) => {
    setFile(next);
    setValidation(null);
    if (next) void runValidate(next);
  }, [runValidate]);

  const canSubmit = tab === 'github'
    ? githubUrl.trim().length > 0
    : file !== null && validation?.valid === true && !validating;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (submitting || !canSubmit) return;
    setSubmitting(true);
    try {
      const response = tab === 'github'
        ? await themesApi.importGitHub(githubUrl.trim(), githubRef.trim() || undefined)
        : await themesApi.importZip(file!);
      toast('success', `已导入主题「${response.data.name}」`);
      onImported();
      reset();
      onClose();
    } catch (cause) {
      // ApiError.detail 携带服务端给出的具体拒绝原因（例如校验器点出的规则与
      // 文件名），拼进 toast 让作者知道具体是哪条规则拒了哪个文件，而不是只
      // 看到一句笼统的 message。
      if (cause instanceof ApiError) {
        toast('error', cause.detail ? `${cause.message}：${cause.detail}` : cause.message);
      } else {
        toast('error', cause instanceof Error ? cause.message : '主题导入失败');
      }
    } finally {
      setSubmitting(false);
    }
  };

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center">
      <div className="absolute inset-0 bg-black/30" onClick={handleClose} />
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="theme-import-dialog-title"
        className="relative bg-background-50 rounded-xl p-6 w-full max-w-md mx-4 max-h-[90vh] overflow-y-auto"
      >
        <div className="flex items-center justify-between mb-2">
          <h3 id="theme-import-dialog-title" className="text-lg font-semibold text-foreground-900">导入主题</h3>
          <button onClick={handleClose} className="w-8 h-8 flex items-center justify-center rounded-lg text-foreground-400 hover:bg-background-100 transition-colors duration-150">
            <X className="w-5 h-5" />
          </button>
        </div>
        <p className="text-[11px] text-foreground-400 mb-4 leading-relaxed">
          仓库根目录或 zip 根目录需要有 theme.json。示例包见
          {' '}
          <a href={STARTER_URL} target="_blank" rel="noopener noreferrer" className="text-primary-600 hover:text-primary-500">theme-starter</a>
          ，规范见
          {' '}
          <a href={THEME_API_URL} target="_blank" rel="noopener noreferrer" className="text-primary-600 hover:text-primary-500">主题 API</a>
          。GitHub 导入不能指向本应用仓库。
        </p>

        <div className="flex items-center bg-background-100 rounded-lg p-0.5 mb-4">
          {([
            { key: 'github' as const, label: 'GitHub 仓库' },
            { key: 'zip' as const, label: '上传 zip' },
          ]).map(t => (
            <button
              key={t.key}
              type="button"
              onClick={() => setTab(t.key)}
              className={cn(
                'flex-1 h-8 rounded-md text-xs font-medium transition-all duration-200 whitespace-nowrap cursor-pointer',
                tab === t.key
                  ? 'bg-background-50 text-foreground-900 shadow-raised'
                  : 'text-foreground-400 hover:text-foreground-600',
              )}
            >
              {t.label}
            </button>
          ))}
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          {tab === 'github' ? (
            <>
              <FormField label="仓库地址">
                <FormInput
                  type="text"
                  value={githubUrl}
                  onChange={e => setGithubUrl(e.target.value)}
                  placeholder="https://github.com/owner/repo"
                  autoFocus
                />
              </FormField>
              <FormField label="分支 / Ref（可选）">
                <FormInput
                  type="text"
                  value={githubRef}
                  onChange={e => setGithubRef(e.target.value)}
                  placeholder="留空则使用默认分支"
                />
              </FormField>
            </>
          ) : (
            <FormField label="主题包（.zip）">
              <input
                ref={fileInputRef}
                type="file"
                accept=".zip"
                data-testid="theme-zip-input"
                onChange={e => handleZipChosen(e.target.files?.[0] ?? null)}
                className="hidden"
              />
              <button
                type="button"
                onClick={() => fileInputRef.current?.click()}
                className="w-full h-9 px-3 rounded-lg border border-dashed border-background-200/70 text-sm text-foreground-500 hover:border-primary-300 hover:text-primary-600 transition-colors duration-150 flex items-center justify-center gap-1.5"
              >
                <FileArchive className="w-4 h-4" />
                选择 zip 文件
              </button>
              {file && (
                <p className="mt-1.5 text-[11px] text-foreground-400 truncate">已选择：{file.name}</p>
              )}
              {validating && (
                <p className="mt-1.5 text-[11px] text-foreground-400 flex items-center gap-1.5">
                  <Loader2 className="w-3 h-3 animate-spin" />
                  正在校验主题包…
                </p>
              )}
              {validation?.valid && (
                <p data-testid="theme-validate-ok" className="mt-1.5 text-[11px] text-accent-700">包可通过校验，可以导入</p>
              )}
              {validation && !validation.valid && (
                <ul data-testid="theme-validate-errors" className="mt-2 space-y-1 rounded-lg border border-red-200/80 bg-red-50/70 px-3 py-2">
                  {validation.errors.map((issue, index) => (
                    <li key={`${issue.stage}-${index}`} className="text-[11px] text-red-700 leading-relaxed">
                      <span className="font-medium">{issue.stage}</span>
                      {issue.path ? ` · ${issue.path}` : ''}
                      {issue.message ? `：${issue.message}` : ''}
                    </li>
                  ))}
                </ul>
              )}
            </FormField>
          )}

          <div className="flex justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={handleClose}
              disabled={submitting}
              className="h-9 px-4 rounded-lg text-sm text-foreground-600 hover:bg-background-100 disabled:opacity-40 transition-colors duration-150 whitespace-nowrap"
            >
              取消
            </button>
            <button
              type="submit"
              disabled={!canSubmit || submitting}
              className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 disabled:opacity-40 disabled:cursor-not-allowed transition-all duration-150 flex items-center gap-1.5 whitespace-nowrap"
            >
              {submitting || (tab === 'zip' && validating) ? <Loader2 className="w-4 h-4 animate-spin" /> : <Import className="w-4 h-4" />}
              {tab === 'zip' && validating ? '校验中' : '导入'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
