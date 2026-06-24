package room_test

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/emilijan/beljot/server/internal/auth"
	"github.com/emilijan/beljot/server/internal/room"
	"github.com/emilijan/beljot/server/internal/user"
)

// seedPrivateRoom inserts a waiting private room (owner = ownerID) with the given
// plaintext password bcrypt-hashed into PasswordHash, mirroring how CreateRoom
// stores it. Direct mock seeding (not via the handler) so join/edit tests start
// from a known private room without driving the create path.
func seedPrivateRoom(t *testing.T, repo *mockRoomRepo, name, code string, ownerID uint, password string) *room.Room {
	t.Helper()
	hash, err := auth.HashPassword(password)
	require.NoError(t, err)
	r := &room.Room{
		Name:         name,
		Code:         code,
		OwnerID:      ownerID,
		Variant:      "bitola",
		MatchMode:    "1001",
		TimerStyle:   "relaxed",
		Status:       "waiting",
		PlayerCount:  1,
		PasswordHash: &hash,
	}
	r.ID = repo.nextID
	r.CreatedAt = time.Now()
	r.UpdatedAt = time.Now()
	repo.nextID++
	repo.rooms = append(repo.rooms, r)
	return r
}

// --- Create-room privacy (AC1, AC2) ---

func TestCreateRoom_Private_HashesPersistsAndHidesHash(t *testing.T) {
	e, repo := setupTest()
	body := `{"name":"Friends Only","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","isPrivate":true,"password":"hunter2"}`
	rec := doCreateRoom(e, body, validToken(5))
	require.Equal(t, http.StatusCreated, rec.Code)

	// The hash must never appear in the JSON response (json:"-").
	raw := rec.Body.String()
	assert.NotContains(t, raw, "passwordHash")
	assert.NotContains(t, raw, "password_hash")
	assert.NotContains(t, raw, "hunter2", "plaintext password must never be serialized")

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var data room.Room
	require.NoError(t, json.Unmarshal(resp["data"], &data))
	assert.True(t, data.IsPrivate, "response carries derived isPrivate=true")

	persisted, _ := repo.FindByID(data.ID)
	require.NotNil(t, persisted)
	require.NotNil(t, persisted.PasswordHash, "private room persists a non-nil password_hash")
	assert.NotEqual(t, "hunter2", *persisted.PasswordHash, "stored value is a hash, not plaintext")
	assert.NoError(t, auth.CheckPassword(*persisted.PasswordHash, "hunter2"), "stored hash verifies the password")
}

func TestCreateRoom_Private_MissingPassword(t *testing.T) {
	e, repo := setupTest()
	body := `{"name":"No Pass","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","isPrivate":true}`
	rec := doCreateRoom(e, body, validToken(5))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "ROOM_PASSWORD_REQUIRED", errCodeOf(t, rec))

	persisted, _ := repo.FindByID(1)
	assert.Nil(t, persisted, "no room created when the private password is missing")
}

func TestCreateRoom_Private_ShortPassword(t *testing.T) {
	e, _ := setupTest()
	body := `{"name":"Too Short","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","isPrivate":true,"password":"ab"}`
	rec := doCreateRoom(e, body, validToken(5))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "ROOM_PASSWORD_TOO_SHORT", errCodeOf(t, rec))
}

func TestCreateRoom_Private_ShortMultibytePassword(t *testing.T) {
	e, _ := setupTest()
	// "аб" is 2 Cyrillic runes but 4 bytes — the minimum counts runes ("characters"),
	// so this is rejected as too short. A byte-count check would wrongly accept it.
	// Story 9.6 review patch (server/client length-unit alignment).
	body := `{"name":"Multibyte Short","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","isPrivate":true,"password":"аб"}`
	rec := doCreateRoom(e, body, validToken(5))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "ROOM_PASSWORD_TOO_SHORT", errCodeOf(t, rec))
}

