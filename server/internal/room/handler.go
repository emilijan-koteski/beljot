package room

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/labstack/echo/v4"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/auth"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/user"
	"github.com/emilijan/beljot/server/internal/ws"
)

var (
	validVariants    = map[string]bool{"bitola": true, "croatia": true}
	validMatchModes  = map[string]bool{"1001": true, "701": true, "501": true}
	validTimerStyles = map[string]bool{"relaxed": true, "per-move": true}
	validStatuses    = map[string]bool{"waiting": true, "playing": true, "completed": true}
)

const (
	codeChars                     = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	codeLength                    = 6
	maxRetries                    = 3
	quickPlayStandardBuyIn        = 500
	quickPlayTimerDurationSeconds = 30
	roomPasswordMinLength         = 4
	roomPasswordMaxBytes          = 72
)

// validateRoomPassword enforces the private-room password rules, counting the minimum in
// runes and capping at bcrypt's 72-byte input limit.
func validateRoomPassword(pw string) error {
	if pw == "" {
		return apperr.ErrRoomPasswordRequired
	}
	if utf8.RuneCountInString(pw) < roomPasswordMinLength {
		return apperr.ErrRoomPasswordTooShort
	}
	if len(pw) > roomPasswordMaxBytes {
		return apperr.ErrRoomPasswordTooLong
	}
	return nil
}

// honorGated reports whether this room enforces an honor requirement at all, so ungated
// rooms (the common case) skip every honor read.
func (r *Room) honorGated() bool {
	return r.MinHonor > 0 || !r.AllowNewPlayers
}

// honorGateError returns nil to admit a player or the apperr to reject with.
// A New Player is admitted only by AllowNewPlayers and is never score-checked;
// anyone else needs at least MinHonor.
func honorGateError(r *Room, snap user.HonorSnapshot) error {
	if snap.IsNewPlayer {
		if r.AllowNewPlayers {
			return nil
		}
		return apperr.ErrNewPlayerNotAllowed
	}
	if snap.Score < r.MinHonor {
		return apperr.ErrHonorTooLow
	}
	return nil
}

// quickPlayBuyIn maps a wallet balance to the Quick Play bracket buy-in, which is both
// the matchmaking pool key and the per-human stake.
func quickPlayBuyIn(balance int) int {
	if balance >= quickPlayStandardBuyIn {
		return quickPlayStandardBuyIn
	}
	return 0
}

type CreateRoomRequest struct {
	Name                 string `json:"name"`
	Variant              string `json:"variant"`
	MatchMode            string `json:"matchMode"`
	TimerStyle           string `json:"timerStyle"`
	TimerDurationSeconds *int   `json:"timerDurationSeconds"`
	ReconnectWindowSec   *int   `json:"reconnectWindowSec"`
	CoinBuyIn            *int   `json:"coinBuyIn"`
	IsPrivate            bool   `json:"isPrivate"`
	Password             string `json:"password"`
	MinHonor             *int   `json:"minHonor"`
	AllowNewPlayers      *bool  `json:"allowNewPlayers"`
	DeclarationsEnabled  *bool  `json:"declarationsEnabled"`
	StopAtTarget         *bool  `json:"stopAtTarget"`
}

// MatchStarter is the interface the room handler uses to start a live match.
type MatchStarter interface {
	StartMatch(roomID uint, variant string, matchMode string, players [4]match.PlayerSeatInfo, timerStyle string, timerDurationSec int, ownerID uint, reconnectWindowSec int, coinBuyIn int, declarationsEnabled bool, stopAtTarget bool) error
}

// WalletService is the subset of *wallet.Service the room handler needs. A nil
// WalletService disables economy enforcement.
type WalletService interface {
	GetBalance(userID uint) (int, error)
	GetBalances(userIDs []uint) (map[uint]int, error)
	ChargeStakes(userIDs []uint, amount int) (insolventUserID uint, err error)
	ApplySettlement(credits map[uint]int) error
}

// HonorService is the subset of *user.HonorService the room handler needs for the honor
// gate. A nil HonorService disables honor enforcement; user must never import room.
type HonorService interface {
	HonorForUsers(userIDs []uint) (map[uint]user.HonorSnapshot, error)
}

// RoomStatusAdapter implements match.RoomStatusUpdater using the room repository.
type RoomStatusAdapter struct {
	Repo RoomRepository
}

// UpdateRoomStatus updates a room's status in the database.
func (a *RoomStatusAdapter) UpdateRoomStatus(roomID uint, status string) error {
	return a.Repo.UpdateStatus(roomID, status)
}

// Broadcaster abstracts WebSocket broadcast capabilities for testability.
type Broadcaster interface {
	BroadcastToUsers(userIDs []uint, msg []byte)
	BroadcastAll(msg []byte)
}

type RoomHandler struct {
	repo          RoomRepository
	matchStarter  MatchStarter
	hub           Broadcaster
	presence      *PresenceRegistry
	walletService WalletService
	honorService  HonorService
	invites       *InviteRegistry
	quickFill     *quickFillScheduler
}

func NewRoomHandler(repo RoomRepository, matchStarter MatchStarter, hub Broadcaster, presence *PresenceRegistry, walletService WalletService, honorService HonorService, invites *InviteRegistry) *RoomHandler {
	if presence == nil {
		presence = NewPresenceRegistry()
	}
	if invites == nil {
		invites = NewInviteRegistry()
	}
	return &RoomHandler{repo: repo, matchStarter: matchStarter, hub: hub, presence: presence, walletService: walletService, honorService: honorService, invites: invites, quickFill: newQuickFillScheduler(defaultQuickFillFastInterval, defaultQuickFillPatientInterval)}
}

// broadcastToRoom sends a best-effort WebSocket message to all players in a room.
func (h *RoomHandler) broadcastToRoom(roomID uint, msgType string, payload interface{}) {
	if h.hub == nil {
		return
	}
	players, err := h.repo.FindPlayersByRoomID(roomID)
	if err != nil {
		slog.Error("broadcast: failed to find room players", "roomID", roomID, "error", err)
		return
	}
	userIDs := make([]uint, 0, len(players))
	for _, p := range players {
		userIDs = append(userIDs, p.UserID)
	}
	h.broadcastToUsers(userIDs, msgType, payload)
}

// broadcastToUsers sends a WebSocket message to a specific set of users.
func (h *RoomHandler) broadcastToUsers(userIDs []uint, msgType string, payload interface{}) {
	if h.hub == nil {
		return
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("broadcast: failed to marshal payload", "type", msgType, "error", err)
		return
	}
	msg, err := json.Marshal(ws.WSMessage{Type: msgType, Payload: payloadBytes})
	if err != nil {
		slog.Error("broadcast: failed to marshal message", "type", msgType, "error", err)
		return
	}
	h.hub.BroadcastToUsers(userIDs, msg)
}

// broadcastToAll sends a WebSocket message to all connected clients (lobby-wide).
func (h *RoomHandler) broadcastToAll(msgType string, payload interface{}) {
	if h.hub == nil {
		return
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("broadcast: failed to marshal payload", "type", msgType, "error", err)
		return
	}
	msg, err := json.Marshal(ws.WSMessage{Type: msgType, Payload: payloadBytes})
	if err != nil {
		slog.Error("broadcast: failed to marshal message", "type", msgType, "error", err)
		return
	}
	h.hub.BroadcastAll(msg)
}

// mergeBotPlayers appends synthetic bot entries to a humans-only players slice. Every
// payload path that serializes players must route through it.
func mergeBotPlayers(players []RoomPlayer, bots []RoomBot) []RoomPlayer {
	if len(bots) == 0 {
		if players == nil {
			return []RoomPlayer{}
		}
		return players
	}
	out := make([]RoomPlayer, 0, len(players)+len(bots))
	out = append(out, players...)
	for _, b := range bots {
		seat := b.Seat
		team := teamForSeat(seat)
		out = append(out, RoomPlayer{
			RoomID:    b.RoomID,
			Seat:      &seat,
			Team:      &team,
			IsBot:     true,
			CreatedAt: b.CreatedAt,
		})
	}
	return out
}

// playersWithBots loads the room's humans and bots and returns the merged wire players
// array, logging rather than returning errors.
func (h *RoomHandler) playersWithBots(roomID uint) []RoomPlayer {
	players, err := h.repo.FindPlayersByRoomID(roomID)
	if err != nil {
		slog.Error("failed to load room players", "roomID", roomID, "error", err)
		players = []RoomPlayer{}
	}
	bots, err := h.repo.FindBotsByRoomID(roomID)
	if err != nil {
		slog.Error("failed to load room bots", "roomID", roomID, "error", err)
		bots = nil
	}
	roster := mergeBotPlayers(players, bots)
	h.attachPlayerHonor(roster)
	return roster
}

