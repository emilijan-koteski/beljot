package match

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/ws"
)

// LiveMatch holds the live game state and player mapping for one active game.
type LiveMatch struct {
	gameState        *game.GameState
	playerIDs        [4]uint // index = seat
	roomID           uint
	startedAt        time.Time
	timerStyle       string      // "relaxed" or "per-move"
	timerDurationSec int         // seconds per move (only used when timerStyle == "per-move")
	turnTimer        *time.Timer // current per-move timer (nil when inactive)
	timerGeneration  uint64      // incremented on each turn; timer callback checks staleness
	// handCompleteExpiresAt is the fixed deadline for the score-reveal pause's
	// auto-continue. Set once when the pause STARTS; the timer is re-created
	// against this same deadline (remaining time) on every action so a player's
	// acknowledgement never pushes it back. Reset on reconnect into the pause.
	handCompleteExpiresAt    time.Time
	reconnectWindowSec       int            // seconds to wait for disconnected player (default 120)
	seatReconnectTimers      [4]*time.Timer // per-seat reconnect countdown timers; nil when seat is online
	seatReconnectGenerations [4]uint64      // per-seat staleness counters — bumped on cancel + start
	closed                   bool           // set when session is being removed
	// handResults buffers per-hand scoring rows during the match. Flushed via
	// matchRepo.CreateWithHands when the match completes (normal end) or is
	// abandoned. Mutated only under session.mu.Lock.
	handResults []HandResult
	mu          sync.RWMutex
}

// RoomStatusUpdater updates a room's status in the database.
type RoomStatusUpdater interface {
	UpdateRoomStatus(roomID uint, status string) error
}

// Broadcaster is the subset of *ws.Hub the manager depends on. Mirrors the
// chat / emote pattern (chat/handler.go, emote/handler.go) so tests can swap
// in a hubSpy without spinning up a real hub. *ws.Hub satisfies this directly.
type Broadcaster interface {
	BroadcastToUsers(userIDs []uint, msg []byte)
	SendToUser(userID uint, msg []byte)
}

// Manager orchestrates game sessions: receives actions via WebSocket,
// calls the rules engine, broadcasts results, and persists completed matches.
type Manager struct {
	sessions         map[uint]*LiveMatch // keyed by roomID
	userToRoom       map[uint]uint       // userID → roomID for quick lookup
	hub              Broadcaster
	matchRepo        MatchRepository
	roomUpdater      RoomStatusUpdater
	userRemovedHooks []func(userID uint)
	mu               sync.RWMutex
}

// NewManager creates a session manager wired to the WebSocket hub and match repository.
func NewManager(hub Broadcaster, matchRepo MatchRepository) *Manager {
	return &Manager{
		sessions:   make(map[uint]*LiveMatch),
		userToRoom: make(map[uint]uint),
		hub:        hub,
		matchRepo:  matchRepo,
	}
}

// SetRoomUpdater sets the interface for updating room status on match completion.
func (m *Manager) SetRoomUpdater(updater RoomStatusUpdater) {
	m.roomUpdater = updater
}

// AddUserRemovedHook registers fn to be called (outside the manager lock)
// for each playerID when RemoveSession tears down a session.
// Reusable for Epic 9 per-user state (wallet rate-limit, daily-claim cooldown).
func (m *Manager) AddUserRemovedHook(fn func(userID uint)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.userRemovedHooks = append(m.userRemovedHooks, fn)
}

