// Package mailer gửi thư chữ thuần UTF-8 qua SMTP.
//
// Dev gửi vào Mailpit (localhost:1025, không TLS, không xác thực). Production
// gửi qua relay thật: có SMTP_USERNAME thì BẮT BUỘC STARTTLS — smtp.PlainAuth
// của thư viện chuẩn tự từ chối gửi mật khẩu qua đường trần tới host khác
// localhost, nên server không hỗ trợ STARTTLS là lỗi chứ không phải lộ mật khẩu.
package mailer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"base-ecommerce/api/pkg/config"
)

// sendTimeout là trần cho TOÀN BỘ một lần gửi (kết nối, bắt tay, truyền thư).
// Worker gửi trong transaction đang giữ khóa dòng thư — treo vô hạn ở đây là
// giữ khóa và kết nối database vô hạn.
const sendTimeout = 15 * time.Second

type SMTP struct {
	addr     string
	host     string
	username string
	password string
	from     *mail.Address
}

func New(cfg config.Mail) (*SMTP, error) {
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("MAIL_FROM %q không hợp lệ: %w", cfg.From, err)
	}
	return &SMTP{addr: net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)), host: cfg.Host,
		username: cfg.Username, password: cfg.Password, from: from}, nil
}

func (s *SMTP) Send(ctx context.Context, to, subject, body string) error {
	rcpt, err := mail.ParseAddress(to)
	if err != nil {
		return fmt.Errorf("người nhận không hợp lệ: %w", err)
	}
	msg, err := s.build(rcpt, subject, body)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", s.addr)
	if err != nil {
		return fmt.Errorf("kết nối SMTP %s: %w", s.addr, err)
	}
	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline) // DialContext chỉ canh lúc kết nối, không canh hội thoại SMTP

	c, err := smtp.NewClient(conn, s.host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("bắt tay SMTP: %w", err)
	}
	defer func() { _ = c.Close() }()

	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("STARTTLS: %w", err)
		}
	}
	if s.username != "" {
		if err := c.Auth(smtp.PlainAuth("", s.username, s.password, s.host)); err != nil {
			return fmt.Errorf("xác thực SMTP: %w", err)
		}
	}
	if err := c.Mail(s.from.Address); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	if err := c.Rcpt(rcpt.Address); err != nil {
		return fmt.Errorf("RCPT TO: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("ghi thư: %w", err)
	}
	// Close của DATA là lúc server trả lời "đã nhận" — lỗi ở đây nghĩa là thư
	// CHƯA được nhận, phải báo lỗi để thử lại.
	if err := w.Close(); err != nil {
		return fmt.Errorf("kết thúc DATA: %w", err)
	}
	return c.Quit()
}

// build dựng thư RFC 5322. Tiêu đề tiếng Việt phải mã hóa kiểu RFC 2047
// (=?utf-8?q?...?=) — header chỉ được chứa ASCII; nội dung dùng
// quoted-printable để không dòng nào vượt 998 byte và không phụ thuộc server
// có hỗ trợ 8BITMIME hay không.
func (s *SMTP) build(to *mail.Address, subject, body string) ([]byte, error) {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return nil, err
	}
	domain := s.from.Address[strings.LastIndex(s.from.Address, "@")+1:]

	var buf bytes.Buffer
	h := func(k, v string) { fmt.Fprintf(&buf, "%s: %s\r\n", k, v) }
	h("From", s.from.String()) // mail.Address.String tự mã hóa tên hiển thị
	h("To", to.String())
	h("Subject", mime.QEncoding.Encode("utf-8", subject))
	h("Date", time.Now().Format(time.RFC1123Z))
	h("Message-ID", "<"+hex.EncodeToString(id)+"@"+domain+">")
	h("MIME-Version", "1.0")
	h("Content-Type", "text/plain; charset=utf-8")
	h("Content-Transfer-Encoding", "quoted-printable")
	buf.WriteString("\r\n")

	qp := quotedprintable.NewWriter(&buf)
	if _, err := qp.Write([]byte(strings.ReplaceAll(body, "\n", "\r\n"))); err != nil {
		return nil, err
	}
	if err := qp.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
