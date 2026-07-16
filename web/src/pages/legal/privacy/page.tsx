// ============================================================
// nav.ax Privacy Policy (self-host template) — /privacy
// ============================================================

import LegalLayout, { Section } from '../LegalLayout';

export default function PrivacyPage() {
  return (
    <LegalLayout title="隐私政策" updated="2026-07-16">
      <p className="text-sm text-foreground-600 leading-relaxed">
        本隐私政策说明 [运营方名称]（以下称“我们”）在你使用本站（[网站地址]，基于开源项目 nav.ax 搭建）时如何收集、使用与保护你的信息。
      </p>

      <Section title="一、我们收集的信息">
        <p>为提供导航站服务，我们仅收集必要信息：</p>
        <ul className="list-disc pl-5 space-y-1">
          <li><strong>账号信息：</strong>用户名、邮箱地址，以及经过 Argon2id 单向加密后的密码（我们不存储明文密码）。</li>
          <li><strong>你创建的内容：</strong>导航页面、分类、站点链接及相关设置。</li>
          <li><strong>访问统计：</strong>为帮助你了解页面访问情况，我们记录聚合的访问事件（设备类型、浏览器、来源域名、国家/地区代码）。我们<strong>不存储完整 IP 地址</strong>，访客标识采用每日轮换的 HMAC，无法反查到具体个人或长期追踪。</li>
          <li><strong>第三方凭据：</strong>若管理员配置了 SMTP 等服务，其密钥会经 AES-256-GCM 加密存储，且永不返回给浏览器。</li>
        </ul>
      </Section>

      <Section title="二、我们如何使用信息">
        <ul className="list-disc pl-5 space-y-1">
          <li>提供、维护与改进导航站功能；</li>
          <li>发送与账号相关的事务性邮件（如邀请、密码找回）——仅在运营方配置了邮件服务时；</li>
          <li>以聚合、匿名的方式向页面所有者展示访问统计；</li>
          <li>保障账号与服务安全（如限流、异常登录防护）。</li>
        </ul>
        <p>我们<strong>不</strong>将你的个人信息用于广告投放，也不会向第三方出售你的信息。</p>
      </Section>

      <Section title="三、Cookie 的使用">
        <p>本站仅使用一个必要的会话 Cookie（<code>navax_session</code>）以保持你的登录状态，它是 HttpOnly、SameSite=Lax、仅限本站域名的。我们不使用第三方广告或跨站追踪 Cookie。详见 <a href="/cookies" className="text-primary-600 hover:text-primary-700">Cookie 说明</a>。</p>
      </Section>

      <Section title="四、数据保留">
        <p>账号与你创建的内容在账号存续期间保留。访问统计按管理员设置的保留期（默认较短周期）自动清除。你注销或删除账号后，相关个人数据将在合理期限内删除或匿名化。</p>
      </Section>

      <Section title="五、数据安全">
        <p>我们采取包括加密存储凭据、密码单向哈希、会话令牌仅存哈希、服务端请求防 SSRF、上传类型与大小限制在内的技术措施保护你的数据。但请注意，没有任何系统能保证绝对安全。</p>
      </Section>

      <Section title="六、你的权利">
        <p>在适用法律允许的范围内，你有权访问、更正或删除你的个人信息，或撤回此前的同意。你可在账号设置中管理资料，或通过 <strong>[联系邮箱]</strong> 联系我们行使上述权利。</p>
      </Section>

      <Section title="七、未成年人">
        <p>本站不面向 [最低年龄，如 14] 周岁以下的未成年人。若你认为未成年人向我们提供了信息，请联系我们删除。</p>
      </Section>

      <Section title="八、政策变更">
        <p>我们可能不时更新本政策。重大变更将在本页公布，并更新顶部的“最后更新”日期。</p>
      </Section>

      <Section title="九、联系我们">
        <p>如对本隐私政策有任何疑问，请联系：<strong>[运营方名称]</strong>，邮箱 <strong>[联系邮箱]</strong>。</p>
      </Section>
    </LegalLayout>
  );
}
