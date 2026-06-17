package mailer

import (
	"context"
	"log/slog"
)

// LogMailer is the development fallback used when SMTP is not configured. It
// logs the reset link instead of sending mail, so the forgot-password flow is
// fully testable without a real mailbox or SMTP credentials.
type LogMailer struct{}

func NewLogMailer() *LogMailer {
	return &LogMailer{}
}

func (m *LogMailer) SendPasswordReset(_ context.Context, to, lang, resetLink string) error {
	slog.Info("password reset email (LogMailer — SMTP not configured)",
		"to", to,
		"lang", lang,
		"resetLink", resetLink,
	)
	return nil
}
