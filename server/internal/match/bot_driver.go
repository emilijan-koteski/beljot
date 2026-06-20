package match

import (
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/emilijan/beljot/server/internal/bot"
	"github.com/emilijan/beljot/server/internal/game"
)

// Bot driver (Story 10.3). Bots are server-side actors driven by the session
// manager — no WebSocket, no client, no auth. Decision logic is the pure
// bot.Decide over a redacted View; this file owns the side effects:
// scheduling, humanized think delays, memory upkeep, and applying decisions
// through the exact apply-and-broadcast path human actions use.

// maybeScheduleBotAction arms think-delay timers for every bot seat that
// currently owes a decision. It must be invoked after EVERY point the game
// state is replaced and broadcast (StartMatch, action success, timer-expiry
// auto-actions, hand-complete force-advance, reconnect resume) — a missed
// call site is a silently stalled match. Cheap no-op for human-only matches.
func (m *Manager) maybeScheduleBotAction(session *LiveMatch) {
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.closed || session.botMemory == nil {
		return
	}
	gs := session.gameState
	for _, seat := range botDecisionSeats(gs) {
		if session.botActionTimers[seat] != nil {
			continue // a think delay is already pending for this seat
		}
		delay := m.botThinkDelay(gs.Phase)
		gen := session.botActionGenerations[seat]
		ctx := botDecisionContextFor(gs, seat)
		s := seat
		session.botActionTimers[seat] = time.AfterFunc(delay, func() {
			m.handleBotActionTimer(session, s, gen, ctx)
		})
	}
}

// botDecisionContext captures WHICH decision point a think delay was armed
// for. If the state has moved to a DIFFERENT decision point by the time the
// timer fires (e.g. a hand-complete ack delay elapsing just after the next
// hand's bidding opened for the same seat), acting immediately would undercut
// the ≥1 s humanization floor — the fire path compares contexts and re-arms a
// fresh delay instead of acting.
type botDecisionContext struct {
	phase            game.Phase
	handNumber       int
	pendingBelot     bool
	awaitingDecl     bool
	surrenderPending bool
}

func botDecisionContextFor(gs *game.GameState, seat int) botDecisionContext {
	return botDecisionContext{
		phase:            gs.Phase,
		handNumber:       gs.HandNumber,
		pendingBelot:     gs.PendingBelotSeat != nil && *gs.PendingBelotSeat == seat,
		awaitingDecl:     gs.AwaitingDeclaration && gs.ActivePlayerSeat == seat,
		surrenderPending: gs.SurrenderProposerSeat != nil && (*gs.SurrenderProposerSeat+2)%4 == seat,
	}
}

// botDecisionSeats resolves which BOT seats owe a decision in the given
// state. Phase table per the story: bidding → active bidder; playing →
// pending belote seat, else active player (covers the declaration prompt —
// it always belongs to the active player); hand_complete → every bot that
// has not acknowledged the score reveal; a pending surrender adds the
// proposer's partner (surrender is team-internal — bots never initiate and
// never respond to opponents' proposals). Paused / disconnected / match_end
// never schedule.
func botDecisionSeats(gs *game.GameState) []int {
	seats := make([]int, 0, 4)
	add := func(seat int) {
		if seat < 0 || seat > 3 || !gs.Players[seat].IsBot {
			return
		}
		for _, s := range seats {
			if s == seat {
				return
			}
		}
		seats = append(seats, seat)
	}

	switch gs.Phase {
	case game.PhaseBidding:
		add(gs.ActivePlayerSeat)
	case game.PhasePlaying:
		if gs.PendingBelotSeat != nil {
			add(*gs.PendingBelotSeat)
		} else {
			add(gs.ActivePlayerSeat)
		}
	case game.PhaseHandComplete:
		for seat := range gs.Players {
			if !gs.HandCompleteReady[seat] {
				add(seat)
			}
		}
		return seats
	default:
		return seats
	}

	if gs.SurrenderProposerSeat != nil {
		add((*gs.SurrenderProposerSeat + 2) % 4)
	}
	return seats
}

// botThinkDelay returns the humanized delay before a bot acts: uniform
// random in [botDelayMin, botDelayMax] for game decisions, a single short
// beat (botDelayMin) for score-reveal acknowledgements — humans plus the
// existing 14 s fallback still pace the reveal.
func (m *Manager) botThinkDelay(phase game.Phase) time.Duration {
	if phase == game.PhaseHandComplete {
		return m.botDelayMin
	}
	spread := m.botDelayMax - m.botDelayMin
	if spread <= 0 {
		return m.botDelayMin
	}
	return m.botDelayMin + time.Duration(rand.Int64N(int64(spread)+1))
}

