package game_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
)

type direction struct {
	name      string
	clockwise bool
}

var directions = []direction{
	{name: "counter-clockwise", clockwise: false},
	{name: "clockwise", clockwise: true},
}

// inDirection returns the fixture switched to clockwise play when the direction asks for it.
func inDirection(gs *game.GameState, d direction) *game.GameState {
	if d.clockwise {
		return testfixtures.WithClockwise(gs)
	}
	return gs
}

// applyOK applies the action and fails the test on any engine error.
func applyOK(t *testing.T, state *game.GameState, action game.Action) *game.GameState {
	t.Helper()
	next, err := game.ApplyAction(state, action)
	require.NoError(t, err, "%s by seat %d", action.Type, action.PlayerSeat)
	return next
}

// passSeats lets the active seat pass n times and returns the seats that passed, in order.
func passSeats(t *testing.T, state *game.GameState, n int) (*game.GameState, []int) {
	t.Helper()
	seats := make([]int, 0, n)
	for range n {
		seat := state.ActivePlayerSeat
		seats = append(seats, seat)
		state = applyOK(t, state, game.Action{Type: game.ActionPassTrump, PlayerSeat: seat})
	}
	return state, seats
}

// playTrickInTurn plays one trick from the active seat on, skipping any declaration or Belote prompt,
// and returns the seats in the order they played.
func playTrickInTurn(t *testing.T, state *game.GameState) (*game.GameState, []int) {
	t.Helper()
	seats := make([]int, 0, 4)
	for range 4 {
		if state.AwaitingDeclaration {
			state = applyOK(t, state, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: state.ActivePlayerSeat})
		}
		seat := state.ActivePlayerSeat
		legal := game.LegalCards(state, seat)
		require.NotEmpty(t, legal, "seat %d must have a legal card", seat)
		card := legal[0]
		seats = append(seats, seat)
		state = applyOK(t, state, game.Action{Type: game.ActionPlayCard, PlayerSeat: seat, Card: &card})
		if state.PendingBelotSeat != nil {
			state = applyOK(t, state, game.Action{Type: game.ActionSkipBelot, PlayerSeat: *state.PendingBelotSeat})
		}
	}
	return state, seats
}

// reserveDealOrder lists the seats in the order they received the stage-2 reserve after a Bitola bid.
func reserveDealOrder(before, after *game.GameState) []int {
	position := make(map[game.Card]int, len(before.Deck))
	for i, c := range before.Deck {
		position[c] = i
	}
	first := map[int]int{}
	for seat := range after.Players {
		for _, c := range after.Players[seat].Hand[len(before.Players[seat].Hand):] {
			if i, ok := position[c]; ok {
				if prev, seen := first[seat]; !seen || i < prev {
					first[seat] = i
				}
			}
		}
	}
	order := []int{0, 1, 2, 3}
	slices.SortFunc(order, func(a, b int) int { return first[a] - first[b] })
	return order
}

func TestClockwise_NewGameMirrorsFlagAndOpensBesideDealer(t *testing.T) {
	firstBidder := map[bool]map[int]int{
		false: {0: 1, 1: 2, 2: 3, 3: 0},
		true:  {0: 3, 1: 0, 2: 1, 3: 2},
	}

	for _, d := range directions {
		for _, variant := range []game.Variant{game.VariantBitola, game.VariantCroatia} {
			t.Run(d.name+"/"+string(variant), func(t *testing.T) {
				assert.False(t, game.RulesFor(variant).Clockwise, "the presets play counter-clockwise")

				for range 64 {
					gs := game.NewGame([4]uint{10, 20, 30, 40}, [4]string{"a", "b", "c", "d"},
						[4]bool{}, variant, "1001", 1, true, false, true, d.clockwise)

					assert.Equal(t, d.clockwise, gs.Rules.Clockwise)
					assert.Equal(t, d.clockwise, gs.Clockwise)
					assert.Equal(t, firstBidder[d.clockwise][gs.DealerSeat], gs.ActivePlayerSeat,
						"dealer %d", gs.DealerSeat)
				}

				gs := game.NewGame([4]uint{10, 20, 30, 40}, [4]string{"a", "b", "c", "d"},
					[4]bool{}, variant, "1001", 1, true, false, true, d.clockwise)
				data, err := json.Marshal(gs)
				require.NoError(t, err)
				var raw map[string]any
				require.NoError(t, json.Unmarshal(data, &raw))
				assert.Equal(t, d.clockwise, raw["clockwise"])
			})
		}
	}
}

