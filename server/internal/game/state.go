package game

import (
	"math/rand/v2"
	"slices"
	"time"
)

// PlayerState represents the state of a single player in the game.
type PlayerState struct {
	Hand         []Card        `json:"hand"`
	Seat         int           `json:"seat"`
	UserID       uint          `json:"userId"`
	Username     string        `json:"username"`
	Team         string        `json:"team"`
	Declarations []Declaration `json:"declarations"`
	Connected    bool          `json:"connected"`
	// IsBot marks a server-driven seat (Story 10.3). Bots carry UserID 0 and
	// an empty Username; they stay Connected forever so no disconnect UI ever
	// shows for them. A field, not a rule — the engine treats bot seats
	// exactly like human seats.
	IsBot bool `json:"isBot"`
	// Level is the player's server-authoritative lifetime level, derived from
	// total_xp via the pure XP curve (user.LevelForXP) and captured ONCE at
	// match start. It is static for the duration of a match (XP is only awarded
	// at match end), so the match engine never recomputes it. Bot seats are 0.
	Level int `json:"level"`
	// FaceDownCards holds the two cards dealt face-down to this seat under
	// DealShapeAllBeforeBidding. They are part of the seat's real holding but
	// live OUTSIDE Hand until bidding resolves, at which point mergeFaceDownCards
	// folds them in and every seat holds eight.
	//
	// Server-only (json:"-") and deliberately so: match_state is serialized once
	// and the identical bytes go to all four seats, so a card that must be
	// visible to exactly one player cannot ride it. Keeping these out of Hand
	// means they are never serialized into ANYONE's snapshot, and their owner
	// learns them through a per-seat WS event instead.
	//
	// Scope of that claim, precisely: these two cards per seat are the ONE
	// exception in an otherwise fully open payload. match_state still ships all
	// four players' Hands and the undealt Deck to every seat — a pre-existing
	// leak that is deliberately out of scope here and recorded in the deferred
	// work log. Nothing about this field makes match_state per-seat safe.
	FaceDownCards []Card `json:"-"`
}

// TrickCard represents a single card played in a trick, with the player who played it.
type TrickCard struct {
	Card       Card `json:"card"`
	PlayerSeat int  `json:"playerSeat"`
}

// HandScore captures the scoring breakdown for a completed hand.
// Populated by scoreHand() before startNewHand() or PhaseMatchEnd,
// so the match manager can broadcast the full breakdown to clients.
type HandScore struct {
	TeamACardPoints int `json:"teamACardPoints"` // Trick-taking card points (Team A) before bonus
	TeamBCardPoints int `json:"teamBCardPoints"` // Trick-taking card points (Team B) before bonus
	TeamADeclPoints int `json:"teamADeclPoints"` // Declaration points (Team A)
	TeamBDeclPoints int `json:"teamBDeclPoints"` // Declaration points (Team B)
	LastTrickTeam   int `json:"lastTrickTeam"`   // Team that won last trick (0=Team A, 1=Team B)
	LastTrickBonus  int `json:"lastTrickBonus"`  // 10 (normal) or 0 (capot replaces it)
	// LastTrickSeat is the seat (0-3) that won the final trick. Server-only
	// (json:"-"): the client validates match_state's lastHandResult with a
	// strict schema and only needs the team. The match manager reads this for
	// the event:trick_resolved winnerSeat on a hand's last trick, because by
	// broadcast time scoreHand/startNewHand have cleared state.TrickWinnerSeat.
	LastTrickSeat   int  `json:"-"`
	Capot           bool `json:"capot"`           // One team took all 8 tricks
	CapotTeam       *int `json:"capotTeam"`       // Team with capot (nil if no capot)
	CapotBonus      int  `json:"capotBonus"`      // 100 or 0
	FailedContract  bool `json:"failedContract"`  // Contracting team lost the hand
	ContractingTeam int  `json:"contractingTeam"` // Team that called trump (0=Team A, 1=Team B)
	TeamAHandTotal  int  `json:"teamAHandTotal"`  // Points actually awarded to Team A this hand
	TeamBHandTotal  int  `json:"teamBHandTotal"`  // Points actually awarded to Team B this hand
}

