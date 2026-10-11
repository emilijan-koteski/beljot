package user

import (
	"regexp"
	"strings"
	"time"

	"github.com/emilijan/beljot/server/internal/apperr"
)

const (
	UsernameMinLength          = 3
	UsernameMaxLength          = 20
	UsernameChangeCooldownDays = 30
)

const UsernameChangeCooldown = UsernameChangeCooldownDays * 24 * time.Hour

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// ValidateUsername trims raw and enforces the shared username length and [a-zA-Z0-9_] rules.
// It returns the trimmed username, or the matching apperr username error.
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
