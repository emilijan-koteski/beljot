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

func TestCreateRoom_ClockwiseDefaultsToOff(t *testing.T) {
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
			body: `{"name":"Null Field","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","clockwise":null}`,
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
			assert.False(t, persisted.Clockwise, "an absent setting must mean counter-clockwise play")
			assert.True(t, persisted.DeclarationsEnabled, "and it must not disturb the declarations toggle")
			assert.True(t, persisted.DeclarationsTransfer, "or the declarations transfer rule")
			assert.False(t, persisted.StopAtTarget, "or the stop-at-target toggle")
		})
	}
}

func TestCreateRoom_ClockwiseOnSurvives(t *testing.T) {
	for _, variant := range []string{"bitola", "croatia"} {
		t.Run(variant, func(t *testing.T) {
			e, repo := setupTest()
			body := `{"name":"Lefty","variant":"` + variant +
				`","matchMode":"1001","timerStyle":"relaxed","clockwise":true}`

			rec := doCreateRoom(e, body, validToken(5))
			require.Equal(t, http.StatusCreated, rec.Code)

			persisted, perr := repo.FindByID(1)
			require.NoError(t, perr)
			require.NotNil(t, persisted)
			assert.True(t, persisted.Clockwise, "an explicit true must survive")
			assert.True(t, persisted.DeclarationsTransfer, "the other rules keep their defaults")
			assert.Equal(t, variant, persisted.Variant)

			var resp struct {
				Data struct {
					Clockwise *bool `json:"clockwise"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			require.NotNil(t, resp.Data.Clockwise, "the key must be present on the response")
			assert.True(t, *resp.Data.Clockwise)
		})
	}
}

func TestQuickPlay_SynthesizedRoomPlaysCounterClockwise(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()

	rec := doQuickPlay(e, validToken(5))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	persisted, perr := repo.FindByID(1)
	require.NoError(t, perr)
	require.NotNil(t, persisted)
	require.True(t, persisted.IsQuickPlay, "this test is meaningless unless a quick-play room was synthesized")
	assert.False(t, persisted.Clockwise, "Quick Play offers no rule choices")

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
	got, ok := payload["clockwise"]
	require.True(t, ok, "QuickPlay's room_created must carry clockwise")
	assert.Equal(t, false, got)
}

func TestStartGame_PassesTheRoomsDirectionToTheSessionManager(t *testing.T) {
	for _, want := range []bool{true, false} {
		name := "counter-clockwise"
		if want {
			name = "clockwise"
		}
		t.Run(name, func(t *testing.T) {
			starter := &fakeMatchStarter{}
			e, repo := setupTestWithStarter(starter, &mockBroadcaster{})

			r := seedRoomWithPlayers(repo, "Direction Start", 1, 1, 2, 3, 4)
			r.Clockwise = want
			r.DeclarationsEnabled = true
			r.DeclarationsTransfer = !want
			r.StopAtTarget = !want
			seats := []int{0, 1, 2, 3}
			teams := []string{"teamA", "teamB", "teamA", "teamB"}
			for i, p := range repo.players {
				p.Seat = intPtr(seats[i])
				p.Team = strPtr(teams[i])
			}

			rec := doStartGame(e, "1", validToken(1))
			require.Equal(t, http.StatusOK, rec.Code)

			require.Equal(t, 1, starter.called)
			assert.Equal(t, want, starter.lastClockwise, "the session manager must be told the room's own direction")
			assert.Equal(t, !want, starter.lastDeclarationsTransfer, "the rule arguments must not be crossed")
			assert.Equal(t, !want, starter.lastStopAtTarget, "the rule arguments must not be crossed")
		})
	}
}

func TestQuickJoin_AutoStartPassesTheRoomsDirection(t *testing.T) {
	starter := &fakeMatchStarter{}
	e, repo, _, _ := setupCoinTestBC(starter, &stubWallet{balance: 5000})
	r := seedQuickPlayRoomWithStake(t, repo, "QPCW01", 500, 100, 200, 300)
	r.Clockwise = true

	rec := doQuickJoin(e, "1", validToken(400))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	require.Equal(t, 1, starter.called)
	assert.True(t, starter.lastClockwise, "the auto-start path must forward the room's direction")
	assert.True(t, starter.lastDeclarationsTransfer, "and the rule arguments are not crossed")
}

func TestRoomLifecyclePayload_CarriesClockwise(t *testing.T) {
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{5: honorOf(96)}}
	e, _, broadcaster, _ := setupHonorTest(nil, nil, honor)
	body := `{"name":"Broadcast Direction","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","clockwise":true}`
	require.Equal(t, http.StatusCreated, doCreateRoom(e, body, validToken(5)).Code)

	require.Len(t, broadcaster.allCalls, 1)
	assert.Equal(t, "system:room_created", msgTypeOf(t, broadcaster.allCalls[0].msg))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(payloadOf(t, broadcaster.allCalls[0].msg), &payload))
	got, ok := payload["clockwise"]
	require.True(t, ok, "roomLifecyclePayload must carry clockwise")
	assert.Equal(t, true, got)
}

func TestStaleRoomAdapter_CarriesClockwise(t *testing.T) {
	for _, want := range []bool{true, false} {
		repo := newMockRoomRepo()
		require.NoError(t, repo.Create(&room.Room{
			Name: "Stale Direction", Code: "STL002", OwnerID: 1,
			Variant: "bitola", MatchMode: "1001", TimerStyle: "relaxed",
			Status: "playing", PlayerCount: 4,
			DeclarationsEnabled: true, DeclarationsTransfer: true,
			Clockwise: want,
		}))
		adapter := &room.StaleRoomRepositoryAdapter{Repo: repo}

		stale, err := adapter.FindByStatus("playing")
		require.NoError(t, err)
		require.Len(t, stale, 1)
		assert.Equal(t, want, stale[0].Clockwise)
		assert.True(t, stale[0].DeclarationsTransfer, "the rule flags are not crossed")
	}
}

func TestRoom_ClockwiseTrueSurvivesRoundtrip(t *testing.T) {
	db := getRoomTestDB(t)
	repo := room.NewGormRepository(db)

	owner := &user.User{Email: "clockwise@room.test", Username: "clockwiseuser", PasswordHash: "x"}
	require.NoError(t, db.Create(owner).Error)

	r := &room.Room{
		Name: "Clockwise DB", Code: "CWR001", OwnerID: owner.ID,
		Variant: "bitola", MatchMode: "1001", TimerStyle: "relaxed",
		Status: "waiting", PlayerCount: 1,
		AllowNewPlayers: true, DeclarationsEnabled: true, DeclarationsTransfer: true, Clockwise: true,
	}
	require.NoError(t, repo.Create(r))

	var got struct {
		Clockwise bool
	}
	require.NoError(t, db.Raw("SELECT clockwise FROM rooms WHERE id = ?", r.ID).Scan(&got).Error)
	assert.True(t, got.Clockwise, "the explicit true must reach the column")

	reread, err := repo.FindByID(r.ID)
	require.NoError(t, err)
	require.NotNil(t, reread)
	assert.True(t, reread.Clockwise, "and it must read back true")
}

func TestRoom_ClockwiseDefaultsFalseOnRawInsert(t *testing.T) {
	db := getRoomTestDB(t)

	owner := &user.User{Email: "clockwise-raw@room.test", Username: "clockwiseraw", PasswordHash: "x"}
	require.NoError(t, db.Create(owner).Error)

	require.NoError(t, db.Exec(
		`INSERT INTO rooms (name, code, owner_id, variant, match_mode, timer_style, status, player_count, min_honor, allow_new_players, declarations_enabled, created_at, updated_at)
		 VALUES ('Legacy Direction Room', 'CWR002', ?, 'bitola', '1001', 'relaxed', 'waiting', 1, 0, TRUE, TRUE, NOW(), NOW())`,
		owner.ID,
	).Error)

	var got struct {
		Clockwise bool
	}
	require.NoError(t, db.Raw("SELECT clockwise FROM rooms WHERE code = 'CWR002'").Scan(&got).Error)
	assert.False(t, got.Clockwise, "a row inserted without the column must land on FALSE")
}
