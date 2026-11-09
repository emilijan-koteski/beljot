package search_test

import (
	"fmt"
	"math/bits"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/bot/search"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
)

func TestRaceDecided(t *testing.T) {
	tests := []struct {
		name        string
		race        search.Race
		gained      [2]int
		wantWinner  int
		wantDecided bool
	}{
		{"nobody reaches the target", search.Race{Target: 1001, Banked: [2]int{900, 950}}, [2]int{40, 30}, 0, false},
		{"team A alone reaches it", search.Race{Target: 1001, Banked: [2]int{980, 950}}, [2]int{21, 30}, game.TeamA, true},
		{"team B alone reaches it", search.Race{Target: 1001, Banked: [2]int{980, 990}}, [2]int{0, 11}, game.TeamB, true},
		{"both reach it and A is higher", search.Race{Target: 1001, Banked: [2]int{1000, 1001}, Caller: game.TeamB}, [2]int{11, 0}, game.TeamA, true},
		{"both reach it equally and B called", search.Race{Target: 1001, Banked: [2]int{1001, 1011}, Caller: game.TeamB}, [2]int{10, 0}, game.TeamB, true},
		{"both reach it equally and A called", search.Race{Target: 1001, Banked: [2]int{1001, 1011}, Caller: game.TeamA}, [2]int{10, 0}, game.TeamA, true},
		{"card points count without any bonus", search.Race{Target: 1001, Banked: [2]int{990, 0}}, [2]int{11, 0}, game.TeamA, true},
		{"one point short is not enough", search.Race{Target: 1001, Banked: [2]int{990, 0}}, [2]int{10, 0}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			winner, decided := search.RaceDecided(tt.race, tt.gained)
			assert.Equal(t, tt.wantDecided, decided)
			if tt.wantDecided {
				assert.Equal(t, tt.wantWinner, winner)
			}
		})
	}
}

type raceEndgame struct {
	hands        [4]uint32
	spadeAce     int
	sevenOfDiams int
	race         search.Race
}

// newRaceEndgame deals two tricks with no trump in play: seat 0 leads, the spade ace takes 11 for team A at once,
// while leading the seven of diamonds gives B 15 first and A the last trick, worth more in plain points.
func newRaceEndgame(bankedA, bankedB int) raceEndgame {
	idx := func(code string) int { return search.CardIdx(cardFromCode(code)) }
	return raceEndgame{
		hands: [4]uint32{
			maskOfIndexes(idx("AS"), idx("7D")),
			maskOfIndexes(idx("7S"), idx("AD")),
			maskOfIndexes(idx("8S"), idx("8D")),
			maskOfIndexes(idx("9S"), idx("KD")),
		},
		spadeAce:     idx("AS"),
		sevenOfDiams: idx("7D"),
		race:         search.Race{Target: 1001, Banked: [2]int{bankedA, bankedB}, Caller: game.TeamA},
	}
}

func cardFromCode(code string) game.Card {
	ranks := map[byte]game.Rank{
		'7': game.Rank7, '8': game.Rank8, '9': game.Rank9, 'T': game.RankTen,
		'J': game.RankJack, 'Q': game.RankQueen, 'K': game.RankKing, 'A': game.RankAce,
	}
	suits := map[byte]game.Suit{'S': game.SuitSpades, 'H': game.SuitHearts, 'D': game.SuitDiamonds, 'C': game.SuitClubs}
	return game.Card{Rank: ranks[code[0]], Suit: suits[code[1]]}
}

func TestSolverRaceReturnsTheMatchValue(t *testing.T) {
	for _, direction := range directions {
		t.Run(direction.name, func(t *testing.T) {
			solver := search.NewSolver(hearts, direction.clockwise)
			plain := solver.RootValues(newRaceEndgame(0, 0).hands, 0, nil, 0)
			require.Greater(t, plain[newRaceEndgame(0, 0).sevenOfDiams], plain[newRaceEndgame(0, 0).spadeAce])

			near := newRaceEndgame(990, 500)
			solver.SetRace(&near.race)
			roots := solver.RootValues(near.hands, 0, nil, 0)
			assert.Equal(t, search.RaceWin, roots[near.spadeAce])
			assert.Equal(t, plain[near.sevenOfDiams], roots[near.sevenOfDiams])
			assert.Equal(t, search.RaceWin, solver.Value(near.hands, 0, nil, 0))

			far := newRaceEndgame(500, 500)
			solver.SetRace(&far.race)
			assert.Equal(t, plain, solver.RootValues(far.hands, 0, nil, 0))

			solver.SetRace(nil)
			assert.Equal(t, plain, solver.RootValues(near.hands, 0, nil, 0))
		})
	}
}

