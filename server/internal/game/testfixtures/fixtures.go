package testfixtures

import (
	"time"

	"github.com/emilijan/beljot/server/internal/game"
)

func NewGameJustDealt() *game.GameState {
	trumpCandidate := game.Card{Rank: game.RankAce, Suit: game.SuitHearts}

	return &game.GameState{
		RoomID:           1,
		Variant:          game.VariantBitola,
		Rules:            game.RulesFor(game.VariantBitola),
		MatchMode:        "1001",
		Phase:            game.PhaseBidding,
		HandNumber:       1,
		DealerSeat:       0,
		TrumpCandidate:   &trumpCandidate,
		BiddingRound:     1,
		BiddingPassCount: 0,
		ActivePlayerSeat: 1,
		TrickNumber:      0,
		CurrentTrick:     []game.TrickCard{},
		Deck:             biddingDeck(),
		Players: [4]game.PlayerState{
			{
				Hand:         lowRanksOfSuit(game.SuitSpades),
				Seat:         0,
				UserID:       10,
				Team:         "teamA",
				Declarations: []game.Declaration{},
				Connected:    true,
			},
			{
				Hand:         lowRanksOfSuit(game.SuitHearts),
				Seat:         1,
				UserID:       20,
				Team:         "teamB",
				Declarations: []game.Declaration{},
				Connected:    true,
			},
			{
				Hand:         lowRanksOfSuit(game.SuitDiamonds),
				Seat:         2,
				UserID:       30,
				Team:         "teamA",
				Declarations: []game.Declaration{},
				Connected:    true,
			},
			{
				Hand:         lowRanksOfSuit(game.SuitClubs),
				Seat:         3,
				UserID:       40,
				Team:         "teamB",
				Declarations: []game.Declaration{},
				Connected:    true,
			},
		},
	}
}

// lowRanksOfSuit returns the five low cards (7..J) of the given suit, in
// ascending order. Used as the stage-1 dealt hand in NewGameJustDealt.
func lowRanksOfSuit(suit game.Suit) []game.Card {
	return []game.Card{
		{Rank: game.Rank7, Suit: suit},
		{Rank: game.Rank8, Suit: suit},
		{Rank: game.Rank9, Suit: suit},
		{Rank: game.RankTen, Suit: suit},
		{Rank: game.RankJack, Suit: suit},
	}
}

// biddingDeck returns the 11-card stage-2 reserve for NewGameJustDealt.
// Its order ensures no pick by any seat leaves all 8 trump cards in one hand.
func biddingDeck() []game.Card {
	return []game.Card{
		{Rank: game.RankQueen, Suit: game.SuitSpades},
		{Rank: game.RankKing, Suit: game.SuitSpades},
		{Rank: game.RankQueen, Suit: game.SuitHearts},
		{Rank: game.RankKing, Suit: game.SuitHearts},
		{Rank: game.RankAce, Suit: game.SuitSpades},
		{Rank: game.RankQueen, Suit: game.SuitDiamonds},
		{Rank: game.RankKing, Suit: game.SuitDiamonds},
		{Rank: game.RankAce, Suit: game.SuitDiamonds},
		{Rank: game.RankQueen, Suit: game.SuitClubs},
		{Rank: game.RankKing, Suit: game.SuitClubs},
		{Rank: game.RankAce, Suit: game.SuitClubs},
	}
}

