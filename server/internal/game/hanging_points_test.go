package game_test

import (
	"encoding/json"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
)

// hangingLastTrick returns a Bitola last trick whose caller is callerTeam and whose final card totals, the last-trick
// bonus included, are aCards and bCards. Seat 0 calls for team A and seat 1 for team B.
func hangingLastTrick(t *testing.T, callerTeam, aCards, bCards int) *game.GameState {
	t.Helper()
	gs := teamATakerLastTrick(t, aCards, bCards)
	caller := callerTeam
	gs.TrumpCallerSeat = &caller
	return gs
}

func hanging(amount, team int) *game.HangingPoints {
	return &game.HangingPoints{Amount: amount, Team: team}
}

func TestHangingPoints_Matrix(t *testing.T) {
	tests := []struct {
		name        string
		build       func(t *testing.T) *game.GameState
		prior       *game.HangingPoints
		wantAwarded [2]int
		wantFailed  bool
		wantEntry   *game.HangingPoints
		wantPaid    *game.HangingPoints
	}{
		{
			name:        "B ties 81:81 with nothing hanging",
			build:       func(t *testing.T) *game.GameState { return hangingLastTrick(t, game.TeamB, 81, 81) },
			wantAwarded: [2]int{81, 0},
			wantEntry:   hanging(81, game.TeamB),
		},
		{
			name:        "A calls and passes, so A takes its own 81 back",
			build:       func(t *testing.T) *game.GameState { return hangingLastTrick(t, game.TeamA, 100, 62) },
			prior:       hanging(81, game.TeamA),
			wantAwarded: [2]int{181, 62},
			wantPaid:    hanging(81, game.TeamA),
		},
		{
			name:        "A calls and falls, so B takes the hand and A's 81",
			build:       func(t *testing.T) *game.GameState { return hangingLastTrick(t, game.TeamA, 60, 102) },
			prior:       hanging(81, game.TeamA),
			wantAwarded: [2]int{0, 243},
			wantFailed:  true,
			wantPaid:    hanging(81, game.TeamB),
		},
		{
			name:        "A calls and ties, so B takes its half and A's old 81 while A's new 81 hangs",
			build:       func(t *testing.T) *game.GameState { return hangingLastTrick(t, game.TeamA, 81, 81) },
			prior:       hanging(81, game.TeamA),
			wantAwarded: [2]int{0, 162},
			wantEntry:   hanging(81, game.TeamA),
			wantPaid:    hanging(81, game.TeamB),
		},
		{
			name:        "B calls and passes, so B takes A's 81",
			build:       func(t *testing.T) *game.GameState { return hangingLastTrick(t, game.TeamB, 62, 100) },
			prior:       hanging(81, game.TeamA),
			wantAwarded: [2]int{62, 181},
			wantPaid:    hanging(81, game.TeamB),
		},
		{
			name:        "B calls and falls, so A takes the hand and its own 81 back",
			build:       func(t *testing.T) *game.GameState { return hangingLastTrick(t, game.TeamB, 102, 60) },
			prior:       hanging(81, game.TeamA),
			wantAwarded: [2]int{243, 0},
			wantFailed:  true,
			wantPaid:    hanging(81, game.TeamA),
		},
		{
			name:        "B calls and ties, so A takes its half and its own 81 while B's 81 hangs",
			build:       func(t *testing.T) *game.GameState { return hangingLastTrick(t, game.TeamB, 81, 81) },
			prior:       hanging(81, game.TeamA),
			wantAwarded: [2]int{162, 0},
			wantEntry:   hanging(81, game.TeamB),
			wantPaid:    hanging(81, game.TeamA),
		},
		{
			name:        "A capots and takes the whole hand and the 81",
			build:       func(*testing.T) *game.GameState { return testfixtures.NewGameCapotInProgress() },
			prior:       hanging(81, game.TeamA),
			wantAwarded: [2]int{333, 0},
			wantPaid:    hanging(81, game.TeamA),
		},
		{
			name:        "B capots A's contract and takes the whole hand and A's 81",
			build:       func(*testing.T) *game.GameState { return teamBCapotsTeamATaker() },
			prior:       hanging(81, game.TeamA),
			wantAwarded: [2]int{0, 333},
			wantFailed:  true,
			wantPaid:    hanging(81, game.TeamB),
		},
		{
			name: "a Croatian tie stays a fall with nothing hanging",
			build: func(t *testing.T) *game.GameState {
				return testfixtures.WithCroatianRules(hangingLastTrick(t, game.TeamB, 81, 81))
			},
			wantAwarded: [2]int{162, 0},
			wantFailed:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := tt.build(t)
			gs.HangingPoints = tt.prior

			result := playTrick8(t, gs)

			require.Equal(t, game.PhaseHandComplete, result.Phase)
			assert.Equal(t, tt.wantAwarded, result.TeamScores, "match scores after the hand")
			assert.Equal(t, tt.wantEntry, result.HangingPoints, "the entry left hanging")

			hr := result.LastHandResult
			require.NotNil(t, hr)
			assert.Equal(t, tt.wantAwarded, [2]int{hr.TeamAHandTotal, hr.TeamBHandTotal},
				"hand totals include any hanging points paid")
			assert.Equal(t, tt.wantFailed, hr.FailedContract)
			assert.Equal(t, tt.wantEntry, hr.HangingCreated)
			assert.Equal(t, tt.wantPaid, hr.HangingPaid)
		})
	}
}