func TestChooseRacesForTheMatch(t *testing.T) {
	eg := newRaceEndgame(990, 500)
	in := search.Information{Seat: 0, Own: eg.hands[0]}
	for seat := 1; seat < 4; seat++ {
		in.Known[seat] = eg.hands[seat]
	}
	pos := search.Position{Trump: hearts, Leader: 0, Race: &eg.race}
	legal := legalFor(pos, eg.hands[0])
	settle := settlementOver(hearts, nil, eg.hands[:]...)
	opts := search.Options{Samples: 4}

	racing, ok := search.Choose(in, pos, legal, settle, opts, farFuture())
	require.True(t, ok)
	assert.Equal(t, search.IdxCard(eg.spadeAce), racing.Card)

	pos.Race = nil
	plain, ok := search.Choose(in, pos, legal, settle, opts, farFuture())
	require.True(t, ok)
	assert.Equal(t, search.IdxCard(eg.sevenOfDiams), plain.Card)
}

type raceBruteForce struct {
	bruteForce
	race search.Race
}

const beloteBonus = 20

func (b raceBruteForce) value(hands [4]uint32, leader int, trick []int, won, gained, belote [2]int) int {
	seat := game.StepSeat(leader, len(trick), b.clockwise)
	maximize := seat%2 == 0
	best, first := 0, true
	for _, card := range search.CardsOf(b.solver.Legal(hands[seat], trick)) {
		v := b.afterCard(hands, leader, trick, search.CardIdx(card), won, gained, belote)
		if first || maximize && v > best || !maximize && v < best {
			best, first = v, false
		}
	}
	return best
}

func (b raceBruteForce) afterCard(hands [4]uint32, leader int, trick []int, card int, won, gained, belote [2]int) int {
	seat := game.StepSeat(leader, len(trick), b.clockwise)
	rest := hands
	rest[seat] &^= 1 << uint(card)
	if b.announcesBelote(hands[seat], seat, card) {
		belote[game.TeamForSeat(seat)] = beloteBonus
		if winner, decided := search.RaceDecided(b.race, addTotals(gained, belote)); decided {
			return (1 - 2*winner) * search.RaceWin
		}
	}
	played := append(slices.Clone(trick), card)
	if len(played) < 4 {
		return b.value(rest, leader, played, won, gained, belote)
	}
	if rest == [4]uint32{} {
		return gained[0] - gained[1] + b.bruteForce.afterCard(hands, leader, trick, card, won)
	}
	winnerSeat := game.StepSeat(leader, b.solver.Winner(played), b.clockwise)
	team := game.TeamForSeat(winnerSeat)
	for _, c := range played {
		gained[team] += search.Points(c, b.trump)
	}
	won[team]++
	if winner, decided := search.RaceDecided(b.race, addTotals(gained, belote)); decided {
		return (1 - 2*winner) * search.RaceWin
	}
	return b.value(rest, winnerSeat, nil, won, gained, belote)
}

// announcesBelote reports that the card is the first of the race's Belote pair its holder plays.
func (b raceBruteForce) announcesBelote(hand uint32, seat, card int) bool {
	pair := b.race.BeloteCards
	return pair != 0 && seat == b.race.BeloteSeat && hand&pair == pair && pair&(1<<uint(card)) != 0
}

func addTotals(a, b [2]int) [2]int {
	return [2]int{a[0] + b[0], a[1] + b[1]}
}

func TestSolverRaceMatchesBruteForce(t *testing.T) {
	assertRaceMatchesBruteForce(t, 41, 43, false)
}

func TestSolverRaceWithABelotePairMatchesBruteForce(t *testing.T) {
	assertRaceMatchesBruteForce(t, 67, 71, true)
}

