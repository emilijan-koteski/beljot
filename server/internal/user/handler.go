package user

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/season"
)

// SeasonRankReader returns a user's standing in the active season, or (nil, nil) when they have not played in it.
// It is declared here so that season never has to import user.
type SeasonRankReader interface {
	CurrentSeasonRank(userID uint, now time.Time) (*season.SeasonRankView, error)
}

// ProfileResponse is the self profile returned by GetProfile when the path id is the viewer.
// Its wallet, streak, preference and username-change fields are private and must never reach a public shape.
type ProfileResponse struct {
	ID                    uint                   `json:"id"`
	Username              string                 `json:"username"`
	AvatarURL             *string                `json:"avatarUrl"`
	AvatarLargeURL        *string                `json:"avatarLargeUrl"`
	UsernameChangedAt     *time.Time             `json:"usernameChangedAt,omitempty"`
	LanguagePreference    string                 `json:"languagePreference"`
	CardDeckPreference    string                 `json:"cardDeckPreference"`
	SoundEnabled          bool                   `json:"soundEnabled"`
	MusicEnabled          bool                   `json:"musicEnabled"`
	SoundVolume           int                    `json:"soundVolume"`
	MusicVolume           int                    `json:"musicVolume"`
	AllowNonFriendInvites bool                   `json:"allowNonFriendInvites"`
	WalletBalance         int                    `json:"walletBalance"`
	LoginStreakDays       int                    `json:"loginStreakDays"`
	TotalXP               int                    `json:"totalXp"`
	Level                 int                    `json:"level"`
	XPIntoLevel           int                    `json:"xpIntoLevel"`
	XPForNextLevel        int                    `json:"xpForNextLevel"`
	HonorScore            int                    `json:"honorScore"`
	HonorTier             string                 `json:"honorTier"`
	HonorCompletedTotal   int64                  `json:"honorCompletedTotal"`
	HonorAbandonedTotal   int64                  `json:"honorAbandonedTotal"`
	IsNewPlayer           bool                   `json:"isNewPlayer"`
	HonorTrendDelta       int                    `json:"honorTrendDelta"`
	HonorTrendDirection   string                 `json:"honorTrendDirection"`
	SeasonRank            *season.SeasonRankView `json:"seasonRank"`
	CreatedAt             time.Time              `json:"createdAt"`
	TotalGamesPlayed      int                    `json:"totalGamesPlayed"`
	Wins                  int                    `json:"wins"`
	Losses                int                    `json:"losses"`
	Abandoned             int                    `json:"abandoned"`
}

// PublicProfileResponse is the public profile returned when the path id is not the viewer.
// It carries only public-safe fields, all computed for the subject rather than the viewer.
type PublicProfileResponse struct {
	ID                  uint                   `json:"id"`
	Username            string                 `json:"username"`
	AvatarURL           *string                `json:"avatarUrl"`
	AvatarLargeURL      *string                `json:"avatarLargeUrl"`
	CreatedAt           time.Time              `json:"createdAt"`
	TotalXP             int                    `json:"totalXp"`
	Level               int                    `json:"level"`
	XPIntoLevel         int                    `json:"xpIntoLevel"`
	XPForNextLevel      int                    `json:"xpForNextLevel"`
	HonorScore          int                    `json:"honorScore"`
	HonorTier           string                 `json:"honorTier"`
	HonorCompletedTotal int64                  `json:"honorCompletedTotal"`
	HonorAbandonedTotal int64                  `json:"honorAbandonedTotal"`
	IsNewPlayer         bool                   `json:"isNewPlayer"`
	HonorTrendDelta     int                    `json:"honorTrendDelta"`
	HonorTrendDirection string                 `json:"honorTrendDirection"`
	SeasonRank          *season.SeasonRankView `json:"seasonRank"`
	TotalGamesPlayed    int                    `json:"totalGamesPlayed"`
	Wins                int                    `json:"wins"`
	Losses              int                    `json:"losses"`
	Abandoned           int                    `json:"abandoned"`
}

// UpdatePreferencesRequest is a partial update whose pointer fields tell "absent" apart from empty, false or 0.
type UpdatePreferencesRequest struct {
	LanguagePreference    *string `json:"languagePreference"`
	CardDeckPreference    *string `json:"cardDeckPreference"`
	SoundEnabled          *bool   `json:"soundEnabled"`
	MusicEnabled          *bool   `json:"musicEnabled"`
	SoundVolume           *int    `json:"soundVolume"`
	MusicVolume           *int    `json:"musicVolume"`
	AllowNonFriendInvites *bool   `json:"allowNonFriendInvites"`
}

var supportedLanguages = map[string]struct{}{
	"en": {},
	"sr": {},
	"mk": {},
	"hr": {},
}

