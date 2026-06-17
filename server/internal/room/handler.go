package room

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/auth"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/ws"
)

var (
	validVariants    = map[string]bool{"bitola": true}
	validMatchModes  = map[string]bool{"1001": true, "501": true}
	validTimerStyles = map[string]bool{"relaxed": true, "per-move": true}
	validStatuses    = map[string]bool{"waiting": true, "playing": true, "completed": true}
)

const (
	codeChars  = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	codeLength = 6
	maxRetries = 3
)

type CreateRoomRequest struct {
	Name                 string `json:"name"`
	Variant              string `json:"variant"`
	MatchMode            string `json:"matchMode"`
	TimerStyle           string `json:"timerStyle"`
	TimerDurationSeconds *int   `json:"timerDurationSeconds"`
	ReconnectWindowSec   *int   `json:"reconnectWindowSec"`
}

// MatchStarter is the interface the room handler uses to start a live match.
type MatchStarter interface {
	StartMatch(roomID uint, variant string, matchMode string, players [4]match.PlayerSeatInfo, timerStyle string, timerDurationSec int, ownerID uint, reconnectWindowSec int) error
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
	repo         RoomRepository
	matchStarter MatchStarter
	hub          Broadcaster
	presence     *PresenceRegistry
}

func NewRoomHandler(repo RoomRepository, matchStarter MatchStarter, hub Broadcaster, presence *PresenceRegistry) *RoomHandler {
	// Default to a private registry when none is injected (keeps test setups
	// that don't exercise presence working without threading a registry).
	if presence == nil {
		presence = NewPresenceRegistry()
	}
	return &RoomHandler{repo: repo, matchStarter: matchStarter, hub: hub, presence: presence}
}

// broadcastToRoom sends a WebSocket message to all players in a room.
// Broadcast is best-effort — errors are logged but never fail the HTTP response.
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

// mergeBotPlayers appends synthetic bot entries to a humans-only players
// slice. This is the single place bots enter a players array — every payload
// path that serializes players must route through it. The wire signature of
// a bot is {id:0, userId:0, username:"", seat, team, isBot:true}; team
// derives from seat parity exactly like humans.
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
			RoomID: b.RoomID,
			Seat:   &seat,
			Team:   &team,
			IsBot:  true,
			// Real seat time from the room_bots row — without it the wire
			// carries the zero time.Time ("0001-01-01T00:00:00Z").
			CreatedAt: b.CreatedAt,
		})
	}
	return out
}

// playersWithBots loads the room's humans + bots and returns the merged wire
// players array. Broadcast-path errors are logged, not returned — same
// best-effort contract as the other broadcast helpers.
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
	return mergeBotPlayers(players, bots)
}

