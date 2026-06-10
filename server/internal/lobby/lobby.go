// Package lobby exposes lightweight lobby-scoped read endpoints. Today it only
// serves aggregate player-state stats (in-lobby / in-room / in-match / online /
// registered) consumed by the lobby UI's activity panel.
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

// StatsResponse is the wire payload returned by GET /lobby/stats.
//
// All values are connection-aware: a user is only counted in inLobby /
// inRoom / inMatch if they currently hold a live WebSocket connection — with
// one deliberate exception: the REQUESTER is always treated as online, since
// on login the first stats fetch races the WS handshake (see GetStats). Stale
// DB rows (e.g. a user in room_players who has since dropped their socket)
// do not appear in any bucket. By construction:
//
//	online == inLobby + inRoom + inMatch
type StatsResponse struct {
	InLobby    int   `json:"inLobby"`
	InRoom     int   `json:"inRoom"`
	InMatch    int   `json:"inMatch"`
	Online     int   `json:"online"`
	Registered int64 `json:"registered"`
}

// PublicStatsResponse is the wire payload for the unauthenticated landing-page
// stats endpoint (GET /api/v1/stats). Deliberately minimal — only the two
// social-proof counts the marketing hero shows, nothing user-identifying.
type PublicStatsResponse struct {
	Online    int `json:"online"`    // currently-connected players (WS-live)
	OpenRooms int `json:"openRooms"` // rooms accepting players (status="waiting")
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

// GetStats bucket-counts every currently-connected user as in-match / in-room /
// in-lobby and returns the totals alongside the registered-user count.
//
// Bucketing precedence is in-match → in-room → in-lobby: a user mapped to an
// active session is counted as in-match even if a stale DB row also places
// them in a waiting room (defensive — the session manager wins).
func (h *Handler) GetStats(c echo.Context) error {
	connected := h.hub.ConnectedUserIDs()
	connectedSet := make(map[uint]struct{}, len(connected)+1)
	for _, uid := range connected {
		connectedSet[uid] = struct{}{}
	}

	// The requester is online by definition — they just made an authenticated
	// request — but their WebSocket may not have registered yet: on login the
	// lobby's first stats fetch races the WS auth handshake, and losing that
	// race made a lone player see "0 online" until the next poll. Count them
	// explicitly so the first paint is self-inclusive. Best-effort: the route
	// sits behind the auth middleware, so a missing userID only occurs in
	// direct-invocation tests.
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
		// Defensive precedence: never double-count a user who is also in a session.
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

// GetPublicStats serves the landing page's social-proof counts WITHOUT auth:
// total connected players and the number of open (waiting) rooms. Registered
// on the bare echo instance so no JWT is required. Online mirrors GetStats'
// connection-aware count; open rooms are those in "waiting" status — the same
// set the lobby's default room list ("open tables") shows.
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
