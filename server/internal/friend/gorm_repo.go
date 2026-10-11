package friend

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/emilijan/beljot/server/internal/apperr"
)

const uniqueViolation = "23505"

// GormRepository is the Postgres-backed friendship Repository.
type GormRepository struct {
	db *gorm.DB
}

func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

// Create inserts a pending friendship. The unique index covers the normalized pair, so
// a duplicate in either direction returns apperr.ErrFriendRequestExists.
func (r *GormRepository) Create(f *Friendship) error {
	if err := r.db.Create(f).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return apperr.ErrFriendRequestExists
		}
		return err
	}
	return nil
}

// FindByID returns the row with the given id, or (nil, nil) when none match.
func (r *GormRepository) FindByID(id uint) (*Friendship, error) {
	var f Friendship
	if err := r.db.First(&f, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &f, nil
}

// FindByPair returns the friendship between a and b in either direction,
// or (nil, nil) when none exists.
func (r *GormRepository) FindByPair(a, b uint) (*Friendship, error) {
	var f Friendship
	err := r.db.
		Where("(user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)", a, b, b, a).
		First(&f).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &f, nil
}

// Accept atomically flips a pending row to accepted when recipientID is its recipient.
// It returns rows affected: 0 means not the recipient, already accepted, or missing.
func (r *GormRepository) Accept(id, recipientID uint) (int64, error) {
	res := r.db.Model(&Friendship{}).
		Where("id = ? AND friend_id = ? AND status = ?", id, recipientID, FriendStatusPending).
		Update("status", FriendStatusAccepted)
	return res.RowsAffected, res.Error
}

// Delete declines a pending row under the same recipient-only guard as Accept
// and returns rows affected.
func (r *GormRepository) Delete(id, recipientID uint) (int64, error) {
	res := r.db.
		Where("id = ? AND friend_id = ? AND status = ?", id, recipientID, FriendStatusPending).
		Delete(&Friendship{})
	return res.RowsAffected, res.Error
}

// Unfriend atomically removes an accepted row when userID is either party.
// It returns rows affected: 0 means not a party, not accepted, or missing.
func (r *GormRepository) Unfriend(id, userID uint) (int64, error) {
	res := r.db.
		Where("id = ? AND (user_id = ? OR friend_id = ?) AND status = ?", id, userID, userID, FriendStatusAccepted).
		Delete(&Friendship{})
	return res.RowsAffected, res.Error
}

// ListAccepted returns the user's accepted friendships in either direction,
// as a non-nil slice.
func (r *GormRepository) ListAccepted(userID uint) ([]Friendship, error) {
	rows := []Friendship{}
	if err := r.db.
		Where("(user_id = ? OR friend_id = ?) AND status = ?", userID, userID, FriendStatusAccepted).
		Order("updated_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListIncomingPending returns the pending requests sent to the user, newest first,
// as a non-nil slice.
func (r *GormRepository) ListIncomingPending(userID uint) ([]Friendship, error) {
	rows := []Friendship{}
	if err := r.db.
		Where("friend_id = ? AND status = ?", userID, FriendStatusPending).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// AreFriends reports whether a and b have an accepted friendship in either direction.
func (r *GormRepository) AreFriends(a, b uint) (bool, error) {
	var count int64
	if err := r.db.Model(&Friendship{}).
		Where("((user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)) AND status = ?",
			a, b, b, a, FriendStatusAccepted).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
