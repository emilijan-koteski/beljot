package game_test

import (
	"testing"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const lastTrickPointsToTeamB = 31

type transferScenario int

const (
	scenarioPlayedOut transferScenario = iota
	scenarioTeamBCapot
	scenarioTeamACapot
)

// teamATakerLastTrick returns a last-trick state with team A as taker, where trick 8 hands team B
// 21 card points plus the 10 last-trick bonus, so aCards and bCards are the hand's final card totals.
func teamATakerLastTrick(t *testing.T, aCards, bCards int) *game.GameState {
	t.Helper()
	require.Equal(t, 162, aCards+bCards, "a played-out hand's card points always sum to 162")
	gs := testfixtures.NewGameLastTrick()
	caller := 0
	gs.TrumpCallerSeat = &caller
	gs.HandPoints = [2]int{aCards, bCards - lastTrickPointsToTeamB}
	return gs
}

// teamBCapotsTeamATaker returns a last-trick state where team B has won the first seven tricks
// against team A's contract and leads the trump Jack into trick 8.
func teamBCapotsTeamATaker() *game.GameState {
	gs := testfixtures.NewGameCapotInProgress()
	gs.Players[0].Hand = []game.Card{{Rank: game.Rank7, Suit: game.SuitClubs}}
	gs.Players[1].Hand = []game.Card{{Rank: game.RankJack, Suit: game.SuitHearts}}
	gs.Players[2].Hand = []game.Card{{Rank: game.Rank7, Suit: game.SuitDiamonds}}
	gs.Players[3].Hand = []game.Card{{Rank: game.RankAce, Suit: game.SuitHearts}}
	leader := 1
	gs.TrickWinnerSeat = &leader
	gs.ActivePlayerSeat = leader
	gs.HandPoints = [2]int{0, 121}
	gs.TricksWon = [2]int{0, 7}
	return gs
}

// TestDeclarationsTransfer_HandAwards walks every row of the rule's I/O matrix: team A is the taker
// throughout, and the hand totals must equal the match-score delta while the declaration fields stay raw.
func TestDeclarationsTransfer_HandAwards(t *testing.T) {
	tests := []struct {
		name        string
		transfer    bool
		scenario    transferScenario
		aCards      int
		bCards      int
		declPoints  [2]int
		belotPoints [2]int
		wantAwarded [2]int
		wantFailed  bool
		wantCapot   bool
	}{
		{
			name: "flag on, fall", transfer: true, aCards: 50, bCards: 112,
			declPoints: [2]int{20, 0}, wantAwarded: [2]int{0, 182}, wantFailed: true,
		},
		{
			name: "flag off, contract made", transfer: false, aCards: 100, bCards: 62,
			declPoints: [2]int{50, 0}, wantAwarded: [2]int{150, 62}, wantFailed: false,
		},
		{
			name: "flag off, fall, winners declared", transfer: false, aCards: 60, bCards: 102,
			declPoints: [2]int{0, 50}, wantAwarded: [2]int{0, 212}, wantFailed: true,
		},
		{
			name: "flag off, fall, fallen team declared", transfer: false, aCards: 50, bCards: 112,
			declPoints: [2]int{20, 0}, wantAwarded: [2]int{0, 162}, wantFailed: true,
		},
		{
			name: "flag off, fall, fallen team has Belote", transfer: false, aCards: 40, bCards: 122,
			belotPoints: [2]int{20, 0}, wantAwarded: [2]int{0, 162}, wantFailed: true,
		},
		{
			name: "flag off, fall, both have Belote or declarations", transfer: false, aCards: 50, bCards: 112,
			declPoints: [2]int{0, 50}, belotPoints: [2]int{20, 0}, wantAwarded: [2]int{0, 212}, wantFailed: true,
		},
		{
			name: "flag off, tie fails", transfer: false, aCards: 71, bCards: 91,
			declPoints: [2]int{20, 0}, wantAwarded: [2]int{0, 162}, wantFailed: true,
		},
		{
			name: "flag on, B capots A", transfer: true, scenario: scenarioTeamBCapot,
			declPoints: [2]int{20, 0}, wantAwarded: [2]int{0, 272}, wantFailed: true, wantCapot: true,
		},
		{
			name: "flag off, B capots A", transfer: false, scenario: scenarioTeamBCapot,
			declPoints: [2]int{20, 0}, wantAwarded: [2]int{0, 252}, wantFailed: true, wantCapot: true,
		},
		{
			name: "flag off, A capots, B won declarations", transfer: false, scenario: scenarioTeamACapot,
			declPoints: [2]int{0, 50}, wantAwarded: [2]int{252, 0}, wantFailed: false, wantCapot: true,
		},
		{
			name: "flag off, A capots, B has Belote", transfer: false, scenario: scenarioTeamACapot,
			belotPoints: [2]int{0, 20}, wantAwarded: [2]int{252, 0}, wantFailed: false, wantCapot: true,
		},
		{
			name: "flag off, A capots and keeps its own declarations", transfer: false, scenario: scenarioTeamACapot,
			declPoints: [2]int{20, 0}, belotPoints: [2]int{20, 0}, wantAwarded: [2]int{292, 0}, wantFailed: false, wantCapot: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gs *game.GameState
			switch tt.scenario {
			case scenarioTeamBCapot:
				gs = teamBCapotsTeamATaker()
			case scenarioTeamACapot:
				gs = testfixtures.NewGameCapotInProgress()
			default:
				gs = teamATakerLastTrick(t, tt.aCards, tt.bCards)
			}
			if !tt.transfer {
				gs = testfixtures.WithoutDeclarationsTransfer(gs)
			}
			require.Equal(t, game.TeamA, game.TeamForSeat(*gs.TrumpCallerSeat), "team A is the taker in every row")
			gs.DeclarationPoints = tt.declPoints
			gs.BelotPoints = tt.belotPoints

			result := playTrick8(t, gs)

			require.Equal(t, game.PhaseHandComplete, result.Phase)
			assert.Equal(t, tt.wantAwarded, result.TeamScores, "match scores after the hand")

			hr := result.LastHandResult
			require.NotNil(t, hr)
			assert.Equal(t, tt.wantFailed, hr.FailedContract)
			assert.Equal(t, tt.wantCapot, hr.Capot)
			assert.Equal(t, tt.wantAwarded[game.TeamA], hr.TeamAHandTotal, "team A hand total is its award")
			assert.Equal(t, tt.wantAwarded[game.TeamB], hr.TeamBHandTotal, "team B hand total is its award")
			assert.Equal(t, tt.declPoints[game.TeamA]+tt.belotPoints[game.TeamA], hr.TeamADeclPoints,
				"the breakdown keeps team A's raw declarations even when they are voided")
			assert.Equal(t, tt.declPoints[game.TeamB]+tt.belotPoints[game.TeamB], hr.TeamBDeclPoints,
				"the breakdown keeps team B's raw declarations even when they are voided")
		})
	}
}

