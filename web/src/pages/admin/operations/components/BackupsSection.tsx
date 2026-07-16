import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Archive, Download, HardDriveDownload, KeyRound, RotateCw, ShieldAlert, X } from 'lucide-react';
import { adminApi } from '@/api/admin';
import type { Backup, RestoreToken } from '@/api/types';
import { EmptyState, ErrorState, LoadingSkeleton } from '@/components/base/SharedUI';
import { FormField, FormInput } from '@/components/base/FormField';
import { useToast } from '@/components/base/Toast';

const reasonLabels: Record<Backup['reason'], string> = { manual: '手动', 'pre-update': '更新前', scheduled: '计划任务' };

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MiB`;
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GiB`;
}

function RestoreDialog({ backup, onClose }: { backup: Backup; onClose: () => void }) {
  const { toast } = useToast();
  const [password, setPassword] = useState('');
  const [token, setToken] = useState<RestoreToken | null>(null);
  const [confirmation, setConfirmation] = useState('');
  const tokenMutation = useMutation({
    mutationFn: () => adminApi.createRestoreToken(backup.id, password),
    onSuccess: response => { setPassword(''); setToken(response.data); toast('success', '管理员身份已验证'); },
    onError: (error: Error) => toast('error', error.message || '密码验证失败'),
  });
  const restoreMutation = useMutation({
    mutationFn: () => adminApi.restoreBackup(backup.id, token?.restoreToken ?? ''),
    onSuccess: () => { toast('warning', '恢复已确认，服务将重启并应用备份'); onClose(); },
    onError: (error: Error) => toast('error', error.message || '确认恢复失败'),
  });

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center p-4">
      <button aria-label="关闭恢复窗口" className="absolute inset-0 bg-black/35" onClick={onClose} />
      <div className="relative w-full max-w-lg rounded-xl bg-background-50 shadow-overlay border border-background-200/70 p-5">
        <div className="flex items-start gap-3"><div className="w-9 h-9 rounded-lg bg-red-50 flex items-center justify-center"><ShieldAlert className="w-4 h-4 text-red-600" /></div><div className="flex-1"><h3 className="text-base font-semibold text-foreground-900">恢复实例备份</h3><p className="text-xs text-foreground-400 mt-1">{new Date(backup.createdAt).toLocaleString('zh-CN')} · {formatBytes(backup.size)}</p></div><button onClick={onClose} className="w-8 h-8 rounded-lg flex items-center justify-center hover:bg-background-100"><X className="w-4 h-4 text-foreground-500" /></button></div>
        <div className="mt-4 rounded-lg border border-red-100 bg-red-50 p-3 text-xs text-red-700">恢复会替换当前数据库并触发服务重启。请先确认当前数据已另行备份。</div>
        {!token ? (
          <div className="mt-4 space-y-3"><FormField label="当前管理员密码"><FormInput type="password" autoComplete="current-password" value={password} onChange={event => setPassword(event.target.value)} /></FormField><button onClick={() => tokenMutation.mutate()} disabled={!password || tokenMutation.isPending} className="w-full h-9 rounded-lg bg-foreground-900 text-background-50 text-xs font-medium flex items-center justify-center gap-2 disabled:opacity-40">{tokenMutation.isPending ? <RotateCw className="w-3.5 h-3.5 animate-spin" /> : <KeyRound className="w-3.5 h-3.5" />}验证身份并获取一次性令牌</button></div>
        ) : (
          <div className="mt-4 space-y-3"><div className="rounded-lg bg-accent-50 p-3 text-xs text-accent-700">一次性恢复令牌已安全保存在当前页面内存中，将于 {new Date(token.expiresAt).toLocaleTimeString('zh-CN')} 过期。</div><FormField label="输入 RESTORE_BACKUP 最终确认"><FormInput value={confirmation} onChange={event => setConfirmation(event.target.value)} placeholder="RESTORE_BACKUP" /></FormField><button onClick={() => restoreMutation.mutate()} disabled={confirmation !== 'RESTORE_BACKUP' || restoreMutation.isPending} className="w-full h-9 rounded-lg bg-red-600 text-white text-xs font-medium flex items-center justify-center gap-2 disabled:opacity-40">{restoreMutation.isPending ? <RotateCw className="w-3.5 h-3.5 animate-spin" /> : <ShieldAlert className="w-3.5 h-3.5" />}确认恢复并重启</button></div>
        )}
      </div>
    </div>
  );
}

