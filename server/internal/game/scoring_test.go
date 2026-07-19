package game_test

import (
	"testing"

	game "github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// playTrick8 plays the last trick by having each player play their single
// remaining card in seat order starting from the active player.
// Returns the final state after all 4 cards are played and scoring completes.
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

	// Seat 0 (team A) leads with AS (Ace of Spades)
	// Seat 1 plays 8D, seat 2 plays TD, seat 3 plays 7H (trump)
	// Trump 7H beats non-trump cards — seat 3 (team B) wins trick 8
	result := playTrick8(t, gs)

	// After trick 8 + scoring, state should be PhaseBidding (new hand) or PhaseMatchEnd
	assert.NotEqual(t, game.PhasePlaying, result.Phase, "should have left playing phase")

	// Team B (seat 3) won trick 8 — check that +10 bonus was applied
	// Initial: team A=70, team B=61. Trick 8 cards: AS(11) + 8D(0) + TD(10) + 7H(0) = 21
	// Trump 7H wins (it's a trump card beating non-trump), so team B gets 21 card pts + 10 bonus
	// Team A total: 70 + 0 (declarations) = 70
	// Team B total: 61 + 21 (trick 8 card points) + 10 (last trick bonus) + 0 (declarations) = 92
	// Team B is contracting team (seat 1), and team B total (92) > team A total (70), so normal scoring
	// TeamScores: team A += 70, team B += 92
	assert.Equal(t, 70, result.TeamScores[game.TeamA], "Team A score")
	assert.Equal(t, 92, result.TeamScores[game.TeamB], "Team B score (includes +10 bonus)")
}

func TestHandScoring_LastTrickSeatPreserved(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()

	hc := playTrick8(t, gs)

	// Seat 3 (trump 7H) wins trick 8 → team B. The hand holds complete with the
	// live winner still set and recorded in the hand result for the broadcast.
	require.Equal(t, game.PhaseHandComplete, hc.Phase)
	require.NotNil(t, hc.TrickWinnerSeat)
	assert.Equal(t, 3, *hc.TrickWinnerSeat)
	require.NotNil(t, hc.LastHandResult)
	assert.Equal(t, 3, hc.LastHandResult.LastTrickSeat, "winner seat recorded for broadcast")
	assert.Equal(t, game.TeamB, hc.LastHandResult.LastTrickTeam, "seat 3 is team B")

	// Advancing deals the next hand: startNewHand clears the live TrickWinnerSeat,
	// but the seat survives in LastHandResult so a late/duplicate broadcast still
	// resolves the right winner.
	next, err := game.ForceAdvanceHandComplete(hc)
	require.NoError(t, err)
	assert.Nil(t, next.TrickWinnerSeat, "startNewHand clears the live winner field")
	require.NotNil(t, next.LastHandResult)
	assert.Equal(t, 3, next.LastHandResult.LastTrickSeat, "winner seat preserved across startNewHand")
}

func TestHandScoring_CapotScoring(t *testing.T) {
	gs := testfixtures.NewGameCapotInProgress()

	// Seat 0 leads with JH (trump Jack, strongest card) — team A wins
	result := playTrick8(t, gs)

	// Team A won all 8 tricks — Capot!
	// HandPoints before scoring: team A=121, team B=0
	// Trick 8: JH(20 trump) + 7D(0) + AH(11 trump) + 7C(0) = 31 pts to team A
	// After trick resolve: team A HandPoints = 121 + 31 = 152 (all card points)
	// Capot bonus: +100 (replaces +10 last-trick)
	// Team A total: 152 + 100 = 252 + 0 declarations = 252
	// Team B total: 0
	// Team A is contracting team (seat 0), team A total > team B total → normal scoring
	assert.Equal(t, 252, result.TeamScores[game.TeamA], "Team A gets all 152 card pts + 100 Capot")
	assert.Equal(t, 0, result.TeamScores[game.TeamB], "Team B gets nothing")
}

