package friend

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/user"
)

func getTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("BELJOT_DB_URL")
	if dsn == "" {
		dsn = "postgres://beljot:beljot_dev_password@localhost:5433/beljot?sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skip("skipping integration test: database not available")
	}

	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	return tx
}

func makeUser(t *testing.T, db *gorm.DB, email string) *user.User {
	t.Helper()
	u := &user.User{
		Email:              email,
		Username:           email[:min(len(email), 12)],
		PasswordHash:       "x",
		LanguagePreference: "en",
	}
	require.NoError(t, db.Create(u).Error)
	return u
}

func TestGormRepository_CreateAndFindByPair_BothDirections(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	a := makeUser(t, db, "a@f.test")
	b := makeUser(t, db, "b@f.test")

	require.NoError(t, repo.Create(&Friendship{UserID: a.ID, FriendID: b.ID, Status: FriendStatusPending}))

	got, err := repo.FindByPair(a.ID, b.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, a.ID, got.UserID)
	assert.Equal(t, b.ID, got.FriendID)
	assert.Equal(t, FriendStatusPending, got.Status)

	rev, err := repo.FindByPair(b.ID, a.ID)
	require.NoError(t, err)
	require.NotNil(t, rev)
	assert.Equal(t, got.ID, rev.ID)

	c := makeUser(t, db, "c@f.test")
	none, err := repo.FindByPair(a.ID, c.ID)
	require.NoError(t, err)
	assert.Nil(t, none)
}

func TestGormRepository_Create_ExactDuplicateConflicts(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	a := makeUser(t, db, "dupa@f.test")
	b := makeUser(t, db, "dupb@f.test")

	require.NoError(t, repo.Create(&Friendship{UserID: a.ID, FriendID: b.ID, Status: FriendStatusPending}))

	err := repo.Create(&Friendship{UserID: a.ID, FriendID: b.ID, Status: FriendStatusPending})
	assert.ErrorIs(t, err, apperr.ErrFriendRequestExists)
}

func TestGormRepository_ReverseDuplicate_BlockedByNormalizedIndex(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	a := makeUser(t, db, "rda@f.test")
	b := makeUser(t, db, "rdb@f.test")

	require.NoError(t, repo.Create(&Friendship{UserID: a.ID, FriendID: b.ID, Status: FriendStatusPending}))

	got, err := repo.FindByPair(a.ID, b.ID)
	require.NoError(t, err)
	require.NotNil(t, got)

	err = repo.Create(&Friendship{UserID: b.ID, FriendID: a.ID, Status: FriendStatusPending})
	assert.ErrorIs(t, err, apperr.ErrFriendRequestExists)
}

