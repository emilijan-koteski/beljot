package season_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/season"
)

// --- Mock Repository (in-memory, mirrors the GORM semantics) ---

type mockRepo struct {
	current      *season.Season
	currentErr   error
	currentCalls int
	// rows is keyed "userID:seasonID"; a missing key is the zero state.
	rows    map[string]*season.PlayerSeason
	findErr error
}

func newMockRepo(current *season.Season) *mockRepo {
	return &mockRepo{current: current, rows: map[string]*season.PlayerSeason{}}
}

func key(userID, seasonID uint) string {
	return fmt.Sprintf("%d:%d", userID, seasonID)
}

func (m *mockRepo) CurrentSeason(time.Time) (*season.Season, error) {
	m.currentCalls++
	if m.currentErr != nil {
		return nil, m.currentErr
	}
	return m.current, nil
}

func (m *mockRepo) ApplySeasonPoints(seasonID uint, awards map[uint]season.SPAward) (map[uint]season.PlayerSeasonSnapshot, error) {
	out := make(map[uint]season.PlayerSeasonSnapshot, len(awards))
	for userID, a := range awards {
		row := m.rows[key(userID, seasonID)]
		prev := 0
		played, completed := 0, 0
		if row != nil {
			prev, played, completed = row.SP, row.GamesPlayed, row.GamesCompleted
		}
		played++
		if a.Completed {
			completed++
		}
		next := prev + a.SP
		m.rows[key(userID, seasonID)] = &season.PlayerSeason{
			UserID: userID, SeasonID: seasonID, SP: next,
			RankTier: season.TierForSP(next), GamesPlayed: played, GamesCompleted: completed,
		}
		out[userID] = season.PlayerSeasonSnapshot{
			SP: next, PreviousSP: prev, Tier: season.TierForSP(next),
			GamesPlayed: played, GamesCompleted: completed,
		}
	}
	return out, nil
}

func (m *mockRepo) FindPlayerSeason(userID, seasonID uint) (*season.PlayerSeason, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.rows[key(userID, seasonID)], nil
}

// --- Test harness ---

var testWindow = &season.Season{
	ID:        7,
	Name:      "2026 Q3",
	StartedAt: time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
	EndsAt:    time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC),
}