func TestCreateRoom_Private_TooLongPassword(t *testing.T) {
	e, _ := setupTest()
	longPw := strings.Repeat("x", 73) // 73 bytes > bcrypt's 72-byte limit
	body := `{"name":"Too Long","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","isPrivate":true,"password":"` + longPw + `"}`
	rec := doCreateRoom(e, body, validToken(5))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "ROOM_PASSWORD_TOO_LONG", errCodeOf(t, rec))
}

func TestCreateRoom_Public_NoPasswordHash(t *testing.T) {
	e, repo := setupTest()
	// isPrivate omitted (false) — even with a stray password it stays public.
	body := `{"name":"Open Table","variant":"bitola","matchMode":"1001","timerStyle":"relaxed"}`
	rec := doCreateRoom(e, body, validToken(5))
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var data room.Room
	require.NoError(t, json.Unmarshal(resp["data"], &data))
	assert.False(t, data.IsPrivate)

	persisted, _ := repo.FindByID(data.ID)
	require.NotNil(t, persisted)
	assert.Nil(t, persisted.PasswordHash, "public room has a nil password_hash")
}

// --- Join gate (AC4, AC5) ---

func TestJoinRoom_Private_CorrectPasswordSeats(t *testing.T) {
	e, repo := setupTest()
	seedPrivateRoom(t, repo, "Private Join", "PRVJ01", 1, "letmein")

	rec := doPostJSON(e, "/api/v1/rooms/1/join", validToken(10), `{"password":"letmein"}`)
	require.Equal(t, http.StatusOK, rec.Code)

	players, _ := repo.FindPlayersByRoomID(1)
	var joined bool
	for _, p := range players {
		if p.UserID == 10 {
			joined = true
		}
	}
	assert.True(t, joined, "correct password seats the joiner")
}

func TestJoinRoom_Private_WrongPassword(t *testing.T) {
	e, repo := setupTest()
	seedPrivateRoom(t, repo, "Private Wrong", "PRVW01", 1, "letmein")

	rec := doPostJSON(e, "/api/v1/rooms/1/join", validToken(10), `{"password":"nope"}`)
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "WRONG_ROOM_PASSWORD", errCodeOf(t, rec))

	players, _ := repo.FindPlayersByRoomID(1)
	assert.Empty(t, players, "a wrong password must not seat the joiner")
}

func TestJoinRoom_Private_MissingPasswordRejected(t *testing.T) {
	e, repo := setupTest()
	seedPrivateRoom(t, repo, "Private Missing", "PRVM01", 1, "letmein")

	// No body at all (doJoinRoom sends nil) — missing == wrong, indistinguishable.
	rec := doJoinRoom(e, "1", validToken(10))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "WRONG_ROOM_PASSWORD", errCodeOf(t, rec))
}

func TestJoinRoom_Public_NoBodySucceeds(t *testing.T) {
	// AC5 regression guard: adding the optional password bind must not break the
	// public-room join, which sends no body.
	e, repo := setupTest()
	seedRoom(repo, "Public Join", "waiting")

	rec := doJoinRoom(e, "1", validToken(10))
	require.Equal(t, http.StatusOK, rec.Code)

	players, _ := repo.FindPlayersByRoomID(1)
	require.Len(t, players, 1)
	assert.Equal(t, uint(10), players[0].UserID)
}

// --- Owner privacy edit (AC6) ---

func TestUpdateRoomPrivacy_OwnerSetsPassword(t *testing.T) {
	e, repo := setupTest()
	seedRoom(repo, "Becoming Private", "waiting") // owner = 1, public

	rec := doPostJSON(e, "/api/v1/rooms/1/privacy", validToken(1), `{"isPrivate":true,"password":"sesame"}`)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var data room.Room
	require.NoError(t, json.Unmarshal(resp["data"], &data))
	assert.True(t, data.IsPrivate)

	persisted, _ := repo.FindByID(1)
	require.NotNil(t, persisted.PasswordHash)
	assert.NoError(t, auth.CheckPassword(*persisted.PasswordHash, "sesame"))
}

