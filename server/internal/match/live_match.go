package match

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/bot"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/ws"
)

// LiveMatch holds the live game state and player mapping for one active game.
type LiveMatch struct {
	gameState                *game.GameState
	playerIDs                [4]uint
	roomID                   uint
	startedAt                time.Time
	coinBuyIn                int
	timerStyle               string
	timerDurationSec         int
	turnTimer                *time.Timer
	timerGeneration          uint64
	handCompleteExpiresAt    time.Time
	declarationExpiresAt     time.Time
	reconnectWindowSec       int
	seatReconnectTimers      [4]*time.Timer
	seatReconnectGenerations [4]uint64
	closed                   bool
	handResults              []HandResult
	botActionTimers          [4]*time.Timer
	botActionGenerations     [4]uint64
	botMemory                *bot.Memory
	botSearchResults         [4]*botSearchEntry
	dealGraceUntil           time.Time
	mu                       sync.RWMutex
}

// RoomStatusUpdater updates a room's status in the database.
type RoomStatusUpdater interface {
	UpdateRoomStatus(roomID uint, status string) error
}

// WalletSettler credits winning human seats and reads balances at match end.
// Nil skips settlement.
type WalletSettler interface {
	ApplySettlement(credits map[uint]int) error
	GetBalances(userIDs []uint) (map[uint]int, error)
}

// XPAwarder awards lifetime XP at match end and derives levels from totals.
// It is an interface because user imports match, so match must never import user.
type XPAwarder interface {
	ApplyXPAwards(awards map[uint]int) (map[uint]int, error)
	LevelForXP(totalXP int) int
	LevelsForUsers(ids []uint) (map[uint]int, error)
}

// HonorEvent is one player's honor contribution from a finished match.
// Abandoned is true only for the seat whose reconnect window expired.
type HonorEvent struct {
	Abandoned bool
}

// HonorSnapshot is one player's precomputed honor state after the match-end write.
type HonorSnapshot struct {
	Score          int
	Tier           string
	CompletedTotal int64
	AbandonedTotal int64
	IsNewPlayer    bool
}

// HonorRecorder records honor at match end without match importing the user package.
// Nil skips honor entirely.
type HonorRecorder interface {
	ApplyHonorEvents(events map[uint]HonorEvent, now time.Time) (map[uint]HonorSnapshot, error)
}

// PlayerSeasonSnapshot is one player's precomputed season standing after the match-end write.
type PlayerSeasonSnapshot struct {
	SeasonName   string
	SP           int
	SPChange     int
	RankTier     string
	RankDivision *int
	RankChange   string
	Reason       string
}

// OutcomeSeat is one seat of a finished match as the Season Points formula reads it.
type OutcomeSeat struct {
	UserID    uint
	IsBot     bool
	Team      int
	Completed bool
}

// MatchOutcome is everything the season service needs to score one finished match.
type MatchOutcome struct {
	Seats         [4]OutcomeSeat
	WinnerTeam    int
	TeamScores    [2]int
	Target        int
	Surrender     bool
	InstantWin    bool
	CapotTeams    [2]bool
	AbandonedSeat int
}

// SPAwarder applies Season Points for a whole match outcome without match importing season.
// now decides which season the match lands in; nil skips SP entirely.
type SPAwarder interface {
	ApplySeasonPoints(outcome MatchOutcome, now time.Time) (map[uint]PlayerSeasonSnapshot, error)
}

// Broadcaster is the subset of *ws.Hub the manager depends on.
// SendFrames delivers per-seat match_state frames as one logical send.
type Broadcaster interface {
	BroadcastToUsers(userIDs []uint, msg []byte)
	SendToUser(userID uint, msg []byte)
	SendFrames(frames []ws.UserFrame)
}

// Manager orchestrates game sessions: receives actions via WebSocket,
// calls the rules engine, broadcasts results, and persists completed matches.
type Manager struct {
	sessions         map[uint]*LiveMatch
	userToRoom       map[uint]uint
	hub              Broadcaster
	matchRepo        MatchRepository
	roomUpdater      RoomStatusUpdater
	walletSettler    WalletSettler
	xpAwarder        XPAwarder
	honorRecorder    HonorRecorder
	spAwarder        SPAwarder
	userRemovedHooks []func(userID uint)
	botDelayMin      time.Duration
	botDelayMax      time.Duration
	botSearch        BotSearchSettings
	searchPool       *searchPool
	dealGraceEnabled bool
	mu               sync.RWMutex
}

// NewManager creates a session manager wired to the WebSocket hub and match repository.
func NewManager(hub Broadcaster, matchRepo MatchRepository) *Manager {
	return &Manager{
		sessions:         make(map[uint]*LiveMatch),
		userToRoom:       make(map[uint]uint),
		hub:              hub,
		matchRepo:        matchRepo,
		botDelayMin:      time.Second,
		botDelayMax:      2500 * time.Millisecond,
		dealGraceEnabled: true,
	}
}

// humanUserIDs filters bot placeholders (UserID 0) out of a seat-indexed
// playerIDs array for hub broadcasts. The hub tolerates unknown IDs, but
// bot seats must never ride a recipient list by contract.
func humanUserIDs(playerIDs [4]uint) []uint {
	out := make([]uint, 0, 4)
	for _, id := range playerIDs {
		if id != 0 {
			out = append(out, id)
		}
	}
	return out
}

// SetBotSearch enables the bots' card search with the given settings and replaces any earlier pool, so call it before
// serving. Unset workers and deadline take their defaults, and a FromTrick of zero or less turns the search off.
func (m *Manager) SetBotSearch(s BotSearchSettings) {
	defaults := DefaultBotSearchSettings()
	if s.Workers <= 0 {
		s.Workers = defaults.Workers
	}
	if s.Deadline <= 0 {
		s.Deadline = defaults.Deadline
	}
	m.botSearch = s
	m.searchPool = nil
	if s.FromTrick > 0 {
		m.searchPool = newSearchPool(s.Workers)
	}
}

// SetRoomUpdater sets the interface for updating room status on match completion.
func (m *Manager) SetRoomUpdater(updater RoomStatusUpdater) {
	m.roomUpdater = updater
}

// SetWalletSettler injects the wallet service used to settle coins at match end
// Optional — when unset, settlement is skipped.
func (m *Manager) SetWalletSettler(settler WalletSettler) {
	m.walletSettler = settler
}

// SetXPAwarder injects the user-side XP service used to award lifetime XP at
// match end. Optional — when unset, XP awards are skipped.
func (m *Manager) SetXPAwarder(awarder XPAwarder) {
	m.xpAwarder = awarder
}

// SetHonorRecorder injects the user-side honor service used to record match
// completions and abandonments at match end. Optional — when unset,
// honor is skipped entirely (no mutation, no event).
func (m *Manager) SetHonorRecorder(recorder HonorRecorder) {
	m.honorRecorder = recorder
}

// SetSPAwarder injects the season service used to apply Season Points and
// refresh the rank tier at match end. Optional — when unset, SP is
// skipped entirely (no mutation, no event).
func (m *Manager) SetSPAwarder(awarder SPAwarder) {
	m.spAwarder = awarder
}