// roomLifecyclePayload builds the WS payload shared by system:room_created and
// system:room_updated, merging bots into the humans-only r.Players.
func (h *RoomHandler) roomLifecyclePayload(r *Room) map[string]any {
	if r.OwnerUsername == "" {
		if err := h.repo.LoadOwnerUsernames([]*Room{r}); err != nil {
			slog.Error("broadcast: failed to load owner username", "roomID", r.ID, "error", err)
		}
	}
	players := r.Players
	if players == nil {
		fetched, err := h.repo.FindPlayersByRoomID(r.ID)
		if err != nil {
			slog.Error("broadcast: failed to load room players", "roomID", r.ID, "error", err)
			fetched = []RoomPlayer{}
		}
		players = fetched
	}
	bots, err := h.repo.FindBotsByRoomID(r.ID)
	if err != nil {
		slog.Error("broadcast: failed to load room bots", "roomID", r.ID, "error", err)
		bots = nil
	}
	players = mergeBotPlayers(players, bots)
	return map[string]any{
		"id":                   r.ID,
		"name":                 r.Name,
		"code":                 r.Code,
		"ownerId":              r.OwnerID,
		"ownerUsername":        r.OwnerUsername,
		"ownerAvatarUrl":       r.OwnerAvatarURL,
		"players":              players,
		"variant":              r.Variant,
		"matchMode":            r.MatchMode,
		"timerStyle":           r.TimerStyle,
		"timerDurationSeconds": r.TimerDurationSeconds,
		"coinBuyIn":            r.CoinBuyIn,
		"isPrivate":            r.PasswordHash != nil,
		"minHonor":             r.MinHonor,
		"allowNewPlayers":      r.AllowNewPlayers,
		"declarationsEnabled":  r.DeclarationsEnabled,
		"stopAtTarget":         r.StopAtTarget,
		"playerCount":          r.PlayerCount,
		"status":               r.Status,
		"isQuickPlay":          r.IsQuickPlay,
		"createdAt":            r.CreatedAt.UTC().Format(time.RFC3339),
		"updatedAt":            r.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// broadcastRoomUpdated sends a system:room_updated event to all connected clients.
func (h *RoomHandler) broadcastRoomUpdated(r *Room) {
	h.broadcastToAll(ws.SystemRoomUpdated, h.roomLifecyclePayload(r))
}

// broadcastRoomSeatSnapshot pushes a system:room_updated event with fresh players to
// every connected client so lobby seat chips stay in sync.
func (h *RoomHandler) broadcastRoomSeatSnapshot(roomID uint, players []RoomPlayer) {
	r, err := h.repo.FindByID(roomID)
	if err != nil || r == nil {
		return
	}
	r.Players = players
	h.broadcastRoomUpdated(r)
}

func (h *RoomHandler) CreateRoom(c echo.Context) error {
	userID, err := auth.GetUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	existingRoom, err := h.repo.FindPlayerRoom(userID)
	if err != nil {
		return fmt.Errorf("checking existing room: %w", err)
	}
	if existingRoom != nil {
		return apperr.ErrAlreadyInRoom
	}

	var req CreateRoomRequest
	if err := c.Bind(&req); err != nil {
		return apperr.ErrBadRequest
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return apperr.ErrRoomNameRequired
	}
	if len(name) > 100 {
		return apperr.ErrRoomNameTooLong
	}

	variant := req.Variant
	if variant == "" {
		variant = "bitola"
	}
	if !validVariants[variant] {
		return apperr.ErrInvalidVariant
	}

	matchMode := req.MatchMode
	if matchMode == "" {
		matchMode = "1001"
	}
	if !validMatchModes[matchMode] {
		return apperr.ErrInvalidMatchMode
	}

	timerStyle := req.TimerStyle
	if timerStyle == "" {
		timerStyle = "relaxed"
	}
	if !validTimerStyles[timerStyle] {
		return apperr.ErrInvalidTimerStyle
	}

	var timerDuration *int
	if timerStyle == "per-move" {
		if req.TimerDurationSeconds == nil {
			return apperr.ErrTimerDurationRequired
		}
		d := *req.TimerDurationSeconds
		if d < 10 || d > 120 {
			return apperr.ErrTimerDurationOutOfRange
		}
		timerDuration = req.TimerDurationSeconds
	}

	var reconnectWindow *int
	if req.ReconnectWindowSec != nil {
		rw := *req.ReconnectWindowSec
		if rw < 30 || rw > 300 {
			return apperr.ErrReconnectWindowOutOfRange
		}
		reconnectWindow = req.ReconnectWindowSec
	}

	coinBuyIn := 500
	if req.CoinBuyIn != nil {
		coinBuyIn = *req.CoinBuyIn
		if coinBuyIn < 0 {
			return apperr.ErrBadRequest
		}
	}

	if coinBuyIn > 0 && h.walletService != nil {
		balance, balErr := h.walletService.GetBalance(userID)
		if balErr != nil {
			return fmt.Errorf("reading wallet balance: %w", balErr)
		}
		if balance < coinBuyIn {
			return apperr.ErrInsufficientCoins
		}
	}

	minHonor := 0
	if req.MinHonor != nil {
		minHonor = *req.MinHonor
		if minHonor < 0 || minHonor > 100 {
			return apperr.ErrInvalidMinHonor
		}
	}
	allowNewPlayers := true
	if req.AllowNewPlayers != nil {
		allowNewPlayers = *req.AllowNewPlayers
	}

	declarationsEnabled := true
	if req.DeclarationsEnabled != nil {
		declarationsEnabled = *req.DeclarationsEnabled
	}

	stopAtTarget := false
	if req.StopAtTarget != nil {
		stopAtTarget = *req.StopAtTarget
	}

	gateProbe := &Room{MinHonor: minHonor, AllowNewPlayers: allowNewPlayers}
	if gateProbe.honorGated() {
		snap, enforced, herr := h.honorSnapshotFor(userID)
		if herr != nil {
			return herr
		}
		if enforced {
			if gateErr := honorGateError(gateProbe, snap); gateErr != nil {
				return gateErr
			}
			if snap.IsNewPlayer && snap.Score < minHonor {
				return apperr.ErrHonorTooLow
			}
		}
	}

	var passwordHash *string
	if req.IsPrivate {
		if err := validateRoomPassword(req.Password); err != nil {
			return err
		}
		hashed, hashErr := auth.HashPassword(req.Password)
		if hashErr != nil {
			return fmt.Errorf("hashing room password: %w", hashErr)
		}
		passwordHash = &hashed
	}

	code, err := generateRoomCode()
	if err != nil {
		return fmt.Errorf("generating room code: %w", err)
	}

	room := &Room{
		Name:                 name,
		Code:                 code,
		OwnerID:              userID,
		Variant:              variant,
		MatchMode:            matchMode,
		TimerStyle:           timerStyle,
		TimerDurationSeconds: timerDuration,
		ReconnectWindowSec:   reconnectWindow,
		CoinBuyIn:            coinBuyIn,
		PasswordHash:         passwordHash,
		MinHonor:             minHonor,
		AllowNewPlayers:      allowNewPlayers,
		DeclarationsEnabled:  declarationsEnabled,
		StopAtTarget:         stopAtTarget,
		Status:               "waiting",
		PlayerCount:          1,
	}

	var createErr error
	for i := 0; i < maxRetries; i++ {
		createErr = h.repo.RunInTransaction(func(tx RoomRepository) error {
			if err := tx.Create(room); err != nil {
				return err
			}
			rp := &RoomPlayer{RoomID: room.ID, UserID: userID}
			if err := tx.AddPlayer(rp); err != nil {
				return fmt.Errorf("adding creator to room players: %w", err)
			}
			ownerSeat := 0
			if err := tx.UpdatePlayerSeat(room.ID, userID, ownerSeat, teamForSeat(ownerSeat)); err != nil {
				return fmt.Errorf("auto-seating creator: %w", err)
			}
			return nil
		})
		if createErr == nil {
			break
		}
		if errors.Is(createErr, apperr.ErrRoomCodeTaken) {
			newCode, codeErr := generateRoomCode()
			if codeErr != nil {
				return fmt.Errorf("generating room code: %w", codeErr)
			}
			room.Code = newCode
			continue
		}
		return createErr
	}
	if createErr != nil {
		return createErr
	}

	h.presence.Add(room.ID, userID)

	room.IsPrivate = room.PasswordHash != nil

	h.broadcastToAll(ws.SystemRoomCreated, h.roomLifecyclePayload(room))

	return c.JSON(http.StatusCreated, map[string]interface{}{"data": room})
}

func (h *RoomHandler) ListRooms(c echo.Context) error {
	status := c.QueryParam("status")
	if status == "" {
		status = "waiting"
	}

	if !validStatuses[status] {
		return apperr.ErrInvalidRoomStatus
	}

	rooms, err := h.repo.FindByStatus(status)
	if err != nil {
		return fmt.Errorf("listing rooms: %w", err)
	}

	roomPtrs := make([]*Room, len(rooms))
	roomIDs := make([]uint, len(rooms))
	for i := range rooms {
		roomPtrs[i] = &rooms[i]
		roomIDs[i] = rooms[i].ID
	}
	if err := h.repo.LoadOwnerUsernames(roomPtrs); err != nil {
		slog.Error("list rooms: failed to load owner usernames", "error", err)
	}
	if playersByRoom, perr := h.repo.FindPlayersByRoomIDs(roomIDs); perr != nil {
		slog.Error("list rooms: failed to load players", "error", perr)
	} else {
		botsByRoom, berr := h.repo.FindBotsByRoomIDs(roomIDs)
		if berr != nil {
			slog.Error("list rooms: failed to load bots", "error", berr)
			botsByRoom = map[uint][]RoomBot{}
		}
		for i := range rooms {
			rooms[i].Players = mergeBotPlayers(playersByRoom[rooms[i].ID], botsByRoom[rooms[i].ID])
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"data": rooms})
}

type RoomDetailResponse struct {
	Room            *Room        `json:"room"`
	Players         []RoomPlayer `json:"players"`
	ReturnedUserIds []uint       `json:"returnedUserIds"`
}

func (h *RoomHandler) GetRoom(c echo.Context) error {
	roomID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.ErrRoomNotFound
	}

	room, err := h.repo.FindByID(uint(roomID))
	if err != nil {
		return fmt.Errorf("finding room: %w", err)
	}
	if room == nil {
		return apperr.ErrRoomNotFound
	}

	players, err := h.repo.FindPlayersByRoomID(uint(roomID))
	if err != nil {
		return fmt.Errorf("finding room players: %w", err)
	}
	bots, err := h.repo.FindBotsByRoomID(uint(roomID))
	if err != nil {
		return fmt.Errorf("finding room bots: %w", err)
	}

	if err := h.repo.LoadOwnerUsernames([]*Room{room}); err != nil {
		slog.Error("get room: failed to load owner username", "roomID", room.ID, "error", err)
	}

	roster := mergeBotPlayers(players, bots)
	h.attachPlayerHonor(roster)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": RoomDetailResponse{
			Room:            room,
			Players:         roster,
			ReturnedUserIds: h.presence.Present(uint(roomID)),
		},
	})
}

func (h *RoomHandler) GetRoomByCode(c echo.Context) error {
	code := strings.ToUpper(strings.TrimSpace(c.Param("code")))
	if len(code) != codeLength {
		return apperr.ErrRoomNotFound
	}

	room, err := h.repo.FindByCode(code)
	if err != nil {
		return fmt.Errorf("finding room by code: %w", err)
	}
	if room == nil || room.Status != "waiting" {
		return apperr.ErrRoomNotFound
	}

	players, err := h.repo.FindPlayersByRoomID(room.ID)
	if err != nil {
		return fmt.Errorf("finding room players: %w", err)
	}
	bots, err := h.repo.FindBotsByRoomID(room.ID)
	if err != nil {
		return fmt.Errorf("finding room bots: %w", err)
	}

	if err := h.repo.LoadOwnerUsernames([]*Room{room}); err != nil {
		slog.Error("get room by code: failed to load owner username", "roomID", room.ID, "error", err)
	}

	roster := mergeBotPlayers(players, bots)
	h.attachPlayerHonor(roster)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": RoomDetailResponse{
			Room:            room,
			Players:         roster,
			ReturnedUserIds: h.presence.Present(room.ID),
		},
	})
}

// JoinRoomRequest carries the optional private-room password; public-room joins send no
// body.
type JoinRoomRequest struct {
	Password string `json:"password"`
}

// voidInvitesIfFull drops a room's outstanding friend invites once all four seats are
// covered. It is best-effort and never fails the request that filled the room.
func (h *RoomHandler) voidInvitesIfFull(roomID uint) {
	r, err := h.repo.FindByID(roomID)
	if err != nil || r == nil {
		return
	}
	bots, err := h.repo.FindBotsByRoomID(roomID)
	if err != nil {
		return
	}
	if r.PlayerCount+len(bots) >= 4 {
		h.invites.VoidRoom(roomID)
	}
}

func (h *RoomHandler) JoinRoom(c echo.Context) error {
	userID, err := auth.GetUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	roomID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.ErrRoomNotFound
	}

	var req JoinRoomRequest
	_ = c.Bind(&req)

	room, err := h.repo.FindByID(uint(roomID))
	if err != nil {
		return fmt.Errorf("finding room: %w", err)
	}
	if room == nil {
		return apperr.ErrRoomNotFound
	}

	if room.Status != "waiting" {
		return apperr.ErrRoomNotFound
	}

	if room.PasswordHash != nil {
		if !h.invites.HasHostGrant(uint(roomID), userID) {
			if auth.CheckPassword(*room.PasswordHash, req.Password) != nil {
				return apperr.ErrWrongRoomPassword
			}
		}
	}

	if room.PlayerCount >= 4 {
		return apperr.ErrRoomFull
	}

	bots, err := h.repo.FindBotsByRoomID(uint(roomID))
	if err != nil {
		return fmt.Errorf("counting room bots: %w", err)
	}
	if room.PlayerCount+len(bots) >= 4 {
		return apperr.ErrRoomFull
	}

	existingRoom, err := h.repo.FindPlayerRoom(userID)
	if err != nil {
		return fmt.Errorf("checking existing room: %w", err)
	}
	if existingRoom != nil {
		return apperr.ErrAlreadyInRoom
	}

	if room.CoinBuyIn > 0 && h.walletService != nil {
		balance, balErr := h.walletService.GetBalance(userID)
		if balErr != nil {
			return fmt.Errorf("reading wallet balance: %w", balErr)
		}
		if balance < room.CoinBuyIn {
			return apperr.ErrInsufficientCoins
		}
	}

	if room.honorGated() {
		snap, enforced, herr := h.honorSnapshotFor(userID)
		if herr != nil {
			return herr
		}
		if enforced {
			if gateErr := honorGateError(room, snap); gateErr != nil {
				return gateErr
			}
		}
	}

	var updatedRoom *Room
	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		rp := &RoomPlayer{RoomID: uint(roomID), UserID: userID}
		if err := tx.AddPlayer(rp); err != nil {
			return err
		}
		if err := tx.IncrementPlayerCount(uint(roomID)); err != nil {
			return fmt.Errorf("incrementing player count: %w", err)
		}
		r, err := tx.FindByID(uint(roomID))
		if err != nil {
			return fmt.Errorf("re-fetching room after join: %w", err)
		}
		updatedRoom = r
		return nil
	}); err != nil {
		if errors.Is(err, apperr.ErrAlreadyInRoom) || errors.Is(err, apperr.ErrRoomNotFound) {
			return err
		}
		return fmt.Errorf("joining room: %w", err)
	}

	h.presence.Add(uint(roomID), userID)

	h.invites.VoidUser(userID)
	h.voidInvitesIfFull(uint(roomID))

	players, broadcastErr := h.repo.FindPlayersByRoomID(uint(roomID))
	if broadcastErr == nil {
		h.attachPlayerHonor(players)
		var joiner *RoomPlayer
		for i := range players {
			if players[i].UserID == userID {
				joiner = &players[i]
				break
			}
		}
		userIDs := make([]uint, 0, len(players))
		for _, p := range players {
			userIDs = append(userIDs, p.UserID)
		}
		payload := map[string]interface{}{
			"roomId":      roomID,
			"userId":      userID,
			"playerCount": updatedRoom.PlayerCount,
		}
		if joiner != nil {
			payload["username"] = joiner.Username
			payload["avatarUrl"] = joiner.AvatarURL
			if joiner.HonorScore != nil {
				payload["honorScore"] = *joiner.HonorScore
			}
			if joiner.HonorTier != nil {
				payload["honorTier"] = *joiner.HonorTier
			}
			if joiner.Level != nil {
				payload["level"] = *joiner.Level
			}
		} else {
			payload["username"] = ""
			payload["avatarUrl"] = nil
		}
		h.broadcastToUsers(userIDs, ws.SystemPlayerJoined, payload)
	}

	h.broadcastRoomUpdated(updatedRoom)

	return c.JSON(http.StatusOK, map[string]interface{}{"data": updatedRoom})
}

// UpdateRoomPrivacyRequest is the body of POST /rooms/:id/privacy.
type UpdateRoomPrivacyRequest struct {
	IsPrivate bool   `json:"isPrivate"`
	Password  string `json:"password"`
}

