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
			// Candidate inclusion (rule 1a): only 2 hearts in hand, but the
			// face-up AH the picker would receive completes J+9+A — 3 trumps incl.
			// the Jack with no 7/8 — so it is a take.
			name:     "round 1 candidate completes jack+nine to a qualifying three picks",
			seat:     1,
			hand:     cards("JH", "9H", "7S", "8S", "9S"),
			round:    1,
			wantType: game.ActionPickTrump,
		},
		{
			// Same two trumps, but the candidate is the 7H, so the completed three
			// is J+9+7 — a 7 is present, so rule 1b makes it too weak to call.
			name:      "round 1 candidate completes to three with a seven passes",
			seat:      1,
			hand:      cards("JH", "9H", "7S", "8S", "9S"),
			round:     1,
			candidate: "7H",
			wantType:  game.ActionPassTrump,
		},
		{
			// Rule 1b: J+7+8 (and any 7-or-8) is too weak. Candidate 8H completes
			// J+7 in hand to the bare J+7+8 three.
			name:      "round 1 three with jack and a seven and eight passes",
			seat:      1,
			hand:      cards("JH", "7H", "7S", "8S", "9S"),
			round:     1,
			candidate: "8H",
			wantType:  game.ActionPassTrump,
		},
		{
			// Rule 1b exception: the weak J+8+7 three still calls when the hand
			// holds a side Ace (AS) as an outside winner.
			name:      "round 1 three with jack and a seven and eight but a side ace picks",
			seat:      1,
			hand:      cards("JH", "8H", "AS", "7C", "8C"),
			round:     1,
			candidate: "7H",
			wantType:  game.ActionPickTrump,
		},
		{
			// Same weak three but no side Ace anywhere — still a pass.
			name:      "round 1 three with jack and a seven and eight and no side ace passes",
			seat:      1,
			hand:      cards("JH", "8H", "KS", "7C", "8C"),
			round:     1,
			candidate: "7H",
			wantType:  game.ActionPassTrump,
		},
		{
			// Rule 1c: 9+Ace pair with a side Ace (spades AS) — take.
			name:      "round 1 nine ace pair with a side ace picks",
			seat:      1,
			hand:      cards("9H", "AH", "AS", "8S", "KD"),
			round:     1,
			candidate: "QH",
			wantType:  game.ActionPickTrump,
		},
		{
			// Point 1: a lone side Ace is now enough — the Ace no longer needs a
			// second card of its suit to back it, so a singleton AS still calls.
			name:      "round 1 nine ace pair with a lone side ace picks",
			seat:      1,
			hand:      cards("9H", "AH", "AS", "8D", "KC"),
			round:     1,
			candidate: "QH",
			wantType:  game.ActionPickTrump,
		},
		{
			// Rule 1c still needs a side Ace: 9+Ace pair with no outside Ace passes.
			name:      "round 1 nine ace pair with no side ace passes",
			seat:      1,
			hand:      cards("9H", "AH", "KS", "8D", "KC"),
			round:     1,
			candidate: "QH",
			wantType:  game.ActionPassTrump,
		},
		{
			// Point 2: exactly two trumps that are the Jack and the 9, plus a side
			// Ace — a take. Candidate 9H completes JH-in-hand to the {J,9} pair.
			name:      "round 1 two trumps jack and nine with a side ace picks",
			seat:      1,
			hand:      cards("JH", "AS", "7C", "8C", "7D"),
			round:     1,
			candidate: "9H",
			wantType:  game.ActionPickTrump,
		},
		{
			// Point 2: {J,9} but no side Ace anywhere — pass.
			name:      "round 1 two trumps jack and nine without a side ace passes",
			seat:      1,
			hand:      cards("JH", "7C", "8C", "7D", "8D"),
			round:     1,
			candidate: "9H",
			wantType:  game.ActionPassTrump,
		},
		{
			// Point 2 is {J,9}-only: two trumps that are the Jack and Ace (not the
			// 9) do not qualify, even with a side Ace.
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
			// Round-2 candidate awareness: diamonds is a 9+Ace pick and the club Ace
			// supplies the side Ace. The face-up candidate TC still lands in the
			// picker's hand as a side card, but a lone side Ace already suffices.
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
			// Partner trumped and the trick is closed, but the AS is an
			// unprotected boss (no TS behind it): throwing it surrenders control
			// of spades, so the bot smears the QS and keeps the Ace as the master.
			// A duck would have dumped the 7S, so the smear stays observable.
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
			// Partner's Ace wins outright, and with it gone the bot's TS is the
			// promoted boss of spades — unprotected (the KS still out kills the
			// JS as a backup), so the Ten stays home and the JS carries the
			// points. A duck would have dumped the 8S.
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
		hand            []game.Card // bot seat 0; nil = the default KD,7D
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
			// Boss-guard on this smear site too: with the AD falling in this very
			// trick the bot's TD is the promoted diamond boss with no backup (the
			// KD and QD still out kill the JD as one) — kept home. The JD carries
			// the points instead, where a duck would have dumped the 7D.
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
			gs := testfixtures.NewGameMidPlay(1) // trump = Hearts
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

// --- Card-play tweaks (Rules 4, 5.1.1, 6, 7) ---

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
	declaredBySeat map[int][]string
	wantCard       string
}