// assertRaceMatchesBruteForce races random endgames near the target against the brute force, each with a seat
// holding the trump King and Queen as its Belote when withBelote is set.
func assertRaceMatchesBruteForce(t *testing.T, seed1, seed2 uint64, withBelote bool) {
	const endgamesPerDirection = 150
	for _, direction := range directions {
		t.Run(direction.name, func(t *testing.T) {
			rng := rand.New(rand.NewPCG(seed1, seed2))
			solvers := solversByTrump(direction.clockwise)
			for i := 0; i < endgamesPerDirection; i++ {
				trump := rng.IntN(4)
				solver := solvers[trump]
				eg := randomEndgame(rng, solver, direction.clockwise)
				race := search.Race{
					Target: 1001,
					Banked: [2]int{1000 - rng.IntN(60), 1000 - rng.IntN(60)},
					Caller: rng.IntN(2),
				}
				if withBelote {
					var ok bool
					if eg, race.BeloteSeat, ok = withBelotePair(rng, eg, trump); !ok {
						i--
						continue
					}
					race.BeloteCards = belotePair(trump)
				}
				reference := raceBruteForce{
					bruteForce: bruteForce{solver: solver, trump: trump, clockwise: direction.clockwise},
					race:       race,
				}
				won := tricksWonFromFlag(eg.capot)
				solver.SetRace(&race)
				seat := game.StepSeat(eg.leader, len(eg.trick), direction.clockwise)
				roots := solver.RootValues(eg.hands, eg.leader, eg.trick, eg.capot)
				for _, card := range search.CardsOf(solver.Legal(eg.hands[seat], eg.trick)) {
					idx := search.CardIdx(card)
					want := reference.afterCard(eg.hands, eg.leader, eg.trick, idx, won, [2]int{}, [2]int{})
					require.Equal(t, want, roots[idx], "endgame %d card %d race %+v", i, idx, race)
				}
				want := reference.value(eg.hands, eg.leader, eg.trick, won, [2]int{}, [2]int{})
				require.Equal(t, want, solver.Value(eg.hands, eg.leader, eg.trick, eg.capot), "endgame %d", i)
				solver.SetRace(nil)
			}
		})
	}
}

func belotePair(trump int) uint32 {
	var pair uint32
	for _, c := range game.NewDeck() {
		if search.SuitIdx(c.Suit) == trump && (c.Rank == game.RankKing || c.Rank == game.RankQueen) {
			pair |= 1 << uint(search.CardIdx(c))
		}
	}
	return pair
}

// withBelotePair moves the trump King and Queen into a random seat holding at least two cards, swapping out cards
// of that seat, and reports false when either is already in the open trick.
func withBelotePair(rng *rand.Rand, eg endgame, trump int) (endgame, int, bool) {
	pair := belotePair(trump)
	for _, c := range eg.trick {
		if pair&(1<<uint(c)) != 0 {
			return eg, 0, false
		}
	}
	var seats []int
	for seat, hand := range eg.hands {
		if bits.OnesCount32(hand) >= 2 {
			seats = append(seats, seat)
		}
	}
	if len(seats) == 0 {
		return eg, 0, false
	}
	seat := seats[rng.IntN(len(seats))]
	for m := pair; m != 0; m &= m - 1 {
		card := uint32(1) << uint(bits.TrailingZeros32(m))
		if eg.hands[seat]&card != 0 {
			continue
		}
		others := search.CardsOf(eg.hands[seat] &^ pair)
		victim := uint32(1) << uint(search.CardIdx(others[rng.IntN(len(others))]))
		for other := range eg.hands {
			if eg.hands[other]&card != 0 {
				eg.hands[other] = eg.hands[other]&^card | victim
			}
		}
		eg.hands[seat] = eg.hands[seat]&^victim | card
	}
	return eg, seat, true
}