// GameState is the complete, serializable game state.
// Fields are ordered per Architecture spec:
// 1. Match metadata
// 2. Current hand state
// 3. Current trick state
// 4. Player states
// 5. Scoring
// 6. Timer state
type GameState struct {
	// Match metadata
	ID        uint    `json:"id"`
	RoomID    uint    `json:"roomId"`
	Variant   Variant `json:"variant"`
	MatchMode string  `json:"matchMode"`
	Phase     Phase   `json:"phase"`
	OwnerSeat int     `json:"ownerSeat"` // Seat index of the room owner (for pause override)
	// Rules is the resolved per-rule configuration for Variant, set once by
	// NewGame via RulesFor and never mutated afterwards. Every variant
	// divergence in this package reads a field here rather than comparing
	// Variant (D-VAR-1).
	//
	// Server-only (json:"-"): the client validates match_state with a strict
	// schema and derives nothing from the config — the server is authoritative
	// on every rule it selects. All-value-typed, so cloneGameState's shallow
	// struct copy carries it correctly.
	Rules VariantRules `json:"-"`

	// Current hand state
	HandNumber       int   `json:"handNumber"`
	DealerSeat       int   `json:"dealerSeat"`
	TrumpSuit        *Suit `json:"trumpSuit"`
	TrumpCallerSeat  *int  `json:"trumpCallerSeat"`
	TrumpCandidate   *Card `json:"trumpCandidate"`
	BiddingRound     int   `json:"biddingRound"`
	BiddingPassCount int   `json:"biddingPassCount"`
	// Deck is the undealt remainder held back for post-pick distribution. Under
	// DealShapeCandidate it holds 11 cards through bidding and is emptied when
	// bidding resolves; under DealShapeAllBeforeBidding every card is dealt up
	// front so it is ALWAYS empty — handlePickTrump's deal-shape guard depends on
	// exactly that.
	Deck []Card `json:"deck"`
	// FaceDownRevealed records that round 1 was passed out under
	// VariantRules.RevealFaceDownOnRound2, so every seat's two face-down cards
	// are now known to their owner. Server-only (json:"-") — the cards
	// themselves never ride match_state, so neither does this flag; the match
	// layer reads it to deliver (and, on reconnect, re-deliver) each seat's own
	// two cards through a per-seat event.
	FaceDownRevealed bool `json:"-"`

	// Current trick state
	TrickNumber          int         `json:"trickNumber"`
	CurrentTrick         []TrickCard `json:"currentTrick"`
	LeadSuit             *Suit       `json:"leadSuit"`
	TrickWinnerSeat      *int        `json:"trickWinnerSeat"`
	AwaitingDeclaration  bool        `json:"awaitingDeclaration"`
	DeclarationsResolved bool        `json:"declarationsResolved"`
	// DeclarationSeatsAnswered counts how many seats the PhaseDeclaring cursor
	// has already visited, walking counter-clockwise from (DealerSeat+1)%4. A
	// seat counts as visited once it declares, skips, or is stepped past for
	// holding no meld — so the phase ends exactly when this reaches 4, and a
	// skip is never confused with "not asked yet" (stored Declarations cannot
	// tell those two apart). Always 0 outside the phase.
	//
	// Server-only (json:"-"): the client drives the prompt entirely from phase +
	// awaitingDeclaration + activePlayerSeat, so this must not widen
	// match_state. Value-typed, so cloneGameState's shallow struct copy carries
	// it with no clone line of its own.
	DeclarationSeatsAnswered int `json:"-"`

	// Player states
	Players [4]PlayerState `json:"players"`

	// Scoring (index 0=Team A, 1=Team B)
	TeamScores        [2]int `json:"teamScores"`
	HandPoints        [2]int `json:"handPoints"` // Trick-taking card points only — belote is tracked separately in BelotPoints.
	DeclarationPoints [2]int `json:"declarationPoints"`
	// BelotPoints holds the 20-pt belote/rebelote bonus (K+Q of trump), kept
	// apart from HandPoints so it is classified as a declaration — not card
	// points — everywhere it surfaces. It is awarded to whoever announces it
	// (independent of the declaration contest) but, like all hand points, still
	// transfers to the opponents on a failed contract or capot.
	BelotPoints      [2]int     `json:"belotPoints"`
	TricksWon        [2]int     `json:"tricksWon"`
	PendingBelotSeat *int       `json:"pendingBelotSeat"`
	BelotAnnounced   bool       `json:"belotAnnounced"`
	WinnerTeam       *int       `json:"winnerTeam"`
	LastHandResult   *HandScore `json:"lastHandResult"`
	// HandCompleteReady tracks which seats have acknowledged the PhaseHandComplete
	// pause (action:continue). The next hand deals once every connected seat is
	// ready (or the session manager's auto-continue timeout fires). Server-only
	// (json:"-"): the client validates match_state with a strict schema and only
	// needs the phase; it shows a local "waiting" state after the player continues.
	HandCompleteReady [4]bool `json:"-"`

	// Timer state
	ActivePlayerSeat int        `json:"activePlayerSeat"`
	TurnExpiresAt    *time.Time `json:"turnExpiresAt"`
	TimerDurationSec int        `json:"timerDurationSec"`

	// Pause state
	PreviousPhase     Phase   `json:"previousPhase"`     // Phase before pause/disconnect (for resume)
	PausedPlayers     [4]bool `json:"pausedPlayers"`     // Which seats have active pauses
	PauseUsed         [4]bool `json:"pauseUsed"`         // Which seats have used their one-time pause
	TurnTimeRemaining int64   `json:"turnTimeRemaining"` // Milliseconds remaining on turn timer when paused/disconnected

	// Surrender state (Story 8.2)
	SurrenderProposerSeat *int    `json:"surrenderProposerSeat"` // nil when no proposal pending; seat of the proposer otherwise
	SurrenderUsed         [4]bool `json:"surrenderUsed"`         // each seat may initiate a surrender at most once per match

	// Disconnect state
	//
	// `PlayerReconnectExpiresAt` holds a per-seat window so concurrent
	// disconnects don't share a single clock — each player gets their own
	// `reconnectWindowSec` from their own drop. `DisconnectedSeat` and
	// `ReconnectExpiresAt` are derived views: the seat whose window closes
	// soonest (drives the abandon timer + the dialog's center countdown) and
	// that seat's expiry, respectively. They stay populated for backwards
	// compat with clients that pre-date the per-seat array.
	DisconnectedSeat         int           `json:"disconnectedSeat"`         // -1 when nobody is disconnected, otherwise seat with the earliest expiry
	ReconnectExpiresAt       *time.Time    `json:"reconnectExpiresAt"`       // earliest of PlayerReconnectExpiresAt — when match abandons if no one returns
	PlayerReconnectExpiresAt [4]*time.Time `json:"playerReconnectExpiresAt"` // per-seat reconnect window expiry; nil when seat is online
}

