package lobby

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/emilijan/beljot/server/internal/auth"
	"github.com/emilijan/beljot/server/internal/room"
	"github.com/emilijan/beljot/server/internal/user"
)

// ConnectionTracker reports the set of user IDs currently holding a live
// WebSocket connection. Implemented by *ws.Hub.
type ConnectionTracker interface {
	ConnectedUserIDs() []uint
}

// SessionTracker reports the set of user IDs currently mapped to an active
// game session. Implemented by *match.Manager.
type SessionTracker interface {
	InMatchUserIDs() []uint
}

// StatsResponse is the wire payload returned by GET /lobby/stats. Only
// WebSocket-connected users (plus the requester) are counted, so
// Online == InLobby + InRoom + InMatch.
type StatsResponse struct {
	InLobby    int   `json:"inLobby"`
	InRoom     int   `json:"inRoom"`
	InMatch    int   `json:"inMatch"`
	Online     int   `json:"online"`
	Registered int64 `json:"registered"`
}

// PublicStatsResponse is the wire payload for the unauthenticated landing-page
// stats endpoint (GET /api/v1/stats), with nothing user-identifying.
type PublicStatsResponse struct {
	Online    int `json:"online"`
	OpenRooms int `json:"openRooms"`
}

// Handler serves lobby-scoped read endpoints.
type Handler struct {
	hub      ConnectionTracker
	sessions SessionTracker
	rooms    room.RoomRepository
	users    user.UserRepository
}

// NewHandler wires a Handler with its four data sources.
func NewHandler(hub ConnectionTracker, sessions SessionTracker, rooms room.RoomRepository, users user.UserRepository) *Handler {
	return &Handler{hub: hub, sessions: sessions, rooms: rooms, users: users}
}

// GetStats counts every connected user, including the requester whose socket may
// still be handshaking, as in-match, in-room or in-lobby (in that precedence) and
// returns the totals alongside the registered-user count.
func (h *Handler) GetStats(c echo.Context) error {
	connected := h.hub.ConnectedUserIDs()
	connectedSet := make(map[uint]struct{}, len(connected)+1)
	for _, uid := range connected {
		connectedSet[uid] = struct{}{}
	}

	if requesterID, err := auth.GetUserID(c); err == nil {
		connectedSet[requesterID] = struct{}{}
	}

	inMatchSet := make(map[uint]struct{})
	for _, uid := range h.sessions.InMatchUserIDs() {
		if _, online := connectedSet[uid]; online {
			inMatchSet[uid] = struct{}{}
		}
	}

	waitingUserIDs, err := h.rooms.FindUserIDsByRoomStatus("waiting")
	if err != nil {
		return err
	}
	inRoomSet := make(map[uint]struct{})
	for _, uid := range waitingUserIDs {
		if _, online := connectedSet[uid]; !online {
			continue
		}
		if _, inMatch := inMatchSet[uid]; inMatch {
			continue
		}
		inRoomSet[uid] = struct{}{}
	}

	registered, err := h.users.Count()
	if err != nil {
		return err
	}

	online := len(connectedSet)
	inMatch := len(inMatchSet)
	inRoom := len(inRoomSet)
	inLobby := online - inMatch - inRoom
	if inLobby < 0 {
		inLobby = 0
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": StatsResponse{
			InLobby:    inLobby,
			InRoom:     inRoom,
			InMatch:    inMatch,
			Online:     online,
			Registered: registered,
		},
	})
}

// GetPublicStats serves the landing page's counts without auth: connected
// players and the number of rooms in "waiting" status.
func (h *Handler) GetPublicStats(c echo.Context) error {
	online := len(h.hub.ConnectedUserIDs())

	waiting, err := h.rooms.FindByStatus("waiting")
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": PublicStatsResponse{
			Online:    online,
			OpenRooms: len(waiting),
		},
	})
}
