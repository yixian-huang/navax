import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, Info, KeyRound, Mail, Network, RotateCw, Save, Server } from 'lucide-react';
import { adminApi } from '@/api/admin';
import type { ProviderConfig, ProviderKind, ProviderSettings } from '@/api/types';
import { ErrorState, LoadingSkeleton } from '@/components/base/SharedUI';
import { FormField, FormInput, FormSelect } from '@/components/base/FormField';
import { useToast } from '@/components/base/Toast';

const providerKinds: ProviderKind[] = ['smtp', 'storage', 'dns'];

const providerMeta = {
  smtp: { label: 'SMTP 邮件', description: '邀请邮件、密码找回等系统通知', icon: Mail, secret: 'password', secretLabel: 'SMTP 密码' },
  storage: { label: '对象存储', description: '图标 / 图片上传（本地磁盘或 S3 兼容存储）', icon: Server, secret: 'secretKey', secretLabel: 'Secret Key' },
  dns: { label: 'DNS 服务', description: '子域名自动化扩展预留（暂未接入）', icon: Network, secret: 'token', secretLabel: 'API Token' },
} as const;

type FormSettings = Record<string, string | number | boolean>;

function text(settings: Record<string, unknown>, key: string, fallback = '') {
  return typeof settings[key] === 'string' ? settings[key] as string : fallback;
}

function number(settings: Record<string, unknown>, key: string, fallback: number) {
  return typeof settings[key] === 'number' ? settings[key] as number : fallback;
}

function boolean(settings: Record<string, unknown>, key: string, fallback = false) {
  return typeof settings[key] === 'boolean' ? settings[key] as boolean : fallback;
}

function initialSettings(provider: ProviderConfig): FormSettings {
  const settings = provider.settings;
  if (provider.kind === 'smtp') {
    return {
      host: text(settings, 'host'), port: number(settings, 'port', 587), tlsMode: text(settings, 'tlsMode', 'starttls'),
      username: text(settings, 'username'), fromName: text(settings, 'fromName'), fromAddress: text(settings, 'fromAddress'),
    };
  }
  if (provider.kind === 'storage') {
    return {
      driver: text(settings, 'driver', 'local'), endpoint: text(settings, 'endpoint'), region: text(settings, 'region'),
      bucket: text(settings, 'bucket'), prefix: text(settings, 'prefix'), pathStyle: boolean(settings, 'pathStyle'),
      accessKey: text(settings, 'accessKey'), publicBaseUrl: text(settings, 'publicBaseUrl'),
    };
  }
  return {
    provider: text(settings, 'provider'), zoneId: text(settings, 'zoneId'), apiEndpoint: text(settings, 'apiEndpoint'),
    ttl: number(settings, 'ttl', 300),
  };
}

function providerSettings(kind: ProviderKind, settings: FormSettings): ProviderSettings {
  if (kind === 'smtp') {
    return {
      host: String(settings.host), port: Number(settings.port), tlsMode: settings.tlsMode as 'none' | 'starttls' | 'tls',
      username: String(settings.username), fromName: String(settings.fromName), fromAddress: String(settings.fromAddress),
    };
  }
  if (kind === 'storage') {
    const driver = settings.driver as 'local' | 's3';
    if (driver === 'local') return { driver };
    const publicBaseUrl = String(settings.publicBaseUrl).trim();
    const prefix = String(settings.prefix).trim();
    return {
      driver,
      endpoint: String(settings.endpoint).trim(),
      region: String(settings.region).trim(),
      bucket: String(settings.bucket).trim(),
      accessKey: String(settings.accessKey).trim(),
      pathStyle: Boolean(settings.pathStyle),
      ...(prefix ? { prefix } : {}),
      ...(publicBaseUrl ? { publicBaseUrl } : {}),
    };
  }
  const apiEndpoint = String(settings.apiEndpoint).trim();
  return {
    provider: String(settings.provider).trim(), zoneId: String(settings.zoneId).trim(), ttl: Number(settings.ttl),
    ...(apiEndpoint ? { apiEndpoint } : {}),
  };
}

