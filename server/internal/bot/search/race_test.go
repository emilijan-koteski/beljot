package search_test

import (
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

func (b raceBruteForce) value(hands [4]uint32, leader int, trick []int, won, gained [2]int) int {
	seat := game.StepSeat(leader, len(trick), b.clockwise)
	maximize := seat%2 == 0
	best, first := 0, true
	for _, card := range search.CardsOf(b.solver.Legal(hands[seat], trick)) {
		v := b.afterCard(hands, leader, trick, search.CardIdx(card), won, gained)
		if first || maximize && v > best || !maximize && v < best {
			best, first = v, false
		}
	}
	return best
}

func (b raceBruteForce) afterCard(hands [4]uint32, leader int, trick []int, card int, won, gained [2]int) int {
	seat := game.StepSeat(leader, len(trick), b.clockwise)
	rest := hands
	rest[seat] &^= 1 << uint(card)
	played := append(slices.Clone(trick), card)
	if len(played) < 4 {
		return b.value(rest, leader, played, won, gained)
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
	if winner, decided := search.RaceDecided(b.race, gained); decided {
		return (1 - 2*winner) * search.RaceWin
	}
	return b.value(rest, winnerSeat, nil, won, gained)
}

func TestSolverRaceMatchesBruteForce(t *testing.T) {
	const endgamesPerDirection = 150
	for _, direction := range directions {
		t.Run(direction.name, func(t *testing.T) {
			rng := rand.New(rand.NewPCG(41, 43))
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
					want := reference.afterCard(eg.hands, eg.leader, eg.trick, idx, won, [2]int{})
					require.Equal(t, want, roots[idx], "endgame %d card %d race %+v", i, idx, race)
				}
				want := reference.value(eg.hands, eg.leader, eg.trick, won, [2]int{})
				require.Equal(t, want, solver.Value(eg.hands, eg.leader, eg.trick, eg.capot), "endgame %d", i)
				solver.SetRace(nil)
			}
		})
	}
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
