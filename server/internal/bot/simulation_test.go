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

// TestSimulation_HeuristicBeatsRandomBaseline is the AC4 evidence: seats 0+2
// (team A) play bot.Decide, seats 1+3 (team B) play a random-legal baseline,
// across ≥200 full hands driven purely through game.NewGame/ApplyAction (no
// session manager, no WS). The heuristic team must accumulate ≥60% of all
// points awarded — deliberately far below the expected true rate so CI never
// flakes. Zero ApplyAction errors across the whole simulation doubles as the
// AC3 "always legal" proof.
//
// The baseline uses a seeded source (math/rand/v2 has no global Seed); deck
// shuffles stay nondeterministic, which is fine at this sample size.
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
			game.VariantBitola, "1001", 1,
		)
		// NewGame draws the first-hand dealer at random. This experiment measures
		// heuristic quality, not seat luck, so the dealer is pinned to seat 0 and
		// the opening bidder derived from it -- the same bidding rotation the
		// >=60% threshold below was calibrated against, when NewGame hardcoded
		// dealer 0.
		//
		// Note this pins the rotation, not the deal: stage-1 hands were already
		// dealt from the RANDOM dealer inside NewGame above, and only stage-2
		// distribution follows the value set here. That costs nothing
		// statistically -- the deck is uniformly shuffled, so each seat still
		// holds a uniformly random hand either way -- but the state is not what a
		// coherent dealer-0 deal would produce, so do not add assertions here that
		// depend on which seat received which card.
		gs.DealerSeat = 0
		gs.ActivePlayerSeat = (gs.DealerSeat + 1) % 4
		mem := bot.NewMemory()

		result, ok := playOneHand(t, gs, mem, rng)
		if !ok {
			continue // instant-win deal — no hand was scored
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
// Swapping it is how a run changes WHO is playing without forking the loop —
// and therefore without forking the bot-memory bookkeeping, which is the part
// that silently drifts when a second driver is copy-pasted.
type simDriver func(gs *game.GameState, mem *bot.Memory, rng *rand.Rand) (int, game.Action)

// simObserver is called with the PRE-action state on every step, so a run can
// assert phase invariants and action legality at the exact moment a decision is
// made. nil for runs that only care about the outcome.
type simObserver func(t *testing.T, gs *game.GameState, seat int, action game.Action)

// playOneHand drives a fresh deal to its scored end and returns the hand
// result. Returns ok=false when the deal ended the match without scoring a
// hand (instant-win). Every ApplyAction error fails the test immediately —
// that is the AC3 always-legal contract.
func playOneHand(t *testing.T, gs *game.GameState, mem *bot.Memory, rng *rand.Rand) (game.HandScore, bool) {
	t.Helper()
	return playOneHandWith(t, gs, mem, rng, nextSimAction, nil)
}

// playOneHandWith is playOneHand with the decision source and an optional
// per-step observer injected. Every run shares this one loop so they all feed
// bot memory identically — a driver that learned nothing would exercise
// chooseCard on empty PlayedCards / KnownVoids / KnownCards, i.e. a different
// code path from production.
func playOneHandWith(
	t *testing.T,
	gs *game.GameState,
	mem *bot.Memory,
	rng *rand.Rand,
	drive simDriver,
	observe simObserver,
) (game.HandScore, bool) {
	t.Helper()

	// Safety bound: a hand resolves in well under 100 actions; repeated
	// all-pass reshuffles add a few more rounds.
	for steps := 0; steps < 500; steps++ {
		switch gs.Phase {
		case game.PhaseDealing:
			// Mirror the session manager's dealing → bidding auto-transition.
			gs.Phase = game.PhaseBidding

		case game.PhaseBidding, game.PhaseDeclaring, game.PhasePlaying:
			// PhaseDeclaring is unreachable under DeclarationTimingDuringFirstTrick
			// (declarations are collected inside trick 1); it is live for the
			// dedicated-phase run — see TestSimulation_CroatianHandWithBotSeats.
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
			// Mirror the match layer: once the contest resolves, fold the
			// publicly revealed declarations into memory so the heuristic team
			// actually exercises declaration awareness in this guard.
			if gs.DeclarationsResolved {
				mem.ObserveDeclarations(gs.Players)
			}

		case game.PhaseHandComplete:
			require.NotNil(t, gs.LastHandResult)
			return *gs.LastHandResult, true

		case game.PhaseMatchEnd:
			if gs.LastHandResult == nil {
				return game.HandScore{}, false // instant-win, nothing scored
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
		// Round 2: pass or any non-candidate suit, uniformly.
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

// TestSimulation_CroatianHandWithBotSeats is the deadlock proof for the
// dedicated declaration phase (Story 12.6): full Croatian hands, all four seats
// driven by bot.Decide, from the deal to a scored hand. It is the engine-level
// counterpart of the session-manager tests — no timers, no WS — so anything it
// catches is a rules-engine or bot-ladder defect, not a scheduling one.
//
// It runs through the SAME playOneHandWith loop as the Bitola experiment, only
// with a different driver, so the bots learn from played cards and resolved
// declarations exactly as they do in production. A hand-rolled loop here would
// quietly feed chooseCard an empty memory and test a path no real match takes.
//
// What it pins:
//   - bidding always finds a taker and hands off to the declaration phase (or
//     straight to trick 1 when no seat holds a meld);
//   - the phase terminates: every seat is visited once and nobody is asked twice;
//   - bot.Decide never reaches for a card there — View.LegalCards is nil in the
//     phase and chooseCard would panic on legal[0];
//   - trick 1 opens with the contest already resolved, so no declaration is ever
//     outstanding during play;
//   - the FORCED DEALER PICK: when round 1 is passed out and three seats pass
//     again, the dealer has no legal pass, and bot.Decide must name a suit. A
//     pass here is rejected by the engine, which playOneHandWith's
//     require.NoError turns into an immediate failure — so this run is also the
//     deadlock proof for Story 12.8;
//   - the hand scores.
func TestSimulation_CroatianHandWithBotSeats(t *testing.T) {
	// 200 hands: 60 already clears the phase-was-entered assertions (only hands
	// where some seat is dealt a meld open the phase), and the wider sample is
	// nearly free at this speed, so it buys extra deal coverage for the
	// forced-pick path — which needs BOTH bidding rounds to reach the dealer and
	// so shows up in roughly 5% of deals.
	//
	// Deck shuffles are NOT seeded, so that 5% is a rate, not a guarantee: over
	// 200 hands a zero is around 1 in 3,500, which is a flake rather than a
	// proof. The forced pick is therefore observed and logged here but asserted
	// on the deterministic tail hand below, and pinned exhaustively by
	// TestDecide_ForcedDealerNeverPasses.
	const handsTarget = 200
	rng := rand.New(rand.NewPCG(12, 6))

	sawDeclarationPhase := 0
	sawPromptedSeat := 0
	sawForcedPick := 0

	// tailObserver holds the forced-pick invariant on its own, so the per-hand
	// observer below and the deterministic tail hand at the end share exactly one
	// definition of it.
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
			game.VariantCroatia, "1001", 1,
		)
		require.Equal(t, game.DeclarationTimingDedicatedPhase, gs.Rules.DeclarationTiming)
		mem := bot.NewMemory()

		// Seats already answered this hand — the phase must visit each exactly once.
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
				require.True(t, cur.AwaitingDeclaration,
					"the phase only persists with a prompt outstanding — otherwise the table has nothing to wait for")
				require.NotNil(t, cur.TrumpSuit, "the phase is only reachable after a resolved bid")
				require.Equal(t, cur.ActivePlayerSeat, seat, "only the prompted seat may answer")
				require.False(t, answered[seat], "seat %d was prompted twice", seat)
				answered[seat] = true
				sawPromptedSeat++
				require.Contains(t,
					[]string{game.ActionDeclare, game.ActionSkipDeclare}, action.Type,
					"bot must answer the prompt, not play a card, in the declaration phase")

			case game.PhasePlaying:
				require.True(t, cur.DeclarationsResolved,
					"declarations are settled before the first card under this config")
				require.False(t, cur.AwaitingDeclaration,
					"no declaration may be owed once play has started")
			}
		}

		_, _ = playOneHandWith(t, gs, mem, rng, croatianBotDriver, observe)

		if enteredPhase {
			require.NotEmpty(t, answered, "the phase opened, so at least one seat was prompted")
		}
	}

	// The phase is not vacuously "safe" because it was never entered.
	assert.Positive(t, sawDeclarationPhase, "no hand ever opened the declaration phase")
	assert.Positive(t, sawPromptedSeat, "no seat was ever prompted inside the phase")
	t.Logf("declaration phase opened in %d/%d random hands, %d seats prompted, %d forced dealer picks",
		sawDeclarationPhase, handsTarget, sawPromptedSeat, sawForcedPick)

	// Deterministic tail: one hand started FROM the forced-pick state, so this
	// run always exercises it regardless of how the 200 random deals fell. Same
	// driver, same observer, so the observer's "the dealer must not pass"
	// assertions apply — and ApplyAction's require.NoError inside
	// playOneHandWith makes a pass an immediate failure.
	forcedBefore := sawForcedPick
	forced := testfixtures.NewGameCroatianMidBidding(3)
	require.True(t, game.MustPickTrump(forced, forced.ActivePlayerSeat),
		"the tail fixture must be the state where the dealer has no legal pass")
	_, _ = playOneHandWith(t, forced, bot.NewMemory(), rng, croatianBotDriver, tailObserver)
	assert.Greater(t, sawForcedPick, forcedBefore,
		"the deterministic tail hand must have gone through the forced dealer pick")
}

// croatianBotDriver plays every seat with bot.Decide — no substitutions, no
// retries. It used to swap in a free-suit pick for the one bid the bot could not
// make (the dealer bidding last under AllPassDealerMustPick has no
// legal pass); that crutch is gone, so playOneHandWith's require.NoError is now
// a strict always-legal proof over the bot's OWN decisions, including the forced
// dealer pick.
func croatianBotDriver(gs *game.GameState, mem *bot.Memory, rng *rand.Rand) (int, game.Action) {
	_ = rng // every seat is deterministic here; randomness lives in the deal

	seat := gs.ActivePlayerSeat
	if gs.Phase == game.PhasePlaying && gs.PendingBelotSeat != nil {
		seat = *gs.PendingBelotSeat
	}

	return seat, bot.Decide(viewFromState(gs, seat, mem))
}
