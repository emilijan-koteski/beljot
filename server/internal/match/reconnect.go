package match

import (
	"log/slog"
	"time"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/ws"
)

// recomputeEarliestReconnectExpiryLocked scans gs.PlayerReconnectExpiresAt and
// updates the legacy single-pointer fields (DisconnectedSeat +
// ReconnectExpiresAt) to track whichever seat's window closes soonest. Must
// be called under session.mu.Lock() after any change to the per-seat array.
//
// The pre-Story 7 "single timer" data model only carried one expiry; the
// per-seat array was added so concurrent disconnects each get their own
// `reconnectWindowSec`. To keep the wire shape stable for older clients we
// keep DisconnectedSeat / ReconnectExpiresAt as the *earliest* derivation —
// new clients can compute richer behaviour from PlayerReconnectExpiresAt
// directly.
func recomputeEarliestReconnectExpiryLocked(gs *game.GameState) {
	earliestSeat := -1
	var earliest *time.Time
	for i, t := range gs.PlayerReconnectExpiresAt {
		if t == nil {
			continue
		}
		if earliest == nil || t.Before(*earliest) {
			earliestSeat = i
			tCopy := *t
			earliest = &tCopy
		}
	}
	gs.DisconnectedSeat = earliestSeat
	gs.ReconnectExpiresAt = earliest
}

// startSeatReconnectTimerLocked arms a per-seat abandon timer that fires after
// `session.reconnectWindowSec`. Must be called under session.mu.Lock().
//
// Each seat owns its own timer + generation counter so concurrent disconnects
// don't share a clock — Player B dropping mid-Player-A's window gets the full
// `reconnectWindowSec` from B's drop, regardless of how much time A had left.
// The hub abandons the match the moment ANY seat's timer fires.
func (m *Manager) startSeatReconnectTimerLocked(session *LiveMatch, seat int) {
	session.cancelSeatReconnectTimer(seat) // bumps generation + nils any prior timer
	gen := session.seatReconnectGenerations[seat]
	// expiryGrace: clients render ReconnectExpiresAt and their ring reaches 0
	// exactly at the window's end — abandon a beat later so nobody watches the
	// match die while their countdown still shows time (same contract as the
	// per-move turn timer).
	session.seatReconnectTimers[seat] = time.AfterFunc(
		time.Duration(session.reconnectWindowSec)*time.Second+expiryGrace,
		func() {
			m.handleSeatReconnectTimeout(session, seat, gen)
		},
	)
}

