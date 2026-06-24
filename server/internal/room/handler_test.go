package room_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/auth"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/room"
)

// --- Mock Repository ---

type mockRoomRepo struct {
	rooms          []*room.Room
	players        []*room.RoomPlayer
	bots           []*room.RoomBot
	nextID         uint
	nextPID        uint
	nextBID        uint
	ownerUsernames map[uint]string
}

func newMockRoomRepo() *mockRoomRepo {
	return &mockRoomRepo{nextID: 1, nextPID: 1, nextBID: 1, ownerUsernames: map[uint]string{}}
}

func (m *mockRoomRepo) Create(r *room.Room) error {
	for _, existing := range m.rooms {
		if existing.Name == r.Name {
			return apperr.ErrRoomNameTaken
		}
	}
	r.ID = m.nextID
	r.CreatedAt = time.Now()
	r.UpdatedAt = time.Now()
	m.nextID++
	m.rooms = append(m.rooms, r)
	return nil
}

func (m *mockRoomRepo) Update(r *room.Room) error {
	for i, existing := range m.rooms {
		if existing.ID == r.ID {
			m.rooms[i] = r
			return nil
		}
	}
	return nil
}

func (m *mockRoomRepo) FindByID(id uint) (*room.Room, error) {
	for _, r := range m.rooms {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, nil
}

func (m *mockRoomRepo) FindByIDForUpdate(id uint) (*room.Room, error) {
	return m.FindByID(id)
}

func (m *mockRoomRepo) FindByCode(code string) (*room.Room, error) {
	for _, r := range m.rooms {
		if r.Code == code {
			return r, nil
		}
	}
	return nil, nil
}

func (m *mockRoomRepo) FindByStatus(status string) ([]room.Room, error) {
	var result []room.Room
	for _, r := range m.rooms {
		if r.Status == status {
			result = append(result, *r)
		}
	}
	if result == nil {
		result = []room.Room{}
	}
	return result, nil
}

func (m *mockRoomRepo) AddPlayer(p *room.RoomPlayer) error {
	for _, existing := range m.players {
		if existing.RoomID == p.RoomID && existing.UserID == p.UserID {
			return apperr.ErrAlreadyInRoom
		}
	}
	p.ID = m.nextPID
	p.CreatedAt = time.Now()
	m.nextPID++
	m.players = append(m.players, p)
	return nil
}

func (m *mockRoomRepo) RemovePlayer(roomID uint, userID uint) error {
	for i, p := range m.players {
		if p.RoomID == roomID && p.UserID == userID {
			m.players = append(m.players[:i], m.players[i+1:]...)
			return nil
		}
	}
	return apperr.ErrNotInRoom
}

func (m *mockRoomRepo) FindPlayersByRoomID(roomID uint) ([]room.RoomPlayer, error) {
	var result []room.RoomPlayer
	for _, p := range m.players {
		if p.RoomID == roomID {
			result = append(result, *p)
		}
	}
	if result == nil {
		result = []room.RoomPlayer{}
	}
	return result, nil
}

func (m *mockRoomRepo) FindPlayerRoom(userID uint) (*room.RoomPlayer, error) {
	for _, p := range m.players {
		for _, r := range m.rooms {
			if r.ID == p.RoomID && r.Status == "waiting" && p.UserID == userID {
				return p, nil
			}
		}
	}
	return nil, nil
}

func (m *mockRoomRepo) IncrementPlayerCount(roomID uint) error {
	for _, r := range m.rooms {
		if r.ID == roomID {
			r.PlayerCount++
			return nil
		}
	}
	return nil
}

func (m *mockRoomRepo) DecrementPlayerCount(roomID uint) error {
	for _, r := range m.rooms {
		if r.ID == roomID && r.PlayerCount > 0 {
			r.PlayerCount--
			return nil
		}
	}
	return nil
}

func (m *mockRoomRepo) UpdatePlayerSeat(roomID uint, userID uint, seat int, team string) error {
	for _, p := range m.players {
		if p.RoomID == roomID && p.UserID == userID {
			p.Seat = &seat
			p.Team = &team
			return nil
		}
	}
	return apperr.ErrNotInRoom
}

func (m *mockRoomRepo) ClearPlayerSeat(roomID uint, userID uint) error {
	for _, p := range m.players {
		if p.RoomID == roomID && p.UserID == userID {
			p.Seat = nil
			p.Team = nil
			return nil
		}
	}
	return apperr.ErrNotInRoom
}

func (m *mockRoomRepo) FindPlayerBySeat(roomID uint, seat int) (*room.RoomPlayer, error) {
	for _, p := range m.players {
		if p.RoomID == roomID && p.Seat != nil && *p.Seat == seat {
			return p, nil
		}
	}
	return nil, nil
}

func (m *mockRoomRepo) FindQuickPlayRoom(buyIn int) (*room.Room, error) {
	return m.FindQuickPlayRoomExcluding(nil, buyIn)
}

func (m *mockRoomRepo) FindQuickPlayRoomExcluding(excluded map[uint]bool, buyIn int) (*room.Room, error) {
	for _, r := range m.rooms {
		if excluded[r.ID] {
			continue
		}
		// Story 9.4: only return a room in the caller's affordability bracket.
		if r.IsQuickPlay && r.Status == "waiting" && r.PlayerCount < 4 && r.CoinBuyIn == buyIn {
			return r, nil
		}
	}
	return nil, nil
}

func (m *mockRoomRepo) FindUserIDsByRoomStatus(status string) ([]uint, error) {
	matchingRooms := make(map[uint]bool)
	for _, r := range m.rooms {
		if r.Status == status {
			matchingRooms[r.ID] = true
		}
	}
	out := make([]uint, 0)
	for _, p := range m.players {
		if matchingRooms[p.RoomID] {
			out = append(out, p.UserID)
		}
	}
	return out, nil
}

func (m *mockRoomRepo) UpdateStatus(roomID uint, status string) error {
	for _, r := range m.rooms {
		if r.ID == roomID {
			r.Status = status
			return nil
		}
	}
	return nil
}

func (m *mockRoomRepo) LoadOwnerUsernames(rooms []*room.Room) error {
	for _, rm := range rooms {
		if rm == nil || rm.OwnerUsername != "" {
			continue
		}
		if name, ok := m.ownerUsernames[rm.OwnerID]; ok {
			rm.OwnerUsername = name
		}
	}
	return nil
}

func (m *mockRoomRepo) FindPlayersByRoomIDs(roomIDs []uint) (map[uint][]room.RoomPlayer, error) {
	out := make(map[uint][]room.RoomPlayer)
	for _, id := range roomIDs {
		for _, p := range m.players {
			if p.RoomID == id {
				out[id] = append(out[id], *p)
			}
		}
	}
	return out, nil
}

func (m *mockRoomRepo) AddBot(roomID uint, seat int) error {
	for _, b := range m.bots {
		if b.RoomID == roomID && b.Seat == seat {
			return apperr.ErrSeatTaken
		}
	}
	m.bots = append(m.bots, &room.RoomBot{ID: m.nextBID, RoomID: roomID, Seat: seat, CreatedAt: time.Now()})
	m.nextBID++
	return nil
}

func (m *mockRoomRepo) RemoveBot(roomID uint, seat int) error {
	for i, b := range m.bots {
		if b.RoomID == roomID && b.Seat == seat {
			m.bots = append(m.bots[:i], m.bots[i+1:]...)
			return nil
		}
	}
	return apperr.ErrNoBotOnSeat
}

func (m *mockRoomRepo) UpdateBotSeat(roomID uint, fromSeat, toSeat int) error {
	for _, b := range m.bots {
		if b.RoomID == roomID && b.Seat == toSeat {
			return apperr.ErrSeatTaken
		}
	}
	for _, b := range m.bots {
		if b.RoomID == roomID && b.Seat == fromSeat {
			b.Seat = toSeat
			return nil
		}
	}
	return apperr.ErrNoBotOnSeat
}

func (m *mockRoomRepo) FindBotsByRoomID(roomID uint) ([]room.RoomBot, error) {
	var result []room.RoomBot
	for _, b := range m.bots {
		if b.RoomID == roomID {
			result = append(result, *b)
		}
	}
	return result, nil
}

func (m *mockRoomRepo) FindBotsByRoomIDs(roomIDs []uint) (map[uint][]room.RoomBot, error) {
	out := make(map[uint][]room.RoomBot)
	for _, id := range roomIDs {
		for _, b := range m.bots {
			if b.RoomID == id {
				out[id] = append(out[id], *b)
			}
		}
	}
	return out, nil
}

func (m *mockRoomRepo) RunInTransaction(fn func(room.RoomRepository) error) error {
	return fn(m)
}

// --- Mock Broadcaster ---

type broadcastCall struct {
	userIDs []uint
	msg     []byte
}

type allBroadcastCall struct {
	msg []byte
}

type mockBroadcaster struct {
	calls    []broadcastCall
	allCalls []allBroadcastCall
}

func (m *mockBroadcaster) BroadcastToUsers(userIDs []uint, msg []byte) {
	m.calls = append(m.calls, broadcastCall{userIDs: userIDs, msg: msg})
}

func (m *mockBroadcaster) BroadcastAll(msg []byte) {
	m.allCalls = append(m.allCalls, allBroadcastCall{msg: msg})
}

// --- Test Infrastructure ---

func testErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	var appErr *apperr.AppError
	if errors.As(err, &appErr) {
		_ = c.JSON(appErr.Status, map[string]interface{}{
			"error": map[string]string{
				"code":    appErr.Code,
				"message": appErr.Message,
			},
		})
		return
	}

	_ = c.JSON(http.StatusInternalServerError, map[string]interface{}{
		"error": map[string]string{
			"code":    "INTERNAL_ERROR",
			"message": "An internal error occurred",
		},
	})
}

func setupTest() (*echo.Echo, *mockRoomRepo) {
	repo := newMockRoomRepo()
	handler := room.NewRoomHandler(repo, nil, nil, nil, nil)

	e := echo.New()
	e.HTTPErrorHandler = testErrorHandler
	api := e.Group("/api/v1", auth.AuthMiddleware("test-jwt-secret"))
	api.POST("/rooms", handler.CreateRoom)
	api.GET("/rooms", handler.ListRooms)
	api.POST("/rooms/quick-play", handler.QuickPlay)
	api.GET("/rooms/code/:code", handler.GetRoomByCode)
	api.GET("/rooms/:id", handler.GetRoom)
	api.POST("/rooms/:id/join", handler.JoinRoom)
	api.POST("/rooms/:id/quick-join", handler.QuickJoin)
	api.POST("/rooms/:id/leave", handler.LeaveRoom)
	api.POST("/rooms/:id/return", handler.ReturnToRoom)
	api.POST("/rooms/:id/seat", handler.SelectSeat)
	api.POST("/rooms/:id/leave-seat", handler.LeaveSeat)
	api.POST("/rooms/:id/start", handler.StartMatch)
	api.POST("/rooms/:id/kick", handler.KickPlayer)
	api.POST("/rooms/:id/swap-seats", handler.SwapSeats)
	api.POST("/rooms/:id/transfer-ownership", handler.TransferOwnership)
	api.POST("/rooms/:id/privacy", handler.UpdateRoomPrivacy)
	api.POST("/rooms/:id/bots", handler.AddBot)
	api.DELETE("/rooms/:id/bots/:seat", handler.RemoveBot)

	return e, repo
}

func setupTestWithBroadcast() (*echo.Echo, *mockRoomRepo, *mockBroadcaster) {
	repo := newMockRoomRepo()
	broadcaster := &mockBroadcaster{}
	handler := room.NewRoomHandler(repo, nil, broadcaster, nil, nil)

	e := echo.New()
	e.HTTPErrorHandler = testErrorHandler
	api := e.Group("/api/v1", auth.AuthMiddleware("test-jwt-secret"))
	api.POST("/rooms", handler.CreateRoom)
	api.GET("/rooms", handler.ListRooms)
	api.POST("/rooms/quick-play", handler.QuickPlay)
	api.GET("/rooms/code/:code", handler.GetRoomByCode)
	api.GET("/rooms/:id", handler.GetRoom)
	api.POST("/rooms/:id/join", handler.JoinRoom)
	api.POST("/rooms/:id/quick-join", handler.QuickJoin)
	api.POST("/rooms/:id/leave", handler.LeaveRoom)
	api.POST("/rooms/:id/return", handler.ReturnToRoom)
	api.POST("/rooms/:id/seat", handler.SelectSeat)
	api.POST("/rooms/:id/leave-seat", handler.LeaveSeat)
	api.POST("/rooms/:id/start", handler.StartMatch)
	api.POST("/rooms/:id/kick", handler.KickPlayer)
	api.POST("/rooms/:id/swap-seats", handler.SwapSeats)
	api.POST("/rooms/:id/transfer-ownership", handler.TransferOwnership)
	api.POST("/rooms/:id/privacy", handler.UpdateRoomPrivacy)
	api.POST("/rooms/:id/bots", handler.AddBot)
	api.DELETE("/rooms/:id/bots/:seat", handler.RemoveBot)

	return e, repo, broadcaster
}

// registerRoomRoutes wires the full room route set onto api. Shared by the
// presence-aware setup helpers (Story 9.3) so they don't re-list every route.
func registerRoomRoutes(api *echo.Group, handler *room.RoomHandler) {
	api.POST("/rooms", handler.CreateRoom)
	api.GET("/rooms", handler.ListRooms)
	api.POST("/rooms/quick-play", handler.QuickPlay)
	api.GET("/rooms/code/:code", handler.GetRoomByCode)
	api.GET("/rooms/:id", handler.GetRoom)
	api.POST("/rooms/:id/join", handler.JoinRoom)
	api.POST("/rooms/:id/quick-join", handler.QuickJoin)
	api.POST("/rooms/:id/leave", handler.LeaveRoom)
	api.POST("/rooms/:id/return", handler.ReturnToRoom)
	api.POST("/rooms/:id/seat", handler.SelectSeat)
	api.POST("/rooms/:id/leave-seat", handler.LeaveSeat)
	api.POST("/rooms/:id/start", handler.StartMatch)
	api.POST("/rooms/:id/kick", handler.KickPlayer)
	api.POST("/rooms/:id/swap-seats", handler.SwapSeats)
	api.POST("/rooms/:id/transfer-ownership", handler.TransferOwnership)
	api.POST("/rooms/:id/privacy", handler.UpdateRoomPrivacy)
	api.POST("/rooms/:id/bots", handler.AddBot)
	api.DELETE("/rooms/:id/bots/:seat", handler.RemoveBot)
}

