package user

import (
	"errors"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/emilijan/beljot/server/internal/apperr"
)

type GormUserRepository struct {
	db *gorm.DB
}

func NewGormUserRepository(db *gorm.DB) *GormUserRepository {
	return &GormUserRepository{db: db}
}

func (r *GormUserRepository) Create(user *User) error {
	if err := r.db.Create(user).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if strings.Contains(pgErr.ConstraintName, "email") {
				return apperr.ErrEmailTaken
			}
			if strings.Contains(pgErr.ConstraintName, "username") {
				return apperr.ErrUsernameTaken
			}
		}
		return err
	}
	return nil
}

func (r *GormUserRepository) FindByEmail(email string) (*User, error) {
	var u User
	if err := r.db.Where("email = ?", email).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *GormUserRepository) FindByUsername(username string) (*User, error) {
	var u User
	if err := r.db.Where("username = ?", username).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *GormUserRepository) FindByID(id uint) (*User, error) {
	var u User
	if err := r.db.First(&u, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *GormUserRepository) FindManyByIDs(ids []uint) ([]User, error) {
	if len(ids) == 0 {
		return []User{}, nil
	}
	var users []User
	if err := r.db.Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *GormUserRepository) Count() (int64, error) {
	var n int64
	if err := r.db.Model(&User{}).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

func (r *GormUserRepository) UpdateLanguagePreference(id uint, lang string) error {
	result := r.db.Model(&User{}).Where("id = ?", id).Update("language_preference", lang)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return apperr.ErrUserNotFound
	}
	return nil
}

func (r *GormUserRepository) UpdatePasswordHash(id uint, hash string) error {
	result := r.db.Model(&User{}).Where("id = ?", id).Update("password_hash", hash)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return apperr.ErrUserNotFound
	}
	return nil
}

// AddXP adds each delta to the matching user's total_xp inside one transaction
// and returns each user's resulting total (Story 9.5). Mirrors the wallet
// repo's ChargeStakes/ApplySettlement discipline: rows are locked FOR UPDATE in
// ascending userID order, so a concurrent wallet settlement (same order) and an
// XP award can't deadlock. Zero-delta entries are skipped (never locked, never
// returned). A missing row aborts and rolls back the whole batch with
// ErrUserNotFound — all-or-nothing, like the wallet path.
func (r *GormUserRepository) AddXP(awards map[uint]int) (map[uint]int, error) {
	newTotals := make(map[uint]int, len(awards))

	ids := make([]uint, 0, len(awards))
	for id, delta := range awards {
		if delta == 0 {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return newTotals, nil
	}
	slices.Sort(ids)

	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			var u User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, id).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return apperr.ErrUserNotFound
				}
				return err
			}
			newTotal := u.TotalXP + awards[id]
			if err := tx.Model(&User{}).Where("id = ?", id).
				Update("total_xp", newTotal).Error; err != nil {
				return err
			}
			newTotals[id] = newTotal
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return newTotals, nil
}
