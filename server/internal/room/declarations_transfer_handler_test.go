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

// TestCreateRoom_DeclarationsTransferDefaultsToOn checks an omitted or null field keeps today's rule,
// where the side that takes the whole hand also banks the other side's declarations.
func TestCreateRoom_DeclarationsTransferDefaultsToOn(t *testing.T) {
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
			body: `{"name":"Null Field","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","declarationsTransfer":null}`,
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
			assert.True(t, persisted.DeclarationsTransfer, "an absent setting must mean declarations transfer")
			assert.True(t, persisted.DeclarationsEnabled, "and it must not disturb the declarations toggle")
			assert.False(t, persisted.StopAtTarget, "or the stop-at-target toggle")
		})
	}
}

// TestCreateRoom_DeclarationsTransferOffSurvives checks an explicit false reaches the persisted row
// in either variant instead of flipping back to the default.
func TestCreateRoom_DeclarationsTransferOffSurvives(t *testing.T) {
	for _, variant := range []string{"bitola", "croatia"} {
		t.Run(variant, func(t *testing.T) {
			e, repo := setupTest()
			body := `{"name":"No Transfer","variant":"` + variant +
				`","matchMode":"1001","timerStyle":"relaxed","declarationsTransfer":false}`

			rec := doCreateRoom(e, body, validToken(5))
			require.Equal(t, http.StatusCreated, rec.Code)

			persisted, perr := repo.FindByID(1)
			require.NoError(t, perr)
			require.NotNil(t, persisted)
			assert.False(t, persisted.DeclarationsTransfer, "an explicit false must survive")
			assert.True(t, persisted.DeclarationsEnabled, "the declarations toggle is independent")
			assert.Equal(t, variant, persisted.Variant)
		})
	}
}

// TestCreateRoom_DeclarationsTransferForcedOnWithoutDeclarations checks a room without declarations never
// stores a voided transfer, since there is nothing to void.
func TestCreateRoom_DeclarationsTransferForcedOnWithoutDeclarations(t *testing.T) {
	e, repo := setupTest()
	body := `{"name":"Bez Zvanja","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","declarationsEnabled":false,"declarationsTransfer":false}`

	rec := doCreateRoom(e, body, validToken(5))
	require.Equal(t, http.StatusCreated, rec.Code)

	persisted, perr := repo.FindByID(1)
	require.NoError(t, perr)
	require.NotNil(t, persisted)
	assert.False(t, persisted.DeclarationsEnabled)
	assert.True(t, persisted.DeclarationsTransfer, "a no-declarations room must keep the default transfer rule")
}

