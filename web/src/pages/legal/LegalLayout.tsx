// ============================================================
// nav.ax Legal page layout — shared shell for privacy / terms / cookies.
// Filled in for the official nav.ax instance (operator: NavAx); self-hosters
// should swap in their own operator name, contact email, and site URL.
// ============================================================

import { Link } from 'react-router-dom';
import PublicShell from '@/components/feature/PublicShell';

export function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="space-y-2">
      <h2 className="text-base font-semibold text-foreground-900">{title}</h2>
      <div className="space-y-2 text-sm text-foreground-600 leading-relaxed">{children}</div>
    </section>
  );
}

export default function LegalLayout({
  title,
  updated,
  children,
}: {
  title: string;
  updated: string;
  children: React.ReactNode;
}) {
  return (
    <PublicShell showSearch={false}>
      <div className="mx-auto max-w-3xl px-6 md:px-8 py-12">
        <Link to="/" className="text-sm text-primary-600 hover:text-primary-700">← 返回首页</Link>
        <h1 className="mt-6 text-2xl font-bold font-heading text-foreground-950">{title}</h1>
        <p className="mt-2 text-xs text-foreground-400">最后更新：{updated}</p>
        <div className="mt-5 rounded-lg bg-amber-50 border border-amber-200/70 p-3 text-xs text-amber-800 leading-relaxed">
          本页适用于官方 nav.ax 实例（运营方 NavAx）。若你基于开源项目 nav.ax 自托管，请将运营方名称、联系邮箱与网站地址替换为你自己的信息，并根据适用法律自行审阅。本页不构成法律意见。
        </div>
        <div className="mt-8 space-y-8">{children}</div>
        <nav className="mt-12 pt-6 border-t border-background-200/60 flex items-center gap-4 text-xs text-foreground-400">
          <Link to="/privacy" className="hover:text-primary-500">隐私政策</Link>
          <Link to="/terms" className="hover:text-primary-500">服务条款</Link>
          <Link to="/cookies" className="hover:text-primary-500">Cookie 说明</Link>
        </nav>
      </div>
    </PublicShell>
  );
}
