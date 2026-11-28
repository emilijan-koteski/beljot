package mailer

import (
	"context"
	"fmt"
	"strings"
	"time"

	mail "github.com/wneessen/go-mail"
)

// SMTPMailer sends real email over SMTP via go-mail with mandatory STARTTLS.
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
		return fmt.Errorf("setting to address: %w", redactRecipient(err, to))
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
		return fmt.Errorf("sending password reset email: %w", redactRecipient(err, to))
	}
	return nil
}

// recipientRedactedError keeps the wrapped SMTP error for errors.Is and errors.As
// but prints it with the recipient address masked, so logs never carry it.
type recipientRedactedError struct {
	err  error
	text string
}

func (e *recipientRedactedError) Error() string { return e.text }

func (e *recipientRedactedError) Unwrap() error { return e.err }

// redactRecipient masks every occurrence of the recipient address in err's message.
func redactRecipient(err error, to string) error {
	text := err.Error()
	if to != "" {
		text = strings.ReplaceAll(text, to, "[recipient]")
	}
	return &recipientRedactedError{err: err, text: text}
}
