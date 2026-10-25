package match

import (
	"errors"
	"time"

	"github.com/emilijan/beljot/server/internal/bot"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/ws"
)

// SetBotDelayForTest overrides the bot think-delay bounds so manager tests
// with bots don't sleep for real.
func (m *Manager) SetBotDelayForTest(minDelay, maxDelay time.Duration) {
	m.botDelayMin = minDelay
	m.botDelayMax = maxDelay
}

// SetDealGraceForTest switches the deal grace (deal_grace.go) on or off. Tests
// that are not about the deal turn it off so deadlines and bot delays stay at
// their bare values.
func (m *Manager) SetDealGraceForTest(enabled bool) {
	m.dealGraceEnabled = enabled
}

// DealGraceRemainingForTest reports how much deal grace the room's current
// turn still carries.
func (m *Manager) DealGraceRemainingForTest(roomID uint) time.Duration {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return 0
	}
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.dealGraceRemaining()
}

// BotKnownCardsForTest returns the cards the session's bot memory holds per seat, or the zero
// value when the room has no session or no bot memory.
func (m *Manager) BotKnownCardsForTest(roomID uint) [4][]game.Card {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return [4][]game.Card{}
	}
	lm.mu.Lock()
	defer lm.mu.Unlock()
	if lm.botMemory == nil {
		return [4][]game.Card{}
	}
	return lm.botMemory.KnownCards()
}

// BotThinkDelayForTest exposes botThinkDelay: the delay a bot at `seat` would
// draw before acting on gs.
func (m *Manager) BotThinkDelayForTest(gs *game.GameState, seat int) time.Duration {
	return m.botThinkDelay(gs, seat)
}

// BotSchedule exposes maybeScheduleBotAction for tests that inject a game
// state via SetGameStateForTest and need the driver to re-evaluate it.
func (m *Manager) BotSchedule(roomID uint) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	m.maybeScheduleBotAction(lm)
}

// HandleMatchEndForTest exposes handleMatchEnd so tests can assert the
// persist-before-broadcast invariant without driving a full game.
func (m *Manager) HandleMatchEndForTest(roomID uint, finalState *game.GameState, surrenderedBy *uint, payload ws.MatchEndPayload) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	m.handleMatchEnd(lm, finalState, surrenderedBy, payload)
}

// AutoActionTypeFor exposes autoActionTypeFor for tests in the external
// session_test package. The wire-format mapping is the contract surface;
// keep it tested independently so future refactors don't silently drift.
func AutoActionTypeFor(actionType string) (ws.AutoActionType, bool) {
	return autoActionTypeFor(actionType)
}

// AbandonSeatForTest marks the seat disconnected and invokes the reconnect-timeout
// handler directly, driving the abandonment path without waiting on a real timer.
func (m *Manager) AbandonSeatForTest(roomID uint, seat int) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	lm.mu.Lock()
	lm.gameState.Players[seat].Connected = false
	gen := lm.seatReconnectGenerations[seat]
	lm.mu.Unlock()
	m.handleSeatReconnectTimeout(lm, seat, gen)
}

// BufferHandResultIfScored exposes bufferHandResultIfScored for tests in the
// external session_test package.
func (m *Manager) BufferHandResultIfScored(roomID uint, oldState, newState *game.GameState) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	m.bufferHandResultIfScored(lm, oldState, newState)
}

// HandResults returns a copy of the lm's buffered hand results for tests.
func (m *Manager) HandResults(roomID uint) []HandResult {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return nil
	}
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	out := make([]HandResult, len(lm.handResults))
	copy(out, lm.handResults)
	return out
}

// ApplyActionForTest drives applyAndBroadcastAction for one action, so tests can
// assert what a single successful action puts on the wire without going through
// a ws.Client. Tests using this helper must call StartMatch first.
func (m *Manager) ApplyActionForTest(roomID uint, action game.Action) error {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return errNoSession
	}
	return m.applyAndBroadcastAction(lm, action)
}

// SetGameStateForTest replaces the lm's game state to drive specific mid-game
// states. Tests using this helper must call StartMatch first.
func (m *Manager) SetGameStateForTest(roomID uint, gs *game.GameState) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	lm.mu.Lock()
	lm.gameState = gs
	lm.mu.Unlock()
}

// HandCompleteExpiresAtForTest returns the session's fixed score-reveal
// auto-continue deadline (zero if unset). Exposed so tests can assert that a
// player's continue acknowledgement never pushes the deadline back.
func (m *Manager) HandCompleteExpiresAtForTest(roomID uint) time.Time {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return time.Time{}
	}
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.handCompleteExpiresAt
}

