package room_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/room"
	"github.com/emilijan/beljot/server/internal/user"
)

// Every roster-carrying payload in the room package carries the player's
// avatar URL beside their username: the RoomPlayer struct serializes it, and
// the hand-built WS maps have to add it by hand. These tests pin the hand-built
// ones, where a missing key would be invisible to every struct-based test.

const avatarP2 = "https://assets.test/avatars/22222222-2222-4222-8222-222222222222/128.webp"

// payloadMap decodes a broadcast message's payload into a generic map.
func payloadMap(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var p map[string]any
	require.NoError(t, json.Unmarshal(payloadOf(t, raw), &p))
	return p
}

// findBroadcast returns the payload of the first broadcast of msgType.
func findBroadcast(t *testing.T, b *mockBroadcaster, msgType string) map[string]any {
	t.Helper()
	for _, c := range b.calls {
		if msgTypeOf(t, c.msg) == msgType {
			return payloadMap(t, c.msg)
		}
	}
	for _, c := range b.allCalls {
		if msgTypeOf(t, c.msg) == msgType {
			return payloadMap(t, c.msg)
		}
	}
	t.Fatalf("no %s broadcast", msgType)
	return nil
}

func TestSelectSeat_SeatUpdatedCarriesAvatarURL(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()
	r := &room.Room{Name: "Avatar Seat", OwnerID: 100, Status: "waiting", PlayerCount: 2}
	require.NoError(t, repo.Create(r))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 100, Username: "P1"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 200, Username: "P2", AvatarURL: strPtr(avatarP2)}))

	rec := doSelectSeat(e, "1", `{"seat": 1}`, validToken(200))
	require.Equal(t, http.StatusOK, rec.Code)

	p := findBroadcast(t, broadcaster, "system:seat_updated")
	assert.Equal(t, avatarP2, p["avatarUrl"])

	// The HTTP roster is RoomPlayer-serialized: the avatar rides it too, and a
	// player without one carries an explicit null.
	var resp struct {
		Data struct {
			Players []map[string]any `json:"players"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	byUser := map[float64]map[string]any{}
	for _, pl := range resp.Data.Players {
		if id, ok := pl["userId"].(float64); ok && id != 0 {
			byUser[id] = pl
		}
	}
	assert.Equal(t, avatarP2, byUser[200]["avatarUrl"])
	require.Contains(t, byUser[100], "avatarUrl")
	assert.Nil(t, byUser[100]["avatarUrl"])
}

func TestJoinRoom_PlayerJoinedCarriesAvatarURL(t *testing.T) {
	t.Run("joiner without an avatar", func(t *testing.T) {
		e, repo, broadcaster := setupTestWithBroadcast()
		r := &room.Room{Name: "Avatar Join", OwnerID: 100, Status: "waiting", PlayerCount: 1}
		require.NoError(t, repo.Create(r))
		require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 100, Username: "Owner"}))

		require.Equal(t, http.StatusOK, doJoinRoom(e, "1", validToken(200)).Code)

		p := findBroadcast(t, broadcaster, "system:player_joined")
		require.Contains(t, p, "avatarUrl", "the key is always sent so the roster can draw the disc")
		assert.Nil(t, p["avatarUrl"])
	})

	t.Run("joiner with an avatar", func(t *testing.T) {
		e, repo, broadcaster := setupTestWithBroadcast()
		r := &room.Room{Name: "Avatar Join Pic", OwnerID: 100, Status: "waiting", PlayerCount: 1}
		require.NoError(t, repo.Create(r))
		require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 100, Username: "Owner"}))
		// The mock stores the handler's row as-is; the real JOIN fills the
		// avatar on the re-read, which this stands in for.
		repo.addPlayerHook = func(p *room.RoomPlayer) {
			if p.UserID == 200 {
				p.AvatarURL = strPtr(avatarP2)
			}
		}

		require.Equal(t, http.StatusOK, doJoinRoom(e, "1", validToken(200)).Code)

		p := findBroadcast(t, broadcaster, "system:player_joined")
		assert.Equal(t, avatarP2, p["avatarUrl"])
	})
}

func TestSwapSeats_SeatUpdatedCarriesAvatarURL(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()
	seedSeatedRoom(t, repo) // owner=100 seat 0, P2=200 seat 1, P3=300 seat 2, P4=400 seat 3
	for _, p := range repo.players {
		if p.UserID == 200 {
			p.AvatarURL = strPtr(avatarP2)
		}
	}

	rec := doSwapSeats(e, "1", `{"seatA": 1, "seatB": 2}`, validToken(100))
	require.Equal(t, http.StatusOK, rec.Code)

	seen := 0
	for _, c := range broadcaster.calls {
		if msgTypeOf(t, c.msg) != "system:seat_updated" {
			continue
		}
		p := payloadMap(t, c.msg)
		require.Contains(t, p, "avatarUrl")
		if p["userId"] == float64(200) {
			assert.Equal(t, avatarP2, p["avatarUrl"])
			seen++
		} else {
			assert.Nil(t, p["avatarUrl"])
		}
	}
	assert.Equal(t, 1, seen, "P2's move carries P2's avatar")
}

func TestLeaveSeat_SeatUpdatedCarriesAvatarURL(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()
	r := &room.Room{Name: "Avatar Leave Seat", OwnerID: 100, Status: "waiting", PlayerCount: 2}
	require.NoError(t, repo.Create(r))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 100, Username: "Owner", Seat: intPtr(0), Team: strPtr("teamA")}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 200, Username: "P2", Seat: intPtr(1), Team: strPtr("teamB"), AvatarURL: strPtr(avatarP2)}))

	rec := doLeaveSeat(e, "1", validToken(200))
	require.Equal(t, http.StatusOK, rec.Code)

	p := findBroadcast(t, broadcaster, "system:seat_updated")
	assert.Nil(t, p["seat"])
	assert.Equal(t, avatarP2, p["avatarUrl"])
}

func TestTransferOwnership_OwnerChangedCarriesNewOwnerAvatar(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()
	seedSeatedRoom(t, repo)
	repo.ownerUsernames[200] = "P2"
	repo.ownerAvatarURLs = map[uint]*string{200: strPtr(avatarP2)}

	rec := doTransferOwnership(e, "1", `{"userId": 200}`, validToken(100))
	require.Equal(t, http.StatusOK, rec.Code)

	p := findBroadcast(t, broadcaster, "system:room_owner_changed")
	assert.Equal(t, avatarP2, p["newOwnerAvatarUrl"])

	// The lobby card update reuses the same hydration.
	lobby := findBroadcast(t, broadcaster, "system:room_updated")
	assert.Equal(t, avatarP2, lobby["ownerAvatarUrl"])
	assert.Equal(t, "P2", lobby["ownerUsername"])
}

func TestCreateRoom_LifecyclePayloadCarriesOwnerAvatar(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()
	repo.ownerUsernames[100] = "Host"
	repo.ownerAvatarURLs = map[uint]*string{100: strPtr(avatarP2)}

	rec := doCreateRoom(e, `{"name": "Avatar Host Room", "variant": "bitola", "matchMode": "1001", "timerStyle": "relaxed"}`, validToken(100))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	p := findBroadcast(t, broadcaster, "system:room_created")
	assert.Equal(t, avatarP2, p["ownerAvatarUrl"])
}

func TestStartGame_SeatInfoCarriesAvatarURL(t *testing.T) {
	starter := &fakeMatchStarter{}
	e, repo := setupTestWithStarter(starter, &mockBroadcaster{})
	seedRoomWithPlayers(repo, "Avatar Start", 1, 1, 2, 3, 4)
	seats := []int{0, 1, 2, 3}
	teams := []string{"teamA", "teamB", "teamA", "teamB"}
	for i, p := range repo.players {
		p.Seat = intPtr(seats[i])
		p.Team = strPtr(teams[i])
		if p.UserID == 2 {
			p.AvatarURL = strPtr(avatarP2)
		}
	}

	require.Equal(t, http.StatusOK, doStartGame(e, "1", validToken(1)).Code)
	require.Equal(t, 1, starter.called)
	require.NotNil(t, starter.lastPlayers[1].AvatarURL)
	assert.Equal(t, avatarP2, *starter.lastPlayers[1].AvatarURL)
	assert.Nil(t, starter.lastPlayers[0].AvatarURL)
}

func TestListInvitableFriends_CarriesAvatarURL(t *testing.T) {
	h := setupInviteTest(nil)
	seedInviteRoom(t, h.repo, 100, "", 0, true)
	h.befriend(100, "owner", 200, "pictured")
	h.befriend(100, "owner", 201, "plain")
	h.friends.avatars = map[uint]*string{200: strPtr(avatarP2)}
	h.available(200, 201)

	rec := doListInvitable(h.e, "1", validToken(100))
	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	byID := map[float64]map[string]any{}
	for _, f := range resp.Data {
		byID[f["userId"].(float64)] = f
	}
	assert.Equal(t, avatarP2, byID[200]["avatarUrl"])
	require.Contains(t, byID[201], "avatarUrl")
	assert.Nil(t, byID[201]["avatarUrl"])
}

// The JOINs behind every roster read, and the owner hydration, turn the
// stored users.avatar_key into the public 128 URL (DB-backed).
func TestGormRepository_RosterReadsCarryAvatarURL(t *testing.T) {
	db := getRoomTestDB(t)
	repo := room.NewGormRepository(db)
	user.SetPublicAssetsURL("https://assets.test")
	t.Cleanup(func() { user.SetPublicAssetsURL("") })

	prefix := "avatars/66666666-6666-4666-8666-666666666666"
	want := "https://assets.test/" + prefix + "/128.webp"
	pictured := &user.User{Email: "pic@room-avatar.test", Username: "roompic", PasswordHash: "x", AvatarKey: &prefix}
	plain := &user.User{Email: "plain@room-avatar.test", Username: "roomplain", PasswordHash: "x"}
	require.NoError(t, db.Create(pictured).Error)
	require.NoError(t, db.Create(plain).Error)

	r := &room.Room{Name: "Avatar Roster Room", Code: "AVRST1", OwnerID: pictured.ID, Variant: "bitola", MatchMode: "1001", TimerStyle: "relaxed", Status: "waiting", PlayerCount: 2}
	require.NoError(t, repo.Create(r))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: pictured.ID, Seat: intPtr(0), Team: strPtr("teamA")}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: plain.ID, Seat: intPtr(1), Team: strPtr("teamB")}))

	assertRoster := func(t *testing.T, players []room.RoomPlayer) {
		t.Helper()
		require.Len(t, players, 2)
		for _, p := range players {
			if p.UserID == pictured.ID {
				require.NotNil(t, p.AvatarURL)
				assert.Equal(t, want, *p.AvatarURL)
			} else {
				assert.Nil(t, p.AvatarURL, "no avatar_key means null")
			}
		}
	}

	byRoom, err := repo.FindPlayersByRoomID(r.ID)
	require.NoError(t, err)
	assertRoster(t, byRoom)

	byRooms, err := repo.FindPlayersByRoomIDs([]uint{r.ID})
	require.NoError(t, err)
	assertRoster(t, byRooms[r.ID])

	seat0, err := repo.FindPlayerBySeat(r.ID, 0)
	require.NoError(t, err)
	require.NotNil(t, seat0)
	require.NotNil(t, seat0.AvatarURL)
	assert.Equal(t, want, *seat0.AvatarURL)

	seat1, err := repo.FindPlayerBySeat(r.ID, 1)
	require.NoError(t, err)
	require.NotNil(t, seat1)
	assert.Nil(t, seat1.AvatarURL)

	loaded, err := repo.FindByID(r.ID)
	require.NoError(t, err)
	require.NoError(t, repo.LoadOwnerUsernames([]*room.Room{loaded}))
	assert.Equal(t, "roompic", loaded.OwnerUsername)
	require.NotNil(t, loaded.OwnerAvatarURL)
	assert.Equal(t, want, *loaded.OwnerAvatarURL)
}

// Quick Play's own player_joined / seat_updated pair is the only source of a
// later joiner's picture in the matchmaking orbit, so both carry avatarUrl:
// the URL when the joiner has one, an explicit null when not.
func TestQuickPlay_SeatedBroadcastsCarryAvatarURL(t *testing.T) {
	for name, avatar := range map[string]*string{"with avatar": strPtr(avatarP2), "without": nil} {
		t.Run(name, func(t *testing.T) {
			e, repo, broadcaster := setupTestWithBroadcast()
			existing := &room.Room{
				Name: "Quick Play AVQP01", Code: "AVQP01", OwnerID: 20,
				Variant: "croatia", MatchMode: "501", TimerStyle: "relaxed",
				IsQuickPlay: true, Status: "waiting", PlayerCount: 1,
			}
			require.NoError(t, repo.Create(existing))
			require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: existing.ID, UserID: 20, Seat: intPtr(0), Team: strPtr("teamA")}))
			// Stands in for the JOIN the real re-read does.
			repo.addPlayerHook = func(p *room.RoomPlayer) {
				if p.UserID == 30 {
					p.AvatarURL = avatar
				}
			}

			require.Equal(t, http.StatusOK, doQuickPlay(e, validToken(30)).Code)

			for _, msgType := range []string{"system:player_joined", "system:seat_updated"} {
				p := findBroadcast(t, broadcaster, msgType)
				require.Equal(t, float64(30), p["userId"], msgType)
				require.Contains(t, p, "avatarUrl", msgType)
				if avatar == nil {
					assert.Nil(t, p["avatarUrl"], msgType)
				} else {
					assert.Equal(t, *avatar, p["avatarUrl"], msgType)
				}
			}
		})
	}
}
