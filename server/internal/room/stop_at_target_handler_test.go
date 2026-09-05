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

// The room-level "dosta" (stop at target) toggle: request shape, defaulting,
// persistence, what reaches the session manager, and what the lobby is told.
//
// Direct sibling of declarations_handler_test.go, and deliberately shaped after
// it — same feature class, same traps. The POLARITY is the one thing that
// differs: here the safe default is FALSE, so the interesting failure is an
// explicit TRUE being lost rather than an absent field being read as off.

// The default must be OFF: finishing the hand before the target is checked is
// how every match here has ever ended. The request field is a *bool for symmetry
// with declarationsEnabled, and this pins the resolution of both "absent" forms.
func TestCreateRoom_StopAtTargetDefaultsToOff(t *testing.T) {
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
			body: `{"name":"Null Field","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","stopAtTarget":null}`,
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
			assert.False(t, persisted.StopAtTarget,
				"an absent setting must mean the historical behaviour: finish the hand")
			assert.True(t, persisted.DeclarationsEnabled,
				"and it must not disturb the other room-level rule field")
		})
	}
}

// An explicit true must survive all the way to the persisted row, in either
// variant. This is the assertion the missing GORM `default` tag protects: with
// one, GORM would drop the field from the INSERT and the toggle could never be
// turned on.
func TestCreateRoom_StopAtTargetEnabledSurvives(t *testing.T) {
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
			body := `{"name":"Dosta","variant":"` + tt.variant +
				`","matchMode":"1001","timerStyle":"relaxed","stopAtTarget":true}`

			rec := doCreateRoom(e, body, validToken(5))
			require.Equal(t, http.StatusCreated, rec.Code)

			persisted, perr := repo.FindByID(1)
			require.NoError(t, perr)
			require.NotNil(t, persisted)
			assert.True(t, persisted.StopAtTarget,
				"an explicit true must survive, not flip to the default")
			assert.Equal(t, tt.variant, persisted.Variant,
				"the toggle is variant-independent — it must not disturb the variant")
		})
	}
}

// The two room-level rule fields are independent switches, and nothing else in
// either suite proves it: both are resolved in the same handler block and
// layered onto the same config struct, so a copy-paste that read one pointer
// twice would pass every single-field test above.
func TestCreateRoom_RuleTogglesAreIndependent(t *testing.T) {
	// Marshalled from a struct rather than spliced together from JSON fragments:
	// the request shape is the thing under test, so it should be readable as one.
	type createBody struct {
		Name                string `json:"name"`
		Variant             string `json:"variant"`
		MatchMode           string `json:"matchMode"`
		TimerStyle          string `json:"timerStyle"`
		DeclarationsEnabled bool   `json:"declarationsEnabled"`
		StopAtTarget        bool   `json:"stopAtTarget"`
	}

	tests := []struct {
		name         string
		declarations bool
		stopAtTarget bool
	}{
		{name: "declarations off, finish the hand", declarations: false, stopAtTarget: false},
		{name: "declarations off, stop at target", declarations: false, stopAtTarget: true},
		{name: "declarations on, stop at target", declarations: true, stopAtTarget: true},
		{name: "declarations on, finish the hand", declarations: true, stopAtTarget: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, repo := setupTest()
			raw, merr := json.Marshal(createBody{
				Name:                "Mixed Rules",
				Variant:             "bitola",
				MatchMode:           "1001",
				TimerStyle:          "relaxed",
				DeclarationsEnabled: tt.declarations,
				StopAtTarget:        tt.stopAtTarget,
			})
			require.NoError(t, merr)

			rec := doCreateRoom(e, string(raw), validToken(5))
			require.Equal(t, http.StatusCreated, rec.Code)

			persisted, perr := repo.FindByID(1)
			require.NoError(t, perr)
			require.NotNil(t, persisted)
			assert.Equal(t, tt.declarations, persisted.DeclarationsEnabled)
			assert.Equal(t, tt.stopAtTarget, persisted.StopAtTarget)
		})
	}
}

