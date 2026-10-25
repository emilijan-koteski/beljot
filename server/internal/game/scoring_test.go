package game_test

import (
	"testing"

	game "github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// playTrick8 plays the last trick, each player playing their single remaining card from the active seat.
// Returns the state after the trick and hand scoring complete.
func playTrick8(t *testing.T, gs *game.GameState) *game.GameState {
	t.Helper()
	state := gs
	for i := 0; i < 4; i++ {
		seat := state.ActivePlayerSeat
		require.Len(t, state.Players[seat].Hand, 1, "seat %d should have 1 card at trick 8 play %d", seat, i)
		card := state.Players[seat].Hand[0]
		newState, err := game.ApplyAction(state, game.Action{
			Type:       game.ActionPlayCard,
			PlayerSeat: seat,
			Card:       &card,
		})
		require.NoError(t, err, "play_card for seat %d (play %d)", seat, i)
		state = newState
	}
	return state
}

func TestHandScoring_LastTrickBonus(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()

	result := playTrick8(t, gs)

	assert.NotEqual(t, game.PhasePlaying, result.Phase, "should have left playing phase")

	assert.Equal(t, 70, result.TeamScores[game.TeamA], "Team A score")
	assert.Equal(t, 92, result.TeamScores[game.TeamB], "Team B score (includes +10 bonus)")
}

func TestHandScoring_LastTrickSeatPreserved(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()

	hc := playTrick8(t, gs)

	require.Equal(t, game.PhaseHandComplete, hc.Phase)
	require.NotNil(t, hc.TrickWinnerSeat)
	assert.Equal(t, 3, *hc.TrickWinnerSeat)
	require.NotNil(t, hc.LastHandResult)
	assert.Equal(t, 3, hc.LastHandResult.LastTrickSeat, "winner seat recorded for broadcast")
	assert.Equal(t, game.TeamB, hc.LastHandResult.LastTrickTeam, "seat 3 is team B")

	next, err := game.ForceAdvanceHandComplete(hc)
	require.NoError(t, err)
	assert.Nil(t, next.TrickWinnerSeat, "startNewHand clears the live winner field")
	require.NotNil(t, next.LastHandResult)
	assert.Equal(t, 3, next.LastHandResult.LastTrickSeat, "winner seat preserved across startNewHand")
}

func TestHandScoring_CapotScoring(t *testing.T) {
	gs := testfixtures.NewGameCapotInProgress()

	result := playTrick8(t, gs)

	assert.Equal(t, 252, result.TeamScores[game.TeamA], "Team A gets all 152 card pts + 100 Capot")
	assert.Equal(t, 0, result.TeamScores[game.TeamB], "Team B gets nothing")
}

// TestHandScoring_CapotForfeitsLosersDeclarations checks that the no-trick team in a Capot forfeits
// even the declarations it won, and those points go to the Capot team.
func TestHandScoring_CapotForfeitsLosersDeclarations(t *testing.T) {
	gs := testfixtures.NewGameCapotInProgress()
	gs.DeclarationPoints = [2]int{0, 40}

	result := playTrick8(t, gs)

	assert.Equal(t, 292, result.TeamScores[game.TeamA], "Capot team takes everything, incl. opponent's declarations")
	assert.Equal(t, 0, result.TeamScores[game.TeamB], "no-trick team scores 0, forfeiting its declarations")

	require.NotNil(t, result.LastHandResult)
	assert.True(t, result.LastHandResult.Capot, "should be a capot")
	assert.Equal(t, 292, result.LastHandResult.TeamAHandTotal, "Team A hand total includes forfeited declarations")
	assert.Equal(t, 0, result.LastHandResult.TeamBHandTotal, "Team B hand total is 0")
}

func TestHandScoring_CapotBroken(t *testing.T) {
	gs := testfixtures.NewGameCapotInProgress()
	gs.Players[0].Hand = []game.Card{{Rank: game.Rank7, Suit: game.SuitDiamonds}}
	gs.Players[1].Hand = []game.Card{{Rank: game.RankJack, Suit: game.SuitHearts}}

	result := playTrick8(t, gs)

	assert.Equal(t, 121, result.TeamScores[game.TeamA], "Team A keeps their card points")
	assert.Equal(t, 41, result.TeamScores[game.TeamB], "Team B gets trick 8 pts + 10 bonus")
}

func TestHandScoring_FailedContract(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	gs.HandPoints = [2]int{100, 20}

	result := playTrick8(t, gs)

	assert.Equal(t, 151, result.TeamScores[game.TeamA], "Team A gets ALL points on failed contract")
	assert.Equal(t, 0, result.TeamScores[game.TeamB], "Team B gets 0 on failed contract")
}

func TestHandScoring_EqualPointsIsFailure(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	gs.HandPoints = [2]int{80, 49}

	result := playTrick8(t, gs)

	assert.Equal(t, 160, result.TeamScores[game.TeamA], "Team A (opponent) gets ALL points on a tie")
	assert.Equal(t, 0, result.TeamScores[game.TeamB], "Team B (caller) gets 0 — a tie is a failure")
	require.NotNil(t, result.LastHandResult)
	assert.True(t, result.LastHandResult.FailedContract, "a tie marks the hand failed for the caller")
}

func TestHandScoring_TieFailsTrumpCaller(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	gs.HandPoints = [2]int{81, 50}
	gs.TeamScores = [2]int{0, 0}

	result := playTrick8(t, gs)

	require.NotNil(t, result.LastHandResult)
	assert.True(t, result.LastHandResult.FailedContract, "81:81 is a failed hand for the caller")
	assert.Equal(t, game.TeamB, result.LastHandResult.ContractingTeam, "team B called trump")
	assert.Equal(t, 162, result.TeamScores[game.TeamA], "opponents win all 162 points")
	assert.Equal(t, 0, result.TeamScores[game.TeamB], "caller scores 0 on the tie")
}

func TestHandScoring_TieWithDeclarationsFailsTrumpCaller(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	gs.HandPoints = [2]int{71, 60}
	gs.DeclarationPoints = [2]int{20, 0}
	gs.TeamScores = [2]int{0, 0}

	result := playTrick8(t, gs)

	require.NotNil(t, result.LastHandResult)
	assert.True(t, result.LastHandResult.FailedContract, "91:91 is a failed hand for the caller")
	assert.Equal(t, 182, result.TeamScores[game.TeamA], "opponents win all 182 points incl. declarations")
	assert.Equal(t, 0, result.TeamScores[game.TeamB], "caller scores 0 on the tie")
	assert.Equal(t, 182, result.LastHandResult.TeamAHandTotal, "broadcast total mirrors the transfer")
	assert.Equal(t, 0, result.LastHandResult.TeamBHandTotal, "broadcast total mirrors the transfer")
}

func TestHandScoring_NormalScoring(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	gs.HandPoints = [2]int{40, 91}

	result := playTrick8(t, gs)

	assert.Equal(t, 40, result.TeamScores[game.TeamA], "Team A keeps own points")
	assert.Equal(t, 122, result.TeamScores[game.TeamB], "Team B keeps own points")
}

func TestHandScoring_MatchEndTriggered(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	gs.TeamScores = [2]int{950, 0}

	result := playTrick8(t, gs)

	assert.Equal(t, game.PhaseMatchEnd, result.Phase, "match should end when team crosses 1001")
	assert.GreaterOrEqual(t, result.TeamScores[game.TeamA], 1001)
	require.NotNil(t, result.WinnerTeam, "WinnerTeam must be set on match end")
	assert.Equal(t, game.TeamA, *result.WinnerTeam, "Team A should win")
}

func TestHandScoring_MatchContinues(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	gs.TeamScores = [2]int{100, 200}

	hc := playTrick8(t, gs)
	require.Equal(t, game.PhaseHandComplete, hc.Phase, "hand holds for the continue pause before dealing")
	result, err := game.ForceAdvanceHandComplete(hc)
	require.NoError(t, err)

	assert.Equal(t, game.PhaseDealing, result.Phase, "should start new hand in dealing phase")
	assert.Equal(t, 2, result.HandNumber, "hand number should increment")
	assert.Equal(t, 1, result.DealerSeat, "dealer should rotate")
	for i, p := range result.Players {
		assert.Len(t, p.Hand, 5, "player at seat %d should have 5 cards after stage-1 re-deal", i)
	}
	assert.Len(t, result.Deck, 11, "Deck should hold 11 cards for stage-2 of the new hand")
	require.NotNil(t, result.TrumpCandidate, "candidate revealed for the new hand")
}

func TestHandScoring_NewHandStateReset(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	gs.TeamScores = [2]int{0, 0}
	gs.DeclarationPoints = [2]int{50, 20}

	hc := playTrick8(t, gs)
	require.Equal(t, game.PhaseHandComplete, hc.Phase, "hand holds for the continue pause before dealing")
	result, err := game.ForceAdvanceHandComplete(hc)
	require.NoError(t, err)

	assert.Equal(t, game.PhaseDealing, result.Phase)

	assert.Equal(t, [2]int{0, 0}, result.HandPoints, "HandPoints reset")
	assert.Equal(t, [2]int{0, 0}, result.DeclarationPoints, "DeclarationPoints reset")
	assert.Equal(t, [2]int{0, 0}, result.TricksWon, "TricksWon reset")
	assert.False(t, result.AwaitingDeclaration, "AwaitingDeclaration reset")
	assert.False(t, result.DeclarationsResolved, "DeclarationsResolved reset")
	assert.Nil(t, result.PendingBelotSeat, "PendingBelotSeat reset")
	assert.False(t, result.BelotAnnounced, "BelotAnnounced reset")
	assert.Equal(t, 0, result.TrickNumber, "TrickNumber reset")
	assert.Nil(t, result.TrumpSuit, "TrumpSuit reset")
	assert.Nil(t, result.TrumpCallerSeat, "TrumpCallerSeat reset")
	assert.Nil(t, result.LeadSuit, "LeadSuit reset")
	assert.Nil(t, result.TrickWinnerSeat, "TrickWinnerSeat reset")
	assert.Nil(t, result.WinnerTeam, "WinnerTeam reset")
	assert.Nil(t, result.TurnExpiresAt, "TurnExpiresAt reset")
	for i, p := range result.Players {
		assert.Nil(t, p.Declarations, "player %d declarations reset", i)
	}
}

func TestHandScoring_DeclarationsIncluded(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	gs.DeclarationPoints = [2]int{50, 0}

	result := playTrick8(t, gs)

	assert.Equal(t, 212, result.TeamScores[game.TeamA], "declarations included in scoring")
	assert.Equal(t, 0, result.TeamScores[game.TeamB], "failed contract = 0 for contracting team")
}

func TestHandScoring_FailedContractBothTeamsHaveDeclarations(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	gs.HandPoints = [2]int{60, 20}
	gs.DeclarationPoints = [2]int{40, 30}

	result := playTrick8(t, gs)

	assert.Equal(t, 181, result.TeamScores[game.TeamA], "Team A gets ALL points including team B's declarations")
	assert.Equal(t, 0, result.TeamScores[game.TeamB], "Team B gets 0 on failed contract")
}

func TestHandScoring_BelotBonusIncluded(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	gs.HandPoints = [2]int{70, 61}
	gs.BelotPoints = [2]int{20, 0}

	result := playTrick8(t, gs)

	assert.Equal(t, 90, result.TeamScores[game.TeamA], "Team A total includes the belote bonus")
	assert.Equal(t, 92, result.TeamScores[game.TeamB], "Team B keeps own total")
}

// TestHandScoring_BeloteCountsAsDeclaration checks that belote is reported as declaration points,
// not card points, while still counting toward the team's hand total.
func TestHandScoring_BeloteCountsAsDeclaration(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	gs.HandPoints = [2]int{40, 91}
	gs.BelotPoints = [2]int{20, 0}

	result := playTrick8(t, gs)
	hr := result.LastHandResult
	require.NotNil(t, hr)

	assert.Equal(t, 20, hr.TeamADeclPoints, "belote is folded into declaration points")
	assert.Equal(t, 40, hr.TeamACardPoints, "belote is NOT counted as card points")
	assert.Equal(t, 60, result.TeamScores[game.TeamA], "team A total = 40 card + 20 belote declaration")
}

// TestHandScoring_BeloteForfeitedOnFailedContract checks that belote transfers with the rest of
// the points on a failed contract.
func TestHandScoring_BeloteForfeitedOnFailedContract(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	gs.HandPoints = [2]int{100, 20}
	gs.BelotPoints = [2]int{0, 20}

	result := playTrick8(t, gs)

	assert.Equal(t, 171, result.TeamScores[game.TeamA], "opponent gets all points including the caller's belote")
	assert.Equal(t, 0, result.TeamScores[game.TeamB], "caller forfeits everything, belote included")
}

func TestHandScoring_501MatchMode(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	gs.MatchMode = "501"
	gs.TeamScores = [2]int{450, 0}

	result := playTrick8(t, gs)

	assert.Equal(t, game.PhaseMatchEnd, result.Phase, "match should end at 501 threshold")
	assert.GreaterOrEqual(t, result.TeamScores[game.TeamA], 501)
	require.NotNil(t, result.WinnerTeam, "WinnerTeam must be set")
	assert.Equal(t, game.TeamA, *result.WinnerTeam, "Team A wins at 501 threshold")
}

func TestHandScoring_StateImmutability(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	originalTeamScores := gs.TeamScores
	originalHandPoints := gs.HandPoints
	originalPhase := gs.Phase

	_ = playTrick8(t, gs)

	assert.Equal(t, originalTeamScores, gs.TeamScores, "original TeamScores unchanged")
	assert.Equal(t, originalHandPoints, gs.HandPoints, "original HandPoints unchanged")
	assert.Equal(t, originalPhase, gs.Phase, "original Phase unchanged")
}

func TestMatchEnd_SingleTeamReaches1001(t *testing.T) {
	gs := testfixtures.NewGameNearEnd(950, 0)

	result := playTrick8(t, gs)

	assert.Equal(t, game.PhaseMatchEnd, result.Phase)
	require.NotNil(t, result.WinnerTeam)
	assert.Equal(t, game.TeamA, *result.WinnerTeam, "only team A crossed 1001")
}

func TestMatchEnd_BothTeamsExceed1001_HigherScoreWins(t *testing.T) {
	gs := testfixtures.NewGameNearEnd(950, 920)

	result := playTrick8(t, gs)

	assert.Equal(t, game.PhaseMatchEnd, result.Phase)
	require.NotNil(t, result.WinnerTeam)
	assert.Equal(t, game.TeamA, *result.WinnerTeam, "Team A has higher score when both cross 1001")
	assert.Greater(t, result.TeamScores[game.TeamA], result.TeamScores[game.TeamB])
}

func TestMatchEnd_BothTeamsExceed1001_TiedScore_ContractingTeamWins(t *testing.T) {
	gs := testfixtures.NewGameNearEnd(1000, 978)

	result := playTrick8(t, gs)

	assert.Equal(t, game.PhaseMatchEnd, result.Phase)
	require.NotNil(t, result.WinnerTeam)
	assert.Equal(t, result.TeamScores[game.TeamA], result.TeamScores[game.TeamB],
		"scores should be tied")
	assert.Equal(t, game.TeamB, *result.WinnerTeam,
		"contracting team wins tiebreaker")
}

func TestMatchEnd_WinnerTeamFieldSet(t *testing.T) {
	t.Run("nil when match continues", func(t *testing.T) {
		gs := testfixtures.NewGameNearEnd(100, 200)
		hc := playTrick8(t, gs)
		require.Equal(t, game.PhaseHandComplete, hc.Phase, "hand holds for the continue pause")
		assert.Nil(t, hc.WinnerTeam, "WinnerTeam should be nil when match continues")

		result, err := game.ForceAdvanceHandComplete(hc)
		require.NoError(t, err)
		assert.Equal(t, game.PhaseDealing, result.Phase)
		assert.Nil(t, result.WinnerTeam, "WinnerTeam should be nil when match continues")
	})

	t.Run("set when match ends", func(t *testing.T) {
		gs := testfixtures.NewGameNearEnd(950, 0)
		result := playTrick8(t, gs)

		assert.Equal(t, game.PhaseMatchEnd, result.Phase)
		require.NotNil(t, result.WinnerTeam, "WinnerTeam must be set on match end")
	})
}

func TestMatchEnd_501Mode(t *testing.T) {
	gs := testfixtures.NewGameNearEnd(450, 0)
	gs.MatchMode = "501"

	result := playTrick8(t, gs)

	assert.Equal(t, game.PhaseMatchEnd, result.Phase)
	require.NotNil(t, result.WinnerTeam)
	assert.Equal(t, game.TeamA, *result.WinnerTeam)
	assert.GreaterOrEqual(t, result.TeamScores[game.TeamA], 501)
}

func TestMatchEnd_BothTeamsExceed501_HigherScoreWins(t *testing.T) {
	gs := testfixtures.NewGameNearEnd(450, 420)
	gs.MatchMode = "501"

	result := playTrick8(t, gs)

	assert.Equal(t, game.PhaseMatchEnd, result.Phase)
	require.NotNil(t, result.WinnerTeam)
	assert.Equal(t, game.TeamA, *result.WinnerTeam, "Team A has higher score when both cross 501")
	assert.Greater(t, result.TeamScores[game.TeamA], result.TeamScores[game.TeamB])
}

func TestMatchEnd_BothTeamsExceed501_TiedScore_ContractingTeamWins(t *testing.T) {
	gs := testfixtures.NewGameNearEnd(500, 478)
	gs.MatchMode = "501"

	result := playTrick8(t, gs)

	assert.Equal(t, game.PhaseMatchEnd, result.Phase)
	require.NotNil(t, result.WinnerTeam)
	assert.Equal(t, result.TeamScores[game.TeamA], result.TeamScores[game.TeamB],
		"scores should be tied")
	assert.Equal(t, game.TeamB, *result.WinnerTeam,
		"contracting team wins tiebreaker at 501")
}

func TestMatchEnd_501Mode_ContinuesBelowThreshold(t *testing.T) {
	gs := testfixtures.NewGameNearEnd(300, 200)
	gs.MatchMode = "501"

	result := playTrick8(t, gs)

	assert.Equal(t, game.PhaseHandComplete, result.Phase, "hand holds for the continue pause")
	assert.Nil(t, result.WinnerTeam, "WinnerTeam should be nil when match continues")
}

func TestMatchEnd_501Mode_ExactlyAtThreshold(t *testing.T) {
	gs := testfixtures.NewGameNearEnd(431, 400)
	gs.MatchMode = "501"

	result := playTrick8(t, gs)

	assert.Equal(t, game.PhaseMatchEnd, result.Phase, "match ends at exactly 501")
	require.NotNil(t, result.WinnerTeam)
	assert.Equal(t, game.TeamA, *result.WinnerTeam)
	assert.Equal(t, 501, result.TeamScores[game.TeamA])
}

func TestMatchTarget(t *testing.T) {
	tests := []struct {
		mode string
		want int
	}{
		{"501", 501},
		{"701", 701},
		{"1001", 1001},
		{"", 1001},
		{"999", 1001},
	}

	for _, tc := range tests {
		t.Run("mode "+tc.mode, func(t *testing.T) {
			assert.Equal(t, tc.want, game.MatchTarget(tc.mode))
		})
	}
}

func TestMatchEnd_701Mode(t *testing.T) {
	t.Run("ends at exactly 701", func(t *testing.T) {
		gs := testfixtures.NewGameNearEnd(631, 600)
		gs.MatchMode = "701"

		result := playTrick8(t, gs)

		assert.Equal(t, game.PhaseMatchEnd, result.Phase)
		require.NotNil(t, result.WinnerTeam)
		assert.Equal(t, game.TeamA, *result.WinnerTeam)
		assert.Equal(t, 701, result.TeamScores[game.TeamA])
		assert.Less(t, result.TeamScores[game.TeamB], 701)
	})

	t.Run("plays on at 700", func(t *testing.T) {
		gs := testfixtures.NewGameNearEnd(630, 600)
		gs.MatchMode = "701"

		result := playTrick8(t, gs)

		assert.Equal(t, game.PhaseHandComplete, result.Phase, "700 is not 701")
		assert.Nil(t, result.WinnerTeam)
		assert.Equal(t, 700, result.TeamScores[game.TeamA])
	})
}

func TestMatchEnd_BlueTeamWins(t *testing.T) {
	gs := testfixtures.NewGameNearEnd(0, 920)

	result := playTrick8(t, gs)

	assert.Equal(t, game.PhaseMatchEnd, result.Phase)
	require.NotNil(t, result.WinnerTeam)
	assert.Equal(t, game.TeamB, *result.WinnerTeam, "Team B should win")
}

func TestInstantWin_NotTriggered_PartialTrump(t *testing.T) {
	gs := testfixtures.NewGameJustDealt()
	candidate := game.Card{Rank: game.Rank7, Suit: game.SuitHearts}
	gs.TrumpCandidate = &candidate
	gs.Players[1].Hand = []game.Card{
		{Rank: game.Rank8, Suit: game.SuitHearts},
		{Rank: game.Rank9, Suit: game.SuitHearts},
		{Rank: game.RankTen, Suit: game.SuitHearts},
		{Rank: game.RankJack, Suit: game.SuitHearts},
		{Rank: game.RankQueen, Suit: game.SuitHearts},
	}
	gs.Deck = []game.Card{
		{Rank: game.RankKing, Suit: game.SuitHearts},
		{Rank: game.RankQueen, Suit: game.SuitSpades},
		{Rank: game.RankKing, Suit: game.SuitSpades},
		{Rank: game.RankAce, Suit: game.SuitSpades},
		{Rank: game.RankAce, Suit: game.SuitHearts},
		{Rank: game.RankQueen, Suit: game.SuitDiamonds},
		{Rank: game.RankKing, Suit: game.SuitDiamonds},
		{Rank: game.RankAce, Suit: game.SuitDiamonds},
		{Rank: game.RankQueen, Suit: game.SuitClubs},
		{Rank: game.RankKing, Suit: game.SuitClubs},
		{Rank: game.RankAce, Suit: game.SuitClubs},
	}

	result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPickTrump, PlayerSeat: 1})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, game.PhasePlaying, result.Phase, "7 trump cards must NOT trigger instant-win")
	assert.Nil(t, result.WinnerTeam)

	hearts := 0
	for _, c := range result.Players[1].Hand {
		if c.Suit == game.SuitHearts {
			hearts++
		}
	}
	assert.Equal(t, 7, hearts, "picker should hold exactly 7 hearts (one short of instant-win)")
}