func TestUpdateRoomPrivacy_OwnerChangesPassword(t *testing.T) {
	e, repo := setupTest()
	seedPrivateRoom(t, repo, "Change Pass", "CHGP01", 1, "old-one")

	rec := doPostJSON(e, "/api/v1/rooms/1/privacy", validToken(1), `{"isPrivate":true,"password":"new-one"}`)
	require.Equal(t, http.StatusOK, rec.Code)

	persisted, _ := repo.FindByID(1)
	require.NotNil(t, persisted.PasswordHash)
	assert.Error(t, auth.CheckPassword(*persisted.PasswordHash, "old-one"), "old password no longer valid")
	assert.NoError(t, auth.CheckPassword(*persisted.PasswordHash, "new-one"), "new password takes effect")
}

func TestUpdateRoomPrivacy_OwnerClearsToPublic(t *testing.T) {
	e, repo := setupTest()
	seedPrivateRoom(t, repo, "Going Public", "GOPB01", 1, "secret")

	rec := doPostJSON(e, "/api/v1/rooms/1/privacy", validToken(1), `{"isPrivate":false}`)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var data room.Room
	require.NoError(t, json.Unmarshal(resp["data"], &data))
	assert.False(t, data.IsPrivate)

	persisted, _ := repo.FindByID(1)
	assert.Nil(t, persisted.PasswordHash, "clearing privacy nulls the hash")
}

