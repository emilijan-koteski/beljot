package user_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	sortpkg "sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/auth"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/user"
)

// --- Mock Match Repository (for user handler tests) ---

type mockMatchRepo struct {
	matches []match.Match // newest first (by completed_at); seeded in test order
	err     error

	// Stats-path overrides — when statsOverride is non-nil, GetStatsForUser
	// returns these values regardless of r.matches. This lets the handler
	// test drive the stats branch without seeding full Match rows.
	statsOverride   *struct{ wins, losses, abandoned int }
	statsErr        error
	getStatsCalls   int
	lastStatsUserID uint

	// Career-path overrides drive the GetCareer handler without full SQL.
	careerAgg      match.CareerAggregates
	careerErr      error
	partners       []match.PartnerAggregate
	partnersErr    error
	rivals         []match.RivalAggregate
	rivalsErr      error
	getCareerCalls int
}

func newMockMatchRepo() *mockMatchRepo {
	return &mockMatchRepo{}
}

// seatID maps a nullable player ID to its comparison value (0 for bot/NULL
// seats — never a real user ID).
func seatID(p *uint) uint {
	if p == nil {
		return 0
	}
	return *p
}

func (r *mockMatchRepo) Create(*match.Match) error { return nil }

func (r *mockMatchRepo) CreateWithHands(*match.Match, []match.HandResult) error { return nil }

