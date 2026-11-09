package bot_test

import (
	"fmt"
	"math"
	"math/bits"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/bot"
	"github.com/emilijan/beljot/server/internal/bot/search"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
)

const allCards = uint32(math.MaxUint32)

type earlierDiscard struct {
	seat int
	card string
	lead game.Suit
}

func memoryOfMissingCards(gs *game.GameState, discards ...earlierDiscard) *bot.Memory {
	mem := bot.NewMemory()
	held := map[game.Card]bool{}
	for _, player := range gs.Players {
		for _, c := range player.Hand {
			held[c] = true
		}
	}
	for _, tc := range gs.CurrentTrick {
		held[tc.Card] = true
	}
	for _, d := range discards {
		lead := d.lead
		mem.ObservePlay(d.seat, card(d.card), &lead, gs.TrumpSuit, nil)
		held[card(d.card)] = true
	}
	for _, c := range game.NewDeck() {
		if !held[c] {
			mem.ObservePlay(0, c, nil, gs.TrumpSuit, nil)
		}
	}
	return mem
}

func playToTrick(t *testing.T, gs *game.GameState, seat int, c game.Card) {
	t.Helper()
	i := slices.Index(gs.Players[seat].Hand, c)
	require.GreaterOrEqual(t, i, 0, "seat %d does not hold %v", seat, c)
	gs.Players[seat].Hand = slices.Delete(slices.Clone(gs.Players[seat].Hand), i, i+1)
	if len(gs.CurrentTrick) == 0 {
		lead := c.Suit
		gs.LeadSuit = &lead
	}
	gs.CurrentTrick = append(gs.CurrentTrick, game.TrickCard{Card: c, PlayerSeat: seat})
	gs.ActivePlayerSeat = gs.Rules.NextSeat(seat)
}

func trickCards(gs *game.GameState) []game.Card {
	out := make([]game.Card, len(gs.CurrentTrick))
	for i, tc := range gs.CurrentTrick {
		out[i] = tc.Card
	}
	return out
}

func declaredRunsScenario(t *testing.T) (*game.GameState, *bot.Memory) {
	t.Helper()
	gs := testfixtures.NewGameMidPlay(3)
	gs.Rules.Clockwise = false
	playToTrick(t, gs, 0, card("TH"))
	playToTrick(t, gs, 1, card("9H"))
	gs.Players[0].Declarations = []game.Declaration{{
		Type: game.DeclarationSequence, Cards: cards("QS", "KS", "AS"), PlayerSeat: 0,
	}}
	gs.Players[3].Declarations = []game.Declaration{{
		Type: game.DeclarationSequence, Cards: cards("7D", "8D", "9D"), PlayerSeat: 3,
	}}
	mem := memoryOfMissingCards(gs, earlierDiscard{seat: 3, card: "JC", lead: game.SuitSpades})
	mem.ObserveDeclarations(gs.Players)
	return gs, mem
}

func TestInformationFromViewAccountsForEveryCard(t *testing.T) {
	gs, mem := declaredRunsScenario(t)
	v := viewFromState(gs, 2, mem)

	in := bot.InformationFromForTest(v)

	known := in.Known[0] | in.Known[1] | in.Known[2] | in.Known[3]
	played := search.MaskOf(mem.PlayedCards())
	trick := search.MaskOf(trickCards(gs))
	assert.Equal(t, allCards&^played, in.Own|in.Unseen|known|trick)
	assert.Equal(t, search.NCards,
		bits.OnesCount32(in.Own)+bits.OnesCount32(in.Unseen)+bits.OnesCount32(known)+
			bits.OnesCount32(trick)+bits.OnesCount32(played), "every card sits in exactly one place")

	assert.Equal(t, 2, in.Seat)
	assert.Equal(t, search.MaskOf(v.Hand), in.Own)
	assert.Equal(t, search.MaskOf(cards("QS", "KS", "AS")), in.Known[0])
	assert.Equal(t, search.MaskOf(cards("7D", "8D", "9D")), in.Known[3])
	assert.Zero(t, in.Known[1])
	assert.Zero(t, in.Known[2])
	assert.Zero(t, known&in.Unseen)

	assert.Equal(t, [4]int{2, 5, 0, 3}, in.Need)
	assert.Equal(t, bits.OnesCount32(in.Unseen), in.Need[0]+in.Need[1]+in.Need[2]+in.Need[3])

	assert.Equal(t, search.MaskOf(cards("JS")), in.CannotHold[0], "the card below a declared A-K-Q run")
	assert.Zero(t, in.CannotHold[1])
	assert.Zero(t, in.CannotHold[2])
	voidSuits := search.SuitMask(search.SuitIdx(game.SuitSpades)) | search.SuitMask(search.SuitIdx(game.SuitHearts))
	assert.Equal(t, voidSuits|search.MaskOf(cards("TD")), in.CannotHold[3],
		"the void suits and the card above a declared 7-8-9 run")
	for seat := range 4 {
		assert.Zero(t, in.Known[seat]&in.CannotHold[seat], "seat %d", seat)
	}
}