func TestRaceBookkeepingMatchesTheEngine(t *testing.T) {
	rng := rand.New(rand.NewPCG(47, 53))
	var decided, undecided int
	for i := range 200 {
		gs := raceDeal(rng, 800+rng.IntN(201), 800+rng.IntN(201))
		trump := search.SuitIdx(*gs.TrumpSuit)
		race := search.Race{
			Target: game.MatchTarget(gs.MatchMode),
			Banked: gs.TeamScores,
			Caller: game.TeamForSeat(*gs.TrumpCallerSeat),
		}
		solver := search.NewSolver(trump, gs.Rules.Clockwise)
		var gained [2]int
		for gs.Phase == game.PhasePlaying {
			seat := gs.ActivePlayerSeat
			legal := game.LegalCards(gs, seat)
			card := legal[rng.IntN(len(legal))]
			closing := len(gs.CurrentTrick) == 3
			lastTrick := gs.TrickNumber == 8
			trick := append(trickIndexes(gs), search.CardIdx(card))
			leader := seat
			if len(gs.CurrentTrick) > 0 {
				leader = gs.CurrentTrick[0].PlayerSeat
			}
			next, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: seat, Card: &card})
			require.NoError(t, err)
			gs = next
			if !closing {
				continue
			}
			if lastTrick {
				assert.False(t, gs.StoppedAtTarget, "deal %d: the engine settles the last trick through the hand score", i)
				break
			}
			winnerSeat := game.StepSeat(leader, solver.Winner(trick), gs.Rules.Clockwise)
			for _, c := range trick {
				gained[game.TeamForSeat(winnerSeat)] += search.Points(c, trump)
			}
			winner, isDecided := search.RaceDecided(race, gained)
			require.Equal(t, isDecided, gs.Phase == game.PhaseMatchEnd, "deal %d gained %v race %+v", i, gained, race)
			if isDecided {
				require.Equal(t, winner, *gs.WinnerTeam, "deal %d", i)
				decided++
				break
			}
			undecided++
		}
	}
	assert.Positive(t, decided)
	assert.Positive(t, undecided)
}

func trickIndexes(gs *game.GameState) []int {
	out := make([]int, len(gs.CurrentTrick))
	for i, tc := range gs.CurrentTrick {
		out[i] = search.CardIdx(tc.Card)
	}
	return out
}

// raceDeal is a stop-at-target state at the first trick with the given match scores, a random deal and trump, and
// declarations off so only card points move the running totals.
func raceDeal(rng *rand.Rand, scoreA, scoreB int) *game.GameState {
	gs := testfixtures.WithStopAtTarget(testfixtures.NewGameMidPlayWithScores(1, scoreA, scoreB))
	gs.Rules.DeclarationsEnabled = false
	trump := []game.Suit{game.SuitSpades, game.SuitHearts, game.SuitDiamonds, game.SuitClubs}[rng.IntN(4)]
	gs.TrumpSuit = &trump
	deck := game.NewDeck()
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	for seat := range gs.Players {
		gs.Players[seat].Hand = slices.Clone(deck[seat*8 : (seat+1)*8])
	}
	return gs
}

// newBeloteRaceEndgame deals three tricks with hearts trump: seat 0 leads holding the King and Queen of hearts, 15
// short of the target, while the opponents, 30 short, take 31 if it leads the seven of spades.
func newBeloteRaceEndgame() (raceEndgame, [2]int) {
	idx := func(code string) int { return search.CardIdx(cardFromCode(code)) }
	pair := maskOfIndexes(idx("KH"), idx("QH"))
	return raceEndgame{
		hands: [4]uint32{
			pair | maskOfIndexes(idx("7S")),
			maskOfIndexes(idx("JH"), idx("9D"), idx("TD")),
			maskOfIndexes(idx("AS"), idx("8D"), idx("8C")),
			maskOfIndexes(idx("7C"), idx("9C"), idx("7D")),
		},
		race: search.Race{Target: 1001, Banked: [2]int{986, 971}, Caller: game.TeamA, BeloteSeat: 0, BeloteCards: pair},
	}, [2]int{idx("KH"), idx("QH")}
}

func TestSolverRaceCreditsTheBeloteWhenItIsPlayed(t *testing.T) {
	sevenOfSpades := search.CardIdx(cardFromCode("7S"))
	for _, direction := range directions {
		t.Run(direction.name, func(t *testing.T) {
			eg, pair := newBeloteRaceEndgame()
			solver := search.NewSolver(hearts, direction.clockwise)
			solver.SetRace(&eg.race)
			roots := solver.RootValues(eg.hands, 0, nil, 0)
			assert.Equal(t, -search.RaceWin, roots[sevenOfSpades], "the opponents cross first")
			assert.Equal(t, search.RaceWin, roots[pair[0]], "the King announces the Belote")
			assert.Equal(t, search.RaceWin, roots[pair[1]], "the Queen announces the Belote")
		})
	}
}

