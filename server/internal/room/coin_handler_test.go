package room_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/auth"
	"github.com/emilijan/beljot/server/internal/room"
)

// stubWallet implements room.WalletService for the coin-economy handler tests.
type stubWallet struct {
	balance         int
	balanceErr      error
	chargeErr       error
	chargeInsolvent uint
	// balances overrides per-user balances for GetBalances; when nil every
	// requested userID resolves to `balance` (uniform-wallet shorthand). Used
	// by Story 9.3 ejection tests to make some seats insolvent and others not.
	balances    map[uint]int
	balancesErr error

	chargeCalls   int
	chargedIDs    []uint
	chargedAmount int

	settleCalls    int
	settledCredits map[uint]int
}

func (s *stubWallet) GetBalance(userID uint) (int, error) {
	// Mirror the real wallet: a per-user balance (when the `balances` map is set)
	// is authoritative for that user; otherwise fall back to the uniform scalar.
	// Keeps GetBalance and GetBalances consistent so the join-time bracket read
	// and the charge-time prefilter agree on a user's balance.
	if s.balances != nil {
		if b, ok := s.balances[userID]; ok {
			return b, s.balanceErr
		}
	}
	return s.balance, s.balanceErr
}

func (s *stubWallet) GetBalances(userIDs []uint) (map[uint]int, error) {
	if s.balancesErr != nil {
		return nil, s.balancesErr
	}
	out := make(map[uint]int, len(userIDs))
	for _, id := range userIDs {
		if s.balances != nil {
			out[id] = s.balances[id]
		} else {
			out[id] = s.balance
		}
	}
	return out, nil
}

func (s *stubWallet) ChargeStakes(userIDs []uint, amount int) (uint, error) {
	s.chargeCalls++
	s.chargedIDs = append([]uint(nil), userIDs...)
	s.chargedAmount = amount
	return s.chargeInsolvent, s.chargeErr
}

func (s *stubWallet) ApplySettlement(credits map[uint]int) error {
	s.settleCalls++
	s.settledCredits = credits
	return nil
}

// setupCoinTest wires the room handler with an injected wallet stub and starter.
func setupCoinTest(starter room.MatchStarter, wallet room.WalletService) (*echo.Echo, *mockRoomRepo) {
	repo := newMockRoomRepo()
	handler := room.NewRoomHandler(repo, starter, &mockBroadcaster{}, nil, wallet)

	e := echo.New()
	e.HTTPErrorHandler = testErrorHandler
	api := e.Group("/api/v1", auth.AuthMiddleware("test-jwt-secret"))
	api.POST("/rooms", handler.CreateRoom)
	api.POST("/rooms/quick-play", handler.QuickPlay)
	api.POST("/rooms/:id/join", handler.JoinRoom)
	api.POST("/rooms/:id/start", handler.StartMatch)
	api.POST("/rooms/:id/bots", handler.AddBot)
	return e, repo
}

// setupCoinTestBC is setupCoinTest with the full route set plus an observable
// broadcaster and a caller-controlled presence registry — for Story 9.3 tests
// that assert WS fan-out (insolvent_ejected, room_closed_insolvent,
// match_start_failed) and exercise the return / leave paths.
func setupCoinTestBC(starter room.MatchStarter, wallet room.WalletService) (*echo.Echo, *mockRoomRepo, *mockBroadcaster, *room.PresenceRegistry) {
	repo := newMockRoomRepo()
	broadcaster := &mockBroadcaster{}
	reg := room.NewPresenceRegistry()
	handler := room.NewRoomHandler(repo, starter, broadcaster, reg, wallet)

	e := echo.New()
	e.HTTPErrorHandler = testErrorHandler
	api := e.Group("/api/v1", auth.AuthMiddleware("test-jwt-secret"))
	registerRoomRoutes(api, handler)
	return e, repo, broadcaster, reg
}

// broadcastsOfType returns every per-user broadcast (BroadcastToUsers) whose
// message type matches, with the recipient userIDs preserved — used to assert
// per-user pushes like system:insolvent_ejected.
func broadcastsOfType(t *testing.T, b *mockBroadcaster, msgType string) []broadcastCall {
	t.Helper()
	var out []broadcastCall
	for _, c := range b.calls {
		if msgTypeOf(t, c.msg) == msgType {
			out = append(out, c)
		}
	}
	return out
}

// --- Create-room buy-in (AC #1) ---