// UpdateRoomPrivacy lets a waiting room's owner set, change, or clear the password.
// Seated players are not re-checked, since the privacy gate applies only at join time.
func (h *RoomHandler) UpdateRoomPrivacy(c echo.Context) error {
	userID, err := auth.GetUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	roomID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.ErrRoomNotFound
	}

	room, err := h.repo.FindByID(uint(roomID))
	if err != nil {
		return fmt.Errorf("finding room: %w", err)
	}
	if room == nil {
		return apperr.ErrRoomNotFound
	}
	if room.OwnerID != userID {
		return apperr.ErrNotRoomOwner
	}
	if room.Status != "waiting" {
		return apperr.ErrRoomNotWaiting
	}
	if room.IsQuickPlay {
		return apperr.ErrQuickPlayRoomPrivacy
	}

	var req UpdateRoomPrivacyRequest
	if err := c.Bind(&req); err != nil {
		return apperr.ErrBadRequest
	}

	if req.IsPrivate {
		if err := validateRoomPassword(req.Password); err != nil {
			return err
		}
		hashed, hashErr := auth.HashPassword(req.Password)
		if hashErr != nil {
			return fmt.Errorf("hashing room password: %w", hashErr)
		}
		room.PasswordHash = &hashed
	} else {
		room.PasswordHash = nil
	}

	if err := h.repo.Update(room); err != nil {
		return fmt.Errorf("updating room privacy: %w", err)
	}

	room.IsPrivate = room.PasswordHash != nil
	h.broadcastRoomUpdated(room)

	return c.JSON(http.StatusOK, map[string]interface{}{"data": room})
}

// honorSnapshotsFor reads the authoritative honor snapshots for the given users, with
// enforced false when no honor service is wired. A read error or a missing row is
// returned as an error, never an admit.
func (h *RoomHandler) honorSnapshotsFor(userIDs []uint) (snaps map[uint]user.HonorSnapshot, enforced bool, err error) {
	if h.honorService == nil {
		return nil, false, nil
	}
	wanted := make([]uint, 0, len(userIDs))
	for _, id := range userIDs {
		if id != 0 {
			wanted = append(wanted, id)
		}
	}
	if len(wanted) == 0 {
		return map[uint]user.HonorSnapshot{}, true, nil
	}
	read, rerr := h.honorService.HonorForUsers(wanted)
	if rerr != nil {
		return nil, true, fmt.Errorf("reading honor for room gate: %w", rerr)
	}
	for _, id := range wanted {
		if _, ok := read[id]; !ok {
			return nil, true, fmt.Errorf("reading honor for room gate: no honor row for user %d", id)
		}
	}
	return read, true, nil
}

// attachPlayerHonor hydrates HonorScore, HonorTier, and Level on each human seat in
// place. Unlike the gate's reader it is lenient: failures just leave a seat without honor
// data.
func (h *RoomHandler) attachPlayerHonor(players []RoomPlayer) {
	if h.honorService == nil || len(players) == 0 {
		return
	}
	ids := make([]uint, 0, len(players))
	for _, p := range players {
		if p.UserID != 0 && !p.IsBot {
			ids = append(ids, p.UserID)
		}
	}
	if len(ids) == 0 {
		return
	}
	snaps, err := h.honorService.HonorForUsers(ids)
	if err != nil {
		slog.Error("failed to read honor for room roster", "error", err)
		return
	}
	for i := range players {
		snap, ok := snaps[players[i].UserID]
		if !ok {
			continue
		}
		score := snap.Score
		tier := string(snap.Tier)
		level := snap.Level
		players[i].HonorScore = &score
		players[i].HonorTier = &tier
		players[i].Level = &level
	}
}

// honorSnapshotFor is the single-user form of honorSnapshotsFor.
func (h *RoomHandler) honorSnapshotFor(userID uint) (snap user.HonorSnapshot, enforced bool, err error) {
	snaps, enforced, err := h.honorSnapshotsFor([]uint{userID})
	if err != nil || !enforced {
		return user.HonorSnapshot{}, enforced, err
	}
	snap, ok := snaps[userID]
	if !ok {
		return user.HonorSnapshot{}, true, fmt.Errorf("reading honor for room gate: no honor row for user %d", userID)
	}
	return snap, true, nil
}

// honorCandidateSnapshots reads honor for ownership-heir candidates, omitting any
// candidate whose row is missing instead of failing. A read error is still returned.
func (h *RoomHandler) honorCandidateSnapshots(userIDs []uint) (map[uint]user.HonorSnapshot, error) {
	if h.honorService == nil || len(userIDs) == 0 {
		return map[uint]user.HonorSnapshot{}, nil
	}
	wanted := make([]uint, 0, len(userIDs))
	for _, id := range userIDs {
		if id != 0 {
			wanted = append(wanted, id)
		}
	}
	if len(wanted) == 0 {
		return map[uint]user.HonorSnapshot{}, nil
	}
	read, err := h.honorService.HonorForUsers(wanted)
	if err != nil {
		return nil, fmt.Errorf("reading candidate honor for ownership transfer: %w", err)
	}
	return read, nil
}

// honorPrefilterAtStart re-checks the honor gate for every seated human at match start
// and ejects failing seats, returning the 409 apperr when any were ejected. It must run
// before any money moves.
func (h *RoomHandler) honorPrefilterAtStart(roomID uint, room *Room, players []RoomPlayer) error {
	if !room.honorGated() {
		return nil
	}

	humanIDs := make([]uint, 0, len(players))
	for _, p := range players {
		if p.UserID != 0 {
			humanIDs = append(humanIDs, p.UserID)
		}
	}
	snaps, enforced, err := h.honorSnapshotsFor(humanIDs)
	if err != nil {
		return err
	}
	if !enforced {
		return nil
	}

	notices := make(map[uint]ejectNotice)
	rejectByUser := make(map[uint]error)
	for _, id := range humanIDs {
		if gateErr := honorGateError(room, snaps[id]); gateErr != nil {
			notices[id] = honorNotice(room.MinHonor, snaps[id].Score)
			rejectByUser[id] = gateErr
		}
	}
	if len(notices) == 0 {
		return nil
	}

	honorEligible := func(candidateID uint) bool {
		cs, ok := snaps[candidateID]
		if !ok {
			return false
		}
		return honorGateError(room, cs) == nil
	}

	var balances map[uint]int
	if room.CoinBuyIn > 0 && h.walletService != nil {
		seatedIDs := make([]uint, 0, len(humanIDs))
		for _, id := range humanIDs {
			if _, ejected := notices[id]; !ejected {
				seatedIDs = append(seatedIDs, id)
			}
		}
		if b, berr := h.walletService.GetBalances(seatedIDs); berr == nil {
			balances = b
		} else {
			slog.Error("failed to read balances for honor-eject heir selection", "roomID", roomID, "error", berr)
		}
	}

	h.ejectSeatsAtStart(roomID, room, notices, balances, honorEligible, players)

	if ownerErr, ok := rejectByUser[room.OwnerID]; ok {
		return ownerErr
	}
	ejected := make([]uint, 0, len(rejectByUser))
	for id := range rejectByUser {
		ejected = append(ejected, id)
	}
	sort.Slice(ejected, func(i, j int) bool { return ejected[i] < ejected[j] })
	return rejectByUser[ejected[0]]
}

// transferOwnershipOrClose hands ownership to the first eligible seated human by seat
// order, or closes the room when none qualifies. It runs inside the caller's tx and never
// broadcasts.
func (h *RoomHandler) transferOwnershipOrClose(tx RoomRepository, room *Room, isEligible func(candidateID uint) bool) (newOwnerID *uint, closed bool, seatedNotify []uint, err error) {
	players, err := tx.FindPlayersByRoomID(room.ID)
	if err != nil {
		return nil, false, nil, fmt.Errorf("finding remaining players: %w", err)
	}

	seated := make([]RoomPlayer, 0, len(players))
	for _, p := range players {
		if p.Seat != nil && p.UserID != 0 {
			seated = append(seated, p)
		}
	}
	sort.Slice(seated, func(i, j int) bool { return *seated[i].Seat < *seated[j].Seat })

	for _, p := range seated {
		if isEligible(p.UserID) {
			room.OwnerID = p.UserID
			if err := tx.Update(room); err != nil {
				return nil, false, nil, fmt.Errorf("transferring room ownership: %w", err)
			}
			uid := p.UserID
			return &uid, false, nil, nil
		}
	}

	for _, p := range seated {
		seatedNotify = append(seatedNotify, p.UserID)
	}
	room.Status = "completed"
	if err := tx.Update(room); err != nil {
		return nil, false, nil, fmt.Errorf("closing ownerless room: %w", err)
	}
	for _, p := range players {
		if err := tx.RemovePlayer(room.ID, p.UserID); err != nil {
			return nil, false, nil, fmt.Errorf("evicting member on close: %w", err)
		}
	}
	bots, err := tx.FindBotsByRoomID(room.ID)
	if err != nil {
		return nil, false, nil, fmt.Errorf("finding bots on close: %w", err)
	}
	for _, b := range bots {
		if err := tx.RemoveBot(room.ID, b.Seat); err != nil {
			return nil, false, nil, fmt.Errorf("clearing bot on close: %w", err)
		}
	}
	return nil, true, seatedNotify, nil
}

// ejectKind names why a seat was ejected, selecting the per-user WS push and the returned
// apperr.
type ejectKind int

const (
	ejectKindInsolvent ejectKind = iota
	ejectKindHonor
)

// ejectNotice describes one seat's ejection: the reason plus the numbers its push
// carries.
type ejectNotice struct {
	kind     ejectKind
	buyIn    int
	balance  int
	minHonor int
	honor    int
}

// push resolves this notice into the message type and payload for the per-user WS push.
func (n ejectNotice) push(roomID uint) (string, any) {
	if n.kind == ejectKindHonor {
		return ws.SystemHonorEjected, ws.HonorEjectedPayload{
			RoomID:   roomID,
			MinHonor: n.minHonor,
			Honor:    n.honor,
		}
	}
	return ws.SystemInsolventEjected, ws.InsolventEjectedPayload{
		RoomID:  roomID,
		BuyIn:   n.buyIn,
		Balance: n.balance,
	}
}

// insolventNotice builds the insolvency ejection notice for one seat.
func insolventNotice(buyIn, balance int) ejectNotice {
	return ejectNotice{kind: ejectKindInsolvent, buyIn: buyIn, balance: balance}
}

// honorNotice builds the honor ejection notice for one seat.
func honorNotice(minHonor, honor int) ejectNotice {
	return ejectNotice{kind: ejectKindHonor, minHonor: minHonor, honor: honor}
}

// insolventNotices builds the per-seat notices for an insolvency eject at start.
func insolventNotices(ids []uint, buyIn int, balances map[uint]int) map[uint]ejectNotice {
	out := make(map[uint]ejectNotice, len(ids))
	for _, id := range ids {
		out[id] = insolventNotice(buyIn, balances[id])
	}
	return out
}

// ejectReturner frees a barred returner's seat, moves or closes ownership in one locked
// tx, broadcasts post-commit, and returns rejectWith. extraEligible, when non-nil,
// narrows ownership eligibility.
func (h *RoomHandler) ejectReturner(roomID, userID uint, notice ejectNotice, rejectWith error, extraEligible func(candidateID uint) bool, room *Room, members []RoomPlayer) error {
	var leavingUsername string
	for _, p := range members {
		if p.UserID == userID {
			leavingUsername = p.Username
			break
		}
	}

	var ownerEligible func(candidateID uint) bool
	if room.OwnerID == userID {
		presentSet := make(map[uint]bool)
		for _, id := range h.presence.Present(roomID) {
			presentSet[id] = true
		}
		var balances map[uint]int
		if room.CoinBuyIn > 0 && h.walletService != nil {
			seatedHumanIDs := make([]uint, 0, len(members))
			for _, p := range members {
				if p.UserID != userID && p.Seat != nil && p.UserID != 0 {
					seatedHumanIDs = append(seatedHumanIDs, p.UserID)
				}
			}
			b, berr := h.walletService.GetBalances(seatedHumanIDs)
			if berr != nil {
				return fmt.Errorf("reading candidate balances for ownership transfer: %w", berr)
			}
			balances = b
		}
		buyIn := room.CoinBuyIn
		ownerEligible = func(candidateID uint) bool {
			if !presentSet[candidateID] {
				return false
			}
			if buyIn > 0 && balances != nil && balances[candidateID] < buyIn {
				return false
			}
			return extraEligible == nil || extraEligible(candidateID)
		}
	}

	var newOwnerID *uint
	var roomClosed bool
	var closedNotify []uint
	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		freshRoom, err := tx.FindByIDForUpdate(roomID)
		if err != nil {
			return fmt.Errorf("re-fetching room for returner eject: %w", err)
		}
		if freshRoom == nil {
			return apperr.ErrRoomNotFound
		}
		if err := tx.RemovePlayer(roomID, userID); err != nil {
			return err
		}
		if err := tx.DecrementPlayerCount(roomID); err != nil {
			return fmt.Errorf("decrementing player count: %w", err)
		}
		if freshRoom.OwnerID == userID {
			refetched, err := tx.FindByID(roomID)
			if err != nil {
				return fmt.Errorf("re-fetching room: %w", err)
			}
			no, didClose, notify, terr := h.transferOwnershipOrClose(tx, refetched, ownerEligible)
			if terr != nil {
				return terr
			}
			newOwnerID = no
			roomClosed = didClose
			closedNotify = notify
		}
		return nil
	}); err != nil {
		if errors.Is(err, apperr.ErrNotInRoom) || errors.Is(err, apperr.ErrRoomNotFound) {
			return err
		}
		return fmt.Errorf("ejecting barred returner: %w", err)
	}

	h.presence.Remove(roomID, userID)
	if roomClosed {
		h.presence.Clear(roomID)
		h.invites.VoidRoom(roomID)
	}

	remainingPlayers, rerr := h.repo.FindPlayersByRoomID(roomID)
	postRoom, perr := h.repo.FindByID(roomID)
	if rerr == nil && len(remainingPlayers) > 0 {
		actualPlayerCount := len(remainingPlayers)
		if perr == nil && postRoom != nil {
			actualPlayerCount = postRoom.PlayerCount
		}
		userIDs := make([]uint, 0, len(remainingPlayers))
		for _, p := range remainingPlayers {
			userIDs = append(userIDs, p.UserID)
		}
		payload := map[string]interface{}{
			"roomId":      roomID,
			"userId":      userID,
			"username":    leavingUsername,
			"playerCount": actualPlayerCount,
		}
		if newOwnerID != nil {
			payload["newOwnerId"] = *newOwnerID
		}
		h.broadcastToUsers(userIDs, ws.SystemPlayerLeft, payload)
	}
	if perr == nil && postRoom != nil {
		h.broadcastRoomUpdated(postRoom)
	}
	if roomClosed && len(closedNotify) > 0 {
		h.broadcastToUsers(closedNotify, ws.SystemRoomClosedInsolvent, ws.RoomClosedInsolventPayload{RoomID: roomID})
	}

	msgType, payload := notice.push(roomID)
	h.broadcastToUsers([]uint{userID}, msgType, payload)

	return rejectWith
}