func TestInstantWin_TriggeredOnPick(t *testing.T) {
	gs := testfixtures.NewGameJustDealt()
	candidate := game.Card{Rank: game.Rank7, Suit: game.SuitHearts}
	gs.TrumpCandidate = &candidate
	gs.Players[1].Hand = []game.Card{
		{Rank: game.Rank8, Suit: game.SuitHearts},
		{Rank: game.Rank9, Suit: game.SuitHearts},
		{Rank: game.RankTen, Suit: game.SuitHearts},
		{Rank: game.RankJack, Suit: game.SuitHearts},
		{Rank: game.RankQueen, Suit: game.SuitHearts},
	}
	gs.Deck = []game.Card{
		{Rank: game.RankKing, Suit: game.SuitHearts},
		{Rank: game.RankAce, Suit: game.SuitHearts},
		{Rank: game.RankQueen, Suit: game.SuitSpades},
		{Rank: game.RankKing, Suit: game.SuitSpades},
		{Rank: game.RankAce, Suit: game.SuitSpades},
		{Rank: game.RankQueen, Suit: game.SuitDiamonds},
		{Rank: game.RankKing, Suit: game.SuitDiamonds},
		{Rank: game.RankAce, Suit: game.SuitDiamonds},
		{Rank: game.RankQueen, Suit: game.SuitClubs},
		{Rank: game.RankKing, Suit: game.SuitClubs},
		{Rank: game.RankAce, Suit: game.SuitClubs},
	}

	result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPickTrump, PlayerSeat: 1})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, game.PhaseMatchEnd, result.Phase, "instant-win should set match-end phase")
	require.NotNil(t, result.WinnerTeam, "instant-win must populate WinnerTeam")
	assert.Equal(t, game.TeamB, *result.WinnerTeam, "seat 1 holds all 8 hearts → team B wins")
}