// handleBotActionTimer fires when a bot's think delay elapses. Verification
// (generation, the seat STILL owing a decision, the decision point being the
// one the delay was sized for) and the bot.Decide call all run inside the
// build callback — i.e. inside the SAME critical section that applies the
// action — so no state change can slip between verify and act. Bot actions
// carry no autoPlayed marker — event:card_played looks identical to a human
// play. The per-move turn timer stays armed throughout the think delay as the
// safety net; relaxed rooms have none, so an engine rejection re-schedules
// the seat instead of silently stalling.
func (m *Manager) handleBotActionTimer(session *LiveMatch, seat int, generation uint64, armedCtx botDecisionContext) {
	var (
		rearm      bool
		actionType string
	)
	err := m.applyAndBroadcastActionWith(session, func(gs *game.GameState) (game.Action, bool) {
		if session.botActionGenerations[seat] != generation {
			return game.Action{}, false
		}
		// Consume this firing: bump the generation so a stale duplicate can
		// never double-fire, and clear the slot so the next schedule pass can
		// re-arm.
		session.botActionGenerations[seat]++
		session.botActionTimers[seat] = nil

		still := false
		for _, s := range botDecisionSeats(gs) {
			if s == seat {
				still = true
				break
			}
		}
		if !still {
			return game.Action{}, false
		}
		if botDecisionContextFor(gs, seat) != armedCtx {
			// The seat owes a decision, but a DIFFERENT one than this delay
			// was armed for (e.g. a hand-complete ack delay firing right after
			// the next hand's bidding opened). Acting now would undercut the
			// ≥1 s humanization floor — re-arm a fresh delay instead.
			rearm = true
			return game.Action{}, false
		}

		var action game.Action
		if gs.Phase == game.PhaseHandComplete {
			// Nothing to decide — the bot acknowledges the score reveal.
			action = game.Action{Type: game.ActionContinue, PlayerSeat: seat}
		} else {
			action = bot.Decide(buildBotView(gs, seat, session.botMemory))
		}
		actionType = action.Type
		return action, true
	})
	if err != nil {
		// The seat still owes its decision and nothing else will re-schedule
		// it (rejections outside a racing state change have no other wake-up;
		// relaxed rooms have no auto-play backstop) — re-arm so the match
		// never stalls on a dropped bot action.
		slog.Warn("bot: action rejected by engine; rescheduling seat",
			"roomID", session.roomID, "seat", seat, "type", actionType, "error", err)
		rearm = true
	}
	if rearm {
		m.maybeScheduleBotAction(session)
	}
}

// buildBotView projects the seat-local, redacted View handed to bot.Decide.
// The redaction is structural no-peeking: only this seat's hand and public
// state cross the boundary — Decide never receives other players' hands even
// though gs has them. Must be called under session.mu (reads gameState +
// botMemory).
func buildBotView(gs *game.GameState, seat int, mem *bot.Memory) bot.View {
	v := bot.View{
		Seat:                seat,
		Hand:                gs.Players[seat].Hand,
		Phase:               gs.Phase,
		BiddingRound:        gs.BiddingRound,
		TrumpCandidate:      gs.TrumpCandidate,
		TrumpSuit:           gs.TrumpSuit,
		TrumpCallerSeat:     gs.TrumpCallerSeat,
		DealerSeat:          gs.DealerSeat,
		CurrentTrick:        gs.CurrentTrick,
		LeadSuit:            gs.LeadSuit,
		ActivePlayerSeat:    gs.ActivePlayerSeat,
		AwaitingDeclaration: gs.AwaitingDeclaration && gs.ActivePlayerSeat == seat,
		PendingBelot:        gs.PendingBelotSeat != nil && *gs.PendingBelotSeat == seat,
		TeamScores:          gs.TeamScores,
		HandPoints:          gs.HandPoints,
		TricksWon:           gs.TricksWon,
	}
	if gs.SurrenderProposerSeat != nil && (*gs.SurrenderProposerSeat+2)%4 == seat {
		v.PartnerProposedSurrender = true
	}
	if gs.Phase == game.PhasePlaying {
		v.LegalCards = game.LegalCards(gs, seat)
	}
	if mem != nil {
		v.PlayedCards = mem.PlayedCards()
		v.KnownVoids = mem.KnownVoids()
		v.KnownCards = mem.KnownCards()
	}
	return v
}

// observeBotMemory keeps the per-match bot memory current after a successful
// state transition: resolved card plays feed the seen-cards + void inference
// (the OLD state's LeadSuit tells the void story), a hand-number advance resets
// the per-hand sets, and a resolved declaration contest records the publicly
// revealed cards as known holdings. No-op for human-only matches.
func (m *Manager) observeBotMemory(session *LiveMatch, oldState, newState *game.GameState, action game.Action) {
	session.mu.Lock()
	defer session.mu.Unlock()
	mem := session.botMemory
	if mem == nil {
		return
	}
	if action.Type == game.ActionPlayCard && action.Card != nil {
		mem.ObservePlay(action.PlayerSeat, *action.Card, oldState.LeadSuit)
	}
	mem.SyncHand(newState.HandNumber)
	// Once the contest resolves, the engine has nil'd the losing team's
	// declarations, so this records only the publicly revealed winning cards.
	if newState.DeclarationsResolved {
		mem.ObserveDeclarations(newState.Players)
	}
}
