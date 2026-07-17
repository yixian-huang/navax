import { useDeferredValue, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import PublicShell from '@/components/feature/PublicShell';
import { useDiscoverPages } from '@/hooks/useQueries';
import type { DiscoveredPage } from '@/api/types';

type SortMode = 'popular' | 'recent';

function formatCount(n: number): string {
  if (n >= 10000) return `${(n / 10000).toFixed(1)}万`;
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
  return String(n);
}

function DiscoverCard({ page }: { page: DiscoveredPage }) {
  const cover = (page.coverImage || '').trim();
  return (
    <Link
      to={`/u/${page.slug}`}
      className="group material-card overflow-hidden rounded-xl flex flex-col transition-all duration-300 hover:-translate-y-1 cursor-pointer"
    >
      <div className="relative w-full h-[180px] overflow-hidden bg-gradient-to-br from-primary-100 via-background-100 to-secondary-100">
        {cover ? (
          <>
            <img
              src={cover}
              alt=""
              loading="lazy"
              referrerPolicy="no-referrer"
              className="absolute inset-0 w-full h-full object-cover transition-transform duration-500 group-hover:scale-105"
            />
            <div className="absolute inset-0 bg-gradient-to-t from-black/55 via-black/15 to-black/5" />
            <div className="absolute inset-x-0 bottom-0 p-4">
              <span className="font-heading text-base font-semibold text-white line-clamp-1 drop-shadow-sm">
                {page.title}
              </span>
              <span className="mt-0.5 block text-[10px] uppercase tracking-widest text-white/75">
                {page.themeId}
              </span>
            </div>
          </>
        ) : (
          <div className="absolute inset-0 flex flex-col items-center justify-center px-6 text-center transition-transform duration-500 group-hover:scale-105">
            <div className="w-12 h-12 rounded-2xl bg-background-50/80 shadow-sm flex items-center justify-center mb-3 text-primary-500">
              <i className="ri-compass-3-line text-2xl" />
            </div>
            <span className="font-heading text-base font-semibold text-foreground-800 line-clamp-1">{page.title}</span>
            <span className="mt-1 text-[10px] uppercase tracking-widest text-foreground-400">{page.themeId}</span>
          </div>
        )}
        {page.featured ? (
          <div className="absolute top-3 right-3 px-2 py-1 rounded-full bg-background-50/90 text-[10px] font-medium text-accent-600">
            <i className="ri-star-fill mr-1" />精选
          </div>
        ) : null}
      </div>

      <div className="flex flex-col flex-1 p-4">
        <div className="flex items-center gap-1.5 mb-2.5 flex-wrap">
          {page.tags.slice(0, 2).map(tag => (
            <span key={tag} className="px-2 py-0.5 text-[10px] font-medium rounded-full bg-secondary-100 text-secondary-700 whitespace-nowrap">
              {tag}
            </span>
          ))}
          {page.tags.length > 2 ? <span className="text-[10px] text-foreground-300">+{page.tags.length - 2}</span> : null}
        </div>
        <h3 className="font-heading text-[15px] font-semibold text-foreground-900 leading-snug mb-1.5 line-clamp-1 group-hover:text-primary-500 transition-colors duration-200">
          {page.title}
        </h3>
        <p className="text-[12px] text-foreground-400 leading-relaxed line-clamp-2 mb-3 flex-1">{page.description}</p>
        <div className="flex items-center justify-between pt-3 border-t border-background-200/50">
          <div className="flex items-center gap-2 min-w-0">
            <div className="w-6 h-6 rounded-full flex-shrink-0 bg-primary-100 text-primary-600 flex items-center justify-center text-[10px] font-medium">
              {page.ownerName.slice(0, 1).toUpperCase()}
            </div>
            <span className="text-[11px] text-foreground-500 truncate">{page.ownerName}</span>
          </div>
          <span className="flex items-center gap-1 text-[10px] text-foreground-300">
            <i className="ri-eye-line text-[11px]" />
            {formatCount(page.viewCount)}
          </span>
        </div>
      </div>
    </Link>
  );
}

export default function DiscoverPage() {
  const [activeTag, setActiveTag] = useState('全部');
  const [sortMode, setSortMode] = useState<SortMode>('popular');
  const [searchQuery, setSearchQuery] = useState('');
  const deferredSearch = useDeferredValue(searchQuery.trim());
  const tagCatalogQuery = useDiscoverPages({ sort: 'featured', page: 1, pageSize: 100 });
  const pagesQuery = useDiscoverPages({
    search: deferredSearch || undefined,
    tag: activeTag === '全部' ? undefined : activeTag,
    sort: sortMode === 'recent' ? 'latest' : 'popular',
    page: 1,
    pageSize: 60,
  });
  const pages = pagesQuery.data?.items ?? [];

  const allTags = useMemo(() => {
    const tags = new Set<string>();
    tagCatalogQuery.data?.items.forEach(page => page.tags.forEach(tag => tags.add(tag)));
    if (activeTag !== '全部') tags.add(activeTag);
    return ['全部', ...Array.from(tags).sort((a, b) => a.localeCompare(b, 'zh-CN'))];
  }, [activeTag, tagCatalogQuery.data?.items]);

  return (
    <PublicShell showSearch={false}>
      <div className="mx-auto max-w-6xl px-6 md:px-8 pt-14 md:pt-20 pb-28">
        <div className="text-center mb-10 md:mb-14 rise-in">
          <h1 className="font-heading text-3xl md:text-4xl tracking-tight text-foreground-950 mb-4">发现精选导航</h1>
          <p className="text-sm md:text-[15px] text-foreground-400 max-w-lg mx-auto leading-relaxed">
            来自独立开发者、设计师、产品经理等创作者精心整理的导航合集，发现灵感，一键复用。
          </p>
        </div>

        <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-3 mb-8 rise-in" style={{ animationDelay: '40ms' }}>
          <div className="relative flex-1 max-w-md">
            <i className="ri-search-line absolute left-3.5 top-1/2 -translate-y-1/2 text-sm text-foreground-300" />
            <input
              type="text"
              value={searchQuery}
              onChange={event => setSearchQuery(event.target.value)}
              placeholder="搜索导航页..."
              className="w-full h-10 pl-9 pr-4 rounded-lg bg-background-50 border border-background-200/70 text-[13px] text-foreground-800 placeholder:text-foreground-300 focus:outline-none focus:border-primary-300 focus:ring-2 focus:ring-primary-100/50 transition-all duration-200"
            />
          </div>
          <div className="flex items-center gap-1.5 bg-background-50 rounded-lg border border-background-200/70 p-1 self-start">
            {([
              { value: 'popular' as const, label: '最受欢迎', icon: 'ri-fire-line' },
              { value: 'recent' as const, label: '最新发布', icon: 'ri-time-line' },
            ]).map(option => (
              <button
                key={option.value}
                onClick={() => setSortMode(option.value)}
                className={`px-3 py-1.5 rounded-md text-[12px] font-medium transition-all duration-200 whitespace-nowrap cursor-pointer ${
                  sortMode === option.value ? 'bg-primary-500 text-background-50 shadow-sm' : 'text-foreground-400 hover:text-foreground-600'
                }`}
              >
                <i className={`${option.icon} text-[11px] mr-1`} />{option.label}
              </button>
            ))}
          </div>
        </div>

        <div className="flex items-center gap-1.5 overflow-x-auto pb-3 mb-8 scrollbar-none rise-in" style={{ animationDelay: '60ms' }}>
          {allTags.map(tag => (
            <button
              key={tag}
              onClick={() => setActiveTag(tag)}
              className={`flex items-center gap-1 flex-shrink-0 px-3 py-1.5 rounded-full text-[12px] font-medium transition-all duration-200 whitespace-nowrap cursor-pointer ${
                activeTag === tag
                  ? 'bg-primary-500 text-background-50 shadow-sm'
                  : 'bg-background-50 border border-background-200/70 text-foreground-500 hover:text-foreground-700 hover:border-background-300/60'
              }`}
            >
              {tag === '全部' ? <i className="ri-apps-line text-[11px]" /> : <i className="ri-price-tag-3-line text-[11px]" />}
              {tag}
            </button>
          ))}
        </div>

        <div className="flex items-center gap-2 mb-6 rise-in" style={{ animationDelay: '80ms' }}>
          <span className="text-[11px] text-foreground-300 tracking-wide">
            {pagesQuery.data?.total ?? 0} 个导航页
            {deferredSearch ? <span className="ml-1">· &ldquo;{deferredSearch}&rdquo;</span> : null}
            {activeTag !== '全部' ? <span className="ml-1">· {activeTag}</span> : null}
          </span>
        </div>

        {pagesQuery.isLoading ? (
          <div className="py-20 flex items-center justify-center gap-2 text-sm text-foreground-400">
            <i className="ri-loader-4-line animate-spin" />正在加载发现页
          </div>
        ) : pagesQuery.isError ? (
          <div className="text-center py-20 rise-in">
            <i className="ri-error-warning-line text-2xl text-red-400" />
            <h3 className="font-heading text-lg text-foreground-500 mt-3 mb-1">发现页加载失败</h3>
            <button onClick={() => pagesQuery.refetch()} className="text-[13px] text-primary-500 hover:text-primary-600">重新加载</button>
          </div>
        ) : pages.length === 0 ? (
          <div className="text-center py-20 rise-in">
            <div className="w-14 h-14 mx-auto mb-4 flex items-center justify-center rounded-2xl bg-background-100">
              <i className="ri-search-line text-2xl text-foreground-300" />
            </div>
            <h3 className="font-heading text-lg text-foreground-500 mb-1">没有找到匹配的导航页</h3>
            <p className="text-[13px] text-foreground-300">试试换个关键词或标签筛选</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
            {pages.map((page, index) => (
              <div key={page.slug} className="rise-in" style={{ animationDelay: `${100 + index * 50}ms` }}>
                <DiscoverCard page={page} />
              </div>
            ))}
          </div>
        )}

        <div className="mt-20 text-center rise-in" style={{ animationDelay: '200ms' }}>
          <div className="hairline-gradient mb-8" />
          <p className="text-[13px] text-foreground-400 mb-4">也想分享你的导航页？发布后即可出现在发现广场。</p>
          <Link to="/app/publish" className="inline-flex items-center gap-2 h-10 px-5 rounded-lg bg-primary-500 text-background-50 text-[13px] font-medium hover:bg-primary-600 transition-colors duration-150 whitespace-nowrap cursor-pointer">
            <i className="ri-rocket-line text-sm" />立即发布我的导航页
          </Link>
        </div>
      </div>
    </PublicShell>
  );
}