// TeamA is the index for Team A (seats 0, 2) in score arrays.
const TeamA = 0

// TeamB is the index for Team B (seats 1, 3) in score arrays.
const TeamB = 1

// TeamForSeat returns the team index (0=Team A, 1=Team B) for a given seat number.
func TeamForSeat(seat int) int {
	return seat % 2
}

// TeamStringForIndex returns "teamA" for 0, "teamB" for 1. Returns empty string for values outside {0, 1}.
func TeamStringForIndex(i int) string {
	switch i {
	case TeamA:
		return "teamA"
	case TeamB:
		return "teamB"
	}
	return ""
}

// TeamIndexForString returns 0 for "teamA", 1 for "teamB". Returns -1 for unknown.
func TeamIndexForString(s string) int {
	switch s {
	case "teamA":
		return TeamA
	case "teamB":
		return TeamB
	}
	return -1
}

// ShuffleDeck randomly shuffles a deck of cards in-place.
// Uses math/rand/v2 which is automatically seeded in Go 1.22+.
func ShuffleDeck(deck []Card) {
	rand.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})
}

// NewGame creates a new game state with 4 players, resolves the variant's rule
// config ONCE (the only place a config is resolved), then shuffles and deals
// per that config's deal shape. bots marks the server-driven seats (UserID 0,
// empty username) — see PlayerState.IsBot.
func NewGame(playerIDs [4]uint, usernames [4]string, bots [4]bool, variant Variant, matchMode string, roomID uint) *GameState {
	// The first-hand dealer is drawn uniformly at random, the way a real
	// table cuts for the deal. Hardcoding seat 0 handed the same seat the
	// deal (and the seat after it the opening bid) in every single match;
	// per-hand rotation only ever moved that fixed starting point. Uses the
	// same auto-seeded math/rand/v2 global as ShuffleDeck above.
	dealerSeat := rand.IntN(4)

	gs := &GameState{
		RoomID:           roomID,
		Variant:          variant,
		Rules:            RulesFor(variant),
		MatchMode:        matchMode,
		Phase:            PhaseDealing,
		HandNumber:       1,
		DealerSeat:       dealerSeat,
		ActivePlayerSeat: (dealerSeat + 1) % 4, // player after dealer (counter-clockwise)
		BiddingRound:     1,
		BiddingPassCount: 0,
		TrickNumber:      0,
		CurrentTrick:     []TrickCard{},
		DisconnectedSeat: -1,
	}

	// Assign players to seats and teams
	for i, userID := range playerIDs {
		team := "teamA"
		if i%2 == 1 {
			team = "teamB"
		}
		gs.Players[i] = PlayerState{
			Hand:         []Card{},
			Seat:         i,
			UserID:       userID,
			Username:     usernames[i],
			Team:         team,
			Declarations: []Declaration{},
			Connected:    true,
			IsBot:        bots[i],
		}
	}

	// Generate, shuffle, and deal per gs.Rules.DealShape. Instant-win can't be
	// determined here — final hands aren't known until the picker is decided.
	deck := NewDeck()
	ShuffleDeck(deck)
	dealCards(gs, deck)

	return gs
}