func NewGameMidPlay(trickNum int) *game.GameState {
	if trickNum < 1 {
		trickNum = 1
	}
	if trickNum > 8 {
		trickNum = 8
	}

	trumpSuit := game.SuitHearts
	callerSeat := 1

	gs := &game.GameState{
		RoomID:               1,
		Variant:              game.VariantBitola,
		Rules:                game.RulesFor(game.VariantBitola),
		MatchMode:            "1001",
		Phase:                game.PhasePlaying,
		HandNumber:           1,
		DealerSeat:           0,
		TrumpSuit:            &trumpSuit,
		TrumpCallerSeat:      &callerSeat,
		BiddingRound:         1,
		TrickNumber:          trickNum,
		CurrentTrick:         []game.TrickCard{},
		DeclarationsResolved: true,
		BelotAnnounced:       true,
		Players: [4]game.PlayerState{
			{Seat: 0, UserID: 10, Team: "teamA", Declarations: []game.Declaration{}, Connected: true},
			{Seat: 1, UserID: 20, Team: "teamB", Declarations: []game.Declaration{}, Connected: true},
			{Seat: 2, UserID: 30, Team: "teamA", Declarations: []game.Declaration{}, Connected: true},
			{Seat: 3, UserID: 40, Team: "teamB", Declarations: []game.Declaration{}, Connected: true},
		},
		ActivePlayerSeat: 0,
	}

	fullHands := [4][]game.Card{
		{
			{Rank: game.RankAce, Suit: game.SuitSpades},
			{Rank: game.RankTen, Suit: game.SuitSpades},
			{Rank: game.RankKing, Suit: game.SuitSpades},
			{Rank: game.RankQueen, Suit: game.SuitSpades},
			{Rank: game.RankAce, Suit: game.SuitHearts},
			{Rank: game.RankTen, Suit: game.SuitHearts},
			{Rank: game.RankKing, Suit: game.SuitDiamonds},
			{Rank: game.Rank7, Suit: game.SuitClubs},
		},
		{
			{Rank: game.RankJack, Suit: game.SuitSpades},
			{Rank: game.Rank9, Suit: game.SuitSpades},
			{Rank: game.Rank8, Suit: game.SuitSpades},
			{Rank: game.Rank7, Suit: game.SuitSpades},
			{Rank: game.RankJack, Suit: game.SuitHearts},
			{Rank: game.Rank9, Suit: game.SuitHearts},
			{Rank: game.RankQueen, Suit: game.SuitDiamonds},
			{Rank: game.Rank8, Suit: game.SuitClubs},
		},
		{
			{Rank: game.RankAce, Suit: game.SuitDiamonds},
			{Rank: game.RankTen, Suit: game.SuitDiamonds},
			{Rank: game.RankKing, Suit: game.SuitHearts},
			{Rank: game.RankQueen, Suit: game.SuitHearts},
			{Rank: game.Rank8, Suit: game.SuitHearts},
			{Rank: game.Rank7, Suit: game.SuitHearts},
			{Rank: game.RankAce, Suit: game.SuitClubs},
			{Rank: game.RankTen, Suit: game.SuitClubs},
		},
		{
			{Rank: game.RankJack, Suit: game.SuitDiamonds},
			{Rank: game.Rank9, Suit: game.SuitDiamonds},
			{Rank: game.Rank8, Suit: game.SuitDiamonds},
			{Rank: game.Rank7, Suit: game.SuitDiamonds},
			{Rank: game.RankKing, Suit: game.SuitClubs},
			{Rank: game.RankQueen, Suit: game.SuitClubs},
			{Rank: game.RankJack, Suit: game.SuitClubs},
			{Rank: game.Rank9, Suit: game.SuitClubs},
		},
	}

	cardsPerPlayer := 8 - (trickNum - 1)
	for i := range gs.Players {
		if cardsPerPlayer < len(fullHands[i]) {
			gs.Players[i].Hand = make([]game.Card, cardsPerPlayer)
			copy(gs.Players[i].Hand, fullHands[i][:cardsPerPlayer])
		} else {
			gs.Players[i].Hand = make([]game.Card, len(fullHands[i]))
			copy(gs.Players[i].Hand, fullHands[i])
		}
	}

	if trickNum > 1 {
		gs.HandPoints = [2]int{10 * (trickNum - 1), 5 * (trickNum - 1)}
		gs.TricksWon = [2]int{(trickNum - 1 + 1) / 2, (trickNum - 1) / 2}
	}

	return gs
}

