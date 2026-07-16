// ============================================================
// nav.ax Cookie Notice (self-host template) — /cookies
// ============================================================

import LegalLayout, { Section } from '../LegalLayout';

export default function CookiesPage() {
  return (
    <LegalLayout title="Cookie 说明" updated="2026-07-16">
      <p className="text-sm text-foreground-600 leading-relaxed">
        本站（[网站地址]）在 Cookie 使用上力求克制。我们<strong>不使用</strong>任何第三方广告或跨站追踪 Cookie。
      </p>

      <Section title="一、我们使用的 Cookie">
        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="text-left text-foreground-500 border-b border-background-200/60">
                <th className="py-2 pr-4 font-medium">名称</th>
                <th className="py-2 pr-4 font-medium">用途</th>
                <th className="py-2 font-medium">属性</th>
              </tr>
            </thead>
            <tbody className="text-foreground-600">
              <tr className="border-b border-background-200/40">
                <td className="py-2 pr-4 font-mono text-xs">navax_session</td>
                <td className="py-2 pr-4">保持你的登录状态，是使用账号所必需的。</td>
                <td className="py-2 text-xs">HttpOnly · SameSite=Lax · 仅本站域名 · 会话/有效期内</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p>该 Cookie 属于“严格必要”类别：没有它你将无法登录。它仅在你登录后设置，退出登录后失效。</p>
      </Section>

      <Section title="二、本地存储">
        <p>为记住你的界面偏好（如主题、搜索引擎选择），本站可能在浏览器的本地存储（localStorage）中保存少量非敏感设置。这些数据只保存在你的设备上，不会发送给服务器用于追踪。</p>
      </Section>

      <Section title="三、我们不使用的">
        <ul className="list-disc pl-5 space-y-1">
          <li>第三方广告 / 营销 Cookie；</li>
          <li>跨站追踪或用户画像 Cookie；</li>
          <li>会关联到完整 IP 的分析 Cookie（我们的访问统计不写入追踪 Cookie，且不存储完整 IP）。</li>
        </ul>
      </Section>

      <Section title="四、如何管理">
        <p>你可以通过浏览器设置清除或阻止 Cookie。请注意，若阻止会话 Cookie，你将无法登录使用账号功能。浏览本站的公开导航页面不需要任何 Cookie。</p>
      </Section>

      <Section title="五、联系我们">
        <p>如对 Cookie 使用有任何疑问，请联系 <strong>[运营方名称]</strong>，邮箱 <strong>[联系邮箱]</strong>。</p>
      </Section>
    </LegalLayout>
  );
}