// HandleDisconnect is called by the hub when a client truly disconnects (not replaced).
// If the user is in an active game session, this transitions the game to PhaseDisconnected,
// broadcasts the disconnect event, and starts a reconnect countdown timer.
func (m *Manager) HandleDisconnect(userID uint) {
	m.mu.RLock()
	roomID, ok := m.userToRoom[userID]
	if !ok {
		m.mu.RUnlock()
		return // Not in a game session — lobby disconnect, no game impact
	}
	session, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}

	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		return
	}

	gs := session.gameState

	// [F3] Only handle disconnect during stable player-facing phases (AC1).
	// PhaseDisconnected is handled separately below (concurrent disconnect path).
	// PhaseHandComplete (the score-reveal pause) is included: a drop there must
	// pause the table — otherwise the seat stays Connected=true and the
	// allConnectedReady check waits the full auto-continue window for an
	// acknowledgement that can never arrive, jamming the table (seen when a
	// mobile client's socket drops during the pause).
	switch gs.Phase {
	case game.PhasePlaying, game.PhaseBidding, game.PhasePaused, game.PhaseHandComplete:
		// These are valid phases for disconnect handling — proceed
	case game.PhaseDisconnected:
		// Concurrent disconnect: another player's reconnect window is already
		// active. The data model tracks only one DisconnectedSeat / one reconnect
		// timer, so we cannot start a second independent window — but we MUST
		// mark this player's Connected=false and broadcast the event, otherwise
		// the server keeps treating them as present. If the first player
		// reconnects, HandleReconnect detects the still-disconnected seat and
		// chains a fresh disconnect transition for them.
		m.handleConcurrentDisconnectLocked(session, gs, userID)
		return
	default:
		// Transient phases (dealing, trick_resolving, hand_scoring, match_end) — skip
		session.mu.Unlock()
		return
	}

	// Find the seat index for the disconnected user
	seat := -1
	for i, uid := range session.playerIDs {
		if uid == userID {
			seat = i
			break
		}
	}
	if seat == -1 {
		session.mu.Unlock()
		return
	}

	slog.Info("session: player disconnected", "roomID", roomID, "userID", userID, "seat", seat)

	// Story 8.2: clear pending surrender proposal if the proposer or the
	// proposer's partner is disconnecting — partner can no longer act on the
	// prompt while in PhaseDisconnected, so leaving the pointer set would
	// strand the partner's UI. Capture the proposer seat for a follow-up
	// event:surrender_declined broadcast so all four clients drop the
	// prompt/banner. The proposer's SurrenderUsed flag stays consumed.
	clearedSurrenderProposer := -1
	if gs.SurrenderProposerSeat != nil {
		proposer := *gs.SurrenderProposerSeat
		partner := (proposer + 2) % 4
		if seat == proposer || seat == partner {
			clearedSurrenderProposer = proposer
			gs.SurrenderProposerSeat = nil
		}
	}

	// [F5] Track whether we auto-cleared a pause — if so, TurnTimeRemaining
	// is already preserved from the original pause and should not be overwritten.
	pauseWasAutoCleared := false

	// Auto-clear disconnected player's active pause (fixes D54)
	if gs.PausedPlayers[seat] {
		gs.PausedPlayers[seat] = false
		// If game was paused and no other pauses remain, restore previous phase
		anyPaused := false
		for _, p := range gs.PausedPlayers {
			if p {
				anyPaused = true
				break
			}
		}
		if !anyPaused && gs.Phase == game.PhasePaused {
			gs.Phase = gs.PreviousPhase
			gs.PreviousPhase = ""
			pauseWasAutoCleared = true
		}
	}

	// Mark player as disconnected
	gs.Players[seat].Connected = false

	// [F4] Cancel turn timer (bumps timerGeneration to invalidate any in-flight
	// stale callback — generation bump is now implicit in cancelTurnTimer).
	// Capture remaining time (same pattern as pause).
	session.cancelTurnTimer()
	// [F5] Only capture TurnTimeRemaining from TurnExpiresAt if we didn't just auto-clear
	// a pause — in the pause case, TurnTimeRemaining was already stored by the original
	// pause action and TurnExpiresAt is nil, so we should keep the existing value.
	if !pauseWasAutoCleared {
		if gs.TurnExpiresAt != nil {
			remaining := time.Until(*gs.TurnExpiresAt)
			if remaining > 0 {
				gs.TurnTimeRemaining = remaining.Milliseconds()
			}
		}
		gs.TurnExpiresAt = nil
	}

	// [F2] Transition to disconnected phase — when current phase is PhasePaused
	// (other players still have active pauses), save the pre-pause phase, not PhasePaused itself.
	if gs.Phase == game.PhasePaused {
		// gs.PreviousPhase already holds the pre-pause phase — keep it as the restore target
		// (don't overwrite with PhasePaused)
	} else {
		gs.PreviousPhase = gs.Phase
	}
	gs.Phase = game.PhaseDisconnected

	// Per-seat reconnect window: this seat gets the full `reconnectWindowSec`
	// from now, independent of any other seat's clock. Earliest-expiry view
	// (gs.DisconnectedSeat / gs.ReconnectExpiresAt) is recomputed afterwards
	// so the legacy single-timer wire fields still reflect the soonest abandon.
	reconnectExpiry := time.Now().Add(time.Duration(session.reconnectWindowSec) * time.Second)
	gs.PlayerReconnectExpiresAt[seat] = &reconnectExpiry
	recomputeEarliestReconnectExpiryLocked(gs)

	// Start this seat's per-seat abandon timer.
	m.startSeatReconnectTimerLocked(session, seat)

	// [F1] Capture immutable values and build messages BEFORE unlocking to avoid data races.
	// The gs pointer aliases session.gameState — concurrent HandleAction could mutate after unlock.
	playerIDs := session.playerIDs
	username := gs.Players[seat].Username

	disconnectPayload := ws.PlayerDisconnectedPayload{
		PlayerSeat:         seat,
		Username:           username,
		ReconnectExpiresAt: reconnectExpiry.UTC().Format(time.RFC3339Nano),
	}

	disconnectMsg := buildMessage(ws.EventPlayerDisconnected, disconnectPayload)
	stateMsg := buildMessage(ws.EventMatchState, gs)

	var surrenderDeclinedMsg []byte
	if clearedSurrenderProposer >= 0 {
		surrenderDeclinedMsg = buildMessage(ws.EventSurrenderDeclined, ws.SurrenderDeclinedPayload{
			ProposerSeat:  clearedSurrenderProposer,
			DecliningSeat: seat,
		})
	}

	session.mu.Unlock()

	// Broadcast to the remaining human players (exclude the disconnected
	// player; bot seats have no sockets).
	remainingPlayers := make([]uint, 0, 3)
	for i, uid := range playerIDs {
		if i != seat && uid != 0 {
			remainingPlayers = append(remainingPlayers, uid)
		}
	}
	if surrenderDeclinedMsg != nil {
		// Drop the pending prompt/banner before disconnect overlay takes over.
		m.hub.BroadcastToUsers(remainingPlayers, surrenderDeclinedMsg)
	}
	m.hub.BroadcastToUsers(remainingPlayers, disconnectMsg)
	m.hub.BroadcastToUsers(remainingPlayers, stateMsg)
}

