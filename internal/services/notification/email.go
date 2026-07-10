package notification

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
	"sync"
)

// EmailSender SMTP 邮件发送器，支持明文和 HTML 邮件
// EmailSender SMTP 邮件发送器，支持明文和 HTML 邮件
type EmailSender struct {
	name     string       // 发送器名称
	host     string       // SMTP 服务器地址
	port     int          // SMTP 服务器端口
	username string       // SMTP 用户名
	password string       // SMTP 密码
	from     string       // 发件人地址
	mu       sync.RWMutex // 读写锁
}

// NewEmailSender 创建 SMTP 邮件发送器
func NewEmailSender(name, host string, port int, username, password, from string) *EmailSender {
	return &EmailSender{
		name:     name,
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
	}
}

// Name 返回发送器名称
func (e *EmailSender) Name() string { return e.name }

// Channel 返回邮件渠道标识
func (e *EmailSender) Channel() Channel { return ChannelEmail }

// Send 发送邮件，msg.Body 为纯文本，msg.HTMLBody 不为空时自动切换为 HTML
func (e *EmailSender) Send(ctx context.Context, msg Message) (*Result, error) {
	if len(msg.To) == 0 {
		return nil, fmt.Errorf("notification: email has no recipients")
	}

	auth := smtp.PlainAuth("", e.username, e.password, e.host)

	subject := msg.Subject
	if subject == "" {
		subject = "Notification"
	}

	body := msg.Body
	contentType := "text/plain; charset=UTF-8"
	if msg.HTMLBody != "" {
		body = msg.HTMLBody
		contentType = "text/html; charset=UTF-8"
	}

	header := make([]string, 0)
	header = append(header, fmt.Sprintf("From: %s", e.from))
	header = append(header, fmt.Sprintf("To: %s", strings.Join(msg.To, ",")))
	header = append(header, fmt.Sprintf("Subject: %s", subject))
	header = append(header, "MIME-Version: 1.0")
	header = append(header, fmt.Sprintf("Content-Type: %s", contentType))
	header = append(header, "")
	header = append(header, body)

	msgData := []byte(strings.Join(header, "\r\n"))

	addr := fmt.Sprintf("%s:%d", e.host, e.port)
	if err := smtp.SendMail(addr, auth, e.from, msg.To, msgData); err != nil {
		return &Result{
			MessageID: msg.ID,
			Channel:   ChannelEmail,
			Success:   false,
			Error:     err.Error(),
		}, nil
	}

	return &Result{
		MessageID: msg.ID,
		Channel:   ChannelEmail,
		Success:   true,
	}, nil
}

// Close 关闭邮件发送器（空操作）
func (e *EmailSender) Close() error { return nil }
