package bot_test

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/bot"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
)

// TestSimulation_HeuristicBeatsRandomBaseline checks that bot.Decide on team A wins at least 60% of points
// against a random-legal team B over 200 hands, with no ApplyAction errors.
func TestSimulation_HeuristicBeatsRandomBaseline(t *testing.T) {
	const handsTarget = 200
	rng := rand.New(rand.NewPCG(42, 1))

	heuristicPoints := 0
	totalPoints := 0
	handsPlayed := 0

	for handsPlayed < handsTarget {
		gs := game.NewGame(
			[4]uint{0, 0, 0, 0},
			[4]string{"", "", "", ""},
			[4]bool{true, true, true, true},
			game.VariantBitola, "1001", 1, true, false, true,
		)
		gs.DealerSeat = 0
		gs.ActivePlayerSeat = (gs.DealerSeat + 1) % 4
		mem := bot.NewMemory()

		result, ok := playOneHand(t, gs, mem, rng)
		if !ok {
			continue
		}
		heuristicPoints += result.TeamAHandTotal
		totalPoints += result.TeamAHandTotal + result.TeamBHandTotal
		handsPlayed++
	}

	require.Positive(t, totalPoints)
	share := float64(heuristicPoints) / float64(totalPoints)
	t.Logf("heuristic share over %d hands: %.1f%% (%d/%d points)",
		handsPlayed, share*100, heuristicPoints, totalPoints)
	assert.GreaterOrEqual(t, share, 0.60,
		"heuristic must take at least 60%% of all points vs the random baseline")
}

// simDriver supplies the acting seat and its action for one simulation step.
type simDriver func(gs *game.GameState, mem *bot.Memory, rng *rand.Rand) (int, game.Action)

// simObserver is called with the PRE-action state on every step, so a run can
// assert phase invariants and action legality at the exact moment a decision is
// made. nil for runs that only care about the outcome.
type simObserver func(t *testing.T, gs *game.GameState, seat int, action game.Action)

// playOneHand drives a fresh deal to its scored end with the default driver.
// It returns ok=false when the deal ended the match without scoring a hand.
func playOneHand(t *testing.T, gs *game.GameState, mem *bot.Memory, rng *rand.Rand) (game.HandScore, bool) {
	t.Helper()
	return playOneHandWith(t, gs, mem, rng, nextSimAction, nil)
}

// playOneHandWith is playOneHand with an injected driver and optional per-step observer.
func playOneHandWith(
	t *testing.T,
	gs *game.GameState,
	mem *bot.Memory,
	rng *rand.Rand,
	drive simDriver,
	observe simObserver,
) (game.HandScore, bool) {
	t.Helper()

	for steps := 0; steps < 500; steps++ {
		switch gs.Phase {
		case game.PhaseDealing:
			gs.Phase = game.PhaseBidding

		case game.PhaseBidding, game.PhaseDeclaring, game.PhasePlaying:
			seat, action := drive(gs, mem, rng)
			if observe != nil {
				observe(t, gs, seat, action)
			}
			oldLead := gs.LeadSuit
			next, err := game.ApplyAction(gs, action)
			require.NoError(t, err,
				"ApplyAction must never reject a simulated action (seat %d, %s)", seat, action.Type)
			if action.Type == game.ActionPlayCard {
				mem.ObservePlay(seat, *action.Card, oldLead)
			}
			gs = next
			if gs.DeclarationsResolved {
				mem.ObserveDeclarations(gs.Players)
			}

		case game.PhaseHandComplete:
			require.NotNil(t, gs.LastHandResult)
			return *gs.LastHandResult, true

		case game.PhaseMatchEnd:
			if gs.LastHandResult == nil {
				return game.HandScore{}, false
			}
			return *gs.LastHandResult, true

		default:
			t.Fatalf("simulation reached unexpected phase %q", gs.Phase)
		}
	}
	t.Fatal("simulation hand did not terminate within the step bound")
	return game.HandScore{}, false
}

// nextSimAction picks the acting seat and its action: team A (seats 0/2) via
// bot.Decide over a redacted view, team B (seats 1/3) via random-legal moves.
func nextSimAction(gs *game.GameState, mem *bot.Memory, rng *rand.Rand) (int, game.Action) {
	seat := gs.ActivePlayerSeat
	if gs.Phase == game.PhasePlaying && gs.PendingBelotSeat != nil {
		seat = *gs.PendingBelotSeat
	}
	if seat%2 == 0 {
		return seat, bot.Decide(viewFromState(gs, seat, mem))
	}
	return seat, randomLegalAction(gs, seat, rng)
}

// randomLegalAction picks uniformly among the legal options at the seat's
// current decision point.
func randomLegalAction(gs *game.GameState, seat int, rng *rand.Rand) game.Action {
	if gs.Phase == game.PhaseBidding {
		if gs.BiddingRound == 1 {
			if rng.IntN(2) == 0 {
				return game.Action{Type: game.ActionPickTrump, PlayerSeat: seat}
			}
			return game.Action{Type: game.ActionPassTrump, PlayerSeat: seat}
		}
		options := make([]game.Suit, 0, 3)
		for _, s := range game.AllSuits {
			if gs.TrumpCandidate != nil && s == gs.TrumpCandidate.Suit {
				continue
			}
			options = append(options, s)
		}
		pick := rng.IntN(len(options) + 1)
		if pick == len(options) {
			return game.Action{Type: game.ActionPassTrump, PlayerSeat: seat}
		}
		s := options[pick]
		return game.Action{Type: game.ActionPickTrump, PlayerSeat: seat, Suit: &s}
	}

	if gs.PendingBelotSeat != nil && *gs.PendingBelotSeat == seat {
		if rng.IntN(2) == 0 {
			return game.Action{Type: game.ActionAnnounceBelot, PlayerSeat: seat}
		}
		return game.Action{Type: game.ActionSkipBelot, PlayerSeat: seat}
	}

	if gs.AwaitingDeclaration && gs.ActivePlayerSeat == seat {
		if rng.IntN(2) == 0 {
			return game.Action{Type: game.ActionDeclare, PlayerSeat: seat}
		}
		return game.Action{Type: game.ActionSkipDeclare, PlayerSeat: seat}
	}

	legal := game.LegalCards(gs, seat)
	c := legal[rng.IntN(len(legal))]
	return game.Action{Type: game.ActionPlayCard, PlayerSeat: seat, Card: &c}
}

