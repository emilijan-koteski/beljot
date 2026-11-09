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

// perfectInformationChoice is the card the race search picks for the seat on turn when it sees every hand, and the
// position's value for team A.
func perfectInformationChoice(t *testing.T, gs *game.GameState, deciding bool) (game.Card, int) {
	t.Helper()
	seat := gs.ActivePlayerSeat
	hands := handMasks(gs)
	in := search.Information{Seat: seat, Own: hands[seat]}
	for other := range hands {
		if other != seat {
			in.Known[other] = hands[other]
		}
	}
	leader := seat
	if len(gs.CurrentTrick) > 0 {
		leader = gs.CurrentTrick[0].PlayerSeat
	}
	race := handEndRace(gs, deciding)
	pos := search.Position{
		Trump: search.SuitIdx(*gs.TrumpSuit), Clockwise: gs.Rules.Clockwise, Leader: leader, Trick: trickIndexes(gs),
		Capot: search.CapotFlag(gs.TricksWon), Race: &race,
	}
	settlement := search.Settlement{
		HandPoints: gs.HandPoints, TricksWon: gs.TricksWon, CallerTeam: game.TeamForSeat(*gs.TrumpCallerSeat),
		DeclarationsTransfer: gs.Rules.DeclarationsTransfer,
	}
	for _, c := range append(search.CardsOf(hands[0]|hands[1]|hands[2]|hands[3]), trickCards(gs)...) {
		settlement.RemainingCardPoints += search.Points(search.CardIdx(c), pos.Trump)
	}
	res, ok := search.Choose(in, pos, game.LegalCards(gs, seat), settlement, search.Options{Samples: 1}, farFuture())
	require.True(t, ok)
	return res.Card, (1 - 2*game.TeamForSeat(seat)) * int(res.Mean)
}

func trickCards(gs *game.GameState) []game.Card {
	out := make([]game.Card, len(gs.CurrentTrick))
	for i, tc := range gs.CurrentTrick {
		out[i] = tc.Card
	}
	return out
}

// crossingKind names what carried a team over the target at the hand's end: a Capot, a fallen contract, the last
// trick's bonus, or the card points alone.
func crossingKind(result game.HandScore, winner, target int, scores [2]int) string {
	totals := [2]int{result.TeamAHandTotal, result.TeamBHandTotal}
	switch {
	case result.Capot:
		return "capot"
	case result.FailedContract:
		return "fall"
	case result.LastTrickTeam == winner && scores[winner]+totals[winner]-result.LastTrickBonus < target:
		return "bonus"
	}
	return "points"
}

func TestRaceCrossingFromTricksSixAndSevenMatchesTheEngine(t *testing.T) {
	for _, stopAtTarget := range []bool{true, false} {
		t.Run(fmt.Sprintf("stopAtTarget=%v", stopAtTarget), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(101, 103))
			kinds := map[string]int{}
			for i := range 300 {
				gs := endgameDeal(t, rng, stopAtTarget, 6+i%2, 90)
				scores, target := gs.TeamScores, game.MatchTarget(gs.MatchMode)
				_, value := perfectInformationChoice(t, gs, !stopAtTarget)
				for gs.Phase == game.PhasePlaying {
					card, step := perfectInformationChoice(t, gs, !stopAtTarget)
					require.Equal(t, value, step, "deal %d: optimal play keeps the root value", i)
					next, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: gs.ActivePlayerSeat, Card: &card})
					require.NoError(t, err)
					gs = next
				}
				if gs.Phase != game.PhaseMatchEnd {
					require.NotNil(t, gs.LastHandResult, "deal %d", i)
					require.Equal(t, gs.LastHandResult.TeamAHandTotal-gs.LastHandResult.TeamBHandTotal, value, "deal %d", i)
					kinds["open"]++
					continue
				}
				require.Equal(t, (1-2*(*gs.WinnerTeam))*search.RaceWin, value, "deal %d", i)
				if gs.TrickNumber < 8 || gs.LastHandResult == nil {
					kinds["mid-hand"]++
					continue
				}
				kinds[crossingKind(*gs.LastHandResult, *gs.WinnerTeam, target, scores)]++
			}
			t.Logf("outcomes %v", kinds)
			assert.Positive(t, kinds["open"])
			assert.Positive(t, kinds["fall"], "no crossing through a fallen contract")
			assert.Positive(t, kinds["bonus"], "no crossing through the last trick's bonus")
			assert.Positive(t, kinds["capot"], "no crossing through a Capot")
			assert.Equal(t, stopAtTarget, kinds["mid-hand"] > 0, "only a stop-at-target match ends mid-hand")
		})
	}
}
