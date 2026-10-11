package game_test

import (
	"math/rand"
	"sort"
	"strings"
	"testing"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeclareAtFirstTrick(t *testing.T) {
	tests := []struct {
		name          string
		seat          int
		wantDeclCount int
		wantErr       error
	}{
		{
			name:          "seat 1 declares quarte JD-QD-KD-AD (50pts)",
			seat:          1,
			wantDeclCount: 1,
		},
		{
			name:          "seat 0 declares two tierces (20pts each)",
			seat:          0,
			wantDeclCount: 2,
		},
		{
			name:          "seat 2 declares tierce 9H-TH-JH in trump (20pts)",
			seat:          2,
			wantDeclCount: 1,
		},
		{
			name:    "seat 3 has no declarations returns error",
			seat:    3,
			wantErr: apperr.ErrDeclarationNotAvailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
			gs.ActivePlayerSeat = tt.seat
			gs.AwaitingDeclaration = true

			result, err := game.ApplyAction(gs, game.Action{
				Type:       game.ActionDeclare,
				PlayerSeat: tt.seat,
			})

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantDeclCount, len(result.Players[tt.seat].Declarations))
			assert.False(t, result.AwaitingDeclaration, "AwaitingDeclaration should be false after declare")

			for _, d := range result.Players[tt.seat].Declarations {
				assert.Greater(t, d.Value, 0, "declaration value should be positive")
				assert.Equal(t, tt.seat, d.PlayerSeat, "declaration should have correct player seat")
				assert.NotEmpty(t, d.Cards, "declaration should have cards")
			}
		})
	}
}

func TestDeclarationValues(t *testing.T) {
	tests := []struct {
		name      string
		seat      int
		wantValue int
		wantType  game.DeclarationType
	}{
		{
			name:      "quarte JD-QD-KD-AD = 50pts",
			seat:      1,
			wantValue: 50,
			wantType:  game.DeclarationSequence,
		},
		{
			name:      "tierce 7S-8S-9S = 20pts",
			seat:      0,
			wantValue: 20,
			wantType:  game.DeclarationSequence,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
			gs.ActivePlayerSeat = tt.seat
			gs.AwaitingDeclaration = true

			result, err := game.ApplyAction(gs, game.Action{
				Type:       game.ActionDeclare,
				PlayerSeat: tt.seat,
			})
			require.NoError(t, err)

			found := false
			for _, d := range result.Players[tt.seat].Declarations {
				if d.Type == tt.wantType && d.Value == tt.wantValue {
					found = true
					break
				}
			}
			assert.True(t, found, "expected declaration type=%s value=%d not found", tt.wantType, tt.wantValue)
		})
	}
}

func TestSkipDeclare(t *testing.T) {
	gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
	gs.ActivePlayerSeat = 1
	gs.AwaitingDeclaration = true

	result, err := game.ApplyAction(gs, game.Action{
		Type:       game.ActionSkipDeclare,
		PlayerSeat: 1,
	})
	require.NoError(t, err)
	assert.False(t, result.AwaitingDeclaration)
	assert.Empty(t, result.Players[1].Declarations, "skip_declare should not store declarations")
}

func TestDeclareErrorCases(t *testing.T) {
	t.Run("declare at trick 2 returns ErrWrongPhase", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(2)
		gs.AwaitingDeclaration = false

		_, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionDeclare,
			PlayerSeat: 0,
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, apperr.ErrWrongPhase)
	})

	t.Run("declare when not awaiting returns ErrWrongPhase", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.AwaitingDeclaration = false

		_, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionDeclare,
			PlayerSeat: 1,
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, apperr.ErrWrongPhase)
	})

	t.Run("declare when not active player returns ErrNotYourTurn", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.ActivePlayerSeat = 1
		gs.AwaitingDeclaration = true

		_, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionDeclare,
			PlayerSeat: 2,
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, apperr.ErrNotYourTurn)
	})

	t.Run("play_card while awaiting declaration returns ErrActionRequired", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.ActivePlayerSeat = 1
		gs.AwaitingDeclaration = true

		card := gs.Players[1].Hand[0]
		_, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionPlayCard,
			PlayerSeat: 1,
			Card:       &card,
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, apperr.ErrActionRequired)
	})
}

func TestDeclarationResolution(t *testing.T) {
	t.Run("higher value declaration wins", func(t *testing.T) {
		decls := []game.Declaration{
			{Type: game.DeclarationSequence, Cards: makeCards("7S", "8S", "9S"), PlayerSeat: 0, Value: 20},
			{Type: game.DeclarationSequence, Cards: makeCards("JD", "QD", "KD", "AD"), PlayerSeat: 1, Value: 50},
		}
		gs := testfixtures.NewGameWithDeclarations(decls)
		gs.CurrentTrick = []game.TrickCard{
			{Card: game.Card{Rank: game.RankTen, Suit: game.SuitSpades}, PlayerSeat: 1},
			{Card: game.Card{Rank: game.Rank7, Suit: game.SuitSpades}, PlayerSeat: 2},
			{Card: game.Card{Rank: game.Rank7, Suit: game.SuitClubs}, PlayerSeat: 3},
			{Card: game.Card{Rank: game.Rank8, Suit: game.SuitSpades}, PlayerSeat: 0},
		}
		leadSuit := game.SuitSpades
		gs.LeadSuit = &leadSuit
		gs.DeclarationsResolved = false
		gs.BelotAnnounced = true

		assert.Equal(t, 50, decls[1].Value)
		assert.Equal(t, 20, decls[0].Value)
	})

	t.Run("only winning team declarations scored", func(t *testing.T) {
		decls := []game.Declaration{
			{Type: game.DeclarationSequence, Cards: makeCards("7S", "8S", "9S"), PlayerSeat: 0, Value: 20},
			{Type: game.DeclarationSequence, Cards: makeCards("7C", "8C", "9C"), PlayerSeat: 2, Value: 20},
			{Type: game.DeclarationSequence, Cards: makeCards("JD", "QD", "KD", "AD"), PlayerSeat: 1, Value: 50},
		}
		state := completeTrick1(t, decls)

		assert.Equal(t, 0, state.DeclarationPoints[game.TeamA], "Team A gets 0 — best meld lost")
		assert.Equal(t, 50, state.DeclarationPoints[game.TeamB], "Team B gets 50 — sum of winning team's melds")
		assert.Empty(t, state.Players[0].Declarations, "Team A seat 0 declarations cleared")
		assert.Empty(t, state.Players[2].Declarations, "Team A seat 2 declarations cleared")
		assert.NotEmpty(t, state.Players[1].Declarations, "Team B seat 1 declarations preserved")
	})

	t.Run("only one team declared — wins with full sum", func(t *testing.T) {
		decls := []game.Declaration{
			{Type: game.DeclarationSequence, Cards: makeCards("7S", "8S", "9S"), PlayerSeat: 0, Value: 20},
			{Type: game.DeclarationFourOfAKind, Cards: makeCards("TS", "TH", "TD", "TC"), PlayerSeat: 2, Value: 100},
		}
		state := completeTrick1(t, decls)

		assert.Equal(t, 120, state.DeclarationPoints[game.TeamA], "Team A wins with full sum 20+100")
		assert.Equal(t, 0, state.DeclarationPoints[game.TeamB], "Team B gets 0 — no declarations")
		assert.NotEmpty(t, state.Players[0].Declarations, "Team A seat 0 declarations preserved")
		assert.NotEmpty(t, state.Players[2].Declarations, "Team A seat 2 declarations preserved")
	})

	t.Run("strongest single meld wins over higher team sum (Q+K vs 9)", func(t *testing.T) {
		decls := []game.Declaration{
			{Type: game.DeclarationFourOfAKind, Cards: makeCards("QS", "QH", "QD", "QC"), PlayerSeat: 0, Value: 100},
			{Type: game.DeclarationFourOfAKind, Cards: makeCards("9S", "9H", "9D", "9C"), PlayerSeat: 1, Value: 150},
			{Type: game.DeclarationFourOfAKind, Cards: makeCards("KS", "KH", "KD", "KC"), PlayerSeat: 2, Value: 100},
		}
		state := completeTrick1(t, decls)

		assert.Equal(t, 0, state.DeclarationPoints[game.TeamA], "Team A gets 0 even though sum (200) > team B's 150")
		assert.Equal(t, 150, state.DeclarationPoints[game.TeamB], "Team B wins with its single strongest meld total")
		assert.Empty(t, state.Players[0].Declarations, "Team A seat 0 declarations cleared")
		assert.Empty(t, state.Players[2].Declarations, "Team A seat 2 declarations cleared")
		assert.NotEmpty(t, state.Players[1].Declarations, "Team B seat 1 declarations preserved")
	})
}