// AddUserRemovedHook registers fn to be called (outside the manager lock)
// for each playerID when RemoveSession tears down a session.
func (m *Manager) AddUserRemovedHook(fn func(userID uint)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.userRemovedHooks = append(m.userRemovedHooks, fn)
}

// StartMatch creates a new game session from room data and broadcasts the initial state.
// coinBuyIn is the per-human stake for settlement, 0 for no-economy matches.
func (m *Manager) StartMatch(roomID uint, variant string, matchMode string, players [4]PlayerSeatInfo, timerStyle string, timerDurationSec int, ownerID uint, reconnectWindowSec int, coinBuyIn int, declarationsEnabled bool, stopAtTarget bool, declarationsTransfer bool, clockwise bool) error {
	m.mu.Lock()
	if _, exists := m.sessions[roomID]; exists {
		m.mu.Unlock()
		return fmt.Errorf("session already exists for room %d", roomID)
	}

	var playerIDs [4]uint
	var usernames [4]string
	var botSeats [4]bool
	for _, p := range players {
		playerIDs[p.Seat] = p.UserID
		usernames[p.Seat] = p.Username
		botSeats[p.Seat] = p.IsBot
	}

	gs := game.NewGame(playerIDs, usernames, botSeats, game.Variant(variant), matchMode, roomID, declarationsEnabled, stopAtTarget, declarationsTransfer, clockwise)

	for _, p := range players {
		if !p.IsBot {
			gs.Players[p.Seat].AvatarURL = p.AvatarURL
		}
	}

	if m.xpAwarder != nil {
		humanLevelIDs := humanUserIDs(playerIDs)
		if len(humanLevelIDs) > 0 {
			levels, err := m.xpAwarder.LevelsForUsers(humanLevelIDs)
			if err != nil {
				slog.Error("session: failed to load player levels", "roomID", roomID, "error", err)
			} else {
				for seat, uid := range playerIDs {
					if uid == 0 {
						continue
					}
					gs.Players[seat].Level = levels[uid]
				}
			}
		}
	}

	gs.OwnerSeat = -1
	for i, uid := range playerIDs {
		if uid == ownerID {
			gs.OwnerSeat = i
			break
		}
	}

	session := &LiveMatch{
		gameState:          gs,
		playerIDs:          playerIDs,
		roomID:             roomID,
		startedAt:          time.Now(),
		coinBuyIn:          coinBuyIn,
		timerStyle:         timerStyle,
		timerDurationSec:   timerDurationSec,
		reconnectWindowSec: reconnectWindowSec,
	}
	if botSeats[0] || botSeats[1] || botSeats[2] || botSeats[3] {
		session.botMemory = bot.NewMemory()
	}

	m.sessions[roomID] = session
	for _, uid := range playerIDs {
		if uid == 0 {
			continue
		}
		m.userToRoom[uid] = roomID
	}
	m.mu.Unlock()

	slog.Info("session: game started", "roomID", roomID, "players", playerIDs)

	m.broadcastState(playerIDs, gs)

	if gs.Phase == game.PhaseDealing {
		session.mu.Lock()
		gs.Phase = game.PhaseBidding
		game.RefreshDerivedFlags(gs)
		m.setDealGraceLocked(session, matchStartSplash+firstDealGrace(gs.Rules))
		m.setTurnExpiry(session, gs)
		m.startTimerLocked(session, gs.ActivePlayerSeat)
		session.mu.Unlock()
		m.broadcastState(playerIDs, gs)
	}

	m.maybeScheduleBotAction(session)

	return nil
}

// HandleAction processes a player's game action received via WebSocket.
// This is called by the hub's action handler (dispatched in a goroutine).
func (m *Manager) HandleAction(client *ws.Client, msg ws.WSMessage) {
	m.mu.RLock()
	roomID, ok := m.userToRoom[client.UserID]
	if !ok {
		m.mu.RUnlock()
		m.sendError(client.UserID, ws.ErrorInvalidAction, "not in a game session")
		return
	}
	session, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		m.sendError(client.UserID, ws.ErrorInvalidAction, "game session not found")
		return
	}

	action, err := m.parseAction(client.UserID, session, msg)
	if err != nil {
		m.sendError(client.UserID, ws.ErrorInvalidAction, err.Error())
		return
	}

	if err := m.applyAndBroadcastAction(session, action); err != nil {
		m.sendGameError(client.UserID, err)
	}
}

// applyAndBroadcastAction applies and broadcasts a human or bot action through one path.
// It returns the rules-engine rejection, or nil on success and benign drops.
func (m *Manager) applyAndBroadcastAction(session *LiveMatch, action game.Action) error {
	return m.applyAndBroadcastActionWith(session, func(*game.GameState) (game.Action, bool) {
		return action, true
	})
}

