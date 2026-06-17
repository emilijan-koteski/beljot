package mailer

import (
	"context"
	"fmt"
	"time"

	mail "github.com/wneessen/go-mail"
)

// SMTPMailer sends real email over SMTP via go-mail. Configured for STARTTLS
// (e.g. Gmail on smtp.gmail.com:587 with an app password).
type SMTPMailer struct {
	host     string
	port     int
	username string
	password string
	fromAddr string
	fromName string
}

func NewSMTPMailer(host string, port int, username, password, fromAddr, fromName string) *SMTPMailer {
	return &SMTPMailer{
		host:     host,
		port:     port,
		username: username,
		password: password,
		fromAddr: fromAddr,
		fromName: fromName,
	}
}

func (m *SMTPMailer) SendPasswordReset(ctx context.Context, to, lang, resetLink string) error {
	subject, body := renderPasswordReset(lang, resetLink)

	msg := mail.NewMsg()
	if err := msg.FromFormat(m.fromName, m.fromAddr); err != nil {
		return fmt.Errorf("setting from address: %w", err)
	}
	if err := msg.To(to); err != nil {
		return fmt.Errorf("setting to address: %w", err)
	}
	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextHTML, body)

	client, err := mail.NewClient(m.host,
		mail.WithPort(m.port),
		mail.WithTLSPolicy(mail.TLSMandatory),
		mail.WithSMTPAuth(mail.SMTPAuthLogin),
		mail.WithUsername(m.username),
		mail.WithPassword(m.password),
		mail.WithTimeout(15*time.Second),
	)
	if err != nil {
		return fmt.Errorf("creating mail client: %w", err)
	}

	if err := client.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf("sending password reset email: %w", err)
	}
	return nil
}