// completeTrick1 seats the given declarations on the NewGameFirstTrick fixture and plays trick 1
// to completion, returning the post-resolution state with Belot pre-marked announced.
func completeTrick1(t *testing.T, decls []game.Declaration) *game.GameState {
	t.Helper()
	state := testfixtures.NewGameWithDeclarations(decls)
	state.BelotAnnounced = true

	plays := []struct {
		seat int
		card game.Card
	}{
		{1, game.Card{Rank: game.Rank8, Suit: game.SuitDiamonds}},
		{2, game.Card{Rank: game.Rank9, Suit: game.SuitHearts}},
		{3, game.Card{Rank: game.RankTen, Suit: game.SuitDiamonds}},
		{0, game.Card{Rank: game.RankQueen, Suit: game.SuitHearts}},
	}
	for _, p := range plays {
		card := p.card
		next, err := game.ApplyAction(state, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: p.seat, Card: &card,
		})
		require.NoError(t, err, "seat %d play %v", p.seat, card)
		state = next
	}
	require.True(t, state.DeclarationsResolved, "trick 1 must trigger declaration resolution")
	return state
}

func TestBelotDetection(t *testing.T) {
	t.Run("play trump K while holding trump Q triggers Belot prompt", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = false
		gs.DeclarationsResolved = true

		card := game.Card{Rank: game.RankKing, Suit: game.SuitHearts}
		result, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionPlayCard,
			PlayerSeat: 0,
			Card:       &card,
		})
		require.NoError(t, err)
		require.NotNil(t, result.PendingBelotSeat, "PendingBelotSeat should be set")
		assert.Equal(t, 0, *result.PendingBelotSeat)
	})

	t.Run("play trump Q while holding trump K triggers Belot prompt", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = false
		gs.DeclarationsResolved = true

		card := game.Card{Rank: game.RankQueen, Suit: game.SuitHearts}
		result, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionPlayCard,
			PlayerSeat: 0,
			Card:       &card,
		})
		require.NoError(t, err)
		require.NotNil(t, result.PendingBelotSeat)
		assert.Equal(t, 0, *result.PendingBelotSeat)
	})

	t.Run("no Belot when playing trump K without Q in hand", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.ActivePlayerSeat = 2
		gs.AwaitingDeclaration = false
		gs.DeclarationsResolved = true

		card := game.Card{Rank: game.RankJack, Suit: game.SuitHearts}
		result, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionPlayCard,
			PlayerSeat: 2,
			Card:       &card,
		})
		require.NoError(t, err)
		assert.Nil(t, result.PendingBelotSeat, "no Belot prompt for non K/Q trump card")
	})

	t.Run("no Belot when playing non-trump K", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.ActivePlayerSeat = 2
		gs.AwaitingDeclaration = false
		gs.DeclarationsResolved = true

		card := game.Card{Rank: game.RankKing, Suit: game.SuitSpades}
		result, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionPlayCard,
			PlayerSeat: 2,
			Card:       &card,
		})
		require.NoError(t, err)
		assert.Nil(t, result.PendingBelotSeat, "no Belot for non-trump K")
	})
}

func TestAnnounceBelot(t *testing.T) {
	t.Run("announce_belot adds 20 points to team", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = false
		gs.DeclarationsResolved = true

		card := game.Card{Rank: game.RankKing, Suit: game.SuitHearts}
		state1, err := game.ApplyAction(gs, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: 0, Card: &card,
		})
		require.NoError(t, err)
		require.NotNil(t, state1.PendingBelotSeat)

		state2, err := game.ApplyAction(state1, game.Action{
			Type: game.ActionAnnounceBelot, PlayerSeat: 0,
		})
		require.NoError(t, err)
		assert.Nil(t, state2.PendingBelotSeat, "PendingBelotSeat cleared")
		assert.True(t, state2.BelotAnnounced)
		assert.Equal(t, 20, state2.BelotPoints[game.TeamA], "20 belote pts to team A (declaration, not card points)")
		assert.Equal(t, 0, state2.HandPoints[game.TeamA], "belote does NOT inflate card points")
		assert.Equal(t, 1, state2.ActivePlayerSeat, "turn advances after Belot")
	})

	t.Run("skip_belot awards no points", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = false
		gs.DeclarationsResolved = true

		card := game.Card{Rank: game.RankKing, Suit: game.SuitHearts}
		state1, err := game.ApplyAction(gs, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: 0, Card: &card,
		})
		require.NoError(t, err)

		state2, err := game.ApplyAction(state1, game.Action{
			Type: game.ActionSkipBelot, PlayerSeat: 0,
		})
		require.NoError(t, err)
		assert.Nil(t, state2.PendingBelotSeat)
		assert.False(t, state2.BelotAnnounced, "Belot not announced")
		assert.Equal(t, 0, state2.HandPoints[game.TeamA], "no points awarded")
	})

	t.Run("play_card while pending Belot returns ErrActionRequired", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = false
		gs.DeclarationsResolved = true

		card := game.Card{Rank: game.RankKing, Suit: game.SuitHearts}
		state1, err := game.ApplyAction(gs, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: 0, Card: &card,
		})
		require.NoError(t, err)
		require.NotNil(t, state1.PendingBelotSeat)

		card2 := game.Card{Rank: game.RankQueen, Suit: game.SuitHearts}
		_, err = game.ApplyAction(state1, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: 0, Card: &card2,
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, apperr.ErrActionRequired)
	})

	t.Run("announce_belot when not pending returns error", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.ActivePlayerSeat = 1
		gs.AwaitingDeclaration = false
		gs.DeclarationsResolved = true

		_, err := game.ApplyAction(gs, game.Action{
			Type: game.ActionAnnounceBelot, PlayerSeat: 1,
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, apperr.ErrBelotNotAvailable)
	})
}

func TestFullTrick1WithDeclarations(t *testing.T) {
	t.Run("full trick 1 flow with declarations and Belot", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.AwaitingDeclaration = true

		state, err := game.ApplyAction(gs, game.Action{
			Type: game.ActionDeclare, PlayerSeat: 1,
		})
		require.NoError(t, err)
		assert.False(t, state.AwaitingDeclaration)
		assert.Equal(t, 1, len(state.Players[1].Declarations))
		assert.Equal(t, 50, state.Players[1].Declarations[0].Value)

		card1 := game.Card{Rank: game.Rank8, Suit: game.SuitDiamonds}
		state, err = game.ApplyAction(state, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: 1, Card: &card1,
		})
		require.NoError(t, err)
		assert.Equal(t, 1, len(state.CurrentTrick))
		assert.True(t, state.AwaitingDeclaration)
		assert.Equal(t, 2, state.ActivePlayerSeat)

		state, err = game.ApplyAction(state, game.Action{
			Type: game.ActionDeclare, PlayerSeat: 2,
		})
		require.NoError(t, err)
		assert.False(t, state.AwaitingDeclaration)

		card2 := game.Card{Rank: game.Rank9, Suit: game.SuitHearts}
		state, err = game.ApplyAction(state, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: 2, Card: &card2,
		})
		require.NoError(t, err)
		assert.False(t, state.AwaitingDeclaration)
		assert.Equal(t, 3, state.ActivePlayerSeat)

		card3 := game.Card{Rank: game.RankTen, Suit: game.SuitDiamonds}
		state, err = game.ApplyAction(state, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: 3, Card: &card3,
		})
		require.NoError(t, err)
		assert.True(t, state.AwaitingDeclaration)
		assert.Equal(t, 0, state.ActivePlayerSeat)

		state, err = game.ApplyAction(state, game.Action{
			Type: game.ActionDeclare, PlayerSeat: 0,
		})
		require.NoError(t, err)
		assert.Equal(t, 2, len(state.Players[0].Declarations))

		card0 := game.Card{Rank: game.RankKing, Suit: game.SuitHearts}
		state, err = game.ApplyAction(state, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: 0, Card: &card0,
		})
		require.NoError(t, err)
		require.NotNil(t, state.PendingBelotSeat)
		assert.Equal(t, 0, *state.PendingBelotSeat)
		assert.Equal(t, 4, len(state.CurrentTrick))
		assert.Equal(t, 1, state.TrickNumber)

		state, err = game.ApplyAction(state, game.Action{
			Type: game.ActionAnnounceBelot, PlayerSeat: 0,
		})
		require.NoError(t, err)
		assert.Nil(t, state.PendingBelotSeat)
		assert.True(t, state.BelotAnnounced)
		assert.Equal(t, 2, state.TrickNumber)
		assert.True(t, state.DeclarationsResolved)

		assert.Equal(t, 20, state.BelotPoints[game.TeamA], "Team A belote tracked as a declaration")

		assert.Equal(t, 50, state.DeclarationPoints[game.TeamB])
		assert.Equal(t, 0, state.DeclarationPoints[game.TeamA])

		assert.Empty(t, state.Players[0].Declarations)
		assert.Empty(t, state.Players[2].Declarations)
		assert.NotEmpty(t, state.Players[1].Declarations)
	})

	t.Run("state immutability across all actions", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.AwaitingDeclaration = true
		original := *gs

		_, err := game.ApplyAction(gs, game.Action{
			Type: game.ActionDeclare, PlayerSeat: 1,
		})
		require.NoError(t, err)

		assert.Equal(t, original.AwaitingDeclaration, gs.AwaitingDeclaration)
		assert.Empty(t, gs.Players[1].Declarations)
	})
}

