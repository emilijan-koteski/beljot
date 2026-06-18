package match_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/ws"
)

// mixedPlayers returns the default seat assignment with the given seats
// replaced by bots (UserID 0, empty username).
func mixedPlayers(botSeats ...int) [4]match.PlayerSeatInfo {
	players := defaultPlayers()
	for _, s := range botSeats {
		players[s] = match.PlayerSeatInfo{Seat: s, IsBot: true}
	}
	return players
}

// markBots flags the given seats as bots on an injected fixture state (the
// factories default IsBot to false).
func markBots(gs *game.GameState, seats ...int) *game.GameState {
	for _, s := range seats {
		gs.Players[s].IsBot = true
		gs.Players[s].UserID = 0
		gs.Players[s].Username = ""
	}
	return gs
}

// waitFor polls cond every 5ms until it returns true or the timeout elapses.
func waitFor(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}

// driveHumanSeat0 performs the scripted human response for seat 0 when the
// state requires one: pass round-1 bidding, pick a suit in round 2 (so an
// all-pass reshuffle loop cannot spin forever), skip prompts, play the first
// legal card, acknowledge score reveals.
func driveHumanSeat0(mgr *match.Manager, st *game.GameState) {
	client := &ws.Client{UserID: 10}
	send := func(msgType string, payload any) {
		raw, _ := json.Marshal(payload)
		mgr.HandleAction(client, ws.WSMessage{Type: msgType, Payload: raw})
	}

	switch st.Phase {
	case game.PhaseBidding:
		if st.ActivePlayerSeat != 0 {
			return
		}
		if st.BiddingRound == 1 {
			send("action:pass_trump", map[string]string{})
			return
		}
		for _, s := range game.AllSuits {
			if st.TrumpCandidate == nil || s != st.TrumpCandidate.Suit {
				send("action:pick_trump", map[string]string{"suit": string(s)})
				return
			}
		}
	case game.PhasePlaying:
		if st.PendingBelotSeat != nil && *st.PendingBelotSeat == 0 {
			send("action:decline_belot", map[string]string{})
			return
		}
		if st.ActivePlayerSeat != 0 {
			return
		}
		if st.AwaitingDeclaration {
			send("action:skip_declare", map[string]string{})
			return
		}
		legal := game.LegalCards(st, 0)
		if len(legal) > 0 {
			send("action:play_card", map[string]string{"cardId": legal[0].String()})
		}
	case game.PhaseHandComplete:
		if !st.HandCompleteReady[0] {
			send("action:continue", map[string]string{})
		}
	}
}

// TestBotMatch_ProgressesToNextHandWithoutClientActions is the AC2/AC3 core:
// a match with three bots progresses Dealing → Bidding → Playing →
// HandComplete → next hand with no client input except the single human's.
func TestBotMatch_ProgressesToNextHandWithoutClientActions(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	mgr.SetBotDelayForTest(time.Millisecond, 2*time.Millisecond)

	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", mixedPlayers(1, 2, 3), "relaxed", 0, 10, 120, 0))

	reachedNextHand := waitFor(20*time.Second, func() bool {
		st := mgr.GetStateSnapshot(100)
		if st == nil {
			return false
		}
		if st.HandNumber >= 2 {
			return true
		}
		driveHumanSeat0(mgr, st)
		return false
	})

	require.True(t, reachedNextHand, "bot match must reach hand 2 without stalling")
}

// TestBot_ActsAfterMinDelayAndBeforeTimerExpiry pins AC3's boundaries with
// the PRODUCTION delay bounds: the bot's first bidding action lands no
// earlier than 1s after its turn opened (never instantaneous) and far inside
// the 10s per-move window, and it is never surfaced as a timeout auto-action.
func TestBot_ActsAfterMinDelayAndBeforeTimerExpiry(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo) // production delays: 1–2.5s

	start := time.Now()
	// Per-move timer at the 10s minimum; first bidder (seat 1) is a bot.
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", mixedPlayers(1), "per-move", 10, 10, 120, 0))

	acted := waitFor(5*time.Second, func() bool {
		st := mgr.GetStateSnapshot(100)
		if st == nil {
			return false
		}
		// Seat 1 acted: it passed (count>0 or turn advanced) or picked trump.
		return st.Phase != game.PhaseBidding || st.BiddingPassCount > 0 || st.ActivePlayerSeat != 1
	})
	elapsed := time.Since(start)

	require.True(t, acted, "bot must act within the think-delay window")
	assert.GreaterOrEqual(t, elapsed, time.Second, "bot must never act instantaneously (≥1s think delay)")
	assert.Less(t, elapsed, 10*time.Second, "bot must act before the per-move timer expires")

	// No timeout auto-action was emitted for the bot's decision.
	for _, call := range hub.snapshot() {
		var msg ws.WSMessage
		require.NoError(t, json.Unmarshal(call.msg, &msg))
		assert.NotEqual(t, ws.EventAutoAction, msg.Type, "bot actions must not surface as timeout auto-actions")
	}
}