func TestLastHandResult_NormalHand(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()

	result := playTrick8(t, gs)

	require.NotNil(t, result.LastHandResult, "LastHandResult must be populated after hand scoring")
	hr := result.LastHandResult

	assert.Equal(t, 70, hr.TeamACardPoints, "Team A card points before bonus")
	assert.Equal(t, 82, hr.TeamBCardPoints, "Team B card points before bonus")
	assert.Equal(t, 0, hr.TeamADeclPoints, "Team A declaration points")
	assert.Equal(t, 0, hr.TeamBDeclPoints, "Team B declaration points")
	assert.Equal(t, game.TeamB, hr.LastTrickTeam, "Team B won last trick")
	assert.Equal(t, 10, hr.LastTrickBonus, "last-trick bonus is +10")
	assert.False(t, hr.Capot, "not a capot")
	assert.Nil(t, hr.CapotTeam, "no capot team")
	assert.Equal(t, 0, hr.CapotBonus, "no capot bonus")
	assert.False(t, hr.FailedContract, "not a failed contract")
	assert.Equal(t, game.TeamB, hr.ContractingTeam, "Team B (seat 1) called trump")

	assert.Equal(t, hr.TeamAHandTotal+hr.TeamBHandTotal,
		result.TeamScores[game.TeamA]+result.TeamScores[game.TeamB],
		"hand totals should equal team scores (starting from 0)")
}