// setupTestWithPresence mirrors setupTest but injects a caller-controlled
// presence registry so tests can mark seated members "present". Required since
// Story 9.3 gates owner-transfer on present-AND-solvent and direct repo seeding
// bypasses the join/return paths that populate presence in production.
func setupTestWithPresence() (*echo.Echo, *mockRoomRepo, *room.PresenceRegistry) {
	repo := newMockRoomRepo()
	reg := room.NewPresenceRegistry()
	handler := room.NewRoomHandler(repo, nil, &mockBroadcaster{}, reg, nil)

	e := echo.New()
	e.HTTPErrorHandler = testErrorHandler
	api := e.Group("/api/v1", auth.AuthMiddleware("test-jwt-secret"))
	registerRoomRoutes(api, handler)

	return e, repo, reg
}

// setupTestWithBroadcastAndPresence is setupTestWithBroadcast plus a caller-
// controlled presence registry (Story 9.3).
func setupTestWithBroadcastAndPresence() (*echo.Echo, *mockRoomRepo, *mockBroadcaster, *room.PresenceRegistry) {
	repo := newMockRoomRepo()
	broadcaster := &mockBroadcaster{}
	reg := room.NewPresenceRegistry()
	handler := room.NewRoomHandler(repo, nil, broadcaster, reg, nil)

	e := echo.New()
	e.HTTPErrorHandler = testErrorHandler
	api := e.Group("/api/v1", auth.AuthMiddleware("test-jwt-secret"))
	registerRoomRoutes(api, handler)

	return e, repo, broadcaster, reg
}

func doCreateRoom(e *echo.Echo, body string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rooms", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func validToken(userID uint) string {
	token, _ := auth.GenerateAccessToken(userID, "test-jwt-secret")
	return token
}

// --- Tests ---

func TestCreateRoom_Success(t *testing.T) {
	e, _ := setupTest()
	token := validToken(5)

	body := `{"name":"Zagreb Ekipa","variant":"bitola","matchMode":"1001","timerStyle":"relaxed"}`
	rec := doCreateRoom(e, body, token)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var data room.Room
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	assert.Equal(t, uint(1), data.ID)
	assert.Equal(t, "Zagreb Ekipa", data.Name)
	assert.Equal(t, uint(5), data.OwnerID)
	assert.Equal(t, "bitola", data.Variant)
	assert.Equal(t, "1001", data.MatchMode)
	assert.Equal(t, "relaxed", data.TimerStyle)
	assert.Nil(t, data.TimerDurationSeconds)
	assert.Equal(t, "waiting", data.Status)
	assert.Equal(t, 1, data.PlayerCount)
	assert.Len(t, data.Code, 6)
}

func TestCreateRoom_501MatchMode(t *testing.T) {
	e, repo := setupTest()
	token := validToken(5)

	body := `{"name":"Quick 501","variant":"bitola","matchMode":"501","timerStyle":"relaxed"}`
	rec := doCreateRoom(e, body, token)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var data room.Room
	require.NoError(t, json.Unmarshal(resp["data"], &data))
	assert.Equal(t, "501", data.MatchMode)

	persisted, err := repo.FindByID(data.ID)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	assert.Equal(t, "501", persisted.MatchMode)
}

func TestCreateRoom_AutoSeatsCreator(t *testing.T) {
	// The creator is auto-seated at seat 0 / teamA so the room is startable
	// from creation: with the 4-player cap an unseated owner could otherwise
	// get locked out by 3 invitees taking the open seats. Mirrors the QuickPlay
	// auto-seat path.
	e, repo := setupTest()
	token := validToken(7)

	body := `{"name":"Auto Seated","variant":"bitola","matchMode":"1001","timerStyle":"relaxed"}`
	rec := doCreateRoom(e, body, token)
	require.Equal(t, http.StatusCreated, rec.Code)

	players, err := repo.FindPlayersByRoomID(1)
	require.NoError(t, err)
	require.Len(t, players, 1)
	require.NotNil(t, players[0].Seat)
	assert.Equal(t, 0, *players[0].Seat)
	require.NotNil(t, players[0].Team)
	assert.Equal(t, "teamA", *players[0].Team)
	assert.Equal(t, uint(7), players[0].UserID)
}

func TestCreateRoom_WithDefaults(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	body := `{"name":"My Room"}`
	rec := doCreateRoom(e, body, token)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var data room.Room
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	assert.Equal(t, "bitola", data.Variant)
	assert.Equal(t, "1001", data.MatchMode)
	assert.Equal(t, "relaxed", data.TimerStyle)
	assert.Nil(t, data.TimerDurationSeconds)
}

func TestCreateRoom_EmptyName(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	body := `{"name":""}`
	rec := doCreateRoom(e, body, token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "ROOM_NAME_REQUIRED", errResp["error"]["code"])
}

func TestCreateRoom_WhitespaceOnlyName(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	body := `{"name":"   "}`
	rec := doCreateRoom(e, body, token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "ROOM_NAME_REQUIRED", errResp["error"]["code"])
}

func TestCreateRoom_DuplicateName(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	body := `{"name":"Duplicate Room"}`
	rec := doCreateRoom(e, body, token)
	assert.Equal(t, http.StatusCreated, rec.Code)

	// A DIFFERENT user attempts the same name — user 1 is now in their room and
	// would (correctly) hit the already-in-room guard first.
	rec2 := doCreateRoom(e, body, validToken(2))
	assert.Equal(t, http.StatusConflict, rec2.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &errResp))
	assert.Equal(t, "ROOM_NAME_TAKEN", errResp["error"]["code"])
}

func TestCreateRoom_OwnerIDFromAuth(t *testing.T) {
	e, repo := setupTest()
	token := validToken(42)

	body := `{"name":"Owner Test Room"}`
	rec := doCreateRoom(e, body, token)

	assert.Equal(t, http.StatusCreated, rec.Code)
	require.Len(t, repo.rooms, 1)
	assert.Equal(t, uint(42), repo.rooms[0].OwnerID)
}

func TestCreateRoom_CodeFormat(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	body := `{"name":"Code Test Room"}`
	rec := doCreateRoom(e, body, token)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var data room.Room
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	assert.Len(t, data.Code, 6)
	for _, ch := range data.Code {
		assert.Contains(t, "ABCDEFGHJKLMNPQRSTUVWXYZ23456789", string(ch),
			"room code should only contain allowed characters")
	}
}

func TestCreateRoom_Unauthorized(t *testing.T) {
	e, _ := setupTest()

	body := `{"name":"No Auth Room"}`
	rec := doCreateRoom(e, body, "")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCreateRoom_PerMoveTimerWithDuration(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	body := `{"name":"Timer Room","timerStyle":"per-move","timerDurationSeconds":45}`
	rec := doCreateRoom(e, body, token)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var data room.Room
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	assert.Equal(t, "per-move", data.TimerStyle)
	require.NotNil(t, data.TimerDurationSeconds)
	assert.Equal(t, 45, *data.TimerDurationSeconds)
}

func TestCreateRoom_RelaxedTimerNullsDuration(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	body := `{"name":"Relaxed Room","timerStyle":"relaxed","timerDurationSeconds":30}`
	rec := doCreateRoom(e, body, token)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var data room.Room
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	assert.Equal(t, "relaxed", data.TimerStyle)
	assert.Nil(t, data.TimerDurationSeconds)
}

func TestCreateRoom_NameTooLong(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	longName := strings.Repeat("a", 101)
	body := `{"name":"` + longName + `"}`
	rec := doCreateRoom(e, body, token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "ROOM_NAME_TOO_LONG", errResp["error"]["code"])
}

func TestCreateRoom_InvalidVariant(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	body := `{"name":"Bad Variant","variant":"unknown"}`
	rec := doCreateRoom(e, body, token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "INVALID_VARIANT", errResp["error"]["code"])
}

func TestCreateRoom_InvalidMatchMode(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	body := `{"name":"Bad Mode","matchMode":"999"}`
	rec := doCreateRoom(e, body, token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "INVALID_MATCH_MODE", errResp["error"]["code"])
}

func TestCreateRoom_InvalidTimerStyle(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	body := `{"name":"Bad Timer","timerStyle":"turbo"}`
	rec := doCreateRoom(e, body, token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "INVALID_TIMER_STYLE", errResp["error"]["code"])
}

func TestCreateRoom_PerMoveTimerWithoutDuration(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	body := `{"name":"No Duration","timerStyle":"per-move"}`
	rec := doCreateRoom(e, body, token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "TIMER_DURATION_REQUIRED", errResp["error"]["code"])
}

func TestCreateRoom_TimerDurationTooLow(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	body := `{"name":"Low Timer","timerStyle":"per-move","timerDurationSeconds":5}`
	rec := doCreateRoom(e, body, token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "TIMER_DURATION_OUT_OF_RANGE", errResp["error"]["code"])
}

func TestCreateRoom_TimerDurationTooHigh(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	body := `{"name":"High Timer","timerStyle":"per-move","timerDurationSeconds":999}`
	rec := doCreateRoom(e, body, token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "TIMER_DURATION_OUT_OF_RANGE", errResp["error"]["code"])
}

// --- ListRooms Tests ---

func doListRooms(e *echo.Echo, query string, token string) *httptest.ResponseRecorder {
	url := "/api/v1/rooms"
	if query != "" {
		url += "?" + query
	}
	req := httptest.NewRequest(http.MethodGet, url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func seedRoom(repo *mockRoomRepo, name string, status string) {
	r := &room.Room{
		Name:        name,
		Code:        "ABC123",
		OwnerID:     1,
		Variant:     "bitola",
		MatchMode:   "1001",
		TimerStyle:  "relaxed",
		Status:      status,
		PlayerCount: 1,
	}
	r.ID = repo.nextID
	r.CreatedAt = time.Now()
	r.UpdatedAt = time.Now()
	repo.nextID++
	repo.rooms = append(repo.rooms, r)
}

func TestListRooms_DefaultsToWaiting(t *testing.T) {
	e, repo := setupTest()
	token := validToken(1)

	seedRoom(repo, "Open Room", "waiting")
	seedRoom(repo, "Full Room", "playing")

	rec := doListRooms(e, "", token)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string][]room.Room
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	require.Len(t, resp["data"], 1)
	assert.Equal(t, "Open Room", resp["data"][0].Name)
}

func TestListRooms_RespectsStatusParam(t *testing.T) {
	e, repo := setupTest()
	token := validToken(1)

	seedRoom(repo, "Waiting Room", "waiting")
	seedRoom(repo, "Playing Room", "playing")

	rec := doListRooms(e, "status=playing", token)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string][]room.Room
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	require.Len(t, resp["data"], 1)
	assert.Equal(t, "Playing Room", resp["data"][0].Name)
}

func TestListRooms_EmptyArrayNotNull(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	rec := doListRooms(e, "", token)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Verify the raw JSON contains [] not null
	body := rec.Body.String()
	assert.Contains(t, body, `"data":[]`)
}

func TestListRooms_Unauthorized(t *testing.T) {
	e, _ := setupTest()

	rec := doListRooms(e, "", "")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestListRooms_InvalidStatus(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	rec := doListRooms(e, "status=invalid", token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "INVALID_ROOM_STATUS", errResp["error"]["code"])
}

// --- GetRoom Tests ---

func doGetRoom(e *echo.Echo, id string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rooms/"+id, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestGetRoom_Success(t *testing.T) {
	e, repo := setupTest()
	token := validToken(1)

	seedRoom(repo, "Test Room", "waiting")
	repo.players = append(repo.players, &room.RoomPlayer{
		ID: 1, RoomID: 1, UserID: 1, Username: "player1", CreatedAt: time.Now(),
	})

	rec := doGetRoom(e, "1", token)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]room.RoomDetailResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.Equal(t, "Test Room", resp["data"].Room.Name)
	require.Len(t, resp["data"].Players, 1)
	assert.Equal(t, uint(1), resp["data"].Players[0].UserID)
}

func TestGetRoom_NotFound(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	rec := doGetRoom(e, "999", token)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "ROOM_NOT_FOUND", errResp["error"]["code"])
}

func TestGetRoom_EmptyPlayersArray(t *testing.T) {
	e, repo := setupTest()
	token := validToken(1)

	seedRoom(repo, "Empty Room", "waiting")

	rec := doGetRoom(e, "1", token)

	assert.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.Contains(t, body, `"players":[]`)
}

func TestGetRoom_Unauthorized(t *testing.T) {
	e, _ := setupTest()

	rec := doGetRoom(e, "1", "")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- GetRoomByCode Tests ---

func doGetRoomByCode(e *echo.Echo, code string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rooms/code/"+code, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func seedRoomWithCode(repo *mockRoomRepo, name string, code string, status string) {
	r := &room.Room{
		Name:        name,
		Code:        code,
		OwnerID:     1,
		Variant:     "bitola",
		MatchMode:   "1001",
		TimerStyle:  "relaxed",
		Status:      status,
		PlayerCount: 1,
	}
	r.ID = repo.nextID
	r.CreatedAt = time.Now()
	r.UpdatedAt = time.Now()
	repo.nextID++
	repo.rooms = append(repo.rooms, r)
}

func TestGetRoomByCode_Success(t *testing.T) {
	e, repo := setupTest()
	token := validToken(1)

	seedRoomWithCode(repo, "Code Room", "XYZ789", "waiting")
	repo.players = append(repo.players, &room.RoomPlayer{
		ID: 1, RoomID: 1, UserID: 1, Username: "player1", CreatedAt: time.Now(),
	})

	rec := doGetRoomByCode(e, "XYZ789", token)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]room.RoomDetailResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.Equal(t, "Code Room", resp["data"].Room.Name)
	assert.Equal(t, "XYZ789", resp["data"].Room.Code)
	require.Len(t, resp["data"].Players, 1)
}

func TestGetRoomByCode_NotFound(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	rec := doGetRoomByCode(e, "NOROOM", token)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "ROOM_NOT_FOUND", errResp["error"]["code"])
}