// roomLifecyclePayload builds the WS payload shared by `system:room_created`
// and `system:room_updated`. Ensures `ownerUsername` is hydrated so the lobby
// grid can render host avatars without an extra round-trip per row, embeds
// `players` so seat chips render correctly the instant the card appears, and
// always carries `createdAt`/`updatedAt` so the client's <RelativeTime>
// component has a valid ISO to format.
//
// r.Players, when pre-populated by the caller, must be the HUMANS-ONLY slice
// (every call site passes FindPlayersByRoomID output) — bots are merged here
// so the payload always carries the full seat picture.
func (h *RoomHandler) roomLifecyclePayload(r *Room) map[string]any {
	if r.OwnerUsername == "" {
		if err := h.repo.LoadOwnerUsernames([]*Room{r}); err != nil {
			slog.Error("broadcast: failed to load owner username", "roomID", r.ID, "error", err)
		}
	}
	// Always include `players` — even an empty slice — so the client can rely
	// on the field's presence and never end up with `undefined` seat chips on
	// a freshly-broadcast room.
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
		"players":              players,
		"variant":              r.Variant,
		"matchMode":            r.MatchMode,
		"timerStyle":           r.TimerStyle,
		"timerDurationSeconds": r.TimerDurationSeconds,
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

// broadcastRoomSeatSnapshot pushes a system:room_updated event to every
// connected client with the freshly-fetched players[] so lobby grid seat
// chips stay in sync after seat changes. Without this, system:seat_updated
// (which only fans out to room participants) leaves third-party lobby
// watchers showing stale empty chips.
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

	// Reject if the user is already in an active room (mirrors JoinRoom /
	// QuickPlay / QuickJoin). Without this guard a user already seated in a
	// room — e.g. a second device, or a client that drifted onto the lobby —
	// could spin up a brand-new room and orphan the old one (observed in
	// production as duplicate/ownerless rooms).
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
			// Auto-seat the creator at seat 0. With the 4-player room cap an
			// unseated owner could be locked out if 3 invitees took the open
			// seats first; combined with the owner-cannot-leave-seat rule
			// this guarantees the room is always startable from creation.
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

	// The creator is present from the moment the room exists — without this the
	// reopened-room Start gate (all seated humans present) would block the owner
	// on the very first match.
	h.presence.Add(room.ID, userID)

	// Broadcast system:room_created to all connected clients (lobby-wide).
	// roomLifecyclePayload also populates room.OwnerUsername so the JSON
	// response immediately below carries it.
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

	// Hydrate `ownerUsername` + `players` on each row via two batch queries
	// so the lobby grid renders host avatars + seat chips in a single fetch
	// (no N+1 per visible card).
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
	Room    *Room        `json:"room"`
	Players []RoomPlayer `json:"players"`
	// ReturnedUserIds lists the seated/joined users currently "present" in a
	// reopened room (returned via "Return to room" or freshly joined), as
	// opposed to ex-players still lingering on the match result dialog. Drives
	// the RoomPage "waiting to return" state and the owner Start gate.
	ReturnedUserIds []uint `json:"returnedUserIds"`
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

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": RoomDetailResponse{
			Room:            room,
			Players:         mergeBotPlayers(players, bots),
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

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": RoomDetailResponse{
			Room:            room,
			Players:         mergeBotPlayers(players, bots),
			ReturnedUserIds: h.presence.Present(room.ID),
		},
	})
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

	if room.PlayerCount >= 4 {
		return apperr.ErrRoomFull
	}

	// Bot-covered seats count toward capacity: every member must be able to
	// claim a seat eventually (humans ≤ seated humans + free seats), so a
	// room where humans + bots already cover all four seats is full for
	// joiners even though PlayerCount is below 4.
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

	// Mark the fresh joiner present so they count toward the reopened-room
	// Start gate (and seed presence for first-ever joins, which is harmless).
	h.presence.Add(uint(roomID), userID)

	// Broadcast system:player_joined to room participants
	players, broadcastErr := h.repo.FindPlayersByRoomID(uint(roomID))
	if broadcastErr == nil {
		var username string
		for _, p := range players {
			if p.UserID == userID {
				username = p.Username
				break
			}
		}
		userIDs := make([]uint, 0, len(players))
		for _, p := range players {
			userIDs = append(userIDs, p.UserID)
		}
		h.broadcastToUsers(userIDs, ws.SystemPlayerJoined, map[string]interface{}{
			"roomId":      roomID,
			"userId":      userID,
			"username":    username,
			"playerCount": updatedRoom.PlayerCount,
		})
	}

	// Broadcast system:room_updated to lobby browse page
	h.broadcastRoomUpdated(updatedRoom)

	return c.JSON(http.StatusOK, map[string]interface{}{"data": updatedRoom})
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

	// Capture the leaving player's username before the transaction removes them
	var leavingUsername string
	prePlayers, _ := h.repo.FindPlayersByRoomID(uint(roomID))
	for _, p := range prePlayers {
		if p.UserID == userID {
			leavingUsername = p.Username
			break
		}
	}

	var newOwnerID *uint
	var roomClosed bool
	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		// Story 8.5-1 AC3: row-lock the room INSIDE the tx so the status check
		// is serialized against any concurrent auto-start tx that flips status
		// to "playing". FindByIDForUpdate issues SELECT ... FOR UPDATE under
		// the hood; without it, default READ COMMITTED isolation would let
		// both tx read status="waiting" and both commit.
		freshRoom, err := tx.FindByIDForUpdate(uint(roomID))
		if err != nil {
			return fmt.Errorf("re-fetching room for leave gate: %w", err)
		}
		if freshRoom == nil {
			return apperr.ErrRoomNotFound
		}
		// Only block leaves while a match is actively in progress. Allow leaves
		// on "completed" rooms so post-match unmount auto-leave
		// (room-page unmount cleanup) does not log spurious 409s — and a
		// manual click on Leave for a completed match does what the user
		// expects.
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
			// Re-fetch room inside tx to get current state after decrement
			freshRoom, err := tx.FindByID(uint(roomID))
			if err != nil {
				return fmt.Errorf("re-fetching room: %w", err)
			}
			players, err := tx.FindPlayersByRoomID(uint(roomID))
			if err != nil {
				return fmt.Errorf("finding remaining players: %w", err)
			}
			// Promote the first SEATED human. An unseated member must never
			// inherit ownership: they can't start the match, and with bots
			// filling the remaining seats the room would limp on as a dead
			// "unseated owner + bots" shell (observed in production). If no
			// seated human remains, close the room for everyone — bots can't
			// sustain a room on their own.
			var nextOwner *RoomPlayer
			for i := range players {
				if players[i].Seat != nil {
					nextOwner = &players[i]
					break
				}
			}
			if nextOwner != nil {
				freshRoom.OwnerID = nextOwner.UserID
				newOwnerID = &nextOwner.UserID
				if err := tx.Update(freshRoom); err != nil {
					return fmt.Errorf("transferring room ownership: %w", err)
				}
			} else {
				// No seated human left (only unseated humans and/or bots):
				// close the room and evict the stragglers + bots so nobody is
				// stranded as a member of a dead room and it can't be reopened
				// into a broken, owner-less state.
				freshRoom.Status = "completed"
				if err := tx.Update(freshRoom); err != nil {
					return fmt.Errorf("closing ownerless room: %w", err)
				}
				for i := range players {
					if err := tx.RemovePlayer(uint(roomID), players[i].UserID); err != nil {
						return fmt.Errorf("evicting unseated member on close: %w", err)
					}
				}
				bots, err := tx.FindBotsByRoomID(uint(roomID))
				if err != nil {
					return fmt.Errorf("finding bots on close: %w", err)
				}
				for i := range bots {
					if err := tx.RemoveBot(uint(roomID), bots[i].Seat); err != nil {
						return fmt.Errorf("clearing bot on close: %w", err)
					}
				}
				roomClosed = true
			}
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

	// Drop the leaver's presence. Remove auto-clears the room's whole presence
	// entry once the last present user leaves (empty-room close).
	h.presence.Remove(uint(roomID), userID)
	// When the owner-leave closed the room (no seated human remained), wipe the
	// whole presence entry — the evicted stragglers are no longer members.
	if roomClosed {
		h.presence.Clear(uint(roomID))
	}

	// Broadcast system:player_left to remaining room participants (not the leaving player)
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

	// Broadcast system:room_updated to ALL lobby browsers — even when the
	// room emptied out and was flipped to "completed". Without this, a
	// lobby grid that received the room_created event has no way to learn
	// the room closed, so the stale tile lingers forever.
	if postErr == nil && postRoom != nil {
		h.broadcastRoomUpdated(postRoom)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"data": map[string]string{"message": "left room"}})
}