func TestChooseAnnouncesTheBeloteThatWinsTheRace(t *testing.T) {
	eg, pair := newBeloteRaceEndgame()
	in := search.Information{Seat: 0, Own: eg.hands[0]}
	for seat := 1; seat < 4; seat++ {
		in.Known[seat] = eg.hands[seat]
	}
	pos := search.Position{Trump: hearts, Leader: 0, Race: &eg.race}
	settle := settlementOver(hearts, nil, eg.hands[:]...)
	res, ok := search.Choose(in, pos, legalFor(pos, eg.hands[0]), settle, search.Options{Samples: 4}, farFuture())
	require.True(t, ok)
	assert.Contains(t, []game.Card{search.IdxCard(pair[0]), search.IdxCard(pair[1])}, res.Card)
}

func TestRaceBeloteStopsWhereTheEngineDoes(t *testing.T) {
	rng := rand.New(rand.NewPCG(79, 83))
	var atAnnouncement, atTrick int
	for i := range 150 {
		gs, holder := raceBeloteDeal(rng)
		trump := search.SuitIdx(*gs.TrumpSuit)
		pair := belotePair(trump)
		solver := search.NewSolver(trump, gs.Rules.Clockwise)
		for gs.Phase == game.PhasePlaying {
			seat := gs.ActivePlayerSeat
			legal := game.LegalCards(gs, seat)
			card := legal[rng.IntN(len(legal))]
			idx := search.CardIdx(card)
			hands := handMasks(gs)
			leader := seat
			if len(gs.CurrentTrick) > 0 {
				leader = gs.CurrentTrick[0].PlayerSeat
			}
			race := engineRace(gs, holder, pair)
			solver.SetRace(&race)
			value := solver.RootValues(hands, leader, trickIndexes(gs), search.CapotFlag(gs.TricksWon))[idx]
			announcing := seat == holder && hands[seat]&pair == pair && pair&(1<<uint(idx)) != 0
			closing := len(gs.CurrentTrick) == 3

			next, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: seat, Card: &card})
			require.NoError(t, err)
			gs = next
			if gs.PendingBelotSeat != nil {
				require.True(t, announcing, "deal %d: the engine prompts only on the first of the pair", i)
				gs, err = game.ApplyAction(gs, game.Action{Type: game.ActionAnnounceBelot, PlayerSeat: seat})
				require.NoError(t, err)
			}
			if !gs.StoppedAtTarget {
				continue
			}
			require.Equal(t, (1-2*(*gs.WinnerTeam))*search.RaceWin, value, "deal %d card %v race %+v", i, card, race)
			if closing {
				atTrick++
			} else {
				require.True(t, announcing, "deal %d: a stop mid-trick comes from the Belote", i)
				atAnnouncement++
			}
		}
	}
	assert.Positive(t, atAnnouncement)
	assert.Positive(t, atTrick)
}

// engineRace is the race the bridge builds from an engine state: running totals with every announced point, and the
// holder's Belote while it still holds both cards.
func engineRace(gs *game.GameState, holder int, pair uint32) search.Race {
	race := search.Race{Target: game.MatchTarget(gs.MatchMode), Caller: game.TeamForSeat(*gs.TrumpCallerSeat)}
	for team := range race.Banked {
		race.Banked[team] = gs.TeamScores[team] + gs.HandPoints[team] + gs.DeclarationPoints[team] + gs.BelotPoints[team]
	}
	if search.MaskOf(gs.Players[holder].Hand)&pair == pair {
		race.BeloteSeat, race.BeloteCards = holder, pair
	}
	return race
}

