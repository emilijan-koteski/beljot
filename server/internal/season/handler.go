package season

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/emilijan/beljot/server/internal/apperr"
)

// CurrentSeasonView is the GET /api/v1/seasons/current body: the active window plus the caller's
// own rank, decomposed so the client does no ladder arithmetic. EndsAt is an absolute timestamp
// and RankDivision is null for Master and Grandmaster.
type CurrentSeasonView struct {
	SeasonName        string    `json:"seasonName"`
	EndsAt            time.Time `json:"endsAt"`
	SP                int       `json:"sp"`
	RankTier          string    `json:"rankTier"`
	RankDivision      *int      `json:"rankDivision"`
	SPIntoDivision    int       `json:"spIntoDivision"`
	SPForNextDivision int       `json:"spForNextDivision"`
	GamesPlayed       int       `json:"gamesPlayed"`
	GamesCompleted    int       `json:"gamesCompleted"`
	GamesWon          int       `json:"gamesWon"`
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

// GetCurrentSeason handles GET /api/v1/seasons/current for the JWT caller only, never a path id.
// A caller who has not played this season gets the Iron 1 zero state, not a 404.
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

// LeaderboardRowView is one leaderboard row, with Position as its 1-based slot in the full season order.
// Tier and Division derive from SP for a running season and come from the stored snapshot for an ended one.
type LeaderboardRowView struct {
	Position    int     `json:"position"`
	UserID      uint    `json:"userId"`
	Username    string  `json:"username"`
	AvatarURL   *string `json:"avatarUrl"`
	SP          int     `json:"sp"`
	Tier        string  `json:"tier"`
	Division    *int    `json:"division"`
	GamesPlayed int     `json:"gamesPlayed"`
	GamesWon    int     `json:"gamesWon"`
}

// LeaderboardViewerView is the caller's own standing, positioned under the list's total order
// (sp DESC, user_id ASC). It deliberately omits the username, which the client already holds.
type LeaderboardViewerView struct {
	Position    int    `json:"position"`
	UserID      uint   `json:"userId"`
	SP          int    `json:"sp"`
	Tier        string `json:"tier"`
	Division    *int   `json:"division"`
	GamesPlayed int    `json:"gamesPlayed"`
	GamesWon    int    `json:"gamesWon"`
}

// LeaderboardView is the GET /api/v1/leaderboard body in the shared { items, total, limit, offset } shape.
// Viewer is null, never omitted, when the caller has not played the season.
type LeaderboardView struct {
	Items  []LeaderboardRowView   `json:"items"`
	Total  int64                  `json:"total"`
	Limit  int                    `json:"limit"`
	Offset int                    `json:"offset"`
	Viewer *LeaderboardViewerView `json:"viewer"`
}

// parseLeaderboardQuery validates limit, offset and season, returning apperr.ErrBadRequest instead of
// coercing bad values; empty values take the defaults. Season ids parse as 32-bit so an over-range id never truncates to another season.
func parseLeaderboardQuery(c echo.Context) (limit, offset int, seasonID uint, err error) {
	const defaultLimit = 10
	const maxLimit = 50
	const maxOffset = 10_000

	limit = defaultLimit
	if raw := c.QueryParam("limit"); raw != "" {
		v, convErr := strconv.Atoi(raw)
		if convErr != nil || v < 1 || v > maxLimit {
			return 0, 0, 0, apperr.ErrBadRequest
		}
		limit = v
	}

	offset = 0
	if raw := c.QueryParam("offset"); raw != "" {
		v, convErr := strconv.Atoi(raw)
		if convErr != nil || v < 0 || v > maxOffset {
			return 0, 0, 0, apperr.ErrBadRequest
		}
		offset = v
	}

	switch raw := c.QueryParam("season"); raw {
	case "", "current":
	default:
		v, convErr := strconv.ParseUint(raw, 10, 32)
		if convErr != nil || v < 1 {
			return 0, 0, 0, apperr.ErrBadRequest
		}
		seasonID = uint(v)
	}

	return limit, offset, seasonID, nil
}

// GetLeaderboard handles GET /api/v1/leaderboard?season=current|<id> for an authenticated caller.
// The viewer block is keyed off the JWT subject, so other players appear only in the ranked list.
func (h *Handler) GetLeaderboard(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	limit, offset, seasonID, err := parseLeaderboardQuery(c)
	if err != nil {
		return err
	}

	view, err := h.service.LeaderboardView(userID, seasonID, limit, offset, time.Now().UTC())
	if err != nil {
		if errors.Is(err, apperr.ErrSeasonNotFound) {
			return err
		}
		return fmt.Errorf("loading leaderboard: %w", err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": view,
	})
}

// SeasonListItemView is one row of GET /api/v1/seasons -- the picker's feed.
// Name is the machine-stable "YYYY QN" token, rendered verbatim and never
// translated; the timestamps are absolute ISO 8601, like every wire timestamp.
type SeasonListItemView struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	StartedAt time.Time `json:"startedAt"`
	EndsAt    time.Time `json:"endsAt"`
}

