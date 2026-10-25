package bot_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/bot"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
)

// viewFromState derives the redacted View the match layer would hand to
// Decide for the given seat. Mirrors buildBotView in match/bot_driver.go but
// stays test-local so the bot package tests carry no match dependency.
func viewFromState(gs *game.GameState, seat int, mem *bot.Memory) bot.View {
	if mem == nil {
		mem = bot.NewMemory()
	}
	v := bot.View{
		Seat:                seat,
		Hand:                gs.Players[seat].Hand,
		Phase:               gs.Phase,
		BiddingRound:        gs.BiddingRound,
		TrumpCandidate:      gs.TrumpCandidate,
		TrumpSuit:           gs.TrumpSuit,
		TrumpCallerSeat:     gs.TrumpCallerSeat,
		HasTrumpCandidate:   gs.Rules.HasTrumpCandidate,
		DealerSeat:          gs.DealerSeat,
		CurrentTrick:        gs.CurrentTrick,
		LeadSuit:            gs.LeadSuit,
		ActivePlayerSeat:    gs.ActivePlayerSeat,
		AwaitingDeclaration: gs.AwaitingDeclaration && gs.ActivePlayerSeat == seat,
		HasDeclarations:     game.HasDeclarableCombinations(gs, seat),
		PendingBelot:        gs.PendingBelotSeat != nil && *gs.PendingBelotSeat == seat,
		TeamScores:          gs.TeamScores,
		HandPoints:          gs.HandPoints,
		TricksWon:           gs.TricksWon,
		PlayedCards:         mem.PlayedCards(),
		KnownVoids:          mem.KnownVoids(),
		KnownCards:          mem.KnownCards(),
		DeclaredCards:       mem.DeclaredCards(),
	}
	if gs.SurrenderProposerSeat != nil && (*gs.SurrenderProposerSeat+2)%4 == seat {
		v.PartnerProposedSurrender = true
	}
	if gs.Phase == game.PhasePlaying {
		v.LegalCards = game.LegalCards(gs, seat)
	}
	v.MustPickTrump = game.MustPickTrump(gs, seat)
	return v
}

func card(id string) game.Card {
	c, err := game.ParseCard(id)
	if err != nil {
		panic(err)
	}
	return c
}

func cards(ids ...string) []game.Card {
	out := make([]game.Card, len(ids))
	for i, id := range ids {
		out[i] = card(id)
	}
	return out
}

func TestDecide_Bidding(t *testing.T) {
	tests := []struct {
		name        string
		seat        int
		hand        []game.Card
		round       int
		candidate   string
		noCandidate bool
		faceDown    []game.Card
		forcedPick  bool
		wantType    string
		wantSuit    *game.Suit
	}{
		{
			name:     "round 1 five of candidate suit picks",
			seat:     1,
			hand:     cards("7H", "8H", "9H", "TH", "JH"),
			round:    1,
			wantType: game.ActionPickTrump,
		},
		{
			name:     "round 1 four of candidate suit picks",
			seat:     1,
			hand:     cards("7H", "8H", "QH", "KH", "7S"),
			round:    1,
			wantType: game.ActionPickTrump,
		},
		{
			name:     "round 1 candidate completes jack+nine to a qualifying three picks",
			seat:     1,
			hand:     cards("JH", "9H", "7S", "8S", "9S"),
			round:    1,
			wantType: game.ActionPickTrump,
		},
		{
			name:      "round 1 candidate completes to three with a seven passes",
			seat:      1,
			hand:      cards("JH", "9H", "7S", "8S", "9S"),
			round:     1,
			candidate: "7H",
			wantType:  game.ActionPassTrump,
		},
		{
			name:      "round 1 three with jack and a seven and eight passes",
			seat:      1,
			hand:      cards("JH", "7H", "7S", "8S", "9S"),
			round:     1,
			candidate: "8H",
			wantType:  game.ActionPassTrump,
		},
		{
			name:      "round 1 three with jack and a seven and eight but a side ace picks",
			seat:      1,
			hand:      cards("JH", "8H", "AS", "7C", "8C"),
			round:     1,
			candidate: "7H",
			wantType:  game.ActionPickTrump,
		},
		{
			name:      "round 1 three with jack and a seven and eight and no side ace passes",
			seat:      1,
			hand:      cards("JH", "8H", "KS", "7C", "8C"),
			round:     1,
			candidate: "7H",
			wantType:  game.ActionPassTrump,
		},
		{
			name:      "round 1 nine ace pair with a side ace picks",
			seat:      1,
			hand:      cards("9H", "AH", "AS", "8S", "KD"),
			round:     1,
			candidate: "QH",
			wantType:  game.ActionPickTrump,
		},
		{
			name:      "round 1 nine ace pair with a lone side ace picks",
			seat:      1,
			hand:      cards("9H", "AH", "AS", "8D", "KC"),
			round:     1,
			candidate: "QH",
			wantType:  game.ActionPickTrump,
		},
		{
			name:      "round 1 nine ace pair with no side ace passes",
			seat:      1,
			hand:      cards("9H", "AH", "KS", "8D", "KC"),
			round:     1,
			candidate: "QH",
			wantType:  game.ActionPassTrump,
		},
		{
			name:      "round 1 two trumps jack and nine with a side ace picks",
			seat:      1,
			hand:      cards("JH", "AS", "7C", "8C", "7D"),
			round:     1,
			candidate: "9H",
			wantType:  game.ActionPickTrump,
		},
		{
			name:      "round 1 two trumps jack and nine without a side ace passes",
			seat:      1,
			hand:      cards("JH", "7C", "8C", "7D", "8D"),
			round:     1,
			candidate: "9H",
			wantType:  game.ActionPassTrump,
		},
		{
			name:      "round 1 two trumps jack and ace does not qualify",
			seat:      1,
			hand:      cards("JH", "AS", "7C", "8C", "7D"),
			round:     1,
			candidate: "AH",
			wantType:  game.ActionPassTrump,
		},
		{
			name:      "round 1 three without jack or nine+ace passes",
			seat:      1,
			hand:      cards("KH", "QH", "7S", "8S", "9S"),
			round:     1,
			candidate: "TH",
			wantType:  game.ActionPassTrump,
		},
		{
			name:     "round 1 junk hand passes",
			seat:     0,
			hand:     cards("7S", "8S", "9S", "TS", "JS"),
			round:    1,
			wantType: game.ActionPassTrump,
		},
		{
			name:     "round 2 qualifying side suit picked",
			seat:     1,
			hand:     cards("JS", "9S", "AS", "9C", "AC"),
			round:    2,
			wantType: game.ActionPickTrump,
			wantSuit: suitPtr(game.SuitSpades),
		},
		{
			name:      "round 2 candidate side card completes a nine ace pick",
			seat:      1,
			hand:      cards("9D", "AD", "KD", "AC", "7H"),
			round:     2,
			candidate: "TC",
			wantType:  game.ActionPickTrump,
			wantSuit:  suitPtr(game.SuitDiamonds),
		},
		{
			name:     "round 2 candidate suit locked out",
			seat:     1,
			hand:     cards("7H", "8H", "9H", "TH", "JH"),
			round:    2,
			wantType: game.ActionPassTrump,
		},
		{
			name:     "round 2 junk passes",
			seat:     1,
			hand:     cards("7S", "8D", "9C", "TS", "KD"),
			round:    2,
			wantType: game.ActionPassTrump,
		},
		{
			name:        "no candidate round 1 picks and carries the suit",
			seat:        1,
			hand:        cards("7S", "8S", "9S", "TS", "JS", "7H"),
			round:       1,
			noCandidate: true,
			faceDown:    cards("7D", "8D"),
			wantType:    game.ActionPickTrump,
			wantSuit:    suitPtr(game.SuitSpades),
		},
		{
			name:        "no candidate round 1 junk passes",
			seat:        1,
			hand:        cards("7S", "8H", "9D", "TC", "QS", "KH"),
			round:       1,
			noCandidate: true,
			faceDown:    cards("7D", "8D"),
			wantType:    game.ActionPassTrump,
		},
		{
			name:        "no candidate does not count the hidden pair",
			seat:        1,
			hand:        cards("7D", "8D", "9D", "7S", "8S", "7H"),
			round:       1,
			noCandidate: true,
			faceDown:    cards("TD", "JD"),
			wantType:    game.ActionPassTrump,
		},
		{
			name:        "forced dealer picks its best suit rather than passing",
			seat:        0,
			hand:        cards("7D", "8D", "JD", "7S", "8H", "9C"),
			round:       1,
			noCandidate: true,
			faceDown:    cards("QC", "KH"),
			forcedPick:  true,
			wantType:    game.ActionPickTrump,
			wantSuit:    suitPtr(game.SuitDiamonds),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gs *game.GameState
			if tt.noCandidate {
				gs = testfixtures.NewGameCroatianJustDealt()
				if tt.faceDown != nil {
					gs.Players[tt.seat].FaceDownCards = tt.faceDown
				}
				if tt.forcedPick {
					gs.BiddingPassCount = 3
					gs.DealerSeat = tt.seat
				}
			} else {
				gs = testfixtures.NewGameJustDealt()
			}
			gs.BiddingRound = tt.round
			gs.ActivePlayerSeat = tt.seat
			gs.Players[tt.seat].Hand = tt.hand
			if tt.candidate != "" {
				c := card(tt.candidate)
				gs.TrumpCandidate = &c
			}
			require.Equal(t, tt.forcedPick, game.MustPickTrump(gs, tt.seat),
				"the fixture must match the row's forcedPick expectation")

			action := bot.Decide(viewFromState(gs, tt.seat, nil))

			assert.Equal(t, tt.wantType, action.Type)
			assert.Equal(t, tt.seat, action.PlayerSeat)
			if tt.round == 1 && gs.TrumpCandidate != nil && tt.wantType == game.ActionPickTrump {
				assert.Nil(t, action.Suit, "round 1 pick on a candidate carries no suit")
			}
			if tt.wantSuit != nil {
				require.NotNil(t, action.Suit)
				assert.Equal(t, *tt.wantSuit, *action.Suit)
			}
		})
	}
}

