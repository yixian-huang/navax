package integrations

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/netguard"
)

// ErrMailNotConfigured reports that no enabled, configured SMTP provider exists,
// so outbound mail cannot be sent.
var ErrMailNotConfigured = errors.New("smtp provider is not configured")

// Message is a single outbound email. TextBody is required; HTMLBody is optional
// and, when present, is sent as the richer alternative.
type Message struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}

// MailConfigured reports whether an enabled, configured SMTP provider is present.
// Callers use it to decide whether to attempt delivery (e.g. password recovery).
func (s *Service) MailConfigured(ctx context.Context) bool {
	provider, err := s.Get(ctx, SMTP)
	if err != nil {
		return false
	}
	return provider.Enabled && provider.Configured
}

// SendMail delivers msg through the configured SMTP provider. It reuses the
// internal-friendly SSRF guard so a relay on a private network is permitted
// while loopback and cloud-metadata targets stay blocked.
func (s *Service) SendMail(ctx context.Context, msg Message) error {
	provider, err := s.Get(ctx, SMTP)
	if err != nil {
		return err
	}
	if !provider.Enabled || !provider.Configured {
		return ErrMailNotConfigured
	}
	secrets, err := s.mergedSecrets(ctx, SMTP, nil)
	if err != nil {
		return err
	}
	if strings.TrimSpace(msg.To) == "" || strings.TrimSpace(msg.TextBody) == "" {
		return fmt.Errorf("%w: recipient and text body are required", ErrInvalidSettings)
	}
	return sendSMTP(ctx, netguard.NewInternalValidator(nil), provider.Settings, secrets, msg)
}

func sendSMTP(ctx context.Context, validator netguard.Validator, settings map[string]any, secrets map[string]string, msg Message) error {
	host := stringSetting(settings, "host")
	port, _ := numericSetting(settings, "port")
	from := stringSetting(settings, "fromAddress")
	address := net.JoinHostPort(host, strconv.Itoa(port))

	dialer := netguard.Dialer{Validator: validator, Dialer: net.Dialer{Timeout: 10 * time.Second}}
	connection, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("连接 SMTP 失败: %w", err)
	}
	if stringSetting(settings, "tlsMode") == "tls" {
		tlsConnection := tls.Client(connection, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		if handshakeErr := tlsConnection.HandshakeContext(ctx); handshakeErr != nil {
			_ = connection.Close()
			return fmt.Errorf("SMTP TLS 握手失败: %w", handshakeErr)
		}
		connection = tlsConnection
	}
	defer connection.Close()

	client, err := smtp.NewClient(connection, host)
	if err != nil {
		return fmt.Errorf("SMTP 握手失败: %w", err)
	}
	defer client.Close()

	if stringSetting(settings, "tlsMode") == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("SMTP 服务器不支持 STARTTLS")
		}
		if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("SMTP STARTTLS 失败: %w", err)
		}
	}
	if username := stringSetting(settings, "username"); username != "" {
		password := secrets["password"]
		if password == "" {
			return errors.New("SMTP 密码尚未配置")
		}
		if err := client.Auth(smtp.PlainAuth("", username, password, host)); err != nil {
			return fmt.Errorf("SMTP 认证失败: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("SMTP MAIL FROM 失败: %w", err)
	}
	if err := client.Rcpt(msg.To); err != nil {
		return fmt.Errorf("SMTP RCPT TO 失败: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA 失败: %w", err)
	}
	if _, err := writer.Write(buildMIME(from, stringSetting(settings, "fromName"), msg, time.Now())); err != nil {
		_ = writer.Close()
		return fmt.Errorf("写入邮件正文失败: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("提交邮件失败: %w", err)
	}
	return client.Quit()
}

// buildMIME renders an RFC 5322 message. When an HTML body is present it emits a
// multipart/alternative body so clients can pick the richer part; otherwise it
// sends a single text/plain part. Headers with non-ASCII values use MIME
// encoded-words so Chinese subjects and display names survive transport.
func buildMIME(from, fromName string, msg Message, now time.Time) []byte {
	var buffer bytes.Buffer
	header := textproto.MIMEHeader{}
	if strings.TrimSpace(fromName) != "" {
		header.Set("From", fmt.Sprintf("%s <%s>", mime.BEncoding.Encode("utf-8", fromName), from))
	} else {
		header.Set("From", from)
	}
	header.Set("To", msg.To)
	header.Set("Subject", mime.BEncoding.Encode("utf-8", msg.Subject))
	header.Set("Date", now.Format(time.RFC1123Z))
	header.Set("MIME-Version", "1.0")

	if strings.TrimSpace(msg.HTMLBody) == "" {
		header.Set("Content-Type", "text/plain; charset=utf-8")
		header.Set("Content-Transfer-Encoding", "8bit")
		writeHeaders(&buffer, header)
		buffer.WriteString("\r\n")
		buffer.WriteString(normalizeCRLF(msg.TextBody))
		return buffer.Bytes()
	}

	boundary := "navax-" + strconv.FormatInt(now.UnixNano(), 36)
	header.Set("Content-Type", "multipart/alternative; boundary="+boundary)
	writeHeaders(&buffer, header)
	buffer.WriteString("\r\n")
	writePart(&buffer, boundary, "text/plain; charset=utf-8", msg.TextBody)
	writePart(&buffer, boundary, "text/html; charset=utf-8", msg.HTMLBody)
	buffer.WriteString("--" + boundary + "--\r\n")
	return buffer.Bytes()
}

func writeHeaders(buffer *bytes.Buffer, header textproto.MIMEHeader) {
	for _, key := range []string{"From", "To", "Subject", "Date", "MIME-Version", "Content-Type", "Content-Transfer-Encoding"} {
		if value := header.Get(key); value != "" {
			buffer.WriteString(key + ": " + value + "\r\n")
		}
	}
}

func writePart(buffer *bytes.Buffer, boundary, contentType, body string) {
	buffer.WriteString("--" + boundary + "\r\n")
	buffer.WriteString("Content-Type: " + contentType + "\r\n")
	buffer.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	buffer.WriteString(normalizeCRLF(body))
	buffer.WriteString("\r\n")
}

func normalizeCRLF(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	return strings.ReplaceAll(body, "\n", "\r\n")
}
