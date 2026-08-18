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

// --- Integration tests (Postgres; skipped when the DB is unavailable) ---

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

	// Per-test transaction rolled back on cleanup — tests create their own data.
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

	// Found from the requester's perspective...
	got, err := repo.FindByPair(a.ID, b.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, a.ID, got.UserID)
	assert.Equal(t, b.ID, got.FriendID)
	assert.Equal(t, FriendStatusPending, got.Status)

	// ...and from the recipient's perspective (reversed args) — the same row.
	rev, err := repo.FindByPair(b.ID, a.ID)
	require.NoError(t, err)
	require.NotNil(t, rev)
	assert.Equal(t, got.ID, rev.ID)

	// An unrelated pair returns (nil, nil).
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

	// Same direction again → the unique index fires → mapped ErrFriendRequestExists.
	err := repo.Create(&Friendship{UserID: a.ID, FriendID: b.ID, Status: FriendStatusPending})
	assert.ErrorIs(t, err, apperr.ErrFriendRequestExists)
}

func TestGormRepository_ReverseDuplicate_BlockedByNormalizedIndex(t *testing.T) {
	// PINS the race backstop: the unique index is on the NORMALIZED (unordered)
	// pair, so B->A while A->B exists is refused at the DB level (23505) and
	// mapped to ErrFriendRequestExists. This closes the reverse-duplicate race the
	// handler's non-atomic FindByPair pre-check cannot on its own. If the reverse
	// insert ever succeeds again, the index regressed to a single direction and
	// the race is reopened.
	db := getTestDB(t)
	repo := NewGormRepository(db)
	a := makeUser(t, db, "rda@f.test")
	b := makeUser(t, db, "rdb@f.test")

	require.NoError(t, repo.Create(&Friendship{UserID: a.ID, FriendID: b.ID, Status: FriendStatusPending}))

	// FindByPair resolves the single relationship regardless of argument order.
	got, err := repo.FindByPair(a.ID, b.ID)
	require.NoError(t, err)
	require.NotNil(t, got)

	// The reverse row (B->A) is refused by the normalized unique index (23505 →
	// ErrFriendRequestExists), closing the race. Asserted LAST: the failing INSERT
	// aborts the surrounding per-test transaction, so no query may follow it here
	// (in production each Create is its own auto-commit statement — the handler
	// just returns the 409).
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

	// The requester cannot accept their own outgoing request → 0 rows.
	rows, err := repo.Accept(f.ID, requester.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rows)

	// The recipient flips it to accepted → 1 row.
	rows, err = repo.Accept(f.ID, recipient.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	got, err := repo.FindByID(f.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, FriendStatusAccepted, got.Status)

	// Accepting an already-accepted row is a no-op (no longer pending) → 0 rows.
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

	// The requester cannot decline their own outgoing request → 0 rows.
	rows, err := repo.Delete(f.ID, requester.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rows)

	// The recipient declines → 1 row, and the row is gone.
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

	// The requester unfriends → 1 row, and the row is gone.
	f := accepted()
	rows, err := repo.Unfriend(f.ID, requester.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows)
	got, err := repo.FindByID(f.ID)
	require.NoError(t, err)
	assert.Nil(t, got)

	// The recipient unfriends a fresh row just the same — the guard is
	// party-agnostic, unlike Accept/Delete.
	f = accepted()
	rows, err = repo.Unfriend(f.ID, recipient.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	// A second delete of the same id (the both-unfriend race) → 0 rows.
	rows, err = repo.Unfriend(f.ID, requester.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rows)

	// After the unfriend, a fresh request for the same pair — in the REVERSE
	// direction, no less — succeeds: the hard delete leaves no residue in the
	// normalized unique index, so the "send a new request later" promise holds.
	require.NoError(t, repo.Create(&Friendship{UserID: recipient.ID, FriendID: requester.ID, Status: FriendStatusPending}))
}

func TestGormRepository_Unfriend_GuardMisses(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	requester := makeUser(t, db, "ufga@f.test")
	recipient := makeUser(t, db, "ufgb@f.test")
	third := makeUser(t, db, "ufgc@f.test")

	// A pending row is decline's job, not unfriend's → 0 rows, row survives.
	f := &Friendship{UserID: requester.ID, FriendID: recipient.ID, Status: FriendStatusPending}
	require.NoError(t, repo.Create(f))
	rows, err := repo.Unfriend(f.ID, recipient.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rows)

	acceptRows, err := repo.Accept(f.ID, recipient.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), acceptRows)

	// A third party cannot unfriend an accepted row → 0 rows, row survives.
	rows, err = repo.Unfriend(f.ID, third.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rows)
	got, err := repo.FindByID(f.ID)
	require.NoError(t, err)
	require.NotNil(t, got)

	// A missing id → 0 rows.
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

	// a->b accepted (a is requester); c->a accepted (a is recipient).
	f1 := &Friendship{UserID: a.ID, FriendID: b.ID, Status: FriendStatusPending}
	require.NoError(t, repo.Create(f1))
	_, err := repo.Accept(f1.ID, b.ID)
	require.NoError(t, err)

	f2 := &Friendship{UserID: c.ID, FriendID: a.ID, Status: FriendStatusPending}
	require.NoError(t, repo.Create(f2))
	_, err = repo.Accept(f2.ID, a.ID)
	require.NoError(t, err)

	// A pending row for a must NOT appear in the accepted list.
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

	// x->me pending (incoming); me->y pending (outgoing, must NOT appear).
	require.NoError(t, repo.Create(&Friendship{UserID: x.ID, FriendID: me.ID, Status: FriendStatusPending}))
	require.NoError(t, repo.Create(&Friendship{UserID: me.ID, FriendID: y.ID, Status: FriendStatusPending}))

	incoming, err := repo.ListIncomingPending(me.ID)
	require.NoError(t, err)
	require.Len(t, incoming, 1)
	assert.Equal(t, x.ID, incoming[0].UserID)

	// x is only ever a requester here (x->me), so it has no incoming requests —
	// and gets a non-nil empty slice, not nil.
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

	// A pending relationship is not friendship.
	ok, err := repo.AreFriends(a.ID, b.ID)
	require.NoError(t, err)
	assert.False(t, ok)

	_, err = repo.Accept(f.ID, b.ID)
	require.NoError(t, err)

	// Accepted → friends, in either argument order.
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

	// user_id == friend_id violates chk_friendships_not_self. The repo only maps
	// 23505 to a friendly error, so a CHECK violation (23514) surfaces as a raw
	// error — the point is that the DB refuses the row (defense-in-depth behind
	// the handler's self guard).
	err := repo.Create(&Friendship{UserID: a.ID, FriendID: a.ID, Status: FriendStatusPending})
	assert.Error(t, err)
}
