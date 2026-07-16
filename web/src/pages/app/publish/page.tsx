// ============================================================
// nav.ax — Publish & Domain Page (merged)
// ============================================================

import { useState, useEffect, useRef } from 'react';
import { Link } from 'react-router-dom';
import { Globe, Copy, Check, ExternalLink, Eye, ShieldCheck, Loader2, X, Clock, AlertTriangle } from 'lucide-react';
import { useMyPage, usePublish, useUnpublish, useSubdomain, useApplySubdomain, useCancelSubdomainApplication, useUpdatePublication } from '@/hooks/useQueries';
import { LoadingSkeleton, ErrorState } from '@/components/base/SharedUI';
import { useToast } from '@/components/base/Toast';
import { cn } from '@/lib/utils';
import type { SubdomainStatus, Visibility } from '@/api/types';

export default function PublishPage() {
  const { data: page, isLoading, isError, error, refetch } = useMyPage();
  const { mutate: publishMutation, isPending: publishing } = usePublish();
  const { mutate: unpublishMutation, isPending: unpublishing } = useUnpublish();
  const updatePublication = useUpdatePublication();
  const { data: subdomainInfo } = useSubdomain();
  const applyMutation = useApplySubdomain();
  const cancelMutation = useCancelSubdomainApplication();
  const { toast } = useToast();

  const [copied, setCopied] = useState(false);
  const [domainInput, setDomainInput] = useState('');
  const [applying, setApplying] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const [domainError, setDomainError] = useState('');
  const [visibility, setVisibility] = useState<Visibility>('unlisted');
  const [publicationSlug, setPublicationSlug] = useState('');
  const [showAuthor, setShowAuthor] = useState(true);
  const toastShown = useRef(false);

  const publication = page?.publication;
  const isPublished = publication?.published ?? false;
  const hasChanges = publication?.hasUnpublishedChanges ?? false;
  const slug = publication?.slug || 'demo';
  const shareUrl = typeof window !== 'undefined'
    ? `${window.location.origin}/u/${slug}`
    : `/u/${slug}`;
  const subdomain = subdomainInfo?.label ?? subdomainInfo?.subdomain ?? '';
  const customDomainUrl = subdomainInfo?.fullDomain ? `https://${subdomainInfo.fullDomain}` : (subdomain ? `https://${subdomain}.nav.ax` : '');

  useEffect(() => {
    if (!publication) return;
    setVisibility(publication.visibility);
    setPublicationSlug(publication.slug);
    setShowAuthor(publication.showAuthor);
  }, [publication]);

  const handleTogglePublish = () => {
    if (isPublished) {
      unpublishMutation(undefined, {
        onSuccess: () => { toast('success', '已取消发布'); toastShown.current = true; },
        onError: (e: Error) => { toast('error', e.message || '取消发布失败'); toastShown.current = true; },
      });
    } else {
      publishMutation(undefined, {
        onSuccess: () => { toast('success', '发布成功！'); toastShown.current = true; },
        onError: (e: Error) => { toast('error', e.message || '发布失败'); toastShown.current = true; },
      });
    }
  };

  const handleCopy = async () => {
    const url = customDomainUrl || shareUrl;
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast('error', '复制失败');
    }
  };

  const handleApplyDomain = (e: React.FormEvent) => {
    e.preventDefault();
    if (!domainInput.trim()) return;
    if (!/^[a-z0-9-]+$/.test(domainInput)) {
      setDomainError('只能包含小写字母、数字和连字符');
      return;
    }
    setDomainError('');
    setApplying(true);
    applyMutation.mutate({ label: domainInput.trim() }, {
      onSuccess: () => { toast('success', '申请已提交，等待审核'); setDomainInput(''); setApplying(false); },
      onError: (e: Error) => { toast('error', e.message || '申请失败'); setApplying(false); },
    });
  };

  const handleCancel = () => {
    setCancelling(true);
    cancelMutation.mutate(undefined, {
      onSuccess: () => { toast('success', '已取消申请'); setCancelling(false); },
      onError: (e: Error) => { toast('error', e.message || '取消失败'); setCancelling(false); },
    });
  };

  const handleSavePublication = () => {
    updatePublication.mutate({
      visibility,
      slug: publicationSlug.trim(),
      showAuthor,
      seoTitle: page?.title,
      seoDescription: page?.description,
    }, {
      onSuccess: () => toast('success', '发布设置已保存'),
      onError: (cause: Error) => toast('error', cause.message || '发布设置保存失败'),
    });
  };

  useEffect(() => {
    if (!applyMutation.isPending && applying) setApplying(false);
  }, [applyMutation.isPending, applying]);
  useEffect(() => {
    if (!cancelMutation.isPending && cancelling) setCancelling(false);
  }, [cancelMutation.isPending, cancelling]);

  if (isLoading) return <LoadingSkeleton count={4} />;
  if (isError || !page) {
    return <ErrorState message={error instanceof Error ? error.message : '加载失败'} onRetry={() => refetch()} />;
  }

  const status = subdomainInfo?.status as SubdomainStatus | undefined;

  const statusMap: Record<SubdomainStatus, { label: string; color: string; icon: React.ReactNode }> = {
    none: { label: '未申请', color: 'text-foreground-400', icon: <Globe className="w-4 h-4" /> },
    pending: { label: '审核中', color: 'text-accent-600', icon: <Loader2 className="w-4 h-4 animate-spin" /> },
    approved: { label: '已通过', color: 'text-green-600', icon: <ShieldCheck className="w-4 h-4" /> },
    rejected: { label: '未通过', color: 'text-red-500', icon: <X className="w-4 h-4" /> },
  };
  const s = status ? statusMap[status] : statusMap.none;

  return (
    <div className="space-y-5">
      {/* Page header */}
      <div>
        <h2 className="text-xl font-semibold text-foreground-900">发布 & 域名</h2>
        <p className="text-sm text-foreground-500 mt-0.5">控制导航页的公开状态并管理自定义域名</p>
      </div>

      {/* --- Publish Toggle --- */}
      <div className="bg-background-50 border border-background-200/70 rounded-xl p-5 space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-sm font-semibold text-foreground-900">发布状态</h3>
            <p className="text-xs text-foreground-400 mt-0.5">
              {isPublished ? '导航页已对外公开，任何人都可以访问' : '导航页未发布，只有你能看到'}
            </p>
          </div>
          <button
            onClick={handleTogglePublish}
            disabled={publishing || unpublishing}
            className={cn(
              'relative w-14 h-8 rounded-full transition-colors duration-200 flex items-center',
              isPublished ? 'bg-green-500' : 'bg-background-200',
              (publishing || unpublishing) && 'opacity-60 cursor-not-allowed'
            )}
            aria-label={isPublished ? '取消发布' : '发布'}
          >
            <span
              className={cn(
                'absolute top-1 w-6 h-6 bg-white rounded-full shadow transition-transform duration-200',
                isPublished ? 'translate-x-7' : 'translate-x-1'
              )}
            />
          </button>
        </div>

        {/* Status summary */}
        <div className="flex items-center gap-4 text-xs text-foreground-400">
          <span className="flex items-center gap-1.5">
            {isPublished ? (
              <>
                <Eye className="w-3.5 h-3.5 text-green-500" />
                已发布
              </>
            ) : (
              <>
                <ShieldCheck className="w-3.5 h-3.5 text-foreground-300" />
                未发布
              </>
            )}
          </span>
          {publication?.publishedAt && (
            <span className="flex items-center gap-1.5">
              <Clock className="w-3.5 h-3.5" />
              上次发布 {new Date(publication.publishedAt).toLocaleDateString('zh-CN')}
            </span>
          )}
          {hasChanges && (
            <span className="text-accent-600">存在未发布的更改</span>
          )}
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
        {/* Left: Page Info + Domain */}
        <div className="lg:col-span-2 space-y-5">
          {/* Page info */}
          <div className="bg-background-50 border border-background-200/70 rounded-xl p-5 space-y-4">
            <h3 className="text-sm font-semibold text-foreground-900">页面信息</h3>
            <div className="space-y-3">
              <div>
                <label className="text-xs text-foreground-400 block mb-1">页面标题</label>
                <div className="text-sm font-medium text-foreground-800">{page.title}</div>
              </div>
              <div>
                <label className="text-xs text-foreground-400 block mb-1">公开链接</label>
                <div className="flex items-center gap-2">
                  <Link
                    to={`/u/${slug}`}
                    className="text-sm text-primary-600 hover:underline truncate"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    /u/{slug}
                  </Link>
                  <button onClick={handleCopy} className="text-foreground-400 hover:text-foreground-600 transition-colors">
                    {copied ? <Check className="w-3.5 h-3.5 text-green-500" /> : <Copy className="w-3.5 h-3.5" />}
                  </button>
                </div>
              </div>
              <div>
                <label className="text-xs text-foreground-400 block mb-1">描述</label>
                <div className="text-sm text-foreground-600">{page.description || '无描述'}</div>
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2">
                <label className="text-xs text-foreground-500">
                  可见性
                  <select
                    value={visibility}
                    onChange={event => setVisibility(event.target.value as Visibility)}
                    className="mt-1 w-full h-9 px-3 rounded-lg bg-background-50 border border-background-200/70 text-sm text-foreground-900"
                  >
                    <option value="private">私密</option>
                    <option value="unlisted">知道链接即可访问</option>
                    <option value="public">公开展示</option>
                  </select>
                </label>
                <label className="text-xs text-foreground-500">
                  页面标识
                  <input
                    value={publicationSlug}
                    onChange={event => setPublicationSlug(event.target.value)}
                    className="mt-1 w-full h-9 px-3 rounded-lg bg-background-50 border border-background-200/70 text-sm text-foreground-900"
                  />
                </label>
              </div>
              <div className="flex items-center justify-between pt-1">
                <label className="inline-flex items-center gap-2 text-sm text-foreground-600">
                  <input type="checkbox" checked={showAuthor} onChange={event => setShowAuthor(event.target.checked)} />
                  在公开页展示作者
                </label>
                <button
                  onClick={handleSavePublication}
                  disabled={updatePublication.isPending || !publicationSlug.trim()}
                  className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 text-sm font-medium disabled:opacity-50"
                >
                  {updatePublication.isPending ? '保存中…' : '保存发布设置'}
                </button>
              </div>
            </div>
          </div>

          {/* Domain Management */}
          <div className="bg-background-50 border border-background-200/70 rounded-xl p-5 space-y-4">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold text-foreground-900">自定义域名</h3>
              <span className={cn('flex items-center gap-1 text-xs', s.color)}>
                {s.icon}
                {s.label}
              </span>
            </div>

            {status === 'approved' && subdomain ? (
              <div className="space-y-3">
                <div className="flex items-center gap-2">
                  <Globe className="w-4 h-4 text-green-500" />
                  <a
                    href={customDomainUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm font-medium text-primary-600 hover:underline"
                  >
                    {customDomainUrl}
                  </a>
                </div>
                <button
                  onClick={handleCopy}
                  className="inline-flex items-center gap-1.5 text-xs text-foreground-500 hover:text-foreground-700 transition-colors"
                >
                  {copied ? <Check className="w-3.5 h-3.5 text-green-500" /> : <Copy className="w-3.5 h-3.5" />}
                  复制链接
                </button>
              </div>
            ) : status === 'pending' ? (
              <div className="space-y-3">
                <div className="flex items-center gap-2 text-sm text-foreground-600">
                  <Loader2 className="w-4 h-4 animate-spin text-accent-500" />
                  <span>你申请的域名 <strong>{subdomain}.nav.ax</strong> 正在审核中...</span>
                </div>
                <button
                  onClick={handleCancel}
                  disabled={cancelling}
                  className="inline-flex items-center gap-1.5 text-xs text-red-500 hover:text-red-600 transition-colors disabled:opacity-50"
                >
                  <X className="w-3.5 h-3.5" />
                  {cancelling ? '取消中...' : '取消申请'}
                </button>
              </div>
            ) : status === 'rejected' ? (
              <div className="space-y-3">
                <div className="bg-red-50 border border-red-100 rounded-lg p-3">
                  <div className="flex items-start gap-2">
                    <AlertTriangle className="w-4 h-4 text-red-500 mt-0.5 flex-shrink-0" />
                    <div>
                      <p className="text-sm text-red-700 font-medium">域名申请未通过</p>
                      <p className="text-xs text-red-500 mt-0.5">
                        {subdomainInfo?.reason || subdomainInfo?.rejectionReason || '请重新提交申请'}
                      </p>
                    </div>
                  </div>
                </div>
                <form onSubmit={handleApplyDomain} className="flex items-center gap-2">
                  <span className="text-sm text-foreground-400">https://</span>
                  <input
                    type="text"
                    value={domainInput}
                    onChange={e => { setDomainInput(e.target.value); setDomainError(''); }}
                    placeholder="your-name"
                    className="flex-1 h-9 px-3 rounded-lg bg-background-50 border border-background-200/70 text-sm focus:outline-none focus:border-primary-300 focus:ring-1 focus:ring-primary-200"
                  />
                  <span className="text-sm text-foreground-400">.nav.ax</span>
                  <button
                    type="submit"
                    disabled={applying || !domainInput.trim()}
                    className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-150 whitespace-nowrap"
                  >
                    {applying ? '申请中...' : '重新申请'}
                  </button>
                </form>
                {domainError && <p className="text-xs text-red-500">{domainError}</p>}
              </div>
            ) : (
              <div className="space-y-3">
                <p className="text-sm text-foreground-500">
                  申请专属域名，让你的导航页拥有自己的地址，例如 <strong>lucas.nav.ax</strong>
                </p>
                <form onSubmit={handleApplyDomain} className="flex items-center gap-2">
                  <span className="text-sm text-foreground-400">https://</span>
                  <input
                    type="text"
                    value={domainInput}
                    onChange={e => { setDomainInput(e.target.value); setDomainError(''); }}
                    placeholder="your-name"
                    className="flex-1 h-9 px-3 rounded-lg bg-background-50 border border-background-200/70 text-sm focus:outline-none focus:border-primary-300 focus:ring-1 focus:ring-primary-200"
                  />
                  <span className="text-sm text-foreground-400">.nav.ax</span>
                  <button
                    type="submit"
                    disabled={applying || !domainInput.trim()}
                    className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-150 whitespace-nowrap"
                  >
                    {applying ? '申请中...' : '申请域名'}
                  </button>
                </form>
                {domainError && <p className="text-xs text-red-500">{domainError}</p>}
              </div>
            )}
          </div>
        </div>

        {/* Right: URLs */}
        <div className="space-y-5">
          <div className="bg-background-50 border border-background-200/70 rounded-xl p-5 space-y-4">
            <h3 className="text-sm font-semibold text-foreground-900">访问地址</h3>
            <div className="space-y-3">
              {customDomainUrl && (
                <div>
                  <div className="text-xs text-foreground-400 mb-1">专属域名</div>
                  <div className="flex items-center gap-2">
                    <ShieldCheck className="w-3.5 h-3.5 text-green-500" />
                    <a href={customDomainUrl} target="_blank" rel="noopener noreferrer" className="text-sm text-primary-600 hover:underline truncate">
                      {customDomainUrl}
                    </a>
                  </div>
                </div>
              )}
              <div>
                <div className="text-xs text-foreground-400 mb-1">公开地址</div>
                <div className="flex items-center gap-2">
                  <Globe className="w-3.5 h-3.5 text-foreground-300" />
                  <Link to={`/u/${slug}`} target="_blank" rel="noopener noreferrer" className="text-sm text-foreground-600 hover:underline truncate">
                    /u/{slug}
                  </Link>
                </div>
              </div>
            </div>
          </div>

          {/* Tips */}
          <div className="bg-secondary-50 border border-secondary-100 rounded-xl p-4 space-y-2">
            <h4 className="text-xs font-semibold text-secondary-800">小贴士</h4>
            <ul className="text-xs text-secondary-600 space-y-1 list-disc list-inside">
              <li>发布后才能通过公开地址访问</li>
              <li>自定义域名需审核通过后生效</li>
              <li>取消发布不会影响域名状态</li>
              <li>更改导航内容后记得重新发布</li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
}