func TestUpdateRoomPrivacy_SetMissingPasswordRejected(t *testing.T) {
	e, repo := setupTest()
	seedRoom(repo, "No New Pass", "waiting")

	rec := doPostJSON(e, "/api/v1/rooms/1/privacy", validToken(1), `{"isPrivate":true}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "ROOM_PASSWORD_REQUIRED", errCodeOf(t, rec))

	persisted, _ := repo.FindByID(1)
	assert.Nil(t, persisted.PasswordHash, "room stays public when the new password is missing")
}

func TestUpdateRoomPrivacy_NonOwnerRejected(t *testing.T) {
	e, repo := setupTest()
	seedRoom(repo, "Not Yours", "waiting") // owner = 1

	rec := doPostJSON(e, "/api/v1/rooms/1/privacy", validToken(99), `{"isPrivate":true,"password":"sesame"}`)
	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "NOT_ROOM_OWNER", errCodeOf(t, rec))
}

func TestUpdateRoomPrivacy_NonWaitingRejected(t *testing.T) {
	e, repo := setupTest()
	seedRoom(repo, "Already Playing", "playing") // owner = 1

	rec := doPostJSON(e, "/api/v1/rooms/1/privacy", validToken(1), `{"isPrivate":true,"password":"sesame"}`)
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "ROOM_NOT_WAITING", errCodeOf(t, rec))
}

func TestUpdateRoomPrivacy_DoesNotEjectSeatedPlayers(t *testing.T) {
	e, repo := setupTest()
	// Owner 1 + two seated members; make it private first so clearing exercises
	// the no-ejection path on a state change.
	r := seedRoomWithPlayers(repo, "Keep Seats", 1, 1, 10, 20)
	hash, err := auth.HashPassword("secret")
	require.NoError(t, err)
	r.PasswordHash = &hash

	before, _ := repo.FindPlayersByRoomID(r.ID)
	require.Len(t, before, 3)

	rec := doPostJSON(e, "/api/v1/rooms/1/privacy", validToken(1), `{"isPrivate":false}`)
	require.Equal(t, http.StatusOK, rec.Code)

	after, _ := repo.FindPlayersByRoomID(r.ID)
	assert.Len(t, after, 3, "changing privacy must not eject seated players")
}

func TestUpdateRoomPrivacy_BroadcastsRoomUpdated(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()
	seedRoom(repo, "Broadcast Me", "waiting") // owner = 1

	rec := doPostJSON(e, "/api/v1/rooms/1/privacy", validToken(1), `{"isPrivate":true,"password":"sesame"}`)
	require.Equal(t, http.StatusOK, rec.Code)

	var found bool
	for _, c := range broadcaster.allCalls {
		if msgTypeOf(t, c.msg) != "system:room_updated" {
			continue
		}
		var payload map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(payloadOf(t, c.msg), &payload))
		var isPrivate bool
		require.NoError(t, json.Unmarshal(payload["isPrivate"], &isPrivate))
		if isPrivate {
			found = true
		}
	}
	assert.True(t, found, "owner privacy edit broadcasts system:room_updated with isPrivate=true")
}

func TestUpdateRoomPrivacy_QuickPlayRoomRejected(t *testing.T) {
	e, repo := setupTest()
	// A quick-play room owned by user 1. Privatizing it must be rejected
	// server-side (AC7 invariant) even though the owner+waiting checks pass —
	// the UI hides this control for quick-play rooms, so this guards a direct call.
	r := &room.Room{
		Name:        "Quick Play ABC123",
		Code:        "QPABC1",
		OwnerID:     1,
		Variant:     "bitola",
		MatchMode:   "1001",
		TimerStyle:  "per-move",
		Status:      "waiting",
		PlayerCount: 1,
		IsQuickPlay: true,
	}
	r.ID = repo.nextID
	r.CreatedAt = time.Now()
	r.UpdatedAt = time.Now()
	repo.nextID++
	repo.rooms = append(repo.rooms, r)

	rec := doPostJSON(e, "/api/v1/rooms/1/privacy", validToken(1), `{"isPrivate":true,"password":"sesame"}`)
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "QUICK_PLAY_ROOM_PRIVACY", errCodeOf(t, rec))

	persisted, _ := repo.FindByID(r.ID)
	assert.Nil(t, persisted.PasswordHash, "quick-play room stays public")
}

// --- Quick Play stays public (AC7) ---

func TestQuickPlay_SynthesizedRoomStaysPublic(t *testing.T) {
	e, repo := setupCoinTest(nil, &stubWallet{balance: 5000})

	rec := doQuickPlay(e, validToken(7))
	require.Equal(t, http.StatusOK, rec.Code)

	persisted, _ := repo.FindByID(1)
	require.NotNil(t, persisted, "quick play synthesizes a room")
	assert.True(t, persisted.IsQuickPlay)
	assert.Nil(t, persisted.PasswordHash, "quick-play rooms are never private (AC7)")
}

// --- AfterFind derivation (AC2/AC3, DB-backed) ---

// getRoomTestDB opens a per-test transaction against the dev DB (BELJOT_DB_URL,
// default port 5433) and rolls it back on cleanup. Skips when no DB is reachable
// — mirrors the wallet/user integration-test convention.
func getRoomTestDB(t *testing.T) *gorm.DB {
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

func TestRoom_AfterFindDerivesIsPrivate(t *testing.T) {
	db := getRoomTestDB(t)
	repo := room.NewGormRepository(db)

	owner := &user.User{Email: "afterfind@room.test", Username: "afindowner", PasswordHash: "x"}
	require.NoError(t, db.Create(owner).Error)

	hash, err := auth.HashPassword("secret")
	require.NoError(t, err)
	priv := &room.Room{Name: "AfterFind Private", Code: "AFPRV1", OwnerID: owner.ID, Variant: "bitola", MatchMode: "1001", TimerStyle: "relaxed", Status: "waiting", PlayerCount: 1, PasswordHash: &hash}
	pub := &room.Room{Name: "AfterFind Public", Code: "AFPUB1", OwnerID: owner.ID, Variant: "bitola", MatchMode: "1001", TimerStyle: "relaxed", Status: "waiting", PlayerCount: 1}
	require.NoError(t, repo.Create(priv))
	require.NoError(t, repo.Create(pub))

	gotPriv, err := repo.FindByID(priv.ID)
	require.NoError(t, err)
	require.NotNil(t, gotPriv)
	assert.True(t, gotPriv.IsPrivate, "AfterFind derives isPrivate=true from a non-nil hash")
	require.NotNil(t, gotPriv.PasswordHash)
	assert.NoError(t, auth.CheckPassword(*gotPriv.PasswordHash, "secret"), "hash round-trips through the DB")

	gotPub, err := repo.FindByID(pub.ID)
	require.NoError(t, err)
	require.NotNil(t, gotPub)
	assert.False(t, gotPub.IsPrivate, "AfterFind derives isPrivate=false from a NULL hash")
}
