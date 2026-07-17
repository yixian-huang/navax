import { Calendar, Clock, MessageCircle } from 'lucide-react';
import { useMyPage, useUpdatePageSettings } from '@/hooks/useQueries';
import { ErrorState, LoadingSkeleton } from '@/components/base/SharedUI';
import { useToast } from '@/components/base/Toast';
import { draftSaveToastMessage } from '@/lib/publish-state';

const displayOptions = [
  { key: 'showClock', label: '时钟', description: '在导航首页显示当前时间', icon: Clock },
  { key: 'showDate', label: '日期', description: '显示完整日期和星期', icon: Calendar },
  { key: 'showGreeting', label: '欢迎词', description: '根据时段显示问候和用户名', icon: MessageCircle },
] as const;

export default function WidgetsPage() {
  const pageQuery = useMyPage();
  const updateSettings = useUpdatePageSettings();
  const { toast } = useToast();

  if (pageQuery.isLoading) return <LoadingSkeleton count={3} />;
  if (pageQuery.isError || !pageQuery.data?.settings) {
    return <ErrorState message={pageQuery.error?.message || '加载显示设置失败'} onRetry={() => pageQuery.refetch()} />;
  }

  const settings = pageQuery.data.settings;
  const handleToggle = (key: (typeof displayOptions)[number]['key']) => {
    updateSettings.mutate({
      ...settings,
      display: { ...settings.display, [key]: !settings.display[key] },
    }, {
      onSuccess: () => toast('success', draftSaveToastMessage(pageQuery.data?.publication)),
      onError: error => toast('error', error.message || '保存失败'),
    });
  };

  return (
    <div className="max-w-2xl">
      <div className="mb-6">
        <h1 className="text-2xl font-bold font-heading text-foreground-950">首页信息</h1>
        <p className="text-sm text-foreground-400 mt-1">控制导航首页的时钟、日期和欢迎词</p>
      </div>
      <div className="space-y-3">
        {displayOptions.map(option => {
          const Icon = option.icon;
          const enabled = settings.display[option.key];
          return (
            <div key={option.key} className="flex items-center gap-4 p-4 rounded-xl bg-white border border-background-200/70">
              <div className="w-10 h-10 rounded-lg bg-primary-50 flex items-center justify-center">
                <Icon className="w-5 h-5 text-primary-600" />
              </div>
              <div className="flex-1">
                <div className="text-sm font-medium text-foreground-900">{option.label}</div>
                <div className="text-xs text-foreground-400 mt-0.5">{option.description}</div>
              </div>
              <button
                type="button"
                role="switch"
                aria-checked={enabled}
                aria-label={`${enabled ? '关闭' : '开启'}${option.label}`}
                disabled={updateSettings.isPending}
                onClick={() => handleToggle(option.key)}
                className={`relative w-11 h-6 rounded-full transition-colors ${enabled ? 'bg-primary-500' : 'bg-background-200'} disabled:opacity-50`}
              >
                <span className={`absolute left-1 top-1 w-4 h-4 rounded-full bg-white shadow transition-transform ${enabled ? 'translate-x-5' : 'translate-x-0'}`} />
              </button>
            </div>
          );
        })}
      </div>
    </div>
  );
}
