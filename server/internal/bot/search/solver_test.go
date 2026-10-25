package search_test

import (
	"cmp"
	"fmt"
	"math"
	"math/bits"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/bot/search"
	"github.com/emilijan/beljot/server/internal/game"
)

type bruteForce struct {
	solver    *search.Solver
	trump     int
	clockwise bool
}

func (b bruteForce) value(hands [4]uint32, leader int, trick []int, won [2]int) int {
	if len(trick) == 0 && hands == [4]uint32{} {
		return 0
	}
	seat := game.StepSeat(leader, len(trick), b.clockwise)
	maximize := seat%2 == 0
	best := math.MaxInt32
	if maximize {
		best = math.MinInt32
	}
	for m := b.solver.Legal(hands[seat], trick); m != 0; m &= m - 1 {
		v := b.afterCard(hands, leader, trick, bits.TrailingZeros32(m), won)
		if maximize && v > best || !maximize && v < best {
			best = v
		}
	}
	return best
}

func (b bruteForce) afterCard(hands [4]uint32, leader int, trick []int, card int, won [2]int) int {
	seat := game.StepSeat(leader, len(trick), b.clockwise)
	hands[seat] &^= 1 << uint(card)
	played := append(slices.Clone(trick), card)
	if len(played) < 4 {
		return b.value(hands, leader, played, won)
	}
	winnerSeat := game.StepSeat(leader, b.solver.Winner(played), b.clockwise)
	points := 0
	for _, c := range played {
		points += search.Points(c, b.trump)
	}
	team := game.TeamForSeat(winnerSeat)
	sign := 1 - 2*team
	won[team]++
	if hands == [4]uint32{} {
		switch {
		case won[1] == 0:
			return points + 100
		case won[0] == 0:
			return -points - 100
		}
		return sign * (points + 10)
	}
	return sign*points + b.value(hands, winnerSeat, nil, won)
}

func tricksWonFromFlag(flag int8) [2]int {
	switch flag {
	case 1:
		return [2]int{1, 0}
	case 2:
		return [2]int{0, 1}
	case 3:
		return [2]int{1, 1}
	}
	return [2]int{}
}

type endgame struct {
	hands  [4]uint32
	leader int
	trick  []int
	capot  int8
}

func roughlyStrongestFirst(rng *rand.Rand) []game.Card {
	deck := game.NewDeck()
	strength := make(map[game.Card]float64, len(deck))
	for _, c := range deck {
		strength[c] = float64(game.NonTrumpRankOrder[c.Rank]) + 4*rng.Float64()
	}
	slices.SortFunc(deck, func(a, b game.Card) int { return cmp.Compare(strength[b], strength[a]) })
	return deck
}

func dealOrderedDeck(rng *rand.Rand) ([]game.Card, [4]int) {
	switch rng.IntN(3) {
	case 0:
		return roughlyStrongestFirst(rng), [4]int{0, 2, 1, 3}
	case 1:
		return roughlyStrongestFirst(rng), [4]int{1, 3, 0, 2}
	}
	deck := game.NewDeck()
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	return deck, [4]int{0, 1, 2, 3}
}

func randomEndgame(rng *rand.Rand, solver *search.Solver, clockwise bool) endgame {
	deck, seatOrder := dealOrderedDeck(rng)
	cardsEach := 3 + rng.IntN(3)
	eg := endgame{leader: rng.IntN(4), capot: int8(rng.IntN(4))}
	for i, seat := range seatOrder {
		eg.hands[seat] = search.MaskOf(deck[i*cardsEach : (i+1)*cardsEach])
	}
	for played := rng.IntN(4); played > 0; played-- {
		seat := game.StepSeat(eg.leader, len(eg.trick), clockwise)
		legal := search.CardsOf(solver.Legal(eg.hands[seat], eg.trick))
		card := search.CardIdx(legal[rng.IntN(len(legal))])
		eg.hands[seat] &^= 1 << uint(card)
		eg.trick = append(eg.trick, card)
	}
	return eg
}

