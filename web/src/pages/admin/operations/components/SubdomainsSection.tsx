import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, Globe2, RotateCw, ShieldX, X } from 'lucide-react';
import { adminApi } from '@/api/admin';
import type { AdminSubdomainRequest, ContractSubdomainStatus, SubdomainReviewRequest } from '@/api/types';
import { EmptyState, ErrorState, LoadingSkeleton } from '@/components/base/SharedUI';
import { FormField, FormSelect, FormTextarea } from '@/components/base/FormField';
import { useToast } from '@/components/base/Toast';

const statusLabels: Record<ContractSubdomainStatus, string> = { pending: '待审核', approved: '已批准', rejected: '已拒绝', revoked: '已撤销' };
const statusStyles: Record<ContractSubdomainStatus, string> = { pending: 'bg-primary-50 text-primary-700', approved: 'bg-accent-50 text-accent-700', rejected: 'bg-red-50 text-red-600', revoked: 'bg-background-100 text-foreground-500' };

function ReviewDialog({ request, decision, onClose }: { request: AdminSubdomainRequest; decision: SubdomainReviewRequest['decision']; onClose: () => void }) {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [reason, setReason] = useState('');
  const labels = { approve: '批准', reject: '拒绝', revoke: '撤销' } as const;
  const mutation = useMutation({
    mutationFn: () => adminApi.reviewSubdomainRequest(request.id, { decision, ...(reason.trim() ? { reason: reason.trim() } : {}) }),
    onSuccess: async () => { await queryClient.invalidateQueries({ queryKey: ['admin', 'operations', 'subdomains'] }); toast('success', `子域名申请已${labels[decision]}`); onClose(); },
    onError: (error: Error) => toast('error', error.message || '审核子域名申请失败'),
  });
  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center p-4"><button aria-label="关闭审核窗口" className="absolute inset-0 bg-black/30" onClick={onClose} /><div className="relative bg-background-50 rounded-xl shadow-overlay border border-background-200/70 p-5 w-full max-w-md"><div className="flex items-start"><div className="flex-1"><h3 className="text-base font-semibold text-foreground-900">{labels[decision]}子域名申请</h3><p className="text-xs text-foreground-400 mt-1">{request.username ?? request.userId} · {request.fullDomain}</p></div><button onClick={onClose} className="w-8 h-8 rounded-lg flex items-center justify-center hover:bg-background-100"><X className="w-4 h-4" /></button></div><FormField label="审核说明（可选）" className="mt-4"><FormTextarea rows={4} maxLength={300} value={reason} onChange={event => setReason(event.target.value)} placeholder="记录审核原因，最多 300 字" /></FormField><div className="mt-4 flex justify-end gap-2"><button onClick={onClose} className="h-8 px-3 rounded-lg text-xs text-foreground-500 hover:bg-background-100">取消</button><button onClick={() => mutation.mutate()} disabled={mutation.isPending} className={`h-8 px-3 rounded-lg text-xs font-medium text-white flex items-center gap-1.5 disabled:opacity-50 ${decision === 'approve' ? 'bg-primary-500' : 'bg-red-600'}`}>{mutation.isPending ? <RotateCw className="w-3.5 h-3.5 animate-spin" /> : decision === 'approve' ? <Check className="w-3.5 h-3.5" /> : <ShieldX className="w-3.5 h-3.5" />}{labels[decision]}</button></div></div></div>
  );
}

