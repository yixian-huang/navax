// ============================================================
// nav.ax IconPicker — visual icon selector with 4 tabs
// Emoji · Remix Icon Grid · Image URL · File Upload
// ============================================================

import { useState, useMemo } from 'react';
import { Search, Link2, Upload, Smile, Grid3X3, X } from 'lucide-react';
import { cn } from '@/lib/utils';
import IconRenderer, { detectIconType } from '@/components/base/IconRenderer';

// ---- Emoji catalog ----
const EMOJI_LIST = [
  '🔧', '🎨', '🚀', '📚', '💬', '📰', '🛒', '🎮',
  '☁️', '🗄️', '🌐', '❤️', '🔥', '💻', '📱', '⚙️',
  '🛠️', '📦', '📝', '🔍', '📊', '🎯', '💡', '🏠',
  '⭐', '🔗', '📌', '🏷️', '🎵', '🎬', '📷', '🎤',
  '🎧', '📡', '🛡️', '🔐', '🏆', '🎓', '📖', '✨',
  '💼', '🗂️', '📋', '✅', '⚡', '🕐', '🔔', '📢',
  '🧠', '🎪', '🏗️', '🧩', '🌟', '💎', '🏃', '🎁',
  '🗺️', '🧪', '🎛️', '📯', '🔮', '🧲', '🪄', '🎠',
];

// ---- Remix Icon catalog (common tech/productivity icons) ----
const REMIX_ICONS = [
  'ri-github-fill', 'ri-gitlab-fill', 'ri-stack-overflow-fill', 'ri-npmjs-fill',
  'ri-vercel-fill', 'ri-notion-fill', 'ri-pen-nib-fill', 'ri-slack-fill',
  'ri-discord-fill', 'ri-youtube-fill', 'ri-twitter-x-fill', 'ri-reddit-fill',
  'ri-instagram-fill', 'ri-facebook-fill', 'ri-linkedin-fill', 'ri-medium-fill',
  'ri-google-fill', 'ri-chrome-fill', 'ri-edge-fill', 'ri-safari-fill',
  'ri-server-fill', 'ri-ubuntu-fill', 'ri-apple-fill', 'ri-windows-fill',
  'ri-android-fill', 'ri-dribbble-fill', 'ri-behance-fill', 'ri-codepen-fill',
  'ri-terminal-box-fill', 'ri-code-s-slash-line', 'ri-code-box-fill', 'ri-braces-fill',
  'ri-palette-line', 'ri-paint-fill', 'ri-pen-nib-fill', 'ri-pencil-ruler-line',
  'ri-book-open-line', 'ri-book-2-fill', 'ri-book-3-line', 'ri-booklet-line',
  'ri-rocket-line', 'ri-rocket-2-fill', 'ri-flashlight-fill', 'ri-lightbulb-flash-line',
  'ri-cloud-fill', 'ri-cloud-line', 'ri-server-fill', 'ri-database-2-fill',
  'ri-global-line', 'ri-earth-fill', 'ri-compass-line', 'ri-map-pin-line',
  'ri-lock-fill', 'ri-shield-fill', 'ri-key-2-fill', 'ri-fingerprint-line',
  'ri-file-code-fill', 'ri-file-text-line', 'ri-markdown-line', 'ri-article-line',
  'ri-camera-line', 'ri-image-line', 'ri-video-line', 'ri-movie-line',
  'ri-music-fill', 'ri-headphone-fill', 'ri-mic-fill', 'ri-speaker-line',
  'ri-shopping-bag-line', 'ri-shopping-cart-line', 'ri-store-line', 'ri-bank-card-line',
  'ri-heart-fill', 'ri-star-fill', 'ri-thumb-up-fill', 'ri-fire-fill',
  'ri-chat-3-line', 'ri-question-answer-line', 'ri-message-2-fill', 'ri-chat-smile-2-line',
  'ri-newspaper-line', 'ri-news-fill', 'ri-rss-fill', 'ri-bookmark-line',
  'ri-calendar-line', 'ri-calendar-check-line', 'ri-timer-line', 'ri-history-line',
  'ri-check-double-line', 'ri-task-line', 'ri-list-check', 'ri-survey-line',
  'ri-ruler-line', 'ri-scissors-line', 'ri-tools-line', 'ri-settings-4-line',
  'ri-mail-line', 'ri-mail-send-line', 'ri-inbox-line', 'ri-at-line',
  'ri-user-line', 'ri-group-line', 'ri-team-line', 'ri-user-star-line',
  'ri-layout-4-fill', 'ri-layout-grid-fill', 'ri-table-2', 'ri-dashboard-line',
  'ri-pie-chart-line', 'ri-bar-chart-line', 'ri-line-chart-fill', 'ri-funds-line',
  'ri-gamepad-line', 'ri-ghost-line', 'ri-emotion-line', 'ri-bug-line',
  'ri-brain-fill', 'ri-psychotherapy-line', 'ri-lightbulb-line', 'ri-award-line',
];

