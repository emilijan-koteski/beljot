package game_test

import (
	"testing"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stopTrick plays one trick through ApplyAction using each seat's first legal card.
// It returns early once the match ends, so a crossing trick never deals the seats that follow.
func stopTrick(t *testing.T, state *game.GameState, seats []int) *game.GameState {
	t.Helper()
	for _, seat := range seats {
		if state.Phase == game.PhaseMatchEnd {
			return state
		}
		legal := game.LegalCards(state, seat)
		require.NotEmpty(t, legal, "seat %d must have a legal card", seat)
		card := legal[0]
		next, err := game.ApplyAction(state, game.Action{
			Type:       game.ActionPlayCard,
			PlayerSeat: seat,
			Card:       &card,
		})
		require.NoError(t, err, "seat %d playing %s", seat, card)
		state = next
	}
	return state
}

// midPlayTrickPoints replays a trick with the rule off to learn what it is worth and who takes it.
func midPlayTrickPoints(t *testing.T, trickNum int, seats []int) (winnerTeam, points int) {
	t.Helper()
	base := testfixtures.NewGameMidPlayWithScores(trickNum, 0, 0)
	before := base.HandPoints
	after := stopTrick(t, base, seats)
	require.NotEqual(t, game.PhaseMatchEnd, after.Phase, "the control fixture must not end a match")

	winnerTeam = game.TeamB
	if after.HandPoints[game.TeamA] != before[game.TeamA] {
		winnerTeam = game.TeamA
	}
	points = after.HandPoints[winnerTeam] - before[winnerTeam]
	require.Positive(t, points,
		"the fixture's trick must actually be worth something, or the crossing it is supposed to cause is vacuous")
	return winnerTeam, points
}

// TestStopAtTargetEndsMatchOnACrossingTrick is the headline row of the matrix: a
// team crosses part-way through a hand and the match is over on that very
// action, with the remaining tricks never played.
func TestStopAtTargetEndsMatchOnACrossingTrick(t *testing.T) {
	const (
		trickNum = 5
		target   = 1001
	)
	seats := []int{0, 1, 2, 3}

	winnerTeam, trickPoints := midPlayTrickPoints(t, trickNum, seats)
	require.Positive(t, trickPoints, "the fixture's trick must actually be worth something")

	base := testfixtures.NewGameMidPlayWithScores(trickNum, 0, 0)
	seed := target - base.HandPoints[winnerTeam] - trickPoints
	require.Positive(t, seed, "fixture must leave room below the target")

	t.Run("on: the match ends on the crossing trick", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlayWithScores(trickNum, 0, 0)
		gs.TeamScores[winnerTeam] = seed
		gs = testfixtures.WithStopAtTarget(gs)
		loserTeam := 1 - winnerTeam
		wantLoser := gs.TeamScores[loserTeam] + gs.HandPoints[loserTeam]
		wantHandPoints := gs.HandPoints
		wantHandPoints[winnerTeam] += trickPoints
		wantDeclPoints := gs.DeclarationPoints
		wantBelotPoints := gs.BelotPoints

		after := stopTrick(t, gs, seats)

		require.Equal(t, game.PhaseMatchEnd, after.Phase)
		require.NotNil(t, after.WinnerTeam)
		assert.Equal(t, winnerTeam, *after.WinnerTeam)
		assert.Equal(t, target, after.TeamScores[winnerTeam],
			"the banked score is the running total that crossed")
		assert.Equal(t, wantLoser, after.TeamScores[loserTeam],
			"the other team banks its running total too")

		assert.Equal(t, wantHandPoints, after.HandPoints,
			"the hand's card points stay visible after being banked")
		assert.Equal(t, wantDeclPoints, after.DeclarationPoints)
		assert.Equal(t, wantBelotPoints, after.BelotPoints)

		assert.Nil(t, after.LastHandResult)

		assert.Equal(t, trickNum+1, after.TrickNumber,
			"resolveTrick had already advanced the counter; the stop does not rewind trick state")
		assert.Equal(t, trickNum, after.TricksWon[game.TeamA]+after.TricksWon[game.TeamB],
			"tricks %d-8 are never played", trickNum+1)

		assert.Nil(t, after.TurnExpiresAt)
		assert.Zero(t, after.TurnTimeRemaining)
		assert.False(t, after.AwaitingDeclaration)
		assert.Nil(t, after.PendingBelotSeat)
		assert.Nil(t, after.SurrenderProposerSeat)
	})

	t.Run("off: the same fixture plays on", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlayWithScores(trickNum, 0, 0)
		gs.TeamScores[winnerTeam] = seed
		require.False(t, gs.Rules.StopAtTarget, "the control must be a normal room")
		wantHand := gs.HandPoints[winnerTeam] + trickPoints

		after := stopTrick(t, gs, seats)

		assert.Equal(t, game.PhasePlaying, after.Phase, "the hand plays on")
		assert.Nil(t, after.WinnerTeam)
		assert.Equal(t, seed, after.TeamScores[winnerTeam],
			"nothing is banked mid-hand when the rule is off")
		assert.Equal(t, wantHand, after.HandPoints[winnerTeam],
			"the points stay in the hand accumulator, exactly as before this rule existed")
		assert.Equal(t, trickNum+1, after.TrickNumber)
	})
}