// applyAndBroadcastActionWith builds the action under the session lock and applies it in the same
// critical section. build returning ok=false is a benign drop that leaves state and timers untouched.
func (m *Manager) applyAndBroadcastActionWith(session *LiveMatch, build func(gs *game.GameState) (game.Action, bool)) error {
	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		return nil
	}

	action, ok := build(session.gameState)
	if !ok {
		session.mu.Unlock()
		return nil
	}

	if action.Type == game.ActionContinue && session.gameState.Phase != game.PhaseHandComplete {
		session.mu.Unlock()
		return nil
	}

	session.cancelTurnTimer()

	oldState := session.gameState
	newState, err := game.ApplyAction(oldState, action)
	if err != nil {
		if oldState.Phase == game.PhaseHandComplete {
			remaining := max(time.Until(session.handCompleteExpiresAt), 0)
			gen := session.timerGeneration
			session.turnTimer = time.AfterFunc(remaining, func() {
				m.handleHandCompleteTimeout(session, gen)
			})
		} else if oldState.Phase == game.PhaseDeclaring {
			remaining := max(time.Until(session.declarationExpiresAt), 0)
			gen := session.timerGeneration
			session.turnTimer = time.AfterFunc(remaining, func() {
				m.handleDeclarationTimeout(session, gen)
			})
		} else if oldState.Phase != game.PhasePaused && session.timerStyle == "per-move" && oldState.TurnExpiresAt != nil {
			m.armTurnTimerLocked(session, time.Until(*oldState.TurnExpiresAt), oldState.ActivePlayerSeat)
		}
		session.mu.Unlock()
		return err
	}

	if newState.Phase == game.PhaseDealing {
		newState.Phase = game.PhaseBidding
		game.RefreshDerivedFlags(newState)
	}

	m.setDealGraceLocked(session, dealGraceFor(oldState, newState))

	if action.Type == game.ActionPause && newState.Phase == game.PhasePaused {
		if oldState.TurnExpiresAt != nil {
			remaining := time.Until(*oldState.TurnExpiresAt)
			if remaining > 0 {
				newState.TurnTimeRemaining = remaining.Milliseconds()
			}
		}
		newState.TurnExpiresAt = nil
	} else if (action.Type == game.ActionUnpause || action.Type == game.ActionOwnerUnpause) &&
		newState.Phase != game.PhasePaused && newState.Phase != game.PhaseDeclaring {
		if session.timerStyle == "per-move" {
			const minResumeMs int64 = 3000
			remaining := time.Duration(newState.TurnTimeRemaining) * time.Millisecond
			if newState.TurnTimeRemaining > 0 && newState.TurnTimeRemaining < minResumeMs {
				remaining = time.Duration(minResumeMs) * time.Millisecond
			} else if newState.TurnTimeRemaining <= 0 {
				remaining = time.Duration(minResumeMs) * time.Millisecond
			}
			expiry := time.Now().Add(remaining)
			newState.TurnExpiresAt = &expiry
			newState.TurnTimeRemaining = 0
			session.cancelTurnTimer()
			m.armTurnTimerLocked(session, remaining, newState.ActivePlayerSeat)
		} else {
			newState.TurnTimeRemaining = 0
		}
	} else if newState.Phase == game.PhaseDeclaring {
		newState.TurnExpiresAt = nil
		if oldState.Phase != game.PhaseDeclaring {
			session.declarationExpiresAt = time.Now().Add(declarationAutoClose)
		}
		remaining := max(time.Until(session.declarationExpiresAt), 0)
		gen := session.timerGeneration
		session.turnTimer = time.AfterFunc(remaining, func() {
			m.handleDeclarationTimeout(session, gen)
		})
	} else if newState.Phase == game.PhasePlaying || newState.Phase == game.PhaseBidding {
		preserveTimer := session.timerStyle == "per-move" &&
			oldState.TurnExpiresAt != nil &&
			newState.ActivePlayerSeat == oldState.ActivePlayerSeat &&
			newState.Phase == oldState.Phase
		if preserveTimer {
			newState.TurnExpiresAt = oldState.TurnExpiresAt
			newState.TimerDurationSec = oldState.TimerDurationSec
			m.armTurnTimerLocked(session, time.Until(*oldState.TurnExpiresAt), newState.ActivePlayerSeat)
		} else {
			m.setTurnExpiry(session, newState)
			m.startTimerLocked(session, newState.ActivePlayerSeat)
		}
	} else if newState.Phase == game.PhaseHandComplete {
		newState.TurnExpiresAt = nil
		if oldState.Phase != game.PhaseHandComplete {
			session.handCompleteExpiresAt = time.Now().Add(handCompleteAutoContinue)
		}
		remaining := max(time.Until(session.handCompleteExpiresAt), 0)
		gen := session.timerGeneration
		session.turnTimer = time.AfterFunc(remaining, func() {
			m.handleHandCompleteTimeout(session, gen)
		})
	}

	session.gameState = newState
	playerIDs := session.playerIDs
	startedAt := session.startedAt
	session.mu.Unlock()

	m.broadcastActionResult(playerIDs, oldState, newState, action, false)

	m.bufferHandResultIfScored(session, oldState, newState)

	m.observeBotMemory(session, oldState, newState, action)

	if newState.Phase == game.PhaseMatchEnd {
		var surrenderedBy *uint
		if action.Type == game.ActionSurrenderAccept && oldState.SurrenderProposerSeat != nil {
			proposerSeat := *oldState.SurrenderProposerSeat
			if proposerSeat >= 0 && proposerSeat < 4 {
				uid := playerIDs[proposerSeat]
				surrenderedBy = &uid
			}
		}
		matchEndPayload := buildMatchEndPayload(oldState, newState, action, startedAt)
		m.handleMatchEnd(session, newState, surrenderedBy, matchEndPayload)
		return nil
	}

	m.maybeScheduleBotAction(session)
	return nil
}

// GetStateSnapshot returns a shallow copy of the current game state for a room
// (used for reconnection and tests). Returns a copy to prevent callers from
// observing concurrent mutations after the lock is released.
func (m *Manager) GetStateSnapshot(roomID uint) *game.GameState {
	m.mu.RLock()
	session, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return nil
	}
	session.mu.RLock()
	defer session.mu.RUnlock()
	snapshot := *session.gameState
	return &snapshot
}

// RemoveSession cleans up a game session, cancelling any active timer.
func (m *Manager) RemoveSession(roomID uint) {
	m.mu.Lock()
	var removedIDs []uint
	if session, ok := m.sessions[roomID]; ok {
		session.mu.Lock()
		session.closed = true
		session.cancelTurnTimer()
		session.cancelAllReconnectTimers()
		session.cancelAllBotActionTimers()
		session.mu.Unlock()
		for _, uid := range session.playerIDs {
			if uid == 0 {
				continue
			}
			delete(m.userToRoom, uid)
			removedIDs = append(removedIDs, uid)
		}
		delete(m.sessions, roomID)
		slog.Info("session: removed", "roomID", roomID)
	}
	hooks := append([]func(userID uint){}, m.userRemovedHooks...)
	m.mu.Unlock()

	for _, uid := range removedIDs {
		for _, fn := range hooks {
			fn(uid)
		}
	}
}

// HasSession checks if a game session exists for a room.
func (m *Manager) HasSession(roomID uint) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.sessions[roomID]
	return ok
}

// IsUserInMatch returns true if the user is currently part of an active game session.
// Used by the chat handler to enforce the "no global chat while in a game" rule.
func (m *Manager) IsUserInMatch(userID uint) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.userToRoom[userID]
	return ok
}

// InMatchUserIDs returns a snapshot of every userID mapped to an active session, in no order.
func (m *Manager) InMatchUserIDs() []uint {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]uint, 0, len(m.userToRoom))
	for uid := range m.userToRoom {
		ids = append(ids, uid)
	}
	return ids
}

// MatchParticipants returns the four player userIDs for the active session keyed by roomID.
// It returns false when no session exists for that roomID.
func (m *Manager) MatchParticipants(roomID uint) ([4]uint, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[roomID]
	if !ok {
		return [4]uint{}, false
	}
	return s.playerIDs, true
}

// MatchParticipantsByUser returns the participants and the user's seat index for the user's active session.
// It returns ([4]uint{}, -1, false) when the user is not in any active session.
func (m *Manager) MatchParticipantsByUser(userID uint) ([4]uint, int, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	roomID, ok := m.userToRoom[userID]
	if !ok {
		return [4]uint{}, -1, false
	}
	s, ok := m.sessions[roomID]
	if !ok {
		return [4]uint{}, -1, false
	}
	for i, uid := range s.playerIDs {
		if uid == userID {
			return s.playerIDs, i, true
		}
	}
	return [4]uint{}, -1, false
}