func runPlayTweakCases(t *testing.T, tests []playTweakCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := testfixtures.NewGameMidPlay(1) // trump = Hearts
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
				mem.ObservePlay(o.seat, card(o.card), o.lead)
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

// TestDecide_DrawTrumpsForPartner covers Rule 4: when the PARTNER (seat 2)
// called trump and the bot leads without the master, it leads its highest trump
// to strip opponents (assuming the partner holds the top trumps), unless the
// partner is void in trump or a known opponent holding outranks the bot's best.
func TestDecide_DrawTrumpsForPartner(t *testing.T) {
	hearts := game.SuitHearts
	runPlayTweakCases(t, []playTweakCase{
		{
			// 9 + 8 trumps, no honor: sacrifice the low 8 and KEEP the 9 — never
			// lead the 9 (a near-master) into the partner's Jack.
			name:       "partner called with nine and junk: lead the low trump and keep the nine",
			hand:       cards("9H", "8H", "AS", "7C"),
			callerSeat: 2,
			wantCard:   "8H",
		},
		{
			// Honor order: Queen is the weakest honor, so it leads first.
			name:       "partner called with honors: sacrifice the queen first",
			hand:       cards("QH", "KH", "AS", "7C"),
			callerSeat: 2,
			wantCard:   "QH",
		},
		{
			// Ten precedes the Ace in the honor order, so it leads before the ace.
			name:       "partner called with ten and ace: lead the ten before the ace",
			hand:       cards("TH", "AH", "8S", "7C"),
			callerSeat: 2,
			wantCard:   "TH",
		},
		{
			// 9 + Ace: the Ace is an honor and leads; the 9 is kept back.
			name:       "partner called with nine and ace: lead the ace and keep the nine",
			hand:       cards("9H", "AH", "8S", "7C"),
			callerSeat: 2,
			wantCard:   "AH",
		},
		{
			// The lone trump is the 9 — never lead it; fall through to the side boss.
			name:       "partner called with a lone nine: do not draw, cash the side boss",
			hand:       cards("9H", "AS", "8D", "7C"),
			callerSeat: 2,
			wantCard:   "AS",
		},
		{
			name:       "partner void in trump: do not draw, cash the side boss",
			hand:       cards("9H", "8H", "AS", "7C"),
			callerSeat: 2,
			observes:   []obs{{seat: 2, card: "8D", lead: &hearts}}, // void hearts
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
			// No honor and no boss: lead the lowest non-9 trump (the 7).
			name:       "partner called with low trumps and no boss: lead the lowest non-nine trump",
			hand:       cards("8H", "7H", "KD", "7C"),
			callerSeat: 2,
			wantCard:   "7H",
		},
		{
			// Partner (seat 2) called and declared the 8-9-10 trump tierce — a
			// MAXIMAL run, so it provably lacks the JH (it would have shown the
			// quarte J-10-9-8). The bot lacks it too, so an opponent holds the
			// master: drawing a trump only donates the trick to it. Skip the draw
			// and cash the side Ace instead.
			name:           "partner's declared tierce proves the opponents hold the master: do not draw",
			hand:           cards("QH", "KH", "AS", "7C"),
			callerSeat:     2,
			declaredBySeat: map[int][]string{2: {"8H", "9H", "TH"}},
			wantCard:       "AS",
		},
		{
			// Control: the partner's declared run INCLUDES the Jack (J-10-9), so the
			// team controls the master — the bot still draws, sacrificing its
			// weakest honor (the Queen).
			name:           "partner's declared run includes the jack: still draw for the partner",
			hand:           cards("QH", "KH", "AS", "7C"),
			callerSeat:     2,
			declaredBySeat: map[int][]string{2: {"JH", "TH", "9H"}},
			wantCard:       "QH",
		},
	})
}

// TestDecide_PreserveBossOnForcedOvertake covers Rule 5.1.1: when the partner
// has safely won the trick but every legal card would overtake (e.g. the
// overplay rule forces the bot higher), the bot captures with its strongest
// card unless that card is the boss/master — then it drops to the second
// strongest and keeps the boss for later.
func TestDecide_PreserveBossOnForcedOvertake(t *testing.T) {
	runPlayTweakCases(t, []playTweakCase{
		{
			// Point 7: partner (seat 2) wins with QS, bot closes the trick holding
			// AS+TS, both forced over QS. Holding BOTH the Ace and Ten, the bot
			// smears the AS — the TS becomes the suit's new boss, so no control is
			// lost — banking the higher points now.
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
			// The Ace+Ten exception is exactly that pair: with the Ace boss but no
			// Ten held, preserve the Ace and drop to the King.
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
			// With AS and TS already gone, KS is now the boss → drop to QS.
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
			// AS still unseen, so KS is not the boss → play the strongest KS.
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
			// Void in led: bot must cut over the partner's winning AD; both trumps
			// overtake. JH is the trump master → drop to 9H.
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

// TestDecide_LastToPlayBanksHighest covers Rule 6: as the last player winning
// over an opponent by following a non-trump led suit, the bot banks the
// highest-point led-suit winner (safe now) instead of the cheapest (which it
// would otherwise have to lead next trick into a possible ruff). Ruff wins and
// non-last positions are unaffected.
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
			// Leader is seat 3, so seat 0 plays SECOND (not last) — Rule 6 is gated
			// out and the bot wins cheaply with the ten, keeping the ace.
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

// TestDecide_LeadIntoPartnerVoid covers Rule 7: with no boss to cash, the bot
// leads the lowest card of a side suit the partner is known void in (and the
// partner can still ruff, i.e. not void in trump) so the partner trumps and
// wins. Opponents called trump in these cases, so the partner trump-draw is off.
func TestDecide_LeadIntoPartnerVoid(t *testing.T) {
	diamonds := game.SuitDiamonds
	spades := game.SuitSpades
	hearts := game.SuitHearts
	runPlayTweakCases(t, []playTweakCase{
		{
			name:       "partner void in diamonds with trump: lead the lowest diamond",
			hand:       cards("7D", "KD", "9C"),
			callerSeat: 1,
			observes:   []obs{{seat: 2, card: "7C", lead: &diamonds}}, // partner void diamonds
			wantCard:   "7D",
		},
		{
			name:       "partner also void in trump: cannot ruff, lead safe low",
			hand:       cards("8C", "KD", "9S"),
			callerSeat: 1,
			observes: []obs{
				{seat: 2, card: "7C", lead: &diamonds}, // void diamonds
				{seat: 2, card: "8D", lead: &hearts},   // void trump
			},
			wantCard: "8C",
		},
		{
			name:       "bot void in the partner's void suit: cannot feed, lead safe low",
			hand:       cards("KD", "9C", "7H"),
			callerSeat: 1,
			observes:   []obs{{seat: 2, card: "7C", lead: &spades}}, // partner void spades
			wantCard:   "9C",
		},
		{
			name:       "own boss outranks feeding the ruff: cash the ace first",
			hand:       cards("AS", "7D", "9C"),
			callerSeat: 1,
			observes:   []obs{{seat: 2, card: "7C", lead: &diamonds}}, // partner void diamonds
			wantCard:   "AS",
		},
	})
}

// TestDecide_SmearOntoPartnerBoss covers Rule 8: when the partner LEADS the
// boss of a non-trump suit (e.g. the Ace) and the bot follows 3rd with one
// opponent still to play, the bot smears high points onto the trick instead of
// dumping a 7/8 — accepting the unknown ruff risk. It keeps points home only
// when the last opponent is KNOWN void in the led suit (near-certain ruff).
// Seating: partner (seat 2) leads, opp (seat 3) follows, bot (seat 0) is 3rd,
// opp (seat 1) is the 4th still to play. Trump = Hearts.
func TestDecide_SmearOntoPartnerBoss(t *testing.T) {
	spades := game.SuitSpades
	hearts := game.SuitHearts
	runPlayTweakCases(t, []playTweakCase{
		{
			// Partner's Ace is the boss; the last opponent's void is unknown. Once
			// the Ace cashes, the bot's TS is the promoted boss of spades with no
			// backup (the unseen KS kills the JS as one) — so the risk-smear gives
			// the JS and keeps the Ten. A duck would have dumped the 7S.
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
			// Void in led and in trump: the AD is an unprotected boss (no TD
			// behind it), so the bot keeps it and smears the best boss-safe card
			// of another suit — the QC. A duck would have dumped the 7C.
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
			// The last opponent (seat 1) is KNOWN void in spades and trumps still
			// remain it could hold — near-certain ruff, so keep points home.
			name: "keep points home when the void opponent can still ruff",
			hand: cards("TS", "7S", "8C"),
			trick: []game.TrickCard{
				{Card: card("AS"), PlayerSeat: 2},
				{Card: card("8S"), PlayerSeat: 3},
			},
			callerSeat: 1,
			observes:   []obs{{seat: 1, card: "7C", lead: &spades}}, // seat 1 void spades
			wantCard:   "7S",
		},
		{
			// Last opponent (seat 1) is void in spades BUT also known void in trump,
			// so it cannot ruff — smear anyway even into the led-suit void. The KS
			// is boss-safe (the TS is still out), so the smear stays observable:
			// kept points home, this would have been the 7S.
			name: "smear into a void opponent that cannot ruff",
			hand: cards("KS", "7S", "8C"),
			trick: []game.TrickCard{
				{Card: card("AS"), PlayerSeat: 2},
				{Card: card("8S"), PlayerSeat: 3},
			},
			callerSeat: 1,
			observes: []obs{
				{seat: 1, card: "7C", lead: &spades}, // seat 1 void spades
				{seat: 1, card: "8D", lead: &hearts}, // seat 1 void trump -> cannot ruff
			},
			wantCard: "KS",
		},
		{
			// Partner is winning with the Ten but the Ace is still unseen — not the
			// boss, so don't smear; keep points home. (Bot's K+7 are both below the
			// Ten, so no overplay is forced and the choice is free.)
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
			// Void in led but HOLDING trump: forced to ruff the partner (Bitola has
			// no exemption). Nothing to smear, so the cheap ruff is played.
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
			// Regression: when the partner's win is already SAFE (bot closes the
			// trick), the safe-smear still fires — but the TS, promoted to boss by
			// the partner's Ace, is unprotected and stays home. The JS carries the
			// points, where a duck would have dumped the 9S.
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

// TestDecide_CheapestSecureWinner covers the secure-take rule: when the bot
// takes a contested trick with an opponent still to play — forced over a
// winning partner, or overtaking an opponent — and there is material at stake
// (points in the trick or on the cheapest take), it prefers the cheapest card
// no yet-to-play opponent could still beat, gambling on the cheapest beatable
// winner only when nothing is provably secure. The security check is
// seat-aware: cards declared by an opponent who already acted this trick,
// unseen suits a remaining opponent is provably void in, and cards a seat
// pinned to the led suit could never legally play are not threats. On a
// pointless trick no secure winner is ever spent. (The guaranteed-partner-
// take duck lives in TestDecide_PartnerTakesTrickTrumpLed.) Trump = Hearts.
func TestDecide_CheapestSecureWinner(t *testing.T) {
	hearts := game.SuitHearts
	runPlayTweakCases(t, []playTweakCase{
		{
			// Forced overplay over the partner's QH with the TH still unseen: the
			// cheap KH would donate the trick (and its points) to the Ten's
			// holder. Holding the 9H and JH itself, the bot's AH is the cheapest
			// trump no opponent card can beat — it secures the take.
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
			// Forced overplay with nothing secure: AH/TH/9H/JH are all unseen, so
			// every overplay the bot owns can be overtaken. Donate minimally —
			// the cheapest overtaker — exactly as before.
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
			// Opponent leads the KS (4 pts at stake) and the bot must ruff. The
			// 8H is the cheapest winner but any unseen higher trump overtakes it;
			// with the JH already played the bot's 9H is the master trump — the
			// cheapest card that provably takes the trick.
			name: "ruff over an opponent prefers the secure trump over the beatable cheap one",
			hand: cards("9H", "8H", "7C"),
			trick: []game.TrickCard{
				{Card: card("KS"), PlayerSeat: 3},
			},
			callerSeat: 1,
			observes:   []obs{{seat: 1, card: "JH"}},
			wantCard:   "9H",
		},
		{
			// Seat-aware security: the TH and 9H are still unseen, but the only
			// player left behind the bot (seat 1) is known void in trump — nobody
			// who still acts can beat ANY of the bot's trumps, so the cheap KH is
			// already guaranteed. Burning the JH here would waste the master on a
			// trick the King takes for free.
			name: "forced overplay with the remaining opponent void in trump plays the king",
			hand: cards("KH", "AH", "JH", "7C"),
			trick: []game.TrickCard{
				{Card: card("QH"), PlayerSeat: 2},
				{Card: card("8H"), PlayerSeat: 3},
			},
			callerSeat: 1,
			observes:   []obs{{seat: 1, card: "8D", lead: &hearts}}, // seat 1 void trump
			wantCard:   "KH",
		},
		{
			// Seat-aware security: seat 3 declared the JH but has ALREADY acted
			// this trick (it led the KS), so the Jack can no longer fall on it —
			// the bot's 9H is effectively the master and ruffs securely. A raw
			// threat scan would let the dead Jack veto the 9H and dump the
			// beatable 8H instead.
			name: "declared jack behind the leader does not veto the master ruff",
			hand: cards("9H", "8H", "7C"),
			trick: []game.TrickCard{
				{Card: card("KS"), PlayerSeat: 3},
			},
			callerSeat:     1,
			declaredBySeat: map[int][]string{3: {"JH", "JS", "JD", "JC"}},
			wantCard:       "9H",
		},
		{
			// Material-stake gate: a pointless 7D lead is ruffed with the 8H even
			// though every unseen higher trump beats it — zero points sit in the
			// trick and the 8H adds none, so there is nothing worth securing and
			// the master JH is not burnt to win nothing. (A side 7C keeps the
			// hand off the two-card retention path, which would bank the JH for
			// its own reasons.)
			name: "pointless trick is ruffed cheaply, the master is not burnt to secure it",
			hand: cards("JH", "8H", "7C"),
			trick: []game.TrickCard{
				{Card: card("7D"), PlayerSeat: 3},
			},
			callerSeat: 1,
			wantCard:   "8H",
		},
		{
			// Follow-suit security: the yet-to-play opponent (seat 1) revealed a
			// spade tierce plus the KH — holding the led suit, it is FORCED to
			// follow spades, so its KH can never land on this trick. With the JH
			// already played, the cheap 8H ruff is provably secure; without the
			// follow-suit filter the revealed KH would veto it and the bot would
			// burn the 14-pt 9H instead.
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

// TestDecide_PartnerTakesTrickTrumpLed covers the trump-led extension of the
// partnerTakesTrick duck: on a trump lead the partner needs no void proof —
// it must follow trump, and the overplay rule forces its known threat-proof
// beater out, so the trick is the team's. The bot then never spends a secure
// winner of its own; forced to overtake anyway, it banks high points via
// strongestPreservingBoss. A declared beater that is NOT threat-proof gives
// no guarantee, and the normal take/duck path runs. Trump = Hearts.
func TestDecide_PartnerTakesTrickTrumpLed(t *testing.T) {
	runPlayTweakCases(t, []playTweakCase{
		{
			// Seat 3 leads the KH while the partner's declared JH is still behind
			// it — the overplay rule forces the Jack (threat-proof) out, so the
			// trick is already the team's. The bot, squeezed over the King
			// itself, donates the TH and never spends the 14-pt 9H on a take the
			// partner has locked up.
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
			// Forced overtake under the guaranteed take: both the KH and TH must
			// overplay the led QH, and the partner's declared JH still wins the
			// trick — whatever the bot plays lands in the team's pile, so it
			// banks the 10-pt Ten (strongestPreservingBoss; the TH is no master
			// while the AH and 9H are out) instead of wasting the slot on the
			// 4-pt King.
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
			// Negative gate: the partner declared the AH, but the JH and 9H are
			// still unseen — the Ace is NOT threat-proof, so the trump lead gives
			// no guarantee and the bot must not smear. It falls through to the
			// normal path and discards low (a wrong duck would smear the KD).
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

// TestDecide_BossPreservingSmear covers the boss-guard on smears: a boss is
// smeared only when another held card of its suit is also a boss once it
// leaves (the Ace backed by the Ten, generalized — including the trump
// master), or when it cannot convert to a future trick anyway (endgame with a
// possible ruff looming, or a suit an opponent already ruffs). When no
// candidate is boss-safe at all, the plain highest-point smear stands.
// Trump = Hearts.
func TestDecide_BossPreservingSmear(t *testing.T) {
	hearts := game.SuitHearts
	diamonds := game.SuitDiamonds
	runPlayTweakCases(t, []playTweakCase{
		{
			// Partner trumped and the bot closes the trick holding BOTH the AS and
			// TS: the Ace is a protected boss — the Ten stays boss once the Ace
			// leaves — so the highest points are still smeared.
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
			// Every smear candidate is an unprotected boss worth guarding: the
			// lone AS, and the TC promoted once the AC was played (both opponents
			// are out of trump, so the endgame exception stays quiet). Nothing is
			// boss-safe, so the fallback smears the highest points — the old
			// behavior, preserved.
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
				{seat: 1, card: "7C", lead: &hearts}, // seat 1 void trump
				{seat: 3, card: "9C", lead: &hearts}, // seat 3 void trump
			},
			wantCard: "AS",
		},
		{
			// The boss-guard covers the trump master too: partner ruffed with the
			// JH and the bot closes, forced to under-ruff. With the Jack gone the
			// bot's 9H is the master trump and has no backup — keep it (a trump
			// master can never be ruffed away) and smear the QH.
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
			// Same under-ruff but holding 9H AND AH: once the Jack falls they are
			// BOTH masters, each protecting the other, so the bot banks the higher
			// points — the canonical 9 smeared under the partner's Jack.
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
			// Endgame exception: two cards left and an opponent may still hold a
			// trump. The lone AS wins trick 8 only if nobody ruffs — hoarding it
			// just donates its 11 points to that ruff, so it is banked NOW onto
			// the partner's safe trick.
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
			// Same endgame, but both opponents are provably out of trump: the AS
			// is an uncuttable trick-8 winner, so the guard holds — smear the 8C
			// and keep the Ace for the last trick.
			name: "endgame uncuttable boss stays guarded",
			hand: cards("AS", "8C"),
			trick: []game.TrickCard{
				{Card: card("7D"), PlayerSeat: 1},
				{Card: card("KH"), PlayerSeat: 2},
				{Card: card("8D"), PlayerSeat: 3},
			},
			callerSeat: 1,
			observes: []obs{
				{seat: 1, card: "7C", lead: &hearts}, // seat 1 void trump
				{seat: 3, card: "9C", lead: &hearts}, // seat 3 void trump
			},
			wantCard: "8C",
		},
		{
			// Dead-suit exception: both opponents already showed out of diamonds
			// and can still hold trumps, so the AD gets ruffed the moment its
			// suit is led — it can never convert. Bank its 11 points now instead
			// of hoarding a dead master.
			name: "dead-suit boss is banked when an opponent ruffs its suit",
			hand: cards("AD", "QC", "7C"),
			trick: []game.TrickCard{
				{Card: card("8S"), PlayerSeat: 1},
				{Card: card("KH"), PlayerSeat: 2},
				{Card: card("7S"), PlayerSeat: 3},
			},
			callerSeat: 1,
			observes: []obs{
				{seat: 1, card: "8H", lead: &diamonds}, // seat 1 void diamonds
				{seat: 3, card: "9H", lead: &diamonds}, // seat 3 void diamonds
			},
			wantCard: "AD",
		},
	})
}

// TestDecide_RetainUncuttableBoss covers the endgame-retention generalization
// (Point 3): an uncuttable non-trump boss is a guaranteed trick-8 winner too.
// Holding the master trump PLUS such a boss, the bot spends the master first
// (leads the trump) and keeps the boss for the last trick. Holding only a LOWER
// trump plus the boss, it defers — cashing the boss now is already best, since
// the partner keeps trick-8 control. Trump = Hearts; both opponents (seats 1, 3)
// are marked void in trump so the side boss cannot be ruffed.
func TestDecide_RetainUncuttableBoss(t *testing.T) {
	hearts := game.SuitHearts
	oppsVoidTrump := []obs{
		{seat: 1, card: "7C", lead: &hearts},
		{seat: 3, card: "8C", lead: &hearts},
	}
	runPlayTweakCases(t, []playTweakCase{
		{
			// Master JH + uncuttable boss AS: lead the master first, keep the AS.
			// (The old rule spent the AS first and banked the JH.)
			name:       "master trump plus uncuttable boss leads the trump first",
			hand:       cards("JH", "AS"),
			callerSeat: 0,
			observes:   oppsVoidTrump,
			wantCard:   "JH",
		},
		{
			// Lower trump 9H + boss AS: JH is still out, so 9H is not the master —
			// defer and cash the boss now (the partner secures trick 8).
			name:       "lower trump plus boss cashes the boss now",
			hand:       cards("9H", "AS"),
			callerSeat: 0,
			observes:   oppsVoidTrump,
			wantCard:   "AS",
		},
	})
}

// TestDecide_CashBossPrefersTenWithAceTen covers Point 4: when cashing a side
// boss and holding BOTH the Ace and Ten of that suit, the bot leads the Ten
// (also a boss, since it holds the Ace) and keeps the Ace as the guaranteed
// master. The exception is the Ace+Ten pair only. Opponents (seat 1) called
// trump, so the trump-draw leads are off.
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
			// King+Queen is not the Ace+Ten pair — cash the higher (promoted) boss.
			name:       "king and queen boss cashes the king not the queen",
			hand:       cards("KS", "QS", "7C"),
			callerSeat: 1,
			observes:   []obs{{seat: 1, card: "AS"}, {seat: 1, card: "TS"}},
			wantCard:   "KS",
		},
	})
}

// TestDecide_OnlyTrumpsLeadMasterElseLowest covers Point 5: with only trumps in
// hand the bot leads the highest trump ONLY when it is the master; otherwise it
// leads the lowest trump, keeping its stronger trumps back. Opponents (seat 1)
// called trump so the trump-draw leads are off and the "only trumps left" branch
// is reached; three trumps keep it out of the two-card retention path.
func TestDecide_OnlyTrumpsLeadMasterElseLowest(t *testing.T) {
	runPlayTweakCases(t, []playTweakCase{
		{
			// JH is the absolute master → lead it.
			name:       "master in hand leads the highest trump",
			hand:       cards("JH", "9H", "7H"),
			callerSeat: 1,
			wantCard:   "JH",
		},
		{
			// JH still out, so 9H is not the master → lead the lowest trump (7H).
			name:       "no master leads the lowest trump",
			hand:       cards("9H", "8H", "7H"),
			callerSeat: 1,
			wantCard:   "7H",
		},
	})
}

// TestDecide_NoDrawWhenOpponentsOutOfTrump covers Point 6: once BOTH opponents
// are known void in trump, the remaining trumps are split between the bot and
// its partner, so leading trump to "draw" only strips the partner's control. The
// bot leads a side suit instead (cashing a boss / leading safe) and lets the
// partner keep the trump lead. Without this the partner's UNKNOWN trumps look
// unseen and the bot would lead its master trump. Our team called trump.
func TestDecide_NoDrawWhenOpponentsOutOfTrump(t *testing.T) {
	hearts := game.SuitHearts
	oppsVoidTrump := []obs{
		{seat: 1, card: "7C", lead: &hearts},
		{seat: 3, card: "8C", lead: &hearts},
	}
	runPlayTweakCases(t, []playTweakCase{
		{
			// Holds the master JH, but opponents are out of trump — cash the side
			// boss AS rather than lead the master into the partner.
			name:       "opponents out of trump cash the side boss not the master",
			hand:       cards("JH", "AS", "KD", "7C"),
			callerSeat: 0,
			observes:   oppsVoidTrump,
			wantCard:   "AS",
		},
		{
			// No side boss either — lead a safe low side card, still never the trump.
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
