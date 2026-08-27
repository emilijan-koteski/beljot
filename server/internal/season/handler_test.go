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
	// usernames backs the join the GORM repo does against `users`. A row with no
	// entry here is treated as INVISIBLE — the mock's stand-in for
	// `users.deleted_at IS NULL`, applied to the page, the total AND CountAhead
	// exactly as the real predicate is.
	usernames    map[uint]string
	pageErr      error
	countErr     error
	entryErr     error
	pageCalls    int
	aheadCalls   int
	entryCalls   int
	lastPageArgs [3]int

	// Story 13.3 state. `seasons` are the EXTRA windows beside `current` (the
	// mock's stand-in for the seasons table); FindSeasonByID / ListSeasons /
	// PlayerSeasonArchive all read them plus `current`, mirroring the one real
	// table the GORM repo queries.
	seasons       []season.Season
	listErr       error
	findSeasonErr error
	archiveErr    error
	archiveCalls  int
}

func newMockRepo(current *season.Season) *mockRepo {
	return &mockRepo{
		current:   current,
		rows:      map[string]*season.PlayerSeason{},
		usernames: map[uint]string{},
	}
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

// visibleRows is the mock's copy of leaderboardScope: every LISTABLE row of the
// season, in the repository's own total order (sp DESC, user_id ASC). ONE helper
// feeds LeaderboardPage, CountAhead AND FindLeaderboardEntry here for the same
// reason the GORM repo has one scope — if the mock let them diverge it would
// happily pass a test the real repository fails.
//
// Two exclusions, mirroring the real predicate: no username entry stands in for
// `users.deleted_at IS NOT NULL`, and `sp <= 0` is the SP-earners-only
// membership rule.
func (m *mockRepo) visibleRows(seasonID uint) []*season.PlayerSeason {
	out := make([]*season.PlayerSeason, 0, len(m.rows))
	for _, row := range m.rows {
		if row.SeasonID != seasonID {
			continue
		}
		if _, visible := m.usernames[row.UserID]; !visible {
			continue
		}
		// THE LADDER IS SP EARNERS ONLY (owner decision 2026-08-27). Mirrors the
		// real `player_seasons.sp > 0` in leaderboardScope. A row at 0 SP is
		// written for any seat absent at a match end, so these exist normally --
		// and listing one would contradict the viewer block, which reports no
		// standing at 0 SP.
		if row.SP <= 0 {
			continue
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SP != out[j].SP {
			return out[i].SP > out[j].SP
		}
		return out[i].UserID < out[j].UserID
	})
	return out
}

func (m *mockRepo) LeaderboardPage(seasonID uint, limit, offset int) ([]season.LeaderboardEntry, int64, error) {
	m.pageCalls++
	m.lastPageArgs = [3]int{int(seasonID), limit, offset}
	if m.pageErr != nil {
		return nil, 0, m.pageErr
	}
	// The interface documents limit >= 1 and offset >= 0 as a PRECONDITION, so the
	// mock enforces it too: otherwise a service bug that passed limit=0 would sail
	// through here and fail only against Postgres.
	if limit < 1 || offset < 0 {
		return nil, 0, fmt.Errorf("mock: precondition violated limit=%d offset=%d", limit, offset)
	}
	rows := m.visibleRows(seasonID)
	total := int64(len(rows))

	entries := make([]season.LeaderboardEntry, 0, limit)
	for i := offset; i < len(rows) && len(entries) < limit; i++ {
		entries = append(entries, season.LeaderboardEntry{
			UserID:      rows[i].UserID,
			Username:    m.usernames[rows[i].UserID],
			SP:          rows[i].SP,
			GamesPlayed: rows[i].GamesPlayed,
		})
	}
	return entries, total, nil
}

// FindLeaderboardEntry runs through visibleRows, so it inherits BOTH exclusions
// -- which is the entire reason the real implementation exists separately from
// FindPlayerSeason, whose model query sees neither.
func (m *mockRepo) FindLeaderboardEntry(seasonID, userID uint) (*season.LeaderboardEntry, error) {
	m.entryCalls++
	if m.entryErr != nil {
		return nil, m.entryErr
	}
	for _, row := range m.visibleRows(seasonID) {
		if row.UserID != userID {
			continue
		}
		return &season.LeaderboardEntry{
			UserID:      row.UserID,
			Username:    m.usernames[row.UserID],
			SP:          row.SP,
			GamesPlayed: row.GamesPlayed,
		}, nil
	}
	return nil, nil
}

func (m *mockRepo) CountAhead(seasonID uint, sp int, userID uint) (int64, error) {
	m.aheadCalls++
	if m.countErr != nil {
		return 0, m.countErr
	}
	var ahead int64
	for _, row := range m.visibleRows(seasonID) {
		if row.SP > sp || (row.SP == sp && row.UserID < userID) {
			ahead++
		}
	}
	return ahead, nil
}

// allWindows is the mock's seasons table: the extra seeded windows plus the
// current one, deduped by id — every Story 13.3 read consults this one list the
// way the real reads consult the one real table.
func (m *mockRepo) allWindows() []season.Season {
	out := make([]season.Season, 0, len(m.seasons)+1)
	out = append(out, m.seasons...)
	if m.current != nil {
		dup := false
		for _, s := range out {
			if s.ID == m.current.ID {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, *m.current)
		}
	}
	return out
}

func (m *mockRepo) FindSeasonByID(id uint) (*season.Season, error) {
	if m.findSeasonErr != nil {
		return nil, m.findSeasonErr
	}
	for _, s := range m.allWindows() {
		if s.ID == id {
			found := s
			return &found, nil
		}
	}
	return nil, nil
}

func (m *mockRepo) ListSeasons() ([]season.Season, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	out := m.allWindows()
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out, nil
}

// PlayerSeasonArchive mirrors the real predicate — games_played >= 1 AND the
// window ENDED — over the same rows map the other reads use, so a service bug
// that reused leaderboardScope's sp > 0 rule would fail here too.
func (m *mockRepo) PlayerSeasonArchive(userID uint, now time.Time) ([]season.ArchiveEntry, error) {
	m.archiveCalls++
	if m.archiveErr != nil {
		return nil, m.archiveErr
	}
	windows := map[uint]season.Season{}
	for _, s := range m.allWindows() {
		windows[s.ID] = s
	}
	entries := make([]season.ArchiveEntry, 0, len(m.rows))
	for _, row := range m.rows {
		if row.UserID != userID || row.GamesPlayed < 1 {
			continue
		}
		w, ok := windows[row.SeasonID]
		if !ok || w.EndsAt.After(now) {
			continue
		}
		entries = append(entries, season.ArchiveEntry{
			SeasonID:    w.ID,
			SeasonName:  w.Name,
			StartedAt:   w.StartedAt,
			EndsAt:      w.EndsAt,
			SP:          row.SP,
			GamesPlayed: row.GamesPlayed,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].StartedAt.After(entries[j].StartedAt) })
	return entries, nil
}

// seed adds one visible player to the season under test.
func (m *mockRepo) seed(userID uint, username string, sp, gamesPlayed int) {
	m.rows[key(userID, testWindow.ID)] = &season.PlayerSeason{
		UserID: userID, SeasonID: testWindow.ID, SP: sp,
		RankTier: season.TierForSP(sp), GamesPlayed: gamesPlayed, GamesCompleted: gamesPlayed,
	}
	m.usernames[userID] = username
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

// --- Story 13.2: GET /api/v1/leaderboard ---

// callLeaderboard issues GET /leaderboard with the given raw query string and
// userID already on the context, the way the auth middleware leaves it.
func callLeaderboard(t *testing.T, repo *mockRepo, userID uint, rawQuery string) (*httptest.ResponseRecorder, error) {
	t.Helper()
	e := echo.New()
	target := "/api/v1/leaderboard"
	if rawQuery != "" {
		target += "?" + rawQuery
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if userID != 0 {
		c.Set("userID", userID)
	}
	h := season.NewHandler(season.NewService(repo))
	return rec, h.GetLeaderboard(c)
}

func decodeLeaderboard(t *testing.T, rec *httptest.ResponseRecorder) season.LeaderboardView {
	t.Helper()
	var env struct {
		Data season.LeaderboardView `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env), "response must be wrapped in a data envelope")
	return env.Data
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// seedLadder fills the season with n players named p1..pn, descending SP so
// user 1 is first. SP steps of 100 keep every player untied.
func seedLadder(repo *mockRepo, n int) {
	for i := 1; i <= n; i++ {
		repo.seed(uint(i), fmt.Sprintf("p%d", i), (n-i+1)*100, i)
	}
}

// The same gate TestGetCurrentSeason_WirePayloadKeysAreExact is: `decode` above
// unmarshals into the handler's own structs, so it agrees with whatever spelling
// they currently carry. Rename `gamesPlayed` and every Go and TS test still
// passes while the client silently renders `undefined`.
//
// Assert the LITERAL wire keys for all three shapes -- envelope, row and viewer --
// since the client's LeaderboardResponse is hand-maintained against them and
// there is no golden for HTTP payloads.
func TestGetLeaderboard_WirePayloadKeysAreExact(t *testing.T) {
	repo := newMockRepo(testWindow)
	seedLadder(repo, 3)

	rec, err := callLeaderboard(t, repo, 2, "season=current")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	var env map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.Contains(t, env, "data", "the response is always wrapped in a data envelope")
	assert.Len(t, env, 1, "nothing rides beside `data`")

	data, ok := env["data"].(map[string]any)
	require.True(t, ok, "data must be an object")
	assert.Equal(t, []string{"items", "limit", "offset", "total", "viewer"}, sortedKeys(data),
		"exact envelope key set -- the paginated shape is {items,total,limit,offset} plus viewer")

	items, ok := data["items"].([]any)
	require.True(t, ok, "items must be an array")
	require.Len(t, items, 3)
	row, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []string{"gamesPlayed", "position", "sp", "tier", "userId", "username"},
		sortedKeys(row), "exact row key set")

	viewer, ok := data["viewer"].(map[string]any)
	require.True(t, ok, "viewer must be an object when the caller has SP")
	// No `username`: the viewer is the authenticated caller and the client
	// already holds their name (Story 13.2 D3).
	assert.Equal(t, []string{"gamesPlayed", "position", "sp", "tier", "userId"},
		sortedKeys(viewer), "the viewer block deliberately carries no username")

	// A tag that survives but changes shape is the same class of silent break.
	assert.IsType(t, "", row["username"])
	assert.IsType(t, "", row["tier"])
	for _, numeric := range []string{"position", "userId", "sp", "gamesPlayed"} {
		assert.IsType(t, float64(0), row[numeric], "%s must be a JSON number", numeric)
	}
	for _, numeric := range []string{"total", "limit", "offset"} {
		assert.IsType(t, float64(0), data[numeric], "%s must be a JSON number", numeric)
	}
}

// The lobby widget sends no limit at all and must get a TOP TEN.
func TestGetLeaderboard_DefaultsToTenRows(t *testing.T) {
	repo := newMockRepo(testWindow)
	seedLadder(repo, 25)

	rec, err := callLeaderboard(t, repo, 1, "season=current")
	require.NoError(t, err)

	got := decodeLeaderboard(t, rec)
	assert.Equal(t, 10, got.Limit)
	assert.Equal(t, 0, got.Offset)
	assert.Equal(t, int64(25), got.Total, "total is the season's row count, not the page length")
	require.Len(t, got.Items, 10)
	for i, row := range got.Items {
		assert.Equal(t, i+1, row.Position)
	}
}

// The `season` selector is accepted when absent, so a client that omits it gets
// the active window rather than a 400.
func TestGetLeaderboard_SeasonSelectorIsOptional(t *testing.T) {
	repo := newMockRepo(testWindow)
	seedLadder(repo, 3)

	rec, err := callLeaderboard(t, repo, 1, "")
	require.NoError(t, err)
	assert.Len(t, decodeLeaderboard(t, rec).Items, 3)
}

// Positions are absolute in the season order, NOT indices into the page -- that
// is the whole point of echoing `offset` back.
func TestGetLeaderboard_ExplicitPagingNumbersRowsAbsolutely(t *testing.T) {
	repo := newMockRepo(testWindow)
	seedLadder(repo, 70)

	rec, err := callLeaderboard(t, repo, 1, "season=current&limit=20&offset=40")
	require.NoError(t, err)

	got := decodeLeaderboard(t, rec)
	assert.Equal(t, 20, got.Limit)
	assert.Equal(t, 40, got.Offset)
	require.Len(t, got.Items, 20)
	assert.Equal(t, 41, got.Items[0].Position)
	assert.Equal(t, 60, got.Items[19].Position)
	assert.Equal(t, uint(41), got.Items[0].UserID, "row 41 of a descending ladder is player 41")
}

// Every malformed parameter is a 400, never a silent coercion to the default:
// a client bug must surface, not quietly serve the wrong page.
func TestGetLeaderboard_RejectsBadQueryParams(t *testing.T) {
	cases := []struct {
		name  string
		query string
	}{
		{"limit below the floor", "limit=0"},
		{"limit above the cap", "limit=51"},
		{"limit not a number", "limit=abc"},
		{"limit negative", "limit=-5"},
		{"offset negative", "offset=-1"},
		{"offset not a number", "offset=x"},
		{"season is a quarter token", "season=2026Q1"},
		{"season is a bare word", "season=previous"},
		// Story 13.3 opened the selector to POSITIVE INTEGERS — everything
		// below is still malformed, not merely unknown, so it must 400 before
		// the database is touched (an unknown-but-well-formed id is the 404
		// tested separately).
		{"season not a number", "season=abc"},
		{"season negative", "season=-1"},
		{"season zero", "season=0"},
		{"season with a sign", "season=+5"},
		{"season a decimal", "season=1.5"},
		// Ids are 32-bit SERIALs. A 64-bit parse followed by uint() would
		// TRUNCATE these on a 32-bit build and serve a DIFFERENT season's
		// standings under the requested id; bit size 32 makes them 400s
		// everywhere. 4294967296 is 2^32 exactly -- the first value that lies.
		{"season above the 32-bit range", "season=4294967296"},
		{"season absurdly large", "season=99999999999999999999"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMockRepo(testWindow)
			seedLadder(repo, 3)

			rec, err := callLeaderboard(t, repo, 1, tc.query)
			require.Error(t, err)
			assert.ErrorIs(t, err, apperr.ErrBadRequest)
			assert.Empty(t, rec.Body.String(), "a rejected request writes no body")
			assert.Zero(t, repo.pageCalls, "validation runs before the repository is touched")
		})
	}
}

// The boundary values on the accepted side of each bound.
func TestGetLeaderboard_AcceptsTheBoundaryLimits(t *testing.T) {
	for _, limit := range []int{1, 50} {
		repo := newMockRepo(testWindow)
		seedLadder(repo, 60)

		rec, err := callLeaderboard(t, repo, 1, fmt.Sprintf("season=current&limit=%d", limit))
		require.NoError(t, err)
		assert.Equal(t, limit, decodeLeaderboard(t, rec).Limit)
	}
}

// An empty season is a normal 200 with an EMPTY ARRAY -- never `null`, which the
// client would have to guard on every map().
func TestGetLeaderboard_EmptySeasonSerializesAnEmptyArray(t *testing.T) {
	repo := newMockRepo(testWindow)

	rec, err := callLeaderboard(t, repo, 42, "season=current")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	assert.Contains(t, rec.Body.String(), `"items":[]`, "an empty page must serialize as [], not null")
	got := decodeLeaderboard(t, rec)
	assert.Empty(t, got.Items)
	assert.Equal(t, int64(0), got.Total)
	assert.Nil(t, got.Viewer)
}

// Offset past the end: an empty page, but the TOTAL is unchanged so the client
// can still tell how long the list is.
func TestGetLeaderboard_OffsetPastTheEndKeepsTheTotal(t *testing.T) {
	repo := newMockRepo(testWindow)
	seedLadder(repo, 5)

	rec, err := callLeaderboard(t, repo, 1, "season=current&offset=99")
	require.NoError(t, err)

	got := decodeLeaderboard(t, rec)
	assert.Empty(t, got.Items)
	assert.Equal(t, int64(5), got.Total)
	assert.Contains(t, rec.Body.String(), `"items":[]`)
}

// AC4: a viewer who never played has no own-row marker and nothing pinned, and
// the request still succeeds.
func TestGetLeaderboard_ViewerWhoNeverPlayedIsNull(t *testing.T) {
	repo := newMockRepo(testWindow)
	seedLadder(repo, 3)

	rec, err := callLeaderboard(t, repo, 999, "season=current")
	require.NoError(t, err)

	got := decodeLeaderboard(t, rec)
	assert.Nil(t, got.Viewer)
	assert.Len(t, got.Items, 3, "the list is unaffected by the viewer having no standing")
	assert.Zero(t, repo.aheadCalls, "no position is counted for a player with no row")
}

// AC4 again, the case a `record != nil` check alone would get wrong: the row
// EXISTS (they played) but carries no SP, and the AC marks an own-row only for a
// player with ANY SP.
func TestGetLeaderboard_ViewerWithZeroSPIsNull(t *testing.T) {
	repo := newMockRepo(testWindow)
	seedLadder(repo, 3)
	repo.seed(50, "zero", 0, 4)

	rec, err := callLeaderboard(t, repo, 50, "season=current")
	require.NoError(t, err)

	got := decodeLeaderboard(t, rec)
	assert.Nil(t, got.Viewer, "0 SP is a real Iron player, but has no ladder position to pin")
	assert.Zero(t, repo.aheadCalls)

	// THE HALF THIS TEST USED TO MISS, and exactly why the 0-SP leak survived
	// review: asserting only `Viewer` allowed the 0-SP row to be LISTED at a real
	// position while its owner was told they had no standing. The list and the
	// viewer block have to agree, so both halves are asserted now.
	assert.Len(t, got.Items, 3, "the 0-SP row is not on the ladder either")
	assert.Equal(t, int64(3), got.Total, "and it does not inflate the total")
	for _, row := range got.Items {
		assert.NotEqual(t, uint(50), row.UserID)
	}
}

// The `viewer` KEY is always present, even when null: the client distinguishes
// "no standing" from "a server that does not send this field".
func TestGetLeaderboard_ViewerKeyIsPresentWhenNull(t *testing.T) {
	repo := newMockRepo(testWindow)

	rec, err := callLeaderboard(t, repo, 42, "season=current")
	require.NoError(t, err)

	var env struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.Contains(t, env.Data, "viewer", "the key must be emitted, never omitempty'd away")
	assert.Nil(t, env.Data["viewer"])
}

// The viewer's position and their own row in the list must be the SAME NUMBER.
// They come from two different queries, so nothing but a shared order makes them
// agree.
func TestGetLeaderboard_ViewerPositionMatchesTheirRowOnPage(t *testing.T) {
	repo := newMockRepo(testWindow)
	seedLadder(repo, 10)

	rec, err := callLeaderboard(t, repo, 4, "season=current")
	require.NoError(t, err)

	got := decodeLeaderboard(t, rec)
	require.NotNil(t, got.Viewer)
	assert.Equal(t, uint(4), got.Viewer.UserID)

	var own *season.LeaderboardRowView
	for i := range got.Items {
		if got.Items[i].UserID == 4 {
			own = &got.Items[i]
		}
	}
	require.NotNil(t, own, "the viewer is inside the returned page")
	assert.Equal(t, own.Position, got.Viewer.Position)
	assert.Equal(t, own.SP, got.Viewer.SP)
	assert.Equal(t, own.Tier, got.Viewer.Tier)
	assert.Equal(t, own.GamesPlayed, got.Viewer.GamesPlayed)
}

// THE CASE THAT KILLS `COUNT(sp > x)`: three players tied at 900 SP. The list
// numbers them 1,2,3 by ascending user id, and a tied viewer's `position` must
// be their OWN slot -- not the 1 that every tied player would share.
func TestGetLeaderboard_TiedPlayersGetDistinctAgreeingPositions(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.seed(7, "seven", 900, 5)
	repo.seed(3, "three", 900, 5)
	repo.seed(5, "five", 900, 5)

	rec, err := callLeaderboard(t, repo, 7, "season=current")
	require.NoError(t, err)

	got := decodeLeaderboard(t, rec)
	require.Len(t, got.Items, 3)
	assert.Equal(t, []uint{3, 5, 7}, []uint{got.Items[0].UserID, got.Items[1].UserID, got.Items[2].UserID},
		"ties break by ASCENDING user_id")
	assert.Equal(t, []int{1, 2, 3}, []int{got.Items[0].Position, got.Items[1].Position, got.Items[2].Position})

	require.NotNil(t, got.Viewer)
	assert.Equal(t, 3, got.Viewer.Position,
		"the tied viewer sits in slot 3 of the list they are looking at -- a COUNT(sp > x) would say 1")
}

// A soft-deleted player (no `users` row the join can see) is missing from the
// items, from the total, AND from every position -- all three, or the numbers
// contradict each other.
func TestGetLeaderboard_InvisibleUserIsExcludedEverywhere(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.seed(1, "top", 5000, 10)
	repo.seed(2, "second", 3000, 10)
	// A row with no username -- the mock's stand-in for users.deleted_at NOT NULL.
	repo.rows[key(9, testWindow.ID)] = &season.PlayerSeason{
		UserID: 9, SeasonID: testWindow.ID, SP: 99000, GamesPlayed: 1,
	}

	rec, err := callLeaderboard(t, repo, 2, "season=current")
	require.NoError(t, err)

	got := decodeLeaderboard(t, rec)
	assert.Equal(t, int64(2), got.Total, "the deleted account is not counted in the total")
	require.Len(t, got.Items, 2)
	for _, row := range got.Items {
		assert.NotEqual(t, uint(9), row.UserID, "the deleted account is not listed")
	}
	require.NotNil(t, got.Viewer)
	assert.Equal(t, 2, got.Viewer.Position,
		"the deleted account does not push the viewer down a slot")
}

// D7: every response tier is DERIVED from `sp`, never the stored rank_tier
// column, in the rows AND in the viewer block.
func TestGetLeaderboard_TierIsDerivedNotTheStoredColumn(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.seed(1, "gm", 20000, 40)
	repo.seed(2, "viewer", 9000, 30)
	// Deliberately wrong snapshots, as a lagging column would be.
	repo.rows[key(1, testWindow.ID)].RankTier = "iron"
	repo.rows[key(2, testWindow.ID)].RankTier = "iron"

	rec, err := callLeaderboard(t, repo, 2, "season=current")
	require.NoError(t, err)

	got := decodeLeaderboard(t, rec)
	require.Len(t, got.Items, 2)
	assert.Equal(t, "grandmaster", got.Items[0].Tier, "20000 SP is Grandmaster, whatever the column says")
	assert.Equal(t, "diamond", got.Items[1].Tier)
	require.NotNil(t, got.Viewer)
	assert.Equal(t, "diamond", got.Viewer.Tier)
}

// The endpoint is keyed off the JWT subject only -- with no authenticated user it
// is a 401, and the repository is never reached.
func TestGetLeaderboard_RequiresAuth(t *testing.T) {
	repo := newMockRepo(testWindow)
	seedLadder(repo, 3)

	_, err := callLeaderboard(t, repo, 0, "season=current")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrUnauthorized)
	assert.Zero(t, repo.pageCalls)
}

// A READ MUST NOT WRITE. FindPlayerSeason's contract is explicit that a GET which
// lazily created a player_seasons row would put everyone who merely opened the
// lobby onto this very leaderboard -- so the read path is asserted to leave the
// row set exactly as it found it.
func TestGetLeaderboard_ReadCreatesNoPlayerRecord(t *testing.T) {
	repo := newMockRepo(testWindow)
	seedLadder(repo, 3)
	before := len(repo.rows)

	// A caller with no row of their own is the case that would create one.
	_, err := callLeaderboard(t, repo, 4242, "season=current")
	require.NoError(t, err)

	assert.Len(t, repo.rows, before, "the leaderboard read must not materialise a player_seasons row")
	assert.NotContains(t, repo.rows, key(4242, testWindow.ID))
}

// A repository failure surfaces as a plain wrapped error (a 500 through
// appErrorHandler), never as an auth or bad-request error.
func TestGetLeaderboard_RepositoryFailureSurfaces(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.pageErr = errors.New("db down")

	_, err := callLeaderboard(t, repo, 1, "season=current")
	require.Error(t, err)
	assert.NotErrorIs(t, err, apperr.ErrUnauthorized)
	assert.NotErrorIs(t, err, apperr.ErrBadRequest)
}

func TestGetLeaderboard_CountAheadFailureSurfaces(t *testing.T) {
	repo := newMockRepo(testWindow)
	seedLadder(repo, 3)
	repo.countErr = errors.New("db down")

	_, err := callLeaderboard(t, repo, 1, "season=current")
	require.Error(t, err)
	assert.NotErrorIs(t, err, apperr.ErrBadRequest)
}

// The same contract-violation guard the current-season path has: a repository
// that returns (nil, nil) must produce an error, not a nil dereference.
func TestGetLeaderboard_NilSeasonIsAnErrorNotAPanic(t *testing.T) {
	repo := newMockRepo(nil)

	_, err := callLeaderboard(t, repo, 42, "season=current")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no season")
}

// --- Review follow-ups (P1, P7, P9, P12) ---

// P12: nothing previously proved that the handler's PARSED parameters, and the
// resolved season id, arrive at the repository unchanged. A parse that clamped
// silently, or a service that swapped limit and offset, would still produce a
// well-formed response — the numbers would just describe a different page than
// the one asked for.
func TestGetLeaderboard_ParsedParamsReachTheRepositoryUnchanged(t *testing.T) {
	repo := newMockRepo(testWindow)
	seedLadder(repo, 60)

	_, err := callLeaderboard(t, repo, 1, "season=current&limit=20&offset=40")
	require.NoError(t, err)

	assert.Equal(t, 1, repo.pageCalls, "exactly one page read per request")
	assert.Equal(t, [3]int{int(testWindow.ID), 20, 40}, repo.lastPageArgs,
		"the resolved season id, the parsed limit and the parsed offset, in that order")
}

// And the defaults travel just as literally: the widget sends no limit at all.
func TestGetLeaderboard_DefaultParamsReachTheRepositoryUnchanged(t *testing.T) {
	repo := newMockRepo(testWindow)
	seedLadder(repo, 60)

	_, err := callLeaderboard(t, repo, 1, "season=current")
	require.NoError(t, err)

	assert.Equal(t, [3]int{int(testWindow.ID), 10, 0}, repo.lastPageArgs)
}

// P10: `?limit=` and `?offset=` are PRESENT BUT EMPTY. They take the defaults
// rather than 400, matching parseMatchesQuery — so a client that interpolates an
// undefined value gets the same answer from both endpoints. Pinned because the
// doc now makes a claim about it.
func TestGetLeaderboard_EmptyParamValuesTakeTheDefaults(t *testing.T) {
	repo := newMockRepo(testWindow)
	seedLadder(repo, 30)

	rec, err := callLeaderboard(t, repo, 1, "season=current&limit=&offset=")
	require.NoError(t, err)

	got := decodeLeaderboard(t, rec)
	assert.Equal(t, 10, got.Limit, "an empty value is an absent value")
	assert.Equal(t, 0, got.Offset)
	assert.Equal(t, [3]int{int(testWindow.ID), 10, 0}, repo.lastPageArgs)
}

// P9: `offset` had two bounds fewer than `limit`. A caller-chosen offset in the
// billions parses cleanly and makes Postgres sort and discard that many rows, so
// the cost of a request grew with a number the client picked freely.
func TestGetLeaderboard_RejectsAnOffsetPastTheCeiling(t *testing.T) {
	cases := []string{
		"offset=10001",
		"offset=9223372036854775807",
		"offset=99999999",
	}
	for _, query := range cases {
		t.Run(query, func(t *testing.T) {
			repo := newMockRepo(testWindow)
			seedLadder(repo, 3)

			_, err := callLeaderboard(t, repo, 1, "season=current&"+query)
			require.Error(t, err)
			assert.ErrorIs(t, err, apperr.ErrBadRequest)
			assert.Zero(t, repo.pageCalls, "rejected before the database is touched")
		})
	}
}

// The accepted side of the same bound.
func TestGetLeaderboard_AcceptsTheOffsetCeilingItself(t *testing.T) {
	repo := newMockRepo(testWindow)
	seedLadder(repo, 3)

	rec, err := callLeaderboard(t, repo, 1, "season=current&offset=10000")
	require.NoError(t, err)
	assert.Equal(t, 10000, decodeLeaderboard(t, rec).Offset)
}

// P1: THE LADDER IS SP EARNERS ONLY. A 0-SP row is written for every seat absent
// at a match end, so these are ordinary rows, not corruption. Listing one would
// give a player a real position while the viewer block told that same player
// they had no standing, and would falsify the empty state's copy.
func TestGetLeaderboard_ZeroSPRowsAreNotOnTheLadder(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.seed(1, "earner", 500, 5)
	repo.seed(2, "absentee", 0, 3)
	repo.seed(3, "other", 250, 4)

	rec, err := callLeaderboard(t, repo, 1, "season=current")
	require.NoError(t, err)

	got := decodeLeaderboard(t, rec)
	assert.Equal(t, int64(2), got.Total, "the 0-SP row is excluded from the total")
	require.Len(t, got.Items, 2)
	assert.Equal(t, []uint{1, 3}, []uint{got.Items[0].UserID, got.Items[1].UserID})
	assert.Equal(t, []int{1, 2}, []int{got.Items[0].Position, got.Items[1].Position},
		"positions close up — an excluded row leaves no gap")
}

// A season whose ONLY rows are 0-SP reads as empty, which is what makes the
// empty-state copy ("Nobody has earned Season Points yet") true.
func TestGetLeaderboard_SeasonOfOnlyZeroSPRowsReadsAsEmpty(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.seed(1, "a", 0, 2)
	repo.seed(2, "b", 0, 1)

	rec, err := callLeaderboard(t, repo, 42, "season=current")
	require.NoError(t, err)

	got := decodeLeaderboard(t, rec)
	assert.Empty(t, got.Items)
	assert.Equal(t, int64(0), got.Total)
	assert.Nil(t, got.Viewer)
	assert.Contains(t, rec.Body.String(), `"items":[]`)
}

// The agreement that the exclusion must not break: with a 0-SP row present in
// the season, every listed row's position still equals its own CountAhead + 1.
func TestGetLeaderboard_PositionsStillAgreeWithAZeroSPRowPresent(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.seed(1, "top", 5000, 9)
	repo.seed(2, "absentee", 0, 1)
	repo.seed(3, "mid", 900, 7)
	repo.seed(4, "tied", 900, 7)

	// Each listed player asks for their own standing in turn.
	for _, viewerID := range []uint{1, 3, 4} {
		rec, err := callLeaderboard(t, repo, viewerID, "season=current")
		require.NoError(t, err)
		got := decodeLeaderboard(t, rec)

		require.NotNil(t, got.Viewer, "user %d earned SP and must have a standing", viewerID)
		var own *season.LeaderboardRowView
		for i := range got.Items {
			if got.Items[i].UserID == viewerID {
				own = &got.Items[i]
			}
		}
		require.NotNil(t, own)
		assert.Equal(t, own.Position, got.Viewer.Position,
			"user %d: the viewer block and the list must not disagree", viewerID)
	}
}

// P7: a SOFT-DELETED account holding an unexpired JWT used to receive a viewer
// block (FindPlayerSeason has no `users` join) while being absent from the list —
// a position counted against a population that excluded it, plus a pinned row the
// caller could never find. The viewer now runs through the list's own predicate.
func TestGetLeaderboard_SoftDeletedViewerHasNoStanding(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.seed(1, "alive", 1000, 10)
	// A row with SP but no username entry: the mock's soft-deleted account.
	repo.rows[key(9, testWindow.ID)] = &season.PlayerSeason{
		UserID: 9, SeasonID: testWindow.ID, SP: 50000, GamesPlayed: 40,
	}

	rec, err := callLeaderboard(t, repo, 9, "season=current")
	require.NoError(t, err)

	got := decodeLeaderboard(t, rec)
	assert.Nil(t, got.Viewer,
		"a player who is not listable has no standing — no phantom position, no pinned row")
	require.Len(t, got.Items, 1)
	assert.Equal(t, uint(1), got.Items[0].UserID)
	assert.Zero(t, repo.aheadCalls, "and no position is counted for them")
}

// The viewer lookup failing is a 500, not a 400 and not a silent nil standing.
func TestGetLeaderboard_ViewerLookupFailureSurfaces(t *testing.T) {
	repo := newMockRepo(testWindow)
	seedLadder(repo, 3)
	repo.entryErr = errors.New("db down")

	_, err := callLeaderboard(t, repo, 1, "season=current")
	require.Error(t, err)
	assert.NotErrorIs(t, err, apperr.ErrBadRequest)
	assert.NotErrorIs(t, err, apperr.ErrUnauthorized)
}

// --- Story 13.3: prior-season selector, GET /seasons, GET /users/:id/seasons ---

// endedWindow is the quarter BEFORE testWindow — the archive's and the
// prior-season selector's subject.
var endedWindow = &season.Season{
	ID:        5,
	Name:      "2026 Q2",
	StartedAt: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
	EndsAt:    time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
}

// seedEnded drops one player into endedWindow directly (mockRepo.seed only
// writes into testWindow).
func seedEnded(repo *mockRepo, userID uint, username string, sp, gamesPlayed int) {
	repo.rows[key(userID, endedWindow.ID)] = &season.PlayerSeason{
		UserID: userID, SeasonID: endedWindow.ID, SP: sp,
		RankTier: season.TierForSP(sp), GamesPlayed: gamesPlayed, GamesCompleted: gamesPlayed,
	}
	repo.usernames[userID] = username
}

func callSeasons(t *testing.T, repo *mockRepo, userID uint) (*httptest.ResponseRecorder, error) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/seasons", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if userID != 0 {
		c.Set("userID", userID)
	}
	h := season.NewHandler(season.NewService(repo))
	return rec, h.GetSeasons(c)
}

func callArchive(t *testing.T, repo *mockRepo, userID uint, subjectParam string) (*httptest.ResponseRecorder, error) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+subjectParam+"/seasons", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(subjectParam)
	if userID != 0 {
		c.Set("userID", userID)
	}
	h := season.NewHandler(season.NewService(repo))
	return rec, h.GetPlayerSeasonArchive(c)
}

// A WELL-FORMED but unknown ?season=<id> is a 404 SEASON_NOT_FOUND with no
// body — a MISS, distinct from the 400 a malformed selector gets, and never a
// silent fallback to the current window.
func TestGetLeaderboard_UnknownSeasonIdIs404(t *testing.T) {
	repo := newMockRepo(testWindow)
	seedLadder(repo, 3)

	rec, err := callLeaderboard(t, repo, 1, "season=999")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrSeasonNotFound)
	assert.Empty(t, rec.Body.String(), "a rejected request writes no body")
	assert.Zero(t, repo.pageCalls, "no page is read for a season that does not exist")
}

// Picking an ENDED season renders THAT season's standings — and the viewer
// block runs under the same sp > 0 rule it has on the current window.
func TestGetLeaderboard_EndedSeasonByIdRendersItsStandings(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.seasons = []season.Season{*endedWindow}
	// Current window: a ladder that must NOT leak into the prior season's view.
	seedLadder(repo, 3)
	// The ended season: two earners and the viewer at 0 SP.
	seedEnded(repo, 21, "past-top", 4000, 12)
	seedEnded(repo, 22, "past-second", 900, 8)
	seedEnded(repo, 23, "past-zero", 0, 3)

	rec, err := callLeaderboard(t, repo, 22, "season=5")
	require.NoError(t, err)

	got := decodeLeaderboard(t, rec)
	assert.Equal(t, int64(2), got.Total, "the prior season's OWN population, sp > 0 only")
	require.Len(t, got.Items, 2)
	assert.Equal(t, []uint{21, 22}, []uint{got.Items[0].UserID, got.Items[1].UserID})
	assert.Equal(t, "gold", got.Items[0].Tier, "tier derived from the frozen SP")

	require.NotNil(t, got.Viewer, "the viewer earned SP in that season")
	assert.Equal(t, 2, got.Viewer.Position)
	assert.Equal(t, 900, got.Viewer.SP)

	// And the resolved id — not the current window's — reached the repository.
	assert.Equal(t, [3]int{int(endedWindow.ID), 10, 0}, repo.lastPageArgs)
	assert.Zero(t, repo.currentCalls, "a by-id read never resolves (or creates) the current window")
}

// The current window's own id is also a legal selector — an id is an id.
func TestGetLeaderboard_CurrentSeasonByIdWorksToo(t *testing.T) {
	repo := newMockRepo(testWindow)
	seedLadder(repo, 3)

	rec, err := callLeaderboard(t, repo, 1, fmt.Sprintf("season=%d", testWindow.ID))
	require.NoError(t, err)
	assert.Len(t, decodeLeaderboard(t, rec).Items, 3)
	assert.Equal(t, [3]int{int(testWindow.ID), 10, 0}, repo.lastPageArgs)
}

// THE JSON TAGS ARE THE CONTRACT — the same gate the other two wire tests are.
// GET /seasons feeds the picker; the client's SeasonsListResponse is
// hand-maintained against these literal keys.
func TestGetSeasons_WirePayloadKeysAreExact(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.seasons = []season.Season{*endedWindow}

	rec, err := callSeasons(t, repo, 42)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	var env map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.Contains(t, env, "data")
	assert.Len(t, env, 1, "nothing rides beside `data`")

	data, ok := env["data"].(map[string]any)
	require.True(t, ok, "data must be an object")
	assert.Equal(t, []string{"items"}, sortedKeys(data))

	items, ok := data["items"].([]any)
	require.True(t, ok, "items must be an array")
	require.Len(t, items, 2)
	row, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []string{"endsAt", "id", "name", "startedAt"}, sortedKeys(row),
		"exact wire key set for a season row")

	assert.IsType(t, float64(0), row["id"])
	assert.IsType(t, "", row["name"])
	for _, ts := range []string{"startedAt", "endsAt"} {
		_, parseErr := time.Parse(time.RFC3339, row[ts].(string))
		assert.NoError(t, parseErr, "%s must be an absolute ISO 8601 timestamp", ts)
	}
}

// Newest-first — the order the picker renders verbatim — and the current
// window is present because the read resolves it before listing (the same lazy
// self-heal every other read leans on).
func TestGetSeasons_NewestFirstIncludingCurrent(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.seasons = []season.Season{*endedWindow}

	rec, err := callSeasons(t, repo, 42)
	require.NoError(t, err)

	var env struct {
		Data season.SeasonsListView `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.Len(t, env.Data.Items, 2)
	assert.Equal(t, "2026 Q3", env.Data.Items[0].Name, "the current window leads")
	assert.Equal(t, "2026 Q2", env.Data.Items[1].Name)
	assert.Positive(t, repo.currentCalls, "the listing resolves the current window first")
}

func TestGetSeasons_RequiresAuth(t *testing.T) {
	repo := newMockRepo(testWindow)
	_, err := callSeasons(t, repo, 0)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrUnauthorized)
}

func TestGetSeasons_ResolverFailureSurfaces(t *testing.T) {
	repo := newMockRepo(nil)
	repo.currentErr = errors.New("db down")

	_, err := callSeasons(t, repo, 42)
	require.Error(t, err)
	assert.NotErrorIs(t, err, apperr.ErrUnauthorized)
}

// The archive payload's exact wire keys — the client's SeasonArchiveResponse is
// hand-maintained against them, and the matrix names all seven.
func TestGetPlayerSeasonArchive_WirePayloadKeysAreExact(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.seasons = []season.Season{*endedWindow}
	seedEnded(repo, 42, "archiver", 1800, 14)

	rec, err := callArchive(t, repo, 7, "42")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	var env map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.Contains(t, env, "data")
	assert.Len(t, env, 1)

	data, ok := env["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []string{"items"}, sortedKeys(data))

	items, ok := data["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	row, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t,
		[]string{"endsAt", "gamesPlayed", "seasonId", "seasonName", "sp", "startedAt", "tier"},
		sortedKeys(row), "exact wire key set for an archive row")

	assert.IsType(t, "", row["seasonName"])
	assert.IsType(t, "", row["tier"])
	assert.Equal(t, "silver", row["tier"], "1800 SP derives Silver — never the stored column")
	for _, numeric := range []string{"seasonId", "sp", "gamesPlayed"} {
		assert.IsType(t, float64(0), row[numeric], "%s must be a JSON number", numeric)
	}
	for _, ts := range []string{"startedAt", "endsAt"} {
		_, parseErr := time.Parse(time.RFC3339, row[ts].(string))
		assert.NoError(t, parseErr, "%s must be an absolute ISO 8601 timestamp", ts)
	}
}

// The archive's membership through the HTTP surface: the ACTIVE season is
// excluded, a played-but-0-SP ended season is included, newest-first.
func TestGetPlayerSeasonArchive_ActiveExcludedZeroSPKept(t *testing.T) {
	repo := newMockRepo(testWindow)
	older := season.Season{
		ID:        4,
		Name:      "2026 Q1",
		StartedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		EndsAt:    time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
	}
	repo.seasons = []season.Season{*endedWindow, older}
	// Active season: must not appear.
	repo.seed(42, "archiver", 5000, 9)
	// Ended seasons: one earned, one played at 0 SP — BOTH archive rows.
	seedEnded(repo, 42, "archiver", 900, 8)
	repo.rows[key(42, older.ID)] = &season.PlayerSeason{
		UserID: 42, SeasonID: older.ID, SP: 0, RankTier: "iron", GamesPlayed: 2,
	}

	rec, err := callArchive(t, repo, 42, "42")
	require.NoError(t, err)

	var env struct {
		Data season.ArchiveView `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.Len(t, env.Data.Items, 2, "the active season is not history yet")

	assert.Equal(t, "2026 Q2", env.Data.Items[0].SeasonName, "newest-first")
	assert.Equal(t, 900, env.Data.Items[0].SP)
	assert.Equal(t, "bronze", env.Data.Items[0].Tier)

	assert.Equal(t, "2026 Q1", env.Data.Items[1].SeasonName)
	assert.Equal(t, 0, env.Data.Items[1].SP, "a played 0-SP season stays in the archive")
	assert.Equal(t, "iron", env.Data.Items[1].Tier)
	assert.Equal(t, 2, env.Data.Items[1].GamesPlayed)
}

// An unknown subject is `{items: []}` with a 200 — DELIBERATELY no
// user-existence 404 (the profile query owns that surface) — and the empty
// slice serializes as [], never null.
func TestGetPlayerSeasonArchive_UnknownUserIsEmpty200(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.seasons = []season.Season{*endedWindow}

	rec, err := callArchive(t, repo, 7, "424242")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"items":[]`, "an empty archive must serialize as [], not null")
}

// Malformed subject ids are 400s, mirroring every other :id route.
func TestGetPlayerSeasonArchive_RejectsBadSubjectIds(t *testing.T) {
	// 4294967296 is 2^32: parsed at 64 bits and cast to uint it would truncate
	// to a DIFFERENT user's id on a 32-bit build and serve their archive.
	for _, bad := range []string{"abc", "0", "-1", "1.5", "4294967296", "99999999999999999999"} {
		t.Run(bad, func(t *testing.T) {
			repo := newMockRepo(testWindow)
			_, err := callArchive(t, repo, 7, bad)
			require.Error(t, err)
			assert.ErrorIs(t, err, apperr.ErrBadRequest)
			assert.Zero(t, repo.archiveCalls, "validation runs before the repository is touched")
		})
	}
}

func TestGetPlayerSeasonArchive_RequiresAuth(t *testing.T) {
	repo := newMockRepo(testWindow)
	_, err := callArchive(t, repo, 0, "42")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrUnauthorized)
	assert.Zero(t, repo.archiveCalls)
}

func TestGetPlayerSeasonArchive_RepositoryFailureSurfaces(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.archiveErr = errors.New("db down")

	_, err := callArchive(t, repo, 7, "42")
	require.Error(t, err)
	assert.NotErrorIs(t, err, apperr.ErrBadRequest)
	assert.NotErrorIs(t, err, apperr.ErrUnauthorized)
}

// --- Story 13.3: CurrentSeasonRank (the profile's narrow reader) ---

// The service satisfies user.SeasonRankReader structurally; these pin the
// contract the profile depends on without importing `user` (season_test must
// not — the edge is one-way).
func TestCurrentSeasonRank_NilWhenNeverPlayed(t *testing.T) {
	repo := newMockRepo(testWindow)
	svc := season.NewService(repo)

	rank, err := svc.CurrentSeasonRank(42, time.Now().UTC())
	require.NoError(t, err)
	assert.Nil(t, rank, "no row means seasonRank: null, never a fabricated zero block")
	assert.Empty(t, repo.rows, "the read must not create a player_seasons row")
}

func TestCurrentSeasonRank_DerivesTierFromSP(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.rows[key(42, testWindow.ID)] = &season.PlayerSeason{
		UserID: 42, SeasonID: testWindow.ID, SP: 4000,
		RankTier:    "iron", // deliberately stale — must be ignored (D7)
		GamesPlayed: 30, GamesCompleted: 28,
	}
	svc := season.NewService(repo)

	rank, err := svc.CurrentSeasonRank(42, time.Now().UTC())
	require.NoError(t, err)
	require.NotNil(t, rank)
	assert.Equal(t, "2026 Q3", rank.SeasonName)
	assert.Equal(t, "gold", rank.Tier, "derived from SP, whatever the column says")
	assert.Equal(t, 4000, rank.SP)
}

// A 0-SP row is a REAL rank (Iron) — the row's existence gates the block, not
// leaderboardScope's sp > 0 membership rule.
func TestCurrentSeasonRank_ZeroSPRowIsIronNotNil(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.rows[key(42, testWindow.ID)] = &season.PlayerSeason{
		UserID: 42, SeasonID: testWindow.ID, SP: 0, GamesPlayed: 2,
	}
	svc := season.NewService(repo)

	rank, err := svc.CurrentSeasonRank(42, time.Now().UTC())
	require.NoError(t, err)
	require.NotNil(t, rank, "played-at-0-SP is Iron, not unranked")
	assert.Equal(t, "iron", rank.Tier)
	assert.Equal(t, 0, rank.SP)
}

func TestCurrentSeasonRank_ResolverFailureSurfaces(t *testing.T) {
	repo := newMockRepo(nil)
	repo.currentErr = errors.New("db down")
	svc := season.NewService(repo)

	rank, err := svc.CurrentSeasonRank(42, time.Now().UTC())
	require.Error(t, err)
	assert.Nil(t, rank)
}