func TestGormRepository_Accept_RecipientOnlyAndPending(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	requester := makeUser(t, db, "areq@f.test")
	recipient := makeUser(t, db, "arec@f.test")
	f := &Friendship{UserID: requester.ID, FriendID: recipient.ID, Status: FriendStatusPending}
	require.NoError(t, repo.Create(f))

	rows, err := repo.Accept(f.ID, requester.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rows)

	rows, err = repo.Accept(f.ID, recipient.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	got, err := repo.FindByID(f.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, FriendStatusAccepted, got.Status)

	rows, err = repo.Accept(f.ID, recipient.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rows)
}

func TestGormRepository_Delete_RecipientOnly(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	requester := makeUser(t, db, "dreq@f.test")
	recipient := makeUser(t, db, "drec@f.test")
	f := &Friendship{UserID: requester.ID, FriendID: recipient.ID, Status: FriendStatusPending}
	require.NoError(t, repo.Create(f))

	rows, err := repo.Delete(f.ID, requester.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rows)

	rows, err = repo.Delete(f.ID, recipient.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	got, err := repo.FindByID(f.ID)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestGormRepository_Unfriend_EitherParty(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	requester := makeUser(t, db, "ufreq@f.test")
	recipient := makeUser(t, db, "ufrec@f.test")

	accepted := func() *Friendship {
		f := &Friendship{UserID: requester.ID, FriendID: recipient.ID, Status: FriendStatusPending}
		require.NoError(t, repo.Create(f))
		rows, err := repo.Accept(f.ID, recipient.ID)
		require.NoError(t, err)
		require.Equal(t, int64(1), rows)
		return f
	}

	f := accepted()
	rows, err := repo.Unfriend(f.ID, requester.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows)
	got, err := repo.FindByID(f.ID)
	require.NoError(t, err)
	assert.Nil(t, got)

	f = accepted()
	rows, err = repo.Unfriend(f.ID, recipient.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	rows, err = repo.Unfriend(f.ID, requester.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rows)

	require.NoError(t, repo.Create(&Friendship{UserID: recipient.ID, FriendID: requester.ID, Status: FriendStatusPending}))
}

func TestGormRepository_Unfriend_GuardMisses(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	requester := makeUser(t, db, "ufga@f.test")
	recipient := makeUser(t, db, "ufgb@f.test")
	third := makeUser(t, db, "ufgc@f.test")

	f := &Friendship{UserID: requester.ID, FriendID: recipient.ID, Status: FriendStatusPending}
	require.NoError(t, repo.Create(f))
	rows, err := repo.Unfriend(f.ID, recipient.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rows)

	acceptRows, err := repo.Accept(f.ID, recipient.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), acceptRows)

	rows, err = repo.Unfriend(f.ID, third.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rows)
	got, err := repo.FindByID(f.ID)
	require.NoError(t, err)
	require.NotNil(t, got)

	rows, err = repo.Unfriend(f.ID+9999, requester.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rows)
}

func TestGormRepository_ListAccepted_Symmetric(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	a := makeUser(t, db, "lsa@f.test")
	b := makeUser(t, db, "lsb@f.test")
	c := makeUser(t, db, "lsc@f.test")
	d := makeUser(t, db, "lsd@f.test")

	f1 := &Friendship{UserID: a.ID, FriendID: b.ID, Status: FriendStatusPending}
	require.NoError(t, repo.Create(f1))
	_, err := repo.Accept(f1.ID, b.ID)
	require.NoError(t, err)

	f2 := &Friendship{UserID: c.ID, FriendID: a.ID, Status: FriendStatusPending}
	require.NoError(t, repo.Create(f2))
	_, err = repo.Accept(f2.ID, a.ID)
	require.NoError(t, err)

	require.NoError(t, repo.Create(&Friendship{UserID: a.ID, FriendID: d.ID, Status: FriendStatusPending}))

	friends, err := repo.ListAccepted(a.ID)
	require.NoError(t, err)
	require.Len(t, friends, 2)

	others := map[uint]bool{}
	for _, f := range friends {
		assert.Equal(t, FriendStatusAccepted, f.Status)
		other := f.UserID
		if other == a.ID {
			other = f.FriendID
		}
		others[other] = true
	}
	assert.True(t, others[b.ID], "friend where a was the requester")
	assert.True(t, others[c.ID], "friend where a was the recipient")
}

func TestGormRepository_ListIncomingPending(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	me := makeUser(t, db, "lipme@f.test")
	x := makeUser(t, db, "lipx@f.test")
	y := makeUser(t, db, "lipy@f.test")

	require.NoError(t, repo.Create(&Friendship{UserID: x.ID, FriendID: me.ID, Status: FriendStatusPending}))
	require.NoError(t, repo.Create(&Friendship{UserID: me.ID, FriendID: y.ID, Status: FriendStatusPending}))

	incoming, err := repo.ListIncomingPending(me.ID)
	require.NoError(t, err)
	require.Len(t, incoming, 1)
	assert.Equal(t, x.ID, incoming[0].UserID)

	empty, err := repo.ListIncomingPending(x.ID)
	require.NoError(t, err)
	require.NotNil(t, empty)
	assert.Len(t, empty, 0)
}

func TestGormRepository_AreFriends(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	a := makeUser(t, db, "afa@f.test")
	b := makeUser(t, db, "afb@f.test")

	f := &Friendship{UserID: a.ID, FriendID: b.ID, Status: FriendStatusPending}
	require.NoError(t, repo.Create(f))

	ok, err := repo.AreFriends(a.ID, b.ID)
	require.NoError(t, err)
	assert.False(t, ok)

	_, err = repo.Accept(f.ID, b.ID)
	require.NoError(t, err)

	ok, err = repo.AreFriends(a.ID, b.ID)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = repo.AreFriends(b.ID, a.ID)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestGormRepository_SelfRowRejectedByCheck(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	a := makeUser(t, db, "selfrow@f.test")

	err := repo.Create(&Friendship{UserID: a.ID, FriendID: a.ID, Status: FriendStatusPending})
	assert.Error(t, err)
}