// TestStopAtTargetCrossingByTheTakersOpponents checks a stop never applies the failed-contract transfer, which needs a finished hand.
// The defenders cross while the taker is behind, and the taker keeps what it had banked.
func TestStopAtTargetCrossingByTheTakersOpponents(t *testing.T) {
	const (
		trickNum = 5
		target   = 1001
	)
	seats := []int{0, 1, 2, 3}

	crossingTeam, trickPoints := midPlayTrickPoints(t, trickNum, seats)

	gs := testfixtures.NewGameMidPlayWithScores(trickNum, 0, 0)
	takerTeam := game.TeamForSeat(*gs.TrumpCallerSeat)
	require.NotEqual(t, takerTeam, crossingTeam,
		"this test is only meaningful when the DEFENDERS are the ones who cross")
	require.Positive(t, gs.HandPoints[takerTeam],
		"the taker must hold points this hand, or there is nothing a transfer could move")

	gs.TeamScores[crossingTeam] = target - gs.HandPoints[crossingTeam] - trickPoints
	gs.TeamScores[takerTeam] = 300
	gs = testfixtures.WithStopAtTarget(gs)

	takerRunningTotal := gs.TeamScores[takerTeam] + gs.HandPoints[takerTeam]
	everythingInTheHand := gs.HandPoints[game.TeamA] + gs.HandPoints[game.TeamB] + trickPoints

	after := stopTrick(t, gs, seats)

	require.Equal(t, game.PhaseMatchEnd, after.Phase)
	require.NotNil(t, after.WinnerTeam)
	assert.Equal(t, crossingTeam, *after.WinnerTeam,
		"the defenders crossed, so the defenders win — the taker's contract is never judged")
	assert.Equal(t, target, after.TeamScores[crossingTeam])

	assert.Equal(t, takerRunningTotal, after.TeamScores[takerTeam],
		"the taker keeps its own banked points — a stop evaluates no failed-hand transfer")
	assert.NotEqual(t, 300, after.TeamScores[takerTeam],
		"a failed-contract transfer would have zeroed the taker's hand points")
	assert.Less(t, after.TeamScores[crossingTeam], target+everythingInTheHand,
		"and it would have handed the taker's points to the crossing team on top")

	assert.Nil(t, after.LastHandResult, "no HandScore, so no failedContract verdict exists at all")
}

// TestStopAtTargetNilsAStalePreviousHandResult checks a stop clears the previous hand's result, which startNewHand keeps.
// A stale result would emit a false hand_scored event and a duplicate hand_results row.
func TestStopAtTargetNilsAStalePreviousHandResult(t *testing.T) {
	const trickNum = 5
	seats := []int{0, 1, 2, 3}

	winnerTeam, trickPoints := midPlayTrickPoints(t, trickNum, seats)

	gs := testfixtures.WithScoredPreviousHand(
		testfixtures.NewGameMidPlayWithScores(trickNum, 0, 0), 3)
	gs.TeamScores[winnerTeam] = 1001 - gs.HandPoints[winnerTeam] - trickPoints
	gs = testfixtures.WithStopAtTarget(gs)

	require.NotNil(t, gs.LastHandResult,
		"the fixture must actually carry hand 2's result, or this test proves nothing")
	require.Equal(t, 3, gs.HandNumber)
	staleTotal := gs.LastHandResult.TeamAHandTotal

	after := stopTrick(t, gs, seats)

	require.Equal(t, game.PhaseMatchEnd, after.Phase)
	require.Nil(t, after.LastHandResult,
		"the stop must clear the PREVIOUS hand's result (teamAHandTotal %d), or the match "+
			"layer emits a false event:hand_scored and persists a duplicate hand_results row",
		staleTotal)
	assert.Equal(t, 3, after.HandNumber, "the hand number itself is untouched")
}

// TestStopAtTargetDoesNotTouchActivePlayerSeat checks the stop leaves ActivePlayerSeat alone.
// The match layer reads it as the trick winner fallback, so overwriting it would broadcast the wrong winner.
func TestStopAtTargetDoesNotTouchActivePlayerSeat(t *testing.T) {
	const trickNum = 5
	seats := []int{0, 1, 2, 3}

	winnerTeam, trickPoints := midPlayTrickPoints(t, trickNum, seats)

	control := testfixtures.NewGameMidPlayWithScores(trickNum, 0, 0)
	controlAfter := stopTrick(t, control, seats)
	wantSeat := controlAfter.ActivePlayerSeat

	gs := testfixtures.NewGameMidPlayWithScores(trickNum, 0, 0)
	gs.TeamScores[winnerTeam] = 1001 - gs.HandPoints[winnerTeam] - trickPoints
	gs = testfixtures.WithStopAtTarget(gs)

	after := stopTrick(t, gs, seats)

	require.Equal(t, game.PhaseMatchEnd, after.Phase)
	assert.Equal(t, wantSeat, after.ActivePlayerSeat,
		"the stop must leave the trick winner on the clock, so the final trick_resolved names the right seat")
}

// TestStopAtTargetAwardsNoLastTrickOrCapotBonus checks an unfinished hand earns neither bonus.
// The crossing team has taken every trick so far, the shape that would become a Capot.
func TestStopAtTargetAwardsNoLastTrickOrCapotBonus(t *testing.T) {
	const trickNum = 6
	seats := []int{0, 1, 2, 3}

	winnerTeam, trickPoints := midPlayTrickPoints(t, trickNum, seats)

	gs := testfixtures.NewGameMidPlayWithScores(trickNum, 0, 0)
	gs.TricksWon = [2]int{0, 0}
	gs.TricksWon[winnerTeam] = trickNum - 1
	seed := 1001 - gs.HandPoints[winnerTeam] - trickPoints
	gs.TeamScores[winnerTeam] = seed
	gs = testfixtures.WithStopAtTarget(gs)

	after := stopTrick(t, gs, seats)

	require.Equal(t, game.PhaseMatchEnd, after.Phase)
	assert.Equal(t, 1001, after.TeamScores[winnerTeam],
		"exactly the running total — no +10 last trick and no +100 Capot")
	assert.Nil(t, after.LastHandResult,
		"and no HandScore claiming either bonus — no synthetic Capot flag or last-trick team")
}

// TestStopAtTargetTrick8StillScoresNormally checks a completed hand still goes through scoreHand with its bonuses.
// Team B is seeded at 909, so only the applied last-trick bonus reaches 1001.
func TestStopAtTargetTrick8StillScoresNormally(t *testing.T) {
	gs := testfixtures.WithStopAtTarget(testfixtures.NewGameNearEnd(0, 909))
	require.Equal(t, 8, gs.TrickNumber, "the fixture must be at the last trick")
	require.Less(t, gs.TeamScores[game.TeamB]+gs.HandPoints[game.TeamB], 1001,
		"team B must be short of the target until the bonus lands")

	after := stopTrick(t, gs, []int{0, 1, 2, 3})

	require.Equal(t, game.PhaseMatchEnd, after.Phase)
	require.NotNil(t, after.WinnerTeam)
	assert.Equal(t, game.TeamB, *after.WinnerTeam)
	assert.Equal(t, [2]int{70, 1001}, after.TeamScores)

	require.NotNil(t, after.LastHandResult, "a completed hand always gets its HandScore")
	assert.Equal(t, 10, after.LastHandResult.LastTrickBonus,
		"the last-trick bonus is applied on trick 8 whatever the room's dosta setting")
	assert.Equal(t, game.TeamB, after.LastHandResult.LastTrickTeam)
}