// dealCards deals a full 32-card deck according to gs.Rules.DealShape. It is
// the single dealing entry point — NewGame, startNewHand, and
// reshuffleAndRedeal all route through it, so a variant's deal shape holds for
// every hand of the match, not just the first.
//
// DealShapeAllBeforeBidding (see dealAllBeforeBidding) deals everything up
// front. DealShapeCandidate performs the two-stage deal below:
//
// Round 1: 3 cards to each player counter-clockwise from dealer (12 cards)
// Round 2: 2 cards to each player (8 cards, 20 total)
// Then the next card (deck[20]) is lifted onto the table as TrumpCandidate.
// The remaining 11 cards (deck[21:32]) are stored in gs.Deck for stage-2
// distribution after a player picks trump.
//
// Each player holds 5 cards after stage-1; the candidate is public and held
// aside, not in any hand. handlePickTrump completes the deal once bidding
// resolves.
func dealCards(gs *GameState, deck []Card) {
	if gs.Rules.DealShape == DealShapeAllBeforeBidding {
		dealAllBeforeBidding(gs, deck)
		return
	}

	cardIdx := 0
	dealer := gs.DealerSeat

	// Round 1: 3 cards to each player
	for i := 0; i < 4; i++ {
		seat := (dealer + 1 + i) % 4 // start from player after dealer
		gs.Players[seat].Hand = append(gs.Players[seat].Hand, slices.Clone(deck[cardIdx:cardIdx+3])...)
		cardIdx += 3
	}

	// Round 2: 2 cards to each player
	for i := 0; i < 4; i++ {
		seat := (dealer + 1 + i) % 4
		gs.Players[seat].Hand = append(gs.Players[seat].Hand, slices.Clone(deck[cardIdx:cardIdx+2])...)
		cardIdx += 2
	}

	// Trump candidate: flipped face-up on the table — public to all players,
	// not in any hand yet. Round 1 bidding offers this exact card; round 2
	// allows free-suit choice. Picker takes it as their 8th card during stage-2.
	candidate := deck[cardIdx]
	gs.TrumpCandidate = &candidate
	cardIdx++

	// Remaining 11 cards held in the deck for stage-2 distribution.
	gs.Deck = slices.Clone(deck[cardIdx:])
}

// dealAllBeforeBidding deals every card before bidding opens (the
// DealShapeAllBeforeBidding sequence):
//
// Round 1: 3 cards to each player counter-clockwise from dealer (12 cards)
// Round 2: 3 more cards to each player (12 cards, 24 total)
// Round 3: the last 2 cards per player, placed FACE-DOWN (8 cards, 32 total)
//
// Each player therefore physically holds 8 cards, but only 6 sit in Hand — the
// other 2 live in PlayerState.FaceDownCards, unknown even to their owner until
// round 1 is passed out (see VariantRules.RevealFaceDownOnRound2) or bidding
// resolves. There is no trump candidate and nothing is held back, so Deck is
// empty and bidding is a bare named suit in both rounds.
func dealAllBeforeBidding(gs *GameState, deck []Card) {
	cardIdx := 0
	dealer := gs.DealerSeat

	// Two open batches of 3.
	for batch := 0; batch < 2; batch++ {
		for i := 0; i < 4; i++ {
			seat := (dealer + 1 + i) % 4 // start from player after dealer
			gs.Players[seat].Hand = append(gs.Players[seat].Hand, slices.Clone(deck[cardIdx:cardIdx+3])...)
			cardIdx += 3
		}
	}

	// Final batch of 2 per seat, face-down. Assigned (not appended) so a
	// re-deal cannot accumulate a previous hand's hidden cards.
	for i := 0; i < 4; i++ {
		seat := (dealer + 1 + i) % 4
		gs.Players[seat].FaceDownCards = slices.Clone(deck[cardIdx : cardIdx+2])
		cardIdx += 2
	}

	// No candidate, no stage-2 reserve, and nothing revealed yet.
	gs.TrumpCandidate = nil
	gs.Deck = nil
	gs.FaceDownRevealed = false
}
