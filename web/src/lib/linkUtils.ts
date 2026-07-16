// ============================================================
// nav.ax Link Utilities — link auto-recognition
// ============================================================

export interface RecognizedLinkInfo {
  title: string;
  icon: string;
  description: string;
  faviconUrl: string;
}

// Common site mappings for title/description generation
const SITE_PATTERNS: Record<string, { title: string; description: string; icon: string }> = {
  'github.com': { title: 'GitHub', description: '全球最大的代码托管平台，开发者协作社区', icon: 'ri-github-fill' },
  'gitlab.com': { title: 'GitLab', description: '完整的 DevOps 平台', icon: 'ri-gitlab-fill' },
  'stackoverflow.com': { title: 'Stack Overflow', description: '全球最大的程序员问答社区', icon: 'ri-stack-overflow-fill' },
  'figma.com': { title: 'Figma', description: '基于浏览器的协作设计工具', icon: 'ri-pen-nib-fill' },
  'dribbble.com': { title: 'Dribbble', description: '全球设计师作品展示与发现平台', icon: 'ri-dribbble-fill' },
  'behance.net': { title: 'Behance', description: 'Adobe 旗下创意作品展示平台', icon: 'ri-behance-line' },
  'notion.so': { title: 'Notion', description: '全能笔记、文档与项目管理工具', icon: 'ri-notion-fill' },
  'linear.app': { title: 'Linear', description: '为现代软件团队打造的极速项目管理', icon: 'ri-layout-4-fill' },
  'vercel.com': { title: 'Vercel', description: '前端部署与边缘计算平台', icon: 'ri-vercel-fill' },
  'netlify.com': { title: 'Netlify', description: '现代 Web 项目一键部署平台', icon: 'ri-cloud-fill' },
  'google.com': { title: 'Google', description: '全球最大的搜索引擎', icon: 'ri-google-fill' },
  'youtube.com': { title: 'YouTube', description: '全球最大的视频分享平台', icon: 'ri-youtube-fill' },
  'x.com': { title: 'X (Twitter)', description: '实时社交网络与资讯平台', icon: 'ri-twitter-x-fill' },
  'twitter.com': { title: 'Twitter', description: '实时社交网络与资讯平台', icon: 'ri-twitter-x-fill' },
  'reddit.com': { title: 'Reddit', description: '全球最大的社区内容聚合平台', icon: 'ri-reddit-fill' },
  'discord.com': { title: 'Discord', description: '即时通讯与社区交流平台', icon: 'ri-discord-fill' },
  'slack.com': { title: 'Slack', description: '团队沟通与协作平台', icon: 'ri-slack-fill' },
  'medium.com': { title: 'Medium', description: '深度阅读与写作平台', icon: 'ri-medium-fill' },
  'dev.to': { title: 'DEV Community', description: '开发者写作与交流社区', icon: 'ri-file-code-fill' },
  'npmjs.com': { title: 'npm', description: '世界上最大的 JavaScript 包注册中心', icon: 'ri-npmjs-fill' },
  'docker.com': { title: 'Docker', description: '领先的容器化应用平台', icon: 'ri-server-fill' },
  'kubernetes.io': { title: 'Kubernetes', description: '开源容器编排平台', icon: 'ri-cloud-fill' },
  'leetcode.com': { title: 'LeetCode', description: '全球最大的算法练习与面试准备平台', icon: 'ri-brain-fill' },
  'freecodecamp.org': { title: 'freeCodeCamp', description: '免费学习编程的开放社区', icon: 'ri-code-box-fill' },
  'coursera.org': { title: 'Coursera', description: '全球顶尖大学在线课程平台', icon: 'ri-book-2-line' },
  'udemy.com': { title: 'Udemy', description: '海量在线技能学习课程', icon: 'ri-play-circle-line' },
  'wikipedia.org': { title: 'Wikipedia', description: '自由开放的多语言百科全书', icon: 'ri-earth-line' },
  'unsplash.com': { title: 'Unsplash', description: '免费高质量摄影图片资源库', icon: 'ri-image-line' },
  'codepen.io': { title: 'CodePen', description: '在线前端代码编辑与分享社区', icon: 'ri-codepen-fill' },
  'codesandbox.io': { title: 'CodeSandbox', description: '在线 Web 开发沙盒环境', icon: 'ri-code-box-line' },
  'pinterest.com': { title: 'Pinterest', description: '全球灵感图片发现与收藏平台', icon: 'ri-pinterest-fill' },
  'deepl.com': { title: 'DeepL', description: '全球最精准的 AI 翻译工具', icon: 'ri-translate' },
  'producthunt.com': { title: 'Product Hunt', description: '每日新产品、新工具发现平台', icon: 'ri-rocket-line' },
  'miro.com': { title: 'Miro', description: '在线协作白板与头脑风暴工具', icon: 'ri-grid-fill' },
  'raycast.com': { title: 'Raycast', description: 'macOS 效率启动器与工作流自动化', icon: 'ri-flashlight-fill' },
  'tailwindcss.com': { title: 'Tailwind CSS', description: '实用优先的 CSS 框架', icon: 'ri-tailwind-css-line' },
  'excalidraw.com': { title: 'Excalidraw', description: '手绘风格的协作白板工具', icon: 'ri-pencil-ruler-line' },
  'khanacademy.org': { title: 'Khan Academy', description: '可汗学院 — 面向所有人的免费教育', icon: 'ri-lightbulb-line' },
  'w3schools.com': { title: 'W3Schools', description: 'Web 技术在线教程与参考', icon: 'ri-code-box-line' },
  'atlassian.com': { title: 'Atlassian', description: '团队协作与软件开发工具套件', icon: 'ri-trello-fill' },
  'grafana.com': { title: 'Grafana', description: '开源监控与可观测性平台', icon: 'ri-line-chart-fill' },
  'terraform.io': { title: 'Terraform', description: '基础设施即代码 (IaC) 工具', icon: 'ri-code-box-line' },
  'hub.docker.com': { title: 'Docker Hub', description: '容器镜像注册与分发中心', icon: 'ri-server-fill' },
};

