// ============================================================
// nav.ax Invite Registration Page
// ============================================================

import { useState, useEffect, useMemo } from 'react';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import PublicShell from '@/components/feature/PublicShell';
import NavaxLogo from '@/components/base/NavaxLogo';
import { User, Lock, Mail, ArrowRight, CheckCircle, Loader2 } from 'lucide-react';
import { useToast } from '@/components/base/Toast';
import { authApi } from '@/api/auth';
import { useQueryClient } from '@tanstack/react-query';

export default function InvitePage() {
  const { token: pathToken } = useParams<{ token: string }>();
  const [searchParams] = useSearchParams();
  // Support both /invite/:token and legacy /invite?token=
  const token = useMemo(
    () => pathToken || searchParams.get('token') || '',
    [pathToken, searchParams],
  );
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [code, setCode] = useState('');
  const [step, setStep] = useState<'form' | 'code'>('form');
  const [loading, setLoading] = useState(false);
  const [sending, setSending] = useState(false);
  const [validating, setValidating] = useState(true);
  const [valid, setValid] = useState(false);
  const [inviterName, setInviterName] = useState('');
  const [oauthProviders, setOauthProviders] = useState<string[]>([]);
  const { toast } = useToast();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  useEffect(() => {
    let cancelled = false;
    const validate = async () => {
      if (!token) {
        setValid(false);
        setValidating(false);
        return;
      }
      try {
        const response = await authApi.validateInviteToken(token);
        if (!cancelled) {
          setValid(response.data.valid);
          setInviterName(response.data.inviterName);
        }
      } catch {
        if (!cancelled) setValid(false);
      } finally {
        if (!cancelled) setValidating(false);
      }
    };
    void validate();
    authApi.listOAuthProviders()
      .then(res => { if (!cancelled) setOauthProviders(res.data.providers ?? []); })
      .catch(() => {});
    return () => { cancelled = true; };
  }, [token]);

  const handleSendCode = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!token) return;
    setSending(true);
    try {
      await authApi.requestEmailCode({
        email, purpose: 'register', username, password, invitationToken: token,
      });
      setStep('code');
      toast('success', '验证码已发送，请查收邮件');
    } catch (error) {
      toast('error', error instanceof Error ? error.message : '发送验证码失败');
    } finally {
      setSending(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    if (!token) return;
    try {
      const response = await authApi.registerWithEmailCode({ email, code });
      queryClient.setQueryData(['auth', 'session'], {
        authenticated: true,
        user: response.data.user,
        expiresAt: response.data.expiresAt ?? null,
      });
      toast('success', '注册成功！正在进入你的导航主页');
      navigate('/app?scope=personal', { replace: true });
    } catch (error) {
      toast('error', error instanceof Error ? error.message : '注册失败，请检查输入后重试');
    } finally {
      setLoading(false);
    }
  };

  if (validating) {
    return (
      <PublicShell showSearch={false}>
        <div className="min-h-[80vh] flex items-center justify-center">
          <div className="flex items-center gap-3 text-foreground-400">
            <Loader2 className="w-5 h-5 animate-spin" />
            <span className="text-sm">验证邀请链接...</span>
          </div>
        </div>
      </PublicShell>
    );
  }

  if (!valid) {
    return (
      <PublicShell showSearch={false}>
        <div className="min-h-[80vh] flex items-center justify-center px-4">
          <div className="text-center max-w-sm">
            <div className="w-16 h-16 rounded-full bg-red-50 flex items-center justify-center mx-auto mb-4">
              <User className="w-8 h-8 text-red-400" />
            </div>
            <h1 className="text-xl font-semibold text-foreground-900 mb-2">邀请链接无效</h1>
            <p className="text-sm text-foreground-400 mb-6">该邀请链接可能已过期、已被撤销或已达到使用上限</p>
            <Link to="/" className="inline-flex items-center gap-2 h-10 px-5 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 transition-colors duration-150">
              返回首页
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
            <Link to="/" className="group inline-flex justify-center" aria-label="nav.ax 首页">
              <NavaxLogo size="lg" />
            </Link>
            <h1 className="mt-4 text-xl font-semibold text-foreground-900">创建你的账号</h1>
            <p className="mt-1 text-sm text-foreground-400">
              {inviterName} 邀请你加入 nav.ax
            </p>
          </div>

          {step === 'form' ? (
          <form onSubmit={handleSendCode} className="space-y-4">
            <div>
              <label htmlFor="username" className="block text-sm font-medium text-foreground-700 mb-1.5">用户名</label>
              <div className="relative">
                <User className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-foreground-300" />
                <input
                  id="username"
                  name="username"
                  type="text"
                  value={username}
                  onChange={e => setUsername(e.target.value)}
                  placeholder="你的用户名"
                  required
                  minLength={2}
                  className="w-full h-11 pl-10 pr-4 rounded-lg bg-background-50 border border-background-200/70 text-sm text-foreground-900 placeholder:text-foreground-300 focus:outline-none focus:border-primary-300 focus:ring-1 focus:ring-primary-200 transition-all duration-150"
                />
              </div>
            </div>

            <div>
              <label htmlFor="email" className="block text-sm font-medium text-foreground-700 mb-1.5">邮箱</label>
              <div className="relative">
                <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-foreground-300" />
                <input
                  id="email"
                  name="email"
                  type="email"
                  value={email}
                  onChange={e => setEmail(e.target.value)}
                  placeholder="your@email.com"
                  required
                  className="w-full h-11 pl-10 pr-4 rounded-lg bg-background-50 border border-background-200/70 text-sm text-foreground-900 placeholder:text-foreground-300 focus:outline-none focus:border-primary-300 focus:ring-1 focus:ring-primary-200 transition-all duration-150"
                />
              </div>
            </div>

            <div>
              <label htmlFor="password" className="block text-sm font-medium text-foreground-700 mb-1.5">密码</label>
              <div className="relative">
                <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-foreground-300" />
                <input
                  id="password"
                  name="password"
                  type="password"
                  value={password}
                  onChange={e => setPassword(e.target.value)}
                  placeholder="至少 12 位密码"
                  required
                  minLength={12}
                  className="w-full h-11 pl-10 pr-4 rounded-lg bg-background-50 border border-background-200/70 text-sm text-foreground-900 placeholder:text-foreground-300 focus:outline-none focus:border-primary-300 focus:ring-1 focus:ring-primary-200 transition-all duration-150"
                />
              </div>
            </div>

            <button
              type="submit"
              disabled={sending}
              className="w-full h-11 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-150 flex items-center justify-center gap-2 whitespace-nowrap"
            >
              {sending ? '发送中...' : (
                <>
                  发送邮箱验证码
                  <ArrowRight className="w-4 h-4" />
                </>
              )}
            </button>
          </form>
          ) : (
          <form onSubmit={handleSubmit} className="space-y-4">
            <p className="text-xs text-foreground-500">验证码已发送至 <span className="font-medium">{email}</span></p>
            <div>
              <label className="block text-sm font-medium text-foreground-700 mb-1.5">验证码</label>
              <input
                value={code}
                onChange={e => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                required
                minLength={6}
                maxLength={6}
                inputMode="numeric"
                className="w-full h-11 px-4 rounded-lg bg-background-50 border border-background-200/70 text-sm tracking-widest focus:outline-none focus:border-primary-300"
              />
            </div>
            <button
              type="submit"
              disabled={loading || code.length < 6}
              className="w-full h-11 rounded-lg bg-primary-500 text-background-50 text-sm font-medium disabled:opacity-50"
            >
              {loading ? '创建中...' : '验证并创建账号'}
            </button>
            <button type="button" onClick={() => setStep('form')} className="w-full text-xs text-foreground-400">
              返回修改信息
            </button>
          </form>
          )}

          {oauthProviders.length > 0 && step === 'form' && token && (
            <div className="mt-4 grid gap-2">
              {oauthProviders.includes('google') && (
                <a href={authApi.oauthStartURL('google', token)} className="h-10 rounded-lg border border-background-200/70 text-sm inline-flex items-center justify-center gap-2 hover:bg-background-100">
                  <i className="ri-google-fill" /> 使用 Google 接受邀请
                </a>
              )}
              {oauthProviders.includes('github') && (
                <a href={authApi.oauthStartURL('github', token)} className="h-10 rounded-lg border border-background-200/70 text-sm inline-flex items-center justify-center gap-2 hover:bg-background-100">
                  <i className="ri-github-fill" /> 使用 GitHub 接受邀请
                </a>
              )}
            </div>
          )}

          <p className="mt-6 text-center text-sm text-foreground-400">
            已有账号？<Link to="/login" className="text-primary-600 hover:text-primary-700 font-medium">登录</Link>
          </p>
        </div>
      </div>
    </PublicShell>
  );
}
