package room_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/auth"
	"github.com/emilijan/beljot/server/internal/room"
	"github.com/emilijan/beljot/server/internal/user"
)

// Story 9.8 (FR57) — honor-gated rooms. Sibling of coin_handler_test.go /
// privacy_handler_test.go, reusing the same mockRoomRepo / mockBroadcaster /
// PresenceRegistry harness.

// --- stubHonor -------------------------------------------------------------

// stubHonor implements room.HonorService for the honor-gate tests. Mirrors
// stubWallet's shape, plus a CALL COUNTER: AC4's "an ungated room performs no
// honor read at all" is only provable with one — asserted by inspection it would
// silently rot the first time a call site forgot to short-circuit.
type stubHonor struct {
	// snaps is the authoritative per-user snapshot the service would recompute.
	// A user absent from the map is absent from the result, which the handler
	// treats as a failure rather than an admit.
	snaps map[uint]user.HonorSnapshot
	err   error

	calls       int
	requestedBy [][]uint
}

func (s *stubHonor) HonorForUsers(userIDs []uint) (map[uint]user.HonorSnapshot, error) {
	s.calls++
	s.requestedBy = append(s.requestedBy, append([]uint(nil), userIDs...))
	if s.err != nil {
		return nil, s.err
	}
	out := make(map[uint]user.HonorSnapshot, len(userIDs))
	for _, id := range userIDs {
		if snap, ok := s.snaps[id]; ok {
			out[id] = snap
		}
	}
	return out, nil
}

// honorOf is shorthand for an EXPERIENCED player's snapshot at a given score.
// The raw totals are what make IsNewPlayer false, so they are set past the
// 5-match floor rather than left at zero.
func honorOf(score int) user.HonorSnapshot {
	return user.HonorSnapshot{
		Score:          score,
		Tier:           user.HonorTier(score),
		CompletedTotal: 20,
		AbandonedTotal: 2,
		IsNewPlayer:    false,
	}
}

// newPlayerHonorOf is a NEW player's snapshot: fewer than 5 finished matches.
//
// A New Player's score can never be below 19 — the floor caps a newcomer at 4
// finished matches, so their worst reachable case is 0 completed / 4 abandoned:
// 100 x 4 / (4 + 4*4 + 1) = 400/21 = 19. Fixtures below use 80 (the untouched
// Beta(4,1) prior) and 19 (the worst case) for that reason; a newcomer paired
// with a score of, say, 5 is a state the system cannot produce.
func newPlayerHonorOf(score int) user.HonorSnapshot {
	return user.HonorSnapshot{
		Score:          score,
		Tier:           user.HonorTier(score),
		CompletedTotal: 0,
		AbandonedTotal: 4,
		IsNewPlayer:    true,
	}
}

// setupHonorTest wires the room handler with an injected honor stub (and an
// optional wallet stub), the full route set, an observable broadcaster and a
// caller-controlled presence registry.
func setupHonorTest(
	starter room.MatchStarter,
	wallet room.WalletService,
	honor room.HonorService,
) (*echo.Echo, *mockRoomRepo, *mockBroadcaster, *room.PresenceRegistry) {
	repo := newMockRoomRepo()
	broadcaster := &mockBroadcaster{}
	reg := room.NewPresenceRegistry()
	handler := room.NewRoomHandler(repo, starter, broadcaster, reg, wallet, honor)

	e := echo.New()
	e.HTTPErrorHandler = testErrorHandler
	api := e.Group("/api/v1", auth.AuthMiddleware("test-jwt-secret"))
	registerRoomRoutes(api, handler)
	return e, repo, broadcaster, reg
}

// seedGatedRoom seats the owner (100) at seat 0 of a waiting room carrying the
// given honor gate.
func seedGatedRoom(t *testing.T, repo *mockRoomRepo, minHonor int, allowNewPlayers bool) *room.Room {
	t.Helper()
	r := &room.Room{
		Name:            "Gated",
		Code:            "GATED1",
		OwnerID:         100,
		Variant:         "bitola",
		MatchMode:       "1001",
		TimerStyle:      "relaxed",
		Status:          "waiting",
		PlayerCount:     1,
		MinHonor:        minHonor,
		AllowNewPlayers: allowNewPlayers,
	}
	require.NoError(t, repo.Create(r))
	seat := 0
	team := teamNameForSeat(0)
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{
		RoomID: r.ID, UserID: 100, Username: "owner", Seat: &seat, Team: &team,
	}))
	return r
}

// --- The D1 truth table (AC4) ---------------------------------------------