func TestInformationFromViewFollowsTheDirectionOfPlay(t *testing.T) {
	for leader := range 4 {
		t.Run(fmt.Sprintf("leader %d", leader), func(t *testing.T) {
			gs := testfixtures.NewGameMidPlay(3)
			gs.Rules.Clockwise = true
			second := game.StepSeat(leader, 1, true)
			viewer := game.StepSeat(leader, 2, true)
			playToTrick(t, gs, leader, gs.Players[leader].Hand[0])
			playToTrick(t, gs, second, gs.Players[second].Hand[0])
			mem := memoryOfMissingCards(gs)

			in := bot.InformationFromForTest(viewFromState(gs, viewer, mem))

			var want [4]int
			for seat := range 4 {
				switch seat {
				case viewer:
					want[seat] = 0
				case leader, second:
					want[seat] = 5
				default:
					want[seat] = 6
				}
			}
			assert.Equal(t, want, in.Need)
			assert.Equal(t, bits.OnesCount32(in.Unseen), in.Need[0]+in.Need[1]+in.Need[2]+in.Need[3])
		})
	}
}

func TestPositionFromViewListsTheTrickInPlayOrder(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(3)
	gs.Rules.Clockwise = false
	hearts := search.SuitIdx(game.SuitHearts)

	empty := bot.PositionFromForTest(viewFromState(gs, 0, memoryOfMissingCards(gs)), bot.SearchTuning{})
	assert.Equal(t, 0, empty.Leader)
	assert.Empty(t, empty.Trick)
	assert.Equal(t, hearts, empty.Trump)
	assert.False(t, empty.Clockwise)
	assert.Equal(t, search.CapotFlag(gs.TricksWon), empty.Capot)

	gs.Rules.Clockwise = true
	playToTrick(t, gs, 1, card("9S"))
	playToTrick(t, gs, 0, card("AS"))
	opened := bot.PositionFromForTest(viewFromState(gs, 3, memoryOfMissingCards(gs)), bot.SearchTuning{})
	assert.Equal(t, 1, opened.Leader)
	assert.Equal(t, []int{search.CardIdx(card("9S")), search.CardIdx(card("AS"))}, opened.Trick)
	assert.True(t, opened.Clockwise)
}

func TestSettlementFromViewCarriesTheHandAndCountsTheCardsInPlay(t *testing.T) {
	gs, mem := declaredRunsScenario(t)
	gs.DeclarationPoints = [2]int{50, 20}
	gs.BelotPoints = [2]int{0, 20}
	gs.Rules.DeclarationsTransfer = true
	v := viewFromState(gs, 2, mem)

	s := bot.SettlementFromForTest(v)

	assert.Equal(t, gs.HandPoints, s.HandPoints)
	assert.Equal(t, [2]int{70, 40}, s.DeclPoints)
	assert.Equal(t, gs.TricksWon, s.TricksWon)
	assert.Equal(t, game.TeamForSeat(*gs.TrumpCallerSeat), s.CallerTeam)
	assert.True(t, s.DeclarationsTransfer)

	hearts := search.SuitIdx(game.SuitHearts)
	inPlay := allCards &^ search.MaskOf(mem.PlayedCards())
	want := 0
	for _, c := range search.CardsOf(inPlay) {
		want += search.Points(search.CardIdx(c), hearts)
	}
	assert.Equal(t, want, s.RemainingCardPoints)
	assert.Equal(t, want, bot.RemainingCardPointsForTest(v))
}