// TestStopAtTargetTrick8CrossingOnCardPointsStillScoresNormally checks a pre-bonus crossing on trick 8 still runs scoreHand.
// Without the PhaseHandScoring guard the stop would skip the last-trick bonus, Capot and failed-hand checks.
func TestStopAtTargetTrick8CrossingOnCardPointsStillScoresNormally(t *testing.T) {
	gs := testfixtures.WithStopAtTarget(testfixtures.NewGameNearEnd(1001-70, 0))
	require.Equal(t, 8, gs.TrickNumber, "the fixture must be at the last trick")
	require.GreaterOrEqual(t, gs.TeamScores[game.TeamA]+gs.HandPoints[game.TeamA], 1001,
		"team A must ALREADY be at the target before any bonus — that is what makes this "+
			"test exercise the PhaseHandScoring guard instead of the target check in scoreHand")

	after := stopTrick(t, gs, []int{0, 1, 2, 3})

	require.Equal(t, game.PhaseMatchEnd, after.Phase)
	require.NotNil(t, after.WinnerTeam)
	assert.Equal(t, game.TeamA, *after.WinnerTeam, "team A crossed; the taker (team B) did not")

	require.NotNil(t, after.LastHandResult,
		"a hand that reached trick 8 must be scored by scoreHand, never short-circuited by the stop")
	assert.Equal(t, 10, after.LastHandResult.LastTrickBonus,
		"the last-trick bonus is applied on a completed hand whatever the dosta setting")
	assert.Equal(t, game.TeamB, after.LastHandResult.LastTrickTeam)
	assert.False(t, after.LastHandResult.FailedContract,
		"and the failed-hand rule was actually evaluated, not skipped")

	assert.Equal(t, [2]int{1001, 92}, after.TeamScores,
		"team B's score includes the trick and the +10 — proof scoreHand ran")
}

// TestStopAtTargetOnBeloteAnnouncement covers checkpoint 3: the +20 itself
// crosses. handleAnnounceBelot must return BEFORE finishCardPlay, so play never
// advances and the trick in progress is left exactly where it stopped.
func TestStopAtTargetOnBeloteAnnouncement(t *testing.T) {
	beloteFixture := func(t *testing.T, leaderSeat int, seat2TrumpOnly bool) *game.GameState {
		t.Helper()
		gs := testfixtures.NewGameMidPlayWithScores(2, 0, 0)
		gs.BelotAnnounced = false
		gs.ActivePlayerSeat = leaderSeat
		if seat2TrumpOnly {
			gs.Players[2].Hand = []game.Card{
				{Rank: game.RankKing, Suit: game.SuitHearts},
				{Rank: game.RankQueen, Suit: game.SuitHearts},
				{Rank: game.Rank8, Suit: game.SuitHearts},
				{Rank: game.Rank7, Suit: game.SuitHearts},
			}
		}
		return gs
	}

	promptBelot := func(t *testing.T, gs *game.GameState, before []int) *game.GameState {
		t.Helper()
		state := stopTrick(t, gs, before)
		king := game.Card{Rank: game.RankKing, Suit: game.SuitHearts}
		next, err := game.ApplyAction(state, game.Action{
			Type:       game.ActionPlayCard,
			PlayerSeat: 2,
			Card:       &king,
		})
		require.NoError(t, err)
		require.NotNil(t, next.PendingBelotSeat, "seat 2's K of trump must raise the prompt")
		require.Equal(t, 2, *next.PendingBelotSeat)
		return next
	}

	tests := []struct {
		name           string
		leaderSeat     int
		before         []int
		wantCards      int
		seat2TrumpOnly bool
	}{
		{"mid-trick", 1, []int{1}, 2, false},
		{"fourth card of the trick", 3, []int{3, 0, 1}, 4, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			control := promptBelot(t, beloteFixture(t, tc.leaderSeat, tc.seat2TrumpOnly), tc.before)
			require.Len(t, control.CurrentTrick, tc.wantCards)
			handSoFar := control.HandPoints[game.TeamA]
			seed := 1001 - handSoFar - 20

			t.Run("on: the match ends on the +20", func(t *testing.T) {
				gs := beloteFixture(t, tc.leaderSeat, tc.seat2TrumpOnly)
				gs.TeamScores[game.TeamA] = seed
				gs = testfixtures.WithStopAtTarget(gs)

				prompted := promptBelot(t, gs, tc.before)
				after, err := game.ApplyAction(prompted, game.Action{
					Type:       game.ActionAnnounceBelot,
					PlayerSeat: 2,
				})
				require.NoError(t, err)

				require.Equal(t, game.PhaseMatchEnd, after.Phase)
				require.NotNil(t, after.WinnerTeam)
				assert.Equal(t, game.TeamA, *after.WinnerTeam)
				assert.Equal(t, 1001, after.TeamScores[game.TeamA])
				assert.Equal(t, 20, after.BelotPoints[game.TeamA],
					"banked into teamScores AND left visible — scoreHand does the same at a normal end")
				assert.Nil(t, after.LastHandResult)
				assert.Nil(t, after.PendingBelotSeat)

				assert.Len(t, after.CurrentTrick, tc.wantCards,
					"the abandoned trick keeps the cards that were face up")
				assert.Equal(t, prompted.TricksWon, after.TricksWon,
					"an unresolved trick is won by nobody")
				assert.Equal(t, prompted.ActivePlayerSeat, after.ActivePlayerSeat,
					"play never advanced past the announcing seat")
			})

			t.Run("off: the same fixture plays on", func(t *testing.T) {
				gs := beloteFixture(t, tc.leaderSeat, tc.seat2TrumpOnly)
				gs.TeamScores[game.TeamA] = seed

				prompted := promptBelot(t, gs, tc.before)
				after, err := game.ApplyAction(prompted, game.Action{
					Type:       game.ActionAnnounceBelot,
					PlayerSeat: 2,
				})
				require.NoError(t, err)

				assert.NotEqual(t, game.PhaseMatchEnd, after.Phase)
				assert.Nil(t, after.WinnerTeam)
				assert.Equal(t, seed, after.TeamScores[game.TeamA])
				assert.Equal(t, 20, after.BelotPoints[game.TeamA],
					"the +20 stays in the hand accumulator")
			})
		})
	}
}