// Regression: a Capot (one team wins all 8 tricks) leaves the no-trick team
// with NOTHING — it forfeits even declarations it won in the declaration
// contest, and those points flow to the Capot team. Reported bug: opponents
// made Capot but the no-trick team kept its 40 declaration points instead of
// them transferring to the Capot side.
func TestHandScoring_CapotForfeitsLosersDeclarations(t *testing.T) {
	gs := testfixtures.NewGameCapotInProgress()
	// Team A (contracting, seat 0) is on track for Capot. Team B won the
	// declaration contest (40 pts) but takes no trick this hand.
	gs.DeclarationPoints = [2]int{0, 40}

	result := playTrick8(t, gs)

	// Team A: 152 card pts + 100 Capot bonus + Team B's forfeited 40 = 292.
	// Team B: 0 — a team that wins no trick banks nothing, declarations included.
	assert.Equal(t, 292, result.TeamScores[game.TeamA], "Capot team takes everything, incl. opponent's declarations")
	assert.Equal(t, 0, result.TeamScores[game.TeamB], "no-trick team scores 0, forfeiting its declarations")

	require.NotNil(t, result.LastHandResult)
	assert.True(t, result.LastHandResult.Capot, "should be a capot")
	assert.Equal(t, 292, result.LastHandResult.TeamAHandTotal, "Team A hand total includes forfeited declarations")
	assert.Equal(t, 0, result.LastHandResult.TeamBHandTotal, "Team B hand total is 0")
}

func TestHandScoring_CapotBroken(t *testing.T) {
	gs := testfixtures.NewGameCapotInProgress()
	// Swap seat 0 and seat 1 cards so team B can win trick 8
	// Give seat 1 the JH (trump Jack) and seat 0 the 7D
	gs.Players[0].Hand = []game.Card{{Rank: game.Rank7, Suit: game.SuitDiamonds}}
	gs.Players[1].Hand = []game.Card{{Rank: game.RankJack, Suit: game.SuitHearts}}

	result := playTrick8(t, gs)

	// Team A won 7, team B wins trick 8 → TricksWon = [7, 1] — no Capot
	// Last-trick bonus (+10) goes to team B (seat 1 wins with JH)
	// Trick 8: 7D(0) + JH(20 trump) + AH(11 trump) + 7C(0) = 31 pts to team B
	// After trick resolve: team A=121, team B=0+31=31
	// Last-trick bonus: team B += 10 → team B=41
	// Team A total: 121 + 0 = 121, team B total: 41 + 0 = 41
	// Team A is contracting (seat 0), team A(121) > team B(41) → normal scoring
	assert.Equal(t, 121, result.TeamScores[game.TeamA], "Team A keeps their card points")
	assert.Equal(t, 41, result.TeamScores[game.TeamB], "Team B gets trick 8 pts + 10 bonus")
}

func TestHandScoring_FailedContract(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	// Team B (seat 1) is contracting team. Adjust HandPoints so team B has fewer total.
	// Set team A high, team B low to guarantee failed contract after trick 8.
	gs.HandPoints = [2]int{100, 20}

	result := playTrick8(t, gs)

	// Trick 8: AS(11) + 8D(0) + TD(10) + 7H(0 trump) = 21 pts
	// 7H is trump, so team B (seat 3) wins trick 8 → team B gets 21 pts + 10 bonus
	// Team A total: 100 + 0 (declarations) = 100
	// Team B total: 20 + 21 + 10 + 0 (declarations) = 51
	// Team B is contracting (seat 1), team B(51) < team A(100) → FAILED CONTRACT
	// Team A gets ALL: 100 + 51 = 151
	assert.Equal(t, 151, result.TeamScores[game.TeamA], "Team A gets ALL points on failed contract")
	assert.Equal(t, 0, result.TeamScores[game.TeamB], "Team B gets 0 on failed contract")
}

func TestHandScoring_EqualPointsIsFailure(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	// Team B (seat 1) is contracting. Arrange so totals are EQUAL after trick 8.
	// Trick 8: 7H(trump) wins, team B gets 21 card pts + 10 bonus = 31
	// Set: team A=80, team B=49 → after trick 8: team A=80, team B=49+21+10=80. Equal!
	gs.HandPoints = [2]int{80, 49}

	result := playTrick8(t, gs)

	// Equal totals (80 = 80): the trump-calling team must score STRICTLY MORE to
	// succeed, so a tie is a FAILED hand → all points transfer to the opponents.
	assert.Equal(t, 160, result.TeamScores[game.TeamA], "Team A (opponent) gets ALL points on a tie")
	assert.Equal(t, 0, result.TeamScores[game.TeamB], "Team B (caller) gets 0 — a tie is a failure")
	require.NotNil(t, result.LastHandResult)
	assert.True(t, result.LastHandResult.FailedContract, "a tie marks the hand failed for the caller")
}

