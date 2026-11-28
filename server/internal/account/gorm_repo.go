package account

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/emilijan/beljot/server/internal/apperr"
)

const activeRoomQuery = `
SELECT EXISTS (
  SELECT 1 FROM room_players rp
  JOIN rooms r ON r.id = rp.room_id
  WHERE rp.user_id = ? AND r.status IN ('waiting', 'playing') AND r.deleted_at IS NULL
)`

const lockLiveUserQuery = `SELECT id, avatar_key FROM users WHERE id = ? AND deleted_at IS NULL FOR UPDATE`

var satelliteDeletes = []string{
	`DELETE FROM refresh_tokens WHERE user_id = ?`,
	`DELETE FROM password_reset_tokens WHERE user_id = ?`,
	`DELETE FROM user_identities WHERE user_id = ?`,
	`DELETE FROM friendships WHERE user_id = ? OR friend_id = ?`,
	`DELETE FROM player_seasons WHERE user_id = ?`,
	`DELETE FROM room_players WHERE user_id = ?`,
}

const renameOwnedRooms = `UPDATE rooms SET name = 'room-' || id, password_hash = NULL WHERE owner_id = ?`

const scrubUser = `
UPDATE users SET
  email = 'erased+' || id || '@invalid', username = 'erased_' || id, password_hash = '',
  avatar_key = NULL, language_preference = 'en', card_deck_preference = 'french',
  theme_preference = 'brass', sound_enabled = true, music_enabled = true,
  sound_volume = 70, music_volume = 70, allow_non_friend_invites = true,
  wallet_balance = 0, last_login_at = NULL, login_streak_days = 0, total_xp = 0,
  username_changed_at = NULL, honor_completed_weight = 0, honor_abandoned_weight = 0,
  honor_decayed_at = NULL, honor_completed_total = 0, honor_abandoned_total = 0,
  honor_score = 80, created_at = now(), updated_at = now(), deleted_at = now()
WHERE id = ?`

// GormRepository implements Repository with raw SQL, since erasure spans tables owned by several domains.
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository creates the GORM-backed account repository.
func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

// InActiveRoom reports whether the user holds a place in a waiting or playing room.
func (r *GormRepository) InActiveRoom(userID uint) (bool, error) {
	return inActiveRoom(r.db, userID)
}

func inActiveRoom(db *gorm.DB, userID uint) (bool, error) {
	var seated bool
	if err := db.Raw(activeRoomQuery, userID).Scan(&seated).Error; err != nil {
		return false, fmt.Errorf("checking active room for user %d: %w", userID, err)
	}
	return seated, nil
}

// Erase runs the whole anonymisation in one transaction, rechecking the room precondition under the user's row lock.
// It returns apperr.ErrUserNotFound for a missing or already erased user and apperr.ErrAccountDeleteInRoom while seated.
func (r *GormRepository) Erase(userID uint) (Erased, error) {
	var erased Erased
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var row struct {
			ID        uint
			AvatarKey *string
		}
		if err := tx.Raw(lockLiveUserQuery, userID).Scan(&row).Error; err != nil {
			return fmt.Errorf("locking user %d: %w", userID, err)
		}
		if row.ID == 0 {
			return apperr.ErrUserNotFound
		}
		seated, err := inActiveRoom(tx, userID)
		if err != nil {
			return err
		}
		if seated {
			return apperr.ErrAccountDeleteInRoom
		}
		for _, stmt := range satelliteDeletes {
			if err := tx.Exec(stmt, repeatID(stmt, userID)...).Error; err != nil {
				return fmt.Errorf("erasing user %d: %w", userID, err)
			}
		}
		if err := tx.Exec(renameOwnedRooms, userID).Error; err != nil {
			return fmt.Errorf("renaming rooms of user %d: %w", userID, err)
		}
		if err := tx.Exec(scrubUser, userID).Error; err != nil {
			return fmt.Errorf("scrubbing user %d: %w", userID, err)
		}
		erased.AvatarKey = row.AvatarKey
		return nil
	})
	if err != nil {
		return Erased{}, err
	}
	return erased, nil
}

// repeatID binds userID to every placeholder of stmt.
func repeatID(stmt string, userID uint) []any {
	args := []any{}
	for _, ch := range stmt {
		if ch == '?' {
			args = append(args, userID)
		}
	}
	return args
}