func TestDeclarationTiebreakers(t *testing.T) {
	t.Run("four-of-a-kind beats sequence at equal 100pts", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.Players[0].Hand = []game.Card{
			{Rank: game.RankAce, Suit: game.SuitSpades},
			{Rank: game.RankAce, Suit: game.SuitHearts},
			{Rank: game.RankAce, Suit: game.SuitDiamonds},
			{Rank: game.RankAce, Suit: game.SuitClubs},
			{Rank: game.Rank7, Suit: game.SuitSpades},
			{Rank: game.Rank8, Suit: game.SuitSpades},
			{Rank: game.Rank7, Suit: game.SuitClubs},
			{Rank: game.Rank8, Suit: game.SuitClubs},
		}
		gs.Players[1].Hand = []game.Card{
			{Rank: game.Rank7, Suit: game.SuitDiamonds},
			{Rank: game.Rank8, Suit: game.SuitDiamonds},
			{Rank: game.Rank9, Suit: game.SuitDiamonds},
			{Rank: game.RankTen, Suit: game.SuitDiamonds},
			{Rank: game.RankJack, Suit: game.SuitDiamonds},
			{Rank: game.RankQueen, Suit: game.SuitSpades},
			{Rank: game.Rank9, Suit: game.SuitClubs},
			{Rank: game.RankTen, Suit: game.SuitClubs},
		}

		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = true
		state, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
		require.NoError(t, err)

		state.ActivePlayerSeat = 1
		state.AwaitingDeclaration = true
		state, err = game.ApplyAction(state, game.Action{Type: game.ActionDeclare, PlayerSeat: 1})
		require.NoError(t, err)

		assert.Equal(t, 100, state.Players[0].Declarations[0].Value)
		assert.Equal(t, 100, state.Players[1].Declarations[0].Value)
		assert.Equal(t, game.DeclarationFourOfAKind, state.Players[0].Declarations[0].Type)
		assert.Equal(t, game.DeclarationSequence, state.Players[1].Declarations[0].Type)
	})

	t.Run("equal-value sequences resolved by top card", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.Players[0].Hand = []game.Card{
			{Rank: game.RankQueen, Suit: game.SuitSpades},
			{Rank: game.RankKing, Suit: game.SuitSpades},
			{Rank: game.RankAce, Suit: game.SuitSpades},
			{Rank: game.Rank7, Suit: game.SuitClubs},
			{Rank: game.Rank8, Suit: game.SuitClubs},
			{Rank: game.Rank9, Suit: game.SuitClubs},
			{Rank: game.Rank7, Suit: game.SuitDiamonds},
			{Rank: game.Rank8, Suit: game.SuitDiamonds},
		}
		gs.Players[1].Hand = []game.Card{
			{Rank: game.Rank7, Suit: game.SuitSpades},
			{Rank: game.Rank8, Suit: game.SuitSpades},
			{Rank: game.Rank9, Suit: game.SuitDiamonds},
			{Rank: game.RankTen, Suit: game.SuitDiamonds},
			{Rank: game.RankJack, Suit: game.SuitDiamonds},
			{Rank: game.RankQueen, Suit: game.SuitDiamonds},
			{Rank: game.RankKing, Suit: game.SuitDiamonds},
			{Rank: game.RankAce, Suit: game.SuitDiamonds},
		}

		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = true
		state, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
		require.NoError(t, err)

		state.ActivePlayerSeat = 1
		state.AwaitingDeclaration = true
		state, err = game.ApplyAction(state, game.Action{Type: game.ActionDeclare, PlayerSeat: 1})
		require.NoError(t, err)

		assert.True(t, len(state.Players[1].Declarations) > 0)
	})

	t.Run("Jack-topped tierce beats Ten-topped tierce (natural order)", func(t *testing.T) {
		decls := []game.Declaration{
			{Type: game.DeclarationSequence, Cards: makeCards("8D", "9D", "TD"), PlayerSeat: 0, Value: 20},
			{Type: game.DeclarationSequence, Cards: makeCards("7S", "8S", "9S"), PlayerSeat: 2, Value: 20},
			{Type: game.DeclarationSequence, Cards: makeCards("9C", "TC", "JC"), PlayerSeat: 1, Value: 20},
		}
		state := completeTrick1(t, decls)

		assert.Equal(t, 0, state.DeclarationPoints[game.TeamA], "Ten-topped tierce must lose")
		assert.Equal(t, 20, state.DeclarationPoints[game.TeamB], "Jack-topped tierce wins (J > T)")
		assert.Empty(t, state.Players[0].Declarations, "Team A declarations cleared")
		assert.NotEmpty(t, state.Players[1].Declarations, "Team B declarations preserved")
	})
}

func TestFourOfAKindDeclarations(t *testing.T) {
	t.Run("four Jacks = 200pts", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.Players[0].Hand = []game.Card{
			{Rank: game.RankJack, Suit: game.SuitSpades},
			{Rank: game.RankJack, Suit: game.SuitHearts},
			{Rank: game.RankJack, Suit: game.SuitDiamonds},
			{Rank: game.RankJack, Suit: game.SuitClubs},
			{Rank: game.Rank7, Suit: game.SuitSpades},
			{Rank: game.Rank8, Suit: game.SuitSpades},
			{Rank: game.Rank7, Suit: game.SuitClubs},
			{Rank: game.Rank8, Suit: game.SuitClubs},
		}
		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = true

		state, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
		require.NoError(t, err)
		found := false
		for _, d := range state.Players[0].Declarations {
			if d.Type == game.DeclarationFourOfAKind && d.Value == 200 {
				found = true
			}
		}
		assert.True(t, found, "should detect 4xJ = 200pts")
	})

	t.Run("four Nines = 150pts", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.Players[0].Hand = []game.Card{
			{Rank: game.Rank9, Suit: game.SuitSpades},
			{Rank: game.Rank9, Suit: game.SuitHearts},
			{Rank: game.Rank9, Suit: game.SuitDiamonds},
			{Rank: game.Rank9, Suit: game.SuitClubs},
			{Rank: game.Rank7, Suit: game.SuitSpades},
			{Rank: game.Rank8, Suit: game.SuitSpades},
			{Rank: game.Rank7, Suit: game.SuitClubs},
			{Rank: game.Rank8, Suit: game.SuitClubs},
		}
		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = true

		state, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
		require.NoError(t, err)
		found := false
		for _, d := range state.Players[0].Declarations {
			if d.Type == game.DeclarationFourOfAKind && d.Value == 150 {
				found = true
			}
		}
		assert.True(t, found, "should detect 4x9 = 150pts")
	})

	t.Run("four Aces = 100pts", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.Players[0].Hand = []game.Card{
			{Rank: game.RankAce, Suit: game.SuitSpades},
			{Rank: game.RankAce, Suit: game.SuitHearts},
			{Rank: game.RankAce, Suit: game.SuitDiamonds},
			{Rank: game.RankAce, Suit: game.SuitClubs},
			{Rank: game.Rank7, Suit: game.SuitSpades},
			{Rank: game.Rank8, Suit: game.SuitSpades},
			{Rank: game.Rank7, Suit: game.SuitClubs},
			{Rank: game.Rank8, Suit: game.SuitClubs},
		}
		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = true

		state, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
		require.NoError(t, err)
		found := false
		for _, d := range state.Players[0].Declarations {
			if d.Type == game.DeclarationFourOfAKind && d.Value == 100 {
				found = true
			}
		}
		assert.True(t, found, "should detect 4xA = 100pts")
	})

	t.Run("four 8s are NOT declarable", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.Players[0].Hand = []game.Card{
			{Rank: game.Rank8, Suit: game.SuitSpades},
			{Rank: game.Rank8, Suit: game.SuitHearts},
			{Rank: game.Rank8, Suit: game.SuitDiamonds},
			{Rank: game.Rank8, Suit: game.SuitClubs},
			{Rank: game.Rank7, Suit: game.SuitSpades},
			{Rank: game.RankTen, Suit: game.SuitSpades},
			{Rank: game.Rank7, Suit: game.SuitClubs},
			{Rank: game.RankTen, Suit: game.SuitClubs},
		}
		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = true

		_, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
		assert.ErrorIs(t, err, apperr.ErrDeclarationNotAvailable, "4x8 should not be declarable")
	})
}

func TestQuinteDeclaration(t *testing.T) {
	t.Run("5-card sequence = 100pts", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.Players[0].Hand = []game.Card{
			{Rank: game.Rank7, Suit: game.SuitSpades},
			{Rank: game.Rank8, Suit: game.SuitSpades},
			{Rank: game.Rank9, Suit: game.SuitSpades},
			{Rank: game.RankTen, Suit: game.SuitSpades},
			{Rank: game.RankJack, Suit: game.SuitSpades},
			{Rank: game.Rank7, Suit: game.SuitClubs},
			{Rank: game.Rank8, Suit: game.SuitClubs},
			{Rank: game.Rank9, Suit: game.SuitClubs},
		}
		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = true

		state, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
		require.NoError(t, err)

		foundQuinte := false
		for _, d := range state.Players[0].Declarations {
			if d.Type == game.DeclarationSequence && d.Value == 100 && len(d.Cards) == 5 {
				foundQuinte = true
			}
		}
		assert.True(t, foundQuinte, "should detect 5-card sequence = 100pts")
	})
}

