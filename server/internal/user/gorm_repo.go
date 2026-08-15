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

// Delete soft-deletes the user (GORM stamps DeletedAt). The partial unique
// indexes on users (WHERE deleted_at IS NULL) mean the row's email and
// username become free again — exactly what the SSO-registration compensation
// path relies on.
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

// likeEscaper escapes the three Postgres LIKE/ILIKE metacharacters. Backslash
// (the escape character itself) is listed first, and strings.NewReplacer does a
// single left-to-right pass without re-scanning replaced text, so an escaped
// backslash is never double-escaped.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// escapeLike neutralises LIKE wildcards in user-supplied search text so `%` and
// `_` match literally. `_` is a legal username character, so an un-escaped one
// would act as a single-char wildcard and over-match.
func escapeLike(s string) string {
	return likeEscaper.Replace(s)
}

// SearchByUsername implements UserRepository. It is a model query, so GORM's
// default scope appends `deleted_at IS NULL` — a raw .Table("users") query would
// leak soft-deleted rows. ILIKE gives the case-insensitive substring match;
// ESCAPE '\' pairs with escapeLike above so escaped metacharacters are treated
// literally (Postgres' default LIKE escape is already '\', but stating it keeps
// the pairing explicit). The bounded Limit and self-exclusion are the caller's.
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

func (r *GormUserRepository) UpdateUsername(id uint, username string) (time.Time, error) {
	now := time.Now().UTC()
	// The cooldown predicate lives in the WHERE clause so enforcement is atomic:
	// the handler's pre-check is a fast, friendly path, but two concurrent
	// requests that both pass it cannot both write — only one satisfies the
	// "not within cooldown" condition once the first commits. Returns the stamp
	// so the handler echoes the persisted value (not a slightly-later one).
	cutoff := now.Add(-UsernameChangeCooldown)
	result := r.db.Model(&User{}).
		Where("id = ? AND (username_changed_at IS NULL OR username_changed_at < ?)", id, cutoff).
		Updates(map[string]interface{}{
			"username":            username,
			"username_changed_at": now,
		})
	if err := result.Error; err != nil {
		// A concurrent change that took this username between the handler's
		// pre-check and this write surfaces as pg 23505 on the username unique
		// index — map it to ErrUsernameTaken (409), mirroring Create.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "username") {
			return time.Time{}, apperr.ErrUsernameTaken
		}
		return time.Time{}, err
	}
	if result.RowsAffected == 0 {
		// The handler loaded the row and validated the name just before this
		// call, so a no-op update means a concurrent change consumed the
		// cooldown window in between — enforce it here (the race backstop).
		return time.Time{}, apperr.ErrUsernameChangeTooSoon
	}
	return now, nil
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

// TotalXPForUsers reads each requested user's total_xp in a single query and
// returns them keyed by ID. Read-only — used at match start to stamp static
// per-seat levels (Story: level-in-match). An empty input is a no-op (no DB
// round-trip); unknown IDs are simply absent from the result (no error).
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

// ApplyHonorEvents records one finished match per listed user (Story 9.7).
// Structure copied from AddXP above — one transaction, rows locked FOR UPDATE
// in ascending userID order — because honor is written from the same two match
// finalizers that settle the wallet and award XP, and a differing lock order
// between them is exactly how a deadlock gets introduced.
//
// Like AddXP it reads under the lock, computes in Go, and writes the ABSOLUTE
// value rather than a gorm.Expr increment. That is not incidental: the decay
// step (weight × 0.5^(age/half-life)) is not expressible as a simple column
// increment, and doing it in Go keeps the arithmetic identical to the pure
// HonorScore used by every read path.
func (r *GormUserRepository) ApplyHonorEvents(events map[uint]HonorEvent, now time.Time) (map[uint]HonorSnapshot, error) {
	snapshots := make(map[uint]HonorSnapshot, len(events))

	ids := make([]uint, 0, len(events))
	for id := range events {
		// The bot placeholder must never reach the users table.
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

			// Decay the stored weights forward to `now` FIRST, then add the new
			// event at full strength. Doing it in this order is what makes the
			// running weight exactly equal to the from-scratch sum of every
			// match's own decayed weight (Story 9.7 D3).
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

			// The weights are now current as of `now`, so the score is computed
			// against that same stamp (DecayFactor of a zero interval is 1.0).
			//
			// The stamp must never move BACKWARDS. DecayFactor already refuses to
			// inflate weights when now < decayedAt (clock skew between app
			// instances, an NTP step, or the 000017 backfill's Postgres NOW()
			// running ahead of this host), but writing the earlier `now` would
			// roll the reference back — and the NEXT write would then decay
			// across the skew interval a second time, silently over-decaying the
			// row toward the 80 prior and forgiving abandonments early. Clamp
			// forward instead (code review 2026-07-29).
			stamp := now
			if u.HonorDecayedAt != nil && stamp.Before(*u.HonorDecayedAt) {
				// Bounded by honorMaxClockSkew: clamping forward without a ceiling
				// traded the double-decay bug for an unbounded NO-decay window,
				// because DecayFactor returns 1.0 for any future stamp. One badly
				// skewed peer or a fat-fingered SQL fix could then freeze decay —
				// and therefore freeze forgiveness — indefinitely, silently.
				// Beyond the ceiling the stamp is treated as corrupt and reset to
				// now, which resumes decay; the loss is bounded and it is logged.
				// (Review pass 2.)
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
					// Refresh the denormalized filter-only snapshot column.
					// Nothing reads this back for display — see model.go.
					"honor_score": snapshot.Score,
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

// ResetHonor pardons one user: it clears the PENALTY but preserves the
// EXPERIENCE (Story 9.7 AC9 — operator forgiveness). Locked FOR UPDATE inside a
// transaction so it cannot interleave with a match-end ApplyHonorEvents and
// leave a half-reset row.
//
// honor_completed_total is deliberately left ALONE. It is the only input to
// IsNewPlayer, so zeroing it (as this originally did) turned a pardoned
// 500-match veteran into a "New Player": the profile hid their score behind the
// newcomer chip and invited them to "Play 5 matches to earn an honor score"
// while the untouched StatsGrid still displayed their real history, and Story
// 9.8's join gate — which reads isNewPlayer off the auth envelope — reclassified
// them as a newcomer rather than as a clean veteran. Found on review pass 2;
// fixed by PO decision 2026-07-29.
//
// Keeping a nonzero total alongside a zero weight is a state the model already
// supports and already means the right thing: a long-idle veteran's weights have
// decayed toward zero while their lifetime counts stand. A pardon simply puts
// them there deliberately.
//
// honor_decayed_at goes back to NULL, which DecayFactor reads as "never
// decayed" — the correct state for zero weights.
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
				// honor_completed_total intentionally NOT reset — see above.
				"honor_abandoned_total": 0,
				// Derived, not hard-coded 80, so a retune of the Beta prior
				// moves the reset target with it.
				"honor_score": HonorScore(0, 0, nil, time.Time{}),
			}).Error
	})
}
