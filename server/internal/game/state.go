package game

import (
	"math/rand/v2"
	"slices"
	"time"
)

// PlayerState represents the state of a single player in the game.
// FaceDownCards is server-only because those cards stay hidden from everyone, owner included, until bidding resolves.
type PlayerState struct {
	Hand                []Card        `json:"hand"`
	Seat                int           `json:"seat"`
	UserID              uint          `json:"userId"`
	Username            string        `json:"username"`
	Team                string        `json:"team"`
	Declarations        []Declaration `json:"declarations"`
	Connected           bool          `json:"connected"`
	IsBot               bool          `json:"isBot"`
	Level               int           `json:"level"`
	AvatarURL           *string       `json:"avatarUrl"`
	FaceDownCards       []Card        `json:"-"`
	FaceDownCount       int           `json:"faceDownCount"`
	HandCount           int           `json:"handCount"`
	DeclarationAnswered bool          `json:"declarationAnswered"`
}

// syncFaceDownCounts refreshes every seat's public FaceDownCount from its server-only FaceDownCards.
// Only dealCards calls it, so any path that writes FaceDownCards must fall through to dealCards.
func syncFaceDownCounts(state *GameState) {
	for i := range state.Players {
		state.Players[i].FaceDownCount = len(state.Players[i].FaceDownCards)
	}
}

// TrickCard represents a single card played in a trick, with the player who played it.
type TrickCard struct {
	Card       Card `json:"card"`
	PlayerSeat int  `json:"playerSeat"`
}

// HandScore captures the scoring breakdown for a completed hand.
// HandNumber says which hand it describes, because LastHandResult survives into later hands.
type HandScore struct {
	HandNumber      int  `json:"-"`
	TeamACardPoints int  `json:"teamACardPoints"`
	TeamBCardPoints int  `json:"teamBCardPoints"`
	TeamADeclPoints int  `json:"teamADeclPoints"`
	TeamBDeclPoints int  `json:"teamBDeclPoints"`
	LastTrickTeam   int  `json:"lastTrickTeam"`
	LastTrickBonus  int  `json:"lastTrickBonus"`
	LastTrickSeat   int  `json:"-"`
	Capot           bool `json:"capot"`
	CapotTeam       *int `json:"capotTeam"`
	CapotBonus      int  `json:"capotBonus"`
	FailedContract  bool `json:"failedContract"`
	ContractingTeam int  `json:"contractingTeam"`
	TeamAHandTotal  int  `json:"teamAHandTotal"`
	TeamBHandTotal  int  `json:"teamBHandTotal"`
}

// GameState is the complete, serializable game state. ProjectForSeat masks by enumeration, so every new
// field must be triaged as public, server-only (json:"-"), or masked per seat in ProjectForSeat.
type GameState struct {
	ID                   uint         `json:"id"`
	RoomID               uint         `json:"roomId"`
	Variant              Variant      `json:"variant"`
	MatchMode            string       `json:"matchMode"`
	Phase                Phase        `json:"phase"`
	OwnerSeat            int          `json:"ownerSeat"`
	Rules                VariantRules `json:"-"`
	DeclarationsEnabled  bool         `json:"declarationsEnabled"`
	StopAtTarget         bool         `json:"stopAtTarget"`
	DeclarationsTransfer bool         `json:"declarationsTransfer"`
	Clockwise            bool         `json:"clockwise"`
	StoppedAtTarget      bool         `json:"-"`
	WonByInstantWin      bool         `json:"-"`

	HandNumber       int    `json:"handNumber"`
	DealerSeat       int    `json:"dealerSeat"`
	TrumpSuit        *Suit  `json:"trumpSuit"`
	TrumpCallerSeat  *int   `json:"trumpCallerSeat"`
	TrumpCandidate   *Card  `json:"trumpCandidate"`
	BiddingRound     int    `json:"biddingRound"`
	BiddingPassCount int    `json:"biddingPassCount"`
	MustPickTrump    bool   `json:"mustPickTrump"`
	Deck             []Card `json:"-"`

	TrickNumber           int         `json:"trickNumber"`
	CurrentTrick          []TrickCard `json:"currentTrick"`
	LeadSuit              *Suit       `json:"leadSuit"`
	TrickWinnerSeat       *int        `json:"trickWinnerSeat"`
	AwaitingDeclaration   bool        `json:"awaitingDeclaration"`
	DeclarationsResolved  bool        `json:"declarationsResolved"`
	DeclarationsContested bool        `json:"-"`

	Players [4]PlayerState `json:"players"`

	TeamScores        [2]int     `json:"teamScores"`
	HandPoints        [2]int     `json:"handPoints"`
	DeclarationPoints [2]int     `json:"declarationPoints"`
	BelotPoints       [2]int     `json:"belotPoints"`
	TricksWon         [2]int     `json:"tricksWon"`
	PendingBelotSeat  *int       `json:"pendingBelotSeat"`
	BelotAnnounced    bool       `json:"belotAnnounced"`
	WinnerTeam        *int       `json:"winnerTeam"`
	LastHandResult    *HandScore `json:"lastHandResult"`
	HandCompleteReady [4]bool    `json:"-"`

	ActivePlayerSeat int        `json:"activePlayerSeat"`
	TurnExpiresAt    *time.Time `json:"turnExpiresAt"`
	TimerDurationSec int        `json:"timerDurationSec"`

	PreviousPhase     Phase   `json:"previousPhase"`
	PausedPlayers     [4]bool `json:"pausedPlayers"`
	PauseUsed         [4]bool `json:"pauseUsed"`
	TurnTimeRemaining int64   `json:"turnTimeRemaining"`

	SurrenderProposerSeat *int    `json:"surrenderProposerSeat"`
	SurrenderUsed         [4]bool `json:"surrenderUsed"`

	DisconnectedSeat         int           `json:"disconnectedSeat"`
	ReconnectExpiresAt       *time.Time    `json:"reconnectExpiresAt"`
	PlayerReconnectExpiresAt [4]*time.Time `json:"playerReconnectExpiresAt"`
}