interface IconPickerProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  /** Show a compact mode for smaller spaces */
  compact?: boolean;
}

type PickerTab = 'emoji' | 'icons' | 'url' | 'upload';

export default function IconPicker({ value, onChange, placeholder, compact = false }: IconPickerProps) {
  const [tab, setTab] = useState<PickerTab>(() => {
    if (!value) return 'emoji';
    const t = detectIconType(value);
    if (t === 'image') return 'url';
    if (t === 'emoji') return 'emoji';
    return 'icons';
  });
  const [search, setSearch] = useState('');
  const [urlInput, setUrlInput] = useState(value && detectIconType(value) === 'image' ? value : '');
  const [dragOver, setDragOver] = useState(false);
  const [previewUrl, setPreviewUrl] = useState(value && detectIconType(value) === 'image' ? value : '');

  const filteredIcons = useMemo(() => {
    if (!search.trim()) return REMIX_ICONS;
    const q = search.toLowerCase().trim();
    return REMIX_ICONS.filter(icon => icon.replace('ri-', '').replace(/-/g, ' ').includes(q));
  }, [search]);

  const handleEmojiPick = (emoji: string) => {
    onChange(emoji);
  };

  const handleRemixPick = (icon: string) => {
    onChange(icon);
  };

  const handleUrlChange = (url: string) => {
    setUrlInput(url);
    setPreviewUrl(url);
    if (/^https?:\/\/.+/.test(url.trim())) {
      onChange(url.trim());
    }
  };

  const handleFileDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
    const file = e.dataTransfer.files[0];
    if (file && file.type.startsWith('image/')) {
      handleFileSelect(file);
    }
  };

  const handleFileSelect = (file: File) => {
    // For now, we use a local data URL. In production, this should upload to Supabase Storage.
    const reader = new FileReader();
    reader.onload = (e) => {
      const dataUrl = e.target?.result as string;
      setPreviewUrl(dataUrl);
      setUrlInput(dataUrl);
      onChange(dataUrl);
    };
    reader.readAsDataURL(file);
  };

  const currentIconType = detectIconType(value);

  const tabs: { key: PickerTab; icon: React.ReactNode; label: string }[] = [
    { key: 'emoji', icon: <Smile className="w-3.5 h-3.5" />, label: 'Emoji' },
    { key: 'icons', icon: <Grid3X3 className="w-3.5 h-3.5" />, label: '图标库' },
    { key: 'url', icon: <Link2 className="w-3.5 h-3.5" />, label: '图片链接' },
    { key: 'upload', icon: <Upload className="w-3.5 h-3.5" />, label: '上传' },
  ];

  return (
    <div className="space-y-2">
      {/* Current icon preview */}
      {value && (
        <div className="flex items-center gap-2 px-2 py-1.5 rounded-lg bg-background-50 border border-background-200/70">
          <IconRenderer
            icon={value}
            containerClassName={compact ? 'w-8 h-8' : 'w-10 h-10'}
            className={compact ? 'text-sm' : 'text-lg'}
          />
          <div className="flex-1 min-w-0">
            <div className="text-xs font-medium text-foreground-700 truncate">当前图标</div>
            <div className="text-[10px] text-foreground-400 truncate">
              {currentIconType === 'emoji' && 'Emoji 表情'}
              {currentIconType === 'image' && '自定义图片'}
              {currentIconType === 'remix' && 'Remix Icon'}
              {currentIconType === 'fallback' && '未设置'}
            </div>
          </div>
          <button
            type="button"
            onClick={() => onChange('')}
            className="w-6 h-6 flex items-center justify-center rounded text-foreground-300 hover:text-foreground-500 hover:bg-background-100 transition-colors duration-150"
          >
            <X className="w-3 h-3" />
          </button>
        </div>
      )}

      {/* Tabs */}
      <div className="flex items-center bg-background-100 rounded-lg p-0.5">
        {tabs.map(t => (
          <button
            key={t.key}
            type="button"
            onClick={() => setTab(t.key)}
            className={cn(
              'flex items-center gap-1 flex-1 h-7 rounded-md text-[10px] font-medium transition-colors duration-150 whitespace-nowrap justify-center',
              tab === t.key
                ? 'bg-white text-foreground-900 shadow-sm'
                : 'text-foreground-400 hover:text-foreground-600',
            )}
          >
            {t.icon}
            <span className="hidden sm:inline">{t.label}</span>
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div className={cn('rounded-lg bg-background-50 border border-background-200/70', compact ? 'max-h-40' : 'max-h-52')}>
        {tab === 'emoji' && (
          <div className="overflow-y-auto p-2 max-h-full">
            <div className="grid grid-cols-8 gap-0.5">
              {EMOJI_LIST.map(emoji => (
                <button
                  key={emoji}
                  type="button"
                  onClick={() => handleEmojiPick(emoji)}
                  className={cn(
                    'w-full aspect-square rounded-md flex items-center justify-center text-sm hover:bg-background-100 transition-colors duration-150',
                    value === emoji && 'bg-primary-100 ring-1 ring-primary-300',
                  )}
                >
                  {emoji}
                </button>
              ))}
            </div>
          </div>
        )}

        {tab === 'icons' && (
          <div className="flex flex-col">
            <div className="px-2 pt-2 pb-1.5">
              <div className="relative">
                <Search className="w-3 h-3 absolute left-2 top-1/2 -translate-y-1/2 text-foreground-300" />
                <input
                  type="text"
                  value={search}
                  onChange={e => setSearch(e.target.value)}
                  placeholder="搜索图标..."
                  className="w-full h-7 pl-7 pr-3 rounded-md bg-white border border-background-200/70 text-[11px] text-foreground-900 focus:outline-none focus:border-primary-300 transition-all duration-150"
                />
              </div>
            </div>
            <div className="overflow-y-auto p-2 max-h-full">
              <div className={cn('grid gap-0.5', compact ? 'grid-cols-7' : 'grid-cols-8')}>
                {filteredIcons.map(icon => (
                  <button
                    key={icon}
                    type="button"
                    onClick={() => handleRemixPick(icon)}
                    title={icon}
                    className={cn(
                      'w-full aspect-square rounded-md flex items-center justify-center text-xs hover:bg-background-100 transition-colors duration-150',
                      value === icon && 'bg-primary-100 ring-1 ring-primary-300 text-primary-600',
                      value !== icon && 'text-foreground-500',
                    )}
                  >
                    <i className={cn(icon, 'text-sm')} />
                  </button>
                ))}
              </div>
              {filteredIcons.length === 0 && (
                <div className="py-4 text-center text-[11px] text-foreground-400">
                  无匹配图标
                </div>
              )}
            </div>
          </div>
        )}

        {tab === 'url' && (
          <div className="p-2 space-y-2">
            <div className="relative">
              <Link2 className="w-3.5 h-3.5 absolute left-2.5 top-1/2 -translate-y-1/2 text-foreground-300" />
              <input
                type="url"
                value={urlInput}
                onChange={e => handleUrlChange(e.target.value)}
                placeholder="https://example.com/icon.png"
                className="w-full h-8 pl-8 pr-3 rounded-md bg-white border border-background-200/70 text-[11px] text-foreground-900 focus:outline-none focus:border-primary-300 transition-all duration-150"
              />
            </div>
            {previewUrl && detectIconType(previewUrl) === 'image' && (
              <div className="flex items-center justify-center p-2">
                <img
                  src={previewUrl}
                  alt="预览"
                  className={cn('rounded object-contain', compact ? 'max-h-16 max-w-full' : 'max-h-20 max-w-full')}
                  onError={(e) => {
                    (e.target as HTMLImageElement).style.display = 'none';
                  }}
                />
              </div>
            )}
            <p className="text-[10px] text-foreground-400 px-1">
              粘贴图片链接（支持 PNG / JPG / SVG / WebP）
            </p>
          </div>
        )}

        {tab === 'upload' && (
          <div
            className={cn(
              'p-3 flex flex-col items-center justify-center gap-2 border-2 border-dashed rounded-lg m-2 transition-colors duration-150',
              dragOver ? 'border-primary-400 bg-primary-50/50' : 'border-background-300',
            )}
            onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
            onDragLeave={() => setDragOver(false)}
            onDrop={handleFileDrop}
          >
            <Upload className="w-5 h-5 text-foreground-300" />
            <p className="text-[11px] text-foreground-500 text-center">
              拖拽图片到此处
            </p>
            <label className="h-7 px-3 rounded-md bg-primary-500 text-background-50 dark:text-foreground-950 text-[10px] font-medium hover:bg-primary-600 transition-colors duration-150 flex items-center gap-1 cursor-pointer whitespace-nowrap">
              <Upload className="w-3 h-3" />
              选择文件
              <input
                type="file"
                accept="image/png,image/jpeg,image/svg+xml,image/webp"
                className="hidden"
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file) handleFileSelect(file);
                }}
              />
            </label>
            <p className="text-[10px] text-foreground-400">
              PNG / JPG / SVG / WebP（建议 64x64px）
            </p>
            {previewUrl && detectIconType(previewUrl) === 'image' && (
              <img
                src={previewUrl}
                alt="预览"
                className={cn('rounded object-contain', compact ? 'max-h-12 max-w-full' : 'max-h-16 max-w-full')}
              />
            )}
          </div>
        )}
      </div>
    </div>
  );
}