// TestBot_CardPlayCarriesNoAutoPlayedMarker pins 4.6: a bot's card play is
// broadcast exactly like a human's — autoPlayed stays false.
func TestBot_CardPlayCarriesNoAutoPlayedMarker(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	mgr.SetBotDelayForTest(time.Millisecond, 2*time.Millisecond)

	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", mixedPlayers(1), "relaxed", 0, 10, 120, 0))

	gs := markBots(testfixtures.NewGameMidPlay(1), 1)
	gs.ActivePlayerSeat = 1
	mgr.SetGameStateForTest(100, gs)
	mgr.BotSchedule(100)

	played := waitFor(2*time.Second, func() bool {
		for _, call := range hub.snapshot() {
			var msg ws.WSMessage
			if json.Unmarshal(call.msg, &msg) == nil && msg.Type == ws.EventCardPlayed {
				return true
			}
		}
		return false
	})
	require.True(t, played, "bot must play a card")

	for _, call := range hub.snapshot() {
		var msg ws.WSMessage
		require.NoError(t, json.Unmarshal(call.msg, &msg))
		if msg.Type != ws.EventCardPlayed {
			continue
		}
		var payload ws.CardPlayedPayload
		require.NoError(t, json.Unmarshal(msg.Payload, &payload))
		assert.Equal(t, 1, payload.PlayerSeat)
		assert.False(t, payload.AutoPlayed, "bot plays carry no autoPlayed marker")
	}
}

// TestBot_StaleTimerNeverFiresAfterStateChange pins the staleness guard: a
// think delay armed for a bidding decision no-ops when the state moves to a
// non-actionable phase before it fires.
func TestBot_StaleTimerNeverFiresAfterStateChange(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	mgr.SetBotDelayForTest(50*time.Millisecond, 50*time.Millisecond)

	// Seat 1 (bot) is the first bidder — StartMatch schedules its decision.
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", mixedPlayers(1), "relaxed", 0, 10, 120, 0))

	// Before the 50ms delay fires, the table pauses.
	paused := markBots(testfixtures.NewGamePaused(0), 1)
	mgr.SetGameStateForTest(100, paused)

	time.Sleep(150 * time.Millisecond)

	st := mgr.GetStateSnapshot(100)
	require.NotNil(t, st)
	assert.Equal(t, game.PhasePaused, st.Phase, "stale bot timer must not act on a paused table")
	assert.Equal(t, 0, st.BiddingPassCount, "no bidding action may have been applied")
}

// TestBot_RemoveSessionCancelsPendingBotTimers pins teardown: pending think
// delays die with the session (no panic, no late action).
func TestBot_RemoveSessionCancelsPendingBotTimers(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	mgr.SetBotDelayForTest(50*time.Millisecond, 50*time.Millisecond)

	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", mixedPlayers(1), "relaxed", 0, 10, 120, 0))
	mgr.RemoveSession(100)

	time.Sleep(150 * time.Millisecond)
	assert.False(t, mgr.HasSession(100))
}

// TestBot_RespondsToBelotPrompt: a bot holding the pending Belote decision
// always announces (+20 to its team).
func TestBot_RespondsToBelotPrompt(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	mgr.SetBotDelayForTest(time.Millisecond, 2*time.Millisecond)

	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", mixedPlayers(1), "relaxed", 0, 10, 120, 0))

	// Bot at seat 1 just played KH (trump) while holding QH — prompt pending.
	gs := markBots(testfixtures.NewGameMidPlay(1), 1)
	pending := 1
	gs.BelotAnnounced = false
	gs.PendingBelotSeat = &pending
	gs.CurrentTrick = []game.TrickCard{{Card: game.Card{Rank: game.RankKing, Suit: game.SuitHearts}, PlayerSeat: 1}}
	lead := game.SuitHearts
	gs.LeadSuit = &lead
	gs.ActivePlayerSeat = 1
	mgr.SetGameStateForTest(100, gs)
	mgr.BotSchedule(100)

	announced := waitFor(2*time.Second, func() bool {
		st := mgr.GetStateSnapshot(100)
		return st != nil && st.BelotAnnounced
	})
	require.True(t, announced, "bot must announce Belote")

	st := mgr.GetStateSnapshot(100)
	assert.Equal(t, 20, st.BelotPoints[game.TeamB], "announcing team banks the 20-point bonus")
	assert.Nil(t, st.PendingBelotSeat)
}