// Every row of the Story 9.8 D1 truth table, evaluated through the real join
// endpoint. The two gates are INDEPENDENT: isNewPlayer is checked first and a
// New Player is never score-checked, which is what makes allowNewPlayers
// meaningful (and is the half of the gate the epic got wrong).
func TestJoinRoom_HonorGateTruthTable(t *testing.T) {
	const joiner = uint(200)

	tests := []struct {
		name            string
		minHonor        int
		allowNewPlayers bool
		snap            user.HonorSnapshot
		wantStatus      int
		wantCode        string
		wantHonorReads  int
	}{
		{
			// Row 1: ungated room. No honor read is performed at all (D5).
			name:     "row 1: ungated room admits anyone with no honor read",
			minHonor: 0, allowNewPlayers: true,
			snap:       newPlayerHonorOf(19),
			wantStatus: http.StatusOK, wantHonorReads: 0,
		},
		{
			// Row 2: the newcomer bar bites even at minHonor 0 — the epic's
			// scoping would have made this toggle dead in a 0-gate room.
			name:     "row 2: new player rejected in a minHonor 0 veterans-only room",
			minHonor: 0, allowNewPlayers: false,
			snap:       newPlayerHonorOf(80),
			wantStatus: http.StatusConflict, wantCode: "NEW_PLAYER_NOT_ALLOWED", wantHonorReads: 1,
		},
		{
			name:     "row 3: experienced player admitted at minHonor 0 whatever the score",
			minHonor: 0, allowNewPlayers: false,
			snap:       honorOf(5),
			wantStatus: http.StatusOK, wantHonorReads: 1,
		},
		{
			// Rows 4-5: a New Player is NEVER score-checked. This is the whole
			// point of the toggle — the owner's explicit "I'll take an unknown".
			name:     "row 4: new player admitted in a minHonor 80 room on the prior",
			minHonor: 80, allowNewPlayers: true,
			snap:       newPlayerHonorOf(80),
			wantStatus: http.StatusOK, wantHonorReads: 1,
		},
		{
			name:     "row 5: new player admitted at the worst score a newcomer can hold",
			minHonor: 80, allowNewPlayers: true,
			snap:       newPlayerHonorOf(19),
			wantStatus: http.StatusOK, wantHonorReads: 1,
		},
		{
			// Row 6: precedence — isNewPlayer is evaluated FIRST, so this is
			// NEW_PLAYER_NOT_ALLOWED and never HONOR_TOO_LOW.
			name:     "row 6: new player barred by the toggle even when the score would pass",
			minHonor: 80, allowNewPlayers: false,
			snap:       newPlayerHonorOf(80),
			wantStatus: http.StatusConflict, wantCode: "NEW_PLAYER_NOT_ALLOWED", wantHonorReads: 1,
		},
		{
			name:     "row 7: experienced player one below the threshold is rejected",
			minHonor: 80, allowNewPlayers: true,
			snap:       honorOf(79),
			wantStatus: http.StatusConflict, wantCode: "HONOR_TOO_LOW", wantHonorReads: 1,
		},
		{
			name:     "row 8: the threshold boundary is inclusive",
			minHonor: 80, allowNewPlayers: true,
			snap:       honorOf(80),
			wantStatus: http.StatusOK, wantHonorReads: 1,
		},
		{
			// Row 9: a minHonor of 100 is very nearly a closed room — reaching a
			// rounded 100 needs ~195 decayed completions inside one 90-day
			// half-life. This rejection is correct, not a bug to "fix".
			name:     "row 9: a minHonor 100 room rejects a 99",
			minHonor: 100, allowNewPlayers: true,
			snap:       honorOf(99),
			wantStatus: http.StatusConflict, wantCode: "HONOR_TOO_LOW", wantHonorReads: 1,
		},
		{
			name:     "row 10: the newcomer bar does not apply to an experienced player",
			minHonor: 80, allowNewPlayers: false,
			snap:       honorOf(95),
			wantStatus: http.StatusOK, wantHonorReads: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{joiner: tt.snap}}
			e, repo, _, _ := setupHonorTest(nil, nil, honor)
			seedGatedRoom(t, repo, tt.minHonor, tt.allowNewPlayers)

			rec := doJoinRoom(e, "1", validToken(joiner))
			require.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantCode != "" {
				assert.Equal(t, tt.wantCode, errCodeOf(t, rec))
			}
			assert.Equal(t, tt.wantHonorReads, honor.calls,
				"an ungated room must perform ZERO honor reads; a gated one exactly one")

			players, _ := repo.FindPlayersByRoomID(1)
			if tt.wantStatus == http.StatusOK {
				assert.Len(t, players, 2, "an admitted joiner takes a seat")
			} else {
				assert.Len(t, players, 1, "a rejected joiner never becomes a member")
			}
		})
	}
}

// AC4: a nil honorService means no enforcement, mirroring the nil-walletService
// affordance — a gated room admits everyone rather than rejecting everyone.
func TestJoinRoom_NilHonorServiceSkipsGate(t *testing.T) {
	e, repo, _, _ := setupHonorTest(nil, nil, nil)
	seedGatedRoom(t, repo, 95, false)

	rec := doJoinRoom(e, "1", validToken(200))
	require.Equal(t, http.StatusOK, rec.Code)

	players, _ := repo.FindPlayersByRoomID(1)
	assert.Len(t, players, 2)
}

