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

func (r *GormRepository) RotateAndReplace(oldID uint, successor *RefreshToken) (bool, error) {
	rotated := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Serialize rotate-vs-revoke per family (see lockFamily). A concurrent
		// RevokeFamily either runs entirely before us (our guarded update then
		// finds the row revoked → we lose) or entirely after us (it sees our
		// committed successor and revokes it too). Without this, a revoke whose
		// snapshot predates our commit could miss the successor and leave a
		// "revoked" family with one live token.
		if err := lockFamily(tx, successor.FamilyID); err != nil {
			return err
		}
		// The `rotated_at IS NULL AND revoked_at IS NULL` guard makes rotation
		// atomic: only one of two concurrent refreshes bearing the same live
		// token sees RowsAffected == 1.
		res := tx.Model(&RefreshToken{}).
			Where("id = ? AND rotated_at IS NULL AND revoked_at IS NULL", oldID).
			Update("rotated_at", time.Now())
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			// Lost the race (already rotated or revoked). Do not create a
			// successor; leave rotated=false.
			return nil
		}
		// Insert the successor in the same transaction: if this fails the
		// rotation above is rolled back, leaving the old token live.
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

func (r *GormRepository) RevokeFamily(familyID string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Serialize against a concurrent RotateAndReplace so we never revoke the
		// family while a successor is being minted invisibly (see lockFamily).
		if err := lockFamily(tx, familyID); err != nil {
			return err
		}
		return tx.Model(&RefreshToken{}).
			Where("family_id = ? AND revoked_at IS NULL", familyID).
			Update("revoked_at", time.Now()).Error
	})
}

// lockFamily takes a transaction-scoped Postgres advisory lock keyed on the
// family id, held until the surrounding transaction commits/rolls back. Rotate
// and revoke for the same family thus run one-at-a-time; different families hash
// to different keys and never contend.
func lockFamily(tx *gorm.DB, familyID string) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", familyID).Error
}