func TestCreateRoom_DefaultsBuyInTo500(t *testing.T) {
	e, repo := setupTest()
	body := `{"name":"Default Stake","variant":"bitola","matchMode":"1001","timerStyle":"relaxed"}`
	rec := doCreateRoom(e, body, validToken(5))
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var data room.Room
	require.NoError(t, json.Unmarshal(resp["data"], &data))
	assert.Equal(t, 500, data.CoinBuyIn)

	persisted, _ := repo.FindByID(data.ID)
	require.NotNil(t, persisted)
	assert.Equal(t, 500, persisted.CoinBuyIn)
}

func TestCreateRoom_ExplicitBuyInPersisted(t *testing.T) {
	e, repo := setupTest()
	body := `{"name":"High Roller","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","coinBuyIn":2500}`
	rec := doCreateRoom(e, body, validToken(5))
	require.Equal(t, http.StatusCreated, rec.Code)

	persisted, _ := repo.FindByID(1)
	require.NotNil(t, persisted)
	assert.Equal(t, 2500, persisted.CoinBuyIn)
}

func TestCreateRoom_ExplicitZeroBuyInAllowed(t *testing.T) {
	e, _ := setupTest()
	body := `{"name":"Free Table","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","coinBuyIn":0}`
	rec := doCreateRoom(e, body, validToken(5))
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var data room.Room
	require.NoError(t, json.Unmarshal(resp["data"], &data))
	assert.Equal(t, 0, data.CoinBuyIn, "explicit 0 must survive (not be replaced by default 500)")
}

func TestCreateRoom_NegativeBuyInRejected(t *testing.T) {
	e, _ := setupTest()
	body := `{"name":"Bad Stake","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","coinBuyIn":-5}`
	rec := doCreateRoom(e, body, validToken(5))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "BAD_REQUEST", errCodeOf(t, rec))
}

func TestCreateRoom_InsufficientCoinsRejected(t *testing.T) {
	// Creator is auto-seated and charged at start, so a buy-in above their
	// balance is rejected at create time (mirrors the join check).
	e, repo := setupCoinTest(nil, &stubWallet{balance: 100})
	body := `{"name":"High Roller","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","coinBuyIn":500}`
	rec := doCreateRoom(e, body, validToken(5))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "INSUFFICIENT_COINS", errCodeOf(t, rec))

	persisted, _ := repo.FindByID(1)
	assert.Nil(t, persisted, "no room is created when the creator can't afford the buy-in")
}

func TestCreateRoom_SufficientCoinsAllowed(t *testing.T) {
	e, repo := setupCoinTest(nil, &stubWallet{balance: 500})
	body := `{"name":"Affordable","variant":"bitola","matchMode":"1001","timerStyle":"relaxed","coinBuyIn":500}`
	rec := doCreateRoom(e, body, validToken(5))
	require.Equal(t, http.StatusCreated, rec.Code)

	persisted, _ := repo.FindByID(1)
	require.NotNil(t, persisted)
	assert.Equal(t, 500, persisted.CoinBuyIn)
}

// Story 9.4 (AC1, AC5): a synthesized quick-play room carries the caller's
// affordability bracket as its stake — 500 for a player who can afford it, 0
// for a player who cannot (including exactly 0) — and always defaults to the
// per-move 30s timer regardless of bracket. (Supersedes 9.2's "quick-play is
// always free" TestQuickPlay_RoomIsFree.)
func TestQuickPlay_SynthesizedRoomCarriesBracketAndTimer(t *testing.T) {
	tests := []struct {
		name      string
		balance   int
		wantBuyIn int
	}{
		{"affordable player pools at 500", 5000, 500},
		{"exactly at threshold pools at 500", 500, 500},
		{"just under threshold pools free", 499, 0},
		{"zero-coin player pools free", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, repo := setupCoinTest(&fakeMatchStarter{}, &stubWallet{balance: tt.balance})
			rec := doQuickPlay(e, validToken(7))
			require.Equal(t, http.StatusOK, rec.Code)

			// Quick-play synthesizes a brand-new room when none is available.
			persisted, _ := repo.FindByID(1)
			require.NotNil(t, persisted)
			assert.True(t, persisted.IsQuickPlay)
			assert.Equal(t, tt.wantBuyIn, persisted.CoinBuyIn, "stake matches the caller's bracket")
			assert.Equal(t, "per-move", persisted.TimerStyle, "AC5: per-move timer on every synthesized room")
			require.NotNil(t, persisted.TimerDurationSeconds)
			assert.Equal(t, 30, *persisted.TimerDurationSeconds)
		})
	}
}

