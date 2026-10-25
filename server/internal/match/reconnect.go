package match

import (
	"log/slog"
	"time"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/ws"
)

// recomputeEarliestReconnectExpiryLocked points DisconnectedSeat and
// ReconnectExpiresAt at the seat whose window closes soonest. Call under session.mu.
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

// startSeatReconnectTimerLocked arms this seat's own abandon timer for
// reconnectWindowSec. Call under session.mu.
func (m *Manager) startSeatReconnectTimerLocked(session *LiveMatch, seat int) {
	session.cancelSeatReconnectTimer(seat)
	gen := session.seatReconnectGenerations[seat]
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
		return
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

	switch gs.Phase {
	case game.PhasePlaying, game.PhaseBidding, game.PhaseDeclaring, game.PhasePaused, game.PhaseHandComplete:
	case game.PhaseDisconnected:
		m.handleConcurrentDisconnectLocked(session, gs, userID)
		return
	default:
		session.mu.Unlock()
		return
	}

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

	clearedSurrenderProposer := -1
	if gs.SurrenderProposerSeat != nil {
		proposer := *gs.SurrenderProposerSeat
		partner := (proposer + 2) % 4
		if seat == proposer || seat == partner {
			clearedSurrenderProposer = proposer
			gs.SurrenderProposerSeat = nil
		}
	}

	pauseWasAutoCleared := false

	if gs.PausedPlayers[seat] {
		gs.PausedPlayers[seat] = false
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

	gs.Players[seat].Connected = false

	session.cancelTurnTimer()
	if !pauseWasAutoCleared {
		if gs.TurnExpiresAt != nil {
			remaining := time.Until(*gs.TurnExpiresAt)
			if remaining > 0 {
				gs.TurnTimeRemaining = remaining.Milliseconds()
			}
		}
		gs.TurnExpiresAt = nil
	}

	if gs.Phase == game.PhasePaused {
	} else {
		gs.PreviousPhase = gs.Phase
	}
	gs.Phase = game.PhaseDisconnected
	game.RefreshDerivedFlags(gs)

	reconnectExpiry := time.Now().Add(time.Duration(session.reconnectWindowSec) * time.Second)
	gs.PlayerReconnectExpiresAt[seat] = &reconnectExpiry
	recomputeEarliestReconnectExpiryLocked(gs)

	m.startSeatReconnectTimerLocked(session, seat)

	playerIDs := session.playerIDs
	username := gs.Players[seat].Username

	disconnectPayload := ws.PlayerDisconnectedPayload{
		PlayerSeat:         seat,
		Username:           username,
		ReconnectExpiresAt: reconnectExpiry.UTC().Format(time.RFC3339Nano),
	}

	disconnectMsg := buildMessage(ws.EventPlayerDisconnected, disconnectPayload)
	stateFrames := buildStateFrames(playerIDs, gs, seat)

	var surrenderDeclinedMsg []byte
	if clearedSurrenderProposer >= 0 {
		surrenderDeclinedMsg = buildMessage(ws.EventSurrenderDeclined, ws.SurrenderDeclinedPayload{
			ProposerSeat:  clearedSurrenderProposer,
			DecliningSeat: seat,
		})
	}

	session.mu.Unlock()

	remainingPlayers := make([]uint, 0, 3)
	for i, uid := range playerIDs {
		if i != seat && uid != 0 {
			remainingPlayers = append(remainingPlayers, uid)
		}
	}
	if surrenderDeclinedMsg != nil {
		m.hub.BroadcastToUsers(remainingPlayers, surrenderDeclinedMsg)
	}
	m.hub.BroadcastToUsers(remainingPlayers, disconnectMsg)
	m.hub.SendFrames(stateFrames)
}

// handleConcurrentDisconnectLocked gives a seat that drops while another window is
// running its own reconnect window. Called with session.mu locked; unlocks it.
func (m *Manager) handleConcurrentDisconnectLocked(session *LiveMatch, gs *game.GameState, userID uint) {
	seat := -1
	for i, uid := range session.playerIDs {
		if uid == userID {
			seat = i
			break
		}
	}
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
	playerIDs := session.playerIDs
	stateFrames := buildStateFrames(playerIDs, gs, seat)
	session.mu.Unlock()

	remaining := make([]uint, 0, 3)
	for i, uid := range playerIDs {
		if i != seat && uid != 0 {
			remaining = append(remaining, uid)
		}
	}
	m.hub.BroadcastToUsers(remaining, disconnectMsg)
	m.hub.SendFrames(stateFrames)
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

// HandleReconnect restores a disconnected player's seat in an active session and
// broadcasts the reconnection. It returns immediately for users not in a game.
func (m *Manager) HandleReconnect(userID uint) {
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

	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		return
	}

	gs := session.gameState

	if gs.Phase != game.PhaseDisconnected {
		session.mu.Unlock()
		return
	}

	mySeat := -1
	for i, uid := range session.playerIDs {
		if uid == userID {
			mySeat = i
			break
		}
	}
	if mySeat == -1 {
		session.mu.Unlock()
		return
	}
	if gs.Players[mySeat].Connected {
		session.mu.Unlock()
		return
	}

	if gs.PlayerReconnectExpiresAt[mySeat] == nil ||
		!time.Now().Before(gs.PlayerReconnectExpiresAt[mySeat].Add(expiryGrace)) {
		session.mu.Unlock()
		m.sendError(userID, ws.ErrorInvalidAction, "reconnection rejected: reconnect window expired")
		return
	}

	slog.Info("session: player reconnected", "roomID", roomID, "userID", userID, "seat", mySeat)

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
		gs.Phase = gs.PreviousPhase
		gs.PreviousPhase = ""
		if hasActivePause(gs.PausedPlayers) {
			gs.PreviousPhase = gs.Phase
			gs.Phase = game.PhasePaused
			if gs.PreviousPhase == game.PhaseDeclaring {
				session.declarationExpiresAt = time.Now().Add(declarationAutoClose)
			}
		} else if session.timerStyle == "per-move" &&
			(gs.Phase == game.PhasePlaying || gs.Phase == game.PhaseBidding) {
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
			session.cancelTurnTimer()
			m.armTurnTimerLocked(session, remaining, gs.ActivePlayerSeat)
		} else if gs.Phase == game.PhaseDeclaring {
			gs.TurnExpiresAt = nil
			session.declarationExpiresAt = time.Now().Add(declarationAutoClose)
			session.cancelTurnTimer()
			gen := session.timerGeneration
			session.turnTimer = time.AfterFunc(declarationAutoClose, func() {
				m.handleDeclarationTimeout(session, gen)
			})
		} else if gs.Phase == game.PhaseHandComplete {
			gs.TurnExpiresAt = nil
			session.handCompleteExpiresAt = time.Now().Add(handCompleteAutoContinue)
			session.cancelTurnTimer()
			gen := session.timerGeneration
			session.turnTimer = time.AfterFunc(handCompleteAutoContinue, func() {
				m.handleHandCompleteTimeout(session, gen)
			})
		}
	}

	game.RefreshDerivedFlags(gs)

	playerIDs := session.playerIDs
	reconnectPayload := ws.PlayerReconnectedPayload{PlayerSeat: mySeat}
	reconnectMsg := buildMessage(ws.EventPlayerReconnected, reconnectPayload)
	stateFrames := buildStateFrames(playerIDs, gs, -1)

	session.mu.Unlock()

	humanIDs := humanUserIDs(playerIDs)
	m.hub.BroadcastToUsers(humanIDs, reconnectMsg)
	m.hub.SendFrames(stateFrames)

	m.maybeScheduleBotAction(session)
}

