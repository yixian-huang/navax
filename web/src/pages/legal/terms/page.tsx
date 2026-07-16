// ============================================================
// nav.ax Terms of Service (self-host template) — /terms
// ============================================================

import LegalLayout, { Section } from '../LegalLayout';

export default function TermsPage() {
  return (
    <LegalLayout title="服务条款" updated="2026-07-16">
      <p className="text-sm text-foreground-600 leading-relaxed">
        欢迎使用本站（[网站地址]，由 [运营方名称] 运营，基于开源项目 nav.ax 搭建）。使用本站即表示你已阅读、理解并同意本服务条款。
      </p>

      <Section title="一、账号与资格">
        <ul className="list-disc pl-5 space-y-1">
          <li>你需通过邀请注册账号，并对账号下的所有活动负责；</li>
          <li>你应妥善保管登录凭据，如发现未经授权的使用应立即通知我们；</li>
          <li>你确认已达到所在地区可独立订立合同的法定年龄。</li>
        </ul>
      </Section>

      <Section title="二、可接受使用">
        <p>你同意不利用本站从事以下行为：</p>
        <ul className="list-disc pl-5 space-y-1">
          <li>发布或链接违法、侵权、恶意软件、欺诈或违反公序良俗的内容；</li>
          <li>侵犯他人知识产权、隐私或其他合法权益；</li>
          <li>试图未经授权访问、干扰或破坏本站的正常运行与安全；</li>
          <li>滥用接口、进行自动化爬取或规避限流等技术措施。</li>
        </ul>
      </Section>

      <Section title="三、你的内容">
        <p>你对自己创建的导航内容保留所有权利，并对其合法性负责。为运行服务，你授予我们在必要范围内存储、展示与传输该内容的许可。你发布为“公开”的页面可被任何人访问。</p>
      </Section>

      <Section title="四、服务可用性">
        <p>本站按“现状”提供，我们会尽合理努力保持其可用，但不保证服务不中断、无错误或永久可用。我们可能因维护、升级或不可抗力暂停或调整服务。</p>
      </Section>

      <Section title="五、账号终止">
        <p>若你违反本条款，我们可暂停或终止你的账号。你也可随时停止使用并请求删除账号。终止后，本条款中依其性质应继续有效的条款仍然有效。</p>
      </Section>

      <Section title="六、免责声明与责任限制">
        <p>在适用法律允许的最大范围内，本站不对因使用或无法使用服务而产生的任何间接、偶然或后果性损失承担责任。我们对本站所链接的第三方网站的内容不负责任。</p>
      </Section>

      <Section title="七、开源许可">
        <p>本站基于开源项目 nav.ax 构建，该项目以 GNU AGPL-3.0 许可证发布。本服务条款约束的是你与 [运营方名称] 之间的服务关系，与项目源码的开源许可相互独立。</p>
      </Section>

      <Section title="八、适用法律">
        <p>本条款的订立、解释与争议解决适用 <strong>[司法辖区/所在地区]</strong> 法律。因本条款引起的争议，双方应先友好协商解决。</p>
      </Section>

      <Section title="九、条款变更">
        <p>我们可能修订本条款，重大变更将在本页公布。变更后你继续使用本站即视为接受修订后的条款。</p>
      </Section>

      <Section title="十、联系我们">
        <p>如对本服务条款有任何疑问，请联系：<strong>[运营方名称]</strong>，邮箱 <strong>[联系邮箱]</strong>。</p>
      </Section>
    </LegalLayout>
  );
}
