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

// ApplySeasonPoints mirrors the GORM transaction's shape: every listed player's
// current total (missing = 0) goes to `changes` at once, each new total is
// floored at 0, and the counters move. Nothing is written if `changes` fails.
func (m *mockRepo) ApplySeasonPoints(seasonID uint, completed map[uint]bool, changes season.SPChanges) (map[uint]season.PlayerSeasonSnapshot, error) {
	current := make(map[uint]int, len(completed))
	for userID := range completed {
		if row := m.rows[key(userID, seasonID)]; row != nil {
			current[userID] = row.SP
		} else {
			current[userID] = 0
		}
	}
	deltas, err := changes(current)
	if err != nil {
		return nil, err
	}

	out := make(map[uint]season.PlayerSeasonSnapshot, len(completed))
	for userID, present := range completed {
		row := m.rows[key(userID, seasonID)]
		played, done := 0, 0
		if row != nil {
			played, done = row.GamesPlayed, row.GamesCompleted
		}
		played++
		if present {
			done++
		}
		delta, ok := deltas[userID]
		if !ok {
			return nil, fmt.Errorf("mock: no change for user %d", userID)
		}
		prev := current[userID]
		next := season.ApplySPChange(prev, delta)
		tier, division := season.RankForSP(next)
		m.rows[key(userID, seasonID)] = &season.PlayerSeason{
			UserID: userID, SeasonID: seasonID, SP: next,
			RankTier: tier, RankDivision: divPtr(division), GamesPlayed: played, GamesCompleted: done,
		}
		out[userID] = season.PlayerSeasonSnapshot{
			SP: next, PreviousSP: prev, Tier: tier, Division: division,
			GamesPlayed: played, GamesCompleted: done,
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
// `users.deleted_at IS NOT NULL`, and `games_played < 1` is outside the
// played-this-season membership rule.
func (m *mockRepo) visibleRows(seasonID uint) []*season.PlayerSeason {
	out := make([]*season.PlayerSeason, 0, len(m.rows))
	for _, row := range m.rows {
		if row.SeasonID != seasonID {
			continue
		}
		if _, visible := m.usernames[row.UserID]; !visible {
			continue
		}
		// THE LADDER IS EVERYONE WHO PLAYED (Story 13.4). Mirrors the real
		// `player_seasons.games_played >= 1` in leaderboardScope: a player at
		// 0 SP who has played is listed.
		if row.GamesPlayed < 1 {
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
			UserID:       rows[i].UserID,
			Username:     m.usernames[rows[i].UserID],
			SP:           rows[i].SP,
			RankTier:     rows[i].RankTier,
			RankDivision: rows[i].RankDivision,
			GamesPlayed:  rows[i].GamesPlayed,
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
			UserID:       row.UserID,
			Username:     m.usernames[row.UserID],
			SP:           row.SP,
			RankTier:     row.RankTier,
			RankDivision: row.RankDivision,
			GamesPlayed:  row.GamesPlayed,
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
// that dropped either half would fail here too.
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
			SeasonID:     w.ID,
			SeasonName:   w.Name,
			StartedAt:    w.StartedAt,
			EndsAt:       w.EndsAt,
			SP:           row.SP,
			RankTier:     row.RankTier,
			RankDivision: row.RankDivision,
			GamesPlayed:  row.GamesPlayed,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].StartedAt.After(entries[j].StartedAt) })
	return entries, nil
}

// seed adds one visible player to the season under test, with the rank
// snapshot an award at that total would have written.
func (m *mockRepo) seed(userID uint, username string, sp, gamesPlayed int) {
	tier, division := season.RankForSP(sp)
	m.rows[key(userID, testWindow.ID)] = &season.PlayerSeason{
		UserID: userID, SeasonID: testWindow.ID, SP: sp,
		RankTier: tier, RankDivision: divPtr(division), GamesPlayed: gamesPlayed, GamesCompleted: gamesPlayed,
	}
	m.usernames[userID] = username
}

// divPtr mirrors the repository's NULL for "no division" (Master and
// Grandmaster report 0).
func divPtr(division int) *int {
	if division == 0 {
		return nil
	}
	return &division
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
		UserID: 42, SeasonID: testWindow.ID, SP: 680,
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
		"rankDivision",
		"rankTier",
		"seasonName",
		"sp",
		"spForNextDivision",
		"spIntoDivision",
	}, got, "exact wire key set — a renamed or dropped tag breaks the client silently")

	// Spot-check the types too: a tag that survives but changes shape (an int
	// serialised as a string, say) is the same class of silent break.
	assert.IsType(t, "", data["seasonName"])
	assert.IsType(t, "", data["endsAt"])
	assert.IsType(t, "", data["rankTier"])
	for _, numeric := range []string{"sp", "rankDivision", "spIntoDivision", "spForNextDivision", "gamesPlayed", "gamesCompleted"} {
		assert.IsType(t, float64(0), data[numeric], "%s must be a JSON number", numeric)
	}
	assert.Equal(t, float64(2), data["rankDivision"], "680 SP is Gold 2")
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
	require.NotNil(t, got.RankDivision)
	assert.Equal(t, 1, *got.RankDivision, "0 SP is Iron 1")
	assert.Equal(t, 0, got.SPIntoDivision)
	assert.Equal(t, 50, got.SPForNextDivision, "the whole Iron 1 step to climb")
	assert.Equal(t, 0, got.GamesPlayed)
	assert.Equal(t, 0, got.GamesCompleted)

	assert.Empty(t, repo.rows, "a read must not create a player_seasons row")
}