// SyncStateOnConnect pushes the current match state to a user who just registered
// a WebSocket while in a session. Call after HandleReconnect.
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
	mySeat := -1
	for seat, uid := range session.playerIDs {
		if uid == userID {
			mySeat = seat
			break
		}
	}
	if mySeat == -1 {
		session.mu.RUnlock()
		return
	}
	stateFrames := []ws.UserFrame{{
		UserID: userID,
		Msg:    buildMessage(ws.EventMatchState, game.ProjectForSeat(session.gameState, mySeat)),
	}}
	session.mu.RUnlock()

	m.hub.SendFrames(stateFrames)
}

// handleSeatReconnectTimeout abandons the match when a seat's reconnect window
// expires, ignoring callbacks from stale timer generations.
func (m *Manager) handleSeatReconnectTimeout(session *LiveMatch, seat int, generation uint64) {
	session.mu.Lock()

	if session.closed || seat < 0 || seat >= 4 ||
		session.seatReconnectGenerations[seat] != generation {
		session.mu.Unlock()
		return
	}

	gs := session.gameState

	if gs.Players[seat].Connected {
		session.mu.Unlock()
		return
	}

	abandonedSeat := seat
	abandonedPlayerID := session.playerIDs[abandonedSeat]

	gs.Phase = game.PhaseMatchEnd
	gs.DisconnectedSeat = -1
	gs.ReconnectExpiresAt = nil
	for i := 0; i < 4; i++ {
		gs.PlayerReconnectExpiresAt[i] = nil
	}
	gs.SurrenderProposerSeat = nil

	session.cancelAllReconnectTimers()
	session.cancelTurnTimer()

	playerIDs := session.playerIDs
	roomID := session.roomID
	startedAt := session.startedAt
	coinBuyIn := session.coinBuyIn
	teamAScore := gs.TeamScores[game.TeamA]
	teamBScore := gs.TeamScores[game.TeamB]
	variant := string(gs.Variant)
	matchMode := gs.MatchMode
	declarationsTransfer := gs.Rules.DeclarationsTransfer
	var botSeats [4]bool
	var connected [4]bool
	for i := range gs.Players {
		botSeats[i] = gs.Players[i].IsBot
		connected[i] = gs.Players[i].Connected
	}
	handsCopy := make([]HandResult, len(session.handResults))
	copy(handsCopy, session.handResults)
	wonByInstantWin := gs.WonByInstantWin

	abandonedPayload := ws.MatchAbandonedPayload{
		AbandonedByPlayer: abandonedSeat,
		TeamAFinalScore:   teamAScore,
		TeamBFinalScore:   teamBScore,
		MatchDurationSec:  int(time.Since(startedAt).Seconds()),
	}
	abandonedMsg := buildMessage(ws.EventMatchAbandoned, abandonedPayload)
	stateFrames := buildStateFrames(playerIDs, gs, -1)

	session.mu.Unlock()

	winningTeam := 1 - game.TeamForSeat(abandonedSeat)
	deltas, settlementMsgs := m.settleMatch(roomID, playerIDs, botSeats, winningTeam, coinBuyIn)

	xpMsgs := m.awardXP(roomID, playerIDs, botSeats, [2]int{teamAScore, teamBScore}, abandonedSeat)

	honorMsgs := m.recordHonor(roomID, playerIDs, botSeats, connected, abandonedSeat)

	finalizedAt := time.Now().UTC()
	spMsgs := m.awardSeasonPoints(roomID, buildSPOutcome(spMatchFacts{
		playerIDs:     playerIDs,
		botSeats:      botSeats,
		connected:     connected,
		winnerTeam:    winningTeam,
		teamScores:    [2]int{teamAScore, teamBScore},
		matchMode:     matchMode,
		instantWin:    wonByInstantWin,
		hands:         handsCopy,
		abandonedSeat: abandonedSeat,
	}), finalizedAt)

	userIDs := humanUserIDs(playerIDs)
	m.hub.BroadcastToUsers(userIDs, abandonedMsg)
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
	m.hub.SendFrames(stateFrames)

	slog.Info("session: match abandoned due to reconnect timeout",
		"roomID", roomID,
		"abandonedBy", abandonedPlayerID,
		"abandonedSeat", abandonedSeat,
	)

	ids, botFlags, hasBots := matchSeatColumns(playerIDs, botSeats)
	matchRecord := &Match{
		RoomID:               roomID,
		Player1ID:            ids[0],
		Player2ID:            ids[1],
		Player3ID:            ids[2],
		Player4ID:            ids[3],
		Player1IsBot:         botFlags[0],
		Player2IsBot:         botFlags[1],
		Player3IsBot:         botFlags[2],
		Player4IsBot:         botFlags[3],
		HasBots:              hasBots,
		TeamAScore:           teamAScore,
		TeamBScore:           teamBScore,
		WinnerTeam:           winningTeam,
		Variant:              variant,
		MatchMode:            matchMode,
		DeclarationsTransfer: declarationsTransfer,
		StartedAt:            startedAt,
		CompletedAt:          time.Now(),
		Status:               "abandoned",
		AbandonedBy:          &abandonedPlayerID,
		CoinBuyIn:            coinBuyIn,
		Player1CoinDelta:     deltas[0],
		Player2CoinDelta:     deltas[1],
		Player3CoinDelta:     deltas[2],
		Player4CoinDelta:     deltas[3],
	}

	if err := m.matchRepo.CreateWithHands(matchRecord, handsCopy); err != nil {
		slog.Error("session: failed to persist abandoned match", "roomID", roomID, "error", err)
	} else {
		slog.Info("session: abandoned match persisted", "roomID", roomID, "matchID", matchRecord.ID, "hands", len(handsCopy))
	}

	if m.roomUpdater != nil {
		if err := m.roomUpdater.UpdateRoomStatus(roomID, "completed"); err != nil {
			slog.Error("session: failed to update room status", "roomID", roomID, "error", err)
		}
	}

	m.RemoveSession(roomID)
}
