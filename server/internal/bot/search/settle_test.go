package search_test

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/bot/search"
	"github.com/emilijan/beljot/server/internal/game"
)

func TestSettleHandCases(t *testing.T) {
	const (
		callerA = game.TeamA
		callerB = game.TeamB
	)
	tests := []struct {
		name    string
		settle  search.Settlement
		remDiff int
		wantOn  int
		wantOff int
	}{
		{
			name: "caller A makes the contract and banks the card and declaration difference",
			settle: search.Settlement{
				HandPoints: [2]int{90, 42}, DeclPoints: [2]int{20, 0}, TricksWon: [2]int{4, 3},
				CallerTeam: callerA, RemainingCardPoints: 20,
			},
			remDiff: 30, wantOn: 98, wantOff: 98,
		},
		{
			name: "caller B makes the contract on the strength of its declarations",
			settle: search.Settlement{
				HandPoints: [2]int{60, 72}, DeclPoints: [2]int{0, 50}, TricksWon: [2]int{3, 4},
				CallerTeam: callerB, RemainingCardPoints: 20,
			},
			remDiff: -30, wantOn: -92, wantOff: -92,
		},
		{
			name: "caller A makes the contract by two points",
			settle: search.Settlement{
				HandPoints: [2]int{60, 52}, DeclPoints: [2]int{20, 0}, TricksWon: [2]int{3, 4},
				CallerTeam: callerA, RemainingCardPoints: 40,
			},
			remDiff: -26, wantOn: 2, wantOff: 2,
		},
		{
			name: "caller A ties and the opponents take the whole hand",
			settle: search.Settlement{
				HandPoints: [2]int{60, 52}, DeclPoints: [2]int{20, 0}, TricksWon: [2]int{3, 4},
				CallerTeam: callerA, RemainingCardPoints: 40,
			},
			remDiff: -28, wantOn: -182, wantOff: -162,
		},
		{
			name: "caller B ties and the opponents take the whole hand",
			settle: search.Settlement{
				HandPoints: [2]int{60, 52}, DeclPoints: [2]int{0, 20}, TricksWon: [2]int{3, 4},
				CallerTeam: callerB, RemainingCardPoints: 40,
			},
			remDiff: 12, wantOn: 182, wantOff: 162,
		},
		{
			name: "caller B fails and team A banks the whole hand",
			settle: search.Settlement{
				HandPoints: [2]int{70, 42}, DeclPoints: [2]int{20, 50}, TricksWon: [2]int{4, 3},
				CallerTeam: callerB, RemainingCardPoints: 40,
			},
			remDiff: 30, wantOn: 232, wantOff: 182,
		},
		{
			name: "caller A fails and team B banks the whole hand",
			settle: search.Settlement{
				HandPoints: [2]int{30, 82}, DeclPoints: [2]int{50, 40}, TricksWon: [2]int{3, 4},
				CallerTeam: callerA, RemainingCardPoints: 40,
			},
			remDiff: -30, wantOn: -252, wantOff: -202,
		},
		{
			name: "team A wins every trick and banks the whole hand with the Capot bonus",
			settle: search.Settlement{
				HandPoints: [2]int{132, 0}, DeclPoints: [2]int{20, 50}, TricksWon: [2]int{7, 0},
				CallerTeam: callerB, RemainingCardPoints: 20,
			},
			remDiff: 120, wantOn: 322, wantOff: 272,
		},
		{
			name: "team B wins every trick and banks the whole hand with the Capot bonus",
			settle: search.Settlement{
				HandPoints: [2]int{0, 132}, DeclPoints: [2]int{20, 50}, TricksWon: [2]int{0, 7},
				CallerTeam: callerA, RemainingCardPoints: 20,
			},
			remDiff: -120, wantOn: -322, wantOff: -302,
		},
		{
			name: "team A wins seven tricks and loses the last so there is no Capot and caller A makes it",
			settle: search.Settlement{
				HandPoints: [2]int{132, 0}, TricksWon: [2]int{7, 0},
				CallerTeam: callerA, RemainingCardPoints: 20,
			},
			remDiff: -30, wantOn: 102, wantOff: 102,
		},
		{
			name: "team A wins seven tricks and loses the last so there is no Capot and caller B falls",
			settle: search.Settlement{
				HandPoints: [2]int{132, 0}, TricksWon: [2]int{7, 0},
				CallerTeam: callerB, RemainingCardPoints: 20,
			},
			remDiff: -30, wantOn: 162, wantOff: 162,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			on := tt.settle
			on.DeclarationsTransfer = true
			off := tt.settle
			off.DeclarationsTransfer = false
			assert.Equal(t, tt.wantOn, search.Settle(on, tt.remDiff), "declarations transfer on")
			assert.Equal(t, tt.wantOff, search.Settle(off, tt.remDiff), "declarations transfer off")
		})
	}
}

type settledHand struct {
	settlement search.Settlement
	value      int
	result     *game.HandScore
}

type settleStats struct {
	hands             int
	failedContracts   int
	failedTransferOn  int
	failedTransferOff int
	capots            int
	withDeclarations  int
}