type chainHand struct {
	callerTeam   int
	aCards       int
	bCards       int
	aDecl        int
	wantTotals   [2]int
	wantHanging  *game.HangingPoints
	wantPaidTeam int
}

// playChain scores each hand on a fresh last trick carrying the previous hand's scores and entry through the real
// hand-complete advance, and checks the hand totals add up to the match score.
func playChain(t *testing.T, hands []chainHand) *game.GameState {
	t.Helper()
	var scores [2]int
	var carried *game.HangingPoints
	var last *game.GameState
	var sum [2]int
	for i, h := range hands {
		gs := hangingLastTrick(t, h.callerTeam, h.aCards, h.bCards)
		gs.DeclarationPoints = [2]int{h.aDecl, 0}
		gs.TeamScores = scores
		gs.HangingPoints = carried

		hc := playTrick8(t, gs)
		require.Equal(t, game.PhaseHandComplete, hc.Phase, "hand %d", i+1)
		hr := hc.LastHandResult
		require.NotNil(t, hr)
		assert.Equal(t, h.wantTotals, [2]int{hr.TeamAHandTotal, hr.TeamBHandTotal}, "hand %d totals", i+1)
		assert.Equal(t, h.wantHanging, hc.HangingPoints, "hand %d entry", i+1)
		if carried != nil {
			require.NotNil(t, hr.HangingPaid, "hand %d pays the previous entry", i+1)
			assert.Equal(t, *hanging(carried.Amount, h.wantPaidTeam), *hr.HangingPaid, "hand %d payout", i+1)
		}
		sum[game.TeamA] += hr.TeamAHandTotal
		sum[game.TeamB] += hr.TeamBHandTotal

		next, err := game.ForceAdvanceHandComplete(hc)
		require.NoError(t, err)
		assert.Equal(t, hc.HangingPoints, next.HangingPoints, "hand %d entry survives the deal", i+1)
		scores = next.TeamScores
		carried = next.HangingPoints
		last = hc
	}
	assert.Equal(t, sum, last.TeamScores, "hand totals add up to the match score")
	return last
}