// ReturnToRoom reopens a finished room so the same group can play another match
// without recreating it. Ending a match flips the room to "completed" and tears
// down its live session, but every room_players row (with its seat/team)
// survives — so a returner reclaims their original seat for free and no one else
// can take it (seat-selection already blocks occupied seats).
//
// The first caller flips "completed" -> "waiting" and clears any bots left over
// from the previous match; a later/concurrent call when the room is already
// "waiting" is an idempotent no-op success. Only a current member may reopen the
// room, which inherently bars a player the owner kicked (or who left)
// post-match. This is v1 — there is deliberately no presence layer; see
// deferred-work.md (v2) for the "waiting to return" display and start-gating on
// every seated human being present.
func (h *RoomHandler) ReturnToRoom(c echo.Context) error {
	userID, err := auth.GetUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	roomID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.ErrRoomNotFound
	}

	// Reject non-members up front: a kicked/left player's room_players row is
	// gone, so they can neither reopen nor re-enter the room.
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

	// clearedSeats records bot seats removed during the reopen so we can fan out
	// system:bot_removed after the tx commits — broadcasts are best-effort and
	// must never run inside the transaction. reopened is false on the idempotent
	// "already waiting" path so we don't re-broadcast a redundant reopen (the
	// first returner already told the lobby).
	var clearedSeats []int
	var reopened bool
	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		// Row-lock the room so concurrent first-returns (all four clients see the
		// result dialog at the same instant) serialize: only the first flips
		// status, the rest fall through the "waiting" no-op below.
		freshRoom, err := tx.FindByIDForUpdate(uint(roomID))
		if err != nil {
			return fmt.Errorf("re-fetching room for return: %w", err)
		}
		if freshRoom == nil {
			return apperr.ErrRoomNotFound
		}
		switch freshRoom.Status {
		case "waiting":
			return nil // already reopened — idempotent
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
			// "playing": a match is live; there is nothing to reopen.
			return apperr.ErrMatchAlreadyStarted
		}
	}); err != nil {
		if errors.Is(err, apperr.ErrRoomNotFound) ||
			errors.Is(err, apperr.ErrMatchAlreadyStarted) {
			return err
		}
		return fmt.Errorf("returning to room: %w", err)
	}

	// Post-commit, best-effort broadcasts — only when THIS call performed the
	// reopen. An idempotent "already waiting" return must stay silent: the first
	// returner already cleared bots and refreshed the lobby, so re-broadcasting
	// would fan out a redundant room_updated per remaining click.
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

	// Mark the returner present and announce it to the room — on EVERY successful
	// return (member), not just the reopen, so each returner flips their own
	// "waiting to return" seat to present and the owner Start gate can re-evaluate.
	h.presence.Add(uint(roomID), userID)
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

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": RoomDetailResponse{
			Room:            postRoom,
			Players:         mergeBotPlayers(players, bots),
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
		// Row-lock the room so seat takes serialize against the bot endpoints
		// (AddBot/RemoveBot/SwapSeats) and the start transition — no DB
		// constraint spans room_players.seat/room_bots.seat, so without the
		// lock a human and a bot can race onto the same seat.
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

		// Check if seat is already taken
		existing, err := tx.FindPlayerBySeat(uint(roomID), seat)
		if err != nil {
			return fmt.Errorf("checking seat occupancy: %w", err)
		}
		if existing != nil {
			if existing.UserID == userID {
				// Player already in this seat — no-op
				return nil
			}
			return apperr.ErrSeatTaken
		}
		// Bot seats read as OCCUPIED for self-seating — only the owner's swap
		// flow may displace a bot.
		if taken, err := botOccupiesSeat(tx, uint(roomID), seat); err != nil {
			return fmt.Errorf("checking bot seat occupancy: %w", err)
		} else if taken {
			return apperr.ErrSeatTaken
		}

		// Check if player is in this room and has an existing seat to clear
		player, err := tx.FindPlayerRoom(userID)
		if err != nil {
			return fmt.Errorf("finding player room: %w", err)
		}
		if player == nil || player.RoomID != uint(roomID) {
			return apperr.ErrNotInRoom
		}

		// Capture previous seat before clearing
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

	// Broadcast system:seat_updated to room participants
	if seatChanged {
		var username string
		for _, p := range players {
			if p.UserID == userID {
				username = p.Username
				break
			}
		}
		seatPayload := map[string]interface{}{
			"roomId":       roomID,
			"userId":       userID,
			"username":     username,
			"seat":         seat,
			"team":         team,
			"previousSeat": previousSeat,
		}
		h.broadcastToRoom(uint(roomID), ws.SystemSeatUpdated, seatPayload)
		h.broadcastRoomSeatSnapshot(uint(roomID), players)
	}

	// Check if Quick Play room should auto-start now that a seat was taken.
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

