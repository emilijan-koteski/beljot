package user

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/match"
)

// ProfileResponse is the SELF profile DTO returned by GetProfile when the
// path id equals the authenticated viewer (Story 11.3 made the endpoint
// public — a foreign id now gets the narrower PublicProfileResponse instead of
// a 403). WalletBalance, LoginStreakDays, LanguagePreference and
// UsernameChangedAt are PRIVATE self-only figures: they are absent from
// PublicProfileResponse and must never be added to any shared/public shape.
type ProfileResponse struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	// UsernameChangedAt is when the username was last changed (null if never).
	// The client derives the change-cooldown state / next-allowed date from it.
	UsernameChangedAt  *time.Time `json:"usernameChangedAt,omitempty"`
	LanguagePreference string     `json:"languagePreference"`
	WalletBalance      int        `json:"walletBalance"`
	LoginStreakDays    int        `json:"loginStreakDays"`
	// XP & level (Story 9.5). TotalXP is the lifetime total; Level is derived
	// from it (never stored); XPIntoLevel/XPForNextLevel drive the profile XP
	// bar (fill = XPIntoLevel / XPForNextLevel). Story 11.3 (D2) made these
	// PUBLIC: level is already public on room seat tiles and XP is non-sensitive
	// progression data, so the epic AC lifts level + total XP onto the public
	// profile. PublicProfileResponse carries all four verbatim.
	TotalXP        int `json:"totalXp"`
	Level          int `json:"level"`
	XPIntoLevel    int `json:"xpIntoLevel"`
	XPForNextLevel int `json:"xpForNextLevel"`
	// Honor (Story 9.7). Unlike every private field above, these are
	// PUBLIC-SAFE: honor exists precisely so other players can judge whether to
	// play with you, and Story 9.8 gates room access on it. When Epic 11 builds
	// the public player-profile DTO it can lift these seven fields verbatim
	// without re-litigating the privacy question.
	//
	// HonorScore is the AUTHORITATIVE recomputed value, never the lagging
	// users.honor_score snapshot column. HonorTier is a stable machine token
	// the client maps to an i18n label. The Completed/Abandoned totals are RAW
	// undecayed lifetime counts. IsNewPlayer is presentation-only suppression:
	// the score and tier are still populated and still authoritative when it is
	// true. HonorTrendDelta/Direction compare the last honorTrendWindow matches
	// against the honorTrendWindow matches BEFORE those (never against the
	// lifetime score — different sample sizes carry different Bayesian prior drag,
	// which made flawless players read "Slipping"). The one honor figure that
	// costs a query.
	HonorScore          int       `json:"honorScore"`
	HonorTier           string    `json:"honorTier"`
	HonorCompletedTotal int64     `json:"honorCompletedTotal"`
	HonorAbandonedTotal int64     `json:"honorAbandonedTotal"`
	IsNewPlayer         bool      `json:"isNewPlayer"`
	HonorTrendDelta     int       `json:"honorTrendDelta"`
	HonorTrendDirection string    `json:"honorTrendDirection"`
	CreatedAt           time.Time `json:"createdAt"`
	TotalGamesPlayed    int       `json:"totalGamesPlayed"`
	Wins                int       `json:"wins"`
	Losses              int       `json:"losses"`
	Abandoned           int       `json:"abandoned"`
}