// TestSimulation_CroatianHandWithBotSeats plays full Croatian hands with bots in every seat
// and checks the declaration phase and forced dealer pick never deadlock.
func TestSimulation_CroatianHandWithBotSeats(t *testing.T) {
	const handsTarget = 200
	rng := rand.New(rand.NewPCG(12, 6))

	sawDeclarationPhase := 0
	sawAnsweredSeat := 0
	sawForcedPick := 0

	tailObserver := func(t *testing.T, cur *game.GameState, seat int, action game.Action) {
		if !game.MustPickTrump(cur, seat) {
			return
		}
		sawForcedPick++
		require.Equal(t, cur.DealerSeat, seat,
			"only the dealer can be the seat with no legal pass")
		require.Equal(t, game.ActionPickTrump, action.Type,
			"the dealer has no legal pass here — a pass is the deadlock this run proves gone")
		require.NotNil(t, action.Suit, "a free-suit pick must carry a suit")
	}

	for hand := 0; hand < handsTarget; hand++ {
		gs := game.NewGame(
			[4]uint{0, 0, 0, 0},
			[4]string{"", "", "", ""},
			[4]bool{true, true, true, true},
			game.VariantCroatia, "1001", 1, true, false, true,
		)
		require.Equal(t, game.DeclarationTimingDedicatedPhase, gs.Rules.DeclarationTiming)
		mem := bot.NewMemory()

		answered := map[int]bool{}
		enteredPhase := false

		observe := func(t *testing.T, cur *game.GameState, seat int, action game.Action) {
			tailObserver(t, cur, seat, action)
			switch cur.Phase {
			case game.PhaseDeclaring:
				if !enteredPhase {
					enteredPhase = true
					sawDeclarationPhase++
				}
				require.Equal(t, 0, cur.TrickNumber, "no trick is open during the declaration phase")
				require.False(t, cur.AwaitingDeclaration,
					"the phase asks all four seats at once — no seat is ever on the clock")
				require.NotNil(t, cur.TrumpSuit, "the phase is only reachable after a resolved bid")
				require.Equal(t, (cur.DealerSeat+1)%4, cur.ActivePlayerSeat,
					"pinned to the trick-1 leader for the phase's whole duration")
				require.False(t, cur.Players[seat].DeclarationAnswered,
					"seat %d is already marked answered in the engine state", seat)
				require.False(t, answered[seat], "the driver drove seat %d twice", seat)
				answered[seat] = true
				sawAnsweredSeat++
				require.Contains(t,
					[]string{game.ActionDeclare, game.ActionSkipDeclare}, action.Type,
					"bot must answer, not play a card, in the declaration phase")

			case game.PhasePlaying:
				require.True(t, cur.DeclarationsResolved,
					"declarations are settled before the first card under this config")
				require.False(t, cur.AwaitingDeclaration,
					"no declaration may be owed once play has started")
			}
		}

		_, _ = playOneHandWith(t, gs, mem, rng, croatianBotDriver, observe)

		if enteredPhase {
			require.Len(t, answered, 4, "the phase opened, so every seat answered exactly once")
		}
	}

	assert.Positive(t, sawDeclarationPhase, "no hand ever opened the declaration phase")
	assert.Positive(t, sawAnsweredSeat, "no seat ever answered inside the phase")
	t.Logf("declaration phase opened in %d/%d random hands, %d seats answered, %d forced dealer picks",
		sawDeclarationPhase, handsTarget, sawAnsweredSeat, sawForcedPick)

	forcedBefore := sawForcedPick
	forced := testfixtures.NewGameCroatianMidBidding(3)
	require.True(t, game.MustPickTrump(forced, forced.ActivePlayerSeat),
		"the tail fixture must be the state where the dealer has no legal pass")
	_, _ = playOneHandWith(t, forced, bot.NewMemory(), rng, croatianBotDriver, tailObserver)
	assert.Greater(t, sawForcedPick, forcedBefore,
		"the deterministic tail hand must have gone through the forced dealer pick")
}

// croatianBotDriver plays every seat with bot.Decide, with no substitutions.
func croatianBotDriver(gs *game.GameState, mem *bot.Memory, rng *rand.Rand) (int, game.Action) {
	_ = rng

	seat := gs.ActivePlayerSeat
	if gs.Phase == game.PhasePlaying && gs.PendingBelotSeat != nil {
		seat = *gs.PendingBelotSeat
	}
	if gs.Phase == game.PhaseDeclaring {
		for s := range gs.Players {
			if !gs.Players[s].DeclarationAnswered {
				seat = s
				break
			}
		}
	}

	return seat, bot.Decide(viewFromState(gs, seat, mem))
}