// StartMatch creates a new game session from room data and broadcasts the initial state.
func (m *Manager) StartMatch(roomID uint, variant string, matchMode string, players [4]PlayerSeatInfo, timerStyle string, timerDurationSec int, ownerID uint, reconnectWindowSec int) error {
	m.mu.Lock()
	if _, exists := m.sessions[roomID]; exists {
		m.mu.Unlock()
		return fmt.Errorf("session already exists for room %d", roomID)
	}

	var playerIDs [4]uint
	var usernames [4]string
	for _, p := range players {
		playerIDs[p.Seat] = p.UserID
		usernames[p.Seat] = p.Username
	}

	gs := game.NewGame(playerIDs, usernames, game.Variant(variant), matchMode, roomID)

	// Map room owner to seat index for pause override validation.
	// Default to -1 (no owner override available) if ownerID not found among players.
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
		timerStyle:         timerStyle,
		timerDurationSec:   timerDurationSec,
		reconnectWindowSec: reconnectWindowSec,
	}

	m.sessions[roomID] = session
	for _, uid := range playerIDs {
		m.userToRoom[uid] = roomID
	}
	m.mu.Unlock()

	slog.Info("session: game started", "roomID", roomID, "players", playerIDs)

	// Broadcast dealing-phase state (client shows deal animation)
	m.hub.BroadcastToUsers(playerIDs[:], buildMessage(ws.EventMatchState, gs))

	// Auto-transition to bidding phase (client's DealAnimation handles visual timing)
	if gs.Phase == game.PhaseDealing {
		session.mu.Lock()
		gs.Phase = game.PhaseBidding
		m.setTurnExpiry(session, gs)
		m.startTimerLocked(session, gs.ActivePlayerSeat)
		session.mu.Unlock()
		m.hub.BroadcastToUsers(playerIDs[:], buildMessage(ws.EventMatchState, gs))
	}

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

	// Parse the action from the WS message
	action, err := m.parseAction(client.UserID, session, msg)
	if err != nil {
		m.sendError(client.UserID, ws.ErrorInvalidAction, err.Error())
		return
	}

	// Lock the session for state mutation — cancel timer first to prevent race
	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		return
	}

	// A continue that arrives after the score-reveal pause already advanced is
	// a benign race, not a player error: the client's 8s auto-ack countdown
	// starts when its dialog MOUNTS — gated behind the trick-collect sweep and
	// the capot banner — so it can trail the server's force-advance. Drop it
	// silently BEFORE cancelTurnTimer so the new turn's timer is untouched,
	// and send no error (the "cannot perform this action" toast was pure noise
	// the player could do nothing about).
	if action.Type == game.ActionContinue && session.gameState.Phase != game.PhaseHandComplete {
		session.mu.Unlock()
		return
	}

	session.cancelTurnTimer()

	oldState := session.gameState
	newState, err := game.ApplyAction(oldState, action)
	if err != nil {
		// The pre-mutation cancelTurnTimer above also stops the score-reveal
		// auto-continue fallback when the table is paused in hand_complete —
		// and the per-move restore branch below can never re-arm it, because
		// TurnExpiresAt is always nil during the pause. Re-create it here
		// against the SAME fixed deadline, or one rejected action (a stray tap
		// that sends play_card while the collect animation still covers the
		// table, a pause click, …) would permanently disarm the fallback and a
		// connected-but-unresponsive seat could strand the table forever.
		// Applies regardless of timer style, mirroring the success branch.
		//
		// Otherwise restart the timer we cancelled preemptively above,
		// restoring the ORIGINAL deadline rather than minting a fresh full
		// window. Two reasons:
		//   1. We don't broadcast state on error, so all four clients still
		//      hold the original TurnExpiresAt. A fresh full window would
		//      silently desync server↔clients: the UI ticks to 0 while the
		//      server quietly waits another full duration before auto-acting.
		//      Players see "0s" sit on the screen for several seconds before
		//      the card finally auto-throws.
		//   2. Fairness: any player (not just the active one) can hit this
		//      branch by clicking an illegal card / sending a stale action.
		//      Resetting to a fresh window would let any client extend the
		//      active player's turn indefinitely via spam.
		if oldState.Phase == game.PhaseHandComplete {
			remaining := max(time.Until(session.handCompleteExpiresAt), 0)
			gen := session.timerGeneration
			session.turnTimer = time.AfterFunc(remaining, func() {
				m.handleHandCompleteTimeout(session, gen)
			})
		} else if oldState.Phase != game.PhasePaused && session.timerStyle == "per-move" && oldState.TurnExpiresAt != nil {
			// Clamp to ≥ 0 so an already-expired deadline fires the auto-action
			// immediately rather than tripping AfterFunc's negative-duration path.
			remaining := max(time.Until(*oldState.TurnExpiresAt), 0)
			gen := session.timerGeneration
			expectedSeat := oldState.ActivePlayerSeat
			session.turnTimer = time.AfterFunc(remaining, func() {
				m.handleTimerExpiry(session, gen, expectedSeat)
			})
		}
		session.mu.Unlock()
		m.sendGameError(client.UserID, err)
		return
	}

	// Handle dealing→bidding auto-transition inside the lock (prevents data race)
	if newState.Phase == game.PhaseDealing {
		newState.Phase = game.PhaseBidding
	}

	// Timer management for pause/unpause
	if action.Type == game.ActionPause && newState.Phase == game.PhasePaused {
		// Capture remaining timer time before discarding it
		if oldState.TurnExpiresAt != nil {
			remaining := time.Until(*oldState.TurnExpiresAt)
			if remaining > 0 {
				newState.TurnTimeRemaining = remaining.Milliseconds()
			}
		}
		newState.TurnExpiresAt = nil
		// Do NOT start timer — game is paused
	} else if (action.Type == game.ActionUnpause || action.Type == game.ActionOwnerUnpause) && newState.Phase != game.PhasePaused {
		// Game resumed — restore timer from preserved remaining time
		if session.timerStyle == "per-move" {
			// Enforce a minimum floor of 3 seconds to give the player reaction time after unpause
			const minResumeMs int64 = 3000
			remaining := time.Duration(newState.TurnTimeRemaining) * time.Millisecond
			if newState.TurnTimeRemaining > 0 && newState.TurnTimeRemaining < minResumeMs {
				remaining = time.Duration(minResumeMs) * time.Millisecond
			} else if newState.TurnTimeRemaining <= 0 {
				// Timer had expired or was not active — give minimum floor, not a full reset
				remaining = time.Duration(minResumeMs) * time.Millisecond
			}
			expiry := time.Now().Add(remaining)
			newState.TurnExpiresAt = &expiry
			newState.TurnTimeRemaining = 0
			// Start timer with remaining duration. cancelTurnTimer bumps
			// timerGeneration so the captured gen is the post-cancel value.
			session.cancelTurnTimer()
			gen := session.timerGeneration
			expectedSeat := newState.ActivePlayerSeat
			session.turnTimer = time.AfterFunc(remaining, func() {
				m.handleTimerExpiry(session, gen, expectedSeat)
			})
		} else {
			newState.TurnTimeRemaining = 0
		}
	} else if newState.Phase == game.PhasePlaying || newState.Phase == game.PhaseBidding {
		// Within-turn predicate: when seat and phase are unchanged, the action
		// resolved a prompt (declare/skip_declare, play_card-into-belot,
		// surrender request/decline) without advancing the turn. The original
		// turn deadline must persist so prompts cost the active player the same
		// budget as a normal play. Otherwise (seat advanced or phase changed)
		// the turn truly transitioned — issue a fresh expiry for the next seat.
		preserveTimer := session.timerStyle == "per-move" &&
			oldState.TurnExpiresAt != nil &&
			newState.ActivePlayerSeat == oldState.ActivePlayerSeat &&
			newState.Phase == oldState.Phase
		if preserveTimer {
			newState.TurnExpiresAt = oldState.TurnExpiresAt
			newState.TimerDurationSec = oldState.TimerDurationSec
			remaining := time.Until(*oldState.TurnExpiresAt)
			if remaining > 0 {
				// HandleAction's pre-mutation cancelTurnTimer (above) already
				// bumped timerGeneration; the captured gen is the post-cancel
				// value. Any future cancel/restart will invalidate it.
				gen := session.timerGeneration
				expectedSeat := newState.ActivePlayerSeat
				session.turnTimer = time.AfterFunc(remaining, func() {
					m.handleTimerExpiry(session, gen, expectedSeat)
				})
			}
		} else {
			// Turn advanced or phase changed — fresh window for the next seat.
			// Pass newState.ActivePlayerSeat explicitly: session.gameState is
			// reassigned to newState only after this block (see below), so a
			// state-less startTimerLocked would capture the OLD active seat
			// and the timer would auto-act for the wrong player.
			m.setTurnExpiry(session, newState)
			m.startTimerLocked(session, newState.ActivePlayerSeat)
		}
	} else if newState.Phase == game.PhaseHandComplete {
		// Hand-complete pause: no active turn. Arm the auto-continue safety net so
		// a connected-but-idle player can't stall the table; players advance early
		// via action:continue. Applies regardless of timer style.
		//
		// The deadline is fixed when the pause STARTS and is NOT pushed back by
		// each acknowledgement — otherwise the present players' continue clicks
		// would keep extending the wait for a missing/idle seat. cancelTurnTimer()
		// at the top of HandleAction already stopped the prior tick, so we always
		// re-create the timer here, but against the SAME deadline (remaining time)
		// rather than a fresh full window.
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
	// Capture immutable values for use after unlock
	playerIDs := session.playerIDs
	startedAt := session.startedAt
	session.mu.Unlock()

	// Broadcast the result using captured local variables (not session.gameState)
	m.broadcastActionResult(playerIDs, oldState, newState, action, false)

	// Buffer per-hand scoring for persistence at match end
	m.bufferHandResultIfScored(session, oldState, newState)

	// Check for match completion
	if newState.Phase == game.PhaseMatchEnd {
		// Story 8.2: when accepting surrender, the proposer seat lives on
		// oldState (newState clears it). Resolve to userID for persistence.
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
	}
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
		session.mu.Unlock()
		for _, uid := range session.playerIDs {
			delete(m.userToRoom, uid)
			removedIDs = append(removedIDs, uid)
		}
		delete(m.sessions, roomID)
		slog.Info("session: removed", "roomID", roomID)
	}
	hooks := append([]func(userID uint){}, m.userRemovedHooks...) // snapshot under lock
	m.mu.Unlock()

	// Call hooks after releasing the manager lock to prevent deadlocks.
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