func TestHangingPoints_Chains(t *testing.T) {
	t.Run("tie, tie, tie", func(t *testing.T) {
		last := playChain(t, []chainHand{
			{callerTeam: game.TeamB, aCards: 81, bCards: 81, wantTotals: [2]int{81, 0}, wantHanging: hanging(81, game.TeamB)},
			{
				callerTeam: game.TeamA, aCards: 71, bCards: 91, aDecl: 20,
				wantTotals: [2]int{0, 172}, wantHanging: hanging(91, game.TeamA), wantPaidTeam: game.TeamB,
			},
			{
				callerTeam: game.TeamB, aCards: 81, bCards: 81,
				wantTotals: [2]int{172, 0}, wantHanging: hanging(81, game.TeamB), wantPaidTeam: game.TeamA,
			},
		})
		assert.Equal(t, [2]int{253, 172}, last.TeamScores)
	})

	t.Run("tie, then the caller passes", func(t *testing.T) {
		last := playChain(t, []chainHand{
			{callerTeam: game.TeamB, aCards: 81, bCards: 81, wantTotals: [2]int{81, 0}, wantHanging: hanging(81, game.TeamB)},
			{callerTeam: game.TeamB, aCards: 62, bCards: 100, wantTotals: [2]int{62, 181}, wantPaidTeam: game.TeamB},
		})
		assert.Equal(t, [2]int{143, 181}, last.TeamScores)
	})

	t.Run("tie, then the caller falls", func(t *testing.T) {
		last := playChain(t, []chainHand{
			{callerTeam: game.TeamB, aCards: 81, bCards: 81, wantTotals: [2]int{81, 0}, wantHanging: hanging(81, game.TeamB)},
			{callerTeam: game.TeamB, aCards: 102, bCards: 60, wantTotals: [2]int{243, 0}, wantPaidTeam: game.TeamA},
		})
		assert.Equal(t, [2]int{324, 0}, last.TeamScores)
	})
}

func TestHangingPoints_MatchEnd(t *testing.T) {
	t.Run("the opponents' half of a tie can end the match and the new entry is discarded", func(t *testing.T) {
		gs := hangingLastTrick(t, game.TeamB, 81, 81)
		gs.TeamScores = [2]int{950, 0}

		result := playTrick8(t, gs)

		require.Equal(t, game.PhaseMatchEnd, result.Phase)
		require.NotNil(t, result.WinnerTeam)
		assert.Equal(t, game.TeamA, *result.WinnerTeam)
		assert.Equal(t, [2]int{1031, 0}, result.TeamScores, "the hanging half counts for nobody")
		assert.Nil(t, result.HangingPoints, "points still hanging at match end are discarded")
		require.NotNil(t, result.LastHandResult)
		assert.Equal(t, hanging(81, game.TeamB), result.LastHandResult.HangingCreated, "the hand still reports its tie")
	})

	t.Run("hanging points paid that hand count toward the target", func(t *testing.T) {
		gs := hangingLastTrick(t, game.TeamA, 100, 62)
		gs.TeamScores = [2]int{850, 0}
		control := playTrick8(t, gs)
		require.Equal(t, game.PhaseHandComplete, control.Phase, "without the payout 950 is short of 1001")

		gs = testfixtures.WithHangingPoints(hangingLastTrick(t, game.TeamA, 100, 62), 81, game.TeamA)
		gs.TeamScores = [2]int{850, 0}

		result := playTrick8(t, gs)

		require.Equal(t, game.PhaseMatchEnd, result.Phase)
		require.NotNil(t, result.WinnerTeam)
		assert.Equal(t, game.TeamA, *result.WinnerTeam)
		assert.Equal(t, [2]int{1031, 62}, result.TeamScores)
		assert.Nil(t, result.HangingPoints)
	})

	t.Run("a stop at target banks running totals without the hanging points and discards them", func(t *testing.T) {
		const trickNum = 5
		seats := []int{0, 1, 2, 3}
		winnerTeam, trickPoints := midPlayTrickPoints(t, trickNum, seats)

		gs := testfixtures.NewGameMidPlayWithScores(trickNum, 0, 0)
		gs.TeamScores[winnerTeam] = 1001 - gs.HandPoints[winnerTeam] - trickPoints
		gs = testfixtures.WithHangingPoints(testfixtures.WithStopAtTarget(gs), 81, winnerTeam)

		after := stopTrick(t, gs, seats)

		require.Equal(t, game.PhaseMatchEnd, after.Phase)
		assert.Equal(t, 1001, after.TeamScores[winnerTeam], "the running total excludes the hanging points")
		assert.Nil(t, after.HangingPoints)
	})

	t.Run("an all-eight-trumps deal discards the hanging points", func(t *testing.T) {
		gs := testfixtures.WithHangingPoints(instantWinOnPick(t), 81, game.TeamA)

		result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPickTrump, PlayerSeat: 1})

		require.NoError(t, err)
		require.Equal(t, game.PhaseMatchEnd, result.Phase)
		require.True(t, result.WonByInstantWin)
		assert.Nil(t, result.HangingPoints)
		assert.Equal(t, hanging(81, game.TeamA), gs.HangingPoints, "the source state is untouched")
	})

	t.Run("a surrender discards the hanging points", func(t *testing.T) {
		gs := testfixtures.WithHangingPoints(testfixtures.NewGameMidPlayWithScores(1, 300, 200), 81, game.TeamA)
		proposer := 1
		gs.SurrenderProposerSeat = &proposer
		gs.SurrenderUsed[proposer] = true

		result, err := game.ApplyAction(gs, game.Action{Type: game.ActionSurrenderAccept, PlayerSeat: 3})

		require.NoError(t, err)
		require.Equal(t, game.PhaseMatchEnd, result.Phase)
		assert.Equal(t, [2]int{300, 200}, result.TeamScores)
		assert.Nil(t, result.HangingPoints)
		assert.Equal(t, hanging(81, game.TeamA), gs.HangingPoints, "the source state is untouched")
	})
}

