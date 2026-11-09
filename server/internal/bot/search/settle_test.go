package search_test

import (
	"fmt"
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
			name: "caller A ties under the Croatian rule and the opponents take the whole hand",
			settle: search.Settlement{
				HandPoints: [2]int{60, 52}, DeclPoints: [2]int{20, 0}, TricksWon: [2]int{3, 4},
				CallerTeam: callerA, TieRule: game.TieRuleAllToOpponents, RemainingCardPoints: 40,
			},
			remDiff: -28, wantOn: -182, wantOff: -162,
		},
		{
			name: "caller B ties under the Croatian rule and the opponents take the whole hand",
			settle: search.Settlement{
				HandPoints: [2]int{60, 52}, DeclPoints: [2]int{0, 20}, TricksWon: [2]int{3, 4},
				CallerTeam: callerB, TieRule: game.TieRuleAllToOpponents, RemainingCardPoints: 40,
			},
			remDiff: 12, wantOn: 182, wantOff: 162,
		},
		{
			name: "caller A ties under the Bitola rule and the opponents bank only their own half",
			settle: search.Settlement{
				HandPoints: [2]int{60, 52}, DeclPoints: [2]int{20, 0}, TricksWon: [2]int{3, 4},
				CallerTeam: callerA, TieRule: game.TieRuleHangingPoints, RemainingCardPoints: 40,
			},
			remDiff: -28, wantOn: -91, wantOff: -91,
		},
		{
			name: "caller B ties under the Bitola rule and the opponents bank only their own half",
			settle: search.Settlement{
				HandPoints: [2]int{60, 52}, DeclPoints: [2]int{0, 20}, TricksWon: [2]int{3, 4},
				CallerTeam: callerB, TieRule: game.TieRuleHangingPoints, RemainingCardPoints: 40,
			},
			remDiff: 12, wantOn: 91, wantOff: 91,
		},
		{
			name: "caller B ties with team B's 81 hanging, so team A takes it as well",
			settle: search.Settlement{
				HandPoints: [2]int{60, 52}, DeclPoints: [2]int{0, 20}, TricksWon: [2]int{3, 4},
				CallerTeam: callerB, TieRule: game.TieRuleHangingPoints, RemainingCardPoints: 40,
				Hanging: &game.HangingPoints{Amount: 81, Team: game.TeamB},
			},
			remDiff: 12, wantOn: 172, wantOff: 172,
		},
		{
			name: "caller A makes the contract by two points and takes its own hanging 81 back",
			settle: search.Settlement{
				HandPoints: [2]int{60, 52}, DeclPoints: [2]int{20, 0}, TricksWon: [2]int{3, 4},
				CallerTeam: callerA, TieRule: game.TieRuleHangingPoints, RemainingCardPoints: 40,
				Hanging: &game.HangingPoints{Amount: 81, Team: game.TeamA},
			},
			remDiff: -26, wantOn: 83, wantOff: 83,
		},
		{
			name: "caller A fails and team B takes the whole hand and A's hanging 81",
			settle: search.Settlement{
				HandPoints: [2]int{30, 82}, DeclPoints: [2]int{50, 40}, TricksWon: [2]int{3, 4},
				CallerTeam: callerA, TieRule: game.TieRuleHangingPoints, RemainingCardPoints: 40,
				Hanging: &game.HangingPoints{Amount: 81, Team: game.TeamA},
			},
			remDiff: -30, wantOn: -333, wantOff: -283,
		},
		{
			name: "team A wins every trick and takes the whole hand and B's hanging 81",
			settle: search.Settlement{
				HandPoints: [2]int{132, 0}, DeclPoints: [2]int{20, 50}, TricksWon: [2]int{7, 0},
				CallerTeam: callerB, TieRule: game.TieRuleHangingPoints, RemainingCardPoints: 20,
				Hanging: &game.HangingPoints{Amount: 81, Team: game.TeamB},
			},
			remDiff: 120, wantOn: 403, wantOff: 353,
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
		TieRule:              gs.Rules.TieRule,
		Hanging:              gs.HangingPoints,
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
			variant, "1001", 1, true, false, transfer, clockwise, game.DefaultDeclarationLastTrick,
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

func randomSettlement(rng *rand.Rand) (search.Settlement, int) {
	remaining := rng.IntN(153)
	handA := rng.IntN(153 - remaining)
	decl := []int{0, 20, 50, 70, 100}
	s := search.Settlement{
		HandPoints:           [2]int{handA, 152 - remaining - handA},
		DeclPoints:           [2]int{decl[rng.IntN(len(decl))], decl[rng.IntN(len(decl))]},
		TricksWon:            [2]int{rng.IntN(5), rng.IntN(4)},
		CallerTeam:           rng.IntN(2),
		DeclarationsTransfer: rng.IntN(2) == 0,
		TieRule:              []game.TieRule{game.TieRuleAllToOpponents, game.TieRuleHangingPoints}[rng.IntN(2)],
		RemainingCardPoints:  remaining,
	}
	if s.TieRule == game.TieRuleHangingPoints && rng.IntN(3) == 0 {
		s.Hanging = &game.HangingPoints{Amount: 81 + rng.IntN(60), Team: rng.IntN(2)}
	}
	winner := rng.IntN(2)
	if rng.IntN(4) == 0 {
		s.TricksWon[1-winner] = 0
		s.HandPoints[winner], s.HandPoints[1-winner] = 152-remaining, 0
		if rng.IntN(2) == 0 {
			return s, (1 - 2*winner) * (remaining + 100)
		}
	}
	remA := rng.IntN(remaining + 11)
	return s, 2*remA - remaining - 10
}

// expectedTotals is the engine's hand award worked out by hand: the whole hand to a Capot or to the side of a failed
// contract, the opponents' half alone on a Bitola tie, else each side's total, plus any hanging points to the taker.
// It also reports whether the hand was a Bitola tie.
func expectedTotals(s search.Settlement, remDiff int) ([2]int, bool) {
	cards, wholeTo := 152+10, -1
	if remDiff == s.RemainingCardPoints+100 || remDiff == -(s.RemainingCardPoints+100) {
		cards += 90
		wholeTo = game.TeamA
		if remDiff < 0 {
			wholeTo = game.TeamB
		}
	}
	remA := (s.RemainingCardPoints + 10 + remDiff) / 2
	full := [2]int{
		s.HandPoints[game.TeamA] + remA + s.DeclPoints[game.TeamA],
		s.HandPoints[game.TeamB] + s.RemainingCardPoints + 10 - remA + s.DeclPoints[game.TeamB],
	}
	caller, opponents := s.CallerTeam, 1-s.CallerTeam
	taker := wholeTo
	bitolaTie := false
	var totals [2]int
	switch {
	case wholeTo >= 0:
	case full[caller] > full[opponents]:
		totals, taker = full, caller
	case full[caller] == full[opponents] && s.TieRule == game.TieRuleHangingPoints:
		totals[opponents], taker, bitolaTie = full[opponents], opponents, true
	default:
		wholeTo, taker = opponents, opponents
	}
	if wholeTo >= 0 {
		totals[wholeTo] = cards + s.DeclPoints[wholeTo]
		if s.DeclarationsTransfer {
			totals[wholeTo] += s.DeclPoints[1-wholeTo]
		}
	}
	if s.Hanging != nil {
		totals[taker] += s.Hanging.Amount
	}
	return totals, bitolaTie
}

func TestSettleTotalsAgreesWithSettle(t *testing.T) {
	rng := rand.New(rand.NewPCG(1151, 9))
	var capots, falls, splits, bitolaTies, payouts int
	for i := range 20000 {
		s, remDiff := randomSettlement(rng)
		totals := search.SettleTotals(s, remDiff)
		require.Equal(t, search.Settle(s, remDiff), totals[game.TeamA]-totals[game.TeamB], "case %d %+v remDiff %d", i, s, remDiff)
		want, bitolaTie := expectedTotals(s, remDiff)
		require.Equal(t, want, totals, "case %d %+v remDiff %d", i, s, remDiff)
		if bitolaTie {
			bitolaTies++
		}
		if s.Hanging != nil {
			payouts++
		}
		switch {
		case remDiff == s.RemainingCardPoints+100 || remDiff == -(s.RemainingCardPoints+100):
			capots++
		case totals[game.TeamA] == 0 || totals[game.TeamB] == 0:
			falls++
		default:
			splits++
		}
	}
	assert.Positive(t, capots)
	assert.Positive(t, falls)
	assert.Positive(t, splits)
	assert.Positive(t, bitolaTies)
	assert.Positive(t, payouts)
}

func TestSettleTotalsMatchEngineAtTheLastTrick(t *testing.T) {
	rng := rand.New(rand.NewPCG(1152, 11))
	var hands, wholeHands, payouts int
	for attempts := 0; hands < 200; attempts++ {
		require.Less(t, attempts, 400, "too many deals ended before the last trick")
		variant := game.VariantBitola
		if hands%2 == 1 {
			variant = game.VariantCroatia
		}
		gs := game.NewGame(
			[4]uint{}, [4]string{}, [4]bool{true, true, true, true},
			variant, "1001", 1, true, false, hands/2%2 == 0, hands/4%2 == 1, game.DefaultDeclarationLastTrick,
		)
		if gs.Rules.TieRule == game.TieRuleHangingPoints && hands/8%2 == 0 {
			gs.HangingPoints = &game.HangingPoints{Amount: 81 + attempts%40, Team: hands / 16 % 2}
		}
		hand, ok := playUntilSettled(t, gs, rng, rng.IntN(2))
		if !ok {
			continue
		}
		want := [2]int{hand.result.TeamAHandTotal, hand.result.TeamBHandTotal}
		require.Equal(t, want, search.SettleTotals(hand.settlement, hand.value), "settlement %+v value %d result %+v",
			hand.settlement, hand.value, *hand.result)
		hands++
		if hand.result.Capot || hand.result.FailedContract {
			wholeHands++
		}
		if hand.result.HangingPaid != nil {
			payouts++
		}
	}
	assert.Positive(t, wholeHands)
	assert.Positive(t, payouts, "some Bitola hands must pay out hanging points")
}

func TestCallerMade(t *testing.T) {
	tests := []struct {
		name    string
		settle  search.Settlement
		remDiff int
		want    bool
	}{
		{
			name: "caller A scores more",
			settle: search.Settlement{
				HandPoints: [2]int{60, 52}, DeclPoints: [2]int{20, 0}, TricksWon: [2]int{3, 4},
				CallerTeam: game.TeamA, RemainingCardPoints: 40,
			},
			remDiff: -26, want: true,
		},
		{
			name: "caller A ties and falls under the Croatian rule",
			settle: search.Settlement{
				HandPoints: [2]int{60, 52}, DeclPoints: [2]int{20, 0}, TricksWon: [2]int{3, 4},
				CallerTeam: game.TeamA, TieRule: game.TieRuleAllToOpponents, RemainingCardPoints: 40,
			},
			remDiff: -28, want: false,
		},
		{
			name: "caller A ties under the Bitola rule and banks nothing",
			settle: search.Settlement{
				HandPoints: [2]int{60, 52}, DeclPoints: [2]int{20, 0}, TricksWon: [2]int{3, 4},
				CallerTeam: game.TeamA, TieRule: game.TieRuleHangingPoints, RemainingCardPoints: 40,
			},
			remDiff: -28, want: false,
		},
		{
			name: "caller A passes and takes back the points it left hanging",
			settle: search.Settlement{
				HandPoints: [2]int{60, 52}, DeclPoints: [2]int{20, 0}, TricksWon: [2]int{3, 4},
				CallerTeam: game.TeamA, TieRule: game.TieRuleHangingPoints, RemainingCardPoints: 40,
				Hanging: &game.HangingPoints{Amount: 81, Team: game.TeamA},
			},
			remDiff: -26, want: true,
		},
		{
			name: "caller B makes on its declarations",
			settle: search.Settlement{
				HandPoints: [2]int{60, 72}, DeclPoints: [2]int{0, 50}, TricksWon: [2]int{3, 4},
				CallerTeam: game.TeamB, RemainingCardPoints: 20,
			},
			remDiff: -30, want: true,
		},
		{
			name: "caller B falls",
			settle: search.Settlement{
				HandPoints: [2]int{70, 42}, DeclPoints: [2]int{20, 50}, TricksWon: [2]int{4, 3},
				CallerTeam: game.TeamB, RemainingCardPoints: 40,
			},
			remDiff: 30, want: false,
		},
		{
			name: "caller A takes every trick",
			settle: search.Settlement{
				HandPoints: [2]int{120, 0}, TricksWon: [2]int{6, 0}, CallerTeam: game.TeamA, RemainingCardPoints: 42,
			},
			remDiff: 142, want: true,
		},
		{
			name: "the opponents take every trick from caller A",
			settle: search.Settlement{
				HandPoints: [2]int{0, 120}, DeclPoints: [2]int{100, 0}, TricksWon: [2]int{0, 6},
				CallerTeam: game.TeamA, RemainingCardPoints: 42,
			},
			remDiff: -142, want: false,
		},
	}
	for _, tt := range tests {
		for _, transfer := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s transfer %v", tt.name, transfer), func(t *testing.T) {
				s := tt.settle
				s.DeclarationsTransfer = transfer
				assert.Equal(t, tt.want, search.CallerMade(s, tt.remDiff))
			})
		}
	}
}