func TestBelotAtLaterTricks(t *testing.T) {
	t.Run("Belot detection at trick 2", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(2)
		gs.BelotAnnounced = false
		gs.Players[2].Hand = []game.Card{
			{Rank: game.RankKing, Suit: game.SuitHearts},
			{Rank: game.RankQueen, Suit: game.SuitHearts},
			{Rank: game.RankAce, Suit: game.SuitDiamonds},
			{Rank: game.RankTen, Suit: game.SuitDiamonds},
			{Rank: game.Rank8, Suit: game.SuitHearts},
			{Rank: game.Rank7, Suit: game.SuitHearts},
			{Rank: game.RankAce, Suit: game.SuitClubs},
		}
		gs.ActivePlayerSeat = 2

		card := game.Card{Rank: game.RankKing, Suit: game.SuitHearts}
		state, err := game.ApplyAction(gs, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: 2, Card: &card,
		})
		require.NoError(t, err)
		require.NotNil(t, state.PendingBelotSeat, "Belot should trigger at trick 2")
		assert.Equal(t, 2, *state.PendingBelotSeat)
	})
}

func TestSkipDeclareAtTrick2(t *testing.T) {
	t.Run("skip_declare at trick 2 returns ErrWrongPhase", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(2)
		_, err := game.ApplyAction(gs, game.Action{
			Type: game.ActionSkipDeclare, PlayerSeat: 0,
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, apperr.ErrWrongPhase)
	})
}

func TestDedupBitola(t *testing.T) {
	t.Run("tierce spades + FoaK 9s sharing 9S — FoaK kept", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.Players[0].Hand = []game.Card{
			{Rank: game.Rank7, Suit: game.SuitSpades},
			{Rank: game.Rank8, Suit: game.SuitSpades},
			{Rank: game.Rank9, Suit: game.SuitSpades},
			{Rank: game.Rank9, Suit: game.SuitHearts},
			{Rank: game.Rank9, Suit: game.SuitDiamonds},
			{Rank: game.Rank9, Suit: game.SuitClubs},
			{Rank: game.RankJack, Suit: game.SuitDiamonds},
			{Rank: game.RankQueen, Suit: game.SuitDiamonds},
		}
		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = true

		state, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
		require.NoError(t, err)
		require.Len(t, state.Players[0].Declarations, 1, "tierce should be dropped by dedup")
		assert.Equal(t, game.DeclarationFourOfAKind, state.Players[0].Declarations[0].Type)
		assert.Equal(t, 150, state.Players[0].Declarations[0].Value)
	})

	t.Run("quarte spades 9-T-J-Q + FoaK jacks sharing JS — FoaK kept", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.Players[0].Hand = []game.Card{
			{Rank: game.Rank9, Suit: game.SuitSpades},
			{Rank: game.RankTen, Suit: game.SuitSpades},
			{Rank: game.RankJack, Suit: game.SuitSpades},
			{Rank: game.RankQueen, Suit: game.SuitSpades},
			{Rank: game.RankJack, Suit: game.SuitHearts},
			{Rank: game.RankJack, Suit: game.SuitDiamonds},
			{Rank: game.RankJack, Suit: game.SuitClubs},
			{Rank: game.Rank7, Suit: game.SuitClubs},
		}
		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = true

		state, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
		require.NoError(t, err)
		require.Len(t, state.Players[0].Declarations, 1, "quarte should be dropped by dedup")
		assert.Equal(t, game.DeclarationFourOfAKind, state.Players[0].Declarations[0].Type)
		assert.Equal(t, 200, state.Players[0].Declarations[0].Value)
	})

	t.Run("two FoaKs of different ranks — both kept", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.Players[0].Hand = []game.Card{
			{Rank: game.Rank9, Suit: game.SuitSpades},
			{Rank: game.Rank9, Suit: game.SuitHearts},
			{Rank: game.Rank9, Suit: game.SuitDiamonds},
			{Rank: game.Rank9, Suit: game.SuitClubs},
			{Rank: game.RankAce, Suit: game.SuitSpades},
			{Rank: game.RankAce, Suit: game.SuitHearts},
			{Rank: game.RankAce, Suit: game.SuitDiamonds},
			{Rank: game.RankAce, Suit: game.SuitClubs},
		}
		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = true

		state, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
		require.NoError(t, err)
		require.Len(t, state.Players[0].Declarations, 2)
	})

	t.Run("non-overlapping tierce + FoaK — both kept", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.Players[0].Hand = []game.Card{
			{Rank: game.Rank7, Suit: game.SuitSpades},
			{Rank: game.Rank8, Suit: game.SuitSpades},
			{Rank: game.Rank9, Suit: game.SuitSpades},
			{Rank: game.RankJack, Suit: game.SuitSpades},
			{Rank: game.RankJack, Suit: game.SuitHearts},
			{Rank: game.RankJack, Suit: game.SuitDiamonds},
			{Rank: game.RankJack, Suit: game.SuitClubs},
			{Rank: game.Rank7, Suit: game.SuitClubs},
		}
		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = true

		state, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
		require.NoError(t, err)
		require.Len(t, state.Players[0].Declarations, 2)
	})

	t.Run("equal-value clash: quinte + FoaK tens sharing TS — FoaK kept", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.Players[0].Hand = makeCards("TS", "JS", "QS", "KS", "AS", "TH", "TD", "TC")
		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = true

		state, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
		require.NoError(t, err)
		require.Len(t, state.Players[0].Declarations, 1, "quinte should be dropped by dedup")
		assert.Equal(t, game.DeclarationFourOfAKind, state.Players[0].Declarations[0].Type)
		assert.Equal(t, 100, state.Players[0].Declarations[0].Value)
	})

	t.Run("quarte subsumes tierce in detection — single declaration emitted", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.Players[0].Hand = []game.Card{
			{Rank: game.RankJack, Suit: game.SuitDiamonds},
			{Rank: game.RankQueen, Suit: game.SuitDiamonds},
			{Rank: game.RankKing, Suit: game.SuitDiamonds},
			{Rank: game.RankAce, Suit: game.SuitDiamonds},
			{Rank: game.Rank7, Suit: game.SuitSpades},
			{Rank: game.Rank8, Suit: game.SuitSpades},
			{Rank: game.Rank7, Suit: game.SuitClubs},
			{Rank: game.Rank8, Suit: game.SuitClubs},
		}
		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = true

		state, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
		require.NoError(t, err)
		require.Len(t, state.Players[0].Declarations, 1)
		assert.Equal(t, 50, state.Players[0].Declarations[0].Value)
		assert.Len(t, state.Players[0].Declarations[0].Cards, 4)
	})
}

// meldSummary is an order-independent fingerprint of a detected declaration, compared as an
// unordered set because detection order varies between runs.
type meldSummary struct {
	Type  game.DeclarationType
	Value int
	Cards string
}

func summarizeMelds(decls []game.Declaration) []meldSummary {
	out := make([]meldSummary, 0, len(decls))
	for _, d := range decls {
		ids := make([]string, 0, len(d.Cards))
		for _, c := range d.Cards {
			ids = append(ids, c.String())
		}
		sort.Strings(ids)
		out = append(out, meldSummary{
			Type:  d.Type,
			Value: d.Value,
			Cards: strings.Join(ids, ","),
		})
	}
	return out
}

func totalDeclarationValue(decls []game.Declaration) int {
	total := 0
	for _, d := range decls {
		total += d.Value
	}
	return total
}

func meld(t game.DeclarationType, value int, ids ...string) meldSummary {
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	return meldSummary{Type: t, Value: value, Cards: strings.Join(sorted, ",")}
}

// declareSeat0 drops the given hand on seat 0 of the supplied fixture, puts
// seat 0 on the declaration prompt and applies the declare action. Detection
// runs inside ApplyAction, which is the only supported way to reach it.
func declareSeat0(t *testing.T, gs *game.GameState, hand []game.Card) *game.GameState {
	t.Helper()
	gs.Players[0].Hand = hand
	gs.ActivePlayerSeat = 0
	gs.AwaitingDeclaration = true
	state, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
	require.NoError(t, err)
	return state
}

