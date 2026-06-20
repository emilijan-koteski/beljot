package bot_test

import (
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
		DealerSeat:          gs.DealerSeat,
		CurrentTrick:        gs.CurrentTrick,
		LeadSuit:            gs.LeadSuit,
		ActivePlayerSeat:    gs.ActivePlayerSeat,
		AwaitingDeclaration: gs.AwaitingDeclaration && gs.ActivePlayerSeat == seat,
		PendingBelot:        gs.PendingBelotSeat != nil && *gs.PendingBelotSeat == seat,
		TeamScores:          gs.TeamScores,
		HandPoints:          gs.HandPoints,
		TricksWon:           gs.TricksWon,
		PlayedCards:         mem.PlayedCards(),
		KnownVoids:          mem.KnownVoids(),
		KnownCards:          mem.KnownCards(),
	}
	if gs.SurrenderProposerSeat != nil && (*gs.SurrenderProposerSeat+2)%4 == seat {
		v.PartnerProposedSurrender = true
	}
	if gs.Phase == game.PhasePlaying {
		v.LegalCards = game.LegalCards(gs, seat)
	}
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

// --- Bidding evaluator ---

func TestDecide_Bidding(t *testing.T) {
	tests := []struct {
		name      string
		seat      int
		hand      []game.Card
		round     int
		candidate string // card id; "" keeps the fixture default (AH)
		wantType  string
		wantSuit  *game.Suit
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
			name:     "round 1 three with jack but only 7+8 alongside passes",
			seat:     1,
			hand:     cards("JH", "8H", "7H", "7S", "8S"),
			round:    1,
			wantType: game.ActionPassTrump,
		},
		{
			name:     "round 1 three with jack and a nine picks",
			seat:     1,
			hand:     cards("JH", "9H", "7H", "7S", "8S"),
			round:    1,
			wantType: game.ActionPickTrump,
		},
		{
			name:      "round 1 nine and ace without a side ace passes",
			seat:      1,
			hand:      cards("9H", "AH", "7H", "7S", "8S"),
			round:     1,
			candidate: "QH",
			wantType:  game.ActionPassTrump,
		},
		{
			name:      "round 1 nine and ace with a side ace picks",
			seat:      1,
			hand:      cards("9H", "AH", "7H", "AS", "8S"),
			round:     1,
			candidate: "QH",
			wantType:  game.ActionPickTrump,
		},
		{
			name:     "round 1 three without jack or nine+ace passes",
			seat:     1,
			hand:     cards("KH", "QH", "TH", "7S", "8S"),
			round:    1,
			wantType: game.ActionPassTrump,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := testfixtures.NewGameJustDealt()
			gs.BiddingRound = tt.round
			gs.ActivePlayerSeat = tt.seat
			gs.Players[tt.seat].Hand = tt.hand
			if tt.candidate != "" {
				c := card(tt.candidate)
				gs.TrumpCandidate = &c
			}

			action := bot.Decide(viewFromState(gs, tt.seat, nil))

			assert.Equal(t, tt.wantType, action.Type)
			assert.Equal(t, tt.seat, action.PlayerSeat)
			if tt.round == 1 && tt.wantType == game.ActionPickTrump {
				assert.Nil(t, action.Suit, "round 1 pick carries no suit")
			}
			if tt.wantSuit != nil {
				require.NotNil(t, action.Suit)
				assert.Equal(t, *tt.wantSuit, *action.Suit)
			}
		})
	}
}

func suitPtr(s game.Suit) *game.Suit { return &s }

// --- Prompt decisions ---

func TestDecide_AlwaysDeclares(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(1)
	gs.AwaitingDeclaration = true
	gs.ActivePlayerSeat = 0

	action := bot.Decide(viewFromState(gs, 0, nil))
	assert.Equal(t, game.ActionDeclare, action.Type)
	assert.Equal(t, 0, action.PlayerSeat)
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
	proposer := 2 // partner of seat 0
	gs.SurrenderProposerSeat = &proposer
	gs.ActivePlayerSeat = 0

	action := bot.Decide(viewFromState(gs, 0, nil))
	assert.Equal(t, game.ActionSurrenderAccept, action.Type,
		"surrender accept takes priority over card play")
}

// --- Card play (trump is Hearts in NewGameMidPlay; bot at seat 0, team A) ---