func TestSearchAppliesGates(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(3)
	base := viewFromState(gs, 0, memoryOfMissingCards(gs))
	require.Equal(t, 3, base.TrickNumber)
	remaining := bot.RemainingCardPointsForTest(base)
	require.Greater(t, remaining, 10)

	withoutRace := bot.DefaultSearchTuning()
	withoutRace.RaceObjective = false
	banked := func(v bot.View, team int) int {
		return v.TeamScores[team] + v.HandPoints[team] + v.DeclPoints[team]
	}
	tests := []struct {
		name      string
		fromTrick int
		mutate    func(v *bot.View)
		want      bool
	}{
		{"trick 3 from trick 3", 3, func(v *bot.View) {}, true},
		{"trick 2 from trick 3", 3, func(v *bot.View) { v.TrickNumber = 2 }, false},
		{"trick 2 from trick 1", 1, func(v *bot.View) { v.TrickNumber = 2 }, true},
		{"trick 3 from trick 1", 1, func(v *bot.View) {}, true},
		{"from trick 0 never searches", 0, func(v *bot.View) {}, false},
		{"one legal card", 3, func(v *bot.View) { v.LegalCards = v.LegalCards[:1] }, false},
		{"pending belote", 3, func(v *bot.View) { v.PendingBelot = true }, false},
		{"awaiting declaration", 3, func(v *bot.View) { v.AwaitingDeclaration = true }, false},
		{"another seat is active", 3, func(v *bot.View) { v.ActivePlayerSeat = 1 }, false},
		{"partner proposed surrender", 3, func(v *bot.View) { v.PartnerProposedSurrender = true }, false},
		{"not the playing phase", 3, func(v *bot.View) { v.Phase = game.PhaseDeclaring }, false},
		{"no trump", 3, func(v *bot.View) { v.TrumpSuit = nil }, false},
		{"team A 20 short of the target", 3, func(v *bot.View) {
			v.StopAtTarget = true
			v.MatchTarget = banked(*v, 0) + 20
		}, false},
		{"team A 200 short of the target", 3, func(v *bot.View) {
			v.StopAtTarget = true
			v.MatchTarget = banked(*v, 0) + 200
		}, true},
		{"team B alone in reach of the target", 3, func(v *bot.View) {
			v.StopAtTarget = true
			v.TeamScores = [2]int{0, 500}
			v.MatchTarget = banked(*v, 1) + 20
		}, false},
		{"exactly the points left plus the last trick bonus", 3, func(v *bot.View) {
			v.StopAtTarget = true
			v.MatchTarget = banked(*v, 0) + remaining + 10
		}, false},
		{"one point beyond the points left plus the bonus", 3, func(v *bot.View) {
			v.StopAtTarget = true
			v.MatchTarget = banked(*v, 0) + remaining + 11
		}, true},
		{"the target is ignored without stop at target", 3, func(v *bot.View) {
			v.MatchTarget = banked(*v, 0) + 20
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := base
			tt.mutate(&v)
			assert.Equal(t, tt.want, bot.SearchAppliesWith(v, tt.fromTrick, withoutRace))
		})
	}
}

func giveSeatTheTrumpPair(gs *game.GameState, seat int) {
	pair := cards("KH", "QH")
	next := 0
	for other := range gs.Players {
		if other == seat {
			continue
		}
		for i, c := range gs.Players[other].Hand {
			if !slices.Contains(pair, c) {
				continue
			}
			for gs.Players[seat].Hand[next].Suit == game.SuitHearts {
				next++
			}
			gs.Players[other].Hand[i], gs.Players[seat].Hand[next] = gs.Players[seat].Hand[next], c
		}
	}
}

func TestStopAtTargetReachCountsTheBotsOwnBelote(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(3)
	giveSeatTheTrumpPair(gs, 0)
	v := viewFromState(gs, 0, memoryOfMissingCards(gs))
	require.Equal(t, game.SuitHearts, *v.TrumpSuit)
	require.Subset(t, v.Hand, cards("KH", "QH"))
	require.True(t, v.DeclarationsEnabled)
	require.True(t, bot.SearchAppliesWith(v, 3, bot.SearchTuning{}))

	v.StopAtTarget = true
	banked := v.TeamScores[0] + v.HandPoints[0] + v.DeclPoints[0]
	reach := bot.RemainingCardPointsForTest(v) + 10
	withoutRace := bot.SearchTuning{}

	v.MatchTarget = banked + reach + 20
	assert.False(t, bot.SearchAppliesWith(v, 3, withoutRace), "the certain Belote's 20 brings the target within reach")
	v.MatchTarget++
	assert.True(t, bot.SearchAppliesWith(v, 3, withoutRace), "one point beyond the Belote's reach")

	v.MatchTarget = banked + reach + 20
	v.DeclarationsEnabled = false
	assert.True(t, bot.SearchAppliesWith(v, 3, withoutRace), "without declarations there is no Belote to count")
}

func TestSearchCardPlaysALegalCardDeterministically(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(5)
	v := viewFromState(gs, 0, memoryOfMissingCards(gs))
	require.Equal(t, 5, v.TrickNumber)
	require.True(t, bot.SearchApplies(v, 3))

	first, ok := bot.SearchCard(v, time.Now().Add(5*time.Second))
	require.True(t, ok)
	second, ok := bot.SearchCard(v, time.Now().Add(5*time.Second))
	require.True(t, ok)

	assert.Equal(t, first, second)
	assert.Contains(t, v.LegalCards, first.Card)
	assert.Positive(t, first.Samples)
}

