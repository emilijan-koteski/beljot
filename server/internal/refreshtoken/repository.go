package refreshtoken

// Repository is the persistence boundary for refresh-token rotation, reuse
// detection, and revocation.
type Repository interface {
	Create(token *RefreshToken) error
	FindByHash(tokenHash string) (*RefreshToken, error)
	RotateAndReplace(oldID uint, successor *RefreshToken) (rotated bool, err error)
	RevokeFamily(familyID string) error
}