// parseAction converts a WS message into a game.Action for the rules engine.
func (m *Manager) parseAction(userID uint, session *LiveMatch, msg ws.WSMessage) (game.Action, error) {
	seat := -1
	for i, uid := range session.playerIDs {
		if uid == userID {
			seat = i
			break
		}
	}
	if seat == -1 {
		return game.Action{}, fmt.Errorf("user %d not found in session", userID)
	}

	actionType := ""
	if len(msg.Type) > 7 && msg.Type[:7] == "action:" {
		actionType = msg.Type[7:]
	}
	if actionType == "" {
		return game.Action{}, fmt.Errorf("invalid action type: %s", msg.Type)
	}

	if actionType == "decline_belot" {
		actionType = game.ActionSkipBelot
	}

	action := game.Action{
		Type:       actionType,
		PlayerSeat: seat,
	}

	if actionType == game.ActionPlayCard {
		var payload struct {
			CardID string `json:"cardId"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return game.Action{}, fmt.Errorf("invalid play_card payload: %w", err)
		}
		card, err := game.ParseCard(payload.CardID)
		if err != nil {
			return game.Action{}, fmt.Errorf("invalid card: %w", err)
		}
		action.Card = &card
	}

	if actionType == game.ActionPickTrump {
		var payload struct {
			Suit string `json:"suit"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err == nil && payload.Suit != "" {
			suit := game.Suit(payload.Suit)
			action.Suit = &suit
		}
	}

	return action, nil
}

// trickResolvedWinnerSeat returns the winner of the trick that just completed.
// On a non-final hand's last trick the winner is only kept in LastHandResult.LastTrickSeat.
func trickResolvedWinnerSeat(oldState, newState *game.GameState) int {
	switch {
	case newState.TrickWinnerSeat != nil:
		return *newState.TrickWinnerSeat
	case oldState.HandNumber < newState.HandNumber && newState.LastHandResult != nil:
		return newState.LastHandResult.LastTrickSeat
	default:
		return newState.ActivePlayerSeat
	}
}

// broadcastActionResult sends the events for a successful action from local values only.
// Match-end broadcasts are left to handleMatchEnd, which sends them after persistence.
func (m *Manager) broadcastActionResult(playerIDs [4]uint, oldState, newState *game.GameState, action game.Action, autoPlayed bool) {
	userIDs := humanUserIDs(playerIDs)

	switch action.Type {
	case game.ActionPlayCard:
		cardPlayed := map[string]interface{}{
			"playerSeat": action.PlayerSeat,
			"cardId":     action.Card.String(),
			"autoPlayed": autoPlayed,
		}
		m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventCardPlayed, cardPlayed))

		trickCompleted := len(oldState.CurrentTrick) == 3 &&
			(len(newState.CurrentTrick) == 0 || newState.Phase != game.PhasePlaying)

		if trickCompleted {
			winnerSeat := trickResolvedWinnerSeat(oldState, newState)

			trickCards := make([]string, 0, 4)
			for _, tc := range oldState.CurrentTrick {
				trickCards = append(trickCards, tc.Card.String())
			}
			if action.Card != nil {
				trickCards = append(trickCards, action.Card.String())
			}

			trickResolved := map[string]interface{}{
				"winnerSeat": winnerSeat,
				"winnerTeam": game.TeamForSeat(winnerSeat),
				"cards":      trickCards,
			}
			m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventTrickResolved, trickResolved))
		}

		m.broadcastDeclarationsResolvedIfTransition(oldState, newState, userIDs)

		handJustScored := newState.LastHandResult != nil &&
			newState.LastHandResult.HandNumber == oldState.HandNumber &&
			oldState.Phase != game.PhaseHandComplete &&
			oldState.Phase != newState.Phase &&
			(newState.Phase == game.PhaseHandComplete || newState.Phase == game.PhaseMatchEnd)
		if handJustScored {
			hr := newState.LastHandResult
			handScored := map[string]interface{}{
				"teamACardPoints": hr.TeamACardPoints,
				"teamBCardPoints": hr.TeamBCardPoints,
				"teamADeclPoints": hr.TeamADeclPoints,
				"teamBDeclPoints": hr.TeamBDeclPoints,
				"lastTrickTeam":   hr.LastTrickTeam,
				"lastTrickBonus":  hr.LastTrickBonus,
				"capot":           hr.Capot,
				"capotTeam":       hr.CapotTeam,
				"capotBonus":      hr.CapotBonus,
				"failedContract":  hr.FailedContract,
				"contractingTeam": hr.ContractingTeam,
				"teamAHandTotal":  hr.TeamAHandTotal,
				"teamBHandTotal":  hr.TeamBHandTotal,
				"teamAMatchScore": newState.TeamScores[game.TeamA],
				"teamBMatchScore": newState.TeamScores[game.TeamB],
			}
			m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventHandScored, handScored))
		}

		if newState.Phase == game.PhaseMatchEnd {
			return
		}

		m.broadcastState(playerIDs, newState)

	case game.ActionPickTrump:
		if newState.TrumpSuit != nil {
			switch {
			case oldState.TrumpCandidate != nil:
				m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventTrumpSelected, ws.TrumpSelectedPayload{
					PlayerSeat: action.PlayerSeat,
					TrumpSuit:  string(*newState.TrumpSuit),
					CardID:     oldState.TrumpCandidate.String(),
				}))
			case !oldState.Rules.HasTrumpCandidate:
				m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventTrumpSelected, ws.TrumpSelectedPayload{
					PlayerSeat: action.PlayerSeat,
					TrumpSuit:  string(*newState.TrumpSuit),
					CardID:     "",
				}))
			default:
				slog.Warn("session: trump_selected suppressed; oldState.TrumpCandidate is nil under a candidate config",
					"roomID", newState.RoomID, "playerSeat", action.PlayerSeat)
			}
		}
		m.broadcastDeclarationsResolvedIfTransition(oldState, newState, userIDs)
		m.broadcastState(playerIDs, newState)

	case game.ActionPassTrump:
		m.broadcastState(playerIDs, newState)

	case game.ActionDeclare, game.ActionSkipDeclare:
		if action.Type == game.ActionDeclare && oldState.Phase != game.PhaseDeclaring {
			declared := ws.PlayerDeclaredPayload{PlayerSeat: action.PlayerSeat}
			m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventPlayerDeclared, declared))
		}
		m.broadcastDeclarationsResolvedIfTransition(oldState, newState, userIDs)
		if newState.Phase == game.PhaseMatchEnd {
			return
		}
		m.broadcastState(playerIDs, newState)

	case game.ActionAnnounceBelot, game.ActionSkipBelot:
		if newState.BelotAnnounced && !oldState.BelotAnnounced {
			var cardID string
			if n := len(oldState.CurrentTrick); n > 0 {
				cardID = oldState.CurrentTrick[n-1].Card.String()
			}
			belot := ws.BelotAnnouncedPayload{
				PlayerSeat: action.PlayerSeat,
				Team:       game.TeamForSeat(action.PlayerSeat),
				CardID:     cardID,
			}
			m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventBelotAnnounced, belot))
		}
		if len(oldState.CurrentTrick) == 4 && newState.TrickNumber != oldState.TrickNumber {
			winnerSeat := trickResolvedWinnerSeat(oldState, newState)
			trickCards := make([]string, 0, 4)
			for _, tc := range oldState.CurrentTrick {
				trickCards = append(trickCards, tc.Card.String())
			}
			trickResolved := map[string]interface{}{
				"winnerSeat": winnerSeat,
				"winnerTeam": game.TeamForSeat(winnerSeat),
				"cards":      trickCards,
			}
			m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventTrickResolved, trickResolved))
			m.broadcastDeclarationsResolvedIfTransition(oldState, newState, userIDs)
		}
		if newState.Phase == game.PhaseMatchEnd {
			return
		}
		m.broadcastState(playerIDs, newState)

	case game.ActionContinue:
		if newState.Phase == game.PhaseMatchEnd {
			return
		}
		m.broadcastState(playerIDs, newState)

	case game.ActionPause:
		paused := ws.MatchPausedPayload{
			PausedBy:      action.PlayerSeat,
			PausedPlayers: newState.PausedPlayers,
		}
		m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventMatchPaused, paused))
		m.broadcastState(playerIDs, newState)

	case game.ActionSurrenderRequest:
		proposerSeat := action.PlayerSeat
		var proposerUsername string
		if proposerSeat >= 0 && proposerSeat < 4 {
			proposerUsername = newState.Players[proposerSeat].Username
		}
		proposed := ws.SurrenderProposedPayload{
			ProposerSeat:     proposerSeat,
			ProposerTeam:     game.TeamForSeat(proposerSeat),
			ProposerUsername: proposerUsername,
			PartnerSeat:      (proposerSeat + 2) % 4,
		}
		m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventSurrenderProposed, proposed))
		m.broadcastState(playerIDs, newState)

	case game.ActionSurrenderDecline:
		if oldState.SurrenderProposerSeat != nil {
			declined := ws.SurrenderDeclinedPayload{
				ProposerSeat:  *oldState.SurrenderProposerSeat,
				DecliningSeat: action.PlayerSeat,
			}
			m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventSurrenderDeclined, declined))
		} else {
			slog.Warn("session: surrender_declined broadcast suppressed; oldState.SurrenderProposerSeat is nil",
				"decliningSeat", action.PlayerSeat)
		}
		m.broadcastState(playerIDs, newState)

	case game.ActionSurrenderAccept:
		if newState.Phase == game.PhaseMatchEnd {
			return
		}
		m.broadcastState(playerIDs, newState)

	case game.ActionUnpause, game.ActionOwnerUnpause:
		resumed := ws.MatchResumedPayload{
			ResumedBy:     action.PlayerSeat,
			OwnerOverride: action.Type == game.ActionOwnerUnpause,
		}
		if newState.Phase != game.PhasePaused {
			m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventMatchResumed, resumed))
		}
		m.broadcastState(playerIDs, newState)

	default:
		m.broadcastState(playerIDs, newState)
	}
}