// bitolaTrick1BelotePrompt drives NewGameFirstTrick to the Belote prompt on card 2 of trick 1.
// Seat 0 has declared its melds, but the contest is still open and they are not yet scored.
func bitolaTrick1BelotePrompt(t *testing.T, gs *game.GameState) *game.GameState {
	t.Helper()
	require.Equal(t, 1, gs.TrickNumber, "the deferral only exists at trick 1")
	require.False(t, gs.DeclarationsResolved, "and only while the contest is open")

	gs.ActivePlayerSeat = 3
	lead := game.Card{Rank: game.RankTen, Suit: game.SuitDiamonds}
	next, err := game.ApplyAction(gs, game.Action{
		Type: game.ActionPlayCard, PlayerSeat: 3, Card: &lead,
	})
	require.NoError(t, err, "seat 3 leads TD")
	gs = next

	require.True(t, gs.AwaitingDeclaration, "seat 0 holds two tierces and must be asked first")
	next, err = game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
	require.NoError(t, err)
	gs = next
	require.NotEmpty(t, gs.Players[0].Declarations, "seat 0's melds are on the table, unconverted")

	king := game.Card{Rank: game.RankKing, Suit: game.SuitHearts}
	next, err = game.ApplyAction(gs, game.Action{
		Type: game.ActionPlayCard, PlayerSeat: 0, Card: &king,
	})
	require.NoError(t, err, "seat 0 cuts with the trump King")
	require.NotNil(t, next.PendingBelotSeat, "which raises the Belote prompt")
	require.Equal(t, 0, *next.PendingBelotSeat)
	require.Len(t, next.CurrentTrick, 2, "as CARD 2 of the trick")
	require.False(t, next.DeclarationsResolved, "the contest is still open at this moment")
	return next
}

// finishBitolaTrick1 plays the last two cards of trick 1 so the meld contest settles.
// Seat 1 skips its meld, so team A wins the contest.
func finishBitolaTrick1(t *testing.T, gs *game.GameState) *game.GameState {
	t.Helper()

	require.True(t, gs.AwaitingDeclaration, "seat 1 holds a quarte and is asked next")
	next, err := game.ApplyAction(gs, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: 1})
	require.NoError(t, err)
	gs = next

	follow := game.Card{Rank: game.Rank8, Suit: game.SuitDiamonds}
	next, err = game.ApplyAction(gs, game.Action{
		Type: game.ActionPlayCard, PlayerSeat: 1, Card: &follow,
	})
	require.NoError(t, err, "seat 1 follows the diamond lead")
	gs = next

	require.True(t, gs.AwaitingDeclaration, "seat 2 holds a trump tierce and is asked next")
	next, err = game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 2})
	require.NoError(t, err)
	gs = next

	overTrump := game.Card{Rank: game.RankJack, Suit: game.SuitHearts}
	next, err = game.ApplyAction(gs, game.Action{
		Type: game.ActionPlayCard, PlayerSeat: 2, Card: &overTrump,
	})
	require.NoError(t, err, "seat 2 over-trumps, completing the trick")
	return next
}

// announceBeloteAt is the announcement itself, kept separate so each test can
// assert on the state it produces before driving any further.
func announceBeloteAt(t *testing.T, gs *game.GameState, seat int) *game.GameState {
	t.Helper()
	next, err := game.ApplyAction(gs, game.Action{Type: game.ActionAnnounceBelot, PlayerSeat: seat})
	require.NoError(t, err)
	return next
}

// TestStopAtTargetDefersABeloteCrossingWhileTheTrick1ContestIsOpen checks a Belote crossing waits until trick 1 resolves the meld contest.
// Stopping on the +20 mid-trick would drop every declared meld.
func TestStopAtTargetDefersABeloteCrossingWhileTheTrick1ContestIsOpen(t *testing.T) {
	controlPrompt := bitolaTrick1BelotePrompt(t, testfixtures.NewGameFirstTrick(game.SuitHearts))
	control := finishBitolaTrick1(t, announceBeloteAt(t, controlPrompt, 0))
	require.True(t, control.DeclarationsResolved, "the control's contest must settle")
	wantDecl := control.DeclarationPoints[game.TeamA]
	wantHand := control.HandPoints[game.TeamA]
	wantBelot := control.BelotPoints[game.TeamA]
	require.Positive(t, wantDecl,
		"team A must WIN the contest, or a discarded meld would not show on its score")
	require.Equal(t, 20, wantBelot)

	const seed = 1001 - 20

	t.Run("on: the announcement does not end the match", func(t *testing.T) {
		gs := testfixtures.WithStopAtTarget(testfixtures.NewGameFirstTrick(game.SuitHearts))
		gs.TeamScores[game.TeamA] = seed

		announced := announceBeloteAt(t, bitolaTrick1BelotePrompt(t, gs), 0)

		if announced.Phase == game.PhaseMatchEnd {
			assert.Failf(t, "declared melds discarded by a trick-1 Belote stop",
				"the match ended on the +20 with %d points of declared meld never converted: "+
					"seat 0 still holds %d unconverted declaration(s), declarationPoints=%v, "+
					"banked teamScores=%v",
				wantDecl, len(announced.Players[0].Declarations),
				announced.DeclarationPoints, announced.TeamScores)
		}

		require.NotEqual(t, game.PhaseMatchEnd, announced.Phase,
			"the +20 crossed, but the trick-1 contest is still open — the stop must wait")
		assert.Equal(t, game.PhasePlaying, announced.Phase)
		assert.Equal(t, 20, announced.BelotPoints[game.TeamA], "the +20 is still banked")
		assert.Equal(t, [2]int{0, 0}, announced.DeclarationPoints,
			"and no meld has been converted yet, which is exactly why stopping here loses them")
		assert.False(t, announced.DeclarationsResolved)
		assert.Nil(t, announced.WinnerTeam)
		assert.Equal(t, seed, announced.TeamScores[game.TeamA], "nothing banked into the match score")

		after := finishBitolaTrick1(t, announced)

		require.Equal(t, game.PhaseMatchEnd, after.Phase,
			"the stop fires at the trick-1 resolution instead")
		require.NotNil(t, after.WinnerTeam)
		assert.Equal(t, game.TeamA, *after.WinnerTeam)
		assert.True(t, after.DeclarationsResolved, "the contest settled first")
		assert.Equal(t, wantDecl, after.DeclarationPoints[game.TeamA],
			"the declared melds ARE converted and included in the banked total")
		assert.Equal(t, seed+wantHand+wantDecl+wantBelot, after.TeamScores[game.TeamA],
			"the banked score is the COMPLETE running total: match score, trick points, melds, Belote")
		assert.Greater(t, after.TeamScores[game.TeamA], seed+wantBelot,
			"strictly more than an immediate stop on the +20 would have banked — the "+
				"difference is the melds and trick points a trick-1 stop discarded")
	})

	t.Run("off: the same fixture plays on past trick 1", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.TeamScores[game.TeamA] = seed

		announced := announceBeloteAt(t, bitolaTrick1BelotePrompt(t, gs), 0)
		after := finishBitolaTrick1(t, announced)

		assert.Equal(t, game.PhasePlaying, after.Phase)
		assert.Nil(t, after.WinnerTeam)
		assert.Equal(t, 2, after.TrickNumber)
		assert.Equal(t, seed, after.TeamScores[game.TeamA],
			"nothing banked mid-hand when the rule is off")
	})
}

