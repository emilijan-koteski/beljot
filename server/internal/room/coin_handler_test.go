package room_test

import (
	"encoding/json"
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

	chargeCalls   int
	chargedIDs    []uint
	chargedAmount int
}

func (s *stubWallet) GetBalance(userID uint) (int, error) { return s.balance, s.balanceErr }

func (s *stubWallet) ChargeStakes(userIDs []uint, amount int) (uint, error) {
	s.chargeCalls++
	s.chargedIDs = append([]uint(nil), userIDs...)
	s.chargedAmount = amount
	return s.chargeInsolvent, s.chargeErr
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

func TestQuickPlay_RoomIsFree(t *testing.T) {
	e, repo := setupCoinTest(&fakeMatchStarter{}, &stubWallet{balance: 5000})
	rec := doQuickPlay(e, validToken(7))
	require.Equal(t, http.StatusOK, rec.Code)

	// Quick-play synthesizes a brand-new room when none is available.
	persisted, _ := repo.FindByID(1)
	require.NotNil(t, persisted)
	assert.True(t, persisted.IsQuickPlay)
	assert.Equal(t, 0, persisted.CoinBuyIn, "quick-play rooms are free in Story 9.2")
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

func TestStartMatch_InsolventRevertsRoomToWaiting(t *testing.T) {
	starter := &fakeMatchStarter{}
	wallet := &stubWallet{balance: 0, chargeErr: apperr.ErrInsufficientCoins, chargeInsolvent: 200}
	e, repo := setupCoinTest(starter, wallet)
	r := seedMixedRoom(t, e, repo, 4, 0)
	r.CoinBuyIn = 500

	rec := doStartMatch(e, "1", validToken(100))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "INSUFFICIENT_COINS", errCodeOf(t, rec))

	assert.Equal(t, 0, starter.called, "the match must NOT start with an unpaid stake")
	persisted, _ := repo.FindByID(1)
	assert.Equal(t, "waiting", persisted.Status, "room is reverted to waiting on insolvency")
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