func TestHandScoring_TieFailsTrumpCaller(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	// Canonical case: an exact 81:81 split of the 162 base points.
	// Team B (seat 1) is the trump caller. Trick 8 (7H trump wins) gives team B
	// 21 card pts + 10 last-trick bonus = 31. Set team B=50 so it lands on 81;
	// team A=81. Final totals: A=81, B=81 — a tie.
	gs.HandPoints = [2]int{81, 50}
	gs.TeamScores = [2]int{0, 0}

	result := playTrick8(t, gs)

	// 81:81 — the caller (team B) did not score strictly more, so the hand fails:
	// team B loses its points and team A wins all 162.
	require.NotNil(t, result.LastHandResult)
	assert.True(t, result.LastHandResult.FailedContract, "81:81 is a failed hand for the caller")
	assert.Equal(t, game.TeamB, result.LastHandResult.ContractingTeam, "team B called trump")
	assert.Equal(t, 162, result.TeamScores[game.TeamA], "opponents win all 162 points")
	assert.Equal(t, 0, result.TeamScores[game.TeamB], "caller scores 0 on the tie")
}

func TestHandScoring_TieWithDeclarationsFailsTrumpCaller(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	// Tie where declarations are part of both the totals and the transferred
	// pool: 162 base points + 20 declarations = 182 in play. Team B (seat 1) is
	// the trump caller. Trick 8 (7H trump wins) gives team B 21 card pts + 10
	// last-trick bonus. Team A holds a tierce (+20): A = 71 + 20 = 91,
	// B = 60 + 21 + 10 = 91 — a 91:91 tie.
	gs.HandPoints = [2]int{71, 60}
	gs.DeclarationPoints = [2]int{20, 0}
	gs.TeamScores = [2]int{0, 0}

	result := playTrick8(t, gs)

	// 91:91 — the caller fails and the opponents collect the ENTIRE pool,
	// declarations included: 0:182.
	require.NotNil(t, result.LastHandResult)
	assert.True(t, result.LastHandResult.FailedContract, "91:91 is a failed hand for the caller")
	assert.Equal(t, 182, result.TeamScores[game.TeamA], "opponents win all 182 points incl. declarations")
	assert.Equal(t, 0, result.TeamScores[game.TeamB], "caller scores 0 on the tie")
	assert.Equal(t, 182, result.LastHandResult.TeamAHandTotal, "broadcast total mirrors the transfer")
	assert.Equal(t, 0, result.LastHandResult.TeamBHandTotal, "broadcast total mirrors the transfer")
}

func TestHandScoring_NormalScoring(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	// Team B is contracting. Set points so team B wins comfortably after trick 8.
	gs.HandPoints = [2]int{40, 91}

	result := playTrick8(t, gs)

	// Trick 8: 7H(trump) wins, team B gets 21 card pts + 10 bonus = 31
	// Team A total: 40, team B total: 91 + 21 + 10 = 122
	// Team B is contracting, team B(122) > team A(40) → normal scoring
	assert.Equal(t, 40, result.TeamScores[game.TeamA], "Team A keeps own points")
	assert.Equal(t, 122, result.TeamScores[game.TeamB], "Team B keeps own points")
}

func TestHandScoring_MatchEndTriggered(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	// Set TeamScores near 1001 so scoring pushes over
	gs.TeamScores = [2]int{950, 0}

	result := playTrick8(t, gs)

	// Team A starts at 950. After trick 8 scoring, team A gains their hand points.
	// This should push team A over 1001 → PhaseMatchEnd
	assert.Equal(t, game.PhaseMatchEnd, result.Phase, "match should end when team crosses 1001")
	assert.GreaterOrEqual(t, result.TeamScores[game.TeamA], 1001)
	require.NotNil(t, result.WinnerTeam, "WinnerTeam must be set on match end")
	assert.Equal(t, game.TeamA, *result.WinnerTeam, "Team A should win")
}

