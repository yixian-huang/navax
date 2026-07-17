// Open registration with email verification code.

import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import PublicShell from '@/components/feature/PublicShell';
import NavaxLogo from '@/components/base/NavaxLogo';
import { User, Lock, Mail, ArrowRight, Loader2, KeyRound } from 'lucide-react';
import { useToast } from '@/components/base/Toast';
import { authApi } from '@/api/auth';
import { getPublicConfig } from '@/api/assets';
import { useQueryClient } from '@tanstack/react-query';

export default function RegisterPage() {
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [code, setCode] = useState('');
  const [step, setStep] = useState<'form' | 'code'>('form');
  const [loading, setLoading] = useState(false);
  const [sending, setSending] = useState(false);
  const [checking, setChecking] = useState(true);
  const [open, setOpen] = useState(false);
  const [oauthProviders, setOauthProviders] = useState<string[]>([]);
  const { toast } = useToast();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  useEffect(() => {
    let cancelled = false;
    getPublicConfig()
      .then(response => {
        if (!cancelled) setOpen(response.data.registrationMode === 'open');
      })
      .catch(() => {
        if (!cancelled) setOpen(false);
      })
      .finally(() => {
        if (!cancelled) setChecking(false);
      });
    authApi.listOAuthProviders()
      .then(res => { if (!cancelled) setOauthProviders(res.data.providers ?? []); })
      .catch(() => {});
    return () => { cancelled = true; };
  }, []);

  const handleSendCode = async (e: React.FormEvent) => {
    e.preventDefault();
    setSending(true);
    try {
      await authApi.requestEmailCode({
        email, purpose: 'register', username, password,
      });
      setStep('code');
      toast('success', '验证码已发送，请查收邮件');
    } catch (error) {
      toast('error', error instanceof Error ? error.message : '发送验证码失败');
    } finally {
      setSending(false);
    }
  };

  const handleVerify = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
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
      toast('error', error instanceof Error ? error.message : '验证失败');
      setLoading(false);
    }
  };

  if (checking) {
    return (
      <PublicShell showSearch={false}>
        <div className="min-h-[80vh] flex items-center justify-center">
          <Loader2 className="w-5 h-5 animate-spin text-foreground-400" />
        </div>
      </PublicShell>
    );
  }

  if (!open) {
    return (
      <PublicShell showSearch={false}>
        <div className="min-h-[80vh] flex items-center justify-center px-4">
          <div className="text-center max-w-sm">
            <h1 className="text-xl font-semibold text-foreground-900">暂未开放公开注册</h1>
            <p className="mt-2 text-sm text-foreground-400">请使用邀请链接注册，或联系管理员。</p>
            <Link to="/login" className="inline-block mt-6 text-sm text-primary-600 hover:underline">返回登录</Link>
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
            <h1 className="mt-4 text-xl font-semibold text-foreground-900">创建账号</h1>
            <p className="mt-1 text-sm text-foreground-400">
              {step === 'form' ? '填写信息并验证邮箱' : '输入邮箱收到的 6 位验证码'}
            </p>
          </div>

          {step === 'form' ? (
            <form onSubmit={handleSendCode} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-foreground-700 mb-1.5">用户名</label>
                <div className="relative">
                  <User className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-foreground-300" />
                  <input value={username} onChange={e => setUsername(e.target.value)} required minLength={3} maxLength={32}
                    pattern="[a-zA-Z0-9_-]{3,32}"
                    className="w-full h-11 pl-10 pr-4 rounded-lg bg-background-50 border border-background-200/70 text-sm focus:outline-none focus:border-primary-300" />
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-foreground-700 mb-1.5">邮箱</label>
                <div className="relative">
                  <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-foreground-300" />
                  <input type="email" value={email} onChange={e => setEmail(e.target.value)} required
                    className="w-full h-11 pl-10 pr-4 rounded-lg bg-background-50 border border-background-200/70 text-sm focus:outline-none focus:border-primary-300" />
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-foreground-700 mb-1.5">密码（至少 12 位）</label>
                <div className="relative">
                  <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-foreground-300" />
                  <input type="password" value={password} onChange={e => setPassword(e.target.value)} required minLength={12}
                    className="w-full h-11 pl-10 pr-4 rounded-lg bg-background-50 border border-background-200/70 text-sm focus:outline-none focus:border-primary-300" />
                </div>
              </div>
              <button type="submit" disabled={sending}
                className="w-full h-11 rounded-lg bg-primary-500 text-background-50 text-sm font-medium hover:bg-primary-600 disabled:opacity-50 inline-flex items-center justify-center gap-2">
                {sending ? '发送中...' : <>发送邮箱验证码 <ArrowRight className="w-4 h-4" /></>}
              </button>
            </form>
          ) : (
            <form onSubmit={handleVerify} className="space-y-4">
              <p className="text-xs text-foreground-500">验证码已发送至 <span className="font-medium text-foreground-700">{email}</span></p>
              <div>
                <label className="block text-sm font-medium text-foreground-700 mb-1.5">验证码</label>
                <div className="relative">
                  <KeyRound className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-foreground-300" />
                  <input value={code} onChange={e => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                    required minLength={6} maxLength={6} inputMode="numeric"
                    className="w-full h-11 pl-10 pr-4 rounded-lg bg-background-50 border border-background-200/70 text-sm tracking-widest focus:outline-none focus:border-primary-300" />
                </div>
              </div>
              <button type="submit" disabled={loading || code.length < 6}
                className="w-full h-11 rounded-lg bg-primary-500 text-background-50 text-sm font-medium disabled:opacity-50">
                {loading ? '注册中...' : '验证并完成注册'}
              </button>
              <button type="button" onClick={() => setStep('form')} className="w-full text-xs text-foreground-400 hover:text-foreground-600">
                返回修改信息
              </button>
            </form>
          )}

          {oauthProviders.length > 0 && step === 'form' && (
            <div className="mt-6 grid gap-2">
              {oauthProviders.includes('google') && (
                <a href={authApi.oauthStartURL('google')} className="h-10 rounded-lg border border-background-200/70 text-sm font-medium inline-flex items-center justify-center gap-2 hover:bg-background-100">
                  <i className="ri-google-fill" /> 使用 Google 注册/登录
                </a>
              )}
              {oauthProviders.includes('github') && (
                <a href={authApi.oauthStartURL('github')} className="h-10 rounded-lg border border-background-200/70 text-sm font-medium inline-flex items-center justify-center gap-2 hover:bg-background-100">
                  <i className="ri-github-fill" /> 使用 GitHub 注册/登录
                </a>
              )}
            </div>
          )}

          <p className="mt-6 text-center text-sm text-foreground-400">
            已有账号？{' '}
            <Link to="/login" className="text-primary-600 hover:underline">去登录</Link>
          </p>
        </div>
      </div>
    </PublicShell>
  );
}