// A populated record, with the progress decomposed within the current RANK
// STEP (Story 13.5): Gold 2 spans 667-733 and fills toward Gold 3.
func TestGetCurrentSeason_MidTierProgressDecomposition(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.rows[key(42, testWindow.ID)] = &season.PlayerSeason{
		UserID: 42, SeasonID: testWindow.ID, SP: 700,
		RankTier: "gold", GamesPlayed: 31, GamesCompleted: 29,
	}

	rec, err := call(t, repo, 42)
	require.NoError(t, err)

	got := decode(t, rec)
	assert.Equal(t, 700, got.SP)
	assert.Equal(t, "gold", got.RankTier)
	require.NotNil(t, got.RankDivision)
	assert.Equal(t, 2, *got.RankDivision)
	// 700 sits 33 into Gold 2's 67-SP step (667 -> 734).
	assert.Equal(t, 33, got.SPIntoDivision)
	assert.Equal(t, 67, got.SPForNextDivision)
	assert.Equal(t, 31, got.GamesPlayed)
	assert.Equal(t, 29, got.GamesCompleted)
}

// The ACTIVE season derives its rank from SP, never the stored snapshot. A stale
// snapshot must not reach the client.
func TestGetCurrentSeason_IgnoresAStaleStoredTier(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.rows[key(42, testWindow.ID)] = &season.PlayerSeason{
		UserID: 42, SeasonID: testWindow.ID, SP: 700,
		RankTier: "iron", RankDivision: divPtr(1), // deliberately stale
		GamesPlayed: 60, GamesCompleted: 60,
	}

	rec, err := call(t, repo, 42)
	require.NoError(t, err)
	got := decode(t, rec)
	assert.Equal(t, "gold", got.RankTier, "700 SP is Gold, whatever the snapshot says")
	require.NotNil(t, got.RankDivision)
	assert.Equal(t, 2, *got.RankDivision, "700 SP is Gold 2")
}

// At the top of the ladder there is no next step, and the client must be able
// to tell: spForNextDivision is 0 rather than a fabricated band, and the
// division is null (Grandmaster is single).
func TestGetCurrentSeason_GrandmasterIsTerminal(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.rows[key(42, testWindow.ID)] = &season.PlayerSeason{
		UserID: 42, SeasonID: testWindow.ID, SP: 1500, RankTier: "grandmaster",
	}

	rec, err := call(t, repo, 42)
	require.NoError(t, err)

	got := decode(t, rec)
	assert.Equal(t, "grandmaster", got.RankTier)
	assert.Nil(t, got.RankDivision)
	assert.Equal(t, 100, got.SPIntoDivision)
	assert.Zero(t, got.SPForNextDivision)
	assert.Contains(t, rec.Body.String(), `"rankDivision":null`, "null, never omitted")
}

