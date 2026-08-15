package friend

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

func (r *GormRepository) Create(f *Friendship) error {
	if err := r.db.Create(f).Error; err != nil {
		// The unique index idx_friendships_pair is on the NORMALIZED pair
		// (LEAST/GREATEST), so it fires here for a duplicate in EITHER direction —
		// including the reverse-duplicate race the handler's non-atomic FindByPair
		// pre-check cannot close on its own. Both map to ErrFriendRequestExists.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperr.ErrFriendRequestExists
		}
		return err
	}
	return nil
}

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

// Accept flips a pending row to accepted in a single conditional UPDATE — the
// recipient-only guard (friend_id = recipientID) and the pending precondition
// live in the WHERE clause, so enforcement is atomic and a double-accept race
// cannot both win (modeled on user.UpdateUsername). GORM stamps updated_at
// automatically on the Update. rows==0 means: not the recipient, already
// accepted, or the row is gone — the handler maps all three to a uniform 404.
func (r *GormRepository) Accept(id, recipientID uint) (int64, error) {
	res := r.db.Model(&Friendship{}).
		Where("id = ? AND friend_id = ? AND status = ?", id, recipientID, FriendStatusPending).
		Update("status", FriendStatusAccepted)
	return res.RowsAffected, res.Error
}

// Delete removes a pending row under the same recipient-only guard (decline).
func (r *GormRepository) Delete(id, recipientID uint) (int64, error) {
	res := r.db.
		Where("id = ? AND friend_id = ? AND status = ?", id, recipientID, FriendStatusPending).
		Delete(&Friendship{})
	return res.RowsAffected, res.Error
}

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
