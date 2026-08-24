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

// The room-level declarations toggle: request shape, defaulting, persistence,
// what reaches the session manager, and what the lobby is told.

// The default must be ON. This is the single most important assertion in the
// file: the request field is a *bool precisely so an omitted key cannot be read
// as "off", and a plain bool here would silently strip melds and Belote from
// every room created by a client that has not shipped the toggle yet.
func TestCreateRoom_DeclarationsDefaultToEnabled(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "field omitted entirely",
			body: `{"name":"Old Client","variant":"bitola","matchMode":"1001","timerStyle":"relaxed"}`,
		},
		{
			name: "field sent as null",
			body: `{"name":"Null Field","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","declarationsEnabled":null}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, repo := setupTest()

			rec := doCreateRoom(e, tt.body, validToken(5))
			require.Equal(t, http.StatusCreated, rec.Code)

			persisted, perr := repo.FindByID(1)
			require.NoError(t, perr)
			require.NotNil(t, persisted)
			assert.True(t, persisted.DeclarationsEnabled,
				"an absent setting must mean declarations ON, never the boolean zero value")
		})
	}
}

// An explicit false must survive all the way to the persisted row. The GORM
// `default` tag omitted on room.Room is what makes this possible; if someone
// adds one back, this test is what catches it.
func TestCreateRoom_DeclarationsDisabledSurvives(t *testing.T) {
	tests := []struct {
		name    string
		variant string
	}{
		{"bitola", "bitola"},
		{"croatia", "croatia"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, repo := setupTest()
			body := `{"name":"Bez Zvanja","variant":"` + tt.variant +
				`","matchMode":"1001","timerStyle":"relaxed","declarationsEnabled":false}`

			rec := doCreateRoom(e, body, validToken(5))
			require.Equal(t, http.StatusCreated, rec.Code)

			persisted, perr := repo.FindByID(1)
			require.NoError(t, perr)
			require.NotNil(t, persisted)
			assert.False(t, persisted.DeclarationsEnabled,
				"an explicit false must survive, not flip to the default")
			assert.Equal(t, tt.variant, persisted.Variant,
				"the toggle is variant-independent — it must not disturb the variant")
		})
	}
}