func NewGameFirstTrick(trump game.Suit) *game.GameState {
	trumpSuit := trump
	callerSeat := 1

	gs := &game.GameState{
		RoomID:           1,
		Variant:          game.VariantBitola,
		Rules:            game.RulesFor(game.VariantBitola),
		MatchMode:        "1001",
		Phase:            game.PhasePlaying,
		HandNumber:       1,
		DealerSeat:       0,
		TrumpSuit:        &trumpSuit,
		TrumpCallerSeat:  &callerSeat,
		BiddingRound:     1,
		TrickNumber:      1,
		CurrentTrick:     []game.TrickCard{},
		ActivePlayerSeat: 1,
		Players: [4]game.PlayerState{
			{
				Hand: []game.Card{
					{Rank: game.RankKing, Suit: game.SuitHearts},
					{Rank: game.RankQueen, Suit: game.SuitHearts},
					{Rank: game.Rank7, Suit: game.SuitSpades},
					{Rank: game.Rank8, Suit: game.SuitSpades},
					{Rank: game.Rank9, Suit: game.SuitSpades},
					{Rank: game.Rank7, Suit: game.SuitClubs},
					{Rank: game.Rank8, Suit: game.SuitClubs},
					{Rank: game.Rank9, Suit: game.SuitClubs},
				},
				Seat: 0, UserID: 10, Team: "teamA", Declarations: []game.Declaration{}, Connected: true,
			},
			{
				Hand: []game.Card{
					{Rank: game.RankJack, Suit: game.SuitDiamonds},
					{Rank: game.RankQueen, Suit: game.SuitDiamonds},
					{Rank: game.RankKing, Suit: game.SuitDiamonds},
					{Rank: game.RankAce, Suit: game.SuitDiamonds},
					{Rank: game.RankQueen, Suit: game.SuitSpades},
					{Rank: game.Rank8, Suit: game.SuitDiamonds},
					{Rank: game.Rank9, Suit: game.SuitDiamonds},
					{Rank: game.RankTen, Suit: game.SuitClubs},
				},
				Seat: 1, UserID: 20, Team: "teamB", Declarations: []game.Declaration{}, Connected: true,
			},
			{
				Hand: []game.Card{
					{Rank: game.RankJack, Suit: game.SuitHearts},
					{Rank: game.Rank9, Suit: game.SuitHearts},
					{Rank: game.RankAce, Suit: game.SuitHearts},
					{Rank: game.RankTen, Suit: game.SuitHearts},
					{Rank: game.RankKing, Suit: game.SuitSpades},
					{Rank: game.RankAce, Suit: game.SuitSpades},
					{Rank: game.RankQueen, Suit: game.SuitClubs},
					{Rank: game.RankJack, Suit: game.SuitClubs},
				},
				Seat: 2, UserID: 30, Team: "teamA", Declarations: []game.Declaration{}, Connected: true,
			},
			{
				Hand: []game.Card{
					{Rank: game.Rank7, Suit: game.SuitHearts},
					{Rank: game.Rank8, Suit: game.SuitHearts},
					{Rank: game.RankTen, Suit: game.SuitDiamonds},
					{Rank: game.Rank7, Suit: game.SuitDiamonds},
					{Rank: game.RankTen, Suit: game.SuitSpades},
					{Rank: game.RankJack, Suit: game.SuitSpades},
					{Rank: game.RankAce, Suit: game.SuitClubs},
					{Rank: game.RankKing, Suit: game.SuitClubs},
				},
				Seat: 3, UserID: 40, Team: "teamB", Declarations: []game.Declaration{}, Connected: true,
			},
		},
	}

	return gs
}

// NewGameWithDeclarations returns a GameState at trick 1 with pre-populated
// declarations on the given player seats.
func NewGameWithDeclarations(decls []game.Declaration) *game.GameState {
	gs := NewGameFirstTrick(game.SuitHearts)

	for _, d := range decls {
		gs.Players[d.PlayerSeat].Declarations = append(
			gs.Players[d.PlayerSeat].Declarations, d,
		)
	}

	return gs
}