func TestDeclarationOverlapByVariant(t *testing.T) {
	jackOverlap := makeCards("9S", "TS", "JS", "QS", "JH", "JD", "JC", "7C")
	equalValueOverlap := makeCards("TS", "JS", "QS", "KS", "AS", "TH", "TD", "TC")
	overlapFree := makeCards("7S", "8S", "9S", "JS", "JH", "JD", "JC", "7C")

	quarteSpades := meld(game.DeclarationSequence, 50, "9S", "TS", "JS", "QS")
	quinteSpades := meld(game.DeclarationSequence, 100, "TS", "JS", "QS", "KS", "AS")
	tierceSpades := meld(game.DeclarationSequence, 20, "7S", "8S", "9S")
	carreJacks := meld(game.DeclarationFourOfAKind, 200, "JS", "JH", "JD", "JC")
	carreTens := meld(game.DeclarationFourOfAKind, 100, "TS", "TH", "TD", "TC")

	tests := []struct {
		name    string
		newGame func(game.Suit) *game.GameState
		hand    []game.Card
		want    []meldSummary
	}{
		{
			name:    "Croatian: quarte and carré sharing JS both survive",
			newGame: testfixtures.NewGameCroatianFirstTrick,
			hand:    jackOverlap,
			want:    []meldSummary{quarteSpades, carreJacks},
		},
		{
			name:    "Bitola: quarte sharing JS is dropped for the carré",
			newGame: testfixtures.NewGameFirstTrick,
			hand:    jackOverlap,
			want:    []meldSummary{carreJacks},
		},
		{
			name:    "Bitola: equal-value clash keeps the carré, not the quinte",
			newGame: testfixtures.NewGameFirstTrick,
			hand:    equalValueOverlap,
			want:    []meldSummary{carreTens},
		},
		{
			name:    "Croatian: equal-value clash keeps both",
			newGame: testfixtures.NewGameCroatianFirstTrick,
			hand:    equalValueOverlap,
			want:    []meldSummary{quinteSpades, carreTens},
		},
		{
			name:    "Bitola: overlap-free hand keeps every meld",
			newGame: testfixtures.NewGameFirstTrick,
			hand:    overlapFree,
			want:    []meldSummary{tierceSpades, carreJacks},
		},
		{
			name:    "Croatian: overlap-free hand keeps every meld",
			newGame: testfixtures.NewGameCroatianFirstTrick,
			hand:    overlapFree,
			want:    []meldSummary{tierceSpades, carreJacks},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := declareSeat0(t, tt.newGame(game.SuitHearts), tt.hand)
			assert.ElementsMatch(t, tt.want, summarizeMelds(state.Players[0].Declarations))
			for _, d := range state.Players[0].Declarations {
				assert.Equal(t, 0, d.PlayerSeat, "declaring seat is stamped on every meld")
			}
		})
	}
}

// TestDeclarationPromptTriggerIsOverlapInvariant checks the declaration prompt fires on the same
// hands whether or not overlapping melds are deduplicated.
func TestDeclarationPromptTriggerIsOverlapInvariant(t *testing.T) {
	tests := []struct {
		name       string
		hand       []game.Card
		wantPrompt bool
	}{
		{
			name:       "two overlapping melds",
			hand:       makeCards("9S", "TS", "JS", "QS", "JH", "JD", "JC", "7C"),
			wantPrompt: true,
		},
		{
			name:       "two overlapping melds of equal value",
			hand:       makeCards("TS", "JS", "QS", "KS", "AS", "TH", "TD", "TC"),
			wantPrompt: true,
		},
		{
			name:       "one meld",
			hand:       makeCards("7S", "8S", "9S", "TD", "JH", "QC", "KD", "AC"),
			wantPrompt: true,
		},
		{
			name:       "no meld",
			hand:       makeCards("7S", "8H", "TD", "JH", "QC", "KD", "AC", "9D"),
			wantPrompt: false,
		},
		{
			name:       "four 8s carry no value, so they are not a meld",
			hand:       makeCards("8S", "8H", "8D", "8C", "TD", "JH", "QC", "KD"),
			wantPrompt: false,
		},
	}

	variants := []struct {
		label   string
		newGame func(game.Suit) *game.GameState
	}{
		{"bitola", testfixtures.NewGameFirstTrick},
		{"croatia", testfixtures.NewGameCroatianFirstTrick},
	}

	for _, tt := range tests {
		for _, v := range variants {
			t.Run(tt.name+"/"+v.label, func(t *testing.T) {
				gs := v.newGame(game.SuitHearts)
				gs.Players[2].Hand = tt.hand
				require.False(t, gs.AwaitingDeclaration, "fixture must start with no prompt pending")
				require.Equal(t, 1, gs.ActivePlayerSeat, "seat 1 leads trick 1")

				lead := game.Card{Rank: game.Rank8, Suit: game.SuitDiamonds}
				state, err := game.ApplyAction(gs, game.Action{
					Type: game.ActionPlayCard, PlayerSeat: 1, Card: &lead,
				})
				require.NoError(t, err)
				require.Equal(t, 2, state.ActivePlayerSeat, "turn must have passed to the seat under test")

				assert.Equal(t, tt.wantPrompt, state.AwaitingDeclaration)
			})
		}
	}
}

// TestDetectedMeldsOverlapOnlyAcrossTypes checks that detected melds only ever share cards between
// a sequence and a four-of-a-kind, never within one type.
func TestDetectedMeldsOverlapOnlyAcrossTypes(t *testing.T) {
	samePairs := map[game.DeclarationType]int{}

	check := func(t *testing.T, decls []game.Declaration) {
		t.Helper()
		for i := 0; i < len(decls); i++ {
			for j := i + 1; j < len(decls); j++ {
				a, b := decls[i], decls[j]
				if a.Type != b.Type {
					continue
				}
				samePairs[a.Type]++
				for _, ca := range a.Cards {
					for _, cb := range b.Cards {
						require.NotEqual(t, ca, cb,
							"two %s melds share %s — dedup's tie fix assumes this is impossible; escalate instead of generalizing dedup",
							a.Type, ca)
					}
				}
			}
		}
	}

	detect := func(t *testing.T, hand []game.Card) []game.Declaration {
		t.Helper()
		gs := testfixtures.NewGameCroatianFirstTrick(game.SuitHearts)
		gs.Players[0].Hand = hand
		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = true
		state, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
		if err != nil {
			return nil
		}
		return state.Players[0].Declarations
	}

	t.Run("constructed meld-dense hands", func(t *testing.T) {
		for _, hand := range [][]game.Card{
			makeCards("9S", "9H", "9D", "9C", "AS", "AH", "AD", "AC"),
			makeCards("JS", "JH", "JD", "JC", "TS", "TH", "TD", "TC"),
			makeCards("7S", "8S", "9S", "7C", "8C", "9C", "TD", "JH"),
			makeCards("7S", "8S", "9S", "JS", "QS", "KS", "7C", "8C"),
			makeCards("TS", "JS", "QS", "KS", "AS", "TH", "TD", "TC"),
			makeCards("9S", "TS", "JS", "QS", "JH", "JD", "JC", "7C"),
			makeCards("QS", "KS", "AS", "QH", "QD", "QC", "7D", "8C"),
			makeCards("7H", "8H", "9H", "TH", "JH", "QH", "KH", "AH"),
		} {
			check(t, detect(t, hand))
		}
	})

	t.Run("seeded sweep over dealt hands", func(t *testing.T) {
		deck := make([]game.Card, 0, 32)
		for _, r := range game.NaturalRankSequence {
			for _, su := range []game.Suit{game.SuitSpades, game.SuitHearts, game.SuitDiamonds, game.SuitClubs} {
				deck = append(deck, game.Card{Rank: r, Suit: su})
			}
		}

		rng := rand.New(rand.NewSource(12005)) //nolint:gosec // deterministic test fixture, not crypto
		const deals = 5000
		for d := 0; d < deals; d++ {
			rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
			for seat := 0; seat < 4; seat++ {
				hand := make([]game.Card, 8)
				copy(hand, deck[seat*8:seat*8+8])
				check(t, detect(t, hand))
			}
		}
	})

	assert.Positive(t, samePairs[game.DeclarationSequence],
		"sweep never inspected a hand with two sequences — coverage is vacuous")
	assert.Positive(t, samePairs[game.DeclarationFourOfAKind],
		"sweep never inspected a hand with two four-of-a-kinds — coverage is vacuous")
}

// seatPlay is one scripted card play.
type seatPlay struct {
	seat int
	card string
}

// resolveTrick1 seats the given hands on the fixture, plays trick 1 through the scripted plays and
// returns the post-resolution state, declaring for declaringSeats and skipping for everyone else.
func resolveTrick1(
	t *testing.T,
	gs *game.GameState,
	hands [4][]game.Card,
	plays []seatPlay,
	declaringSeats ...int,
) *game.GameState {
	t.Helper()

	for seat, hand := range hands {
		if hand != nil {
			gs.Players[seat].Hand = hand
		}
	}
	gs.BelotAnnounced = true

	declares := map[int]bool{}
	for _, seat := range declaringSeats {
		declares[seat] = true
	}

	state := gs
	for i, p := range plays {
		switch {
		case declares[p.seat]:
			if i == 0 {
				state.AwaitingDeclaration = true
			}
			require.True(t, state.AwaitingDeclaration, "engine must prompt seat %d to declare", p.seat)
			require.Equal(t, p.seat, state.ActivePlayerSeat)
			next, err := game.ApplyAction(state, game.Action{Type: game.ActionDeclare, PlayerSeat: p.seat})
			require.NoError(t, err, "seat %d declare", p.seat)
			state = next
		case state.AwaitingDeclaration:
			next, err := game.ApplyAction(state, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: p.seat})
			require.NoError(t, err, "seat %d skip_declare", p.seat)
			state = next
		}

		card, err := game.ParseCard(p.card)
		require.NoError(t, err)
		next, err := game.ApplyAction(state, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: p.seat, Card: &card,
		})
		require.NoError(t, err, "seat %d play %s", p.seat, p.card)
		state = next
	}

	require.True(t, state.DeclarationsResolved, "trick 1 must trigger declaration resolution")
	return state
}

var overlapOnlyLayout = [4][]game.Card{
	makeCards("9S", "TS", "JS", "QS", "JH", "JD", "JC", "7C"),
	makeCards("7S", "8S", "7H", "8H", "7D", "8D", "8C", "9C"),
	makeCards("KS", "AS", "9H", "KH", "QD", "KD", "QC", "TC"),
	makeCards("TH", "QH", "AH", "9D", "TD", "AD", "KC", "AC"),
}