func TestSearchCardDeclinesWhereTheRulesPlay(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(2)
	early := viewFromState(gs, 0, memoryOfMissingCards(gs))
	_, ok := bot.SearchCardWith(early, time.Now().Add(5*time.Second), bot.SearchTuning{RaceObjective: true})
	assert.False(t, ok, "no samples are budgeted before trick 3 without opening samples")

	gs = testfixtures.NewGameMidPlay(5)
	v := viewFromState(gs, 0, memoryOfMissingCards(gs))
	_, ok = bot.SearchCard(v, time.Now().Add(-time.Second))
	assert.False(t, ok, "an expired deadline yields no decision")

	v.TrumpSuit = nil
	_, ok = bot.SearchCard(v, time.Now().Add(5*time.Second))
	assert.False(t, ok, "no trump yields no decision")
}

func farFuture() time.Time { return time.Now().Add(time.Hour) }

func TestSearchCardWithZeroTuningMatchesTheChooser(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(3)
	checked := 0
	for seat := range 4 {
		v := viewFromState(gs, seat, memoryOfMissingCards(gs))
		if !bot.SearchApplies(v, 3) {
			continue
		}
		want, okWant := bot.ChooserResultForTest(v, farFuture())
		got, okGot := bot.SearchCardWith(v, farFuture(), bot.SearchTuning{})
		require.Equal(t, okWant, okGot)
		assert.Equal(t, want.Card, got.Card)
		assert.Equal(t, want.Samples, got.Samples)
		assert.Equal(t, want.Margin, got.Margin)
		assert.Equal(t, want.SE, got.SE)
		checked++
	}
	assert.Positive(t, checked)
}

func TestSearchCardWithSeedSaltIsDeterministic(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(3)
	v := viewFromState(gs, 0, memoryOfMissingCards(gs))
	require.True(t, bot.SearchApplies(v, 3))
	salted := bot.SearchTuning{SeedSalt: 7}

	first, ok := bot.SearchCardWith(v, farFuture(), salted)
	require.True(t, ok)
	second, ok := bot.SearchCardWith(v, farFuture(), salted)
	require.True(t, ok)

	assert.Equal(t, first, second)
	assert.Contains(t, v.LegalCards, first.Card)
}

func TestSearchAppliesWithOpeningSamples(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(3)
	v := viewFromState(gs, 0, memoryOfMissingCards(gs))
	v.TrickNumber = 1
	assert.False(t, bot.SearchAppliesWith(v, 1, bot.SearchTuning{}))
	assert.True(t, bot.SearchAppliesWith(v, 1, bot.SearchTuning{OpeningSamples: 12}))
	assert.False(t, bot.SearchAppliesWith(v, 3, bot.SearchTuning{OpeningSamples: 12}))
}

func TestDecideWithFallsBackToTheRules(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(3)
	base := viewFromState(gs, 0, memoryOfMissingCards(gs))
	rules := bot.Decide(base)
	require.NotNil(t, rules.Card)
	var other game.Card
	for _, c := range base.LegalCards {
		if c != *rules.Card {
			other = c
			break
		}
	}
	require.NotEqual(t, game.Card{}, other)

	played := func(c game.Card) game.Action {
		return game.Action{Type: game.ActionPlayCard, PlayerSeat: base.Seat, Card: &c}
	}
	tests := []struct {
		name   string
		mutate func(v *bot.View)
		pre    *bot.SearchResult
		want   game.Action
	}{
		{"no search result", func(v *bot.View) {}, nil, rules},
		{"search card not legal", func(v *bot.View) {}, &bot.SearchResult{Card: card("7D")}, rules},
		{"search card legal", func(v *bot.View) {}, &bot.SearchResult{Card: other}, played(other)},
		{"belote pending", func(v *bot.View) { v.PendingBelot = true }, &bot.SearchResult{Card: other},
			game.Action{Type: game.ActionAnnounceBelot, PlayerSeat: base.Seat}},
		{"declaration pending", func(v *bot.View) { v.AwaitingDeclaration = true }, &bot.SearchResult{Card: other},
			game.Action{Type: game.ActionDeclare, PlayerSeat: base.Seat}},
		{"partner proposed surrender", func(v *bot.View) { v.PartnerProposedSurrender = true },
			&bot.SearchResult{Card: other},
			game.Action{Type: game.ActionSurrenderAccept, PlayerSeat: base.Seat}},
		{"not the playing phase", func(v *bot.View) {
			v.Phase = game.PhaseDeclaring
			v.HasDeclarations = false
		}, &bot.SearchResult{Card: other},
			game.Action{Type: game.ActionSkipDeclare, PlayerSeat: base.Seat}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := base
			tt.mutate(&v)
			assert.Equal(t, tt.want, bot.DecideWith(v, tt.pre))
		})
	}
}

