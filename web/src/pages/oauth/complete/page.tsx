// Complete first-time OAuth registration with email OTP (+ invite when needed).

import { useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import PublicShell from '@/components/feature/PublicShell';
import NavaxLogo from '@/components/base/NavaxLogo';
import { KeyRound, Mail, Ticket, Loader2 } from 'lucide-react';
import { useToast } from '@/components/base/Toast';
import { authApi } from '@/api/auth';
import { useQueryClient } from '@tanstack/react-query';

export default function OAuthCompletePage() {
  const [searchParams] = useSearchParams();
  const email = (searchParams.get('email') || '').trim();
  const needsInvite = searchParams.get('needsInvite') === '1';
  const [code, setCode] = useState('');
  const [invite, setInvite] = useState('');
  const [loading, setLoading] = useState(false);
  const [resending, setResending] = useState(false);
  const { toast } = useToast();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  if (!email) {
    return (
      <PublicShell showSearch={false}>
        <div className="min-h-[80vh] flex items-center justify-center px-4">
          <div className="text-center max-w-sm">
            <h1 className="text-xl font-semibold text-foreground-900">缺少注册信息</h1>
            <p className="mt-2 text-sm text-foreground-400">请重新从登录页发起第三方登录。</p>
            <Link to="/login" className="inline-block mt-6 text-sm text-primary-600 hover:underline">返回登录</Link>
          </div>
        </div>
      </PublicShell>
    );
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (needsInvite && !invite.trim()) {
      toast('error', '请填写邀请码（或完整邀请链接中的 token）');
      return;
    }
    setLoading(true);
    try {
      const res = await authApi.completeOAuthRegister({
        email,
        code,
        invitationToken: invite.trim() || undefined,
      });
      queryClient.setQueryData(['auth', 'session'], {
        authenticated: true,
        user: res.data.user,
        expiresAt: res.data.expiresAt ?? null,
      });
      toast('success', '注册成功，正在进入工作台');
      const role = (res.data.user as { role?: string } | null)?.role;
      navigate(role === 'admin' ? '/admin' : '/app?scope=personal', { replace: true });
    } catch (error) {
      toast('error', error instanceof Error ? error.message : '验证失败');
      setLoading(false);
    }
  };

  const handleResend = async () => {
    setResending(true);
    try {
      await authApi.resendOAuthRegisterCode(email);
      toast('success', '验证码已重新发送');
    } catch (error) {
      toast('error', error instanceof Error ? error.message : '发送失败');
    } finally {
      setResending(false);
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
            <h1 className="mt-4 text-xl font-semibold text-foreground-900">完成第三方注册</h1>
            <p className="mt-1 text-sm text-foreground-400">
              验证码已发送至你的第三方邮箱，确认后即可开通账号
            </p>
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-foreground-700 mb-1.5">邮箱</label>
              <div className="relative">
                <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-foreground-300" />
                <input
                  value={email}
                  readOnly
                  className="w-full h-11 pl-10 pr-4 rounded-lg bg-background-100 border border-background-200/70 text-sm text-foreground-600"
                />
              </div>
            </div>

            {needsInvite && (
              <div>
                <label htmlFor="invite" className="block text-sm font-medium text-foreground-700 mb-1.5">
                  邀请码
                </label>
                <div className="relative">
                  <Ticket className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-foreground-300" />
                  <input
                    id="invite"
                    value={invite}
                    onChange={e => setInvite(e.target.value.trim())}
                    required
                    placeholder="粘贴邀请 token 或完整链接中的 token"
                    className="w-full h-11 pl-10 pr-4 rounded-lg bg-background-50 border border-background-200/70 text-sm focus:outline-none focus:border-primary-300"
                  />
                </div>
                <p className="mt-1 text-[11px] text-foreground-400">
                  当前为邀请注册。也可先打开邀请链接，在邀请页使用 Google/GitHub。
                </p>
              </div>
            )}

            <div>
              <label htmlFor="otp" className="block text-sm font-medium text-foreground-700 mb-1.5">
                邮箱验证码
              </label>
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
                    className="w-full h-11 pl-10 pr-3 rounded-lg bg-background-50 border border-background-200/70 text-sm tracking-widest focus:outline-none focus:border-primary-300"
                  />
                </div>
                <button
                  type="button"
                  disabled={resending}
                  onClick={() => void handleResend()}
                  className="h-11 px-3 rounded-lg border border-background-200 text-xs font-medium text-foreground-600 hover:bg-background-100 disabled:opacity-50 whitespace-nowrap"
                >
                  {resending ? '发送中' : '重新发送'}
                </button>
              </div>
            </div>

            <button
              type="submit"
              disabled={loading || code.length < 6}
              className="w-full h-11 rounded-lg bg-primary-500 text-background-50 text-sm font-medium hover:bg-primary-600 disabled:opacity-50 inline-flex items-center justify-center gap-2"
            >
              {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : null}
              {loading ? '验证中…' : '验证并完成注册'}
            </button>
          </form>

          <p className="mt-6 text-center text-sm text-foreground-400">
            <Link to="/login" className="text-primary-600 hover:underline">返回登录</Link>
          </p>
        </div>
      </div>
    </PublicShell>
  );
}
