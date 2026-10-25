package match_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/ws"
)

// eventKindsAfter decodes the hub log from a watermark into a plain list of
// event types, which is all the ordering assertions below need.
func eventKindsAfter(t *testing.T, hub *hubSpy, before int) []string {
	t.Helper()
	events := wireEvents(t, hub.snapshot()[before:])
	kinds := make([]string, 0, len(events))
	for _, e := range events {
		kinds = append(kinds, e.kind)
	}
	return kinds
}

func indexOf(kinds []string, want string) int {
	for i, k := range kinds {
		if k == want {
			return i
		}
	}
	return -1
}

// The room's stop-at-target choice survives StartMatch and lands on the resolved
// config the engine reads, not just on the wire mirror.
func TestStartMatch_CarriesStopAtTargetIntoTheSession(t *testing.T) {
	tests := []struct {
		name         string
		variant      string
		stopAtTarget bool
	}{
		{"bitola finishing the hand", "bitola", false},
		{"bitola stopping at the target", "bitola", true},
		{"croatia finishing the hand", "croatia", false},
		{"croatia stopping at the target", "croatia", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const roomID = uint(100)
			hub := &hubSpy{}
			mgr := match.NewManager(hub, newMockMatchRepo())
			require.NoError(t, mgr.StartMatch(
				roomID, tt.variant, "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0,
				true, tt.stopAtTarget, true, false,
			))
			t.Cleanup(func() { mgr.RemoveSession(roomID) })

			gs := mgr.GetStateSnapshot(roomID)
			require.NotNil(t, gs)

			assert.Equal(t, tt.stopAtTarget, gs.Rules.StopAtTarget,
				"the rule config the engine reads")
			assert.Equal(t, tt.stopAtTarget, gs.StopAtTarget,
				"and the wire flag the client renders")

			assert.True(t, gs.Rules.DeclarationsEnabled,
				"the declarations setting is independent")
			preset := game.RulesFor(game.Variant(tt.variant))
			assert.Equal(t, preset.DealShape, gs.Rules.DealShape)
			assert.Equal(t, preset.DeclarationTiming, gs.Rules.DeclarationTiming)
			assert.Equal(t, preset.TieRule, gs.Rules.TieRule)
		})
	}
}

// beloteFourthCardBase builds a mid-play Bitola trick where seat 2 cuts with the King
// of trump as the fourth card, leaving a deferred trick behind a Belote prompt.
func beloteFourthCardBase(t *testing.T, roomID uint, trickNum int) *game.GameState {
	t.Helper()

	gs := testfixtures.NewGameMidPlayWithScores(trickNum, 0, 0)
	gs.RoomID = roomID
	gs.BelotAnnounced = false
	gs.ActivePlayerSeat = 3
	gs.Players[2].Hand = []game.Card{
		{Rank: game.RankKing, Suit: game.SuitHearts},
		{Rank: game.RankQueen, Suit: game.SuitHearts},
		{Rank: game.Rank8, Suit: game.SuitHearts},
		{Rank: game.Rank7, Suit: game.SuitHearts},
	}
	if trickNum == 1 {
		gs.DeclarationsResolved = false
	}
	return gs
}