func TestSearchCardWithDefersAGuessedTrumpDraw(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(3)
	checked := 0
	for seat := range 4 {
		v := viewFromState(gs, seat, memoryOfMissingCards(gs))
		if !bot.SearchApplies(v, 3) {
			continue
		}
		got, ok := bot.SearchCardWith(v, farFuture(), bot.SearchTuning{TrumpDrawConditional: true})
		require.True(t, ok)
		assert.Contains(t, v.LegalCards, got.Card)
		if got.Deferred {
			assert.Equal(t, bot.ChooseCardForTest(v), got.Card)
		}
		checked++
	}
	assert.Positive(t, checked)
}

func TestSettlementFromViewCountsTheBotsOwnBelote(t *testing.T) {
	base := func() bot.View {
		trump, caller := game.SuitHearts, 1
		return bot.View{
			Seat: 0, Phase: game.PhasePlaying, TrumpSuit: &trump, TrumpCallerSeat: &caller,
			DeclarationsEnabled: true, DeclPoints: [2]int{20, 50},
			Hand: cards("KH", "QH", "7S"),
		}
	}
	tests := []struct {
		name   string
		mutate func(v *bot.View)
		want   [2]int
	}{
		{"king and queen of trump", func(v *bot.View) {}, [2]int{40, 50}},
		{"queen already played", func(v *bot.View) { v.Hand = cards("KH", "7S") }, [2]int{20, 50}},
		{"declarations disabled", func(v *bot.View) { v.DeclarationsEnabled = false }, [2]int{20, 50}},
		{"king and queen of a side suit", func(v *bot.View) { v.Hand = cards("KS", "QS", "7S") }, [2]int{20, 50}},
		{"the seat of team B", func(v *bot.View) { v.Seat = 3 }, [2]int{20, 70}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := base()
			tt.mutate(&v)
			assert.Equal(t, tt.want, bot.SettlementFromForTest(v).DeclPoints)
		})
	}
}

func TestOwnBeloteSettlementMatchesTheEngine(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(7)
	gs.BelotAnnounced = false
	gs.Players[0].Hand = cards("KH", "QH")
	gs.Players[1].Hand = cards("JS", "9S")
	gs.Players[2].Hand = cards("AD", "TD")
	gs.Players[3].Hand = cards("JD", "9D")
	gs.ActivePlayerSeat = 0
	require.True(t, gs.Rules.DeclarationsEnabled)

	var hands [4]uint32
	for seat := range hands {
		hands[seat] = search.MaskOf(gs.Players[seat].Hand)
	}
	view := viewFromState(gs, 0, memoryOfMissingCards(gs))
	require.True(t, view.DeclarationsEnabled)
	pos := bot.PositionFromForTest(view, bot.SearchTuning{})
	remainder := search.NewSolver(pos.Trump, pos.Clockwise).Value(hands, pos.Leader, pos.Trick, pos.Capot)
	predicted := search.Settle(bot.SettlementFromForTest(view), remainder)

	for range 8 {
		if gs.Phase != game.PhasePlaying {
			break
		}
		seat := gs.ActivePlayerSeat
		next, err := game.ApplyAction(gs, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: seat, Card: &game.LegalCards(gs, seat)[0],
		})
		require.NoError(t, err)
		gs = next
		if gs.PendingBelotSeat != nil {
			require.Equal(t, 0, *gs.PendingBelotSeat)
			gs, err = game.ApplyAction(gs, game.Action{Type: game.ActionAnnounceBelot, PlayerSeat: 0})
			require.NoError(t, err)
		}
	}

	require.NotNil(t, gs.LastHandResult, "the two tricks must end the hand")
	assert.Equal(t, 20, gs.BelotPoints[game.TeamA])
	assert.Equal(t, predicted, gs.LastHandResult.TeamAHandTotal-gs.LastHandResult.TeamBHandTotal)
}

func TestSearchAppliesWithRaceObjective(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(3)
	v := viewFromState(gs, 0, memoryOfMissingCards(gs))
	v.StopAtTarget = true
	v.MatchTarget = v.TeamScores[0] + v.HandPoints[0] + v.DeclPoints[0] + 20

	racing := bot.SearchTuning{RaceObjective: true}
	assert.False(t, bot.SearchAppliesWith(v, 3, bot.SearchTuning{}))
	assert.True(t, bot.SearchAppliesWith(v, 3, racing))

	pos := bot.PositionFromForTest(v, racing)
	require.NotNil(t, pos.Race)
	assert.Equal(t, search.Race{
		Target: v.MatchTarget,
		Banked: [2]int{v.TeamScores[0] + v.HandPoints[0] + v.DeclPoints[0], v.TeamScores[1] + v.HandPoints[1] + v.DeclPoints[1]},
		Scores: v.TeamScores,
		Caller: game.TeamForSeat(*v.TrumpCallerSeat),
	}, *pos.Race)
	assert.Nil(t, bot.PositionFromForTest(v, bot.SearchTuning{}).Race)

	v.StopAtTarget = false
	assert.Nil(t, bot.PositionFromForTest(v, racing).Race)
}