// broadcastDeclarationsResolvedIfTransition fires event:declarations_resolved once,
// when DeclarationsResolved flips from false to true.
func (m *Manager) broadcastDeclarationsResolvedIfTransition(oldState, newState *game.GameState, userIDs []uint) {
	if !newState.DeclarationsResolved || oldState.DeclarationsResolved {
		return
	}

	decls := make([]ws.DeclarationPayload, 0)
	for _, p := range newState.Players {
		for _, d := range p.Declarations {
			decls = append(decls, ws.DeclarationPayload{
				PlayerSeat: d.PlayerSeat,
				Type:       string(d.Type),
				Value:      d.Value,
				Cards:      cardsToIDs(d.Cards),
			})
		}
	}

	var declWinnerTeam *int
	if newState.DeclarationPoints[game.TeamA] > 0 {
		t := game.TeamA
		declWinnerTeam = &t
	} else if newState.DeclarationPoints[game.TeamB] > 0 {
		t := game.TeamB
		declWinnerTeam = &t
	}

	payload := ws.DeclarationsResolvedPayload{
		WinnerTeam:   declWinnerTeam,
		Contested:    newState.DeclarationsContested,
		Declarations: decls,
	}
	m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventDeclarationsResolved, payload))
}

// bufferHandResultIfScored appends the completed hand's result to session.handResults
// when the transition scored a hand; otherwise it does nothing.
func (m *Manager) bufferHandResultIfScored(session *LiveMatch, oldState, newState *game.GameState) {
	if oldState == nil || newState == nil || newState.LastHandResult == nil {
		return
	}
	handAdvanced := oldState.HandNumber < newState.HandNumber
	matchEndTransition := oldState.Phase != game.PhaseMatchEnd && newState.Phase == game.PhaseMatchEnd
	if !handAdvanced && !matchEndTransition {
		return
	}
	hr := newState.LastHandResult
	var capotTeam *int
	if hr.CapotTeam != nil {
		v := *hr.CapotTeam
		capotTeam = &v
	}
	row := HandResult{
		HandNumber:      hr.HandNumber,
		TeamACardPoints: hr.TeamACardPoints,
		TeamBCardPoints: hr.TeamBCardPoints,
		TeamADeclPoints: hr.TeamADeclPoints,
		TeamBDeclPoints: hr.TeamBDeclPoints,
		LastTrickTeam:   hr.LastTrickTeam,
		LastTrickBonus:  hr.LastTrickBonus,
		Capot:           hr.Capot,
		CapotTeam:       capotTeam,
		CapotBonus:      hr.CapotBonus,
		FailedContract:  hr.FailedContract,
		ContractingTeam: hr.ContractingTeam,
		TeamAHandTotal:  hr.TeamAHandTotal,
		TeamBHandTotal:  hr.TeamBHandTotal,
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	for _, existing := range session.handResults {
		if existing.HandNumber == row.HandNumber {
			return
		}
	}
	session.handResults = append(session.handResults, row)
}

// handleMatchEnd persists the match and room status, then always broadcasts event:match_end
// followed by event:match_state, and removes the session.
func (m *Manager) handleMatchEnd(session *LiveMatch, finalState *game.GameState, surrenderedBy *uint, matchEndPayload ws.MatchEndPayload) {
	winnerTeam := 0
	if finalState.WinnerTeam != nil {
		winnerTeam = *finalState.WinnerTeam
	}

	var botSeats [4]bool
	for i := range finalState.Players {
		botSeats[i] = finalState.Players[i].IsBot
	}
	ids, botFlags, hasBots := matchSeatColumns(session.playerIDs, botSeats)

	session.mu.RLock()
	handsCopy := make([]HandResult, len(session.handResults))
	copy(handsCopy, session.handResults)
	session.mu.RUnlock()

	deltas, settlementMsgs := m.settleMatch(session.roomID, session.playerIDs, botSeats, winnerTeam, session.coinBuyIn)

	xpMsgs := m.awardXP(session.roomID, session.playerIDs, botSeats, finalState.TeamScores, -1)

	var connected [4]bool
	for i := range finalState.Players {
		connected[i] = finalState.Players[i].Connected
	}
	honorMsgs := m.recordHonor(session.roomID, session.playerIDs, botSeats, connected, -1)

	finalizedAt := time.Now().UTC()
	spMsgs := m.awardSeasonPoints(session.roomID, buildSPOutcome(spMatchFacts{
		playerIDs:     session.playerIDs,
		botSeats:      botSeats,
		connected:     connected,
		winnerTeam:    winnerTeam,
		teamScores:    finalState.TeamScores,
		matchMode:     finalState.MatchMode,
		surrender:     matchEndPayload.OutcomeReason == ws.OutcomeReasonSurrender,
		instantWin:    finalState.WonByInstantWin,
		hands:         handsCopy,
		abandonedSeat: -1,
	}), finalizedAt)

	matchRecord := &Match{
		RoomID:               session.roomID,
		Player1ID:            ids[0],
		Player2ID:            ids[1],
		Player3ID:            ids[2],
		Player4ID:            ids[3],
		Player1IsBot:         botFlags[0],
		Player2IsBot:         botFlags[1],
		Player3IsBot:         botFlags[2],
		Player4IsBot:         botFlags[3],
		HasBots:              hasBots,
		TeamAScore:           finalState.TeamScores[game.TeamA],
		TeamBScore:           finalState.TeamScores[game.TeamB],
		WinnerTeam:           winnerTeam,
		Variant:              string(finalState.Variant),
		MatchMode:            finalState.MatchMode,
		DeclarationsEnabled:  finalState.Rules.DeclarationsEnabled,
		StopAtTarget:         finalState.Rules.StopAtTarget,
		DeclarationsTransfer: finalState.Rules.DeclarationsTransfer,
		Clockwise:            finalState.Rules.Clockwise,
		StartedAt:            session.startedAt,
		CompletedAt:          time.Now(),
		Status:               "completed",
		SurrenderedBy:        surrenderedBy,
		CoinBuyIn:            session.coinBuyIn,
		Player1CoinDelta:     deltas[0],
		Player2CoinDelta:     deltas[1],
		Player3CoinDelta:     deltas[2],
		Player4CoinDelta:     deltas[3],
	}

	if err := m.matchRepo.CreateWithHands(matchRecord, handsCopy); err != nil {
		slog.Error("session: failed to persist match", "roomID", session.roomID, "error", err)
	} else {
		slog.Info("session: match persisted", "roomID", session.roomID, "matchID", matchRecord.ID, "hands", len(handsCopy))
	}

	if m.roomUpdater != nil {
		if err := m.roomUpdater.UpdateRoomStatus(session.roomID, "completed"); err != nil {
			slog.Error("session: failed to update room status", "roomID", session.roomID, "error", err)
		}
	}

	userIDs := humanUserIDs(session.playerIDs)
	m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventMatchEnd, matchEndPayload))
	for _, sm := range settlementMsgs {
		m.hub.SendToUser(sm.userID, sm.msg)
	}
	for _, xm := range xpMsgs {
		m.hub.SendToUser(xm.userID, xm.msg)
	}
	for _, hm := range honorMsgs {
		m.hub.SendToUser(hm.userID, hm.msg)
	}
	for _, spm := range spMsgs {
		m.hub.SendToUser(spm.userID, spm.msg)
	}
	m.broadcastState(session.playerIDs, finalState)

	m.RemoveSession(session.roomID)
}