func TestHangingPoints_PayoutIsFixedWhenDeclarationsDoNotTransfer(t *testing.T) {
	gs := testfixtures.WithoutDeclarationsTransfer(hangingLastTrick(t, game.TeamA, 60, 102))
	gs.DeclarationPoints = [2]int{20, 0}
	gs.HangingPoints = hanging(91, game.TeamA)

	result := playTrick8(t, gs)

	require.NotNil(t, result.LastHandResult)
	assert.True(t, result.LastHandResult.FailedContract)
	assert.Equal(t, [2]int{0, 162 + 91}, result.TeamScores,
		"the fall voids A's new declaration, but the whole 91 hanging is paid to B")
	assert.Equal(t, hanging(91, game.TeamB), result.LastHandResult.HangingPaid)
}

func TestHangingPoints_ReshuffleKeepsTheEntry(t *testing.T) {
	gs := testfixtures.WithHangingPoints(testfixtures.NewGameMidBidding(7), 81, game.TeamA)

	result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPassTrump, PlayerSeat: 0})

	require.NoError(t, err)
	require.Equal(t, game.PhaseDealing, result.Phase, "four passes in round 2 reshuffle")
	assert.Equal(t, hanging(81, game.TeamA), result.HangingPoints, "the entry waits for the next scored hand")
}

func TestHangingPoints_ProjectionAndClone(t *testing.T) {
	gs := testfixtures.WithHangingPoints(hangingLastTrick(t, game.TeamA, 81, 81), 81, game.TeamB)
	hc := playTrick8(t, gs)
	require.NotNil(t, hc.HangingPoints)
	require.NotNil(t, hc.LastHandResult.HangingPaid)

	for seat := 0; seat < 4; seat++ {
		projected := game.ProjectForSeat(hc, seat)
		assert.Equal(t, hanging(81, game.TeamA), projected.HangingPoints, "seat %d sees the entry", seat)
		assert.NotSame(t, hc.HangingPoints, projected.HangingPoints, "seat %d gets its own copy", seat)

		data, err := json.Marshal(projected)
		require.NoError(t, err)
		var raw map[string]any
		require.NoError(t, json.Unmarshal(data, &raw))
		assert.Equal(t, map[string]any{"amount": float64(81), "team": float64(game.TeamA)}, raw["hangingPoints"])
		hand, ok := raw["lastHandResult"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, map[string]any{"amount": float64(81), "team": float64(game.TeamA)}, hand["hangingCreated"])
		assert.Equal(t, map[string]any{"amount": float64(81), "team": float64(game.TeamB)}, hand["hangingPaid"])

		projected.HangingPoints.Amount = 1
		projected.LastHandResult.HangingCreated.Amount = 1
		projected.LastHandResult.HangingPaid.Amount = 1
	}
	assert.Equal(t, hanging(81, game.TeamA), hc.HangingPoints, "projections never write through to the source")
	assert.Equal(t, hanging(81, game.TeamA), hc.LastHandResult.HangingCreated)
	assert.Equal(t, hanging(81, game.TeamB), hc.LastHandResult.HangingPaid)
	assert.Equal(t, hanging(81, game.TeamB), gs.HangingPoints, "scoring never writes through to its input")
}

