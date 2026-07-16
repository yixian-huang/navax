export type HomeLayout = 'full' | 'search-focus' | 'browse-first' | 'sidebar';

export const HOME_LAYOUTS: HomeLayout[] = ['full', 'search-focus', 'browse-first', 'sidebar'];

export const HOME_LAYOUT_META: Record<HomeLayout, { name: string; description: string; icon: string }> = {
  full: {
    name: '全功能',
    description: '问候语 + 时钟 + 搜索 + 分类标签 + 站点网格，信息完整，适合日常使用',
    icon: 'ri-layout-5-line',
  },
  'search-focus': {
    name: '搜索聚焦',
    description: '搜索框居中放大，去掉问候区和时钟，主打快速搜索入口',
    icon: 'ri-search-eye-line',
  },
  'browse-first': {
    name: '浏览优先',
    description: '分类标签和站点卡片优先展示在上方，搜索收起到顶部，适合收藏夹型用户',
    icon: 'ri-apps-2-line',
  },
  sidebar: {
    name: '侧边导航',
    description: '左侧固定分类侧边栏，右侧站点网格，适合站点数量多的重度用户',
    icon: 'ri-layout-left-2-line',
  },
};
