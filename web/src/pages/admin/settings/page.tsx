import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { RotateCw, Save, Palette } from 'lucide-react';
import { useAdminSettings, useUpdateAdminSettings } from '@/hooks/useQueries';
import { ErrorState, LoadingSkeleton } from '@/components/base/SharedUI';
import { useToast } from '@/components/base/Toast';
import type { SystemSettings } from '@/api/types';

const inputClass = 'w-full h-9 px-3 rounded-lg bg-background-50 border border-background-200/70 text-sm text-foreground-900 focus:outline-none focus:border-primary-300';

export default function AdminSettingsPage() {
  const settingsQuery = useAdminSettings();
  const updateSettings = useUpdateAdminSettings();
  const { toast } = useToast();
  const [form, setForm] = useState<SystemSettings | null>(null);

  useEffect(() => {
    if (settingsQuery.data) setForm(structuredClone(settingsQuery.data));
  }, [settingsQuery.data]);

  if (settingsQuery.isLoading) return <LoadingSkeleton count={4} />;
  if (settingsQuery.isError || !form) {
    return <ErrorState message={settingsQuery.error?.message || '加载系统设置失败'} onRetry={() => settingsQuery.refetch()} />;
  }

  const handleSave = () => {
    updateSettings.mutate(form, {
      onSuccess: response => {
        setForm(structuredClone(response.data));
        toast('success', '系统配置已保存');
      },
      onError: error => toast('error', error.message || '保存失败'),
    });
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-5">
        <div>
          <h1 className="text-xl font-bold font-heading text-foreground-950">系统配置</h1>
          <p className="text-xs text-foreground-400 mt-0.5">严格对应实例公开地址、注册、配额、统计和子域名设置</p>
        </div>
        <button
          onClick={handleSave}
          disabled={updateSettings.isPending}
          className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 text-sm font-medium disabled:opacity-50 inline-flex items-center gap-2"
        >
          {updateSettings.isPending ? <RotateCw className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
          保存配置
        </button>
      </div>

      <div className="max-w-2xl space-y-4">
        <Section title="实例信息">
          <Field label="实例名称">
            <input value={form.instanceName} onChange={event => setForm({ ...form, instanceName: event.target.value })} required maxLength={60} className={inputClass} />
          </Field>
          <Field label="公开基础 URL">
            <input type="url" value={form.publicBaseUrl} onChange={event => setForm({ ...form, publicBaseUrl: event.target.value })} required className={inputClass} />
          </Field>
          <Field label="注册模式">
            <select value={form.registrationMode} onChange={event => setForm({ ...form, registrationMode: event.target.value as 'invite' | 'closed' | 'open' })} className={inputClass}>
              <option value="invite">仅邀请注册</option>
              <option value="open">公开注册</option>
              <option value="closed">关闭注册</option>
            </select>
          </Field>
        </Section>

        <Section title="页面与上传限制">
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <NumberField label="每页最大分类" value={form.limits.maxCategoriesPerPage} min={1} max={500} onChange={value => setForm({ ...form, limits: { ...form.limits, maxCategoriesPerPage: value } })} />
            <NumberField label="每页最大站点" value={form.limits.maxSitesPerPage} min={1} max={10000} onChange={value => setForm({ ...form, limits: { ...form.limits, maxSitesPerPage: value } })} />
            <NumberField label="最大上传字节" value={form.limits.maxUploadBytes} min={1024} max={52428800} onChange={value => setForm({ ...form, limits: { ...form.limits, maxUploadBytes: value } })} />
          </div>
        </Section>

        <Section title="访问统计">
          <Toggle label="启用匿名访问统计" checked={form.analytics.enabled} onChange={enabled => setForm({ ...form, analytics: { ...form.analytics, enabled } })} />
          <NumberField label="统计保留天数" value={form.analytics.retentionDays} min={7} max={365} onChange={value => setForm({ ...form, analytics: { ...form.analytics, retentionDays: value } })} />
        </Section>

        <Section title="子域名">
          <Field label="根域名（启用子域名时必填；留空并开启时将尝试从公开 URL 推导）">
            <input
              value={form.domain.rootDomain ?? ''}
              onChange={event => setForm({ ...form, domain: { ...form.domain, rootDomain: event.target.value.trim() || null } })}
              placeholder="例如 nav.ax"
              className={inputClass}
            />
          </Field>
          <Toggle
            label="允许用户申请子域名"
            checked={form.domain.subdomainsEnabled}
            onChange={subdomainsEnabled => setForm({ ...form, domain: { ...form.domain, subdomainsEnabled } })}
          />
          {form.domain.subdomainsEnabled && !form.domain.rootDomain && (
            <p className="text-xs text-accent-600">开启后若未填写根域名，保存时将用公开基础 URL 的主机名作为根域名；否则申请接口会报「尚未启用」。</p>
          )}
        </Section>

        <Section title="平台主题库">
          <p className="text-xs text-foreground-500 leading-relaxed">
            启用/停用内置主题、设置新建导航的默认主题。这与工作台里用户为自己的导航选外观不同。
          </p>
          <Link
            to="/admin/themes"
            className="inline-flex items-center gap-2 h-9 px-3 rounded-lg border border-background-200 text-sm text-foreground-700 hover:bg-background-100 transition-colors"
          >
            <Palette className="w-4 h-4 text-primary-600" />
            管理主题库
          </Link>
        </Section>
      </div>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return <section className="bg-white rounded-lg border border-background-200/70 p-4 space-y-3"><h2 className="text-sm font-semibold text-foreground-700">{title}</h2>{children}</section>;
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <label className="block"><span className="block text-xs font-medium text-foreground-500 mb-1.5">{label}</span>{children}</label>;
}

function NumberField({ label, value, min, max, onChange }: { label: string; value: number; min: number; max: number; onChange: (value: number) => void }) {
  return <Field label={label}><input type="number" value={value} min={min} max={max} onChange={event => onChange(Number(event.target.value))} className={inputClass} /></Field>;
}

function Toggle({ label, checked, onChange }: { label: string; checked: boolean; onChange: (checked: boolean) => void }) {
  return (
    <label className="flex items-center justify-between gap-4 text-sm text-foreground-700">
      {label}
      <input type="checkbox" checked={checked} onChange={event => onChange(event.target.checked)} className="w-4 h-4 accent-primary-500" />
    </label>
  );
}