// handleConcurrentDisconnectLocked is called when another player disconnects
// while at least one seat's reconnect window is already active. session.mu
// MUST be locked on entry; this function unlocks it before returning.
//
// Each disconnect gets its own per-seat window (`reconnectWindowSec` from the
// drop time) and its own per-seat abandon timer — this seat's clock is
// independent of any seat that dropped earlier. The earliest-expiry view in
// gs.DisconnectedSeat / gs.ReconnectExpiresAt is recomputed afterwards so
// the dialog's "soonest abandon" pointer stays correct.
func (m *Manager) handleConcurrentDisconnectLocked(session *LiveMatch, gs *game.GameState, userID uint) {
	seat := -1
	for i, uid := range session.playerIDs {
		if uid == userID {
			seat = i
			break
		}
	}
	// Unknown user, or the seat is already marked disconnected — nothing to do.
	if seat == -1 || !gs.Players[seat].Connected {
		session.mu.Unlock()
		return
	}

	slog.Info("session: concurrent player disconnected during reconnect window",
		"roomID", session.roomID, "userID", userID, "seat", seat,
		"primaryDisconnectedSeat", gs.DisconnectedSeat,
	)

	gs.Players[seat].Connected = false
	username := gs.Players[seat].Username

	// Fresh per-seat window for the new drop — does NOT inherit the existing
	// earliest expiry. Without this, a player who dropped 60 s after the
	// first one would only get 60 s of window, even though the contract is
	// `reconnectWindowSec` from their own drop.
	reconnectExpiry := time.Now().Add(time.Duration(session.reconnectWindowSec) * time.Second)
	gs.PlayerReconnectExpiresAt[seat] = &reconnectExpiry
	recomputeEarliestReconnectExpiryLocked(gs)
	m.startSeatReconnectTimerLocked(session, seat)

	disconnectPayload := ws.PlayerDisconnectedPayload{
		PlayerSeat:         seat,
		Username:           username,
		ReconnectExpiresAt: reconnectExpiry.UTC().Format(time.RFC3339Nano),
	}
	disconnectMsg := buildMessage(ws.EventPlayerDisconnected, disconnectPayload)
	stateMsg := buildMessage(ws.EventMatchState, gs)

	playerIDs := session.playerIDs
	session.mu.Unlock()

	// Broadcast to the remaining human players (exclude the just-disconnected
	// player; bot seats have no sockets).
	remaining := make([]uint, 0, 3)
	for i, uid := range playerIDs {
		if i != seat && uid != 0 {
			remaining = append(remaining, uid)
		}
	}
	m.hub.BroadcastToUsers(remaining, disconnectMsg)
	m.hub.BroadcastToUsers(remaining, stateMsg)
}