// TestStopAtTargetDoesNotDeferOutsideAnOpenTrick1Contest checks the deferral depends only on TrickNumber and DeclarationsResolved, not the variant.
func TestStopAtTargetDoesNotDeferOutsideAnOpenTrick1Contest(t *testing.T) {
	t.Run("croatia has already resolved its contest before trick 1", func(t *testing.T) {
		croatian := testfixtures.NewGameCroatianDeclaring(game.SuitHearts)
		for _, seat := range []int{1, 2, 3, 0} {
			next, err := game.ApplyAction(croatian, game.Action{
				Type: game.ActionDeclare, PlayerSeat: seat,
			})
			require.NoError(t, err, "seat %d", seat)
			croatian = next
		}
		require.Equal(t, game.PhasePlaying, croatian.Phase)
		require.Equal(t, 1, croatian.TrickNumber)
		require.True(t, croatian.DeclarationsResolved,
			"THE reason no variant check is needed: Croatian reaches trick 1 already resolved")

		gs := testfixtures.WithStopAtTarget(testfixtures.NewGameCroatianFirstTrick(game.SuitHearts))
		gs.DeclarationsResolved = true
		gs.TeamScores[game.TeamA] = 1001 - 20
		gs.ActivePlayerSeat = 3

		lead := game.Card{Rank: game.RankTen, Suit: game.SuitDiamonds}
		led, err := game.ApplyAction(gs, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: 3, Card: &lead,
		})
		require.NoError(t, err)
		require.False(t, led.AwaitingDeclaration, "a settled contest asks nobody")

		king := game.Card{Rank: game.RankKing, Suit: game.SuitHearts}
		prompted, err := game.ApplyAction(led, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: 0, Card: &king,
		})
		require.NoError(t, err)
		require.NotNil(t, prompted.PendingBelotSeat, "Belote is live in a Croatian room")

		announced := announceBeloteAt(t, prompted, 0)

		assert.Equal(t, game.PhaseMatchEnd, announced.Phase,
			"no deferral: there is no open contest to wait for")
		require.NotNil(t, announced.WinnerTeam)
		assert.Equal(t, game.TeamA, *announced.WinnerTeam)
		assert.Equal(t, 1001, announced.TeamScores[game.TeamA])
		assert.Len(t, announced.CurrentTrick, 2, "the trick is abandoned where it stopped")
	})

	t.Run("a declarations-off room has no contest and no Belote at all", func(t *testing.T) {
		gs := testfixtures.WithStopAtTarget(
			testfixtures.WithoutDeclarations(testfixtures.NewGameFirstTrick(game.SuitHearts)),
		)
		gs.TeamScores[game.TeamA] = 1001 - 20
		require.True(t, gs.DeclarationsResolved,
			"THE reason no flag is needed: a declarations-off hand starts already resolved")

		gs.ActivePlayerSeat = 3
		lead := game.Card{Rank: game.RankTen, Suit: game.SuitDiamonds}
		led, err := game.ApplyAction(gs, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: 3, Card: &lead,
		})
		require.NoError(t, err)
		assert.False(t, led.AwaitingDeclaration, "no meld prompt in a bez-zvanja room")

		king := game.Card{Rank: game.RankKing, Suit: game.SuitHearts}
		after, err := game.ApplyAction(led, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: 0, Card: &king,
		})
		require.NoError(t, err)
		assert.Nil(t, after.PendingBelotSeat,
			"and no Belote prompt either, so the Belote checkpoint is never reached")
		assert.Equal(t, [2]int{0, 0}, after.BelotPoints)
		assert.NotEqual(t, game.PhaseMatchEnd, after.Phase, "nothing crossed: nothing was awarded")
	})
}