// Story 9.4 (AC4): tapping a quick-play room in the wrong coin bracket is
// rejected with QUICK_PLAY_BRACKET_MISMATCH and the player is never seated; a
// matching bracket seats normally.
func TestQuickJoin_CrossBracketRejected(t *testing.T) {
	tests := []struct {
		name          string
		roomBuyIn     int
		callerBalance int
		wantStatus    int
		wantCode      string
	}{
		{"free player taps 500 table", 500, 100, http.StatusConflict, "QUICK_PLAY_BRACKET_MISMATCH"},
		{"affordable player taps free table", 0, 5000, http.StatusConflict, "QUICK_PLAY_BRACKET_MISMATCH"},
		{"matching 500 bracket seats", 500, 5000, http.StatusOK, ""},
		{"matching free bracket seats", 0, 100, http.StatusOK, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, repo, _, _ := setupCoinTestBC(&fakeMatchStarter{}, &stubWallet{balance: tt.callerBalance})
			r := &room.Room{
				Name: "QP Table", Code: "QPJOIN", OwnerID: 20, Variant: "bitola", MatchMode: "1001",
				TimerStyle: "per-move", IsQuickPlay: true, Status: "waiting", PlayerCount: 1,
				CoinBuyIn: tt.roomBuyIn,
			}
			require.NoError(t, repo.Create(r))
			seat := 0
			team := teamNameForSeat(0)
			require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 20, Username: "owner", Seat: &seat, Team: &team}))

			rec := doQuickJoin(e, "1", validToken(30))
			require.Equal(t, tt.wantStatus, rec.Code)

			persisted, _ := repo.FindByID(1)
			if tt.wantCode != "" {
				assert.Equal(t, tt.wantCode, errCodeOf(t, rec))
				assert.Equal(t, 1, persisted.PlayerCount, "rejected tap must not seat the player")
			} else {
				assert.Equal(t, 2, persisted.PlayerCount, "matching bracket seats the player")
			}
		})
	}
}

// seedQuickPlayRoomWithStake creates a waiting quick-play room with the given
// coin bracket + per-move/30s timer and seats the listed humans at the lowest
// seats — mirroring the state a partially-filled Story 9.4 quick-play room is
// left in. The first ID is the owner. Returns the room.
func seedQuickPlayRoomWithStake(t *testing.T, repo *mockRoomRepo, code string, buyIn int, seatedUserIDs ...uint) *room.Room {
	t.Helper()
	dur := 30
	r := &room.Room{
		Name:                 "Quick Play " + code,
		Code:                 code,
		OwnerID:              seatedUserIDs[0],
		Variant:              "bitola",
		MatchMode:            "1001",
		TimerStyle:           "per-move",
		TimerDurationSeconds: &dur,
		IsQuickPlay:          true,
		Status:               "waiting",
		PlayerCount:          len(seatedUserIDs),
		CoinBuyIn:            buyIn,
	}
	require.NoError(t, repo.Create(r))
	for i, uid := range seatedUserIDs {
		seat := i
		team := teamNameForSeat(i)
		require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: uid, Username: "H", Seat: &seat, Team: &team}))
	}
	return r
}

// Story 9.4 (AC1): matchmaking keeps the two affordability pools strictly
// separate — QuickPlay never seats a player into a room of the other bracket;
// it synthesizes a fresh same-bracket room instead.
func TestQuickPlay_BracketsKeepPoolsSeparate(t *testing.T) {
	tests := []struct {
		name             string
		existingBuyIn    int
		callerBalance    int
		wantJoinExisting bool
		wantNewBuyIn     int
	}{
		{"free player skips a 500 room", 500, 100, false, 0},
		{"500 player skips a free room", 0, 5000, false, 500},
		{"500 player joins a 500 room", 500, 5000, true, 500},
		{"free player joins a free room", 0, 100, true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, repo := setupCoinTest(&fakeMatchStarter{}, &stubWallet{balance: tt.callerBalance})
			// A waiting, not-full quick-play room (1 seated) in the existing bracket.
			existing := seedQuickPlayRoomWithStake(t, repo, "EXQP01", tt.existingBuyIn, 20)

			rec := doQuickPlay(e, validToken(30))
			require.Equal(t, http.StatusOK, rec.Code)

			reloaded, _ := repo.FindByID(existing.ID)
			newRoom, _ := repo.FindByID(2)
			if tt.wantJoinExisting {
				assert.Equal(t, 2, reloaded.PlayerCount, "joined the same-bracket room")
				assert.Nil(t, newRoom, "no new room synthesized when a same-bracket room exists")
			} else {
				assert.Equal(t, 1, reloaded.PlayerCount, "cross-bracket room left untouched")
				require.NotNil(t, newRoom, "a new same-bracket room is synthesized")
				assert.True(t, newRoom.IsQuickPlay)
				assert.Equal(t, tt.wantNewBuyIn, newRoom.CoinBuyIn)
			}
		})
	}
}