// SeasonsListView is the GET /api/v1/seasons response body.
// Items is newest-first and never null, so an empty table serializes as [].
type SeasonsListView struct {
	Items []SeasonListItemView `json:"items"`
}

// ArchiveRowView is one ended, played season in a player's archive, with the rank the season finished on.
// Division is null for Master, Grandmaster and rows scored before divisions existed.
type ArchiveRowView struct {
	SeasonID    uint      `json:"seasonId"`
	SeasonName  string    `json:"seasonName"`
	SP          int       `json:"sp"`
	Tier        string    `json:"tier"`
	Division    *int      `json:"division"`
	GamesPlayed int       `json:"gamesPlayed"`
	GamesWon    int       `json:"gamesWon"`
	StartedAt   time.Time `json:"startedAt"`
	EndsAt      time.Time `json:"endsAt"`
}

// ArchiveView is the GET /api/v1/users/:id/seasons body. An unknown user id gets an empty list, not a 404.
type ArchiveView struct {
	Items []ArchiveRowView `json:"items"`
}

// SeasonRankView is the subject's active-season standing on both profile DTOs, null when they have not played.
// It exposes only what the leaderboard already shows; Position is 0 when the leaderboard would not list them.
type SeasonRankView struct {
	SeasonName  string `json:"seasonName"`
	Tier        string `json:"tier"`
	Division    *int   `json:"division"`
	SP          int    `json:"sp"`
	Position    int    `json:"position"`
	Players     int64  `json:"players"`
	GamesPlayed int    `json:"gamesPlayed"`
	GamesWon    int    `json:"gamesWon"`
}

// GetSeasons handles GET /api/v1/seasons: every season window, newest-first,
// feeding the leaderboard page's season picker. Authenticated like the rest of
// the api group; carries no per-player data at all.
func (h *Handler) GetSeasons(c echo.Context) error {
	if _, err := getUserID(c); err != nil {
		return apperr.ErrUnauthorized
	}

	view, err := h.service.SeasonsView(time.Now().UTC())
	if err != nil {
		return fmt.Errorf("loading seasons: %w", err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": view,
	})
}

// GetPlayerSeasonArchive handles GET /api/v1/users/:id/seasons, listing ended played seasons newest-first.
// An unknown id is an empty archive rather than a 404, since the profile query owns not-found.
func (h *Handler) GetPlayerSeasonArchive(c echo.Context) error {
	if _, err := getUserID(c); err != nil {
		return apperr.ErrUnauthorized
	}

	paramID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || paramID == 0 {
		return apperr.ErrBadRequest
	}

	view, err := h.service.ArchiveView(uint(paramID), time.Now().UTC())
	if err != nil {
		return fmt.Errorf("loading season archive: %w", err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": view,
	})
}