// driveToBelotePrompt plays the trick up to seat 2's King of trump, answering any
// trick-1 meld prompt with skip_declare, and returns the state holding the Belote prompt.
func driveToBelotePrompt(t *testing.T, gs *game.GameState) *game.GameState {
	t.Helper()

	answerAnyPrompt := func(gs *game.GameState) *game.GameState {
		if !gs.AwaitingDeclaration {
			return gs
		}
		next, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionSkipDeclare,
			PlayerSeat: gs.ActivePlayerSeat,
		})
		require.NoError(t, err, "skip_declare for seat %d", gs.ActivePlayerSeat)
		return next
	}

	for _, seat := range []int{3, 0, 1} {
		gs = answerAnyPrompt(gs)
		require.Equal(t, seat, gs.ActivePlayerSeat, "expected seat %d on the clock", seat)
		legal := game.LegalCards(gs, seat)
		require.NotEmpty(t, legal)
		card := legal[0]
		next, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionPlayCard,
			PlayerSeat: seat,
			Card:       &card,
		})
		require.NoError(t, err, "seat %d", seat)
		gs = next
	}
	gs = answerAnyPrompt(gs)

	king := game.Card{Rank: game.RankKing, Suit: game.SuitHearts}
	prompted, err := game.ApplyAction(gs, game.Action{
		Type:       game.ActionPlayCard,
		PlayerSeat: 2,
		Card:       &king,
	})
	require.NoError(t, err)
	require.NotNil(t, prompted.PendingBelotSeat, "seat 2's K of trump must raise the prompt")
	require.Equal(t, 2, *prompted.PendingBelotSeat)
	require.Len(t, prompted.CurrentTrick, 4, "the King must be the fourth card")
	return prompted
}

// teamHandTotal is everything a team has accumulated in the current hand.
func teamHandTotal(gs *game.GameState, team int) int {
	return gs.HandPoints[team] + gs.DeclarationPoints[team] + gs.BelotPoints[team]
}

// beloteCrossingSeed replays the trick and Belote answer with the rule off and returns
// the team A seed that lands exactly on 1001 at the last award, plus its hand total at the prompt.
func beloteCrossingSeed(t *testing.T, trickNum int, answer string) (seed, handAtPrompt int) {
	t.Helper()

	prompted := driveToBelotePrompt(t, beloteFourthCardBase(t, 0, trickNum))
	handAtPrompt = teamHandTotal(prompted, game.TeamA)

	resolved, err := game.ApplyAction(prompted, game.Action{Type: answer, PlayerSeat: 2})
	require.NoError(t, err)
	require.NotEqual(t, game.PhaseMatchEnd, resolved.Phase, "the control run must not end a match")
	require.NotEqual(t, prompted.TrickNumber, resolved.TrickNumber,
		"%s must resolve the deferred trick — that is the whole premise of these cases", answer)

	handAfter := teamHandTotal(resolved, game.TeamA)
	require.Greater(t, handAfter, handAtPrompt,
		"the trick must award team A something, or the crossing it is meant to cause is vacuous")
	return 1001 - handAfter, handAtPrompt
}

// beloteStopPrompt builds the case where the Belote +20 lands team A exactly on 1001,
// so the trick never resolves and no event:trick_resolved may be sent.
func beloteStopPrompt(t *testing.T, roomID uint, stopAtTarget bool) *game.GameState {
	t.Helper()

	gs := beloteFourthCardBase(t, roomID, 2)
	gs.TeamScores[game.TeamA] = 1001 - teamHandTotal(gs, game.TeamA) - 20
	if stopAtTarget {
		gs = testfixtures.WithStopAtTarget(gs)
	}
	return driveToBelotePrompt(t, gs)
}