// Master has no division either, and its step is the climb to the Grandmaster
// floor.
func TestGetCurrentSeason_MasterFillsTowardGrandmaster(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.rows[key(42, testWindow.ID)] = &season.PlayerSeason{
		UserID: 42, SeasonID: testWindow.ID, SP: 1250, GamesPlayed: 40, GamesCompleted: 40,
	}

	rec, err := call(t, repo, 42)
	require.NoError(t, err)

	got := decode(t, rec)
	assert.Equal(t, "master", got.RankTier)
	assert.Nil(t, got.RankDivision)
	assert.Equal(t, 50, got.SPIntoDivision)
	assert.Equal(t, 200, got.SPForNextDivision, "Master fills toward the Grandmaster floor")
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

// fourHumanOutcome is a natural 1001 finish with users 1 and 3 (team A) beating
// users 2 and 4 (team B) 1100:700.
func fourHumanOutcome() match.MatchOutcome {
	return match.MatchOutcome{
		Seats: [4]match.OutcomeSeat{
			{UserID: 1, Team: 0, Completed: true},
			{UserID: 2, Team: 1, Completed: true},
			{UserID: 3, Team: 0, Completed: true},
			{UserID: 4, Team: 1, Completed: true},
		},
		WinnerTeam: 0, TeamScores: [2]int{1100, 700}, Target: 1001, AbandonedSeat: -1,
	}
}

func TestApplySeasonPoints_NilSeasonIsAnErrorNotAPanic(t *testing.T) {
	svc := season.NewService(newMockRepo(nil))

	out, err := svc.ApplySeasonPoints(fourHumanOutcome(), time.Now().UTC())
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "no season")
}

// The awarder path never touches the DB for an outcome with no human seat —
// notably it must not create a season row for an all-bot match.
func TestApplySeasonPoints_AllBotOutcomeResolvesNoSeason(t *testing.T) {
	repo := newMockRepo(testWindow)
	svc := season.NewService(repo)

	allBots := match.MatchOutcome{
		Seats: [4]match.OutcomeSeat{
			{IsBot: true, Team: 0}, {IsBot: true, Team: 1}, {IsBot: true, Team: 0}, {IsBot: true, Team: 1},
		},
		Target: 1001, AbandonedSeat: -1,
	}
	out, err := svc.ApplySeasonPoints(allBots, time.Now().UTC())
	require.NoError(t, err)
	assert.Empty(t, out)
	assert.Zero(t, repo.currentCalls, "no season is resolved (or created) for an all-bot match")
}

// A malformed outcome fails the award (the finalizer logs and skips) instead of
// writing a guessed change.
func TestApplySeasonPoints_MalformedOutcomeWritesNothing(t *testing.T) {
	repo := newMockRepo(testWindow)
	svc := season.NewService(repo)

	bad := fourHumanOutcome()
	bad.Target = 0
	_, err := svc.ApplySeasonPoints(bad, time.Now().UTC())
	require.Error(t, err)
	assert.Empty(t, repo.rows)
}

