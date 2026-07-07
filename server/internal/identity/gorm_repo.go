package identity

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/emilijan/beljot/server/internal/apperr"
)

type GormRepository struct {
	db *gorm.DB
}

func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

func (r *GormRepository) Create(identity *Identity) error {
	if err := r.db.Create(identity).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// Either unique index (provider+subject taken, or the user already
			// has an identity for this provider) is the same conflict to callers.
			return apperr.ErrSSOIdentityInUse
		}
		return err
	}
	return nil
}

func (r *GormRepository) FindByProviderSubject(provider, subject string) (*Identity, error) {
	var i Identity
	err := r.db.Where("provider = ? AND provider_user_id = ?", provider, subject).First(&i).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &i, nil
}

func (r *GormRepository) FindByUserID(userID uint) ([]Identity, error) {
	var identities []Identity
	if err := r.db.Where("user_id = ?", userID).Order("created_at asc").Find(&identities).Error; err != nil {
		return nil, err
	}
	return identities, nil
}

func (r *GormRepository) DeleteByUserProvider(userID uint, provider string) (int64, error) {
	res := r.db.Where("user_id = ? AND provider = ?", userID, provider).Delete(&Identity{})
	return res.RowsAffected, res.Error
}