// TestStopAtTargetOnCroatianDeclarationContest covers checkpoint 2: the contest's
// award crosses inside the dedicated phase, so the match ends before a single
// card is played. The phase never becomes playing and trickNumber never leaves 0.
func TestStopAtTargetOnCroatianDeclarationContest(t *testing.T) {
	answers := []int{1, 2, 3, 0}

	control := testfixtures.NewGameCroatianDeclaring(game.SuitHearts)
	for _, seat := range answers {
		next, err := game.ApplyAction(control, game.Action{Type: game.ActionDeclare, PlayerSeat: seat})
		require.NoError(t, err, "seat %d", seat)
		control = next
	}
	require.Equal(t, game.PhasePlaying, control.Phase, "the control contest must close normally")
	require.Equal(t, 1, control.TrickNumber)

	winnerTeam := game.TeamA
	if control.DeclarationPoints[game.TeamB] > 0 {
		winnerTeam = game.TeamB
	}
	award := control.DeclarationPoints[winnerTeam]
	require.Positive(t, award, "the fixture layout must produce a scoring contest")

	declareAll := func(t *testing.T, gs *game.GameState) *game.GameState {
		t.Helper()
		for _, seat := range answers {
			if gs.Phase == game.PhaseMatchEnd {
				return gs
			}
			next, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: seat})
			require.NoError(t, err, "seat %d", seat)
			gs = next
		}
		return gs
	}

	t.Run("on: the match ends inside the declaring phase", func(t *testing.T) {
		gs := testfixtures.WithStopAtTarget(testfixtures.NewGameCroatianDeclaring(game.SuitHearts))
		gs.TeamScores[winnerTeam] = 1001 - award

		after := declareAll(t, gs)

		require.Equal(t, game.PhaseMatchEnd, after.Phase,
			"the phase must never become playing when the contest crosses")
		require.NotNil(t, after.WinnerTeam)
		assert.Equal(t, winnerTeam, *after.WinnerTeam)
		assert.Equal(t, 1001, after.TeamScores[winnerTeam])
		assert.Equal(t, 0, after.TrickNumber, "trick 1 never opened")
		assert.Equal(t, award, after.DeclarationPoints[winnerTeam],
			"the contest's award stays readable so the reveal can name its winner")
		assert.Nil(t, after.LastHandResult)
		assert.Nil(t, after.TurnExpiresAt)
		assert.True(t, after.DeclarationsResolved)
	})

	t.Run("off: the same fixture opens trick 1", func(t *testing.T) {
		gs := testfixtures.NewGameCroatianDeclaring(game.SuitHearts)
		gs.TeamScores[winnerTeam] = 1001 - award

		after := declareAll(t, gs)

		assert.Equal(t, game.PhasePlaying, after.Phase)
		assert.Equal(t, 1, after.TrickNumber)
		assert.Nil(t, after.WinnerTeam)
		assert.Equal(t, 1001-award, after.TeamScores[winnerTeam])
		assert.Equal(t, award, after.DeclarationPoints[winnerTeam])
	})
}

// TestStopAtTargetOnForcedDeclarationClose checks the crossing also stops when ForceCloseDeclarationPhase closes the contest.
func TestStopAtTargetOnForcedDeclarationClose(t *testing.T) {
	answered := []int{1, 2, 3}

	declareSome := func(t *testing.T, gs *game.GameState) *game.GameState {
		t.Helper()
		for _, seat := range answered {
			next, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: seat})
			require.NoError(t, err, "seat %d", seat)
			gs = next
		}
		require.Equal(t, game.PhaseDeclaring, gs.Phase, "seat 0 still owes an answer")
		return gs
	}

	control, err := game.ForceCloseDeclarationPhase(
		declareSome(t, testfixtures.NewGameCroatianDeclaring(game.SuitHearts)),
	)
	require.NoError(t, err)
	require.Equal(t, game.PhasePlaying, control.Phase)

	winnerTeam := game.TeamA
	if control.DeclarationPoints[game.TeamB] > 0 {
		winnerTeam = game.TeamB
	}
	award := control.DeclarationPoints[winnerTeam]
	require.Positive(t, award, "the forced close must still award the answered seats' melds")

	gs := testfixtures.WithStopAtTarget(testfixtures.NewGameCroatianDeclaring(game.SuitHearts))
	gs.TeamScores[winnerTeam] = 1001 - award

	after, err := game.ForceCloseDeclarationPhase(declareSome(t, gs))
	require.NoError(t, err)

	require.Equal(t, game.PhaseMatchEnd, after.Phase)
	require.NotNil(t, after.WinnerTeam)
	assert.Equal(t, winnerTeam, *after.WinnerTeam)
	assert.Equal(t, 1001, after.TeamScores[winnerTeam])
	assert.Equal(t, 0, after.TrickNumber)
	assert.Nil(t, after.LastHandResult)
}

// TestStopAtTargetOffIsAWholeMatchNoop checks that with the rule off every checkpoint is a no-op.
// Fixtures are seeded above the target so a leaked checkpoint would fire immediately.
func TestStopAtTargetOffIsAWholeMatchNoop(t *testing.T) {
	t.Run("a trick well past the target changes nothing", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlayWithScores(4, 2000, 2000)
		require.False(t, gs.Rules.StopAtTarget)

		after := stopTrick(t, gs, []int{0, 1, 2, 3})

		assert.Equal(t, game.PhasePlaying, after.Phase)
		assert.Nil(t, after.WinnerTeam)
		assert.Equal(t, [2]int{2000, 2000}, after.TeamScores,
			"nothing is banked mid-hand and no match ends")
		assert.Equal(t, 5, after.TrickNumber)
	})

	t.Run("a declaration contest well past the target changes nothing", func(t *testing.T) {
		gs := testfixtures.NewGameCroatianDeclaring(game.SuitHearts)
		gs.TeamScores = [2]int{2000, 2000}

		for _, seat := range []int{1, 2, 3, 0} {
			next, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: seat})
			require.NoError(t, err)
			gs = next
		}

		assert.Equal(t, game.PhasePlaying, gs.Phase)
		assert.Equal(t, 1, gs.TrickNumber)
		assert.Nil(t, gs.WinnerTeam)
		assert.Equal(t, [2]int{2000, 2000}, gs.TeamScores)
	})
}

// TestStopAtTargetWithBothTeamsOverTarget checks determineMatchWinner decides when both teams are over the target.
func TestStopAtTargetWithBothTeamsOverTarget(t *testing.T) {
	const trickNum = 5
	seats := []int{0, 1, 2, 3}

	gs := testfixtures.WithStopAtTarget(testfixtures.NewGameMidPlayWithScores(trickNum, 1500, 1200))
	require.NotNil(t, gs.TrumpCallerSeat, "determineMatchWinner dereferences this at every checkpoint")

	after := stopTrick(t, gs, seats)

	require.Equal(t, game.PhaseMatchEnd, after.Phase)
	require.NotNil(t, after.WinnerTeam)
	assert.Equal(t, game.TeamA, *after.WinnerTeam, "the higher score wins when both crossed")
	assert.Greater(t, after.TeamScores[game.TeamA], after.TeamScores[game.TeamB])
}