export default function BackupsSection() {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [restoreBackup, setRestoreBackup] = useState<Backup | null>(null);
  const query = useQuery({ queryKey: ['admin', 'operations', 'backups'], queryFn: async () => (await adminApi.getBackups()).data });
  const createMutation = useMutation({
    mutationFn: adminApi.createBackup,
    onSuccess: async () => { await queryClient.invalidateQueries({ queryKey: ['admin', 'operations', 'backups'] }); toast('success', '手动备份已创建'); },
    onError: (error: Error) => toast('error', error.message || '创建备份失败'),
  });
  const downloadMutation = useMutation({
    mutationFn: adminApi.downloadBackup,
    onSuccess: attachment => {
      const url = URL.createObjectURL(attachment.blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = attachment.filename;
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      setTimeout(() => URL.revokeObjectURL(url), 0);
    },
    onError: (error: Error) => toast('error', error.message || '下载备份失败'),
  });
  if (query.isLoading) return <LoadingSkeleton count={3} />;
  if (query.error || !query.data) return <ErrorState message={(query.error as Error)?.message || '加载备份列表失败'} onRetry={() => query.refetch()} />;

  return (
    <>
      <section className="bg-white rounded-xl border border-background-200/70 overflow-hidden">
        <div className="p-4 border-b border-background-200/70 flex items-center justify-between gap-3"><div><h3 className="text-sm font-semibold text-foreground-800">完整实例备份</h3><p className="text-xs text-foreground-400 mt-0.5">包含数据库、本地上传资源与实例生成密钥</p></div><button onClick={() => createMutation.mutate()} disabled={createMutation.isPending} className="h-8 px-3 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-xs font-medium flex items-center gap-1.5 disabled:opacity-50">{createMutation.isPending ? <RotateCw className="w-3.5 h-3.5 animate-spin" /> : <Archive className="w-3.5 h-3.5" />}创建备份</button></div>
        {query.data.length === 0 ? <EmptyState icon={Archive} title="暂无备份" description="创建首个手动备份，以便在必要时恢复实例。" /> : (
          <div className="overflow-x-auto"><table className="w-full text-left"><thead><tr className="bg-background-50 border-b border-background-200/70 text-[10px] text-foreground-400"><th className="px-4 py-2.5 font-medium">创建时间</th><th className="px-4 py-2.5 font-medium">原因</th><th className="px-4 py-2.5 font-medium">大小</th><th className="px-4 py-2.5 font-medium">SHA-256</th><th className="px-4 py-2.5 font-medium text-right">操作</th></tr></thead><tbody>{query.data.map(backup => <tr key={backup.id} className="border-b border-background-100 last:border-0"><td className="px-4 py-3 text-xs text-foreground-600 whitespace-nowrap">{new Date(backup.createdAt).toLocaleString('zh-CN')}</td><td className="px-4 py-3 text-xs text-foreground-500">{reasonLabels[backup.reason]}</td><td className="px-4 py-3 text-xs text-foreground-500 whitespace-nowrap">{formatBytes(backup.size)}</td><td className="px-4 py-3"><span title={backup.sha256} className="font-mono text-[10px] text-foreground-400">{backup.sha256.slice(0, 12)}…</span></td><td className="px-4 py-3"><div className="flex justify-end gap-1.5"><button onClick={() => downloadMutation.mutate(backup.id)} disabled={downloadMutation.isPending} className="h-7 px-2.5 rounded-md text-xs text-foreground-600 hover:bg-background-100 flex items-center gap-1"><Download className="w-3 h-3" />下载</button><button onClick={() => setRestoreBackup(backup)} className="h-7 px-2.5 rounded-md text-xs text-red-600 hover:bg-red-50 flex items-center gap-1"><HardDriveDownload className="w-3 h-3" />恢复</button></div></td></tr>)}</tbody></table></div>
        )}
      </section>
      {restoreBackup ? <RestoreDialog backup={restoreBackup} onClose={() => setRestoreBackup(null)} /> : null}
    </>
  );
}