func solversByTrump(clockwise bool) [4]*search.Solver {
	var solvers [4]*search.Solver
	for trump := range solvers {
		solvers[trump] = search.NewSolver(trump, clockwise)
	}
	return solvers
}

var directions = []struct {
	name      string
	clockwise bool
}{
	{"counter-clockwise", false},
	{"clockwise", true},
}

func TestSolverMatchesBruteForce(t *testing.T) {
	const endgamesPerDirection = 150
	for _, direction := range directions {
		t.Run(direction.name, func(t *testing.T) {
			rng := rand.New(rand.NewPCG(7, 11))
			solvers := solversByTrump(direction.clockwise)
			for i := 0; i < endgamesPerDirection; i++ {
				trump := rng.IntN(4)
				solver := solvers[trump]
				eg := randomEndgame(rng, solver, direction.clockwise)
				reference := bruteForce{solver: solver, trump: trump, clockwise: direction.clockwise}
				want := reference.value(eg.hands, eg.leader, eg.trick, tricksWonFromFlag(eg.capot))
				got := solver.Value(eg.hands, eg.leader, eg.trick, eg.capot)
				require.Equal(t, want, got, "endgame %d trump %d leader %d trick %v capot %d hands %v",
					i, trump, eg.leader, eg.trick, eg.capot, eg.hands)
			}
		})
	}
}

func TestSolverRootValuesAreTheValueOfEachLegalCard(t *testing.T) {
	const endgamesPerDirection = 100
	for _, direction := range directions {
		t.Run(direction.name, func(t *testing.T) {
			rng := rand.New(rand.NewPCG(13, 17))
			solvers := solversByTrump(direction.clockwise)
			for i := 0; i < endgamesPerDirection; i++ {
				trump := rng.IntN(4)
				solver := solvers[trump]
				eg := randomEndgame(rng, solver, direction.clockwise)
				reference := bruteForce{solver: solver, trump: trump, clockwise: direction.clockwise}
				seat := game.StepSeat(eg.leader, len(eg.trick), direction.clockwise)
				legal := solver.Legal(eg.hands[seat], eg.trick)

				roots := solver.RootValues(eg.hands, eg.leader, eg.trick, eg.capot)

				require.Len(t, roots, bits.OnesCount32(legal), "endgame %d", i)
				best := math.MinInt32
				if seat%2 == 1 {
					best = math.MaxInt32
				}
				for card, got := range roots {
					require.NotZero(t, legal&(1<<uint(card)), "endgame %d card %d is not legal", i, card)
					want := reference.afterCard(eg.hands, eg.leader, eg.trick, card, tricksWonFromFlag(eg.capot))
					require.Equal(t, want, got, "endgame %d card %d", i, card)
					if seat%2 == 0 && got > best || seat%2 == 1 && got < best {
						best = got
					}
				}
				require.Equal(t, solver.Value(eg.hands, eg.leader, eg.trick, eg.capot), best, "endgame %d", i)
			}
		})
	}
}

func TestSolverValueLastTrickAndCapotBonus(t *testing.T) {
	heart := func(rank game.Rank) int { return search.CardIdx(game.Card{Suit: game.SuitHearts, Rank: rank}) }
	handOf := func(cards ...int) [4]uint32 {
		var hands [4]uint32
		for seat, card := range cards {
			hands[seat] = 1 << uint(card)
		}
		return hands
	}
	teamAWinsLast := handOf(heart(game.RankAce), heart(game.Rank7), heart(game.RankKing), heart(game.Rank8))
	teamBWinsLast := handOf(heart(game.Rank7), heart(game.RankAce), heart(game.Rank8), heart(game.RankKing))
	const trickPoints = 15

	tests := []struct {
		name  string
		hands [4]uint32
		capot int8
		want  int
	}{
		{"A wins the last trick having won none before", teamAWinsLast, 0, trickPoints + 100},
		{"A wins the last trick having won all before", teamAWinsLast, 1, trickPoints + 100},
		{"A wins the last trick after B won all before", teamAWinsLast, 2, trickPoints + 10},
		{"A wins the last trick in a mixed hand", teamAWinsLast, 3, trickPoints + 10},
		{"B wins the last trick having won none before", teamBWinsLast, 0, -trickPoints - 100},
		{"B wins the last trick after A won all before", teamBWinsLast, 1, -trickPoints - 10},
		{"B wins the last trick having won all before", teamBWinsLast, 2, -trickPoints - 100},
		{"B wins the last trick in a mixed hand", teamBWinsLast, 3, -trickPoints - 10},
	}
	for _, direction := range directions {
		for _, tt := range tests {
			t.Run(direction.name+"/"+tt.name, func(t *testing.T) {
				solver := search.NewSolver(0, direction.clockwise)
				assert.Equal(t, tt.want, solver.Value(tt.hands, 0, nil, tt.capot))
			})
		}
	}
}