// The service hands the match manager a FULLY PRECOMPUTED snapshot — season
// name, the APPLIED change, the rank, the rank change and the reason — so the
// manager never runs ladder arithmetic it cannot see (Story 13.1 D8). The rank
// change compares (tier, division), never SP alone.
func TestApplySeasonPoints_PrecomputesTheSnapshot(t *testing.T) {
	repo := newMockRepo(testWindow)
	svc := season.NewService(repo)

	// Team A averages 120, team B 85: E_A = 0.552, so A wins +24, B loses -16.
	repo.rows[key(1, testWindow.ID)] = &season.PlayerSeason{UserID: 1, SeasonID: testWindow.ID, SP: 140}
	repo.rows[key(2, testWindow.ID)] = &season.PlayerSeason{UserID: 2, SeasonID: testWindow.ID, SP: 160}
	repo.rows[key(3, testWindow.ID)] = &season.PlayerSeason{UserID: 3, SeasonID: testWindow.ID, SP: 100}
	repo.rows[key(4, testWindow.ID)] = &season.PlayerSeason{UserID: 4, SeasonID: testWindow.ID, SP: 10}

	got, err := svc.ApplySeasonPoints(fourHumanOutcome(), time.Now().UTC())
	require.NoError(t, err)

	assert.Equal(t, match.PlayerSeasonSnapshot{
		SeasonName: "2026 Q3", SP: 164, SPChange: 24, RankTier: "bronze", RankDivision: divPtr(1),
		RankChange: "promoted", Reason: "normal",
	}, got[1], "140 (Iron 3) -> 164 crosses the 150 Bronze floor")

	assert.Equal(t, match.PlayerSeasonSnapshot{
		SeasonName: "2026 Q3", SP: 124, SPChange: 24, RankTier: "iron", RankDivision: divPtr(3),
		RankChange: "none", Reason: "normal",
	}, got[3], "100 -> 124 is a gain inside Iron 3: no rank change")

	// The one a "the SP changed" shortcut gets wrong: a DROP is a demotion.
	assert.Equal(t, match.PlayerSeasonSnapshot{
		SeasonName: "2026 Q3", SP: 144, SPChange: -16, RankTier: "iron", RankDivision: divPtr(3),
		RankChange: "demoted", Reason: "normal",
	}, got[2], "160 (Bronze 1) -> 144 drops to Iron 3")

	// The floor: the formula's -16 applies as -10, and Iron 1 stays Iron 1.
	assert.Equal(t, match.PlayerSeasonSnapshot{
		SeasonName: "2026 Q3", SP: 0, SPChange: -10, RankTier: "iron", RankDivision: divPtr(1),
		RankChange: "none", Reason: "normal",
	}, got[4], "10 -> 0 reports the applied change")
}

