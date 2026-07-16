import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { ArrowRight, CheckCircle2, LoaderCircle } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import { authApi } from '@/api/auth';
import PublicShell from '@/components/feature/PublicShell';
import { useToast } from '@/components/base/Toast';

type FormState = {
  setupToken: string;
  instanceName: string;
  publicBaseUrl: string;
  adminUsername: string;
  adminEmail: string;
  adminPassword: string;
};

const initialForm: FormState = {
  setupToken: '',
  instanceName: 'nav.ax',
  publicBaseUrl: typeof window === 'undefined' ? '' : window.location.origin,
  adminUsername: '',
  adminEmail: '',
  adminPassword: '',
};

export default function SetupPage() {
  const [form, setForm] = useState(initialForm);
  const [checking, setChecking] = useState(true);
  const [initialized, setInitialized] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();

  useEffect(() => {
    authApi.getBootstrapStatus()
      .then(response => {
        setInitialized(response.data.initialized);
        setForm(current => ({
          ...current,
          instanceName: response.data.instanceName || current.instanceName,
          publicBaseUrl: response.data.publicBaseUrl || current.publicBaseUrl,
        }));
      })
      .catch(() => toast('error', '无法读取实例初始化状态'))
      .finally(() => setChecking(false));
  }, [toast]);

  const update = (key: keyof FormState, value: string) => setForm(current => ({ ...current, [key]: value }));

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setSubmitting(true);
    try {
      const response = await authApi.bootstrap(form.setupToken.trim(), {
        instanceName: form.instanceName.trim(),
        publicBaseUrl: form.publicBaseUrl.trim().replace(/\/$/, ''),
        adminUsername: form.adminUsername.trim(),
        adminEmail: form.adminEmail.trim(),
        adminPassword: form.adminPassword,
      });
      queryClient.setQueryData(['auth', 'session'], response.data);
      toast('success', '实例初始化完成');
      navigate('/admin', { replace: true });
    } catch (error) {
      toast('error', error instanceof Error ? error.message : '初始化失败');
      setSubmitting(false);
    }
  };

  if (checking) {
    return <PublicShell showSearch={false}><div className="min-h-[70vh] flex items-center justify-center"><LoaderCircle className="w-6 h-6 animate-spin text-primary-500" aria-label="正在检查初始化状态" /></div></PublicShell>;
  }

  if (initialized) {
    return (
      <PublicShell showSearch={false}>
        <div className="min-h-[70vh] flex items-center justify-center px-4">
          <div className="w-full max-w-md rounded-2xl border border-background-200 bg-background-50 p-8 text-center">
            <CheckCircle2 className="w-10 h-10 text-green-500 mx-auto mb-4" />
            <h1 className="text-xl font-semibold text-foreground-900">实例已经初始化</h1>
            <p className="mt-2 text-sm text-foreground-400">请使用管理员账号登录。</p>
            <Link to="/login" className="mt-6 inline-flex h-10 px-5 items-center gap-2 rounded-lg bg-primary-500 text-background-50 text-sm font-medium">前往登录<ArrowRight className="w-4 h-4" /></Link>
          </div>
        </div>
      </PublicShell>
    );
  }

  return (
    <PublicShell showSearch={false}>
      <div className="min-h-[80vh] flex items-center justify-center px-4 py-10">
        <div className="w-full max-w-lg">
          <div className="text-center mb-7">
            <h1 className="text-2xl font-bold font-heading text-foreground-950">初始化 nav.ax</h1>
            <p className="mt-2 text-sm text-foreground-400">创建首个管理员并设置实例公开地址</p>
          </div>
          <form onSubmit={submit} className="rounded-2xl border border-background-200 bg-background-50 p-6 space-y-4">
            <SetupInput label="初始化令牌" type="password" autoComplete="off" value={form.setupToken} onChange={value => update('setupToken', value)} minLength={32} />
            <div className="grid sm:grid-cols-2 gap-4">
              <SetupInput label="实例名称" value={form.instanceName} onChange={value => update('instanceName', value)} />
              <SetupInput label="公开地址" type="url" value={form.publicBaseUrl} onChange={value => update('publicBaseUrl', value)} />
              <SetupInput label="管理员用户名" autoComplete="username" value={form.adminUsername} onChange={value => update('adminUsername', value)} minLength={3} />
              <SetupInput label="管理员邮箱" type="email" autoComplete="email" value={form.adminEmail} onChange={value => update('adminEmail', value)} />
            </div>
            <SetupInput label="管理员密码" type="password" autoComplete="new-password" value={form.adminPassword} onChange={value => update('adminPassword', value)} minLength={12} />
            <button type="submit" disabled={submitting} className="w-full h-11 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 disabled:opacity-50 flex items-center justify-center gap-2">
              {submitting ? <LoaderCircle className="w-4 h-4 animate-spin" /> : <ArrowRight className="w-4 h-4" />}
              {submitting ? '正在初始化…' : '完成初始化'}
            </button>
          </form>
        </div>
      </div>
    </PublicShell>
  );
}

function SetupInput({ label, value, onChange, type = 'text', autoComplete, minLength }: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  type?: string;
  autoComplete?: string;
  minLength?: number;
}) {
  return (
    <label className="block">
      <span className="block text-sm font-medium text-foreground-700 mb-1.5">{label}</span>
      <input type={type} autoComplete={autoComplete} minLength={minLength} required value={value} onChange={event => onChange(event.target.value)} className="w-full h-10 px-3 rounded-lg bg-background-100 border border-background-200 text-sm text-foreground-900 focus:outline-none focus:border-primary-300 focus:ring-1 focus:ring-primary-200" />
    </label>
  );
}
