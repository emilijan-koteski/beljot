package user

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/match"
)

// GetRoomLastMatch returns the room's most recent match as a viewer-relative MatchListItem.
// Only a participant of that match can read it; anyone else gets 404.
func (h *UserHandler) GetRoomLastMatch(c echo.Context) error {
	viewerID, err := getUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	paramID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || paramID == 0 {
		return apperr.ErrBadRequest
	}
	roomID := uint(paramID)

	m, err := h.matchRepo.GetLastMatchForRoomAndUser(roomID, viewerID)
	if err != nil {
		return fmt.Errorf("fetching room last match: %w", err)
	}
	if m == nil {
		return apperr.ErrNotFound
	}

	identities, err := h.loadIdentitiesForMatches([]match.Match{*m})
	if err != nil {
		return fmt.Errorf("loading match usernames: %w", err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": buildMatchListItem(*m, viewerID, identities),
	})
}
