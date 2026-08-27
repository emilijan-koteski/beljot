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

// --- Story 13.2: GET /api/v1/leaderboard ---

// LeaderboardRowView is one row of the seasonal leaderboard.
//
// Tier is the DERIVED value (TierForSP over `sp`), never the denormalized
// rank_tier column, and it is a stable machine token the client maps to a label
// and a colour -- same contract as CurrentSeasonView.RankTier. Position is the
// row's 1-based slot in the FULL season order, not its index in this page.
type LeaderboardRowView struct {
	Position    int    `json:"position"`
	UserID      uint   `json:"userId"`
	Username    string `json:"username"`
	SP          int    `json:"sp"`
	Tier        string `json:"tier"`
	GamesPlayed int    `json:"gamesPlayed"`
}

// LeaderboardViewerView is the CALLER'S OWN standing, so the client can mark
// their row in the list and pin it into view when it falls outside the loaded
// pages.
//
// NO USERNAME, on purpose: the viewer is the authenticated caller and the client
// already holds their name in authStore, so the pinned row renders through the
// same component with the name supplied locally (Story 13.2 D3).
//
// Position is counted under the LIST'S OWN total order (sp DESC, user_id ASC),
// so a tied viewer's number matches the slot they occupy rather than the shared
// number a COUNT(sp > x) would give every tied player.
type LeaderboardViewerView struct {
	Position    int    `json:"position"`
	UserID      uint   `json:"userId"`
	SP          int    `json:"sp"`
	Tier        string `json:"tier"`
	GamesPlayed int    `json:"gamesPlayed"`
}

// LeaderboardView is the GET /api/v1/leaderboard response body (the value inside
// the { "data": ... } envelope).
//
// { items, total, limit, offset } is this project's ONE paginated shape, taken
// verbatim from MatchesListResponse (user/handler.go). Total is the whole
// season's visible row count, not the page length, so the client's load-more can
// tell when it has reached the end.
//
// Viewer is NULL when the caller has no season row or has earned no SP -- the AC
// marks an own-row only for a player with any SP. `omitempty` is deliberately
// NOT set: the key must always be present so the client can distinguish "no
// standing" from "an older server that does not send this".
type LeaderboardView struct {
	Items  []LeaderboardRowView   `json:"items"`
	Total  int64                  `json:"total"`
	Limit  int                    `json:"limit"`
	Offset int                    `json:"offset"`
	Viewer *LeaderboardViewerView `json:"viewer"`
}

// parseLeaderboardQuery reads limit/offset/season and applies the documented
// bounds and allowlist. Returns apperr.ErrBadRequest on any violation, mirroring
// parseMatchesQuery (user/handler.go) -- including its strictness: a MALFORMED or
// out-of-range value is rejected rather than coerced, so a client bug surfaces as
// a 400 instead of a quietly wrong page.
//
// AN EMPTY VALUE IS NOT A MALFORMED ONE. `?limit=` and `?offset=` (present, no
// value) are treated as ABSENT and take the defaults, because the `raw != ""`
// guard cannot distinguish them from an omitted parameter -- and neither can the
// precedent this mirrors: parseMatchesQuery behaves identically, so a client that
// builds `limit=${x}` with an undefined x gets the same answer from both
// endpoints. Only a non-empty value that fails to parse, or parses outside the
// bounds, is a 400.
//
// defaultLimit is 10 because the lobby widget is a TOP TEN and sends no limit at
// all; the full page passes its own.
//
// maxOffset EXISTS BECAUSE `offset` HAD NO CEILING AT ALL while `limit` had two
// bounds. Without it `offset=9223372036854775807` parses cleanly and reaches
// Postgres, and every deep offset makes the database sort and discard that many
// rows before returning a page -- so the cost of a request grew with a number the
// caller chose freely. 10,000 is 400 load-more pages deep, far past any real
// reader, and is a bound rather than a target: if the product ever needs to page
// past it, the fix is a keyset cursor (WHERE (sp, user_id) < (?, ?)), not a
// larger number here.
//
// `season` accepts "current", absence, or a POSITIVE INTEGER season id (Story
// 13.3's prior-season selector -- the value GET /api/v1/seasons hands the
// picker). The returned seasonID is 0 for the current-window selector. The
// integer form is parsed with ParseUint, base 10, BIT SIZE 32 -- ids are
// 32-bit SERIALs, and a 64-bit parse followed by a uint cast would silently
// TRUNCATE an over-range value to a different id on a 32-bit build. So "+5",
// "-1", "5.0", "abc" and anything above 2^32-1 are all 400s -- MALFORMED input
// never reaches the database, while a well-formed id that matches no row is
// the service's 404 (apperr.ErrSeasonNotFound), never a silent fallback to the
// active window, which would show the wrong standings under the right heading.
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
		// seasonID 0: the active window, resolved by the service.
	default:
		v, convErr := strconv.ParseUint(raw, 10, 32)
		if convErr != nil || v < 1 {
			return 0, 0, 0, apperr.ErrBadRequest
		}
		seasonID = uint(v)
	}

	return limit, offset, seasonID, nil
}

