package main

import (
	"bufio"
	"fmt"
	"math/rand/v2"
	"os"
	"sync"
	"time"

	"github.com/emilijan/beljot/server/internal/botlab"
	"github.com/emilijan/beljot/server/internal/game"
)

// bidpos walks the natural bidding of each deal and, at every decision point, measures the
// forced-call and forced-pass expectations by rollout. Rows go to a CSV for analysis.
func bidpos(v game.Variant, csvPath string) {
	t0 := time.Now()
	deals := make([]botlab.Deal, *n)
	rng := rand.New(rand.NewPCG(*seed, 10))
	for i := range deals {
		deals[i] = botlab.RandomDeal(rng)
	}
	f, err := os.Create(csvPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	wr := bufio.NewWriter(f)
	defer wr.Flush()
	var mu sync.Mutex
	fmt.Fprintln(wr, "variant,round,passesBefore,suit,count,J,9,A,T,K,Q,8,7,sideA,sideT,sideK,sideCount,natural,evCall,evPass")
	var wg sync.WaitGroup
	next := make(chan int)
	go func() {
		for i := 0; i < *n; i++ {
			next <- i
		}
		close(next)
	}()
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			A := makePolicy(*polA, rand.New(rand.NewPCG(*seed, uint64(500+w))), *flagsA)
			pol := [4]botlab.Policy{A, A, A, A}
			var lines []string
			for i := range next {
				gs := botlab.NewDealState(deals[i], v)
				gs.Phase = game.PhaseBidding
				game.RefreshDerivedFlags(gs)
				for step := 0; step < 8; step++ {
					seat := gs.ActivePlayerSeat
					team := seat % 2
					view := botlab.BuildView(gs, seat, botlab.NewMemory())
					natural := A.Decide(view)
					hand := view.Hand
					var suits []game.Suit
					switch {
					case gs.TrumpCandidate != nil && gs.BiddingRound == 1:
						suits = []game.Suit{gs.TrumpCandidate.Suit}
						hand = append(append([]game.Card{}, hand...), *gs.TrumpCandidate)
					case gs.TrumpCandidate != nil:
						hand = append(append([]game.Card{}, hand...), *gs.TrumpCandidate)
						for _, s := range game.AllSuits {
							if s != gs.TrumpCandidate.Suit {
								suits = append(suits, s)
							}
						}
					default:
						suits = game.AllSuits[:]
					}
					evPass := 0.0
					canPass := !game.MustPickTrump(gs, seat)
					if canPass {
						passed, err := game.ApplyAction(gs, game.Action{Type: game.ActionPassTrump, PlayerSeat: seat})
						if err != nil {
							panic(err)
						}
						out, err := botlab.PlayHand(passed, pol, nil)
						if err != nil {
							panic(err)
						}
						a, b := botlab.Award(out)
						evPass = float64(a - b)
						if team == 1 {
							evPass = -evPass
						}
					}
					for _, s := range suits {
						act := game.Action{Type: game.ActionPickTrump, PlayerSeat: seat}
						if !(gs.TrumpCandidate != nil && gs.BiddingRound == 1) {
							ss := s
							act.Suit = &ss
						}
						forced, err := game.ApplyAction(gs, act)
						if err != nil {
							panic(err)
						}
						out, err := botlab.PlayHand(forced, pol, nil)
						if err != nil {
							panic(err)
						}
						if out.Status != botlab.HandScored {
							continue
						}
						a, b := botlab.Award(out)
						evCall := float64(a - b)
						if team == 1 {
							evCall = -evCall
						}
						nat := 0
						if natural.Type == game.ActionPickTrump && (natural.Suit == nil || *natural.Suit == s) {
							nat = 1
						}
						lines = append(lines, featureLine(v, gs.BiddingRound, gs.BiddingPassCount, s, hand, nat, evCall, evPass))
					}
					if natural.Type == game.ActionPickTrump || !canPass {
						break
					}
					if gs.Rules.AllPassOutcome == game.AllPassReshuffleAndRotate && gs.BiddingRound == 2 && gs.BiddingPassCount == 3 {
						break
					}
					nextState, err := game.ApplyAction(gs, natural)
					if err != nil {
						panic(err)
					}
					gs = nextState
				}
				if len(lines) > 2000 {
					mu.Lock()
					for _, l := range lines {
						fmt.Fprintln(wr, l)
					}
					mu.Unlock()
					lines = lines[:0]
				}
			}
			mu.Lock()
			for _, l := range lines {
				fmt.Fprintln(wr, l)
			}
			mu.Unlock()
		}(w)
	}
	wg.Wait()
	fmt.Printf("bidpos %s: %d deals in %.0fs -> %s\n", v, *n, time.Since(t0).Seconds(), csvPath)
}

func featureLine(v game.Variant, round, passes int, suit game.Suit, hand []game.Card, nat int, evCall, evPass float64) string {
	has := map[game.Rank]int{}
	count := 0
	sideA, sideT, sideK, sideCount := 0, 0, 0, 0
	for _, c := range hand {
		if c.Suit == suit {
			count++
			has[c.Rank] = 1
			continue
		}
		sideCount++
		switch c.Rank {
		case game.RankAce:
			sideA++
		case game.RankTen:
			sideT++
		case game.RankKing:
			sideK++
		}
	}
	return fmt.Sprintf("%s,%d,%d,%s,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%.0f,%.0f",
		v, round, passes, suit, count, has[game.RankJack], has[game.Rank9], has[game.RankAce], has[game.RankTen],
		has[game.RankKing], has[game.RankQueen], has[game.Rank8], has[game.Rank7], sideA, sideT, sideK, sideCount, nat, evCall, evPass)
}