// raceBeloteDeal is a stop-at-target state at trick 5 with declarations on and resolved, random trump, caller,
// direction and leader, and a random holder of the trump King and Queen whose team is 1 to 45 short of the target.
func raceBeloteDeal(rng *rand.Rand) (*game.GameState, int) {
	gs := testfixtures.WithStopAtTarget(testfixtures.NewGameMidPlay(5))
	gs.Rules.DeclarationsEnabled, gs.DeclarationsEnabled = true, true
	gs.Rules.Clockwise = rng.IntN(2) == 1
	gs.Clockwise = gs.Rules.Clockwise
	gs.BelotAnnounced = false
	trump := []game.Suit{game.SuitSpades, game.SuitHearts, game.SuitDiamonds, game.SuitClubs}[rng.IntN(4)]
	gs.TrumpSuit = &trump
	caller := rng.IntN(4)
	gs.TrumpCallerSeat = &caller
	gs.ActivePlayerSeat = rng.IntN(4)
	holder := rng.IntN(4)

	king, queen := game.Card{Rank: game.RankKing, Suit: trump}, game.Card{Rank: game.RankQueen, Suit: trump}
	deck := slices.DeleteFunc(game.NewDeck(), func(c game.Card) bool { return c == king || c == queen })
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	deck = append([]game.Card{king, queen}, deck...)
	for k := range gs.Players {
		gs.Players[(holder+k)%4].Hand = slices.Clone(deck[k*4 : (k+1)*4])
	}

	target := game.MatchTarget(gs.MatchMode)
	short := [2]int{1 + rng.IntN(60), 1 + rng.IntN(60)}
	short[game.TeamForSeat(holder)] = 1 + rng.IntN(45)
	for team := range gs.TeamScores {
		gs.TeamScores[team] = target - short[team] - gs.HandPoints[team]
	}
	return gs, holder
}

func TestHandEndDecided(t *testing.T) {
	race := search.Race{Target: 1001, Banked: [2]int{2000, 2000}, Scores: [2]int{900, 950}, Caller: game.TeamB}
	tests := []struct {
		name        string
		settled     [2]int
		wantWinner  int
		wantDecided bool
	}{
		{"nobody reaches the target", [2]int{100, 50}, 0, false},
		{"team A alone reaches it", [2]int{101, 50}, game.TeamA, true},
		{"team B takes the whole hand", [2]int{0, 162}, game.TeamB, true},
		{"both reach it and B is higher", [2]int{110, 61}, game.TeamB, true},
		{"both reach it equally and B called", [2]int{111, 61}, game.TeamB, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			winner, decided := search.HandEndDecided(race, tt.settled)
			assert.Equal(t, tt.wantDecided, decided)
			if tt.wantDecided {
				assert.Equal(t, tt.wantWinner, winner)
			}
		})
	}
	race.Caller = game.TeamA
	winner, decided := search.HandEndDecided(race, [2]int{111, 61})
	assert.True(t, decided)
	assert.Equal(t, game.TeamA, winner, "a tie goes to the caller")
}

func TestRaceHandEndCrossingMatchesTheEngine(t *testing.T) {
	for _, stopAtTarget := range []bool{true, false} {
		t.Run(fmt.Sprintf("stopAtTarget=%v", stopAtTarget), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(89, 97))
			var crossed, open int
			for i := range 300 {
				gs := lastTrickDeal(t, rng, stopAtTarget)
				seat := gs.ActivePlayerSeat
				hands := handMasks(gs)
				in := search.Information{Seat: seat, Own: hands[seat]}
				for other := range hands {
					if other != seat {
						in.Known[other] = hands[other]
					}
				}
				race := handEndRace(gs, !stopAtTarget)
				pos := search.Position{
					Trump: search.SuitIdx(*gs.TrumpSuit), Clockwise: gs.Rules.Clockwise, Leader: seat,
					Capot: search.CapotFlag(gs.TricksWon), Race: &race,
				}
				settlement, _ := settlementAtTheLastTrick(gs, search.NewSolver(pos.Trump, pos.Clockwise))
				res, ok := search.Choose(in, pos, game.LegalCards(gs, seat), settlement, search.Options{Samples: 1}, farFuture())
				require.True(t, ok)
				value := (1 - 2*game.TeamForSeat(seat)) * int(res.Mean)

				for gs.Phase == game.PhasePlaying {
					next, err := game.ApplyAction(gs, game.Action{
						Type: game.ActionPlayCard, PlayerSeat: gs.ActivePlayerSeat, Card: &game.LegalCards(gs, gs.ActivePlayerSeat)[0],
					})
					require.NoError(t, err)
					gs = next
				}
				require.NotNil(t, gs.LastHandResult, "deal %d", i)
				if gs.Phase == game.PhaseMatchEnd {
					require.Equal(t, (1-2*(*gs.WinnerTeam))*search.RaceWin, value, "deal %d race %+v result %+v", i, race, *gs.LastHandResult)
					crossed++
					continue
				}
				require.Equal(t, gs.LastHandResult.TeamAHandTotal-gs.LastHandResult.TeamBHandTotal, value, "deal %d race %+v", i, race)
				open++
			}
			assert.Positive(t, crossed)
			assert.Positive(t, open)
		})
	}
}

