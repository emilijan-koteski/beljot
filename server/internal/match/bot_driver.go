package match

import (
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/emilijan/beljot/server/internal/bot"
	"github.com/emilijan/beljot/server/internal/game"
)

// maybeScheduleBotAction arms think-delay timers for every bot seat that owes a
// decision. Call it after every game-state replacement; a missed call site stalls the match.
func (m *Manager) maybeScheduleBotAction(session *LiveMatch) {
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.closed || session.botMemory == nil {
		return
	}
	gs := session.gameState
	for _, seat := range botDecisionSeats(gs) {
		if session.botActionTimers[seat] != nil {
			continue
		}
		delay := m.botThinkDelay(gs, seat) + session.dealGraceRemaining()
		gen := session.botActionGenerations[seat]
		ctx := botDecisionContextFor(gs, seat)
		s := seat
		session.botActionTimers[seat] = time.AfterFunc(delay, func() {
			m.handleBotActionTimer(session, s, gen, ctx)
		})
	}
}

// botDecisionContext identifies the decision point a think delay was armed for.
// If it changed by the time the timer fires, the delay is re-armed instead of acting.
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

// botDecisionSeats resolves which bot seats owe a decision in the given state,
// including every unanswered seat while declaring and a surrender proposer's partner.
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
	case game.PhaseDeclaring:
		for seat := range gs.Players {
			if !gs.Players[seat].DeclarationAnswered {
				add(seat)
			}
		}
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

const botBelotBeat = 150 * time.Millisecond

// botThinkDelay returns the humanized delay before the bot at seat acts: random in
// [botDelayMin, botDelayMax] for bids and plays, botDelayMin for fixed-window phases,
// and botBelotBeat for its own Belote answer, already decided when it played the card.
func (m *Manager) botThinkDelay(gs *game.GameState, seat int) time.Duration {
	if gs.Phase == game.PhasePlaying && gs.PendingBelotSeat != nil && *gs.PendingBelotSeat == seat {
		return min(botBelotBeat, m.botDelayMax)
	}
	phase := gs.Phase
	if phase == game.PhaseHandComplete || phase == game.PhaseDeclaring {
		return m.botDelayMin
	}
	spread := m.botDelayMax - m.botDelayMin
	if spread <= 0 {
		return m.botDelayMin
	}
	return m.botDelayMin + time.Duration(rand.Int64N(int64(spread)+1))
}

// handleBotActionTimer verifies and decides inside the same critical section that
// applies the action. A rejected or stale decision re-arms the seat so the match never stalls.
func (m *Manager) handleBotActionTimer(session *LiveMatch, seat int, generation uint64, armedCtx botDecisionContext) {
	var (
		rearm      bool
		actionType string
	)
	err := m.applyAndBroadcastActionWith(session, func(gs *game.GameState) (game.Action, bool) {
		if session.botActionGenerations[seat] != generation {
			return game.Action{}, false
		}
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
			rearm = true
			return game.Action{}, false
		}

		var action game.Action
		if gs.Phase == game.PhaseHandComplete {
			action = game.Action{Type: game.ActionContinue, PlayerSeat: seat}
		} else {
			action = bot.Decide(buildBotView(gs, seat, session.botMemory))
		}
		actionType = action.Type
		return action, true
	})
	if err != nil {
		slog.Warn("bot: action rejected by engine; rescheduling seat",
			"roomID", session.roomID, "seat", seat, "type", actionType, "error", err)
		rearm = true
	}
	if rearm {
		m.maybeScheduleBotAction(session)
	}
}

// buildBotView projects the seat-local, redacted View handed to bot.Decide, so it
// never sees other players' hands. Call under session.mu.
func buildBotView(gs *game.GameState, seat int, mem *bot.Memory) bot.View {
	v := bot.View{
		Seat:                seat,
		Hand:                gs.Players[seat].Hand,
		Phase:               gs.Phase,
		BiddingRound:        gs.BiddingRound,
		TrumpCandidate:      gs.TrumpCandidate,
		TrumpSuit:           gs.TrumpSuit,
		TrumpCallerSeat:     gs.TrumpCallerSeat,
		HasTrumpCandidate:   gs.Rules.HasTrumpCandidate,
		DealerSeat:          gs.DealerSeat,
		CurrentTrick:        gs.CurrentTrick,
		LeadSuit:            gs.LeadSuit,
		ActivePlayerSeat:    gs.ActivePlayerSeat,
		AwaitingDeclaration: gs.AwaitingDeclaration && gs.ActivePlayerSeat == seat,
		HasDeclarations:     gs.Phase == game.PhaseDeclaring && game.HasDeclarableCombinations(gs, seat),
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
	v.MustPickTrump = game.MustPickTrump(gs, seat)
	if mem != nil {
		v.PlayedCards = mem.PlayedCards()
		v.KnownVoids = mem.KnownVoids()
		v.KnownCards = mem.KnownCards()
		v.DeclaredCards = mem.DeclaredCards()
	}
	return v
}

// observeBotMemory updates the per-match bot memory after a successful transition,
// taking session.mu itself.
func (m *Manager) observeBotMemory(session *LiveMatch, oldState, newState *game.GameState, action game.Action) {
	session.mu.Lock()
	defer session.mu.Unlock()
	m.observeBotMemoryLocked(session, oldState, newState, action)
}

// observeBotMemoryLocked records card plays, hand resets and revealed declarations in the
// bot memory. The caller holds session.mu; no-op for human-only matches.
func (m *Manager) observeBotMemoryLocked(session *LiveMatch, oldState, newState *game.GameState, action game.Action) {
	mem := session.botMemory
	if mem == nil {
		return
	}
	if action.Type == game.ActionPlayCard && action.Card != nil {
		mem.ObservePlay(action.PlayerSeat, *action.Card, oldState.LeadSuit, oldState.TrumpSuit)
	}
	mem.SyncHand(newState.HandNumber)
	if action.Type == game.ActionPickTrump && oldState.TrumpCandidate != nil {
		mem.ObserveLocatedCard(action.PlayerSeat, *oldState.TrumpCandidate)
	}
	if action.Type == game.ActionAnnounceBelot && newState.TrumpSuit != nil {
		mem.ObserveLocatedCard(action.PlayerSeat, game.Card{Rank: game.RankKing, Suit: *newState.TrumpSuit})
		mem.ObserveLocatedCard(action.PlayerSeat, game.Card{Rank: game.RankQueen, Suit: *newState.TrumpSuit})
	}
	if newState.DeclarationsResolved && newState.Rules.DeclarationsEnabled {
		mem.ObserveDeclarations(newState.Players)
	}
}