// A Belote stop sends no event:trick_resolved, checked against an OFF control on the
// identical state that does send it.
func TestStopAtTarget_BeloteStopEmitsNoTrickResolved(t *testing.T) {
	t.Run("on: belot_announced then match_end, no trick_resolved", func(t *testing.T) {
		const roomID = uint(710)
		hub := &hubSpy{}
		repo := newMockMatchRepo()
		mgr := match.NewManager(hub, repo)
		require.NoError(t, mgr.StartMatch(
			roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, true, true, false,
		))
		t.Cleanup(func() { mgr.RemoveSession(roomID) })

		mgr.SetGameStateForTest(roomID, beloteStopPrompt(t, roomID, true))

		before := len(hub.snapshot())
		require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{
			Type:       game.ActionAnnounceBelot,
			PlayerSeat: 2,
		}))

		kinds := eventKindsAfter(t, hub, before)
		belotIdx := indexOf(kinds, ws.EventBelotAnnounced)
		endIdx := indexOf(kinds, ws.EventMatchEnd)
		require.GreaterOrEqual(t, belotIdx, 0, "the announcement really happened: %v", kinds)
		require.GreaterOrEqual(t, endIdx, 0, "and it ended the match: %v", kinds)
		assert.Less(t, belotIdx, endIdx, "the announcement rides ahead of the match end")
		assert.Equal(t, -1, indexOf(kinds, ws.EventTrickResolved),
			"the trick was never resolved, so no winner may be broadcast: %v", kinds)
		assert.Equal(t, -1, indexOf(kinds, ws.EventHandScored),
			"no hand was scored — the stop nils LastHandResult, which is what gates "+
				"handJustScored, so a stale hand must not be re-announced: %v", kinds)

		if stateIdx := indexOf(kinds, ws.EventMatchState); stateIdx >= 0 {
			assert.Greater(t, stateIdx, endIdx,
				"match_state must not be broadcast ahead of match_end: %v", kinds)
		}

		matches := repo.getMatches()
		require.Len(t, matches, 1, "the match must be persisted")
		assert.Equal(t, 1001, matches[0].TeamAScore)
		assert.Empty(t, repo.getHands(0),
			"the aborted hand gets no hand_results row — asserted on what was PERSISTED, "+
				"since Manager.HandResults reads a session handleMatchEnd has already removed")
		assert.False(t, mgr.HasSession(roomID), "the session is removed")
	})

	t.Run("off: the identical state emits trick_resolved and plays on", func(t *testing.T) {
		const roomID = uint(711)
		hub := &hubSpy{}
		mgr := match.NewManager(hub, newMockMatchRepo())
		require.NoError(t, mgr.StartMatch(
			roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true, false,
		))
		t.Cleanup(func() { mgr.RemoveSession(roomID) })

		mgr.SetGameStateForTest(roomID, beloteStopPrompt(t, roomID, false))

		before := len(hub.snapshot())
		require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{
			Type:       game.ActionAnnounceBelot,
			PlayerSeat: 2,
		}))

		kinds := eventKindsAfter(t, hub, before)
		assert.GreaterOrEqual(t, indexOf(kinds, ws.EventBelotAnnounced), 0)
		assert.GreaterOrEqual(t, indexOf(kinds, ws.EventTrickResolved), 0,
			"the deferred trick resolves normally when the rule is off: %v", kinds)
		assert.Equal(t, -1, indexOf(kinds, ws.EventMatchEnd), "%v", kinds)
		assert.True(t, mgr.HasSession(roomID))
	})
}