// finishBitolaTrick1ContestToTeamB is finishBitolaTrick1 with seat 1 declaring its quarte.
// Team B wins the meld contest while team A takes the trick.
func finishBitolaTrick1ContestToTeamB(t *testing.T, gs *game.GameState) *game.GameState {
	t.Helper()

	require.True(t, gs.AwaitingDeclaration, "seat 1 holds a quarte and is asked next")
	next, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 1})
	require.NoError(t, err)
	gs = next

	follow := game.Card{Rank: game.Rank8, Suit: game.SuitDiamonds}
	next, err = game.ApplyAction(gs, game.Action{
		Type: game.ActionPlayCard, PlayerSeat: 1, Card: &follow,
	})
	require.NoError(t, err, "seat 1 follows the diamond lead")
	gs = next

	require.True(t, gs.AwaitingDeclaration, "seat 2 holds a trump tierce and is asked next")
	next, err = game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 2})
	require.NoError(t, err)
	gs = next

	overTrump := game.Card{Rank: game.RankJack, Suit: game.SuitHearts}
	next, err = game.ApplyAction(gs, game.Action{
		Type: game.ActionPlayCard, PlayerSeat: 2, Card: &overTrump,
	})
	require.NoError(t, err, "seat 2 over-trumps, completing the trick")
	return next
}

// TestStopAtTargetBothTeamsCrossAtTheTrick1Resolution checks both teams crossing at the deferred trick-1 checkpoint.
// The winner is settled by determineMatchWinner on score, not by who crossed first.
func TestStopAtTargetBothTeamsCrossAtTheTrick1Resolution(t *testing.T) {
	controlPrompt := bitolaTrick1BelotePrompt(t, testfixtures.NewGameFirstTrick(game.SuitHearts))
	control := finishBitolaTrick1ContestToTeamB(t, announceBeloteAt(t, controlPrompt, 0))
	require.True(t, control.DeclarationsResolved, "the control's contest must settle")

	aGain := control.HandPoints[game.TeamA] + control.DeclarationPoints[game.TeamA] +
		control.BelotPoints[game.TeamA]
	bGain := control.HandPoints[game.TeamB] + control.DeclarationPoints[game.TeamB] +
		control.BelotPoints[game.TeamB]
	require.Positive(t, control.DeclarationPoints[game.TeamB],
		"team B must win the contest, or only one team can cross and this case is vacuous")
	require.Positive(t, control.HandPoints[game.TeamA],
		"team A must take the trick, so the two awards really do split")

	const target = 1001
	seedA := target - aGain
	seedB := target - bGain
	require.Positive(t, seedA)
	require.Positive(t, seedB)

	gs := testfixtures.WithStopAtTarget(testfixtures.NewGameFirstTrick(game.SuitHearts))
	gs.TeamScores[game.TeamA] = seedA
	gs.TeamScores[game.TeamB] = seedB

	announced := announceBeloteAt(t, bitolaTrick1BelotePrompt(t, gs), 0)
	require.NotEqual(t, game.PhaseMatchEnd, announced.Phase, "the deferral still holds")

	after := finishBitolaTrick1ContestToTeamB(t, announced)

	require.Equal(t, game.PhaseMatchEnd, after.Phase)
	require.NotNil(t, after.WinnerTeam)

	assert.GreaterOrEqual(t, after.TeamScores[game.TeamA], target,
		"team A crossed on its banked Belote")
	assert.GreaterOrEqual(t, after.TeamScores[game.TeamB], target,
		"and team B crossed on the contest, at the same checkpoint")

	wantWinner := game.TeamA
	switch {
	case seedB+bGain > seedA+aGain:
		wantWinner = game.TeamB
	case seedA+aGain == seedB+bGain:
		wantWinner = game.TeamForSeat(*after.TrumpCallerSeat)
	}
	assert.Equal(t, wantWinner, *after.WinnerTeam,
		"determineMatchWinner settles a double crossing: higher score, then the taker")
}

// TestStopAtTargetRespectsThe501Target proves the rule reads MatchTarget rather
// than a hardcoded 1001, which is the only thing that makes it work in the
// shorter match mode most tables actually play.
func TestStopAtTargetRespectsThe501Target(t *testing.T) {
	const trickNum = 5
	seats := []int{0, 1, 2, 3}

	winnerTeam, trickPoints := midPlayTrickPoints(t, trickNum, seats)

	gs := testfixtures.NewGameMidPlayWithScores(trickNum, 0, 0)
	gs.MatchMode = "501"
	seed := 501 - gs.HandPoints[winnerTeam] - trickPoints
	require.Positive(t, seed)
	gs.TeamScores[winnerTeam] = seed
	gs = testfixtures.WithStopAtTarget(gs)

	after := stopTrick(t, gs, seats)

	require.Equal(t, game.PhaseMatchEnd, after.Phase)
	require.NotNil(t, after.WinnerTeam)
	assert.Equal(t, winnerTeam, *after.WinnerTeam)
	assert.Equal(t, 501, after.TeamScores[winnerTeam])
}

// TestStopAtTargetRespectsThe701Target stops a 701 match on reaching 701, which a 1001 fallback would play past.
func TestStopAtTargetRespectsThe701Target(t *testing.T) {
	const trickNum = 5
	seats := []int{0, 1, 2, 3}

	winnerTeam, trickPoints := midPlayTrickPoints(t, trickNum, seats)

	gs := testfixtures.NewGameMidPlayWithScores(trickNum, 0, 0)
	gs.MatchMode = "701"
	seed := 701 - gs.HandPoints[winnerTeam] - trickPoints
	require.Positive(t, seed)
	gs.TeamScores[winnerTeam] = seed
	gs = testfixtures.WithStopAtTarget(gs)

	after := stopTrick(t, gs, seats)

	require.Equal(t, game.PhaseMatchEnd, after.Phase)
	require.NotNil(t, after.WinnerTeam)
	assert.Equal(t, winnerTeam, *after.WinnerTeam)
	assert.Equal(t, 701, after.TeamScores[winnerTeam])
}

// TestStopAtTargetJustShortOfTheTargetPlaysOn is the other side of the boundary:
// one point short is not a stop. Without it, an off-by-one in the comparison
// would pass every crossing test above.
func TestStopAtTargetJustShortOfTheTargetPlaysOn(t *testing.T) {
	const trickNum = 5
	seats := []int{0, 1, 2, 3}

	winnerTeam, trickPoints := midPlayTrickPoints(t, trickNum, seats)

	gs := testfixtures.NewGameMidPlayWithScores(trickNum, 0, 0)
	gs.TeamScores[winnerTeam] = 1001 - gs.HandPoints[winnerTeam] - trickPoints - 1
	gs = testfixtures.WithStopAtTarget(gs)

	after := stopTrick(t, gs, seats)

	assert.Equal(t, game.PhasePlaying, after.Phase, "1000 is not 1001")
	assert.Nil(t, after.WinnerTeam)
	assert.Equal(t, trickNum+1, after.TrickNumber)
}