// ejectSeatsAtStart frees every barred seat at match start, pushes each ejected player
// their notice, and reverts the room to waiting or closes it. DB errors are logged and
// the pushes still go out.
func (h *RoomHandler) ejectSeatsAtStart(roomID uint, room *Room, notices map[uint]ejectNotice, balances map[uint]int, extraEligible func(candidateID uint) bool, members []RoomPlayer) {
	ejectedIDs := make([]uint, 0, len(notices))
	for id := range notices {
		ejectedIDs = append(ejectedIDs, id)
	}
	sort.Slice(ejectedIDs, func(i, j int) bool { return ejectedIDs[i] < ejectedIDs[j] })

	ejectedSet := make(map[uint]bool, len(ejectedIDs))
	for _, id := range ejectedIDs {
		ejectedSet[id] = true
	}

	buyIn := room.CoinBuyIn
	startEligible := func(candidateID uint) bool {
		if ejectedSet[candidateID] {
			return false
		}
		if buyIn > 0 && balances != nil && balances[candidateID] < buyIn {
			return false
		}
		return extraEligible == nil || extraEligible(candidateID)
	}

	var newOwnerID *uint
	var roomClosed bool
	var closedNotify []uint
	txErr := h.repo.RunInTransaction(func(tx RoomRepository) error {
		freshRoom, err := tx.FindByIDForUpdate(roomID)
		if err != nil {
			return fmt.Errorf("re-fetching room for start eject: %w", err)
		}
		if freshRoom == nil {
			return apperr.ErrRoomNotFound
		}
		ownerEjected := false
		for _, id := range ejectedIDs {
			if rmErr := tx.RemovePlayer(roomID, id); rmErr != nil {
				if !errors.Is(rmErr, apperr.ErrNotInRoom) {
					return fmt.Errorf("freeing ejected seat %d: %w", id, rmErr)
				}
				continue
			}
			if err := tx.DecrementPlayerCount(roomID); err != nil {
				return fmt.Errorf("decrementing player count: %w", err)
			}
			if freshRoom.OwnerID == id {
				ownerEjected = true
			}
		}

		if ownerEjected {
			refetched, err := tx.FindByID(roomID)
			if err != nil {
				return fmt.Errorf("re-fetching room: %w", err)
			}
			no, didClose, notify, terr := h.transferOwnershipOrClose(tx, refetched, startEligible)
			if terr != nil {
				return terr
			}
			newOwnerID = no
			roomClosed = didClose
			closedNotify = notify
			if !didClose {
				refetched.Status = "waiting"
				if err := tx.Update(refetched); err != nil {
					return fmt.Errorf("reverting room to waiting: %w", err)
				}
			}
		} else if err := tx.UpdateStatus(roomID, "waiting"); err != nil {
			return fmt.Errorf("reverting room to waiting: %w", err)
		}
		return nil
	})
	if txErr != nil {
		slog.Error("failed to eject barred seats at start", "roomID", roomID, "error", txErr)
		if uerr := h.repo.UpdateStatus(roomID, "waiting"); uerr != nil {
			slog.Error("failed to revert room to waiting after eject failure", "roomID", roomID, "error", uerr)
		}
	}

	for _, id := range ejectedIDs {
		msgType, payload := notices[id].push(roomID)
		h.broadcastToUsers([]uint{id}, msgType, payload)
	}

	if roomClosed {
		h.presence.Clear(roomID)
		h.invites.VoidRoom(roomID)
	}

	postRoom, perr := h.repo.FindByID(roomID)
	if perr == nil && postRoom != nil {
		h.broadcastRoomUpdated(postRoom)
	}
	if roomClosed && len(closedNotify) > 0 {
		h.broadcastToUsers(closedNotify, ws.SystemRoomClosedInsolvent, ws.RoomClosedInsolventPayload{RoomID: roomID})
	}

	if txErr == nil && !roomClosed {
		usernames := make(map[uint]string, len(members))
		for _, p := range members {
			usernames[p.UserID] = p.Username
		}
		remainingPlayers, rerr := h.repo.FindPlayersByRoomID(roomID)
		if rerr == nil && len(remainingPlayers) > 0 {
			remainingIDs := make([]uint, 0, len(remainingPlayers))
			for _, p := range remainingPlayers {
				remainingIDs = append(remainingIDs, p.UserID)
			}
			playerCount := len(remainingPlayers)
			if postRoom != nil {
				playerCount = postRoom.PlayerCount
			}
			for _, id := range ejectedIDs {
				payload := map[string]interface{}{
					"roomId":      roomID,
					"userId":      id,
					"username":    usernames[id],
					"playerCount": playerCount,
				}
				if newOwnerID != nil {
					payload["newOwnerId"] = *newOwnerID
				}
				h.broadcastToUsers(remainingIDs, ws.SystemPlayerLeft, payload)
			}
		}
	}
}

func (h *RoomHandler) LeaveRoom(c echo.Context) error {
	userID, err := auth.GetUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	roomID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.ErrRoomNotFound
	}

	room, err := h.repo.FindByID(uint(roomID))
	if err != nil {
		return fmt.Errorf("finding room: %w", err)
	}
	if room == nil {
		return apperr.ErrRoomNotFound
	}

	var leavingUsername string
	prePlayers, _ := h.repo.FindPlayersByRoomID(uint(roomID))
	for _, p := range prePlayers {
		if p.UserID == userID {
			leavingUsername = p.Username
			break
		}
	}

	var ownerEligible func(candidateID uint) bool
	if room.OwnerID == userID {
		presentSet := make(map[uint]bool)
		for _, id := range h.presence.Present(uint(roomID)) {
			presentSet[id] = true
		}
		var balances map[uint]int
		if room.CoinBuyIn > 0 && h.walletService != nil {
			seatedHumanIDs := make([]uint, 0, len(prePlayers))
			for _, p := range prePlayers {
				if p.UserID != userID && p.Seat != nil && p.UserID != 0 {
					seatedHumanIDs = append(seatedHumanIDs, p.UserID)
				}
			}
			b, berr := h.walletService.GetBalances(seatedHumanIDs)
			if berr != nil {
				return fmt.Errorf("reading candidate balances for ownership transfer: %w", berr)
			}
			balances = b
		}
		buyIn := room.CoinBuyIn
		ownerEligible = func(candidateID uint) bool {
			if !presentSet[candidateID] {
				return false
			}
			return buyIn == 0 || (balances != nil && balances[candidateID] >= buyIn)
		}
	}

	var newOwnerID *uint
	var roomClosed bool
	var closedNotify []uint
	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		freshRoom, err := tx.FindByIDForUpdate(uint(roomID))
		if err != nil {
			return fmt.Errorf("re-fetching room for leave gate: %w", err)
		}
		if freshRoom == nil {
			return apperr.ErrRoomNotFound
		}
		if freshRoom.Status == "playing" {
			return apperr.ErrMatchAlreadyStarted
		}
		if err := tx.RemovePlayer(uint(roomID), userID); err != nil {
			return err
		}
		if err := tx.DecrementPlayerCount(uint(roomID)); err != nil {
			return fmt.Errorf("decrementing player count: %w", err)
		}
		if room.OwnerID == userID {
			freshRoom, err := tx.FindByID(uint(roomID))
			if err != nil {
				return fmt.Errorf("re-fetching room: %w", err)
			}
			no, didClose, notify, terr := h.transferOwnershipOrClose(tx, freshRoom, ownerEligible)
			if terr != nil {
				return terr
			}
			newOwnerID = no
			roomClosed = didClose
			closedNotify = notify
		}
		return nil
	}); err != nil {
		if errors.Is(err, apperr.ErrNotInRoom) ||
			errors.Is(err, apperr.ErrRoomNotFound) ||
			errors.Is(err, apperr.ErrMatchAlreadyStarted) {
			return err
		}
		return fmt.Errorf("leaving room: %w", err)
	}

	h.presence.Remove(uint(roomID), userID)
	h.invites.VoidInviter(uint(roomID), userID)
	if roomClosed {
		h.presence.Clear(uint(roomID))
		h.invites.VoidRoom(uint(roomID))
	}

	remainingPlayers, broadcastErr := h.repo.FindPlayersByRoomID(uint(roomID))
	postRoom, postErr := h.repo.FindByID(uint(roomID))
	if broadcastErr == nil && len(remainingPlayers) > 0 {
		actualPlayerCount := len(remainingPlayers)
		if postErr == nil && postRoom != nil {
			actualPlayerCount = postRoom.PlayerCount
		}
		userIDs := make([]uint, 0, len(remainingPlayers))
		for _, p := range remainingPlayers {
			userIDs = append(userIDs, p.UserID)
		}
		payload := map[string]interface{}{
			"roomId":      roomID,
			"userId":      userID,
			"username":    leavingUsername,
			"playerCount": actualPlayerCount,
		}
		if newOwnerID != nil {
			payload["newOwnerId"] = *newOwnerID
		}
		h.broadcastToUsers(userIDs, ws.SystemPlayerLeft, payload)
	}

	if roomClosed && len(closedNotify) > 0 {
		h.broadcastToUsers(closedNotify, ws.SystemRoomClosedInsolvent, ws.RoomClosedInsolventPayload{RoomID: uint(roomID)})
	}

	if postErr == nil && postRoom != nil {
		h.broadcastRoomUpdated(postRoom)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"data": map[string]string{"message": "left room"}})
}