func TestRaceCarriesTheBotsOwnBeloteApartFromTheBank(t *testing.T) {
	trump, caller := game.SuitHearts, 1
	v := bot.View{
		Seat: 2, Phase: game.PhasePlaying, TrumpSuit: &trump, TrumpCallerSeat: &caller,
		DeclarationsEnabled: true, StopAtTarget: true, MatchTarget: 1001,
		TeamScores: [2]int{950, 900}, HandPoints: [2]int{10, 5}, DeclPoints: [2]int{0, 20},
		Hand: cards("KH", "QH", "7S"),
	}
	pos := bot.PositionFromForTest(v, bot.SearchTuning{RaceObjective: true})
	require.NotNil(t, pos.Race)
	assert.Equal(t, [2]int{960, 925}, pos.Race.Banked)
	assert.Equal(t, game.TeamB, pos.Race.Caller)
	assert.Equal(t, 2, pos.Race.BeloteSeat)
	assert.Equal(t, search.MaskOf(cards("KH", "QH")), pos.Race.BeloteCards)

	v.Hand = cards("KH", "7S")
	pos = bot.PositionFromForTest(v, bot.SearchTuning{RaceObjective: true})
	require.NotNil(t, pos.Race)
	assert.Equal(t, [2]int{960, 925}, pos.Race.Banked)
	assert.Zero(t, pos.Race.BeloteCards)
}

func TestCannotHoldIncludesCardsAboveTheBound(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(3)
	v := viewFromState(gs, 0, memoryOfMissingCards(gs))
	require.Equal(t, game.SuitHearts, *v.TrumpSuit)
	v.RankBounds[1][bot.SuitIndex(game.SuitSpades)] = game.NonTrumpRankOrder[game.RankKing]
	v.RankBounds[1][bot.SuitIndex(game.SuitHearts)] = game.TrumpRankOrder[game.Rank9]

	in := bot.InformationFromForTest(v)

	cannot := func(id string) bool { return in.CannotHold[1]&(1<<uint(search.CardIdx(card(id)))) != 0 }
	assert.True(t, cannot("AS"))
	assert.True(t, cannot("TS"))
	assert.False(t, cannot("KS"))
	assert.False(t, cannot("7S"))
	assert.True(t, cannot("JH"))
	assert.False(t, cannot("9H"))
	assert.False(t, cannot("AH"))
	assert.Zero(t, in.CannotHold[2]&search.SuitMask(search.SuitIdx(game.SuitSpades)))
}

// shortOfTarget is seat 0's view of a default-table state with each team's running total the given points short of
// the target.
func shortOfTarget(t *testing.T, gs *game.GameState, shortA, shortB int) bot.View {
	t.Helper()
	require.False(t, gs.Rules.StopAtTarget)
	v := viewFromState(gs, 0, memoryOfMissingCards(gs))
	short := [2]int{shortA, shortB}
	for team := range v.TeamScores {
		v.TeamScores[team] = v.MatchTarget - short[team] - v.HandPoints[team] - v.DeclPoints[team]
	}
	return v
}

func TestRaceForTheDecidingHand(t *testing.T) {
	v := shortOfTarget(t, testfixtures.NewGameMidPlay(3), 500, 60)
	deciding := bot.SearchTuning{DecidingHand: true}
	pos := bot.PositionFromForTest(v, deciding)
	require.NotNil(t, pos.Race)
	assert.True(t, pos.Race.DecidingHand)
	assert.Equal(t, v.MatchTarget, pos.Race.Target)
	assert.Equal(t, v.TeamScores, pos.Race.Scores)
	assert.Equal(t, game.TeamForSeat(*v.TrumpCallerSeat), pos.Race.Caller)

	assert.Nil(t, bot.PositionFromForTest(v, bot.SearchTuning{RaceObjective: true}).Race)

	v.StopAtTarget = true
	assert.Nil(t, bot.PositionFromForTest(v, deciding).Race, "a stop-at-target table out of reach does not race")
}