// PublicProfileResponse is the PUBLIC projection returned by GET
// /users/:id/profile when :id is NOT the authenticated viewer (Story 11.3
// FR47). It carries only public-safe fields — identity, member-since,
// progression (level + XP, per D2), the full honor section, and the
// win/loss/abandoned record — and deliberately OMITS every private figure the
// self ProfileResponse holds: Email, PasswordHash, WalletBalance,
// LoginStreakDays, LanguagePreference, UsernameChangedAt. Every value is
// computed for the PATH id (the subject), never the viewer — see GetProfile.
type PublicProfileResponse struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"createdAt"`
	// Progression (Story 9.5) — public per Story 11.3 D2.
	TotalXP        int `json:"totalXp"`
	Level          int `json:"level"`
	XPIntoLevel    int `json:"xpIntoLevel"`
	XPForNextLevel int `json:"xpForNextLevel"`
	// Honor (Story 9.7) — public by construction; the client decides New Player
	// suppression while the score/tier stay authoritative (mirrors the self DTO).
	HonorScore          int    `json:"honorScore"`
	HonorTier           string `json:"honorTier"`
	HonorCompletedTotal int64  `json:"honorCompletedTotal"`
	HonorAbandonedTotal int64  `json:"honorAbandonedTotal"`
	IsNewPlayer         bool   `json:"isNewPlayer"`
	HonorTrendDelta     int    `json:"honorTrendDelta"`
	HonorTrendDirection string `json:"honorTrendDirection"`
	TotalGamesPlayed    int    `json:"totalGamesPlayed"`
	Wins                int    `json:"wins"`
	Losses              int    `json:"losses"`
	Abandoned           int    `json:"abandoned"`
}

type UpdatePreferencesRequest struct {
	LanguagePreference string `json:"languagePreference"`
}

var supportedLanguages = map[string]struct{}{
	"en": {},
	"sr": {},
	"mk": {},
	"hr": {},
}

// IsSupportedLanguage reports whether code is one of the registered UI
// languages. Exported so the auth package can validate register-time language
// preferences without duplicating the allowlist.
func IsSupportedLanguage(code string) bool {
	_, ok := supportedLanguages[code]
	return ok
}

// MatchPlayer is the per-seat participant embedded in a match list item.
// Bot seats carry userId 0 and an empty username with isBot true — the
// client renders the localized seat-derived bot name.
type MatchPlayer struct {
	Seat     int    `json:"seat"`
	UserID   uint   `json:"userId"`
	Username string `json:"username"`
	IsBot    bool   `json:"isBot"`
}

// MatchHandView is the per-hand scoring breakdown embedded in a match list item.
type MatchHandView struct {
	HandNumber      int  `json:"handNumber"`
	TeamACardPoints int  `json:"teamACardPoints"`
	TeamBCardPoints int  `json:"teamBCardPoints"`
	TeamADeclPoints int  `json:"teamADeclPoints"`
	TeamBDeclPoints int  `json:"teamBDeclPoints"`
	LastTrickTeam   int  `json:"lastTrickTeam"`
	LastTrickBonus  int  `json:"lastTrickBonus"`
	Capot           bool `json:"capot"`
	CapotTeam       *int `json:"capotTeam,omitempty"`
	CapotBonus      int  `json:"capotBonus"`
	FailedContract  bool `json:"failedContract"`
	ContractingTeam int  `json:"contractingTeam"`
	TeamAHandTotal  int  `json:"teamAHandTotal"`
	TeamBHandTotal  int  `json:"teamBHandTotal"`
}

// MatchListItem is the per-match DTO returned by GET /users/:id/matches.
type MatchListItem struct {
	ID          uint      `json:"id"`
	Variant     string    `json:"variant"`
	MatchMode   string    `json:"matchMode"`
	StartedAt   time.Time `json:"startedAt"`
	CompletedAt time.Time `json:"completedAt"`
	Status      string    `json:"status"`
	WinnerTeam  int       `json:"winnerTeam"`
	TeamAScore  int       `json:"teamAScore"`
	TeamBScore  int       `json:"teamBScore"`
	HasBots     bool      `json:"hasBots"`
	AbandonedBy *uint     `json:"abandonedBy,omitempty"`
	ViewerSeat  int       `json:"viewerSeat"`
	Outcome     string    `json:"outcome"`
	// EndReason says why the match ended: "abandonment" (abandoned rows),
	// "surrender" (completed via an accepted surrender), or "natural". The
	// client renders the muted "ended early" history marker from it.
	EndReason string          `json:"endReason"`
	Players   []MatchPlayer   `json:"players"`
	Hands     []MatchHandView `json:"hands"`
}

// MatchesListResponse is the envelope returned by GET /users/:id/matches.
type MatchesListResponse struct {
	Items  []MatchListItem `json:"items"`
	Total  int64           `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