// Story 9.4 (AC2, AC5): when the 4th joiner fills a default-bracket (500) quick-
// play room, the auto-start path charges all four humans the stake atomically,
// threads the stake into StartMatch (so 9.2 settlement applies), and forwards
// the per-move 30s timer.
func TestQuickJoin_AutoStartChargesDefaultBracket(t *testing.T) {
	starter := &fakeMatchStarter{}
	wallet := &stubWallet{balance: 5000} // all four afford 500
	e, repo, broadcaster, _ := setupCoinTestBC(starter, wallet)
	seedQuickPlayRoomWithStake(t, repo, "QP500A", 500, 100, 200, 300)

	rec := doQuickJoin(e, "1", validToken(400))
	require.Equal(t, http.StatusOK, rec.Code)

	require.Equal(t, 1, wallet.chargeCalls)
	assert.Equal(t, 500, wallet.chargedAmount)
	assert.ElementsMatch(t, []uint{100, 200, 300, 400}, wallet.chargedIDs, "all four humans pay the stake")

	require.Equal(t, 1, starter.called)
	assert.Equal(t, 500, starter.lastCoinBuyIn, "the bracket stake threads into the session for settlement")
	assert.Equal(t, "per-move", starter.lastTimerStyle, "AC5: timer style reaches StartMatch on auto-start")
	assert.Equal(t, 30, starter.lastTimerDurationSec, "AC5: per-move 30s duration reaches StartMatch")

	persisted, _ := repo.FindByID(1)
	assert.Equal(t, "playing", persisted.Status)
	assert.Empty(t, broadcastsOfType(t, broadcaster, "error:match_start_failed"))
}

// Story 9.4 (AC3): a free-bracket (0) quick-play match auto-starts with no
// charge and no settlement — exactly as before the economy work, since the
// charge block is skipped when CoinBuyIn == 0.
func TestQuickJoin_AutoStartFreeBracketNoCharge(t *testing.T) {
	starter := &fakeMatchStarter{}
	wallet := &stubWallet{balance: 0} // free bracket
	e, repo, _, _ := setupCoinTestBC(starter, wallet)
	seedQuickPlayRoomWithStake(t, repo, "QPFREE", 0, 100, 200, 300)

	rec := doQuickJoin(e, "1", validToken(400))
	require.Equal(t, http.StatusOK, rec.Code)

	assert.Equal(t, 0, wallet.chargeCalls, "free bracket charges nothing")
	assert.Equal(t, 0, wallet.settleCalls, "free bracket settles nothing")
	require.Equal(t, 1, starter.called)
	assert.Equal(t, 0, starter.lastCoinBuyIn)

	persisted, _ := repo.FindByID(1)
	assert.Equal(t, "playing", persisted.Status)
}

// Story 9.4 (Task 5 safety net): if a seat is insolvent at charge time on the
// auto-start path (a near-impossible edge), the eject path frees that seat,
// reverts the room to waiting, pushes system:insolvent_ejected to the ejected
// player only, and the match does not start. Critically, the generic
// error:match_start_failed broadcast must NOT fire — the insolvency was handled.
func TestQuickJoin_AutoStartInsolventSeatEjected(t *testing.T) {
	starter := &fakeMatchStarter{}
	wallet := &stubWallet{balances: map[uint]int{100: 5000, 200: 0, 300: 5000, 400: 5000}}
	e, repo, broadcaster, _ := setupCoinTestBC(starter, wallet)
	seedQuickPlayRoomWithStake(t, repo, "QP500B", 500, 100, 200, 300)

	rec := doQuickJoin(e, "1", validToken(400))
	require.Equal(t, http.StatusOK, rec.Code, "the solvent joiner seats fine; auto-start handles the insolvent seat")

	assert.Equal(t, 0, starter.called, "the match must not start with an unpaid stake")
	assert.Equal(t, 0, wallet.chargeCalls, "the prefilter ejects before any charge")

	persisted, _ := repo.FindByID(1)
	assert.Equal(t, "waiting", persisted.Status, "room reverts to waiting after eject")

	players, _ := repo.FindPlayersByRoomID(1)
	for _, p := range players {
		assert.NotEqual(t, uint(200), p.UserID, "insolvent seat must be freed")
	}

	ejects := broadcastsOfType(t, broadcaster, "system:insolvent_ejected")
	require.Len(t, ejects, 1)
	assert.Equal(t, []uint{200}, ejects[0].userIDs)

	assert.Empty(t, broadcastsOfType(t, broadcaster, "error:match_start_failed"),
		"insolvency is handled by the eject path; the generic failure broadcast must not fire")
}