// A crossing after skip_belot or a short announce_belot still resolves the trick, so
// its event:trick_resolved and the trick-1 declarations reveal are owed.
func TestStopAtTarget_TrickCrossingAfterABeloteAnswerStillResolvesTheTrick(t *testing.T) {
	tests := []struct {
		name         string
		answer       string
		trickNum     int
		wantReveal   bool
		wantAnnounce bool
	}{
		{
			name:     "skip_belot, trick crosses",
			answer:   game.ActionSkipBelot,
			trickNum: 2,
		},
		{
			name:       "skip_belot at trick 1, trick crosses",
			answer:     game.ActionSkipBelot,
			trickNum:   1,
			wantReveal: true,
		},
		{
			name:         "announce_belot short of the target, trick crosses",
			answer:       game.ActionAnnounceBelot,
			trickNum:     2,
			wantAnnounce: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			seed, handAtPrompt := beloteCrossingSeed(t, tc.trickNum, tc.answer)

			t.Run("on: trick_resolved is still emitted, then match_end", func(t *testing.T) {
				const roomID = uint(720)
				hub := &hubSpy{}
				repo := newMockMatchRepo()
				mgr := match.NewManager(hub, repo)
				require.NoError(t, mgr.StartMatch(
					roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, true, true, false,
				))
				t.Cleanup(func() { mgr.RemoveSession(roomID) })

				gs := beloteFourthCardBase(t, roomID, tc.trickNum)
				gs.TeamScores[game.TeamA] = seed
				gs = testfixtures.WithStopAtTarget(gs)
				prompted := driveToBelotePrompt(t, gs)

				belotIfAnnounced := 0
				if tc.wantAnnounce {
					belotIfAnnounced = 20
				}
				require.Less(t, seed+handAtPrompt+belotIfAnnounced, 1001,
					"the answer itself must leave team A short, or this is the +20-crosses case instead")

				mgr.SetGameStateForTest(roomID, prompted)
				before := len(hub.snapshot())
				require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{
					Type:       tc.answer,
					PlayerSeat: 2,
				}))

				kinds := eventKindsAfter(t, hub, before)
				trickIdx := indexOf(kinds, ws.EventTrickResolved)
				endIdx := indexOf(kinds, ws.EventMatchEnd)
				require.GreaterOrEqual(t, endIdx, 0, "the trick must have ended the match: %v", kinds)
				require.GreaterOrEqual(t, trickIdx, 0,
					"the trick really resolved, so its winner must still be broadcast: %v", kinds)
				assert.Less(t, trickIdx, endIdx,
					"the trick winner rides ahead of the match end: %v", kinds)

				if tc.wantAnnounce {
					belotIdx := indexOf(kinds, ws.EventBelotAnnounced)
					require.GreaterOrEqual(t, belotIdx, 0, "%v", kinds)
					assert.Less(t, belotIdx, trickIdx,
						"the announcement precedes the trick it did not stop: %v", kinds)
				}
				if tc.wantReveal {
					revealIdx := indexOf(kinds, ws.EventDeclarationsResolved)
					require.GreaterOrEqual(t, revealIdx, 0,
						"trick 1 resolved the contest, so the reveal is owed: %v", kinds)
					assert.Less(t, revealIdx, endIdx, "%v", kinds)
				}

				if stateIdx := indexOf(kinds, ws.EventMatchState); stateIdx >= 0 {
					assert.Greater(t, stateIdx, endIdx,
						"match_state must not be broadcast ahead of match_end: %v", kinds)
				}
				assert.Equal(t, -1, indexOf(kinds, ws.EventHandScored),
					"no hand was scored: %v", kinds)

				matches := repo.getMatches()
				require.Len(t, matches, 1, "the match must be persisted")
				assert.Equal(t, 1001, matches[0].TeamAScore)
				assert.Empty(t, repo.getHands(0), "the aborted hand gets no hand_results row")
				assert.False(t, mgr.HasSession(roomID), "the session is removed")
			})

			t.Run("off: the identical state plays on", func(t *testing.T) {
				const roomID = uint(721)
				hub := &hubSpy{}
				mgr := match.NewManager(hub, newMockMatchRepo())
				require.NoError(t, mgr.StartMatch(
					roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true, false,
				))
				t.Cleanup(func() { mgr.RemoveSession(roomID) })

				gs := beloteFourthCardBase(t, roomID, tc.trickNum)
				gs.TeamScores[game.TeamA] = seed
				mgr.SetGameStateForTest(roomID, driveToBelotePrompt(t, gs))

				before := len(hub.snapshot())
				require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{
					Type:       tc.answer,
					PlayerSeat: 2,
				}))

				kinds := eventKindsAfter(t, hub, before)
				assert.GreaterOrEqual(t, indexOf(kinds, ws.EventTrickResolved), 0, "%v", kinds)
				assert.Equal(t, -1, indexOf(kinds, ws.EventMatchEnd), "%v", kinds)
				assert.True(t, mgr.HasSession(roomID))
			})
		})
	}
}