export default function SubdomainsSection() {
  const [status, setStatus] = useState<ContractSubdomainStatus | ''>('pending');
  const [page, setPage] = useState(1);
  const [review, setReview] = useState<{ request: AdminSubdomainRequest; decision: SubdomainReviewRequest['decision'] } | null>(null);
  const pageSize = 20;
  const query = useQuery({
    queryKey: ['admin', 'operations', 'subdomains', status, page],
    queryFn: async () => (await adminApi.getSubdomainRequests({ status: status || undefined, page, pageSize })).data,
  });
  if (query.isLoading) return <LoadingSkeleton count={4} />;
  if (query.error || !query.data) return <ErrorState message={(query.error as Error)?.message || '加载子域名申请失败'} onRetry={() => query.refetch()} />;
  const totalPages = query.data.totalPages;

  return (
    <>
      <section className="bg-white rounded-xl border border-background-200/70 overflow-hidden">
        <div className="p-4 border-b border-background-200/70 flex items-center justify-between gap-3"><div><h3 className="text-sm font-semibold text-foreground-800">子域名申请</h3><p className="text-xs text-foreground-400 mt-0.5">共 {query.data.total} 条记录</p></div><FormSelect value={status} onChange={event => { setStatus(event.target.value as ContractSubdomainStatus | ''); setPage(1); }} className="w-32 h-8 text-xs"><option value="">全部状态</option><option value="pending">待审核</option><option value="approved">已批准</option><option value="rejected">已拒绝</option><option value="revoked">已撤销</option></FormSelect></div>
        {query.data.items.length === 0 ? <EmptyState icon={Globe2} title="没有匹配的申请" description="调整状态筛选后再试。" /> : <div className="overflow-x-auto"><table className="w-full text-left"><thead><tr className="bg-background-50 border-b border-background-200/70 text-[10px] text-foreground-400"><th className="px-4 py-2.5 font-medium">用户</th><th className="px-4 py-2.5 font-medium">域名</th><th className="px-4 py-2.5 font-medium">状态</th><th className="px-4 py-2.5 font-medium">申请时间</th><th className="px-4 py-2.5 font-medium">说明</th><th className="px-4 py-2.5 font-medium text-right">审核</th></tr></thead><tbody>{query.data.items.map(item => <tr key={item.id} className="border-b border-background-100 last:border-0"><td className="px-4 py-3"><p className="text-xs font-medium text-foreground-700">{item.username ?? '未知用户'}</p><p className="text-[10px] font-mono text-foreground-400 mt-0.5">{item.userId}</p></td><td className="px-4 py-3 text-xs font-mono text-foreground-700">{item.fullDomain}</td><td className="px-4 py-3"><span className={`text-[10px] px-2 py-0.5 rounded-full ${statusStyles[item.status]}`}>{statusLabels[item.status]}</span></td><td className="px-4 py-3 text-xs text-foreground-500 whitespace-nowrap">{new Date(item.appliedAt).toLocaleString('zh-CN')}</td><td className="px-4 py-3 text-xs text-foreground-500 max-w-48 truncate" title={item.reason}>{item.reason || '—'}</td><td className="px-4 py-3"><div className="flex justify-end gap-1">{item.status === 'pending' ? <><button onClick={() => setReview({ request: item, decision: 'approve' })} className="h-7 px-2.5 rounded-md text-xs text-accent-700 hover:bg-accent-50">批准</button><button onClick={() => setReview({ request: item, decision: 'reject' })} className="h-7 px-2.5 rounded-md text-xs text-red-600 hover:bg-red-50">拒绝</button></> : item.status === 'approved' ? <button onClick={() => setReview({ request: item, decision: 'revoke' })} className="h-7 px-2.5 rounded-md text-xs text-red-600 hover:bg-red-50">撤销</button> : <span className="text-xs text-foreground-300">已处理</span>}</div></td></tr>)}</tbody></table></div>}
        {totalPages > 1 ? <div className="p-3 border-t border-background-200/70 flex items-center justify-between"><span className="text-xs text-foreground-400">第 {page} / {totalPages} 页</span><div className="flex gap-1.5"><button onClick={() => setPage(value => Math.max(1, value - 1))} disabled={page <= 1} className="h-7 px-2.5 rounded-md border border-background-200 text-xs disabled:opacity-40">上一页</button><button onClick={() => setPage(value => Math.min(totalPages, value + 1))} disabled={page >= totalPages} className="h-7 px-2.5 rounded-md border border-background-200 text-xs disabled:opacity-40">下一页</button></div></div> : null}
      </section>
      {review ? <ReviewDialog request={review.request} decision={review.decision} onClose={() => setReview(null)} /> : null}
    </>
  );
}