func settlementAtTheLastTrick(gs *game.GameState, solver *search.Solver) (search.Settlement, int) {
	s := search.Settlement{
		HandPoints:           gs.HandPoints,
		TricksWon:            gs.TricksWon,
		CallerTeam:           game.TeamForSeat(*gs.TrumpCallerSeat),
		DeclarationsTransfer: gs.Rules.DeclarationsTransfer,
	}
	for team := range s.DeclPoints {
		s.DeclPoints[team] = gs.DeclarationPoints[team] + gs.BelotPoints[team]
	}
	trump := search.SuitIdx(*gs.TrumpSuit)
	hands := handMasks(gs)
	for _, hand := range hands {
		for _, card := range search.CardsOf(hand) {
			s.RemainingCardPoints += search.Points(search.CardIdx(card), trump)
		}
	}
	value := solver.Value(hands, gs.ActivePlayerSeat, nil, search.CapotFlag(gs.TricksWon))
	return s, value
}

func strongestLegalCard(gs *game.GameState, seat int) *game.Card {
	trump := *gs.TrumpSuit
	var best game.Card
	bestStrength := -1
	for _, card := range game.LegalCards(gs, seat) {
		strength := game.NonTrumpRankOrder[card.Rank]
		if card.Suit == trump {
			strength = 100 + game.TrumpRankOrder[card.Rank]
		}
		if strength > bestStrength {
			best, bestStrength = card, strength
		}
	}
	return &best
}

func playUntilSettled(t *testing.T, gs *game.GameState, rng *rand.Rand, dominantTeam int) (settledHand, bool) {
	t.Helper()

	var hand settledHand
	captured := false
	for steps := 0; steps < 500; steps++ {
		switch gs.Phase {
		case game.PhaseDealing:
			gs.Phase = game.PhaseBidding
			continue
		case game.PhaseBidding, game.PhaseDeclaring, game.PhasePlaying:
		default:
			hand.result = gs.LastHandResult
			return hand, captured && !gs.StoppedAtTarget && hand.result != nil
		}

		seat, action := randomAction(t, gs, rng)
		if action.Type == game.ActionPlayCard {
			if game.TeamForSeat(seat) == dominantTeam {
				action.Card = strongestLegalCard(gs, seat)
			}
			if gs.TrickNumber == 8 && len(gs.CurrentTrick) == 0 {
				solver := search.NewSolver(search.SuitIdx(*gs.TrumpSuit), gs.Clockwise)
				hand.settlement, hand.value = settlementAtTheLastTrick(gs, solver)
				captured = true
			}
		}
		next, err := game.ApplyAction(gs, action)
		require.NoError(t, err, "ApplyAction rejected a legal action (seat %d, %s)", seat, action.Type)
		gs = next
	}
	t.Fatal("the hand did not terminate within the step bound")
	return hand, false
}

func TestSettleMatchesEngineAtTheLastTrick(t *testing.T) {
	const handsTarget = 300
	rng := rand.New(rand.NewPCG(2026, 25))
	var stats settleStats

	for attempts := 0; stats.hands < handsTarget; attempts++ {
		require.Less(t, attempts, 2*handsTarget, "too many deals ended before the last trick")
		variant := game.VariantBitola
		if stats.hands%2 == 1 {
			variant = game.VariantCroatia
		}
		transfer := stats.hands/2%2 == 0
		clockwise := stats.hands/4%2 == 1
		dominantTeam := rng.IntN(2)

		gs := game.NewGame(
			[4]uint{}, [4]string{}, [4]bool{true, true, true, true},
			variant, "1001", 1, true, false, transfer, clockwise,
		)
		hand, ok := playUntilSettled(t, gs, rng, dominantTeam)
		if !ok {
			continue
		}

		result := hand.result
		want := result.TeamAHandTotal - result.TeamBHandTotal
		got := search.Settle(hand.settlement, hand.value)
		require.Equal(t, want, got, "variant %s transfer %v clockwise %v settlement %+v value %d result %+v",
			variant, transfer, clockwise, hand.settlement, hand.value, *result)

		stats.hands++
		if result.FailedContract {
			stats.failedContracts++
			if transfer {
				stats.failedTransferOn++
			} else {
				stats.failedTransferOff++
			}
		}
		if result.Capot {
			stats.capots++
		}
		if result.TeamADeclPoints+result.TeamBDeclPoints > 0 {
			stats.withDeclarations++
		}
	}

	assert.Positive(t, stats.failedTransferOn, "no hand failed its contract with declarations transferring")
	assert.Positive(t, stats.failedTransferOff, "no hand failed its contract with declarations not transferring")
	assert.Positive(t, stats.capots, "no hand ended in a Capot")
	assert.Positive(t, stats.withDeclarations, "no hand carried declarations")
	t.Logf("%d hands: %d failed contracts (%d transfer on, %d transfer off), %d capots, %d with declarations",
		stats.hands, stats.failedContracts, stats.failedTransferOn, stats.failedTransferOff, stats.capots,
		stats.withDeclarations)
}