// Story 9.4 (Task 5, review patch): the TOCTOU branch — every seat clears the
// prefilter (all >= 500), but the authoritative atomic ChargeStakes still
// surfaces ONE insolvent user (a balance drop racing the charge). That seat
// must be ejected and the room reverted, exactly like the prefilter path, but
// here the charge IS attempted first. Distinct from
// TestQuickJoin_AutoStartInsolventSeatEjected, where the prefilter ejects
// before any charge (chargeCalls == 0).
func TestQuickJoin_AutoStartChargeTOCTOUEjectsInsolventSeat(t *testing.T) {
	starter := &fakeMatchStarter{}
	// Prefilter sees all four solvent; the atomic charge then races and reports
	// user 200 insolvent (chargeInsolvent + ErrInsufficientCoins).
	wallet := &stubWallet{
		balances:        map[uint]int{100: 5000, 200: 5000, 300: 5000, 400: 5000},
		chargeErr:       apperr.ErrInsufficientCoins,
		chargeInsolvent: 200,
	}
	e, repo, broadcaster, _ := setupCoinTestBC(starter, wallet)
	seedQuickPlayRoomWithStake(t, repo, "QP500C", 500, 100, 200, 300)

	rec := doQuickJoin(e, "1", validToken(400))
	require.Equal(t, http.StatusOK, rec.Code, "the solvent joiner seats fine; auto-start handles the TOCTOU insolvency")

	assert.Equal(t, 1, wallet.chargeCalls, "the atomic charge is attempted (prefilter passed), unlike the prefilter-eject path")
	assert.Equal(t, 0, starter.called, "the match must not start with an unpaid stake")
	assert.Equal(t, 0, wallet.settleCalls, "nothing was charged (atomic rollback), so nothing to refund")

	persisted, _ := repo.FindByID(1)
	assert.Equal(t, "waiting", persisted.Status, "room reverts to waiting after the TOCTOU eject")

	players, _ := repo.FindPlayersByRoomID(1)
	for _, p := range players {
		assert.NotEqual(t, uint(200), p.UserID, "the TOCTOU-insolvent seat must be freed")
	}

	ejects := broadcastsOfType(t, broadcaster, "system:insolvent_ejected")
	require.Len(t, ejects, 1)
	assert.Equal(t, []uint{200}, ejects[0].userIDs)

	assert.Empty(t, broadcastsOfType(t, broadcaster, "error:match_start_failed"),
		"insolvency is handled by the eject path; the generic failure broadcast must not fire")
}

// Story 9.4 (Task 5): when the charge succeeds on the auto-start path but the
// session fails to start, the charged stakes are refunded (no coins destroyed),
// the room reverts to waiting, and error:match_start_failed is broadcast.
func TestQuickJoin_AutoStartRefundOnStartFailure(t *testing.T) {
	starter := &fakeMatchStarter{err: errors.New("session manager unavailable")}
	wallet := &stubWallet{balance: 5000}
	e, repo, broadcaster, _ := setupCoinTestBC(starter, wallet)
	seedQuickPlayRoomWithStake(t, repo, "QP500C", 500, 100, 200, 300)

	rec := doQuickJoin(e, "1", validToken(400))
	require.Equal(t, http.StatusOK, rec.Code)

	require.Equal(t, 1, starter.called)
	require.Equal(t, 1, wallet.chargeCalls, "stakes were charged before the start failed")
	require.Equal(t, 1, wallet.settleCalls, "charged stakes must be refunded")
	assert.Equal(t, map[uint]int{100: 500, 200: 500, 300: 500, 400: 500}, wallet.settledCredits)

	persisted, _ := repo.FindByID(1)
	assert.Equal(t, "waiting", persisted.Status, "room reverted to waiting, never stranded in playing")

	require.Len(t, broadcastsOfType(t, broadcaster, "error:match_start_failed"), 1)
	assert.Empty(t, broadcastsOfType(t, broadcaster, "system:match_started"))
}

// --- Join affordability check (AC #2) ---