// CareerStreak is the viewer's current win/loss streak. Kind is "win", "loss",
// or "none" (no completed matches yet); Length is 0 when Kind is "none".
type CareerStreak struct {
	Kind   string `json:"kind"`
	Length int    `json:"length"`
}

// BestHand is the single highest-scoring hand the viewer's team ever recorded.
type BestHand struct {
	Points      int       `json:"points"`
	HandNumber  int       `json:"handNumber"`
	CompletedAt time.Time `json:"completedAt"`
}

// PartnerStat is one most-played-teammate row in the career response.
type PartnerStat struct {
	UserID   uint   `json:"userId"`
	Username string `json:"username"`
	Played   int    `json:"played"`
	Wins     int    `json:"wins"`
}

// RivalStat is one most-faced-opponent row in the career response.
type RivalStat struct {
	UserID   uint   `json:"userId"`
	Username string `json:"username"`
	Wins     int    `json:"wins"`
	Losses   int    `json:"losses"`
}

// CareerResponse is the envelope returned by GET /users/:id/career — the
// derived stats that power the profile hero (capots), streak callout,
// milestones, partner spotlight, and rivalries.
type CareerResponse struct {
	Capots          int `json:"capots"`
	AvgMatchSeconds int `json:"avgMatchSeconds"`
	// CareerPoints is the lifetime sum of the subject's own team score across
	// their COMPLETED matches (Story 11.3 net-new aggregate — see
	// MatchRepository.GetCareerPointsForUser). Surfaced on both the public and
	// self career views.
	CareerPoints int64         `json:"careerPoints"`
	Streak       CareerStreak  `json:"streak"`
	BestHand     *BestHand     `json:"bestHand,omitempty"`
	LastPlayedAt *time.Time    `json:"lastPlayedAt,omitempty"`
	TopPartners  []PartnerStat `json:"topPartners"`
	TopRivals    []RivalStat   `json:"topRivals"`
}

// careerListLimit caps how many partner / rival rows the career endpoint
// returns (one featured + a short list, matching the profile sidebar design).
const careerListLimit = 4

type UserHandler struct {
	userRepo  UserRepository
	matchRepo match.MatchRepository
}

func NewUserHandler(userRepo UserRepository, matchRepo match.MatchRepository) *UserHandler {
	return &UserHandler{userRepo: userRepo, matchRepo: matchRepo}
}

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