func TestNoRaceWhenNobodyCanCross(t *testing.T) {
	v := shortOfTarget(t, testfixtures.NewGameMidPlay(3), 500, 500)
	deciding := bot.DefaultSearchTuning()
	deciding.DecidingHand = true
	noDeciding := deciding
	noDeciding.DecidingHand = false
	assert.Nil(t, bot.PositionFromForTest(v, deciding).Race)

	with, ok := bot.SearchCardWith(v, farFuture(), deciding)
	require.True(t, ok)
	without, ok := bot.SearchCardWith(v, farFuture(), noDeciding)
	require.True(t, ok)
	assert.Equal(t, without, with)
}

func TestDecidingHandValuesAFallAsTheLostMatch(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(7)
	caller := 2
	gs.TrumpCallerSeat = &caller
	gs.Players[0].Hand = cards("TC", "7D")
	gs.Players[1].Hand = cards("TS", "9C")
	gs.Players[2].Hand = cards("QD", "AS")
	gs.Players[3].Hand = cards("7H", "8C")
	gs.HandPoints, gs.TricksWon = [2]int{75, 43}, [2]int{3, 3}
	v := shortOfTarget(t, gs, 500, 100)
	deciding := bot.DefaultSearchTuning()
	deciding.DecidingHand = true
	noDeciding := deciding
	noDeciding.DecidingHand = false

	points, ok := bot.SearchCardWith(v, farFuture(), noDeciding)
	require.True(t, ok)
	require.Equal(t, card("7D"), points.Card, "for points the seven risks the fall; the deals come from the "+
		"sampler's seed, so after a sampler change re-derive the hands until this holds")
	match, ok := bot.SearchCardWith(v, farFuture(), deciding)
	require.True(t, ok)
	assert.Equal(t, card("TC"), match.Card)

	res, ok := bot.ChooseSearchForTest(v, deciding)
	require.True(t, ok)
	assert.Less(t, res.Cards[search.CardIdx(card("7D"))].Mean, res.Cards[search.CardIdx(card("TC"))].Mean)
	const worstHandAward = 152 + 100
	assert.Less(t, res.Cards[search.CardIdx(card("7D"))].Mean, -float64(worstHandAward), "some deals lose the match")
}

// beloteFallView is seat 0 of the calling team just after leading the Queen of trump at trick 7, holding the King,
// with every unseen card worth nothing and the opponents far ahead, so the contract falls whatever is played.
func beloteFallView() bot.View {
	trump, caller := game.SuitHearts, 0
	lead := game.SuitHearts
	held := cards("KH", "QH", "7S", "8S", "7D", "8D", "7C", "8C")
	var played []game.Card
	for _, c := range game.NewDeck() {
		if !slices.Contains(held, c) {
			played = append(played, c)
		}
	}
	return bot.View{
		Seat: 0, Phase: game.PhasePlaying, TrumpSuit: &trump, TrumpCallerSeat: &caller, LeadSuit: &lead,
		Hand: cards("KH"), LegalCards: cards("KH"), ActivePlayerSeat: 0, PendingBelot: true,
		CurrentTrick: []game.TrickCard{{Card: card("QH"), PlayerSeat: 0}},
		HandPoints:   [2]int{10, 135}, TricksWon: [2]int{1, 5}, TrickNumber: 7, MatchTarget: 1001,
		DeclarationsEnabled: true, DeclarationsTransfer: true, PlayedCards: played, RankBounds: bot.NoRankBounds(),
	}
}

func beloteGate() bot.SearchTuning {
	t := bot.DefaultSearchTuning()
	t.BeloteSkipMargin = 5
	return t
}

func TestInformationFromViewAfterTheSeatPlayed(t *testing.T) {
	in := bot.InformationFromForTest(beloteFallView())

	assert.Equal(t, [4]int{0, 2, 2, 2}, in.Need)
	assert.Equal(t, search.MaskOf(cards("7S", "8S", "7D", "8D", "7C", "8C")), in.Unseen)
}

func TestBeloteAnswerSkipsWhenTheFallIsCertain(t *testing.T) {
	announce, ok := bot.BeloteAnswerWith(beloteFallView(), farFuture(), beloteGate())
	require.True(t, ok)
	assert.False(t, announce, "skipping keeps the 20 from the opponents")

	announce, ok = bot.BeloteAnswerWith(beloteFallView(), farFuture(), bot.DefaultSearchTuning())
	assert.False(t, ok)
	assert.True(t, announce, "a zero margin always announces")
}

func TestBeloteAnswerDefenderAlwaysAnnounces(t *testing.T) {
	v := beloteFallView()
	caller := 1
	v.TrumpCallerSeat = &caller

	announce, ok := bot.BeloteAnswerWith(v, farFuture(), beloteGate())
	assert.False(t, ok)
	assert.True(t, announce)
}

