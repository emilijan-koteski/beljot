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

	// Current hand state
	HandNumber       int    `json:"handNumber"`
	DealerSeat       int    `json:"dealerSeat"`
	TrumpSuit        *Suit  `json:"trumpSuit"`
	TrumpCallerSeat  *int   `json:"trumpCallerSeat"`
	TrumpCandidate   *Card  `json:"trumpCandidate"`
	BiddingRound     int    `json:"biddingRound"`
	BiddingPassCount int    `json:"biddingPassCount"`
	Deck             []Card `json:"deck"` // Undealt remainder during bidding (11 cards); empty once bidding resolves.

	// Current trick state
	TrickNumber          int         `json:"trickNumber"`
	CurrentTrick         []TrickCard `json:"currentTrick"`
	LeadSuit             *Suit       `json:"leadSuit"`
	TrickWinnerSeat      *int        `json:"trickWinnerSeat"`
	AwaitingDeclaration  bool        `json:"awaitingDeclaration"`
	DeclarationsResolved bool        `json:"declarationsResolved"`

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

// NewGame creates a new game state with 4 players, shuffles and deals cards
// using the Bitola 3+2 dealing sequence.
func NewGame(playerIDs [4]uint, usernames [4]string, variant Variant, matchMode string, roomID uint) *GameState {
	gs := &GameState{
		RoomID:           roomID,
		Variant:          variant,
		MatchMode:        matchMode,
		Phase:            PhaseDealing,
		HandNumber:       1,
		DealerSeat:       0,
		ActivePlayerSeat: 1, // player after dealer (counter-clockwise)
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
		}
	}

	// Generate, shuffle, and deal stage-1 (5 cards per seat + visible candidate + 11-card Deck).
	// Instant-win can't be determined here — final hands aren't known until the picker is decided.
	deck := NewDeck()
	ShuffleDeck(deck)
	dealCards(gs, deck)

	return gs
}

// dealCards performs the stage-1 Bitola deal:
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
