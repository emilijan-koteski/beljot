package passwordreset

// Repository is the persistence boundary for password-reset tokens.
type Repository interface {
	Create(token *PasswordResetToken) error
	FindValidByHash(tokenHash string) (*PasswordResetToken, error)
	MarkUsed(id uint) (consumed bool, err error)
	DeleteByUserID(userID uint) error
}