func TestGetRoomByCode_RejectsNonWaitingRoom(t *testing.T) {
	e, repo := setupTest()
	token := validToken(1)

	seedRoomWithCode(repo, "Playing Room", "PLAY99", "playing")

	rec := doGetRoomByCode(e, "PLAY99", token)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetRoomByCode_RejectsInvalidLength(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	rec := doGetRoomByCode(e, "AB", token)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = doGetRoomByCode(e, "TOOLONGCODE", token)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetRoomByCode_LowercaseNormalized(t *testing.T) {
	e, repo := setupTest()
	token := validToken(1)

	seedRoomWithCode(repo, "Lower Room", "ABC456", "waiting")

	rec := doGetRoomByCode(e, "abc456", token)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestGetRoomByCode_Unauthorized(t *testing.T) {
	e, _ := setupTest()

	rec := doGetRoomByCode(e, "ABC123", "")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- JoinRoom Tests ---

func doJoinRoom(e *echo.Echo, id string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/"+id+"/join", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestJoinRoom_Success(t *testing.T) {
	e, repo := setupTest()
	token := validToken(10)

	seedRoom(repo, "Join Test", "waiting")

	rec := doJoinRoom(e, "1", token)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]room.Room
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp["data"].PlayerCount)

	// Verify player was added
	require.Len(t, repo.players, 1)
	assert.Equal(t, uint(10), repo.players[0].UserID)
}

func TestJoinRoom_NotFound(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	rec := doJoinRoom(e, "999", token)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "ROOM_NOT_FOUND", errResp["error"]["code"])
}

func TestJoinRoom_Full(t *testing.T) {
	e, repo := setupTest()
	token := validToken(10)

	seedRoom(repo, "Full Room", "waiting")
	repo.rooms[0].PlayerCount = 4

	rec := doJoinRoom(e, "1", token)

	assert.Equal(t, http.StatusConflict, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "ROOM_FULL", errResp["error"]["code"])
}

func TestJoinRoom_AlreadyInRoom(t *testing.T) {
	e, repo := setupTest()
	token := validToken(10)

	seedRoom(repo, "Room A", "waiting")
	repo.players = append(repo.players, &room.RoomPlayer{
		ID: 1, RoomID: 1, UserID: 10, CreatedAt: time.Now(),
	})

	seedRoom(repo, "Room B", "waiting")

	rec := doJoinRoom(e, "2", token)

	assert.Equal(t, http.StatusConflict, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "ALREADY_IN_ROOM", errResp["error"]["code"])
}

func TestJoinRoom_NotWaitingStatus(t *testing.T) {
	e, repo := setupTest()
	token := validToken(10)

	seedRoom(repo, "Playing Room", "playing")

	rec := doJoinRoom(e, "1", token)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestJoinRoom_Unauthorized(t *testing.T) {
	e, _ := setupTest()

	rec := doJoinRoom(e, "1", "")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- LeaveRoom Tests ---

func doLeaveRoom(e *echo.Echo, id string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/"+id+"/leave", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestLeaveRoom_Success(t *testing.T) {
	e, repo := setupTest()
	token := validToken(10)

	seedRoom(repo, "Leave Test", "waiting")
	repo.rooms[0].PlayerCount = 2
	repo.players = append(repo.players,
		&room.RoomPlayer{ID: 1, RoomID: 1, UserID: 1, CreatedAt: time.Now()},
		&room.RoomPlayer{ID: 2, RoomID: 1, UserID: 10, CreatedAt: time.Now()},
	)

	rec := doLeaveRoom(e, "1", token)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, repo.rooms[0].PlayerCount)
	require.Len(t, repo.players, 1)
}

func TestLeaveRoom_NotInRoom(t *testing.T) {
	e, repo := setupTest()
	token := validToken(10)

	seedRoom(repo, "Not In Room", "waiting")

	rec := doLeaveRoom(e, "1", token)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "NOT_IN_ROOM", errResp["error"]["code"])
}

func TestLeaveRoom_OwnerTransfersOwnership(t *testing.T) {
	e, repo, reg := setupTestWithPresence()
	token := validToken(1) // user 1 is the owner (seedRoom sets OwnerID=1)

	seedRoom(repo, "Owner Leave", "waiting")
	repo.rooms[0].PlayerCount = 2
	// The inheritor must be SEATED — an unseated member never inherits ownership
	// (see TestLeaveRoom_Owner_NoSeatedHuman_ClosesRoom).
	repo.players = append(repo.players,
		&room.RoomPlayer{ID: 1, RoomID: 1, UserID: 1, Seat: intPtr(0), CreatedAt: time.Now()},
		&room.RoomPlayer{ID: 2, RoomID: 1, UserID: 20, Seat: intPtr(1), CreatedAt: time.Now().Add(time.Second)},
	)
	// Story 9.3: ownership transfers only to a PRESENT seated human. In
	// production join/return populate presence; here the direct seeding does not,
	// so mark the inheritor present explicitly. The room is free (CoinBuyIn 0) so
	// solvency is trivially satisfied.
	reg.Add(1, 20)

	rec := doLeaveRoom(e, "1", token)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, uint(20), repo.rooms[0].OwnerID)
}

func TestLeaveRoom_OwnerAloneClosesRoom(t *testing.T) {
	e, repo := setupTest()
	token := validToken(1)

	seedRoom(repo, "Solo Owner", "waiting")
	repo.players = append(repo.players,
		&room.RoomPlayer{ID: 1, RoomID: 1, UserID: 1, CreatedAt: time.Now()},
	)

	rec := doLeaveRoom(e, "1", token)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "completed", repo.rooms[0].Status)
}

// When the owner leaves and the only remaining member is UNSEATED (with bots
// filling seats), the room must CLOSE for everyone — never hand ownership to an
// unseated player. Reproduces the production "unseated owner + 2 bots" zombie.
func TestLeaveRoom_Owner_NoSeatedHuman_ClosesRoom(t *testing.T) {
	e, repo := setupTest()

	seedRoom(repo, "Zombie Guard", "waiting")
	repo.rooms[0].PlayerCount = 2
	repo.players = append(repo.players,
		&room.RoomPlayer{ID: 1, RoomID: 1, UserID: 1, Seat: intPtr(0), CreatedAt: time.Now()},
		&room.RoomPlayer{ID: 2, RoomID: 1, UserID: 20, CreatedAt: time.Now().Add(time.Second)}, // unseated
	)
	repo.bots = append(repo.bots, &room.RoomBot{ID: 1, RoomID: 1, Seat: 2, CreatedAt: time.Now()})

	rec := doLeaveRoom(e, "1", validToken(1))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "completed", repo.rooms[0].Status, "room must close when no seated human remains")
	assert.NotEqual(t, uint(20), repo.rooms[0].OwnerID, "unseated member must not inherit ownership")
	for _, p := range repo.players {
		assert.NotEqual(t, uint(20), p.UserID, "unseated straggler must be evicted on close")
	}
	assert.Empty(t, repo.bots, "bots must be cleared on close")
}

// Ownership must skip an unseated member and go to a seated one, even when the
// unseated member is listed first.
func TestLeaveRoom_OwnerTransfersToSeated_NotUnseated(t *testing.T) {
	e, repo, reg := setupTestWithPresence()

	seedRoom(repo, "Prefer Seated", "waiting")
	repo.rooms[0].PlayerCount = 3
	repo.players = append(repo.players,
		&room.RoomPlayer{ID: 1, RoomID: 1, UserID: 1, Seat: intPtr(0), CreatedAt: time.Now()},
		&room.RoomPlayer{ID: 2, RoomID: 1, UserID: 20, CreatedAt: time.Now().Add(time.Second)},                      // unseated, first
		&room.RoomPlayer{ID: 3, RoomID: 1, UserID: 30, Seat: intPtr(2), CreatedAt: time.Now().Add(2 * time.Second)}, // seated
	)
	// Both candidates present (Story 9.3); only the seated one (30) is eligible.
	reg.Add(1, 20)
	reg.Add(1, 30)

	rec := doLeaveRoom(e, "1", validToken(1))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "waiting", repo.rooms[0].Status)
	assert.Equal(t, uint(30), repo.rooms[0].OwnerID, "ownership must skip the unseated member for the seated one")
}

func TestLeaveRoom_Unauthorized(t *testing.T) {
	e, _ := setupTest()

	rec := doLeaveRoom(e, "1", "")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// A user already in an active room cannot CREATE another (orphan-room guard,
// mirrors JoinRoom / QuickPlay / QuickJoin).
func TestCreateRoom_AlreadyInRoom_Rejected(t *testing.T) {
	e, repo := setupTest()

	seedRoom(repo, "Existing", "waiting")
	repo.players = append(repo.players,
		&room.RoomPlayer{ID: 1, RoomID: 1, UserID: 1, Seat: intPtr(0), CreatedAt: time.Now()},
	)

	rec := doCreateRoom(e, `{"name":"Second Room","variant":"bitola","matchMode":"501","timerStyle":"relaxed"}`, validToken(1))

	assert.Equal(t, http.StatusConflict, rec.Code)
	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "ALREADY_IN_ROOM", errResp["error"]["code"])
}

// --- SelectSeat Tests ---

func doSelectSeat(e *echo.Echo, id string, body string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/"+id+"/seat", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func intPtr(v int) *int { return &v }

func strPtr(v string) *string { return &v }

func seedRoomWithPlayers(repo *mockRoomRepo, name string, ownerID uint, playerIDs ...uint) *room.Room {
	r := &room.Room{
		Name:        name,
		Code:        "TEST01",
		OwnerID:     ownerID,
		Variant:     "bitola",
		MatchMode:   "1001",
		TimerStyle:  "relaxed",
		Status:      "waiting",
		PlayerCount: len(playerIDs),
	}
	r.ID = repo.nextID
	r.CreatedAt = time.Now()
	r.UpdatedAt = time.Now()
	repo.nextID++
	repo.rooms = append(repo.rooms, r)

	for _, uid := range playerIDs {
		p := &room.RoomPlayer{
			ID:        repo.nextPID,
			RoomID:    r.ID,
			UserID:    uid,
			Username:  "user" + string(rune('0'+uid%10)),
			CreatedAt: time.Now(),
		}
		repo.nextPID++
		repo.players = append(repo.players, p)
	}

	return r
}

func TestSelectSeat_Success(t *testing.T) {
	e, repo := setupTest()
	token := validToken(10)

	seedRoomWithPlayers(repo, "Seat Test", 1, 1, 10)

	rec := doSelectSeat(e, "1", `{"seat":0}`, token)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Find the player and verify seat assignment
	for _, p := range repo.players {
		if p.UserID == 10 {
			require.NotNil(t, p.Seat)
			assert.Equal(t, 0, *p.Seat)
			require.NotNil(t, p.Team)
			assert.Equal(t, "teamA", *p.Team)
		}
	}
}

func TestSelectSeat_TeamDerivation(t *testing.T) {
	tests := []struct {
		seat int
		team string
	}{
		{0, "teamA"},
		{1, "teamB"},
		{2, "teamA"},
		{3, "teamB"},
	}

	for _, tc := range tests {
		t.Run("seat_"+string(rune('0'+tc.seat)), func(t *testing.T) {
			e, repo := setupTest()
			token := validToken(10)

			seedRoomWithPlayers(repo, "Team Test", 1, 1, 10)

			body := `{"seat":` + string(rune('0'+tc.seat)) + `}`
			rec := doSelectSeat(e, "1", body, token)

			assert.Equal(t, http.StatusOK, rec.Code)

			for _, p := range repo.players {
				if p.UserID == 10 && p.Team != nil {
					assert.Equal(t, tc.team, *p.Team)
				}
			}
		})
	}
}

func TestSelectSeat_Switching(t *testing.T) {
	e, repo := setupTest()
	token := validToken(10)

	seedRoomWithPlayers(repo, "Switch Test", 1, 1, 10)
	// Pre-assign player 10 to seat 0
	for _, p := range repo.players {
		if p.UserID == 10 {
			p.Seat = intPtr(0)
			p.Team = strPtr("teamA")
		}
	}

	rec := doSelectSeat(e, "1", `{"seat":3}`, token)

	assert.Equal(t, http.StatusOK, rec.Code)

	for _, p := range repo.players {
		if p.UserID == 10 {
			require.NotNil(t, p.Seat)
			assert.Equal(t, 3, *p.Seat)
			require.NotNil(t, p.Team)
			assert.Equal(t, "teamB", *p.Team)
		}
	}
}

func TestSelectSeat_Taken(t *testing.T) {
	e, repo := setupTest()
	token := validToken(10)

	seedRoomWithPlayers(repo, "Taken Test", 1, 1, 10)
	// Player 1 already in seat 0
	for _, p := range repo.players {
		if p.UserID == 1 {
			p.Seat = intPtr(0)
			p.Team = strPtr("teamA")
		}
	}

	rec := doSelectSeat(e, "1", `{"seat":0}`, token)

	assert.Equal(t, http.StatusConflict, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "SEAT_TAKEN", errResp["error"]["code"])
}

func TestSelectSeat_InvalidSeatNumber(t *testing.T) {
	e, repo := setupTest()
	token := validToken(10)

	seedRoomWithPlayers(repo, "Invalid Seat", 1, 1, 10)

	rec := doSelectSeat(e, "1", `{"seat":5}`, token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "INVALID_SEAT", errResp["error"]["code"])
}

func TestSelectSeat_NegativeSeat(t *testing.T) {
	e, repo := setupTest()
	token := validToken(10)

	seedRoomWithPlayers(repo, "Neg Seat", 1, 1, 10)

	rec := doSelectSeat(e, "1", `{"seat":-1}`, token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "INVALID_SEAT", errResp["error"]["code"])
}

func TestSelectSeat_MissingSeat(t *testing.T) {
	e, repo := setupTest()
	token := validToken(10)

	seedRoomWithPlayers(repo, "Missing Seat", 1, 1, 10)

	rec := doSelectSeat(e, "1", `{}`, token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "INVALID_SEAT", errResp["error"]["code"])
}

func TestSelectSeat_NotInRoom(t *testing.T) {
	e, repo := setupTest()
	token := validToken(99) // user 99 not in room

	seedRoomWithPlayers(repo, "Not In Room", 1, 1, 10)

	rec := doSelectSeat(e, "1", `{"seat":0}`, token)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "NOT_IN_ROOM", errResp["error"]["code"])
}

func TestSelectSeat_RoomNotWaiting(t *testing.T) {
	e, repo := setupTest()
	token := validToken(10)

	seedRoomWithPlayers(repo, "In Progress", 1, 1, 10)
	repo.rooms[0].Status = "in_progress"

	rec := doSelectSeat(e, "1", `{"seat":0}`, token)

	assert.Equal(t, http.StatusConflict, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "MATCH_NOT_STARTABLE", errResp["error"]["code"])
}