// The created-room response is what the modal reads back, so the field has to be
// on the wire and not merely in the database.
func TestCreateRoom_StopAtTargetOnTheResponse(t *testing.T) {
	e, _ := setupTest()
	body := `{"name":"Dosta","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","stopAtTarget":true}`

	rec := doCreateRoom(e, body, validToken(5))
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp struct {
		Data struct {
			StopAtTarget *bool `json:"stopAtTarget"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotNil(t, resp.Data.StopAtTarget,
		"the key must be present — every client reader compares against true")
	assert.True(t, *resp.Data.StopAtTarget)
}

// Quick Play offers no rule choices, and its room is hand-built rather than
// created through CreateRoom. It must come out finishing the hand.
func TestQuickPlay_SynthesizedRoomFinishesTheHand(t *testing.T) {
	e, repo := setupTest()

	rec := doQuickPlay(e, validToken(5))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	persisted, perr := repo.FindByID(1)
	require.NoError(t, perr)
	require.NotNil(t, persisted)
	require.True(t, persisted.IsQuickPlay, "this test is meaningless unless a quick-play room was synthesized")
	assert.False(t, persisted.StopAtTarget,
		"Quick Play is Croatian, 501, declarations on, finish the hand")
}

// QuickPlay's system:room_created is the SECOND hand-built room payload in the
// package — a separate key list from roomLifecyclePayload, which is exactly why
// it has hidden a missing field before. The lobby card treats an absent key as
// off, so an omission here is harmless TODAY and becomes a bug the day the
// default stops matching; assert the key is really sent.
func TestQuickPlayRoomCreatedPayload_CarriesStopAtTarget(t *testing.T) {
	e, _, broadcaster := setupTestWithBroadcast()

	rec := doQuickPlay(e, validToken(5))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var created []byte
	for _, c := range broadcaster.allCalls {
		if msgTypeOf(t, c.msg) == "system:room_created" {
			created = c.msg
			break
		}
	}
	require.NotNil(t, created, "QuickPlay must broadcast system:room_created for a synthesized room")

	var payload map[string]any
	require.NoError(t, json.Unmarshal(payloadOf(t, created), &payload))
	got, ok := payload["stopAtTarget"]
	require.True(t, ok, "QuickPlay's own room_created map must carry stopAtTarget")
	assert.Equal(t, false, got)
}

// The value the session manager receives is the one the engine will play by, and
// nothing downstream can validate it: true is legal, so a room that persisted
// finish-the-hand but handed the manager true would cut every match short
// mid-hand and look entirely healthy. Same reasoning as the variant string and
// the declarations flag before it.
func TestStartGame_PassesTheRoomsStopAtTargetToTheSessionManager(t *testing.T) {
	for _, want := range []bool{true, false} {
		name := "stop at target"
		if !want {
			name = "finish the hand"
		}
		t.Run(name, func(t *testing.T) {
			starter := &fakeMatchStarter{}
			e, repo := setupTestWithStarter(starter, &mockBroadcaster{})

			r := seedRoomWithPlayers(repo, "Dosta Start", 1, 1, 2, 3, 4)
			r.StopAtTarget = want
			r.DeclarationsEnabled = true
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
			assert.Equal(t, want, starter.lastStopAtTarget,
				"the session manager must be told the room's own setting")
			assert.True(t, starter.lastDeclarationsEnabled,
				"and the two rule arguments must not be crossed")
		})
	}
}

// DB-backed: the column round-trips a true, and the migration's DEFAULT FALSE
// backfills a row that never mentions it.
func TestRoom_StopAtTargetTrueSurvivesRoundtrip(t *testing.T) {
	db := getRoomTestDB(t)
	repo := room.NewGormRepository(db)

	owner := &user.User{Email: "dosta@room.test", Username: "dostauser", PasswordHash: "x"}
	require.NoError(t, db.Create(owner).Error)

	r := &room.Room{
		Name: "Dosta DB", Code: "SAT001", OwnerID: owner.ID,
		Variant: "bitola", MatchMode: "1001", TimerStyle: "relaxed",
		Status: "waiting", PlayerCount: 1,
		AllowNewPlayers: true, DeclarationsEnabled: true, StopAtTarget: true,
	}
	require.NoError(t, repo.Create(r))

	var got struct {
		StopAtTarget bool
	}
	require.NoError(t, db.Raw(
		"SELECT stop_at_target FROM rooms WHERE id = ?", r.ID,
	).Scan(&got).Error)
	assert.True(t, got.StopAtTarget, "the explicit true must reach the column")

	reread, err := repo.FindByID(r.ID)
	require.NoError(t, err)
	require.NotNil(t, reread)
	assert.True(t, reread.StopAtTarget, "and it must read back true")
}

// The DB-side DEFAULT FALSE is the migration's backfill: it is what keeps every
// room that predates the column finishing its hand exactly as it did.
func TestRoom_StopAtTargetDefaultsFalseOnRawInsert(t *testing.T) {
	db := getRoomTestDB(t)

	owner := &user.User{Email: "dosta-raw@room.test", Username: "dostaraw", PasswordHash: "x"}
	require.NoError(t, db.Create(owner).Error)

	require.NoError(t, db.Exec(
		`INSERT INTO rooms (name, code, owner_id, variant, match_mode, timer_style, status, player_count, min_honor, allow_new_players, declarations_enabled, created_at, updated_at)
		 VALUES ('Legacy Dosta Room', 'SAT002', ?, 'bitola', '1001', 'relaxed', 'waiting', 1, 0, TRUE, TRUE, NOW(), NOW())`,
		owner.ID,
	).Error)

	var got struct {
		StopAtTarget bool
	}
	require.NoError(t, db.Raw(
		"SELECT stop_at_target FROM rooms WHERE code = 'SAT002'",
	).Scan(&got).Error)
	assert.False(t, got.StopAtTarget,
		"a row inserted without the column must land on FALSE, or the migration silently changed every existing room")
}

// The lobby's LIVE path. roomLifecyclePayload is a hand-built key list, so the
// room struct's json tag does not protect it: dropping the key compiles, every
// HTTP and DB test above still passes, and a freshly created dosta room arrives
// at every connected lobby looking like an ordinary table until the next full
// refetch. This one map feeds BOTH system:room_created and system:room_updated.
func TestRoomLifecyclePayload_CarriesStopAtTarget(t *testing.T) {
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{5: honorOf(96)}}
	e, _, broadcaster, _ := setupHonorTest(nil, nil, honor)
	body := `{"name":"Broadcast Dosta","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","stopAtTarget":true}`
	require.Equal(t, http.StatusCreated, doCreateRoom(e, body, validToken(5)).Code)

	require.Len(t, broadcaster.allCalls, 1)
	assert.Equal(t, "system:room_created", msgTypeOf(t, broadcaster.allCalls[0].msg))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(payloadOf(t, broadcaster.allCalls[0].msg), &payload))
	got, ok := payload["stopAtTarget"]
	require.True(t, ok, "roomLifecyclePayload must carry stopAtTarget")
	assert.Equal(t, true, got, "the lobby card's chip reads this key and nothing else")
}
