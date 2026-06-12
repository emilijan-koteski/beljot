package bot_test

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/bot"
	"github.com/emilijan/beljot/server/internal/game"
)

// TestSimulation_HeuristicBeatsRandomBaseline is the AC4 evidence: seats 0+2
// (team A) play bot.Decide, seats 1+3 (team B) play a random-legal baseline,
// across ≥200 full hands driven purely through game.NewGame/ApplyAction (no
// session manager, no WS). The heuristic team must accumulate ≥60% of all
// points awarded — deliberately far below the expected true rate so CI never
// flakes. Zero ApplyAction errors across the whole simulation doubles as the
// AC3 "always legal" proof.
//
// The baseline uses a seeded source (math/rand/v2 has no global Seed); deck
// shuffles stay nondeterministic, which is fine at this sample size.
func TestSimulation_HeuristicBeatsRandomBaseline(t *testing.T) {
	const handsTarget = 200
	rng := rand.New(rand.NewPCG(42, 1))

	heuristicPoints := 0
	totalPoints := 0
	handsPlayed := 0

	for handsPlayed < handsTarget {
		gs := game.NewGame(
			[4]uint{0, 0, 0, 0},
			[4]string{"", "", "", ""},
			[4]bool{true, true, true, true},
			game.VariantBitola, "1001", 1,
		)
		mem := bot.NewMemory()

		result, ok := playOneHand(t, gs, mem, rng)
		if !ok {
			continue // instant-win deal — no hand was scored
		}
		heuristicPoints += result.TeamAHandTotal
		totalPoints += result.TeamAHandTotal + result.TeamBHandTotal
		handsPlayed++
	}

	require.Positive(t, totalPoints)
	share := float64(heuristicPoints) / float64(totalPoints)
	t.Logf("heuristic share over %d hands: %.1f%% (%d/%d points)",
		handsPlayed, share*100, heuristicPoints, totalPoints)
	assert.GreaterOrEqual(t, share, 0.60,
		"heuristic must take at least 60%% of all points vs the random baseline")
}

// playOneHand drives a fresh deal to its scored end and returns the hand
// result. Returns ok=false when the deal ended the match without scoring a
// hand (instant-win). Every ApplyAction error fails the test immediately —
// that is the AC3 always-legal contract.
func playOneHand(t *testing.T, gs *game.GameState, mem *bot.Memory, rng *rand.Rand) (game.HandScore, bool) {
	t.Helper()

	// Safety bound: a hand resolves in well under 100 actions; repeated
	// all-pass reshuffles add a few more rounds.
	for steps := 0; steps < 500; steps++ {
		switch gs.Phase {
		case game.PhaseDealing:
			// Mirror the session manager's dealing → bidding auto-transition.
			gs.Phase = game.PhaseBidding

		case game.PhaseBidding, game.PhasePlaying:
			seat, action := nextSimAction(gs, mem, rng)
			oldLead := gs.LeadSuit
			next, err := game.ApplyAction(gs, action)
			require.NoError(t, err,
				"ApplyAction must never reject a simulated action (seat %d, %s)", seat, action.Type)
			if action.Type == game.ActionPlayCard {
				mem.ObservePlay(seat, *action.Card, oldLead)
			}
			gs = next

		case game.PhaseHandComplete:
			require.NotNil(t, gs.LastHandResult)
			return *gs.LastHandResult, true

		case game.PhaseMatchEnd:
			if gs.LastHandResult == nil {
				return game.HandScore{}, false // instant-win, nothing scored
			}
			return *gs.LastHandResult, true

		default:
			t.Fatalf("simulation reached unexpected phase %q", gs.Phase)
		}
	}
	t.Fatal("simulation hand did not terminate within the step bound")
	return game.HandScore{}, false
}

// nextSimAction picks the acting seat and its action: team A (seats 0/2) via
// bot.Decide over a redacted view, team B (seats 1/3) via random-legal moves.
func nextSimAction(gs *game.GameState, mem *bot.Memory, rng *rand.Rand) (int, game.Action) {
	seat := gs.ActivePlayerSeat
	if gs.Phase == game.PhasePlaying && gs.PendingBelotSeat != nil {
		seat = *gs.PendingBelotSeat
	}
	if seat%2 == 0 {
		return seat, bot.Decide(viewFromState(gs, seat, mem))
	}
	return seat, randomLegalAction(gs, seat, rng)
}

// randomLegalAction picks uniformly among the legal options at the seat's
// current decision point.
func randomLegalAction(gs *game.GameState, seat int, rng *rand.Rand) game.Action {
	if gs.Phase == game.PhaseBidding {
		if gs.BiddingRound == 1 {
			if rng.IntN(2) == 0 {
				return game.Action{Type: game.ActionPickTrump, PlayerSeat: seat}
			}
			return game.Action{Type: game.ActionPassTrump, PlayerSeat: seat}
		}
		// Round 2: pass or any non-candidate suit, uniformly.
		options := make([]game.Suit, 0, 3)
		for _, s := range game.AllSuits {
			if gs.TrumpCandidate != nil && s == gs.TrumpCandidate.Suit {
				continue
			}
			options = append(options, s)
		}
		pick := rng.IntN(len(options) + 1)
		if pick == len(options) {
			return game.Action{Type: game.ActionPassTrump, PlayerSeat: seat}
		}
		s := options[pick]
		return game.Action{Type: game.ActionPickTrump, PlayerSeat: seat, Suit: &s}
	}

	if gs.PendingBelotSeat != nil && *gs.PendingBelotSeat == seat {
		if rng.IntN(2) == 0 {
			return game.Action{Type: game.ActionAnnounceBelot, PlayerSeat: seat}
		}
		return game.Action{Type: game.ActionSkipBelot, PlayerSeat: seat}
	}

	if gs.AwaitingDeclaration && gs.ActivePlayerSeat == seat {
		if rng.IntN(2) == 0 {
			return game.Action{Type: game.ActionDeclare, PlayerSeat: seat}
		}
		return game.Action{Type: game.ActionSkipDeclare, PlayerSeat: seat}
	}

	legal := game.LegalCards(gs, seat)
	c := legal[rng.IntN(len(legal))]
	return game.Action{Type: game.ActionPlayCard, PlayerSeat: seat, Card: &c}
}
