package refreshtoken

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type GormRepository struct {
	db *gorm.DB
}

func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

func (r *GormRepository) Create(token *RefreshToken) error {
	return r.db.Create(token).Error
}

// FindByHash returns the token with this hash whatever its rotated/revoked/expired state,
// or (nil, nil) if none matches.
func (r *GormRepository) FindByHash(tokenHash string) (*RefreshToken, error) {
	var t RefreshToken
	err := r.db.Where("token_hash = ?", tokenHash).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

// RotateAndReplace rotates a still-live token and inserts its successor in one transaction,
// reporting whether this call won the rotation. The family lock serializes it against
// RevokeFamily, so a revoke can never miss a freshly minted successor.
func (r *GormRepository) RotateAndReplace(oldID uint, successor *RefreshToken) (bool, error) {
	rotated := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockFamily(tx, successor.FamilyID); err != nil {
			return err
		}
		res := tx.Model(&RefreshToken{}).
			Where("id = ? AND rotated_at IS NULL AND revoked_at IS NULL", oldID).
			Update("rotated_at", time.Now())
		if res.Error != nil {
			return res.Error
		}
		lostRace := res.RowsAffected != 1
		if lostRace {
			return nil
		}
		if err := tx.Create(successor).Error; err != nil {
			return err
		}
		rotated = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return rotated, nil
}

// RevokeFamily stamps revoked_at on every not-yet-revoked token in the family,
// holding the family lock so it cannot race a concurrent rotation.
func (r *GormRepository) RevokeFamily(familyID string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockFamily(tx, familyID); err != nil {
			return err
		}
		return tx.Model(&RefreshToken{}).
			Where("family_id = ? AND revoked_at IS NULL", familyID).
			Update("revoked_at", time.Now()).Error
	})
}

// lockFamily takes a transaction-scoped Postgres advisory lock keyed on the family id,
// so rotate and revoke for the same family run one at a time.
func lockFamily(tx *gorm.DB, familyID string) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", familyID).Error
}