// lastTrickDeal plays a random hand with declarations off up to the last trick, then sets the match scores so that
// each team's running total is 1 to 60 short of the target and only the hand's settlement can reach it.
func lastTrickDeal(t *testing.T, rng *rand.Rand, stopAtTarget bool) *game.GameState {
	t.Helper()
	return endgameDeal(t, rng, stopAtTarget, 8, 60)
}

// endgameDeal plays a random hand with declarations off up to the start of the given trick, then sets the match
// scores so that each team's running total is 1 to maxShort short of the target.
func endgameDeal(t *testing.T, rng *rand.Rand, stopAtTarget bool, trick, maxShort int) *game.GameState {
	t.Helper()
	gs := raceDeal(rng, 0, 0)
	gs.Rules.StopAtTarget, gs.StopAtTarget = stopAtTarget, stopAtTarget
	gs.Rules.Clockwise = rng.IntN(2) == 1
	gs.Clockwise = gs.Rules.Clockwise
	caller := rng.IntN(4)
	gs.TrumpCallerSeat = &caller
	gs.ActivePlayerSeat = rng.IntN(4)
	for gs.TrickNumber < trick || len(gs.CurrentTrick) > 0 {
		seat := gs.ActivePlayerSeat
		legal := game.LegalCards(gs, seat)
		next, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: seat, Card: &legal[rng.IntN(len(legal))]})
		require.NoError(t, err)
		gs = next
	}
	require.Equal(t, game.PhasePlaying, gs.Phase)
	target := game.MatchTarget(gs.MatchMode)
	for team := range gs.TeamScores {
		gs.TeamScores[team] = target - gs.HandPoints[team] - 1 - rng.IntN(maxShort)
	}
	return gs
}

// perfectInformationChoice is the card the race search picks for the seat on turn when it sees every hand, and the
// position's value for team A.
func perfectInformationChoice(t *testing.T, gs *game.GameState, deciding bool) (game.Card, int) {
	t.Helper()
	seat := gs.ActivePlayerSeat
	hands := handMasks(gs)
	in := search.Information{Seat: seat, Own: hands[seat]}
	for other := range hands {
		if other != seat {
			in.Known[other] = hands[other]
		}
	}
	leader := seat
	if len(gs.CurrentTrick) > 0 {
		leader = gs.CurrentTrick[0].PlayerSeat
	}
	race := handEndRace(gs, deciding)
	pos := search.Position{
		Trump: search.SuitIdx(*gs.TrumpSuit), Clockwise: gs.Rules.Clockwise, Leader: leader, Trick: trickIndexes(gs),
		Capot: search.CapotFlag(gs.TricksWon), Race: &race,
	}
	settlement := search.Settlement{
		HandPoints: gs.HandPoints, TricksWon: gs.TricksWon, CallerTeam: game.TeamForSeat(*gs.TrumpCallerSeat),
		DeclarationsTransfer: gs.Rules.DeclarationsTransfer,
	}
	for _, c := range append(search.CardsOf(hands[0]|hands[1]|hands[2]|hands[3]), trickCards(gs)...) {
		settlement.RemainingCardPoints += search.Points(search.CardIdx(c), pos.Trump)
	}
	res, ok := search.Choose(in, pos, game.LegalCards(gs, seat), settlement, search.Options{Samples: 1}, farFuture())
	require.True(t, ok)
	return res.Card, (1 - 2*game.TeamForSeat(seat)) * int(res.Mean)
}

func trickCards(gs *game.GameState) []game.Card {
	out := make([]game.Card, len(gs.CurrentTrick))
	for i, tc := range gs.CurrentTrick {
		out[i] = tc.Card
	}
	return out
}