func TestLastHandResult_Capot(t *testing.T) {
	gs := testfixtures.NewGameCapotInProgress()

	result := playTrick8(t, gs)

	require.NotNil(t, result.LastHandResult)
	hr := result.LastHandResult

	assert.True(t, hr.Capot, "should be capot")
	require.NotNil(t, hr.CapotTeam, "capot team must be set")
	assert.Equal(t, game.TeamA, *hr.CapotTeam, "Team A got capot")
	assert.Equal(t, 100, hr.CapotBonus, "capot bonus is 100")
	assert.Equal(t, 0, hr.LastTrickBonus, "last-trick bonus is 0 when capot")
	assert.False(t, hr.FailedContract, "capot team is also contracting, so not a failed contract")
	assert.Equal(t, 252, hr.TeamAHandTotal, "Team A gets all 152 card points + 100 capot bonus")
	assert.Equal(t, 0, hr.TeamBHandTotal, "Team B gets nothing")
}

func TestLastHandResult_FailedContract(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	gs.HandPoints = [2]int{100, 20}

	result := playTrick8(t, gs)

	require.NotNil(t, result.LastHandResult)
	hr := result.LastHandResult

	assert.True(t, hr.FailedContract, "should be a failed contract")
	assert.Equal(t, game.TeamB, hr.ContractingTeam, "Team B (seat 1) called trump")
	assert.Equal(t, 0, hr.TeamBHandTotal, "contracting team gets 0 on failed contract")
	assert.Equal(t, 151, hr.TeamAHandTotal, "opposing team gets all points")
}

