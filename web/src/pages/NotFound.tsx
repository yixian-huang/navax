import { Link } from 'react-router-dom';
import { Home } from 'lucide-react';

export default function NotFound() {
  return (
    <div className="min-h-screen bg-background-50 flex items-center justify-center px-4">
      <div className="text-center">
        <h1 className="text-8xl md:text-9xl font-black text-background-200 select-none">404</h1>
        <h2 className="mt-4 text-xl font-semibold text-foreground-800">页面未找到</h2>
        <p className="mt-2 text-sm text-foreground-400 max-w-sm mx-auto">
          你访问的页面不存在或已被移除。检查一下地址，或者返回 nav.ax 首页。
        </p>
        <Link
          to="/"
          className="mt-6 inline-flex items-center gap-2 h-10 px-5 rounded-lg bg-primary-500 text-background-50 dark:text-foreground-950 text-sm font-medium hover:bg-primary-600 transition-colors duration-150 whitespace-nowrap"
        >
          <Home className="w-4 h-4" />
          返回 nav.ax 首页
        </Link>
      </div>
    </div>
  );
}