func TestBeloteAnswerTransferOffAlwaysAnnounces(t *testing.T) {
	v := beloteFallView()
	v.DeclarationsTransfer = false

	announce, ok := bot.BeloteAnswerWith(v, farFuture(), beloteGate())
	assert.False(t, ok)
	assert.True(t, announce)
}

func TestBeloteAnswerMissedDeadlineAnnounces(t *testing.T) {
	announce, ok := bot.BeloteAnswerWith(beloteFallView(), time.Now().Add(-time.Second), beloteGate())
	assert.False(t, ok)
	assert.True(t, announce)
}

func TestBeloteAnswerAnnouncesWhenTheBeloteDecidesTheRace(t *testing.T) {
	v := beloteFallView()
	v.StopAtTarget = true
	v.TeamScores = [2]int{v.MatchTarget - 10 - 15, 0}

	announce, ok := bot.BeloteAnswerWith(v, farFuture(), beloteGate())
	assert.False(t, ok, "the race is decided before any search")
	assert.True(t, announce)

	v.TeamScores[0] -= 10
	announce, ok = bot.BeloteAnswerWith(v, farFuture(), beloteGate())
	assert.True(t, ok)
	assert.False(t, announce, "five points short after the Belote, the fall still decides")
}

func TestBeloteAnswerAnnouncesWhenTheTwentyMakesTheContract(t *testing.T) {
	v := beloteFallView()
	v.HandPoints = [2]int{60, 85}

	announce, ok := bot.BeloteAnswerWith(v, farFuture(), beloteGate())
	require.True(t, ok)
	assert.True(t, announce, "77 card points fall against 85, the Belote's 20 makes the contract")

	v.HandPoints = [2]int{40, 85}
	announce, ok = bot.BeloteAnswerWith(v, farFuture(), beloteGate())
	require.True(t, ok)
	assert.False(t, announce, "57 card points plus the 20 still fall against 85, the Belote would only go to the opponents")

	v.HandPoints = [2]int{75, 70}
	announce, ok = bot.BeloteAnswerWith(v, farFuture(), beloteGate())
	require.True(t, ok)
	assert.True(t, announce, "a contract made either way keeps its Belote")
}

func TestBeloteAnswerIsDeterministic(t *testing.T) {
	v := beloteFallView()
	v.HandPoints = [2]int{60, 85}
	first, ok := bot.BeloteAnswerWith(v, farFuture(), beloteGate())
	require.True(t, ok)
	second, ok := bot.BeloteAnswerWith(v, farFuture(), beloteGate())
	require.True(t, ok)
	assert.Equal(t, first, second)
}

func TestDecideWithSkipsTheBeloteOnASearchedSkip(t *testing.T) {
	v := beloteFallView()
	skip := game.Action{Type: game.ActionSkipBelot, PlayerSeat: 0}
	announce := game.Action{Type: game.ActionAnnounceBelot, PlayerSeat: 0}

	assert.Equal(t, announce, bot.Decide(v))
	assert.Equal(t, announce, bot.DecideWith(v, nil))
	assert.Equal(t, announce, bot.DecideWith(v, &bot.SearchResult{}))
	assert.Equal(t, skip, bot.DecideWith(v, &bot.SearchResult{BeloteSkip: true}))

	v.PendingBelot = false
	assert.NotEqual(t, skip.Type, bot.DecideWith(v, &bot.SearchResult{BeloteSkip: true}).Type)
}

func bidConsistency(t bot.SearchTuning) bot.SearchTuning {
	t.BidConsistency = true
	return t
}

func TestSearchFallsBackWhenNoDealFitsTheAuction(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(5)
	v := viewFromState(gs, 0, memoryOfMissingCards(gs))
	require.True(t, bot.SearchApplies(v, 3))
	cand := card("7C")
	impossible := []bot.BidRecord{passRec(1, 1, 0, &cand), callRec(1, 1, 0, game.SuitClubs, &cand)}
	v.Bids = impossible
	require.NotNil(t, bot.BidConsistentForTest(v))

	want, ok := bot.SearchCardWith(v, farFuture(), bot.DefaultSearchTuning())
	require.True(t, ok)
	got, ok := bot.SearchCardWith(v, farFuture(), bidConsistency(bot.DefaultSearchTuning()))
	require.True(t, ok, "a contradictory auction falls back to unfiltered sampling")
	assert.Equal(t, want, got)

	belote := beloteFallView()
	belote.Bids = impossible
	wantAnnounce, ok := bot.BeloteAnswerWith(belote, farFuture(), beloteGate())
	require.True(t, ok)
	gotAnnounce, ok := bot.BeloteAnswerWith(belote, farFuture(), bidConsistency(beloteGate()))
	require.True(t, ok)
	assert.Equal(t, wantAnnounce, gotAnnounce)
}