func NewGameLastTrick() *game.GameState {
	trumpSuit := game.SuitHearts
	callerSeat := 1
	trickWinner := 0

	return &game.GameState{
		RoomID:               1,
		Variant:              game.VariantBitola,
		Rules:                game.RulesFor(game.VariantBitola),
		MatchMode:            "1001",
		Phase:                game.PhasePlaying,
		HandNumber:           1,
		DealerSeat:           0,
		TrumpSuit:            &trumpSuit,
		TrumpCallerSeat:      &callerSeat,
		TrickNumber:          8,
		CurrentTrick:         []game.TrickCard{},
		TrickWinnerSeat:      &trickWinner,
		DeclarationsResolved: true,
		BelotAnnounced:       true,
		ActivePlayerSeat:     0,
		Players: [4]game.PlayerState{
			{
				Hand:         []game.Card{{Rank: game.RankAce, Suit: game.SuitSpades}},
				Seat:         0,
				UserID:       10,
				Team:         "teamA",
				Declarations: nil,
				Connected:    true,
			},
			{
				Hand:         []game.Card{{Rank: game.Rank8, Suit: game.SuitDiamonds}},
				Seat:         1,
				UserID:       20,
				Team:         "teamB",
				Declarations: nil,
				Connected:    true,
			},
			{
				Hand:         []game.Card{{Rank: game.RankTen, Suit: game.SuitDiamonds}},
				Seat:         2,
				UserID:       30,
				Team:         "teamA",
				Declarations: nil,
				Connected:    true,
			},
			{
				Hand:         []game.Card{{Rank: game.Rank7, Suit: game.SuitHearts}},
				Seat:         3,
				UserID:       40,
				Team:         "teamB",
				Declarations: nil,
				Connected:    true,
			},
		},
		TeamScores:    [2]int{0, 0},
		HandPoints:    [2]int{70, 61},
		TricksWon:     [2]int{4, 3},
		TurnExpiresAt: nil,
	}
}

func NewGameCapotInProgress() *game.GameState {
	trumpSuit := game.SuitHearts
	callerSeat := 0
	trickWinner := 0

	return &game.GameState{
		RoomID:               1,
		Variant:              game.VariantBitola,
		Rules:                game.RulesFor(game.VariantBitola),
		MatchMode:            "1001",
		Phase:                game.PhasePlaying,
		HandNumber:           1,
		DealerSeat:           0,
		TrumpSuit:            &trumpSuit,
		TrumpCallerSeat:      &callerSeat,
		TrickNumber:          8,
		CurrentTrick:         []game.TrickCard{},
		TrickWinnerSeat:      &trickWinner,
		DeclarationsResolved: true,
		BelotAnnounced:       true,
		ActivePlayerSeat:     0,
		Players: [4]game.PlayerState{
			{
				Hand:         []game.Card{{Rank: game.RankJack, Suit: game.SuitHearts}},
				Seat:         0,
				UserID:       10,
				Team:         "teamA",
				Declarations: nil,
				Connected:    true,
			},
			{
				Hand:         []game.Card{{Rank: game.Rank7, Suit: game.SuitDiamonds}},
				Seat:         1,
				UserID:       20,
				Team:         "teamB",
				Declarations: nil,
				Connected:    true,
			},
			{
				Hand:         []game.Card{{Rank: game.RankAce, Suit: game.SuitHearts}},
				Seat:         2,
				UserID:       30,
				Team:         "teamA",
				Declarations: nil,
				Connected:    true,
			},
			{
				Hand:         []game.Card{{Rank: game.Rank7, Suit: game.SuitClubs}},
				Seat:         3,
				UserID:       40,
				Team:         "teamB",
				Declarations: nil,
				Connected:    true,
			},
		},
		TeamScores:    [2]int{0, 0},
		HandPoints:    [2]int{121, 0},
		TricksWon:     [2]int{7, 0},
		TurnExpiresAt: nil,
	}
}

func NewGameNearEnd(teamAScore, teamBScore int) *game.GameState {
	gs := NewGameLastTrick()
	gs.TeamScores = [2]int{teamAScore, teamBScore}
	return gs
}

func NewGamePaused(pausedBySeat int) *game.GameState {
	gs := NewGameMidPlay(1)
	gs.Phase = game.PhasePaused
	gs.PreviousPhase = game.PhasePlaying
	gs.PausedPlayers[pausedBySeat] = true
	gs.PauseUsed[pausedBySeat] = true
	return gs
}