// startAutoStartedMatch invokes matchStarter.StartMatch for an auto-start path
// that has already flipped room.Status to "playing" inside its tx. Returns
// nil when no matchStarter is wired (test setups skip this) or when StartMatch
// succeeds. The caller is responsible for reverting the status flip when this
// returns a non-nil error (Story 8.5-1 AC2).
func (h *RoomHandler) startAutoStartedMatch(autoStartRoom *Room, players []RoomPlayer) error {
	if h.matchStarter == nil {
		return nil
	}
	var seatInfo [4]match.PlayerSeatInfo
	for _, p := range players {
		if p.Seat != nil {
			seatInfo[*p.Seat] = match.PlayerSeatInfo{
				UserID:   p.UserID,
				Username: p.Username,
				Seat:     *p.Seat,
			}
		}
	}
	timerDuration := 0
	if autoStartRoom.TimerDurationSeconds != nil {
		timerDuration = *autoStartRoom.TimerDurationSeconds
	}
	reconnectWindow := resolveReconnectWindow(autoStartRoom.ReconnectWindowSec)
	return h.matchStarter.StartMatch(autoStartRoom.ID, autoStartRoom.Variant, autoStartRoom.MatchMode, seatInfo, autoStartRoom.TimerStyle, timerDuration, autoStartRoom.OwnerID, reconnectWindow)
}