func (h *UserHandler) GetProfile(c echo.Context) error {
	authUserID, err := getUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	paramID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || paramID == 0 {
		return apperr.ErrBadRequest
	}

	// Story 11.3: this endpoint is PUBLIC. The subject is ALWAYS the path id —
	// every repo call and honor query below keys on subjectID, not authUserID.
	// (Removing the old self-gate WITHOUT this swap would silently serve the
	// viewer's own data under another player's URL.) A non-existent subject 404s
	// exactly as a self-lookup did; a huge/wrapped id resolves to no user and
	// 404s too, so there is no self-serve via truncation.
	subjectID := uint(paramID)

	u, err := h.userRepo.FindByID(subjectID)
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}
	if u == nil {
		return apperr.ErrUserNotFound
	}

	wins, losses, abandoned, err := h.matchRepo.GetStatsForUser(subjectID)
	if err != nil {
		return fmt.Errorf("fetching profile stats: %w", err)
	}

	level, xpIntoLevel, xpForNextLevel := LevelProgress(u.TotalXP)

	// Honor (Story 9.7). The lifetime score is recomputed from the stored
	// weights at request time — that recompute IS the authority, so it can
	// never be stale no matter how long ago the row was last written.
	now := time.Now().UTC()
	honor := NewHonorSnapshot(
		u.HonorCompletedWeight, u.HonorAbandonedWeight, u.HonorDecayedAt,
		u.HonorCompletedTotal, u.HonorAbandonedTotal, now,
	)

	// The trend is the ONE honor figure that must query `matches`: the stored
	// weights are running aggregates and cannot be windowed. It runs here, on
	// the profile read path only — never on the auth envelope and never in
	// Story 9.8's join gate. Both windows are undecayed; the windows themselves
	// are the recency mechanism.
	//
	// It compares the newest window against the one before it, NOT against the
	// lifetime score — comparing a 20-match window to a lifetime aggregate mixed
	// two different Bayesian prior drags and made flawless active players read
	// "Slipping" forever (code review 2026-07-29). HonorTrendWindowed enforces
	// the equal-sample-size precondition and returns flat when it does not hold.
	//
	// Best-effort: a failed trend query degrades to a flat trend rather than
	// failing the whole profile, which would take the far more important
	// score/tier/counters down with it.
	trendDelta, trendDirection := 0, HonorTrendFlat
	windows, err := h.matchRepo.GetHonorTrendWindowsForUser(subjectID, HonorTrendWindow())
	if err != nil {
		slog.Error("profile: failed to read honor trend windows", "userID", subjectID, "error", err)
	} else {
		trendDelta, trendDirection = HonorTrendWindowed(
			windows.RecentCompleted, windows.RecentAbandoned,
			windows.PriorCompleted, windows.PriorAbandoned,
			now,
		)
	}

	// Self → the full private profile (wallet / streak / language / username
	// cooldown); any other viewer → the narrower public projection. The branch
	// compares paramID as uint64 to stay wraparound-safe, mirroring the old
	// self-gate (D86). subjectID above has already resolved the actual subject.
	if paramID == uint64(authUserID) {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"data": ProfileResponse{
				ID:                  u.ID,
				Username:            u.Username,
				UsernameChangedAt:   u.UsernameChangedAt,
				LanguagePreference:  u.LanguagePreference,
				WalletBalance:       u.WalletBalance,
				LoginStreakDays:     u.LoginStreakDays,
				TotalXP:             u.TotalXP,
				Level:               level,
				XPIntoLevel:         xpIntoLevel,
				XPForNextLevel:      xpForNextLevel,
				HonorScore:          honor.Score,
				HonorTier:           honor.Tier,
				HonorCompletedTotal: honor.CompletedTotal,
				HonorAbandonedTotal: honor.AbandonedTotal,
				IsNewPlayer:         honor.IsNewPlayer,
				HonorTrendDelta:     trendDelta,
				HonorTrendDirection: trendDirection,
				CreatedAt:           u.CreatedAt,
				TotalGamesPlayed:    wins + losses + abandoned,
				Wins:                wins,
				Losses:              losses,
				Abandoned:           abandoned,
			},
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": PublicProfileResponse{
			ID:                  u.ID,
			Username:            u.Username,
			CreatedAt:           u.CreatedAt,
			TotalXP:             u.TotalXP,
			Level:               level,
			XPIntoLevel:         xpIntoLevel,
			XPForNextLevel:      xpForNextLevel,
			HonorScore:          honor.Score,
			HonorTier:           honor.Tier,
			HonorCompletedTotal: honor.CompletedTotal,
			HonorAbandonedTotal: honor.AbandonedTotal,
			IsNewPlayer:         honor.IsNewPlayer,
			HonorTrendDelta:     trendDelta,
			HonorTrendDirection: trendDirection,
			TotalGamesPlayed:    wins + losses + abandoned,
			Wins:                wins,
			Losses:              losses,
			Abandoned:           abandoned,
		},
	})
}

