package integrations

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yixian-huang/navax/internal/database"
)

func TestBuildMIMEEncodesSubjectAndText(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 30, 0, 0, time.UTC)
	raw := string(buildMIME("noreply@nav.ax", "nav.ax 团队", Message{
		To: "user@example.com", Subject: "重置你的密码", TextBody: "点击链接\n重置密码",
	}, now))

	if !strings.Contains(raw, "Subject: =?utf-8?b?") {
		t.Fatalf("subject not MIME-encoded:\n%s", raw)
	}
	if !strings.Contains(raw, "From: =?utf-8?b?") || !strings.Contains(raw, "<noreply@nav.ax>") {
		t.Fatalf("from display name not encoded:\n%s", raw)
	}
	if !strings.Contains(raw, "Content-Type: text/plain; charset=utf-8") {
		t.Fatalf("missing text content type:\n%s", raw)
	}
	if !strings.Contains(raw, "点击链接\r\n重置密码") {
		t.Fatalf("body not CRLF-normalized:\n%s", raw)
	}
}

func TestBuildMIMEMultipartWhenHTMLPresent(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 30, 0, 0, time.UTC)
	raw := string(buildMIME("noreply@nav.ax", "", Message{
		To: "user@example.com", Subject: "invite", TextBody: "plain", HTMLBody: "<p>rich</p>",
	}, now))

	if !strings.Contains(raw, "Content-Type: multipart/alternative; boundary=navax-") {
		t.Fatalf("expected multipart/alternative:\n%s", raw)
	}
	if !strings.Contains(raw, "Content-Type: text/plain; charset=utf-8") || !strings.Contains(raw, "Content-Type: text/html; charset=utf-8") {
		t.Fatalf("expected both alternative parts:\n%s", raw)
	}
	if strings.Contains(raw, "From: =?") {
		t.Fatalf("empty display name should not be encoded:\n%s", raw)
	}
}

func TestSendMailWithoutProviderReturnsNotConfigured(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service, err := NewService(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	if service.MailConfigured(ctx) {
		t.Fatal("MailConfigured() = true without an SMTP provider")
	}
	if err := service.SendMail(ctx, Message{To: "user@example.com", Subject: "s", TextBody: "t"}); err == nil {
		t.Fatal("SendMail() succeeded without a configured provider")
	}
}