func TestSelectSeat_OwnCurrentSeat(t *testing.T) {
	e, repo := setupTest()
	token := validToken(10)

	seedRoomWithPlayers(repo, "No-op Test", 1, 1, 10)
	// Player 10 already in seat 2
	for _, p := range repo.players {
		if p.UserID == 10 {
			p.Seat = intPtr(2)
			p.Team = strPtr("teamA")
		}
	}

	rec := doSelectSeat(e, "1", `{"seat":2}`, token)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Seat should remain unchanged
	for _, p := range repo.players {
		if p.UserID == 10 {
			require.NotNil(t, p.Seat)
			assert.Equal(t, 2, *p.Seat)
		}
	}
}

func TestSelectSeat_Unauthorized(t *testing.T) {
	e, _ := setupTest()

	rec := doSelectSeat(e, "1", `{"seat":0}`, "")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- StartGame Tests ---

func doStartGame(e *echo.Echo, id string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/"+id+"/start", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestStartGame_Success(t *testing.T) {
	e, repo := setupTest()
	token := validToken(1) // owner

	seedRoomWithPlayers(repo, "Start Test", 1, 1, 2, 3, 4)
	// Seat all 4 players
	seats := []int{0, 1, 2, 3}
	teams := []string{"teamA", "teamB", "teamA", "teamB"}
	for i, p := range repo.players {
		p.Seat = intPtr(seats[i])
		p.Team = strPtr(teams[i])
	}

	rec := doStartGame(e, "1", token)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "playing", repo.rooms[0].Status)

	var resp map[string]room.Room
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "playing", resp["data"].Status)
}

func TestStartGame_NotOwner(t *testing.T) {
	e, repo := setupTest()
	token := validToken(10) // not the owner

	seedRoomWithPlayers(repo, "Not Owner", 1, 1, 10, 3, 4)
	for i, p := range repo.players {
		p.Seat = intPtr(i)
		teams := []string{"teamA", "teamB", "teamA", "teamB"}
		p.Team = strPtr(teams[i])
	}

	rec := doStartGame(e, "1", token)

	assert.Equal(t, http.StatusForbidden, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "NOT_ROOM_OWNER", errResp["error"]["code"])
}

func TestStartGame_NotAllSeated(t *testing.T) {
	e, repo := setupTest()
	token := validToken(1)

	seedRoomWithPlayers(repo, "Not All Seated", 1, 1, 2, 3, 4)
	// Only seat 2 out of 4
	repo.players[0].Seat = intPtr(0)
	repo.players[0].Team = strPtr("teamA")
	repo.players[1].Seat = intPtr(1)
	repo.players[1].Team = strPtr("teamB")

	rec := doStartGame(e, "1", token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "NOT_ALL_SEATED", errResp["error"]["code"])
}

func TestStartGame_RoomNotWaiting(t *testing.T) {
	e, repo := setupTest()
	token := validToken(1)

	seedRoomWithPlayers(repo, "Already Started", 1, 1, 2, 3, 4)
	repo.rooms[0].Status = "in_progress"

	rec := doStartGame(e, "1", token)

	assert.Equal(t, http.StatusConflict, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "MATCH_NOT_STARTABLE", errResp["error"]["code"])
}

func TestStartGame_RoomNotFound(t *testing.T) {
	e, _ := setupTest()
	token := validToken(1)

	rec := doStartGame(e, "999", token)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestStartGame_Unauthorized(t *testing.T) {
	e, _ := setupTest()

	rec := doStartGame(e, "1", "")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestStartGame_RejectsQuickPlayRoom(t *testing.T) {
	e, repo := setupTest()
	token := validToken(1)

	seedRoomWithPlayers(repo, "Quick Play QPTEST", 1, 1, 2, 3, 4)
	repo.rooms[0].IsQuickPlay = true

	rec := doStartGame(e, "1", token)

	assert.Equal(t, http.StatusConflict, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "MATCH_NOT_STARTABLE", errResp["error"]["code"])
}

// --- Quick Play Tests ---

func doQuickPlay(e *echo.Echo, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/quick-play", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func doQuickJoin(e *echo.Echo, id string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/"+id+"/quick-join", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// seedQuickPlayRoom creates a waiting quick-play room owned by ownerID, who is
// auto-seated at the lowest seats together with any extra occupants. Mirrors
// the state QuickPlay leaves a partially-filled room in. Returns the room.
func seedQuickPlayRoom(repo *mockRoomRepo, code string, seatedUserIDs ...uint) *room.Room {
	r := &room.Room{
		Name:        "Quick Play " + code,
		Code:        code,
		OwnerID:     seatedUserIDs[0],
		Variant:     "bitola",
		MatchMode:   "1001",
		TimerStyle:  "relaxed",
		IsQuickPlay: true,
		Status:      "waiting",
		PlayerCount: len(seatedUserIDs),
	}
	_ = repo.Create(r)
	for i, uid := range seatedUserIDs {
		seat := i
		team := "teamA"
		if seat%2 == 1 {
			team = "teamB"
		}
		_ = repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: uid, Seat: &seat, Team: &team})
	}
	return r
}

func decodeQuickJoinData(t *testing.T, rec *httptest.ResponseRecorder) struct {
	Room         room.Room `json:"room"`
	Seat         int       `json:"seat"`
	MatchStarted bool      `json:"matchStarted"`
} {
	t.Helper()
	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var data struct {
		Room         room.Room `json:"room"`
		Seat         int       `json:"seat"`
		MatchStarted bool      `json:"matchStarted"`
	}
	require.NoError(t, json.Unmarshal(resp["data"], &data))
	return data
}

func TestQuickJoin_SeatsPlayerInTargetRoom(t *testing.T) {
	e, repo := setupTest()
	qpRoom := seedQuickPlayRoom(repo, "ABC123", 20)

	rec := doQuickJoin(e, "1", validToken(30))

	assert.Equal(t, http.StatusOK, rec.Code)
	data := decodeQuickJoinData(t, rec)

	assert.Equal(t, qpRoom.ID, data.Room.ID)
	assert.Equal(t, 2, data.Room.PlayerCount)
	// Joiner gets the next empty seat (1 → team B) since seat 0 is taken.
	assert.Equal(t, 1, data.Seat)
	assert.False(t, data.MatchStarted)

	var seat *int
	var team *string
	for _, p := range repo.players {
		if p.UserID == 30 {
			seat, team = p.Seat, p.Team
		}
	}
	require.NotNil(t, seat)
	assert.Equal(t, 1, *seat)
	require.NotNil(t, team)
	assert.Equal(t, "teamB", *team)
}

func TestQuickJoin_FourthPlayerAutoStarts(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()
	qpRoom := seedQuickPlayRoom(repo, "FILLED", 500, 501, 502)

	rec := doQuickJoin(e, "1", validToken(503))

	assert.Equal(t, http.StatusOK, rec.Code)
	data := decodeQuickJoinData(t, rec)

	assert.Equal(t, 3, data.Seat)
	assert.True(t, data.MatchStarted)

	updated, _ := repo.FindByID(qpRoom.ID)
	assert.Equal(t, "playing", updated.Status)

	gotMatchStarted := false
	for _, call := range broadcaster.calls {
		if strings.Contains(string(call.msg), "system:match_started") {
			gotMatchStarted = true
		}
	}
	assert.True(t, gotMatchStarted, "expected system:match_started broadcast")
}

func TestQuickJoin_AlreadyInRoom(t *testing.T) {
	e, repo := setupTest()
	seedQuickPlayRoom(repo, "ABC123", 20)
	// User 30 is already in another room.
	other := seedQuickPlayRoom(repo, "OTHER1", 30)
	_ = other

	rec := doQuickJoin(e, "1", validToken(30))

	assert.Equal(t, http.StatusConflict, rec.Code)
	var resp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "ALREADY_IN_ROOM", resp["error"]["code"])
}

func TestQuickJoin_RoomNotQuickPlay(t *testing.T) {
	e, repo := setupTest()
	// A manual (custom) room — quick-join must be rejected.
	manual := &room.Room{
		Name: "Manual", Code: "MAN001", OwnerID: 50,
		Variant: "bitola", MatchMode: "1001", TimerStyle: "relaxed",
		IsQuickPlay: false, Status: "waiting", PlayerCount: 1,
	}
	_ = repo.Create(manual)
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: manual.ID, UserID: 50})

	rec := doQuickJoin(e, "1", validToken(60))

	assert.Equal(t, http.StatusConflict, rec.Code)
	var resp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "ROOM_NOT_QUICK_PLAY", resp["error"]["code"])
}

func TestQuickJoin_RoomFull(t *testing.T) {
	e, repo := setupTest()
	seedQuickPlayRoom(repo, "FULL01", 70, 71, 72, 73)

	rec := doQuickJoin(e, "1", validToken(80))

	assert.Equal(t, http.StatusConflict, rec.Code)
	var resp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "ROOM_FULL", resp["error"]["code"])
}

func TestQuickJoin_RoomNotFound(t *testing.T) {
	e, _ := setupTest()

	rec := doQuickJoin(e, "999", validToken(1))

	assert.Equal(t, http.StatusNotFound, rec.Code)
	var resp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "ROOM_NOT_FOUND", resp["error"]["code"])
}

func TestQuickJoin_NotWaitingStatus(t *testing.T) {
	e, repo := setupTest()
	qpRoom := seedQuickPlayRoom(repo, "PLAY01", 90, 91, 92, 93)
	qpRoom.Status = "playing"

	rec := doQuickJoin(e, "1", validToken(95))

	// Matches JoinRoom's convention: a non-waiting room reads as not found.
	assert.Equal(t, http.StatusNotFound, rec.Code)
	var resp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "ROOM_NOT_FOUND", resp["error"]["code"])
}

func TestQuickJoin_Unauthorized(t *testing.T) {
	e, _ := setupTest()

	rec := doQuickJoin(e, "1", "")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestQuickPlay_CreatesNewRoom(t *testing.T) {
	e, repo := setupTest()
	token := validToken(10)

	rec := doQuickPlay(e, token)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var data struct {
		Room         room.Room `json:"room"`
		Seat         int       `json:"seat"`
		MatchStarted bool      `json:"matchStarted"`
	}
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	assert.True(t, data.Room.IsQuickPlay)
	assert.Equal(t, "bitola", data.Room.Variant)
	assert.Equal(t, "1001", data.Room.MatchMode)
	// Story 9.4 (AC5): synthesized quick-play rooms default to per-move 30s.
	assert.Equal(t, "per-move", data.Room.TimerStyle)
	require.NotNil(t, data.Room.TimerDurationSeconds)
	assert.Equal(t, 30, *data.Room.TimerDurationSeconds)
	assert.Equal(t, "waiting", data.Room.Status)
	assert.Equal(t, 1, data.Room.PlayerCount)
	assert.Equal(t, uint(10), data.Room.OwnerID)
	assert.Contains(t, data.Room.Name, "Quick Play ")
	assert.Len(t, data.Room.Code, 6)

	// Auto-seat: first joiner lands at seat 0 (team A).
	assert.Equal(t, 0, data.Seat)
	assert.False(t, data.MatchStarted)

	require.Len(t, repo.players, 1)
	require.NotNil(t, repo.players[0].Seat)
	assert.Equal(t, 0, *repo.players[0].Seat)
	require.NotNil(t, repo.players[0].Team)
	assert.Equal(t, "teamA", *repo.players[0].Team)
}

func TestQuickPlay_JoinsExistingRoom(t *testing.T) {
	e, repo := setupTest()

	// Create an existing Quick Play room owned by user 20, who is already
	// auto-seated at seat 0 (the state Quick Play leaves a single-player room in).
	existingRoom := &room.Room{
		Name:        "Quick Play ABC123",
		Code:        "ABC123",
		OwnerID:     20,
		Variant:     "bitola",
		MatchMode:   "1001",
		TimerStyle:  "relaxed",
		IsQuickPlay: true,
		Status:      "waiting",
		PlayerCount: 1,
	}
	_ = repo.Create(existingRoom)
	seat0 := 0
	teamA := "teamA"
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: existingRoom.ID, UserID: 20, Seat: &seat0, Team: &teamA})

	token := validToken(30)
	rec := doQuickPlay(e, token)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var data struct {
		Room         room.Room `json:"room"`
		Seat         int       `json:"seat"`
		MatchStarted bool      `json:"matchStarted"`
	}
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	assert.Equal(t, existingRoom.ID, data.Room.ID)
	assert.Equal(t, 2, data.Room.PlayerCount)
	// Joiner gets the next empty seat (1 → team B) since seat 0 is taken.
	assert.Equal(t, 1, data.Seat)
	assert.False(t, data.MatchStarted)

	// Verify the joining player record has seat 1 / team B.
	var joinerSeat *int
	var joinerTeam *string
	for _, p := range repo.players {
		if p.UserID == 30 {
			joinerSeat = p.Seat
			joinerTeam = p.Team
		}
	}
	require.NotNil(t, joinerSeat)
	assert.Equal(t, 1, *joinerSeat)
	require.NotNil(t, joinerTeam)
	assert.Equal(t, "teamB", *joinerTeam)
}

func TestQuickPlay_AlreadyInRoom(t *testing.T) {
	e, repo := setupTest()

	// User 40 is already in a room
	existingRoom := &room.Room{
		Name:        "Some Room",
		Code:        "XYZ789",
		OwnerID:     40,
		Variant:     "bitola",
		MatchMode:   "1001",
		TimerStyle:  "relaxed",
		Status:      "waiting",
		PlayerCount: 1,
	}
	_ = repo.Create(existingRoom)
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: existingRoom.ID, UserID: 40})

	token := validToken(40)
	rec := doQuickPlay(e, token)

	assert.Equal(t, http.StatusConflict, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var errBody map[string]string
	require.NoError(t, json.Unmarshal(resp["error"], &errBody))
	assert.Equal(t, "ALREADY_IN_ROOM", errBody["code"])
}

func TestQuickPlay_Unauthorized(t *testing.T) {
	e, _ := setupTest()

	rec := doQuickPlay(e, "")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestQuickPlay_SkipsNonQuickPlayRooms(t *testing.T) {
	e, repo := setupTest()

	// A manual room with matching settings should NOT be matched
	manualRoom := &room.Room{
		Name:        "Manual Room",
		Code:        "MAN001",
		OwnerID:     50,
		Variant:     "bitola",
		MatchMode:   "1001",
		TimerStyle:  "relaxed",
		IsQuickPlay: false,
		Status:      "waiting",
		PlayerCount: 1,
	}
	_ = repo.Create(manualRoom)
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: manualRoom.ID, UserID: 50})

	token := validToken(60)
	rec := doQuickPlay(e, token)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var data struct {
		Room         room.Room `json:"room"`
		Seat         int       `json:"seat"`
		MatchStarted bool      `json:"matchStarted"`
	}
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	// Should create a new room, not join the manual one.
	assert.NotEqual(t, manualRoom.ID, data.Room.ID)
	assert.True(t, data.Room.IsQuickPlay)
	assert.Equal(t, 0, data.Seat)
	assert.False(t, data.MatchStarted)
}