// GetCareer returns the derived career stats for the SUBJECT (path id):
// capots won, average match length, lifetime career points, current streak,
// best hand, and the most-played partners / most-faced rivals. Story 11.3 made
// it public — any authenticated viewer may read any player's career (an unknown
// subject 404s). Like the other user endpoints, only participant usernames +
// ids are exposed — never email, password hash, or language preference.
func (h *UserHandler) GetCareer(c echo.Context) error {
	// Auth is still required (the route sits under the authenticated group), but
	// the viewer's own id is no longer the subject — the path id is (Story 11.3).
	if _, err := getUserID(c); err != nil {
		return apperr.ErrUnauthorized
	}

	paramID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || paramID == 0 {
		return apperr.ErrBadRequest
	}

	// Story 11.3: career is public — the subject is the path id, and every
	// aggregate below keys on subjectID, never authUserID (D1). An unknown
	// subject 404s (unlike the old self-only path, which relied on the token to
	// prove the user existed).
	subjectID := uint(paramID)

	subject, err := h.userRepo.FindByID(subjectID)
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}
	if subject == nil {
		return apperr.ErrUserNotFound
	}

	agg, err := h.matchRepo.GetCareerAggregatesForUser(subjectID)
	if err != nil {
		return fmt.Errorf("fetching career aggregates: %w", err)
	}

	careerPoints, err := h.matchRepo.GetCareerPointsForUser(subjectID)
	if err != nil {
		return fmt.Errorf("fetching career points: %w", err)
	}

	partnerAggs, err := h.matchRepo.GetTopPartnersForUser(subjectID, careerListLimit)
	if err != nil {
		return fmt.Errorf("fetching career partners: %w", err)
	}

	rivalAggs, err := h.matchRepo.GetTopRivalsForUser(subjectID, careerListLimit)
	if err != nil {
		return fmt.Errorf("fetching career rivals: %w", err)
	}

	usernames, err := h.loadUsernamesForAggregates(partnerAggs, rivalAggs)
	if err != nil {
		return fmt.Errorf("loading career usernames: %w", err)
	}

	partners := make([]PartnerStat, 0, len(partnerAggs))
	for _, p := range partnerAggs {
		partners = append(partners, PartnerStat{
			UserID:   p.UserID,
			Username: usernames[p.UserID],
			Played:   p.Played,
			Wins:     p.Wins,
		})
	}

	rivals := make([]RivalStat, 0, len(rivalAggs))
	for _, r := range rivalAggs {
		rivals = append(rivals, RivalStat{
			UserID:   r.UserID,
			Username: usernames[r.UserID],
			Wins:     r.Wins,
			Losses:   r.Losses,
		})
	}

	var bestHand *BestHand
	if agg.HasBestHand {
		bestHand = &BestHand{
			Points:      agg.BestHandPoints,
			HandNumber:  agg.BestHandNumber,
			CompletedAt: agg.BestHandAt,
		}
	}

	var lastPlayedAt *time.Time
	if agg.HasLastPlayed {
		v := agg.LastPlayedAt
		lastPlayedAt = &v
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": CareerResponse{
			Capots:          agg.Capots,
			AvgMatchSeconds: agg.AvgMatchSeconds,
			CareerPoints:    careerPoints,
			Streak:          CareerStreak{Kind: agg.StreakKind, Length: agg.StreakLength},
			BestHand:        bestHand,
			LastPlayedAt:    lastPlayedAt,
			TopPartners:     partners,
			TopRivals:       rivals,
		},
	})
}