// sendError sends an error event to a single user.
func (m *Manager) sendError(userID uint, errorType string, message string) {
	payload := map[string]string{"message": message}
	m.hub.SendToUser(userID, buildMessage(errorType, payload))
}

// sendGameError maps rules engine errors to appropriate WS error events.
func (m *Manager) sendGameError(userID uint, err error) {
	eventType := ws.ErrorInvalidAction
	switch {
	case errors.Is(err, apperr.ErrNotYourTurn):
		eventType = ws.ErrorNotYourTurn
	case errors.Is(err, apperr.ErrWrongPhase):
		eventType = ws.ErrorWrongPhase
	case errors.Is(err, apperr.ErrIllegalPlay):
		eventType = ws.ErrorIllegalPlay
	case errors.Is(err, apperr.ErrPauseExhausted):
		eventType = ws.ErrorPauseExhausted
	case errors.Is(err, apperr.ErrNoActivePause):
		eventType = ws.ErrorNoActivePause
	case errors.Is(err, apperr.ErrNotRoomOwner):
		eventType = ws.ErrorNotRoomOwner
	case errors.Is(err, apperr.ErrPlayerDisconnected):
		eventType = ws.ErrorPlayerDisconnected
	case errors.Is(err, apperr.ErrSurrenderExhausted):
		eventType = ws.ErrorSurrenderExhausted
	case errors.Is(err, apperr.ErrMustPickTrump):
		eventType = ws.ErrorMustPickTrump
	}
	m.sendError(userID, eventType, err.Error())
}

// buildStateFrames builds one ProjectForSeat-masked match_state frame per human seat, in seat order.
// excludeSeat omits one seat (-1 for none); bot seats never get a frame.
func buildStateFrames(playerIDs [4]uint, state *game.GameState, excludeSeat int) []ws.UserFrame {
	frames := make([]ws.UserFrame, 0, 4)
	for seat, uid := range playerIDs {
		if uid == 0 || seat == excludeSeat {
			continue
		}
		frames = append(frames, ws.UserFrame{
			UserID: uid,
			Msg:    buildMessage(ws.EventMatchState, game.ProjectForSeat(state, seat)),
		})
	}
	return frames
}

// broadcastState sends the state to every human seat, projected per recipient.
func (m *Manager) broadcastState(playerIDs [4]uint, state *game.GameState) {
	m.hub.SendFrames(buildStateFrames(playerIDs, state, -1))
}

// buildMessage creates a JSON-encoded WS message.
func buildMessage(eventType string, payload interface{}) []byte {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("session: failed to marshal payload", "eventType", eventType, "error", err)
		payloadBytes = []byte(`{}`)
	}
	now := time.Now()
	msg, err := json.Marshal(ws.WSMessage{
		Type:      eventType,
		Payload:   payloadBytes,
		ServerNow: &now,
	})
	if err != nil {
		slog.Error("session: failed to marshal message", "eventType", eventType, "error", err)
		return nil
	}
	return msg
}

// setTurnExpiry sets TurnExpiresAt from the timer config, after any deal-animation grace.
// Must be called under session.mu.Lock().
func (m *Manager) setTurnExpiry(session *LiveMatch, gs *game.GameState) {
	if session.timerStyle == "per-move" && session.timerDurationSec > 0 {
		window := time.Duration(session.timerDurationSec)*time.Second + session.dealGraceRemaining()
		expiry := time.Now().Add(window)
		gs.TurnExpiresAt = &expiry
		gs.TimerDurationSec = session.timerDurationSec
	} else {
		gs.TurnExpiresAt = nil
		gs.TimerDurationSec = 0
	}
}

const expiryGrace = 400 * time.Millisecond

// armTurnTimerLocked arms the turn timer to fire expiryGrace after remaining (clamped at 0)
// for the current generation. Callers must cancel first and hold session.mu.
func (m *Manager) armTurnTimerLocked(session *LiveMatch, remaining time.Duration, expectedSeat int) {
	if session.turnTimer != nil {
		session.turnTimer.Stop()
	}
	gen := session.timerGeneration
	session.turnTimer = time.AfterFunc(max(remaining, 0)+expiryGrace, func() {
		m.handleTimerExpiry(session, gen, expectedSeat)
	})
}

// startTimerLocked starts the per-move turn timer for expectedSeat, the post-action seat.
// Must be called under session.mu.Lock().
func (m *Manager) startTimerLocked(session *LiveMatch, expectedSeat int) {
	if session.timerStyle != "per-move" || session.timerDurationSec <= 0 {
		return
	}
	session.cancelTurnTimer()
	window := time.Duration(session.timerDurationSec)*time.Second + session.dealGraceRemaining()
	m.armTurnTimerLocked(session, window, expectedSeat)
}

const handCompleteAutoContinue = 14 * time.Second

const declarationAutoClose = 20 * time.Second

