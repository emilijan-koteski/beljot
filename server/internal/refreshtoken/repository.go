package refreshtoken

// Repository is the persistence boundary for refresh-token rotation, reuse
// detection, and revocation. Handlers depend on this interface, never on GORM
// directly.
type Repository interface {
	Create(token *RefreshToken) error
	// FindByHash returns the token row whose token_hash matches, regardless of
	// its rotated/revoked/expired state — the handler inspects those fields to
	// decide rotate vs. reuse-detect. Returns (nil, nil) when no row matches.
	FindByHash(tokenHash string) (*RefreshToken, error)
	// RotateAndReplace atomically consumes a still-live token and inserts its
	// successor in ONE transaction: it stamps rotated_at on the token only if it
	// is still live (rotated_at IS NULL AND revoked_at IS NULL) and, only then,
	// creates the successor. Returns rotated=true only if THIS call won the
	// rotation. If the successor insert fails the rotation is rolled back, so a
	// failure never strands the family without a live token. A concurrent caller
	// that lost the race (already rotated/revoked) gets rotated=false and no
	// successor is created.
	RotateAndReplace(oldID uint, successor *RefreshToken) (rotated bool, err error)
	// RevokeFamily stamps revoked_at on every not-yet-revoked token in the
	// family, terminating the whole session (logout, reuse detection, expiry).
	RevokeFamily(familyID string) error
}
