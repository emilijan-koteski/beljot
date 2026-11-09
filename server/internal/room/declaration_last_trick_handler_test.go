package room_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/room"
	"github.com/emilijan/beljot/server/internal/user"
)

// TestCreateRoom_DeclarationLastTrickDefaultsToThree checks an omitted or null field stores the standard window.
func TestCreateRoom_DeclarationLastTrickDefaultsToThree(t *testing.T) {
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
			body: `{"name":"Null Field","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","declarationLastTrick":null}`,
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
			assert.Equal(t, 3, persisted.DeclarationLastTrick)
		})
	}
}

// TestCreateRoom_DeclarationLastTrickSurvivesForBitola checks every valid window reaches a Bitola room with declarations on.
func TestCreateRoom_DeclarationLastTrickSurvivesForBitola(t *testing.T) {
	for _, n := range []int{1, 2, 3} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			e, repo := setupTest()
			body := `{"name":"Window","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","declarationLastTrick":` +
				strconv.Itoa(n) + `}`

			rec := doCreateRoom(e, body, validToken(5))
			require.Equal(t, http.StatusCreated, rec.Code)

			persisted, perr := repo.FindByID(1)
			require.NoError(t, perr)
			require.NotNil(t, persisted)
			assert.Equal(t, n, persisted.DeclarationLastTrick)
		})
	}
}

// TestCreateRoom_DeclarationLastTrickStoresThreeWhereUnused checks a Croatian room and a room without declarations
// store the default, since neither has a declaration window.
func TestCreateRoom_DeclarationLastTrickStoresThreeWhereUnused(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "croatian",
			body: `{"name":"Croatian","variant":"croatia","matchMode":"1001","timerStyle":"relaxed","declarationLastTrick":1}`,
		},
		{
			name: "bitola without declarations",
			body: `{"name":"Bez Zvanja","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","declarationsEnabled":false,"declarationLastTrick":1}`,
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
			assert.Equal(t, 3, persisted.DeclarationLastTrick)
		})
	}
}

// TestCreateRoom_DeclarationLastTrickOutOfRange checks a window outside 1..3 is a 400, whatever the variant.
func TestCreateRoom_DeclarationLastTrickOutOfRange(t *testing.T) {
	for _, n := range []int{0, 4, -1} {
		for _, variant := range []string{"bitola", "croatia"} {
			t.Run(variant+"/"+strconv.Itoa(n), func(t *testing.T) {
				e, repo := setupTest()
				body := `{"name":"Bad Window","variant":"` + variant +
					`","matchMode":"1001","timerStyle":"relaxed","declarationLastTrick":` + strconv.Itoa(n) + `}`

				rec := doCreateRoom(e, body, validToken(5))
				require.Equal(t, http.StatusBadRequest, rec.Code)
				assert.Equal(t, "INVALID_DECLARATION_LAST_TRICK", errCodeOf(t, rec))

				persisted, perr := repo.FindByID(1)
				require.NoError(t, perr)
				assert.Nil(t, persisted, "a rejected request must not create a room")
			})
		}
	}
}