func TestLastHandResult_WithDeclarations(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	gs.DeclarationPoints = [2]int{50, 20}
	gs.TeamScores = [2]int{0, 0}

	result := playTrick8(t, gs)

	require.NotNil(t, result.LastHandResult)
	hr := result.LastHandResult

	assert.Equal(t, 50, hr.TeamADeclPoints, "Team A declaration points")
	assert.Equal(t, 20, hr.TeamBDeclPoints, "Team B declaration points")
}

func TestLastHandResult_MatchEnd(t *testing.T) {
	gs := testfixtures.NewGameNearEnd(950, 0)

	result := playTrick8(t, gs)

	assert.Equal(t, game.PhaseMatchEnd, result.Phase)
	require.NotNil(t, result.LastHandResult, "LastHandResult must persist on match end (no startNewHand)")
	assert.Equal(t, result.LastHandResult.TeamAHandTotal,
		result.TeamScores[game.TeamA]-950,
		"hand total should equal the score delta from match start")
}

func TestLastHandResult_TotalsMatchTeamScoreDelta(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	gs.TeamScores = [2]int{200, 300}

	result := playTrick8(t, gs)

	require.NotNil(t, result.LastHandResult)
	hr := result.LastHandResult

	teamADelta := result.TeamScores[game.TeamA] - 200
	teamBDelta := result.TeamScores[game.TeamB] - 300
	assert.Equal(t, hr.TeamAHandTotal, teamADelta, "TeamAHandTotal matches actual team A score increase")
	assert.Equal(t, hr.TeamBHandTotal, teamBDelta, "TeamBHandTotal matches actual team B score increase")
}

func TestInstantWin_FirstHand(t *testing.T) {
	for range 50 {
		gs := game.NewGame([4]uint{10, 20, 30, 40}, [4]string{"p1", "p2", "p3", "p4"}, [4]bool{}, game.VariantBitola, "1001", 1, true, false, true, false)
		assert.Equal(t, game.PhaseDealing, gs.Phase, "NewGame returns PhaseDealing post stage-1")
		assert.Nil(t, gs.WinnerTeam, "instant-win is deferred to stage-2 (post-pick)")
		assert.Len(t, gs.Deck, 11, "11 cards remain for stage-2")
		require.NotNil(t, gs.TrumpCandidate, "candidate revealed during stage-1")
		for i, p := range gs.Players {
			assert.Len(t, p.Hand, 5, "seat %d holds 5 cards after stage-1", i)
		}
	}
}
