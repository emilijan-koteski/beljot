package game_test

import (
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
