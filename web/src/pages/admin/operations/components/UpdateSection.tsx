import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, CheckCircle2, Download, RefreshCw, RotateCw, Save, ShieldCheck } from 'lucide-react';
import { adminApi } from '@/api/admin';
import type { UpdateState } from '@/api/types';
import { ErrorState, LoadingSkeleton } from '@/components/base/SharedUI';
import { FormField, FormInput } from '@/components/base/FormField';
import { useToast } from '@/components/base/Toast';

const statusLabels: Record<UpdateState['status'], string> = {
  idle: '空闲', checking: '检查中', available: '有可用更新', downloading: '下载中', applying: '应用中',
  'restart-required': '等待重启', failed: '失败',
};

function makeIdempotencyKey() {
  const suffix = typeof crypto.randomUUID === 'function' ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`;
  return `update-apply-${suffix}`;
}

function UpdateControls({ state }: { state: UpdateState }) {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [autoCheck, setAutoCheck] = useState(state.autoCheck);
  const [autoApply, setAutoApply] = useState(state.autoApply);
  const [maintenanceWindow, setMaintenanceWindow] = useState(state.maintenanceWindow ?? '');
  const [confirmation, setConfirmation] = useState('');
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['admin', 'operations', 'update'] });
  const settingsMutation = useMutation({
    mutationFn: () => adminApi.updateUpdateSettings({ autoCheck, autoApply, maintenanceWindow: maintenanceWindow.trim() || null }),
    onSuccess: async () => { await refresh(); toast('success', '更新策略已保存'); },
    onError: (error: Error) => toast('error', error.message || '保存更新策略失败'),
  });
  const checkMutation = useMutation({
    mutationFn: adminApi.checkForUpdates,
    onSuccess: async response => { await refresh(); toast(response.data.status === 'available' ? 'info' : 'success', response.data.status === 'available' ? `发现版本 ${response.data.latestVersion}` : '当前已是最新版本'); },
    onError: (error: Error) => toast('error', error.message || '检查更新失败'),
  });
  const applyMutation = useMutation({
    mutationFn: (idempotencyKey: string) => adminApi.applyUpdate(state.latestVersion ?? '', idempotencyKey),
    onSuccess: async () => { setConfirmation(''); await refresh(); toast('warning', '更新已进入应用流程，请关注重启状态'); },
    onError: (error: Error) => toast('error', error.message || '应用更新失败'),
  });
  const canApply = state.deployment === 'binary' && state.status === 'available' && Boolean(state.latestVersion);

  return (
    <div className="grid lg:grid-cols-[1fr_1fr] gap-4">
      <section className="bg-white rounded-xl border border-background-200/70 p-4 space-y-4">
        <div><h3 className="text-sm font-semibold text-foreground-800">自动更新策略</h3><p className="text-xs text-foreground-400 mt-0.5">稳定通道 · 维护窗口使用 HH:MM-HH:MM</p></div>
        <label className="flex items-center justify-between gap-4 text-sm text-foreground-600"><span><strong className="font-medium text-foreground-700">自动检查</strong><span className="block text-xs text-foreground-400">定期获取签名更新清单</span></span><input type="checkbox" checked={autoCheck} onChange={event => setAutoCheck(event.target.checked)} className="accent-primary-500" /></label>
        <label className="flex items-center justify-between gap-4 text-sm text-foreground-600"><span><strong className="font-medium text-foreground-700">自动应用</strong><span className="block text-xs text-foreground-400">仅原生二进制部署支持</span></span><input type="checkbox" checked={autoApply} disabled={state.deployment === 'container'} onChange={event => setAutoApply(event.target.checked)} className="accent-primary-500" /></label>
        <FormField label="维护窗口"><FormInput value={maintenanceWindow} onChange={event => setMaintenanceWindow(event.target.value)} placeholder="02:00-04:00" pattern="^([01][0-9]|2[0-3]):[0-5][0-9]-([01][0-9]|2[0-3]):[0-5][0-9]$" /></FormField>
        <div className="flex justify-end"><button onClick={() => settingsMutation.mutate()} disabled={settingsMutation.isPending} className="h-8 px-3 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-xs font-medium flex items-center gap-1.5 disabled:opacity-50">{settingsMutation.isPending ? <RotateCw className="w-3.5 h-3.5 animate-spin" /> : <Save className="w-3.5 h-3.5" />}保存策略</button></div>
      </section>

      <section className="bg-white rounded-xl border border-background-200/70 p-4 space-y-4">
        <div className="flex items-start justify-between gap-3"><div><h3 className="text-sm font-semibold text-foreground-800">版本状态</h3><p className="text-xs text-foreground-400 mt-0.5">{state.deployment} 部署 · {state.channel} 通道</p></div><span className={`text-[10px] px-2 py-1 rounded-full ${state.status === 'failed' ? 'bg-red-50 text-red-600' : state.status === 'available' ? 'bg-primary-50 text-primary-700' : 'bg-background-100 text-foreground-600'}`}>{statusLabels[state.status]}</span></div>
        <div className="grid grid-cols-2 gap-3"><div className="rounded-lg bg-background-50 p-3"><span className="text-[10px] text-foreground-400">当前版本</span><p className="text-sm font-mono font-medium text-foreground-800 mt-1">{state.currentVersion}</p></div><div className="rounded-lg bg-background-50 p-3"><span className="text-[10px] text-foreground-400">最新版本</span><p className="text-sm font-mono font-medium text-foreground-800 mt-1">{state.latestVersion ?? '尚未检查'}</p></div></div>
        <div className="flex items-center justify-between gap-3"><span className="text-xs text-foreground-400">{state.checkedAt ? `上次检查 ${new Date(state.checkedAt).toLocaleString('zh-CN')}` : '尚未执行更新检查'}</span><button onClick={() => checkMutation.mutate()} disabled={checkMutation.isPending || ['checking', 'downloading', 'applying'].includes(state.status)} className="h-8 px-3 rounded-lg border border-background-200 text-xs text-foreground-600 hover:bg-background-50 flex items-center gap-1.5 disabled:opacity-50"><RefreshCw className={`w-3.5 h-3.5 ${checkMutation.isPending ? 'animate-spin' : ''}`} />检查更新</button></div>
        {state.error ? <div className="rounded-lg bg-red-50 border border-red-100 p-3 text-xs text-red-700 flex gap-2"><AlertTriangle className="w-4 h-4 flex-shrink-0" />{state.error}</div> : null}
      </section>

      <section className="lg:col-span-2 bg-white rounded-xl border border-background-200/70 p-4">
        <div className="flex items-start gap-3"><ShieldCheck className="w-5 h-5 text-primary-500 mt-0.5" /><div className="flex-1"><h3 className="text-sm font-semibold text-foreground-800">确认应用更新</h3><p className="text-xs text-foreground-400 mt-1">应用前会自动创建备份。容器部署应通过编排平台更新。</p></div></div>
        {state.releaseNotes ? <div className="mt-3 rounded-lg bg-background-50 p-3 text-xs text-foreground-600 whitespace-pre-wrap max-h-36 overflow-auto">{state.releaseNotes}</div> : null}
        <div className="mt-4 flex flex-col sm:flex-row sm:items-end gap-3"><FormField label="输入 APPLY_UPDATE 确认" className="flex-1"><FormInput value={confirmation} onChange={event => setConfirmation(event.target.value)} placeholder="APPLY_UPDATE" /></FormField><button onClick={() => applyMutation.mutate(makeIdempotencyKey())} disabled={!canApply || confirmation !== 'APPLY_UPDATE' || applyMutation.isPending} className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-xs font-medium flex items-center justify-center gap-1.5 disabled:opacity-40"><Download className="w-3.5 h-3.5" />应用 {state.latestVersion ?? '更新'}</button></div>
        {state.status === 'restart-required' ? <p className="mt-3 text-xs text-accent-700 flex items-center gap-1.5"><CheckCircle2 className="w-3.5 h-3.5" />更新已应用，服务正在等待重启。</p> : null}
      </section>
    </div>
  );
}

export default function UpdateSection() {
  const query = useQuery({
    queryKey: ['admin', 'operations', 'update'],
    queryFn: async () => (await adminApi.getUpdateState()).data,
    refetchInterval: current => ['checking', 'downloading', 'applying'].includes(current.state.data?.status ?? '') ? 2000 : false,
  });
  if (query.isLoading) return <LoadingSkeleton count={3} />;
  if (query.error || !query.data) return <ErrorState message={(query.error as Error)?.message || '加载更新状态失败'} onRetry={() => query.refetch()} />;
  return <UpdateControls key={`${query.data.checkedAt ?? 'never'}:${query.data.status}:${query.data.autoCheck}:${query.data.autoApply}:${query.data.maintenanceWindow ?? ''}:${query.data.latestVersion ?? ''}`} state={query.data} />;
}
