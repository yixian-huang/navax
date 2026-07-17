// ============================================================
// nav.ax — Publish & Domain Page (merged)
// ============================================================

import { useState, useEffect } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { Globe, Copy, Check, Eye, ShieldCheck, Loader2, X, AlertTriangle } from 'lucide-react';
import {
  usePublish,
  useUnpublish,
  useSubdomain,
  useApplySubdomain,
  useCancelSubdomainApplication,
  useUpdatePublication,
} from '@/hooks/useQueries';
import { usePublishUiState } from '@/hooks/usePublishUiState';
import { previewPath, toastForPublishSuccess } from '@/lib/publish-actions';
import { LoadingSkeleton, ErrorState } from '@/components/base/SharedUI';
import { useToast } from '@/components/base/Toast';
import { cn } from '@/lib/utils';
import { request } from '@/api/client';
import type { ApiResponse, SubdomainStatus, Visibility } from '@/api/types';

function formatTime(iso: string | null | undefined): string | null {
  if (!iso) return null;
  return new Date(iso).toLocaleString('zh-CN');
}

export default function PublishPage() {
  const [searchParams] = useSearchParams();
  const {
    state,
    scope,
    slug: stateSlug,
    isLoading,
    isError,
    publication,
    page,
    refetch,
  } = usePublishUiState('publish_page');
  const { mutate: publishMutation, isPending: publishing } = usePublish();
  const { mutate: unpublishMutation, isPending: unpublishing } = useUnpublish();
  const updatePublication = useUpdatePublication();
  const { data: subdomainInfo, refetch: refetchSubdomain } = useSubdomain();
  const applyMutation = useApplySubdomain();
  const cancelMutation = useCancelSubdomainApplication();
  const { toast } = useToast();

  const [copied, setCopied] = useState(false);
  const [domainInput, setDomainInput] = useState('');
  const [cnameInput, setCnameInput] = useState('');
  const [applying, setApplying] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const [savingCname, setSavingCname] = useState(false);
  const [domainError, setDomainError] = useState('');
  const [visibility, setVisibility] = useState<Visibility>('unlisted');
  const [publicationSlug, setPublicationSlug] = useState('');
  const [showAuthor, setShowAuthor] = useState(true);
  const [seoTitle, setSeoTitle] = useState('');
  const [seoDescription, setSeoDescription] = useState('');
  const [highlightVisibility, setHighlightVisibility] = useState(false);

  const isPublished = publication?.published ?? false;
  const slug = stateSlug || publication?.slug || 'demo';
  const shareUrl = typeof window !== 'undefined'
    ? `${window.location.origin}/u/${slug}`
    : `/u/${slug}`;
  const subdomain = subdomainInfo?.label ?? subdomainInfo?.subdomain ?? '';
  const subdomainUrl = subdomainInfo?.fullDomain ? `https://${subdomainInfo.fullDomain}` : (subdomain ? `https://${subdomain}.nav.ax` : '');
  const cnameHost = subdomainInfo?.customDomain ?? '';
  const customDomainUrl = cnameHost ? `https://${cnameHost}` : subdomainUrl;
  const rootSuffix = subdomainInfo?.fullDomain?.includes('.')
    ? subdomainInfo.fullDomain.split('.').slice(1).join('.')
    : 'nav.ax';

  useEffect(() => {
    if (!publication) return;
    setVisibility(publication.visibility);
    setPublicationSlug(publication.slug);
    setShowAuthor(publication.showAuthor);
    setSeoTitle(publication.seoTitle || page?.title || '');
    setSeoDescription(publication.seoDescription || page?.description || '');
  }, [publication, page?.title, page?.description]);

  useEffect(() => {
    setCnameInput(cnameHost || '');
  }, [cnameHost]);

  useEffect(() => {
    if (searchParams.get('highlight') !== 'visibility') return;
    const timer = window.setTimeout(() => {
      const el = document.getElementById('publication-visibility');
      if (!el) return;
      el.scrollIntoView({ behavior: 'smooth', block: 'center' });
      el.focus();
      setHighlightVisibility(true);
    }, 100);
    const clearHighlight = window.setTimeout(() => setHighlightVisibility(false), 3200);
    return () => {
      window.clearTimeout(timer);
      window.clearTimeout(clearHighlight);
    };
  }, [searchParams]);

  const handlePublish = () => {
    if (state.primaryDisabled || state.primaryAction === 'none') return;
    const stateBefore = state;
    publishMutation(undefined, {
      onSuccess: () => toast('success', toastForPublishSuccess(stateBefore)),
      onError: (e: Error) => toast('error', e.message || '发布失败'),
    });
  };

  const handleUnpublish = () => {
    if (!window.confirm('取消后公开链接将不可访问；草稿保留。确定取消发布？')) return;
    unpublishMutation(undefined, {
      onSuccess: () => toast('success', '已取消发布'),
      onError: (e: Error) => toast('error', e.message || '取消发布失败'),
    });
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

  const handleSaveCname = async () => {
    setSavingCname(true);
    try {
      const value = cnameInput.trim();
      await request<ApiResponse<unknown>>('/me/subdomain', {
        method: 'PATCH',
        body: { customDomain: value || null },
      });
      await refetchSubdomain();
      toast('success', value ? 'CNAME 域名已保存' : '已清除 CNAME 域名');
    } catch (cause) {
      toast('error', cause instanceof Error ? cause.message : '保存 CNAME 失败');
    } finally {
      setSavingCname(false);
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
      seoTitle: seoTitle.trim(),
      seoDescription: seoDescription.trim(),
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
    return <ErrorState message="加载失败" onRetry={() => refetch()} />;
  }

  const status = subdomainInfo?.status as SubdomainStatus | undefined;

  const statusMap: Record<SubdomainStatus, { label: string; color: string; icon: React.ReactNode }> = {
    none: { label: '未申请', color: 'text-foreground-400', icon: <Globe className="w-4 h-4" /> },
    pending: { label: '审核中', color: 'text-accent-600', icon: <Loader2 className="w-4 h-4 animate-spin" /> },
    approved: { label: '已通过', color: 'text-green-600', icon: <ShieldCheck className="w-4 h-4" /> },
    rejected: { label: '未通过', color: 'text-red-500', icon: <X className="w-4 h-4" /> },
  };
  const s = status ? statusMap[status] : statusMap.none;

  const draftTime = formatTime(state.draftUpdatedAt);
  const publishedTime = formatTime(state.publishedAt);
  const primaryDisabled = publishing || state.primaryDisabled || state.primaryAction === 'none';

  return (
    <div className="space-y-5">
      {/* Page header */}
      <div>
        <h2 className="text-xl font-semibold text-foreground-900">发布 & 域名</h2>
        <p className="text-sm text-foreground-500 mt-0.5">先确认内容上线，再管理访问方式与域名</p>
      </div>

      {/* --- Primary publish status card --- */}
      <div className="bg-background-50 border border-background-200/70 rounded-xl p-5 space-y-4">
        <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-4">
          <div className="min-w-0">
            <h3
              className={cn(
                'text-base font-semibold',
                state.id === 'published_with_draft' && 'text-accent-700',
                state.id === 'published_current' && 'text-green-700',
                state.id === 'never_published' && 'text-foreground-900',
              )}
            >
              {state.shortLabel}
            </h3>
            <p className="text-xs text-foreground-400 mt-1">
              {draftTime && <>草稿更新于 {draftTime}</>}
              {draftTime && publishedTime && ' · '}
              {publishedTime && <>上次发布 {publishedTime}</>}
              {!draftTime && !publishedTime && '尚无草稿或发布记录'}
            </p>
            {state.id === 'published_with_draft' && (
              <p className="text-xs text-accent-600 mt-2">当前访客仍看到线上版</p>
            )}
            {state.blockReason && (
              <p className="text-xs text-red-500 mt-2" id="visibility-hint">
                {state.blockReason}
              </p>
            )}
          </div>

          <div className="flex flex-col items-stretch sm:items-end gap-2 shrink-0">
            <button
              type="button"
              onClick={handlePublish}
              disabled={primaryDisabled}
              className={cn(
                'h-10 px-5 rounded-lg text-sm font-medium transition-colors duration-150 inline-flex items-center justify-center gap-1.5',
                state.primaryAction === 'none'
                  ? 'bg-background-200 text-foreground-500 cursor-default'
                  : 'bg-primary-500 text-background-50 dark:text-foreground-950 hover:bg-primary-600 disabled:opacity-50 disabled:cursor-not-allowed',
              )}
            >
              {publishing && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
              {publishing ? '发布中…' : state.primaryLabel}
            </button>

            <div className="flex flex-wrap gap-x-3 gap-y-1.5 justify-end text-xs">
              <Link
                to={previewPath(scope)}
                className="text-foreground-500 hover:text-foreground-700 inline-flex items-center gap-1 transition-colors duration-150"
              >
                <Eye className="w-3.5 h-3.5" />
                草稿预览
              </Link>
              {isPublished && (
                <Link
                  to={`/u/${slug}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-primary-600 hover:text-primary-700 inline-flex items-center gap-1 transition-colors duration-150"
                >
                  <Globe className="w-3.5 h-3.5" />
                  打开线上版
                </Link>
              )}
              {state.showUnpublish && (
                <button
                  type="button"
                  onClick={handleUnpublish}
                  disabled={unpublishing}
                  className="text-red-500 hover:text-red-600 disabled:opacity-50 transition-colors duration-150"
                >
                  {unpublishing ? '取消中…' : '取消发布'}
                </button>
              )}
            </div>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
        {/* Left: Page Info + Domain */}
        <div className="lg:col-span-2 space-y-5">
          {/* Publication settings */}
          <div className="bg-background-50 border border-background-200/70 rounded-xl p-5 space-y-4">
            <h3 className="text-sm font-semibold text-foreground-900">发布设置</h3>
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
                    id="publication-visibility"
                    value={visibility}
                    onChange={event => setVisibility(event.target.value as Visibility)}
                    className={cn(
                      'mt-1 w-full h-9 px-3 rounded-lg bg-background-50 border border-background-200/70 text-sm text-foreground-900 transition-shadow duration-300',
                      highlightVisibility && 'ring-2 ring-primary-300 border-primary-300',
                    )}
                    aria-describedby={state.blockReason ? 'visibility-hint' : undefined}
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
              <label className="text-xs text-foreground-500 block">
                SEO 标题（搜索/分享，留空则用页面标题）
                <input
                  value={seoTitle}
                  onChange={event => setSeoTitle(event.target.value)}
                  maxLength={70}
                  className="mt-1 w-full h-9 px-3 rounded-lg bg-background-50 border border-background-200/70 text-sm text-foreground-900"
                />
              </label>
              <label className="text-xs text-foreground-500 block">
                SEO 描述
                <textarea
                  value={seoDescription}
                  onChange={event => setSeoDescription(event.target.value)}
                  maxLength={160}
                  rows={2}
                  className="mt-1 w-full px-3 py-2 rounded-lg bg-background-50 border border-background-200/70 text-sm text-foreground-900 resize-none"
                />
              </label>
              <p className="text-[11px] text-foreground-400">
                分享图（og:image）自动使用主题设置中的背景图；请在「主题」页上传背景后重新发布。
              </p>
              <div className="flex items-center justify-between pt-1 gap-3 flex-wrap">
                <label className="inline-flex items-center gap-2 text-sm text-foreground-600">
                  <input type="checkbox" checked={showAuthor} onChange={event => setShowAuthor(event.target.checked)} />
                  在公开页展示作者
                </label>
                <button
                  onClick={handleSavePublication}
                  disabled={updatePublication.isPending || !publicationSlug.trim()}
                  className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 text-sm font-medium disabled:opacity-50"
                >
                  {updatePublication.isPending ? '保存中…' : '保存设置'}
                </button>
              </div>
              <p className="text-[11px] text-foreground-400">
                保存设置后，若页面已发布，需再点「发布更新」才会进入公开快照（含 slug / SEO 等）。
              </p>
            </div>
          </div>

          {/* Domain Management */}
          <div className="bg-background-50 border border-background-200/70 rounded-xl p-5 space-y-4">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold text-foreground-900">子域名</h3>
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
                    href={subdomainUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm font-medium text-primary-600 hover:underline"
                  >
                    {subdomainUrl}
                  </a>
                </div>
                <button
                  onClick={handleCopy}
                  className="inline-flex items-center gap-1.5 text-xs text-foreground-500 hover:text-foreground-700 transition-colors"
                >
                  {copied ? <Check className="w-3.5 h-3.5 text-green-500" /> : <Copy className="w-3.5 h-3.5" />}
                  复制链接
                </button>
                <div className="rounded-lg border border-background-200/70 p-3 space-y-2">
                  <div className="text-xs font-medium text-foreground-700">扩展：CNAME 自有域名</div>
                  <p className="text-[11px] text-foreground-400 leading-relaxed">
                    在 DNS 将自有域名 CNAME 指向 <code className="text-foreground-600">{subdomainInfo?.fullDomain || `${subdomain}.${rootSuffix}`}</code>，
                    并在此填写主机名（需实例 TLS/反代已覆盖该主机）。
                  </p>
                  <div className="flex items-center gap-2">
                    <input
                      value={cnameInput}
                      onChange={e => setCnameInput(e.target.value)}
                      placeholder="links.example.com"
                      className="flex-1 h-9 px-3 rounded-lg bg-background-50 border border-background-200/70 text-sm"
                    />
                    <button
                      type="button"
                      onClick={handleSaveCname}
                      disabled={savingCname}
                      className="h-9 px-3 rounded-lg bg-primary-500 text-background-50 text-xs font-medium disabled:opacity-50"
                    >
                      {savingCname ? '保存中…' : '保存'}
                    </button>
                  </div>
                </div>
              </div>
            ) : status === 'pending' ? (
              <div className="space-y-3">
                <div className="flex items-center gap-2 text-sm text-foreground-600">
                  <Loader2 className="w-4 h-4 animate-spin text-accent-500" />
                  <span>你申请的子域名 <strong>{subdomain}.{rootSuffix}</strong> 正在审核中...</span>
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
                      <p className="text-sm text-red-700 font-medium">子域名申请未通过</p>
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
                  <span className="text-sm text-foreground-400">.{rootSuffix}</span>
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
                  申请专属子域名，例如 <strong>lucas.{rootSuffix}</strong>；通过后还可扩展 CNAME 自有域名。
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
                  <span className="text-sm text-foreground-400">.{rootSuffix}</span>
                  <button
                    type="submit"
                    disabled={applying || !domainInput.trim()}
                    className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-150 whitespace-nowrap"
                  >
                    {applying ? '申请中...' : '申请子域名'}
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
              {subdomainUrl && (
                <div>
                  <div className="text-xs text-foreground-400 mb-1">子域名</div>
                  <div className="flex items-center gap-2">
                    <ShieldCheck className="w-3.5 h-3.5 text-green-500" />
                    <a href={subdomainUrl} target="_blank" rel="noopener noreferrer" className="text-sm text-primary-600 hover:underline truncate">
                      {subdomainUrl}
                    </a>
                  </div>
                </div>
              )}
              {cnameHost && (
                <div>
                  <div className="text-xs text-foreground-400 mb-1">CNAME 域名</div>
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
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
}