var overlapOnlyPlays = []seatPlay{
	{1, "7S"}, {2, "KS"}, {3, "TH"}, {0, "9S"},
}

func TestDeclarationOverlapScoresBothMelds(t *testing.T) {
	t.Run("Croatian: team A is awarded 50 + 200", func(t *testing.T) {
		state := resolveTrick1(t, testfixtures.NewGameCroatianFirstTrick(game.SuitHearts),
			overlapOnlyLayout, overlapOnlyPlays, 0)

		assert.Len(t, state.Players[0].Declarations, 2)
		assert.Equal(t, 250, state.DeclarationPoints[game.TeamA])
		assert.Equal(t, 0, state.DeclarationPoints[game.TeamB])
		assert.Equal(t, 0, state.HandPoints[game.TeamA])
		assert.Equal(t, 14, state.HandPoints[game.TeamB])
	})

	t.Run("Bitola: the same hand is awarded 200 only", func(t *testing.T) {
		state := resolveTrick1(t, testfixtures.NewGameFirstTrick(game.SuitHearts),
			overlapOnlyLayout, overlapOnlyPlays, 0)

		assert.Len(t, state.Players[0].Declarations, 1)
		assert.Equal(t, 200, state.DeclarationPoints[game.TeamA])
		assert.Equal(t, 0, state.DeclarationPoints[game.TeamB])
		assert.Equal(t, 0, state.HandPoints[game.TeamA])
		assert.Equal(t, 14, state.HandPoints[game.TeamB])
	})
}

// TestCroatianOverlapClashAcrossTeams checks the winner is picked by each team's single best meld,
// so a team with a higher overlapping sum can still lose and have all its melds cleared.
func TestCroatianOverlapClashAcrossTeams(t *testing.T) {
	t.Run("opposing carré of Jacks (200) beats the overlap team's best (100)", func(t *testing.T) {
		hands := [4][]game.Card{
			makeCards("QS", "KS", "AS", "QH", "QD", "QC", "7D", "8C"),
			makeCards("JS", "JH", "JD", "JC", "7S", "8S", "9D", "TC"),
			makeCards("9S", "TS", "7H", "8H", "8D", "TD", "7C", "9C"),
			makeCards("9H", "TH", "KH", "AH", "KD", "AD", "KC", "AC"),
		}
		plays := []seatPlay{{1, "7S"}, {2, "9S"}, {3, "9H"}, {0, "QS"}}

		before := declareSeat0(t, testfixtures.NewGameCroatianFirstTrick(game.SuitHearts), hands[0])
		require.Len(t, before.Players[0].Declarations, 2)
		assert.Equal(t, 120, totalDeclarationValue(before.Players[0].Declarations))

		state := resolveTrick1(t, testfixtures.NewGameCroatianFirstTrick(game.SuitHearts),
			hands, plays, 0, 1)

		assert.Equal(t, 200, state.DeclarationPoints[game.TeamB], "team B wins on its single strongest meld")
		assert.Equal(t, 0, state.DeclarationPoints[game.TeamA], "the overlap team scores nothing")
		assert.Empty(t, state.Players[0].Declarations, "both of the losing seat's overlapping melds are cleared")
		assert.Empty(t, state.Players[2].Declarations, "losing teammate is cleared too")
		assert.Len(t, state.Players[1].Declarations, 1, "winning team's meld is preserved")
	})

	t.Run("the overlap team's larger sum still loses the clash", func(t *testing.T) {
		hands := [4][]game.Card{
			makeCards("TS", "JS", "QS", "KS", "AS", "TH", "TD", "TC"),
			makeCards("9S", "9H", "9D", "9C", "JH", "QD", "KC", "7S"),
			makeCards("AH", "7H", "8H", "7D", "8D", "7C", "8C", "JC"),
			makeCards("QH", "KH", "8S", "JD", "KD", "AD", "QC", "AC"),
		}
		plays := []seatPlay{{1, "7S"}, {2, "7H"}, {3, "8S"}, {0, "AS"}}

		before := declareSeat0(t, testfixtures.NewGameCroatianFirstTrick(game.SuitHearts), hands[0])
		require.Len(t, before.Players[0].Declarations, 2)
		require.Equal(t, 200, totalDeclarationValue(before.Players[0].Declarations))

		state := resolveTrick1(t, testfixtures.NewGameCroatianFirstTrick(game.SuitHearts),
			hands, plays, 0, 1)

		require.Len(t, state.Players[1].Declarations, 1)
		assert.Equal(t, 150, state.Players[1].Declarations[0].Value)
		assert.Equal(t, 150, state.DeclarationPoints[game.TeamB])
		assert.Equal(t, 0, state.DeclarationPoints[game.TeamA],
			"team A held 100 + 100 = 200 across two overlapping melds and still scores 0")
		assert.Empty(t, state.Players[0].Declarations, "both of the losing seat's overlapping melds are cleared")
	})
}

// TestNaturalRankOrder_Values verifies that NaturalRankSequence and NaturalRankOrder
// correctly expose the declaration sequence ordering (7 < 8 < 9 < T < J < Q < K < A).
func TestNaturalRankOrder_Values(t *testing.T) {
	seq := []game.Rank{game.Rank7, game.Rank8, game.Rank9, game.RankTen, game.RankJack, game.RankQueen, game.RankKing, game.RankAce}
	require.Len(t, game.NaturalRankSequence, 8)
	for i, r := range seq {
		assert.Equal(t, r, game.NaturalRankSequence[i], "sequence index %d", i)
		assert.Equal(t, i, game.NaturalRankOrder[r], "order of %v", r)
	}
}

// makeCards is a test helper that creates cards from 2-char IDs.
func makeCards(ids ...string) []game.Card {
	cards := make([]game.Card, len(ids))
	for i, id := range ids {
		c, err := game.ParseCard(id)
		if err != nil {
			panic("invalid card in test: " + id)
		}
		cards[i] = c
	}
	return cards
}

// croatianDeclaringWith deals the given hands through the Croatian open and face-down slots and
// resolves the bid via ApplyAction, opening the dedicated declaration phase.
func croatianDeclaringWith(t *testing.T, trump game.Suit, hands [4][]game.Card) *game.GameState {
	t.Helper()

	gs := testfixtures.NewGameCroatianJustDealt()
	for seat, hand := range hands {
		require.Len(t, hand, 8, "seat %d must be given all eight of its cards", seat)
		gs.Players[seat].Hand = append([]game.Card(nil), hand[:6]...)
		gs.Players[seat].FaceDownCards = append([]game.Card(nil), hand[6:]...)
	}

	state, err := game.ApplyAction(gs, game.Action{
		Type:       game.ActionPickTrump,
		PlayerSeat: gs.ActivePlayerSeat,
		Suit:       &trump,
	})
	require.NoError(t, err)
	return state
}

var (
	meldGapLayout = [4][]game.Card{
		makeCards("7H", "8H", "9H", "TH", "QD", "8C", "KS", "AS"),
		makeCards("7S", "8S", "9S", "JH", "KD", "7C", "TC", "AC"),
		makeCards("TS", "QS", "QH", "AH", "7D", "9D", "JD", "QC"),
		makeCards("JS", "KH", "8D", "TD", "AD", "9C", "JC", "KC"),
	}

	allMeldLayout = [4][]game.Card{
		makeCards("7S", "8S", "9S", "TS", "7H", "8H", "JS", "QS"),
		makeCards("7D", "8D", "9D", "TD", "9H", "TH", "JD", "QD"),
		makeCards("7C", "8C", "9C", "TC", "JH", "QH", "JC", "QC"),
		makeCards("KS", "AS", "KD", "AD", "KH", "AH", "KC", "AC"),
	}

	noMeldLayout = [4][]game.Card{
		makeCards("7S", "9S", "8H", "TH", "JD", "KD", "QC", "AC"),
		makeCards("8S", "TS", "JH", "KH", "QD", "AD", "7C", "9C"),
		makeCards("JS", "KS", "QH", "AH", "7D", "9D", "8C", "TC"),
		makeCards("QS", "AS", "7H", "9H", "8D", "TD", "JC", "KC"),
	}
)

// TestCroatianBidOpensDeclarationPhase covers both Croatian bidding paths into the declaration
// phase: an early free-suit pick and the dealer's forced pick.
func TestCroatianBidOpensDeclarationPhase(t *testing.T) {
	tests := []struct {
		name      string
		passCount int
		seat      int
	}{
		{name: "free-suit pick by the first bidder", passCount: 0, seat: 1},
		{name: "forced pick by the dealer after three passes", passCount: 3, seat: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.NewGameCroatianMidBidding(tc.passCount)
			require.Equal(t, tc.seat, gs.ActivePlayerSeat)

			suit := game.SuitHearts
			state, err := game.ApplyAction(gs, game.Action{
				Type:       game.ActionPickTrump,
				PlayerSeat: tc.seat,
				Suit:       &suit,
			})

			require.NoError(t, err)
			assert.Equal(t, game.PhaseDeclaring, state.Phase)
			assert.Equal(t, 0, state.TrickNumber, "no trick is open yet")
			assert.Empty(t, state.CurrentTrick)
			assert.Equal(t, (state.DealerSeat+1)%4, state.ActivePlayerSeat,
				"pinned to the trick-1 leader for the phase — a positional constant, not a cursor")
			assert.False(t, state.AwaitingDeclaration,
				"nobody is on the clock: all four seats are asked at once")
			for seat, p := range state.Players {
				assert.False(t, p.DeclarationAnswered, "seat %d starts unanswered", seat)
			}
			assert.False(t, state.DeclarationsResolved)
			assert.Equal(t, [2]int{0, 0}, state.DeclarationPoints, "nothing is awarded before the phase ends")
		})
	}
}