func TestHandScoring_MatchContinues(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	// TeamScores well below 1001
	gs.TeamScores = [2]int{100, 200}

	hc := playTrick8(t, gs)
	require.Equal(t, game.PhaseHandComplete, hc.Phase, "hand holds for the continue pause before dealing")
	result, err := game.ForceAdvanceHandComplete(hc)
	require.NoError(t, err)

	// Neither team reaches 1001 → new hand starts (dealing phase before session manager transitions to bidding)
	assert.Equal(t, game.PhaseDealing, result.Phase, "should start new hand in dealing phase")
	assert.Equal(t, 2, result.HandNumber, "hand number should increment")
	// Dealer rotates: was 0, now 1
	assert.Equal(t, 1, result.DealerSeat, "dealer should rotate")
	// New deal is stage-1 (Bitola): 5 cards per seat, 11-card Deck, candidate visible.
	for i, p := range result.Players {
		assert.Len(t, p.Hand, 5, "player at seat %d should have 5 cards after stage-1 re-deal", i)
	}
	assert.Len(t, result.Deck, 11, "Deck should hold 11 cards for stage-2 of the new hand")
	require.NotNil(t, result.TrumpCandidate, "candidate revealed for the new hand")
}

func TestHandScoring_NewHandStateReset(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	gs.TeamScores = [2]int{0, 0}
	gs.DeclarationPoints = [2]int{50, 20} // Set declarations to verify they're included in scoring then reset

	hc := playTrick8(t, gs)
	require.Equal(t, game.PhaseHandComplete, hc.Phase, "hand holds for the continue pause before dealing")
	result, err := game.ForceAdvanceHandComplete(hc)
	require.NoError(t, err)

	assert.Equal(t, game.PhaseDealing, result.Phase)

	// Per-hand fields must be reset
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
	gs.DeclarationPoints = [2]int{50, 0} // Team A has 50 declaration points

	result := playTrick8(t, gs)

	// Team A total: 70 (hand) + 50 (declarations) = 120
	// Team B wins trick 8 with 7H trump: team B gets 21 + 10 = 31
	// Team B total: 61 + 31 + 0 (declarations) = 92
	// Team B is contracting (seat 1), team B(92) < team A(120) → FAILED CONTRACT
	// Team A gets ALL: 120 + 92 = 212
	assert.Equal(t, 212, result.TeamScores[game.TeamA], "declarations included in scoring")
	assert.Equal(t, 0, result.TeamScores[game.TeamB], "failed contract = 0 for contracting team")
}

func TestHandScoring_FailedContractBothTeamsHaveDeclarations(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	// Team B (seat 1) is contracting. Give both teams declaration points.
	gs.HandPoints = [2]int{60, 20}
	gs.DeclarationPoints = [2]int{40, 30}

	result := playTrick8(t, gs)

	// Trick 8: 7H (trump) wins, team B gets 21 card pts + 10 bonus
	// Team A total: 60 + 40 (declarations) = 100
	// Team B total: 20 + 21 + 10 + 30 (declarations) = 81
	// Team B is contracting (seat 1), team B(81) < team A(100) → FAILED CONTRACT
	// Team A gets ALL: 100 + 81 = 181 (includes team B's own declarations)
	assert.Equal(t, 181, result.TeamScores[game.TeamA], "Team A gets ALL points including team B's declarations")
	assert.Equal(t, 0, result.TeamScores[game.TeamB], "Team B gets 0 on failed contract")
}

func TestHandScoring_BelotBonusIncluded(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	// Team A announced belote this hand (K+Q of trump) — 20 pts, tracked in
	// BelotPoints (a declaration), NOT in HandPoints (card points).
	gs.HandPoints = [2]int{70, 61}
	gs.BelotPoints = [2]int{20, 0}

	result := playTrick8(t, gs)

	// Team A total: 70 card + 0 decl + 20 belote = 90
	// Team B gets trick 8: 21 card pts + 10 bonus → team B total: 61 + 21 + 10 = 92
	// Team B is contracting (seat 1), team B(92) > team A(90) → normal scoring
	assert.Equal(t, 90, result.TeamScores[game.TeamA], "Team A total includes the belote bonus")
	assert.Equal(t, 92, result.TeamScores[game.TeamB], "Team B keeps own total")
}