func TestQuickPlay_FillsFirstEmptySeat(t *testing.T) {
	e, repo := setupTest()

	// Seats 0 and 2 are already occupied. The new joiner should land at seat 1
	// (the first empty seat in 0..3 order), NOT seat 3.
	qpRoom := &room.Room{
		Name:        "Quick Play GAPS",
		Code:        "GAPSEA",
		OwnerID:     400,
		Variant:     "bitola",
		MatchMode:   "1001",
		TimerStyle:  "relaxed",
		IsQuickPlay: true,
		Status:      "waiting",
		PlayerCount: 2,
	}
	_ = repo.Create(qpRoom)

	seat0, seat2 := 0, 2
	teamA := "teamA"
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 400, Seat: &seat0, Team: &teamA, Username: "P1"})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 401, Seat: &seat2, Team: &teamA, Username: "P3"})

	token := validToken(402)
	rec := doQuickPlay(e, token)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var data struct {
		Room         room.Room `json:"room"`
		Seat         int       `json:"seat"`
		MatchStarted bool      `json:"matchStarted"`
	}
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	assert.Equal(t, qpRoom.ID, data.Room.ID)
	assert.Equal(t, 1, data.Seat)
	assert.False(t, data.MatchStarted)

	var newJoinerSeat *int
	var newJoinerTeam *string
	for _, p := range repo.players {
		if p.UserID == 402 {
			newJoinerSeat = p.Seat
			newJoinerTeam = p.Team
		}
	}
	require.NotNil(t, newJoinerSeat)
	assert.Equal(t, 1, *newJoinerSeat)
	require.NotNil(t, newJoinerTeam)
	assert.Equal(t, "teamB", *newJoinerTeam)
}

func TestQuickPlay_AutoStartsOnFourthJoiner(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()

	// Three players are already auto-seated (0, 1, 2). The 4th QuickPlay call
	// should land at seat 3, fill the room, and trigger auto-start.
	qpRoom := &room.Room{
		Name:        "Quick Play FILL",
		Code:        "FILLED",
		OwnerID:     500,
		Variant:     "bitola",
		MatchMode:   "1001",
		TimerStyle:  "relaxed",
		IsQuickPlay: true,
		Status:      "waiting",
		PlayerCount: 3,
	}
	_ = repo.Create(qpRoom)

	seat0, seat1, seat2 := 0, 1, 2
	teamA, teamB := "teamA", "teamB"
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 500, Seat: &seat0, Team: &teamA, Username: "P1"})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 501, Seat: &seat1, Team: &teamB, Username: "P2"})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 502, Seat: &seat2, Team: &teamA, Username: "P3"})

	token := validToken(503)
	rec := doQuickPlay(e, token)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var data struct {
		Room         room.Room `json:"room"`
		Seat         int       `json:"seat"`
		MatchStarted bool      `json:"matchStarted"`
	}
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	assert.Equal(t, 3, data.Seat)
	assert.True(t, data.MatchStarted)

	updatedRoom, _ := repo.FindByID(qpRoom.ID)
	assert.Equal(t, "playing", updatedRoom.Status)

	// system:match_started must reach the room participants.
	gotMatchStarted := false
	for _, call := range broadcaster.calls {
		if strings.Contains(string(call.msg), "system:match_started") {
			gotMatchStarted = true
			break
		}
	}
	assert.True(t, gotMatchStarted, "expected system:match_started broadcast")
}

// TestLeaveRoom_ReturnsErrMatchAlreadyStarted_WhenRoomPlaying locks in Story
// 8.5-1 AC3: the LeaveRoom transaction must re-fetch the room inside the tx
// and reject with ErrMatchAlreadyStarted (HTTP 409) when status != "waiting".
// This prevents the race where a leave observes status="waiting" pre-tx and
// a concurrent auto-start tx flips status to "playing" between the read and
// the RemovePlayer write — the rules-engine would then receive a seatInfo
// snapshot containing a player who has already left.
func TestLeaveRoom_ReturnsErrMatchAlreadyStarted_WhenRoomPlaying(t *testing.T) {
	e, repo := setupTest()

	// Room is already in "playing" status — simulates a concurrent auto-start
	// having flipped status before this leave's tx body runs.
	playingRoom := &room.Room{
		Name:        "Started Room",
		Code:        "STARTD",
		OwnerID:     800,
		Variant:     "bitola",
		MatchMode:   "1001",
		TimerStyle:  "relaxed",
		IsQuickPlay: true,
		Status:      "playing",
		PlayerCount: 4,
	}
	require.NoError(t, repo.Create(playingRoom))
	seat0, seat1, seat2, seat3 := 0, 1, 2, 3
	teamA, teamB := "teamA", "teamB"
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: playingRoom.ID, UserID: 800, Seat: &seat0, Team: &teamA, Username: "P1"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: playingRoom.ID, UserID: 801, Seat: &seat1, Team: &teamB, Username: "P2"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: playingRoom.ID, UserID: 802, Seat: &seat2, Team: &teamA, Username: "P3"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: playingRoom.ID, UserID: 803, Seat: &seat3, Team: &teamB, Username: "P4"}))

	token := validToken(803)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/1/leave", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code, "leave during playing must surface 409 MATCH_ALREADY_STARTED")

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var errBody struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(resp["error"], &errBody))
	assert.Equal(t, "MATCH_ALREADY_STARTED", errBody.Code)

	// No player removed — the player count must remain 4 and the seat row
	// for user 803 must still exist.
	postRoom, _ := repo.FindByID(playingRoom.ID)
	require.NotNil(t, postRoom)
	assert.Equal(t, 4, postRoom.PlayerCount, "PlayerCount must NOT be decremented when leave is rejected")

	stillSeated := false
	for _, p := range repo.players {
		if p.UserID == 803 && p.RoomID == playingRoom.ID {
			stillSeated = true
			break
		}
	}
	assert.True(t, stillSeated, "user 803 must still be in the room — RemovePlayer must not have run")
}

// --- Test fakes for Story 8.5-1 AC2 (gameStarter) ---

type fakeMatchStarter struct {
	called               int
	lastRoom             uint
	lastPlayers          [4]match.PlayerSeatInfo
	lastCoinBuyIn        int
	lastTimerStyle       string
	lastTimerDurationSec int
	err                  error
}

func (g *fakeMatchStarter) StartMatch(roomID uint, _ string, _ string, players [4]match.PlayerSeatInfo, timerStyle string, timerDurationSec int, _ uint, _ int, coinBuyIn int) error {
	g.called++
	g.lastRoom = roomID
	g.lastPlayers = players
	g.lastCoinBuyIn = coinBuyIn
	g.lastTimerStyle = timerStyle
	g.lastTimerDurationSec = timerDurationSec
	return g.err
}

func setupTestWithStarter(starter room.MatchStarter, broadcaster room.Broadcaster) (*echo.Echo, *mockRoomRepo) {
	repo := newMockRoomRepo()
	handler := room.NewRoomHandler(repo, starter, broadcaster, nil, nil)

	e := echo.New()
	e.HTTPErrorHandler = testErrorHandler
	api := e.Group("/api/v1", auth.AuthMiddleware("test-jwt-secret"))
	api.POST("/rooms", handler.CreateRoom)
	api.POST("/rooms/quick-play", handler.QuickPlay)
	api.POST("/rooms/:id/join", handler.JoinRoom)
	api.POST("/rooms/:id/quick-join", handler.QuickJoin)
	api.POST("/rooms/:id/leave", handler.LeaveRoom)
	api.POST("/rooms/:id/return", handler.ReturnToRoom)
	api.POST("/rooms/:id/seat", handler.SelectSeat)
	api.POST("/rooms/:id/leave-seat", handler.LeaveSeat)
	api.POST("/rooms/:id/start", handler.StartMatch)
	api.POST("/rooms/:id/bots", handler.AddBot)
	api.DELETE("/rooms/:id/bots/:seat", handler.RemoveBot)
	api.POST("/rooms/:id/swap-seats", handler.SwapSeats)
	return e, repo
}

func broadcastTypes(t *testing.T, b *mockBroadcaster) []string {
	t.Helper()
	out := make([]string, 0, len(b.calls)+len(b.allCalls))
	for _, c := range b.calls {
		out = append(out, msgTypeOf(t, c.msg))
	}
	for _, c := range b.allCalls {
		out = append(out, msgTypeOf(t, c.msg))
	}
	return out
}

func containsString(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// TestSelectSeat_AutoStart_BroadcastsWhenStartGameSucceeds locks in Story
// 8.5-1 AC2 success path: when the 4th seat fills and StartGame succeeds,
// system:match_started is broadcast and the room status sticks at "playing".
func TestSelectSeat_AutoStart_BroadcastsWhenStartGameSucceeds(t *testing.T) {
	starter := &fakeMatchStarter{}
	broadcaster := &mockBroadcaster{}
	e, repo := setupTestWithStarter(starter, broadcaster)

	qpRoom := &room.Room{
		Name:        "Quick Play AC2OK",
		Code:        "AC2OK1",
		OwnerID:     500,
		Variant:     "bitola",
		MatchMode:   "1001",
		TimerStyle:  "relaxed",
		IsQuickPlay: true,
		Status:      "waiting",
		PlayerCount: 4,
	}
	require.NoError(t, repo.Create(qpRoom))
	seat0, seat1, seat2 := 0, 1, 2
	teamA, teamB := "teamA", "teamB"
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 500, Seat: &seat0, Team: &teamA, Username: "P1"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 501, Seat: &seat1, Team: &teamB, Username: "P2"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 502, Seat: &seat2, Team: &teamA, Username: "P3"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 503, Username: "P4"}))

	token := validToken(503)
	rec := doSelectSeat(e, "1", `{"seat": 3}`, token)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, starter.called, "StartGame must be called exactly once on the 4th seat")

	updated, _ := repo.FindByID(qpRoom.ID)
	require.NotNil(t, updated)
	assert.Equal(t, "playing", updated.Status, "room status must remain 'playing' on StartGame success")

	types := broadcastTypes(t, broadcaster)
	assert.True(t, containsString(types, "system:match_started"), "system:match_started must be broadcast on success")
	assert.False(t, containsString(types, "error:match_start_failed"), "no error event on success")
}

// TestSelectSeat_AutoStart_RevertsWhenStartGameFails locks in Story 8.5-1 AC2
// failure path: when StartGame returns an error, the status flip is reverted
// to "waiting", system:match_started is NOT broadcast, and
// error:match_start_failed reaches the four would-be participants.
// playerCount and seat assignments survive (no partial writes lost).
func TestSelectSeat_AutoStart_RevertsWhenStartGameFails(t *testing.T) {
	starter := &fakeMatchStarter{err: errors.New("session manager unavailable")}
	broadcaster := &mockBroadcaster{}
	e, repo := setupTestWithStarter(starter, broadcaster)

	qpRoom := &room.Room{
		Name:        "Quick Play AC2FAIL",
		Code:        "AC2F01",
		OwnerID:     600,
		Variant:     "bitola",
		MatchMode:   "1001",
		TimerStyle:  "relaxed",
		IsQuickPlay: true,
		Status:      "waiting",
		PlayerCount: 4,
	}
	require.NoError(t, repo.Create(qpRoom))
	seat0, seat1, seat2 := 0, 1, 2
	teamA, teamB := "teamA", "teamB"
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 600, Seat: &seat0, Team: &teamA, Username: "P1"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 601, Seat: &seat1, Team: &teamB, Username: "P2"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 602, Seat: &seat2, Team: &teamA, Username: "P3"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 603, Username: "P4"}))

	token := validToken(603)
	rec := doSelectSeat(e, "1", `{"seat": 3}`, token)

	// HTTP success — the seat selection itself succeeded; the auto-start
	// failure is communicated via the error WS event, not an HTTP error.
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, starter.called, "StartGame must be called once")

	updated, _ := repo.FindByID(qpRoom.ID)
	require.NotNil(t, updated)
	assert.Equal(t, "waiting", updated.Status, "room status MUST be reverted to 'waiting' on StartGame failure")

	types := broadcastTypes(t, broadcaster)
	assert.False(t, containsString(types, "system:match_started"), "system:match_started MUST NOT be broadcast on StartGame failure")
	assert.True(t, containsString(types, "error:match_start_failed"), "error:match_start_failed MUST be broadcast to participants")

	// Verify the error reached all four would-be participants.
	gotErrUserIDs := map[uint]bool{}
	for _, c := range broadcaster.calls {
		if msgTypeOf(t, c.msg) == "error:match_start_failed" {
			for _, uid := range c.userIDs {
				gotErrUserIDs[uid] = true
			}
		}
	}
	for _, expectedUID := range []uint{600, 601, 602, 603} {
		assert.True(t, gotErrUserIDs[expectedUID], "user %d must receive error:match_start_failed", expectedUID)
	}

	// Seat assignments + player count survive the rollback: only the room
	// status flipped back. Players are still seated; PlayerCount is still 4.
	players, err := repo.FindPlayersByRoomID(qpRoom.ID)
	require.NoError(t, err)
	assert.Len(t, players, 4, "all four players still seated")
	assert.Equal(t, 4, updated.PlayerCount, "PlayerCount preserved across rollback")
}

// TestQuickPlay_AutoStart_RevertsWhenStartGameFails locks in the same AC2
// failure-path invariant for the QuickPlay last-seat auto-start path.
func TestQuickPlay_AutoStart_RevertsWhenStartGameFails(t *testing.T) {
	starter := &fakeMatchStarter{err: errors.New("session manager unavailable")}
	broadcaster := &mockBroadcaster{}
	e, repo := setupTestWithStarter(starter, broadcaster)

	qpRoom := &room.Room{
		Name:        "Quick Play AC2QPFAIL",
		Code:        "AC2QF1",
		OwnerID:     700,
		Variant:     "bitola",
		MatchMode:   "1001",
		TimerStyle:  "relaxed",
		IsQuickPlay: true,
		Status:      "waiting",
		PlayerCount: 3,
	}
	require.NoError(t, repo.Create(qpRoom))
	seat0, seat1, seat2 := 0, 1, 2
	teamA, teamB := "teamA", "teamB"
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 700, Seat: &seat0, Team: &teamA, Username: "P1"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 701, Seat: &seat1, Team: &teamB, Username: "P2"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 702, Seat: &seat2, Team: &teamA, Username: "P3"}))

	token := validToken(703)
	rec := doQuickPlay(e, token)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, starter.called, "StartGame must be called once on the 4th joiner")

	updated, _ := repo.FindByID(qpRoom.ID)
	require.NotNil(t, updated)
	assert.Equal(t, "waiting", updated.Status, "room status MUST be reverted to 'waiting' on StartGame failure")

	types := broadcastTypes(t, broadcaster)
	assert.False(t, containsString(types, "system:match_started"), "system:match_started MUST NOT be broadcast on StartGame failure")
	assert.True(t, containsString(types, "error:match_start_failed"), "error:match_start_failed MUST be broadcast to participants")
}