// The reason walks the outcome's seats: the expired seat is "abandoned", its
// teammate "partner_abandoned", both opponents "normal". Master and Grandmaster
// carry no division.
func TestApplySeasonPoints_AbandonmentReasons(t *testing.T) {
	repo := newMockRepo(testWindow)
	svc := season.NewService(repo)
	for id, sp := range map[uint]int{1: 1400, 2: 700, 3: 700, 4: 700} {
		repo.rows[key(id, testWindow.ID)] = &season.PlayerSeason{UserID: id, SeasonID: testWindow.ID, SP: sp}
	}

	outcome := match.MatchOutcome{
		Seats: [4]match.OutcomeSeat{
			{UserID: 1, Team: 0, Completed: false},
			{UserID: 2, Team: 1, Completed: true},
			{UserID: 3, Team: 0, Completed: true},
			{UserID: 4, Team: 1, Completed: true},
		},
		WinnerTeam: 1, TeamScores: [2]int{300, 500}, Target: 1001, AbandonedSeat: 0,
	}
	got, err := svc.ApplySeasonPoints(outcome, time.Now().UTC())
	require.NoError(t, err)

	assert.Equal(t, match.PlayerSeasonSnapshot{
		SeasonName: "2026 Q3", SP: 1280, SPChange: -120, RankTier: "master", RankDivision: nil,
		RankChange: "demoted", Reason: "abandoned",
	}, got[1], "the fixed penalty drops Grandmaster (1400) to Master, which has no division")
	assert.Equal(t, "partner_abandoned", got[3].Reason)
	assert.Negative(t, got[3].SPChange, "the teammate takes half a surrender loss")
	assert.Equal(t, "normal", got[2].Reason)
	assert.Equal(t, "normal", got[4].Reason)
	assert.Positive(t, got[2].SPChange)
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
	assert.Equal(t, []string{"division", "gamesPlayed", "position", "sp", "tier", "userId", "username"},
		sortedKeys(row), "exact row key set")

	viewer, ok := data["viewer"].(map[string]any)
	require.True(t, ok, "viewer must be an object when the caller has SP")
	// No `username`: the viewer is the authenticated caller and the client
	// already holds their name (Story 13.2 D3).
	assert.Equal(t, []string{"division", "gamesPlayed", "position", "sp", "tier", "userId"},
		sortedKeys(viewer), "the viewer block deliberately carries no username")

	// A tag that survives but changes shape is the same class of silent break.
	assert.IsType(t, "", row["username"])
	assert.IsType(t, "", row["tier"])
	for _, numeric := range []string{"position", "userId", "sp", "division", "gamesPlayed"} {
		assert.IsType(t, float64(0), row[numeric], "%s must be a JSON number", numeric)
	}
	assert.IsType(t, float64(0), viewer["division"])
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

// A player who has played down to 0 SP is ON THE LADDER (Story 13.4): listed at
// a real position, counted in the total, and given a viewer block that agrees
// with their row. The list and the viewer block have to agree, so both halves
// are asserted.
func TestGetLeaderboard_ViewerAtZeroSPHasAStanding(t *testing.T) {
	repo := newMockRepo(testWindow)
	seedLadder(repo, 3)
	repo.seed(50, "zero", 0, 4)

	rec, err := callLeaderboard(t, repo, 50, "season=current")
	require.NoError(t, err)

	got := decodeLeaderboard(t, rec)
	require.NotNil(t, got.Viewer, "0 SP after playing is a real Iron 1 player with a place")
	assert.Equal(t, 4, got.Viewer.Position)
	assert.Equal(t, 0, got.Viewer.SP)
	assert.Equal(t, "iron", got.Viewer.Tier)

	assert.Len(t, got.Items, 4, "the 0-SP player is listed")
	assert.Equal(t, int64(4), got.Total, "and counted")
	assert.Equal(t, uint(50), got.Items[3].UserID)
	assert.Equal(t, 4, got.Items[3].Position, "the viewer block and the row agree")
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

// On the CURRENT season every rank is DERIVED from `sp`, never the stored
// snapshot, in the rows AND in the viewer block (Story 13.5: the snapshot is an
// ended season's answer only).
func TestGetLeaderboard_TierIsDerivedNotTheStoredColumn(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.seed(1, "gm", 1600, 40)
	repo.seed(2, "viewer", 1100, 30)
	// Deliberately wrong snapshots, as a stale row would carry.
	repo.rows[key(1, testWindow.ID)].RankTier = "iron"
	repo.rows[key(1, testWindow.ID)].RankDivision = divPtr(1)
	repo.rows[key(2, testWindow.ID)].RankTier = "iron"
	repo.rows[key(2, testWindow.ID)].RankDivision = divPtr(3)

	rec, err := callLeaderboard(t, repo, 2, "season=current")
	require.NoError(t, err)

	got := decodeLeaderboard(t, rec)
	require.Len(t, got.Items, 2)
	assert.Equal(t, "grandmaster", got.Items[0].Tier, "1600 SP is Grandmaster, whatever the column says")
	assert.Nil(t, got.Items[0].Division, "Grandmaster is single")
	assert.Equal(t, "diamond", got.Items[1].Tier)
	require.NotNil(t, got.Items[1].Division)
	assert.Equal(t, 2, *got.Items[1].Division, "1100 SP is Diamond 2")
	require.NotNil(t, got.Viewer)
	assert.Equal(t, "diamond", got.Viewer.Tier)
	require.NotNil(t, got.Viewer.Division)
	assert.Equal(t, 2, *got.Viewer.Division)
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

// THE LADDER IS EVERYONE WHO PLAYED (Story 13.4). A 0-SP player is listed in SP
// order like anyone else; only a row with no games played stays off.
func TestGetLeaderboard_ZeroSPPlayersAreOnTheLadder(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.seed(1, "earner", 500, 5)
	repo.seed(2, "floored", 0, 3)
	repo.seed(3, "other", 250, 4)
	repo.seed(4, "unplayed", 0, 0)

	rec, err := callLeaderboard(t, repo, 1, "season=current")
	require.NoError(t, err)

	got := decodeLeaderboard(t, rec)
	assert.Equal(t, int64(3), got.Total, "the 0-SP player counts; the unplayed row does not")
	require.Len(t, got.Items, 3)
	assert.Equal(t, []uint{1, 3, 2}, []uint{got.Items[0].UserID, got.Items[1].UserID, got.Items[2].UserID})
	assert.Equal(t, []int{1, 2, 3}, []int{got.Items[0].Position, got.Items[1].Position, got.Items[2].Position},
		"positions close up — an excluded row leaves no gap")
}

// A season whose ONLY rows have no games played reads as empty.
func TestGetLeaderboard_SeasonOfOnlyUnplayedRowsReadsAsEmpty(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.seed(1, "a", 0, 0)
	repo.seed(2, "b", 0, 0)

	rec, err := callLeaderboard(t, repo, 42, "season=current")
	require.NoError(t, err)

	got := decodeLeaderboard(t, rec)
	assert.Empty(t, got.Items)
	assert.Equal(t, int64(0), got.Total)
	assert.Nil(t, got.Viewer)
	assert.Contains(t, rec.Body.String(), `"items":[]`)
}

// The agreement the membership rule must not break: with 0-SP players listed and
// an unplayed row present, every listed row's position still equals its own
// CountAhead + 1 — including the tied 0-SP players at the bottom.
func TestGetLeaderboard_PositionsAgreeWithZeroSPPlayersListed(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.seed(1, "top", 5000, 9)
	repo.seed(2, "floored", 0, 1)
	repo.seed(3, "mid", 900, 7)
	repo.seed(4, "tied", 900, 7)
	repo.seed(5, "floored2", 0, 2)
	repo.seed(6, "unplayed", 0, 0)

	// Each listed player asks for their own standing in turn.
	for _, viewerID := range []uint{1, 2, 3, 4, 5} {
		rec, err := callLeaderboard(t, repo, viewerID, "season=current")
		require.NoError(t, err)
		got := decodeLeaderboard(t, rec)
		require.Len(t, got.Items, 5)

		require.NotNil(t, got.Viewer, "user %d played and must have a standing", viewerID)
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
// writes into testWindow) with an EXPLICIT stored rank snapshot: an ended
// season shows what was stored, which for a pre-division season is a bare
// tier scored on old floors.
func seedEnded(repo *mockRepo, userID uint, username string, sp, gamesPlayed int, tier string, division *int) {
	repo.rows[key(userID, endedWindow.ID)] = &season.PlayerSeason{
		UserID: userID, SeasonID: endedWindow.ID, SP: sp,
		RankTier: tier, RankDivision: division, GamesPlayed: gamesPlayed, GamesCompleted: gamesPlayed,
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
// block runs under the same games_played >= 1 rule it has on the current window.
func TestGetLeaderboard_EndedSeasonByIdRendersItsStandings(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.seasons = []season.Season{*endedWindow}
	// Current window: a ladder that must NOT leak into the prior season's view.
	seedLadder(repo, 3)
	// The ended season: three players, one of them finished on 0 SP.
	seedEnded(repo, 21, "past-top", 1000, 12, "diamond", nil)
	seedEnded(repo, 22, "past-second", 400, 8, "silver", nil)
	seedEnded(repo, 23, "past-zero", 0, 3, "iron", nil)

	rec, err := callLeaderboard(t, repo, 22, "season=5")
	require.NoError(t, err)

	got := decodeLeaderboard(t, rec)
	assert.Equal(t, int64(3), got.Total, "the prior season's OWN population, everyone who played")
	require.Len(t, got.Items, 3)
	assert.Equal(t, []uint{21, 22, 23}, []uint{got.Items[0].UserID, got.Items[1].UserID, got.Items[2].UserID})
	assert.Equal(t, "diamond", got.Items[0].Tier, "the stored snapshot of the ended season")

	require.NotNil(t, got.Viewer, "the viewer played in that season")
	assert.Equal(t, 2, got.Viewer.Position)
	assert.Equal(t, 400, got.Viewer.SP)

	// And the resolved id — not the current window's — reached the repository.
	assert.Equal(t, [3]int{int(endedWindow.ID), 10, 0}, repo.lastPageArgs)
	assert.Zero(t, repo.currentCalls, "a by-id read never resolves (or creates) the current window")
}

// AN ENDED SEASON SHOWS ITS STORED RANK, NEVER A RE-DERIVED ONE (Story 13.5). The Q3
// matrix row: an old-formula total of 3500 SP stored as a bare "gold" reads as
// Gold with no division, never as Grandmaster, in the rows AND the viewer
// block. Beside it, a row written after divisions existed keeps its stored
// division, and the CURRENT season, read in the same test, still derives.
func TestGetLeaderboard_EndedSeasonReadsTheStoredRank(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.seasons = []season.Season{*endedWindow}
	seedEnded(repo, 31, "q3-veteran", 3500, 90, "gold", nil)
	seedEnded(repo, 32, "divided", 900, 20, "platinum", divPtr(2))
	// The same veteran on the running window, with a stale snapshot.
	repo.seed(31, "q3-veteran", 700, 4)
	repo.rows[key(31, testWindow.ID)].RankTier = "iron"
	repo.rows[key(31, testWindow.ID)].RankDivision = divPtr(1)

	rec, err := callLeaderboard(t, repo, 31, "season=5")
	require.NoError(t, err)
	past := decodeLeaderboard(t, rec)
	require.Len(t, past.Items, 2)
	assert.Equal(t, "gold", past.Items[0].Tier, "3500 old-formula SP is the stored Gold, never Grandmaster")
	assert.Nil(t, past.Items[0].Division, "a pre-division row has no division")
	assert.Equal(t, "platinum", past.Items[1].Tier)
	require.NotNil(t, past.Items[1].Division)
	assert.Equal(t, 2, *past.Items[1].Division, "a stored division is shown as stored")
	require.NotNil(t, past.Viewer)
	assert.Equal(t, "gold", past.Viewer.Tier, "the pinned row follows the list's rule")
	assert.Nil(t, past.Viewer.Division)
	assert.Contains(t, rec.Body.String(), `"division":null`, "null, never omitted")

	rec, err = callLeaderboard(t, repo, 31, "season=current")
	require.NoError(t, err)
	now := decodeLeaderboard(t, rec)
	require.Len(t, now.Items, 1)
	assert.Equal(t, "gold", now.Items[0].Tier)
	require.NotNil(t, now.Items[0].Division)
	assert.Equal(t, 2, *now.Items[0].Division, "the running season derives Gold 2 from 700 SP")
}

// A by-id window that has NOT ended is a running season: it derives, exactly
// like the current-window selector. The window here ends far in the future so
// the test never ages into the ended branch.
func TestGetLeaderboard_RunningSeasonByIdDerivesTheRank(t *testing.T) {
	future := &season.Season{
		ID:        9,
		Name:      "2099 Q1",
		StartedAt: time.Date(2099, time.January, 1, 0, 0, 0, 0, time.UTC),
		EndsAt:    time.Date(2099, time.April, 1, 0, 0, 0, 0, time.UTC),
	}
	repo := newMockRepo(testWindow)
	repo.seasons = []season.Season{*future}
	repo.rows[key(41, future.ID)] = &season.PlayerSeason{
		UserID: 41, SeasonID: future.ID, SP: 700, RankTier: "iron", RankDivision: divPtr(1),
		GamesPlayed: 3, GamesCompleted: 3,
	}
	repo.usernames[41] = "runner"

	rec, err := callLeaderboard(t, repo, 41, "season=9")
	require.NoError(t, err)
	got := decodeLeaderboard(t, rec)
	require.Len(t, got.Items, 1)
	assert.Equal(t, "gold", got.Items[0].Tier, "the stale snapshot is ignored on a running season")
	require.NotNil(t, got.Items[0].Division)
	assert.Equal(t, 2, *got.Items[0].Division)
	require.NotNil(t, got.Viewer)
	assert.Equal(t, "gold", got.Viewer.Tier)
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
	seedEnded(repo, 42, "archiver", 450, 14, "platinum", divPtr(3))

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
		[]string{"division", "endsAt", "gamesPlayed", "seasonId", "seasonName", "sp", "startedAt", "tier"},
		sortedKeys(row), "exact wire key set for an archive row")

	assert.IsType(t, "", row["seasonName"])
	assert.IsType(t, "", row["tier"])
	assert.Equal(t, "platinum", row["tier"], "the STORED rank the season finished on, never the one 450 SP derives")
	assert.Equal(t, float64(3), row["division"])
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
	seedEnded(repo, 42, "archiver", 200, 8, "bronze", divPtr(2))
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
	assert.Equal(t, 200, env.Data.Items[0].SP)
	assert.Equal(t, "bronze", env.Data.Items[0].Tier)
	require.NotNil(t, env.Data.Items[0].Division)
	assert.Equal(t, 2, *env.Data.Items[0].Division)

	assert.Equal(t, "2026 Q1", env.Data.Items[1].SeasonName)
	assert.Equal(t, 0, env.Data.Items[1].SP, "a played 0-SP season stays in the archive")
	assert.Equal(t, "iron", env.Data.Items[1].Tier)
	assert.Nil(t, env.Data.Items[1].Division, "a pre-division row has none")
	assert.Equal(t, 2, env.Data.Items[1].GamesPlayed)
}

// THE ARCHIVE READS THE STORED RANK, NEVER A RE-DERIVED ONE (Story 13.5), the matrix's
// ended-Q3 row: stored gold, no division, 3500 old-formula SP. On the 13.4
// floors 3500 would derive Grandmaster.
func TestGetPlayerSeasonArchive_ReadsTheStoredRank(t *testing.T) {
	repo := newMockRepo(testWindow)
	repo.seasons = []season.Season{*endedWindow}
	seedEnded(repo, 42, "veteran", 3500, 90, "gold", nil)

	rec, err := callArchive(t, repo, 7, "42")
	require.NoError(t, err)

	var env struct {
		Data season.ArchiveView `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.Len(t, env.Data.Items, 1)
	assert.Equal(t, 3500, env.Data.Items[0].SP)
	assert.Equal(t, "gold", env.Data.Items[0].Tier, "never Grandmaster")
	assert.Nil(t, env.Data.Items[0].Division)
	assert.Contains(t, rec.Body.String(), `"division":null`, "null, never omitted")
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
		UserID: 42, SeasonID: testWindow.ID, SP: 700,
		// Deliberately stale — the active season derives, so it is ignored.
		RankTier: "iron", RankDivision: divPtr(1),
		GamesPlayed: 30, GamesCompleted: 28,
	}
	svc := season.NewService(repo)

	rank, err := svc.CurrentSeasonRank(42, time.Now().UTC())
	require.NoError(t, err)
	require.NotNil(t, rank)
	assert.Equal(t, "2026 Q3", rank.SeasonName)
	assert.Equal(t, "gold", rank.Tier, "derived from SP, whatever the column says")
	require.NotNil(t, rank.Division)
	assert.Equal(t, 2, *rank.Division, "700 SP is Gold 2")
	assert.Equal(t, 700, rank.SP)
}

// A 0-SP row is a REAL rank (Iron) — the row's existence gates the block.
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
	require.NotNil(t, rank.Division)
	assert.Equal(t, 1, *rank.Division, "0 SP is Iron 1")
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
