package user

import (
	"errors"
	"log/slog"
	"slices"
	"strings"
	"time"

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

// Delete soft-deletes the user, which frees their email and username under the partial unique indexes.
func (r *GormUserRepository) Delete(id uint) error {
	result := r.db.Delete(&User{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return apperr.ErrUserNotFound
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

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// escapeLike escapes LIKE wildcards so `%` and `_` (a legal username character) match literally.
func escapeLike(s string) string {
	return likeEscaper.Replace(s)
}

// SearchByUsername returns live users whose username contains the query, case-insensitively, excluding excludeUserID.
func (r *GormUserRepository) SearchByUsername(query string, excludeUserID uint, limit int) ([]User, error) {
	var users []User
	pattern := "%" + escapeLike(query) + "%"
	if err := r.db.
		Where(`username ILIKE ? ESCAPE '\'`, pattern).
		Where("id <> ?", excludeUserID).
		Order("username ASC").
		Limit(limit).
		Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
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

// UpdatePreferences writes every supplied preference column in one UPDATE.
// It uses a map so false and 0 values are written, since GORM skips zero-value struct fields.
func (r *GormUserRepository) UpdatePreferences(id uint, prefs PreferencesUpdate) error {
	columns := make(map[string]interface{}, 7)
	if prefs.LanguagePreference != nil {
		columns["language_preference"] = *prefs.LanguagePreference
	}
	if prefs.CardDeckPreference != nil {
		columns["card_deck_preference"] = *prefs.CardDeckPreference
	}
	if prefs.SoundEnabled != nil {
		columns["sound_enabled"] = *prefs.SoundEnabled
	}
	if prefs.MusicEnabled != nil {
		columns["music_enabled"] = *prefs.MusicEnabled
	}
	if prefs.SoundVolume != nil {
		columns["sound_volume"] = *prefs.SoundVolume
	}
	if prefs.MusicVolume != nil {
		columns["music_volume"] = *prefs.MusicVolume
	}
	if prefs.AllowNonFriendInvites != nil {
		columns["allow_non_friend_invites"] = *prefs.AllowNonFriendInvites
	}
	if len(columns) == 0 {
		return apperr.ErrBadRequest
	}
	result := r.db.Model(&User{}).Where("id = ?", id).Updates(columns)
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

func (r *GormUserRepository) UpdateUsername(id uint, username string) (time.Time, error) {
	now := time.Now().UTC()
	cutoff := now.Add(-UsernameChangeCooldown)
	result := r.db.Model(&User{}).
		Where("id = ? AND (username_changed_at IS NULL OR username_changed_at < ?)", id, cutoff).
		Updates(map[string]interface{}{
			"username":            username,
			"username_changed_at": now,
		})
	if err := result.Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "username") {
			return time.Time{}, apperr.ErrUsernameTaken
		}
		return time.Time{}, err
	}
	if result.RowsAffected == 0 {
		return time.Time{}, apperr.ErrUsernameChangeTooSoon
	}
	return now, nil
}

// UpdateAvatarKey replaces the user's avatar prefix (nil clears it) and returns the previous one, read under the same row lock.
// Returns ErrUserNotFound when no live row matches.
func (r *GormUserRepository) UpdateAvatarKey(id uint, key *string) (*string, error) {
	var previous *string
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var u User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "avatar_key").
			First(&u, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperr.ErrUserNotFound
			}
			return err
		}
		previous = u.AvatarKey
		return tx.Model(&User{}).Where("id = ?", id).Update("avatar_key", key).Error
	})
	if err != nil {
		return nil, err
	}
	return previous, nil
}

// AddXP adds each nonzero delta to total_xp in one transaction and returns the new totals.
// Rows are locked in ascending user ID order, like the wallet repo, so concurrent settlements cannot deadlock.
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

// TotalXPForUsers returns each requested user's total_xp keyed by ID; unknown IDs are absent.
func (r *GormUserRepository) TotalXPForUsers(ids []uint) (map[uint]int, error) {
	totals := make(map[uint]int, len(ids))
	if len(ids) == 0 {
		return totals, nil
	}
	var rows []struct {
		ID      uint
		TotalXP int
	}
	if err := r.db.Model(&User{}).
		Select("id", "total_xp").
		Where("id IN ?", ids).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		totals[row.ID] = row.TotalXP
	}
	return totals, nil
}

// ApplyHonorEvents records one finished match per user, locking rows in ascending ID order like AddXP.
// It writes absolute values computed in Go so the decay matches HonorScore on the read path.
func (r *GormUserRepository) ApplyHonorEvents(events map[uint]HonorEvent, now time.Time) (map[uint]HonorSnapshot, error) {
	snapshots := make(map[uint]HonorSnapshot, len(events))

	ids := make([]uint, 0, len(events))
	for id := range events {
		if id == 0 {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return snapshots, nil
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

			f := DecayFactor(u.HonorDecayedAt, now)
			completedWeight := u.HonorCompletedWeight * f
			abandonedWeight := u.HonorAbandonedWeight * f
			completedTotal := u.HonorCompletedTotal
			abandonedTotal := u.HonorAbandonedTotal

			if events[id].Abandoned {
				abandonedWeight += honorEventWeight
				abandonedTotal++
			} else {
				completedWeight += honorEventWeight
				completedTotal++
			}

			stamp := now
			if u.HonorDecayedAt != nil && stamp.Before(*u.HonorDecayedAt) {
				if u.HonorDecayedAt.Sub(now) <= honorMaxClockSkew {
					stamp = *u.HonorDecayedAt
				} else {
					slog.Warn("honor: stored decay stamp is implausibly far in the future, resetting to now",
						"userID", id,
						"storedDecayedAt", u.HonorDecayedAt,
						"now", now,
						"maxSkew", honorMaxClockSkew,
					)
				}
			}
			snapshot := NewHonorSnapshot(completedWeight, abandonedWeight, &stamp, completedTotal, abandonedTotal, now)

			if err := tx.Model(&User{}).Where("id = ?", id).
				Updates(map[string]interface{}{
					"honor_completed_weight": completedWeight,
					"honor_abandoned_weight": abandonedWeight,
					"honor_decayed_at":       stamp,
					"honor_completed_total":  completedTotal,
					"honor_abandoned_total":  abandonedTotal,
					"honor_score":            snapshot.Score,
				}).Error; err != nil {
				return err
			}

			snapshots[id] = snapshot
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return snapshots, nil
}

// ResetHonor clears a user's honor weights and abandonments under a row lock.
// It keeps honor_completed_total so a pardoned veteran is not reclassified as a new player.
func (r *GormUserRepository) ResetHonor(userID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var u User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperr.ErrUserNotFound
			}
			return err
		}
		return tx.Model(&User{}).Where("id = ?", userID).
			Updates(map[string]interface{}{
				"honor_completed_weight": 0,
				"honor_abandoned_weight": 0,
				"honor_decayed_at":       nil,
				"honor_abandoned_total":  0,
				"honor_score":            HonorScore(0, 0, nil, time.Time{}),
			}).Error
	})
}