// ReturnToRoom reopens a finished room so the same group can play again in their seats.
// Only current members may reopen, and a call on an already waiting room is a no-op
// success.
func (h *RoomHandler) ReturnToRoom(c echo.Context) error {
	userID, err := auth.GetUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	roomID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.ErrRoomNotFound
	}

	members, err := h.repo.FindPlayersByRoomID(uint(roomID))
	if err != nil {
		return fmt.Errorf("finding room players: %w", err)
	}
	isMember := false
	for _, p := range members {
		if p.UserID == userID {
			isMember = true
			break
		}
	}
	if !isMember {
		return apperr.ErrNotInRoom
	}

	gateRoom, err := h.repo.FindByID(uint(roomID))
	if err != nil {
		return fmt.Errorf("finding room for return gate: %w", err)
	}
	if gateRoom == nil {
		return apperr.ErrRoomNotFound
	}
	if gateRoom.CoinBuyIn > 0 && h.walletService != nil {
		balance, berr := h.walletService.GetBalance(userID)
		if berr != nil {
			return fmt.Errorf("reading balance for return gate: %w", berr)
		}
		if balance < gateRoom.CoinBuyIn {
			return h.ejectReturner(uint(roomID), userID, insolventNotice(gateRoom.CoinBuyIn, balance), apperr.ErrInsufficientCoins, nil, gateRoom, members)
		}
	}

	if gateRoom.honorGated() {
		snap, enforced, herr := h.honorSnapshotFor(userID)
		if herr != nil {
			return herr
		}
		if enforced {
			if gateErr := honorGateError(gateRoom, snap); gateErr != nil {
				var honorEligible func(candidateID uint) bool
				if gateRoom.OwnerID == userID {
					candidateIDs := make([]uint, 0, len(members))
					for _, p := range members {
						if p.UserID != userID && p.Seat != nil && p.UserID != 0 {
							candidateIDs = append(candidateIDs, p.UserID)
						}
					}
					candidateSnaps, cerr := h.honorCandidateSnapshots(candidateIDs)
					if cerr != nil {
						return cerr
					}
					honorEligible = func(candidateID uint) bool {
						cs, ok := candidateSnaps[candidateID]
						if !ok {
							return false
						}
						return honorGateError(gateRoom, cs) == nil
					}
				}
				return h.ejectReturner(uint(roomID), userID, honorNotice(gateRoom.MinHonor, snap.Score), gateErr, honorEligible, gateRoom, members)
			}
		}
	}

	var clearedSeats []int
	var reopened bool
	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		freshRoom, err := tx.FindByIDForUpdate(uint(roomID))
		if err != nil {
			return fmt.Errorf("re-fetching room for return: %w", err)
		}
		if freshRoom == nil {
			return apperr.ErrRoomNotFound
		}
		switch freshRoom.Status {
		case "waiting":
			return nil
		case "completed":
			reopened = true
			if err := tx.UpdateStatus(uint(roomID), "waiting"); err != nil {
				return fmt.Errorf("reopening room: %w", err)
			}
			bots, err := tx.FindBotsByRoomID(uint(roomID))
			if err != nil {
				return fmt.Errorf("finding room bots: %w", err)
			}
			for _, b := range bots {
				if err := tx.RemoveBot(uint(roomID), b.Seat); err != nil {
					return fmt.Errorf("clearing bot on seat %d: %w", b.Seat, err)
				}
				clearedSeats = append(clearedSeats, b.Seat)
			}
			return nil
		default:
			return apperr.ErrMatchAlreadyStarted
		}
	}); err != nil {
		if errors.Is(err, apperr.ErrRoomNotFound) ||
			errors.Is(err, apperr.ErrMatchAlreadyStarted) {
			return err
		}
		return fmt.Errorf("returning to room: %w", err)
	}

	postRoom, err := h.repo.FindByID(uint(roomID))
	if err != nil {
		return fmt.Errorf("re-fetching room after return: %w", err)
	}
	if postRoom == nil {
		return apperr.ErrRoomNotFound
	}
	if reopened {
		for _, seat := range clearedSeats {
			h.broadcastToRoom(uint(roomID), ws.SystemBotRemoved, map[string]interface{}{
				"roomId": roomID,
				"seat":   seat,
			})
		}
		h.broadcastRoomUpdated(postRoom)
	}

	h.presence.Add(uint(roomID), userID)
	h.invites.VoidUser(userID)
	h.voidInvitesIfFull(uint(roomID))
	h.broadcastToRoom(uint(roomID), ws.SystemPlayerReturned, ws.PlayerReturnedPayload{
		RoomID: uint(roomID),
		UserID: userID,
	})

	players, err := h.repo.FindPlayersByRoomID(uint(roomID))
	if err != nil {
		return fmt.Errorf("finding room players: %w", err)
	}
	bots, err := h.repo.FindBotsByRoomID(uint(roomID))
	if err != nil {
		return fmt.Errorf("finding room bots: %w", err)
	}
	if err := h.repo.LoadOwnerUsernames([]*Room{postRoom}); err != nil {
		slog.Error("return to room: failed to load owner username", "roomID", postRoom.ID, "error", err)
	}

	roster := mergeBotPlayers(players, bots)
	h.attachPlayerHonor(roster)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": RoomDetailResponse{
			Room:            postRoom,
			Players:         roster,
			ReturnedUserIds: h.presence.Present(uint(roomID)),
		},
	})
}

type SelectSeatRequest struct {
	Seat *int `json:"seat"`
}

type PlayersResponse struct {
	Players []RoomPlayer `json:"players"`
}

func teamForSeat(seat int) string {
	if seat%2 == 0 {
		return "teamA"
	}
	return "teamB"
}

func (h *RoomHandler) SelectSeat(c echo.Context) error {
	userID, err := auth.GetUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	roomID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.ErrRoomNotFound
	}

	var req SelectSeatRequest
	if err := c.Bind(&req); err != nil {
		return apperr.ErrBadRequest
	}

	if req.Seat == nil {
		return apperr.ErrInvalidSeat
	}
	seat := *req.Seat
	if seat < 0 || seat > 3 {
		return apperr.ErrInvalidSeat
	}

	team := teamForSeat(seat)

	var previousSeat *int
	seatChanged := false
	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		room, err := tx.FindByIDForUpdate(uint(roomID))
		if err != nil {
			return fmt.Errorf("finding room: %w", err)
		}
		if room == nil {
			return apperr.ErrRoomNotFound
		}
		if room.Status != "waiting" {
			return apperr.ErrMatchNotStartable
		}

		existing, err := tx.FindPlayerBySeat(uint(roomID), seat)
		if err != nil {
			return fmt.Errorf("checking seat occupancy: %w", err)
		}
		if existing != nil {
			if existing.UserID == userID {
				return nil
			}
			return apperr.ErrSeatTaken
		}
		if taken, err := botOccupiesSeat(tx, uint(roomID), seat); err != nil {
			return fmt.Errorf("checking bot seat occupancy: %w", err)
		} else if taken {
			return apperr.ErrSeatTaken
		}

		player, err := tx.FindPlayerRoom(userID)
		if err != nil {
			return fmt.Errorf("finding player room: %w", err)
		}
		if player == nil || player.RoomID != uint(roomID) {
			return apperr.ErrNotInRoom
		}

		if player.Seat != nil {
			prev := *player.Seat
			previousSeat = &prev
			if err := tx.ClearPlayerSeat(uint(roomID), userID); err != nil {
				return fmt.Errorf("clearing previous seat: %w", err)
			}
		}

		if err := tx.UpdatePlayerSeat(uint(roomID), userID, seat, team); err != nil {
			return fmt.Errorf("updating player seat: %w", err)
		}

		seatChanged = true
		return nil
	}); err != nil {
		if errors.Is(err, apperr.ErrSeatTaken) || errors.Is(err, apperr.ErrNotInRoom) ||
			errors.Is(err, apperr.ErrRoomNotFound) || errors.Is(err, apperr.ErrMatchNotStartable) {
			return err
		}
		return fmt.Errorf("selecting seat: %w", err)
	}

	players, err := h.repo.FindPlayersByRoomID(uint(roomID))
	if err != nil {
		return fmt.Errorf("fetching players after seat update: %w", err)
	}

	if seatChanged {
		var username string
		var avatarURL *string
		for _, p := range players {
			if p.UserID == userID {
				username = p.Username
				avatarURL = p.AvatarURL
				break
			}
		}
		seatPayload := map[string]interface{}{
			"roomId":       roomID,
			"userId":       userID,
			"username":     username,
			"avatarUrl":    avatarURL,
			"seat":         seat,
			"team":         team,
			"previousSeat": previousSeat,
		}
		h.broadcastToRoom(uint(roomID), ws.SystemSeatUpdated, seatPayload)
		h.broadcastRoomSeatSnapshot(uint(roomID), players)
	}

	matchStarted, err := h.autoStartIfFull(uint(roomID))
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"players":      h.playersWithBots(uint(roomID)),
			"matchStarted": matchStarted,
		},
	})
}

// startAutoStartedMatch starts the match for a room an auto-start path already flipped to
// playing. The caller must revert the status flip when this returns an error.
func (h *RoomHandler) startAutoStartedMatch(autoStartRoom *Room, players []RoomPlayer, bots []RoomBot) error {
	if h.matchStarter == nil {
		return nil
	}
	var seatInfo [4]match.PlayerSeatInfo
	for _, p := range players {
		if p.Seat != nil {
			seatInfo[*p.Seat] = match.PlayerSeatInfo{
				UserID:    p.UserID,
				Username:  p.Username,
				AvatarURL: p.AvatarURL,
				Seat:      *p.Seat,
			}
		}
	}
	for _, b := range bots {
		if b.Seat >= 0 && b.Seat <= 3 {
			seatInfo[b.Seat] = match.PlayerSeatInfo{
				Seat:  b.Seat,
				IsBot: true,
			}
		}
	}

	charged := false
	var chargedIDs []uint
	if autoStartRoom.CoinBuyIn > 0 && h.walletService != nil {
		humanIDs := make([]uint, 0, len(players))
		for _, p := range players {
			humanIDs = append(humanIDs, p.UserID)
		}

		balances, berr := h.walletService.GetBalances(humanIDs)
		if berr != nil {
			return fmt.Errorf("reading balances for auto-start: %w", berr)
		}

		insolvent := make([]uint, 0, len(humanIDs))
		for _, id := range humanIDs {
			if balances[id] < autoStartRoom.CoinBuyIn {
				insolvent = append(insolvent, id)
			}
		}
		if len(insolvent) > 0 {
			h.ejectSeatsAtStart(autoStartRoom.ID, autoStartRoom, insolventNotices(insolvent, autoStartRoom.CoinBuyIn, balances), balances, nil, players)
			return apperr.ErrInsufficientCoins
		}

		if insolventID, chargeErr := h.walletService.ChargeStakes(humanIDs, autoStartRoom.CoinBuyIn); chargeErr != nil {
			if errors.Is(chargeErr, apperr.ErrInsufficientCoins) {
				if fresh, ferr := h.walletService.GetBalances([]uint{insolventID}); ferr == nil {
					balances[insolventID] = fresh[insolventID]
				}
				h.ejectSeatsAtStart(autoStartRoom.ID, autoStartRoom, insolventNotices([]uint{insolventID}, autoStartRoom.CoinBuyIn, balances), balances, nil, players)
				return apperr.ErrInsufficientCoins
			}
			return fmt.Errorf("charging stakes for auto-start: %w", chargeErr)
		}
		charged = true
		chargedIDs = humanIDs
	}

	timerDuration := 0
	if autoStartRoom.TimerDurationSeconds != nil {
		timerDuration = *autoStartRoom.TimerDurationSeconds
	}
	reconnectWindow := resolveReconnectWindow(autoStartRoom.ReconnectWindowSec)
	if serr := h.matchStarter.StartMatch(autoStartRoom.ID, autoStartRoom.Variant, autoStartRoom.MatchMode, seatInfo, autoStartRoom.TimerStyle, timerDuration, autoStartRoom.OwnerID, reconnectWindow, autoStartRoom.CoinBuyIn, autoStartRoom.DeclarationsEnabled, autoStartRoom.StopAtTarget); serr != nil {
		if charged {
			refund := make(map[uint]int, len(chargedIDs))
			for _, id := range chargedIDs {
				refund[id] = autoStartRoom.CoinBuyIn
			}
			if rerr := h.walletService.ApplySettlement(refund); rerr != nil {
				slog.Error("failed to refund stakes after auto-start failure", "roomID", autoStartRoom.ID, "error", rerr)
			}
		}
		return serr
	}
	return nil
}

// revertAutoStart flips a room back to waiting after a failed StartMatch and broadcasts
// error:match_start_failed to the would-be participants, re-fetching them when players is
// nil.
func (h *RoomHandler) revertAutoStart(roomID uint, autoStartRoom *Room, players []RoomPlayer) {
	revertErr := h.repo.RunInTransaction(func(tx RoomRepository) error {
		r, err := tx.FindByIDForUpdate(roomID)
		if err != nil {
			return fmt.Errorf("re-fetching room for status revert: %w", err)
		}
		if r == nil {
			return nil
		}
		if r.Status != "playing" {
			autoStartRoom.Status = r.Status
			return nil
		}
		r.Status = "waiting"
		if err := tx.Update(r); err != nil {
			return fmt.Errorf("reverting room status to waiting: %w", err)
		}
		autoStartRoom.Status = "waiting"
		return nil
	})
	if revertErr != nil {
		slog.Error("failed to revert auto-start status flip; aborting failure broadcast", "roomID", roomID, "error", revertErr)
		return
	}

	if len(players) == 0 {
		if fetched, ferr := h.repo.FindPlayersByRoomID(roomID); ferr == nil {
			players = fetched
		} else {
			slog.Error("failed to load players for revertAutoStart broadcast", "roomID", roomID, "error", ferr)
		}
	}

	userIDs := make([]uint, 0, len(players))
	for _, p := range players {
		userIDs = append(userIDs, p.UserID)
	}
	if len(userIDs) > 0 {
		h.broadcastToUsers(userIDs, ws.ErrorMatchStartFailed, map[string]interface{}{
			"roomId":  roomID,
			"message": "Failed to start the game. Please try again.",
		})
	}

	if autoStartRoom.Status == "waiting" {
		h.broadcastRoomUpdated(autoStartRoom)
	}
}

// seatPlayerIntoQuickRoom adds the player to a quick play room at the lowest empty seat.
// It must be called inside a transaction.
func seatPlayerIntoQuickRoom(tx RoomRepository, roomID, userID uint) (seat int, team string, room *Room, err error) {
	rp := &RoomPlayer{RoomID: roomID, UserID: userID}
	if err = tx.AddPlayer(rp); err != nil {
		return 0, "", nil, err
	}
	if err = tx.IncrementPlayerCount(roomID); err != nil {
		return 0, "", nil, fmt.Errorf("incrementing player count: %w", err)
	}
	seat, err = pickFirstEmptySeat(tx, roomID)
	if err != nil {
		return 0, "", nil, err
	}
	team = teamForSeat(seat)
	if err = tx.UpdatePlayerSeat(roomID, userID, seat, team); err != nil {
		return 0, "", nil, fmt.Errorf("auto-seating player: %w", err)
	}
	room, err = tx.FindByID(roomID)
	if err != nil {
		return 0, "", nil, fmt.Errorf("re-fetching room after join: %w", err)
	}
	return seat, team, room, nil
}