const TeamA = 0

const TeamB = 1

// TeamForSeat returns the team index (0=Team A, 1=Team B) for a given seat number.
func TeamForSeat(seat int) int {
	return seat % 2
}

// ShuffleDeck randomly shuffles a deck of cards in-place.
func ShuffleDeck(deck []Card) {
	rand.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})
}

// NewGame creates a 4-player game, resolves the variant rules once with the room's declarationsEnabled,
// stopAtTarget, declarationsTransfer and clockwise overrides, picks a random dealer, and deals. The room flags
// are positional so a forgotten argument fails to compile instead of silently starting a match on the wrong rules.
func NewGame(playerIDs [4]uint, usernames [4]string, bots [4]bool, variant Variant, matchMode string, roomID uint, declarationsEnabled bool, stopAtTarget bool, declarationsTransfer bool, clockwise bool) *GameState {
	dealerSeat := rand.IntN(4)

	rules := RulesFor(variant)
	rules.DeclarationsEnabled = declarationsEnabled
	rules.StopAtTarget = stopAtTarget
	rules.DeclarationsTransfer = declarationsTransfer
	rules.Clockwise = clockwise

	gs := &GameState{
		RoomID:               roomID,
		Variant:              variant,
		Rules:                rules,
		DeclarationsEnabled:  rules.DeclarationsEnabled,
		StopAtTarget:         rules.StopAtTarget,
		DeclarationsTransfer: rules.DeclarationsTransfer,
		Clockwise:            rules.Clockwise,
		MatchMode:            matchMode,
		Phase:                PhaseDealing,
		HandNumber:           1,
		DealerSeat:           dealerSeat,
		ActivePlayerSeat:     rules.NextSeat(dealerSeat),
		BiddingRound:         1,
		BiddingPassCount:     0,
		TrickNumber:          0,
		CurrentTrick:         []TrickCard{},
		DisconnectedSeat:     -1,
		DeclarationsResolved: !rules.DeclarationsEnabled,
	}

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

	deck := NewDeck()
	ShuffleDeck(deck)
	dealCards(gs, deck)

	return gs
}

// dealCards deals a full 32-card deck according to gs.Rules.DealShape and is the single dealing entry point.
// The candidate shape deals 3+2 cards per seat, flips the next card as TrumpCandidate, and keeps the other 11 in gs.Deck.
func dealCards(gs *GameState, deck []Card) {
	if gs.Rules.DealShape == DealShapeAllBeforeBidding {
		dealAllBeforeBidding(gs, deck)
		syncFaceDownCounts(gs)
		return
	}

	cardIdx := 0
	dealer := gs.DealerSeat

	for i := 0; i < 4; i++ {
		seat := gs.Rules.SeatAfter(dealer, 1+i)
		gs.Players[seat].Hand = append(gs.Players[seat].Hand, slices.Clone(deck[cardIdx:cardIdx+3])...)
		cardIdx += 3
	}

	for i := 0; i < 4; i++ {
		seat := gs.Rules.SeatAfter(dealer, 1+i)
		gs.Players[seat].Hand = append(gs.Players[seat].Hand, slices.Clone(deck[cardIdx:cardIdx+2])...)
		cardIdx += 2
	}

	candidate := deck[cardIdx]
	gs.TrumpCandidate = &candidate
	cardIdx++

	gs.Deck = slices.Clone(deck[cardIdx:])

	syncFaceDownCounts(gs)
}

// dealAllBeforeBidding deals 3+3 open cards per seat into Hand and the last 2 face-down into FaceDownCards.
// There is no trump candidate and Deck stays empty.
func dealAllBeforeBidding(gs *GameState, deck []Card) {
	cardIdx := 0
	dealer := gs.DealerSeat

	for batch := 0; batch < 2; batch++ {
		for i := 0; i < 4; i++ {
			seat := gs.Rules.SeatAfter(dealer, 1+i)
			gs.Players[seat].Hand = append(gs.Players[seat].Hand, slices.Clone(deck[cardIdx:cardIdx+3])...)
			cardIdx += 3
		}
	}

	for i := 0; i < 4; i++ {
		seat := gs.Rules.SeatAfter(dealer, 1+i)
		gs.Players[seat].FaceDownCards = slices.Clone(deck[cardIdx : cardIdx+2])
		cardIdx += 2
	}

	gs.TrumpCandidate = nil
	gs.Deck = nil
}