func TestJoinRoom_InsufficientCoinsRejected(t *testing.T) {
	wallet := &stubWallet{balance: 300}
	e, repo := setupCoinTest(&fakeMatchStarter{}, wallet)
	// Owner 100 created a 500-stake room (one seat taken).
	r := &room.Room{Name: "Stake Room", OwnerID: 100, Status: "waiting", PlayerCount: 1, CoinBuyIn: 500}
	require.NoError(t, repo.Create(r))
	seat := 0
	team := teamNameForSeat(0)
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 100, Username: "owner", Seat: &seat, Team: &team}))

	rec := doJoinRoom(e, "1", validToken(200))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "INSUFFICIENT_COINS", errCodeOf(t, rec))

	// Not seated — player count unchanged, joiner absent.
	persisted, _ := repo.FindByID(1)
	assert.Equal(t, 1, persisted.PlayerCount)
	players, _ := repo.FindPlayersByRoomID(1)
	assert.Len(t, players, 1)
}

func TestJoinRoom_SufficientCoinsSeatsWithoutDeduction(t *testing.T) {
	wallet := &stubWallet{balance: 500}
	e, repo := setupCoinTest(&fakeMatchStarter{}, wallet)
	r := &room.Room{Name: "Stake Room", OwnerID: 100, Status: "waiting", PlayerCount: 1, CoinBuyIn: 500}
	require.NoError(t, repo.Create(r))
	seat := 0
	team := teamNameForSeat(0)
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 100, Username: "owner", Seat: &seat, Team: &team}))

	rec := doJoinRoom(e, "1", validToken(200))
	require.Equal(t, http.StatusOK, rec.Code)

	persisted, _ := repo.FindByID(1)
	assert.Equal(t, 2, persisted.PlayerCount)
	// Join is a check only — no charge happens at join.
	assert.Equal(t, 0, wallet.chargeCalls)
}

// --- Match-start charge (AC #4, #5) ---

func TestStartMatch_ChargesHumanSeatsNotBots(t *testing.T) {
	starter := &fakeMatchStarter{}
	wallet := &stubWallet{balance: 5000}
	e, repo := setupCoinTest(starter, wallet)
	// 3 humans (seats 0,1,2: users 100,200,300) + 1 bot (seat 3).
	r := seedMixedRoom(t, e, repo, 3, 1)
	r.CoinBuyIn = 500 // set the stake on the seeded room

	rec := doStartMatch(e, "1", validToken(100))
	require.Equal(t, http.StatusOK, rec.Code)

	require.Equal(t, 1, wallet.chargeCalls)
	assert.Equal(t, 500, wallet.chargedAmount)
	assert.ElementsMatch(t, []uint{100, 200, 300}, wallet.chargedIDs, "only the 3 humans are charged; the bot seat is never charged")

	require.Equal(t, 1, starter.called)
	assert.Equal(t, 500, starter.lastCoinBuyIn, "the buy-in is threaded into the session")
}

// Story 9.3 AC5: a single insolvent NON-owner seat is ejected (per-player, not
// a whole-table rollback); the seat is freed, system:insolvent_ejected is pushed
// to that player, the match does not start, the room reverts to waiting, and the
// owner gets a 409. ChargeStakes is never reached — the prefilter caught it.
func TestStartMatch_InsolventSeatEjected(t *testing.T) {
	starter := &fakeMatchStarter{}
	// 200 is insolvent; owner 100 + 300/400 are fine.
	wallet := &stubWallet{balances: map[uint]int{100: 5000, 200: 0, 300: 5000, 400: 5000}}
	e, repo, broadcaster, _ := setupCoinTestBC(starter, wallet)
	r := seedMixedRoom(t, e, repo, 4, 0)
	r.CoinBuyIn = 500

	rec := doStartMatch(e, "1", validToken(100))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "INSUFFICIENT_COINS", errCodeOf(t, rec))

	assert.Equal(t, 0, starter.called, "the match must NOT start with an unpaid stake")
	assert.Equal(t, 0, wallet.chargeCalls, "the prefilter ejects before any charge")

	persisted, _ := repo.FindByID(1)
	assert.Equal(t, "waiting", persisted.Status, "room reverts to waiting on insolvency")

	// The insolvent seat is freed; the three solvent humans remain.
	players, _ := repo.FindPlayersByRoomID(1)
	assert.Len(t, players, 3)
	for _, p := range players {
		assert.NotEqual(t, uint(200), p.UserID, "insolvent seat must be freed")
	}

	// system:insolvent_ejected pushed only to the ejected player.
	ejects := broadcastsOfType(t, broadcaster, "system:insolvent_ejected")
	require.Len(t, ejects, 1)
	assert.Equal(t, []uint{200}, ejects[0].userIDs)
	var payload struct {
		RoomID  uint `json:"roomId"`
		BuyIn   int  `json:"buyIn"`
		Balance int  `json:"balance"`
	}
	require.NoError(t, json.Unmarshal(payloadOf(t, ejects[0].msg), &payload))
	assert.Equal(t, uint(1), payload.RoomID)
	assert.Equal(t, 500, payload.BuyIn)
	assert.Equal(t, 0, payload.Balance)
}