func TestHangingPoints_NoEntryOnTheWireWhenNothingHangs(t *testing.T) {
	data, err := json.Marshal(game.ProjectForSeat(playTrick8(t, testfixtures.NewGameLastTrick()), 0))
	require.NoError(t, err)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, raw, "hangingPoints")
	assert.Nil(t, raw["hangingPoints"])
}

// croatianSettle is the Croatian settlement written longhand: a Capot or a fall takes the whole hand, and the caller
// fails on a fall or on the other side's Capot.
func croatianSettle(t game.HandTally) ([2]int, bool) {
	totals := [2]int{t.CardPoints[0] + t.DeclPoints[0], t.CardPoints[1] + t.DeclPoints[1]}
	caller, opponents := t.CallerTeam, 1-t.CallerTeam
	failed := totals[caller] <= totals[opponents]
	if t.Capot {
		failed = t.CapotTeam != caller
	}
	whole := func(winner int) int {
		if t.DeclarationsTransfer {
			return t.CardPoints[0] + t.CardPoints[1] + t.DeclPoints[0] + t.DeclPoints[1]
		}
		return t.CardPoints[0] + t.CardPoints[1] + t.DeclPoints[winner]
	}
	var awarded [2]int
	switch {
	case t.Capot:
		awarded[t.CapotTeam] = whole(t.CapotTeam)
	case totals[caller] <= totals[opponents]:
		awarded[opponents] = whole(opponents)
	default:
		awarded = totals
	}
	return awarded, failed
}

func TestSettleHand_CroatianMatchesTheLonghandRule(t *testing.T) {
	rng := rand.New(rand.NewPCG(2026, 1109))
	decl := []int{0, 20, 50, 70, 100, 150}
	var ties int
	for i := range 20000 {
		cardsA := rng.IntN(163)
		tally := game.HandTally{
			CardPoints:           [2]int{cardsA, 162 - cardsA},
			DeclPoints:           [2]int{decl[rng.IntN(len(decl))], decl[rng.IntN(len(decl))]},
			CallerTeam:           rng.IntN(2),
			DeclarationsTransfer: rng.IntN(2) == 0,
			TieRule:              game.TieRuleAllToOpponents,
		}
		if rng.IntN(5) == 0 {
			tally.Capot = true
			tally.CapotTeam = rng.IntN(2)
			tally.CardPoints = [2]int{}
			tally.CardPoints[tally.CapotTeam] = 252
		}
		wantAwarded, wantFailed := croatianSettle(tally)

		got := game.SettleHand(tally, nil)

		require.Equal(t, wantAwarded, got.Awarded, "case %d %+v", i, tally)
		require.Equal(t, wantFailed, got.FailedContract, "case %d %+v", i, tally)
		require.Nil(t, got.Hanging, "case %d", i)
		require.Nil(t, got.Paid, "case %d", i)
		if tally.CardPoints[0]+tally.DeclPoints[0] == tally.CardPoints[1]+tally.DeclPoints[1] {
			ties++
		}
	}
	assert.Positive(t, ties, "the sample must include tied hands")
}

func TestSettleHand_FailedContractOnACapot(t *testing.T) {
	tests := []struct {
		name       string
		capotTeam  int
		declB      int
		wantFailed bool
	}{
		{"the caller's Capot passes against a smaller declaration", game.TeamA, 100, false},
		{"the caller's Capot passes against an equal total", game.TeamA, 252, false},
		{"the caller's Capot passes against a larger declaration", game.TeamA, 300, false},
		{"the other side's Capot fails the caller", game.TeamB, 0, true},
	}
	for _, rule := range []game.TieRule{game.TieRuleAllToOpponents, game.TieRuleHangingPoints} {
		for _, tt := range tests {
			t.Run(string(rule)+": "+tt.name, func(t *testing.T) {
				tally := game.HandTally{
					DeclPoints: [2]int{0, tt.declB},
					CallerTeam: game.TeamA,
					Capot:      true,
					CapotTeam:  tt.capotTeam,
					TieRule:    rule,
				}
				tally.CardPoints[tt.capotTeam] = 252

				got := game.SettleHand(tally, nil)

				assert.Equal(t, tt.wantFailed, got.FailedContract)
				assert.Nil(t, got.Hanging, "a Capot never hangs")
				assert.Zero(t, got.Awarded[1-tt.capotTeam], "the Capot team takes the whole hand")
			})
		}
	}
}