func TestClockwise_BitolaBiddingOrder(t *testing.T) {
	tests := []struct {
		direction
		roundOne        []int
		roundTwo        []int
		reshuffleDealer int
		reshuffleOpener int
	}{
		{directions[0], []int{1, 2, 3, 0}, []int{1, 2, 3, 0}, 1, 2},
		{directions[1], []int{3, 2, 1, 0}, []int{3, 2, 1, 0}, 3, 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := inDirection(testfixtures.NewGameJustDealt(), tc.direction)
			require.Equal(t, 0, state.DealerSeat)

			state, roundOne := passSeats(t, state, 4)
			assert.Equal(t, tc.roundOne, roundOne)
			assert.Equal(t, 2, state.BiddingRound)

			state, roundTwo := passSeats(t, state, 4)
			assert.Equal(t, tc.roundTwo, roundTwo)

			assert.Equal(t, game.PhaseDealing, state.Phase, "eight passes reshuffle")
			assert.Equal(t, 1, state.BiddingRound)
			assert.Equal(t, tc.reshuffleDealer, state.DealerSeat)
			assert.Equal(t, tc.reshuffleOpener, state.ActivePlayerSeat)
			assert.Equal(t, tc.clockwise, state.Clockwise)
		})
	}
}

func TestClockwise_BitolaSecondDealAndTrickOneLeader(t *testing.T) {
	tests := []struct {
		direction
		roundTwoOpener int
		dealOrder      []int
	}{
		{directions[0], 1, []int{1, 2, 3, 0}},
		{directions[1], 3, []int{3, 2, 1, 0}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state, _ := passSeats(t, inDirection(testfixtures.NewGameJustDealt(), tc.direction), 4)
			require.Equal(t, tc.roundTwoOpener, state.ActivePlayerSeat)

			suit := game.SuitSpades
			picked := applyOK(t, state, game.Action{
				Type: game.ActionPickTrump, PlayerSeat: tc.roundTwoOpener, Suit: &suit,
			})

			assert.Equal(t, tc.dealOrder, reserveDealOrder(state, picked))
			assert.Equal(t, game.PhasePlaying, picked.Phase)
			assert.Equal(t, tc.dealOrder[0], picked.ActivePlayerSeat, "the seat after the dealer leads trick 1")
		})
	}
}

func TestClockwise_CroatianDealerMustPickAfterThreePasses(t *testing.T) {
	tests := []struct {
		direction
		passes []int
	}{
		{directions[0], []int{1, 2, 3}},
		{directions[1], []int{3, 2, 1}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state, passes := passSeats(t, inDirection(testfixtures.NewGameCroatianJustDealt(), tc.direction), 3)

			assert.Equal(t, tc.passes, passes)
			assert.Equal(t, 0, state.ActivePlayerSeat, "the dealer speaks last")
			assert.True(t, state.MustPickTrump)
			_, err := game.ApplyAction(state, game.Action{Type: game.ActionPassTrump, PlayerSeat: 0})
			assert.ErrorIs(t, err, apperr.ErrMustPickTrump)
		})
	}
}

func TestClockwise_TrickOnePlayOrder(t *testing.T) {
	tests := []struct {
		direction
		order []int
	}{
		{directions[0], []int{1, 2, 3, 0}},
		{directions[1], []int{3, 2, 1, 0}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := inDirection(testfixtures.NewGameFirstTrick(game.SuitHearts), tc.direction)
			require.Equal(t, tc.order[0], state.ActivePlayerSeat)

			_, order := playTrickInTurn(t, state)

			assert.Equal(t, tc.order, order)
		})
	}
}