// TestCreateRoom_DeclarationLastTrickOnTheResponseAndBroadcast checks the created room and its room_created payload
// carry the key the lobby and the rules popover read.
func TestCreateRoom_DeclarationLastTrickOnTheResponseAndBroadcast(t *testing.T) {
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{5: honorOf(96)}}
	e, _, broadcaster, _ := setupHonorTest(nil, nil, honor)
	body := `{"name":"Short Window","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","declarationLastTrick":2}`

	rec := doCreateRoom(e, body, validToken(5))
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp struct {
		Data struct {
			DeclarationLastTrick *int `json:"declarationLastTrick"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotNil(t, resp.Data.DeclarationLastTrick, "the key must be present on the response")
	assert.Equal(t, 2, *resp.Data.DeclarationLastTrick)

	require.Len(t, broadcaster.allCalls, 1)
	assert.Equal(t, "system:room_created", msgTypeOf(t, broadcaster.allCalls[0].msg))
	var payload map[string]any
	require.NoError(t, json.Unmarshal(payloadOf(t, broadcaster.allCalls[0].msg), &payload))
	assert.Equal(t, float64(2), payload["declarationLastTrick"], "roomLifecyclePayload must carry declarationLastTrick")
}

// TestQuickPlay_SynthesizedRoomUsesTheDefaultWindow checks the hand-built Quick Play room stores and announces 3.
func TestQuickPlay_SynthesizedRoomUsesTheDefaultWindow(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()

	rec := doQuickPlay(e, validToken(5))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	persisted, perr := repo.FindByID(1)
	require.NoError(t, perr)
	require.NotNil(t, persisted)
	require.True(t, persisted.IsQuickPlay)
	assert.Equal(t, 3, persisted.DeclarationLastTrick)

	var created []byte
	for _, c := range broadcaster.allCalls {
		if msgTypeOf(t, c.msg) == "system:room_created" {
			created = c.msg
			break
		}
	}
	require.NotNil(t, created)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(payloadOf(t, created), &payload))
	assert.Equal(t, float64(3), payload["declarationLastTrick"])
}

// TestStartGame_PassesTheRoomsDeclarationLastTrickToTheSessionManager checks the session manager receives the room's
// persisted window, uncrossed with the other rule arguments.
func TestStartGame_PassesTheRoomsDeclarationLastTrickToTheSessionManager(t *testing.T) {
	for _, want := range []int{1, 2, 3} {
		t.Run(strconv.Itoa(want), func(t *testing.T) {
			starter := &fakeMatchStarter{}
			e, repo := setupTestWithStarter(starter, &mockBroadcaster{})

			r := seedRoomWithPlayers(repo, "Window Start", 1, 1, 2, 3, 4)
			r.Variant = "bitola"
			r.DeclarationsEnabled = true
			r.DeclarationsTransfer = true
			r.DeclarationLastTrick = want
			seats := []int{0, 1, 2, 3}
			teams := []string{"teamA", "teamB", "teamA", "teamB"}
			for i, p := range repo.players {
				p.Seat = intPtr(seats[i])
				p.Team = strPtr(teams[i])
			}

			rec := doStartGame(e, "1", validToken(1))
			require.Equal(t, http.StatusOK, rec.Code)

			require.Equal(t, 1, starter.called)
			assert.Equal(t, want, starter.lastDeclarationLastTrick, "the session manager must be told the room's own window")
			assert.True(t, starter.lastDeclarationsEnabled, "the rule arguments must not be crossed")
			assert.True(t, starter.lastDeclarationsTransfer, "the rule arguments must not be crossed")
			assert.False(t, starter.lastClockwise, "the rule arguments must not be crossed")
		})
	}
}

// TestStaleRoomAdapter_CarriesDeclarationLastTrick checks the reconciler receives the room's window for the
// abandoned match row.
func TestStaleRoomAdapter_CarriesDeclarationLastTrick(t *testing.T) {
	repo := newMockRoomRepo()
	require.NoError(t, repo.Create(&room.Room{
		Name: "Stale Window", Code: "STL002", OwnerID: 1,
		Variant: "bitola", MatchMode: "1001", TimerStyle: "relaxed",
		Status: "playing", PlayerCount: 4,
		DeclarationsEnabled: true, DeclarationsTransfer: true, DeclarationLastTrick: 2,
	}))
	adapter := &room.StaleRoomRepositoryAdapter{Repo: repo}

	stale, err := adapter.FindByStatus("playing")
	require.NoError(t, err)
	require.Len(t, stale, 1)
	assert.Equal(t, 2, stale[0].DeclarationLastTrick)
}

// TestRoom_DeclarationLastTrickDefaultsToThreeOnRawInsert checks the migration's DEFAULT 3, which every room that
// predates the column reads as.
func TestRoom_DeclarationLastTrickDefaultsToThreeOnRawInsert(t *testing.T) {
	db := getRoomTestDB(t)

	owner := &user.User{Email: "window-raw@room.test", Username: "windowraw", PasswordHash: "x"}
	require.NoError(t, db.Create(owner).Error)

	require.NoError(t, db.Exec(
		`INSERT INTO rooms (name, code, owner_id, variant, match_mode, timer_style, status, player_count, min_honor, allow_new_players, declarations_enabled, created_at, updated_at)
		 VALUES ('Legacy Window Room', 'DLT001', ?, 'bitola', '1001', 'relaxed', 'waiting', 1, 0, TRUE, TRUE, NOW(), NOW())`,
		owner.ID,
	).Error)

	var got struct {
		DeclarationLastTrick int
	}
	require.NoError(t, db.Raw(
		"SELECT declaration_last_trick FROM rooms WHERE code = 'DLT001'",
	).Scan(&got).Error)
	assert.Equal(t, 3, got.DeclarationLastTrick, "a row inserted without the column must read as 3")
}

// TestRoom_DeclarationLastTrickRoundtrip checks an explicit window reaches the column and reads back.
func TestRoom_DeclarationLastTrickRoundtrip(t *testing.T) {
	db := getRoomTestDB(t)
	repo := room.NewGormRepository(db)

	owner := &user.User{Email: "window@room.test", Username: "windowuser", PasswordHash: "x"}
	require.NoError(t, db.Create(owner).Error)

	r := &room.Room{
		Name: "Window DB", Code: "DLT002", OwnerID: owner.ID,
		Variant: "bitola", MatchMode: "1001", TimerStyle: "relaxed",
		Status: "waiting", PlayerCount: 1,
		AllowNewPlayers: true, DeclarationsEnabled: true, DeclarationsTransfer: true, DeclarationLastTrick: 1,
	}
	require.NoError(t, repo.Create(r))

	reread, err := repo.FindByID(r.ID)
	require.NoError(t, err)
	require.NotNil(t, reread)
	assert.Equal(t, 1, reread.DeclarationLastTrick)
}
