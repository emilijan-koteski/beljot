package testfixtures

import (
	"time"

	"github.com/emilijan/beljot/server/internal/game"
)

// NewGameJustDealt returns a valid GameState in the BIDDING phase after a
// Bitola stage-1 deal: each player holds 5 cards, the trump candidate is
// public (AH on the table), and 11 cards remain in Deck for stage-2
// distribution after a player picks trump.
//
// Layout is deterministic — each seat holds 5 cards of its assigned suit;
// the candidate is the Ace of Hearts; the Deck holds the remaining 11 cards
// in deal order. The layout is engineered so that picking trump (in either
// round, by any seat) never accidentally triggers instant-win, while still
// allowing tests to mutate hand[0] for partial-trump scenarios.
//
//	Initial hands (5 cards each):
//	  Seat 0 (team A):  7S 8S 9S TS JS   (5 Spades, low ranks)
//	  Seat 1 (team B): 7H 8H 9H TH JH   (5 Hearts, low ranks)
//	  Seat 2 (team A):  7D 8D 9D TD JD   (5 Diamonds, low ranks)
//	  Seat 3 (team B): 7C 8C 9C TC JC   (5 Clubs, low ranks)
//
//	TrumpCandidate (public, on table): AH
//
//	Deck (11 cards, deal order): QS KS QH KH AS QD KD AD QC KC AC
//
// Dealer: seat 0, Active bidder: seat 1.
func NewGameJustDealt() *game.GameState {
	trumpCandidate := game.Card{Rank: game.RankAce, Suit: game.SuitHearts}

	return &game.GameState{
		RoomID:           1,
		Variant:          game.VariantBitola,
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
// Order is chosen so that no pick (round 1 or round 2, by any seat) ends
// with all 8 cards of trump suit in one hand.
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

// NewGameMidPlay returns a GameState in the playing phase at the specified
// trick number. Trump is Hearts, trump caller is seat 1. Hands contain mixed
// suits for testing suit-following, trump obligation, and partner exemption.
//
// trickNum 1: 8 cards per player (fresh playing state)
// trickNum 2-7: reduced hand sizes (8 - (trickNum-1) cards each)
// trickNum 8: 1 card per player (final trick)
//
// Hand distribution for trickNum=1 (trump=Hearts):
//
//	Seat 0 (team A):  AS TS KS QS AH TH KD 7C  (4S, 2H-trump, 1D, 1C)
//	Seat 1 (team B): JS 9S 8S 7S JH 9H QD 8C  (4S, 2H-trump, 1D, 1C)
//	Seat 2 (team A):  AD TD KH QH 8H 7H AC TC  (2D, 4H-trump, 2C)
//	Seat 3 (team B): JD 9D 8D 7D KC QC JC 9C  (4D, 0H-trump, 4C)
//
// Active player: seat 0, dealer: seat 0.
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
		MatchMode:            "1001",
		Phase:                game.PhasePlaying,
		HandNumber:           1,
		DealerSeat:           0,
		TrumpSuit:            &trumpSuit,
		TrumpCallerSeat:      &callerSeat,
		BiddingRound:         1,
		TrickNumber:          trickNum,
		CurrentTrick:         []game.TrickCard{},
		DeclarationsResolved: true, // declarations already handled (for card play testing)
		BelotAnnounced:       true, // Belot already handled (for card play testing)
		Players: [4]game.PlayerState{
			{Seat: 0, UserID: 10, Team: "teamA", Declarations: []game.Declaration{}, Connected: true},
			{Seat: 1, UserID: 20, Team: "teamB", Declarations: []game.Declaration{}, Connected: true},
			{Seat: 2, UserID: 30, Team: "teamA", Declarations: []game.Declaration{}, Connected: true},
			{Seat: 3, UserID: 40, Team: "teamB", Declarations: []game.Declaration{}, Connected: true},
		},
		ActivePlayerSeat: 0,
	}

	// Full hands for trick 1 — mixed suits for rule testing
	fullHands := [4][]game.Card{
		{ // Seat 0: 4 Spades, 2 trump Hearts, 1 Diamond, 1 Club
			{Rank: game.RankAce, Suit: game.SuitSpades},
			{Rank: game.RankTen, Suit: game.SuitSpades},
			{Rank: game.RankKing, Suit: game.SuitSpades},
			{Rank: game.RankQueen, Suit: game.SuitSpades},
			{Rank: game.RankAce, Suit: game.SuitHearts},
			{Rank: game.RankTen, Suit: game.SuitHearts},
			{Rank: game.RankKing, Suit: game.SuitDiamonds},
			{Rank: game.Rank7, Suit: game.SuitClubs},
		},
		{ // Seat 1: 4 Spades, 2 trump Hearts, 1 Diamond, 1 Club
			{Rank: game.RankJack, Suit: game.SuitSpades},
			{Rank: game.Rank9, Suit: game.SuitSpades},
			{Rank: game.Rank8, Suit: game.SuitSpades},
			{Rank: game.Rank7, Suit: game.SuitSpades},
			{Rank: game.RankJack, Suit: game.SuitHearts},
			{Rank: game.Rank9, Suit: game.SuitHearts},
			{Rank: game.RankQueen, Suit: game.SuitDiamonds},
			{Rank: game.Rank8, Suit: game.SuitClubs},
		},
		{ // Seat 2: 2 Diamonds, 4 trump Hearts, 2 Clubs
			{Rank: game.RankAce, Suit: game.SuitDiamonds},
			{Rank: game.RankTen, Suit: game.SuitDiamonds},
			{Rank: game.RankKing, Suit: game.SuitHearts},
			{Rank: game.RankQueen, Suit: game.SuitHearts},
			{Rank: game.Rank8, Suit: game.SuitHearts},
			{Rank: game.Rank7, Suit: game.SuitHearts},
			{Rank: game.RankAce, Suit: game.SuitClubs},
			{Rank: game.RankTen, Suit: game.SuitClubs},
		},
		{ // Seat 3: 4 Diamonds, 0 trump Hearts, 4 Clubs
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

	// For trickNum > 1, trim hands from the end to simulate previous tricks
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

	// Simulate accumulated scores from previous tricks
	if trickNum > 1 {
		gs.HandPoints = [2]int{10 * (trickNum - 1), 5 * (trickNum - 1)}
		gs.TricksWon = [2]int{(trickNum - 1 + 1) / 2, (trickNum - 1) / 2}
	}

	return gs
}

// NewGameFirstTrick returns a GameState at trick 1 designed for testing
// declarations and Belot bonus. DeclarationsResolved and BelotAnnounced
// are false. AwaitingDeclaration is NOT pre-set (caller controls flow).
//
// Hand distribution (trump = Hearts always, parameter reserved for future use):
//
//	Seat 0 (team A):  KH QH 7S 8S 9S 7C 8C 9C   → Belot (KH+QH), tierce 7S-8S-9S (20pts), tierce 7C-8C-9C (20pts)
//	Seat 1 (team B): JD QD KD AD QS 8D 9D TC    → quarte JD-QD-KD-AD (50pts)
//	Seat 2 (team A):  JH 9H AH TH KS AS QC JC   → tierce 9H-TH-JH in trump (20pts)
//	Seat 3 (team B): 7H 8H TD 7D TS AC KC       → no declarations (max 2 consecutive per suit, no four-of-a-kind)
//
// Wait - seat 3 has 7 cards above. Let me add JS... no that's at seat 2.
// Corrected seat 3: 7H 8H TD 7D TS JS AC KC = 8 cards. JS(4),TS(3) = 2 consecutive spades.
//
// All 32 cards verified: H(KH QH JH 9H AH TH 7H 8H) S(7S 8S 9S QS JS KS AS TS)
// D(JD QD KD AD 8D 9D TD 7D) C(7C 8C 9C TC AC KC QC JC)... wait, QC and JC are missing.
//
// FINAL verified distribution (all 32 unique cards, 8 per player):
//
//	Seat 0 (team A):  KH QH 7S 8S 9S 7C 8C 9C   → Belot, tierce 7S-8S-9S, tierce 7C-8C-9C
//	Seat 1 (team B): JD QD KD AD QS 8D 9D TC    → quarte JD-QD-KD-AD
//	Seat 2 (team A):  JH 9H AH TH KS AS JC QC   → tierce 9H-TH-JH (trump, 20pts)
//	Seat 3 (team B): 7H 8H TD 7D TS JS AC KC   → no declarations
//
// Active player: seat 1 (player after dealer). Dealer: seat 0.
func NewGameFirstTrick(trump game.Suit) *game.GameState {
	trumpSuit := trump
	callerSeat := 1

	gs := &game.GameState{
		RoomID:           1,
		Variant:          game.VariantBitola,
		MatchMode:        "1001",
		Phase:            game.PhasePlaying,
		HandNumber:       1,
		DealerSeat:       0,
		TrumpSuit:        &trumpSuit,
		TrumpCallerSeat:  &callerSeat,
		BiddingRound:     1,
		TrickNumber:      1,
		CurrentTrick:     []game.TrickCard{},
		ActivePlayerSeat: 1, // player after dealer
		Players: [4]game.PlayerState{
			{ // Seat 0 (team A): KH QH 7S 8S 9S 7C 8C 9C → Belot + 2 tierces
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
			{ // Seat 1 (team B): JD QD KD AD QS 8D 9D TC → quarte JD-QD-KD-AD
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
			{ // Seat 2 (team A): JH 9H AH TH KS AS QC JC → tierce 9H-TH-JH (trump)
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
			{ // Seat 3 (team B): 7H 8H TD 7D TS JS AC KC → no declarations
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
// declarations on the given player seats. Used for testing resolveDeclarations
// without going through the full declare flow.
func NewGameWithDeclarations(decls []game.Declaration) *game.GameState {
	gs := NewGameFirstTrick(game.SuitHearts)

	// Store declarations on respective player seats
	for _, d := range decls {
		gs.Players[d.PlayerSeat].Declarations = append(
			gs.Players[d.PlayerSeat].Declarations, d,
		)
	}

	return gs
}

// NewGameLastTrick returns a GameState at trick 8 (the final trick) for testing
// hand-end scoring: last-trick bonus, Capot detection, failed contracts, and
// match score updates.
//
// Trump = Hearts, TrumpCallerSeat = seat 1 (team B).
// TricksWon = [4, 3] — team A won 4 tricks, team B won 3 (7 total, 8th pending).
// HandPoints = [70, 61] — realistic card point distribution from 7 tricks.
// DeclarationPoints = [0, 0] — no declarations (simplifies scoring tests).
//
// Remaining cards (1 per player):
//
//	Seat 0 (team A):  AS — Ace of Spades (11 pts non-trump)
//	Seat 1 (team B): 8D — 8 of Diamonds (0 pts)
//	Seat 2 (team A):  TD — Ten of Diamonds (10 pts non-trump)
//	Seat 3 (team B): 7H — 7 of Hearts (0 pts trump — can win if Hearts led)
//
// Remaining card points: 11 + 0 + 10 + 0 = 21. Total from 7 tricks: 152 - 21 = 131.
// HandPoints 70 + 61 = 131 ✓
//
// ActivePlayerSeat = 0 (team A, winner of trick 7 — leads trick 8).
func NewGameLastTrick() *game.GameState {
	trumpSuit := game.SuitHearts
	callerSeat := 1  // Team B called trump
	trickWinner := 0 // Seat 0 won trick 7

	return &game.GameState{
		RoomID:               1,
		Variant:              game.VariantBitola,
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
			{ // Seat 0 (team A): AS — strong non-trump
				Hand:         []game.Card{{Rank: game.RankAce, Suit: game.SuitSpades}},
				Seat:         0,
				UserID:       10,
				Team:         "teamA",
				Declarations: nil,
				Connected:    true,
			},
			{ // Seat 1 (team B): 8D — weak non-trump
				Hand:         []game.Card{{Rank: game.Rank8, Suit: game.SuitDiamonds}},
				Seat:         1,
				UserID:       20,
				Team:         "teamB",
				Declarations: nil,
				Connected:    true,
			},
			{ // Seat 2 (team A): TD — medium non-trump (10 pts)
				Hand:         []game.Card{{Rank: game.RankTen, Suit: game.SuitDiamonds}},
				Seat:         2,
				UserID:       30,
				Team:         "teamA",
				Declarations: nil,
				Connected:    true,
			},
			{ // Seat 3 (team B): 7H — low trump (can win if led suit is hearts)
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

// NewGameCapotInProgress returns a GameState at trick 8 where one team (team A)
// has won all 7 previous tricks. Used for testing Capot detection and scoring.
//
// Trump = Hearts, TrumpCallerSeat = seat 0 (team A — the Capot team).
// TricksWon = [7, 0] — team A has won every trick so far.
// HandPoints = [121, 0] — all card points from 7 tricks go to team A.
//
// Remaining cards (1 per player):
//
//	Seat 0 (team A):  JH — Jack of Hearts (20 pts trump, strongest card)
//	Seat 1 (team B): 7D — 7 of Diamonds (0 pts)
//	Seat 2 (team A):  AH — Ace of Hearts (11 pts trump)
//	Seat 3 (team B): 7C — 7 of Clubs (0 pts)
//
// Remaining card points: 20 + 0 + 11 + 0 = 31. Total from 7 tricks: 152 - 31 = 121 ✓
//
// ActivePlayerSeat = 0 (team A, winner of trick 7).
// Team A is guaranteed to win trick 8 (JH is strongest possible trump).
func NewGameCapotInProgress() *game.GameState {
	trumpSuit := game.SuitHearts
	callerSeat := 0  // Team A called trump
	trickWinner := 0 // Seat 0 won trick 7

	return &game.GameState{
		RoomID:               1,
		Variant:              game.VariantBitola,
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
			{ // Seat 0 (team A): JH — Jack of Hearts (trump, strongest)
				Hand:         []game.Card{{Rank: game.RankJack, Suit: game.SuitHearts}},
				Seat:         0,
				UserID:       10,
				Team:         "teamA",
				Declarations: nil,
				Connected:    true,
			},
			{ // Seat 1 (team B): 7D — weakest
				Hand:         []game.Card{{Rank: game.Rank7, Suit: game.SuitDiamonds}},
				Seat:         1,
				UserID:       20,
				Team:         "teamB",
				Declarations: nil,
				Connected:    true,
			},
			{ // Seat 2 (team A): AH — Ace of Hearts (trump)
				Hand:         []game.Card{{Rank: game.RankAce, Suit: game.SuitHearts}},
				Seat:         2,
				UserID:       30,
				Team:         "teamA",
				Declarations: nil,
				Connected:    true,
			},
			{ // Seat 3 (team B): 7C — weakest
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

// NewGameNearEnd returns a GameState at trick 8 (like NewGameLastTrick) but with
// configurable team scores, for testing match completion thresholds and tiebreakers.
//
// Trump = Hearts, TrumpCallerSeat = seat 1 (team B).
// TricksWon = [4, 3] — team A won 4 tricks, team B won 3 (7 total, 8th pending).
// HandPoints = [70, 61] — realistic card point distribution from 7 tricks.
//
// Remaining cards are identical to NewGameLastTrick:
//
//	Seat 0 (team A):  AS — Ace of Spades (11 pts non-trump)
//	Seat 1 (team B): 8D — 8 of Diamonds (0 pts)
//	Seat 2 (team A):  TD — Ten of Diamonds (10 pts non-trump)
//	Seat 3 (team B): 7H — 7 of Hearts (0 pts trump)
//
// After trick 8 resolves (Seat 0 leads AS; seat 3 is void in spades and must
// trump with 7H, winning the trick for team B):
//   - Team A gets: 70 hand points
//   - Team B gets: 61 + 21(trick) + 10(last-trick) = 92 hand points
//   - Team B called trump and 92 > 70 → hand succeeds → normal scoring:
//     team A += 70, team B += 92 (see TestHandScoring_LastTrickBonus)
//
// Use teamAScore/teamBScore to position teams near the 1001/501 threshold.
func NewGameNearEnd(teamAScore, teamBScore int) *game.GameState {
	gs := NewGameLastTrick()
	gs.TeamScores = [2]int{teamAScore, teamBScore}
	return gs
}

// NewGamePaused returns a GameState in PhasePaused, transitioned from PhasePlaying.
// The specified seat has an active pause (PausedPlayers and PauseUsed set).
// PreviousPhase is set to PhasePlaying so unpause restores to playing.
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
// Used for testing owner unpause scenarios.
func NewGamePausedWithOwner(pausedBySeat int, ownerSeat int) *game.GameState {
	gs := NewGamePaused(pausedBySeat)
	gs.OwnerSeat = ownerSeat
	return gs
}

// NewGameDisconnected returns a GameState in PhaseDisconnected with the
// specified seat disconnected. PreviousPhase is PhasePlaying. Used for
// testing that all actions are rejected during the disconnected phase.
func NewGameDisconnected(disconnectedSeat int) *game.GameState {
	gs := NewGameMidPlay(1)
	gs.Phase = game.PhaseDisconnected
	gs.PreviousPhase = game.PhasePlaying
	gs.DisconnectedSeat = disconnectedSeat
	gs.Players[disconnectedSeat].Connected = false
	return gs
}

// NewGameReconnecting returns a GameState in PhaseDisconnected suitable for
// testing reconnection scenarios. The disconnected seat has Connected=false,
// DisconnectedSeat set, ReconnectExpiresAt 2 minutes in the future,
// PreviousPhase=PhasePlaying, and TurnTimeRemaining=15000ms.
func NewGameReconnecting(disconnectedSeat int) *game.GameState {
	gs := NewGameMidPlay(1)
	gs.Phase = game.PhaseDisconnected
	gs.PreviousPhase = game.PhasePlaying
	gs.DisconnectedSeat = disconnectedSeat
	gs.Players[disconnectedSeat].Connected = false
	reconnectExpiry := time.Now().Add(2 * time.Minute)
	gs.ReconnectExpiresAt = &reconnectExpiry
	gs.TurnTimeRemaining = 15000 // 15 seconds preserved from disconnect
	gs.TurnExpiresAt = nil       // Timer paused during disconnect
	return gs
}

// NewGameMidBidding returns a GameState with the specified number of passes
// already recorded. Correctly tracks BiddingRound and ActivePlayerSeat.
//
// passCount 0: same as NewGameJustDealt (round 1, seat 1 active)
// passCount 1-3: round 1 with passes applied
// passCount 4: round 2 just started (0 passes in round 2, seat 1 active)
// passCount 5-7: round 2 with passes applied
//
// Dealer is always seat 0. First bidder is seat 1.
func NewGameMidBidding(passCount int) *game.GameState {
	gs := NewGameJustDealt()

	if passCount <= 0 {
		return gs
	}

	// Clamp to valid range: max 7 (round 2 with 3 passes).
	// passCount 8 would trigger a reshuffle, not a mid-bidding state.
	if passCount > 7 {
		passCount = 7
	}

	if passCount <= 4 {
		// Round 1 passes
		passesInRound := passCount
		if passesInRound == 4 {
			// All 4 passed in round 1 — transition to round 2
			gs.BiddingRound = 2
			gs.BiddingPassCount = 0
			gs.ActivePlayerSeat = (gs.DealerSeat + 1) % 4 // reset to first bidder
		} else {
			gs.BiddingPassCount = passesInRound
			gs.ActivePlayerSeat = (1 + passesInRound) % 4 // seat 1 is first bidder
		}
	} else {
		// Round 2 passes (passCount 5-7)
		passesInRound2 := passCount - 4
		gs.BiddingRound = 2
		gs.BiddingPassCount = passesInRound2
		gs.ActivePlayerSeat = (1 + passesInRound2) % 4 // seat 1 is first bidder in round 2
	}

	return gs
}
