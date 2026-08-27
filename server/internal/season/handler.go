package season

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/emilijan/beljot/server/internal/apperr"
)

// CurrentSeasonView is the GET /api/v1/seasons/current response body (the value
// inside the { "data": ... } envelope). It carries the active window plus the
// caller's own record, decomposed so the RankBanner renders without doing ladder
// arithmetic.
//
// EndsAt is an ABSOLUTE ISO 8601 timestamp, never a relative "days remaining":
// the wire rule across this project is absolute timestamps, and the countdown is
// display state the client recomputes on its own tick.
//
// RankTier is a stable machine token ("iron" ... "grandmaster") that the client maps
// to an i18n label and a colour. A display string never crosses the wire.
type CurrentSeasonView struct {
	SeasonName     string    `json:"seasonName"`
	EndsAt         time.Time `json:"endsAt"`
	SP             int       `json:"sp"`
	RankTier       string    `json:"rankTier"`
	SPIntoTier     int       `json:"spIntoTier"`
	SPForNextTier  int       `json:"spForNextTier"`
	GamesPlayed    int       `json:"gamesPlayed"`
	GamesCompleted int       `json:"gamesCompleted"`
}

// Handler serves the season endpoints.
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// getUserID reads the authenticated user id the auth middleware stored on the
// echo context. Mirrors user.getUserID / wallet.getUserID (unexported there, so
// duplicated here).
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

// GetCurrentSeason handles GET /api/v1/seasons/current.
//
// It returns THE CALLER'S OWN record, keyed off the JWT subject the auth
// middleware resolved -- never a path or query id, so there is no way to read
// another player's season standing through it. (Story 13.2's leaderboard is the
// endpoint that exposes other players, and it exposes them as a ranked list.)
//
// A caller who has not played this season gets the zero state, not a 404: at 0
// SP a player is Iron, which is a real tier that renders normally, and there is
// no "unranked" state in this ladder.
func (h *Handler) GetCurrentSeason(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	view, err := h.service.CurrentSeasonView(userID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("loading current season: %w", err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": view,
	})
}