// call issues GET /seasons/current with userID already on the context, the way
// the auth middleware leaves it.
func call(t *testing.T, repo *mockRepo, userID uint) (*httptest.ResponseRecorder, error) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/seasons/current", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if userID != 0 {
		c.Set("userID", userID)
	}
	h := season.NewHandler(season.NewService(repo))
	return rec, h.GetCurrentSeason(c)
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) season.CurrentSeasonView {
	t.Helper()
	var env struct {
		Data season.CurrentSeasonView `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env), "response must be wrapped in a data envelope")
	return env.Data
}

// THE JSON TAGS ARE THE CONTRACT, and `decode` above cannot police them: it
// unmarshals into season.CurrentSeasonView itself, so it agrees with whatever
// spelling the struct currently carries. Rename the `spForNextTier` tag and every
// Go and TS test still passes, while at runtime the client reads `undefined`,
// seasonBarFill returns 1, `atTop` flips true, and EVERY player's bar renders
// 100% full captioned "Top of the ladder" — silently.
//
// So assert the LITERAL wire keys, exactly and exhaustively. The client's
// CurrentSeasonResponse interface is hand-maintained against these names (there
// is no golden for HTTP payloads the way there is for WS events), which makes
// this the only gate between the two.
func TestGetCurrentSeason_WirePayloadKeysAreExact(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.rows[key(42, testWindow.ID)] = &season.PlayerSeason{
		UserID: 42, SeasonID: testWindow.ID, SP: 4000,
		RankTier: "gold", GamesPlayed: 31, GamesCompleted: 29,
	}

	rec, err := call(t, repo, 42)
	require.NoError(t, err)

	var env map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.Contains(t, env, "data", "the response is always wrapped in a data envelope, never a bare object")
	assert.Len(t, env, 1, "nothing rides beside `data`")

	data, ok := env["data"].(map[string]any)
	require.True(t, ok, "data must be an object")

	got := make([]string, 0, len(data))
	for k := range data {
		got = append(got, k)
	}
	sort.Strings(got)

	// Sorted literals, matching client/src/shared/types/apiTypes.ts's
	// CurrentSeasonResponse field for field.
	assert.Equal(t, []string{
		"endsAt",
		"gamesCompleted",
		"gamesPlayed",
		"rankTier",
		"seasonName",
		"sp",
		"spForNextTier",
		"spIntoTier",
	}, got, "exact wire key set — a renamed or dropped tag breaks the client silently")

	// Spot-check the types too: a tag that survives but changes shape (an int
	// serialised as a string, say) is the same class of silent break.
	assert.IsType(t, "", data["seasonName"])
	assert.IsType(t, "", data["endsAt"])
	assert.IsType(t, "", data["rankTier"])
	for _, numeric := range []string{"sp", "spIntoTier", "spForNextTier", "gamesPlayed", "gamesCompleted"} {
		assert.IsType(t, float64(0), data[numeric], "%s must be a JSON number", numeric)
	}
	// endsAt is an ABSOLUTE RFC 3339 timestamp, never a relative duration.
	_, parseErr := time.Parse(time.RFC3339, data["endsAt"].(string))
	assert.NoError(t, parseErr, "endsAt must be an absolute ISO 8601 timestamp")
}

// AC3: a player who has not played this season gets the ZERO STATE — 0 SP, Iron,
// a full Iron band to climb. Not a 404, and not a lazily created row: reads must
// not write.
func TestGetCurrentSeason_ZeroStateForANewPlayer(t *testing.T) {
	repo := newMockRepo(testWindow)

	rec, err := call(t, repo, 42)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	got := decode(t, rec)
	assert.Equal(t, "2026 Q3", got.SeasonName)
	assert.True(t, testWindow.EndsAt.Equal(got.EndsAt), "absolute timestamp, never a days-remaining count")
	assert.Equal(t, 0, got.SP)
	assert.Equal(t, "iron", got.RankTier, "0 SP is Iron — there is no unranked state")
	assert.Equal(t, 0, got.SPIntoTier)
	assert.Equal(t, 500, got.SPForNextTier)
	assert.Equal(t, 0, got.GamesPlayed)
	assert.Equal(t, 0, got.GamesCompleted)

	assert.Empty(t, repo.rows, "a read must not create a player_seasons row")
}

// AC3: a populated record, with the progress decomposed mid-tier.
func TestGetCurrentSeason_MidTierProgressDecomposition(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.rows[key(42, testWindow.ID)] = &season.PlayerSeason{
		UserID: 42, SeasonID: testWindow.ID, SP: 4000,
		RankTier: "gold", GamesPlayed: 31, GamesCompleted: 29,
	}

	rec, err := call(t, repo, 42)
	require.NoError(t, err)

	got := decode(t, rec)
	assert.Equal(t, 4000, got.SP)
	assert.Equal(t, "gold", got.RankTier)
	// 4000 sits 1000 into Gold's 2500-wide band (3000 -> 5500).
	assert.Equal(t, 1000, got.SPIntoTier)
	assert.Equal(t, 2500, got.SPForNextTier)
	assert.Equal(t, 31, got.GamesPlayed)
	assert.Equal(t, 29, got.GamesCompleted)
}

// D7: the response tier is DERIVED, never the stored rank_tier column. A stale
// column must not reach the client.
func TestGetCurrentSeason_IgnoresAStaleStoredTier(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.rows[key(42, testWindow.ID)] = &season.PlayerSeason{
		UserID: 42, SeasonID: testWindow.ID, SP: 9000,
		RankTier:    "iron", // deliberately wrong
		GamesPlayed: 60, GamesCompleted: 60,
	}

	rec, err := call(t, repo, 42)
	require.NoError(t, err)
	assert.Equal(t, "diamond", decode(t, rec).RankTier, "9000 SP is Diamond, whatever the column says")
}

// At the top of the ladder there is no next tier, and the client must be able to
// tell: spForNextTier is 0 rather than a fabricated band.
func TestGetCurrentSeason_GrandmasterIsTerminal(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.rows[key(42, testWindow.ID)] = &season.PlayerSeason{
		UserID: 42, SeasonID: testWindow.ID, SP: 20000, RankTier: "grandmaster",
	}

	rec, err := call(t, repo, 42)
	require.NoError(t, err)

	got := decode(t, rec)
	assert.Equal(t, "grandmaster", got.RankTier)
	assert.Equal(t, 2000, got.SPIntoTier)
	assert.Zero(t, got.SPForNextTier)
}

// The endpoint is keyed off the JWT subject only — with no authenticated user it
// is a 401, never a fallback to some other id.
func TestGetCurrentSeason_RequiresAuth(t *testing.T) {
	repo := newMockRepo(testWindow)
	_, err := call(t, repo, 0)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrUnauthorized)
}

func TestGetCurrentSeason_ResolverFailureSurfaces(t *testing.T) {
	repo := newMockRepo(nil)
	repo.currentErr = errors.New("db down")

	_, err := call(t, repo, 42)
	require.Error(t, err)
	assert.NotErrorIs(t, err, apperr.ErrUnauthorized, "a DB failure is not an auth failure")
}

// A repository that violates its own "never returns (nil, nil)" contract must
// produce an ERROR, not a nil dereference. Nothing in the type system enforces
// that contract, and on the write path the panic would land inside a match
// finalizer that is mid-way through settling four players.
func TestGetCurrentSeason_NilSeasonIsAnErrorNotAPanic(t *testing.T) {
	repo := newMockRepo(nil) // no error, no season — the contract violation

	_, err := call(t, repo, 42)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no season")
}

func TestApplySeasonPoints_NilSeasonIsAnErrorNotAPanic(t *testing.T) {
	svc := season.NewService(newMockRepo(nil))

	out, err := svc.ApplySeasonPoints(map[uint]match.SPAward{10: {SP: 200, Completed: true}}, time.Now().UTC())
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "no season")
}

// The awarder path never touches the DB for an empty award set — notably it must
// not create a season row for a match with no human seats.
func TestApplySeasonPoints_EmptyAwardsResolvesNoSeason(t *testing.T) {
	repo := newMockRepo(testWindow)
	svc := season.NewService(repo)

	out, err := svc.ApplySeasonPoints(nil, time.Now().UTC())
	require.NoError(t, err)
	assert.Empty(t, out)
	assert.Zero(t, repo.currentCalls, "no season is resolved (or created) for an empty batch")
}

// The service hands the match manager a FULLY PRECOMPUTED snapshot — season
// name, derived tier and tieredUp — so the manager never runs ladder arithmetic
// it cannot see (Story 13.1 D8).
func TestApplySeasonPoints_PrecomputesTheSnapshot(t *testing.T) {
	repo := newMockRepo(testWindow)
	svc := season.NewService(repo)

	// User 1 sits just below Bronze and crosses it. User 2 sits inside Iron and
	// stays. User 3 was absent, earns 0, and must report tieredUp false.
	repo.rows[key(1, testWindow.ID)] = &season.PlayerSeason{UserID: 1, SeasonID: testWindow.ID, SP: 450}
	repo.rows[key(2, testWindow.ID)] = &season.PlayerSeason{UserID: 2, SeasonID: testWindow.ID, SP: 100}
	repo.rows[key(3, testWindow.ID)] = &season.PlayerSeason{UserID: 3, SeasonID: testWindow.ID, SP: 499}

	got, err := svc.ApplySeasonPoints(map[uint]match.SPAward{
		1: {SP: 200, Completed: true},
		2: {SP: 200, Completed: true},
		3: {SP: 0, Completed: false},
	}, time.Now().UTC())
	require.NoError(t, err)

	assert.Equal(t, match.PlayerSeasonSnapshot{
		SeasonName: "2026 Q3", SP: 650, RankTier: "bronze", TieredUp: true,
	}, got[1], "450 -> 650 crosses the 500 Bronze floor")

	assert.Equal(t, match.PlayerSeasonSnapshot{
		SeasonName: "2026 Q3", SP: 300, RankTier: "iron", TieredUp: false,
	}, got[2], "100 -> 300 stays inside Iron")

	// The one that a naive "SP changed, so they must have climbed" shortcut would
	// get wrong: sitting one point below a floor and earning nothing is NOT a
	// tier-up, even though the seat is right on the edge.
	assert.Equal(t, match.PlayerSeasonSnapshot{
		SeasonName: "2026 Q3", SP: 499, RankTier: "iron", TieredUp: false,
	}, got[3], "a 0-SP absent seat never tiers up")
}