// TestCreateRoom_DeclarationsTransferOnTheResponse checks the created room carries the key the
// modal and the lobby read back.
func TestCreateRoom_DeclarationsTransferOnTheResponse(t *testing.T) {
	e, _ := setupTest()
	body := `{"name":"No Transfer","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","declarationsTransfer":false}`

	rec := doCreateRoom(e, body, validToken(5))
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp struct {
		Data struct {
			DeclarationsTransfer *bool `json:"declarationsTransfer"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotNil(t, resp.Data.DeclarationsTransfer, "the key must be present on the response")
	assert.False(t, *resp.Data.DeclarationsTransfer)
}

// TestQuickPlay_SynthesizedRoomTransfersDeclarations checks the hand-built Quick Play room keeps
// today's rule and announces it on its own system:room_created map.
func TestQuickPlay_SynthesizedRoomTransfersDeclarations(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()

	rec := doQuickPlay(e, validToken(5))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	persisted, perr := repo.FindByID(1)
	require.NoError(t, perr)
	require.NotNil(t, persisted)
	require.True(t, persisted.IsQuickPlay, "this test is meaningless unless a quick-play room was synthesized")
	assert.True(t, persisted.DeclarationsTransfer, "Quick Play offers no rule choices")

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
	got, ok := payload["declarationsTransfer"]
	require.True(t, ok, "QuickPlay's own room_created map must carry declarationsTransfer")
	assert.Equal(t, true, got)
}

// TestStartGame_PassesTheRoomsDeclarationsTransferToTheSessionManager checks the session manager
// receives the room's persisted value, uncrossed with the other two rule arguments.
func TestStartGame_PassesTheRoomsDeclarationsTransferToTheSessionManager(t *testing.T) {
	for _, want := range []bool{true, false} {
		name := "declarations transfer"
		if !want {
			name = "declarations are voided"
		}
		t.Run(name, func(t *testing.T) {
			starter := &fakeMatchStarter{}
			e, repo := setupTestWithStarter(starter, &mockBroadcaster{})

			r := seedRoomWithPlayers(repo, "Transfer Start", 1, 1, 2, 3, 4)
			r.DeclarationsTransfer = want
			r.DeclarationsEnabled = true
			r.StopAtTarget = false
			seats := []int{0, 1, 2, 3}
			teams := []string{"teamA", "teamB", "teamA", "teamB"}
			for i, p := range repo.players {
				p.Seat = intPtr(seats[i])
				p.Team = strPtr(teams[i])
			}

			rec := doStartGame(e, "1", validToken(1))
			require.Equal(t, http.StatusOK, rec.Code)

			require.Equal(t, 1, starter.called)
			assert.Equal(t, want, starter.lastDeclarationsTransfer, "the session manager must be told the room's own setting")
			assert.True(t, starter.lastDeclarationsEnabled, "the rule arguments must not be crossed")
			assert.False(t, starter.lastStopAtTarget, "the rule arguments must not be crossed")
		})
	}
}

// TestRoomLifecyclePayload_CarriesDeclarationsTransfer checks the hand-built payload behind
// system:room_created carries the key the lobby card's chip reads.
func TestRoomLifecyclePayload_CarriesDeclarationsTransfer(t *testing.T) {
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{5: honorOf(96)}}
	e, _, broadcaster, _ := setupHonorTest(nil, nil, honor)
	body := `{"name":"Broadcast Transfer","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","declarationsTransfer":false}`
	require.Equal(t, http.StatusCreated, doCreateRoom(e, body, validToken(5)).Code)

	require.Len(t, broadcaster.allCalls, 1)
	assert.Equal(t, "system:room_created", msgTypeOf(t, broadcaster.allCalls[0].msg))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(payloadOf(t, broadcaster.allCalls[0].msg), &payload))
	got, ok := payload["declarationsTransfer"]
	require.True(t, ok, "roomLifecyclePayload must carry declarationsTransfer")
	assert.Equal(t, false, got)
}

// TestRoom_DeclarationsTransferFalseSurvivesRoundtrip checks an explicit false reaches the column,
// which a GORM `default` tag on the field would silently drop.
func TestRoom_DeclarationsTransferFalseSurvivesRoundtrip(t *testing.T) {
	db := getRoomTestDB(t)
	repo := room.NewGormRepository(db)

	owner := &user.User{Email: "transfer@room.test", Username: "transferuser", PasswordHash: "x"}
	require.NoError(t, db.Create(owner).Error)

	r := &room.Room{
		Name: "Transfer DB", Code: "DTR001", OwnerID: owner.ID,
		Variant: "bitola", MatchMode: "1001", TimerStyle: "relaxed",
		Status: "waiting", PlayerCount: 1,
		AllowNewPlayers: true, DeclarationsEnabled: true, DeclarationsTransfer: false,
	}
	require.NoError(t, repo.Create(r))

	var got struct {
		DeclarationsTransfer bool
	}
	require.NoError(t, db.Raw(
		"SELECT declarations_transfer FROM rooms WHERE id = ?", r.ID,
	).Scan(&got).Error)
	assert.False(t, got.DeclarationsTransfer, "the explicit false must reach the column")

	reread, err := repo.FindByID(r.ID)
	require.NoError(t, err)
	require.NotNil(t, reread)
	assert.False(t, reread.DeclarationsTransfer, "and it must read back false")
}

// TestRoom_DeclarationsTransferDefaultsTrueOnRawInsert checks the migration's DEFAULT TRUE, which keeps
// every room that predates the column on today's rule.
func TestRoom_DeclarationsTransferDefaultsTrueOnRawInsert(t *testing.T) {
	db := getRoomTestDB(t)

	owner := &user.User{Email: "transfer-raw@room.test", Username: "transferraw", PasswordHash: "x"}
	require.NoError(t, db.Create(owner).Error)

	require.NoError(t, db.Exec(
		`INSERT INTO rooms (name, code, owner_id, variant, match_mode, timer_style, status, player_count, min_honor, allow_new_players, declarations_enabled, created_at, updated_at)
		 VALUES ('Legacy Transfer Room', 'DTR002', ?, 'bitola', '1001', 'relaxed', 'waiting', 1, 0, TRUE, TRUE, NOW(), NOW())`,
		owner.ID,
	).Error)

	var got struct {
		DeclarationsTransfer bool
	}
	require.NoError(t, db.Raw(
		"SELECT declarations_transfer FROM rooms WHERE code = 'DTR002'",
	).Scan(&got).Error)
	assert.True(t, got.DeclarationsTransfer, "a row inserted without the column must land on TRUE")
}

// TestStaleRoomAdapter_CarriesRuleFlags checks a room left playing by a previous process hands every rule
// flag to the reconciler, uncrossed, which writes them onto the abandoned match row.
func TestStaleRoomAdapter_CarriesRuleFlags(t *testing.T) {
	cases := []struct {
		name                 string
		declarationsEnabled  bool
		stopAtTarget         bool
		declarationsTransfer bool
	}{
		{name: "room defaults", declarationsEnabled: true, stopAtTarget: false, declarationsTransfer: true},
		{name: "no declarations and stop at target", declarationsEnabled: false, stopAtTarget: true, declarationsTransfer: true},
		{name: "stop at target and declarations voided", declarationsEnabled: true, stopAtTarget: true, declarationsTransfer: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMockRoomRepo()
			require.NoError(t, repo.Create(&room.Room{
				Name: "Stale Rules", Code: "STL001", OwnerID: 1,
				Variant: "bitola", MatchMode: "1001", TimerStyle: "relaxed",
				Status: "playing", PlayerCount: 4,
				DeclarationsEnabled:  tc.declarationsEnabled,
				StopAtTarget:         tc.stopAtTarget,
				DeclarationsTransfer: tc.declarationsTransfer,
			}))
			adapter := &room.StaleRoomRepositoryAdapter{Repo: repo}

			stale, err := adapter.FindByStatus("playing")
			require.NoError(t, err)
			require.Len(t, stale, 1)
			assert.Equal(t, tc.declarationsEnabled, stale[0].DeclarationsEnabled, "declarationsEnabled")
			assert.Equal(t, tc.stopAtTarget, stale[0].StopAtTarget, "stopAtTarget")
			assert.Equal(t, tc.declarationsTransfer, stale[0].DeclarationsTransfer, "declarationsTransfer")
		})
	}
}