// handleDeclarationTimeout force-closes the declaration phase when its window elapses,
// treating unanswered seats as having skipped.
func (m *Manager) handleDeclarationTimeout(session *LiveMatch, generation uint64) {
	session.mu.Lock()
	if session.closed || session.timerGeneration != generation {
		session.mu.Unlock()
		return
	}
	oldState := session.gameState
	if oldState.Phase != game.PhaseDeclaring {
		session.mu.Unlock()
		return
	}

	newState, err := game.ForceCloseDeclarationPhase(oldState)
	if err != nil {
		session.mu.Unlock()
		return
	}
	matchEnded := newState.Phase == game.PhaseMatchEnd
	if !matchEnded {
		m.setDealGraceLocked(session, 0)
		m.setTurnExpiry(session, newState)
		m.startTimerLocked(session, newState.ActivePlayerSeat)
	}
	session.gameState = newState
	m.observeBotMemoryLocked(session, oldState, newState, game.Action{})
	playerIDs := session.playerIDs
	startedAt := session.startedAt
	session.mu.Unlock()
	userIDs := humanUserIDs(playerIDs)

	m.broadcastDeclarationsResolvedIfTransition(oldState, newState, userIDs)

	if matchEnded {
		matchEndPayload := buildMatchEndPayload(oldState, newState, game.Action{}, startedAt)
		m.handleMatchEnd(session, newState, nil, matchEndPayload)
		return
	}

	m.broadcastState(playerIDs, newState)

	m.maybeScheduleBotAction(session)
}

// handleHandCompleteTimeout force-advances the hand-complete pause when the
// auto-continue window elapses without every connected player acknowledging.
func (m *Manager) handleHandCompleteTimeout(session *LiveMatch, generation uint64) {
	session.mu.Lock()
	if session.closed || session.timerGeneration != generation {
		session.mu.Unlock()
		return
	}
	oldState := session.gameState
	if oldState.Phase != game.PhaseHandComplete {
		session.mu.Unlock()
		return
	}

	newState, err := game.ForceAdvanceHandComplete(oldState)
	if err != nil {
		session.mu.Unlock()
		return
	}
	if newState.Phase == game.PhaseDealing {
		newState.Phase = game.PhaseBidding
		game.RefreshDerivedFlags(newState)
	}
	m.setDealGraceLocked(session, dealGraceFor(oldState, newState))
	if newState.Phase == game.PhasePlaying || newState.Phase == game.PhaseBidding {
		m.setTurnExpiry(session, newState)
		m.startTimerLocked(session, newState.ActivePlayerSeat)
	}
	session.gameState = newState
	playerIDs := session.playerIDs
	startedAt := session.startedAt
	session.mu.Unlock()

	m.bufferHandResultIfScored(session, oldState, newState)
	m.observeBotMemory(session, oldState, newState, game.Action{})

	if newState.Phase == game.PhaseMatchEnd {
		matchEndPayload := buildMatchEndPayload(oldState, newState, game.Action{Type: game.ActionContinue}, startedAt)
		m.handleMatchEnd(session, newState, nil, matchEndPayload)
		return
	}
	m.broadcastState(playerIDs, newState)

	m.maybeScheduleBotAction(session)
}

// handleTimerExpiry auto-plays for the active player when a per-move timer fires.
// The generation counter drops stale callbacks.
func (m *Manager) handleTimerExpiry(session *LiveMatch, generation uint64, expectedSeat int) {
	session.mu.Lock()

	if session.closed || session.timerGeneration != generation {
		session.mu.Unlock()
		return
	}

	gs := session.gameState

	var action game.Action
	switch {
	case gs.Phase == game.PhaseBidding && game.MustPickTrump(gs, expectedSeat):
		suit, err := game.AutoPickTrumpSuit(gs, expectedSeat)
		if err != nil {
			slog.Error("session: auto-pick trump failed; re-arming",
				"roomID", session.roomID, "seat", expectedSeat, "error", err)
			m.setTurnExpiry(session, gs)
			m.startTimerLocked(session, expectedSeat)
			staleFrames := buildStateFrames(session.playerIDs, gs, -1)
			session.mu.Unlock()
			m.hub.SendFrames(staleFrames)
			return
		}
		action = game.Action{
			Type:       game.ActionPickTrump,
			PlayerSeat: expectedSeat,
			Suit:       &suit,
		}
	case gs.Phase == game.PhaseBidding:
		action = game.Action{
			Type:       game.ActionPassTrump,
			PlayerSeat: expectedSeat,
		}
	case gs.Phase == game.PhasePlaying && gs.AwaitingDeclaration:
		action = game.Action{
			Type:       game.ActionSkipDeclare,
			PlayerSeat: expectedSeat,
		}
	case gs.Phase == game.PhaseDeclaring:
		slog.Error("session: declaring phase reached the per-move timer expiry; re-arming its window",
			"roomID", session.roomID, "activeSeat", gs.ActivePlayerSeat, "expectedSeat", expectedSeat)
		session.declarationExpiresAt = time.Now().Add(declarationAutoClose)
		session.cancelTurnTimer()
		gen := session.timerGeneration
		session.turnTimer = time.AfterFunc(declarationAutoClose, func() {
			m.handleDeclarationTimeout(session, gen)
		})
		session.mu.Unlock()
		return
	case gs.Phase == game.PhasePlaying && gs.PendingBelotSeat != nil && *gs.PendingBelotSeat == expectedSeat:
		action = game.Action{
			Type:       game.ActionSkipBelot,
			PlayerSeat: expectedSeat,
		}
	case gs.Phase == game.PhasePlaying:
		cardID, err := game.AutoPlay(gs)
		if err != nil {
			slog.Error("session: auto-play failed", "roomID", session.roomID, "error", err)
			m.startTimerLocked(session, expectedSeat)
			session.mu.Unlock()
			return
		}
		card, err := game.ParseCard(cardID)
		if err != nil {
			slog.Error("session: auto-play card parse failed", "roomID", session.roomID, "cardID", cardID, "error", err)
			m.startTimerLocked(session, expectedSeat)
			session.mu.Unlock()
			return
		}
		action = game.Action{
			Type:       game.ActionPlayCard,
			PlayerSeat: expectedSeat,
			Card:       &card,
		}
	default:
		session.mu.Unlock()
		return
	}

	oldState := gs
	newState, err := game.ApplyAction(oldState, action)
	if err != nil {
		slog.Error("session: auto-play ApplyAction failed", "roomID", session.roomID, "error", err)
		m.startTimerLocked(session, expectedSeat)
		session.mu.Unlock()
		return
	}

	if newState.Phase == game.PhaseDealing {
		newState.Phase = game.PhaseBidding
		game.RefreshDerivedFlags(newState)
	}

	const maxChainSteps = 3
	type chainStep struct {
		action game.Action
		pre    *game.GameState
		post   *game.GameState
	}
	steps := []chainStep{{action: action, pre: oldState, post: newState}}

	for i := 0; i < maxChainSteps; i++ {
		cur := steps[len(steps)-1].post
		if cur.ActivePlayerSeat != oldState.ActivePlayerSeat || cur.Phase != oldState.Phase {
			break
		}
		if cur.Phase != game.PhasePlaying {
			break
		}
		var next game.Action
		switch {
		case cur.PendingBelotSeat != nil && *cur.PendingBelotSeat == cur.ActivePlayerSeat:
			next = game.Action{Type: game.ActionSkipBelot, PlayerSeat: cur.ActivePlayerSeat}
		case cur.AwaitingDeclaration:
			next = game.Action{Type: game.ActionSkipDeclare, PlayerSeat: cur.ActivePlayerSeat}
		default:
			cardID, autoErr := game.AutoPlay(cur)
			if autoErr != nil {
				slog.Error("session: chain AutoPlay failed", "roomID", session.roomID, "error", autoErr)
				break
			}
			card, parseErr := game.ParseCard(cardID)
			if parseErr != nil {
				slog.Error("session: chain ParseCard failed", "roomID", session.roomID, "cardID", cardID, "error", parseErr)
				break
			}
			next = game.Action{Type: game.ActionPlayCard, PlayerSeat: cur.ActivePlayerSeat, Card: &card}
		}
		if next.Type == "" {
			break
		}
		appliedNS, applyErr := game.ApplyAction(cur, next)
		if applyErr != nil {
			slog.Error("session: chain ApplyAction failed", "roomID", session.roomID, "error", applyErr)
			break
		}
		if appliedNS.Phase == game.PhaseDealing {
			appliedNS.Phase = game.PhaseBidding
			game.RefreshDerivedFlags(appliedNS)
		}
		steps = append(steps, chainStep{action: next, pre: cur, post: appliedNS})
	}

	finalState := steps[len(steps)-1].post

	m.setDealGraceLocked(session, dealGraceFor(oldState, finalState))

	if finalState.Phase == game.PhaseDeclaring {
		finalState.TurnExpiresAt = nil
		session.declarationExpiresAt = time.Now().Add(declarationAutoClose)
		session.cancelTurnTimer()
		gen := session.timerGeneration
		session.turnTimer = time.AfterFunc(declarationAutoClose, func() {
			m.handleDeclarationTimeout(session, gen)
		})
	} else if finalState.Phase == game.PhasePlaying || finalState.Phase == game.PhaseBidding {
		m.setTurnExpiry(session, finalState)
		m.startTimerLocked(session, finalState.ActivePlayerSeat)
	} else if finalState.Phase == game.PhaseHandComplete {
		finalState.TurnExpiresAt = nil
		session.handCompleteExpiresAt = time.Now().Add(handCompleteAutoContinue)
		session.cancelTurnTimer()
		gen := session.timerGeneration
		session.turnTimer = time.AfterFunc(handCompleteAutoContinue, func() {
			m.handleHandCompleteTimeout(session, gen)
		})
	}

	session.gameState = finalState
	playerIDs := session.playerIDs
	startedAt := session.startedAt
	session.mu.Unlock()

	if autoType, ok := autoActionTypeFor(action.Type); ok {
		m.sendAutoAction(playerIDs, expectedSeat, autoType)
	}

	for _, step := range steps {
		isAutoPlayedCard := step.action.Type == game.ActionPlayCard
		m.broadcastActionResult(playerIDs, step.pre, step.post, step.action, isAutoPlayedCard)
		m.bufferHandResultIfScored(session, step.pre, step.post)
		m.observeBotMemory(session, step.pre, step.post, step.action)
	}

	if finalState.Phase == game.PhaseMatchEnd {
		last := steps[len(steps)-1]
		matchEndAction := last.action
		matchEndOld := last.pre
		matchEndPayload := buildMatchEndPayload(matchEndOld, finalState, matchEndAction, startedAt)
		m.handleMatchEnd(session, finalState, nil, matchEndPayload)
		return
	}

	m.maybeScheduleBotAction(session)
}