func TestCapotFlag(t *testing.T) {
	tests := []struct {
		tricksWon [2]int
		want      int8
	}{
		{[2]int{0, 0}, 0},
		{[2]int{3, 0}, 1},
		{[2]int{0, 7}, 2},
		{[2]int{1, 1}, 3},
		{[2]int{5, 2}, 3},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, search.CapotFlag(tt.tricksWon), "tricks won %v", tt.tricksWon)
	}
}

func TestCardIndexRoundTrip(t *testing.T) {
	deck := game.NewDeck()
	seen := map[int]bool{}
	for _, c := range deck {
		idx := search.CardIdx(c)
		require.Equal(t, c, search.IdxCard(idx))
		seen[idx] = true
	}
	require.Len(t, seen, search.NCards)
	assert.Equal(t, uint32(math.MaxUint32), search.MaskOf(deck))
	assert.ElementsMatch(t, deck, search.CardsOf(search.MaskOf(deck)))

	for suit, s := range game.AllSuits {
		assert.Equal(t, suit, search.SuitIdx(s))
		var suitCards []game.Card
		for _, c := range deck {
			if c.Suit == s {
				suitCards = append(suitCards, c)
			}
		}
		assert.Equal(t, search.MaskOf(suitCards), search.SuitMask(suit))
	}
}

func TestPointsMatchEngineTables(t *testing.T) {
	for suit, s := range game.AllSuits {
		for _, rank := range game.AllRanks {
			idx := search.CardIdx(game.Card{Suit: s, Rank: rank})
			for trump := 0; trump < 4; trump++ {
				want := game.NonTrumpCardPoints[rank]
				if trump == suit {
					want = game.TrumpCardPoints[rank]
				}
				assert.Equal(t, want, search.Points(idx, trump), "card %s%s trump %d", rank, s, trump)
			}
		}
	}
}

type auditStats struct {
	tricks           int
	decisions        int
	restricted       int
	forcedCuts       int
	lastTrickChecks  int
	handsByDirection map[bool]int
	handsByVariant   map[game.Variant]int
}

func TestSolverMatchesEngineLegalityAndWinner(t *testing.T) {
	const handsTarget = 200
	rng := rand.New(rand.NewPCG(2026, 10))
	stats := auditStats{handsByDirection: map[bool]int{}, handsByVariant: map[game.Variant]int{}}

	completed := 0
	for attempts := 0; completed < handsTarget; attempts++ {
		require.Less(t, attempts, 2*handsTarget, "too many deals ended before a card was played")
		variant := game.VariantBitola
		if completed%2 == 1 {
			variant = game.VariantCroatia
		}
		clockwise := completed/2%2 == 1
		gs := game.NewGame(
			[4]uint{}, [4]string{}, [4]bool{true, true, true, true},
			variant, "1001", 1, true, false, false, clockwise,
		)
		if auditHand(t, gs, rng, &stats) {
			completed++
			stats.handsByDirection[clockwise]++
			stats.handsByVariant[variant]++
		}
	}

	assert.Equal(t, handsTarget*8, stats.tricks)
	assert.Equal(t, handsTarget, stats.lastTrickChecks)
	assert.Equal(t, handsTarget/2, stats.handsByDirection[true])
	assert.Equal(t, handsTarget/2, stats.handsByVariant[game.VariantCroatia])
	assert.Positive(t, stats.restricted, "no decision ever had a restricted legal set")
	assert.Positive(t, stats.forcedCuts, "no decision ever forced a cut")
	t.Logf("%d hands, %d tricks, %d card decisions (%d restricted, %d forced cuts)",
		completed, stats.tricks, stats.decisions, stats.restricted, stats.forcedCuts)
}

