import { Edit2, Trash2, ChevronRight, Eye, EyeOff } from 'lucide-react';
import { Badge } from '@/components/base/SharedUI';
import IconRenderer from '@/components/base/IconRenderer';
import {
  useContextMenu,
  createCategoryContextActions,
  createSiteContextActions,
} from '@/components/base/ContextMenu';
import { cn } from '@/lib/utils';
import type { Category, Site } from '@/api/types';

export interface GroupedCategoryListProps {
  categories: Category[];
  expandedCat: string | null;
  selectedSiteIds: Set<string>;
  overCategoryId: string | null;
  onToggleCategory: (id: string) => void;
  onToggleSelectAllInCategory: (id: string) => void;
  onToggleSelect: (id: string) => void;
  onToggleCategoryEnabled: (cat: Category) => void;
  onEditCategory: (cat: Category) => void;
  onDeleteCategory: (cat: Category) => void;
  onAddSite: (categoryId: string) => void;
  onToggleSiteEnabled: (site: Site) => void;
  onEditSite: (site: Site) => void;
  onDeleteSite: (site: Site) => void;
}

export default function GroupedCategoryList({
  categories,
  expandedCat,
  selectedSiteIds,
  overCategoryId,
  onToggleCategory,
  onToggleSelectAllInCategory,
  onToggleSelect,
  onToggleCategoryEnabled,
  onEditCategory,
  onDeleteCategory,
  onAddSite,
  onToggleSiteEnabled,
  onEditSite,
  onDeleteSite,
}: GroupedCategoryListProps) {
  const { handleContextMenu, portal } = useContextMenu();

  return (
    <>
      {categories.map(cat => (
        <div
          key={cat.id}
          className={cn(
            'border-b border-background-100 last:border-b-0',
            overCategoryId === cat.id && 'ring-2 ring-primary-300/80 bg-primary-50/20',
          )}
        >
          <button
            onClick={() => onToggleCategory(cat.id)}
            onContextMenu={e =>
              handleContextMenu(
                e,
                createCategoryContextActions(cat, {
                  onEdit: () => onEditCategory(cat),
                  onAddSite: () => onAddSite(cat.id),
                  onToggleEnabled: () => onToggleCategoryEnabled(cat),
                  onDelete: () => onDeleteCategory(cat),
                }),
              )
            }
            className="w-full flex items-center gap-2 px-4 py-2.5 hover:bg-background-50 transition-colors duration-150 text-left"
          >
            {cat.sites.length > 0 && (() => {
              const allInCatSelected = cat.sites.every(s => selectedSiteIds.has(s.id));
              const someInCatSelected = cat.sites.some(s => selectedSiteIds.has(s.id));
              return (
                <button
                  data-no-dnd
                  onClick={e => { e.stopPropagation(); onToggleSelectAllInCategory(cat.id); }}
                  className={cn(
                    'w-4 h-4 rounded border flex items-center justify-center flex-shrink-0 transition-all duration-150',
                    allInCatSelected
                      ? 'bg-primary-500 border-primary-500'
                      : someInCatSelected
                        ? 'border-primary-400 bg-primary-50'
                        : 'border-background-300 hover:border-primary-400',
                  )}
                >
                  {allInCatSelected ? (
                    <i className="ri-check-line text-[10px] text-background-50" />
                  ) : someInCatSelected ? (
                    <div className="w-2 h-0.5 bg-primary-400 rounded-full" />
                  ) : null}
                </button>
              );
            })()}
            <div className="w-6 h-6 rounded-md bg-background-100 flex items-center justify-center flex-shrink-0">
              <IconRenderer icon={cat.icon} className="text-xs text-primary-500" />
            </div>
            <span className={cn(
              'flex-1 text-xs font-medium truncate',
              cat.enabled === false ? 'text-foreground-400' : 'text-foreground-900',
            )}>
              {cat.name}
            </span>
            {cat.enabled === false && (
              <EyeOff className="w-3.5 h-3.5 text-foreground-400 flex-shrink-0" aria-label="分类已隐藏" />
            )}
            <Badge>{cat.sites.length}</Badge>
            <ChevronRight
              className={cn(
                'w-3.5 h-3.5 text-foreground-300 transition-transform duration-150',
                expandedCat === cat.id && 'rotate-90',
              )}
            />
            <button
              data-no-dnd
              onClick={e => {
                e.stopPropagation();
                onToggleCategoryEnabled(cat);
              }}
              className="w-6 h-6 flex items-center justify-center rounded text-foreground-300 hover:text-primary-500 hover:bg-primary-50 transition-colors duration-150"
              aria-label={cat.enabled === false ? `显示分类 ${cat.name}` : `隐藏分类 ${cat.name}`}
              title={cat.enabled === false ? '显示分类（需发布后生效）' : '隐藏分类（需发布后生效）'}
            >
              {cat.enabled === false ? <EyeOff className="w-3 h-3" /> : <Eye className="w-3 h-3" />}
            </button>
            <button
              data-no-dnd
              onClick={e => {
                e.stopPropagation();
                onEditCategory(cat);
              }}
              className="w-6 h-6 flex items-center justify-center rounded text-foreground-300 hover:text-primary-500 hover:bg-primary-50 transition-colors duration-150"
              aria-label={`编辑 ${cat.name}`}
            >
              <Edit2 className="w-3 h-3" />
            </button>
            <button
              data-no-dnd
              onClick={e => {
                e.stopPropagation();
                onDeleteCategory(cat);
              }}
              className="w-6 h-6 flex items-center justify-center rounded text-foreground-300 hover:text-red-500 hover:bg-red-50 transition-colors duration-150"
              aria-label={`删除 ${cat.name}`}
            >
              <Trash2 className="w-3 h-3" />
            </button>
          </button>

          {expandedCat === cat.id && (
            <div className="bg-background-50/50">
              {cat.sites.length === 0 ? (
                <div className="px-4 py-3 text-center">
                  <p className="text-[11px] text-foreground-400">暂无站点</p>
                </div>
              ) : (
                cat.sites.map(site => {
                  const isSiteSelected = selectedSiteIds.has(site.id);
                  return (
                    <div
                      key={site.id}
                      className={cn(
                        'flex items-center gap-2 px-4 py-2 hover:bg-background-100/50 transition-colors duration-150 group',
                        isSiteSelected && 'bg-primary-50/40',
                      )}
                      onContextMenu={e =>
                        handleContextMenu(
                          e,
                          createSiteContextActions(site, {
                            onEdit: () => onEditSite(site),
                            onToggleEnabled: () => onToggleSiteEnabled(site),
                            onDelete: () => onDeleteSite(site),
                          }),
                        )
                      }
                    >
                      <button
                        data-no-dnd
                        onClick={() => onToggleSelect(site.id)}
                        className={cn(
                          'w-4 h-4 rounded border flex items-center justify-center flex-shrink-0 transition-all duration-150',
                          isSiteSelected
                            ? 'bg-primary-500 border-primary-500'
                            : 'border-background-300 hover:border-primary-400',
                        )}
                      >
                        {isSiteSelected && <i className="ri-check-line text-[10px] text-background-50" />}
                      </button>
                      <div className={cn(
                        'w-6 h-6 rounded flex items-center justify-center flex-shrink-0',
                        site.enabled === false ? 'bg-background-100 opacity-70' : 'bg-background-100',
                      )}>
                        <IconRenderer icon={site.icon} url={site.url} size={14} alt={site.title} />
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className={cn(
                          'text-xs font-medium break-words leading-snug',
                          site.enabled === false ? 'text-foreground-500' : 'text-foreground-800',
                        )}>
                          {site.title}
                        </div>
                        {site.description ? (
                          <div className="text-[10px] text-foreground-400 line-clamp-1 break-words">
                            {site.description}
                          </div>
                        ) : null}
                        <a
                          data-no-dnd
                          href={site.url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-[10px] text-primary-600 hover:underline font-mono truncate block"
                          title={site.url}
                          onClick={e => e.stopPropagation()}
                        >
                          {site.url.replace(/^https?:\/\//, '').replace(/^www\./, '')}
                        </a>
                      </div>
                      {site.enabled === false && (
                        <span
                          className="flex-shrink-0 text-foreground-400"
                          title="隐藏：发布后访客不可见"
                          aria-label="已隐藏"
                        >
                          <EyeOff className="w-3.5 h-3.5" />
                        </span>
                      )}
                      <button
                        data-no-dnd
                        onClick={() => onToggleSiteEnabled(site)}
                        className="w-6 h-6 flex items-center justify-center rounded opacity-0 group-hover:opacity-100 transition-all duration-150 flex-shrink-0 text-foreground-300 hover:text-foreground-600 hover:bg-background-100"
                        aria-label={site.enabled === false ? `上架 ${site.title}` : `隐藏 ${site.title}`}
                        title={site.enabled === false ? '上架（需发布后生效）' : '隐藏（需发布后生效）'}
                      >
                        {site.enabled === false ? <Eye className="w-3 h-3" /> : <EyeOff className="w-3 h-3" />}
                      </button>
                      <button
                        data-no-dnd
                        onClick={() => onEditSite(site)}
                        className="w-6 h-6 flex items-center justify-center rounded text-foreground-300 opacity-0 group-hover:opacity-100 hover:text-primary-500 hover:bg-primary-50 transition-all duration-150 flex-shrink-0"
                        aria-label={`编辑 ${site.title}`}
                      >
                        <Edit2 className="w-3 h-3" />
                      </button>
                      <button
                        data-no-dnd
                        onClick={() => onDeleteSite(site)}
                        className="w-6 h-6 flex items-center justify-center rounded text-foreground-300 opacity-0 group-hover:opacity-100 hover:text-red-500 hover:bg-red-50 transition-all duration-150 flex-shrink-0"
                        aria-label={`删除 ${site.title}`}
                      >
                        <Trash2 className="w-3 h-3" />
                      </button>
                    </div>
                  );
                })
              )}
              <button
                onClick={() => onAddSite(cat.id)}
                className="w-full px-4 py-2 text-[10px] text-primary-600 hover:text-primary-700 font-medium hover:bg-primary-50/30 transition-colors duration-150 text-left"
              >
                + 添加站点到此分类
              </button>
            </div>
          )}
        </div>
      ))}
      {portal}
    </>
  );
}