// autoActionTypeFor maps an action type to the AutoActionType emitted on timer expiry.
// It returns false for actions without an EventAutoAction, such as ActionPlayCard.
func autoActionTypeFor(actionType string) (ws.AutoActionType, bool) {
	switch actionType {
	case game.ActionPassTrump:
		return ws.AutoActionPassTrump, true
	case game.ActionSkipDeclare:
		return ws.AutoActionSkipDeclare, true
	case game.ActionSkipBelot:
		return ws.AutoActionSkipBelot, true
	case game.ActionPickTrump:
		return ws.AutoActionPickTrump, true
	}
	return "", false
}

// sendAutoAction tells every human player about a timeout auto-action. A private skip reaches only the timed-out
// player by name; the others see a plain timeout, because the skip would reveal what that seat holds.
func (m *Manager) sendAutoAction(playerIDs [4]uint, seat int, autoType ws.AutoActionType) {
	exact := buildMessage(ws.EventAutoAction, ws.AutoActionPayload{PlayerSeat: seat, Type: autoType})
	if !isPrivateAutoAction(autoType) {
		m.hub.BroadcastToUsers(humanUserIDs(playerIDs), exact)
		return
	}
	masked := buildMessage(ws.EventAutoAction, ws.AutoActionPayload{PlayerSeat: seat, Type: ws.AutoActionTimeout})
	frames := make([]ws.UserFrame, 0, len(playerIDs))
	for s, id := range playerIDs {
		if id == 0 {
			continue
		}
		msg := masked
		if s == seat {
			msg = exact
		}
		frames = append(frames, ws.UserFrame{UserID: id, Msg: msg})
	}
	m.hub.SendFrames(frames)
}

// isPrivateAutoAction reports whether an auto-action would expose the timed-out seat's hand to the other players.
func isPrivateAutoAction(autoType ws.AutoActionType) bool {
	return autoType == ws.AutoActionSkipDeclare || autoType == ws.AutoActionSkipBelot
}

func safeDerefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// buildMatchEndPayload constructs the typed event:match_end payload.
// OutcomeReason is always set explicitly so the wire format is fully symmetric:
// natural-end matches emit "natural", surrender emits "surrender".
func buildMatchEndPayload(oldState, newState *game.GameState, action game.Action, startedAt time.Time) ws.MatchEndPayload {
	durationSec := 0
	if startedAt.IsZero() {
		slog.Error("match: match-end payload built with a zero startedAt; duration reported as 0",
			"roomID", newState.RoomID)
	} else {
		durationSec = int(time.Since(startedAt).Seconds())
	}

	payload := ws.MatchEndPayload{
		WinnerTeam:       safeDerefInt(newState.WinnerTeam),
		TeamAFinalScore:  newState.TeamScores[game.TeamA],
		TeamBFinalScore:  newState.TeamScores[game.TeamB],
		MatchDurationSec: durationSec,
		OutcomeReason:    ws.OutcomeReasonNatural,
	}
	if action.Type == game.ActionSurrenderAccept && oldState != nil && oldState.SurrenderProposerSeat != nil {
		proposerSeat := *oldState.SurrenderProposerSeat
		payload.OutcomeReason = ws.OutcomeReasonSurrender
		payload.SurrenderedBySeat = &proposerSeat
	}
	if newState.StoppedAtTarget {
		payload.OutcomeReason = ws.OutcomeReasonTargetReached
	}
	return payload
}

func cardsToIDs(cards []game.Card) []string {
	ids := make([]string, len(cards))
	for i, c := range cards {
		ids[i] = c.String()
	}
	return ids
}