// TestBot_PartnerAcceptsHumanSurrender: surrender is team-internal — the bot
// partner accepts the human's proposal after its think delay and the match
// ends in the opponents' favor.
func TestBot_PartnerAcceptsHumanSurrender(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	mgr.SetBotDelayForTest(time.Millisecond, 2*time.Millisecond)

	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", mixedPlayers(2), "relaxed", 0, 10, 120, 0))

	// Human seat 0 proposed; its partner (seat 2) is the bot.
	gs := markBots(testfixtures.NewGameMidPlay(3), 2)
	proposer := 0
	gs.SurrenderProposerSeat = &proposer
	gs.SurrenderUsed[0] = true
	mgr.SetGameStateForTest(100, gs)
	mgr.BotSchedule(100)

	ended := waitFor(2*time.Second, func() bool {
		return !mgr.HasSession(100)
	})
	require.True(t, ended, "bot partner must accept and end the match")

	matches := repo.getMatches()
	require.Len(t, matches, 1)
	assert.Equal(t, "completed", matches[0].Status)
	assert.Equal(t, game.TeamB, matches[0].WinnerTeam, "surrendering team's opponents win")
}

// TestBot_MatchEndPersistsBotColumns pins AC5's persistence shape: bot seats
// store NULL player IDs + per-seat flags, the match is marked bot-inclusive,
// and human seats keep their real IDs.
func TestBot_MatchEndPersistsBotColumns(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	mgr.SetBotDelayForTest(time.Millisecond, 2*time.Millisecond)

	// Bots at seats 1 and 2; human seat 0 proposes surrender, partner seat 2
	// (bot) accepts — the fast path to a completed bot-inclusive record.
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", mixedPlayers(1, 2), "relaxed", 0, 10, 120, 0))

	gs := markBots(testfixtures.NewGameMidPlay(2), 1, 2)
	proposer := 0
	gs.SurrenderProposerSeat = &proposer
	gs.SurrenderUsed[0] = true
	mgr.SetGameStateForTest(100, gs)
	mgr.BotSchedule(100)

	require.True(t, waitFor(2*time.Second, func() bool { return !mgr.HasSession(100) }))

	matches := repo.getMatches()
	require.Len(t, matches, 1)
	rec := matches[0]
	assert.True(t, rec.HasBots)
	require.NotNil(t, rec.Player1ID)
	assert.Equal(t, uint(10), *rec.Player1ID)
	assert.Nil(t, rec.Player2ID, "bot seat persists NULL")
	assert.Nil(t, rec.Player3ID, "bot seat persists NULL")
	require.NotNil(t, rec.Player4ID)
	assert.Equal(t, uint(40), *rec.Player4ID)
	assert.False(t, rec.Player1IsBot)
	assert.True(t, rec.Player2IsBot)
	assert.True(t, rec.Player3IsBot)
	assert.False(t, rec.Player4IsBot)
}

// TestBot_AbandonedMatchPersistsBotColumns: an abandoned bot-inclusive match
// records the same NULL-ID + flag shape as a completed one.
func TestBot_AbandonedMatchPersistsBotColumns(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	// Long delays so no bot acts while the abandonment plays out.
	mgr.SetBotDelayForTest(time.Minute, time.Minute)

	// 1-second reconnect window so the abandonment fires fast.
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", mixedPlayers(2, 3), "relaxed", 0, 10, 1, 0))

	mgr.HandleDisconnect(10) // human seat 0 drops and never returns

	abandoned := waitFor(5*time.Second, func() bool {
		return !mgr.HasSession(100)
	})
	require.True(t, abandoned, "match must abandon after the reconnect window")

	matches := repo.getMatches()
	require.Len(t, matches, 1)
	rec := matches[0]
	assert.Equal(t, "abandoned", rec.Status)
	assert.True(t, rec.HasBots)
	require.NotNil(t, rec.Player1ID)
	require.NotNil(t, rec.Player2ID)
	assert.Nil(t, rec.Player3ID)
	assert.Nil(t, rec.Player4ID)
	assert.True(t, rec.Player3IsBot)
	assert.True(t, rec.Player4IsBot)
	require.NotNil(t, rec.AbandonedBy)
	assert.Equal(t, uint(10), *rec.AbandonedBy)
}