// autoStartIfFull row-locks the room and starts the match when a waiting quick play room
// has all four seats filled. It returns true only when the match started.
func (h *RoomHandler) autoStartIfFull(roomID uint) (bool, error) {
	matchStarted := false
	var autoStartRoom *Room
	var autoStartPlayers []RoomPlayer
	var autoStartBots []RoomBot
	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		r, err := tx.FindByIDForUpdate(roomID)
		if err != nil {
			return fmt.Errorf("fetching room for auto-start check: %w", err)
		}
		if r == nil || !r.IsQuickPlay || r.Status != "waiting" {
			return nil
		}
		freshPlayers, err := tx.FindPlayersByRoomID(roomID)
		if err != nil {
			return fmt.Errorf("fetching players for auto-start check: %w", err)
		}
		bots, err := tx.FindBotsByRoomID(roomID)
		if err != nil {
			return fmt.Errorf("fetching bots for auto-start check: %w", err)
		}
		seatedCount := 0
		for _, p := range freshPlayers {
			if p.Seat != nil {
				seatedCount++
			}
		}
		if seatedCount+len(bots) < 4 {
			return nil
		}
		r.Status = "playing"
		if err := tx.Update(r); err != nil {
			return fmt.Errorf("auto-starting quick play room: %w", err)
		}
		matchStarted = true
		autoStartRoom = r
		autoStartPlayers = freshPlayers
		autoStartBots = bots
		return nil
	}); err != nil {
		return false, fmt.Errorf("auto-start check: %w", err)
	}

	if matchStarted && autoStartRoom != nil {
		startErr := h.startAutoStartedMatch(autoStartRoom, autoStartPlayers, autoStartBots)
		if startErr != nil {
			slog.Error("failed to start auto-started game session", "roomID", roomID, "error", startErr)
			if !errors.Is(startErr, apperr.ErrInsufficientCoins) {
				h.revertAutoStart(roomID, autoStartRoom, autoStartPlayers)
			}
			matchStarted = false
		} else {
			h.presence.Clear(roomID)
			h.invites.VoidRoom(roomID)
			h.cancelQuickFill(roomID)
			h.broadcastToRoom(roomID, ws.SystemMatchStarted, map[string]interface{}{
				"roomId": roomID,
			})
			h.broadcastRoomUpdated(autoStartRoom)
		}
	}

	return matchStarted, nil
}

type KickPlayerRequest struct {
	UserID uint `json:"userId"`
}

func (h *RoomHandler) KickPlayer(c echo.Context) error {
	ownerID, err := auth.GetUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	roomID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.ErrRoomNotFound
	}

	var req KickPlayerRequest
	if err := c.Bind(&req); err != nil {
		return apperr.ErrBadRequest
	}
	if req.UserID == 0 {
		return apperr.ErrBadRequest
	}

	var kickedUsername string
	prePlayers, _ := h.repo.FindPlayersByRoomID(uint(roomID))
	for _, p := range prePlayers {
		if p.UserID == req.UserID {
			kickedUsername = p.Username
			break
		}
	}

	var postRoom *Room
	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		r, err := tx.FindByID(uint(roomID))
		if err != nil {
			return fmt.Errorf("finding room: %w", err)
		}
		if r == nil {
			return apperr.ErrRoomNotFound
		}
		if r.Status != "waiting" {
			return apperr.ErrRoomNotWaiting
		}
		if r.OwnerID != ownerID {
			return apperr.ErrNotRoomOwner
		}
		if req.UserID == r.OwnerID {
			return apperr.ErrCannotKickSelf
		}

		target, err := tx.FindPlayerRoom(req.UserID)
		if err != nil {
			return fmt.Errorf("finding target player room: %w", err)
		}
		if target == nil || target.RoomID != uint(roomID) {
			return apperr.ErrNotInRoom
		}

		if err := tx.RemovePlayer(uint(roomID), req.UserID); err != nil {
			return err
		}
		if err := tx.DecrementPlayerCount(uint(roomID)); err != nil {
			return fmt.Errorf("decrementing player count: %w", err)
		}

		fresh, err := tx.FindByID(uint(roomID))
		if err != nil {
			return fmt.Errorf("re-fetching room after kick: %w", err)
		}
		postRoom = fresh
		return nil
	}); err != nil {
		if errors.Is(err, apperr.ErrRoomNotFound) ||
			errors.Is(err, apperr.ErrRoomNotWaiting) ||
			errors.Is(err, apperr.ErrNotRoomOwner) ||
			errors.Is(err, apperr.ErrCannotKickSelf) ||
			errors.Is(err, apperr.ErrNotInRoom) {
			return err
		}
		return fmt.Errorf("kicking player: %w", err)
	}

	if postRoom == nil {
		return apperr.ErrRoomNotFound
	}

	h.presence.Remove(uint(roomID), req.UserID)
	h.invites.VoidInviter(uint(roomID), req.UserID)

	h.broadcastToUsers([]uint{req.UserID}, ws.SystemRoomKicked, ws.RoomKickedPayload{
		RoomID: uint(roomID),
		Reason: "kicked_by_owner",
	})

	remainingPlayers, broadcastErr := h.repo.FindPlayersByRoomID(uint(roomID))
	if broadcastErr == nil {
		userIDs := make([]uint, 0, len(remainingPlayers))
		for _, p := range remainingPlayers {
			userIDs = append(userIDs, p.UserID)
		}
		h.broadcastToUsers(userIDs, ws.SystemPlayerLeft, map[string]interface{}{
			"roomId":      roomID,
			"userId":      req.UserID,
			"username":    kickedUsername,
			"playerCount": postRoom.PlayerCount,
		})

		h.broadcastRoomUpdated(postRoom)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{"playerCount": postRoom.PlayerCount},
	})
}

type TransferOwnershipRequest struct {
	UserID uint `json:"userId"`
}

// TransferOwnership reassigns room ownership from the current owner to another seated
// room member.
func (h *RoomHandler) TransferOwnership(c echo.Context) error {
	ownerID, err := auth.GetUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	roomID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.ErrRoomNotFound
	}

	var req TransferOwnershipRequest
	if err := c.Bind(&req); err != nil {
		return apperr.ErrBadRequest
	}
	if req.UserID == 0 {
		return apperr.ErrBadRequest
	}
	if req.UserID == ownerID {
		return apperr.ErrCannotTransferToSelf
	}

	var (
		postRoom        *Room
		newOwnerName    string
		previousOwnerID uint
	)

	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		r, err := tx.FindByID(uint(roomID))
		if err != nil {
			return fmt.Errorf("finding room: %w", err)
		}
		if r == nil {
			return apperr.ErrRoomNotFound
		}
		if r.Status != "waiting" {
			return apperr.ErrRoomNotWaiting
		}
		if r.OwnerID != ownerID {
			return apperr.ErrNotRoomOwner
		}

		target, err := tx.FindPlayerRoom(req.UserID)
		if err != nil {
			return fmt.Errorf("finding target player room: %w", err)
		}
		if target == nil || target.RoomID != uint(roomID) {
			return apperr.ErrNotInRoom
		}
		if target.Seat == nil {
			return apperr.ErrCannotPromoteUnseated
		}

		previousOwnerID = r.OwnerID
		r.OwnerID = req.UserID
		if err := tx.Update(r); err != nil {
			return fmt.Errorf("updating room owner: %w", err)
		}
		newOwnerName = target.Username
		postRoom = r
		return nil
	}); err != nil {
		if errors.Is(err, apperr.ErrRoomNotFound) ||
			errors.Is(err, apperr.ErrRoomNotWaiting) ||
			errors.Is(err, apperr.ErrNotRoomOwner) ||
			errors.Is(err, apperr.ErrNotInRoom) ||
			errors.Is(err, apperr.ErrCannotPromoteUnseated) ||
			errors.Is(err, apperr.ErrCannotTransferToSelf) {
			return err
		}
		return fmt.Errorf("transferring ownership: %w", err)
	}

	if postRoom == nil {
		return apperr.ErrRoomNotFound
	}

	h.invites.VoidInviter(uint(roomID), previousOwnerID)

	if err := h.repo.LoadOwnerUsernames([]*Room{postRoom}); err != nil {
		slog.Error("transfer ownership: failed to load new owner identity", "roomID", roomID, "error", err)
	}

	h.broadcastToRoom(uint(roomID), ws.SystemRoomOwnerChanged, map[string]interface{}{
		"roomId":            roomID,
		"newOwnerId":        postRoom.OwnerID,
		"newOwnerUsername":  newOwnerName,
		"newOwnerAvatarUrl": postRoom.OwnerAvatarURL,
		"previousOwnerId":   previousOwnerID,
	})
	h.broadcastRoomUpdated(postRoom)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"ownerId": postRoom.OwnerID,
		},
	})
}

type SwapSeatsRequest struct {
	SeatA *int `json:"seatA"`
	SeatB *int `json:"seatB"`
}

func (h *RoomHandler) SwapSeats(c echo.Context) error {
	ownerID, err := auth.GetUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	roomID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.ErrRoomNotFound
	}

	var req SwapSeatsRequest
	if err := c.Bind(&req); err != nil {
		return apperr.ErrBadRequest
	}
	if req.SeatA == nil || req.SeatB == nil {
		return apperr.ErrInvalidSeat
	}
	seatA := *req.SeatA
	seatB := *req.SeatB
	if seatA < 0 || seatA > 3 || seatB < 0 || seatB > 3 || seatA == seatB {
		return apperr.ErrInvalidSeat
	}

	type swapped struct {
		userID       uint
		username     string
		avatarURL    *string
		seat         int
		team         string
		previousSeat int
	}
	var humanMoves []swapped
	type botRelocation struct {
		from int
		to   int
	}
	var botMove *botRelocation
	var botNoOp bool

	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		r, err := tx.FindByIDForUpdate(uint(roomID))
		if err != nil {
			return fmt.Errorf("finding room: %w", err)
		}
		if r == nil {
			return apperr.ErrRoomNotFound
		}
		if r.Status != "waiting" {
			return apperr.ErrRoomNotWaiting
		}
		if r.OwnerID != ownerID {
			return apperr.ErrNotRoomOwner
		}

		pA, err := tx.FindPlayerBySeat(uint(roomID), seatA)
		if err != nil {
			return fmt.Errorf("finding player at seatA: %w", err)
		}
		pB, err := tx.FindPlayerBySeat(uint(roomID), seatB)
		if err != nil {
			return fmt.Errorf("finding player at seatB: %w", err)
		}
		botA, err := botOccupiesSeat(tx, uint(roomID), seatA)
		if err != nil {
			return fmt.Errorf("checking bot at seatA: %w", err)
		}
		botB, err := botOccupiesSeat(tx, uint(roomID), seatB)
		if err != nil {
			return fmt.Errorf("checking bot at seatB: %w", err)
		}
		if pA == nil && pB == nil && !botA && !botB {
			return apperr.ErrSeatNotOccupied
		}

		if botA && botB {
			botNoOp = true
			return nil
		}

		moveHuman := func(p *RoomPlayer, from, to int) error {
			team := teamForSeat(to)
			if err := tx.UpdatePlayerSeat(uint(roomID), p.UserID, to, team); err != nil {
				return fmt.Errorf("updating player seat: %w", err)
			}
			humanMoves = append(humanMoves, swapped{
				userID:       p.UserID,
				username:     p.Username,
				avatarURL:    p.AvatarURL,
				seat:         to,
				team:         team,
				previousSeat: from,
			})
			return nil
		}

		if botA || botB {
			botFrom, botTo := seatA, seatB
			otherHuman := pB
			if botB {
				botFrom, botTo = seatB, seatA
				otherHuman = pA
			}
			if otherHuman != nil {
				if err := moveHuman(otherHuman, botTo, botFrom); err != nil {
					return err
				}
			}
			if err := tx.UpdateBotSeat(uint(roomID), botFrom, botTo); err != nil {
				return fmt.Errorf("moving bot between seats: %w", err)
			}
			botMove = &botRelocation{from: botFrom, to: botTo}
			return nil
		}

		if pA == nil || pB == nil {
			var mover *RoomPlayer
			var fromSeat, toSeat int
			if pA == nil {
				mover, fromSeat, toSeat = pB, seatB, seatA
			} else {
				mover, fromSeat, toSeat = pA, seatA, seatB
			}
			return moveHuman(mover, fromSeat, toSeat)
		}

		if err := moveHuman(pA, seatA, seatB); err != nil {
			return err
		}
		return moveHuman(pB, seatB, seatA)
	}); err != nil {
		if errors.Is(err, apperr.ErrRoomNotFound) ||
			errors.Is(err, apperr.ErrRoomNotWaiting) ||
			errors.Is(err, apperr.ErrNotRoomOwner) ||
			errors.Is(err, apperr.ErrSeatNotOccupied) ||
			errors.Is(err, apperr.ErrInvalidSeat) {
			return err
		}
		return fmt.Errorf("swapping seats: %w", err)
	}

	players := h.playersWithBots(uint(roomID))

	if botNoOp {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"data": PlayersResponse{Players: players},
		})
	}

	for _, mv := range humanMoves {
		h.broadcastToRoom(uint(roomID), ws.SystemSeatUpdated, map[string]interface{}{
			"roomId":       roomID,
			"userId":       mv.userID,
			"username":     mv.username,
			"avatarUrl":    mv.avatarURL,
			"seat":         mv.seat,
			"team":         mv.team,
			"previousSeat": mv.previousSeat,
		})
	}
	if botMove != nil {
		h.broadcastToRoom(uint(roomID), ws.SystemBotRemoved, map[string]interface{}{
			"roomId": roomID,
			"seat":   botMove.from,
		})
		h.broadcastToRoom(uint(roomID), ws.SystemBotAdded, map[string]interface{}{
			"roomId": roomID,
			"seat":   botMove.to,
			"team":   teamForSeat(botMove.to),
		})
	}
	humansOnly, err := h.repo.FindPlayersByRoomID(uint(roomID))
	if err != nil {
		return fmt.Errorf("fetching players after swap: %w", err)
	}
	h.broadcastRoomSeatSnapshot(uint(roomID), humansOnly)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": PlayersResponse{Players: players},
	})
}