// crossingKind names what carried a team over the target at the hand's end: a Capot, a fallen contract, the last
// trick's bonus, or the card points alone.
func crossingKind(result game.HandScore, winner, target int, scores [2]int) string {
	totals := [2]int{result.TeamAHandTotal, result.TeamBHandTotal}
	switch {
	case result.Capot:
		return "capot"
	case result.FailedContract:
		return "fall"
	case result.LastTrickTeam == winner && scores[winner]+totals[winner]-result.LastTrickBonus < target:
		return "bonus"
	}
	return "points"
}

func TestRaceCrossingFromTricksSixAndSevenMatchesTheEngine(t *testing.T) {
	for _, stopAtTarget := range []bool{true, false} {
		t.Run(fmt.Sprintf("stopAtTarget=%v", stopAtTarget), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(101, 103))
			kinds := map[string]int{}
			for i := range 300 {
				gs := endgameDeal(t, rng, stopAtTarget, 6+i%2, 90)
				scores, target := gs.TeamScores, game.MatchTarget(gs.MatchMode)
				_, value := perfectInformationChoice(t, gs, !stopAtTarget)
				for gs.Phase == game.PhasePlaying {
					card, step := perfectInformationChoice(t, gs, !stopAtTarget)
					require.Equal(t, value, step, "deal %d: optimal play keeps the root value", i)
					next, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: gs.ActivePlayerSeat, Card: &card})
					require.NoError(t, err)
					gs = next
				}
				if gs.Phase != game.PhaseMatchEnd {
					require.NotNil(t, gs.LastHandResult, "deal %d", i)
					require.Equal(t, gs.LastHandResult.TeamAHandTotal-gs.LastHandResult.TeamBHandTotal, value, "deal %d", i)
					kinds["open"]++
					continue
				}
				require.Equal(t, (1-2*(*gs.WinnerTeam))*search.RaceWin, value, "deal %d", i)
				if gs.TrickNumber < 8 || gs.LastHandResult == nil {
					kinds["mid-hand"]++
					continue
				}
				kinds[crossingKind(*gs.LastHandResult, *gs.WinnerTeam, target, scores)]++
			}
			t.Logf("outcomes %v", kinds)
			assert.Positive(t, kinds["open"])
			assert.Positive(t, kinds["fall"], "no crossing through a fallen contract")
			assert.Positive(t, kinds["bonus"], "no crossing through the last trick's bonus")
			assert.Positive(t, kinds["capot"], "no crossing through a Capot")
			assert.Equal(t, stopAtTarget, kinds["mid-hand"] > 0, "only a stop-at-target match ends mid-hand")
		})
	}
}

// handEndRace is the race the bridge builds from an engine state with declarations off: the running totals, the match
// scores and the caller's team, deciding only at the hand's end when deciding is set.
func handEndRace(gs *game.GameState, deciding bool) search.Race {
	race := search.Race{
		Target: game.MatchTarget(gs.MatchMode), Scores: gs.TeamScores,
		Caller: game.TeamForSeat(*gs.TrumpCallerSeat), DecidingHand: deciding,
	}
	for team := range race.Banked {
		race.Banked[team] = gs.TeamScores[team] + gs.HandPoints[team]
	}
	return race
}

func TestDecidingHandSkipsMidHandChecks(t *testing.T) {
	for _, direction := range directions {
		t.Run(direction.name, func(t *testing.T) {
			eg := newRaceEndgame(990, 500)
			solver := search.NewSolver(hearts, direction.clockwise)
			plain := solver.RootValues(eg.hands, 0, nil, 0)

			solver.SetRace(&eg.race)
			require.Equal(t, search.RaceWin, solver.RootValues(eg.hands, 0, nil, 0)[eg.spadeAce])

			eg.race.DecidingHand = true
			solver.SetRace(&eg.race)
			assert.Equal(t, plain, solver.RootValues(eg.hands, 0, nil, 0))

			belote, _ := newBeloteRaceEndgame()
			belote.race.DecidingHand = true
			solver.SetRace(&belote.race)
			roots := solver.RootValues(belote.hands, 0, nil, 0)
			solver.SetRace(nil)
			assert.Equal(t, solver.RootValues(belote.hands, 0, nil, 0), roots)
		})
	}
}