// AC4: a honor-read FAILURE fails the join with a 500 — it must never fall
// through to an admit. A failed read must never open a closed door.
func TestJoinRoom_HonorReadErrorFailsClosed(t *testing.T) {
	honor := &stubHonor{err: errors.New("db unavailable")}
	e, repo, _, _ := setupHonorTest(nil, nil, honor)
	seedGatedRoom(t, repo, 80, true)

	rec := doJoinRoom(e, "1", validToken(200))
	require.Equal(t, http.StatusInternalServerError, rec.Code)

	players, _ := repo.FindPlayersByRoomID(1)
	assert.Len(t, players, 1, "the joiner must NOT be admitted when honor cannot be read")
}

// AC4: an authenticated user with no honor row is an internal inconsistency, not
// an admit. The zero-value snapshot would read as an experienced player with a
// score of 0 and would sail straight through a minHonor 0 / veterans-only room.
func TestJoinRoom_MissingHonorRowFailsClosed(t *testing.T) {
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{}}
	e, repo, _, _ := setupHonorTest(nil, nil, honor)
	seedGatedRoom(t, repo, 0, false)

	rec := doJoinRoom(e, "1", validToken(200))
	require.Equal(t, http.StatusInternalServerError, rec.Code)

	players, _ := repo.FindPlayersByRoomID(1)
	assert.Len(t, players, 1)
}

// AC4: the honor gate is appended AFTER the coin check, so a player who is both
// broke and short on honor gets INSUFFICIENT_COINS. Intentional — the hardened
// password/capacity/membership/coin order is left untouched.
func TestJoinRoom_CoinCheckPrecedesHonorGate(t *testing.T) {
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{200: honorOf(10)}}
	wallet := &stubWallet{balance: 0}
	e, repo, _, _ := setupHonorTest(nil, wallet, honor)
	r := seedGatedRoom(t, repo, 80, true)
	r.CoinBuyIn = 500

	rec := doJoinRoom(e, "1", validToken(200))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "INSUFFICIENT_COINS", errCodeOf(t, rec))
	assert.Equal(t, 0, honor.calls, "the coin check short-circuits before the honor read")
}

// --- CreateRoom (AC3, D7) -------------------------------------------------

func TestCreateRoom_DefaultsToUngated(t *testing.T) {
	e, repo, _, _ := setupHonorTest(nil, nil, &stubHonor{})
	body := `{"name":"Open Table","variant":"bitola","matchMode":"1001","timerStyle":"relaxed"}`
	rec := doCreateRoom(e, body, validToken(5))
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var data room.Room
	require.NoError(t, json.Unmarshal(resp["data"], &data))
	assert.Equal(t, 0, data.MinHonor)
	assert.True(t, data.AllowNewPlayers, "an omitted allowNewPlayers defaults to TRUE")

	persisted, _ := repo.FindByID(data.ID)
	require.NotNil(t, persisted)
	assert.Equal(t, 0, persisted.MinHonor)
	assert.True(t, persisted.AllowNewPlayers)
}

func TestCreateRoom_PersistsExplicitHonorGate(t *testing.T) {
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{5: honorOf(96)}}
	e, repo, _, _ := setupHonorTest(nil, nil, honor)
	body := `{"name":"Veterans","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","minHonor":90,"allowNewPlayers":false}`
	rec := doCreateRoom(e, body, validToken(5))
	require.Equal(t, http.StatusCreated, rec.Code)

	persisted, _ := repo.FindByID(1)
	require.NotNil(t, persisted)
	assert.Equal(t, 90, persisted.MinHonor)
	assert.False(t, persisted.AllowNewPlayers, "an explicit false must survive, not flip to the default")
}

