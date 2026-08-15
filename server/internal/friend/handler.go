package friend

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/user"
	"github.com/emilijan/beljot/server/internal/ws"
)

// Notifier is the narrow slice of *ws.Hub the friend handler needs: is a user
// connected, and push them a message. Declared here (not taken as *ws.Hub) so
// the handler stays unit-testable with a spy. *ws.Hub satisfies it already.
type Notifier interface {
	IsConnected(userID uint) bool
	SendToUser(userID uint, msg []byte)
}

// Handler serves the friend endpoints. It owns the friend Repository and reuses
// the shared user.UserRepository for target-existence and username resolution
// (never adding methods to it — that would ripple through every user-repo mock).
type Handler struct {
	repo     Repository
	userRepo user.UserRepository
	notifier Notifier
}

func NewHandler(repo Repository, userRepo user.UserRepository, notifier Notifier) *Handler {
	return &Handler{repo: repo, userRepo: userRepo, notifier: notifier}
}

// --- Response DTOs ---

// FriendshipDTO is the created-request response (POST /friends/request).
type FriendshipDTO struct {
	ID        uint      `json:"id"`
	UserID    uint      `json:"userId"`
	FriendID  uint      `json:"friendId"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

// FriendStatusResponse drives the public-profile friendship button. Status is
// one of none | pending_outgoing | pending_incoming | friends; RequestID is the
// row id when a relationship exists (null otherwise).
type FriendStatusResponse struct {
	Status    string `json:"status"`
	RequestID *uint  `json:"requestId"`
}

// PendingRequestDTO is one incoming pending request (GET /friends/requests).
type PendingRequestDTO struct {
	ID           uint      `json:"id"`
	FromUserID   uint      `json:"fromUserId"`
	FromUsername string    `json:"fromUsername"`
	CreatedAt    time.Time `json:"createdAt"`
}

// FriendDTO is one accepted friend (GET /friends). Online is derived
// server-side from the live hub, never trusted from a client.
type FriendDTO struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Online   bool   `json:"online"`
}

type sendRequestBody struct {
	UserID uint `json:"userId"`
}

// SendRequest handles POST /friends/request. The requester is ALWAYS the
// authenticated caller — never a body-supplied id. Guards run self → target
// exists → either-direction duplicate, then creates the pending row and (only
// if the recipient is currently connected) pushes a best-effort notification.
func (h *Handler) SendRequest(c echo.Context) error {
	authUserID, err := getUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	var body sendRequestBody
	if err := c.Bind(&body); err != nil {
		return apperr.ErrBadRequest
	}
	recipientID := body.UserID
	if recipientID == 0 {
		return apperr.ErrBadRequest
	}
	if recipientID == authUserID {
		return apperr.ErrSelfFriendRequest
	}

	target, err := h.userRepo.FindByID(recipientID)
	if err != nil {
		return fmt.Errorf("finding friend target: %w", err)
	}
	if target == nil {
		return apperr.ErrUserNotFound
	}

	// Direction-agnostic duplicate check — the DB unique index only covers
	// (user_id, friend_id), so a reverse request (B->A while A->B pending) would
	// pass it. FindByPair is what blocks it.
	existing, err := h.repo.FindByPair(authUserID, recipientID)
	if err != nil {
		return fmt.Errorf("checking existing friendship: %w", err)
	}
	if existing != nil {
		if existing.Status == FriendStatusAccepted {
			return apperr.ErrAlreadyFriends
		}
		return apperr.ErrFriendRequestExists
	}

	f := &Friendship{
		UserID:   authUserID,
		FriendID: recipientID,
		Status:   FriendStatusPending,
	}
	if err := h.repo.Create(f); err != nil {
		return fmt.Errorf("creating friend request: %w", err)
	}

	// Best-effort, online-only push. Never fail the request when the recipient
	// is offline — the durable path is their pending-requests list on next load.
	if h.notifier != nil && h.notifier.IsConnected(recipientID) {
		fromUsername := ""
		if requester, ferr := h.userRepo.FindByID(authUserID); ferr == nil && requester != nil {
			fromUsername = requester.Username
		}
		h.pushFriendRequest(recipientID, f.ID, authUserID, fromUsername)
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"data": FriendshipDTO{
			ID:        f.ID,
			UserID:    f.UserID,
			FriendID:  f.FriendID,
			Status:    f.Status,
			CreatedAt: f.CreatedAt,
		},
	})
}

// GetStatus handles GET /friends/status/:id — the relationship between the
// viewer and subject :id. Self resolves to "none" (the button is never shown on
// your own page).
func (h *Handler) GetStatus(c echo.Context) error {
	viewerID, err := getUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}
	subjectID, err := parseIDParam(c)
	if err != nil {
		return err
	}

	if subjectID == viewerID {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"data": FriendStatusResponse{Status: "none"},
		})
	}

	f, err := h.repo.FindByPair(viewerID, subjectID)
	if err != nil {
		return fmt.Errorf("reading friendship status: %w", err)
	}

	resp := FriendStatusResponse{Status: "none"}
	if f != nil {
		id := f.ID
		resp.RequestID = &id
		switch {
		case f.Status == FriendStatusAccepted:
			resp.Status = "friends"
		case f.UserID == viewerID:
			resp.Status = "pending_outgoing"
		default:
			resp.Status = "pending_incoming"
		}
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"data": resp})
}

// ListRequests handles GET /friends/requests — the viewer's incoming pending
// requests with the sender's username. Empty → [].
func (h *Handler) ListRequests(c echo.Context) error {
	viewerID, err := getUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	rows, err := h.repo.ListIncomingPending(viewerID)
	if err != nil {
		return fmt.Errorf("listing friend requests: %w", err)
	}

	usernames, err := h.usernamesFor(senderIDs(rows))
	if err != nil {
		return fmt.Errorf("resolving requester usernames: %w", err)
	}

	items := make([]PendingRequestDTO, 0, len(rows))
	for _, f := range rows {
		username, ok := usernames[f.UserID]
		if !ok {
			// The sender was soft-deleted after sending — omit rather than 500.
			continue
		}
		items = append(items, PendingRequestDTO{
			ID:           f.ID,
			FromUserID:   f.UserID,
			FromUsername: username,
			CreatedAt:    f.CreatedAt,
		})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"data": items})
}

// Accept handles POST /friends/:id/accept — recipient-only, atomic. A miss
// (not the recipient, not pending, or missing) is a uniform 404.
func (h *Handler) Accept(c echo.Context) error {
	viewerID, err := getUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}
	id, err := parseIDParam(c)
	if err != nil {
		return err
	}

	rows, err := h.repo.Accept(id, viewerID)
	if err != nil {
		return fmt.Errorf("accepting friend request: %w", err)
	}
	if rows == 0 {
		return apperr.ErrFriendRequestNotFound
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{"id": id, "status": FriendStatusAccepted},
	})
}

// Decline handles POST /friends/:id/decline — recipient-only removal of a
// pending row. A miss is a uniform 404, same as Accept.
func (h *Handler) Decline(c echo.Context) error {
	viewerID, err := getUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}
	id, err := parseIDParam(c)
	if err != nil {
		return err
	}

	rows, err := h.repo.Delete(id, viewerID)
	if err != nil {
		return fmt.Errorf("declining friend request: %w", err)
	}
	if rows == 0 {
		return apperr.ErrFriendRequestNotFound
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{"id": id, "status": "declined"},
	})
}

// ListFriends handles GET /friends — the viewer's accepted friends with a
// live online flag. The list is symmetric (a friend appears whether the viewer
// was requester or recipient); soft-deleted friends are omitted. Empty → [].
func (h *Handler) ListFriends(c echo.Context) error {
	viewerID, err := getUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	rows, err := h.repo.ListAccepted(viewerID)
	if err != nil {
		return fmt.Errorf("listing friends: %w", err)
	}

	otherIDs := make([]uint, 0, len(rows))
	for _, f := range rows {
		other := f.UserID
		if other == viewerID {
			other = f.FriendID
		}
		otherIDs = append(otherIDs, other)
	}

	usernames, err := h.usernamesFor(otherIDs)
	if err != nil {
		return fmt.Errorf("resolving friend usernames: %w", err)
	}

	items := make([]FriendDTO, 0, len(otherIDs))
	for _, id := range otherIDs {
		username, ok := usernames[id]
		if !ok {
			// Friend row outlived its (soft-deleted) user — omit, don't 500.
			continue
		}
		online := false
		if h.notifier != nil {
			online = h.notifier.IsConnected(id)
		}
		items = append(items, FriendDTO{ID: id, Username: username, Online: online})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"data": items})
}

// pushFriendRequest marshals and sends the best-effort system:friend_request
// per-user push. Marshal failures are logged and swallowed — a notification
// must never take the request down.
func (h *Handler) pushFriendRequest(recipientID, requestID, fromUserID uint, fromUsername string) {
	payload := ws.FriendRequestPayload{
		RequestID:    requestID,
		FromUserID:   fromUserID,
		FromUsername: fromUsername,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("friend: failed to marshal friend_request payload", "error", err)
		return
	}
	msg, err := json.Marshal(ws.WSMessage{Type: ws.SystemFriendRequest, Payload: payloadBytes})
	if err != nil {
		slog.Error("friend: failed to marshal friend_request message", "error", err)
		return
	}
	h.notifier.SendToUser(recipientID, msg)
}

// usernamesFor batches a username lookup for the given ids into one
// FindManyByIDs call, returning a map keyed by userID. Soft-deleted users are
// simply absent from the result (FindManyByIDs excludes them).
func (h *Handler) usernamesFor(ids []uint) (map[uint]string, error) {
	if len(ids) == 0 {
		return map[uint]string{}, nil
	}
	users, err := h.userRepo.FindManyByIDs(ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uint]string, len(users))
	for _, u := range users {
		out[u.ID] = u.Username
	}
	return out, nil
}

// senderIDs collects the distinct requester ids from a set of pending rows.
func senderIDs(rows []Friendship) []uint {
	seen := make(map[uint]struct{}, len(rows))
	ids := make([]uint, 0, len(rows))
	for _, f := range rows {
		if _, ok := seen[f.UserID]; ok {
			continue
		}
		seen[f.UserID] = struct{}{}
		ids = append(ids, f.UserID)
	}
	return ids
}

// getUserID reads the authenticated user id stamped into the request context by
// auth.AuthMiddleware. Mirrors user.getUserID — the friend package keeps its own
// copy rather than exporting one, to avoid cross-package coupling for a 3-liner.
func getUserID(c echo.Context) (uint, error) {
	val := c.Get("userID")
	if val == nil {
		return 0, fmt.Errorf("userID not found in context")
	}
	userID, ok := val.(uint)
	if !ok {
		return 0, fmt.Errorf("userID has unexpected type")
	}
	return userID, nil
}

// parseIDParam parses the :id path param as a positive uint, rejecting 0 and
// non-numeric values with a 400 (mirrors the user handler's id parsing).
func parseIDParam(c echo.Context) (uint, error) {
	paramID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || paramID == 0 {
		return 0, apperr.ErrBadRequest
	}
	return uint(paramID), nil
}