// A crossing on the Croatian fourth declaration answer emits the reveal and then
// event:match_end, with no event:match_state ahead of it.
func TestStopAtTarget_DeclareCrossingEmitsRevealThenMatchEnd(t *testing.T) {
	answers := []int{1, 2, 3, 0}
	winnerTeam, award := croatianFullContestAward(t, answers)

	seededThreeAnswers := func(t *testing.T, roomID uint, stopAtTarget bool) *game.GameState {
		t.Helper()
		gs := testfixtures.NewGameCroatianDeclaring(game.SuitHearts)
		gs.RoomID = roomID
		gs.TeamScores[winnerTeam] = 1001 - award
		if stopAtTarget {
			gs = testfixtures.WithStopAtTarget(gs)
		}
		for _, seat := range answers[:3] {
			next, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: seat})
			require.NoError(t, err, "seat %d", seat)
			gs = next
		}
		require.Equal(t, game.PhaseDeclaring, gs.Phase, "one seat must still owe an answer")
		return gs
	}

	t.Run("on: declarations_resolved then match_end, no state ahead of it", func(t *testing.T) {
		const roomID = uint(730)
		hub := &hubSpy{}
		repo := newMockMatchRepo()
		mgr := match.NewManager(hub, repo)
		require.NoError(t, mgr.StartMatch(
			roomID, "croatia", "1001", defaultPlayers(), "per-move", 60, 10, 120, 0, true, true, true, false,
		))
		t.Cleanup(func() { mgr.RemoveSession(roomID) })

		mgr.SetGameStateForTest(roomID, seededThreeAnswers(t, roomID, true))

		before := len(hub.snapshot())
		require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{
			Type:       game.ActionDeclare,
			PlayerSeat: answers[3],
		}))

		kinds := eventKindsAfter(t, hub, before)
		revealIdx := indexOf(kinds, ws.EventDeclarationsResolved)
		endIdx := indexOf(kinds, ws.EventMatchEnd)
		require.GreaterOrEqual(t, revealIdx, 0,
			"the contest really resolved, so the reveal is owed: %v", kinds)
		require.GreaterOrEqual(t, endIdx, 0, "and it ended the match: %v", kinds)
		assert.Less(t, revealIdx, endIdx, "the reveal rides ahead of the match end: %v", kinds)

		if stateIdx := indexOf(kinds, ws.EventMatchState); stateIdx >= 0 {
			assert.Greater(t, stateIdx, endIdx,
				"match_state must not be broadcast ahead of match_end: %v", kinds)
		}
		assert.Equal(t, -1, indexOf(kinds, ws.EventHandScored), "%v", kinds)

		payload := payloadOfKind(t, hub, before, ws.EventDeclarationsResolved)
		got := declarationsResolvedWinner(t, payload)
		require.NotNil(t, got,
			"a scoring contest ended this match, so its reveal must name the winning team")
		assert.Equal(t, winnerTeam, *got)

		matches := repo.getMatches()
		require.Len(t, matches, 1, "the match must be persisted")
		assert.Equal(t, 1001, teamScoreFor(matches[0], winnerTeam))
		assert.Empty(t, repo.getHands(0), "the aborted hand gets no hand_results row")
		assert.False(t, mgr.HasSession(roomID), "the session is removed, not stranded")
	})

	t.Run("off: the identical state opens trick 1", func(t *testing.T) {
		const roomID = uint(731)
		hub := &hubSpy{}
		mgr := match.NewManager(hub, newMockMatchRepo())
		require.NoError(t, mgr.StartMatch(
			roomID, "croatia", "1001", defaultPlayers(), "per-move", 60, 10, 120, 0, true, false, true, false,
		))
		t.Cleanup(func() { mgr.RemoveSession(roomID) })

		mgr.SetGameStateForTest(roomID, seededThreeAnswers(t, roomID, false))

		before := len(hub.snapshot())
		require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{
			Type:       game.ActionDeclare,
			PlayerSeat: answers[3],
		}))

		kinds := eventKindsAfter(t, hub, before)
		assert.GreaterOrEqual(t, indexOf(kinds, ws.EventDeclarationsResolved), 0, "%v", kinds)
		assert.GreaterOrEqual(t, indexOf(kinds, ws.EventMatchState), 0,
			"the playing-phase snapshot follows normally when the rule is off: %v", kinds)
		assert.Equal(t, -1, indexOf(kinds, ws.EventMatchEnd), "%v", kinds)
		assert.True(t, mgr.HasSession(roomID))
	})
}

