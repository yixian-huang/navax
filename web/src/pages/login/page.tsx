import { useEffect, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import PublicShell from '@/components/feature/PublicShell';
import NavaxLogo from '@/components/base/NavaxLogo';
import { Mail, Lock, Eye, EyeOff, ArrowRight, KeyRound, User } from 'lucide-react';
import { useToast } from '@/components/base/Toast';
import { authApi } from '@/api/auth';
import { getPublicConfig } from '@/api/assets';
import { useQueryClient } from '@tanstack/react-query';
import { cn } from '@/lib/utils';

type Mode = 'password' | 'code';

function oauthToastMessage(code: string | null, registrationMode: string): string | null {
  switch (code) {
    case 'denied':
      return '已取消第三方授权';
    case 'invite_required':
      if (registrationMode === 'open') {
        return '第三方登录失败。若已有账号，请使用同一邮箱的 Google/GitHub 账号。';
      }
      return '当前为邀请注册：已有账号请用同邮箱第三方登录；新用户可在下方填写邀请码后再点 Google/GitHub，或从邀请链接进入。';
    case 'mail_required':
      return '服务器未配置邮件服务，无法完成第三方注册验证码步骤';
    case 'account_disabled':
      return '该账号已被禁用，请联系管理员';
    case 'error':
      return '第三方登录失败，请重试。若已有账号，请使用相同邮箱的 Google/GitHub 账号。';
    default:
      return null;
  }
}

export default function LoginPage() {
  const [account, setAccount] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [code, setCode] = useState('');
  const [mode, setMode] = useState<Mode>('password');
  const [codeSent, setCodeSent] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [sending, setSending] = useState(false);
  const [registrationMode, setRegistrationMode] = useState<'invite' | 'closed' | 'open'>('invite');
  const [oauthProviders, setOauthProviders] = useState<string[]>([]);
  /** Optional invite token carried into OAuth start (invite-mode registration). */
  const [inviteToken, setInviteToken] = useState('');
  const { toast } = useToast();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [searchParams] = useSearchParams();

  useEffect(() => {
    getPublicConfig()
      .then(response => setRegistrationMode(response.data.registrationMode))
      .catch(() => { /* keep default */ });
    authApi.listOAuthProviders()
      .then(res => setOauthProviders(res.data.providers ?? []))
      .catch(() => setOauthProviders([]));
  }, []);

  useEffect(() => {
    const oauth = searchParams.get('oauth');
    const msg = oauthToastMessage(oauth, registrationMode);
    if (msg) toast('error', msg);
  }, [searchParams, toast, registrationMode]);

  const applySession = (authData: { user: unknown; expiresAt?: string | null }) => {
    queryClient.setQueryData(['auth', 'session'], {
      authenticated: true,
      user: authData.user,
      expiresAt: authData.expiresAt ?? null,
    });
    toast('success', '登录成功，正在跳转...');
    const role = (authData.user as { role?: string } | null)?.role;
    navigate(role === 'admin' ? '/admin' : '/app?scope=personal');
  };

  const handlePasswordLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      const res = await authApi.login({ account: account.trim(), password });
      applySession(res.data);
    } catch {
      toast('error', '登录失败，请检查邮箱/用户名和密码');
      setLoading(false);
    }
  };

  const handleSendCode = async () => {
    if (!email.trim()) {
      toast('error', '请先填写邮箱');
      return;
    }
    setSending(true);
    try {
      await authApi.requestEmailCode({ email, purpose: 'login' });
      setCodeSent(true);
      toast('success', '若该邮箱已注册，验证码已发送');
    } catch (error) {
      toast('error', error instanceof Error ? error.message : '发送验证码失败');
    } finally {
      setSending(false);
    }
  };

  const handleCodeLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      const res = await authApi.loginWithEmailCode({ email, code });
      applySession(res.data);
    } catch {
      toast('error', '验证码无效或已过期');
      setLoading(false);
    }
  };

  return (
    <PublicShell showSearch={false}>
      <div className="min-h-[80vh] flex items-center justify-center px-4">
        <div className="w-full max-w-sm">
          <div className="text-center mb-8">
            <Link to="/" className="group inline-flex justify-center" aria-label="nav.ax 首页">
              <NavaxLogo size="lg" />
            </Link>
            <h1 className="mt-4 text-xl font-semibold text-foreground-900">登录你的账号</h1>
            <p className="mt-1 text-sm text-foreground-400">支持密码、邮箱验证码与第三方登录</p>
          </div>

          <div className="flex rounded-lg border border-background-200/70 p-1 mb-4 bg-background-50">
            {([
              { id: 'password' as const, label: '密码登录' },
              { id: 'code' as const, label: '验证码登录' },
            ]).map(tab => (
              <button
                key={tab.id}
                type="button"
                onClick={() => setMode(tab.id)}
                className={cn(
                  'flex-1 h-8 rounded-md text-xs font-medium transition-colors',
                  mode === tab.id ? 'bg-primary-500 text-background-50' : 'text-foreground-500 hover:text-foreground-700',
                )}
              >
                {tab.label}
              </button>
            ))}
          </div>

          {mode === 'password' ? (
            <form onSubmit={handlePasswordLogin} className="space-y-4">
              <div>
                <label htmlFor="account" className="block text-sm font-medium text-foreground-700 mb-1.5">邮箱或用户名</label>
                <div className="relative">
                  <User className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-foreground-300" />
                  <input
                    id="account"
                    type="text"
                    autoComplete="username"
                    value={account}
                    onChange={e => setAccount(e.target.value)}
                    required
                    placeholder="email@example.com 或 username"
                    className="w-full h-11 pl-10 pr-4 rounded-lg bg-background-50 border border-background-200/70 text-sm focus:outline-none focus:border-primary-300"
                  />
                </div>
              </div>
              <div>
                <div className="flex items-center justify-between mb-1.5">
                  <label htmlFor="password" className="block text-sm font-medium text-foreground-700">密码</label>
                  <Link to="/forgot-password" className="text-xs text-primary-600 hover:text-primary-700 font-medium">忘记密码？</Link>
                </div>
                <div className="relative">
                  <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-foreground-300" />
                  <input
                    id="password"
                    type={showPassword ? 'text' : 'password'}
                    autoComplete="current-password"
                    value={password}
                    onChange={e => setPassword(e.target.value)}
                    required
                    className="w-full h-11 pl-10 pr-10 rounded-lg bg-background-50 border border-background-200/70 text-sm focus:outline-none focus:border-primary-300"
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-foreground-300"
                    aria-label={showPassword ? '隐藏密码' : '显示密码'}
                  >
                    {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                  </button>
                </div>
              </div>
              <button
                type="submit"
                disabled={loading}
                className="w-full h-11 rounded-lg bg-primary-500 text-background-50 text-sm font-medium hover:bg-primary-600 disabled:opacity-50 inline-flex items-center justify-center gap-2"
              >
                {loading ? '登录中...' : <>登录 <ArrowRight className="w-4 h-4" /></>}
              </button>
            </form>
          ) : (
            <form onSubmit={handleCodeLogin} className="space-y-4">
              <div>
                <label htmlFor="email-code" className="block text-sm font-medium text-foreground-700 mb-1.5">邮箱</label>
                <div className="relative">
                  <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-foreground-300" />
                  <input
                    id="email-code"
                    type="email"
                    value={email}
                    onChange={e => setEmail(e.target.value)}
                    required
                    className="w-full h-11 pl-10 pr-4 rounded-lg bg-background-50 border border-background-200/70 text-sm focus:outline-none focus:border-primary-300"
                  />
                </div>
              </div>
              <div>
                <label htmlFor="otp" className="block text-sm font-medium text-foreground-700 mb-1.5">验证码</label>
                <div className="flex gap-2">
                  <div className="relative flex-1">
                    <KeyRound className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-foreground-300" />
                    <input
                      id="otp"
                      value={code}
                      onChange={e => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                      required
                      minLength={6}
                      maxLength={6}
                      inputMode="numeric"
                      placeholder="6 位数字"
                      className="w-full h-11 pl-10 pr-3 rounded-lg bg-background-50 border border-background-200/70 text-sm focus:outline-none focus:border-primary-300 tracking-widest"
                    />
                  </div>
                  <button
                    type="button"
                    disabled={sending}
                    onClick={() => void handleSendCode()}
                    className="h-11 px-3 rounded-lg border border-background-200 text-xs font-medium text-foreground-600 hover:bg-background-100 disabled:opacity-50 whitespace-nowrap"
                  >
                    {sending ? '发送中' : codeSent ? '重新发送' : '获取验证码'}
                  </button>
                </div>
              </div>
              <button
                type="submit"
                disabled={loading || code.length < 6}
                className="w-full h-11 rounded-lg bg-primary-500 text-background-50 text-sm font-medium hover:bg-primary-600 disabled:opacity-50"
              >
                {loading ? '登录中...' : '验证并登录'}
              </button>
            </form>
          )}

          {oauthProviders.length > 0 && (
            <div className="mt-6">
              <div className="relative my-4">
                <div className="absolute inset-0 flex items-center"><div className="w-full border-t border-background-200/70" /></div>
                <div className="relative flex justify-center text-[11px]"><span className="px-2 bg-background-100 text-foreground-400">或使用</span></div>
              </div>
              {registrationMode === 'invite' && (
                <label className="block mb-3">
                  <span className="block text-xs text-foreground-500 mb-1.5">
                    邀请码（新用户 OAuth 注册时填写；已有账号可留空）
                  </span>
                  <input
                    value={inviteToken}
                    onChange={e => setInviteToken(e.target.value.trim())}
                    placeholder="邀请 token"
                    className="w-full h-10 px-3 rounded-lg border border-background-200/70 bg-background-50 text-sm focus:outline-none focus:border-primary-300"
                  />
                </label>
              )}
              <div className="grid gap-2">
                {oauthProviders.includes('google') && (
                  <a
                    href={authApi.oauthStartURL('google', inviteToken || undefined)}
                    className="h-10 rounded-lg border border-background-200/70 bg-background-50 text-sm font-medium text-foreground-700 hover:bg-background-100 inline-flex items-center justify-center gap-2"
                  >
                    <i className="ri-google-fill text-base" /> Google 登录
                  </a>
                )}
                {oauthProviders.includes('github') && (
                  <a
                    href={authApi.oauthStartURL('github', inviteToken || undefined)}
                    className="h-10 rounded-lg border border-background-200/70 bg-background-50 text-sm font-medium text-foreground-700 hover:bg-background-100 inline-flex items-center justify-center gap-2"
                  >
                    <i className="ri-github-fill text-base" /> GitHub 登录
                  </a>
                )}
              </div>
              <p className="mt-3 text-[11px] leading-relaxed text-foreground-400 text-center">
                新用户：第三方授权后将向该邮箱发送验证码，验证通过后完成注册。
                {registrationMode === 'invite' ? ' 邀请制需同时提供有效邀请码。' : ''}
                {' '}已有账号：使用与站内相同邮箱的 Google/GitHub 即可直接登录。
              </p>
            </div>
          )}

          <p className="mt-6 text-center text-sm text-foreground-400">
            {registrationMode === 'open' && (
              <>还没有账号？{' '}<Link to="/register" className="text-primary-600 hover:underline font-medium">公开注册</Link></>
            )}
            {registrationMode === 'invite' && '还没有账号？需要邀请链接才能注册'}
            {registrationMode === 'closed' && '当前未开放新用户注册'}
          </p>
        </div>
      </div>
    </PublicShell>
  );
}