// revertAutoStart compensates for a failed matchStarter.StartMatch: it flips
// the room status back to "waiting" and broadcasts error:match_start_failed
// to the four would-be participants so their clients keep them on the
// room-lobby page instead of navigating to a non-existent /game/{id}.
// Story 8.5-1 AC2.
//
// `players` may be nil — revertAutoStart will re-fetch from the room state
// so the four participants get the failure broadcast even when the caller
// failed to load them.
func (h *RoomHandler) revertAutoStart(roomID uint, autoStartRoom *Room, players []RoomPlayer) {
	revertErr := h.repo.RunInTransaction(func(tx RoomRepository) error {
		r, err := tx.FindByIDForUpdate(roomID)
		if err != nil {
			return fmt.Errorf("re-fetching room for status revert: %w", err)
		}
		if r == nil {
			return nil
		}
		// Idempotency: only revert if the room is still in the "playing"
		// state we flipped it into. A concurrent code path may have already
		// transitioned the room to "completed"/"abandoned" and we must not
		// resurrect it back to "waiting".
		if r.Status != "playing" {
			autoStartRoom.Status = r.Status
			return nil
		}
		r.Status = "waiting"
		if err := tx.Update(r); err != nil {
			return fmt.Errorf("reverting room status to waiting: %w", err)
		}
		// Update the caller-visible Room so the subsequent broadcast carries
		// the reverted status.
		autoStartRoom.Status = "waiting"
		return nil
	})
	if revertErr != nil {
		// Bail out on revert-tx failure: room is stuck in "playing" with no
		// live session, broadcasting error:match_start_failed AND telling
		// clients to stay on the room-lobby page would just compound the
		// problem (every subsequent action rejects on status != "waiting").
		// Logging is the best we can do here; a follow-up health check or
		// admin sweep will need to clean the row up.
		slog.Error("failed to revert auto-start status flip; aborting failure broadcast", "roomID", roomID, "error", revertErr)
		return
	}

	// If the caller didn't supply a players slice (e.g. their own
	// FindPlayersByRoomID failed), re-fetch so the four participants still
	// receive the failure broadcast and don't silently stall.
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

	// Tell lobby browse pages that the room is back to "waiting" so their
	// row state matches the reverted DB row.
	if autoStartRoom.Status == "waiting" {
		h.broadcastRoomUpdated(autoStartRoom)
	}
}

// seatPlayerIntoQuickRoom adds the player to an existing quick play room and
// auto-assigns the lowest empty seat. It MUST be called inside a transaction.
// Returns the assigned seat/team and a fresh copy of the room. Shared by the
// QuickPlay "found existing room" branch and the QuickJoin handler.
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

// autoStartIfFull row-locks the room and, when it is a quick play room in
// "waiting" status with all four seats filled, flips it to "playing" and
// starts the game session. On a successful start it broadcasts
// system:match_started to room participants and system:room_updated to the
// lobby and returns true. If the session fails to start it reverts the status
// flip (Story 8.5-1 AC2) and returns false. Returns false (no error) when the
// room is not yet ready to start. This is the single source of truth for the
// auto-start transition shared by SelectSeat, QuickPlay, and QuickJoin.
func (h *RoomHandler) autoStartIfFull(roomID uint) (bool, error) {
	matchStarted := false
	var autoStartRoom *Room
	var autoStartPlayers []RoomPlayer
	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		// Story 8.5-1 AC3 + P1: row-lock the room AND re-fetch players INSIDE
		// the auto-start tx so a concurrent LeaveRoom committing between the
		// seat-update tx and this tx can never leak a departed player into the
		// rules-engine seat snapshot.
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
		seatedCount := 0
		for _, p := range freshPlayers {
			if p.Seat != nil {
				seatedCount++
			}
		}
		if seatedCount < 4 {
			return nil
		}
		r.Status = "playing"
		if err := tx.Update(r); err != nil {
			return fmt.Errorf("auto-starting quick play room: %w", err)
		}
		matchStarted = true
		autoStartRoom = r
		autoStartPlayers = freshPlayers
		return nil
	}); err != nil {
		return false, fmt.Errorf("auto-start check: %w", err)
	}

	if matchStarted && autoStartRoom != nil {
		// Story 8.5-1 AC2: gate system:match_started AND the playing-status
		// broadcast on matchStarter.StartMatch success. On failure, revert the
		// status flip so the room is not stranded in "playing" with no live
		// session and tell the four would-be participants to stay put.
		startErr := h.startAutoStartedMatch(autoStartRoom, autoStartPlayers)
		if startErr != nil {
			slog.Error("failed to start auto-started game session", "roomID", roomID, "error", startErr)
			h.revertAutoStart(roomID, autoStartRoom, autoStartPlayers)
			matchStarted = false
		} else {
			// Match is live — clear presence (quick-play, but keep parity with
			// the manual StartMatch path).
			h.presence.Clear(roomID)
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

	// Capture the kicked player's username before the transaction removes them
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

	// If the post-tx re-fetch returned nil (e.g. concurrent room cleanup), bail
	// out of the broadcast/response branches that need PlayerCount rather than
	// dereferencing a nil pointer.
	if postRoom == nil {
		return apperr.ErrRoomNotFound
	}

	// Drop the kicked player's presence so they no longer count toward the Start
	// gate (their /return is already rejected as a non-member — see ReturnToRoom).
	h.presence.Remove(uint(roomID), req.UserID)

	// Broadcast: kicked user gets system:room_kicked
	h.broadcastToUsers([]uint{req.UserID}, ws.SystemRoomKicked, ws.RoomKickedPayload{
		RoomID: uint(roomID),
		Reason: "kicked_by_owner",
	})

	// Broadcast: remaining members get system:player_left
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

		// Broadcast: lobby browse page gets system:room_updated
		h.broadcastRoomUpdated(postRoom)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{"playerCount": postRoom.PlayerCount},
	})
}