// TestDeclarationsTransfer_ResolvedOnceAtNewGame checks both presets default to transferring and that
// NewGame lands the room's choice on the config the engine reads and on every seat's wire state.
func TestDeclarationsTransfer_ResolvedOnceAtNewGame(t *testing.T) {
	for _, v := range []game.Variant{game.VariantBitola, game.VariantCroatia} {
		assert.True(t, game.RulesFor(v).DeclarationsTransfer, "%s preset transfers declarations", v)
	}

	for _, v := range []game.Variant{game.VariantBitola, game.VariantCroatia} {
		for _, want := range []bool{true, false} {
			gs := game.NewGame(
				[4]uint{10, 20, 30, 40},
				[4]string{"a", "b", "c", "d"},
				[4]bool{},
				v, "1001", 1, true, false, want,
			)
			assert.Equal(t, want, gs.Rules.DeclarationsTransfer, "%s: the config the engine reads", v)
			assert.Equal(t, want, gs.DeclarationsTransfer, "%s: the wire flag the client renders", v)
			assert.True(t, gs.Rules.DeclarationsEnabled, "%s: the declarations toggle is independent", v)
			assert.False(t, gs.Rules.StopAtTarget, "%s: the stop-at-target toggle is independent", v)
			for seat := range 4 {
				assert.Equal(t, want, game.ProjectForSeat(gs, seat).DeclarationsTransfer,
					"%s: seat %d must be told the room's setting", v, seat)
			}
		}
	}
}

// TestDeclarationsTransfer_WireFlagFollowsTheConfig checks the wire mirror is recomputed from Rules
// on every action, so a stale value can never reach the client.
func TestDeclarationsTransfer_WireFlagFollowsTheConfig(t *testing.T) {
	gs := testfixtures.WithoutDeclarationsTransfer(teamATakerLastTrick(t, 50, 112))
	gs.DeclarationsTransfer = true

	result := playTrick8(t, gs)

	assert.False(t, result.DeclarationsTransfer)
}