func describe(gs *game.GameState, seat int) string {
	return fmt.Sprintf("seat %d hand %v trick %v trump %v trick number %d clockwise %v",
		seat, gs.Players[seat].Hand, gs.CurrentTrick, *gs.TrumpSuit, gs.TrickNumber, gs.Clockwise)
}

func handMasks(gs *game.GameState) [4]uint32 {
	var hands [4]uint32
	for seat := range gs.Players {
		hands[seat] = search.MaskOf(gs.Players[seat].Hand)
	}
	return hands
}

func auditHand(t *testing.T, gs *game.GameState, rng *rand.Rand, stats *auditStats) bool {
	t.Helper()

	var solver *search.Solver
	var open []int
	var openLeader int
	var lastTrickValue int

	for steps := 0; steps < 500; steps++ {
		switch gs.Phase {
		case game.PhaseDealing:
			gs.Phase = game.PhaseBidding
			continue
		case game.PhaseBidding, game.PhaseDeclaring, game.PhasePlaying:
		default:
			return gs.LastHandResult != nil
		}

		seat, action := randomAction(t, gs, rng)

		if action.Type == game.ActionPlayCard {
			if solver == nil {
				solver = search.NewSolver(search.SuitIdx(*gs.TrumpSuit), gs.Clockwise)
			}
			trick := make([]int, len(gs.CurrentTrick))
			for i, tc := range gs.CurrentTrick {
				trick[i] = search.CardIdx(tc.Card)
			}
			require.True(t, slices.Equal(trick, open),
				"the audit lost track of the open trick %v: %s", open, describe(gs, seat))
			if len(open) == 0 {
				openLeader = seat
			}
			require.Equal(t, game.StepSeat(openLeader, len(open), gs.Clockwise), seat,
				"the engine skipped a seat: %s", describe(gs, seat))

			hands := handMasks(gs)
			require.Equal(t, 9-gs.TrickNumber, bits.OnesCount32(hands[seat]),
				"the hand mask is incomplete: %s", describe(gs, seat))
			legal := solver.Legal(hands[seat], trick)
			require.Equal(t, search.MaskOf(game.LegalCards(gs, seat)), legal,
				"legality differs: %s", describe(gs, seat))
			stats.decisions++
			if legal != hands[seat] {
				stats.restricted++
			}
			if len(trick) > 0 && hands[seat]&search.SuitMask(trick[0]/8) == 0 &&
				hands[seat]&search.SuitMask(search.SuitIdx(*gs.TrumpSuit)) != 0 {
				stats.forcedCuts++
			}
			if gs.TrickNumber == 8 && len(open) == 0 {
				lastTrickValue = solver.Value(hands, seat, nil, search.CapotFlag(gs.TricksWon))
			}
			open = append(open, search.CardIdx(*action.Card))
		}

		next, err := game.ApplyAction(gs, action)
		require.NoError(t, err, "ApplyAction rejected a legal action (seat %d, %s)", seat, action.Type)

		if len(open) == 4 && len(next.CurrentTrick) == 0 {
			assertTrickMatchesEngine(t, gs, next, solver, open, openLeader, lastTrickValue)
			stats.tricks++
			if next.TrickNumber == gs.TrickNumber {
				stats.lastTrickChecks++
			}
			open = open[:0]
		}
		gs = next
	}
	t.Fatal("the hand did not terminate within the step bound")
	return false
}