// Belote/rebelote (K+Q of trump, 20 pts) must be classified as a DECLARATION,
// not trick-taking card points — in the score breakdown that drives the
// scoreboard, match history, and broadcast. It still counts toward the team's
// hand total (and so transfers on a failed contract / capot).
func TestHandScoring_BeloteCountsAsDeclaration(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	gs.HandPoints = [2]int{40, 91} // team B (caller) clearly ahead → normal scoring
	gs.BelotPoints = [2]int{20, 0} // team A announced belote

	result := playTrick8(t, gs)
	hr := result.LastHandResult
	require.NotNil(t, hr)

	assert.Equal(t, 20, hr.TeamADeclPoints, "belote is folded into declaration points")
	assert.Equal(t, 40, hr.TeamACardPoints, "belote is NOT counted as card points")
	// Trick 8: 7H trump wins for team B (+21 card, +10 bonus). Normal scoring.
	assert.Equal(t, 60, result.TeamScores[game.TeamA], "team A total = 40 card + 20 belote declaration")
}

// On a failed contract the no-trick/loser side forfeits everything including
// belote — the belote declaration transfers with the rest of the points.
func TestHandScoring_BeloteForfeitedOnFailedContract(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	// Team B (seat 1) is the caller. Make team B fall short so the contract fails.
	gs.HandPoints = [2]int{100, 20}
	gs.BelotPoints = [2]int{0, 20} // caller (team B) announced belote

	result := playTrick8(t, gs)

	// Trick 8: 7H trump wins for team B → +21 card +10 bonus.
	// Team A total: 100. Team B total: 20 + 21 + 10 + 20 belote = 71.
	// Caller team B (71) < team A (100) → FAILED: all 171 to team A, incl. belote.
	assert.Equal(t, 171, result.TeamScores[game.TeamA], "opponent gets all points including the caller's belote")
	assert.Equal(t, 0, result.TeamScores[game.TeamB], "caller forfeits everything, belote included")
}

func TestHandScoring_501MatchMode(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()
	gs.MatchMode = "501"
	gs.TeamScores = [2]int{450, 0}

	result := playTrick8(t, gs)

	// Team A starts at 450. After scoring, team A gains 70 pts → 520 >= 501
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

	// Original state should be unchanged (cloneGameState in handlePlayCard protects it)
	assert.Equal(t, originalTeamScores, gs.TeamScores, "original TeamScores unchanged")
	assert.Equal(t, originalHandPoints, gs.HandPoints, "original HandPoints unchanged")
	assert.Equal(t, originalPhase, gs.Phase, "original Phase unchanged")
}

// --- Match-End Tiebreaker Tests (Story 3.6) ---

func TestMatchEnd_SingleTeamReaches1001(t *testing.T) {
	gs := testfixtures.NewGameNearEnd(950, 0)

	result := playTrick8(t, gs)

	assert.Equal(t, game.PhaseMatchEnd, result.Phase)
	require.NotNil(t, result.WinnerTeam)
	assert.Equal(t, game.TeamA, *result.WinnerTeam, "only team A crossed 1001")
}

func TestMatchEnd_BothTeamsExceed1001_HigherScoreWins(t *testing.T) {
	// Team B is contracting (seat 1). After trick 8:
	// Trick 8: 7H (trump) wins → team B gets 21 + 10 = 31
	// Team B total: 61 + 31 = 92 → team B contracting succeeds (92 > 70)
	// Normal scoring: team A += 70, team B += 92
	// Team A final: 950 + 70 = 1020, team B final: 920 + 92 = 1012
	// Both >= 1001, team A has higher score → team A wins
	gs := testfixtures.NewGameNearEnd(950, 920)

	result := playTrick8(t, gs)

	assert.Equal(t, game.PhaseMatchEnd, result.Phase)
	require.NotNil(t, result.WinnerTeam)
	assert.Equal(t, game.TeamA, *result.WinnerTeam, "Team A has higher score when both cross 1001")
	assert.Greater(t, result.TeamScores[game.TeamA], result.TeamScores[game.TeamB])
}

func TestMatchEnd_BothTeamsExceed1001_TiedScore_ContractingTeamWins(t *testing.T) {
	// Need both teams to end at EXACTLY the same score.
	// Team B is contracting (seat 1). After trick 8:
	// 7H (trump) wins → team B gets trick 8 (21 pts) + last-trick bonus (10) = 31
	// Team B total: 61 + 31 = 92, team A total: 70
	// Team B(92) > team A(70) → normal scoring: team A += 70, team B += 92
	//
	// We need: teamAScore + 70 == teamBScore + 92
	// So: teamAScore - teamBScore = 22
	// Example: team A=1000, team B=978 → team A final=1070, team B final=1070
	gs := testfixtures.NewGameNearEnd(1000, 978)

	result := playTrick8(t, gs)

	assert.Equal(t, game.PhaseMatchEnd, result.Phase)
	require.NotNil(t, result.WinnerTeam)
	assert.Equal(t, result.TeamScores[game.TeamA], result.TeamScores[game.TeamB],
		"scores should be tied")
	// Team B is contracting team (seat 1 = team B)
	assert.Equal(t, game.TeamB, *result.WinnerTeam,
		"contracting team wins tiebreaker")
}