// NewGamePausedWithOwner returns a GameState in PhasePaused with the specified
// seat having an active pause and the specified seat as room owner.
func NewGamePausedWithOwner(pausedBySeat int, ownerSeat int) *game.GameState {
	gs := NewGamePaused(pausedBySeat)
	gs.OwnerSeat = ownerSeat
	return gs
}

func NewGameDisconnected(disconnectedSeat int) *game.GameState {
	gs := NewGameMidPlay(1)
	gs.Phase = game.PhaseDisconnected
	gs.PreviousPhase = game.PhasePlaying
	gs.DisconnectedSeat = disconnectedSeat
	gs.Players[disconnectedSeat].Connected = false
	return gs
}

func NewGameReconnecting(disconnectedSeat int) *game.GameState {
	gs := NewGameMidPlay(1)
	gs.Phase = game.PhaseDisconnected
	gs.PreviousPhase = game.PhasePlaying
	gs.DisconnectedSeat = disconnectedSeat
	gs.Players[disconnectedSeat].Connected = false
	reconnectExpiry := time.Now().Add(2 * time.Minute)
	gs.ReconnectExpiresAt = &reconnectExpiry
	gs.TurnTimeRemaining = 15000
	gs.TurnExpiresAt = nil
	return gs
}

func NewGameMidBidding(passCount int) *game.GameState {
	gs := NewGameJustDealt()

	if passCount <= 0 {
		return gs
	}

	if passCount > 7 {
		passCount = 7
	}

	if passCount <= 4 {
		passesInRound := passCount
		if passesInRound == 4 {
			gs.BiddingRound = 2
			gs.BiddingPassCount = 0
			gs.ActivePlayerSeat = (gs.DealerSeat + 1) % 4
		} else {
			gs.BiddingPassCount = passesInRound
			gs.ActivePlayerSeat = (1 + passesInRound) % 4
		}
	} else {
		passesInRound2 := passCount - 4
		gs.BiddingRound = 2
		gs.BiddingPassCount = passesInRound2
		gs.ActivePlayerSeat = (1 + passesInRound2) % 4
	}

	gs.MustPickTrump = game.MustPickTrump(gs, gs.ActivePlayerSeat)

	return gs
}

func NewGameCroatianJustDealt() *game.GameState {
	c := func(r game.Rank, s game.Suit) game.Card { return game.Card{Rank: r, Suit: s} }

	gs := &game.GameState{
		RoomID:           1,
		Variant:          game.VariantCroatia,
		Rules:            game.RulesFor(game.VariantCroatia),
		MatchMode:        "1001",
		Phase:            game.PhaseBidding,
		HandNumber:       1,
		DealerSeat:       0,
		TrumpCandidate:   nil,
		BiddingRound:     1,
		BiddingPassCount: 0,
		ActivePlayerSeat: 1,
		TrickNumber:      0,
		CurrentTrick:     []game.TrickCard{},
		Deck:             nil,
		Players: [4]game.PlayerState{
			{
				Hand: []game.Card{
					c(game.Rank7, game.SuitSpades), c(game.Rank8, game.SuitSpades),
					c(game.Rank9, game.SuitSpades), c(game.RankTen, game.SuitSpades),
					c(game.Rank7, game.SuitHearts), c(game.Rank8, game.SuitHearts),
				},
				FaceDownCards: []game.Card{
					c(game.RankJack, game.SuitSpades), c(game.RankQueen, game.SuitSpades),
				},
				Seat:         0,
				UserID:       10,
				Team:         "teamA",
				Declarations: []game.Declaration{},
				Connected:    true,
			},
			{
				Hand: []game.Card{
					c(game.Rank7, game.SuitDiamonds), c(game.Rank8, game.SuitDiamonds),
					c(game.Rank9, game.SuitDiamonds), c(game.RankTen, game.SuitDiamonds),
					c(game.Rank9, game.SuitHearts), c(game.RankTen, game.SuitHearts),
				},
				FaceDownCards: []game.Card{
					c(game.RankJack, game.SuitDiamonds), c(game.RankQueen, game.SuitDiamonds),
				},
				Seat:         1,
				UserID:       20,
				Team:         "teamB",
				Declarations: []game.Declaration{},
				Connected:    true,
			},
			{
				Hand: []game.Card{
					c(game.Rank7, game.SuitClubs), c(game.Rank8, game.SuitClubs),
					c(game.Rank9, game.SuitClubs), c(game.RankTen, game.SuitClubs),
					c(game.RankJack, game.SuitHearts), c(game.RankQueen, game.SuitHearts),
				},
				FaceDownCards: []game.Card{
					c(game.RankJack, game.SuitClubs), c(game.RankQueen, game.SuitClubs),
				},
				Seat:         2,
				UserID:       30,
				Team:         "teamA",
				Declarations: []game.Declaration{},
				Connected:    true,
			},
			{
				Hand: []game.Card{
					c(game.RankKing, game.SuitSpades), c(game.RankAce, game.SuitSpades),
					c(game.RankKing, game.SuitDiamonds), c(game.RankAce, game.SuitDiamonds),
					c(game.RankKing, game.SuitHearts), c(game.RankAce, game.SuitHearts),
				},
				FaceDownCards: []game.Card{
					c(game.RankKing, game.SuitClubs), c(game.RankAce, game.SuitClubs),
				},
				Seat:         3,
				UserID:       40,
				Team:         "teamB",
				Declarations: []game.Declaration{},
				Connected:    true,
			},
		},
	}

	for i := range gs.Players {
		gs.Players[i].FaceDownCount = len(gs.Players[i].FaceDownCards)
	}
	return gs
}

