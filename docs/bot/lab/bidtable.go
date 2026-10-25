package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/emilijan/beljot/server/internal/botlab"
	"github.com/emilijan/beljot/server/internal/game"
)

type bidRow struct {
	call, pass stat
	picks      int
}

func bidKey(hand []game.Card, suit game.Suit) string {
	count := 0
	var marks []string
	side := 0
	for _, r := range []game.Rank{game.RankJack, game.Rank9, game.RankAce, game.RankTen, game.RankKing, game.RankQueen, game.Rank8, game.Rank7} {
		for _, c := range hand {
			if c.Suit == suit && c.Rank == r {
				count++
				marks = append(marks, string(r))
			}
		}
	}
	for _, c := range hand {
		if c.Suit != suit && c.Rank == game.RankAce {
			side++
		}
	}
	return fmt.Sprintf("n%d %-6s side%d", count, strings.Join(marks, ""), side)
}

// bidtable forces the first bidder to call (each suit under Croatian rules; the candidate
// suit under Bitola round 1) and compares with letting the bidding run its course.
func bidtable(v game.Variant) {
	t0 := time.Now()
	deals := make([]botlab.Deal, *n)
	rng := rand.New(rand.NewPCG(*seed, 9))
	for i := range deals {
		deals[i] = botlab.RandomDeal(rng)
	}
	rows := make([]map[string]*bidRow, *workers)
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
			A := makePolicy(*polA, rand.New(rand.NewPCG(*seed, uint64(400+w))), *flagsA)
			pol := [4]botlab.Policy{A, A, A, A}
			mine := map[string]*bidRow{}
			rows[w] = mine
			for i := range next {
				base := botlab.NewDealState(deals[i], v)
				base.Phase = game.PhaseBidding
				game.RefreshDerivedFlags(base)
				seat := base.ActivePlayerSeat
				team := seat % 2
				view := botlab.BuildView(base, seat, botlab.NewMemory())
				natural := A.Decide(view)
				passed, err := game.ApplyAction(base, game.Action{Type: game.ActionPassTrump, PlayerSeat: seat})
				if err != nil {
					continue
				}
				passOut, err := botlab.PlayHand(passed, pol, nil)
				if err != nil {
					fmt.Println("error:", err)
					os.Exit(1)
				}
				pa, pb := botlab.Award(passOut)
				passDiff := float64(pa - pb)
				if team == 1 {
					passDiff = -passDiff
				}
				var suits []game.Suit
				hand := view.Hand
				if base.TrumpCandidate != nil {
					suits = []game.Suit{base.TrumpCandidate.Suit}
					hand = append(append([]game.Card{}, hand...), *base.TrumpCandidate)
				} else {
					suits = game.AllSuits[:]
				}
				for _, s := range suits {
					key := bidKey(hand, s)
					act := game.Action{Type: game.ActionPickTrump, PlayerSeat: seat}
					if base.TrumpCandidate == nil {
						ss := s
						act.Suit = &ss
					}
					forced, err := game.ApplyAction(base, act)
					if err != nil {
						fmt.Println("error:", err)
						os.Exit(1)
					}
					out, err := botlab.PlayHand(forced, pol, nil)
					if err != nil {
						fmt.Println("error:", err)
						os.Exit(1)
					}
					if out.Status != botlab.HandScored {
						continue
					}
					a, b := botlab.Award(out)
					diff := float64(a - b)
					if team == 1 {
						diff = -diff
					}
					r := mine[key]
					if r == nil {
						r = &bidRow{}
						mine[key] = r
					}
					r.call.add(diff)
					r.pass.add(passDiff)
					if natural.Type == game.ActionPickTrump && (natural.Suit == nil || *natural.Suit == s) {
						r.picks++
					}
				}
			}
		}(w)
	}
	wg.Wait()
	tot := map[string]*bidRow{}
	for _, m := range rows {
		for k, r := range m {
			if tot[k] == nil {
				tot[k] = &bidRow{}
			}
			tot[k].call.merge(&r.call)
			tot[k].pass.merge(&r.pass)
			tot[k].picks += r.picks
		}
	}
	keys := make([]string, 0, len(tot))
	for k := range tot {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("bid table, first bidder, %s, %d deals, rollouts by %s (%.0fs)\n", v, *n, *polA, time.Since(t0).Seconds())
	fmt.Printf("%-22s %6s %9s %9s %9s %7s\n", "hand (suit cards)", "n", "EV(call)", "EV(pass)", "gain", "bot%")
	var missedGain, wrongCalls, totalN float64
	for _, k := range keys {
		r := tot[k]
		if r.call.n < 25 {
			continue
		}
		gain := r.call.mean() - r.pass.mean()
		botPct := 100 * float64(r.picks) / float64(r.call.n)
		fmt.Printf("%-22s %6d %+9.1f %+9.1f %+9.1f %6.0f%%\n", k, r.call.n, r.call.mean(), r.pass.mean(), gain, botPct)
		totalN += float64(r.call.n)
		if gain > 0 {
			missedGain += gain * float64(r.call.n-r.picks)
		} else {
			wrongCalls += -gain * float64(r.picks)
		}
	}
	fmt.Printf("\nmissed gain from passing profitable hands: %.1f pts per deal; cost of unprofitable calls: %.1f pts per deal\n",
		missedGain/totalN, wrongCalls/totalN)
}