/**
 * Extract domain from URL
 */
export function extractDomain(url: string): string {
  try {
    const u = url.startsWith('http') ? new URL(url) : new URL(`https://${url}`);
    return u.hostname.replace(/^www\./, '');
  } catch {
    return url.replace(/^https?:\/\//, '').replace(/^www\./, '').split('/')[0];
  }
}

/**
 * Auto-recognize link info from a URL
 * Returns title, icon, description, and favicon URL
 */
export function recognizeLink(url: string): RecognizedLinkInfo | null {
  if (!url.trim()) return null;

  const domain = extractDomain(url);
  if (!domain) return null;

  // Check known sites
  const matched = SITE_PATTERNS[domain];
  if (matched) {
    return {
      title: matched.title,
      icon: matched.icon,
      description: matched.description,
      faviconUrl: `https://www.google.com/s2/favicons?domain=${domain}&sz=64`,
    };
  }

  // For partial domain matches (subdomains like app.xxx.com)
  for (const [pattern, info] of Object.entries(SITE_PATTERNS)) {
    if (domain.endsWith(`.${pattern}`) || domain === pattern) {
      return {
        title: info.title,
        icon: info.icon,
        description: info.description,
        faviconUrl: `https://www.google.com/s2/favicons?domain=${domain}&sz=64`,
      };
    }
  }

  // Generate from domain
  const name = domain.split('.')[0];
  const capitalized = name.charAt(0).toUpperCase() + name.slice(1);

  return {
    title: capitalized,
    icon: 'ri-link',
    description: `${capitalized} 官方网站`,
    faviconUrl: `https://www.google.com/s2/favicons?domain=${domain}&sz=64`,
  };
}