func TestClockwise_TurnAdvancesAfterBelote(t *testing.T) {
	tests := []struct {
		direction
		next int
	}{
		{directions[0], 1},
		{directions[1], 3},
	}

	for _, action := range []string{game.ActionAnnounceBelot, game.ActionSkipBelot} {
		for _, tc := range tests {
			t.Run(tc.name+"/"+action, func(t *testing.T) {
				state := inDirection(testfixtures.NewGameFirstTrick(game.SuitHearts), tc.direction)
				state.ActivePlayerSeat = 0
				state.DeclarationsResolved = true

				king := game.Card{Rank: game.RankKing, Suit: game.SuitHearts}
				state = applyOK(t, state, game.Action{Type: game.ActionPlayCard, PlayerSeat: 0, Card: &king})
				require.NotNil(t, state.PendingBelotSeat)

				state = applyOK(t, state, game.Action{Type: action, PlayerSeat: 0})

				assert.Equal(t, tc.next, state.ActivePlayerSeat)
			})
		}
	}
}

func TestClockwise_NextHandDealer(t *testing.T) {
	tests := []struct {
		direction
		lastTrick  []int
		nextDealer int
		nextOpener int
	}{
		{directions[0], []int{0, 1, 2, 3}, 1, 2},
		{directions[1], []int{0, 3, 2, 1}, 3, 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state, order := playTrickInTurn(t, inDirection(testfixtures.NewGameLastTrick(), tc.direction))
			assert.Equal(t, tc.lastTrick, order)
			require.Equal(t, game.PhaseHandComplete, state.Phase)

			for seat := range 4 {
				state = applyOK(t, state, game.Action{Type: game.ActionContinue, PlayerSeat: seat})
			}

			assert.Equal(t, 2, state.HandNumber)
			assert.Equal(t, tc.nextDealer, state.DealerSeat)
			assert.Equal(t, tc.nextOpener, state.ActivePlayerSeat)
		})
	}
}

func TestClockwise_DeclarationFullTieGoesToTheEarlierSeat(t *testing.T) {
	hands := [4][]game.Card{
		makeCards("TS", "QS", "AS", "KH", "7C", "9C", "JC", "KC"),
		makeCards("7S", "8S", "9S", "JS", "KS", "7H", "9H", "JH"),
		makeCards("7D", "8D", "9D", "JD", "KD", "8H", "TH", "QH"),
		makeCards("AH", "TD", "QD", "AD", "8C", "TC", "QC", "AC"),
	}
	tests := []struct {
		direction
		leader int
		winner int
	}{
		{directions[0], 1, game.TeamB},
		{directions[1], 3, game.TeamA},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := inDirection(testfixtures.NewGameCroatianJustDealt(), tc.direction)
			for seat, hand := range hands {
				gs.Players[seat].Hand = append([]game.Card(nil), hand[:6]...)
				gs.Players[seat].FaceDownCards = append([]game.Card(nil), hand[6:]...)
			}
			trump := game.SuitClubs
			state := applyOK(t, gs, game.Action{Type: game.ActionPickTrump, PlayerSeat: gs.ActivePlayerSeat, Suit: &trump})
			require.Equal(t, game.PhaseDeclaring, state.Phase)

			state = applyOK(t, state, game.Action{Type: game.ActionDeclare, PlayerSeat: 1})
			state = applyOK(t, state, game.Action{Type: game.ActionDeclare, PlayerSeat: 2})
			require.Equal(t, state.Players[1].Declarations[0].Value, state.Players[2].Declarations[0].Value)
			state = applyOK(t, state, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: 0})
			state = applyOK(t, state, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: 3})

			require.Equal(t, game.PhasePlaying, state.Phase)
			assert.Equal(t, tc.leader, state.ActivePlayerSeat, "the seat after the dealer leads trick 1")
			assert.Equal(t, 20, state.DeclarationPoints[tc.winner])
			assert.Equal(t, 0, state.DeclarationPoints[1-tc.winner])
		})
	}
}