// croatianFullContestAward replays a contest where ALL FOUR seats declare, with
// the rule off, so the test above can seed the crossing team exactly.
func croatianFullContestAward(t *testing.T, answers []int) (winnerTeam, award int) {
	t.Helper()
	gs := testfixtures.NewGameCroatianDeclaring(game.SuitHearts)
	for _, seat := range answers {
		next, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: seat})
		require.NoError(t, err, "seat %d", seat)
		gs = next
	}
	require.Equal(t, game.PhasePlaying, gs.Phase, "the control contest must close normally")

	if gs.DeclarationPoints[game.TeamB] > 0 {
		return game.TeamB, gs.DeclarationPoints[game.TeamB]
	}
	require.Positive(t, gs.DeclarationPoints[game.TeamA],
		"the fixture layout must produce a scoring contest")
	return game.TeamA, gs.DeclarationPoints[game.TeamA]
}

// payloadOfKind returns the payload of the first event of the given type recorded
// after the watermark. Fails the test when there is none.
func payloadOfKind(t *testing.T, hub *hubSpy, before int, kind string) json.RawMessage {
	t.Helper()
	for _, e := range wireEvents(t, hub.snapshot()[before:]) {
		if e.kind == kind {
			return e.payload
		}
	}
	require.FailNowf(t, "event not found", "no %s was broadcast", kind)
	return nil
}

var crossingTrickSeats = []int{0, 1, 2, 3}

// seedCrossingTrick returns the TeamScores value that makes the given team land
// on exactly 1001 when the trick at trickNum resolves, learned by replaying that
// trick with the rule off. Same replay-not-arithmetic rule as beloteCrossingSeed.
func seedCrossingTrick(t *testing.T, trickNum int) (team, seed int) {
	t.Helper()
	gs := testfixtures.NewGameMidPlayWithScores(trickNum, 0, 0)
	before := gs.HandPoints

	for _, seat := range crossingTrickSeats {
		legal := game.LegalCards(gs, seat)
		require.NotEmpty(t, legal)
		card := legal[0]
		next, err := game.ApplyAction(gs, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: seat, Card: &card,
		})
		require.NoError(t, err, "seat %d", seat)
		gs = next
	}
	require.NotEqual(t, game.PhaseMatchEnd, gs.Phase, "the control run must not end a match")

	team = game.TeamB
	if gs.HandPoints[game.TeamA] != before[game.TeamA] {
		team = game.TeamA
	}
	require.Greater(t, gs.HandPoints[team], before[team],
		"the trick must be worth something to somebody")
	return team, 1001 - gs.HandPoints[team]
}

// playCrossingTrick drives the trick through the session, one action at a time,
// and returns the hub watermark taken immediately before the LAST card — so the
// caller reads only the events the crossing action itself produced.
func playCrossingTrick(t *testing.T, mgr *match.Manager, hub *hubSpy, roomID uint) int {
	t.Helper()

	before := 0
	for i, seat := range crossingTrickSeats {
		gs := mgr.GetStateSnapshot(roomID)
		require.NotNil(t, gs)
		legal := game.LegalCards(gs, seat)
		require.NotEmpty(t, legal, "seat %d must have a legal card", seat)
		card := legal[0]
		if i == len(crossingTrickSeats)-1 {
			before = len(hub.snapshot())
		}
		require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: seat, Card: &card,
		}), "seat %d", seat)
	}
	return before
}