var supportedCardDecks = map[string]struct{}{
	CardDeckFrench:   {},
	CardDeckCroatian: {},
}

const (
	CardDeckFrench   = "french"
	CardDeckCroatian = "croatian"
)

// IsSupportedLanguage reports whether code is one of the registered UI languages.
func IsSupportedLanguage(code string) bool {
	_, ok := supportedLanguages[code]
	return ok
}

// IsSupportedCardDeck reports whether deck is one of the shipped card-face decks.
func IsSupportedCardDeck(deck string) bool {
	_, ok := supportedCardDecks[deck]
	return ok
}

const (
	MinAudioVolume = 0
	MaxAudioVolume = 100
)

// isValidAudioVolume reports whether v is an allowed audio level.
func isValidAudioVolume(v int) bool {
	return v >= MinAudioVolume && v <= MaxAudioVolume
}

// MatchPlayer is the per-seat participant in a match list item; bot seats have userId 0 and isBot true.
type MatchPlayer struct {
	Seat      int     `json:"seat"`
	UserID    uint    `json:"userId"`
	Username  string  `json:"username"`
	AvatarURL *string `json:"avatarUrl"`
	IsBot     bool    `json:"isBot"`
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
	ID                   uint            `json:"id"`
	Variant              string          `json:"variant"`
	MatchMode            string          `json:"matchMode"`
	StartedAt            time.Time       `json:"startedAt"`
	CompletedAt          time.Time       `json:"completedAt"`
	Status               string          `json:"status"`
	WinnerTeam           int             `json:"winnerTeam"`
	TeamAScore           int             `json:"teamAScore"`
	TeamBScore           int             `json:"teamBScore"`
	HasBots              bool            `json:"hasBots"`
	AbandonedBy          *uint           `json:"abandonedBy,omitempty"`
	ViewerSeat           int             `json:"viewerSeat"`
	Outcome              string          `json:"outcome"`
	EndReason            string          `json:"endReason"`
	DeclarationsEnabled  bool            `json:"declarationsEnabled"`
	StopAtTarget         bool            `json:"stopAtTarget"`
	DeclarationsTransfer bool            `json:"declarationsTransfer"`
	Clockwise            bool            `json:"clockwise"`
	Players              []MatchPlayer   `json:"players"`
	Hands                []MatchHandView `json:"hands"`
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
	UserID    uint    `json:"userId"`
	Username  string  `json:"username"`
	AvatarURL *string `json:"avatarUrl"`
	Played    int     `json:"played"`
	Wins      int     `json:"wins"`
}

// RivalStat is one most-faced-opponent row in the career response.
type RivalStat struct {
	UserID    uint    `json:"userId"`
	Username  string  `json:"username"`
	AvatarURL *string `json:"avatarUrl"`
	Wins      int     `json:"wins"`
	Losses    int     `json:"losses"`
}

// CareerResponse is the envelope returned by GET /users/:id/career.
type CareerResponse struct {
	Capots          int           `json:"capots"`
	AvgMatchSeconds int           `json:"avgMatchSeconds"`
	CareerPoints    int64         `json:"careerPoints"`
	Streak          CareerStreak  `json:"streak"`
	BestHand        *BestHand     `json:"bestHand,omitempty"`
	LastPlayedAt    *time.Time    `json:"lastPlayedAt,omitempty"`
	TopPartners     []PartnerStat `json:"topPartners"`
	TopRivals       []RivalStat   `json:"topRivals"`
}

const careerListLimit = 4

type UserHandler struct {
	userRepo   UserRepository
	matchRepo  match.MatchRepository
	seasonRank SeasonRankReader
}