// hasActivePause reports whether any seat has an active pause flag set.
func hasActivePause(p [4]bool) bool {
	for _, v := range p {
		if v {
			return true
		}
	}
	return false
}

// HandleReconnect is called by the hub when a client registers (connects or reconnects).
// If the user is in an active game session that is in PhaseDisconnected, this restores
// the game state, cancels the reconnect timer, and broadcasts the reconnection to all players.
// Safe to call for ALL connection registrations — returns immediately for non-game users.
func (m *Manager) HandleReconnect(userID uint) {
	m.mu.RLock()
	roomID, ok := m.userToRoom[userID]
	if !ok {
		m.mu.RUnlock()
		return // Not in a game session — lobby connection, no game impact
	}
	session, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}

	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		return
	}

	gs := session.gameState

	// Only handle reconnection during PhaseDisconnected
	if gs.Phase != game.PhaseDisconnected {
		session.mu.Unlock()
		return
	}

	// Find the seat for the reconnecting user. ConnectHandler fires for ALL
	// connections, including the still-online seats whose WS just authenticated
	// for the first time, so we silently no-op for users who aren't in this
	// session or whose seat is already marked Connected.
	mySeat := -1
	for i, uid := range session.playerIDs {
		if uid == userID {
			mySeat = i
			break
		}
	}
	if mySeat == -1 {
		session.mu.Unlock()
		return // user is not a player in this session
	}
	if gs.Players[mySeat].Connected {
		session.mu.Unlock()
		return // already marked online — no-op (e.g. WS replaced cleanly)
	}

	// Validate this seat's own reconnect window has not expired. Each seat
	// owns its own window now, so we check `PlayerReconnectExpiresAt[mySeat]`
	// rather than the earliest-expiry derivation. A seat whose own window
	// fired already has its abandon path in flight — guard against a racy
	// rejoin landing between fire and timeout-handler-acquires-lock.
	//
	// The abandon timer fires expiryGrace AFTER the advertised deadline, so
	// honor the same cushion here: a reconnect landing inside the grace is
	// late-but-honest input the contract promises to accept, not a stale
	// rejoin (the generation check in handleSeatReconnectTimeout keeps the
	// race-with-abandon path safe either way).
	if gs.PlayerReconnectExpiresAt[mySeat] == nil ||
		!time.Now().Before(gs.PlayerReconnectExpiresAt[mySeat].Add(expiryGrace)) {
		session.mu.Unlock()
		m.sendError(userID, ws.ErrorInvalidAction, "reconnection rejected: reconnect window expired")
		return
	}

	slog.Info("session: player reconnected", "roomID", roomID, "userID", userID, "seat", mySeat)

	// Restore player state for this seat: stop their timer, clear their
	// expiry, recompute the earliest-expiry derivation. The phase only
	// reverts when ALL seats are back online — if other seats are still
	// offline, their timers keep running and we stay in PhaseDisconnected.
	gs.Players[mySeat].Connected = true
	session.cancelSeatReconnectTimer(mySeat)
	gs.PlayerReconnectExpiresAt[mySeat] = nil
	recomputeEarliestReconnectExpiryLocked(gs)

	allConnected := true
	for i := 0; i < 4; i++ {
		if !gs.Players[i].Connected {
			allConnected = false
			break
		}
	}

	if allConnected {
		// Last reconnect — restore the pre-disconnect phase + (where
		// applicable) re-arm the per-move turn timer. Mirrors the old
		// "chain==nil && all online" branch.
		gs.Phase = gs.PreviousPhase
		gs.PreviousPhase = ""
		if hasActivePause(gs.PausedPlayers) {
			// Active pauses from other players survived. Restore PhasePaused
			// (preserve PreviousPhase as the just-restored Playing/Bidding)
			// and skip the timer re-arm. Unpause will recompute
			// TurnExpiresAt from TurnTimeRemaining when the last pause
			// clears.
			gs.PreviousPhase = gs.Phase
			gs.Phase = game.PhasePaused
		} else if session.timerStyle == "per-move" && (gs.Phase == game.PhasePlaying || gs.Phase == game.PhaseBidding) {
			// Restore turn timer (same pattern as unpause timer resume in HandleAction).
			const minResumeMs int64 = 3000
			remaining := time.Duration(gs.TurnTimeRemaining) * time.Millisecond
			if gs.TurnTimeRemaining > 0 && gs.TurnTimeRemaining < minResumeMs {
				remaining = time.Duration(minResumeMs) * time.Millisecond
			} else if gs.TurnTimeRemaining <= 0 {
				remaining = time.Duration(minResumeMs) * time.Millisecond
			}
			expiry := time.Now().Add(remaining)
			gs.TurnExpiresAt = &expiry
			gs.TurnTimeRemaining = 0
			// cancelTurnTimer bumps timerGeneration; armTurnTimerLocked
			// captures the post-cancel generation and adds expiryGrace, so the
			// restored turn keeps the players-see-0-first contract too.
			session.cancelTurnTimer()
			m.armTurnTimerLocked(session, remaining, gs.ActivePlayerSeat)
		} else if gs.Phase == game.PhaseHandComplete {
			// Restored into the score-reveal pause. The disconnect interrupted the
			// pause, so give the returning player a FRESH auto-continue window to
			// read the score (rather than resuming a deadline that may have already
			// elapsed during the outage), and re-arm the timer. Applies regardless
			// of timer style.
			gs.TurnExpiresAt = nil
			session.handCompleteExpiresAt = time.Now().Add(handCompleteAutoContinue)
			session.cancelTurnTimer()
			gen := session.timerGeneration
			session.turnTimer = time.AfterFunc(handCompleteAutoContinue, func() {
				m.handleHandCompleteTimeout(session, gen)
			})
		}
	}

	// Build messages BEFORE unlocking (data-race prevention — same pattern as HandleDisconnect)
	playerIDs := session.playerIDs
	reconnectPayload := ws.PlayerReconnectedPayload{PlayerSeat: mySeat}
	reconnectMsg := buildMessage(ws.EventPlayerReconnected, reconnectPayload)
	stateMsg := buildMessage(ws.EventMatchState, gs)

	session.mu.Unlock()

	// Broadcast to all human players (reconnecting player needs the state too)
	humanIDs := humanUserIDs(playerIDs)
	m.hub.BroadcastToUsers(humanIDs, reconnectMsg)
	m.hub.BroadcastToUsers(humanIDs, stateMsg)

	// The restored phase may put a bot back on the clock (its pending timer
	// no-oped during the disconnect pause).
	m.maybeScheduleBotAction(session)
}

