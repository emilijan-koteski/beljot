package user

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/match"
)

// GetRoomLastMatch returns the room's most recent match in the same
// viewer-relative DTO the profile's match history uses, so both surfaces render
// from one shape.
//
// It lives on UserHandler rather than RoomHandler even though the path is
// /rooms/:id/last-match: the response is a MatchListItem, which needs the
// batched username hydration (loadUsernamesForMatches) plus buildMatchListItem,
// and RoomHandler holds no user repository. Moving the DTO would fork it.
//
// AUTHORIZATION IS MATCH PARTICIPATION, not room membership: the repository
// query only returns a row when the authenticated caller occupied one of its
// four seats. That is both the gate and the correctness guarantee for the
// viewer-relative projection (see GetLastMatchForRoomAndUser). Room membership
// would be the wrong test twice over — room.requireRoomMember 404s any room
// whose status is not "waiting" (this endpoint is also read from the
// end-of-match dialog, while the room is still "completed"), and a player who
// joined the room AFTER the match has no business reading it.
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
	// Absent row and non-participant are the SAME response on purpose: a
	// stranger must not be able to tell whether the room ever hosted a match.
	if m == nil {
		return apperr.ErrNotFound
	}

	usernames, err := h.loadUsernamesForMatches([]match.Match{*m})
	if err != nil {
		return fmt.Errorf("loading match usernames: %w", err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": buildMatchListItem(*m, viewerID, usernames),
	})
}