func TestCreateRoom_RejectsOutOfRangeMinHonor(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"above the ceiling", `{"name":"Too High","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","minHonor":101}`},
		{"below the floor", `{"name":"Too Low","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","minHonor":-1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, repo, _, _ := setupHonorTest(nil, nil, &stubHonor{})
			rec := doCreateRoom(e, tt.body, validToken(5))
			require.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, "INVALID_MIN_HONOR", errCodeOf(t, rec))

			persisted, _ := repo.FindByID(1)
			assert.Nil(t, persisted, "a rejected create must not persist a room")
		})
	}
}

// D7: the creator is auto-seated, so an owner who cannot pass their own gate
// would be ejected from their own room at the first Start. Rejected up front
// with the same code the join gate would have returned — mirroring the existing
// buy-in affordability check, which exists for the identical reason.
func TestCreateRoom_CreatorMustSatisfyOwnGate(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		snap     user.HonorSnapshot
		wantCode string
	}{
		{
			name:     "experienced creator below their own threshold",
			body:     `{"name":"Self Locked","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","minHonor":95}`,
			snap:     honorOf(60),
			wantCode: "HONOR_TOO_LOW",
		},
		{
			name:     "new-player creator barring new players",
			body:     `{"name":"Self Barred","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","allowNewPlayers":false}`,
			snap:     newPlayerHonorOf(80),
			wantCode: "NEW_PLAYER_NOT_ALLOWED",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{5: tt.snap}}
			e, repo, _, _ := setupHonorTest(nil, nil, honor)

			rec := doCreateRoom(e, tt.body, validToken(5))
			require.Equal(t, http.StatusConflict, rec.Code)
			assert.Equal(t, tt.wantCode, errCodeOf(t, rec))

			persisted, _ := repo.FindByID(1)
			assert.Nil(t, persisted, "the room must not exist if its owner cannot enter it")
		})
	}
}

// D7 + D1: a NEW PLAYER creator may still set a high minHonor for others, because
// a New Player is never score-checked against it. The self-gate must not reject
// them for a bar the gate would never apply to them.
func TestCreateRoom_NewPlayerCreatorMaySetHighThreshold(t *testing.T) {
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{5: newPlayerHonorOf(80)}}
	e, repo, _, _ := setupHonorTest(nil, nil, honor)
	body := `{"name":"High Bar","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","minHonor":95,"allowNewPlayers":true}`

	rec := doCreateRoom(e, body, validToken(5))
	require.Equal(t, http.StatusCreated, rec.Code)

	persisted, _ := repo.FindByID(1)
	require.NotNil(t, persisted)
	assert.Equal(t, 95, persisted.MinHonor)
}

// D5: creating an ungated room performs no honor read.
func TestCreateRoom_UngatedPerformsNoHonorRead(t *testing.T) {
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{5: honorOf(90)}}
	e, _, _, _ := setupHonorTest(nil, nil, honor)
	body := `{"name":"Open","variant":"bitola","matchMode":"1001","timerStyle":"relaxed"}`

	require.Equal(t, http.StatusCreated, doCreateRoom(e, body, validToken(5)).Code)
	assert.Equal(t, 0, honor.calls)
}

// --- ReturnToRoom (AC6, AC7) ---------------------------------------------

// AC6: the return-time re-check ejects a seated player whose honor has dropped
// below the room's threshold (in practice: they abandoned the previous match),
// through the same flow insolvency uses — 409 to the caller, seat freed,
// player_left + room_updated fan-out, and a per-user system:honor_ejected.
func TestReturnToRoom_HonorEjectsReturner(t *testing.T) {
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{
		100: honorOf(90), // the owner, who stays
		200: honorOf(55), // the returner, now below the bar
	}}
	e, repo, broadcaster, _ := setupHonorTest(nil, nil, honor)
	r := seedFinishedRoom(repo, 100, map[uint]int{100: 0, 200: 1}, nil)
	r.MinHonor = 80
	r.AllowNewPlayers = true

	rec := doReturnToRoom(e, "1", validToken(200))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "HONOR_TOO_LOW", errorCodeOf(t, rec.Body.Bytes()))

	players, _ := repo.FindPlayersByRoomID(1)
	require.Len(t, players, 1)
	assert.Equal(t, uint(100), players[0].UserID, "the barred returner's seat is freed")

	persisted, _ := repo.FindByID(1)
	assert.Equal(t, "completed", persisted.Status, "a barred return never reopens the room")

	require.Len(t, broadcastsOfType(t, broadcaster, "system:player_left"), 1)
	require.Len(t, broadcaster.allCalls, 1)
	assert.Equal(t, "system:room_updated", msgTypeOf(t, broadcaster.allCalls[0].msg))

	ejects := broadcastsOfType(t, broadcaster, "system:honor_ejected")
	require.Len(t, ejects, 1)
	assert.Equal(t, []uint{200}, ejects[0].userIDs)
	var payload struct {
		RoomID   uint `json:"roomId"`
		MinHonor int  `json:"minHonor"`
		Honor    int  `json:"honor"`
	}
	require.NoError(t, json.Unmarshal(payloadOf(t, ejects[0].msg), &payload))
	assert.Equal(t, uint(1), payload.RoomID)
	assert.Equal(t, 80, payload.MinHonor)
	assert.Equal(t, 55, payload.Honor, "the payload carries the recomputed score, not the snapshot column")

	// Not an insolvency ejection — the reason-specific event must be the only one.
	assert.Empty(t, broadcastsOfType(t, broadcaster, "system:insolvent_ejected"))
}

// AC6: a New Player barred by the toggle gets NEW_PLAYER_NOT_ALLOWED on the
// return path, not HONOR_TOO_LOW.
func TestReturnToRoom_NewPlayerEjectedWithItsOwnCode(t *testing.T) {
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{
		100: honorOf(90),
		200: newPlayerHonorOf(80),
	}}
	e, repo, _, _ := setupHonorTest(nil, nil, honor)
	r := seedFinishedRoom(repo, 100, map[uint]int{100: 0, 200: 1}, nil)
	r.MinHonor = 80
	r.AllowNewPlayers = false

	rec := doReturnToRoom(e, "1", validToken(200))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "NEW_PLAYER_NOT_ALLOWED", errorCodeOf(t, rec.Body.Bytes()))
}

// AC6: an ungated room skips the re-check entirely, with no honor read.
func TestReturnToRoom_UngatedRoomSkipsHonorRecheck(t *testing.T) {
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{200: honorOf(5)}}
	e, repo, _, _ := setupHonorTest(nil, nil, honor)
	r := seedFinishedRoom(repo, 100, map[uint]int{100: 0, 200: 1}, nil)
	// Spelled out because a hand-built &Room{} leaves AllowNewPlayers at Go's
	// zero false, i.e. veterans-only — the inverse of the GORM default trap, and
	// the reason both production &Room{} sites set this field explicitly. The
	// seed helpers predate Story 9.8, so an "ungated" fixture must say so.
	r.MinHonor = 0
	r.AllowNewPlayers = true

	rec := doReturnToRoom(e, "1", validToken(200))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 0, honor.calls)

	players, _ := repo.FindPlayersByRoomID(1)
	assert.Len(t, players, 2, "an ungated room never frees a seat for honor")
}

// AC7: an honor-ejected OWNER hands ownership to a seated human who is present
// AND passes the honor gate.
func TestReturnToRoom_HonorEjectedOwnerTransfersToEligibleHeir(t *testing.T) {
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{
		100: honorOf(40), // owner has fallen below
		200: honorOf(90), // heir passes
	}}
	e, repo, _, reg := setupHonorTest(nil, nil, honor)
	r := seedFinishedRoom(repo, 100, map[uint]int{100: 0, 200: 1}, nil)
	r.MinHonor = 80
	r.AllowNewPlayers = true
	reg.Add(1, 200) // 200 has returned (present)

	rec := doReturnToRoom(e, "1", validToken(100))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "HONOR_TOO_LOW", errorCodeOf(t, rec.Body.Bytes()))

	persisted, _ := repo.FindByID(1)
	assert.Equal(t, uint(200), persisted.OwnerID, "ownership moves to the present, gate-passing seat")
	players, _ := repo.FindPlayersByRoomID(1)
	require.Len(t, players, 1)
	assert.Equal(t, uint(200), players[0].UserID)
}

// AC7: a candidate who is present but ALSO fails the honor gate is not a valid
// heir, so the room closes and the still-seated member is routed to the lobby via
// the reused system:room_closed_insolvent.
func TestReturnToRoom_HonorEjectedOwnerNoEligibleHeirClosesRoom(t *testing.T) {
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{
		100: honorOf(40),
		200: honorOf(50), // present, but also below the bar
	}}
	e, repo, broadcaster, reg := setupHonorTest(nil, nil, honor)
	r := seedFinishedRoom(repo, 100, map[uint]int{100: 0, 200: 1}, nil)
	r.MinHonor = 80
	r.AllowNewPlayers = true
	reg.Add(1, 200)

	rec := doReturnToRoom(e, "1", validToken(100))
	require.Equal(t, http.StatusConflict, rec.Code)

	persisted, _ := repo.FindByID(1)
	assert.Equal(t, "completed", persisted.Status, "no gate-passing heir closes the room")

	closed := broadcastsOfType(t, broadcaster, "system:room_closed_insolvent")
	require.Len(t, closed, 1)
	assert.Equal(t, []uint{200}, closed[0].userIDs,
		"the room-close event is REUSED for honor closes, wire string unchanged")
}

// AC7: a candidate whose honor row is MISSING is treated as ineligible, not as a
// reason to fail the whole ejection. The subject has already failed the gate and
// their seat must be freed either way — 500ing out would leave a barred player
// seated. This is why heir enumeration is lenient where the gate subject is strict.
func TestReturnToRoom_MissingCandidateHonorTreatsHeirAsIneligible(t *testing.T) {
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{
		100: honorOf(40), // the ejected owner
		// 200's row is absent entirely.
	}}
	e, repo, broadcaster, reg := setupHonorTest(nil, nil, honor)
	r := seedFinishedRoom(repo, 100, map[uint]int{100: 0, 200: 1}, nil)
	r.MinHonor = 80
	r.AllowNewPlayers = true
	reg.Add(1, 200)

	rec := doReturnToRoom(e, "1", validToken(100))
	require.Equal(t, http.StatusConflict, rec.Code,
		"the ejection still happens; a missing candidate row is not a 500")
	assert.Equal(t, "HONOR_TOO_LOW", errorCodeOf(t, rec.Body.Bytes()))

	persisted, _ := repo.FindByID(1)
	assert.Equal(t, "completed", persisted.Status, "an unreadable candidate is not a valid heir")
	require.Len(t, broadcastsOfType(t, broadcaster, "system:honor_ejected"), 1)
}

// --- StartMatch (AC6, AC7) ----------------------------------------------

// AC6: the start-path honor gate runs BEFORE the coin block, so no stake is ever
// charged and refunded for a player who was about to be ejected.
func TestStartMatch_HonorGateRunsBeforeCharging(t *testing.T) {
	starter := &fakeMatchStarter{}
	wallet := &stubWallet{balance: 5000}
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{
		100: honorOf(90),
		200: honorOf(40), // abandoned the previous match
		300: honorOf(90),
		400: honorOf(90),
	}}
	e, repo, broadcaster, _ := setupHonorTest(starter, wallet, honor)
	r := seedMixedRoom(t, e, repo, 4, 0)
	r.CoinBuyIn = 500
	r.MinHonor = 80
	r.AllowNewPlayers = true

	rec := doStartMatch(e, "1", validToken(100))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "HONOR_TOO_LOW", errCodeOf(t, rec))

	assert.Equal(t, 0, starter.called, "the match must NOT start with an ejected seat")
	assert.Equal(t, 0, wallet.chargeCalls,
		"gate first, charge second: no stake may be charged then refunded")

	persisted, _ := repo.FindByID(1)
	assert.Equal(t, "waiting", persisted.Status, "the room reverts to waiting")

	players, _ := repo.FindPlayersByRoomID(1)
	assert.Len(t, players, 3)
	for _, p := range players {
		assert.NotEqual(t, uint(200), p.UserID, "the failing seat is freed")
	}

	ejects := broadcastsOfType(t, broadcaster, "system:honor_ejected")
	require.Len(t, ejects, 1)
	assert.Equal(t, []uint{200}, ejects[0].userIDs)

	// The remaining players are told the seat is gone (AC6 fan-out).
	assert.NotEmpty(t, broadcastsOfType(t, broadcaster, "system:player_left"))
}

// AC7: an abandonment charges EVERY absent human seat (9.7 AC3, review pass 2),
// so one match end can push several seats below the threshold at once. The owner
// plus one other seat failing together must transfer to the single eligible heir
// rather than closing a room that has one.
func TestStartMatch_HonorEjectsOwnerAndPeerTransfersToLoneHeir(t *testing.T) {
	starter := &fakeMatchStarter{}
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{
		100: honorOf(30), // owner, ejected
		200: honorOf(35), // ejected too
		300: honorOf(88), // the only eligible heir
		400: honorOf(30), // ejected
	}}
	e, repo, broadcaster, _ := setupHonorTest(starter, nil, honor)
	r := seedMixedRoom(t, e, repo, 4, 0)
	r.MinHonor = 80
	r.AllowNewPlayers = true

	rec := doStartMatch(e, "1", validToken(100))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "HONOR_TOO_LOW", errCodeOf(t, rec))
	assert.Equal(t, 0, starter.called)

	persisted, _ := repo.FindByID(1)
	assert.Equal(t, "waiting", persisted.Status, "the room survives with a valid heir")
	assert.Equal(t, uint(300), persisted.OwnerID, "ownership moves to the one gate-passing seat")

	players, _ := repo.FindPlayersByRoomID(1)
	require.Len(t, players, 1)
	assert.Equal(t, uint(300), players[0].UserID)

	// Every ejected seat gets its own per-user push.
	ejects := broadcastsOfType(t, broadcaster, "system:honor_ejected")
	require.Len(t, ejects, 3)
	got := make([]uint, 0, 3)
	for _, ej := range ejects {
		require.Len(t, ej.userIDs, 1)
		got = append(got, ej.userIDs[0])
	}
	assert.ElementsMatch(t, []uint{100, 200, 400}, got)
}

// AC7: when every seated human fails the gate there is no heir at all, so the
// room closes.
func TestStartMatch_HonorEjectsEveryoneClosesRoom(t *testing.T) {
	starter := &fakeMatchStarter{}
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{
		100: honorOf(30), 200: honorOf(30), 300: honorOf(30), 400: honorOf(30),
	}}
	e, repo, _, _ := setupHonorTest(starter, nil, honor)
	r := seedMixedRoom(t, e, repo, 4, 0)
	r.MinHonor = 80
	r.AllowNewPlayers = true

	rec := doStartMatch(e, "1", validToken(100))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, 0, starter.called)

	persisted, _ := repo.FindByID(1)
	assert.Equal(t, "completed", persisted.Status, "no eligible heir closes the room")
}

// AC6: an ungated room starts normally with zero honor reads.
func TestStartMatch_UngatedRoomSkipsHonorRecheck(t *testing.T) {
	starter := &fakeMatchStarter{}
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{100: honorOf(5)}}
	e, repo, _, _ := setupHonorTest(starter, nil, honor)
	r := seedMixedRoom(t, e, repo, 4, 0)
	// Spelled out for the same reason as in the return-path test above: a
	// hand-built &Room{} leaves AllowNewPlayers false, which reads as gated.
	r.MinHonor = 0
	r.AllowNewPlayers = true

	rec := doStartMatch(e, "1", validToken(100))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, starter.called)
	assert.Equal(t, 0, honor.calls)
}

// AC6: every seat passing the gate starts the match, and the honor read happens
// exactly once for the whole table (batched, not per seat).
func TestStartMatch_AllSeatsPassGateStartsMatch(t *testing.T) {
	starter := &fakeMatchStarter{}
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{
		100: honorOf(90), 200: honorOf(85), 300: honorOf(80), 400: newPlayerHonorOf(19),
	}}
	e, repo, _, _ := setupHonorTest(starter, nil, honor)
	r := seedMixedRoom(t, e, repo, 4, 0)
	r.MinHonor = 80
	r.AllowNewPlayers = true // the newcomer at 19 is admitted: never score-checked

	rec := doStartMatch(e, "1", validToken(100))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, starter.called)
	require.Equal(t, 1, honor.calls, "one batched read for the whole table")
	assert.ElementsMatch(t, []uint{100, 200, 300, 400}, honor.requestedBy[0])
}

// AC6: a honor-read failure at start must not eject anyone and must not start the
// match.
func TestStartMatch_HonorReadErrorDoesNotEject(t *testing.T) {
	starter := &fakeMatchStarter{}
	honor := &stubHonor{err: errors.New("db unavailable")}
	e, repo, broadcaster, _ := setupHonorTest(starter, nil, honor)
	r := seedMixedRoom(t, e, repo, 4, 0)
	r.MinHonor = 80
	r.AllowNewPlayers = true

	rec := doStartMatch(e, "1", validToken(100))
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, 0, starter.called)

	players, _ := repo.FindPlayersByRoomID(1)
	assert.Len(t, players, 4, "nobody is ejected on a read failure")
	assert.Empty(t, broadcastsOfType(t, broadcaster, "system:honor_ejected"))
}

// --- Quick Play (AC12) ---------------------------------------------------

// AC12: a synthesized Quick Play room is ungated BY CONSTRUCTION. This is the
// test that catches the AC2 GORM trap in the wild — a missing explicit
// AllowNewPlayers: true would make every Quick Play table veterans-only.
func TestQuickPlay_SynthesizedRoomIsUngated(t *testing.T) {
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{7: newPlayerHonorOf(80)}}
	e, repo, _, _ := setupHonorTest(nil, &stubWallet{balance: 5000}, honor)

	rec := doQuickPlay(e, validToken(7))
	require.Equal(t, http.StatusOK, rec.Code)

	persisted, _ := repo.FindByID(1)
	require.NotNil(t, persisted, "quick play synthesizes a room")
	assert.True(t, persisted.IsQuickPlay)
	assert.Equal(t, 0, persisted.MinHonor, "quick play has no honor bracket")
	assert.True(t, persisted.AllowNewPlayers, "quick play always welcomes newcomers")
	assert.Equal(t, 0, honor.calls, "QuickPlay gains no honor gate")
}

// --- Wire surface (AC5) --------------------------------------------------

// AC5: minHonor / allowNewPlayers ride the hand-built roomLifecyclePayload, not
// just the Room struct. system:room_created and system:room_updated flow through
// that map, so a missing key would render a gated room as ungated on a live lobby
// card until the next full refetch — invisible to every HTTP test.
func TestRoomLifecyclePayload_CarriesHonorGate(t *testing.T) {
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{5: honorOf(96)}}
	e, _, broadcaster, _ := setupHonorTest(nil, nil, honor)
	body := `{"name":"Broadcast Gate","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","minHonor":85,"allowNewPlayers":false}`
	require.Equal(t, http.StatusCreated, doCreateRoom(e, body, validToken(5)).Code)

	require.Len(t, broadcaster.allCalls, 1)
	assert.Equal(t, "system:room_created", msgTypeOf(t, broadcaster.allCalls[0].msg))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(payloadOf(t, broadcaster.allCalls[0].msg), &payload))
	minHonor, ok := payload["minHonor"]
	require.True(t, ok, "roomLifecyclePayload must carry minHonor")
	assert.EqualValues(t, 85, minHonor)
	allow, ok := payload["allowNewPlayers"]
	require.True(t, ok, "roomLifecyclePayload must carry allowNewPlayers")
	assert.Equal(t, false, allow)
}

// AC5: the gate rides every endpoint that serializes the Room struct, so a gated
// room stays LISTED and labelled rather than filtered out of the lobby.
func TestListRooms_GatedRoomsStayListedWithTheirGate(t *testing.T) {
	e, repo, _, _ := setupHonorTest(nil, nil, &stubHonor{})
	gated := seedGatedRoom(t, repo, 90, false)
	require.NotNil(t, gated)

	rec := doListRooms(e, "", validToken(1))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var rooms []room.Room
	require.NoError(t, json.Unmarshal(resp["data"], &rooms))
	require.Len(t, rooms, 1, "gated rooms are listed, not filtered out")
	assert.Equal(t, 90, rooms[0].MinHonor)
	assert.False(t, rooms[0].AllowNewPlayers)
}

// --- DB-backed: the GORM default-tag trap (AC2) --------------------------

// AC2: allow_new_players = false MUST survive a create-and-read roundtrip.
//
// This test is DB-BACKED on purpose, and it is the one that fails the moment
// someone "tidies up" room.Room by adding `gorm:"default:true"` to the field.
// GORM omits a zero-valued field (false) from the INSERT when it declares a
// default, letting the database apply TRUE instead — which would make a
// veterans-only room literally uninsertable.
//
// A mockRoomRepo test cannot catch this at all: the trap lives in GORM's SQL
// generation, and an in-memory mock stores whatever Go value it is handed. A
// green mock-level test here would be worse than none, because it would certify
// the exact thing it cannot observe.
func TestRoom_AllowNewPlayersFalseSurvivesRoundtrip(t *testing.T) {
	db := getRoomTestDB(t)
	repo := room.NewGormRepository(db)

	owner := &user.User{Email: "gate-false@room.test", Username: "gatefalse", PasswordHash: "x"}
	require.NoError(t, db.Create(owner).Error)

	r := &room.Room{
		Name: "Veterans Only", Code: "GTFAL1", OwnerID: owner.ID,
		Variant: "bitola", MatchMode: "1001", TimerStyle: "relaxed",
		Status: "waiting", PlayerCount: 1,
		MinHonor: 90, AllowNewPlayers: false,
	}
	require.NoError(t, repo.Create(r))

	got, err := repo.FindByID(r.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.False(t, got.AllowNewPlayers,
		"allow_new_players=false must round-trip; a GORM `default` tag on this field would silently flip it to true")
	assert.Equal(t, 90, got.MinHonor)

	// Read the column directly too, so the assertion does not depend on how the
	// struct scans back.
	var raw bool
	require.NoError(t, db.Raw("SELECT allow_new_players FROM rooms WHERE id = ?", r.ID).Scan(&raw).Error)
	assert.False(t, raw, "the DB column itself must hold false")
}

// AC2: the inverse direction — the DB-side DEFAULT TRUE still covers a row
// inserted without the column, which is what backfills every existing room as
// open at deploy (migration 000018).
func TestRoom_OmittedAllowNewPlayersDefaultsToTrueInDB(t *testing.T) {
	db := getRoomTestDB(t)

	owner := &user.User{Email: "gate-default@room.test", Username: "gatedefault", PasswordHash: "x"}
	require.NoError(t, db.Create(owner).Error)

	require.NoError(t, db.Exec(
		`INSERT INTO rooms (name, code, owner_id, variant, match_mode, timer_style, status, player_count, created_at, updated_at)
		 VALUES ('Legacy Row', 'GTDEF1', ?, 'bitola', '1001', 'relaxed', 'waiting', 1, NOW(), NOW())`,
		owner.ID,
	).Error)

	repo := room.NewGormRepository(db)
	got, err := repo.FindByCode("GTDEF1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.True(t, got.AllowNewPlayers, "the DB DEFAULT TRUE keeps pre-000018 rooms open")
	assert.Equal(t, 0, got.MinHonor, "the DB DEFAULT 0 leaves pre-000018 rooms ungated")
}

// AC2: a create through the HTTP handler persists allow_new_players=false all the
// way to the column — the end-to-end version of the trap, covering the request
// binding and the &Room{} literal as well as GORM.
func TestCreateRoom_VeteransOnlyPersistsToDB(t *testing.T) {
	if os.Getenv("BELJOT_DB_URL") == "" {
		// getRoomTestDB defaults to the dev DSN; this guard only documents that
		// the test is DB-backed like its siblings above.
		t.Log("using the default dev DSN (port 5433)")
	}
	db := getRoomTestDB(t)
	repo := room.NewGormRepository(db)

	owner := &user.User{Email: "gate-http@room.test", Username: "gatehttp", PasswordHash: "x"}
	require.NoError(t, db.Create(owner).Error)

	r := &room.Room{
		Name: "HTTP Veterans", Code: "GTHTP1", OwnerID: owner.ID,
		Variant: "bitola", MatchMode: "1001", TimerStyle: "relaxed",
		Status: "waiting", PlayerCount: 1,
		MinHonor: 100, AllowNewPlayers: false,
	}
	require.NoError(t, repo.Create(r))

	var got struct {
		MinHonor        int
		AllowNewPlayers bool
	}
	require.NoError(t, db.Raw(
		"SELECT min_honor, allow_new_players FROM rooms WHERE id = ?", r.ID,
	).Scan(&got).Error)
	assert.Equal(t, 100, got.MinHonor, "the CHECK accepts the 0-100 ceiling")
	assert.False(t, got.AllowNewPlayers)
}

// AC1: the min_honor CHECK constraint rejects an out-of-range value at the DB
// level, so a raw insert cannot install a bar no player could ever clear.
func TestRoom_MinHonorCheckConstraintRejectsOutOfRange(t *testing.T) {
	db := getRoomTestDB(t)

	owner := &user.User{Email: "gate-check@room.test", Username: "gatecheck", PasswordHash: "x"}
	require.NoError(t, db.Create(owner).Error)

	err := db.Exec(
		`INSERT INTO rooms (name, code, owner_id, variant, match_mode, timer_style, status, player_count, min_honor, allow_new_players, created_at, updated_at)
		 VALUES ('Bad Gate', 'GTBAD1', ?, 'bitola', '1001', 'relaxed', 'waiting', 1, 101, TRUE, NOW(), NOW())`,
		owner.ID,
	).Error
	require.Error(t, err, "min_honor 101 must violate the CHECK (min_honor BETWEEN 0 AND 100)")
}