// The created-room response is what the modal reads back, so the field has to be
// on the wire and not merely in the database.
func TestCreateRoom_DeclarationsOnTheResponse(t *testing.T) {
	e, _ := setupTest()
	body := `{"name":"Bez Zvanja","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","declarationsEnabled":false}`

	rec := doCreateRoom(e, body, validToken(5))
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp struct {
		Data struct {
			DeclarationsEnabled *bool `json:"declarationsEnabled"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotNil(t, resp.Data.DeclarationsEnabled,
		"the key must be present — the client compares against false, so a missing key reads as ON")
	assert.False(t, *resp.Data.DeclarationsEnabled)
}

// Quick Play offers no rule choices, and its room is hand-built rather than
// created through CreateRoom — the exact shape that has hidden a missing field
// before. It must come out declarations-ON.
func TestQuickPlay_SynthesizedRoomKeepsDeclarations(t *testing.T) {
	e, repo := setupTest()

	rec := doQuickPlay(e, validToken(5))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	persisted, perr := repo.FindByID(1)
	require.NoError(t, perr)
	require.NotNil(t, persisted)
	require.True(t, persisted.IsQuickPlay, "this test is meaningless unless a quick-play room was synthesized")
	assert.True(t, persisted.DeclarationsEnabled,
		"a hand-built &Room{} that forgets the field inserts false — quick play must be explicit")
}

// The value the session manager receives is the one the engine will play by, and
// nothing downstream can validate it: false is legal, so a room that persisted
// ON but handed the manager OFF would play a silently meldless match and look
// entirely healthy. Deliberately shaped after
// TestStartGame_PassesTheRoomsVariantToTheSessionManager, which exists for the
// same reason about the variant string.
func TestStartGame_PassesTheRoomsDeclarationSettingToTheSessionManager(t *testing.T) {
	for _, want := range []bool{true, false} {
		name := "declarations on"
		if !want {
			name = "declarations off"
		}
		t.Run(name, func(t *testing.T) {
			starter := &fakeMatchStarter{}
			e, repo := setupTestWithStarter(starter, &mockBroadcaster{})

			r := seedRoomWithPlayers(repo, "Decl Start", 1, 1, 2, 3, 4)
			r.DeclarationsEnabled = want
			// StartGame requires four SEATED players.
			seats := []int{0, 1, 2, 3}
			teams := []string{"teamA", "teamB", "teamA", "teamB"}
			for i, p := range repo.players {
				p.Seat = intPtr(seats[i])
				p.Team = strPtr(teams[i])
			}

			rec := doStartGame(e, "1", validToken(1))
			require.Equal(t, http.StatusOK, rec.Code)

			require.Equal(t, 1, starter.called)
			assert.Equal(t, want, starter.lastDeclarationsEnabled,
				"the session manager must be told the room's own setting")
		})
	}
}

// DB-backed: the column round-trips a false, and the migration's DEFAULT TRUE
// backfills a row that never mentions it. Mirrors
// TestRoom_AllowNewPlayersFalseSurvivesRoundtrip, which exists because this
// exact class of bug (GORM dropping a zero-valued field) shipped once already.
func TestRoom_DeclarationsEnabledFalseSurvivesRoundtrip(t *testing.T) {
	db := getRoomTestDB(t)
	repo := room.NewGormRepository(db)

	owner := &user.User{Email: "decl@room.test", Username: "decluser", PasswordHash: "x"}
	require.NoError(t, db.Create(owner).Error)

	r := &room.Room{
		Name: "Bez Zvanja DB", Code: "DCL001", OwnerID: owner.ID,
		Variant: "bitola", MatchMode: "1001", TimerStyle: "relaxed",
		Status: "waiting", PlayerCount: 1,
		AllowNewPlayers: true, DeclarationsEnabled: false,
	}
	require.NoError(t, repo.Create(r))

	var got struct {
		DeclarationsEnabled bool
	}
	require.NoError(t, db.Raw(
		"SELECT declarations_enabled FROM rooms WHERE id = ?", r.ID,
	).Scan(&got).Error)
	assert.False(t, got.DeclarationsEnabled,
		"GORM must send the real boolean — a `default` tag on the field would make false uninsertable")

	reread, err := repo.FindByID(r.ID)
	require.NoError(t, err)
	require.NotNil(t, reread)
	assert.False(t, reread.DeclarationsEnabled, "and it must read back false")
}

// The DB-side DEFAULT TRUE is the migration's backfill: it is what keeps every
// room that predates the column playing exactly as it did.
func TestRoom_DeclarationsEnabledDefaultsTrueOnRawInsert(t *testing.T) {
	db := getRoomTestDB(t)

	owner := &user.User{Email: "decl-raw@room.test", Username: "declraw", PasswordHash: "x"}
	require.NoError(t, db.Create(owner).Error)

	require.NoError(t, db.Exec(
		`INSERT INTO rooms (name, code, owner_id, variant, match_mode, timer_style, status, player_count, min_honor, allow_new_players, created_at, updated_at)
		 VALUES ('Legacy Room', 'DCL002', ?, 'bitola', '1001', 'relaxed', 'waiting', 1, 0, TRUE, NOW(), NOW())`,
		owner.ID,
	).Error)

	var got struct {
		DeclarationsEnabled bool
	}
	require.NoError(t, db.Raw(
		"SELECT declarations_enabled FROM rooms WHERE code = 'DCL002'",
	).Scan(&got).Error)
	assert.True(t, got.DeclarationsEnabled,
		"a row inserted without the column must land on TRUE, or the migration silently changed every existing room")
}

// The lobby's LIVE path. roomLifecyclePayload is a hand-built key list, so the
// room struct's json tag does not protect it: dropping the key compiles, every
// HTTP and DB test above still passes, and a freshly created bez-zvanja room
// arrives at every connected lobby looking like an ordinary table until the next
// full refetch. Direct sibling of TestRoomLifecyclePayload_CarriesHonorGate,
// which exists for exactly this failure mode on the honor gate.
func TestRoomLifecyclePayload_CarriesDeclarationsSetting(t *testing.T) {
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{5: honorOf(96)}}
	e, _, broadcaster, _ := setupHonorTest(nil, nil, honor)
	body := `{"name":"Broadcast Zvanja","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","declarationsEnabled":false}`
	require.Equal(t, http.StatusCreated, doCreateRoom(e, body, validToken(5)).Code)

	require.Len(t, broadcaster.allCalls, 1)
	assert.Equal(t, "system:room_created", msgTypeOf(t, broadcaster.allCalls[0].msg))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(payloadOf(t, broadcaster.allCalls[0].msg), &payload))
	decl, ok := payload["declarationsEnabled"]
	require.True(t, ok, "roomLifecyclePayload must carry declarationsEnabled")
	assert.Equal(t, false, decl, "the lobby card's chip reads this key and nothing else")
}