// From hand 2 the state still carries the previous hand's result, so a stop must
// not re-announce it as event:hand_scored or persist it as a hand_results row.
func TestStopAtTarget_StaleHandResultIsNotReAnnouncedOrPersisted(t *testing.T) {
	const trickNum = 5
	crossingTeam, seed := seedCrossingTrick(t, trickNum)

	const roomID = uint(740)
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(
		roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, true, true, false,
	))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := testfixtures.WithScoredPreviousHand(
		testfixtures.NewGameMidPlayWithScores(trickNum, 0, 0), 3)
	gs.RoomID = roomID
	gs.TeamScores[crossingTeam] = seed
	gs = testfixtures.WithStopAtTarget(gs)
	require.NotNil(t, gs.LastHandResult,
		"the fixture must carry hand 2's result, or this test proves nothing")
	mgr.SetGameStateForTest(roomID, gs)

	before := playCrossingTrick(t, mgr, hub, roomID)

	kinds := eventKindsAfter(t, hub, before)
	endIdx := indexOf(kinds, ws.EventMatchEnd)
	require.GreaterOrEqual(t, endIdx, 0, "the trick must have ended the match: %v", kinds)
	assert.Equal(t, -1, indexOf(kinds, ws.EventHandScored),
		"no hand scored here, so hand 2's result must not be re-announced: %v", kinds)

	require.Eventually(t, func() bool {
		return len(repo.getMatches()) == 1 && !mgr.HasSession(roomID)
	}, 2*time.Second, 5*time.Millisecond, "handleMatchEnd must complete")

	assert.Empty(t, repo.getHands(0),
		"and hand 2's numbers must not be persisted as a hand_results row for the aborted hand")
}

// A stop clears an armed turn timer and prompt, so a finished match never ships a
// snapshot advertising a live turn deadline.
func TestStopAtTarget_ClearsAnArmedTurnTimer(t *testing.T) {
	const (
		trickNum = 5
		timerSec = 30
		roomID   = uint(741)
	)
	crossingTeam, seed := seedCrossingTrick(t, trickNum)

	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(
		roomID, "bitola", "1001", defaultPlayers(), "per-move", timerSec, 10, 120, 0, true, true, true, false,
	))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := testfixtures.WithStopAtTarget(testfixtures.NewGameMidPlayWithScores(trickNum, 0, 0))
	gs.RoomID = roomID
	gs.TeamScores[crossingTeam] = seed
	expiry := time.Now().Add(time.Duration(timerSec) * time.Second)
	gs.TurnExpiresAt = &expiry
	gs.TimerDurationSec = timerSec
	gs.TurnTimeRemaining = 12_000
	mgr.SetGameStateForTest(roomID, gs)

	before := playCrossingTrick(t, mgr, hub, roomID)

	kinds := eventKindsAfter(t, hub, before)
	require.GreaterOrEqual(t, indexOf(kinds, ws.EventMatchEnd), 0,
		"the trick must have ended the match: %v", kinds)

	var snapshot struct {
		Phase             string  `json:"phase"`
		TurnExpiresAt     *string `json:"turnExpiresAt"`
		TurnTimeRemaining int64   `json:"turnTimeRemaining"`
		PendingBelotSeat  *int    `json:"pendingBelotSeat"`
		AwaitingDecl      bool    `json:"awaitingDeclaration"`
		SurrenderProposer *int    `json:"surrenderProposerSeat"`
	}
	require.NoError(t, json.Unmarshal(
		payloadOfKind(t, hub, before, ws.EventMatchState), &snapshot))

	assert.Equal(t, string(game.PhaseMatchEnd), snapshot.Phase)
	assert.Nil(t, snapshot.TurnExpiresAt,
		"a finished match must not advertise a live turn deadline")
	assert.Zero(t, snapshot.TurnTimeRemaining,
		"nor a paused-turn remainder for a turn that will never resume")
	assert.Nil(t, snapshot.PendingBelotSeat)
	assert.False(t, snapshot.AwaitingDecl)
	assert.Nil(t, snapshot.SurrenderProposer)
}

// croatianDeclarationAward replays the three-seats-answered force-close with the rule
// off, so the test below can seed the crossing team exactly.
func croatianDeclarationAward(t *testing.T, answered []int) (winnerTeam, award int) {
	t.Helper()
	gs := testfixtures.NewGameCroatianDeclaring(game.SuitHearts)
	for _, seat := range answered {
		next, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: seat})
		require.NoError(t, err, "seat %d", seat)
		gs = next
	}
	require.Equal(t, game.PhaseDeclaring, gs.Phase, "one seat must still owe an answer")

	closed, err := game.ForceCloseDeclarationPhase(gs)
	require.NoError(t, err)
	require.Equal(t, game.PhasePlaying, closed.Phase)

	if closed.DeclarationPoints[game.TeamB] > 0 {
		return game.TeamB, closed.DeclarationPoints[game.TeamB]
	}
	require.Positive(t, closed.DeclarationPoints[game.TeamA],
		"the forced close must award the answered seats' melds")
	return game.TeamA, closed.DeclarationPoints[game.TeamA]
}

