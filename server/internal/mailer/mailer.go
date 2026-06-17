package mailer

import "context"

// Mailer sends transactional emails. Implementations must be safe for
// concurrent use by multiple goroutines.
type Mailer interface {
	// SendPasswordReset sends the password-reset email to `to`, localized to
	// `lang` (en/sr/mk/hr — unknown or empty falls back to en), embedding the
	// absolute `resetLink`.
	SendPasswordReset(ctx context.Context, to, lang, resetLink string) error
}