func NewUserHandler(userRepo UserRepository, matchRepo match.MatchRepository, seasonRank SeasonRankReader) *UserHandler {
	return &UserHandler{userRepo: userRepo, matchRepo: matchRepo, seasonRank: seasonRank}
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

	now := time.Now().UTC()
	honor := NewHonorSnapshot(
		u.HonorCompletedWeight, u.HonorAbandonedWeight, u.HonorDecayedAt,
		u.HonorCompletedTotal, u.HonorAbandonedTotal, now,
	)

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

	var seasonRank *season.SeasonRankView
	if h.seasonRank != nil {
		rank, rankErr := h.seasonRank.CurrentSeasonRank(subjectID, now)
		if rankErr != nil {
			slog.Error("profile: failed to read season rank", "userID", subjectID, "error", rankErr)
		} else {
			seasonRank = rank
		}
	}

	if paramID == uint64(authUserID) {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"data": ProfileResponse{
				ID:                    u.ID,
				Username:              u.Username,
				AvatarURL:             SmallAvatarURL(u.AvatarKey),
				AvatarLargeURL:        AvatarURL(u.AvatarKey, AvatarSizeLarge),
				UsernameChangedAt:     u.UsernameChangedAt,
				LanguagePreference:    u.LanguagePreference,
				CardDeckPreference:    u.CardDeckPreference,
				SoundEnabled:          u.SoundEnabled,
				MusicEnabled:          u.MusicEnabled,
				SoundVolume:           u.SoundVolume,
				MusicVolume:           u.MusicVolume,
				AllowNonFriendInvites: u.AllowNonFriendInvites,
				WalletBalance:         u.WalletBalance,
				LoginStreakDays:       u.LoginStreakDays,
				TotalXP:               u.TotalXP,
				Level:                 level,
				XPIntoLevel:           xpIntoLevel,
				XPForNextLevel:        xpForNextLevel,
				HonorScore:            honor.Score,
				HonorTier:             honor.Tier,
				HonorCompletedTotal:   honor.CompletedTotal,
				HonorAbandonedTotal:   honor.AbandonedTotal,
				IsNewPlayer:           honor.IsNewPlayer,
				HonorTrendDelta:       trendDelta,
				HonorTrendDirection:   trendDirection,
				SeasonRank:            seasonRank,
				CreatedAt:             u.CreatedAt,
				TotalGamesPlayed:      wins + losses + abandoned,
				Wins:                  wins,
				Losses:                losses,
				Abandoned:             abandoned,
			},
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": PublicProfileResponse{
			ID:                  u.ID,
			Username:            u.Username,
			AvatarURL:           SmallAvatarURL(u.AvatarKey),
			AvatarLargeURL:      AvatarURL(u.AvatarKey, AvatarSizeLarge),
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
			SeasonRank:          seasonRank,
			TotalGamesPlayed:    wins + losses + abandoned,
			Wins:                wins,
			Losses:              losses,
			Abandoned:           abandoned,
		},
	})
}