// loadUsernamesForAggregates batches the username lookup for all partner +
// rival IDs into a single FindManyByIDs call, returning a map keyed by userID.
func (h *UserHandler) loadUsernamesForAggregates(partners []match.PartnerAggregate, rivals []match.RivalAggregate) (map[uint]string, error) {
	seen := make(map[uint]struct{}, len(partners)+len(rivals))
	ids := make([]uint, 0, len(partners)+len(rivals))
	add := func(id uint) {
		if id == 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for _, p := range partners {
		add(p.UserID)
	}
	for _, r := range rivals {
		add(r.UserID)
	}
	if len(ids) == 0 {
		return map[uint]string{}, nil
	}
	users, err := h.userRepo.FindManyByIDs(ids)
	if err != nil {
		return nil, err
	}
	result := make(map[uint]string, len(users))
	for _, u := range users {
		result[u.ID] = u.Username
	}
	return result, nil
}

func (h *UserHandler) UpdatePreferences(c echo.Context) error {
	authUserID, err := getUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	paramID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || paramID == 0 {
		return apperr.ErrBadRequest
	}

	if paramID != uint64(authUserID) {
		return apperr.ErrForbidden
	}

	var req UpdatePreferencesRequest
	if err := c.Bind(&req); err != nil {
		return apperr.ErrBadRequest
	}

	if _, ok := supportedLanguages[req.LanguagePreference]; !ok {
		return apperr.ErrInvalidLanguage
	}

	if err := h.userRepo.UpdateLanguagePreference(authUserID, req.LanguagePreference); err != nil {
		return fmt.Errorf("updating language preference: %w", err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]string{
			"languagePreference": req.LanguagePreference,
		},
	})
}

type UpdateUsernameRequest struct {
	Username string `json:"username"`
}

// UpdateUsername changes the authenticated user's username. Authorisation
// mirrors GetProfile (self-only: :id must equal the authenticated user).
// Checks run in a deliberate order so a redundant or premature request never
// touches the cooldown clock or leaks a uniqueness oracle beyond what register
// already exposes: validate → load → unchanged → cooldown → taken → write.
// The repo write independently maps a lost uniqueness race to ErrUsernameTaken.
func (h *UserHandler) UpdateUsername(c echo.Context) error {
	authUserID, err := getUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	paramID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || paramID == 0 {
		return apperr.ErrBadRequest
	}
	if paramID != uint64(authUserID) {
		return apperr.ErrForbidden
	}

	var req UpdateUsernameRequest
	if err := c.Bind(&req); err != nil {
		return apperr.ErrBadRequest
	}

	username, err := ValidateUsername(req.Username)
	if err != nil {
		return err
	}

	u, err := h.userRepo.FindByID(authUserID)
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}
	if u == nil {
		return apperr.ErrUserNotFound
	}

	// No-op: changing to the exact current name never consumes the cooldown.
	if username == u.Username {
		return apperr.ErrUsernameUnchanged
	}

	// Cooldown: enforced against the LAST change only. A never-changed user
	// (NULL username_changed_at) may always change.
	if u.UsernameChangedAt != nil && time.Since(*u.UsernameChangedAt) < UsernameChangeCooldown {
		return apperr.ErrUsernameChangeTooSoon
	}

	// Uniqueness pre-check (case-sensitive, matching registration). A row owned
	// by a different live user means the name is taken.
	existing, err := h.userRepo.FindByUsername(username)
	if err != nil {
		return fmt.Errorf("checking username: %w", err)
	}
	if existing != nil && existing.ID != authUserID {
		return apperr.ErrUsernameTaken
	}

	changedAt, err := h.userRepo.UpdateUsername(authUserID, username)
	if err != nil {
		return fmt.Errorf("updating username: %w", err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"username":          username,
			"usernameChangedAt": changedAt,
		},
	})
}

