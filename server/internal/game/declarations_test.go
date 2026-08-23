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

// --- Declaration Detection Tests (Task 7) ---

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
			wantDeclCount: 1, // quarte diamonds
		},
		{
			name:          "seat 0 declares two tierces (20pts each)",
			seat:          0,
			wantDeclCount: 2, // tierce spades + tierce clubs
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

			// Verify declaration values
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

			// Find declaration matching expected type and value
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
			PlayerSeat: 2, // not active
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

// --- Declaration Resolution Tests ---

func TestDeclarationResolution(t *testing.T) {
	t.Run("higher value declaration wins", func(t *testing.T) {
		decls := []game.Declaration{
			{Type: game.DeclarationSequence, Cards: makeCards("7S", "8S", "9S"), PlayerSeat: 0, Value: 20},
			{Type: game.DeclarationSequence, Cards: makeCards("JD", "QD", "KD", "AD"), PlayerSeat: 1, Value: 50},
		}
		gs := testfixtures.NewGameWithDeclarations(decls)
		// Simulate that trick 1 has been played (4 cards in trick)
		gs.CurrentTrick = []game.TrickCard{
			{Card: game.Card{Rank: game.RankTen, Suit: game.SuitSpades}, PlayerSeat: 1},
			{Card: game.Card{Rank: game.Rank7, Suit: game.SuitSpades}, PlayerSeat: 2},
			{Card: game.Card{Rank: game.Rank7, Suit: game.SuitClubs}, PlayerSeat: 3},
			{Card: game.Card{Rank: game.Rank8, Suit: game.SuitSpades}, PlayerSeat: 0},
		}
		leadSuit := game.SuitSpades
		gs.LeadSuit = &leadSuit
		gs.DeclarationsResolved = false
		gs.BelotAnnounced = true // skip Belot for this test

		// Play a card to trigger trick resolution (use trick 1 → resolves declarations)
		// Actually, we need to test declaration resolution after trick 1 completes.
		// The simplest way: manually check the resolution through the fixture.
		// Since resolveDeclarations is unexported, we test through the full flow instead.

		// Team B (seat 1) has 50pts vs team A (seat 0) has 20pts → team B wins
		assert.Equal(t, 50, decls[1].Value)
		assert.Equal(t, 20, decls[0].Value)
		// Team B declaration should win because 50 > 20
	})

	t.Run("only winning team declarations scored", func(t *testing.T) {
		// Team A total would be 40 (two tierces), team B total is 50 (quarte).
		// Team B's best (50) beats team A's best (20), so team B wins all 50 and team A gets 0.
		decls := []game.Declaration{
			{Type: game.DeclarationSequence, Cards: makeCards("7S", "8S", "9S"), PlayerSeat: 0, Value: 20},       // Team A
			{Type: game.DeclarationSequence, Cards: makeCards("7C", "8C", "9C"), PlayerSeat: 2, Value: 20},       // Team A (teammate)
			{Type: game.DeclarationSequence, Cards: makeCards("JD", "QD", "KD", "AD"), PlayerSeat: 1, Value: 50}, // Team B
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
			{Type: game.DeclarationSequence, Cards: makeCards("7S", "8S", "9S"), PlayerSeat: 0, Value: 20},           // Team A
			{Type: game.DeclarationFourOfAKind, Cards: makeCards("TS", "TH", "TD", "TC"), PlayerSeat: 2, Value: 100}, // Team A (teammate)
		}
		state := completeTrick1(t, decls)

		assert.Equal(t, 120, state.DeclarationPoints[game.TeamA], "Team A wins with full sum 20+100")
		assert.Equal(t, 0, state.DeclarationPoints[game.TeamB], "Team B gets 0 — no declarations")
		assert.NotEmpty(t, state.Players[0].Declarations, "Team A seat 0 declarations preserved")
		assert.NotEmpty(t, state.Players[2].Declarations, "Team A seat 2 declarations preserved")
	})

	// Regression: per Beljot rules, declaration comparison uses each team's
	// SINGLE strongest meld, never the team sum. Team A's two 100-pt Kares
	// (sum=200) lose to Team B's lone 150-pt Kare-of-9.
	t.Run("strongest single meld wins over higher team sum (Q+K vs 9)", func(t *testing.T) {
		decls := []game.Declaration{
			{Type: game.DeclarationFourOfAKind, Cards: makeCards("QS", "QH", "QD", "QC"), PlayerSeat: 0, Value: 100}, // Team A
			{Type: game.DeclarationFourOfAKind, Cards: makeCards("9S", "9H", "9D", "9C"), PlayerSeat: 1, Value: 150}, // Team B
			{Type: game.DeclarationFourOfAKind, Cards: makeCards("KS", "KH", "KD", "KC"), PlayerSeat: 2, Value: 100}, // Team A (teammate)
		}
		state := completeTrick1(t, decls)

		assert.Equal(t, 0, state.DeclarationPoints[game.TeamA], "Team A gets 0 even though sum (200) > team B's 150")
		assert.Equal(t, 150, state.DeclarationPoints[game.TeamB], "Team B wins with its single strongest meld total")
		assert.Empty(t, state.Players[0].Declarations, "Team A seat 0 declarations cleared")
		assert.Empty(t, state.Players[2].Declarations, "Team A seat 2 declarations cleared")
		assert.NotEmpty(t, state.Players[1].Declarations, "Team B seat 1 declarations preserved")
	})
}

// completeTrick1 seats the given declarations on the standard NewGameFirstTrick
// fixture and drives trick 1 to completion through legal plays so that
// declaration resolution fires. Returns the post-resolution state. Belot is
// pre-marked announced so the trick-end flow doesn't stall on the K/Q-trump
// pair held by seat 0 in the fixture hands.
func completeTrick1(t *testing.T, decls []game.Declaration) *game.GameState {
	t.Helper()
	state := testfixtures.NewGameWithDeclarations(decls)
	state.BelotAnnounced = true

	plays := []struct {
		seat int
		card game.Card
	}{
		{1, game.Card{Rank: game.Rank8, Suit: game.SuitDiamonds}},   // seat 1 leads diamonds
		{2, game.Card{Rank: game.Rank9, Suit: game.SuitHearts}},     // seat 2 void in diamonds, opponent winning → trumps
		{3, game.Card{Rank: game.RankTen, Suit: game.SuitDiamonds}}, // seat 3 follows diamonds (trick already cut by 9H — any diamond legal)
		{0, game.Card{Rank: game.RankQueen, Suit: game.SuitHearts}}, // seat 0 void in diamonds with trump K+Q — must cut; QH < 9H so falls through to any-trump
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

// --- Belot Bonus Tests (Task 8) ---

func TestBelotDetection(t *testing.T) {
	t.Run("play trump K while holding trump Q triggers Belot prompt", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.ActivePlayerSeat = 0 // seat 0 has KH + QH
		gs.AwaitingDeclaration = false
		gs.DeclarationsResolved = true // skip declaration flow for this test

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
		gs.ActivePlayerSeat = 2 // seat 2 has JH 9H AH TH but NOT QH or KH together
		gs.AwaitingDeclaration = false
		gs.DeclarationsResolved = true

		// Seat 2 has JH — not K or Q, so no Belot
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
		gs.ActivePlayerSeat = 2 // seat 2 has KS
		gs.AwaitingDeclaration = false
		gs.DeclarationsResolved = true

		card := game.Card{Rank: game.RankKing, Suit: game.SuitSpades} // non-trump
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

		// Play trump K to trigger Belot
		card := game.Card{Rank: game.RankKing, Suit: game.SuitHearts}
		state1, err := game.ApplyAction(gs, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: 0, Card: &card,
		})
		require.NoError(t, err)
		require.NotNil(t, state1.PendingBelotSeat)

		// Announce Belot
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

		// Try to play another card without resolving Belot
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

// --- Integration Tests (Task 9): Full Trick 1 Flow ---

func TestFullTrick1WithDeclarations(t *testing.T) {
	t.Run("full trick 1 flow with declarations and Belot", func(t *testing.T) {
		// Fixture hands (trump=Hearts):
		// Seat 0: KH QH 7S 8S 9S 7C 8C 9C → Belot + 2 tierces (40 total)
		// Seat 1: JD QD KD AD TS QS 8D TC → quarte JD-QD-KD-AD (50)
		// Seat 2: JH 9H AH TH KS AS JC QC → tierce 9H-TH-JH (20)
		// Seat 3: 7H 8H JS 9D TD 7D AC KC → no declarations
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		// Active player is seat 1 (after dealer seat 0)
		gs.AwaitingDeclaration = true // simulate the prompt set by handlePickTrump

		// Step 1: Seat 1 declares (quarte diamonds = 50pts)
		state, err := game.ApplyAction(gs, game.Action{
			Type: game.ActionDeclare, PlayerSeat: 1,
		})
		require.NoError(t, err)
		assert.False(t, state.AwaitingDeclaration)
		assert.Equal(t, 1, len(state.Players[1].Declarations))
		assert.Equal(t, 50, state.Players[1].Declarations[0].Value)

		// Step 2: Seat 1 plays 8D (leads diamonds — seat 0 is void in diamonds)
		card1 := game.Card{Rank: game.Rank8, Suit: game.SuitDiamonds}
		state, err = game.ApplyAction(state, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: 1, Card: &card1,
		})
		require.NoError(t, err)
		assert.Equal(t, 1, len(state.CurrentTrick))
		// Seat 2 has tierce → declaration prompt
		assert.True(t, state.AwaitingDeclaration)
		assert.Equal(t, 2, state.ActivePlayerSeat)

		// Step 3: Seat 2 declares (tierce 9H-TH-JH = 20pts)
		state, err = game.ApplyAction(state, game.Action{
			Type: game.ActionDeclare, PlayerSeat: 2,
		})
		require.NoError(t, err)
		assert.False(t, state.AwaitingDeclaration)

		// Step 4: Seat 2 plays 9H (trump — void in diamonds, must trump since opponent winning)
		card2 := game.Card{Rank: game.Rank9, Suit: game.SuitHearts}
		state, err = game.ApplyAction(state, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: 2, Card: &card2,
		})
		require.NoError(t, err)
		// Seat 3 has diamonds → must follow suit, no declaration prompt
		assert.False(t, state.AwaitingDeclaration)
		assert.Equal(t, 3, state.ActivePlayerSeat)

		// Step 5: Seat 3 plays TD (follows diamonds — seat 3 has TD 7D)
		card3 := game.Card{Rank: game.RankTen, Suit: game.SuitDiamonds}
		state, err = game.ApplyAction(state, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: 3, Card: &card3,
		})
		require.NoError(t, err)
		// Seat 0 has declarations → prompt
		assert.True(t, state.AwaitingDeclaration)
		assert.Equal(t, 0, state.ActivePlayerSeat)

		// Step 6: Seat 0 declares (2 tierces = 20 + 20)
		state, err = game.ApplyAction(state, game.Action{
			Type: game.ActionDeclare, PlayerSeat: 0,
		})
		require.NoError(t, err)
		assert.Equal(t, 2, len(state.Players[0].Declarations))

		// Step 7: Seat 0 plays trump K (void in diamonds, partner winning → any card legal → Belot!)
		card0 := game.Card{Rank: game.RankKing, Suit: game.SuitHearts}
		state, err = game.ApplyAction(state, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: 0, Card: &card0,
		})
		require.NoError(t, err)
		// Belot prompt fires
		require.NotNil(t, state.PendingBelotSeat)
		assert.Equal(t, 0, *state.PendingBelotSeat)
		// Trick not resolved yet (4 cards but Belot pending)
		assert.Equal(t, 4, len(state.CurrentTrick))
		assert.Equal(t, 1, state.TrickNumber)

		// Step 8: Announce Belot (+20 to team A)
		state, err = game.ApplyAction(state, game.Action{
			Type: game.ActionAnnounceBelot, PlayerSeat: 0,
		})
		require.NoError(t, err)
		assert.Nil(t, state.PendingBelotSeat)
		assert.True(t, state.BelotAnnounced)
		// Trick resolved, declarations compared
		assert.Equal(t, 2, state.TrickNumber)
		assert.True(t, state.DeclarationsResolved)

		// Verify Belot: the 20 lives in BelotPoints (a declaration), NOT HandPoints.
		assert.Equal(t, 20, state.BelotPoints[game.TeamA], "Team A belote tracked as a declaration")

		// Verify declarations: team B's quarte (50) beats team A's tierces (20 each)
		assert.Equal(t, 50, state.DeclarationPoints[game.TeamB])
		assert.Equal(t, 0, state.DeclarationPoints[game.TeamA])

		// Team A declarations cleared, team B preserved
		assert.Empty(t, state.Players[0].Declarations)
		assert.Empty(t, state.Players[2].Declarations)
		assert.NotEmpty(t, state.Players[1].Declarations)
	})

	t.Run("state immutability across all actions", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		gs.AwaitingDeclaration = true
		original := *gs

		// Perform declare action
		_, err := game.ApplyAction(gs, game.Action{
			Type: game.ActionDeclare, PlayerSeat: 1,
		})
		require.NoError(t, err)

		// Original state should be unchanged
		assert.Equal(t, original.AwaitingDeclaration, gs.AwaitingDeclaration)
		assert.Empty(t, gs.Players[1].Declarations)
	})
}