// A declaration timeout that crosses the target runs handleMatchEnd; without it the
// table never persists, settles or releases its session and this test times out.
func TestStopAtTarget_DeclarationTimeoutRunsHandleMatchEnd(t *testing.T) {
	answered := []int{1, 2, 3}
	winnerTeam, award := croatianDeclarationAward(t, answered)

	const roomID = uint(712)
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(
		roomID, "croatia", "1001", defaultPlayers(), "per-move", 60, 10, 120, 0, true, true, true, false,
	))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := testfixtures.WithStopAtTarget(testfixtures.NewGameCroatianDeclaring(game.SuitHearts))
	gs.RoomID = roomID
	gs.TeamScores[winnerTeam] = 1001 - award
	for _, seat := range answered {
		next, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: seat})
		require.NoError(t, err, "seat %d", seat)
		gs = next
	}
	require.Equal(t, game.PhaseDeclaring, gs.Phase, "seat 0 never answers and is force-closed")
	gs.TurnExpiresAt = nil
	mgr.SetGameStateForTest(roomID, gs)
	mgr.SetDeclarationExpiresAtForTest(roomID, time.Now().Add(-time.Second))

	before := len(hub.snapshot())
	mgr.TriggerDeclarationTimeoutForTest(roomID, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		return len(repo.getMatches()) == 1 && !mgr.HasSession(roomID)
	}, 2*time.Second, 5*time.Millisecond,
		"the crossing force-close must run handleMatchEnd and release the session — "+
			"without it the table hangs forever")

	matches := repo.getMatches()
	require.Len(t, matches, 1)
	assert.Equal(t, 1001, teamScoreFor(matches[0], winnerTeam),
		"the persisted row carries the crossing total")
	assert.Empty(t, repo.getHands(0),
		"the aborted hand gets no hand_results row")

	kinds := eventKindsAfter(t, hub, before)
	revealIdx := indexOf(kinds, ws.EventDeclarationsResolved)
	endIdx := indexOf(kinds, ws.EventMatchEnd)
	require.GreaterOrEqual(t, revealIdx, 0, "%v", kinds)
	require.GreaterOrEqual(t, endIdx, 0, "%v", kinds)
	assert.Less(t, revealIdx, endIdx, "the reveal precedes the match end: %v", kinds)
}

// teamScoreFor reads a persisted match row's final score for the given team,
// keeping the assertions above readable when the winner is resolved at runtime.
func teamScoreFor(m *match.Match, team int) int {
	if team == game.TeamA {
		return m.TeamAScore
	}
	return m.TeamBScore
}

// The match-end payload names the stop as its outcome reason, read from the engine's
// StoppedAtTarget flag rather than inferred.
func TestStopAtTarget_MatchEndPayloadNamesTheStop(t *testing.T) {
	const roomID = uint(760)
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(
		roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, true, true, false,
	))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	mgr.SetGameStateForTest(roomID, beloteStopPrompt(t, roomID, true))

	before := len(hub.snapshot())
	require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{
		Type:       game.ActionAnnounceBelot,
		PlayerSeat: 2,
	}))

	var reason string
	for _, e := range wireEvents(t, hub.snapshot()[before:]) {
		if e.kind != ws.EventMatchEnd {
			continue
		}
		var p struct {
			OutcomeReason string `json:"outcomeReason"`
		}
		require.NoError(t, json.Unmarshal(e.payload, &p))
		reason = p.OutcomeReason
	}
	assert.Equal(t, string(ws.OutcomeReasonTargetReached), reason,
		"a stop must not be reported as a natural finish")
}
