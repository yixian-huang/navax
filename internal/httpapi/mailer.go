package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/integrations"
)

// Mailer sends transactional email. *integrations.Service satisfies it; a nil
// Mailer means outbound mail is disabled for this build.
type Mailer interface {
	MailConfigured(ctx context.Context) bool
	SendMail(ctx context.Context, msg integrations.Message) error
}

var emailTemplate = template.Must(template.New("email").Parse(`<!doctype html>
<html><body style="margin:0;padding:24px;background:#f5f5f7;font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;color:#1d1d1f">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tr><td align="center">
<table role="presentation" width="480" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:12px;padding:32px">
<tr><td style="font-size:18px;font-weight:600;padding-bottom:12px">{{.InstanceName}}</td></tr>
<tr><td style="font-size:14px;line-height:1.6;padding-bottom:20px">{{.Intro}}</td></tr>
<tr><td style="padding-bottom:20px"><a href="{{.ActionURL}}" style="display:inline-block;background:#0071e3;color:#ffffff;text-decoration:none;font-size:14px;padding:10px 20px;border-radius:8px">{{.ActionLabel}}</a></td></tr>
<tr><td style="font-size:12px;line-height:1.6;color:#6e6e73;word-break:break-all;padding-bottom:16px">若按钮无法点击，请复制此链接到浏览器打开：<br>{{.ActionURL}}</td></tr>
<tr><td style="font-size:12px;line-height:1.6;color:#6e6e73">{{.Note}}</td></tr>
</table></td></tr></table></body></html>`))

type emailContent struct {
	InstanceName string
	Intro        string
	ActionLabel  string
	ActionURL    string
	Note         string
}

func (c emailContent) render() string {
	var buffer bytes.Buffer
	if err := emailTemplate.Execute(&buffer, c); err != nil {
		return ""
	}
	return buffer.String()
}

func fallbackInstanceName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "nav.ax"
	}
	return name
}

func inviteMessage(instanceName, to, inviteURL string, expiresAt time.Time) integrations.Message {
	instanceName = fallbackInstanceName(instanceName)
	expiry := expiresAt.Format("2006-01-02 15:04 MST")
	text := fmt.Sprintf("你好，\n\n你受邀加入 %s。请在 %s 前打开以下链接完成注册：\n\n%s\n\n如果你并未预期收到此邀请，忽略本邮件即可。",
		instanceName, expiry, inviteURL)
	html := emailContent{
		InstanceName: instanceName,
		Intro:        fmt.Sprintf("你受邀加入 %s，请在 %s 前完成注册。", instanceName, expiry),
		ActionLabel:  "接受邀请",
		ActionURL:    inviteURL,
		Note:         "如果你并未预期收到此邀请，忽略本邮件即可。",
	}.render()
	return integrations.Message{To: to, Subject: fmt.Sprintf("邀请你加入 %s", instanceName), TextBody: text, HTMLBody: html}
}

func passwordResetMessage(instanceName, to, resetURL string, expiresAt time.Time) integrations.Message {
	instanceName = fallbackInstanceName(instanceName)
	expiry := expiresAt.Format("2006-01-02 15:04 MST")
	text := fmt.Sprintf("你好，\n\n我们收到了重置 %s 账号密码的请求。请在 %s 前打开以下链接设置新密码：\n\n%s\n\n如果这不是你本人操作，忽略本邮件即可，你的密码不会改变。",
		instanceName, expiry, resetURL)
	html := emailContent{
		InstanceName: instanceName,
		Intro:        fmt.Sprintf("我们收到了重置账号密码的请求，链接将在 %s 前有效。", expiry),
		ActionLabel:  "重置密码",
		ActionURL:    resetURL,
		Note:         "如果这不是你本人操作，忽略本邮件即可，你的密码不会改变。",
	}.render()
	return integrations.Message{To: to, Subject: fmt.Sprintf("重置你的 %s 密码", instanceName), TextBody: text, HTMLBody: html}
}