// Story 9.3 AC5: when the OWNER is the insolvent seat and a solvent seated human
// remains, ownership transfers to the first such seat and the room reverts to
// waiting. Presence is cleared at start, so eligibility is solvent-only.
func TestStartMatch_InsolventOwnerTransfersOwnership(t *testing.T) {
	starter := &fakeMatchStarter{}
	wallet := &stubWallet{balances: map[uint]int{100: 0, 200: 5000, 300: 5000, 400: 5000}}
	e, repo, _, _ := setupCoinTestBC(starter, wallet)
	r := seedMixedRoom(t, e, repo, 4, 0)
	r.CoinBuyIn = 500

	rec := doStartMatch(e, "1", validToken(100))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, 0, starter.called)

	persisted, _ := repo.FindByID(1)
	assert.Equal(t, "waiting", persisted.Status)
	assert.Equal(t, uint(200), persisted.OwnerID, "ownership transfers to first solvent seated human")
}

// Story 9.3 AC5: insolvent owner with no solvent human heir (only bots remain)
// closes the room.
func TestStartMatch_InsolventOwnerNoHeirClosesRoom(t *testing.T) {
	starter := &fakeMatchStarter{}
	wallet := &stubWallet{balances: map[uint]int{100: 0}}
	e, repo, _, _ := setupCoinTestBC(starter, wallet)
	r := seedMixedRoom(t, e, repo, 1, 3) // lone human owner + 3 bots
	r.CoinBuyIn = 500

	rec := doStartMatch(e, "1", validToken(100))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, 0, starter.called)

	persisted, _ := repo.FindByID(1)
	assert.Equal(t, "completed", persisted.Status, "room closes when no solvent human can host")
}

// Story 9.3 (deferred item 1): when ChargeStakes SUCCEEDS but the session fails
// to start, the charged stakes are refunded (no coins destroyed), the room is
// reverted to waiting (not stranded in playing), error:match_start_failed is
// broadcast, and the owner's request resolves with an error (so the client does
// not auto-navigate into a dead match) — never a success.
func TestStartMatch_RefundOnStartFailure(t *testing.T) {
	starter := &fakeMatchStarter{err: errors.New("session manager unavailable")}
	wallet := &stubWallet{balance: 5000}
	e, repo, broadcaster, _ := setupCoinTestBC(starter, wallet)
	r := seedMixedRoom(t, e, repo, 4, 0)
	r.CoinBuyIn = 500

	rec := doStartMatch(e, "1", validToken(100))
	require.Equal(t, http.StatusInternalServerError, rec.Code, "owner must not get OK on a failed start")

	require.Equal(t, 1, starter.called)
	require.Equal(t, 1, wallet.chargeCalls, "stakes were charged before the start failed")
	require.Equal(t, 1, wallet.settleCalls, "charged stakes must be refunded")
	assert.Equal(t, map[uint]int{100: 500, 200: 500, 300: 500, 400: 500}, wallet.settledCredits)

	persisted, _ := repo.FindByID(1)
	assert.Equal(t, "waiting", persisted.Status, "room reverted to waiting, never stranded in playing")

	require.Len(t, broadcastsOfType(t, broadcaster, "error:match_start_failed"), 1)
	// The success event must NOT have fired.
	assert.Empty(t, broadcastsOfType(t, broadcaster, "system:match_started"))
}