// TestDecide_ForcedDealerNeverPasses checks that no hand produces pass_trump
// in the state where the engine refuses a pass.
func TestDecide_ForcedDealerNeverPasses(t *testing.T) {
	for seat := 0; seat < 4; seat++ {
		gs := testfixtures.NewGameCroatianMidBidding(3)
		gs.DealerSeat = seat
		gs.ActivePlayerSeat = seat
		require.True(t, game.MustPickTrump(gs, seat), "seat %d must be the forced picker", seat)

		action := bot.Decide(viewFromState(gs, seat, nil))
		require.Equal(t, game.ActionPickTrump, action.Type,
			"seat %d: a pass here is rejected by the engine, forever", seat)
		require.NotNil(t, action.Suit, "seat %d: a free-suit pick must carry a suit", seat)

		_, err := game.ApplyAction(gs, action)
		require.NoError(t, err, "seat %d: the forced pick must be legal", seat)
	}
}

func suitPtr(s game.Suit) *game.Suit { return &s }

func TestDecide_AlwaysDeclares(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(1)
	gs.AwaitingDeclaration = true
	gs.ActivePlayerSeat = 0

	action := bot.Decide(viewFromState(gs, 0, nil))
	assert.Equal(t, game.ActionDeclare, action.Type)
	assert.Equal(t, 0, action.PlayerSeat)
}

// TestDecide_AnswersDedicatedDeclarationPhase checks the bot declares or skips on its own in the Croatian phase,
// without falling through to card play, where LegalCards is nil and would panic.
func TestDecide_AnswersDedicatedDeclarationPhase(t *testing.T) {
	gs := testfixtures.NewGameCroatianDeclaring(game.SuitHearts)
	require.Equal(t, game.PhaseDeclaring, gs.Phase)
	require.False(t, gs.AwaitingDeclaration, "the phase never puts a seat on the clock")
	require.Nil(t, viewFromState(gs, gs.ActivePlayerSeat, nil).LegalCards,
		"the phase has no legal cards — that is the panic this guard prevents")

	t.Run("a seat holding a meld declares", func(t *testing.T) {
		for seat := 0; seat < 4; seat++ {
			require.True(t, game.HasDeclarableCombinations(gs, seat), "fixture: seat %d holds a meld", seat)
			action := bot.Decide(viewFromState(gs, seat, nil))
			assert.Equal(t, game.ActionDeclare, action.Type, "seat %d", seat)
			assert.Equal(t, seat, action.PlayerSeat)

			_, err := game.ApplyAction(gs, action)
			require.NoError(t, err, "seat %d: the bot's declare must be legal", seat)
		}
	})

	t.Run("a seat holding nothing skips", func(t *testing.T) {
		bare := testfixtures.NewGameCroatianDeclaring(game.SuitHearts)
		for seat := range bare.Players {
			bare.Players[seat].Hand = nil
		}
		for seat := 0; seat < 4; seat++ {
			require.False(t, game.HasDeclarableCombinations(bare, seat))
			action := bot.Decide(viewFromState(bare, seat, nil))
			assert.Equal(t, game.ActionSkipDeclare, action.Type, "seat %d", seat)
		}
	})

	t.Run("no seat ever reaches for a card", func(t *testing.T) {
		for seat := 0; seat < 4; seat++ {
			action := bot.Decide(viewFromState(gs, seat, nil))
			assert.NotEqual(t, game.ActionPlayCard, action.Type, "seat %d", seat)
		}
	})
}

func TestDecide_AlwaysAnnouncesBelot(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(1)
	seat := 0
	gs.PendingBelotSeat = &seat

	action := bot.Decide(viewFromState(gs, 0, nil))
	assert.Equal(t, game.ActionAnnounceBelot, action.Type)
}

func TestDecide_AcceptsPartnerSurrender(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(1)
	proposer := 2
	gs.SurrenderProposerSeat = &proposer
	gs.ActivePlayerSeat = 0

	action := bot.Decide(viewFromState(gs, 0, nil))
	assert.Equal(t, game.ActionSurrenderAccept, action.Type,
		"surrender accept takes priority over card play")
}