// GetLeaderboard handles GET /api/v1/leaderboard?season=current|<id>.
//
// AUTHENTICATED, like every other route on the api group. The viewer block is
// keyed off the JWT subject -- there is no id parameter -- so this endpoint
// exposes other players only as an SP-ordered list, never as a lookup.
//
// PULL-ONLY: page load plus poll. Standings deliberately have no WebSocket push
// (epic decision), so nothing here has a counterpart in ws/events.go.
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
		// apperr.ErrSeasonNotFound (a well-formed but unknown ?season=<id>) must
		// reach appErrorHandler UNWRAPPED-COMPATIBLY -- errors.As unwraps %w
		// chains, so plain wrapping is fine -- but wrapping it in prose here
		// would bury the one error that is the caller's, not ours. Return it
		// as-is; wrap only genuine internal failures.
		if errors.Is(err, apperr.ErrSeasonNotFound) {
			return err
		}
		return fmt.Errorf("loading leaderboard: %w", err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": view,
	})
}

// --- Story 13.3: GET /api/v1/seasons + GET /api/v1/users/:id/seasons ---

// SeasonListItemView is one row of GET /api/v1/seasons -- the picker's feed.
// Name is the machine-stable "YYYY QN" token, rendered verbatim and never
// translated; the timestamps are absolute ISO 8601, like every wire timestamp.
type SeasonListItemView struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	StartedAt time.Time `json:"startedAt"`
	EndsAt    time.Time `json:"endsAt"`
}

// SeasonsListView is the GET /api/v1/seasons response body (inside the
// { "data": ... } envelope). Items is newest-first and never null -- an empty
// table serializes as [].
type SeasonsListView struct {
	Items []SeasonListItemView `json:"items"`
}

// ArchiveRowView is one ended, played season in a player's archive
// (GET /api/v1/users/:id/seasons).
//
// Tier is DERIVED (TierForSP over the row's immutable SP), never the stored
// rank_tier column -- the same contract every other tier on the wire follows.
// SeasonName is the verbatim machine token.
type ArchiveRowView struct {
	SeasonID    uint      `json:"seasonId"`
	SeasonName  string    `json:"seasonName"`
	SP          int       `json:"sp"`
	Tier        string    `json:"tier"`
	GamesPlayed int       `json:"gamesPlayed"`
	StartedAt   time.Time `json:"startedAt"`
	EndsAt      time.Time `json:"endsAt"`
}

// ArchiveView is the GET /api/v1/users/:id/seasons response body (inside the
// { "data": ... } envelope). Items is newest-first and never null: a player
// with no ended played seasons -- and an UNKNOWN user id alike -- gets
// { "items": [] } with a 200, never a 404. The profile query owns user
// existence; this endpoint only answers "which ended seasons did this id play",
// and the client hides the whole archive section on an empty answer.
type ArchiveView struct {
	Items []ArchiveRowView `json:"items"`
}

// SeasonRankView is the `seasonRank` block on BOTH profile DTOs (Story 13.3):
// the subject's standing in the ACTIVE season. Nil -- serialized as null --
// when the subject has not played this season. Public-safe by construction:
// tier and SP are exactly what the leaderboard already exposes for every
// player, and SeasonName is the machine token.
type SeasonRankView struct {
	SeasonName string `json:"seasonName"`
	Tier       string `json:"tier"`
	SP         int    `json:"sp"`
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

// GetPlayerSeasonArchive handles GET /api/v1/users/:id/seasons: the SUBJECT's
// ended, played seasons, newest-first. Public to any authenticated viewer,
// mirroring the profile endpoints (Story 11.3): the id names the subject, the
// data is exactly what the leaderboard already exposes, and DELIBERATELY NO
// user-existence 404 -- an unknown id is an empty archive (the profile query,
// which the client always issues first, owns the not-found surface).
func (h *Handler) GetPlayerSeasonArchive(c echo.Context) error {
	if _, err := getUserID(c); err != nil {
		return apperr.ErrUnauthorized
	}

	// BIT SIZE 32, like the ?season= selector above: ids are 32-bit SERIALs,
	// and a 64-bit parse cast to uint would truncate to a DIFFERENT user's id
	// on a 32-bit build -- serving someone else's archive instead of a 400.
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
