package passwordreset

// Repository is the persistence boundary for password-reset tokens. Handlers
// depend on this interface, never on GORM directly.
type Repository interface {
	Create(token *PasswordResetToken) error
	// FindValidByHash returns the token row whose token_hash matches and which
	// is neither used nor expired. Returns (nil, nil) when no such row exists —
	// callers treat that as an invalid/expired link.
	FindValidByHash(tokenHash string) (*PasswordResetToken, error)
	// MarkUsed atomically stamps used_at on a still-unused token, returning
	// true only if THIS call consumed it. A concurrent caller that lost the
	// race (token already used) gets false — this is the single-use guard.
	MarkUsed(id uint) (consumed bool, err error)
	// DeleteByUserID removes every reset token for a user. Called before issuing
	// a fresh token so only the most recent link is ever live.
	DeleteByUserID(userID uint) error
}