// TestStopAtTargetIsCarriedOnTheResolvedConfig checks NewGame layers the room's choice over the preset and the wire flag mirrors it.
func TestStopAtTargetIsCarriedOnTheResolvedConfig(t *testing.T) {
	t.Run("both presets default to finishing the hand", func(t *testing.T) {
		for _, v := range []game.Variant{game.VariantBitola, game.VariantCroatia} {
			assert.False(t, game.RulesFor(v).StopAtTarget, "%s preset", v)
		}
	})

	tests := []struct {
		name    string
		variant game.Variant
	}{
		{"bitola", game.VariantBitola},
		{"croatia", game.VariantCroatia},
	}

	for _, tc := range tests {
		for _, want := range []bool{true, false} {
			name := tc.name + " stopping at the target"
			if !want {
				name = tc.name + " finishing the hand"
			}
			t.Run(name, func(t *testing.T) {
				gs := game.NewGame(
					[4]uint{10, 20, 30, 40},
					[4]string{"a", "b", "c", "d"},
					[4]bool{},
					tc.variant, "1001", 1, true, want, true,
				)
				assert.Equal(t, want, gs.Rules.StopAtTarget, "the config the engine reads")
				assert.Equal(t, want, gs.StopAtTarget, "and the wire flag the client renders")

				preset := game.RulesFor(tc.variant)
				assert.Equal(t, preset.DealShape, gs.Rules.DealShape)
				assert.Equal(t, preset.HasTrumpCandidate, gs.Rules.HasTrumpCandidate)
				assert.Equal(t, preset.AllPassOutcome, gs.Rules.AllPassOutcome)
				assert.Equal(t, preset.DeclarationOverlap, gs.Rules.DeclarationOverlap)
				assert.Equal(t, preset.DeclarationTiming, gs.Rules.DeclarationTiming)
				assert.Equal(t, preset.TieRule, gs.Rules.TieRule)
			})
		}
	}

	t.Run("the wire flag is refreshed from the config, never authored", func(t *testing.T) {
		gs := testfixtures.WithStopAtTarget(testfixtures.NewGameMidPlayWithScores(3, 0, 0))
		gs.StopAtTarget = false

		after := stopTrick(t, gs, []int{0})

		assert.True(t, after.StopAtTarget,
			"RefreshDerivedFlags recomputes the wire flag from Rules at ApplyAction's exit")
	})
}

// TestStopAtTargetSurvivesPerSeatProjection checks the rule passes through ProjectForSeat unchanged for all four seats.
func TestStopAtTargetSurvivesPerSeatProjection(t *testing.T) {
	for _, want := range []bool{true, false} {
		name := "stopping at the target"
		if !want {
			name = "finishing the hand"
		}
		t.Run(name, func(t *testing.T) {
			gs := testfixtures.NewGameMidPlayWithScores(3, 0, 0)
			if want {
				gs = testfixtures.WithStopAtTarget(gs)
			}
			require.Equal(t, want, gs.StopAtTarget, "fixture precondition")

			for seat := 0; seat < 4; seat++ {
				projected := game.ProjectForSeat(gs, seat)
				assert.Equal(t, want, projected.StopAtTarget,
					"seat %d must see the room's own stopAtTarget setting", seat)
			}
		})
	}
}

// TestStopAtTargetOutranksASurrenderInsideTheDeferralWindow checks a crossing deferred during trick 1 beats a surrender in that window.
// A surrender still decides the match when nobody has crossed.
func TestStopAtTargetOutranksASurrenderInsideTheDeferralWindow(t *testing.T) {
	const seed = 1001 - 20

	t.Run("a team already past the target wins despite the concession", func(t *testing.T) {
		gs := testfixtures.WithStopAtTarget(testfixtures.NewGameFirstTrick(game.SuitHearts))
		gs.TeamScores[game.TeamA] = seed
		announced := announceBeloteAt(t, bitolaTrick1BelotePrompt(t, gs), 0)
		require.NotEqual(t, game.PhaseMatchEnd, announced.Phase, "the deferral must be holding")

		requested, err := game.ApplyAction(announced, game.Action{
			Type: game.ActionSurrenderRequest, PlayerSeat: 0,
		})
		require.NoError(t, err)
		accepted, err := game.ApplyAction(requested, game.Action{
			Type: game.ActionSurrenderAccept, PlayerSeat: 2,
		})
		require.NoError(t, err)

		require.Equal(t, game.PhaseMatchEnd, accepted.Phase)
		require.NotNil(t, accepted.WinnerTeam)
		assert.Equal(t, game.TeamA, *accepted.WinnerTeam,
			"team A reached the target before the concession, so the concession cannot take it")
		assert.GreaterOrEqual(t, accepted.TeamScores[game.TeamA], 1001,
			"and the crossing total is banked")
		assert.Nil(t, accepted.SurrenderProposerSeat)
	})

	t.Run("an ordinary surrender still awards the opponents", func(t *testing.T) {
		gs := testfixtures.WithStopAtTarget(testfixtures.NewGameFirstTrick(game.SuitHearts))
		announced := announceBeloteAt(t, bitolaTrick1BelotePrompt(t, gs), 0)

		requested, err := game.ApplyAction(announced, game.Action{
			Type: game.ActionSurrenderRequest, PlayerSeat: 0,
		})
		require.NoError(t, err)
		accepted, err := game.ApplyAction(requested, game.Action{
			Type: game.ActionSurrenderAccept, PlayerSeat: 2,
		})
		require.NoError(t, err)

		require.Equal(t, game.PhaseMatchEnd, accepted.Phase)
		require.NotNil(t, accepted.WinnerTeam)
		assert.Equal(t, game.TeamB, *accepted.WinnerTeam,
			"seat 0 conceded and no team had crossed, so team B takes it as always")
	})
}