// TestQuickPlay_RetriesOnErrRoomFull locks in Story 8.5-1 AC5 / D29 symptom fix.
//
// The QuickPlay retry loop must continue when pickFirstEmptySeat raises
// apperr.ErrRoomFull (caused by the player_count denormalized counter saying a
// room has free seats but every seat row is occupied — i.e. counter drift).
// Counter drift must surface as a successful join into a different/new room,
// never as an opaque 5xx.
//
// Setup: a "drifted" QuickPlay room reports PlayerCount=3 but already has all
// four seats occupied. First iteration: FindQuickPlayRoom returns the drifted
// room → pickFirstEmptySeat fails with ErrRoomFull. With the fix, the loop
// continues; on the second iteration FindQuickPlayRoom returns nil (the
// drifted room's PlayerCount has reached 4 from the failed iteration's
// IncrementPlayerCount) and a fresh room is created instead.
func TestQuickPlay_RetriesOnErrRoomFull(t *testing.T) {
	e, repo := setupTest()

	// Drifted room: counter says 3, but all 4 seats are filled.
	driftedRoom := &room.Room{
		Name:        "Quick Play DRIFT",
		Code:        "DRIFT1",
		OwnerID:     900,
		Variant:     "bitola",
		MatchMode:   "1001",
		TimerStyle:  "relaxed",
		IsQuickPlay: true,
		Status:      "waiting",
		PlayerCount: 3,
	}
	require.NoError(t, repo.Create(driftedRoom))

	seat0, seat1, seat2, seat3 := 0, 1, 2, 3
	teamA, teamB := "teamA", "teamB"
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: driftedRoom.ID, UserID: 900, Seat: &seat0, Team: &teamA, Username: "P1"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: driftedRoom.ID, UserID: 901, Seat: &seat1, Team: &teamB, Username: "P2"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: driftedRoom.ID, UserID: 902, Seat: &seat2, Team: &teamA, Username: "P3"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: driftedRoom.ID, UserID: 903, Seat: &seat3, Team: &teamB, Username: "P4"}))

	token := validToken(904)
	rec := doQuickPlay(e, token)

	// Without the AC5 fix this would surface ErrRoomFull as 500 (or the
	// apperr-mapped 4xx). With the fix the loop retries; second iteration
	// creates a new room and the user lands cleanly.
	require.Equal(t, http.StatusOK, rec.Code, "ErrRoomFull from counter drift must be retried, not surfaced as an error")

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var data struct {
		Room         room.Room `json:"room"`
		Seat         int       `json:"seat"`
		MatchStarted bool      `json:"matchStarted"`
	}
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	assert.NotEqual(t, driftedRoom.ID, data.Room.ID, "user must land in a different room than the drifted one")
	assert.True(t, data.Room.IsQuickPlay)
	assert.Equal(t, 0, data.Seat, "first joiner of the new room sits at seat 0")
	assert.False(t, data.MatchStarted)
}

// --- SelectSeat Auto-Start Tests ---

func TestSelectSeat_QuickPlayAutoStart(t *testing.T) {
	e, repo := setupTest()

	qpRoom := &room.Room{
		Name:        "Quick Play QPAUTO",
		Code:        "QPAUTO",
		OwnerID:     100,
		Variant:     "bitola",
		MatchMode:   "1001",
		TimerStyle:  "relaxed",
		IsQuickPlay: true,
		Status:      "waiting",
		PlayerCount: 4,
	}
	_ = repo.Create(qpRoom)

	seat0 := 0
	seat1 := 1
	seat2 := 2
	teamA := "teamA"
	teamB := "teamB"

	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 100, Seat: &seat0, Team: &teamA, Username: "P1"})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 101, Seat: &seat1, Team: &teamB, Username: "P2"})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 102, Seat: &seat2, Team: &teamA, Username: "P3"})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 103, Username: "P4"})

	// Player 103 selects seat 3 — this is the 4th seat, should trigger auto-start
	token := validToken(103)
	rec := doSelectSeat(e, "1", `{"seat": 3}`, token)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var data map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	var matchStarted bool
	require.NoError(t, json.Unmarshal(data["matchStarted"], &matchStarted))
	assert.True(t, matchStarted)

	// Room status should be "playing"
	updatedRoom, _ := repo.FindByID(qpRoom.ID)
	assert.Equal(t, "playing", updatedRoom.Status)
}

func TestSelectSeat_QuickPlayNoAutoStartWith3Seats(t *testing.T) {
	e, repo := setupTest()

	qpRoom := &room.Room{
		Name:        "Quick Play QP3",
		Code:        "QP3SEA",
		OwnerID:     200,
		Variant:     "bitola",
		MatchMode:   "1001",
		TimerStyle:  "relaxed",
		IsQuickPlay: true,
		Status:      "waiting",
		PlayerCount: 4,
	}
	_ = repo.Create(qpRoom)

	seat0 := 0
	seat1 := 1
	teamA := "teamA"
	teamB := "teamB"

	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 200, Seat: &seat0, Team: &teamA, Username: "P1"})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 201, Seat: &seat1, Team: &teamB, Username: "P2"})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 202, Username: "P3"})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 203, Username: "P4"})

	// Player 202 selects seat 2 — only 3 seated now
	token := validToken(202)
	rec := doSelectSeat(e, "1", `{"seat": 2}`, token)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var data map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	var matchStarted bool
	require.NoError(t, json.Unmarshal(data["matchStarted"], &matchStarted))
	assert.False(t, matchStarted)

	// Room status should still be "waiting"
	updatedRoom, _ := repo.FindByID(qpRoom.ID)
	assert.Equal(t, "waiting", updatedRoom.Status)
}

func TestSelectSeat_ManualRoomNoAutoStart(t *testing.T) {
	e, repo := setupTest()

	manualRoom := &room.Room{
		Name:        "Manual Room Auto",
		Code:        "MANUAL",
		OwnerID:     300,
		Variant:     "bitola",
		MatchMode:   "1001",
		TimerStyle:  "relaxed",
		IsQuickPlay: false,
		Status:      "waiting",
		PlayerCount: 4,
	}
	_ = repo.Create(manualRoom)

	seat0 := 0
	seat1 := 1
	seat2 := 2
	teamA := "teamA"
	teamB := "teamB"

	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: manualRoom.ID, UserID: 300, Seat: &seat0, Team: &teamA, Username: "P1"})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: manualRoom.ID, UserID: 301, Seat: &seat1, Team: &teamB, Username: "P2"})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: manualRoom.ID, UserID: 302, Seat: &seat2, Team: &teamA, Username: "P3"})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: manualRoom.ID, UserID: 303, Username: "P4"})

	// Player 303 selects seat 3 — all 4 seated but NOT a Quick Play room
	token := validToken(303)
	rec := doSelectSeat(e, "1", `{"seat": 3}`, token)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var data map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	var matchStarted bool
	require.NoError(t, json.Unmarshal(data["matchStarted"], &matchStarted))
	assert.False(t, matchStarted)

	// Room status should still be "waiting"
	updatedRoom, _ := repo.FindByID(manualRoom.ID)
	assert.Equal(t, "waiting", updatedRoom.Status)
}

// --- Broadcast Tests ---

func TestJoinRoom_BroadcastsPlayerJoined(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()

	ownerRoom := &room.Room{Name: "Broadcast Test", OwnerID: 100, Status: "waiting", PlayerCount: 1}
	_ = repo.Create(ownerRoom)
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: ownerRoom.ID, UserID: 100, Username: "Owner"})

	// Player 200 joins
	token := validToken(200)
	rec := doJoinRoom(e, "1", token)
	assert.Equal(t, http.StatusOK, rec.Code)

	require.Len(t, broadcaster.calls, 1, "expected one BroadcastToUsers call")
	call := broadcaster.calls[0]

	// Broadcast should include both existing (100) and joining (200) player
	assert.ElementsMatch(t, []uint{100, 200}, call.userIDs)

	// Verify the message type is system:player_joined
	var msg map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(call.msg, &msg))
	var msgType string
	require.NoError(t, json.Unmarshal(msg["type"], &msgType))
	assert.Equal(t, "system:player_joined", msgType)

	// Verify payload
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(msg["payload"], &payload))
	var userId float64
	require.NoError(t, json.Unmarshal(payload["userId"], &userId))
	assert.Equal(t, float64(200), userId)
	var playerCount float64
	require.NoError(t, json.Unmarshal(payload["playerCount"], &playerCount))
	assert.Equal(t, float64(2), playerCount)
}

func TestLeaveRoom_BroadcastsPlayerLeft(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()

	ownerRoom := &room.Room{Name: "Leave Test", OwnerID: 100, Status: "waiting", PlayerCount: 2}
	_ = repo.Create(ownerRoom)
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: ownerRoom.ID, UserID: 100, Username: "Owner"})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: ownerRoom.ID, UserID: 200, Username: "Player2"})

	// Player 200 leaves
	token := validToken(200)
	rec := doLeaveRoom(e, "1", token)
	assert.Equal(t, http.StatusOK, rec.Code)

	require.Len(t, broadcaster.calls, 1, "expected one BroadcastToUsers call")
	call := broadcaster.calls[0]

	// Broadcast should only go to remaining player (100)
	assert.ElementsMatch(t, []uint{100}, call.userIDs)

	// Verify the message type is system:player_left
	var msg map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(call.msg, &msg))
	var msgType string
	require.NoError(t, json.Unmarshal(msg["type"], &msgType))
	assert.Equal(t, "system:player_left", msgType)

	// Verify payload contains leaving player's info
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(msg["payload"], &payload))
	var userId float64
	require.NoError(t, json.Unmarshal(payload["userId"], &userId))
	assert.Equal(t, float64(200), userId)
}

func TestLeaveRoom_OwnerTransfer_BroadcastsNewOwner(t *testing.T) {
	e, repo, broadcaster, reg := setupTestWithBroadcastAndPresence()

	ownerRoom := &room.Room{Name: "Owner Leave", OwnerID: 100, Status: "waiting", PlayerCount: 2}
	_ = repo.Create(ownerRoom)
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: ownerRoom.ID, UserID: 100, Username: "Owner", Seat: intPtr(0)})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: ownerRoom.ID, UserID: 200, Username: "Player2", Seat: intPtr(1)})
	reg.Add(ownerRoom.ID, 200) // Story 9.3: heir must be present

	// Owner (100) leaves — ownership should transfer to seated player 200
	token := validToken(100)
	rec := doLeaveRoom(e, "1", token)
	assert.Equal(t, http.StatusOK, rec.Code)

	require.Len(t, broadcaster.calls, 1)
	var msg map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(broadcaster.calls[0].msg, &msg))
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(msg["payload"], &payload))

	// newOwnerId should be present
	var newOwnerId float64
	require.NoError(t, json.Unmarshal(payload["newOwnerId"], &newOwnerId))
	assert.Equal(t, float64(200), newOwnerId)
}

func TestSelectSeat_BroadcastsSeatUpdated(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()

	testRoom := &room.Room{Name: "Seat Test", OwnerID: 100, Status: "waiting", PlayerCount: 2}
	_ = repo.Create(testRoom)
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: testRoom.ID, UserID: 100, Username: "P1"})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: testRoom.ID, UserID: 200, Username: "P2"})

	token := validToken(200)
	rec := doSelectSeat(e, "1", `{"seat": 2}`, token)
	assert.Equal(t, http.StatusOK, rec.Code)

	require.Len(t, broadcaster.calls, 1, "expected one BroadcastToUsers call for seat_updated")
	var msg map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(broadcaster.calls[0].msg, &msg))
	var msgType string
	require.NoError(t, json.Unmarshal(msg["type"], &msgType))
	assert.Equal(t, "system:seat_updated", msgType)

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(msg["payload"], &payload))
	var seatVal float64
	require.NoError(t, json.Unmarshal(payload["seat"], &seatVal))
	assert.Equal(t, float64(2), seatVal)
	var team string
	require.NoError(t, json.Unmarshal(payload["team"], &team))
	assert.Equal(t, "teamA", team)
}

func TestStartGame_BroadcastsGameStarted(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()

	testRoom := &room.Room{Name: "Start Test", OwnerID: 100, Status: "waiting", PlayerCount: 4}
	_ = repo.Create(testRoom)

	seat0 := 0
	seat1 := 1
	seat2 := 2
	seat3 := 3
	teamA := "teamA"
	teamB := "teamB"
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: testRoom.ID, UserID: 100, Seat: &seat0, Team: &teamA, Username: "P1"})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: testRoom.ID, UserID: 200, Seat: &seat1, Team: &teamB, Username: "P2"})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: testRoom.ID, UserID: 300, Seat: &seat2, Team: &teamA, Username: "P3"})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: testRoom.ID, UserID: 400, Seat: &seat3, Team: &teamB, Username: "P4"})

	token := validToken(100)
	rec := doStartGame(e, "1", token)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Should have at least one broadcast for match_started
	require.GreaterOrEqual(t, len(broadcaster.calls), 1, "expected BroadcastToUsers call for match_started")

	// Find the match_started message
	found := false
	for _, call := range broadcaster.calls {
		var msg map[string]json.RawMessage
		if err := json.Unmarshal(call.msg, &msg); err != nil {
			continue
		}
		var msgType string
		if err := json.Unmarshal(msg["type"], &msgType); err != nil {
			continue
		}
		if msgType == "system:match_started" {
			found = true
			// All 4 players should receive the broadcast
			assert.ElementsMatch(t, []uint{100, 200, 300, 400}, call.userIDs)
			break
		}
	}
	assert.True(t, found, "expected system:match_started broadcast")
}