// InMatchUserIDs returns a snapshot of every userID currently mapped to an
// active game session. Used by the lobby stats endpoint to bucket connected
// users into "in game" vs other states. Order is unspecified.
func (m *Manager) InMatchUserIDs() []uint {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]uint, 0, len(m.userToRoom))
	for uid := range m.userToRoom {
		ids = append(ids, uid)
	}
	return ids
}

// MatchParticipants returns the four player userIDs for an active session
// keyed by roomID (the matchID in the chat wire format). Returns
// (zero-value, false) when no session exists for that roomID.
// Used by the chat handler to authorise match-scoped messages.
func (m *Manager) MatchParticipants(roomID uint) ([4]uint, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[roomID]
	if !ok {
		return [4]uint{}, false
	}
	return s.playerIDs, true
}

// MatchParticipantsByUser resolves an active session via the sender's userID
// and returns the four participants alongside the sender's seat index. Used
// by the emote handler, which receives no matchID in its request payload —
// looking up the session through userToRoom keeps the caller decoupled from
// the manager's internal indexing.
// Returns ([4]uint{}, -1, false) when the user is not in any active session.
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
	// Find seat for this user (playerIDs is immutable after StartMatch)
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

	// Extract action type from the WS event type (strip "action:" prefix)
	actionType := ""
	if len(msg.Type) > 7 && msg.Type[:7] == "action:" {
		actionType = msg.Type[7:]
	}
	if actionType == "" {
		return game.Action{}, fmt.Errorf("invalid action type: %s", msg.Type)
	}

	// Map WS event names to rules engine action types where they differ
	if actionType == "decline_belot" {
		actionType = game.ActionSkipBelot
	}

	action := game.Action{
		Type:       actionType,
		PlayerSeat: seat,
	}

	// Parse card from payload for play_card action
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

	// Parse suit from payload for pick_trump action (round 2 requires suit selection)
	if actionType == game.ActionPickTrump {
		var payload struct {
			Suit string `json:"suit"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err == nil && payload.Suit != "" {
			suit := game.Suit(payload.Suit)
			action.Suit = &suit
		}
		// If no suit provided, it's round 1 (picking the trump candidate) — action.Suit stays nil
	}

	return action, nil
}

// broadcastActionResult sends the appropriate event(s) after a successful action.
// All parameters are local values — no session.gameState reads (avoids data races).
// autoPlayed indicates whether the card was played by the timer auto-play system.
//
// Story 8.5-1 AC4: match-end broadcasts (event:match_end and the trailing
// event:match_state) are emitted by handleMatchEnd AFTER persistence; this
// function returns early in those branches.
// trickResolvedWinnerSeat resolves the seat that won the trick that just
// completed, for the event:trick_resolved payload. The winner lives in a
// different field depending on what else the same ApplyAction did:
//
//   - Tricks 1-7: resolveTrick cleared TrickWinnerSeat but set
//     ActivePlayerSeat = winner ("winner leads next trick", playing.go).
//   - Last trick of a NON-final hand: the same ApplyAction chained
//     scoreHand -> startNewHand, which cleared TrickWinnerSeat AND advanced
//     ActivePlayerSeat to the NEXT hand's first bidder. The real winner is
//     preserved in LastHandResult.LastTrickSeat (HandNumber incremented).
//   - Last trick of the FINAL hand (match end): scoreHand returns before
//     startNewHand, so TrickWinnerSeat is still set.
//
// The previous code fell back to ActivePlayerSeat whenever TrickWinnerSeat was
// nil, so on every hand's last trick it sent the next hand's bidder as the
// winner — the collect animation swept to the wrong seat even though scoring
// (which uses the true winner internally) was correct.
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

func (m *Manager) broadcastActionResult(playerIDs [4]uint, oldState, newState *game.GameState, action game.Action, autoPlayed bool) {
	userIDs := playerIDs[:]

	switch action.Type {
	case game.ActionPlayCard:
		// Broadcast card played
		cardPlayed := map[string]interface{}{
			"playerSeat": action.PlayerSeat,
			"cardId":     action.Card.String(),
			"autoPlayed": autoPlayed,
		}
		m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventCardPlayed, cardPlayed))

		// Check if trick completed: oldState had cards in the trick, newState cleared them
		// or hand/match transitioned (trick number wrapped or phase changed)
		trickCompleted := len(oldState.CurrentTrick) == 3 && // 4th card was just played (3 in old + this one)
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

		// In Bitola, declarations resolve at end of trick 1 inside
		// resolveTrickWithDeclarations, which runs under ActionPlayCard — not
		// under ActionDeclare. Broadcast the reveal event here so the client's
		// DeclarationReveal fires at start of trick 2. Must precede the
		// authoritative event:match_state below so the reveal handler sets
		// declarationReveal before any follow-up state logic runs.
		m.broadcastDeclarationsResolvedIfTransition(oldState, newState, userIDs)

		// Check if the hand was just scored. scoreHand now holds in
		// PhaseHandComplete (next hand deals on action:continue) instead of dealing
		// immediately, so detect the phase transition into PhaseHandComplete (or
		// PhaseMatchEnd) rather than a HandNumber increment.
		handJustScored := newState.LastHandResult != nil &&
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

		// Story 8.5-1 AC4: when phase is match_end, the event:match_end and the
		// trailing event:match_state are emitted by handleMatchEnd AFTER the
		// match is persisted. The persist-before-broadcast invariant guarantees
		// a client that receives match_end and immediately reads the match row
		// will find it. handleMatchEnd preserves the (match_end → match_state)
		// client-facing order so MatchPage's stale-state redirect does not race
		// the matchEndData arrival.
		if newState.Phase == game.PhaseMatchEnd {
			return
		}

		// Follow with authoritative state so clients advance activePlayerSeat,
		// remove the played card from hand, refresh turn timer, and pick up
		// awaitingDeclaration / pendingBelotSeat flags. event:card_played only
		// mutates the trick on the client — without this the next player never
		// learns it's their turn.
		m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventMatchState, newState))

	case game.ActionPickTrump:
		if newState.TrumpSuit != nil {
			// oldState.TrumpCandidate is the face-up card the picker took;
			// ApplyAction clears it on newState, so source from oldState.
			// handlePickTrump rejects the action with ErrWrongPhase when
			// TrumpCandidate is nil, so the nil branch here is a defensive
			// guard for future code paths — log loudly rather than emit a
			// payload the client will silently drop on its length-2 guard.
			if oldState.TrumpCandidate == nil {
				slog.Warn("session: trump_selected suppressed; oldState.TrumpCandidate is nil",
					"playerSeat", action.PlayerSeat)
			} else {
				trumpSelected := ws.TrumpSelectedPayload{
					PlayerSeat: action.PlayerSeat,
					TrumpSuit:  string(*newState.TrumpSuit),
					CardID:     oldState.TrumpCandidate.String(),
				}
				m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventTrumpSelected, trumpSelected))
			}
		}
		// Always follow with full state so clients leave the bidding UI and
		// sync activePlayerSeat/phase/trumpCallerSeat. Without this, a
		// successful pick leaves the client stuck on TrumpPrompt and every
		// subsequent click returns error:wrong_phase.
		m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventMatchState, newState))

	case game.ActionPassTrump:
		m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventMatchState, newState))

	case game.ActionDeclare, game.ActionSkipDeclare:
		// Announce WHO declared the moment the declare commits, so the table
		// learns a declaration exists during trick 1 (seat only — the melds
		// themselves stay secret until event:declarations_resolved). Timer
		// expiry only ever auto-SKIPS, so this fires for manual declares only.
		if action.Type == game.ActionDeclare {
			declared := ws.PlayerDeclaredPayload{PlayerSeat: action.PlayerSeat}
			m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventPlayerDeclared, declared))
		}
		// In Bitola, DeclarationsResolved cannot actually flip here — declarations
		// resolve only at end of trick 1 (see ActionPlayCard branch). The helper
		// is still called so future variants (e.g. Croatian, where declarations
		// resolve during a dedicated phase) don't silently regress.
		m.broadcastDeclarationsResolvedIfTransition(oldState, newState, userIDs)
		// Always follow with full state so the client clears awaitingDeclaration,
		// advances activePlayerSeat, and picks up declarationsResolved. The
		// event:declarations_resolved handler is reveal-only and does not sync state.
		m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventMatchState, newState))

	case game.ActionAnnounceBelot, game.ActionSkipBelot:
		if newState.BelotAnnounced && !oldState.BelotAnnounced {
			// The triggering card is the K/Q last appended to the trick before the
			// Belot prompt paused post-play flow. It remains in oldState.CurrentTrick.
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
		// If this Belot action completed a deferred trick — the triggering K/Q was
		// the 4th card, so handlePlayCard set PendingBelotSeat and returned before
		// resolving (playing.go) — emit the event:trick_resolved the deferred
		// play_card could not. Without it the client never receives the trick
		// winner, so pendingResolvedTrick is never set and the just-completed trick
		// gets no collect animation. A Belot prompt can only fire on tricks 1-7 (a
		// player cannot hold both trump K and Q at trick 8), so no hand_scored
		// follows here; trick 1 resolving this way still reveals declarations.
		if len(oldState.CurrentTrick) == 4 {
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
		// Always follow with full state so the client clears pendingBelotSeat,
		// advances activePlayerSeat, and resolves the trick if the belot action
		// came after the 4th card. event:belot_announced is informational only.
		m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventMatchState, newState))

	case game.ActionContinue:
		// Acknowledging the hand-complete pause. If this continue dealt the next
		// hand (all connected players ready) the state is now bidding/dealing; if
		// it merely recorded one player's readiness it is still PhaseHandComplete.
		// Either way the authoritative state syncs clients. An instant-win on the
		// freshly dealt hand is handled by handleMatchEnd (called from HandleAction).
		if newState.Phase == game.PhaseMatchEnd {
			return
		}
		m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventMatchState, newState))

	case game.ActionPause:
		paused := ws.MatchPausedPayload{
			PausedBy:      action.PlayerSeat,
			PausedPlayers: newState.PausedPlayers,
		}
		m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventMatchPaused, paused))
		m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventMatchState, newState))

	case game.ActionSurrenderRequest:
		// Story 8.2 — broadcast typed proposed payload, then authoritative
		// state so opponents/proposer pick up surrenderProposerSeat. Per
		// project-context: separate ordered messages, never batched.
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
		m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventMatchState, newState))

	case game.ActionSurrenderDecline:
		// Proposer is no longer in newState (cleared on decline) — read it
		// from oldState. Per FR28a the proposer's attempt remains consumed.
		// Defensive: rules engine rejects decline when no proposal is pending,
		// so a nil proposer here is unreachable; suppress the broadcast and
		// rely on the authoritative state event rather than ship a malformed
		// payload (proposerSeat=-1) over the wire.
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
		m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventMatchState, newState))

	case game.ActionSurrenderAccept:
		// Story 8.5-1 AC4: match-end transition emits event:match_end AND the
		// trailing event:match_state from handleMatchEnd, AFTER persistence.
		// The (match_end → match_state) client-facing order is preserved.
		if newState.Phase == game.PhaseMatchEnd {
			return
		}
		// Defensive fallback for the (currently unreachable) case where surrender
		// accept does not reach match_end — emit authoritative state so clients
		// pick up the cleared SurrenderProposerSeat.
		m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventMatchState, newState))

	case game.ActionUnpause, game.ActionOwnerUnpause:
		resumed := ws.MatchResumedPayload{
			ResumedBy:     action.PlayerSeat,
			OwnerOverride: action.Type == game.ActionOwnerUnpause,
		}
		// Only send resumed event if game actually left paused phase
		if newState.Phase != game.PhasePaused {
			m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventMatchResumed, resumed))
		}
		m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventMatchState, newState))

	default:
		// For any other action, broadcast full state
		m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventMatchState, newState))
	}
}

// broadcastDeclarationsResolvedIfTransition fires event:declarations_resolved
// exactly once, when DeclarationsResolved flips from false to true. Losing-team
// declarations have already been cleared by resolveDeclarationsForHand at the
// game layer, so iterating every player yields only the winning team's.
func (m *Manager) broadcastDeclarationsResolvedIfTransition(oldState, newState *game.GameState, userIDs []uint) {
	if !newState.DeclarationsResolved || oldState.DeclarationsResolved {
		return
	}

	decls := make([]map[string]interface{}, 0)
	for _, p := range newState.Players {
		for _, d := range p.Declarations {
			decls = append(decls, map[string]interface{}{
				"playerSeat": d.PlayerSeat,
				"type":       string(d.Type),
				"value":      d.Value,
				"cards":      cardsToIDs(d.Cards),
			})
		}
	}

	var declWinnerTeam interface{} = nil
	if newState.DeclarationPoints[game.TeamA] > 0 {
		declWinnerTeam = game.TeamA
	} else if newState.DeclarationPoints[game.TeamB] > 0 {
		declWinnerTeam = game.TeamB
	}

	payload := map[string]interface{}{
		"winnerTeam":   declWinnerTeam,
		"declarations": decls,
	}
	m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventDeclarationsResolved, payload))
}

// bufferHandResultIfScored appends a HandResult to session.handResults
// when the provided state transition represents a scored hand (hand number
// advanced OR match ended) AND LastHandResult is populated. The completed
// hand's number is oldState.HandNumber — on normal hand completion newState
// has already been advanced by startNewHand, and on match-end startNewHand
// does not run so oldState.HandNumber still identifies the final hand. Safe
// to call on every state transition; no-op when the condition is not met.
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
		HandNumber:      oldState.HandNumber,
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
	session.handResults = append(session.handResults, row)
	session.mu.Unlock()
}

// handleMatchEnd persists the match record, updates room status, broadcasts
// event:match_end and the trailing event:match_state, and removes the session.
// Uses the passed finalState (not session.gameState) to avoid data races.
// surrenderedBy is the userID of the player who initiated the accepted surrender,
// or nil for natural match-end. The match Status stays "completed" in both cases —
// the column is the load-bearing signal Story 9.6 (honor system) will consume.
//
// Story 8.5-1 AC4 ordering contract:
//
//  1. Persist matchRecord (CreateWithHands) and update room status FIRST so a
//     client that receives event:match_end and immediately reads the match row
//     will find it.
//  2. event:match_end ALWAYS fires — even if persistence failed — so the four
//     participants are not stranded on the table forever. Persist failures
//     are logged via slog.Error but do not block the broadcast.
//  3. event:match_state follows event:match_end so the (match_end → match_state)
//     client-facing order is preserved (MatchPage redirects to /lobby when it
//     observes phase=="match_end" with matchEndData==null; sending match_state
//     first would race that redirect against matchEndData arrival).
func (m *Manager) handleMatchEnd(session *LiveMatch, finalState *game.GameState, surrenderedBy *uint, matchEndPayload ws.MatchEndPayload) {
	winnerTeam := 0
	if finalState.WinnerTeam != nil {
		winnerTeam = *finalState.WinnerTeam
	}

	matchRecord := &Match{
		RoomID:        session.roomID,
		Player1ID:     session.playerIDs[0],
		Player2ID:     session.playerIDs[1],
		Player3ID:     session.playerIDs[2],
		Player4ID:     session.playerIDs[3],
		TeamAScore:    finalState.TeamScores[game.TeamA],
		TeamBScore:    finalState.TeamScores[game.TeamB],
		WinnerTeam:    winnerTeam,
		Variant:       string(finalState.Variant),
		MatchMode:     finalState.MatchMode,
		StartedAt:     session.startedAt,
		CompletedAt:   time.Now(),
		Status:        "completed",
		SurrenderedBy: surrenderedBy,
	}

	// Copy buffered hand results under RLock to avoid holding the lock during I/O.
	session.mu.RLock()
	handsCopy := make([]HandResult, len(session.handResults))
	copy(handsCopy, session.handResults)
	session.mu.RUnlock()

	if err := m.matchRepo.CreateWithHands(matchRecord, handsCopy); err != nil {
		slog.Error("session: failed to persist match", "roomID", session.roomID, "error", err)
	} else {
		slog.Info("session: match persisted", "roomID", session.roomID, "matchID", matchRecord.ID, "hands", len(handsCopy))
	}

	// Update room status to completed
	if m.roomUpdater != nil {
		if err := m.roomUpdater.UpdateRoomStatus(session.roomID, "completed"); err != nil {
			slog.Error("session: failed to update room status", "roomID", session.roomID, "error", err)
		}
	}

	// Broadcast match_end → match_state AFTER persistence completes. Both fire
	// regardless of persist outcome (clients must not be stranded on the table).
	userIDs := session.playerIDs[:]
	m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventMatchEnd, matchEndPayload))
	m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventMatchState, finalState))

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
	}
	m.sendError(userID, eventType, err.Error())
}

// buildMessage creates a JSON-encoded WS message.
func buildMessage(eventType string, payload interface{}) []byte {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("session: failed to marshal payload", "eventType", eventType, "error", err)
		payloadBytes = []byte(`{}`)
	}
	msg, err := json.Marshal(ws.WSMessage{
		Type:    eventType,
		Payload: payloadBytes,
	})
	if err != nil {
		slog.Error("session: failed to marshal message", "eventType", eventType, "error", err)
		return nil
	}
	return msg
}

// setTurnExpiry sets TurnExpiresAt on the game state based on timer config.
// For "per-move" style, sets an absolute expiry timestamp. For "relaxed", sets nil.
// Must be called under session.mu.Lock().
func (m *Manager) setTurnExpiry(session *LiveMatch, gs *game.GameState) {
	if session.timerStyle == "per-move" && session.timerDurationSec > 0 {
		expiry := time.Now().Add(time.Duration(session.timerDurationSec) * time.Second)
		gs.TurnExpiresAt = &expiry
		gs.TimerDurationSec = session.timerDurationSec
	} else {
		gs.TurnExpiresAt = nil
		gs.TimerDurationSec = 0
	}
}

// startTimerLocked starts the per-move turn timer for the current session.
// Must be called under session.mu.Lock(). The timer callback will acquire
// session.mu.Lock() when it fires (safe — fires in a separate goroutine later).
//
// expectedSeat is passed explicitly because callers commonly invoke this BEFORE
// session.gameState has been reassigned to the post-action state. Reading
// session.gameState here would capture the pre-action seat and the timer would
// fire for the wrong player.
func (m *Manager) startTimerLocked(session *LiveMatch, expectedSeat int) {
	if session.timerStyle != "per-move" || session.timerDurationSec <= 0 {
		return
	}
	// cancelTurnTimer bumps timerGeneration; the captured gen is the
	// post-cancel value, invalidated by any subsequent cancel/restart.
	session.cancelTurnTimer()
	gen := session.timerGeneration

	duration := time.Duration(session.timerDurationSec) * time.Second
	session.turnTimer = time.AfterFunc(duration, func() {
		m.handleTimerExpiry(session, gen, expectedSeat)
	})
}

// handCompleteAutoContinue is the server's fallback ceiling on the score-reveal
// pause before auto-dealing the next hand when not every connected player has
// acknowledged. Each client auto-acknowledges 8s after its dialog MOUNTS
// (SCORE_REVEAL_AUTO_CONTINUE on the client) — and the mount trails this
// deadline's start by the trick-collect sweep (~2s) plus, on capot hands, the
// capot banner (~2.5s). The ceiling must therefore cover 8s + ~4.5s of reveal
// pacing + network grace, or the auto-acks land after the force-advance and
// every slow hand ends in a wrong-phase race (observed as "cannot perform this
// action" toasts at 10s). 14s keeps the normal ack-driven path comfortably
// ahead while still capping how long one stuck seat can hold the table — and
// is measured from when the pause STARTS, never extended by later
// acknowledgements (see handCompleteExpiresAt).
const handCompleteAutoContinue = 14 * time.Second

// handleHandCompleteTimeout force-advances the hand-complete pause when the
// auto-continue window elapses without every connected player acknowledging.
// Mirrors the continue-action advance path (deal next hand, arm the per-move
// timer, persist the finished hand, broadcast) but bypasses ApplyAction since
// it is a server-initiated advance, not a player action.
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
	}
	if newState.Phase == game.PhasePlaying || newState.Phase == game.PhaseBidding {
		m.setTurnExpiry(session, newState)
		m.startTimerLocked(session, newState.ActivePlayerSeat)
	}
	session.gameState = newState
	playerIDs := session.playerIDs
	startedAt := session.startedAt
	session.mu.Unlock()

	m.bufferHandResultIfScored(session, oldState, newState)

	// An instant-win on the freshly dealt hand ends the match here.
	if newState.Phase == game.PhaseMatchEnd {
		matchEndPayload := buildMatchEndPayload(oldState, newState, game.Action{Type: game.ActionContinue}, startedAt)
		m.handleMatchEnd(session, newState, nil, matchEndPayload)
		return
	}
	m.hub.BroadcastToUsers(playerIDs[:], buildMessage(ws.EventMatchState, newState))
}

// handleTimerExpiry is called when a per-move timer fires. It auto-plays for the
// active player and broadcasts the result. The generation counter prevents stale
// timer callbacks from acting on the wrong turn.
func (m *Manager) handleTimerExpiry(session *LiveMatch, generation uint64, expectedSeat int) {
	session.mu.Lock()

	// Guard: session closed or timer is stale (turn already advanced)
	if session.closed || session.timerGeneration != generation {
		session.mu.Unlock()
		return
	}

	gs := session.gameState

	// Determine what action to auto-take based on game phase and state
	var action game.Action
	switch {
	case gs.Phase == game.PhaseBidding:
		// Auto-pass trump on bidding timeout
		action = game.Action{
			Type:       game.ActionPassTrump,
			PlayerSeat: expectedSeat,
		}
	case gs.Phase == game.PhasePlaying && gs.AwaitingDeclaration:
		// Auto-skip declaration on timer expiry
		action = game.Action{
			Type:       game.ActionSkipDeclare,
			PlayerSeat: expectedSeat,
		}
	case gs.Phase == game.PhasePlaying && gs.PendingBelotSeat != nil && *gs.PendingBelotSeat == expectedSeat:
		// Auto-skip belot announcement on timer expiry
		action = game.Action{
			Type:       game.ActionSkipBelot,
			PlayerSeat: expectedSeat,
		}
	case gs.Phase == game.PhasePlaying:
		// Auto-play a card
		cardID, err := game.AutoPlay(gs)
		if err != nil {
			slog.Error("session: auto-play failed", "roomID", session.roomID, "error", err)
			// Restart timer so the game doesn't stall. The seat that timed out
			// is still active — re-arm for the same seat.
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
		// Phase doesn't support auto-play (e.g., match_end, paused)
		session.mu.Unlock()
		return
	}

	oldState := gs
	newState, err := game.ApplyAction(oldState, action)
	if err != nil {
		slog.Error("session: auto-play ApplyAction failed", "roomID", session.roomID, "error", err)
		// Restart timer so the game doesn't stall permanently. The seat that
		// timed out is still active — re-arm for the same seat.
		m.startTimerLocked(session, expectedSeat)
		session.mu.Unlock()
		return
	}

	// Handle dealing→bidding auto-transition inside the lock
	if newState.Phase == game.PhaseDealing {
		newState.Phase = game.PhaseBidding
	}

	// Within-turn auto-action chain. The first ApplyAction may leave the same
	// seat owing a continuation: skip_declare clears the prompt but the player
	// still owes a card; an auto-played card may itself open a belot prompt
	// (K/Q of trump while holding both) which keeps the same seat. Without
	// chaining, setTurnExpiry below would either extend the doomed player's
	// window (the bug we're fixing) or leave the timer unarmed and stall.
	//
	// Loop instead: while seat and phase are still equal to oldState's, pick
	// the next auto-action structurally based on what's blocking newState
	// (PendingBelotSeat → skip_belot, AwaitingDeclaration → skip_declare,
	// otherwise auto-play). Bounded depth as a safety net against pathological
	// state — in practice the chain resolves in at most three steps
	// (skip_declare → auto-play → skip_belot if K/Q of trump came out).
	const maxChainSteps = 3
	type chainStep struct {
		action game.Action
		pre    *game.GameState
		post   *game.GameState
	}
	steps := []chainStep{{action: action, pre: oldState, post: newState}}

	for i := 0; i < maxChainSteps; i++ {
		cur := steps[len(steps)-1].post
		// Done if the seat or phase already transitioned vs the timer-firing
		// state. PhaseMatchEnd / PhasePaused also fall out here.
		if cur.ActivePlayerSeat != oldState.ActivePlayerSeat || cur.Phase != oldState.Phase {
			break
		}
		if cur.Phase != game.PhasePlaying {
			break
		}
		// Pick the next auto-action structurally — no action-type enumeration.
		// PendingBelotSeat takes precedence over AwaitingDeclaration in the same
		// way handleTimerExpiry's initial switch orders them.
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
		// Empty action.Type means the inner switch hit an error path — break.
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
		}
		steps = append(steps, chainStep{action: next, pre: cur, post: appliedNS})
	}

	finalState := steps[len(steps)-1].post

	// Set expiry and start timer for the next player. Three cases:
	//  • Seat or phase advanced past oldState — fresh timer for the new turn.
	//  • Seat unchanged AND phase unchanged AND we exhausted maxChainSteps or
	//    a chain step errored — defensive fresh timer to prevent the game from
	//    stalling. The slog.Error in the chain logs the underlying invariant
	//    violation; arming a fresh timer keeps the game moving while operator
	//    debugs.
	//  • Phase is match_end / paused — outer guard skips both.
	if finalState.Phase == game.PhasePlaying || finalState.Phase == game.PhaseBidding {
		// Pass finalState.ActivePlayerSeat explicitly: session.gameState is
		// reassigned to finalState only after this block, so a state-less
		// startTimerLocked would capture the OLD (timed-out) seat and the next
		// timer would auto-act for the wrong player.
		m.setTurnExpiry(session, finalState)
		m.startTimerLocked(session, finalState.ActivePlayerSeat)
	} else if finalState.Phase == game.PhaseHandComplete {
		// An auto-play ended the hand (the timed-out card was the last of the
		// hand). Arm the score-reveal auto-continue safety net, exactly as the
		// normal-action path does in HandleAction — otherwise this pause would
		// have NO server-side timer and could stall until every player manually
		// continues. cancelTurnTimer bumps the generation so the captured gen is
		// current.
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

	// Inform clients that a non-card auto-action just fired so they can surface
	// a toast naming the timed-out player. Only the *first* step emits this —
	// chained card-play uses the AutoPlayed flag on event:card_played, and
	// further chained skip_belot is implementation detail (a single timer
	// expiry should produce a single user-facing notification).
	if autoType, ok := autoActionTypeFor(action.Type); ok {
		m.hub.BroadcastToUsers(playerIDs[:], buildMessage(ws.EventAutoAction, ws.AutoActionPayload{
			PlayerSeat: expectedSeat,
			Type:       autoType,
		}))
	}

	// Broadcast each step's result in order. Multi-event sequences ride as
	// separate ordered messages per project-context — never batched. Each
	// step within a chain represents an auto-action (the player was AFK), so
	// any step that was an ActionPlayCard rides with autoPlayed=true.
	for _, step := range steps {
		isAutoPlayedCard := step.action.Type == game.ActionPlayCard
		m.broadcastActionResult(playerIDs, step.pre, step.post, step.action, isAutoPlayedCard)
		m.bufferHandResultIfScored(session, step.pre, step.post)
	}

	// Check for match completion. Auto-play never produces a surrender (the
	// per-move timer doesn't auto-resolve a pending proposal — see AC #13),
	// so surrenderedBy is always nil here.
	if finalState.Phase == game.PhaseMatchEnd {
		// The match-ending action is whichever step landed in PhaseMatchEnd —
		// almost always the last one (steps run sequentially and only the
		// terminal step transitions the phase).
		last := steps[len(steps)-1]
		matchEndAction := last.action
		matchEndOld := last.pre
		matchEndPayload := buildMatchEndPayload(matchEndOld, finalState, matchEndAction, startedAt)
		m.handleMatchEnd(session, finalState, nil, matchEndPayload)
	}
}

// autoActionTypeFor maps a rules-engine action type to the wire-format
// AutoActionType emitted on timer expiry. Returns (_, false) for actions that
// should not produce an EventAutoAction (notably ActionPlayCard, which uses the
// AutoPlayed flag on EventCardPlayed).
func autoActionTypeFor(actionType string) (ws.AutoActionType, bool) {
	switch actionType {
	case game.ActionPassTrump:
		return ws.AutoActionPassTrump, true
	case game.ActionSkipDeclare:
		return ws.AutoActionSkipDeclare, true
	case game.ActionSkipBelot:
		return ws.AutoActionSkipBelot, true
	}
	return "", false
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
	payload := ws.MatchEndPayload{
		WinnerTeam:       safeDerefInt(newState.WinnerTeam),
		TeamAFinalScore:  newState.TeamScores[game.TeamA],
		TeamBFinalScore:  newState.TeamScores[game.TeamB],
		MatchDurationSec: int(time.Since(startedAt).Seconds()),
		OutcomeReason:    ws.OutcomeReasonNatural,
	}
	if action.Type == game.ActionSurrenderAccept && oldState != nil && oldState.SurrenderProposerSeat != nil {
		proposerSeat := *oldState.SurrenderProposerSeat
		payload.OutcomeReason = ws.OutcomeReasonSurrender
		payload.SurrenderedBySeat = &proposerSeat
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