// TestBot_AcksScoreReveal: every bot acknowledges the hand-complete pause on
// its own short delay; the hand advances once the human continues too —
// without waiting for the 14s fallback.
func TestBot_AcksScoreReveal(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	mgr.SetBotDelayForTest(time.Millisecond, 2*time.Millisecond)

	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", mixedPlayers(1, 2, 3), "relaxed", 0, 10, 120, 0))

	gs := markBots(testfixtures.NewGameMidPlay(8), 1, 2, 3)
	gs.Phase = game.PhaseHandComplete
	gs.LastHandResult = &game.HandScore{TeamAHandTotal: 100, TeamBHandTotal: 72}
	gs.TurnExpiresAt = nil
	mgr.SetGameStateForTest(100, gs)
	mgr.SetHandCompleteExpiresAtForTest(100, time.Now().Add(14*time.Second))
	mgr.BotSchedule(100)

	botsAcked := waitFor(2*time.Second, func() bool {
		st := mgr.GetStateSnapshot(100)
		return st != nil && st.HandCompleteReady[1] && st.HandCompleteReady[2] && st.HandCompleteReady[3]
	})
	require.True(t, botsAcked, "all three bots must acknowledge the score reveal")

	st := mgr.GetStateSnapshot(100)
	assert.Equal(t, game.PhaseHandComplete, st.Phase, "the reveal waits for the human")

	// Human continues — the next hand deals immediately (no 14s fallback).
	raw, _ := json.Marshal(map[string]string{})
	mgr.HandleAction(&ws.Client{UserID: 10}, ws.WSMessage{Type: "action:continue", Payload: raw})

	advanced := waitFor(2*time.Second, func() bool {
		st := mgr.GetStateSnapshot(100)
		return st != nil && st.HandNumber == 2
	})
	assert.True(t, advanced, "hand must advance once every connected seat acked")
}

// TestBot_ResilienceIsolation pins AC6: a human disconnect pauses the table
// (bots do not act during it), bot seats never receive reconnect windows,
// and bots resume acting after the human returns.
func TestBot_ResilienceIsolation(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	mgr.SetBotDelayForTest(10*time.Millisecond, 20*time.Millisecond)

	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", mixedPlayers(1, 2, 3), "relaxed", 0, 10, 120, 0))

	// Park the table mid-play with the bot at seat 1 on turn.
	gs := markBots(testfixtures.NewGameMidPlay(1), 1, 2, 3)
	gs.ActivePlayerSeat = 1
	mgr.SetGameStateForTest(100, gs)

	// Human (seat 0) drops before the bot's think delay is scheduled.
	mgr.HandleDisconnect(10)

	st := mgr.GetStateSnapshot(100)
	require.NotNil(t, st)
	require.Equal(t, game.PhaseDisconnected, st.Phase)
	require.NotNil(t, st.PlayerReconnectExpiresAt[0], "human seat gets a reconnect window")
	for seat := 1; seat <= 3; seat++ {
		assert.Nil(t, st.PlayerReconnectExpiresAt[seat], "bot seats never get reconnect timers")
		assert.True(t, st.Players[seat].Connected, "bots never read as disconnected")
	}

	// While disconnected, bots must not act.
	mgr.BotSchedule(100)
	time.Sleep(100 * time.Millisecond)
	st = mgr.GetStateSnapshot(100)
	assert.Equal(t, game.PhaseDisconnected, st.Phase)
	assert.Empty(t, st.CurrentTrick, "no bot card may land while the table is disconnected")

	// Human returns: phase restores and the bot on turn resumes acting.
	mgr.HandleReconnect(10)

	resumed := waitFor(2*time.Second, func() bool {
		st := mgr.GetStateSnapshot(100)
		if st == nil {
			return false
		}
		return st.Phase == game.PhasePlaying && (len(st.CurrentTrick) > 0 || st.ActivePlayerSeat != 1)
	})
	assert.True(t, resumed, "bot must resume acting after the human reconnects")
}

// TestBot_HumanOnlyMatchSchedulesNothing pins the cheap no-op: matches with
// no bots never allocate bot state and never schedule bot timers.
func TestBot_HumanOnlyMatchSchedulesNothing(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	mgr.SetBotDelayForTest(time.Millisecond, time.Millisecond)

	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0))

	time.Sleep(100 * time.Millisecond)
	st := mgr.GetStateSnapshot(100)
	require.NotNil(t, st)
	assert.Equal(t, game.PhaseBidding, st.Phase)
	assert.Equal(t, 0, st.BiddingPassCount, "nothing may act for a human seat")
	assert.Equal(t, 1, st.ActivePlayerSeat)
}