// SyncStateOnConnect pushes the current authoritative match state directly to
// a user who just (re)registered a WebSocket while being part of an active
// session. HandleReconnect only restores seats that were marked disconnected —
// when the hub REPLACES a still-registered socket (the user reconnects before
// the dead connection is reaped by the ping loop) no disconnect handler ever
// fires, the seat is still Connected=true, and HandleReconnect no-ops. There
// is no client→server "request state" message, so without this push the
// refreshed client would sit on the reconnect splash with no way to recover
// until some unrelated event happened to broadcast. Call AFTER HandleReconnect
// so the snapshot reflects any phase restore. Safe for every registration:
// no-ops for users outside a session.
func (m *Manager) SyncStateOnConnect(userID uint) {
	m.mu.RLock()
	roomID, ok := m.userToRoom[userID]
	if !ok {
		m.mu.RUnlock()
		return
	}
	session, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}

	session.mu.RLock()
	if session.closed {
		session.mu.RUnlock()
		return
	}
	msg := buildMessage(ws.EventMatchState, session.gameState)
	session.mu.RUnlock()

	m.hub.SendToUser(userID, msg)
}

// handleSeatReconnectTimeout is called when an individual seat's reconnect
// countdown fires. It transitions the game to PhaseMatchEnd (abandoned),
// persists the match record, broadcasts event:match_abandoned to all players,
// and cleans up the session. Called once per per-seat timer — generation
// staleness check ensures a cancelled timer's queued callback is a no-op.
func (m *Manager) handleSeatReconnectTimeout(session *LiveMatch, seat int, generation uint64) {
	session.mu.Lock()

	// Guard: session closed, seat out of range, or generation stale
	// (this seat reconnected and bumped the counter via cancelSeatReconnectTimer).
	if session.closed || seat < 0 || seat >= 4 ||
		session.seatReconnectGenerations[seat] != generation {
		session.mu.Unlock()
		return
	}

	gs := session.gameState

	// Race guard: the seat's timer fired but they reconnected just before we
	// took the lock. Belt-and-braces with the generation check above.
	if gs.Players[seat].Connected {
		session.mu.Unlock()
		return
	}

	abandonedSeat := seat
	abandonedPlayerID := session.playerIDs[abandonedSeat]

	// Transition game state to match_end (abandoned)
	gs.Phase = game.PhaseMatchEnd
	// WinnerTeam stays nil — no winner for abandoned match
	// Clear disconnect fields (singletons + per-seat array — every other
	// seat's timer also gets cancelled below).
	gs.DisconnectedSeat = -1
	gs.ReconnectExpiresAt = nil
	for i := 0; i < 4; i++ {
		gs.PlayerReconnectExpiresAt[i] = nil
	}
	// Story 8.2: clear any surrender proposal that survived from before the
	// disconnect. If HandleDisconnect already cleared it (proposer or partner
	// was the disconnecting seat) this is a no-op; otherwise (proposer and
	// partner both stayed connected) we still must drop it before the
	// abandoned overlay mounts so the SurrenderPrompt doesn't ride along.
	gs.SurrenderProposerSeat = nil

	// Cancel any active timers (every per-seat reconnect timer + the turn timer).
	session.cancelAllReconnectTimers()
	session.cancelTurnTimer()

	// Capture data for broadcast BEFORE unlocking (data race prevention)
	playerIDs := session.playerIDs
	roomID := session.roomID
	startedAt := session.startedAt
	coinBuyIn := session.coinBuyIn
	teamAScore := gs.TeamScores[game.TeamA]
	teamBScore := gs.TeamScores[game.TeamB]
	variant := string(gs.Variant)
	matchMode := gs.MatchMode
	var botSeats [4]bool
	for i := range gs.Players {
		botSeats[i] = gs.Players[i].IsBot
	}
	// Snapshot any hands scored before the abandonment so they persist alongside
	// the match row. Empty when abandonment fires before hand 1 completed.
	handsCopy := make([]HandResult, len(session.handResults))
	copy(handsCopy, session.handResults)

	abandonedPayload := ws.MatchAbandonedPayload{
		AbandonedByPlayer: abandonedSeat,
		TeamAFinalScore:   teamAScore,
		TeamBFinalScore:   teamBScore,
		MatchDurationSec:  int(time.Since(startedAt).Seconds()),
	}
	abandonedMsg := buildMessage(ws.EventMatchAbandoned, abandonedPayload)
	stateMsg := buildMessage(ws.EventMatchState, gs)

	session.mu.Unlock()

	// Story 9.2 (AC #8): the WHOLE abandoning team forfeits (both −S, no
	// teammate refund); the winner is the NON-abandoning team. Settle against
	// that computed winner (also persisted as the record's WinnerTeam below).
	// No-op when coinBuyIn == 0. Deltas ride the match row below; settlement
	// events are sent after event:match_abandoned, before the trailing
	// event:match_state.
	winningTeam := 1 - game.TeamForSeat(abandonedSeat)
	deltas, settlementMsgs := m.settleMatch(roomID, playerIDs, botSeats, winningTeam, coinBuyIn)

	// Story 9.5: award XP for the abandonment. The WHOLE abandoning team forfeits
	// (0 XP); only the non-abandoning team earns (points-so-far). Pass the real
	// abandonedSeat + the snapshotted team scores ([A,B] index order). Best-effort
	// like settlement — a failure logs and skips the events but never blocks the
	// broadcasts. xp_awarded is slotted after coin_settlement, before match_state.
	xpMsgs := m.awardXP(roomID, playerIDs, botSeats, [2]int{teamAScore, teamBScore}, abandonedSeat)

	// Broadcast to all human players (disconnected player gets it if they reconnect to WS later)
	userIDs := humanUserIDs(playerIDs)
	m.hub.BroadcastToUsers(userIDs, abandonedMsg)
	for _, sm := range settlementMsgs {
		m.hub.SendToUser(sm.userID, sm.msg)
	}
	for _, xm := range xpMsgs {
		m.hub.SendToUser(xm.userID, xm.msg)
	}
	m.hub.BroadcastToUsers(userIDs, stateMsg)

	slog.Info("session: match abandoned due to reconnect timeout",
		"roomID", roomID,
		"abandonedBy", abandonedPlayerID,
		"abandonedSeat", abandonedSeat,
	)

	// Persist match record with abandoned status. Bot seats persist exactly
	// as in the completed-match path: NULL IDs + per-seat flags.
	ids, botFlags, hasBots := matchSeatColumns(playerIDs, botSeats)
	matchRecord := &Match{
		RoomID:       roomID,
		Player1ID:    ids[0],
		Player2ID:    ids[1],
		Player3ID:    ids[2],
		Player4ID:    ids[3],
		Player1IsBot: botFlags[0],
		Player2IsBot: botFlags[1],
		Player3IsBot: botFlags[2],
		Player4IsBot: botFlags[3],
		HasBots:      hasBots,
		TeamAScore:   teamAScore,
		TeamBScore:   teamBScore,
		// The real non-abandoning team — the same value settlement used above.
		// Meaningful because AbandonedBy is set: stats/history read win/loss for
		// the three non-abandoners off this column, gated on abandoned_by IS NOT
		// NULL (boot-reconcile rows keep a filler 0 and stay plain "abandoned").
		WinnerTeam:       winningTeam,
		Variant:          variant,
		MatchMode:        matchMode,
		StartedAt:        startedAt,
		CompletedAt:      time.Now(),
		Status:           "abandoned",
		AbandonedBy:      &abandonedPlayerID,
		CoinBuyIn:        coinBuyIn,
		Player1CoinDelta: deltas[0],
		Player2CoinDelta: deltas[1],
		Player3CoinDelta: deltas[2],
		Player4CoinDelta: deltas[3],
	}

	if err := m.matchRepo.CreateWithHands(matchRecord, handsCopy); err != nil {
		slog.Error("session: failed to persist abandoned match", "roomID", roomID, "error", err)
	} else {
		slog.Info("session: abandoned match persisted", "roomID", roomID, "matchID", matchRecord.ID, "hands", len(handsCopy))
	}

	// Update room status
	if m.roomUpdater != nil {
		if err := m.roomUpdater.UpdateRoomStatus(roomID, "completed"); err != nil {
			slog.Error("session: failed to update room status", "roomID", roomID, "error", err)
		}
	}

	m.RemoveSession(roomID)
}