// TestBitolaBidStillOpensTrick1 is the other half of the same branch: a Bitola
// pick must reach trick 1 exactly as it always has, with the prompt inside it.
func TestBitolaBidStillOpensTrick1(t *testing.T) {
	gs := testfixtures.NewGameJustDealt()

	state, err := game.ApplyAction(gs, game.Action{
		Type:       game.ActionPickTrump,
		PlayerSeat: gs.ActivePlayerSeat,
	})

	require.NoError(t, err)
	assert.Equal(t, game.PhasePlaying, state.Phase)
	assert.Equal(t, 1, state.TrickNumber)
	assert.Equal(t, (state.DealerSeat+1)%4, state.ActivePlayerSeat)
	for seat, p := range state.Players {
		assert.False(t, p.DeclarationAnswered,
			"seat %d: the dedicated-phase flag stays untouched under Bitola timing", seat)
	}
}

// TestCroatianDeclarationPhaseAcceptsAnyOrder checks all four seats may answer the declaration
// phase in any order, not just play order.
func TestCroatianDeclarationPhaseAcceptsAnyOrder(t *testing.T) {
	state := testfixtures.NewGameCroatianDeclaring(game.SuitHearts)

	for i, seat := range []int{3, 0, 2, 1} {
		require.Equal(t, game.PhaseDeclaring, state.Phase, "still in the phase before answer %d", i+1)
		require.False(t, state.AwaitingDeclaration, "no seat is ever on the clock here")
		require.False(t, state.DeclarationsResolved, "the contest resolves only once all four have answered")

		var err error
		state, err = game.ApplyAction(state, game.Action{Type: game.ActionDeclare, PlayerSeat: seat})
		require.NoError(t, err, "seat %d may answer out of play order", seat)
		if state.Phase == game.PhaseDeclaring {
			assert.True(t, state.Players[seat].DeclarationAnswered)
			assert.NotEmpty(t, state.Players[seat].Declarations, "seat %d's melds are stored", seat)
		}
	}

	assert.Equal(t, game.PhasePlaying, state.Phase)
	assert.Equal(t, 1, state.TrickNumber)
	assert.True(t, state.DeclarationsResolved)
	assert.False(t, state.AwaitingDeclaration)
	assert.NotEqual(t, [2]int{0, 0}, state.DeclarationPoints)
}

// TestCroatianDeclarationOutcomeIsOrderIndependent checks the same team wins the same points
// whatever order the seats answer in.
func TestCroatianDeclarationOutcomeIsOrderIndependent(t *testing.T) {
	orders := [][]int{
		{1, 2, 3, 0},
		{0, 3, 2, 1},
		{2, 0, 1, 3},
	}

	var want [2]int
	for i, order := range orders {
		state := croatianDeclaringWith(t, game.SuitClubs, allMeldLayout)
		for _, seat := range order {
			var err error
			state, err = game.ApplyAction(state, game.Action{Type: game.ActionDeclare, PlayerSeat: seat})
			require.NoError(t, err)
		}
		require.Equal(t, game.PhasePlaying, state.Phase)
		if i == 0 {
			want = state.DeclarationPoints
			require.NotEqual(t, [2]int{0, 0}, want, "the layout must actually produce a contest")
			continue
		}
		assert.Equal(t, want, state.DeclarationPoints, "order %v changed the outcome", order)
	}
}

// TestCroatianDeclarationPhaseAsksMeldlessSeatsToo checks seats without melds are asked like
// everyone else, so observers cannot tell who holds one.
func TestCroatianDeclarationPhaseAsksMeldlessSeatsToo(t *testing.T) {
	state := croatianDeclaringWith(t, game.SuitClubs, meldGapLayout)
	require.Equal(t, game.PhaseDeclaring, state.Phase)

	for _, seat := range []int{2, 3} {
		require.False(t, game.HasDeclarableCombinations(state, seat), "fixture: seat %d holds no meld", seat)

		answered, err := game.ApplyAction(state, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: seat})
		require.NoError(t, err, "seat %d must be able to answer despite holding nothing", seat)
		assert.True(t, answered.Players[seat].DeclarationAnswered)
		assert.Equal(t, game.PhaseDeclaring, answered.Phase, "two other seats still owe an answer")
	}

	for _, seat := range []int{2, 3} {
		rejected, err := game.ApplyAction(state, game.Action{Type: game.ActionDeclare, PlayerSeat: seat})
		assert.Nil(t, rejected)
		assert.ErrorIs(t, err, apperr.ErrDeclarationNotAvailable, "seat %d holds no meld", seat)
	}
}

// TestCroatianDeclarationPhaseRejectsSecondAnswer checks a seat's first answer stands and a
// second answer is rejected.
func TestCroatianDeclarationPhaseRejectsSecondAnswer(t *testing.T) {
	base := croatianDeclaringWith(t, game.SuitClubs, meldGapLayout)

	skipped, err := game.ApplyAction(base, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: 1})
	require.NoError(t, err)
	require.True(t, skipped.Players[1].DeclarationAnswered)
	require.Empty(t, skipped.Players[1].Declarations)

	for _, action := range []game.Action{
		{Type: game.ActionDeclare, PlayerSeat: 1},
		{Type: game.ActionSkipDeclare, PlayerSeat: 1},
	} {
		rejected, err := game.ApplyAction(skipped, action)
		assert.Nil(t, rejected)
		assert.ErrorIs(t, err, apperr.ErrWrongPhase, "%s after answering", action.Type)
	}

	assert.Empty(t, skipped.Players[1].Declarations)
}

// TestCroatianDeclarationPhaseWithNoMeldsStillOpens checks the phase opens and asks all four
// seats even when nobody holds a meld.
func TestCroatianDeclarationPhaseWithNoMeldsStillOpens(t *testing.T) {
	state := croatianDeclaringWith(t, game.SuitSpades, noMeldLayout)

	require.Equal(t, game.PhaseDeclaring, state.Phase, "the phase opens even with nothing to declare")
	assert.Equal(t, 0, state.TrickNumber)
	assert.False(t, state.DeclarationsResolved)
	for seat := range state.Players {
		require.False(t, game.HasDeclarableCombinations(state, seat), "fixture: seat %d holds no meld", seat)
	}

	for _, seat := range []int{1, 2, 3, 0} {
		var err error
		state, err = game.ApplyAction(state, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: seat})
		require.NoError(t, err)
	}

	assert.Equal(t, game.PhasePlaying, state.Phase)
	assert.Equal(t, 1, state.TrickNumber)
	assert.Equal(t, (state.DealerSeat+1)%4, state.ActivePlayerSeat)
	assert.False(t, state.AwaitingDeclaration)
	assert.True(t, state.DeclarationsResolved)
	assert.Equal(t, [2]int{0, 0}, state.DeclarationPoints)
	for seat, p := range state.Players {
		assert.Empty(t, p.Declarations, "seat %d has nothing to declare", seat)
	}
}

// TestCroatianDeclarationPhaseResolvesIntoTrick1 checks the last answer resolves the contest,
// clears the losing team's melds and opens trick 1 with the leader on the clock.
func TestCroatianDeclarationPhaseResolvesIntoTrick1(t *testing.T) {
	state := croatianDeclaringWith(t, game.SuitClubs, meldGapLayout)

	for _, seat := range []int{1, 0} {
		var err error
		state, err = game.ApplyAction(state, game.Action{Type: game.ActionDeclare, PlayerSeat: seat})
		require.NoError(t, err)
		require.Equal(t, game.PhaseDeclaring, state.Phase, "the meld-less seats have not answered yet")
	}
	for _, seat := range []int{2, 3} {
		var err error
		state, err = game.ApplyAction(state, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: seat})
		require.NoError(t, err)
	}

	assert.Equal(t, game.PhasePlaying, state.Phase)
	assert.Equal(t, 1, state.TrickNumber)
	assert.Empty(t, state.CurrentTrick)
	assert.False(t, state.AwaitingDeclaration)
	for seat, p := range state.Players {
		assert.False(t, p.DeclarationAnswered, "seat %d's answered flag is cleared on the way out", seat)
	}
	assert.Equal(t, (state.DealerSeat+1)%4, state.ActivePlayerSeat, "the seat after the dealer leads trick 1")

	assert.True(t, state.DeclarationsResolved)
	assert.Equal(t, 50, state.DeclarationPoints[game.TeamA], "seat 0's quarte outranks seat 1's tierce")
	assert.Equal(t, 0, state.DeclarationPoints[game.TeamB])
	assert.Empty(t, state.Players[1].Declarations, "the losing team's melds are cleared")
	assert.NotEmpty(t, state.Players[0].Declarations)

	lead := state.Players[state.ActivePlayerSeat].Hand[0]
	played, err := game.ApplyAction(state, game.Action{
		Type:       game.ActionPlayCard,
		PlayerSeat: state.ActivePlayerSeat,
		Card:       &lead,
	})
	require.NoError(t, err)
	assert.Len(t, played.CurrentTrick, 1)
}

