// ============================================================
// nav.ax Domain Settings Page — /app/domain
// ============================================================

import { useState, useEffect, useRef } from 'react';
import { Globe, Loader2, Copy, Check, ExternalLink, X, Clock, AlertTriangle, ShieldCheck, Info } from 'lucide-react';
import { useSubdomain, useApplySubdomain, useCancelSubdomainApplication } from '@/hooks/useQueries';
import { LoadingSkeleton, ErrorState } from '@/components/base/SharedUI';
import { useToast } from '@/components/base/Toast';
import { cn } from '@/lib/utils';
import type { SubdomainInfo } from '@/api/types';

const SUBDOMAIN_REGEX = /^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/;
const BASE_DOMAIN = 'nav.ax';

export default function DomainPage() {
  const { data: subdomainData, isLoading, isError, error, refetch } = useSubdomain();
  const applyMutation = useApplySubdomain();
  const cancelMutation = useCancelSubdomainApplication();
  const { toast } = useToast();

  const [subdomainInput, setSubdomainInput] = useState('');
  const [applying, setApplying] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const [copied, setCopied] = useState(false);
  const [validationError, setValidationError] = useState('');

  // Reset applying state when mutation completes
  useEffect(() => {
    if (!applyMutation.isPending && applying) setApplying(false);
  }, [applyMutation.isPending, applying]);

  useEffect(() => {
    if (!cancelMutation.isPending && cancelling) setCancelling(false);
  }, [cancelMutation.isPending, cancelling]);

  const validateSubdomain = (value: string): string => {
    if (!value.trim()) return '请输入子域名';
    if (value.length > 30) return '子域名最多 30 个字符';
    if (!SUBDOMAIN_REGEX.test(value)) return '仅支持小写字母、数字和连字符，不能以连字符开头或结尾';
    return '';
  };

  const handleApply = async () => {
    const err = validateSubdomain(subdomainInput);
    if (err) {
      setValidationError(err);
      return;
    }
    setValidationError('');
    setApplying(true);
    applyMutation.mutate(
      { subdomain: subdomainInput.trim() },
      {
        onSuccess: response => {
          toast(
            'success',
            response.data.status === 'approved'
              ? '子域名已自动启用'
              : '短子域名申请已提交，请等待审核',
          );
          setSubdomainInput('');
        },
        onError: (err) => {
          toast('error', err instanceof Error ? err.message : '申请提交失败，请稍后重试');
        },
      },
    );
  };

  const handleCancel = () => {
    setCancelling(true);
    cancelMutation.mutate(undefined, {
      onSuccess: () => {
        toast('info', '申请已取消');
      },
      onError: () => {
        toast('error', '取消失败，请稍后重试');
      },
    });
  };

  const handleCopyDomain = (domain: string) => {
    navigator.clipboard.writeText(`https://${domain}`);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
    toast('info', '链接已复制');
  };

  const handleInputChange = (value: string) => {
    const lower = value.toLowerCase().replace(/[^a-z0-9-]/g, '');
    setSubdomainInput(lower);
    if (validationError) setValidationError('');
  };

  if (isLoading) return <LoadingSkeleton count={3} />;
  if (isError) {
    return <ErrorState message={error instanceof Error ? error.message : '加载域名设置失败'} onRetry={() => refetch()} />;
  }

  const current = subdomainData ?? null;
  const status = current?.status ?? 'none';

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold font-heading text-foreground-950">域名设置</h1>
        <p className="text-sm text-foreground-400 mt-1">申请 nav.ax 子域名作为你的个性化导航主页</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Main Content */}
        <div className="lg:col-span-2 space-y-6">
          {/* Info Card */}
          <div className="bg-white rounded-xl border border-background-200/70 p-5">
            <h3 className="text-sm font-semibold text-foreground-700 mb-3 flex items-center gap-2">
              <Info className="w-4 h-4" />
              什么是子域名？
            </h3>
            <div className="text-sm text-foreground-500 leading-relaxed space-y-2">
              <p>
                申请一个专属于你的 <strong className="text-foreground-700">xxx.{BASE_DOMAIN}</strong> 子域名，
                让你的导航主页拥有独立、好记的访问地址。比 <code className="px-1.5 py-0.5 rounded bg-background-100 text-xs text-foreground-600">/u/slug</code> 更专业。
              </p>
              <div className="flex items-start gap-2 mt-3 p-3 rounded-lg bg-background-50 border border-background-100">
                <ShieldCheck className="w-4 h-4 text-primary-600 mt-0.5 flex-shrink-0" />
                <div className="text-xs text-foreground-500">
                  <div className="font-medium text-foreground-700 mb-1">分配规则</div>
                  <ul className="space-y-1 list-disc list-inside">
                    <li>4 个及以上字符的可用子域名提交后立即启用，无需审核</li>
                    <li>1–3 个字符属于稀缺短域名，需要管理员审核</li>
                    <li>不可使用侵权、违规或已被占用的名称</li>
                    <li>已启用的域名如违反平台规则，管理员可撤销</li>
                  </ul>
                </div>
              </div>
            </div>
          </div>

          {/* Status-based content */}
          {status === 'none' && (
            <div className="bg-white rounded-xl border border-background-200/70 p-5">
              <h3 className="text-sm font-semibold text-foreground-700 mb-4 flex items-center gap-2">
                <Globe className="w-4 h-4" />
                申请子域名
              </h3>
              <div className="space-y-4">
                <div>
                  <label className="block text-xs text-foreground-500 mb-1.5">你想要子域名</label>
                  <div className="flex items-center">
                    <input
                      type="text"
                      value={subdomainInput}
                      onChange={e => handleInputChange(e.target.value)}
                      onKeyDown={e => { if (e.key === 'Enter') handleApply(); }}
                      placeholder="例如 lucas"
                      maxLength={30}
                      className="flex-1 h-10 px-3 bg-background-50 border border-background-200/70 rounded-l-lg text-sm text-foreground-900 focus:outline-none focus:border-primary-300 focus:ring-1 focus:ring-primary-200 transition-all duration-150"
                    />
                    <span className="h-10 px-3 flex items-center bg-background-100 border border-l-0 border-background-200/70 rounded-r-lg text-sm text-foreground-400 whitespace-nowrap">
                      .{BASE_DOMAIN}
                    </span>
                  </div>
                  {validationError && (
                    <div className="flex items-center gap-1.5 mt-1.5 text-xs text-red-500">
                      <AlertTriangle className="w-3 h-3 flex-shrink-0" />
                      {validationError}
                    </div>
                  )}
                  {!validationError && subdomainInput.length >= 1 && (
                    <div className="flex items-center gap-1.5 mt-1.5 text-xs text-primary-600">
                      <Check className="w-3 h-3 flex-shrink-0" />
                      {subdomainInput.length >= 4 ? '可自动启用' : '短域名需审核'}：{subdomainInput}.{BASE_DOMAIN}
                    </div>
                  )}
                </div>
                <button
                  onClick={handleApply}
                  disabled={applying || !subdomainInput.trim()}
                  className="h-10 px-6 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 disabled:opacity-40 disabled:cursor-not-allowed transition-all duration-150 flex items-center gap-2 whitespace-nowrap"
                >
                  {applying && <Loader2 className="w-4 h-4 animate-spin" />}
                  {applying ? '提交中...' : '获取子域名'}
                </button>
              </div>
            </div>
          )}

          {status === 'pending' && current && (
            <div className="bg-white rounded-xl border border-accent-200 p-5">
              <h3 className="text-sm font-semibold text-foreground-700 mb-4 flex items-center gap-2">
                <Clock className="w-4 h-4 text-accent-600" />
                审核中
              </h3>
              <div className="flex flex-col items-center py-6 text-center">
                <div className="w-14 h-14 rounded-full bg-accent-50 flex items-center justify-center mb-4">
                  <Loader2 className="w-7 h-7 text-accent-500 animate-spin" />
                </div>
                <div className="text-base font-semibold text-foreground-900">
                  {current.subdomain}.{BASE_DOMAIN}
                </div>
                <div className="text-sm text-foreground-400 mt-1">
                  你的短子域名申请正在审核中
                </div>
                <div className="text-xs text-foreground-300 mt-1">
                  提交于 {new Date(current.appliedAt).toLocaleString('zh-CN')}
                </div>
                <button
                  onClick={handleCancel}
                  disabled={cancelling}
                  className="mt-5 h-9 px-4 rounded-lg border border-background-200/70 text-sm text-foreground-500 hover:text-red-600 hover:border-red-200 transition-colors duration-150 flex items-center gap-2 whitespace-nowrap"
                >
                  {cancelling ? (
                    <Loader2 className="w-4 h-4 animate-spin" />
                  ) : (
                    <X className="w-4 h-4" />
                  )}
                  {cancelling ? '取消中...' : '取消申请'}
                </button>
              </div>
            </div>
          )}

          {status === 'approved' && current && (
            <div className="bg-white rounded-xl border border-green-200 p-5">
              <h3 className="text-sm font-semibold text-foreground-700 mb-4 flex items-center gap-2">
                <ShieldCheck className="w-4 h-4 text-green-600" />
                已激活
              </h3>
              <div className="flex flex-col items-center py-6 text-center">
                <div className="w-14 h-14 rounded-full bg-green-50 flex items-center justify-center mb-4">
                  <Check className="w-7 h-7 text-green-500" />
                </div>
                <div className="text-base font-semibold text-foreground-900">
                  {current.subdomain}.{BASE_DOMAIN}
                </div>
                <div className="text-sm text-foreground-400 mt-1">
                  域名已激活，现在可以通过专属地址访问你的导航主页
                </div>
                <div className="text-xs text-foreground-300 mt-1">
                  启用于 {current.reviewedAt ? new Date(current.reviewedAt).toLocaleString('zh-CN') : '—'}
                </div>
                <div className="flex items-center gap-3 mt-5">
                  <div className="flex items-center gap-1 bg-background-50 rounded-lg border border-background-200/70 p-1">
                    <span className="px-2 text-xs text-foreground-600 select-all">
                      https://{current.subdomain}.{BASE_DOMAIN}
                    </span>
                    <button
                      onClick={() => handleCopyDomain(`${current.subdomain}.${BASE_DOMAIN}`)}
                      className="h-8 px-3 rounded-md bg-primary-500 text-background-50 dark:text-foreground-950 text-xs font-medium hover:bg-primary-600 transition-colors duration-150 flex items-center gap-1 whitespace-nowrap flex-shrink-0"
                    >
                      {copied ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
                      {copied ? '已复制' : '复制'}
                    </button>
                  </div>
                </div>
                <a
                  href={`https://${current.subdomain}.${BASE_DOMAIN}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="mt-3 flex items-center gap-1.5 text-xs text-primary-600 hover:text-primary-700 font-medium transition-colors duration-150"
                >
                  在新窗口打开
                  <ExternalLink className="w-3 h-3" />
                </a>
              </div>
            </div>
          )}

          {status === 'rejected' && current && (
            <div className="bg-white rounded-xl border border-red-200 p-5">
              <h3 className="text-sm font-semibold text-foreground-700 mb-4 flex items-center gap-2">
                <AlertTriangle className="w-4 h-4 text-red-500" />
                申请未通过
              </h3>
              <div className="flex flex-col items-center py-6 text-center">
                <div className="w-14 h-14 rounded-full bg-red-50 flex items-center justify-center mb-4">
                  <X className="w-7 h-7 text-red-400" />
                </div>
                <div className="text-base font-semibold text-foreground-900">
                  {current.subdomain}.{BASE_DOMAIN}
                </div>
                <div className="text-sm text-foreground-400 mt-1">
                  很遗憾，你的子域名申请未通过审核
                </div>
                {current.rejectionReason && (
                  <div className="mt-3 p-3 rounded-lg bg-red-50 border border-red-100 max-w-sm">
                    <div className="text-xs font-medium text-red-700 mb-0.5">审核意见</div>
                    <div className="text-xs text-red-600">{current.rejectionReason}</div>
                  </div>
                )}
                <div className="text-xs text-foreground-300 mt-3">
                  审核于 {current.reviewedAt ? new Date(current.reviewedAt).toLocaleString('zh-CN') : '—'}
                </div>
                <div className="text-xs text-foreground-400 mt-1">
                  你可以修改后重新提交申请
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Sidebar */}
        <div className="space-y-4">
          {/* Quick status */}
          <div className="bg-white rounded-xl border border-background-200/70 p-5">
            <h3 className="text-sm font-semibold text-foreground-700 mb-3">域名状态</h3>
            {!current || status === 'none' ? (
              <div className="flex items-center gap-2 text-sm text-foreground-400">
                <Globe className="w-4 h-4 flex-shrink-0" />
                未申请子域名
              </div>
            ) : (
              <div className="space-y-2 text-sm">
                <div className={cn(
                  'px-2.5 py-1 rounded-full text-xs font-medium inline-block',
                  status === 'approved' && 'bg-green-100 text-green-700',
                  status === 'pending' && 'bg-accent-100 text-accent-700',
                  status === 'rejected' && 'bg-red-100 text-red-700',
                )}>
                  {status === 'approved' && '已激活'}
                  {status === 'pending' && '审核中'}
                  {status === 'rejected' && '未通过'}
                </div>
                <div className="flex justify-between text-xs">
                  <span className="text-foreground-400">子域名</span>
                  <span className="text-foreground-700 font-mono">{current.subdomain}</span>
                </div>
                <div className="flex justify-between text-xs">
                  <span className="text-foreground-400">完整地址</span>
                  <span className="text-foreground-700 font-mono text-right break-all">{current.fullDomain}</span>
                </div>
                {status === 'approved' && (
                  <a
                    href={`https://${current.fullDomain}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center gap-1.5 text-xs text-primary-600 hover:text-primary-700 font-medium mt-2"
                  >
                    访问域名
                    <ExternalLink className="w-3 h-3" />
                  </a>
                )}
              </div>
            )}
          </div>

          {/* Tips */}
          <div className="bg-white rounded-xl border border-background-200/70 p-5">
            <h3 className="text-sm font-semibold text-foreground-700 mb-3">小贴士</h3>
            <ul className="space-y-2.5 text-xs text-foreground-500">
              <li className="flex gap-2">
                <span className="text-primary-500 mt-0.5">•</span>
                使用你的用户名或品牌名作为子域名，方便记忆
              </li>
              <li className="flex gap-2">
                <span className="text-primary-500 mt-0.5">•</span>
                子域名一旦启用，暂时不支持自行修改，如需变更请联系管理员
              </li>
              <li className="flex gap-2">
                <span className="text-primary-500 mt-0.5">•</span>
                确保你的导航页面已经发布了内容，域名的访问者才能看到
              </li>
              <li className="flex gap-2">
                <span className="text-primary-500 mt-0.5">•</span>
                建议将域名分享到社交媒体、简历或个人简介中
              </li>
            </ul>
          </div>

        </div>
      </div>
    </div>
  );
}
