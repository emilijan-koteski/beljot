package mailer

import "context"

// Mailer sends localized transactional emails and must be safe for concurrent use.
type Mailer interface {
	SendPasswordReset(ctx context.Context, to, lang, resetLink string) error
}
