package user

import (
	"regexp"
	"strings"
	"time"

	"github.com/emilijan/beljot/server/internal/apperr"
)

// Username rules — the single source of truth shared by registration
// (auth.validateRegisterRequest) and the change-username flow. Keep the
// register path and this validator in agreement; do not fork the rules.
const (
	UsernameMinLength = 3
	UsernameMaxLength = 20
	// UsernameChangeCooldownDays is how long a user must wait between username
	// changes. Server-authoritative; mirrored on the client in
	// shared/lib/usernameChange.ts (manual sync — the project has no shared
	// type generation between Go and TS).
	UsernameChangeCooldownDays = 30
)

// UsernameChangeCooldown is UsernameChangeCooldownDays expressed as a duration.
const UsernameChangeCooldown = UsernameChangeCooldownDays * 24 * time.Hour

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// ValidateUsername trims raw and enforces the username rules: length
// UsernameMinLength..UsernameMaxLength and characters [a-zA-Z0-9_]. It returns
// the trimmed username on success, or apperr.ErrUsernameTooShort /
// ErrUsernameTooLong / ErrUsernameInvalidChars. Length is byte length, which
// equals rune count here because the regex alphabet is ASCII-only.
func ValidateUsername(raw string) (string, error) {
	username := strings.TrimSpace(raw)
	if len(username) < UsernameMinLength {
		return "", apperr.ErrUsernameTooShort
	}
	if len(username) > UsernameMaxLength {
		return "", apperr.ErrUsernameTooLong
	}
	if !usernameRegex.MatchString(username) {
		return "", apperr.ErrUsernameInvalidChars
	}
	return username, nil
}