// ListMatches returns a paginated list of matches in which the authenticated
// user participated. Query params:
//
//	limit   — 1..50 (default 20)
//	offset  — >= 0  (default 0)
//	outcome — win | loss | abandoned | all (default all), viewer-relative
//	sort    — new | old (default new)
//
// Story 11.3 made it public: the :id path param is the SUBJECT — outcome and
// viewerSeat are computed from the subject's perspective, and an unknown
// subject 404s. Responses never leak email, password hash, or language
// preference — only the 4 participant usernames + ids are included.
func (h *UserHandler) ListMatches(c echo.Context) error {
	_, err := getUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	paramID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || paramID == 0 {
		return apperr.ErrBadRequest
	}
	subjectID := uint(paramID)

	subject, err := h.userRepo.FindByID(subjectID)
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}
	if subject == nil {
		return apperr.ErrUserNotFound
	}

	limit, offset, outcome, sort, err := parseMatchesQuery(c)
	if err != nil {
		return err
	}

	matches, total, err := h.matchRepo.GetMatchesForUser(subjectID, limit, offset, outcome, sort)
	if err != nil {
		return fmt.Errorf("fetching matches: %w", err)
	}

	usernames, err := h.loadUsernamesForMatches(matches)
	if err != nil {
		return fmt.Errorf("loading match usernames: %w", err)
	}

	items := make([]MatchListItem, 0, len(matches))
	for _, m := range matches {
		items = append(items, buildMatchListItem(m, subjectID, usernames))
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": MatchesListResponse{
			Items:  items,
			Total:  total,
			Limit:  limit,
			Offset: offset,
		},
	})
}

// parseMatchesQuery reads the limit/offset/outcome/sort query params and applies
// the documented bounds + allowlists. Returns apperr.ErrBadRequest on any
// violation. outcome normalises "" / "all" to "" (no filter); sort normalises
// "" to "new".
func parseMatchesQuery(c echo.Context) (limit, offset int, outcome, sort string, err error) {
	const defaultLimit = 20
	const maxLimit = 50

	limit = defaultLimit
	if raw := c.QueryParam("limit"); raw != "" {
		v, convErr := strconv.Atoi(raw)
		if convErr != nil || v < 1 || v > maxLimit {
			return 0, 0, "", "", apperr.ErrBadRequest
		}
		limit = v
	}

	offset = 0
	if raw := c.QueryParam("offset"); raw != "" {
		v, convErr := strconv.Atoi(raw)
		if convErr != nil || v < 0 {
			return 0, 0, "", "", apperr.ErrBadRequest
		}
		offset = v
	}

	switch raw := c.QueryParam("outcome"); raw {
	case "", "all":
		outcome = ""
	case "win", "loss", "abandoned":
		outcome = raw
	default:
		return 0, 0, "", "", apperr.ErrBadRequest
	}

	switch raw := c.QueryParam("sort"); raw {
	case "", "new":
		sort = "new"
	case "old":
		sort = "old"
	default:
		return 0, 0, "", "", apperr.ErrBadRequest
	}

	return limit, offset, outcome, sort, nil
}

// loadUsernamesForMatches gathers all participant IDs across the page and
// issues a single batched query via userRepo.FindManyByIDs. Returns a map
// keyed by userID so callers can project the 4 seats per match in O(1).
// Bot seats (NULL player IDs) never reach the users lookup — the client
// renders the localized bot name from the isBot flag.
func (h *UserHandler) loadUsernamesForMatches(matches []match.Match) (map[uint]string, error) {
	if len(matches) == 0 {
		return map[uint]string{}, nil
	}
	seen := make(map[uint]struct{}, len(matches)*4)
	ids := make([]uint, 0, len(matches)*4)
	for _, m := range matches {
		for _, idPtr := range [4]*uint{m.Player1ID, m.Player2ID, m.Player3ID, m.Player4ID} {
			if idPtr == nil {
				continue
			}
			if _, ok := seen[*idPtr]; ok {
				continue
			}
			seen[*idPtr] = struct{}{}
			ids = append(ids, *idPtr)
		}
	}
	if len(ids) == 0 {
		return map[uint]string{}, nil
	}
	users, err := h.userRepo.FindManyByIDs(ids)
	if err != nil {
		return nil, err
	}
	result := make(map[uint]string, len(users))
	for _, u := range users {
		result[u.ID] = u.Username
	}
	return result, nil
}

// teamForSeat returns 0 for team A, 1 for team B — seats 0/2 are team A, 1/3 are team B.
// Duplicated locally instead of importing the game package to keep the user
// package free of game-engine coupling.
func teamForSeat(seat int) int { return seat % 2 }