func assertTrickMatchesEngine(
	t *testing.T, before, after *game.GameState, solver *search.Solver, trick []int, leader, lastTrickValue int,
) {
	t.Helper()

	winnerSeat := game.StepSeat(leader, solver.Winner(trick), before.Clockwise)
	points := 0
	for _, c := range trick {
		points += search.Points(c, search.SuitIdx(*before.TrumpSuit))
	}
	var want [2]int
	want[game.TeamForSeat(winnerSeat)] = points

	if after.TrickNumber != before.TrickNumber {
		require.Equal(t, winnerSeat, after.ActivePlayerSeat, "trick %v led by seat %d", trick, leader)
		got := [2]int{after.HandPoints[0] - before.HandPoints[0], after.HandPoints[1] - before.HandPoints[1]}
		require.Equal(t, want, got, "trick %v led by seat %d", trick, leader)
		return
	}

	result := after.LastHandResult
	require.NotNil(t, result)
	require.Equal(t, winnerSeat, result.LastTrickSeat, "last trick %v led by seat %d", trick, leader)
	got := [2]int{result.TeamACardPoints - before.HandPoints[0], result.TeamBCardPoints - before.HandPoints[1]}
	require.Equal(t, want, got, "last trick %v led by seat %d", trick, leader)

	engineValue := got[0] - got[1]
	bonus, bonusTeam := result.LastTrickBonus, result.LastTrickTeam
	if result.Capot {
		bonus, bonusTeam = result.CapotBonus, *result.CapotTeam
	}
	if bonusTeam == game.TeamA {
		engineValue += bonus
	} else {
		engineValue -= bonus
	}
	require.Equal(t, engineValue, lastTrickValue, "the solver's value of the last trick %v", trick)
}

func randomAction(t *testing.T, gs *game.GameState, rng *rand.Rand) (int, game.Action) {
	t.Helper()

	switch gs.Phase {
	case game.PhaseBidding:
		return gs.ActivePlayerSeat, randomBid(gs, gs.ActivePlayerSeat, rng)
	case game.PhaseDeclaring:
		for seat := range gs.Players {
			if !gs.Players[seat].DeclarationAnswered {
				return seat, randomDeclarationAnswer(gs, seat, rng)
			}
		}
		require.FailNow(t, "the declaration phase is open with every seat answered")
	}

	seat := gs.ActivePlayerSeat
	if gs.PendingBelotSeat != nil {
		seat = *gs.PendingBelotSeat
		if rng.IntN(2) == 0 {
			return seat, game.Action{Type: game.ActionAnnounceBelot, PlayerSeat: seat}
		}
		return seat, game.Action{Type: game.ActionSkipBelot, PlayerSeat: seat}
	}
	if gs.AwaitingDeclaration {
		return seat, randomDeclarationAnswer(gs, seat, rng)
	}
	legal := game.LegalCards(gs, seat)
	card := legal[rng.IntN(len(legal))]
	return seat, game.Action{Type: game.ActionPlayCard, PlayerSeat: seat, Card: &card}
}

func randomDeclarationAnswer(gs *game.GameState, seat int, rng *rand.Rand) game.Action {
	if game.HasDeclarableCombinations(gs, seat) && rng.IntN(2) == 0 {
		return game.Action{Type: game.ActionDeclare, PlayerSeat: seat}
	}
	return game.Action{Type: game.ActionSkipDeclare, PlayerSeat: seat}
}

func randomBid(gs *game.GameState, seat int, rng *rand.Rand) game.Action {
	if gs.Rules.HasTrumpCandidate && gs.BiddingRound == 1 {
		if rng.IntN(2) == 0 {
			return game.Action{Type: game.ActionPickTrump, PlayerSeat: seat}
		}
		return game.Action{Type: game.ActionPassTrump, PlayerSeat: seat}
	}
	var options []game.Suit
	for _, s := range game.AllSuits {
		if gs.TrumpCandidate == nil || s != gs.TrumpCandidate.Suit {
			options = append(options, s)
		}
	}
	pick := rng.IntN(len(options) + 1)
	if game.MustPickTrump(gs, seat) {
		pick = rng.IntN(len(options))
	}
	if pick == len(options) {
		return game.Action{Type: game.ActionPassTrump, PlayerSeat: seat}
	}
	suit := options[pick]
	return game.Action{Type: game.ActionPickTrump, PlayerSeat: seat, Suit: &suit}
}