// SetHandCompleteExpiresAtForTest seeds the score-reveal auto-continue deadline,
// simulating a pause that has already started (in real flow the deadline is set
// when the hand-complete transition first occurs).
func (m *Manager) SetHandCompleteExpiresAtForTest(roomID uint, t time.Time) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	lm.mu.Lock()
	lm.handCompleteExpiresAt = t
	lm.mu.Unlock()
}

// DeclarationExpiresAtForTest returns the session's fixed declaration-phase
// deadline (zero if unset). Exposed so tests can assert that one seat's answer
// never pushes the window back for the others.
func (m *Manager) DeclarationExpiresAtForTest(roomID uint) time.Time {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return time.Time{}
	}
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.declarationExpiresAt
}

// SetDeclarationExpiresAtForTest seeds the declaration-phase deadline,
// simulating a phase that has already been entered (in real flow the deadline is
// set when the transition into PhaseDeclaring first occurs).
func (m *Manager) SetDeclarationExpiresAtForTest(roomID uint, t time.Time) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	lm.mu.Lock()
	lm.declarationExpiresAt = t
	lm.mu.Unlock()
}

// TriggerDeclarationTimeoutForTest cancels any pending timer and re-arms a
// short-duration one against handleDeclarationTimeout, then returns.
func (m *Manager) TriggerDeclarationTimeoutForTest(roomID uint, fireAfter time.Duration) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	lm.mu.Lock()
	lm.cancelTurnTimer()
	gen := lm.timerGeneration
	lm.turnTimer = time.AfterFunc(fireAfter, func() {
		m.handleDeclarationTimeout(lm, gen)
	})
	lm.mu.Unlock()
}

// TriggerTimerExpiryForTest cancels any pending turn timer and re-arms a
// short-duration timer for expectedSeat, then waits for it to fire.
func (m *Manager) TriggerTimerExpiryForTest(roomID uint, expectedSeat int, fireAfter time.Duration) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	lm.mu.Lock()
	lm.cancelTurnTimer()
	gen := lm.timerGeneration
	lm.turnTimer = time.AfterFunc(fireAfter, func() {
		m.handleTimerExpiry(lm, gen, expectedSeat)
	})
	lm.mu.Unlock()
}

// SetBotSearchForTest enables or disables the bot card search with the given settings.
func (m *Manager) SetBotSearchForTest(s BotSearchSettings) {
	m.SetBotSearch(s)
}

// BotSearchResultForTest returns a copy of the search result stored for the seat, or nil when there is none.
func (m *Manager) BotSearchResultForTest(roomID uint, seat int) *bot.SearchResult {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return nil
	}
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	entry := lm.botSearchResults[seat]
	if entry == nil {
		return nil
	}
	result := entry.result
	return &result
}

// StoreBotSearchResultForTest stores a search result for the seat under the given generation and the seat's current
// decision context, as the search job would.
func (m *Manager) StoreBotSearchResultForTest(roomID uint, seat int, generation uint64, result bot.SearchResult) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.botSearchResults[seat] = &botSearchEntry{
		generation: generation,
		ctx:        botDecisionContextFor(lm.gameState, seat),
		result:     result,
	}
}

// BuildBotViewForTest returns the view the driver would hand the bot at seat right now.
func (m *Manager) BuildBotViewForTest(roomID uint, seat int) bot.View {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return bot.View{}
	}
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return buildBotView(lm.gameState, seat, lm.botMemory)
}

// CancelBotTimersForTest stops every armed bot think delay of the room and invalidates its generation, so a test
// that injected a state decides when the driver arms the next one.
func (m *Manager) CancelBotTimersForTest(roomID uint) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	lm.mu.Lock()
	lm.cancelAllBotActionTimers()
	lm.mu.Unlock()
}

// RecordBotPlaysForTest adds the cards to the bot memory as already played this hand, so a state injected mid-hand
// has the history its hands imply.
func (m *Manager) RecordBotPlaysForTest(roomID uint, cards []game.Card) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	lm.mu.Lock()
	defer lm.mu.Unlock()
	if lm.botMemory == nil {
		return
	}
	for _, c := range cards {
		lm.botMemory.ObservePlay(0, c, nil, lm.gameState.TrumpSuit)
	}
}

var errNoSession = errors.New("match: no live session for room")
