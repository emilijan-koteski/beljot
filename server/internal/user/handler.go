package user

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/match"
)

// ProfileResponse is the self-only profile DTO (GetProfile authorises
// paramID == authUserID). WalletBalance and LoginStreakDays are PRIVATE figures
// — when Epic 11 adds public player profiles, the public DTO must NOT include
// them. Never add these fields to a shared/public response shape.
type ProfileResponse struct {
	ID                 uint   `json:"id"`
	Username           string `json:"username"`
	LanguagePreference string `json:"languagePreference"`
	WalletBalance      int    `json:"walletBalance"`
	LoginStreakDays    int    `json:"loginStreakDays"`
	// XP & level (Story 9.5). TotalXP is the lifetime total; Level is derived
	// from it (never stored); XPIntoLevel/XPForNextLevel drive the profile XP
	// bar (fill = XPIntoLevel / XPForNextLevel). These are PRIVATE self-only
	// figures like WalletBalance — keep them off any future public profile DTO.
	TotalXP          int       `json:"totalXp"`
	Level            int       `json:"level"`
	XPIntoLevel      int       `json:"xpIntoLevel"`
	XPForNextLevel   int       `json:"xpForNextLevel"`
	CreatedAt        time.Time `json:"createdAt"`
	TotalGamesPlayed int       `json:"totalGamesPlayed"`
	Wins             int       `json:"wins"`
	Losses           int       `json:"losses"`
	Abandoned        int       `json:"abandoned"`
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
	ID          uint            `json:"id"`
	Variant     string          `json:"variant"`
	MatchMode   string          `json:"matchMode"`
	StartedAt   time.Time       `json:"startedAt"`
	CompletedAt time.Time       `json:"completedAt"`
	Status      string          `json:"status"`
	WinnerTeam  int             `json:"winnerTeam"`
	TeamAScore  int             `json:"teamAScore"`
	TeamBScore  int             `json:"teamBScore"`
	HasBots     bool            `json:"hasBots"`
	AbandonedBy *uint           `json:"abandonedBy,omitempty"`
	ViewerSeat  int             `json:"viewerSeat"`
	Outcome     string          `json:"outcome"`
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
	Capots          int           `json:"capots"`
	AvgMatchSeconds int           `json:"avgMatchSeconds"`
	Streak          CareerStreak  `json:"streak"`
	BestHand        *BestHand     `json:"bestHand,omitempty"`
	LastPlayedAt    *time.Time    `json:"lastPlayedAt,omitempty"`
	TopPartners     []PartnerStat `json:"topPartners"`
	TopRivals       []RivalStat   `json:"topRivals"`
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

	if paramID != uint64(authUserID) {
		return apperr.ErrForbidden
	}

	u, err := h.userRepo.FindByID(authUserID)
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}
	if u == nil {
		return apperr.ErrUserNotFound
	}

	wins, losses, abandoned, err := h.matchRepo.GetStatsForUser(authUserID)
	if err != nil {
		return fmt.Errorf("fetching profile stats: %w", err)
	}

	level, xpIntoLevel, xpForNextLevel := LevelProgress(u.TotalXP)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": ProfileResponse{
			ID:                 u.ID,
			Username:           u.Username,
			LanguagePreference: u.LanguagePreference,
			WalletBalance:      u.WalletBalance,
			LoginStreakDays:    u.LoginStreakDays,
			TotalXP:            u.TotalXP,
			Level:              level,
			XPIntoLevel:        xpIntoLevel,
			XPForNextLevel:     xpForNextLevel,
			CreatedAt:          u.CreatedAt,
			TotalGamesPlayed:   wins + losses + abandoned,
			Wins:               wins,
			Losses:             losses,
			Abandoned:          abandoned,
		},
	})
}

// GetCareer returns the derived career stats for the authenticated user:
// capots won, average match length, current streak, best hand, and the
// most-played partners / most-faced rivals. Authorisation mirrors GetProfile:
// the :id path param must equal the authenticated user's ID. Like the other
// user endpoints, only participant usernames + ids are exposed — never email,
// password hash, or language preference.
func (h *UserHandler) GetCareer(c echo.Context) error {
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

	agg, err := h.matchRepo.GetCareerAggregatesForUser(authUserID)
	if err != nil {
		return fmt.Errorf("fetching career aggregates: %w", err)
	}

	partnerAggs, err := h.matchRepo.GetTopPartnersForUser(authUserID, careerListLimit)
	if err != nil {
		return fmt.Errorf("fetching career partners: %w", err)
	}

	rivalAggs, err := h.matchRepo.GetTopRivalsForUser(authUserID, careerListLimit)
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

// ListMatches returns a paginated list of matches in which the authenticated
// user participated. Query params:
//
//	limit   — 1..50 (default 20)
//	offset  — >= 0  (default 0)
//	outcome — win | loss | abandoned | all (default all), viewer-relative
//	sort    — new | old (default new)
//
// Authorisation mirrors GetProfile: the :id path param must equal the
// authenticated user's ID. Responses never leak email, password hash, or
// language preference — only the 4 participant usernames + ids are included.
func (h *UserHandler) ListMatches(c echo.Context) error {
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

	limit, offset, outcome, sort, err := parseMatchesQuery(c)
	if err != nil {
		return err
	}

	matches, total, err := h.matchRepo.GetMatchesForUser(authUserID, limit, offset, outcome, sort)
	if err != nil {
		return fmt.Errorf("fetching matches: %w", err)
	}

	usernames, err := h.loadUsernamesForMatches(matches)
	if err != nil {
		return fmt.Errorf("loading match usernames: %w", err)
	}

	items := make([]MatchListItem, 0, len(matches))
	for _, m := range matches {
		items = append(items, buildMatchListItem(m, authUserID, usernames))
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