func TestQuickPlayAutoStart_BroadcastsGameStarted(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()

	qpRoom := &room.Room{Name: "QP Room", OwnerID: 100, Status: "waiting", PlayerCount: 4, IsQuickPlay: true}
	_ = repo.Create(qpRoom)

	seat0 := 0
	seat1 := 1
	seat2 := 2
	teamA := "teamA"
	teamB := "teamB"
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 100, Seat: &seat0, Team: &teamA, Username: "P1"})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 200, Seat: &seat1, Team: &teamB, Username: "P2"})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 300, Seat: &seat2, Team: &teamA, Username: "P3"})
	_ = repo.AddPlayer(&room.RoomPlayer{RoomID: qpRoom.ID, UserID: 400, Username: "P4"})

	// Player 400 selects seat 3 — all 4 seated, Quick Play auto-start should trigger
	token := validToken(400)
	rec := doSelectSeat(e, "1", `{"seat": 3}`, token)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Should have broadcasts for seat_updated AND match_started
	require.GreaterOrEqual(t, len(broadcaster.calls), 2, "expected seat_updated + match_started broadcasts")

	// Find the match_started message
	found := false
	for _, call := range broadcaster.calls {
		var msg map[string]json.RawMessage
		if err := json.Unmarshal(call.msg, &msg); err != nil {
			continue
		}
		var msgType string
		if err := json.Unmarshal(msg["type"], &msgType); err != nil {
			continue
		}
		if msgType == "system:match_started" {
			found = true
			assert.ElementsMatch(t, []uint{100, 200, 300, 400}, call.userIDs)
			break
		}
	}
	assert.True(t, found, "expected system:match_started broadcast for quick play auto-start")
}

func TestCreateRoom_BroadcastsRoomCreated(t *testing.T) {
	e, _, broadcaster := setupTestWithBroadcast()

	token := validToken(1)
	rec := doCreateRoom(e, `{"name":"Broadcast Room","variant":"bitola","matchMode":"1001","timerStyle":"relaxed"}`, token)
	assert.Equal(t, http.StatusCreated, rec.Code)

	require.Len(t, broadcaster.allCalls, 1, "expected one BroadcastAll call")
	var msg map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(broadcaster.allCalls[0].msg, &msg))
	var msgType string
	require.NoError(t, json.Unmarshal(msg["type"], &msgType))
	assert.Equal(t, "system:room_created", msgType)

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(msg["payload"], &payload))
	var roomName string
	require.NoError(t, json.Unmarshal(payload["name"], &roomName))
	assert.Equal(t, "Broadcast Room", roomName)
}

// --- KickPlayer Tests ---

func doKickPlayer(e *echo.Echo, id string, body string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/"+id+"/kick", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func msgTypeOf(t *testing.T, raw []byte) string {
	t.Helper()
	var msg map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &msg))
	var msgType string
	require.NoError(t, json.Unmarshal(msg["type"], &msgType))
	return msgType
}

// payloadOf returns the raw JSON of a broadcast message's payload field.
func payloadOf(t *testing.T, raw []byte) []byte {
	t.Helper()
	var msg map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &msg))
	return msg["payload"]
}

func TestKickPlayer_Success(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()

	r := &room.Room{Name: "Kick Test", OwnerID: 100, Status: "waiting", PlayerCount: 3}
	require.NoError(t, repo.Create(r))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 100, Username: "Owner"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 200, Username: "P2"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 300, Username: "P3"}))

	token := validToken(100)
	rec := doKickPlayer(e, "1", `{"userId": 200}`, token)
	require.Equal(t, http.StatusOK, rec.Code)

	// Player count decremented
	updated, _ := repo.FindByID(r.ID)
	require.NotNil(t, updated)
	assert.Equal(t, 2, updated.PlayerCount)

	// 200 removed from room_players
	prooms, _ := repo.FindPlayersByRoomID(r.ID)
	require.Len(t, prooms, 2)
	for _, p := range prooms {
		assert.NotEqual(t, uint(200), p.UserID)
	}

	// Response shape
	var resp map[string]map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, float64(2), resp["data"]["playerCount"])

	// Three broadcasts: kicked user, remaining members, lobby-wide
	require.Len(t, broadcaster.calls, 2, "expected 2 BroadcastToUsers calls")
	require.Len(t, broadcaster.allCalls, 1, "expected 1 BroadcastAll call (room_updated)")

	// First call goes to the kicked user with system:room_kicked
	assert.ElementsMatch(t, []uint{200}, broadcaster.calls[0].userIDs)
	assert.Equal(t, "system:room_kicked", msgTypeOf(t, broadcaster.calls[0].msg))

	// Second call goes to the remaining members with system:player_left
	assert.ElementsMatch(t, []uint{100, 300}, broadcaster.calls[1].userIDs)
	assert.Equal(t, "system:player_left", msgTypeOf(t, broadcaster.calls[1].msg))

	// Lobby-wide broadcast is system:room_updated
	assert.Equal(t, "system:room_updated", msgTypeOf(t, broadcaster.allCalls[0].msg))

	// Verify kick payload contents
	var kickMsg map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(broadcaster.calls[0].msg, &kickMsg))
	var kickPayload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(kickMsg["payload"], &kickPayload))
	var reason string
	require.NoError(t, json.Unmarshal(kickPayload["reason"], &reason))
	assert.Equal(t, "kicked_by_owner", reason)

	// Verify player_left payload includes the kicked user's username
	var leftMsg map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(broadcaster.calls[1].msg, &leftMsg))
	var leftPayload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(leftMsg["payload"], &leftPayload))
	var leftUserID float64
	require.NoError(t, json.Unmarshal(leftPayload["userId"], &leftUserID))
	assert.Equal(t, float64(200), leftUserID)
	var leftUsername string
	require.NoError(t, json.Unmarshal(leftPayload["username"], &leftUsername))
	assert.Equal(t, "P2", leftUsername)
	var pc float64
	require.NoError(t, json.Unmarshal(leftPayload["playerCount"], &pc))
	assert.Equal(t, float64(2), pc)
}

func TestKickPlayer_NotOwner(t *testing.T) {
	e, repo := setupTest()

	r := &room.Room{Name: "Kick NotOwner", OwnerID: 100, Status: "waiting", PlayerCount: 2}
	require.NoError(t, repo.Create(r))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 100, Username: "Owner"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 200, Username: "P2"}))

	token := validToken(200) // not the owner
	rec := doKickPlayer(e, "1", `{"userId": 100}`, token)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "NOT_ROOM_OWNER", errResp["error"]["code"])

	// State unchanged
	updated, _ := repo.FindByID(r.ID)
	assert.Equal(t, 2, updated.PlayerCount)
}

func TestKickPlayer_RoomNotWaiting(t *testing.T) {
	e, repo := setupTest()

	r := &room.Room{Name: "Kick Playing", OwnerID: 100, Status: "playing", PlayerCount: 4}
	require.NoError(t, repo.Create(r))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 100, Username: "Owner"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 200, Username: "P2"}))

	token := validToken(100)
	rec := doKickPlayer(e, "1", `{"userId": 200}`, token)
	assert.Equal(t, http.StatusConflict, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "ROOM_NOT_WAITING", errResp["error"]["code"])

	// State unchanged
	updated, _ := repo.FindByID(r.ID)
	assert.Equal(t, 4, updated.PlayerCount)
}

func TestKickPlayer_MissingUserID(t *testing.T) {
	e, repo := setupTest()

	r := &room.Room{Name: "Kick Bad Body", OwnerID: 100, Status: "waiting", PlayerCount: 2}
	require.NoError(t, repo.Create(r))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 100, Username: "Owner"}))

	token := validToken(100)

	// missing userId
	rec := doKickPlayer(e, "1", `{}`, token)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "BAD_REQUEST", errResp["error"]["code"])

	// userId == 0
	rec2 := doKickPlayer(e, "1", `{"userId": 0}`, token)
	assert.Equal(t, http.StatusBadRequest, rec2.Code)
	var errResp2 map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &errResp2))
	assert.Equal(t, "BAD_REQUEST", errResp2["error"]["code"])
}

func TestKickPlayer_CannotKickSelf(t *testing.T) {
	e, repo := setupTest()

	r := &room.Room{Name: "Kick Self", OwnerID: 100, Status: "waiting", PlayerCount: 2}
	require.NoError(t, repo.Create(r))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 100, Username: "Owner"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 200, Username: "P2"}))

	token := validToken(100)
	rec := doKickPlayer(e, "1", `{"userId": 100}`, token)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "CANNOT_KICK_SELF", errResp["error"]["code"])
}

func TestKickPlayer_NotInRoom(t *testing.T) {
	e, repo := setupTest()

	r := &room.Room{Name: "Kick Stranger", OwnerID: 100, Status: "waiting", PlayerCount: 2}
	require.NoError(t, repo.Create(r))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 100, Username: "Owner"}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 200, Username: "P2"}))

	token := validToken(100)
	// userID 999 is not a member of this room
	rec := doKickPlayer(e, "1", `{"userId": 999}`, token)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "NOT_IN_ROOM", errResp["error"]["code"])
}

// TestKickPlayer_PreviousOwnerLosesPermission locks in TOCTOU behaviour:
// after the original owner leaves and ownership transfers, the original
// owner's authorization no longer survives the in-tx ownership re-check.
func TestKickPlayer_PreviousOwnerLosesPermission(t *testing.T) {
	e, repo, reg := setupTestWithPresence()

	r := &room.Room{Name: "Kick Race", OwnerID: 100, Status: "waiting", PlayerCount: 3}
	require.NoError(t, repo.Create(r))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 100, Username: "A", Seat: intPtr(0)}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 200, Username: "B", Seat: intPtr(1)}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 300, Username: "C", Seat: intPtr(2)}))
	// Story 9.3: heir must be present for the transfer on owner-leave.
	reg.Add(r.ID, 200)
	reg.Add(r.ID, 300)

	// A leaves first — ownership transfers to seated player B (200)
	leaveRec := doLeaveRoom(e, "1", validToken(100))
	require.Equal(t, http.StatusOK, leaveRec.Code)
	updated, _ := repo.FindByID(r.ID)
	require.Equal(t, uint(200), updated.OwnerID)

	// A (no longer owner) attempts to kick C — must return 403.
	// In the mock, A is no longer a member, but the owner-check inside the tx
	// is what matters: A's userID != current OwnerID == 200.
	rec := doKickPlayer(e, "1", `{"userId": 300}`, validToken(100))
	assert.Equal(t, http.StatusForbidden, rec.Code)
	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "NOT_ROOM_OWNER", errResp["error"]["code"])
}

func TestKickPlayer_Unauthorized(t *testing.T) {
	e, _ := setupTest()
	rec := doKickPlayer(e, "1", `{"userId": 200}`, "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- SwapSeats Tests ---

func doSwapSeats(e *echo.Echo, id string, body string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/"+id+"/swap-seats", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// seedSeatedRoom seeds a room with the owner at seat 0 and three other
// players at seats 1, 2, 3 (team A/team B/team A/team B).
func seedSeatedRoom(t *testing.T, repo *mockRoomRepo) *room.Room {
	t.Helper()
	r := &room.Room{Name: "Swap Test", OwnerID: 100, Status: "waiting", PlayerCount: 4}
	require.NoError(t, repo.Create(r))
	seats := []int{0, 1, 2, 3}
	teams := []string{"teamA", "teamB", "teamA", "teamB"}
	users := []uint{100, 200, 300, 400}
	names := []string{"Owner", "P2", "P3", "P4"}
	for i, uid := range users {
		seat := seats[i]
		team := teams[i]
		require.NoError(t, repo.AddPlayer(&room.RoomPlayer{
			RoomID:   r.ID,
			UserID:   uid,
			Username: names[i],
			Seat:     &seat,
			Team:     &team,
		}))
	}
	return r
}

func TestSwapSeats_Success(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()
	r := seedSeatedRoom(t, repo)

	token := validToken(100) // owner
	rec := doSwapSeats(e, "1", `{"seatA": 0, "seatB": 1}`, token)
	require.Equal(t, http.StatusOK, rec.Code)

	// User 100 should now be at seat 1 (team B), user 200 at seat 0 (team A)
	players, _ := repo.FindPlayersByRoomID(r.ID)
	for _, p := range players {
		switch p.UserID {
		case 100:
			require.NotNil(t, p.Seat)
			assert.Equal(t, 1, *p.Seat)
			require.NotNil(t, p.Team)
			assert.Equal(t, "teamB", *p.Team)
		case 200:
			require.NotNil(t, p.Seat)
			assert.Equal(t, 0, *p.Seat)
			require.NotNil(t, p.Team)
			assert.Equal(t, "teamA", *p.Team)
		}
	}

	// Two ordered seat_updated broadcasts to room members, plus a single
	// lobby-wide room_updated snapshot so non-participant lobby viewers
	// see the seat changes on the grid.
	require.Len(t, broadcaster.calls, 2, "expected exactly two BroadcastToUsers calls")
	require.Len(t, broadcaster.allCalls, 1, "swap should emit one lobby-wide room_updated snapshot")
	assert.Equal(t, "system:room_updated", msgTypeOf(t, broadcaster.allCalls[0].msg))

	for _, call := range broadcaster.calls {
		assert.Equal(t, "system:seat_updated", msgTypeOf(t, call.msg))
		// Each broadcast goes to all 4 room members
		assert.ElementsMatch(t, []uint{100, 200, 300, 400}, call.userIDs)
	}

	// First broadcast describes user-100's move (seat 0 → seat 1, prev 0)
	var msgA map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(broadcaster.calls[0].msg, &msgA))
	var pA map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(msgA["payload"], &pA))
	var userIdA float64
	require.NoError(t, json.Unmarshal(pA["userId"], &userIdA))
	assert.Equal(t, float64(100), userIdA)
	var seatA float64
	require.NoError(t, json.Unmarshal(pA["seat"], &seatA))
	assert.Equal(t, float64(1), seatA)
	var prevSeatA float64
	require.NoError(t, json.Unmarshal(pA["previousSeat"], &prevSeatA))
	assert.Equal(t, float64(0), prevSeatA)

	// Second broadcast describes user-200's move (seat 1 → seat 0, prev 1)
	var msgB map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(broadcaster.calls[1].msg, &msgB))
	var pB map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(msgB["payload"], &pB))
	var userIdB float64
	require.NoError(t, json.Unmarshal(pB["userId"], &userIdB))
	assert.Equal(t, float64(200), userIdB)
	var seatBVal float64
	require.NoError(t, json.Unmarshal(pB["seat"], &seatBVal))
	assert.Equal(t, float64(0), seatBVal)
	var prevSeatB float64
	require.NoError(t, json.Unmarshal(pB["previousSeat"], &prevSeatB))
	assert.Equal(t, float64(1), prevSeatB)
}

