package game_test

import (
	"testing"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Rooms configured to STOP AT THE TARGET ("dosta" — enough): the match ends the
// instant a team's running total reaches the 1001/501 target, hand unfinished.
// Everything here goes through ApplyAction, and every state is built from a
// testfixtures factory composed with WithStopAtTarget — never a raw GameState
// literal.
//
// The feature is one cross-cutting gate rather than a new phase, so its coverage
// lives in this one file. Each crossing case is paired with an OFF control on the
// SAME fixture, proving the fixture plays on when the room is configured the way
// every existing room is: that pairing is what makes "OFF is byte-identical to
// before" a tested claim rather than an assertion in a comment.

// stopTrick plays one full trick out of a mid-play fixture, seat by seat through
// ApplyAction, and returns the state after the last card. It stops early — and
// returns what it has — the moment the match ends, which is the whole point: a
// crossing trick must not deal the seats that follow.
//
// The card each seat plays is chosen from the engine's own legal set (the first
// legal card), so the walk can never diverge from the rules under test.
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

// midPlayTrickPoints replays a trick with the rule OFF to learn what the trick
// is worth and who takes it, so the ON assertions below can be exact without
// hard-coding a card-point sum that would drift the day a fixture hand changes.
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
	// Guarded HERE rather than at each call site: a pointless trick would return
	// (TeamB, 0), and every caller then seeds a team already ON the target, so the
	// crossing tests would pass without the trick contributing anything at all.
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

	// Seed the crossing team so the trick lands it EXACTLY on the target: the
	// rule is ">= target", and exact equality is the boundary that a > would get
	// wrong.
	base := testfixtures.NewGameMidPlayWithScores(trickNum, 0, 0)
	seed := target - base.HandPoints[winnerTeam] - trickPoints
	require.Positive(t, seed, "fixture must leave room below the target")

	t.Run("on: the match ends on the crossing trick", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlayWithScores(trickNum, 0, 0)
		gs.TeamScores[winnerTeam] = seed
		gs = testfixtures.WithStopAtTarget(gs)
		loserTeam := 1 - winnerTeam
		wantLoser := gs.TeamScores[loserTeam] + gs.HandPoints[loserTeam]
		// What the accumulators must still read AFTER the stop: the crossing trick
		// added its points to the winner's hand total, and nothing clears them.
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

		// The three accumulators are LEFT POPULATED — the same state shape scoreHand
		// produces at a normal match end. Zeroing them (an earlier draft of this
		// rule) blinded broadcastDeclarationsResolvedIfTransition, which derives the
		// declaration reveal's winnerTeam from DeclarationPoints[team] > 0.
		assert.Equal(t, wantHandPoints, after.HandPoints,
			"the hand's card points stay visible after being banked")
		assert.Equal(t, wantDeclPoints, after.DeclarationPoints)
		assert.Equal(t, wantBelotPoints, after.BelotPoints)

		// No hand ever completed, so no HandScore is fabricated. This nil is what
		// suppresses the match layer's event:hand_scored and its hand_results row.
		assert.Nil(t, after.LastHandResult)

		// The trick that crossed resolved normally and advanced the counter — the
		// stop deliberately leaves trick state alone — but nothing after it is ever
		// dealt: only trickNum tricks were ever won.
		assert.Equal(t, trickNum+1, after.TrickNumber,
			"resolveTrick had already advanced the counter; the stop does not rewind trick state")
		assert.Equal(t, trickNum, after.TricksWon[game.TeamA]+after.TricksWon[game.TeamB],
			"tricks %d-8 are never played", trickNum+1)

		// Timer and prompt fields are cleared so the final snapshot carries no
		// live turn or pending question.
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

// TestStopAtTargetCrossingByTheTakersOpponents is what the "no failed-hand
// transfer" promise is actually about, and nothing else pins it.
//
// At a NORMAL hand end scoreHand applies the failed-contract rule: if the taker
// does not score STRICTLY MORE than the defenders, the taker gets nothing and
// every point in the hand goes to the opponents. That test needs a FINISHED hand
// — 162 card points plus bonuses all accounted for — so a stop must not evaluate
// it. The defenders simply win, and the taker keeps whatever it had banked.
//
// The fixture's taker is seat 1 (team B) and the crossing trick is taken by team
// A, so this is exactly the shape: the defenders cross while the taker is behind,
// which is precisely when a leaked failed-hand rule would move points.
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
	// A modest, non-zero bank for the taker: enough that losing it to a transfer
	// would be visible, nowhere near the target.
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

	// The whole point: the taker keeps its OWN running total. Nothing moved.
	assert.Equal(t, takerRunningTotal, after.TeamScores[takerTeam],
		"the taker keeps its own banked points — a stop evaluates no failed-hand transfer")
	// The two shapes a leaked failed-hand rule would produce, named explicitly so
	// this fails loudly rather than by an off-by-something: the taker stripped back
	// to its pre-hand bank, and the crossing team handed the taker's points on top.
	assert.NotEqual(t, 300, after.TeamScores[takerTeam],
		"a failed-contract transfer would have zeroed the taker's hand points")
	assert.Less(t, after.TeamScores[crossingTeam], target+everythingInTheHand,
		"and it would have handed the taker's points to the crossing team on top")

	assert.Nil(t, after.LastHandResult, "no HandScore, so no failedContract verdict exists at all")
}

// TestStopAtTargetNilsAStalePreviousHandResult is what makes the "LastHandResult
// = nil is load-bearing" line testable at all.
//
// Every other fixture in this file starts on hand 1 with a nil result, so every
// assert.Nil on it passes on the fixture's zero value — delete the line from the
// engine and nothing fails. WithScoredPreviousHand supplies the shape a real
// match has from hand 2 onwards: startNewHand deliberately never clears the
// previous hand's HandScore, because it has to survive into the state the session
// manager broadcasts.
//
// Why it matters downstream: handJustScored and bufferHandResultIfScored both
// gate on "LastHandResult != nil AND a transition into match_end", which a stop
// satisfies. A stale result therefore emits a false event:hand_scored and writes
// the PREVIOUS hand's numbers into the final hand_results row — a duplicate row
// for a hand that already has one. The session-layer half of this is asserted in
// stop_at_target_session_test.go.
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

// TestStopAtTargetDoesNotTouchActivePlayerSeat pins the one field the stop must
// leave alone. trickResolvedWinnerSeat (match layer) reads ActivePlayerSeat as
// its trick-1..7 winner fallback, so overwriting it here would broadcast the
// wrong winner for the final trick — a bug invisible to every other assertion.
func TestStopAtTargetDoesNotTouchActivePlayerSeat(t *testing.T) {
	const trickNum = 5
	seats := []int{0, 1, 2, 3}

	winnerTeam, trickPoints := midPlayTrickPoints(t, trickNum, seats)

	// The OFF control tells us which seat resolveTrick leaves on the clock: the
	// trick winner, who would lead the next trick.
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

// TestStopAtTargetAwardsNoLastTrickOrCapotBonus is the arithmetic promise: the
// hand never completed, so neither bonus was earned. The fixture is rigged so
// the crossing team has taken EVERY trick so far — the shape that would become a
// Capot if the hand were allowed to finish.
func TestStopAtTargetAwardsNoLastTrickOrCapotBonus(t *testing.T) {
	const trickNum = 6
	seats := []int{0, 1, 2, 3}

	winnerTeam, trickPoints := midPlayTrickPoints(t, trickNum, seats)

	gs := testfixtures.NewGameMidPlayWithScores(trickNum, 0, 0)
	// Every trick so far to the crossing team.
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

// TestStopAtTargetTrick8StillScoresNormally is the deliberate NON-checkpoint.
// A completed hand always goes through scoreHand with its bonuses, its failed-
// hand rule and its own target check, in both settings — stopAtTarget can only
// shorten a match, never change how a finished hand scores.
//
// NewGameNearEnd's trick 8 gives team B 21 card points plus the +10 last-trick
// bonus on top of its 61, for 92; team A keeps its 70. Seeding team B at 909
// makes the FINAL total 1001 while its mid-hand running total (909 + 61 = 970)
// is still short — so only a scoreHand that actually applied the bonus can end
// this match.
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

	// The normal path ran: a HandScore exists and it carries the bonus.
	require.NotNil(t, after.LastHandResult, "a completed hand always gets its HandScore")
	assert.Equal(t, 10, after.LastHandResult.LastTrickBonus,
		"the last-trick bonus is applied on trick 8 whatever the room's dosta setting")
	assert.Equal(t, game.TeamB, after.LastHandResult.LastTrickTeam)
}

// TestStopAtTargetTrick8CrossingOnCardPointsStillScoresNormally is what the
// PhaseHandScoring half of the stop's no-op set actually protects, and nothing
// else pins it.
//
// Its sibling above (TestStopAtTargetTrick8StillScoresNormally) cannot: that
// fixture sits at 991 when the checkpoint runs, so it is BELOW the target and the
// guard is never consulted — delete the guard and that test still passes. Here
// team A is seeded so its PRE-BONUS running total is already 1001 at the moment
// resolveTrick sets PhaseHandScoring. Without the guard the stop would fire right
// there and bypass scoreHand entirely: no last-trick +10, no Capot check, no
// failed-hand evaluation, on a hand that genuinely COMPLETED.
//
// NewGameNearEnd's trick 8 gives team B 21 card points and the +10 last trick on
// top of its 61, for 92; team A keeps its 70. So the observable difference is
// team B's final score (92 with scoreHand, 61 without) and the presence of a
// HandScore at all.
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

	// The completed hand went through scoreHand, so it has a HandScore and its
	// bonus was applied.
	require.NotNil(t, after.LastHandResult,
		"a hand that reached trick 8 must be scored by scoreHand, never short-circuited by the stop")
	assert.Equal(t, 10, after.LastHandResult.LastTrickBonus,
		"the last-trick bonus is applied on a completed hand whatever the dosta setting")
	assert.Equal(t, game.TeamB, after.LastHandResult.LastTrickTeam)
	assert.False(t, after.LastHandResult.FailedContract,
		"and the failed-hand rule was actually evaluated, not skipped")

	// The bonus is IN the final score: 61 + 21 + 10. A stop at the checkpoint would
	// have banked team B on 61 and left it there.
	assert.Equal(t, [2]int{1001, 92}, after.TeamScores,
		"team B's score includes the trick and the +10 — proof scoreHand ran")
}

// TestStopAtTargetOnBeloteAnnouncement covers checkpoint 3: the +20 itself
// crosses. handleAnnounceBelot must return BEFORE finishCardPlay, so play never
// advances and the trick in progress is left exactly where it stopped.
func TestStopAtTargetOnBeloteAnnouncement(t *testing.T) {
	// beloteFixture parks a mid-play hand one card away from a Belote prompt:
	// seat 2 holds K+Q of trump hearts and the Belote has not been announced yet.
	// leaderSeat leads a trump the seat cannot beat, so its only legal cards are
	// trumps and the first of them is the King — the card that raises the prompt.
	//
	// seat2TrumpOnly trims seat 2 to its four trumps, the standard way these
	// suites pin a hand shape. It is what forces the King to be the FOURTH card:
	// with its diamonds still in hand seat 2 would be obliged to follow the
	// diamond lead instead of cutting.
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

	// promptBelot walks seats up to (but not including) seat 2, then plays seat
	// 2's King of trump, returning the state holding the Belote prompt.
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
		// Seat 1 leads a spade, seat 2 is void and cuts with the trump King: the
		// King is the SECOND card of the trick.
		{"mid-trick", 1, []int{1}, 2, false},
		// Seat 3 leads a diamond, seats 0 and 1 follow it, and a trump-only seat 2
		// cuts with the King as the FOURTH card — the deferred-trick shape, where
		// the engine never resolves the trick at all.
		{"fourth card of the trick", 3, []int{3, 0, 1}, 4, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Learn the state at the prompt from a control run, so the seed below
			// is exact.
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

				// finishCardPlay never ran: the trick was neither advanced nor
				// resolved, and the cards are still on the table where play stopped.
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

// --- The one deferral: a Bitola trick-1 Belote must not discard melds ---

// bitolaTrick1BelotePrompt drives NewGameFirstTrick to the Belote prompt raised
// as CARD 2 of trick 1, with the meld contest still open.
//
// Seat 3 leads a diamond; seat 0 is void in diamonds and holds trump K+Q, so it
// must cut and the King is legal (no trump on the table yet to over-trump). Seat
// 0 declares its melds first, because at trick 1 checkDeclarationPrompt puts a
// seat holding melds on the clock before it may play — which is exactly the state
// the deferral is about: declared, but not yet converted to points.
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

// finishBitolaTrick1 plays the remaining two cards, so trick 1 resolves and the
// meld contest settles. Seat 1 skips its meld so team A wins the contest, which
// is what makes the discarded-meld loss visible on the Belote team's own score.
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

// TestStopAtTargetDefersABeloteCrossingWhileTheTrick1ContestIsOpen is the one
// deferral in the whole rule, and the defect it fixes is a silent points loss.
//
// Under Bitola timing seats declare DURING trick 1 and the contest is resolved
// only when the trick completes. DeclarationPoints has exactly one writer —
// resolveDeclarationsForHand — so stopping on the +20 mid-trick-1 banks a running
// total with every declared meld still worth nothing: a player who declared a
// quarte simply loses it. The fixture is seeded so the +20 ALONE crosses, which
// is precisely the case that used to end the match early.
func TestStopAtTargetDefersABeloteCrossingWhileTheTrick1ContestIsOpen(t *testing.T) {
	// Control run with the rule off: learn what trick 1 and the contest are
	// actually worth, from the engine rather than from hand-copied meld values.
	controlPrompt := bitolaTrick1BelotePrompt(t, testfixtures.NewGameFirstTrick(game.SuitHearts))
	control := finishBitolaTrick1(t, announceBeloteAt(t, controlPrompt, 0))
	require.True(t, control.DeclarationsResolved, "the control's contest must settle")
	wantDecl := control.DeclarationPoints[game.TeamA]
	wantHand := control.HandPoints[game.TeamA]
	wantBelot := control.BelotPoints[game.TeamA]
	require.Positive(t, wantDecl,
		"team A must WIN the contest, or a discarded meld would not show on its score")
	require.Equal(t, 20, wantBelot)

	// Seeded so the +20 on its own reaches exactly 1001: without the deferral the
	// match ends on the announcement, with wantDecl points of declared meld lost.
	const seed = 1001 - 20

	t.Run("on: the announcement does not end the match", func(t *testing.T) {
		gs := testfixtures.WithStopAtTarget(testfixtures.NewGameFirstTrick(game.SuitHearts))
		gs.TeamScores[game.TeamA] = seed

		announced := announceBeloteAt(t, bitolaTrick1BelotePrompt(t, gs), 0)

		// THE defect, stated directly so a regression reports the LOSS rather than
		// just a premature phase: if the match ended on this action, it ended with
		// seat 0's declared melds still sitting on the player, never converted to
		// points. assert (not require) so this message survives alongside the phase
		// failure below.
		if announced.Phase == game.PhaseMatchEnd {
			assert.Failf(t, "declared melds discarded by a trick-1 Belote stop",
				"the match ended on the +20 with %d points of declared meld never converted: "+
					"seat 0 still holds %d unconverted declaration(s), declarationPoints=%v, "+
					"banked teamScores=%v",
				wantDecl, len(announced.Players[0].Declarations),
				announced.DeclarationPoints, announced.TeamScores)
		}

		// THE deferral. Everything below it is what the deferral buys.
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

// TestStopAtTargetDoesNotDeferOutsideAnOpenTrick1Contest is the other half of the
// rule: the deferral condition is TrickNumber == 1 && !DeclarationsResolved and
// needs NO variant comparison (D-VAR-1) and no new state field. These two rooms
// are the ones a variant check would have been written for, and both fail the
// condition on their own state.
func TestStopAtTargetDoesNotDeferOutsideAnOpenTrick1Contest(t *testing.T) {
	t.Run("croatia has already resolved its contest before trick 1", func(t *testing.T) {
		// First, the fact the condition leans on, proven through the real phase:
		// closing the Croatian dedicated phase opens trick 1 with the contest
		// already settled, so !DeclarationsResolved is false there by construction.
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

		// And a Croatian trick-1 Belote therefore stops immediately. The fixture is
		// the Croatian trick-1 layout with the contest marked settled, which is what
		// the dedicated phase above produces.
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
		// The second reason no flag is needed: "bez zvanja" seeds
		// DeclarationsResolved true, so the condition is false. And the deferral is
		// unreachable anyway, because the same setting switches Belote off — which is
		// the stronger statement, so both halves are asserted.
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

	// Control run: learn which team wins the contest and for how much, from the
	// engine itself rather than a hand-copied meld total.
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
		// LOAD-BEARING, and the reason the zeroing was removed:
		// broadcastDeclarationsResolvedIfTransition reads the reveal's winnerTeam
		// off DeclarationPoints[team] > 0, so an emptied array ships a reveal with
		// winnerTeam null and nothing for the panel to anchor to.
		assert.Equal(t, award, after.DeclarationPoints[winnerTeam],
			"the contest's award stays readable so the reveal can name its winner")
		assert.Nil(t, after.LastHandResult)
		assert.Nil(t, after.TurnExpiresAt)
		// The contest still RESOLVED — the match layer's reveal latch reads this
		// flag, and the reveal must still fire before the match ends.
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

// TestStopAtTargetOnForcedDeclarationClose is the same crossing reached through
// ForceCloseDeclarationPhase — the session manager's window fallback — rather
// than through a fourth answer. Both doors go through closeDeclarationPhase,
// which is why one checkpoint covers them, and this test is what proves it.
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

	// Control run: three seats declare, the window closes over seat 0.
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

// TestStopAtTargetOffIsAWholeMatchNoop is the OFF guarantee stated at the level
// the acceptance criteria state it: with the rule off, every checkpoint is a
// no-op and no state field differs from a room that never heard of it. The
// fixtures here are seeded ABOVE the target on purpose — the one configuration
// in which a leaked checkpoint would fire immediately.
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

// TestStopAtTargetWithBothTeamsOverTarget is the defensive row. It is
// unreachable in real play — only one team gains points per checkpoint, so only
// one can cross at a time — but determineMatchWinner is handed the both-over
// case and must decide rather than panic.
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

// finishBitolaTrick1ContestToTeamB is finishBitolaTrick1 with one change: seat 1
// DECLARES its quarte instead of skipping, so team B wins the meld contest while
// team A still takes the trick. That split is what makes a double crossing
// reachable — the two awards landing in one checkpoint go to opposite teams.
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

// TestStopAtTargetBothTeamsCrossAtTheTrick1Resolution is the case the trick-1
// deferral newly made REACHABLE, and it is the reason the "only one team can gain
// per checkpoint" reasoning no longer holds.
//
// Away from trick 1 that reasoning is sound: a trick resolution pays one team, a
// contest pays one team, a Belote pays one team, so only one total can move and
// only one can cross. The deferral breaks it by design — it banks a Belote +20
// and then waits for the trick-1 resolution, where the trick's card points AND
// the contest award both land in the same call and can go to OPPOSITE teams. So
// the announcing team can cross on its +20 while the other crosses on the
// contest, and both are first observed at one checkpoint.
//
// The winner is then settled the way every other match end settles a double
// crossing: determineMatchWinner, higher score first. Deliberately not "whoever
// crossed first chronologically", which would need the crossing recorded on the
// state and would make a dosta stop resolve this differently from a normal hand
// end.
func TestStopAtTargetBothTeamsCrossAtTheTrick1Resolution(t *testing.T) {
	// Control with the rule off: learn both teams' real trick-1 outcome from the
	// engine, including which side the split contest actually pays.
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

	// Seed BOTH teams to land exactly on the target at the trick-1 resolution.
	// Team A's +20 alone would already cross, which is what the deferral holds
	// back; team B only crosses once the contest settles.
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

	// The reachability claim itself: both totals are at or past the target, first
	// observed at this single checkpoint.
	assert.GreaterOrEqual(t, after.TeamScores[game.TeamA], target,
		"team A crossed on its banked Belote")
	assert.GreaterOrEqual(t, after.TeamScores[game.TeamB], target,
		"and team B crossed on the contest, at the same checkpoint")

	// And the tiebreaker actually applied, derived from the control rather than
	// assumed: higher score wins, taker only on an exact tie.
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

// TestStopAtTargetRespectsThe501Target proves the rule reads matchTarget rather
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

// TestStopAtTargetIsCarriedOnTheResolvedConfig pins the plumbing D-VAR-1 asks
// for: NewGame layers the room's choice over the variant preset, both presets
// ship it false, and the wire mirror can never drift from the config the engine
// reads because RefreshDerivedFlags owns it.
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
			// The boolean belongs in the name: without it the two iterations collide
			// as "bitola" / "bitola#01" and -run cannot address either.
			name := tc.name + " stopping at the target"
			if !want {
				name = tc.name + " finishing the hand"
			}
			t.Run(name, func(t *testing.T) {
				gs := game.NewGame(
					[4]uint{10, 20, 30, 40},
					[4]string{"a", "b", "c", "d"},
					[4]bool{},
					tc.variant, "1001", 1, true, want,
				)
				assert.Equal(t, want, gs.Rules.StopAtTarget, "the config the engine reads")
				assert.Equal(t, want, gs.StopAtTarget, "and the wire flag the client renders")

				// Every other rule stays the variant's own — a room-level override
				// must not disturb the preset it is layered over.
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
		// Force the mirror out of step, the way a handler that assigned it by hand
		// would, and let the engine's single exit put it back.
		gs.StopAtTarget = false

		after := stopTrick(t, gs, []int{0})

		assert.True(t, after.StopAtTarget,
			"RefreshDerivedFlags recomputes the wire flag from Rules at ApplyAction's exit")
	})
}

// The per-seat snapshot is what a RECONNECTING player is sent, and what every
// match_state frame is built from, so the room's rule has to survive projection
// for all four seats. ProjectForSeat masks BY ENUMERATION (Story 12.10), which
// means a newly added json-tagged field passes through by default — this pins
// that stopAtTarget is deliberately in the pass-through class (public room
// configuration, identical for every seat, revealing nothing about anyone's
// cards) rather than accidentally unmasked.
func TestStopAtTargetSurvivesPerSeatProjection(t *testing.T) {
	for _, want := range []bool{true, false} {
		// The boolean goes in the subtest name for the same reason as in
		// TestStopAtTargetIsCarriedOnTheResolvedConfig: two unnamed iterations
		// collide and -run cannot address either.
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

// TestStopAtTargetOutranksASurrenderInsideTheDeferralWindow closes the one hole
// the trick-1 deferral opens.
//
// While the Belote checkpoint is deferred the match is in a state the rule says
// should not exist: a team is past the target but the match has not ended yet,
// for the two or three cards it takes trick 1 to finish. A surrender landing in
// that window used to hand the match to the OTHER team.
//
// The concession still decides a match where nobody has crossed — that control is
// the second half of this test, and without it the fix would look right while
// having quietly broken ordinary surrenders.
func TestStopAtTargetOutranksASurrenderInsideTheDeferralWindow(t *testing.T) {
	// Team A is one Belote short of the target, and the +20 defers because the
	// trick-1 contest is still open.
	const seed = 1001 - 20

	t.Run("a team already past the target wins despite the concession", func(t *testing.T) {
		gs := testfixtures.WithStopAtTarget(testfixtures.NewGameFirstTrick(game.SuitHearts))
		gs.TeamScores[game.TeamA] = seed
		announced := announceBeloteAt(t, bitolaTrick1BelotePrompt(t, gs), 0)
		require.NotEqual(t, game.PhaseMatchEnd, announced.Phase, "the deferral must be holding")

		// Seat 0 (team A) concedes — the team that has actually crossed.
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
		// Nobody near the target.
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
