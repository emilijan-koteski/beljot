package passwordreset

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

func (r *GormRepository) Create(token *PasswordResetToken) error {
	return r.db.Create(token).Error
}

// FindValidByHash returns the unused, unexpired token with this hash, or (nil, nil) if none exists.
func (r *GormRepository) FindValidByHash(tokenHash string) (*PasswordResetToken, error) {
	var t PasswordResetToken
	err := r.db.
		Where("token_hash = ? AND used_at IS NULL AND expires_at > ?", tokenHash, time.Now()).
		First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

// MarkUsed stamps used_at on a still-unused token and reports whether this call consumed it.
// The `used_at IS NULL` guard makes consumption atomic under concurrent resets.
func (r *GormRepository) MarkUsed(id uint) (bool, error) {
	now := time.Now()
	res := r.db.Model(&PasswordResetToken{}).
		Where("id = ? AND used_at IS NULL", id).
		Update("used_at", now)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// DeleteByUserID removes every reset token for a user so only the newest link stays live.
func (r *GormRepository) DeleteByUserID(userID uint) error {
	return r.db.Where("user_id = ?", userID).Delete(&PasswordResetToken{}).Error
}