// GetCareer returns the derived career stats for the subject in the path; any authenticated viewer may read them.
func (h *UserHandler) GetCareer(c echo.Context) error {
	if _, err := getUserID(c); err != nil {
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

	identities, err := h.loadIdentitiesForAggregates(partnerAggs, rivalAggs)
	if err != nil {
		return fmt.Errorf("loading career usernames: %w", err)
	}

	partners := make([]PartnerStat, 0, len(partnerAggs))
	for _, p := range partnerAggs {
		who := identities[p.UserID]
		partners = append(partners, PartnerStat{
			UserID:    p.UserID,
			Username:  who.username,
			AvatarURL: SmallAvatarURL(who.avatarKey),
			Played:    p.Played,
			Wins:      p.Wins,
		})
	}

	rivals := make([]RivalStat, 0, len(rivalAggs))
	for _, r := range rivalAggs {
		who := identities[r.UserID]
		rivals = append(rivals, RivalStat{
			UserID:    r.UserID,
			Username:  who.username,
			AvatarURL: SmallAvatarURL(who.avatarKey),
			Wins:      r.Wins,
			Losses:    r.Losses,
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

// participantIdentity is what every participant DTO shows of a player: the
// name, and the stored avatar prefix the public URL is derived from.
type participantIdentity struct {
	username  string
	avatarKey *string
}

// identitiesByID indexes loaded users by id.
func identitiesByID(users []User) map[uint]participantIdentity {
	result := make(map[uint]participantIdentity, len(users))
	for _, u := range users {
		result[u.ID] = participantIdentity{username: u.Username, avatarKey: u.AvatarKey}
	}
	return result
}

// loadIdentitiesForAggregates loads all partner and rival identities in one query, keyed by user ID.
func (h *UserHandler) loadIdentitiesForAggregates(partners []match.PartnerAggregate, rivals []match.RivalAggregate) (map[uint]participantIdentity, error) {
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
		return map[uint]participantIdentity{}, nil
	}
	users, err := h.userRepo.FindManyByIDs(ids)
	if err != nil {
		return nil, err
	}
	return identitiesByID(users), nil
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
	prefs := PreferencesUpdate(req)

	if prefs.IsEmpty() {
		return apperr.ErrBadRequest
	}

	if req.LanguagePreference != nil && !IsSupportedLanguage(*req.LanguagePreference) {
		return apperr.ErrInvalidLanguage
	}
	if req.CardDeckPreference != nil && !IsSupportedCardDeck(*req.CardDeckPreference) {
		return apperr.ErrInvalidCardDeck
	}
	if req.SoundVolume != nil && !isValidAudioVolume(*req.SoundVolume) {
		return apperr.ErrBadRequest
	}
	if req.MusicVolume != nil && !isValidAudioVolume(*req.MusicVolume) {
		return apperr.ErrBadRequest
	}

	if err := h.userRepo.UpdatePreferences(authUserID, prefs); err != nil {
		return fmt.Errorf("updating preferences: %w", err)
	}

	data := map[string]any{}
	if req.LanguagePreference != nil {
		data["languagePreference"] = *req.LanguagePreference
	}
	if req.CardDeckPreference != nil {
		data["cardDeckPreference"] = *req.CardDeckPreference
	}
	if req.SoundEnabled != nil {
		data["soundEnabled"] = *req.SoundEnabled
	}
	if req.MusicEnabled != nil {
		data["musicEnabled"] = *req.MusicEnabled
	}
	if req.SoundVolume != nil {
		data["soundVolume"] = *req.SoundVolume
	}
	if req.MusicVolume != nil {
		data["musicVolume"] = *req.MusicVolume
	}
	if req.AllowNonFriendInvites != nil {
		data["allowNonFriendInvites"] = *req.AllowNonFriendInvites
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"data": data})
}

type UpdateUsernameRequest struct {
	Username string `json:"username"`
}

// UpdateUsername changes the authenticated user's own username.
// Checks run validate, load, unchanged, cooldown, taken, then write, so a redundant request never touches the cooldown.
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

	if username == u.Username {
		return apperr.ErrUsernameUnchanged
	}

	if u.UsernameChangedAt != nil && time.Since(*u.UsernameChangedAt) < UsernameChangeCooldown {
		return apperr.ErrUsernameChangeTooSoon
	}

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

// PlayerSearchResult is the minimal public user DTO returned by SearchUsers: id, username and avatar only.
type PlayerSearchResult struct {
	ID        uint    `json:"id"`
	Username  string  `json:"username"`
	AvatarURL *string `json:"avatarUrl"`
}

const searchResultLimit = 10

// SearchUsers returns live players other than the caller whose username contains the `search` query, case-insensitively.
// The items slice is never nil, so no match serializes as [].
func (h *UserHandler) SearchUsers(c echo.Context) error {
	callerID, err := getUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	query := strings.TrimSpace(c.QueryParam("search"))
	if query == "" {
		return apperr.ErrBadRequest
	}

	users, err := h.userRepo.SearchByUsername(query, callerID, searchResultLimit)
	if err != nil {
		return fmt.Errorf("searching users: %w", err)
	}

	items := make([]PlayerSearchResult, 0, len(users))
	for _, u := range users {
		items = append(items, PlayerSearchResult{ID: u.ID, Username: u.Username, AvatarURL: SmallAvatarURL(u.AvatarKey)})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"data": items})
}

// ListMatches returns a paginated list of the subject's matches, with outcome and viewerSeat from the subject's side.
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

	identities, err := h.loadIdentitiesForMatches(matches)
	if err != nil {
		return fmt.Errorf("loading match usernames: %w", err)
	}

	items := make([]MatchListItem, 0, len(matches))
	for _, m := range matches {
		items = append(items, buildMatchListItem(m, subjectID, identities))
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

// parseMatchesQuery reads and validates the limit, offset, outcome and sort query params.
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

// loadIdentitiesForMatches loads every human participant on the page in one query, keyed by user ID.
func (h *UserHandler) loadIdentitiesForMatches(matches []match.Match) (map[uint]participantIdentity, error) {
	if len(matches) == 0 {
		return map[uint]participantIdentity{}, nil
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
		return map[uint]participantIdentity{}, nil
	}
	users, err := h.userRepo.FindManyByIDs(ids)
	if err != nil {
		return nil, err
	}
	return identitiesByID(users), nil
}

// teamForSeat returns 0 for seats 0 and 2 and 1 for seats 1 and 3.
func teamForSeat(seat int) int { return seat % 2 }

// buildMatchListItem projects a match and its hands into the viewer-specific list item.
func buildMatchListItem(m match.Match, viewerID uint, identities map[uint]participantIdentity) MatchListItem {
	seats := [4]*uint{m.Player1ID, m.Player2ID, m.Player3ID, m.Player4ID}
	botFlags := [4]bool{m.Player1IsBot, m.Player2IsBot, m.Player3IsBot, m.Player4IsBot}

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
			who := identities[*id]
			p.UserID = *id
			p.Username = who.username
			p.AvatarURL = SmallAvatarURL(who.avatarKey)
		}
		players = append(players, p)
	}

	outcome := "loss"
	switch {
	case m.Status == "abandoned" && (m.AbandonedBy == nil || *m.AbandonedBy == viewerID):
		outcome = "abandoned"
	case (m.Status == "completed" || m.Status == "abandoned") &&
		m.WinnerTeam == teamForSeat(viewerSeat):
		outcome = "win"
	}

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

		DeclarationsEnabled:  m.DeclarationsEnabled,
		StopAtTarget:         m.StopAtTarget,
		DeclarationsTransfer: m.DeclarationsTransfer,
		Clockwise:            m.Clockwise,

		Players: players,
		Hands:   hands,
	}
}
