// ============================================================
// nav.ax — Search Intelligence (无感 AI 层)
// 1. 语义搜索增强：让搜索"懂"口语意图，而非死板字面匹配
// 2. 动态智能建议：基于收藏 + 时间场景生成搜索框引导
// 两者都不改变任何 UI 交互，只让搜索体验更聪明。
// ============================================================

import type { Site } from '@/api/types';

// ---- 语义概念词典 ----
// triggers: 用户可能输入的口语/意图词
// expand:   用于匹配站点 title / description / url 的关键词
interface Concept {
  triggers: string[];
  expand: string[];
}

const concepts: Concept[] = [
  {
    triggers: ['画原型', '原型', '设计稿', '做设计', 'ui设计', '界面设计', '画图', '设计工具', '画界面'],
    expand: ['figma', 'sketch', '设计', 'design', 'dribbble', 'behance', '原型', 'ui', 'excalidraw', 'lottie'],
  },
  {
    triggers: ['写代码', '编程', '敲代码', '代码', '开发', 'coding', '程序', '写程序', '码代码'],
    expand: ['github', 'code', 'vscode', 'codepen', 'stack', '代码', '开发', 'npm', 'mdn', 'tailwind'],
  },
  {
    triggers: ['存图', '图片', '找图', '照片', '壁纸', '图库', '素材', '配图', '找素材'],
    expand: ['unsplash', 'pinterest', '图片', 'image', 'photo', '素材', '壁纸', 'lottie'],
  },
  {
    triggers: ['配色', '颜色', '色彩', '调色', '色卡', '取色'],
    expand: ['color', '配色', 'colorhunt', 'coolors', 'palette', '颜色', '色'],
  },
  {
    triggers: ['记笔记', '笔记', '写文档', '文档', '记录', '写东西', '写作'],
    expand: ['notion', '笔记', 'note', '文档', 'doc', 'medium', '写作'],
  },
  {
    triggers: ['管理任务', '待办', '任务', 'todo', '项目管理', '排期', '看板'],
    expand: ['todoist', 'linear', 'jira', 'trello', 'miro', '任务', '项目', '待办', 'notion', 'slack'],
  },
  {
    triggers: ['学习', '教程', '课程', '学编程', '刷题', '自学', '算法'],
    expand: ['leetcode', 'coursera', 'freecodecamp', 'mdn', '学习', '教程', '课程', 'w3schools', 'khan', '算法'],
  },
  {
    triggers: ['聊天', '社交', '社区', '论坛', '交流', '灌水'],
    expand: ['reddit', 'discord', 'twitter', 'x', '社交', '社区', '论坛', 'slack'],
  },
  {
    triggers: ['翻译', 'translate', '翻译工具', '外语'],
    expand: ['deepl', 'translate', '翻译'],
  },
  {
    triggers: ['ai', '人工智能', '智能助手', '问ai', '大模型', '聊天机器人'],
    expand: ['chatgpt', 'ai', 'gpt', '智能', 'openai'],
  },
  {
    triggers: ['看视频', '视频', '影片', '追剧', '短视频'],
    expand: ['youtube', '视频', 'video'],
  },
  {
    triggers: ['新闻', '资讯', '阅读', '看新闻', '订阅', '博客'],
    expand: ['news', 'hacker', 'medium', 'dev', '资讯', '新闻', '阅读', 'rss', 'feedly'],
  },
  {
    triggers: ['部署', '上线', '托管', '发布网站', '运维'],
    expand: ['vercel', '部署', 'deploy', 'docker', 'kubernetes', 'terraform', 'grafana'],
  },
  {
    triggers: ['搜索', '搜索引擎', '查东西', '查资料'],
    expand: ['google', 'bing', 'duckduckgo', '百度', 'wikipedia', '搜索'],
  },
];

/**
 * 将用户输入扩展为一组匹配关键词。
 * 若输入命中某个语义概念，返回 [原词, ...扩展词]；否则仅返回 [原词]。
 * 保证普通字面搜索行为完全不变。
 */
export function expandQuery(rawQuery: string): string[] {
  const q = rawQuery.toLowerCase().trim();
  if (!q) return [];
  const terms = new Set<string>([q]);

  concepts.forEach(concept => {
    const hit = concept.triggers.some(t => {
      const tl = t.toLowerCase();
      return tl === q || tl.includes(q) || q.includes(tl);
    });
    if (hit) {
      concept.expand.forEach(e => terms.add(e.toLowerCase()));
    }
  });

  return Array.from(terms);
}

/**
 * 语义过滤站点。先做字面 + 语义扩展匹配。
 * 命中任一扩展词（在 title / description / url 中）即保留。
 */
export function semanticFilterSites(sites: Site[], query: string): Site[] {
  const q = query.toLowerCase().trim();
  if (!q) return sites;
  const expanded = expandQuery(q);

  return sites.filter(site => {
    const haystack = `${site.title} ${site.description || ''} ${site.url}`.toLowerCase();
    return expanded.some(term => haystack.includes(term));
  });
}

// ---- 动态智能搜索建议 ----

function shuffle<T>(arr: T[]): T[] {
  const a = [...arr];
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

function sceneLine(hour: number): string {
  if (hour < 6) return '夜深了，早点休息 —— 或搜点什么';
  if (hour < 11) return '新的一天，想先打开什么？';
  if (hour < 14) return '午休时间，随便逛逛';
  if (hour < 18) return '效率低谷？搜点灵感提提神';
  return '忙完了吗？找点放松的';
}

function greetingWord(hour: number): string {
  if (hour < 6) return '夜深了';
  if (hour < 11) return '早上好';
  if (hour < 14) return '中午好';
  if (hour < 18) return '下午好';
  return '晚上好';
}

/**
 * 生成一组动态搜索建议，混合三类内容：
 * 1. 场景问候 + 用户真实收藏站点名（个性化）
 * 2. 语义搜索能力引导（教育用户"搜口语也能找到"）
 * 3. 时段场景文案（呼吸感）
 */
export function buildSearchSuggestions(siteTitles: string[], hour: number): string[] {
  const greeting = greetingWord(hour);
  const picks = shuffle(siteTitles.filter(Boolean)).slice(0, 4);
  const out: string[] = [];

  if (picks[0]) out.push(`${greeting}，从「${picks[0]}」开始`);
  out.push('试试搜「画原型的」找到设计工具');
  if (picks[1]) out.push(`搜索「${picks[1]}」或输入任意网址`);
  out.push('试试搜「写代码的」找到开发工具');
  if (picks[2]) out.push(`快速直达「${picks[2]}」`);
  out.push('输入「存图片」发现图库资源');
  out.push(sceneLine(hour));
  if (picks[3]) out.push(`「${picks[3]}」就在指尖`);

  return out.filter(Boolean);
}
