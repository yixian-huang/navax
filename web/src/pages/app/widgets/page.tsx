import { Calendar, Clock, MessageCircle, Type } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useMyPage, useUpdatePageSettings } from '@/hooks/useQueries';
import { ErrorState, LoadingSkeleton } from '@/components/base/SharedUI';
import { useToast } from '@/components/base/Toast';
import { draftSaveToastMessage } from '@/lib/publish-state';
import type { PageSettings } from '@/api/types';
import { cn } from '@/lib/utils';

const displayOptions = [
  { key: 'showClock' as const, label: '时钟', description: '在导航首页显示当前时间', icon: Clock },
  { key: 'showDate' as const, label: '日期', description: '显示日期和星期', icon: Calendar },
  { key: 'showGreeting' as const, label: '欢迎词', description: '根据时段显示问候和用户名', icon: MessageCircle },
];

export default function WidgetsPage() {
  const pageQuery = useMyPage();
  const updateSettings = useUpdatePageSettings();
  const { toast } = useToast();
  const [subtitle, setSubtitle] = useState('');
  const [clockFormat, setClockFormat] = useState<'24h' | '12h'>('24h');
  const [dateFormat, setDateFormat] = useState<'long' | 'short' | 'compact'>('long');
  const [showSeconds, setShowSeconds] = useState(true);

  const settings = pageQuery.data?.settings;

  useEffect(() => {
    if (!settings) return;
    setSubtitle(settings.display.subtitle ?? '');
    setClockFormat(settings.display.clockFormat === '12h' ? '12h' : '24h');
    setDateFormat(
      settings.display.dateFormat === 'short' || settings.display.dateFormat === 'compact'
        ? settings.display.dateFormat
        : 'long',
    );
    setShowSeconds(settings.display.showSeconds !== false);
  }, [settings]);

  if (pageQuery.isLoading) return <LoadingSkeleton count={3} />;
  if (pageQuery.isError || !settings) {
    return <ErrorState message={pageQuery.error?.message || '加载显示设置失败'} onRetry={() => pageQuery.refetch()} />;
  }

  const persist = (next: PageSettings, okMsg?: string) => {
    updateSettings.mutate(next, {
      onSuccess: () => toast('success', okMsg || draftSaveToastMessage(pageQuery.data?.publication)),
      onError: error => toast('error', error.message || '保存失败'),
    });
  };

  const handleToggle = (key: (typeof displayOptions)[number]['key']) => {
    persist({
      ...settings,
      display: { ...settings.display, [key]: !settings.display[key] },
    });
  };

  const handleSaveMeta = (event: React.FormEvent) => {
    event.preventDefault();
    persist({
      ...settings,
      display: {
        ...settings.display,
        subtitle: subtitle.trim(),
        clockFormat,
        dateFormat,
        showSeconds,
      },
    }, draftSaveToastMessage(pageQuery.data?.publication, '首页文案与格式已写入草稿'));
  };

  return (
    <div className="max-w-3xl">
      <div className="mb-6">
        <h1 className="text-2xl font-bold font-heading text-foreground-950">首页信息</h1>
        <p className="text-sm text-foreground-400 mt-1">控制导航首页的问候、副标题、时钟与日期展示</p>
      </div>

      <div className="space-y-3 mb-8">
        {displayOptions.map(option => {
          const Icon = option.icon;
          const enabled = settings.display[option.key];
          return (
            <div key={option.key} className="flex items-center gap-4 p-4 rounded-xl bg-background-50 border border-background-200/70">
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
                className={cn(
                  'relative w-11 h-6 rounded-full transition-colors disabled:opacity-50',
                  enabled ? 'bg-primary-500' : 'bg-background-200',
                )}
              >
                <span className={cn(
                  'absolute left-1 top-1 w-4 h-4 rounded-full bg-white shadow transition-transform',
                  enabled ? 'translate-x-5' : 'translate-x-0',
                )} />
              </button>
            </div>
          );
        })}
      </div>

      <form onSubmit={handleSaveMeta} className="rounded-xl border border-background-200/70 bg-background-50 p-5 space-y-4">
        <div className="flex items-center gap-2 mb-1">
          <Type className="w-4 h-4 text-primary-500" />
          <h2 className="text-sm font-semibold text-foreground-800">文案与格式</h2>
        </div>

        <label className="block text-xs text-foreground-500">
          副标题 / 标语（显示在问候语下方；留空则用默认文案，壁纸模式下默认不显示）
          <textarea
            value={subtitle}
            onChange={e => setSubtitle(e.target.value)}
            maxLength={120}
            rows={2}
            placeholder="例如：专注工具与阅读 · 私人导航台"
            className="mt-1.5 w-full px-3 py-2 rounded-lg border border-background-200/70 bg-white text-sm text-foreground-900 resize-none"
          />
        </label>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <label className="block text-xs text-foreground-500">
            时钟格式
            <select
              value={clockFormat}
              onChange={e => setClockFormat(e.target.value as '24h' | '12h')}
              className="mt-1.5 w-full h-9 px-3 rounded-lg border border-background-200/70 bg-white text-sm"
            >
              <option value="24h">24 小时制（14:30）</option>
              <option value="12h">12 小时制（2:30 PM）</option>
            </select>
          </label>
          <label className="block text-xs text-foreground-500">
            日期格式
            <select
              value={dateFormat}
              onChange={e => setDateFormat(e.target.value as 'long' | 'short' | 'compact')}
              className="mt-1.5 w-full h-9 px-3 rounded-lg border border-background-200/70 bg-white text-sm"
            >
              <option value="long">完整（2026年7月17日）</option>
              <option value="short">简短（7月17日）</option>
              <option value="compact">数字（07/17）</option>
            </select>
          </label>
        </div>

        <label className="inline-flex items-center gap-2 text-sm text-foreground-600">
          <input
            type="checkbox"
            checked={showSeconds}
            onChange={e => setShowSeconds(e.target.checked)}
          />
          显示秒数（非壁纸模式下时钟下方的 SEC 提示）
        </label>

        <div className="pt-1">
          <button
            type="submit"
            disabled={updateSettings.isPending}
            className="h-9 px-4 rounded-lg bg-primary-500 text-background-50 text-sm font-medium disabled:opacity-50"
          >
            {updateSettings.isPending ? '保存中…' : '保存文案与格式'}
          </button>
        </div>
      </form>
    </div>
  );
}
