package apperr

import (
	"fmt"
	"net/http"
)

type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *AppError) Error() string {
	return e.Message
}

func NewAppError(code string, message string, status int) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Status:  status,
	}
}

func Wrap(err error, appErr *AppError) error {
	return fmt.Errorf("%s: %w", appErr.Message, appErr)
}

// Centralized application errors
var (
	ErrInternal     = NewAppError("INTERNAL_ERROR", "An internal error occurred", http.StatusInternalServerError)
	ErrNotFound     = NewAppError("NOT_FOUND", "Resource not found", http.StatusNotFound)
	ErrUnauthorized = NewAppError("UNAUTHORIZED", "Authentication required", http.StatusUnauthorized)
	ErrForbidden    = NewAppError("FORBIDDEN", "Access denied", http.StatusForbidden)
	ErrBadRequest   = NewAppError("BAD_REQUEST", "Invalid request", http.StatusBadRequest)
	ErrConflict     = NewAppError("CONFLICT", "Resource conflict", http.StatusConflict)

	// Auth domain errors
	ErrInvalidCredentials   = NewAppError("INVALID_CREDENTIALS", "invalid email or password", http.StatusUnauthorized)
	ErrEmailTaken           = NewAppError("EMAIL_TAKEN", "email is already registered", http.StatusConflict)
	ErrUsernameTaken        = NewAppError("USERNAME_TAKEN", "username is already taken", http.StatusConflict)
	ErrInvalidEmail         = NewAppError("INVALID_EMAIL", "invalid email format", http.StatusBadRequest)
	ErrPasswordTooShort     = NewAppError("PASSWORD_TOO_SHORT", "password must be at least 8 characters", http.StatusBadRequest)
	ErrPasswordTooLong      = NewAppError("PASSWORD_TOO_LONG", "password must be at most 72 characters", http.StatusBadRequest)
	ErrUsernameTooShort     = NewAppError("USERNAME_TOO_SHORT", "username must be at least 3 characters", http.StatusBadRequest)
	ErrUsernameTooLong      = NewAppError("USERNAME_TOO_LONG", "username must be at most 20 characters", http.StatusBadRequest)
	ErrUsernameInvalidChars = NewAppError("USERNAME_INVALID_CHARS", "username can only contain letters, numbers, and underscores", http.StatusBadRequest)
	// ErrInvalidResetToken is deliberately generic — it covers unknown, expired,
	// and already-used reset tokens alike so the response never reveals which.
	ErrInvalidResetToken = NewAppError("INVALID_RESET_TOKEN", "reset link is invalid or has expired", http.StatusBadRequest)

	// User domain errors
	ErrUserNotFound    = NewAppError("USER_NOT_FOUND", "user not found", http.StatusNotFound)
	ErrInvalidLanguage = NewAppError("INVALID_LANGUAGE", "language must be one of: en, sr, mk, hr", http.StatusBadRequest)

	// Room domain errors
	ErrRoomNameRequired          = NewAppError("ROOM_NAME_REQUIRED", "room name is required", http.StatusBadRequest)
	ErrRoomNameTooLong           = NewAppError("ROOM_NAME_TOO_LONG", "room name must be at most 100 characters", http.StatusBadRequest)
	ErrRoomNameTaken             = NewAppError("ROOM_NAME_TAKEN", "a room with this name already exists", http.StatusConflict)
	ErrRoomNotFound              = NewAppError("ROOM_NOT_FOUND", "room not found", http.StatusNotFound)
	ErrRoomFull                  = NewAppError("ROOM_FULL", "room is full", http.StatusConflict)
	ErrNotRoomOwner              = NewAppError("NOT_ROOM_OWNER", "only the room owner can perform this action", http.StatusForbidden)
	ErrInvalidVariant            = NewAppError("INVALID_VARIANT", "invalid game variant", http.StatusBadRequest)
	ErrInvalidMatchMode          = NewAppError("INVALID_MATCH_MODE", "invalid match mode", http.StatusBadRequest)
	ErrInvalidTimerStyle         = NewAppError("INVALID_TIMER_STYLE", "timer style must be 'relaxed' or 'per-move'", http.StatusBadRequest)
	ErrTimerDurationRequired     = NewAppError("TIMER_DURATION_REQUIRED", "timer duration is required for per-move timer", http.StatusBadRequest)
	ErrTimerDurationOutOfRange   = NewAppError("TIMER_DURATION_OUT_OF_RANGE", "timer duration must be between 10 and 120 seconds", http.StatusBadRequest)
	ErrRoomCodeTaken             = NewAppError("ROOM_CODE_TAKEN", "room code collision", http.StatusInternalServerError)
	ErrInvalidRoomStatus         = NewAppError("INVALID_ROOM_STATUS", "invalid room status filter", http.StatusBadRequest)
	ErrAlreadyInRoom             = NewAppError("ALREADY_IN_ROOM", "player is already in a room", http.StatusConflict)
	ErrNotInRoom                 = NewAppError("NOT_IN_ROOM", "player is not in this room", http.StatusNotFound)
	ErrSeatTaken                 = NewAppError("SEAT_TAKEN", "seat is already occupied", http.StatusConflict)
	ErrInvalidSeat               = NewAppError("INVALID_SEAT", "seat must be 0, 1, 2, or 3", http.StatusBadRequest)
	ErrNotAllSeated              = NewAppError("NOT_ALL_SEATED", "all 4 players must be seated to start", http.StatusBadRequest)
	ErrMatchNotStartable         = NewAppError("MATCH_NOT_STARTABLE", "room is not in waiting status", http.StatusConflict)
	ErrReconnectWindowOutOfRange = NewAppError("RECONNECT_WINDOW_OUT_OF_RANGE", "reconnect window must be between 30 and 300 seconds", http.StatusBadRequest)
	ErrRoomNotWaiting            = NewAppError("ROOM_NOT_WAITING", "this action is only available before the game starts", http.StatusConflict)
	ErrCannotKickSelf            = NewAppError("CANNOT_KICK_SELF", "the room owner cannot kick themselves", http.StatusBadRequest)
	ErrSeatNotOccupied           = NewAppError("SEAT_NOT_OCCUPIED", "seat must be occupied to be swapped", http.StatusConflict)
	ErrNotSeated                 = NewAppError("NOT_SEATED", "player has no seat to leave", http.StatusConflict)
	ErrQuickPlayLeaveSeatBlocked = NewAppError("QUICK_PLAY_LEAVE_SEAT_BLOCKED", "leaving a seat is not allowed in quick play rooms", http.StatusConflict)
	ErrRoomNotQuickPlay          = NewAppError("ROOM_NOT_QUICK_PLAY", "this room is not a quick play room", http.StatusConflict)
	// ErrQuickPlayBracketMismatch (Story 9.4) — a player tapped a quick-play room
	// whose coin bracket (its buy-in) doesn't match their own affordability
	// bracket. 409 Conflict, consistent with the sibling quick-play guards.
	ErrQuickPlayBracketMismatch = NewAppError("QUICK_PLAY_BRACKET_MISMATCH", "this quick play table is for a different coin bracket", http.StatusConflict)
	ErrOwnerCannotLeaveSeat     = NewAppError("OWNER_CANNOT_LEAVE_SEAT", "the room owner cannot leave their seat — leave the room instead", http.StatusConflict)
	ErrCannotTransferToSelf     = NewAppError("CANNOT_TRANSFER_TO_SELF", "ownership cannot be transferred to yourself", http.StatusBadRequest)
	ErrCannotPromoteUnseated    = NewAppError("CANNOT_PROMOTE_UNSEATED", "the new owner must be seated", http.StatusConflict)
	ErrMatchAlreadyStarted      = NewAppError("MATCH_ALREADY_STARTED", "the match has already started", http.StatusConflict)
	ErrBotsNotAllowed           = NewAppError("BOTS_NOT_ALLOWED", "bots cannot be added to this room", http.StatusConflict)
	ErrNoBotOnSeat              = NewAppError("NO_BOT_ON_SEAT", "no bot occupies this seat", http.StatusNotFound)

	// Economy domain errors (Epic 9). 409 Conflict (not 402) — the project's
	// status set is 200/201/400/401/403/404/409/500. Surfaces from the join
	// affordability check and the atomic match-start stake charge.
	ErrInsufficientCoins = NewAppError("INSUFFICIENT_COINS", "insufficient coins", http.StatusConflict)

	// Game domain errors
	ErrWrongPhase              = NewAppError("WRONG_PHASE", "action not valid in current game phase", http.StatusBadRequest)
	ErrNotYourTurn             = NewAppError("NOT_YOUR_TURN", "it is not your turn", http.StatusForbidden)
	ErrInvalidCard             = NewAppError("INVALID_CARD", "card is not in your hand", http.StatusBadRequest)
	ErrIllegalPlay             = NewAppError("ILLEGAL_PLAY", "card play violates game rules", http.StatusBadRequest)
	ErrGamePaused              = NewAppError("GAME_PAUSED", "game is currently paused", http.StatusConflict)
	ErrPauseExhausted          = NewAppError("PAUSE_EXHAUSTED", "player has already used their pause", http.StatusConflict)
	ErrNotPaused               = NewAppError("NOT_PAUSED", "game is not paused", http.StatusConflict)
	ErrNoActivePause           = NewAppError("NO_ACTIVE_PAUSE", "player does not have an active pause to clear", http.StatusConflict)
	ErrPlayerDisconnected      = NewAppError("PLAYER_DISCONNECTED", "player is disconnected", http.StatusConflict)
	ErrInvalidBid              = NewAppError("INVALID_BID", "invalid bid action", http.StatusBadRequest)
	ErrDeclarationNotAvailable = NewAppError("DECLARATION_NOT_AVAILABLE", "no declarable combinations in hand", http.StatusBadRequest)
	ErrBelotNotAvailable       = NewAppError("BELOT_NOT_AVAILABLE", "belot announcement not available", http.StatusBadRequest)
	ErrActionRequired          = NewAppError("ACTION_REQUIRED", "pending action must be resolved first", http.StatusBadRequest)
	ErrInvalidAction           = NewAppError("INVALID_ACTION", "action is not valid in this context", http.StatusBadRequest)
	ErrSurrenderExhausted      = NewAppError("SURRENDER_EXHAUSTED", "player has already used their surrender for this match", http.StatusConflict)

	// WebSocket domain errors
	ErrWSAuthTimeout    = NewAppError("WS_AUTH_TIMEOUT", "WebSocket authentication timed out", http.StatusUnauthorized)
	ErrWSAuthFailed     = NewAppError("WS_AUTH_FAILED", "WebSocket authentication failed", http.StatusUnauthorized)
	ErrWSInvalidMessage = NewAppError("WS_INVALID_MESSAGE", "invalid WebSocket message format", http.StatusBadRequest)
)