func TestSwapSeats_CrossTeamRecomputesTeam(t *testing.T) {
	e, repo, _ := setupTestWithBroadcast()
	seedSeatedRoom(t, repo)

	token := validToken(100)
	// seat 0 (team A) ↔ seat 3 (team B)
	rec := doSwapSeats(e, "1", `{"seatA": 0, "seatB": 3}`, token)
	require.Equal(t, http.StatusOK, rec.Code)

	players, _ := repo.FindPlayersByRoomID(1)
	for _, p := range players {
		switch p.UserID {
		case 100:
			require.NotNil(t, p.Seat)
			assert.Equal(t, 3, *p.Seat)
			assert.Equal(t, "teamB", *p.Team)
		case 400:
			require.NotNil(t, p.Seat)
			assert.Equal(t, 0, *p.Seat)
			assert.Equal(t, "teamA", *p.Team)
		}
	}
}

func TestSwapSeats_NotOwner(t *testing.T) {
	e, repo := setupTest()
	seedSeatedRoom(t, repo)

	token := validToken(200) // not the owner
	rec := doSwapSeats(e, "1", `{"seatA": 0, "seatB": 1}`, token)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "NOT_ROOM_OWNER", errResp["error"]["code"])
}

func TestSwapSeats_RoomNotWaiting(t *testing.T) {
	e, repo := setupTest()
	r := seedSeatedRoom(t, repo)
	r.Status = "playing"

	token := validToken(100)
	rec := doSwapSeats(e, "1", `{"seatA": 0, "seatB": 1}`, token)
	assert.Equal(t, http.StatusConflict, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "ROOM_NOT_WAITING", errResp["error"]["code"])
}

func TestSwapSeats_InvalidSeat(t *testing.T) {
	e, repo := setupTest()
	seedSeatedRoom(t, repo)

	token := validToken(100)

	cases := []struct {
		name string
		body string
	}{
		{"missing seatA", `{"seatB": 1}`},
		{"missing seatB", `{"seatA": 0}`},
		{"missing both", `{}`},
		{"equal seats", `{"seatA": 0, "seatB": 0}`},
		{"out of range high", `{"seatA": 4, "seatB": 1}`},
		{"out of range negative", `{"seatA": -1, "seatB": 1}`},
		{"both out of range", `{"seatA": 99, "seatB": 100}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doSwapSeats(e, "1", tc.body, token)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			var errResp map[string]map[string]string
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
			assert.Equal(t, "INVALID_SEAT", errResp["error"]["code"])
		})
	}
}

func TestSwapSeats_MoveToEmptySeat(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()

	r := &room.Room{Name: "Move Empty", OwnerID: 100, Status: "waiting", PlayerCount: 2}
	require.NoError(t, repo.Create(r))
	seat0 := 0
	seat1 := 1
	teamA := "teamA"
	teamB := "teamB"
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 100, Username: "Owner", Seat: &seat0, Team: &teamA}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 200, Username: "P2", Seat: &seat1, Team: &teamB}))

	token := validToken(100)
	// seatA=2 is empty, seatB=1 is P2 → P2 moves to seat 2 (team A)
	rec := doSwapSeats(e, "1", `{"seatA": 2, "seatB": 1}`, token)
	require.Equal(t, http.StatusOK, rec.Code)

	players, _ := repo.FindPlayersByRoomID(r.ID)
	for _, p := range players {
		switch p.UserID {
		case 100:
			require.NotNil(t, p.Seat)
			assert.Equal(t, 0, *p.Seat)
			assert.Equal(t, "teamA", *p.Team)
		case 200:
			require.NotNil(t, p.Seat)
			assert.Equal(t, 2, *p.Seat)
			require.NotNil(t, p.Team)
			assert.Equal(t, "teamA", *p.Team)
		}
	}

	// A move-to-empty emits exactly ONE seat_updated broadcast to room
	// members, plus one lobby-wide room_updated snapshot so non-participant
	// viewers see the seat change on the grid.
	require.Len(t, broadcaster.calls, 1, "expected exactly one BroadcastToUsers call for move-to-empty")
	require.Len(t, broadcaster.allCalls, 1, "move-to-empty should emit one lobby-wide room_updated snapshot")
	assert.Equal(t, "system:room_updated", msgTypeOf(t, broadcaster.allCalls[0].msg))
	assert.Equal(t, "system:seat_updated", msgTypeOf(t, broadcaster.calls[0].msg))
	assert.ElementsMatch(t, []uint{100, 200}, broadcaster.calls[0].userIDs)

	var msg map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(broadcaster.calls[0].msg, &msg))
	var p map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(msg["payload"], &p))
	var userID, seat, prevSeat float64
	require.NoError(t, json.Unmarshal(p["userId"], &userID))
	require.NoError(t, json.Unmarshal(p["seat"], &seat))
	require.NoError(t, json.Unmarshal(p["previousSeat"], &prevSeat))
	assert.Equal(t, float64(200), userID)
	assert.Equal(t, float64(2), seat)
	assert.Equal(t, float64(1), prevSeat)
}

func TestSwapSeats_BothSeatsEmpty(t *testing.T) {
	e, repo := setupTest()

	r := &room.Room{Name: "Swap Empty", OwnerID: 100, Status: "waiting", PlayerCount: 2}
	require.NoError(t, repo.Create(r))
	seat0 := 0
	teamA := "teamA"
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 100, Username: "Owner", Seat: &seat0, Team: &teamA}))
	// Second player not yet seated
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 200, Username: "P2"}))

	token := validToken(100)
	// Both seat 1 and seat 2 are empty → SEAT_NOT_OCCUPIED.
	rec := doSwapSeats(e, "1", `{"seatA": 1, "seatB": 2}`, token)
	assert.Equal(t, http.StatusConflict, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "SEAT_NOT_OCCUPIED", errResp["error"]["code"])
}

func TestSwapSeats_Unauthorized(t *testing.T) {
	e, _ := setupTest()
	rec := doSwapSeats(e, "1", `{"seatA": 0, "seatB": 1}`, "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- LeaveSeat Tests ---

func doLeaveSeat(e *echo.Echo, id string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/"+id+"/leave-seat", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// seedRoomWithSeatedNonOwner seeds a non-quick-play waiting room with the
// owner at seat 0 and a non-owner at seat 1. Used by the LeaveSeat tests
// since the owner is forbidden from unseating themselves.
func seedRoomWithSeatedNonOwner(t *testing.T, repo *mockRoomRepo) *room.Room {
	t.Helper()
	r := &room.Room{Name: "Leave Seat", OwnerID: 100, Status: "waiting", PlayerCount: 2}
	require.NoError(t, repo.Create(r))
	seat0, seat1 := 0, 1
	teamA, teamB := "teamA", "teamB"
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 100, Username: "Owner", Seat: &seat0, Team: &teamA}))
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 200, Username: "P2", Seat: &seat1, Team: &teamB}))
	return r
}

func TestLeaveSeat_Success(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()
	r := seedRoomWithSeatedNonOwner(t, repo)

	token := validToken(200)
	rec := doLeaveSeat(e, "1", token)
	require.Equal(t, http.StatusOK, rec.Code)

	// Player 200 should now be unseated; owner unchanged.
	players, _ := repo.FindPlayersByRoomID(r.ID)
	for _, p := range players {
		switch p.UserID {
		case 100:
			require.NotNil(t, p.Seat)
			assert.Equal(t, 0, *p.Seat)
		case 200:
			assert.Nil(t, p.Seat)
			assert.Nil(t, p.Team)
		}
	}

	// Exactly one seat_updated broadcast with seat=null/team=null/previousSeat=1.
	require.Len(t, broadcaster.calls, 1)
	assert.Equal(t, "system:seat_updated", msgTypeOf(t, broadcaster.calls[0].msg))
	assert.ElementsMatch(t, []uint{100, 200}, broadcaster.calls[0].userIDs)

	var msg map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(broadcaster.calls[0].msg, &msg))
	var p map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(msg["payload"], &p))
	assert.Equal(t, "null", string(p["seat"]))
	assert.Equal(t, "null", string(p["team"]))
	var prevSeat float64
	require.NoError(t, json.Unmarshal(p["previousSeat"], &prevSeat))
	assert.Equal(t, float64(1), prevSeat)
	var userID float64
	require.NoError(t, json.Unmarshal(p["userId"], &userID))
	assert.Equal(t, float64(200), userID)
}

func TestLeaveSeat_OwnerForbidden(t *testing.T) {
	e, repo := setupTest()
	seedRoomWithSeatedNonOwner(t, repo)

	token := validToken(100) // owner
	rec := doLeaveSeat(e, "1", token)
	assert.Equal(t, http.StatusConflict, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "OWNER_CANNOT_LEAVE_SEAT", errResp["error"]["code"])
}

func TestLeaveSeat_QuickPlayBlocked(t *testing.T) {
	e, repo := setupTest()
	r := seedRoomWithSeatedNonOwner(t, repo)
	r.IsQuickPlay = true

	token := validToken(200)
	rec := doLeaveSeat(e, "1", token)
	assert.Equal(t, http.StatusConflict, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "QUICK_PLAY_LEAVE_SEAT_BLOCKED", errResp["error"]["code"])
}

func TestLeaveSeat_RoomNotWaiting(t *testing.T) {
	e, repo := setupTest()
	r := seedRoomWithSeatedNonOwner(t, repo)
	r.Status = "playing"

	token := validToken(200)
	rec := doLeaveSeat(e, "1", token)
	assert.Equal(t, http.StatusConflict, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "ROOM_NOT_WAITING", errResp["error"]["code"])
}

func TestLeaveSeat_NotInRoom(t *testing.T) {
	e, repo := setupTest()
	seedRoomWithSeatedNonOwner(t, repo)

	// User 999 is not a member of room 1.
	token := validToken(999)
	rec := doLeaveSeat(e, "1", token)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "NOT_IN_ROOM", errResp["error"]["code"])
}

func TestLeaveSeat_NotSeated(t *testing.T) {
	e, repo := setupTest()
	r := &room.Room{Name: "Unseated", OwnerID: 100, Status: "waiting", PlayerCount: 2}
	require.NoError(t, repo.Create(r))
	seat0 := 0
	teamA := "teamA"
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 100, Username: "Owner", Seat: &seat0, Team: &teamA}))
	// Player 200 is in the room but has no seat.
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 200, Username: "P2"}))

	token := validToken(200)
	rec := doLeaveSeat(e, "1", token)
	assert.Equal(t, http.StatusConflict, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "NOT_SEATED", errResp["error"]["code"])
}

func TestLeaveSeat_Unauthorized(t *testing.T) {
	e, _ := setupTest()
	rec := doLeaveSeat(e, "1", "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- TransferOwnership Tests ---

func doTransferOwnership(e *echo.Echo, id string, body string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/"+id+"/transfer-ownership", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestTransferOwnership_Success(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()
	r := seedSeatedRoom(t, repo) // owner=100 seat 0, P2=200 seat 1, P3=300 seat 2, P4=400 seat 3

	token := validToken(100)
	rec := doTransferOwnership(e, "1", `{"userId": 200}`, token)
	require.Equal(t, http.StatusOK, rec.Code)

	// Owner has flipped to user 200.
	updated, _ := repo.FindByID(r.ID)
	require.NotNil(t, updated)
	assert.Equal(t, uint(200), updated.OwnerID)

	// system:room_owner_changed broadcast to all 4 members.
	require.Len(t, broadcaster.calls, 1, "expected one room broadcast")
	assert.Equal(t, "system:room_owner_changed", msgTypeOf(t, broadcaster.calls[0].msg))
	assert.ElementsMatch(t, []uint{100, 200, 300, 400}, broadcaster.calls[0].userIDs)

	var msg map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(broadcaster.calls[0].msg, &msg))
	var p map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(msg["payload"], &p))
	var newOwnerID, prevOwnerID float64
	require.NoError(t, json.Unmarshal(p["newOwnerId"], &newOwnerID))
	require.NoError(t, json.Unmarshal(p["previousOwnerId"], &prevOwnerID))
	assert.Equal(t, float64(200), newOwnerID)
	assert.Equal(t, float64(100), prevOwnerID)

	// And one lobby-wide system:room_updated broadcast.
	require.Len(t, broadcaster.allCalls, 1)
}

func TestTransferOwnership_NotOwner(t *testing.T) {
	e, repo := setupTest()
	seedSeatedRoom(t, repo)

	token := validToken(200) // not the current owner
	rec := doTransferOwnership(e, "1", `{"userId": 300}`, token)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "NOT_ROOM_OWNER", errResp["error"]["code"])
}

func TestTransferOwnership_RoomNotWaiting(t *testing.T) {
	e, repo := setupTest()
	r := seedSeatedRoom(t, repo)
	r.Status = "playing"

	token := validToken(100)
	rec := doTransferOwnership(e, "1", `{"userId": 200}`, token)
	assert.Equal(t, http.StatusConflict, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "ROOM_NOT_WAITING", errResp["error"]["code"])
}

func TestTransferOwnership_NotInRoom(t *testing.T) {
	e, repo := setupTest()
	seedSeatedRoom(t, repo)

	token := validToken(100)
	rec := doTransferOwnership(e, "1", `{"userId": 999}`, token)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "NOT_IN_ROOM", errResp["error"]["code"])
}

func TestTransferOwnership_CannotPromoteUnseated(t *testing.T) {
	e, repo := setupTest()

	r := &room.Room{Name: "Promote Unseated", OwnerID: 100, Status: "waiting", PlayerCount: 2}
	require.NoError(t, repo.Create(r))
	seat0 := 0
	teamA := "teamA"
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 100, Username: "Owner", Seat: &seat0, Team: &teamA}))
	// Player 200 in room but unseated — cannot be promoted.
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 200, Username: "P2"}))

	token := validToken(100)
	rec := doTransferOwnership(e, "1", `{"userId": 200}`, token)
	assert.Equal(t, http.StatusConflict, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "CANNOT_PROMOTE_UNSEATED", errResp["error"]["code"])

	// Owner unchanged.
	updated, _ := repo.FindByID(r.ID)
	assert.Equal(t, uint(100), updated.OwnerID)
}

func TestTransferOwnership_CannotTransferToSelf(t *testing.T) {
	e, repo := setupTest()
	seedSeatedRoom(t, repo)

	token := validToken(100)
	rec := doTransferOwnership(e, "1", `{"userId": 100}`, token)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "CANNOT_TRANSFER_TO_SELF", errResp["error"]["code"])
}

func TestTransferOwnership_Unauthorized(t *testing.T) {
	e, _ := setupTest()
	rec := doTransferOwnership(e, "1", `{"userId": 200}`, "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