func TestDecide_CardPlay(t *testing.T) {
	tests := []struct {
		name       string
		hand       []game.Card // bot seat 0
		trick      []game.TrickCard
		callerSeat int // fixture default 1 (opponents); 0 = bot's team
		played     []string
		// declaredBySeat maps a seat to the card ids it revealed via
		// declarations (known holdings). The declaration type/value are
		// immaterial to the bot, so the test wraps them in one Declaration.
		declaredBySeat map[int][]string
		wantCard       string
	}{
		{
			name: "smear high points when partner trumped and bot closes the trick",
			hand: cards("AS", "QS", "7H"),
			trick: []game.TrickCard{
				{Card: card("8S"), PlayerSeat: 1},
				{Card: card("KH"), PlayerSeat: 2},
				{Card: card("7D"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "AS",
		},
		{
			name: "smear when partner ace is boss",
			hand: cards("TS", "8S", "7H"),
			trick: []game.TrickCard{
				{Card: card("QS"), PlayerSeat: 1},
				{Card: card("AS"), PlayerSeat: 2},
				{Card: card("7D"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "TS",
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
			// Partner declared the low trumps and opponents are void of trump,
			// so there is nothing to draw — cash the side Ace, never lead the
			// master JH at the partner. (Without declaration memory the bot
			// would lead JH to "draw".)
			name:           "declared: opponents void of trump, cash side ace not the master trump",
			hand:           cards("JH", "AS", "7C"),
			trick:          nil,
			callerSeat:     0,
			played:         []string{"QH", "KH", "AH"},
			declaredBySeat: map[int][]string{2: {"7H", "8H", "9H", "TH"}},
			wantCard:       "AS",
		},
		{
			// Partner declared the master JH (and the rest of the top trumps);
			// opponents still hold lower trumps. The bot now KNOWS the team
			// controls the top, so it draws with its own best trump (9H).
			// (Without declaration memory JH/QH/KH/AH look unseen and the bot
			// would not draw.)
			name:           "declared: partner holds the master so the bot draws with its best trump",
			hand:           cards("9H", "7C"),
			trick:          nil,
			callerSeat:     0,
			declaredBySeat: map[int][]string{2: {"JH", "QH", "KH", "AH"}},
			wantCard:       "9H",
		},
		{
			// Opponents won the contest and declared all four aces, so AS is a
			// KNOWN opponent threat — the bot must not treat its KS as a boss.
			// Guards against dropping known-opponent cards from the threat set.
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
				mem.ObservePlay(1, card(id), nil)
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

// TestDecide_PartnerTakesTrick covers the follow-play "don't overtake a trick
// the partner is guaranteed to take" branch. The bot (seat 0) plays second to
// an AD lead from seat 3; its partner (seat 2, last to play) declared the top
// trumps (9H–QH). The duck is sound ONLY when the partner is provably void in
// the led suit, so it must ruff and — being last — wins. A partner that might
// still have to follow diamonds could be forced under by the opponent between
// them, so the bot must not duck on a mere declared beater.
func TestDecide_PartnerTakesTrick(t *testing.T) {
	tests := []struct {
		name            string
		partnerVoidDiam bool
		playedHearts    []string // declared trumps the partner has already played
		wantCard        string
	}{
		{
			name:            "partner void in led with a threat-proof trump: smear the high card",
			partnerVoidDiam: true,
			wantCard:        "KD",
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
			gs := testfixtures.NewGameMidPlay(1) // trump = Hearts
			gs.Players[0].Hand = cards("KD", "7D")
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
				mem.ObservePlay(2, card("8C"), &lead) // discarded a club on a diamond lead
			}
			for _, id := range tt.playedHearts {
				mem.ObservePlay(2, card(id), nil)
			}
			mem.ObserveDeclarations(gs.Players)

			action := bot.Decide(viewFromState(gs, 0, mem))

			require.Equal(t, game.ActionPlayCard, action.Type)
			require.NotNil(t, action.Card)
			assert.Equal(t, tt.wantCard, action.Card.String())
		})
	}
}

// TestDecide_LastTrickRetention covers endgame retention ("dix de der", +10):
// at the second-to-last trick (two cards in hand) the bot keeps the master
// trump back for the forced trick 8 instead of squandering it now. Bot is
// seat 0 (team A); trump is Hearts in NewGameMidPlay.
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
			// Without retention the bot would draw the JH on trick 7; instead it
			// leads the junk and banks the JH for the forced trick 8.
			name:       "lead retains the master trump and spends the junk",
			hand:       cards("JH", "7C"),
			trick:      nil,
			callerSeat: 0,
			wantCard:   "7C",
		},
		{
			// 9H is the master once the only higher trump (JH) is gone — retain it.
			name:       "non-absolute master is retained once higher trumps are played",
			hand:       cards("9H", "7C"),
			trick:      nil,
			callerSeat: 0,
			played:     []string{"JH"},
			wantCard:   "7C",
		},
		{
			// Void in the led suit forces a cut: the engine's legal set is {JH}
			// only, so the master is played and retention cannot override it.
			name:       "forced cut plays the master — retention never overrides a forced play",
			hand:       cards("JH", "7C"),
			trick:      []game.TrickCard{{Card: card("AD"), PlayerSeat: 3}},
			callerSeat: 0,
			wantCard:   "JH",
		},
		{
			// Two trumps, forced to cut: spend the lower trump (it wins now) and
			// keep the master JH for the last trick.
			name:       "cut with the lower trump and keep the master",
			hand:       cards("JH", "9H"),
			trick:      []game.TrickCard{{Card: card("7D"), PlayerSeat: 3}},
			callerSeat: 0,
			wantCard:   "9H",
		},
		{
			// Partner declared a trump (JH) above the bot's master (9H): the team
			// already takes trick 8, so retention defers and the bot draws as
			// before instead of fighting the partner.
			name:           "partner controls the last trick so retention defers and the bot draws",
			hand:           cards("9H", "7C"),
			trick:          nil,
			callerSeat:     0,
			declaredBySeat: map[int][]string{2: {"JH", "QH", "KH", "AH"}},
			wantCard:       "9H",
		},
		{
			// Following trick 7 while the partner currently wins it (with 7H):
			// the engine's over-trump rule forces the bot to beat its own
			// partner, so it spends the lower trump (9H) — which wins now — and
			// keeps the master JH for the forced trick 8.
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
			// Three cards — not the endgame decision point — so retention is
			// gated out and the bot draws the master as usual.
			name:       "not the endgame leaves draw behavior unchanged",
			hand:       cards("JH", "AS", "7C"),
			trick:      nil,
			callerSeat: 0,
			wantCard:   "JH",
		},
		{
			// No trump held: side bosses are excluded from retention, so the bot
			// cashes the Ace now exactly as before.
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
				mem.ObservePlay(1, card(id), nil)
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

// --- Memory ---

func TestMemory_VoidInference(t *testing.T) {
	mem := bot.NewMemory()
	lead := game.SuitSpades

	// Seat 2 fails to follow spades — void in spades.
	mem.ObservePlay(2, card("7H"), &lead)
	voids := mem.KnownVoids()
	assert.True(t, voids[2][bot.SuitIndex(game.SuitSpades)])
	assert.False(t, voids[2][bot.SuitIndex(game.SuitHearts)])

	// Following the led suit reveals nothing.
	mem.ObservePlay(3, card("9S"), &lead)
	voids = mem.KnownVoids()
	assert.False(t, voids[3][bot.SuitIndex(game.SuitSpades)])

	// A lead (no led suit yet) reveals nothing.
	mem.ObservePlay(0, card("AC"), nil)
	voids = mem.KnownVoids()
	assert.False(t, voids[0][bot.SuitIndex(game.SuitClubs)])

	assert.Len(t, mem.PlayedCards(), 3)
}

func TestMemory_SyncHandResets(t *testing.T) {
	mem := bot.NewMemory()
	lead := game.SuitSpades
	mem.ObservePlay(2, card("7H"), &lead)
	require.NotEmpty(t, mem.PlayedCards())

	// Same hand → no reset.
	mem.SyncHand(1)
	assert.Len(t, mem.PlayedCards(), 1)

	// New hand → full reset.
	mem.SyncHand(2)
	assert.Empty(t, mem.PlayedCards())
	assert.False(t, mem.KnownVoids()[2][bot.SuitIndex(game.SuitSpades)])
}
