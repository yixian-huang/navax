# Security Policy

nav.ax 是自托管服务,安全问题请通过私密渠道报告,不要公开提交 issue。
(Chinese summary: report vulnerabilities privately, never via public issues.)

## Supported Versions

Security fixes land on the `main` branch and the latest release. Older releases
do not receive backports — upgrade to the latest release before reporting.

## Reporting a Vulnerability

Please **do not** open a public issue for anything you believe is a security
vulnerability.

Instead, use GitHub's private vulnerability reporting:

1. Go to the repository's **Security** tab.
2. Click **Report a vulnerability**
   (<https://github.com/yixian-huang/navax/security/advisories/new>).
3. Include the affected version or commit, reproduction steps, and your
   assessment of the impact. A suggested fix is welcome but not required.

You should receive an acknowledgement within 7 days. Please allow a reasonable
disclosure window for a fix to be released before publishing details.

## Scope

Reports in these areas are especially valuable, as the project makes explicit
guarantees about them:

- Session handling (Host-only, HttpOnly, SameSite=Lax cookies) and `Origin`
  validation on non-safe methods
- SSRF guards on all server-side URL fetching (loopback, private, link-local,
  reserved, and cloud-metadata addresses must be rejected on every DNS
  resolution and redirect)
- Upload validation (MIME/size restrictions, SVG rejected by default)
- Token storage (session/invite/recovery tokens stored only as hashes) and
  encryption of third-party secrets under `NAVAX_MASTER_KEY`
- Rate limiting on login, invites, events, and link checks
- Privacy guarantees (full IPs never persisted; daily-rotating HMAC visitor IDs)

Out of scope: vulnerabilities in third-party dependencies without a
demonstrated impact on nav.ax, unvalidated automated-scanner output, and
issues requiring a compromised host or physical access.