// botOccupiesSeat reports whether a bot is seated at the given seat.
func botOccupiesSeat(repo RoomRepository, roomID uint, seat int) (bool, error) {
	bots, err := repo.FindBotsByRoomID(roomID)
	if err != nil {
		return false, err
	}
	for _, b := range bots {
		if b.Seat == seat {
			return true, nil
		}
	}
	return false, nil
}

type AddBotRequest struct {
	Seat *int `json:"seat"`
}

// AddBot seats a bot on an empty seat of a waiting non-quick-play room, owner only,
// inside a row-locking transaction.
func (h *RoomHandler) AddBot(c echo.Context) error {
	userID, err := auth.GetUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	roomID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.ErrRoomNotFound
	}

	var req AddBotRequest
	if err := c.Bind(&req); err != nil {
		return apperr.ErrBadRequest
	}
	if req.Seat == nil {
		return apperr.ErrInvalidSeat
	}
	seat := *req.Seat

	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		r, err := tx.FindByIDForUpdate(uint(roomID))
		if err != nil {
			return fmt.Errorf("finding room: %w", err)
		}
		if r == nil {
			return apperr.ErrRoomNotFound
		}
		if r.OwnerID != userID {
			return apperr.ErrNotRoomOwner
		}
		if r.Status != "waiting" {
			return apperr.ErrRoomNotWaiting
		}
		if r.IsQuickPlay {
			return apperr.ErrBotsNotAllowed
		}
		if seat < 0 || seat > 3 {
			return apperr.ErrInvalidSeat
		}

		human, err := tx.FindPlayerBySeat(uint(roomID), seat)
		if err != nil {
			return fmt.Errorf("checking seat occupancy: %w", err)
		}
		if human != nil {
			return apperr.ErrSeatTaken
		}
		bots, err := tx.FindBotsByRoomID(uint(roomID))
		if err != nil {
			return fmt.Errorf("checking bot seat occupancy: %w", err)
		}
		for _, b := range bots {
			if b.Seat == seat {
				return apperr.ErrSeatTaken
			}
		}
		if r.PlayerCount+len(bots) >= 4 {
			return apperr.ErrRoomFull
		}

		return tx.AddBot(uint(roomID), seat)
	}); err != nil {
		if errors.Is(err, apperr.ErrRoomNotFound) ||
			errors.Is(err, apperr.ErrNotRoomOwner) ||
			errors.Is(err, apperr.ErrRoomNotWaiting) ||
			errors.Is(err, apperr.ErrBotsNotAllowed) ||
			errors.Is(err, apperr.ErrInvalidSeat) ||
			errors.Is(err, apperr.ErrRoomFull) ||
			errors.Is(err, apperr.ErrSeatTaken) {
			return err
		}
		return fmt.Errorf("adding bot: %w", err)
	}

	h.voidInvitesIfFull(uint(roomID))

	h.broadcastToRoom(uint(roomID), ws.SystemBotAdded, map[string]interface{}{
		"roomId": roomID,
		"seat":   seat,
		"team":   teamForSeat(seat),
	})
	humansOnly, err := h.repo.FindPlayersByRoomID(uint(roomID))
	if err != nil {
		return fmt.Errorf("fetching players after add-bot: %w", err)
	}
	h.broadcastRoomSeatSnapshot(uint(roomID), humansOnly)

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"data": PlayersResponse{Players: h.playersWithBots(uint(roomID))},
	})
}

// RemoveBot unseats the bot at the given seat. Owner-only, waiting rooms only.
func (h *RoomHandler) RemoveBot(c echo.Context) error {
	userID, err := auth.GetUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	roomID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.ErrRoomNotFound
	}
	seat, err := strconv.Atoi(c.Param("seat"))
	if err != nil || seat < 0 || seat > 3 {
		return apperr.ErrInvalidSeat
	}

	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		r, err := tx.FindByIDForUpdate(uint(roomID))
		if err != nil {
			return fmt.Errorf("finding room: %w", err)
		}
		if r == nil {
			return apperr.ErrRoomNotFound
		}
		if r.OwnerID != userID {
			return apperr.ErrNotRoomOwner
		}
		if r.Status != "waiting" {
			return apperr.ErrRoomNotWaiting
		}

		return tx.RemoveBot(uint(roomID), seat)
	}); err != nil {
		if errors.Is(err, apperr.ErrRoomNotFound) ||
			errors.Is(err, apperr.ErrNotRoomOwner) ||
			errors.Is(err, apperr.ErrRoomNotWaiting) ||
			errors.Is(err, apperr.ErrNoBotOnSeat) {
			return err
		}
		return fmt.Errorf("removing bot: %w", err)
	}

	h.broadcastToRoom(uint(roomID), ws.SystemBotRemoved, map[string]interface{}{
		"roomId": roomID,
		"seat":   seat,
	})
	humansOnly, err := h.repo.FindPlayersByRoomID(uint(roomID))
	if err != nil {
		return fmt.Errorf("fetching players after remove-bot: %w", err)
	}
	h.broadcastRoomSeatSnapshot(uint(roomID), humansOnly)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": PlayersResponse{Players: h.playersWithBots(uint(roomID))},
	})
}

// LeaveSeat clears the calling player's seat without removing them from the room. It is
// disallowed in quick-play rooms.
func (h *RoomHandler) LeaveSeat(c echo.Context) error {
	userID, err := auth.GetUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	roomID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.ErrRoomNotFound
	}

	var (
		username     string
		previousSeat int
	)

	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		r, err := tx.FindByID(uint(roomID))
		if err != nil {
			return fmt.Errorf("finding room: %w", err)
		}
		if r == nil {
			return apperr.ErrRoomNotFound
		}
		if r.Status != "waiting" {
			return apperr.ErrRoomNotWaiting
		}
		if r.IsQuickPlay {
			return apperr.ErrQuickPlayLeaveSeatBlocked
		}
		if r.OwnerID == userID {
			return apperr.ErrOwnerCannotLeaveSeat
		}

		player, err := tx.FindPlayerRoom(userID)
		if err != nil {
			return fmt.Errorf("finding player room: %w", err)
		}
		if player == nil || player.RoomID != uint(roomID) {
			return apperr.ErrNotInRoom
		}
		if player.Seat == nil {
			return apperr.ErrNotSeated
		}

		previousSeat = *player.Seat
		username = player.Username
		if err := tx.ClearPlayerSeat(uint(roomID), userID); err != nil {
			return fmt.Errorf("clearing seat: %w", err)
		}
		return nil
	}); err != nil {
		if errors.Is(err, apperr.ErrRoomNotFound) ||
			errors.Is(err, apperr.ErrRoomNotWaiting) ||
			errors.Is(err, apperr.ErrQuickPlayLeaveSeatBlocked) ||
			errors.Is(err, apperr.ErrOwnerCannotLeaveSeat) ||
			errors.Is(err, apperr.ErrNotInRoom) ||
			errors.Is(err, apperr.ErrNotSeated) {
			return err
		}
		return fmt.Errorf("leaving seat: %w", err)
	}

	players, err := h.repo.FindPlayersByRoomID(uint(roomID))
	if err != nil {
		return fmt.Errorf("fetching players after leave-seat: %w", err)
	}

	var avatarURL *string
	for _, p := range players {
		if p.UserID == userID {
			avatarURL = p.AvatarURL
			break
		}
	}

	h.broadcastToRoom(uint(roomID), ws.SystemSeatUpdated, map[string]interface{}{
		"roomId":       roomID,
		"userId":       userID,
		"username":     username,
		"avatarUrl":    avatarURL,
		"seat":         nil,
		"team":         nil,
		"previousSeat": previousSeat,
	})
	h.broadcastRoomSeatSnapshot(uint(roomID), players)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": PlayersResponse{Players: h.playersWithBots(uint(roomID))},
	})
}

func (h *RoomHandler) StartMatch(c echo.Context) error {
	userID, err := auth.GetUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	roomID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.ErrRoomNotFound
	}

	var updatedRoom *Room
	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		room, err := tx.FindByIDForUpdate(uint(roomID))
		if err != nil {
			return fmt.Errorf("finding room: %w", err)
		}
		if room == nil {
			return apperr.ErrRoomNotFound
		}

		if room.Status != "waiting" {
			return apperr.ErrMatchNotStartable
		}

		if room.IsQuickPlay {
			return apperr.ErrMatchNotStartable
		}

		if room.OwnerID != userID {
			return apperr.ErrNotRoomOwner
		}

		players, err := tx.FindPlayersByRoomID(uint(roomID))
		if err != nil {
			return fmt.Errorf("finding room players: %w", err)
		}
		bots, err := tx.FindBotsByRoomID(uint(roomID))
		if err != nil {
			return fmt.Errorf("finding room bots: %w", err)
		}

		for _, p := range players {
			if p.Seat == nil {
				return apperr.ErrNotAllSeated
			}
		}

		var seatCovered [4]bool
		for _, p := range players {
			if p.Seat != nil && *p.Seat >= 0 && *p.Seat <= 3 {
				seatCovered[*p.Seat] = true
			}
		}
		for _, b := range bots {
			if b.Seat >= 0 && b.Seat <= 3 {
				seatCovered[b.Seat] = true
			}
		}
		for _, covered := range seatCovered {
			if !covered {
				return apperr.ErrNotAllSeated
			}
		}

		room.Status = "playing"
		if err := tx.Update(room); err != nil {
			return fmt.Errorf("starting game: %w", err)
		}

		updatedRoom = room
		return nil
	}); err != nil {
		if errors.Is(err, apperr.ErrRoomNotFound) || errors.Is(err, apperr.ErrMatchNotStartable) ||
			errors.Is(err, apperr.ErrNotRoomOwner) || errors.Is(err, apperr.ErrNotAllSeated) {
			return err
		}
		return fmt.Errorf("starting game: %w", err)
	}

	h.presence.Clear(uint(roomID))
	h.invites.VoidRoom(uint(roomID))

	if h.matchStarter != nil {
		players, err := h.repo.FindPlayersByRoomID(uint(roomID))
		if err != nil {
			slog.Error("failed to load players for game start", "roomID", roomID, "error", err)
		} else if bots, berr := h.repo.FindBotsByRoomID(uint(roomID)); berr != nil {
			slog.Error("failed to load bots for game start", "roomID", roomID, "error", berr)
		} else {
			var seatInfo [4]match.PlayerSeatInfo
			for _, p := range players {
				if p.Seat != nil {
					seatInfo[*p.Seat] = match.PlayerSeatInfo{
						UserID:    p.UserID,
						Username:  p.Username,
						AvatarURL: p.AvatarURL,
						Seat:      *p.Seat,
					}
				}
			}
			for _, b := range bots {
				if b.Seat >= 0 && b.Seat <= 3 {
					seatInfo[b.Seat] = match.PlayerSeatInfo{
						Seat:  b.Seat,
						IsBot: true,
					}
				}
			}

			if honorErr := h.honorPrefilterAtStart(uint(roomID), updatedRoom, players); honorErr != nil {
				if !errors.Is(honorErr, apperr.ErrHonorTooLow) && !errors.Is(honorErr, apperr.ErrNewPlayerNotAllowed) {
					slog.Error("failed to read honor for start prefilter", "roomID", roomID, "error", honorErr)
					if uerr := h.repo.UpdateStatus(uint(roomID), "waiting"); uerr != nil {
						slog.Error("failed to revert room status after honor-read failure", "roomID", roomID, "error", uerr)
					}
					updatedRoom.Status = "waiting"
					h.broadcastRoomUpdated(updatedRoom)
				}
				return honorErr
			}

			charged := false
			var chargedIDs []uint
			if updatedRoom.CoinBuyIn > 0 && h.walletService != nil {
				humanIDs := make([]uint, 0, len(players))
				for _, p := range players {
					humanIDs = append(humanIDs, p.UserID)
				}

				balances, berr := h.walletService.GetBalances(humanIDs)
				if berr != nil {
					slog.Error("failed to read balances for start prefilter", "roomID", roomID, "error", berr)
					if uerr := h.repo.UpdateStatus(uint(roomID), "waiting"); uerr != nil {
						slog.Error("failed to revert room status after balance-read failure", "roomID", roomID, "error", uerr)
					}
					updatedRoom.Status = "waiting"
					h.broadcastRoomUpdated(updatedRoom)
					return fmt.Errorf("reading balances for start: %w", berr)
				}

				insolvent := make([]uint, 0, len(humanIDs))
				for _, id := range humanIDs {
					if balances[id] < updatedRoom.CoinBuyIn {
						insolvent = append(insolvent, id)
					}
				}
				if len(insolvent) > 0 {
					h.ejectSeatsAtStart(uint(roomID), updatedRoom, insolventNotices(insolvent, updatedRoom.CoinBuyIn, balances), balances, nil, players)
					return apperr.ErrInsufficientCoins
				}

				if insolventID, chargeErr := h.walletService.ChargeStakes(humanIDs, updatedRoom.CoinBuyIn); chargeErr != nil {
					if errors.Is(chargeErr, apperr.ErrInsufficientCoins) {
						if fresh, ferr := h.walletService.GetBalances([]uint{insolventID}); ferr == nil {
							balances[insolventID] = fresh[insolventID]
						}
						h.ejectSeatsAtStart(uint(roomID), updatedRoom, insolventNotices([]uint{insolventID}, updatedRoom.CoinBuyIn, balances), balances, nil, players)
						return apperr.ErrInsufficientCoins
					}
					if uerr := h.repo.UpdateStatus(uint(roomID), "waiting"); uerr != nil {
						slog.Error("failed to revert room status after charge failure", "roomID", roomID, "error", uerr)
					}
					updatedRoom.Status = "waiting"
					h.broadcastRoomUpdated(updatedRoom)
					return fmt.Errorf("charging stakes: %w", chargeErr)
				}
				charged = true
				chargedIDs = humanIDs
			}

			timerDuration := 0
			if updatedRoom.TimerDurationSeconds != nil {
				timerDuration = *updatedRoom.TimerDurationSeconds
			}
			reconnectWindow := resolveReconnectWindow(updatedRoom.ReconnectWindowSec)
			if serr := h.matchStarter.StartMatch(uint(roomID), updatedRoom.Variant, updatedRoom.MatchMode, seatInfo, updatedRoom.TimerStyle, timerDuration, updatedRoom.OwnerID, reconnectWindow, updatedRoom.CoinBuyIn, updatedRoom.DeclarationsEnabled, updatedRoom.StopAtTarget); serr != nil {
				slog.Error("failed to start game session", "roomID", roomID, "error", serr)
				if charged {
					refund := make(map[uint]int, len(chargedIDs))
					for _, id := range chargedIDs {
						refund[id] = updatedRoom.CoinBuyIn
					}
					if rerr := h.walletService.ApplySettlement(refund); rerr != nil {
						slog.Error("failed to refund stakes after start failure", "roomID", roomID, "error", rerr)
					}
				}
				if uerr := h.repo.UpdateStatus(uint(roomID), "waiting"); uerr != nil {
					slog.Error("failed to revert room status after start failure", "roomID", roomID, "error", uerr)
				}
				updatedRoom.Status = "waiting"
				h.broadcastToRoom(uint(roomID), ws.ErrorMatchStartFailed, map[string]interface{}{
					"roomId":  roomID,
					"message": "Failed to start the game. Please try again.",
				})
				h.broadcastRoomUpdated(updatedRoom)
				return fmt.Errorf("starting match session: %w", serr)
			}
		}
	}

	h.broadcastToRoom(uint(roomID), ws.SystemMatchStarted, map[string]interface{}{
		"roomId": roomID,
	})

	h.broadcastRoomUpdated(updatedRoom)

	return c.JSON(http.StatusOK, map[string]interface{}{"data": updatedRoom})
}