type TransferOwnershipRequest struct {
	UserID uint `json:"userId"`
}

// TransferOwnership reassigns room ownership from the current owner to a
// seated room member. Restricted to non-self seated targets; an unseated
// promotion would let the new owner immediately get stuck (4-seat cap with
// no spot to take). All clients converge on the new owner via a single
// system:room_owner_changed broadcast plus the lobby system:room_updated.
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

	// Broadcast: every room member converges on the new owner. Lobby browse
	// page also gets system:room_updated so the room card's "Hosted by …"
	// stays accurate.
	h.broadcastToRoom(uint(roomID), ws.SystemRoomOwnerChanged, map[string]interface{}{
		"roomId":           roomID,
		"newOwnerId":       postRoom.OwnerID,
		"newOwnerUsername": newOwnerName,
		"previousOwnerId":  previousOwnerID,
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
		seat         int
		team         string
		previousSeat int
	}
	// humanMoves collects the seat_updated broadcasts owed to HUMAN
	// participants (0, 1, or 2 entries). botMove, when set, is a bot
	// relocation broadcast as bot_removed{from} + bot_added{to}.
	var humanMoves []swapped
	type botRelocation struct {
		from int
		to   int
	}
	var botMove *botRelocation
	// botNoOp marks a bot ↔ bot swap: identity is seat-derived, so swapping
	// two bots is observably nothing — success with no state change and no
	// broadcasts.
	var botNoOp bool

	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		// Row-lock the room so bot-involved rearrangements serialize against
		// concurrent seat takes, add-bot calls, and the start transition
		// (story-8.5-1 pattern).
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

		// bot ↔ bot: observable no-op (identity is seat-derived).
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
				seat:         to,
				team:         team,
				previousSeat: from,
			})
			return nil
		}

		// Exactly one bot involved → relocate it to the opposite seat. If a
		// human occupies that seat they move to the bot's seat in the same
		// transaction (human ↔ bot swap); otherwise the bot moves to the
		// empty seat.
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

		// Humans only from here on — original swap / move-to-empty semantics.
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

	// bot ↔ bot: success with no state change and no broadcasts.
	if botNoOp {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"data": PlayersResponse{Players: players},
		})
	}

	// Broadcast system:seat_updated for each HUMAN move, then the bot
	// relocation as bot_removed{from} + bot_added{to}. Multi-event sequences
	// are sent as separate messages, never batched.
	for _, mv := range humanMoves {
		h.broadcastToRoom(uint(roomID), ws.SystemSeatUpdated, map[string]interface{}{
			"roomId":       roomID,
			"userId":       mv.userID,
			"username":     mv.username,
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
	// Single snapshot broadcast after the per-room events so lobby viewers
	// see the final state in one cache update, not intermediate ones.
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

// AddBot seats a bot on an empty seat of a waiting room. Owner-only; rejected
// for quick-play rooms (they auto-start on 4 humans). Runs inside a
// row-locking transaction so concurrent seat takes, double add-bot calls, and
// the start transition serialize — the unique (room_id, seat) index is the
// backstop for anything that slips through.
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
		// Seat-range check sits after the ownership/state gates (KickPlayer
		// order): a non-owner probing with a junk seat gets NOT_ROOM_OWNER,
		// not INVALID_SEAT.
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
		// Every member must keep a claimable seat: humans + bots may never
		// exceed the four seats, or unseated members would be stranded in a
		// waiting room they can neither sit in nor start from (JoinRoom
		// enforces the same invariant from the joiner's side).
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

	// Broadcast after commit: bot_added to room participants, then the lobby
	// seat snapshot so browse-page seat chips refresh. Separate ordered
	// messages, never batched.
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

// LeaveSeat clears the calling player's seat without removing them from the
// room. It is the inverse of SelectSeat for the seated state — the player
// stays a room member (player_count unchanged) but is no longer in a seat.
// Disallowed in quick-play rooms, where the seating loop is meant to fill
// instantly and start a game; seated players must instead leave the room.
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
		// Owner stays seated by design: with the 4-player room cap, an unseated
		// owner could be locked out of re-seating once others fill the seats.
		// Owners that want to leave the table use LeaveRoom (which transfers
		// ownership) instead.
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

	// Broadcast a system:seat_updated with seat=null/team=null so other clients
	// remove the player from the seat tile but keep them in the room roster.
	h.broadcastToRoom(uint(roomID), ws.SystemSeatUpdated, map[string]interface{}{
		"roomId":       roomID,
		"userId":       userID,
		"username":     username,
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
		// Row-lock the room so the start transition serializes against seat
		// takes and the bot endpoints — the coverage check below must see a
		// settled seat picture.
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

		// Every room member must hold a seat — pre-bots this was implied by
		// the 4-seated-humans gate (capacity 4 ⇒ all members seated); with
		// bots covering seats it must be explicit, or unseated members would
		// receive system:match_started with no seat, no userToRoom entry and
		// no state. The owner kicks lingering unseated members first.
		for _, p := range players {
			if p.Seat == nil {
				return apperr.ErrNotAllSeated
			}
		}

		// Every seat 0–3 must be covered by exactly one human or bot. The
		// seating paths guarantee exclusivity (a bot can't take a human seat
		// and vice versa), so coverage is the only check needed.
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

	// Match is live — presence is no longer meaningful; clear it so the next
	// reopen starts from an empty "who's back" set.
	h.presence.Clear(uint(roomID))

	// Start match via match starter
	if h.matchStarter != nil {
		players, err := h.repo.FindPlayersByRoomID(uint(roomID))
		if err != nil {
			slog.Error("failed to load players for game start", "roomID", roomID, "error", err)
		} else if bots, berr := h.repo.FindBotsByRoomID(uint(roomID)); berr != nil {
			// Proceeding with bots=nil would leave bot seats zero-valued in
			// seatInfo ({Seat:0, IsBot:false}), clobbering seat 0's human in
			// the manager's seat loop — skip the start exactly like the
			// players-load failure above.
			slog.Error("failed to load bots for game start", "roomID", roomID, "error", berr)
		} else {
			var seatInfo [4]match.PlayerSeatInfo
			for _, p := range players {
				if p.Seat != nil {
					seatInfo[*p.Seat] = match.PlayerSeatInfo{
						UserID:   p.UserID,
						Username: p.Username,
						Seat:     *p.Seat,
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
			timerDuration := 0
			if updatedRoom.TimerDurationSeconds != nil {
				timerDuration = *updatedRoom.TimerDurationSeconds
			}
			reconnectWindow := resolveReconnectWindow(updatedRoom.ReconnectWindowSec)
			if err := h.matchStarter.StartMatch(uint(roomID), updatedRoom.Variant, updatedRoom.MatchMode, seatInfo, updatedRoom.TimerStyle, timerDuration, updatedRoom.OwnerID, reconnectWindow); err != nil {
				slog.Error("failed to start game session", "roomID", roomID, "error", err)
			}
		}
	}

	// Broadcast system:match_started to all room participants
	h.broadcastToRoom(uint(roomID), ws.SystemMatchStarted, map[string]interface{}{
		"roomId": roomID,
	})

	// Broadcast system:room_updated to lobby browse page
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

	var resultRoom *Room
	var assignedSeat int
	var assignedTeam string
	createdNew := false
	var createErr error
	// Story 8.5-1 AC5: track room IDs whose join tx failed in this retry loop
	// so the next iteration's FindQuickPlayRoom skips them. Without this, a
	// drifted room (player_count<4 but every seat taken) would be returned
	// every iteration, the inner pickFirstEmptySeat would raise ErrRoomFull,
	// the tx would roll back leaving the drift unchanged, and the loop would
	// burn its retry budget on the same row before surfacing the opaque
	// ErrRoomFull AC5 promised never to surface.
	triedRoomIDs := make(map[uint]bool)
	var lastTriedRoomID uint
	for i := 0; i < maxRetries; i++ {
		lastTriedRoomID = 0
		createErr = h.repo.RunInTransaction(func(tx RoomRepository) error {
			available, err := tx.FindQuickPlayRoomExcluding(triedRoomIDs)
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

			newRoom := &Room{
				Name:        "Quick Play " + code,
				Code:        code,
				OwnerID:     userID,
				Variant:     "bitola",
				MatchMode:   "1001",
				TimerStyle:  "relaxed",
				IsQuickPlay: true,
				Status:      "waiting",
				PlayerCount: 1,
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
		// Story 8.5-1 AC5 (D29 symptom): pickFirstEmptySeat raises ErrRoomFull
		// when the player_count denormalized counter says the room has free
		// seats but every seat row is occupied. Mark the drifted room as
		// tried and retry — exclusion guarantees the next iteration either
		// picks a different room or falls through to the create-new-room
		// branch, satisfying AC5's "successful join into a different/new
		// room — never an opaque 5xx" promise.
		// TODO: drift root-cause is D29 (Phase 2) — this only treats the symptom.
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

	// Mark present (quick-play rooms auto-start so the gate never applies, but
	// keeps the registry consistent for the room-detail payload).
	h.presence.Add(resultRoom.ID, userID)

	// Broadcast lobby-wide events for QuickPlay
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
			"playerCount":          resultRoom.PlayerCount,
			"status":               resultRoom.Status,
			"isQuickPlay":          resultRoom.IsQuickPlay,
		})
	} else {
		// Joined an existing room — broadcast updated player count
		h.broadcastRoomUpdated(resultRoom)
	}

	// Mirror JoinRoom's broadcasts so existing room members see the QuickPlay
	// joiner appear in their seat.
	h.broadcastQuickPlayerSeated(resultRoom, userID, assignedSeat, assignedTeam)

	// Auto-start when all four seats are filled (4th joiner closes the room).
	matchStarted, err := h.autoStartIfFull(resultRoom.ID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"room":         resultRoom,
			"seat":         assignedSeat,
			"matchStarted": matchStarted,
		},
	})
}

// QuickJoin seats the caller into a SPECIFIC quick play room (the one they
// clicked in the lobby grid) and runs the auto-start check, returning the same
// {room, seat, matchStarted} shape as QuickPlay. Custom (non quick-play) rooms
// are rejected — they go through JoinRoom + manual seat selection. The frontend
// ports the joiner to the matchmaking screen rather than the in-room seat grid.
func (h *RoomHandler) QuickJoin(c echo.Context) error {
	userID, err := auth.GetUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	roomID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.ErrRoomNotFound
	}

	// Reject if the user is already in a room (mirrors QuickPlay).
	existing, err := h.repo.FindPlayerRoom(userID)
	if err != nil {
		return fmt.Errorf("checking existing room: %w", err)
	}
	if existing != nil {
		return apperr.ErrAlreadyInRoom
	}

	var resultRoom *Room
	var assignedSeat int
	var assignedTeam string
	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		// Row-lock the room to serialize against concurrent joiners and the
		// auto-start transition.
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
		// Match JoinRoom's convention: a non-waiting room reads as not found.
		if r.Status != "waiting" {
			return apperr.ErrRoomNotFound
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
		// ErrRoomFull also surfaces from pickFirstEmptySeat's drift guard
		// (player_count<4 but every seat taken). The user picked one specific
		// table, so we cannot retry into another — return ROOM_FULL honestly
		// rather than an opaque 5xx.
		if errors.Is(err, apperr.ErrAlreadyInRoom) || errors.Is(err, apperr.ErrRoomNotFound) ||
			errors.Is(err, apperr.ErrRoomNotQuickPlay) || errors.Is(err, apperr.ErrRoomFull) {
			return err
		}
		return fmt.Errorf("quick joining room: %w", err)
	}

	// Mark present (mirrors QuickPlay; quick-play rooms auto-start).
	h.presence.Add(resultRoom.ID, userID)

	// Joined an existing room — refresh the lobby grid, then mirror JoinRoom's
	// room-scoped player_joined + seat_updated broadcasts.
	h.broadcastRoomUpdated(resultRoom)
	h.broadcastQuickPlayerSeated(resultRoom, userID, assignedSeat, assignedTeam)

	// Auto-start when this join filled the last seat.
	matchStarted, err := h.autoStartIfFull(resultRoom.ID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"room":         resultRoom,
			"seat":         assignedSeat,
			"matchStarted": matchStarted,
		},
	})
}

// broadcastQuickPlayerSeated mirrors JoinRoom's broadcasts for a player who was
// auto-seated into a quick play room: player_joined seeds them into existing
// members' roomStore, seat_updated places them in the auto-assigned seat,
// and a fresh room snapshot updates every lobby viewer's grid card. Multi-event
// sequences are sent as separate ordered messages, never batched.
func (h *RoomHandler) broadcastQuickPlayerSeated(r *Room, userID uint, seat int, team string) {
	roomPlayers, err := h.repo.FindPlayersByRoomID(r.ID)
	if err != nil {
		slog.Error("quick play: loading players for join broadcast", "roomID", r.ID, "error", err)
		return
	}
	var username string
	for _, p := range roomPlayers {
		if p.UserID == userID {
			username = p.Username
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
		"playerCount": r.PlayerCount,
	})
	h.broadcastToUsers(userIDs, ws.SystemSeatUpdated, map[string]interface{}{
		"roomId":       r.ID,
		"userId":       userID,
		"username":     username,
		"seat":         seat,
		"team":         team,
		"previousSeat": nil,
	})
	// Seat broadcast above is room-scoped; push a fresh room snapshot to every
	// lobby viewer so the auto-assigned seat appears on grid cards.
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
