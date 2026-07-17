// ============================================================
// nav.ax Forgot Password Page — /forgot-password
// ============================================================

import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import PublicShell from '@/components/feature/PublicShell';
import { Mail, ArrowRight, MailCheck, ShieldAlert } from 'lucide-react';
import { authApi } from '@/api/auth';
import { getPublicConfig } from '@/api/assets';

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState('');
  const [loading, setLoading] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [mailEnabled, setMailEnabled] = useState<boolean | null>(null);

  useEffect(() => {
    getPublicConfig()
      .then(response => setMailEnabled(response.data.features?.mail === true))
      .catch(() => setMailEnabled(null));
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (mailEnabled === false) return;
    setLoading(true);
    try {
      // The response is intentionally generic; we always show the same
      // confirmation so the page cannot reveal whether an email is registered.
      await authApi.forgotPassword(email);
    } catch {
      // Swallow errors for the same anti-enumeration reason.
    } finally {
      setLoading(false);
      setSubmitted(true);
    }
  };

  if (mailEnabled === false) {
    return (
      <PublicShell showSearch={false}>
        <div className="min-h-[80vh] flex items-center justify-center px-4">
          <div className="w-full max-w-sm text-center">
            <div className="w-16 h-16 rounded-full bg-amber-50 flex items-center justify-center mx-auto mb-4">
              <ShieldAlert className="w-8 h-8 text-amber-600" />
            </div>
            <h1 className="text-xl font-semibold text-foreground-900 mb-2">邮件服务未配置</h1>
            <p className="text-sm text-foreground-400 mb-6">
              当前实例尚未配置 SMTP，无法发送密码重置邮件。请联系站点管理员重置密码。
            </p>
            <Link
              to="/login"
              className="inline-flex items-center gap-2 h-10 px-5 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 transition-colors duration-150"
            >
              返回登录
            </Link>
          </div>
        </div>
      </PublicShell>
    );
  }

  if (submitted) {
    return (
      <PublicShell showSearch={false}>
        <div className="min-h-[80vh] flex items-center justify-center px-4">
          <div className="w-full max-w-sm text-center">
            <div className="w-16 h-16 rounded-full bg-primary-50 flex items-center justify-center mx-auto mb-4">
              <MailCheck className="w-8 h-8 text-primary-500" />
            </div>
            <h1 className="text-xl font-semibold text-foreground-900 mb-2">请检查你的邮箱</h1>
            <p className="text-sm text-foreground-400 mb-6">
              如果 <span className="text-foreground-600">{email}</span> 对应一个有效账号，我们已发送一封包含密码重置链接的邮件。链接将在 1 小时后失效。
            </p>
            <p className="text-xs text-foreground-300 mb-6">没有收到？请检查垃圾邮件文件夹。</p>
            <Link
              to="/login"
              className="inline-flex items-center gap-2 h-10 px-5 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 transition-colors duration-150"
            >
              返回登录
            </Link>
          </div>
        </div>
      </PublicShell>
    );
  }

  return (
    <PublicShell showSearch={false}>
      <div className="min-h-[80vh] flex items-center justify-center px-4">
        <div className="w-full max-w-sm">
          <div className="text-center mb-8">
            <Link to="/" className="inline-block">
              <span className="text-2xl font-bold font-heading text-foreground-950">nav.ax</span>
            </Link>
            <h1 className="mt-4 text-xl font-semibold text-foreground-900">找回密码</h1>
            <p className="mt-1 text-sm text-foreground-400">输入注册邮箱，我们会发送密码重置链接</p>
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label htmlFor="email" className="block text-sm font-medium text-foreground-700 mb-1.5">邮箱</label>
              <div className="relative">
                <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-foreground-300" />
                <input
                  id="email"
                  type="email"
                  name="email"
                  autoComplete="email"
                  value={email}
                  onChange={e => setEmail(e.target.value)}
                  placeholder="your@email.com"
                  required
                  className="w-full h-11 pl-10 pr-4 rounded-lg bg-background-50 border border-background-200/70 text-sm text-foreground-900 placeholder:text-foreground-300 focus:outline-none focus:border-primary-300 focus:ring-1 focus:ring-primary-200 transition-all duration-150"
                />
              </div>
            </div>

            <button
              type="submit"
              disabled={loading || mailEnabled === null}
              className="w-full h-11 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-150 flex items-center justify-center gap-2 whitespace-nowrap"
            >
              {loading ? '发送中...' : (
                <>
                  发送重置链接
                  <ArrowRight className="w-4 h-4" />
                </>
              )}
            </button>
          </form>

          <p className="mt-6 text-center text-sm text-foreground-400">
            想起来了？<Link to="/login" className="text-primary-600 hover:text-primary-700 font-medium">返回登录</Link>
          </p>
        </div>
      </div>
    </PublicShell>
  );
}
