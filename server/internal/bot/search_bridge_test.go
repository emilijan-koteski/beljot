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
		mem.ObservePlay(d.seat, card(d.card), &lead, gs.TrumpSuit)
		held[card(d.card)] = true
	}
	for _, c := range game.NewDeck() {
		if !held[c] {
			mem.ObservePlay(0, c, nil, gs.TrumpSuit)
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

	empty := bot.PositionFromForTest(viewFromState(gs, 0, memoryOfMissingCards(gs)))
	assert.Equal(t, 0, empty.Leader)
	assert.Empty(t, empty.Trick)
	assert.Equal(t, hearts, empty.Trump)
	assert.False(t, empty.Clockwise)
	assert.Equal(t, search.CapotFlag(gs.TricksWon), empty.Capot)

	gs.Rules.Clockwise = true
	playToTrick(t, gs, 1, card("9S"))
	playToTrick(t, gs, 0, card("AS"))
	opened := bot.PositionFromForTest(viewFromState(gs, 3, memoryOfMissingCards(gs)))
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
	assert.Equal(t, [2]int{50, 40}, s.DeclPoints)
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
		{"from trick 0 never searches", 0, func(v *bot.View) {}, false},
		{"one legal card", 3, func(v *bot.View) { v.LegalCards = v.LegalCards[:1] }, false},
		{"pending belote", 3, func(v *bot.View) { v.PendingBelot = true }, false},
		{"awaiting declaration", 3, func(v *bot.View) { v.AwaitingDeclaration = true }, false},
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
			assert.Equal(t, tt.want, bot.SearchApplies(v, tt.fromTrick))
		})
	}
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
	_, ok := bot.SearchCard(early, time.Now().Add(5*time.Second))
	assert.False(t, ok, "no samples are budgeted before trick 3")

	gs = testfixtures.NewGameMidPlay(5)
	v := viewFromState(gs, 0, memoryOfMissingCards(gs))
	_, ok = bot.SearchCard(v, time.Now().Add(-time.Second))
	assert.False(t, ok, "an expired deadline yields no decision")

	v.TrumpSuit = nil
	_, ok = bot.SearchCard(v, time.Now().Add(5*time.Second))
	assert.False(t, ok, "no trump yields no decision")
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