func (r *mockMatchRepo) GetMatchesForUser(userID uint, limit, offset int, outcome, sort string) ([]match.Match, int64, error) {
	if r.err != nil {
		return nil, 0, r.err
	}
	viewerWon := func(m match.Match) bool {
		seats := [4]uint{seatID(m.Player1ID), seatID(m.Player2ID), seatID(m.Player3ID), seatID(m.Player4ID)}
		for i, id := range seats {
			if id == userID {
				return m.WinnerTeam == i%2
			}
		}
		return false
	}
	var filtered []match.Match
	for _, m := range r.matches {
		if m.Status != "completed" && m.Status != "abandoned" {
			continue
		}
		if seatID(m.Player1ID) != userID && seatID(m.Player2ID) != userID && seatID(m.Player3ID) != userID && seatID(m.Player4ID) != userID {
			continue
		}
		// Viewer-relative outcome filter mirroring the production SQL:
		// attributable abandoned rows (abandoned_by set, not the viewer) count
		// win/loss by winner_team; the abandoner and NULL-abandoner legacy rows
		// stay "abandoned".
		countsWinLoss := m.Status == "completed" ||
			(m.Status == "abandoned" && m.AbandonedBy != nil && *m.AbandonedBy != userID)
		switch outcome {
		case "win":
			if !countsWinLoss || !viewerWon(m) {
				continue
			}
		case "loss":
			if !countsWinLoss || viewerWon(m) {
				continue
			}
		case "abandoned":
			if m.Status != "abandoned" || (m.AbandonedBy != nil && *m.AbandonedBy != userID) {
				continue
			}
		}
		filtered = append(filtered, m)
	}
	// Mirror the production query's ORDER BY completed_at <dir>, id <dir>.
	asc := sort == "old"
	sortpkg.SliceStable(filtered, func(i, j int) bool {
		if !filtered[i].CompletedAt.Equal(filtered[j].CompletedAt) {
			if asc {
				return filtered[i].CompletedAt.Before(filtered[j].CompletedAt)
			}
			return filtered[i].CompletedAt.After(filtered[j].CompletedAt)
		}
		if asc {
			return filtered[i].ID < filtered[j].ID
		}
		return filtered[i].ID > filtered[j].ID
	})
	total := int64(len(filtered))
	if offset >= len(filtered) {
		return []match.Match{}, total, nil
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[offset:end], total, nil
}

func (r *mockMatchRepo) GetCareerAggregatesForUser(userID uint) (match.CareerAggregates, error) {
	r.getCareerCalls++
	if r.careerErr != nil {
		return match.CareerAggregates{}, r.careerErr
	}
	return r.careerAgg, nil
}

func (r *mockMatchRepo) GetTopPartnersForUser(userID uint, limit int) ([]match.PartnerAggregate, error) {
	if r.partnersErr != nil {
		return nil, r.partnersErr
	}
	if limit < len(r.partners) {
		return r.partners[:limit], nil
	}
	return r.partners, nil
}

func (r *mockMatchRepo) GetTopRivalsForUser(userID uint, limit int) ([]match.RivalAggregate, error) {
	if r.rivalsErr != nil {
		return nil, r.rivalsErr
	}
	if limit < len(r.rivals) {
		return r.rivals[:limit], nil
	}
	return r.rivals, nil
}

// GetStatsForUser returns the test-controlled stats override if set, else
// computes the four counts from seeded matches using the same predicate as
// the production SQL (participation by any seat; team derived from viewer's
// seat). Tracks call count + last userID so auth-failure tests can assert
// the stats path was NOT invoked.
func (r *mockMatchRepo) GetStatsForUser(userID uint) (int, int, int, error) {
	r.getStatsCalls++
	r.lastStatsUserID = userID
	if r.statsErr != nil {
		return 0, 0, 0, r.statsErr
	}
	if r.statsOverride != nil {
		return r.statsOverride.wins, r.statsOverride.losses, r.statsOverride.abandoned, nil
	}
	var wins, losses, abandoned int
	for _, m := range r.matches {
		seats := [4]uint{seatID(m.Player1ID), seatID(m.Player2ID), seatID(m.Player3ID), seatID(m.Player4ID)}
		viewerSeat := -1
		for i, id := range seats {
			if id == userID {
				viewerSeat = i
				break
			}
		}
		if viewerSeat == -1 {
			continue
		}
		switch m.Status {
		case "abandoned":
			// Per-player: only the abandoner (or every participant of a
			// NULL-abandoner legacy row) counts "abandoned"; the rest score
			// win/loss by winner_team — mirroring the production SQL gate.
			if m.AbandonedBy == nil || *m.AbandonedBy == userID {
				abandoned++
			} else if m.WinnerTeam == viewerSeat%2 {
				wins++
			} else {
				losses++
			}
		case "completed":
			if m.WinnerTeam == viewerSeat%2 {
				wins++
			} else {
				losses++
			}
		}
	}
	return wins, losses, abandoned, nil
}

type mockUserRepo struct {
	users  []*user.User
	nextID uint
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{nextID: 1}
}

func (m *mockUserRepo) Create(u *user.User) error {
	u.ID = m.nextID
	u.CreatedAt = time.Now()
	m.nextID++
	m.users = append(m.users, u)
	return nil
}

func (m *mockUserRepo) Delete(id uint) error {
	for i, u := range m.users {
		if u.ID == id {
			m.users = append(m.users[:i], m.users[i+1:]...)
			return nil
		}
	}
	return apperr.ErrUserNotFound
}

func (m *mockUserRepo) FindByEmail(email string) (*user.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepo) FindByUsername(username string) (*user.User, error) {
	for _, u := range m.users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepo) FindByID(id uint) (*user.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepo) FindManyByIDs(ids []uint) ([]user.User, error) {
	if len(ids) == 0 {
		return []user.User{}, nil
	}
	wanted := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	out := make([]user.User, 0, len(ids))
	for _, u := range m.users {
		if _, ok := wanted[u.ID]; ok {
			out = append(out, *u)
		}
	}
	return out, nil
}

func (m *mockUserRepo) Count() (int64, error) {
	return int64(len(m.users)), nil
}

func (m *mockUserRepo) UpdateLanguagePreference(id uint, lang string) error {
	for _, u := range m.users {
		if u.ID == id {
			u.LanguagePreference = lang
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (m *mockUserRepo) UpdatePasswordHash(id uint, hash string) error {
	for _, u := range m.users {
		if u.ID == id {
			u.PasswordHash = hash
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (m *mockUserRepo) AddXP(awards map[uint]int) (map[uint]int, error) {
	newTotals := make(map[uint]int, len(awards))
	for id, delta := range awards {
		if delta == 0 {
			continue
		}
		found := false
		for _, u := range m.users {
			if u.ID == id {
				u.TotalXP += delta
				newTotals[id] = u.TotalXP
				found = true
				break
			}
		}
		if !found {
			return nil, apperr.ErrUserNotFound
		}
	}
	return newTotals, nil
}

func (m *mockUserRepo) TotalXPForUsers(ids []uint) (map[uint]int, error) {
	totals := make(map[uint]int, len(ids))
	for _, id := range ids {
		for _, u := range m.users {
			if u.ID == id {
				totals[id] = u.TotalXP
				break
			}
		}
	}
	return totals, nil
}

func (m *mockUserRepo) addUser(username, email, lang string) *user.User {
	u := &user.User{
		ID:                 m.nextID,
		Username:           username,
		Email:              email,
		LanguagePreference: lang,
		CreatedAt:          time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
	}
	m.nextID++
	m.users = append(m.users, u)
	return u
}

func testErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	var appErr *apperr.AppError
	if errors.As(err, &appErr) {
		_ = c.JSON(appErr.Status, map[string]interface{}{
			"error": map[string]string{
				"code":    appErr.Code,
				"message": appErr.Message,
			},
		})
		return
	}

	_ = c.JSON(http.StatusInternalServerError, map[string]interface{}{
		"error": map[string]string{
			"code":    "INTERNAL_ERROR",
			"message": "An internal error occurred",
		},
	})
}

const testJWTSecret = "test-jwt-secret"

func setupUserHandler() (*mockUserRepo, *echo.Echo) {
	repo, _, e := setupUserHandlerWithMatches()
	return repo, e
}

func setupUserHandlerWithMatches() (*mockUserRepo, *mockMatchRepo, *echo.Echo) {
	repo := newMockUserRepo()
	matchRepo := newMockMatchRepo()
	handler := user.NewUserHandler(repo, matchRepo)
	e := echo.New()
	e.HTTPErrorHandler = testErrorHandler

	api := e.Group("/api/v1", auth.AuthMiddleware(testJWTSecret))
	api.GET("/users/:id/profile", handler.GetProfile)
	api.GET("/users/:id/career", handler.GetCareer)
	api.GET("/users/:id/matches", handler.ListMatches)
	api.PATCH("/users/:id/preferences", handler.UpdatePreferences)

	return repo, matchRepo, e
}

func doGetCareer(e *echo.Echo, userID string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+userID+"/career", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func doGetProfile(e *echo.Echo, userID string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+userID+"/profile", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func doUpdatePreferences(e *echo.Echo, userID string, body string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/"+userID+"/preferences", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestGetProfile_Success(t *testing.T) {
	repo, e := setupUserHandler()
	u := repo.addUser("testuser", "test@example.com", "en")

	token, err := auth.GenerateAccessToken(u.ID, testJWTSecret)
	require.NoError(t, err)

	rec := doGetProfile(e, "1", token)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var data user.ProfileResponse
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	assert.Equal(t, uint(1), data.ID)
	assert.Equal(t, "testuser", data.Username)
	assert.Equal(t, "en", data.LanguagePreference)
	assert.NotEmpty(t, data.CreatedAt)
}

// TestGetProfile_IncludesWalletFields pins AC #4: the self-only profile carries
// the private wallet balance + login streak from the loaded user.
func TestGetProfile_IncludesWalletFields(t *testing.T) {
	repo, e := setupUserHandler()
	u := repo.addUser("walletuser", "wallet@example.com", "en")
	u.WalletBalance = 7340
	u.LoginStreakDays = 5

	token, err := auth.GenerateAccessToken(u.ID, testJWTSecret)
	require.NoError(t, err)

	rec := doGetProfile(e, strconvUint(u.ID), token)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var data user.ProfileResponse
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	assert.Equal(t, 7340, data.WalletBalance)
	assert.Equal(t, 5, data.LoginStreakDays)
}

// TestGetProfile_IncludesXPAndLevel pins Story 9.5 AC4: the self-only profile
// carries total XP, the derived level, and the per-level progress that drives
// the XP bar. Level 3 band is threshold(3)=450 .. threshold(4)=800 (span 350).
func TestGetProfile_IncludesXPAndLevel(t *testing.T) {
	repo, e := setupUserHandler()
	u := repo.addUser("xpuser", "xp@example.com", "en")
	u.TotalXP = 600 // level 3; 150 into a 350-wide band

	token, err := auth.GenerateAccessToken(u.ID, testJWTSecret)
	require.NoError(t, err)

	rec := doGetProfile(e, strconvUint(u.ID), token)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var data user.ProfileResponse
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	assert.Equal(t, 600, data.TotalXP)
	assert.Equal(t, 3, data.Level)
	assert.Equal(t, 150, data.XPIntoLevel)
	assert.Equal(t, 350, data.XPForNextLevel)
}

func TestGetProfile_UserNotFound(t *testing.T) {
	_, e := setupUserHandler()

	token, err := auth.GenerateAccessToken(99, testJWTSecret)
	require.NoError(t, err)

	rec := doGetProfile(e, "99", token)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "USER_NOT_FOUND", errResp["error"]["code"])
}

func TestGetProfile_Forbidden(t *testing.T) {
	repo, e := setupUserHandler()
	repo.addUser("testuser", "test@example.com", "en")
	repo.addUser("otheruser", "other@example.com", "en")

	token, err := auth.GenerateAccessToken(1, testJWTSecret)
	require.NoError(t, err)

	rec := doGetProfile(e, "2", token)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "FORBIDDEN", errResp["error"]["code"])
}

func TestGetProfile_MissingAuth(t *testing.T) {
	_, e := setupUserHandler()

	rec := doGetProfile(e, "1", "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUpdatePreferences_Success(t *testing.T) {
	cases := []struct {
		name string
		lang string
	}{
		{name: "english", lang: "en"},
		{name: "serbian", lang: "sr"},
		{name: "macedonian", lang: "mk"},
		{name: "croatian", lang: "hr"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, e := setupUserHandler()
			u := repo.addUser("testuser", "test@example.com", "en")

			token, err := auth.GenerateAccessToken(u.ID, testJWTSecret)
			require.NoError(t, err)

			body := `{"languagePreference":"` + tc.lang + `"}`
			rec := doUpdatePreferences(e, "1", body, token)
			assert.Equal(t, http.StatusOK, rec.Code)

			var resp map[string]map[string]string
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			assert.Equal(t, tc.lang, resp["data"]["languagePreference"])

			assert.Equal(t, tc.lang, repo.users[0].LanguagePreference)
		})
	}
}

func TestUpdatePreferences_InvalidLanguage(t *testing.T) {
	repo, e := setupUserHandler()
	u := repo.addUser("testuser", "test@example.com", "en")

	token, err := auth.GenerateAccessToken(u.ID, testJWTSecret)
	require.NoError(t, err)

	rec := doUpdatePreferences(e, "1", `{"languagePreference":"fr"}`, token)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "INVALID_LANGUAGE", errResp["error"]["code"])
}

func TestUpdatePreferences_Forbidden(t *testing.T) {
	repo, e := setupUserHandler()
	repo.addUser("testuser", "test@example.com", "en")
	repo.addUser("otheruser", "other@example.com", "en")

	token, err := auth.GenerateAccessToken(1, testJWTSecret)
	require.NoError(t, err)

	rec := doUpdatePreferences(e, "2", `{"languagePreference":"sr"}`, token)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestUpdatePreferences_MissingAuth(t *testing.T) {
	_, e := setupUserHandler()

	rec := doUpdatePreferences(e, "1", `{"languagePreference":"sr"}`, "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- ListMatches tests (Story 7.1) ---

func doListMatches(e *echo.Echo, userID, query, token string) *httptest.ResponseRecorder {
	path := "/api/v1/users/" + userID + "/matches"
	if query != "" {
		path += "?" + query
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func seedMatch(id uint, started, completed time.Time, seats [4]uint, status string, winnerTeam int, abandonedBy *uint, variant, mode string, a, b int, hands []match.HandResult) match.Match {
	p1, p2, p3, p4 := seats[0], seats[1], seats[2], seats[3]
	return match.Match{
		ID:          id,
		RoomID:      id,
		Player1ID:   &p1,
		Player2ID:   &p2,
		Player3ID:   &p3,
		Player4ID:   &p4,
		TeamAScore:  a,
		TeamBScore:  b,
		WinnerTeam:  winnerTeam,
		Variant:     variant,
		MatchMode:   mode,
		StartedAt:   started,
		CompletedAt: completed,
		Status:      status,
		AbandonedBy: abandonedBy,
		Hands:       hands,
	}
}

func decodeMatchesResponse(t *testing.T, body []byte) user.MatchesListResponse {
	t.Helper()
	var resp struct {
		Data user.MatchesListResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &resp))
	return resp.Data
}

func TestListMatches_EmptyList(t *testing.T) {
	repo, _, e := setupUserHandlerWithMatches()
	u := repo.addUser("alice", "alice@example.com", "en")

	token, err := auth.GenerateAccessToken(u.ID, testJWTSecret)
	require.NoError(t, err)

	rec := doListMatches(e, "1", "", token)
	assert.Equal(t, http.StatusOK, rec.Code)

	resp := decodeMatchesResponse(t, rec.Body.Bytes())
	assert.NotNil(t, resp.Items, "items must be an empty slice, not nil")
	assert.Len(t, resp.Items, 0)
	assert.Equal(t, int64(0), resp.Total)
	assert.Equal(t, 20, resp.Limit)
	assert.Equal(t, 0, resp.Offset)
}

// TestListMatches_BotInclusiveMatchMarksBots pins the Story 10.3 history
// contract: bot-inclusive rows carry hasBots, bot seats serialize as
// {userId:0, username:"", isBot:true}, viewer-seat derivation skips NULL
// player IDs, and bot seats never reach the users lookup.
func TestListMatches_BotInclusiveMatchMarksBots(t *testing.T) {
	repo, matchRepo, e := setupUserHandlerWithMatches()
	viewer := repo.addUser("alice", "alice@example.com", "en")
	mate := repo.addUser("bob", "bob@example.com", "en")

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	viewerID, mateID := viewer.ID, mate.ID
	matchRepo.matches = []match.Match{{
		ID:           1,
		RoomID:       1,
		Player1ID:    &viewerID,
		Player3ID:    &mateID,
		Player2IsBot: true,
		Player4IsBot: true,
		HasBots:      true,
		TeamAScore:   1004,
		TeamBScore:   730,
		WinnerTeam:   0,
		Variant:      "bitola",
		MatchMode:    "501",
		StartedAt:    now.Add(-30 * time.Minute),
		CompletedAt:  now,
		Status:       "completed",
	}}

	token, err := auth.GenerateAccessToken(viewer.ID, testJWTSecret)
	require.NoError(t, err)

	rec := doListMatches(e, "1", "", token)
	require.Equal(t, http.StatusOK, rec.Code)

	resp := decodeMatchesResponse(t, rec.Body.Bytes())
	require.Len(t, resp.Items, 1)
	item := resp.Items[0]

	assert.True(t, item.HasBots)
	assert.Equal(t, 0, item.ViewerSeat)
	assert.Equal(t, "win", item.Outcome)
	require.Len(t, item.Players, 4)
	for _, p := range item.Players {
		switch p.Seat {
		case 0:
			assert.False(t, p.IsBot)
			assert.Equal(t, viewer.ID, p.UserID)
			assert.Equal(t, "alice", p.Username)
		case 1, 3:
			assert.True(t, p.IsBot, "seat %d is a bot", p.Seat)
			assert.Equal(t, uint(0), p.UserID)
			assert.Equal(t, "", p.Username)
		case 2:
			assert.False(t, p.IsBot)
			assert.Equal(t, "bob", p.Username)
		}
	}
}

func TestListMatches_WinLossAbandonedOutcomes(t *testing.T) {
	repo, matchRepo, e := setupUserHandlerWithMatches()
	viewer := repo.addUser("viewer", "v@example.com", "en")
	mate := repo.addUser("mate", "m@example.com", "en")
	opp1 := repo.addUser("opp1", "o1@example.com", "en")
	opp2 := repo.addUser("opp2", "o2@example.com", "en")

	// Viewer sits at seat 0 (team A). seats: [viewer, opp1, mate, opp2].
	seats := [4]uint{viewer.ID, opp1.ID, mate.ID, opp2.ID}
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	// Completed, team A wins -> outcome "win".
	// Completed, team B wins -> outcome "loss".
	// Abandoned BY THE VIEWER -> outcome "abandoned" (non-abandoners get
	// win/loss instead — covered by TestListMatches_PerPlayerAbandonment).
	abandoner := viewer.ID
	matchRepo.matches = []match.Match{
		seedMatch(1, base, base.Add(25*time.Minute), seats, "completed", 0, nil, "bitola", "1001", 1010, 640, nil),
		seedMatch(2, base.Add(-24*time.Hour), base.Add(-24*time.Hour).Add(31*time.Minute), seats, "completed", 1, nil, "bitola", "1001", 820, 1020, nil),
		seedMatch(3, base.Add(-48*time.Hour), base.Add(-48*time.Hour).Add(9*time.Minute), seats, "abandoned", 1, &abandoner, "bitola", "1001", 320, 410, nil),
	}

	token, err := auth.GenerateAccessToken(viewer.ID, testJWTSecret)
	require.NoError(t, err)

	rec := doListMatches(e, strconvUint(viewer.ID), "", token)
	assert.Equal(t, http.StatusOK, rec.Code)

	resp := decodeMatchesResponse(t, rec.Body.Bytes())
	require.Len(t, resp.Items, 3)
	assert.Equal(t, int64(3), resp.Total)

	// Outcomes ordered as seeded (mock returns insertion order).
	assert.Equal(t, "win", resp.Items[0].Outcome)
	assert.Equal(t, "loss", resp.Items[1].Outcome)
	assert.Equal(t, "abandoned", resp.Items[2].Outcome)

	// endReason: natural completions vs the abandoned row.
	assert.Equal(t, "natural", resp.Items[0].EndReason)
	assert.Equal(t, "natural", resp.Items[1].EndReason)
	assert.Equal(t, "abandonment", resp.Items[2].EndReason)

	// Viewer seat is always 0 in this setup.
	for _, item := range resp.Items {
		assert.Equal(t, 0, item.ViewerSeat)
		require.Len(t, item.Players, 4)
		// Seat-ordered 0..3.
		assert.Equal(t, 0, item.Players[0].Seat)
		assert.Equal(t, viewer.ID, item.Players[0].UserID)
		assert.Equal(t, "viewer", item.Players[0].Username)
		assert.Equal(t, "opp1", item.Players[1].Username)
		assert.Equal(t, "mate", item.Players[2].Username)
		assert.Equal(t, "opp2", item.Players[3].Username)
	}

	// Abandoned row carries abandonedBy.
	require.NotNil(t, resp.Items[2].AbandonedBy)
	assert.Equal(t, viewer.ID, *resp.Items[2].AbandonedBy)
}

func TestListMatches_PaginationBounds(t *testing.T) {
	repo, matchRepo, e := setupUserHandlerWithMatches()
	viewer := repo.addUser("viewer", "v@example.com", "en")
	_ = repo.addUser("p2", "p2@example.com", "en")
	_ = repo.addUser("p3", "p3@example.com", "en")
	_ = repo.addUser("p4", "p4@example.com", "en")

	seats := [4]uint{viewer.ID, 2, 3, 4}
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	// Seed so completedAt descends as IDs ascend — the mock sorts by
	// completed_at DESC, id DESC (matching production), so after sorting the
	// page order is [1, 2, 3, 4, 5].
	for i := 1; i <= 5; i++ {
		completed := base.Add(time.Duration(60-10*i) * time.Minute)
		matchRepo.matches = append(matchRepo.matches, seedMatch(
			uint(i), base, completed, seats, "completed", 0, nil, "bitola", "1001", 1001, 500, nil,
		))
	}

	token, err := auth.GenerateAccessToken(viewer.ID, testJWTSecret)
	require.NoError(t, err)

	// total=5, limit=2, offset=2 -> items[2..3] (IDs 3, 4)
	rec := doListMatches(e, strconvUint(viewer.ID), "limit=2&offset=2", token)
	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeMatchesResponse(t, rec.Body.Bytes())
	require.Len(t, resp.Items, 2)
	assert.Equal(t, int64(5), resp.Total)
	assert.Equal(t, 2, resp.Limit)
	assert.Equal(t, 2, resp.Offset)
	assert.Equal(t, uint(3), resp.Items[0].ID)
	assert.Equal(t, uint(4), resp.Items[1].ID)

	// offset past end -> empty items, total still 5
	rec = doListMatches(e, strconvUint(viewer.ID), "limit=2&offset=100", token)
	assert.Equal(t, http.StatusOK, rec.Code)
	resp = decodeMatchesResponse(t, rec.Body.Bytes())
	assert.Len(t, resp.Items, 0)
	assert.Equal(t, int64(5), resp.Total)
}

func TestListMatches_OrdersByCompletedAtDesc(t *testing.T) {
	repo, matchRepo, e := setupUserHandlerWithMatches()
	viewer := repo.addUser("viewer", "v@example.com", "en")
	_ = repo.addUser("p2", "p2@example.com", "en")
	_ = repo.addUser("p3", "p3@example.com", "en")
	_ = repo.addUser("p4", "p4@example.com", "en")

	seats := [4]uint{viewer.ID, 2, 3, 4}
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	// Seed oldest first; the repo must reorder newest first.
	matchRepo.matches = []match.Match{
		seedMatch(10, base, base.Add(5*time.Minute), seats, "completed", 0, nil, "bitola", "1001", 1001, 500, nil),
		seedMatch(11, base.Add(1*time.Hour), base.Add(1*time.Hour+5*time.Minute), seats, "completed", 0, nil, "bitola", "1001", 1001, 500, nil),
		seedMatch(12, base.Add(2*time.Hour), base.Add(2*time.Hour+5*time.Minute), seats, "completed", 0, nil, "bitola", "1001", 1001, 500, nil),
	}

	token, err := auth.GenerateAccessToken(viewer.ID, testJWTSecret)
	require.NoError(t, err)

	rec := doListMatches(e, strconvUint(viewer.ID), "", token)
	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeMatchesResponse(t, rec.Body.Bytes())
	require.Len(t, resp.Items, 3)
	// Newest completedAt first: 12, 11, 10.
	assert.Equal(t, uint(12), resp.Items[0].ID)
	assert.Equal(t, uint(11), resp.Items[1].ID)
	assert.Equal(t, uint(10), resp.Items[2].ID)
}

func TestListMatches_HandsEmbedded(t *testing.T) {
	repo, matchRepo, e := setupUserHandlerWithMatches()
	viewer := repo.addUser("viewer", "v@example.com", "en")
	_ = repo.addUser("p2", "p2@example.com", "en")
	_ = repo.addUser("p3", "p3@example.com", "en")
	_ = repo.addUser("p4", "p4@example.com", "en")

	seats := [4]uint{viewer.ID, 2, 3, 4}
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	capotTeam := 1
	hands := []match.HandResult{
		{HandNumber: 1, TeamACardPoints: 60, TeamBCardPoints: 102, TeamADeclPoints: 20, TeamBDeclPoints: 0,
			LastTrickTeam: 1, LastTrickBonus: 10, Capot: false, CapotTeam: nil, CapotBonus: 0,
			FailedContract: false, ContractingTeam: 0, TeamAHandTotal: 80, TeamBHandTotal: 112},
		{HandNumber: 2, TeamACardPoints: 0, TeamBCardPoints: 162, TeamADeclPoints: 0, TeamBDeclPoints: 50,
			LastTrickTeam: 1, LastTrickBonus: 0, Capot: true, CapotTeam: &capotTeam, CapotBonus: 100,
			FailedContract: false, ContractingTeam: 1, TeamAHandTotal: 0, TeamBHandTotal: 312},
	}
	matchRepo.matches = []match.Match{
		seedMatch(1, base, base.Add(20*time.Minute), seats, "completed", 1, nil, "bitola", "1001", 80, 424, hands),
	}

	token, err := auth.GenerateAccessToken(viewer.ID, testJWTSecret)
	require.NoError(t, err)

	rec := doListMatches(e, strconvUint(viewer.ID), "", token)
	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeMatchesResponse(t, rec.Body.Bytes())
	require.Len(t, resp.Items, 1)
	require.Len(t, resp.Items[0].Hands, 2)
	assert.Equal(t, 1, resp.Items[0].Hands[0].HandNumber)
	assert.False(t, resp.Items[0].Hands[0].Capot)
	assert.Equal(t, 2, resp.Items[0].Hands[1].HandNumber)
	assert.True(t, resp.Items[0].Hands[1].Capot)
	require.NotNil(t, resp.Items[0].Hands[1].CapotTeam)
	assert.Equal(t, 1, *resp.Items[0].Hands[1].CapotTeam)
	assert.Equal(t, "loss", resp.Items[0].Outcome, "viewer is team A, winner is team B -> loss")
}

func TestListMatches_Forbidden_ForeignUserID(t *testing.T) {
	repo, _, e := setupUserHandlerWithMatches()
	u := repo.addUser("alice", "alice@example.com", "en")

	token, err := auth.GenerateAccessToken(u.ID, testJWTSecret)
	require.NoError(t, err)

	rec := doListMatches(e, "999", "", token)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestListMatches_Unauthorized_NoToken(t *testing.T) {
	_, _, e := setupUserHandlerWithMatches()

	rec := doListMatches(e, "1", "", "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestListMatches_BadRequest_InvalidID(t *testing.T) {
	repo, _, e := setupUserHandlerWithMatches()
	u := repo.addUser("alice", "alice@example.com", "en")

	token, err := auth.GenerateAccessToken(u.ID, testJWTSecret)
	require.NoError(t, err)

	for _, pathID := range []string{"abc", "0"} {
		rec := doListMatches(e, pathID, "", token)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "pathID %q should be 400", pathID)
	}
}

func TestListMatches_BadRequest_InvalidPagination(t *testing.T) {
	repo, _, e := setupUserHandlerWithMatches()
	u := repo.addUser("alice", "alice@example.com", "en")

	token, err := auth.GenerateAccessToken(u.ID, testJWTSecret)
	require.NoError(t, err)

	cases := []string{
		"limit=0",
		"limit=51",
		"limit=abc",
		"offset=-1",
		"offset=abc",
	}
	for _, q := range cases {
		rec := doListMatches(e, strconvUint(u.ID), q, token)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "query %q should be 400", q)
	}
}

func TestListMatches_NoPIILeak(t *testing.T) {
	repo, matchRepo, e := setupUserHandlerWithMatches()
	viewer := repo.addUser("viewer", "viewer@example.com", "en")
	_ = repo.addUser("p2", "secret-email@example.com", "en")
	_ = repo.addUser("p3", "p3@example.com", "sr")
	_ = repo.addUser("p4", "p4@example.com", "en")

	seats := [4]uint{viewer.ID, 2, 3, 4}
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	matchRepo.matches = []match.Match{
		seedMatch(1, base, base.Add(10*time.Minute), seats, "completed", 0, nil, "bitola", "1001", 1001, 500, nil),
	}

	token, err := auth.GenerateAccessToken(viewer.ID, testJWTSecret)
	require.NoError(t, err)

	rec := doListMatches(e, strconvUint(viewer.ID), "", token)
	assert.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.NotContains(t, body, "secret-email@example.com", "email must never be in the matches response")
	assert.NotContains(t, body, "passwordHash")
	assert.NotContains(t, body, "languagePreference", "language preference must not leak into matches response")
}

func strconvUint(u uint) string { return strconv.FormatUint(uint64(u), 10) }

// --- Extended GetProfile with aggregate stats (Story 7.2) ---

func TestGetProfile_WithStats(t *testing.T) {
	cases := []struct {
		name     string
		wins     int
		losses   int
		aband    int
		expected int // totalGamesPlayed
	}{
		{"zero games", 0, 0, 0, 0},
		{"mixed outcomes", 7, 3, 1, 11},
		{"wins only", 4, 0, 0, 4},
		{"abandoned only", 0, 0, 2, 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, matchRepo, e := setupUserHandlerWithMatches()
			u := repo.addUser("alice", "alice@example.com", "en")
			matchRepo.statsOverride = &struct{ wins, losses, abandoned int }{tc.wins, tc.losses, tc.aband}

			token, err := auth.GenerateAccessToken(u.ID, testJWTSecret)
			require.NoError(t, err)

			rec := doGetProfile(e, strconvUint(u.ID), token)
			require.Equal(t, http.StatusOK, rec.Code)

			var resp map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			var data user.ProfileResponse
			require.NoError(t, json.Unmarshal(resp["data"], &data))

			assert.Equal(t, tc.wins, data.Wins)
			assert.Equal(t, tc.losses, data.Losses)
			assert.Equal(t, tc.aband, data.Abandoned)
			assert.Equal(t, tc.expected, data.TotalGamesPlayed)
			// Invariant: total == wins + losses + abandoned.
			assert.Equal(t, data.Wins+data.Losses+data.Abandoned, data.TotalGamesPlayed,
				"totalGamesPlayed invariant")
			// Identity fields preserved.
			assert.Equal(t, u.ID, data.ID)
			assert.Equal(t, "alice", data.Username)
			assert.Equal(t, "en", data.LanguagePreference)
		})
	}
}

func TestGetProfile_StatsDBError(t *testing.T) {
	repo, matchRepo, e := setupUserHandlerWithMatches()
	// Distinctive email so any partial-payload leak is obvious in the 500 body.
	u := repo.addUser("alice", "alice-err-probe@example.com", "en")
	matchRepo.statsErr = errors.New("db boom")

	token, err := auth.GenerateAccessToken(u.ID, testJWTSecret)
	require.NoError(t, err)

	rec := doGetProfile(e, strconvUint(u.ID), token)
	assert.Equal(t, http.StatusInternalServerError, rec.Code,
		"DB error must surface as 500, not silently zeroed")

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "INTERNAL_ERROR", errResp["error"]["code"])

	// PII guard: the error-branch response body must not leak user fields either.
	body := rec.Body.String()
	assert.NotContains(t, body, "alice-err-probe@example.com", "email must never leak on 500")
	assert.NotContains(t, body, "\"email\"", "email field must not appear on 500")
	assert.NotContains(t, body, "passwordHash")
	assert.NotContains(t, body, "password_hash")
}

func TestGetProfile_NeverLeaksPII(t *testing.T) {
	repo, matchRepo, e := setupUserHandlerWithMatches()
	// Use a distinctive email so any leak is obvious in the response body.
	u := repo.addUser("alice", "alice-leak-probe@example.com", "en")
	matchRepo.statsOverride = &struct{ wins, losses, abandoned int }{3, 1, 0}

	token, err := auth.GenerateAccessToken(u.ID, testJWTSecret)
	require.NoError(t, err)

	rec := doGetProfile(e, strconvUint(u.ID), token)
	require.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.NotContains(t, body, "alice-leak-probe@example.com", "email must never leak")
	assert.NotContains(t, body, "\"email\"", "email field must not appear")
	assert.NotContains(t, body, "passwordHash")
	assert.NotContains(t, body, "password_hash")
	assert.NotContains(t, body, "deletedAt")
}

func TestGetProfile_AuthFailures_DoNotCallStats(t *testing.T) {
	// 400 on bad id, 401 on missing token, 403 on foreign id — stats repo must
	// not be invoked because the auth check rejects before the stats call.
	t.Run("bad path id", func(t *testing.T) {
		repo, matchRepo, e := setupUserHandlerWithMatches()
		u := repo.addUser("alice", "alice@example.com", "en")
		token, err := auth.GenerateAccessToken(u.ID, testJWTSecret)
		require.NoError(t, err)

		rec := doGetProfile(e, "abc", token)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Zero(t, matchRepo.getStatsCalls, "stats must not be queried on 400")
	})

	t.Run("missing token", func(t *testing.T) {
		_, matchRepo, e := setupUserHandlerWithMatches()
		rec := doGetProfile(e, "1", "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Zero(t, matchRepo.getStatsCalls, "stats must not be queried on 401")
	})

	t.Run("foreign id", func(t *testing.T) {
		repo, matchRepo, e := setupUserHandlerWithMatches()
		repo.addUser("alice", "alice@example.com", "en")
		repo.addUser("bob", "bob@example.com", "en")
		token, err := auth.GenerateAccessToken(1, testJWTSecret)
		require.NoError(t, err)

		rec := doGetProfile(e, "2", token)
		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Zero(t, matchRepo.getStatsCalls, "stats must not be queried on 403")
	})
}

// TestGetProfile_StatsMatchListTotal verifies the invariant from AC #2: the
// sum wins+losses+abandoned computed by GetStatsForUser matches the total
// returned by GetMatchesForUser for the same user, across a realistic seed.
// Uses the mock's seed-driven stats path (statsOverride not set) so both
// code paths exercise the same participation predicate.
func TestGetProfile_StatsMatchListTotal(t *testing.T) {
	repo, matchRepo, e := setupUserHandlerWithMatches()
	viewer := repo.addUser("viewer", "v@example.com", "en")
	p2 := repo.addUser("p2", "p2@example.com", "en")
	p3 := repo.addUser("p3", "p3@example.com", "en")
	p4 := repo.addUser("p4", "p4@example.com", "en")
	bystander := repo.addUser("bystander", "b@example.com", "en")

	// Viewer across multiple seats + team outcomes:
	//  - seat 0 (team A), completed, team A wins -> win
	//  - seat 0 (team A), completed, team B wins -> loss
	//  - seat 3 (team B), completed, team B wins -> win (exercises CASE at seat 3)
	//  - seat 2 (team A), abandoned -> abandoned
	//  - match not involving viewer (bystander-only) -> ignored in both paths
	base := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	abandoner := viewer.ID
	matchRepo.matches = []match.Match{
		seedMatch(1, base, base.Add(25*time.Minute),
			[4]uint{viewer.ID, p2.ID, p3.ID, p4.ID},
			"completed", 0, nil, "bitola", "1001", 1020, 640, nil),
		seedMatch(2, base.Add(-time.Hour), base.Add(-time.Hour+25*time.Minute),
			[4]uint{viewer.ID, p2.ID, p3.ID, p4.ID},
			"completed", 1, nil, "bitola", "1001", 500, 1020, nil),
		seedMatch(3, base.Add(-2*time.Hour), base.Add(-2*time.Hour+30*time.Minute),
			[4]uint{p2.ID, p3.ID, p4.ID, viewer.ID},
			"completed", 1, nil, "bitola", "1001", 600, 1050, nil),
		seedMatch(4, base.Add(-3*time.Hour), base.Add(-3*time.Hour+10*time.Minute),
			[4]uint{p2.ID, p3.ID, viewer.ID, p4.ID},
			"abandoned", 0, &abandoner, "bitola", "1001", 300, 400, nil),
		seedMatch(5, base.Add(-4*time.Hour), base.Add(-4*time.Hour+20*time.Minute),
			[4]uint{bystander.ID, p2.ID, p3.ID, p4.ID},
			"completed", 0, nil, "bitola", "1001", 1001, 500, nil),
	}

	token, err := auth.GenerateAccessToken(viewer.ID, testJWTSecret)
	require.NoError(t, err)

	// Hit /profile — drives GetStatsForUser and returns aggregate counts.
	profRec := doGetProfile(e, strconvUint(viewer.ID), token)
	require.Equal(t, http.StatusOK, profRec.Code)
	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(profRec.Body.Bytes(), &resp))
	var prof user.ProfileResponse
	require.NoError(t, json.Unmarshal(resp["data"], &prof))

	// Hit /matches — drives GetMatchesForUser and returns total count.
	matchRec := doListMatches(e, strconvUint(viewer.ID), "limit=50", token)
	require.Equal(t, http.StatusOK, matchRec.Code)
	matchResp := decodeMatchesResponse(t, matchRec.Body.Bytes())

	// AC #2 invariant: aggregate sum must equal matches total.
	assert.Equal(t, int64(prof.TotalGamesPlayed), matchResp.Total,
		"totalGamesPlayed must equal /matches total for same user")
	assert.Equal(t, prof.Wins+prof.Losses+prof.Abandoned, prof.TotalGamesPlayed)

	// Expected by construction: 2 wins, 1 loss, 1 abandoned, bystander ignored.
	assert.Equal(t, 2, prof.Wins)
	assert.Equal(t, 1, prof.Losses)
	assert.Equal(t, 1, prof.Abandoned)

	// Bystander's profile: sees only their one match (a win — seat 0 team A, team A wins).
	byToken, err := auth.GenerateAccessToken(bystander.ID, testJWTSecret)
	require.NoError(t, err)
	byRec := doGetProfile(e, strconvUint(bystander.ID), byToken)
	require.Equal(t, http.StatusOK, byRec.Code)
	var byResp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(byRec.Body.Bytes(), &byResp))
	var byProf user.ProfileResponse
	require.NoError(t, json.Unmarshal(byResp["data"], &byProf))
	assert.Equal(t, 1, byProf.Wins)
	assert.Equal(t, 0, byProf.Losses)
	assert.Equal(t, 0, byProf.Abandoned)
	assert.Equal(t, 1, byProf.TotalGamesPlayed)
}

func TestGetProfile_Forbidden_WraparoundID(t *testing.T) {
	repo, e := setupUserHandler()
	repo.addUser("alice", "alice@example.com", "en") // gets ID=1

	token, err := auth.GenerateAccessToken(1, testJWTSecret)
	require.NoError(t, err)

	// 4294967297 = 2^32 + 1; on 32-bit uint it would truncate to 1 == userID.
	// On 64-bit with the fixed uint64 comparison, this must return 403.
	rec := doGetProfile(e, "4294967297", token)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"wraparound ID must not bypass auth check (D86)")
}

// --- ListMatches outcome filter + sort ---

func TestListMatches_OutcomeFilter(t *testing.T) {
	repo, matchRepo, e := setupUserHandlerWithMatches()
	viewer := repo.addUser("viewer", "v@example.com", "en")
	_ = repo.addUser("p2", "p2@example.com", "en")
	_ = repo.addUser("p3", "p3@example.com", "en")
	_ = repo.addUser("p4", "p4@example.com", "en")

	seats := [4]uint{viewer.ID, 2, 3, 4} // viewer seat 0 → team A
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	// Per-player abandonment semantics: the opponent's abandonment (match 4)
	// counts as a viewer win; the viewer's own (match 3) and the legacy
	// NULL-abandoner row (match 5) stay "abandoned".
	viewerAb := viewer.ID
	oppAb := uint(2) // seat 1, team B → winner_team 0 = viewer's team
	matchRepo.matches = []match.Match{
		seedMatch(1, base, base.Add(20*time.Minute), seats, "completed", 0, nil, "bitola", "1001", 1010, 600, nil),                                    // win
		seedMatch(2, base.Add(-1*time.Hour), base.Add(-1*time.Hour+20*time.Minute), seats, "completed", 1, nil, "bitola", "1001", 700, 1010, nil),     // loss
		seedMatch(3, base.Add(-2*time.Hour), base.Add(-2*time.Hour+5*time.Minute), seats, "abandoned", 1, &viewerAb, "bitola", "1001", 300, 200, nil), // abandoned (own)
		seedMatch(4, base.Add(-3*time.Hour), base.Add(-3*time.Hour+5*time.Minute), seats, "abandoned", 0, &oppAb, "bitola", "1001", 400, 300, nil),    // win (opponent abandoned)
		seedMatch(5, base.Add(-4*time.Hour), base.Add(-4*time.Hour+5*time.Minute), seats, "abandoned", 0, nil, "bitola", "1001", 0, 0, nil),           // abandoned (legacy NULL)
	}

	token, err := auth.GenerateAccessToken(viewer.ID, testJWTSecret)
	require.NoError(t, err)

	cases := []struct {
		outcome string
		want    int
		expect  string
	}{
		{"win", 2, "win"},
		{"loss", 1, "loss"},
		{"abandoned", 2, "abandoned"},
		{"all", 5, ""},
		{"", 5, ""},
	}
	for _, tc := range cases {
		t.Run("outcome="+tc.outcome, func(t *testing.T) {
			rec := doListMatches(e, strconvUint(viewer.ID), "outcome="+tc.outcome, token)
			require.Equal(t, http.StatusOK, rec.Code)
			resp := decodeMatchesResponse(t, rec.Body.Bytes())
			require.Len(t, resp.Items, tc.want)
			assert.Equal(t, int64(tc.want), resp.Total)
			if tc.expect != "" {
				assert.Equal(t, tc.expect, resp.Items[0].Outcome)
			}
		})
	}
}

// TestListMatches_PerPlayerAbandonment pins the full I/O matrix of the
// per-player outcome + endReason mapping across all four perspectives on one
// abandoned match (abandoner / partner / two opponents), the legacy
// NULL-abandoner row, and the surrender/natural completed variants.
func TestListMatches_PerPlayerAbandonment(t *testing.T) {
	repo, matchRepo, e := setupUserHandlerWithMatches()
	p1 := repo.addUser("p1", "p1@example.com", "en") // seat 0, team A
	p2 := repo.addUser("p2", "p2@example.com", "en") // seat 1, team B
	p3 := repo.addUser("p3", "p3@example.com", "en") // seat 2, team A
	p4 := repo.addUser("p4", "p4@example.com", "en") // seat 3, team B

	seats := [4]uint{p1.ID, p2.ID, p3.ID, p4.ID}
	base := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	abandoner := p1.ID   // team A abandons → winner_team 1
	surrenderer := p2.ID // team B surrenders → completed, winner_team 0

	surrendered := seedMatch(3, base.Add(-2*time.Hour), base.Add(-2*time.Hour+15*time.Minute),
		seats, "completed", 0, nil, "bitola", "1001", 720, 430, nil)
	surrendered.SurrenderedBy = &surrenderer

	matchRepo.matches = []match.Match{
		seedMatch(1, base, base.Add(10*time.Minute), seats, "abandoned", 1, &abandoner, "bitola", "1001", 300, 260, nil),
		seedMatch(2, base.Add(-1*time.Hour), base.Add(-1*time.Hour+10*time.Minute), seats, "abandoned", 0, nil, "bitola", "1001", 0, 0, nil),
		surrendered,
		seedMatch(4, base.Add(-3*time.Hour), base.Add(-3*time.Hour+25*time.Minute), seats, "completed", 1, nil, "bitola", "1001", 600, 1010, nil),
	}

	cases := []struct {
		name   string
		viewer uint
		// Expected per match ID: outcome + endReason.
		want map[uint][2]string
	}{
		{name: "abandoner", viewer: p1.ID, want: map[uint][2]string{
			1: {"abandoned", "abandonment"},
			2: {"abandoned", "abandonment"},
			3: {"win", "surrender"},
			4: {"loss", "natural"},
		}},
		{name: "opponent of abandoner", viewer: p2.ID, want: map[uint][2]string{
			1: {"win", "abandonment"},
			2: {"abandoned", "abandonment"},
			3: {"loss", "surrender"},
			4: {"win", "natural"},
		}},
		{name: "partner of abandoner", viewer: p3.ID, want: map[uint][2]string{
			1: {"loss", "abandonment"},
			2: {"abandoned", "abandonment"},
			3: {"win", "surrender"},
			4: {"loss", "natural"},
		}},
		{name: "second opponent", viewer: p4.ID, want: map[uint][2]string{
			1: {"win", "abandonment"},
			2: {"abandoned", "abandonment"},
			3: {"loss", "surrender"},
			4: {"win", "natural"},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token, err := auth.GenerateAccessToken(tc.viewer, testJWTSecret)
			require.NoError(t, err)

			rec := doListMatches(e, strconvUint(tc.viewer), "", token)
			require.Equal(t, http.StatusOK, rec.Code)
			resp := decodeMatchesResponse(t, rec.Body.Bytes())
			require.Len(t, resp.Items, len(tc.want))
			for _, item := range resp.Items {
				want, ok := tc.want[item.ID]
				require.True(t, ok, "unexpected match %d", item.ID)
				assert.Equal(t, want[0], item.Outcome, "match %d outcome", item.ID)
				assert.Equal(t, want[1], item.EndReason, "match %d endReason", item.ID)
			}
		})
	}
}

func TestListMatches_SortOrder(t *testing.T) {
	repo, matchRepo, e := setupUserHandlerWithMatches()
	viewer := repo.addUser("viewer", "v@example.com", "en")
	_ = repo.addUser("p2", "p2@example.com", "en")
	_ = repo.addUser("p3", "p3@example.com", "en")
	_ = repo.addUser("p4", "p4@example.com", "en")

	seats := [4]uint{viewer.ID, 2, 3, 4}
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	matchRepo.matches = []match.Match{
		seedMatch(10, base, base.Add(5*time.Minute), seats, "completed", 0, nil, "bitola", "1001", 1001, 500, nil),
		seedMatch(11, base.Add(1*time.Hour), base.Add(1*time.Hour+5*time.Minute), seats, "completed", 0, nil, "bitola", "1001", 1001, 500, nil),
		seedMatch(12, base.Add(2*time.Hour), base.Add(2*time.Hour+5*time.Minute), seats, "completed", 0, nil, "bitola", "1001", 1001, 500, nil),
	}

	token, err := auth.GenerateAccessToken(viewer.ID, testJWTSecret)
	require.NoError(t, err)

	rec := doListMatches(e, strconvUint(viewer.ID), "sort=old", token)
	require.Equal(t, http.StatusOK, rec.Code)
	resp := decodeMatchesResponse(t, rec.Body.Bytes())
	require.Len(t, resp.Items, 3)
	assert.Equal(t, uint(10), resp.Items[0].ID, "oldest first")
	assert.Equal(t, uint(12), resp.Items[2].ID)
}

func TestListMatches_BadRequest_InvalidOutcomeOrSort(t *testing.T) {
	repo, _, e := setupUserHandlerWithMatches()
	u := repo.addUser("alice", "alice@example.com", "en")
	token, err := auth.GenerateAccessToken(u.ID, testJWTSecret)
	require.NoError(t, err)

	for _, q := range []string{"outcome=bogus", "sort=sideways"} {
		rec := doListMatches(e, strconvUint(u.ID), q, token)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "query %q should be 400", q)
	}
}

// --- GetCareer ---

func TestGetCareer_Success(t *testing.T) {
	repo, matchRepo, e := setupUserHandlerWithMatches()
	viewer := repo.addUser("viewer", "v@example.com", "en")
	mate := repo.addUser("mate", "m@example.com", "en")
	rival := repo.addUser("rival", "r@example.com", "en")

	matchRepo.careerAgg = match.CareerAggregates{
		Capots:          3,
		AvgMatchSeconds: 1500,
		HasBestHand:     true,
		BestHandPoints:  252,
		BestHandNumber:  4,
		BestHandAt:      time.Date(2026, 5, 1, 18, 0, 0, 0, time.UTC),
		StreakKind:      "win",
		StreakLength:    5,
	}
	matchRepo.partners = []match.PartnerAggregate{{UserID: mate.ID, Played: 8, Wins: 6}}
	matchRepo.rivals = []match.RivalAggregate{{UserID: rival.ID, Wins: 4, Losses: 2}}

	token, err := auth.GenerateAccessToken(viewer.ID, testJWTSecret)
	require.NoError(t, err)

	rec := doGetCareer(e, strconvUint(viewer.ID), token)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var data user.CareerResponse
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	assert.Equal(t, 3, data.Capots)
	assert.Equal(t, 1500, data.AvgMatchSeconds)
	assert.Equal(t, "win", data.Streak.Kind)
	assert.Equal(t, 5, data.Streak.Length)
	require.NotNil(t, data.BestHand)
	assert.Equal(t, 252, data.BestHand.Points)
	require.Len(t, data.TopPartners, 1)
	assert.Equal(t, "mate", data.TopPartners[0].Username)
	assert.Equal(t, 8, data.TopPartners[0].Played)
	require.Len(t, data.TopRivals, 1)
	assert.Equal(t, "rival", data.TopRivals[0].Username)
	assert.Equal(t, 4, data.TopRivals[0].Wins)

	// No PII leak in the career response either.
	body := rec.Body.String()
	assert.NotContains(t, body, "@example.com")
	assert.NotContains(t, body, "passwordHash")
}

func TestGetCareer_EmptyOmitsBestHand(t *testing.T) {
	repo, matchRepo, e := setupUserHandlerWithMatches()
	viewer := repo.addUser("viewer", "v@example.com", "en")
	matchRepo.careerAgg = match.CareerAggregates{StreakKind: "none"}

	token, err := auth.GenerateAccessToken(viewer.ID, testJWTSecret)
	require.NoError(t, err)

	rec := doGetCareer(e, strconvUint(viewer.ID), token)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var data user.CareerResponse
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	assert.Nil(t, data.BestHand, "bestHand omitted when there is none")
	assert.Equal(t, "none", data.Streak.Kind)
	assert.NotNil(t, data.TopPartners, "must be an empty slice, not null")
	assert.NotNil(t, data.TopRivals, "must be an empty slice, not null")
}

func TestGetCareer_AuthFailures_DoNotQuery(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		_, matchRepo, e := setupUserHandlerWithMatches()
		rec := doGetCareer(e, "1", "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Zero(t, matchRepo.getCareerCalls)
	})

	t.Run("foreign id", func(t *testing.T) {
		repo, matchRepo, e := setupUserHandlerWithMatches()
		repo.addUser("alice", "alice@example.com", "en")
		repo.addUser("bob", "bob@example.com", "en")
		token, err := auth.GenerateAccessToken(1, testJWTSecret)
		require.NoError(t, err)
		rec := doGetCareer(e, "2", token)
		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Zero(t, matchRepo.getCareerCalls)
	})
}

func TestGetCareer_DBError(t *testing.T) {
	repo, matchRepo, e := setupUserHandlerWithMatches()
	u := repo.addUser("alice", "alice@example.com", "en")
	matchRepo.careerErr = errors.New("db boom")

	token, err := auth.GenerateAccessToken(u.ID, testJWTSecret)
	require.NoError(t, err)

	rec := doGetCareer(e, strconvUint(u.ID), token)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