// Story 9.3 AC5: a TOCTOU race — the prefilter passed but the authoritative
// ChargeStakes still reports an insolvent user — ejects that user and aborts;
// the match never starts with an unpaid stake.
func TestStartMatch_ChargeTimeInsolventEjects(t *testing.T) {
	starter := &fakeMatchStarter{}
	// Prefilter sees everyone solvent, but the atomic charge reports 200 insolvent.
	wallet := &stubWallet{balance: 5000, chargeErr: apperr.ErrInsufficientCoins, chargeInsolvent: 200}
	e, repo, broadcaster, _ := setupCoinTestBC(starter, wallet)
	r := seedMixedRoom(t, e, repo, 4, 0)
	r.CoinBuyIn = 500

	rec := doStartMatch(e, "1", validToken(100))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "INSUFFICIENT_COINS", errCodeOf(t, rec))

	assert.Equal(t, 0, starter.called)
	assert.Equal(t, 1, wallet.chargeCalls, "the authoritative charge ran")
	assert.Equal(t, 0, wallet.settleCalls, "ChargeStakes rolls back atomically — nothing to refund")

	persisted, _ := repo.FindByID(1)
	assert.Equal(t, "waiting", persisted.Status)

	ejects := broadcastsOfType(t, broadcaster, "system:insolvent_ejected")
	require.Len(t, ejects, 1)
	assert.Equal(t, []uint{200}, ejects[0].userIDs)
}

// Story 9.3 (review P1, HIGH): a TOCTOU race where the OWNER is the user
// ChargeStakes reports insolvent — with solvent seated heirs present — must
// TRANSFER ownership, not close the room. Regression guard: the charge-time
// eject path must pass the FULL prefilter balances map (not a single-entry map),
// or transferOwnershipOrClose sees every heir as broke and wrongly closes.
func TestStartMatch_ChargeTimeInsolventOwnerTransfersOwnership(t *testing.T) {
	starter := &fakeMatchStarter{}
	// Prefilter sees everyone solvent; the atomic charge reports the OWNER (100)
	// insolvent via the TOCTOU race.
	wallet := &stubWallet{
		balances:        map[uint]int{100: 5000, 200: 5000, 300: 5000, 400: 5000},
		chargeErr:       apperr.ErrInsufficientCoins,
		chargeInsolvent: 100,
	}
	e, repo, _, _ := setupCoinTestBC(starter, wallet)
	r := seedMixedRoom(t, e, repo, 4, 0)
	r.CoinBuyIn = 500

	rec := doStartMatch(e, "1", validToken(100))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, 0, starter.called)
	require.Equal(t, 1, wallet.chargeCalls, "the authoritative charge ran")

	persisted, _ := repo.FindByID(1)
	assert.Equal(t, "waiting", persisted.Status, "viable room must NOT close")
	assert.Equal(t, uint(200), persisted.OwnerID, "ownership transfers to the first solvent seated heir")
}

// Story 9.3 (review P2): when a NON-owner seat is ejected at start, the remaining
// in-room players must receive system:player_left for the freed seat — RoomPage
// ignores system:room_updated for the in-room roster, so without player_left the
// ejected seat stays visibly occupied.
func TestStartMatch_InsolventSeatEjected_NotifiesRemainingPlayers(t *testing.T) {
	starter := &fakeMatchStarter{}
	wallet := &stubWallet{balances: map[uint]int{100: 5000, 200: 0, 300: 5000, 400: 5000}}
	e, repo, broadcaster, _ := setupCoinTestBC(starter, wallet)
	r := seedMixedRoom(t, e, repo, 4, 0)
	r.CoinBuyIn = 500

	rec := doStartMatch(e, "1", validToken(100))
	require.Equal(t, http.StatusConflict, rec.Code)

	lefts := broadcastsOfType(t, broadcaster, "system:player_left")
	require.Len(t, lefts, 1, "remaining players are told the insolvent seat was freed")
	assert.ElementsMatch(t, []uint{100, 300, 400}, lefts[0].userIDs)
	var payload struct {
		RoomID uint `json:"roomId"`
		UserID uint `json:"userId"`
	}
	require.NoError(t, json.Unmarshal(payloadOf(t, lefts[0].msg), &payload))
	assert.Equal(t, uint(200), payload.UserID, "the freed seat is the insolvent player")
}

func TestStartMatch_ZeroBuyInSkipsCharge(t *testing.T) {
	starter := &fakeMatchStarter{}
	wallet := &stubWallet{balance: 5000}
	e, repo := setupCoinTest(starter, wallet)
	seedMixedRoom(t, e, repo, 4, 0) // CoinBuyIn defaults to 0

	rec := doStartMatch(e, "1", validToken(100))
	require.Equal(t, http.StatusOK, rec.Code)

	assert.Equal(t, 0, wallet.chargeCalls, "no charge when the room is free")
	require.Equal(t, 1, starter.called)
	assert.Equal(t, 0, starter.lastCoinBuyIn)
}