func TestMatchEnd_WinnerTeamFieldSet(t *testing.T) {
	// Verify WinnerTeam is nil when match continues, set when match ends
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
	// 501 analog of TestMatchEnd_BothTeamsExceed1001_HigherScoreWins.
	// Trick 8 awards team A += 70, team B += 92 (team B contracting, succeeds).
	// Team A final: 450 + 70 = 520, team B final: 420 + 92 = 512
	// Both >= 501, team A has higher score → team A wins
	gs := testfixtures.NewGameNearEnd(450, 420)
	gs.MatchMode = "501"

	result := playTrick8(t, gs)

	assert.Equal(t, game.PhaseMatchEnd, result.Phase)
	require.NotNil(t, result.WinnerTeam)
	assert.Equal(t, game.TeamA, *result.WinnerTeam, "Team A has higher score when both cross 501")
	assert.Greater(t, result.TeamScores[game.TeamA], result.TeamScores[game.TeamB])
}

func TestMatchEnd_BothTeamsExceed501_TiedScore_ContractingTeamWins(t *testing.T) {
	// 501 analog of TestMatchEnd_BothTeamsExceed1001_TiedScore_ContractingTeamWins.
	// Need: teamAScore + 70 == teamBScore + 92, i.e. teamAScore - teamBScore = 22,
	// with both finals >= 501: team A=500, team B=478 → both end at 570.
	gs := testfixtures.NewGameNearEnd(500, 478)
	gs.MatchMode = "501"

	result := playTrick8(t, gs)

	assert.Equal(t, game.PhaseMatchEnd, result.Phase)
	require.NotNil(t, result.WinnerTeam)
	assert.Equal(t, result.TeamScores[game.TeamA], result.TeamScores[game.TeamB],
		"scores should be tied")
	// Team B is contracting team (seat 1 = team B)
	assert.Equal(t, game.TeamB, *result.WinnerTeam,
		"contracting team wins tiebreaker at 501")
}

func TestMatchEnd_501Mode_ContinuesBelowThreshold(t *testing.T) {
	// Finals 370/292 — below 501, the match must continue even though it
	// would also continue under 1001 rules (guards against an accidental
	// always-end at the lower threshold).
	gs := testfixtures.NewGameNearEnd(300, 200)
	gs.MatchMode = "501"

	result := playTrick8(t, gs)

	assert.Equal(t, game.PhaseHandComplete, result.Phase, "hand holds for the continue pause")
	assert.Nil(t, result.WinnerTeam, "WinnerTeam should be nil when match continues")
}

func TestMatchEnd_501Mode_ExactlyAtThreshold(t *testing.T) {
	// Pins the >= comparison at the exact boundary: team A lands on precisely
	// 501 (431 + 70) while team B stays below (400 + 92 = 492). An accidental
	// > regression would let this hand continue.
	gs := testfixtures.NewGameNearEnd(431, 400)
	gs.MatchMode = "501"

	result := playTrick8(t, gs)

	assert.Equal(t, game.PhaseMatchEnd, result.Phase, "match ends at exactly 501")
	require.NotNil(t, result.WinnerTeam)
	assert.Equal(t, game.TeamA, *result.WinnerTeam)
	assert.Equal(t, 501, result.TeamScores[game.TeamA])
}

func TestMatchEnd_BlueTeamWins(t *testing.T) {
	// Team B is contracting (seat 1). Set team B near 1001 so team B's scoring pushes over.
	// After trick 8: team B total = 92 (see above). Team B needs: teamBScore + 92 >= 1001
	// Set team B at 920: 920 + 92 = 1012 >= 1001
	gs := testfixtures.NewGameNearEnd(0, 920)

	result := playTrick8(t, gs)

	assert.Equal(t, game.PhaseMatchEnd, result.Phase)
	require.NotNil(t, result.WinnerTeam)
	assert.Equal(t, game.TeamB, *result.WinnerTeam, "Team B should win")
}