// buildMatchListItem projects a DB Match + preloaded Hands into the viewer-
// specific response DTO (derives viewerSeat and outcome server-side).
func buildMatchListItem(m match.Match, viewerID uint, usernames map[uint]string) MatchListItem {
	seats := [4]*uint{m.Player1ID, m.Player2ID, m.Player3ID, m.Player4ID}
	botFlags := [4]bool{m.Player1IsBot, m.Player2IsBot, m.Player3IsBot, m.Player4IsBot}

	// The viewer is always human — bot seats (NULL IDs) are skipped.
	viewerSeat := 0
	for i, id := range seats {
		if id != nil && *id == viewerID {
			viewerSeat = i
			break
		}
	}

	players := make([]MatchPlayer, 0, 4)
	for i, id := range seats {
		p := MatchPlayer{Seat: i, IsBot: botFlags[i]}
		if id != nil {
			p.UserID = *id
			p.Username = usernames[*id]
		}
		players = append(players, p)
	}

	// Outcome is per-player on abandoned rows: the abandoner keeps "abandoned";
	// partner/opponents get loss/win from winner_team (the non-abandoning team,
	// persisted live and backfilled by migration 000015). winner_team is only
	// meaningful when AbandonedBy is set — NULL-abandoner rows (boot-reconcile)
	// stay "abandoned" for all four seats.
	// Invariant: win/loss is derived from winner_team ONLY for "completed" rows
	// and attributable "abandoned" rows — the repository returns no other
	// status, and any unexpected value must never invent a win (it keeps the
	// safe "loss" default rather than reading a meaningless winner_team).
	outcome := "loss"
	switch {
	case m.Status == "abandoned" && (m.AbandonedBy == nil || *m.AbandonedBy == viewerID):
		outcome = "abandoned"
	case (m.Status == "completed" || m.Status == "abandoned") &&
		m.WinnerTeam == teamForSeat(viewerSeat):
		outcome = "win"
	}

	// Why the match ended — drives the client's "ended early" marker with
	// distinct abandonment/surrender wording.
	endReason := "natural"
	if m.Status == "abandoned" {
		endReason = "abandonment"
	} else if m.SurrenderedBy != nil {
		endReason = "surrender"
	}

	hands := make([]MatchHandView, 0, len(m.Hands))
	for _, h := range m.Hands {
		var capotTeam *int
		if h.CapotTeam != nil {
			v := *h.CapotTeam
			capotTeam = &v
		}
		hands = append(hands, MatchHandView{
			HandNumber:      h.HandNumber,
			TeamACardPoints: h.TeamACardPoints,
			TeamBCardPoints: h.TeamBCardPoints,
			TeamADeclPoints: h.TeamADeclPoints,
			TeamBDeclPoints: h.TeamBDeclPoints,
			LastTrickTeam:   h.LastTrickTeam,
			LastTrickBonus:  h.LastTrickBonus,
			Capot:           h.Capot,
			CapotTeam:       capotTeam,
			CapotBonus:      h.CapotBonus,
			FailedContract:  h.FailedContract,
			ContractingTeam: h.ContractingTeam,
			TeamAHandTotal:  h.TeamAHandTotal,
			TeamBHandTotal:  h.TeamBHandTotal,
		})
	}

	return MatchListItem{
		ID:          m.ID,
		Variant:     m.Variant,
		MatchMode:   m.MatchMode,
		StartedAt:   m.StartedAt,
		CompletedAt: m.CompletedAt,
		Status:      m.Status,
		WinnerTeam:  m.WinnerTeam,
		TeamAScore:  m.TeamAScore,
		TeamBScore:  m.TeamBScore,
		HasBots:     m.HasBots,
		AbandonedBy: m.AbandonedBy,
		ViewerSeat:  viewerSeat,
		Outcome:     outcome,
		EndReason:   endReason,
		Players:     players,
		Hands:       hands,
	}
}