func (h *RoomHandler) QuickPlay(c echo.Context) error {
	userID, err := auth.GetUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	existingRoom, err := h.repo.FindPlayerRoom(userID)
	if err != nil {
		return fmt.Errorf("checking existing room: %w", err)
	}
	if existingRoom != nil {
		return apperr.ErrAlreadyInRoom
	}

	buyIn := 0
	if h.walletService != nil {
		balance, berr := h.walletService.GetBalance(userID)
		if berr != nil {
			return fmt.Errorf("reading balance for quick play bracket: %w", berr)
		}
		buyIn = quickPlayBuyIn(balance)
	}

	var resultRoom *Room
	var assignedSeat int
	var assignedTeam string
	createdNew := false
	var createErr error
	triedRoomIDs := make(map[uint]bool)
	var lastTriedRoomID uint
	for i := 0; i < maxRetries; i++ {
		lastTriedRoomID = 0
		createErr = h.repo.RunInTransaction(func(tx RoomRepository) error {
			available, err := tx.FindQuickPlayRoomExcluding(triedRoomIDs, buyIn)
			if err != nil {
				return fmt.Errorf("finding quick play room: %w", err)
			}

			if available != nil {
				lastTriedRoomID = available.ID
				seat, team, r, err := seatPlayerIntoQuickRoom(tx, available.ID, userID)
				if err != nil {
					return err
				}
				assignedSeat = seat
				assignedTeam = team
				resultRoom = r
				createdNew = false
				return nil
			}

			code, err := generateRoomCode()
			if err != nil {
				return fmt.Errorf("generating room code: %w", err)
			}

			timerDuration := quickPlayTimerDurationSeconds
			newRoom := &Room{
				Name:                 "Quick Play " + code,
				Code:                 code,
				OwnerID:              userID,
				Variant:              "croatia",
				MatchMode:            "501",
				TimerStyle:           "per-move",
				TimerDurationSeconds: &timerDuration,
				IsQuickPlay:          true,
				CoinBuyIn:            buyIn,
				MinHonor:             0,
				AllowNewPlayers:      true,
				DeclarationsEnabled:  true,
				StopAtTarget:         false,
				Status:               "waiting",
				PlayerCount:          1,
			}
			if err := tx.Create(newRoom); err != nil {
				return err
			}
			rp := &RoomPlayer{RoomID: newRoom.ID, UserID: userID}
			if err := tx.AddPlayer(rp); err != nil {
				return fmt.Errorf("adding creator to room players: %w", err)
			}
			seat := 0
			team := teamForSeat(seat)
			if err := tx.UpdatePlayerSeat(newRoom.ID, userID, seat, team); err != nil {
				return fmt.Errorf("auto-seating creator: %w", err)
			}
			assignedSeat = seat
			assignedTeam = team
			resultRoom = newRoom
			createdNew = true
			return nil
		})
		if createErr == nil {
			break
		}
		if errors.Is(createErr, apperr.ErrRoomCodeTaken) || errors.Is(createErr, apperr.ErrRoomNameTaken) {
			continue
		}
		if errors.Is(createErr, apperr.ErrRoomFull) {
			if lastTriedRoomID != 0 {
				triedRoomIDs[lastTriedRoomID] = true
			}
			continue
		}
		if errors.Is(createErr, apperr.ErrAlreadyInRoom) {
			return createErr
		}
		return createErr
	}
	if createErr != nil {
		return createErr
	}

	h.presence.Add(resultRoom.ID, userID)

	if createdNew {
		h.broadcastToAll(ws.SystemRoomCreated, map[string]interface{}{
			"id":                   resultRoom.ID,
			"name":                 resultRoom.Name,
			"code":                 resultRoom.Code,
			"ownerId":              resultRoom.OwnerID,
			"variant":              resultRoom.Variant,
			"matchMode":            resultRoom.MatchMode,
			"timerStyle":           resultRoom.TimerStyle,
			"timerDurationSeconds": resultRoom.TimerDurationSeconds,
			"coinBuyIn":            resultRoom.CoinBuyIn,
			"declarationsEnabled":  resultRoom.DeclarationsEnabled,
			"stopAtTarget":         resultRoom.StopAtTarget,
			"playerCount":          resultRoom.PlayerCount,
			"status":               resultRoom.Status,
			"isQuickPlay":          resultRoom.IsQuickPlay,
		})
	} else {
		h.broadcastRoomUpdated(resultRoom)
	}

	h.broadcastQuickPlayerSeated(resultRoom, userID, assignedSeat, assignedTeam)

	matchStarted, err := h.autoStartIfFull(resultRoom.ID)
	if err != nil {
		return err
	}

	if !matchStarted {
		if createdNew {
			h.startQuickFill(resultRoom.ID, userID)
		} else {
			h.resetQuickFill(resultRoom.ID)
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"room":         resultRoom,
			"seat":         assignedSeat,
			"matchStarted": matchStarted,
		},
	})
}

// QuickJoin seats the caller into a specific quick play room and runs the auto-start
// check, returning the same shape as QuickPlay.
func (h *RoomHandler) QuickJoin(c echo.Context) error {
	userID, err := auth.GetUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	roomID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.ErrRoomNotFound
	}

	existing, err := h.repo.FindPlayerRoom(userID)
	if err != nil {
		return fmt.Errorf("checking existing room: %w", err)
	}
	if existing != nil {
		return apperr.ErrAlreadyInRoom
	}

	callerBuyIn := 0
	if h.walletService != nil {
		balance, berr := h.walletService.GetBalance(userID)
		if berr != nil {
			return fmt.Errorf("reading balance for quick join bracket: %w", berr)
		}
		callerBuyIn = quickPlayBuyIn(balance)
	}

	var resultRoom *Room
	var assignedSeat int
	var assignedTeam string
	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		r, err := tx.FindByIDForUpdate(uint(roomID))
		if err != nil {
			return fmt.Errorf("finding room: %w", err)
		}
		if r == nil {
			return apperr.ErrRoomNotFound
		}
		if !r.IsQuickPlay {
			return apperr.ErrRoomNotQuickPlay
		}
		if r.Status != "waiting" {
			return apperr.ErrRoomNotFound
		}
		if r.CoinBuyIn != callerBuyIn {
			return apperr.ErrQuickPlayBracketMismatch
		}
		if r.PlayerCount >= 4 {
			return apperr.ErrRoomFull
		}
		seat, team, joined, err := seatPlayerIntoQuickRoom(tx, uint(roomID), userID)
		if err != nil {
			return err
		}
		assignedSeat = seat
		assignedTeam = team
		resultRoom = joined
		return nil
	}); err != nil {
		if errors.Is(err, apperr.ErrAlreadyInRoom) || errors.Is(err, apperr.ErrRoomNotFound) ||
			errors.Is(err, apperr.ErrRoomNotQuickPlay) || errors.Is(err, apperr.ErrRoomFull) ||
			errors.Is(err, apperr.ErrQuickPlayBracketMismatch) {
			return err
		}
		return fmt.Errorf("quick joining room: %w", err)
	}

	h.presence.Add(resultRoom.ID, userID)

	h.broadcastRoomUpdated(resultRoom)
	h.broadcastQuickPlayerSeated(resultRoom, userID, assignedSeat, assignedTeam)

	matchStarted, err := h.autoStartIfFull(resultRoom.ID)
	if err != nil {
		return err
	}

	if !matchStarted {
		h.resetQuickFill(resultRoom.ID)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"room":         resultRoom,
			"seat":         assignedSeat,
			"matchStarted": matchStarted,
		},
	})
}

// broadcastQuickPlayerSeated mirrors JoinRoom's broadcasts for a player auto-seated into
// a quick play room, sending each event as a separate ordered message.
func (h *RoomHandler) broadcastQuickPlayerSeated(r *Room, userID uint, seat int, team string) {
	roomPlayers, err := h.repo.FindPlayersByRoomID(r.ID)
	if err != nil {
		slog.Error("quick play: loading players for join broadcast", "roomID", r.ID, "error", err)
		return
	}
	var username string
	var avatarURL *string
	for _, p := range roomPlayers {
		if p.UserID == userID {
			username = p.Username
			avatarURL = p.AvatarURL
			break
		}
	}
	userIDs := make([]uint, 0, len(roomPlayers))
	for _, p := range roomPlayers {
		userIDs = append(userIDs, p.UserID)
	}
	h.broadcastToUsers(userIDs, ws.SystemPlayerJoined, map[string]interface{}{
		"roomId":      r.ID,
		"userId":      userID,
		"username":    username,
		"avatarUrl":   avatarURL,
		"playerCount": r.PlayerCount,
	})
	h.broadcastToUsers(userIDs, ws.SystemSeatUpdated, map[string]interface{}{
		"roomId":       r.ID,
		"userId":       userID,
		"username":     username,
		"avatarUrl":    avatarURL,
		"seat":         seat,
		"team":         team,
		"previousSeat": nil,
	})
	h.broadcastRoomSeatSnapshot(r.ID, roomPlayers)
}

// pickFirstEmptySeat returns the lowest seat index 0..3 currently unoccupied
// in the room, or an error if every seat is taken.
func pickFirstEmptySeat(tx RoomRepository, roomID uint) (int, error) {
	for seat := 0; seat < 4; seat++ {
		existing, err := tx.FindPlayerBySeat(roomID, seat)
		if err != nil {
			return 0, fmt.Errorf("checking seat %d occupancy: %w", seat, err)
		}
		if existing == nil {
			return seat, nil
		}
	}
	return 0, apperr.ErrRoomFull
}

// resolveReconnectWindow returns the reconnect window in seconds,
// defaulting to 120 if the room has no explicit setting.
func resolveReconnectWindow(roomSetting *int) int {
	if roomSetting != nil {
		return *roomSetting
	}
	return 120
}

func generateRoomCode() (string, error) {
	result := make([]byte, codeLength)
	max := big.NewInt(int64(len(codeChars)))
	for i := 0; i < codeLength; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generating random number: %w", err)
		}
		result[i] = codeChars[n.Int64()]
	}
	return string(result), nil
}