function ProviderCard({ provider }: { provider: ProviderConfig }) {
  const meta = providerMeta[provider.kind];
  const Icon = meta.icon;
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [enabled, setEnabled] = useState(provider.enabled);
  const [settings, setSettings] = useState<FormSettings>(() => initialSettings(provider));
  const [secret, setSecret] = useState('');
  const [recipient, setRecipient] = useState('');
  const update = (key: string, value: string | number | boolean) => setSettings(current => ({ ...current, [key]: value }));

  const saveMutation = useMutation({
    mutationFn: () => adminApi.updateProviderConfig(provider.kind, {
      enabled,
      settings: providerSettings(provider.kind, settings),
      ...(secret ? { secrets: { [meta.secret]: secret } } : {}),
    }),
    onSuccess: async () => {
      setSecret('');
      await queryClient.invalidateQueries({ queryKey: ['admin', 'operations', 'providers'] });
      toast('success', `${meta.label}配置已保存`);
    },
    onError: (error: Error) => toast('error', error.message || '保存服务配置失败'),
  });
  const testMutation = useMutation({
    mutationFn: () => adminApi.testProviderConfig(provider.kind, provider.kind === 'smtp' ? recipient.trim() || undefined : undefined),
    onSuccess: response => toast(response.data.success ? 'success' : 'warning', response.data.message),
    onError: (error: Error) => toast('error', error.message || '连接测试失败'),
  });

  return (
    <section className="bg-white rounded-xl border border-background-200/70 p-4 space-y-4">
      <div className="flex items-start gap-3">
        <div className="w-9 h-9 rounded-lg bg-background-100 flex items-center justify-center">
          <Icon className="w-4 h-4 text-foreground-600" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 flex-wrap">
            <h3 className="text-sm font-semibold text-foreground-800">{meta.label}</h3>
            <span className={`text-[10px] px-2 py-0.5 rounded-full ${provider.configured ? 'bg-accent-50 text-accent-700' : 'bg-background-100 text-foreground-500'}`}>
              {provider.configured ? '已配置' : '未配置'}
            </span>
          </div>
          <p className="text-xs text-foreground-400 mt-0.5">{meta.description}</p>
        </div>
        <label className="flex items-center gap-2 text-xs text-foreground-500 cursor-pointer">
          <input type="checkbox" checked={enabled} onChange={event => setEnabled(event.target.checked)} className="accent-primary-500" />
          启用
        </label>
      </div>

      <div className="grid sm:grid-cols-2 gap-3">
        {provider.kind === 'smtp' ? (
          <>
            <FormField label="SMTP 主机"><FormInput value={String(settings.host)} onChange={event => update('host', event.target.value)} /></FormField>
            <FormField label="端口"><FormInput type="number" min={1} max={65535} value={Number(settings.port)} onChange={event => update('port', Number(event.target.value))} /></FormField>
            <FormField label="TLS 模式"><FormSelect value={String(settings.tlsMode)} onChange={event => update('tlsMode', event.target.value)}><option value="none">无</option><option value="starttls">STARTTLS</option><option value="tls">TLS</option></FormSelect></FormField>
            <FormField label="用户名"><FormInput value={String(settings.username)} onChange={event => update('username', event.target.value)} /></FormField>
            <FormField label="发件人名称"><FormInput value={String(settings.fromName)} onChange={event => update('fromName', event.target.value)} /></FormField>
            <FormField label="发件地址"><FormInput type="email" value={String(settings.fromAddress)} onChange={event => update('fromAddress', event.target.value)} /></FormField>
          </>
        ) : provider.kind === 'storage' ? (
          <>
            <FormField label="存储驱动"><FormSelect value={String(settings.driver)} onChange={event => update('driver', event.target.value)}><option value="local">本地存储</option><option value="s3">S3 兼容</option></FormSelect></FormField>
            {settings.driver === 's3' ? (
              <>
                <FormField label="Endpoint"><FormInput value={String(settings.endpoint)} onChange={event => update('endpoint', event.target.value)} /></FormField>
                <FormField label="Region"><FormInput value={String(settings.region)} onChange={event => update('region', event.target.value)} /></FormField>
                <FormField label="Bucket"><FormInput value={String(settings.bucket)} onChange={event => update('bucket', event.target.value)} /></FormField>
                <FormField label="路径前缀"><FormInput value={String(settings.prefix)} onChange={event => update('prefix', event.target.value)} /></FormField>
                <FormField label="Access Key"><FormInput value={String(settings.accessKey)} onChange={event => update('accessKey', event.target.value)} /></FormField>
                <FormField label="公开访问地址"><FormInput type="url" value={String(settings.publicBaseUrl)} onChange={event => update('publicBaseUrl', event.target.value)} /></FormField>
                <label className="flex items-center gap-2 text-xs text-foreground-500 self-end h-9"><input type="checkbox" checked={Boolean(settings.pathStyle)} onChange={event => update('pathStyle', event.target.checked)} className="accent-primary-500" />使用 Path Style</label>
              </>
            ) : null}
          </>
        ) : (
          <>
            <FormField label="DNS Provider"><FormInput value={String(settings.provider)} onChange={event => update('provider', event.target.value)} placeholder="cloudflare" /></FormField>
            <FormField label="Zone ID"><FormInput value={String(settings.zoneId)} onChange={event => update('zoneId', event.target.value)} /></FormField>
            <FormField label="API Endpoint"><FormInput type="url" value={String(settings.apiEndpoint)} onChange={event => update('apiEndpoint', event.target.value)} /></FormField>
            <FormField label="TTL（秒）"><FormInput type="number" min={60} max={86400} value={Number(settings.ttl)} onChange={event => update('ttl', Number(event.target.value))} /></FormField>
          </>
        )}
      </div>

      {provider.kind === 'storage' && settings.driver === 's3' ? (
        <div className="rounded-lg bg-primary-50 border border-primary-200/70 p-3 flex items-start gap-2">
          <Info className="w-3.5 h-3.5 text-primary-600 mt-0.5 shrink-0" />
          <p className="text-xs text-primary-800 leading-relaxed">
            启用后新上传的图片会写入 S3 兼容存储。若填写 publicBaseUrl，返回的资源 URL 将使用该公共前缀；否则仍通过本站 <code>/api/v1/assets/…</code> 代理读取。
          </p>
        </div>
      ) : null}
      {provider.kind === 'dns' ? (
        <div className="rounded-lg bg-amber-50 border border-amber-200/70 p-3 flex items-start gap-2">
          <Info className="w-3.5 h-3.5 text-amber-600 mt-0.5 shrink-0" />
          <p className="text-xs text-amber-800 leading-relaxed">
            DNS 自动化尚未接入：子域名不会自动创建解析记录，需运维在反代 / DNS 侧配置通配解析。此处仅用于保存配置与凭据连通性测试。
          </p>
        </div>
      ) : null}

      <div className="rounded-lg bg-background-50 border border-background-200/60 p-3">
        <div className="flex items-center gap-2 mb-2">
          <KeyRound className="w-3.5 h-3.5 text-foreground-400" />
          <span className="text-xs font-medium text-foreground-600">{meta.secretLabel}</span>
          {provider.hasSecret ? <span className="text-[10px] text-accent-700 flex items-center gap-1"><CheckCircle2 className="w-3 h-3" />已有凭据</span> : null}
        </div>
        <FormInput type="password" autoComplete="new-password" value={secret} onChange={event => setSecret(event.target.value)} placeholder={provider.hasSecret ? '留空以保留现有凭据' : '输入后仅写入，不会回显'} />
      </div>

      {provider.kind === 'smtp' ? <FormField label="测试收件人（可选）"><FormInput type="email" value={recipient} onChange={event => setRecipient(event.target.value)} placeholder="admin@example.com" /></FormField> : null}

      <div className="flex items-center justify-end gap-2">
        <button onClick={() => testMutation.mutate()} disabled={!provider.configured || testMutation.isPending} className="h-8 px-3 rounded-lg border border-background-200 text-xs text-foreground-600 hover:bg-background-50 disabled:opacity-40 flex items-center gap-1.5">
          <RotateCw className={`w-3.5 h-3.5 ${testMutation.isPending ? 'animate-spin' : ''}`} />测试
        </button>
        <button onClick={() => saveMutation.mutate()} disabled={saveMutation.isPending} className="h-8 px-3 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-xs font-medium hover:bg-primary-600 disabled:opacity-50 flex items-center gap-1.5">
          {saveMutation.isPending ? <RotateCw className="w-3.5 h-3.5 animate-spin" /> : <Save className="w-3.5 h-3.5" />}保存
        </button>
      </div>
    </section>
  );
}

export default function ProvidersSection() {
  const query = useQuery({
    queryKey: ['admin', 'operations', 'providers'],
    queryFn: async () => Promise.all(providerKinds.map(async kind => (await adminApi.getProviderConfig(kind)).data)),
  });
  if (query.isLoading) return <LoadingSkeleton count={3} />;
  if (query.error || !query.data) return <ErrorState message={(query.error as Error)?.message || '加载服务配置失败'} onRetry={() => query.refetch()} />;
  return <div className="grid xl:grid-cols-2 gap-4">{query.data.map(provider => <ProviderCard key={`${provider.kind}:${provider.updatedAt ?? 'never'}`} provider={provider} />)}</div>;
}
