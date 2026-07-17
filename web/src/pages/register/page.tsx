// ============================================================
// nav.ax Open Registration Page — /register
// Only available when system registrationMode is "open".
// ============================================================

import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import PublicShell from '@/components/feature/PublicShell';
import { User, Lock, Mail, ArrowRight, Loader2 } from 'lucide-react';
import { useToast } from '@/components/base/Toast';
import { authApi } from '@/api/auth';
import { getPublicConfig } from '@/api/assets';
import { useQueryClient } from '@tanstack/react-query';

export default function RegisterPage() {
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [checking, setChecking] = useState(true);
  const [open, setOpen] = useState(false);
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
    return () => { cancelled = true; };
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      const response = await authApi.registerOpen({ username, email, password });
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
            <Link to="/" className="inline-block">
              <span className="text-2xl font-bold font-heading text-foreground-950">nav.ax</span>
            </Link>
            <h1 className="mt-4 text-xl font-semibold text-foreground-900">创建账号</h1>
            <p className="mt-1 text-sm text-foreground-400">注册后即可拥有自己的导航主页</p>
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label htmlFor="username" className="block text-sm font-medium text-foreground-700 mb-1.5">用户名</label>
              <div className="relative">
                <User className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-foreground-300" />
                <input
                  id="username"
                  value={username}
                  onChange={e => setUsername(e.target.value)}
                  required
                  minLength={2}
                  maxLength={32}
                  className="w-full h-11 pl-10 pr-4 rounded-lg bg-background-50 border border-background-200/70 text-sm focus:outline-none focus:border-primary-300"
                />
              </div>
            </div>
            <div>
              <label htmlFor="email" className="block text-sm font-medium text-foreground-700 mb-1.5">邮箱</label>
              <div className="relative">
                <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-foreground-300" />
                <input
                  id="email"
                  type="email"
                  value={email}
                  onChange={e => setEmail(e.target.value)}
                  required
                  className="w-full h-11 pl-10 pr-4 rounded-lg bg-background-50 border border-background-200/70 text-sm focus:outline-none focus:border-primary-300"
                />
              </div>
            </div>
            <div>
              <label htmlFor="password" className="block text-sm font-medium text-foreground-700 mb-1.5">密码</label>
              <div className="relative">
                <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-foreground-300" />
                <input
                  id="password"
                  type="password"
                  value={password}
                  onChange={e => setPassword(e.target.value)}
                  required
                  minLength={8}
                  className="w-full h-11 pl-10 pr-4 rounded-lg bg-background-50 border border-background-200/70 text-sm focus:outline-none focus:border-primary-300"
                />
              </div>
            </div>
            <button
              type="submit"
              disabled={loading}
              className="w-full h-11 rounded-lg bg-primary-500 text-background-50 text-sm font-medium hover:bg-primary-600 disabled:opacity-50 inline-flex items-center justify-center gap-2"
            >
              {loading ? '注册中...' : <>注册 <ArrowRight className="w-4 h-4" /></>}
            </button>
          </form>

          <p className="mt-6 text-center text-sm text-foreground-400">
            已有账号？{' '}
            <Link to="/login" className="text-primary-600 hover:underline">去登录</Link>
          </p>
        </div>
      </div>
    </PublicShell>
  );
}