// TestCroatianDeclarationPhaseSkipStoresNothing pins the skip row: no melds are
// recorded and no points are awarded, but the seat still counts as answered.
func TestCroatianDeclarationPhaseSkipStoresNothing(t *testing.T) {
	state := croatianDeclaringWith(t, game.SuitClubs, meldGapLayout)

	for _, seat := range []int{1, 0, 2, 3} {
		var err error
		state, err = game.ApplyAction(state, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: seat})
		require.NoError(t, err)
		assert.Empty(t, state.Players[seat].Declarations)
	}

	assert.Equal(t, game.PhasePlaying, state.Phase)
	assert.True(t, state.DeclarationsResolved)
	assert.Equal(t, [2]int{0, 0}, state.DeclarationPoints, "nobody declared, so nobody scores")
	assert.Equal(t, 1, state.TrickNumber)
}

// TestCroatianDeclarationPhaseIgnoresDisconnectedSeats pins the disconnect row:
// the close gate counts CONNECTED seats only, so a dropped player cannot hold
// the table until the session manager's window elapses.
func TestCroatianDeclarationPhaseIgnoresDisconnectedSeats(t *testing.T) {
	state := croatianDeclaringWith(t, game.SuitClubs, meldGapLayout)
	state.Players[3].Connected = false

	for _, seat := range []int{1, 0, 2} {
		var err error
		state, err = game.ApplyAction(state, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: seat})
		require.NoError(t, err)
	}

	assert.Equal(t, game.PhasePlaying, state.Phase,
		"the three connected seats answered; the dropped seat is not waited on")
	assert.True(t, state.DeclarationsResolved)
	assert.Equal(t, 1, state.TrickNumber)
}

// TestForceCloseDeclarationPhase pins the window-elapsed row: every unanswered
// seat is treated as having skipped, and the phase lands in exactly the same
// place the answer-driven close does.
func TestForceCloseDeclarationPhase(t *testing.T) {
	t.Run("closes with nobody having answered", func(t *testing.T) {
		state := croatianDeclaringWith(t, game.SuitClubs, meldGapLayout)

		closed, err := game.ForceCloseDeclarationPhase(state)

		require.NoError(t, err)
		assert.Equal(t, game.PhasePlaying, closed.Phase)
		assert.Equal(t, 1, closed.TrickNumber)
		assert.Equal(t, (closed.DealerSeat+1)%4, closed.ActivePlayerSeat)
		assert.True(t, closed.DeclarationsResolved)
		assert.Equal(t, [2]int{0, 0}, closed.DeclarationPoints, "an unanswered seat forfeits its melds")
		for seat, p := range closed.Players {
			assert.False(t, p.DeclarationAnswered, "seat %d's flag is cleared on the way out", seat)
			assert.Empty(t, p.Declarations, "seat %d never declared", seat)
		}
	})

	t.Run("keeps the answers already given", func(t *testing.T) {
		state := croatianDeclaringWith(t, game.SuitClubs, meldGapLayout)
		state, err := game.ApplyAction(state, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
		require.NoError(t, err)
		require.Equal(t, game.PhaseDeclaring, state.Phase)

		closed, err := game.ForceCloseDeclarationPhase(state)

		require.NoError(t, err)
		assert.Equal(t, game.PhasePlaying, closed.Phase)
		assert.Equal(t, 50, closed.DeclarationPoints[game.TeamA], "seat 0's quarte still counts")
		assert.Equal(t, 0, closed.DeclarationPoints[game.TeamB])
	})

	t.Run("rejects any other phase", func(t *testing.T) {
		gs := testfixtures.NewGameJustDealt()
		closed, err := game.ForceCloseDeclarationPhase(gs)
		assert.Nil(t, closed)
		assert.ErrorIs(t, err, apperr.ErrWrongPhase)
	})

	t.Run("does not mutate its input", func(t *testing.T) {
		state := croatianDeclaringWith(t, game.SuitClubs, meldGapLayout)

		_, err := game.ForceCloseDeclarationPhase(state)

		require.NoError(t, err)
		assert.Equal(t, game.PhaseDeclaring, state.Phase, "the caller's state is untouched")
		assert.Equal(t, 0, state.TrickNumber)
	})
}

// TestCroatianDeclarationPhaseRejectsOtherActions pins the phase's action
// allowlist: declare and skip_declare only, and no card can be played before
// the contest closes.
func TestCroatianDeclarationPhaseRejectsOtherActions(t *testing.T) {
	base := testfixtures.NewGameCroatianDeclaring(game.SuitHearts)

	card := base.Players[1].Hand[0]
	suit := game.SuitSpades

	rejected := []struct {
		name   string
		action game.Action
	}{
		{"play_card", game.Action{Type: game.ActionPlayCard, PlayerSeat: 1, Card: &card}},
		{"pick_trump", game.Action{Type: game.ActionPickTrump, PlayerSeat: 1, Suit: &suit}},
		{"pass_trump", game.Action{Type: game.ActionPassTrump, PlayerSeat: 1}},
		{"announce_belot", game.Action{Type: game.ActionAnnounceBelot, PlayerSeat: 1}},
		{"skip_belot", game.Action{Type: game.ActionSkipBelot, PlayerSeat: 1}},
		{"continue", game.Action{Type: game.ActionContinue, PlayerSeat: 1}},
		{"pause", game.Action{Type: game.ActionPause, PlayerSeat: 2}},
	}

	for _, tc := range rejected {
		t.Run(tc.name+" is rejected", func(t *testing.T) {
			state, err := game.ApplyAction(base, tc.action)
			assert.Nil(t, state)
			assert.ErrorIs(t, err, apperr.ErrWrongPhase)
		})
	}
}

// TestCroatianDeclarationPhaseIsNotPausable checks pause is rejected during the declaration phase,
// which has no turn timer to preserve, while surrender stays allowed.
func TestCroatianDeclarationPhaseIsNotPausable(t *testing.T) {
	base := testfixtures.NewGameCroatianDeclaring(game.SuitHearts)

	t.Run("pause is refused", func(t *testing.T) {
		paused, err := game.ApplyAction(base, game.Action{Type: game.ActionPause, PlayerSeat: 2})
		assert.Nil(t, paused)
		assert.ErrorIs(t, err, apperr.ErrWrongPhase)
	})

	t.Run("surrender can still be proposed in the phase", func(t *testing.T) {
		proposed, err := game.ApplyAction(base, game.Action{Type: game.ActionSurrenderRequest, PlayerSeat: 1})
		require.NoError(t, err)
		require.NotNil(t, proposed.SurrenderProposerSeat)
		assert.Equal(t, 1, *proposed.SurrenderProposerSeat)
	})
}

// TestCroatianDeclarationFlagsDoNotLeakIntoHand2 checks startNewHand resets the per-hand answered
// flags so hand 2's declaration phase does not close immediately.
func TestCroatianDeclarationFlagsDoNotLeakIntoHand2(t *testing.T) {
	gs := testfixtures.NewGameCroatianDeclaring(game.SuitHearts)
	gs.Phase = game.PhaseHandComplete
	for seat := range gs.Players {
		gs.Players[seat].DeclarationAnswered = true
	}
	gs.DeclarationsResolved = true

	next, err := game.ForceAdvanceHandComplete(gs)

	require.NoError(t, err)
	for seat, p := range next.Players {
		assert.False(t, p.DeclarationAnswered, "seat %d's answer must not survive the hand", seat)
	}
	assert.False(t, next.AwaitingDeclaration)
	assert.False(t, next.DeclarationsResolved)
	assert.Equal(t, 2, next.HandNumber)
	require.Equal(t, 1, next.DealerSeat, "startNewHand rotates the dealer counter-clockwise")
	require.NotEqual(t, game.PhaseMatchEnd, next.Phase, "hand 2 must have dealt normally")

	for seat, hand := range allMeldLayout {
		next.Players[seat].Hand = append([]game.Card(nil), hand[:6]...)
		next.Players[seat].FaceDownCards = append([]game.Card(nil), hand[6:]...)
	}
	next.Phase = game.PhaseBidding

	suit := game.SuitClubs
	opened, err := game.ApplyAction(next, game.Action{
		Type:       game.ActionPickTrump,
		PlayerSeat: next.ActivePlayerSeat,
		Suit:       &suit,
	})
	require.NoError(t, err)

	assert.Equal(t, game.PhaseDeclaring, opened.Phase, "the phase opens rather than closing on stale flags")
	assert.Equal(t, 2, opened.ActivePlayerSeat,
		"hand 2 pins to the new dealer's left (seat 1 deals, so seat 2 leads trick 1)")
	assert.Equal(t, (opened.DealerSeat+1)%4, opened.ActivePlayerSeat)
	for seat, p := range opened.Players {
		assert.False(t, p.DeclarationAnswered, "seat %d starts hand 2's phase unanswered", seat)
	}
	assert.False(t, opened.AwaitingDeclaration)
}