func TestDecide_CardPlay(t *testing.T) {
	tests := []struct {
		name           string
		hand           []game.Card
		trick          []game.TrickCard
		callerSeat     int
		played         []string
		declaredBySeat map[int][]string
		wantCard       string
	}{
		{
			name: "smear keeps the unprotected ace boss when partner trumped",
			hand: cards("AS", "QS", "7S"),
			trick: []game.TrickCard{
				{Card: card("8S"), PlayerSeat: 1},
				{Card: card("KH"), PlayerSeat: 2},
				{Card: card("7D"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "QS",
		},
		{
			name: "smear low and keep the promoted ten when partner cashes the ace",
			hand: cards("TS", "JS", "8S"),
			trick: []game.TrickCard{
				{Card: card("QS"), PlayerSeat: 1},
				{Card: card("AS"), PlayerSeat: 2},
				{Card: card("7D"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "JS",
		},
		{
			name: "keep cheap when partner win is contestable",
			hand: cards("9S", "8S", "7H"),
			trick: []game.TrickCard{
				{Card: card("QS"), PlayerSeat: 2},
				{Card: card("7D"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "8S",
		},
		{
			name: "win as cheaply as possible over an opponent",
			hand: cards("AS", "TS", "7C"),
			trick: []game.TrickCard{
				{Card: card("QS"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "TS",
		},
		{
			name: "discard lowest when the trick is lost",
			hand: cards("9S", "TS", "7C"),
			trick: []game.TrickCard{
				{Card: card("AS"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "9S",
		},
		{
			name: "void with no trump discards the weakest card",
			hand: cards("KD", "9C", "7C"),
			trick: []game.TrickCard{
				{Card: card("AS"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "7C",
		},
		{
			name:       "draw trumps with the jack when own team called",
			hand:       cards("JH", "9H", "AS", "7C"),
			trick:      nil,
			callerSeat: 0,
			wantCard:   "JH",
		},
		{
			name:       "lead the side-suit boss when opponents called",
			hand:       cards("AS", "KD", "7C", "8H"),
			trick:      nil,
			callerSeat: 1,
			wantCard:   "AS",
		},
		{
			name:       "promoted king leads once the ace and ten are gone",
			hand:       cards("KS", "7C", "8H"),
			trick:      nil,
			callerSeat: 1,
			played:     []string{"AS", "TS"},
			wantCard:   "KS",
		},
		{
			name:       "no boss with a ten leads a zero-point card not the ten",
			hand:       cards("TS", "9C", "7D"),
			trick:      nil,
			callerSeat: 1,
			wantCard:   "7D",
		},
		{
			name:       "caller without the master trump cashes a side ace not the trump ace",
			hand:       cards("AH", "AD", "7C"),
			trick:      nil,
			callerSeat: 0,
			wantCard:   "AD",
		},
		{
			name:       "caller without the master trump and no boss leads safe low not high trump",
			hand:       cards("AH", "KD", "7C"),
			trick:      nil,
			callerSeat: 0,
			wantCard:   "7C",
		},
		{
			name:           "declared: opponents void of trump, cash side ace not the master trump",
			hand:           cards("JH", "AS", "7C"),
			trick:          nil,
			callerSeat:     0,
			played:         []string{"QH", "KH", "AH"},
			declaredBySeat: map[int][]string{2: {"7H", "8H", "9H", "TH"}},
			wantCard:       "AS",
		},
		{
			name:           "declared: partner holds the master so the bot draws with its best trump",
			hand:           cards("9H", "7C"),
			trick:          nil,
			callerSeat:     0,
			declaredBySeat: map[int][]string{2: {"JH", "QH", "KH", "AH"}},
			wantCard:       "9H",
		},
		{
			name:           "declared: opponent ace keeps the king from being a false boss",
			hand:           cards("KS", "7C", "8D"),
			trick:          nil,
			callerSeat:     1,
			declaredBySeat: map[int][]string{1: {"AS", "AH", "AD", "AC"}},
			wantCard:       "7C",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := testfixtures.NewGameMidPlay(1)
			gs.Players[0].Hand = tt.hand
			gs.CurrentTrick = tt.trick
			gs.ActivePlayerSeat = 0
			caller := tt.callerSeat
			gs.TrumpCallerSeat = &caller
			if len(tt.trick) > 0 {
				lead := tt.trick[0].Card.Suit
				gs.LeadSuit = &lead
			}

			mem := bot.NewMemory()
			for _, id := range tt.played {
				mem.ObservePlay(1, card(id), nil, gs.TrumpSuit)
			}
			for seat, ids := range tt.declaredBySeat {
				gs.Players[seat].Declarations = []game.Declaration{{
					Type:       game.DeclarationSequence,
					Cards:      cards(ids...),
					PlayerSeat: seat,
				}}
			}
			if len(tt.declaredBySeat) > 0 {
				mem.ObserveDeclarations(gs.Players)
			}

			action := bot.Decide(viewFromState(gs, 0, mem))

			require.Equal(t, game.ActionPlayCard, action.Type)
			require.NotNil(t, action.Card)
			assert.Equal(t, tt.wantCard, action.Card.String())
		})
	}
}

// TestDecide_PartnerTakesTrick checks the bot ducks a trick its partner is guaranteed to take,
// which holds only when the partner is provably void in the led suit and can ruff last.
func TestDecide_PartnerTakesTrick(t *testing.T) {
	tests := []struct {
		name            string
		hand            []game.Card
		partnerVoidDiam bool
		playedHearts    []string
		wantCard        string
	}{
		{
			name:            "partner void in led with a threat-proof trump: smear the high card",
			partnerVoidDiam: true,
			wantCard:        "KD",
		},
		{
			name:            "partner takes the trick: smear keeps the promoted ten",
			hand:            cards("TD", "JD", "7D"),
			partnerVoidDiam: true,
			wantCard:        "JD",
		},
		{
			name:            "partner not provably void in led: do not duck, discard low",
			partnerVoidDiam: false,
			wantCard:        "7D",
		},
		{
			name:            "partner's threat-proof trumps already played: do not duck on a stale holding",
			partnerVoidDiam: true,
			playedHearts:    []string{"JH", "9H"},
			wantCard:        "7D",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := testfixtures.NewGameMidPlay(1)
			hand := tt.hand
			if hand == nil {
				hand = cards("KD", "7D")
			}
			gs.Players[0].Hand = hand
			gs.CurrentTrick = []game.TrickCard{{Card: card("AD"), PlayerSeat: 3}}
			lead := game.SuitDiamonds
			gs.LeadSuit = &lead
			gs.ActivePlayerSeat = 0
			gs.Players[2].Declarations = []game.Declaration{{
				Type:       game.DeclarationSequence,
				Cards:      cards("9H", "TH", "JH", "QH"),
				PlayerSeat: 2,
			}}

			mem := bot.NewMemory()
			if tt.partnerVoidDiam {
				mem.ObservePlay(2, card("8C"), &lead, nil)
			}
			for _, id := range tt.playedHearts {
				mem.ObservePlay(2, card(id), nil, gs.TrumpSuit)
			}
			mem.ObserveDeclarations(gs.Players)

			action := bot.Decide(viewFromState(gs, 0, mem))

			require.Equal(t, game.ActionPlayCard, action.Type)
			require.NotNil(t, action.Card)
			assert.Equal(t, tt.wantCard, action.Card.String())
		})
	}
}

// TestDecide_LastTrickRetention checks the bot keeps the master trump back for trick 8
// at the second-to-last trick instead of spending it now.
func TestDecide_LastTrickRetention(t *testing.T) {
	tests := []struct {
		name           string
		hand           []game.Card
		trick          []game.TrickCard
		callerSeat     int
		played         []string
		declaredBySeat map[int][]string
		wantCard       string
	}{
		{
			name:       "lead retains the master trump and spends the junk",
			hand:       cards("JH", "7C"),
			trick:      nil,
			callerSeat: 0,
			wantCard:   "7C",
		},
		{
			name:       "non-absolute master is retained once higher trumps are played",
			hand:       cards("9H", "7C"),
			trick:      nil,
			callerSeat: 0,
			played:     []string{"JH"},
			wantCard:   "7C",
		},
		{
			name:       "forced cut plays the master — retention never overrides a forced play",
			hand:       cards("JH", "7C"),
			trick:      []game.TrickCard{{Card: card("AD"), PlayerSeat: 3}},
			callerSeat: 0,
			wantCard:   "JH",
		},
		{
			name:       "cut with the lower trump and keep the master",
			hand:       cards("JH", "9H"),
			trick:      []game.TrickCard{{Card: card("7D"), PlayerSeat: 3}},
			callerSeat: 0,
			wantCard:   "9H",
		},
		{
			name:           "partner controls the last trick so retention defers and the bot draws",
			hand:           cards("9H", "7C"),
			trick:          nil,
			callerSeat:     0,
			declaredBySeat: map[int][]string{2: {"JH", "QH", "KH", "AH"}},
			wantCard:       "9H",
		},
		{
			name: "follow over a winning partner spends the lower trump and keeps the master",
			hand: cards("JH", "9H"),
			trick: []game.TrickCard{
				{Card: card("8D"), PlayerSeat: 1},
				{Card: card("7H"), PlayerSeat: 2},
				{Card: card("9C"), PlayerSeat: 3},
			},
			callerSeat: 0,
			wantCard:   "9H",
		},
		{
			name:       "not the endgame leaves draw behavior unchanged",
			hand:       cards("JH", "AS", "7C"),
			trick:      nil,
			callerSeat: 0,
			wantCard:   "JH",
		},
		{
			name:       "no master trump cashes the side boss now",
			hand:       cards("AS", "7C"),
			trick:      nil,
			callerSeat: 1,
			wantCard:   "AS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := testfixtures.NewGameMidPlay(1)
			gs.Players[0].Hand = tt.hand
			gs.CurrentTrick = tt.trick
			gs.ActivePlayerSeat = 0
			caller := tt.callerSeat
			gs.TrumpCallerSeat = &caller
			if len(tt.trick) > 0 {
				lead := tt.trick[0].Card.Suit
				gs.LeadSuit = &lead
			}

			mem := bot.NewMemory()
			for _, id := range tt.played {
				mem.ObservePlay(1, card(id), nil, gs.TrumpSuit)
			}
			for seat, ids := range tt.declaredBySeat {
				gs.Players[seat].Declarations = []game.Declaration{{
					Type:       game.DeclarationSequence,
					Cards:      cards(ids...),
					PlayerSeat: seat,
				}}
			}
			if len(tt.declaredBySeat) > 0 {
				mem.ObserveDeclarations(gs.Players)
			}

			action := bot.Decide(viewFromState(gs, 0, mem))

			require.Equal(t, game.ActionPlayCard, action.Type)
			require.NotNil(t, action.Card)
			assert.Equal(t, tt.wantCard, action.Card.String())
		})
	}
}

// obs is one observed play fed to bot Memory. lead is the OLD trick's lead suit
// (nil when the card led the trick); a non-follow marks the seat void in lead.
type obs struct {
	seat int
	card string
	lead *game.Suit
}

// playTweakCase drives bot.Decide for seat 0 (team A) with trump = Hearts
// (NewGameMidPlay), giving full control over the trick, the trump caller, the
// observed plays (and thus inferred voids), and the public declarations.
type playTweakCase struct {
	name           string
	hand           []game.Card
	trick          []game.TrickCard
	callerSeat     int
	observes       []obs
	located        map[int][]string
	declaredBySeat map[int][]string
	wantCard       string
}

func runPlayTweakCases(t *testing.T, tests []playTweakCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := testfixtures.NewGameMidPlay(1)
			gs.Players[0].Hand = tt.hand
			gs.CurrentTrick = tt.trick
			gs.ActivePlayerSeat = 0
			caller := tt.callerSeat
			gs.TrumpCallerSeat = &caller
			if len(tt.trick) > 0 {
				lead := tt.trick[0].Card.Suit
				gs.LeadSuit = &lead
			}

			mem := bot.NewMemory()
			for _, o := range tt.observes {
				mem.ObservePlay(o.seat, card(o.card), o.lead, gs.TrumpSuit)
			}
			for seat, ids := range tt.located {
				for _, id := range ids {
					mem.ObserveLocatedCard(seat, card(id))
				}
			}
			for seat, ids := range tt.declaredBySeat {
				gs.Players[seat].Declarations = []game.Declaration{{
					Type:       game.DeclarationSequence,
					Cards:      cards(ids...),
					PlayerSeat: seat,
				}}
			}
			if len(tt.declaredBySeat) > 0 {
				mem.ObserveDeclarations(gs.Players)
			}

			action := bot.Decide(viewFromState(gs, 0, mem))
			require.Equal(t, game.ActionPlayCard, action.Type)
			require.NotNil(t, action.Card)
			assert.Equal(t, tt.wantCard, action.Card.String())
		})
	}
}

// TestDecide_DrawTrumpsForPartner checks the bot leads its highest trump when the partner called trump,
// unless the partner is void in trump or a known opponent holding outranks it.
func TestDecide_DrawTrumpsForPartner(t *testing.T) {
	hearts := game.SuitHearts
	runPlayTweakCases(t, []playTweakCase{
		{
			name:       "partner called with nine and junk: lead the low trump and keep the nine",
			hand:       cards("9H", "8H", "AS", "7C"),
			callerSeat: 2,
			wantCard:   "8H",
		},
		{
			name:       "partner called with honors: sacrifice the queen first",
			hand:       cards("QH", "KH", "AS", "7C"),
			callerSeat: 2,
			wantCard:   "QH",
		},
		{
			name:       "partner called with ten and ace: lead the ten before the ace",
			hand:       cards("TH", "AH", "8S", "7C"),
			callerSeat: 2,
			wantCard:   "TH",
		},
		{
			name:       "partner called with nine and ace: lead the ace and keep the nine",
			hand:       cards("9H", "AH", "8S", "7C"),
			callerSeat: 2,
			wantCard:   "AH",
		},
		{
			name:       "partner called with a lone nine: do not draw, cash the side boss",
			hand:       cards("9H", "AS", "8D", "7C"),
			callerSeat: 2,
			wantCard:   "AS",
		},
		{
			name:       "partner void in trump: do not draw, cash the side boss",
			hand:       cards("9H", "8H", "AS", "7C"),
			callerSeat: 2,
			observes:   []obs{{seat: 2, card: "8D", lead: &hearts}},
			wantCard:   "AS",
		},
		{
			name:           "opponent declared the jack: drop the draw, cash the side boss",
			hand:           cards("9H", "8H", "AS", "7C"),
			callerSeat:     2,
			declaredBySeat: map[int][]string{1: {"JH", "JS", "JD", "JC"}},
			wantCard:       "AS",
		},
		{
			name:       "bot itself called: no partner-draw, cash the side boss",
			hand:       cards("9H", "8H", "AS", "7C"),
			callerSeat: 0,
			wantCard:   "AS",
		},
		{
			name:       "opponent called: not our trump, cash the side boss",
			hand:       cards("9H", "8H", "AS", "7C"),
			callerSeat: 1,
			wantCard:   "AS",
		},
		{
			name:       "bot holds the master: existing draw-with-master leads the jack",
			hand:       cards("JH", "9H", "AS", "7C"),
			callerSeat: 2,
			wantCard:   "JH",
		},
		{
			name:       "partner called with low trumps and no boss: lead the lowest non-nine trump",
			hand:       cards("8H", "7H", "KD", "7C"),
			callerSeat: 2,
			wantCard:   "7H",
		},
		{
			name:           "partner's declared tierce proves the opponents hold the master: do not draw",
			hand:           cards("QH", "KH", "AS", "7C"),
			callerSeat:     2,
			declaredBySeat: map[int][]string{2: {"8H", "9H", "TH"}},
			wantCard:       "AS",
		},
		{
			name:           "partner's declared run includes the jack: still draw for the partner",
			hand:           cards("QH", "KH", "AS", "7C"),
			callerSeat:     2,
			declaredBySeat: map[int][]string{2: {"JH", "TH", "9H"}},
			wantCard:       "QH",
		},
		{
			name:       "partner's located trumps are no declared run: still draw for the partner",
			hand:       cards("QH", "KH", "AS", "7C"),
			callerSeat: 2,
			located:    map[int][]string{2: {"8H", "9H", "TH"}},
			wantCard:   "QH",
		},
	})
}

// TestDecide_PreserveBossOnForcedOvertake checks that when every legal card overtakes a safe partner win,
// the bot captures with its strongest card but keeps the boss back.
func TestDecide_PreserveBossOnForcedOvertake(t *testing.T) {
	runPlayTweakCases(t, []playTweakCase{
		{
			name: "forced over partner with ace and ten smears the ace",
			hand: cards("AS", "TS", "7H"),
			trick: []game.TrickCard{
				{Card: card("8S"), PlayerSeat: 1},
				{Card: card("QS"), PlayerSeat: 2},
				{Card: card("7S"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "AS",
		},
		{
			name: "forced over partner with the ace boss but no ten keeps the ace",
			hand: cards("AS", "KS", "7H"),
			trick: []game.TrickCard{
				{Card: card("8S"), PlayerSeat: 1},
				{Card: card("QS"), PlayerSeat: 2},
				{Card: card("7S"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "KS",
		},
		{
			name: "promoted king boss drops to the queen",
			hand: cards("KS", "QS", "7H"),
			trick: []game.TrickCard{
				{Card: card("8S"), PlayerSeat: 1},
				{Card: card("JS"), PlayerSeat: 2},
				{Card: card("7S"), PlayerSeat: 3},
			},
			callerSeat: 1,
			observes:   []obs{{seat: 1, card: "AS"}, {seat: 1, card: "TS"}},
			wantCard:   "QS",
		},
		{
			name: "strongest played when it is not the boss",
			hand: cards("KS", "QS", "7H"),
			trick: []game.TrickCard{
				{Card: card("8S"), PlayerSeat: 1},
				{Card: card("JS"), PlayerSeat: 2},
				{Card: card("7S"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "KS",
		},
		{
			name: "forced trump over partner keeps the master jack",
			hand: cards("JH", "9H", "7C"),
			trick: []game.TrickCard{
				{Card: card("QD"), PlayerSeat: 1},
				{Card: card("AD"), PlayerSeat: 2},
				{Card: card("KD"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "9H",
		},
	})
}

// TestDecide_LastToPlayBanksHighest checks the last player winning by following a side suit
// banks its highest-point winner instead of the cheapest.
func TestDecide_LastToPlayBanksHighest(t *testing.T) {
	runPlayTweakCases(t, []playTweakCase{
		{
			name: "last with ace and ten over the king banks the ace",
			hand: cards("AS", "TS", "7C"),
			trick: []game.TrickCard{
				{Card: card("KS"), PlayerSeat: 1},
				{Card: card("7D"), PlayerSeat: 2},
				{Card: card("8S"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "AS",
		},
		{
			name: "not last over the king wins cheaply with the ten",
			hand: cards("AS", "TS", "7C"),
			trick: []game.TrickCard{
				{Card: card("KS"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "TS",
		},
		{
			name: "last but only a ruff wins keeps the cheapest trump",
			hand: cards("TH", "9H", "7C"),
			trick: []game.TrickCard{
				{Card: card("KS"), PlayerSeat: 1},
				{Card: card("7D"), PlayerSeat: 2},
				{Card: card("8S"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "TH",
		},
		{
			name: "last and cannot win discards the lowest",
			hand: cards("7C", "8D", "9C"),
			trick: []game.TrickCard{
				{Card: card("AS"), PlayerSeat: 1},
				{Card: card("7D"), PlayerSeat: 2},
				{Card: card("8S"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "7C",
		},
	})
}

// TestDecide_LeadIntoPartnerVoid checks the bot leads the lowest card of a side suit the partner is void in
// while the partner can still ruff.
func TestDecide_LeadIntoPartnerVoid(t *testing.T) {
	diamonds := game.SuitDiamonds
	spades := game.SuitSpades
	hearts := game.SuitHearts
	runPlayTweakCases(t, []playTweakCase{
		{
			name:       "partner void in diamonds with trump: lead the lowest diamond",
			hand:       cards("7D", "KD", "9C"),
			callerSeat: 1,
			observes:   []obs{{seat: 2, card: "7C", lead: &diamonds}},
			wantCard:   "7D",
		},
		{
			name:       "partner also void in trump: cannot ruff, lead safe low",
			hand:       cards("8C", "KD", "9S"),
			callerSeat: 1,
			observes: []obs{
				{seat: 2, card: "7C", lead: &diamonds},
				{seat: 2, card: "8D", lead: &hearts},
			},
			wantCard: "8C",
		},
		{
			name:       "bot void in the partner's void suit: cannot feed, lead safe low",
			hand:       cards("KD", "9C", "7H"),
			callerSeat: 1,
			observes:   []obs{{seat: 2, card: "7C", lead: &spades}},
			wantCard:   "9C",
		},
		{
			name:       "own boss outranks feeding the ruff: cash the ace first",
			hand:       cards("AS", "7D", "9C"),
			callerSeat: 1,
			observes:   []obs{{seat: 2, card: "7C", lead: &diamonds}},
			wantCard:   "AS",
		},
	})
}

// TestDecide_SmearOntoPartnerBoss checks the bot smears points onto a partner-led boss when following third,
// unless the last opponent is known void in the led suit.
func TestDecide_SmearOntoPartnerBoss(t *testing.T) {
	spades := game.SuitSpades
	hearts := game.SuitHearts
	runPlayTweakCases(t, []playTweakCase{
		{
			name: "risk-smear keeps the promoted ten and gives the jack",
			hand: cards("TS", "JS", "7S", "8C"),
			trick: []game.TrickCard{
				{Card: card("AS"), PlayerSeat: 2},
				{Card: card("8S"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "JS",
		},
		{
			name: "smear a boss-safe card from another suit when void in led and trump",
			hand: cards("AD", "QC", "7C"),
			trick: []game.TrickCard{
				{Card: card("AS"), PlayerSeat: 2},
				{Card: card("8S"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "QC",
		},
		{
			name: "keep points home when the void opponent can still ruff",
			hand: cards("TS", "7S", "8C"),
			trick: []game.TrickCard{
				{Card: card("AS"), PlayerSeat: 2},
				{Card: card("8S"), PlayerSeat: 3},
			},
			callerSeat: 1,
			observes:   []obs{{seat: 1, card: "7C", lead: &spades}},
			wantCard:   "7S",
		},
		{
			name: "smear into a void opponent that cannot ruff",
			hand: cards("KS", "7S", "8C"),
			trick: []game.TrickCard{
				{Card: card("AS"), PlayerSeat: 2},
				{Card: card("8S"), PlayerSeat: 3},
			},
			callerSeat: 1,
			observes: []obs{
				{seat: 1, card: "7C", lead: &spades},
				{seat: 1, card: "8D", lead: &hearts},
			},
			wantCard: "KS",
		},
		{
			name: "keep points home when the partner card is not the boss",
			hand: cards("KS", "7S", "8C"),
			trick: []game.TrickCard{
				{Card: card("TS"), PlayerSeat: 2},
				{Card: card("8S"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "7S",
		},
		{
			name: "forced ruff over the partner plays the cheap trump",
			hand: cards("JH", "9H", "7C"),
			trick: []game.TrickCard{
				{Card: card("AS"), PlayerSeat: 2},
				{Card: card("8S"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "9H",
		},
		{
			name: "safe partner win smears the high boss-safe card",
			hand: cards("TS", "JS", "9S"),
			trick: []game.TrickCard{
				{Card: card("8S"), PlayerSeat: 1},
				{Card: card("AS"), PlayerSeat: 2},
				{Card: card("7S"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "JS",
		},
	})
}

// TestDecide_CheapestSecureWinner checks the bot takes a contested trick with material at stake
// using the cheapest card no remaining opponent could still beat.
func TestDecide_CheapestSecureWinner(t *testing.T) {
	hearts := game.SuitHearts
	runPlayTweakCases(t, []playTweakCase{
		{
			name: "forced overplay over the partner plays the cheapest secure trump",
			hand: cards("KH", "AH", "9H", "JH", "7C"),
			trick: []game.TrickCard{
				{Card: card("QH"), PlayerSeat: 2},
				{Card: card("8H"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "AH",
		},
		{
			name: "forced overplay with no secure winner donates the cheapest",
			hand: cards("QH", "KH"),
			trick: []game.TrickCard{
				{Card: card("8H"), PlayerSeat: 2},
				{Card: card("7H"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "QH",
		},
		{
			name: "low-stake ruff keeps the master nine and cuts with the low trump",
			hand: cards("9H", "8H", "7C"),
			trick: []game.TrickCard{
				{Card: card("KS"), PlayerSeat: 3},
			},
			callerSeat: 1,
			observes:   []obs{{seat: 1, card: "JH"}},
			wantCard:   "8H",
		},
		{
			name: "forced overplay with the remaining opponent void in trump plays the king",
			hand: cards("KH", "AH", "JH", "7C"),
			trick: []game.TrickCard{
				{Card: card("QH"), PlayerSeat: 2},
				{Card: card("8H"), PlayerSeat: 3},
			},
			callerSeat: 1,
			observes:   []obs{{seat: 1, card: "8D", lead: &hearts}},
			wantCard:   "KH",
		},
		{
			name: "low-stake ruff keeps the effective master behind the acted leader",
			hand: cards("9H", "8H", "7C"),
			trick: []game.TrickCard{
				{Card: card("KS"), PlayerSeat: 3},
			},
			callerSeat:     1,
			declaredBySeat: map[int][]string{3: {"JH", "JS", "JD", "JC"}},
			wantCard:       "8H",
		},
		{
			name: "pointless trick is ruffed cheaply, the master is not burnt to secure it",
			hand: cards("JH", "8H", "7C"),
			trick: []game.TrickCard{
				{Card: card("7D"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "8H",
		},
		{
			name: "opponent pinned to the led suit cannot veto the cheap secure ruff",
			hand: cards("9H", "8H", "7C"),
			trick: []game.TrickCard{
				{Card: card("KS"), PlayerSeat: 3},
			},
			callerSeat:     1,
			observes:       []obs{{seat: 1, card: "JH"}},
			declaredBySeat: map[int][]string{1: {"TS", "JS", "QS", "KH"}},
			wantCard:       "8H",
		},
	})
}

// TestDecide_TrumpEconomyTake checks the bot banks the highest safe expendable trump on a take,
// burning a control trump only when the pot is worth it or a known over-ruff forces it.
func TestDecide_TrumpEconomyTake(t *testing.T) {
	spades := game.SuitSpades
	hearts := game.SuitHearts
	runPlayTweakCases(t, []playTweakCase{
		{
			name: "early ruff banks the ace and keeps the master",
			hand: cards("JH", "AH", "7C"),
			trick: []game.TrickCard{
				{Card: card("TS"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "AH",
		},
		{
			name: "fat trick worth the master is secured with the jack",
			hand: cards("JH", "AH", "8H"),
			trick: []game.TrickCard{
				{Card: card("AS"), PlayerSeat: 2},
				{Card: card("TH"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "JH",
		},
		{
			name: "small trick keeps the master and over-ruffs with the ace",
			hand: cards("JH", "AH", "8H"),
			trick: []game.TrickCard{
				{Card: card("8S"), PlayerSeat: 2},
				{Card: card("TH"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "AH",
		},
		{
			name: "known over-ruff on a small trick ruffs cheap and concedes",
			hand: cards("JH", "AH", "8H", "7C"),
			trick: []game.TrickCard{
				{Card: card("KS"), PlayerSeat: 3},
			},
			callerSeat: 1,
			observes:   []obs{{seat: 1, card: "8D", lead: &spades}},
			wantCard:   "8H",
		},

		{
			name: "forced over the partner's trump queen secures with the 9",
			hand: cards("JH", "9H", "TH", "7C", "8S"),
			trick: []game.TrickCard{
				{Card: card("QH"), PlayerSeat: 2},
				{Card: card("7H"), PlayerSeat: 3},
			},
			callerSeat: 0,
			wantCard:   "9H",
		},
		{
			name: "forced over a winning opponent's trump king secures with the 9",
			hand: cards("JH", "9H", "TH", "7C", "8S"),
			trick: []game.TrickCard{
				{Card: card("QH"), PlayerSeat: 2},
				{Card: card("KH"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "9H",
		},
		{
			name: "pointless trump lead keeps the master and donates the queen",
			hand: cards("JH", "QH", "7C", "8S"),
			trick: []game.TrickCard{
				{Card: card("8H"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "QH",
		},
		{
			name: "trump lead with a fat pot spends the master over a cheap trump",
			hand: cards("JH", "QH", "7C", "8S"),
			trick: []game.TrickCard{
				{Card: card("8H"), PlayerSeat: 2},
				{Card: card("AS"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "JH",
		},
		{
			name: "trump lead with the partner behind us still secures",
			hand: cards("JH", "KH", "7C", "8S"),
			trick: []game.TrickCard{
				{Card: card("QH"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "JH",
		},
		{
			name: "trump lead with a non-control secure winner still takes the king",
			hand: cards("KH", "AH", "JH", "7C"),
			trick: []game.TrickCard{
				{Card: card("QH"), PlayerSeat: 2},
				{Card: card("7H"), PlayerSeat: 3},
			},
			callerSeat: 1,
			observes:   []obs{{seat: 1, card: "8D", lead: &hearts}},
			wantCard:   "KH",
		},
		{
			name: "trump lead with no secure winner donates the cheapest",
			hand: cards("KH", "QH", "7C"),
			trick: []game.TrickCard{
				{Card: card("8H"), PlayerSeat: 2},
				{Card: card("7H"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "QH",
		},
	})
}

// TestDecide_PartnerTakesTrickTrumpLed checks the bot ducks a trump lead its partner provably takes,
// banking high points when forced to overtake anyway.
func TestDecide_PartnerTakesTrickTrumpLed(t *testing.T) {
	runPlayTweakCases(t, []playTweakCase{
		{
			name: "partner forced to win the trump lead: dump the ten, keep the nine",
			hand: cards("9H", "TH"),
			trick: []game.TrickCard{
				{Card: card("KH"), PlayerSeat: 3},
			},
			callerSeat:     1,
			declaredBySeat: map[int][]string{2: {"JH", "JS", "JD", "JC"}},
			wantCard:       "TH",
		},
		{
			name: "forced overtake under the partner's guaranteed take banks the ten",
			hand: cards("KH", "TH"),
			trick: []game.TrickCard{
				{Card: card("QH"), PlayerSeat: 3},
			},
			callerSeat:     1,
			declaredBySeat: map[int][]string{2: {"JH", "JS", "JD", "JC"}},
			wantCard:       "TH",
		},
		{
			name: "declared ace that is not threat-proof gives no guaranteed take",
			hand: cards("KD", "7D"),
			trick: []game.TrickCard{
				{Card: card("KH"), PlayerSeat: 3},
			},
			callerSeat:     1,
			declaredBySeat: map[int][]string{2: {"AH", "AS", "AD", "AC"}},
			wantCard:       "7D",
		},
	})
}

// TestDecide_BossPreservingSmear checks the bot smears a boss only when another held card becomes boss after it
// or when the boss cannot win a later trick anyway.
func TestDecide_BossPreservingSmear(t *testing.T) {
	hearts := game.SuitHearts
	diamonds := game.SuitDiamonds
	runPlayTweakCases(t, []playTweakCase{
		{
			name: "smear the ace when the ten protects it",
			hand: cards("AS", "TS", "7H"),
			trick: []game.TrickCard{
				{Card: card("8S"), PlayerSeat: 1},
				{Card: card("KH"), PlayerSeat: 2},
				{Card: card("7D"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "AS",
		},
		{
			name: "all candidates unprotected bosses falls back to the highest points",
			hand: cards("AS", "TC"),
			trick: []game.TrickCard{
				{Card: card("7D"), PlayerSeat: 1},
				{Card: card("KH"), PlayerSeat: 2},
				{Card: card("8D"), PlayerSeat: 3},
			},
			callerSeat: 1,
			observes: []obs{
				{seat: 1, card: "AC"},
				{seat: 1, card: "7C", lead: &hearts},
				{seat: 3, card: "9C", lead: &hearts},
			},
			wantCard: "AS",
		},
		{
			name: "smear keeps the unprotected master trump under the partner jack",
			hand: cards("9H", "QH", "8H"),
			trick: []game.TrickCard{
				{Card: card("KS"), PlayerSeat: 1},
				{Card: card("JH"), PlayerSeat: 2},
				{Card: card("7S"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "QH",
		},
		{
			name: "smear the nine under the partner jack when the ace backs it",
			hand: cards("9H", "AH", "7C"),
			trick: []game.TrickCard{
				{Card: card("KS"), PlayerSeat: 1},
				{Card: card("JH"), PlayerSeat: 2},
				{Card: card("7S"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "9H",
		},
		{
			name: "endgame boss an opponent could ruff is banked not hoarded",
			hand: cards("AS", "8C"),
			trick: []game.TrickCard{
				{Card: card("7D"), PlayerSeat: 1},
				{Card: card("KH"), PlayerSeat: 2},
				{Card: card("8D"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "AS",
		},
		{
			name: "endgame uncuttable boss stays guarded",
			hand: cards("AS", "8C"),
			trick: []game.TrickCard{
				{Card: card("7D"), PlayerSeat: 1},
				{Card: card("KH"), PlayerSeat: 2},
				{Card: card("8D"), PlayerSeat: 3},
			},
			callerSeat: 1,
			observes: []obs{
				{seat: 1, card: "7C", lead: &hearts},
				{seat: 3, card: "9C", lead: &hearts},
			},
			wantCard: "8C",
		},
		{
			name: "dead-suit boss is banked when an opponent ruffs its suit",
			hand: cards("AD", "QC", "7C"),
			trick: []game.TrickCard{
				{Card: card("8S"), PlayerSeat: 1},
				{Card: card("KH"), PlayerSeat: 2},
				{Card: card("7S"), PlayerSeat: 3},
			},
			callerSeat: 1,
			observes: []obs{
				{seat: 1, card: "8H", lead: &diamonds},
				{seat: 3, card: "9H", lead: &diamonds},
			},
			wantCard: "AD",
		},
	})
}

// TestDecide_RetainUncuttableBoss checks the bot treats an uncuttable side boss as a trick-8 winner,
// spending the master trump first and keeping the boss for the last trick.
func TestDecide_RetainUncuttableBoss(t *testing.T) {
	hearts := game.SuitHearts
	oppsVoidTrump := []obs{
		{seat: 1, card: "7C", lead: &hearts},
		{seat: 3, card: "8C", lead: &hearts},
	}
	runPlayTweakCases(t, []playTweakCase{
		{
			name:       "master trump plus uncuttable boss leads the trump first",
			hand:       cards("JH", "AS"),
			callerSeat: 0,
			observes:   oppsVoidTrump,
			wantCard:   "JH",
		},
		{
			name:       "lower trump plus boss cashes the boss now",
			hand:       cards("9H", "AS"),
			callerSeat: 0,
			observes:   oppsVoidTrump,
			wantCard:   "AS",
		},
	})
}

// TestDecide_CashBossPrefersTenWithAceTen checks the bot cashes the Ten before the Ace
// when it holds both of a side suit.
func TestDecide_CashBossPrefersTenWithAceTen(t *testing.T) {
	runPlayTweakCases(t, []playTweakCase{
		{
			name:       "ace and ten of a boss suit leads the ten and keeps the ace",
			hand:       cards("AS", "TS", "7C"),
			callerSeat: 1,
			wantCard:   "TS",
		},
		{
			name:       "ace without the ten cashes the ace",
			hand:       cards("AS", "KS", "7C"),
			callerSeat: 1,
			wantCard:   "AS",
		},
		{
			name:       "king and queen boss cashes the king not the queen",
			hand:       cards("KS", "QS", "7C"),
			callerSeat: 1,
			observes:   []obs{{seat: 1, card: "AS"}, {seat: 1, card: "TS"}},
			wantCard:   "KS",
		},
	})
}

// TestDecide_OnlyTrumpsLeadMasterElseLowest checks that with only trumps in hand
// the bot leads the highest trump when it is the master and the lowest otherwise.
func TestDecide_OnlyTrumpsLeadMasterElseLowest(t *testing.T) {
	runPlayTweakCases(t, []playTweakCase{
		{
			name:       "master in hand leads the highest trump",
			hand:       cards("JH", "9H", "7H"),
			callerSeat: 1,
			wantCard:   "JH",
		},
		{
			name:       "no master leads the lowest trump",
			hand:       cards("9H", "8H", "7H"),
			callerSeat: 1,
			wantCard:   "7H",
		},
	})
}

// TestDecide_NoDrawWhenOpponentsOutOfTrump checks the bot leads a side suit instead of trump
// once both opponents are known void in trump.
func TestDecide_NoDrawWhenOpponentsOutOfTrump(t *testing.T) {
	hearts := game.SuitHearts
	oppsVoidTrump := []obs{
		{seat: 1, card: "7C", lead: &hearts},
		{seat: 3, card: "8C", lead: &hearts},
	}
	runPlayTweakCases(t, []playTweakCase{
		{
			name:       "opponents out of trump cash the side boss not the master",
			hand:       cards("JH", "AS", "KD", "7C"),
			callerSeat: 0,
			observes:   oppsVoidTrump,
			wantCard:   "AS",
		},
		{
			name:       "opponents out of trump and no boss lead safe low not the trump",
			hand:       cards("JH", "KD", "8D", "7C"),
			callerSeat: 0,
			observes:   oppsVoidTrump,
			wantCard:   "7C",
		},
	})
}

// TestDecide_AlwaysLegal pins that every decision is drawn from the legal
// set the engine itself computed.
func TestDecide_AlwaysLegal(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(1)
	gs.ActivePlayerSeat = 0
	gs.CurrentTrick = []game.TrickCard{{Card: card("QD"), PlayerSeat: 3}}
	lead := game.SuitDiamonds
	gs.LeadSuit = &lead

	action := bot.Decide(viewFromState(gs, 0, nil))
	require.Equal(t, game.ActionPlayCard, action.Type)
	require.NotNil(t, action.Card)

	legal := game.LegalCards(gs, 0)
	found := false
	for _, c := range legal {
		if c == *action.Card {
			found = true
		}
	}
	assert.True(t, found, "chosen card %s must be in the engine's legal set", action.Card)
}

func TestMemory_VoidInference(t *testing.T) {
	mem := bot.NewMemory()
	lead := game.SuitSpades

	mem.ObservePlay(2, card("7H"), &lead, nil)
	voids := mem.KnownVoids()
	assert.True(t, voids[2][bot.SuitIndex(game.SuitSpades)])
	assert.False(t, voids[2][bot.SuitIndex(game.SuitHearts)])

	mem.ObservePlay(3, card("9S"), &lead, nil)
	voids = mem.KnownVoids()
	assert.False(t, voids[3][bot.SuitIndex(game.SuitSpades)])

	mem.ObservePlay(0, card("AC"), nil, nil)
	voids = mem.KnownVoids()
	assert.False(t, voids[0][bot.SuitIndex(game.SuitClubs)])

	assert.Len(t, mem.PlayedCards(), 3)
}

func TestMemory_DiscardProvesTrumpVoid(t *testing.T) {
	spades, hearts := game.SuitSpades, game.SuitHearts
	tests := []struct {
		name      string
		lead      game.Suit
		seat      int
		card      string
		wantVoids []game.Suit
	}{
		{"off-suit non-trump discard proves both voids", spades, 1, "9D", []game.Suit{spades, hearts}},
		{"ruff proves only the led suit void", spades, 2, "8H", []game.Suit{spades}},
		{"following suit proves nothing", spades, 3, "8S", nil},
		{"trump led, discard proves void in trump only", hearts, 1, "9D", []game.Suit{hearts}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mem := bot.NewMemory()
			lead := tt.lead
			mem.ObservePlay(tt.seat, card(tt.card), &lead, &hearts)
			voids := mem.KnownVoids()
			for _, s := range game.AllSuits {
				want := slices.Contains(tt.wantVoids, s)
				assert.Equal(t, want, voids[tt.seat][bot.SuitIndex(s)], "suit %s", s)
			}
		})
	}
}

func TestMemory_SyncHandResets(t *testing.T) {
	mem := bot.NewMemory()
	lead := game.SuitSpades
	mem.ObservePlay(2, card("7H"), &lead, nil)
	require.NotEmpty(t, mem.PlayedCards())

	mem.SyncHand(1)
	assert.Len(t, mem.PlayedCards(), 1)

	mem.SyncHand(2)
	assert.Empty(t, mem.PlayedCards())
	assert.False(t, mem.KnownVoids()[2][bot.SuitIndex(game.SuitSpades)])
}

// TestDecide_ReportedTrumpLeadLeakEndToEnd plays the reported trump-lead position through the engine
// and asserts the points outcome, not just the card choice.
func TestDecide_ReportedTrumpLeadLeakEndToEnd(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(1)
	gs.Players[0].Hand = cards("JH", "9H", "TH", "8S")
	gs.Players[1].Hand = cards("AH", "7S", "8D", "9D")
	gs.Players[2].Hand = cards("QH", "AS", "KS", "QS")
	gs.Players[3].Hand = cards("7H", "TS", "7D", "KD")
	gs.ActivePlayerSeat = 2

	play := func(seat int, id string) {
		t.Helper()
		c := card(id)
		next, err := game.ApplyAction(gs, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: seat, Card: &c,
		})
		require.NoError(t, err, "seat %d playing %s must be legal", seat, id)
		gs = next
	}

	play(2, "QH")
	play(3, "7H")

	action := bot.Decide(viewFromState(gs, 0, bot.NewMemory()))
	require.Equal(t, game.ActionPlayCard, action.Type)
	require.NotNil(t, action.Card)
	assert.Equal(t, "9H", action.Card.String(), "bot must secure the trick, not donate the TH")
	play(0, action.Card.String())

	play(1, "AH")

	assert.Equal(t, 1, gs.TricksWon[game.TeamA], "the bot's team must take the trick")
	assert.Equal(t, 0, gs.TricksWon[game.TeamB], "the opponents must not take the trick")
	assert.Equal(t, 28, gs.HandPoints[game.TeamA], "the bot's team must bank the trick points")
	assert.Equal(t, 0, gs.HandPoints[game.TeamB], "the opponents must bank nothing")
}

// TestDecide_TrickSevenRetentionBeatsContesting plays out the last two tricks to show that keeping the master for trick 8
// beats contesting trick 7.
func TestDecide_TrickSevenRetentionBeatsContesting(t *testing.T) {
	build := func() *game.GameState {
		gs := testfixtures.NewGameMidPlay(7)
		gs.Players[0].Hand = cards("JH", "TH")
		gs.Players[1].Hand = cards("9H", "AH")
		gs.Players[2].Hand = cards("QH", "8S")
		gs.Players[3].Hand = cards("7H", "9S")
		gs.ActivePlayerSeat = 2
		gs.HandPoints = [2]int{0, 0}
		gs.TricksWon = [2]int{0, 0}
		return gs
	}
	playOut := func(t *testing.T, trick7Bot, trick8Bot string) [2]int {
		t.Helper()
		gs := build()
		play := func(seat int, id string) {
			t.Helper()
			c := card(id)
			next, err := game.ApplyAction(gs, game.Action{
				Type: game.ActionPlayCard, PlayerSeat: seat, Card: &c,
			})
			require.NoError(t, err, "seat %d playing %s must be legal", seat, id)
			gs = next
		}
		play(2, "QH")
		play(3, "7H")
		play(0, trick7Bot)
		play(1, "AH")
		last := map[int]string{0: trick8Bot, 1: "9H", 2: "8S", 3: "9S"}
		for s, n := gs.ActivePlayerSeat, 0; n < 4; s, n = (s+1)%4, n+1 {
			play(s, last[s])
		}
		return gs.HandPoints
	}

	action := bot.Decide(viewFromState(build(), 0, bot.NewMemory()))
	require.NotNil(t, action.Card)
	assert.Equal(t, "TH", action.Card.String(),
		"at two cards retention must bank the master for the +10 trick")

	retain := playOut(t, "TH", "JH")
	contest := playOut(t, "JH", "TH")

	assert.Equal(t, [2]int{44, 24}, retain, "retaining banks trick 8 and the +10")
	assert.Equal(t, [2]int{34, 34}, contest, "contesting trick 7 gives trick 8 away")
	assert.Greater(t, retain[game.TeamA]-retain[game.TeamB], contest[game.TeamA]-contest[game.TeamB],
		"retention must remain the better line — do not add a contest guard here")
}

// TestObserveDeclarations_OverlapRecordsEachCardOnce checks a card shared by two melds of one seat
// is stored in KnownCards only once.
func TestObserveDeclarations_OverlapRecordsEachCardOnce(t *testing.T) {
	var players [4]game.PlayerState
	players[0].Declarations = []game.Declaration{
		{
			Type:       game.DeclarationSequence,
			Cards:      cards("9S", "TS", "JS", "QS"),
			PlayerSeat: 0,
			Value:      50,
		},
		{
			Type:       game.DeclarationFourOfAKind,
			Cards:      cards("JS", "JH", "JD", "JC"),
			PlayerSeat: 0,
			Value:      200,
		},
	}

	mem := bot.NewMemory()
	mem.ObserveDeclarations(players)
	known := mem.KnownCards()[0]

	ids := make([]string, 0, len(known))
	for _, c := range known {
		ids = append(ids, c.String())
	}
	assert.ElementsMatch(t, []string{"9S", "TS", "JS", "QS", "JH", "JD", "JC"}, ids,
		"the shared JS must be recorded exactly once")
}

func TestMemory_LocatedCardsJoinKnownCardsButNotDeclaredCards(t *testing.T) {
	mem := bot.NewMemory()
	mem.ObserveLocatedCard(2, card("AS"))
	mem.ObserveLocatedCard(2, card("AS"))
	assert.Equal(t, cards("AS"), mem.KnownCards()[2])
	assert.Empty(t, mem.DeclaredCards()[2])
	mem.SyncHand(2)
	assert.Empty(t, mem.KnownCards()[2])
}

func TestDecide_PartnersLocatedAceMakesTheKingABoss(t *testing.T) {
	runPlayTweakCases(t, []playTweakCase{
		{
			name:       "king leads as a boss once the ten is gone and the partner is known to hold the ace",
			hand:       cards("KS", "7C", "8D"),
			callerSeat: 1,
			observes:   []obs{{seat: 1, card: "TS"}},
			located:    map[int][]string{2: {"AS"}},
			wantCard:   "KS",
		},
		{
			name:       "king stays a false boss while the ace is unseen",
			hand:       cards("KS", "7C", "8D"),
			callerSeat: 1,
			observes:   []obs{{seat: 1, card: "TS"}},
			wantCard:   "7C",
		},
	})
}

func TestCardsProvablyAbsent(t *testing.T) {
	tests := []struct {
		name     string
		declared []string
		want     []string
	}{
		{"tierce in the middle excludes both neighbours", []string{"9C", "TC", "JC"}, []string{"8C", "QC"}},
		{"run at the top excludes only below", []string{"QS", "KS", "AS"}, []string{"JS"}},
		{"run at the bottom excludes only above", []string{"7H", "8H", "9H"}, []string{"TH"}},
		{"two suits, two runs", []string{"7H", "8H", "9H", "QS", "KS", "AS"}, []string{"TH", "JS"}},
		{"a pair excludes nothing", []string{"KD", "AD"}, nil},
		{"four of a kind excludes nothing", []string{"JS", "JH", "JD", "JC"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := bot.View{}
			v.DeclaredCards[1] = cards(tt.declared...)

			got := bot.CardsProvablyAbsentForTest(v, 1)

			if len(tt.want) == 0 {
				assert.Empty(t, got)
				return
			}
			keys := make([]game.Card, 0, len(got))
			for c := range got {
				keys = append(keys, c)
			}
			assert.ElementsMatch(t, cards(tt.want...), keys)
		})
	}
}