// --- Instant-Win Tests (Story 3.6) ---
// Note: checkInstantWin is unexported. Deterministic tests are in
// scoring_internal_test.go (package game). These external tests verify
// the plumbing (NewGame integration and partial-trump non-trigger).

func TestInstantWin_NotTriggered_PartialTrump(t *testing.T) {
	// Build a stage-1 state where seat 1 (picker) is one card short of all 8
	// hearts after stage-2: 5 initial hearts, candidate=7H, but Deck[0:2] has
	// only ONE remaining heart and one non-heart. Picker ends with 7 hearts
	// + 1 non-heart — strictly less than 8 trump → no instant-win.
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
	// Deck[0:2] = KH (heart) + QS (non-heart). Picker collects 7 hearts total,
	// not 8. Remaining 9 deck cards are arbitrary unique non-duplicates.
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

	// Sanity: picker actually holds 7 hearts (not 8).
	hearts := 0
	for _, c := range result.Players[1].Hand {
		if c.Suit == game.SuitHearts {
			hearts++
		}
	}
	assert.Equal(t, 7, hearts, "picker should hold exactly 7 hearts (one short of instant-win)")
}

func TestInstantWin_TriggeredOnPick(t *testing.T) {
	// Construct a stage-1 state where seat 1 holds 5 hearts and the deck's
	// stage-2 slice for seat 1 is the remaining hearts (KH AH); together
	// with the candidate (7H) seat 1 ends with all 8 hearts → instant-win.
	gs := testfixtures.NewGameJustDealt()
	// Override deck so seat 1's stage-2 slice [0:2] = remaining hearts; rest unused suits.
	candidate := game.Card{Rank: game.Rank7, Suit: game.SuitHearts}
	gs.TrumpCandidate = &candidate
	// Seat 1 starts with 8H 9H TH JH QH (5 hearts; 7H is the candidate).
	gs.Players[1].Hand = []game.Card{
		{Rank: game.Rank8, Suit: game.SuitHearts},
		{Rank: game.Rank9, Suit: game.SuitHearts},
		{Rank: game.RankTen, Suit: game.SuitHearts},
		{Rank: game.RankJack, Suit: game.SuitHearts},
		{Rank: game.RankQueen, Suit: game.SuitHearts},
	}
	// Deck[0:2] = KH AH so picker (seat 1) collects every heart.
	gs.Deck = []game.Card{
		{Rank: game.RankKing, Suit: game.SuitHearts},
		{Rank: game.RankAce, Suit: game.SuitHearts},
		// Remaining 9 cards are non-hearts (any valid 9 cards we removed); use
		// distinct cards so the deck has no duplicates.
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

// --- LastHandResult Tests (Story 4.6) ---

func TestLastHandResult_NormalHand(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()

	result := playTrick8(t, gs)

	require.NotNil(t, result.LastHandResult, "LastHandResult must be populated after hand scoring")
	hr := result.LastHandResult

	// Team B (seat 3) wins trick 8 with 7H trump
	// Card points before bonus: team A=70, team B=61+21=82
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

	// Verify totals match TeamScores delta
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
	// NewGame now performs only stage-1 (5 cards per seat + candidate +
	// 11-card deck). Instant-win can't be detected at this point, so NewGame
	// always returns PhaseDealing (auto-promoted to PhaseBidding by the
	// session manager) and never sets WinnerTeam.
	for range 50 {
		gs := game.NewGame([4]uint{10, 20, 30, 40}, [4]string{"p1", "p2", "p3", "p4"}, [4]bool{}, game.VariantBitola, "1001", 1)
		assert.Equal(t, game.PhaseDealing, gs.Phase, "NewGame returns PhaseDealing post stage-1")
		assert.Nil(t, gs.WinnerTeam, "instant-win is deferred to stage-2 (post-pick)")
		assert.Len(t, gs.Deck, 11, "11 cards remain for stage-2")
		require.NotNil(t, gs.TrumpCandidate, "candidate revealed during stage-1")
		for i, p := range gs.Players {
			assert.Len(t, p.Hand, 5, "seat %d holds 5 cards after stage-1", i)
		}
	}
}