func NewGameCroatianMidBidding(passCount int) *game.GameState {
	gs := NewGameCroatianJustDealt()

	if passCount <= 0 {
		return gs
	}
	if passCount > 3 {
		passCount = 3
	}

	gs.BiddingPassCount = passCount
	gs.ActivePlayerSeat = (1 + passCount) % 4
	gs.MustPickTrump = game.MustPickTrump(gs, gs.ActivePlayerSeat)
	return gs
}

func NewGameCroatianDeclaring(trump game.Suit) *game.GameState {
	gs := NewGameCroatianJustDealt()

	picked, err := game.ApplyAction(gs, game.Action{
		Type:       game.ActionPickTrump,
		PlayerSeat: gs.ActivePlayerSeat,
		Suit:       &trump,
	})
	if err != nil {
		panic("testfixtures: Croatian pick_trump rejected: " + err.Error())
	}
	return picked
}

func NewGameCroatianFirstTrick(trump game.Suit) *game.GameState {
	gs := NewGameFirstTrick(trump)
	gs.Variant = game.VariantCroatia
	gs.Rules = game.RulesFor(game.VariantCroatia)
	return gs
}

func WithoutDeclarations(gs *game.GameState) *game.GameState {
	gs.Rules.DeclarationsEnabled = false
	gs.DeclarationsEnabled = false
	gs.DeclarationsResolved = true
	return gs
}

func WithStopAtTarget(gs *game.GameState) *game.GameState {
	gs.Rules.StopAtTarget = true
	gs.StopAtTarget = true
	return gs
}

func WithScoredPreviousHand(gs *game.GameState, handNumber int) *game.GameState {
	if handNumber < 2 {
		panic("testfixtures: WithScoredPreviousHand needs handNumber >= 2 — hand 1 has no previous hand")
	}
	gs.HandNumber = handNumber
	gs.LastHandResult = &game.HandScore{
		TeamACardPoints: 81,
		TeamBCardPoints: 81,
		TeamADeclPoints: 20,
		TeamBDeclPoints: 0,
		LastTrickTeam:   game.TeamA,
		LastTrickSeat:   0,
		LastTrickBonus:  10,
		Capot:           false,
		CapotTeam:       nil,
		CapotBonus:      0,
		FailedContract:  false,
		ContractingTeam: game.TeamA,
		TeamAHandTotal:  111,
		TeamBHandTotal:  81,
	}
	return gs
}

func NewGameMidPlayWithScores(trickNum, teamAScore, teamBScore int) *game.GameState {
	gs := NewGameMidPlay(trickNum)
	gs.TeamScores = [2]int{teamAScore, teamBScore}
	return gs
}