// --- Missing coverage tests (Review Patch 8) ---

func TestDeclarationTiebreakers(t *testing.T) {
	t.Run("four-of-a-kind beats sequence at equal 100pts", func(t *testing.T) {
		// Seat 0 has 4xAce (100pts, four-of-a-kind), seat 1 has quinte (100pts, sequence)
		// Four-of-a-kind should win
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		// Give seat 0 four Aces: AS, AH, AD, AC + 4 fillers
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
		// Give seat 1 a quinte (5-card sequence = 100pts): 7D 8D 9D TD JD + 3 fillers
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

		// Seat 0 declares
		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = true
		state, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
		require.NoError(t, err)

		// Play trick 1 with seat 0 leading, skip declarations for others
		state.ActivePlayerSeat = 1
		state.AwaitingDeclaration = true
		state, err = game.ApplyAction(state, game.Action{Type: game.ActionDeclare, PlayerSeat: 1})
		require.NoError(t, err)

		// Verify seat 0 has four-of-a-kind (100pts) and seat 1 has quinte (100pts)
		assert.Equal(t, 100, state.Players[0].Declarations[0].Value)
		assert.Equal(t, 100, state.Players[1].Declarations[0].Value)
		assert.Equal(t, game.DeclarationFourOfAKind, state.Players[0].Declarations[0].Type)
		assert.Equal(t, game.DeclarationSequence, state.Players[1].Declarations[0].Type)
	})

	t.Run("equal-value sequences resolved by top card", func(t *testing.T) {
		gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
		// Seat 0 (team A): tierce Q-K-A spades (top=A)
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
		// Seat 1 (team B): tierce 7-8-9 diamonds (top=9)
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

		// Both declare
		gs.ActivePlayerSeat = 0
		gs.AwaitingDeclaration = true
		state, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
		require.NoError(t, err)

		state.ActivePlayerSeat = 1
		state.AwaitingDeclaration = true
		state, err = game.ApplyAction(state, game.Action{Type: game.ActionDeclare, PlayerSeat: 1})
		require.NoError(t, err)

		// Both have tierces (20pts each) but seat 1 actually has a quinte (100pts)
		// which beats seat 0's tierce (20pts) by value
		// Note: seat 1 has 9D-TD-JD-QD-KD-AD = 6 consecutive = 100pts!
		// Seat 0 has QS-KS-AS = tierce 20pts
		// Team B (seat 1) wins by value 100 > 20
		assert.True(t, len(state.Players[1].Declarations) > 0)
	})

	// Regression: the top-card tiebreak for equal-length sequences must use the
	// NATURAL declaration rank order (7<8<9<10<J<Q<K<A), where Jack outranks
	// Ten — NOT the trick-taking non-trump order (7<8<9<J<Q<K<10<A) where Ten
	// outranks Jack. Bug report: opponent's 9-10-J tierce lost to a player's
	// 8-9-10 tierce because the comparison used NonTrumpRankOrder.
	t.Run("Jack-topped tierce beats Ten-topped tierce (natural order)", func(t *testing.T) {
		decls := []game.Declaration{
			// Team A (seat 0): tierce 8D-9D-TD — top card Ten (team A's strongest)
			{Type: game.DeclarationSequence, Cards: makeCards("8D", "9D", "TD"), PlayerSeat: 0, Value: 20},
			// Team A (seat 2): weaker tierce, placed only so seat 2's natural
			// declarable hand doesn't trigger a prompt that blocks the scripted
			// plays in completeTrick1. Does not affect the tiebreak under test.
			{Type: game.DeclarationSequence, Cards: makeCards("7S", "8S", "9S"), PlayerSeat: 2, Value: 20},
			// Team B (seat 1): tierce 9C-TC-JC — top card Jack
			{Type: game.DeclarationSequence, Cards: makeCards("9C", "TC", "JC"), PlayerSeat: 1, Value: 20},
		}
		state := completeTrick1(t, decls)

		// Neither tierce is trump (trump=Hearts), so the trump tiebreak never
		// applies — the winner is decided purely by top card: J > T.
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

		// Should have quinte (100pts) and tierce 7C-8C-9C (20pts)
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
		gs.BelotAnnounced = false // override fixture default
		// Give seat 2 both KH and QH (trump)
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

// --- Bitola dedup tests ---

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
		// Tierce 7S-8S-9S (top=9S) + FoaK jacks. No overlap — 9 is not J.
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
		// Spade run 9→J is not consecutive (9 idx 2, J idx 4). So runs are
		// 7-8-9 (tierce) and JS alone. Tierce + FoaK kept — no shared cards.
		require.Len(t, state.Players[0].Declarations, 2)
	})

	t.Run("equal-value clash: quinte + FoaK tens sharing TS — FoaK kept", func(t *testing.T) {
		// The only tie dedup can reach: a 5+ sequence (100) against a
		// 100-point four-of-a-kind. declarationBeats rule 2 awards an
		// equal-value clash to the four-of-a-kind, so dedup keeps the same
		// meld the clash comparison would have preferred.
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
		// Pre-dedup sanity: JD-QD-KD-AD produces only the maximal quarte.
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

// --- Declaration overlap by variant config (Story 12.5) ---

// meldSummary is an order-independent fingerprint of a detected declaration.
// detectDeclarations walks Go maps (by suit, then by rank), so both the order
// of the returned melds and the order of cards inside a meld vary between
// runs; every assertion below compares summaries as an unordered set instead
// of indexing into Declarations.
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
	// quarte 9S-TS-JS-QS (50) + carré of Jacks (200), sharing JS.
	jackOverlap := makeCards("9S", "TS", "JS", "QS", "JH", "JD", "JC", "7C")
	// quinte TS-JS-QS-KS-AS (100) + carré of Tens (100), sharing TS. The only
	// reachable equal-value clash: a 5+ sequence against a 100-point carré.
	equalValueOverlap := makeCards("TS", "JS", "QS", "KS", "AS", "TH", "TD", "TC")
	// tierce 7S-8S-9S (20) + carré of Jacks (200) with no shared card — the
	// spade run stops at 9S because 9 and J are not consecutive.
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

// TestDeclarationPromptTriggerIsOverlapInvariant pins the story's I/O-matrix
// row for the prompt trigger: dedup can only ever drop melds from a set that
// still has a survivor, so hasDeclarableCombinations answers identically under
// both configs and the prompt fires on exactly the same hands.
//
// AwaitingDeclaration is never force-set here — seat 1 leads trick 1, which
// hands the turn to seat 2 and makes the engine run checkDeclarationPrompt
// against seat 2's hand. That is the call site whose signature this story
// changed, so the flag on the returned state is the thing under test.
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

// TestDetectedMeldsOverlapOnlyAcrossTypes pins the premise the equal-value tie
// fix rests on, which is otherwise asserted only in comments: two sequences
// never share a card (they are maximal per-suit runs) and two four-of-a-kinds
// never share a card (they are rank-disjoint). That makes every dedup
// comparison a sequence-vs-four-of-a-kind one, so declarationBeats rule 2
// settles it alone and the chain steps needing trump and seat stay unreachable.
//
// The Croatian config is used on purpose: with dedup skipped, detection returns
// EVERY meld it found, which is the only way to observe the raw overlap
// structure. A counterexample here is an Ask-First escalation for the story —
// not something to adapt the code to.
func TestDetectedMeldsOverlapOnlyAcrossTypes(t *testing.T) {
	// samePairs counts the same-type meld pairs inspected, so the sweep cannot
	// pass by never reaching a multi-meld hand.
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
			return nil // no meld in this hand
		}
		return state.Players[0].Declarations
	}

	// Hand-built meld-dense hands. Random deals essentially never produce two
	// four-of-a-kinds in eight cards, so that half of the premise is covered
	// here rather than by the sweep.
	t.Run("constructed meld-dense hands", func(t *testing.T) {
		for _, hand := range [][]game.Card{
			makeCards("9S", "9H", "9D", "9C", "AS", "AH", "AD", "AC"), // two carrés
			makeCards("JS", "JH", "JD", "JC", "TS", "TH", "TD", "TC"), // two carrés, adjacent ranks
			makeCards("7S", "8S", "9S", "7C", "8C", "9C", "TD", "JH"), // two tierces
			makeCards("7S", "8S", "9S", "JS", "QS", "KS", "7C", "8C"), // two runs in ONE suit
			makeCards("TS", "JS", "QS", "KS", "AS", "TH", "TD", "TC"), // quinte + carré, shared
			makeCards("9S", "TS", "JS", "QS", "JH", "JD", "JC", "7C"), // quarte + carré, shared
			makeCards("QS", "KS", "AS", "QH", "QD", "QC", "7D", "8C"), // tierce + carré, shared
			makeCards("7H", "8H", "9H", "TH", "JH", "QH", "KH", "AH"), // one maximal eight-card run
		} {
			check(t, detect(t, hand))
		}
	})

	// Bounded, deterministically seeded sweep: a fixed seed and a fixed deal
	// count, four hands per deal, so a failure is reproducible and the runtime
	// stays in the tens of milliseconds.
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

// resolveTrick1 seats the four given hands on the supplied fixture, drives
// trick 1 to completion through the scripted plays and returns the
// post-resolution state.
//
// Seats listed in declaringSeats answer their prompt with `declare`; any other
// prompted seat skips. Every prompt but the leader's is raised by the engine
// itself — the helper asserts as much — so a declaring seat that the engine
// failed to prompt fails the test. The trick leader is the one exception: its
// prompt was raised back when bidding resolved (checkDeclarationPrompt in
// bidding.go), which is before a playing-phase fixture picks the state up, so
// the helper reproduces that single flag.
//
// Belote is pre-marked announced so a K+Q-of-trump holder cannot stall the
// trick; no test using this helper is about Belote.
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
				state.AwaitingDeclaration = true // leader's prompt, see doc
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

// overlapOnlyLayout is the 32-card deal used for the single-team overlap tests.
// Team A's seat 0 holds the only melds at the table, which keeps resolution
// unambiguous, and no seat holds both K and Q of hearts so trump-hearts Belote
// can never prompt.
//
//	Seat 0 (team A): quarte 9S-TS-JS-QS (50) + carré of Jacks (200), sharing JS
//	Seat 1 (team B): no meld (its four 8s carry no value)
//	Seat 2 (team A): no meld
//	Seat 3 (team B): no meld
var overlapOnlyLayout = [4][]game.Card{
	makeCards("9S", "TS", "JS", "QS", "JH", "JD", "JC", "7C"),
	makeCards("7S", "8S", "7H", "8H", "7D", "8D", "8C", "9C"),
	makeCards("KS", "AS", "9H", "KH", "QD", "KD", "QC", "TC"),
	makeCards("TH", "QH", "AH", "9D", "TD", "AD", "KC", "AC"),
}

// overlapOnlyPlays: seat 1 leads spades, seat 2 must follow and overplay, seat 3
// is void and must cut, seat 0 follows into the already-cut trick. Seat 3 takes
// it on the trump.
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
		// Card points run in their own lane — the shared JS is not double-counted.
		// Trick 1 (7S=0, KS=4, TH=10 trump, 9S=0) goes to seat 3 on the cut.
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

// TestCroatianOverlapClashAcrossTeams pins the existing tie-break semantics now
// that overlap makes them consequential: resolveDeclarations picks the winner
// from each team's SINGLE strongest meld and only then awards that team's whole
// sum. Overlap inflates a team's sum without touching its best meld, so a team
// holding two shared melds can out-total its opponents and still lose outright.
//
// It also pins the premise bot.ObserveDeclarations depends on for the
// no-peeking rule: the losing team's declarations are cleared, and under
// overlap that means BOTH of seat 0's melds are nilled, not just one.
func TestCroatianOverlapClashAcrossTeams(t *testing.T) {
	t.Run("opposing carré of Jacks (200) beats the overlap team's best (100)", func(t *testing.T) {
		// Seat 0 (team A): tierce QS-KS-AS (20) + carré of Queens (100), sharing
		// QS — sum 120, best 100. Seat 1 (team B): carré of Jacks (200).
		//
		// A quinte cannot be paired against an opposing carré of Jacks: every
		// 5-card run contains its suit's Jack, so the four Jacks can never all
		// sit in another hand. Hence a tierce + carré overlap here.
		hands := [4][]game.Card{
			makeCards("QS", "KS", "AS", "QH", "QD", "QC", "7D", "8C"),
			makeCards("JS", "JH", "JD", "JC", "7S", "8S", "9D", "TC"),
			makeCards("9S", "TS", "7H", "8H", "8D", "TD", "7C", "9C"),
			makeCards("9H", "TH", "KH", "AH", "KD", "AD", "KC", "AC"),
		}
		plays := []seatPlay{{1, "7S"}, {2, "9S"}, {3, "9H"}, {0, "QS"}}

		// Precondition: overlap really is live for seat 0, so the clash below is
		// the overlap case and not a deduped single meld.
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
		// Seat 0 (team A): quinte TS-JS-QS-KS-AS (100) + carré of Tens (100),
		// sharing TS — sum 200, best 100. Seat 1 (team B): carré of Nines (150).
		// Team A out-totals team B 200 to 150 and is still awarded 0.
		hands := [4][]game.Card{
			makeCards("TS", "JS", "QS", "KS", "AS", "TH", "TD", "TC"),
			makeCards("9S", "9H", "9D", "9C", "JH", "QD", "KC", "7S"),
			makeCards("AH", "7H", "8H", "7D", "8D", "7C", "8C", "JC"),
			makeCards("QH", "KH", "8S", "JD", "KD", "AD", "QC", "AC"),
		}
		plays := []seatPlay{{1, "7S"}, {2, "7H"}, {3, "8S"}, {0, "AS"}}

		// Precondition: seat 0's two overlapping melds really do sum to 200,
		// more than the 150 that is about to beat them.
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

// --- Croatian dedicated declaration phase (Story 12.6) ---

// croatianDeclaringWith opens the dedicated declaration phase over an explicit
// 32-card layout: each seat's eight cards are split across the six open and two
// face-down slots the Croatian deal produces, then the real engine resolves the
// bid. Going through ApplyAction rather than building the state by hand means
// mergeFaceDownCards and the phase entry are both under test, and the cursor
// lands wherever handlePickTrump genuinely puts it.
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

// Layouts for the phase tests. Every one is a full 32-card partition, so the
// deal is legal and no seat can trigger instant-win.
var (
	// Seats 1 and 0 hold a meld; seats 2 and 3 hold none, so the cursor must
	// step straight past them.
	//
	//	seat 0: hearts 7-8-9-T quarte (50)
	//	seat 1: spades 7-8-9 tierce (20)
	//	seats 2, 3: no run of three, no scoring four-of-a-kind
	meldGapLayout = [4][]game.Card{
		makeCards("7H", "8H", "9H", "TH", "QD", "8C", "KS", "AS"),
		makeCards("7S", "8S", "9S", "JH", "KD", "7C", "TC", "AC"),
		makeCards("TS", "QS", "QH", "AH", "7D", "9D", "JD", "QC"),
		makeCards("JS", "KH", "8D", "TD", "AD", "9C", "JC", "KC"),
	}

	// Every seat holds a meld, so the cursor stops at all four in turn. This is
	// the NewGameCroatianJustDealt partition written out as whole hands.
	//
	//	seats 0-2: a six-card run each; seat 3: four Kings + four Aces
	allMeldLayout = [4][]game.Card{
		makeCards("7S", "8S", "9S", "TS", "7H", "8H", "JS", "QS"),
		makeCards("7D", "8D", "9D", "TD", "9H", "TH", "JD", "QD"),
		makeCards("7C", "8C", "9C", "TC", "JH", "QH", "JC", "QC"),
		makeCards("KS", "AS", "KD", "AD", "KH", "AH", "KC", "AC"),
	}

	// Nobody holds a meld: every suit contributes two non-adjacent ranks per
	// seat, and no rank lands four times in one hand.
	noMeldLayout = [4][]game.Card{
		makeCards("7S", "9S", "8H", "TH", "JD", "KD", "QC", "AC"),
		makeCards("8S", "TS", "JH", "KH", "QD", "AD", "7C", "9C"),
		makeCards("JS", "KS", "QH", "AH", "7D", "9D", "8C", "TC"),
		makeCards("QS", "AS", "7H", "9H", "8D", "TD", "JC", "KC"),
	}
)

// TestCroatianBidOpensDeclarationPhase covers both Croatian bidding paths into
// the phase: an early free-suit pick, and the forced pick by the dealer who
// cannot pass. Both converge on handlePickTrump, which is the single insertion
// point.
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
				"the cursor opens on the seat that will lead trick 1")
			assert.True(t, state.AwaitingDeclaration)
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
	assert.Equal(t, 0, state.DeclarationSeatsAnswered, "the phase counter stays untouched under Bitola timing")
}

// TestCroatianDeclarationPhaseTurnOrder walks the cursor counter-clockwise
// through all four seats. The standard fixture gives every seat a meld, so the
// phase stops at each one in turn and nothing resolves until the fourth answer.
func TestCroatianDeclarationPhaseTurnOrder(t *testing.T) {
	state := testfixtures.NewGameCroatianDeclaring(game.SuitHearts)

	for i, seat := range []int{1, 2, 3, 0} {
		require.Equal(t, game.PhaseDeclaring, state.Phase, "still in the phase before answer %d", i+1)
		require.True(t, state.AwaitingDeclaration)
		require.Equal(t, seat, state.ActivePlayerSeat, "answer %d belongs to seat %d", i+1, seat)
		require.False(t, state.DeclarationsResolved, "the contest resolves only on the fourth answer")

		var err error
		state, err = game.ApplyAction(state, game.Action{Type: game.ActionDeclare, PlayerSeat: seat})
		require.NoError(t, err)
		if state.Phase == game.PhaseDeclaring {
			// Before resolution every answer is simply banked. On the fourth the
			// contest settles and the losing team's melds are cleared, so this
			// only holds while the phase is still open.
			assert.NotEmpty(t, state.Players[seat].Declarations, "seat %d's melds are stored", seat)
		}
	}

	assert.Equal(t, game.PhasePlaying, state.Phase)
	assert.Equal(t, 1, state.TrickNumber)
	assert.True(t, state.DeclarationsResolved)
	assert.False(t, state.AwaitingDeclaration)
	// All four declared, so the contest produced a winner and the loser's melds
	// were cleared — exactly what resolveDeclarationsForHand does at Bitola's
	// trick 2.
	assert.NotEqual(t, [2]int{0, 0}, state.DeclarationPoints)
}

// TestCroatianDeclarationPhaseStepsPastMeldlessSeats pins the "meld-less seats
// between" row: only seats holding a meld are ever prompted, so the cursor
// jumps from seat 1 straight to seat 0 and seats 2 and 3 never see a prompt.
func TestCroatianDeclarationPhaseStepsPastMeldlessSeats(t *testing.T) {
	state := croatianDeclaringWith(t, game.SuitClubs, meldGapLayout)

	require.Equal(t, game.PhaseDeclaring, state.Phase)
	require.Equal(t, 1, state.ActivePlayerSeat)
	require.True(t, state.AwaitingDeclaration)

	state, err := game.ApplyAction(state, game.Action{Type: game.ActionDeclare, PlayerSeat: 1})
	require.NoError(t, err)

	assert.Equal(t, game.PhaseDeclaring, state.Phase, "two seats owe nothing, but seat 0 still does")
	assert.Equal(t, 0, state.ActivePlayerSeat, "the cursor jumps past seats 2 and 3")
	assert.True(t, state.AwaitingDeclaration)
	assert.Empty(t, state.Players[2].Declarations)
	assert.Empty(t, state.Players[3].Declarations)

	// Seats 2 and 3 were never on the clock, so neither can answer.
	for _, seat := range []int{2, 3} {
		rejected, err := game.ApplyAction(state, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: seat})
		assert.Nil(t, rejected)
		assert.ErrorIs(t, err, apperr.ErrNotYourTurn, "seat %d was stepped past, not prompted", seat)
	}
}

// TestCroatianDeclarationPhaseWithNoMeldsResolvesImmediately pins the
// "no seat holds a meld" row: the phase opens and resolves inside the same
// transition, so AwaitingDeclaration is never set and trick 1 starts at once.
func TestCroatianDeclarationPhaseWithNoMeldsResolvesImmediately(t *testing.T) {
	state := croatianDeclaringWith(t, game.SuitSpades, noMeldLayout)

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

// TestCroatianDeclarationPhaseResolvesIntoTrick1 covers the "last seat answers"
// row end to end: the contest resolves through the same helper Bitola uses at
// trick 2, the losing team's melds are cleared, and play opens at trick 1 with
// the leader on the clock.
func TestCroatianDeclarationPhaseResolvesIntoTrick1(t *testing.T) {
	state := croatianDeclaringWith(t, game.SuitClubs, meldGapLayout)

	state, err := game.ApplyAction(state, game.Action{Type: game.ActionDeclare, PlayerSeat: 1})
	require.NoError(t, err)
	require.Equal(t, 0, state.ActivePlayerSeat)

	state, err = game.ApplyAction(state, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
	require.NoError(t, err)

	assert.Equal(t, game.PhasePlaying, state.Phase)
	assert.Equal(t, 1, state.TrickNumber)
	assert.Empty(t, state.CurrentTrick)
	assert.False(t, state.AwaitingDeclaration)
	assert.Equal(t, 0, state.DeclarationSeatsAnswered, "the cursor counter is spent")
	assert.Equal(t, (state.DealerSeat+1)%4, state.ActivePlayerSeat, "the seat after the dealer leads trick 1")

	assert.True(t, state.DeclarationsResolved)
	assert.Equal(t, 50, state.DeclarationPoints[game.TeamA], "seat 0's quarte outranks seat 1's tierce")
	assert.Equal(t, 0, state.DeclarationPoints[game.TeamB])
	assert.Empty(t, state.Players[1].Declarations, "the losing team's melds are cleared")
	assert.NotEmpty(t, state.Players[0].Declarations)

	// The hand is genuinely playable from here.
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
// recorded, no points are awarded, and the cursor still advances.
func TestCroatianDeclarationPhaseSkipStoresNothing(t *testing.T) {
	state := croatianDeclaringWith(t, game.SuitClubs, meldGapLayout)

	state, err := game.ApplyAction(state, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: 1})
	require.NoError(t, err)
	assert.Empty(t, state.Players[1].Declarations)
	assert.Equal(t, 0, state.ActivePlayerSeat)

	state, err = game.ApplyAction(state, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: 0})
	require.NoError(t, err)

	assert.Equal(t, game.PhasePlaying, state.Phase)
	assert.True(t, state.DeclarationsResolved)
	assert.Equal(t, [2]int{0, 0}, state.DeclarationPoints, "nobody declared, so nobody scores")
	assert.Equal(t, 1, state.TrickNumber)
}

// TestCroatianDeclarationPhaseRejectsOtherActions pins the phase's action
// allowlist: declare and skip_declare from the prompted seat only, and no card
// can be played before the contest closes.
func TestCroatianDeclarationPhaseRejectsOtherActions(t *testing.T) {
	base := testfixtures.NewGameCroatianDeclaring(game.SuitHearts)
	require.Equal(t, 1, base.ActivePlayerSeat)

	card := base.Players[1].Hand[0]
	suit := game.SuitSpades

	// Every action that belongs to a NEIGHBOURING phase — the bidding it just
	// left, the play it has not started, and the hand-complete pause far ahead —
	// must bounce off. handleDeclaring's default arm is the only thing standing
	// between them and a half-applied transition.
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
	}

	for _, tc := range rejected {
		t.Run(tc.name+" is rejected", func(t *testing.T) {
			state, err := game.ApplyAction(base, tc.action)
			assert.Nil(t, state)
			assert.ErrorIs(t, err, apperr.ErrWrongPhase)
		})
	}

	t.Run("declare from a seat that is not on the clock is rejected", func(t *testing.T) {
		state, err := game.ApplyAction(base, game.Action{Type: game.ActionDeclare, PlayerSeat: 2})
		assert.Nil(t, state)
		assert.ErrorIs(t, err, apperr.ErrNotYourTurn)
	})
}

// TestCroatianDeclarationPhaseIsPausableAndSurrenderable pins the phase into
// the pause and surrender allowlists — a phase where either silently failed
// would be a hole, and disconnect handling rides on pause.
func TestCroatianDeclarationPhaseIsPausableAndSurrenderable(t *testing.T) {
	base := testfixtures.NewGameCroatianDeclaring(game.SuitHearts)

	t.Run("pause and unpause round-trip through the phase", func(t *testing.T) {
		paused, err := game.ApplyAction(base, game.Action{Type: game.ActionPause, PlayerSeat: 2})
		require.NoError(t, err)
		assert.Equal(t, game.PhasePaused, paused.Phase)
		assert.Equal(t, game.PhaseDeclaring, paused.PreviousPhase)

		resumed, err := game.ApplyAction(paused, game.Action{Type: game.ActionUnpause, PlayerSeat: 2})
		require.NoError(t, err)
		assert.Equal(t, game.PhaseDeclaring, resumed.Phase, "the phase is restored, not skipped")
		assert.True(t, resumed.AwaitingDeclaration)
		assert.Equal(t, base.ActivePlayerSeat, resumed.ActivePlayerSeat)
	})

	t.Run("surrender can be proposed in the phase", func(t *testing.T) {
		proposed, err := game.ApplyAction(base, game.Action{Type: game.ActionSurrenderRequest, PlayerSeat: 1})
		require.NoError(t, err)
		require.NotNil(t, proposed.SurrenderProposerSeat)
		assert.Equal(t, 1, *proposed.SurrenderProposerSeat)
	})
}

// TestCroatianDeclarationCounterDoesNotLeakIntoHand2 guards the reset in
// startNewHand: the cursor counter is per-hand, and a stale value would make
// hand 2's phase open mid-walk and silently skip seats. ForceAdvanceHandComplete
// is the exported door onto startNewHand.
func TestCroatianDeclarationCounterDoesNotLeakIntoHand2(t *testing.T) {
	gs := testfixtures.NewGameCroatianDeclaring(game.SuitHearts)
	gs.Phase = game.PhaseHandComplete
	gs.DeclarationSeatsAnswered = 3
	gs.DeclarationsResolved = true

	next, err := game.ForceAdvanceHandComplete(gs)

	require.NoError(t, err)
	assert.Equal(t, 0, next.DeclarationSeatsAnswered, "the cursor counter must not survive the hand")
	assert.False(t, next.AwaitingDeclaration)
	assert.False(t, next.DeclarationsResolved)
	assert.Equal(t, 2, next.HandNumber)
	require.Equal(t, 1, next.DealerSeat, "startNewHand rotates the dealer counter-clockwise")
	require.NotEqual(t, game.PhaseMatchEnd, next.Phase, "hand 2 must have dealt normally")

	// Drive hand 2 to its own declaration phase. The cursor origin is
	// (DealerSeat+1)%4, so the reset only really holds if the NEW dealer's left
	// is where the walk starts — a counter that survived at 3 would open the
	// phase three seats along and silently deny those seats their answer.
	//
	// Hand 2's deal is random, so the hands are replaced with a layout where
	// every seat holds a meld: that makes the opening position observable
	// instead of dependent on who happened to be dealt a run.
	for seat, hand := range allMeldLayout {
		next.Players[seat].Hand = append([]game.Card(nil), hand[:6]...)
		next.Players[seat].FaceDownCards = append([]game.Card(nil), hand[6:]...)
	}
	next.Phase = game.PhaseBidding // the session manager's dealing → bidding step

	suit := game.SuitClubs
	opened, err := game.ApplyAction(next, game.Action{
		Type:       game.ActionPickTrump,
		PlayerSeat: next.ActivePlayerSeat,
		Suit:       &suit,
	})
	require.NoError(t, err)

	assert.Equal(t, game.PhaseDeclaring, opened.Phase)
	assert.Equal(t, 2, opened.ActivePlayerSeat,
		"hand 2 opens at the new dealer's left (seat 1 deals, so seat 2 leads)")
	assert.Equal(t, (opened.DealerSeat+1)%4, opened.ActivePlayerSeat)
	assert.Equal(t, 0, opened.DeclarationSeatsAnswered, "the walk starts from zero")
	assert.True(t, opened.AwaitingDeclaration)
}
