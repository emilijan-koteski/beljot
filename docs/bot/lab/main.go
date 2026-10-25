package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emilijan/beljot/server/internal/botlab"
	"github.com/emilijan/beljot/server/internal/game"
)

var (
	exp     = flag.String("exp", "bench", "bench|ddcheck|duplicate|regret")
	polA    = flag.String("a", "current", "policy A")
	polB    = flag.String("b", "random", "policy B")
	n       = flag.Int("n", 200, "deals")
	seed    = flag.Uint64("seed", 1, "seed")
	variant = flag.String("variant", "bitola", "bitola|croatia")
	flagsA  = flag.String("flagsA", "", "lab flags for A (comma list)")
	flagsB  = flag.String("flagsB", "", "lab flags for B (comma list)")
	workers = flag.Int("workers", runtime.NumCPU(), "parallel workers")
	csvOut  = flag.String("csv", "bidpos.csv", "csv output for bidpos")
)

func makePolicy(name string, rng *rand.Rand, labFlags string) botlab.Policy {
	switch {
	case name == "current":
		return botlab.CurrentBot{}
	case name == "random":
		return botlab.RandomBot{Rng: rng}
	case name == "cheat":
		return botlab.CheatBot{Fallback: botlab.LabBot{Flags: botlab.ParseFlags(labFlags)}}
	case name == "lab":
		return botlab.LabBot{Flags: botlab.ParseFlags(labFlags)}
	case strings.HasPrefix(name, "pimcs:"):
		var sched []int
		for _, part := range strings.Split(strings.TrimPrefix(name, "pimcs:"), ",") {
			k, _ := strconv.Atoi(part)
			sched = append(sched, k)
		}
		return &botlab.PIMC{Samples: sched[0], Schedule: sched, Rng: rng, Fallback: botlab.LabBot{Flags: botlab.ParseFlags(labFlags)}, Label: name + "[" + labFlags + "]"}
	case strings.HasPrefix(name, "pimc:"):
		parts := strings.Split(strings.TrimPrefix(name, "pimc:"), ":")
		k, _ := strconv.Atoi(parts[0])
		from := 1
		if len(parts) > 1 {
			from, _ = strconv.Atoi(parts[1])
		}
		return &botlab.PIMC{Samples: k, FromTrick: from, Rng: rng, Fallback: botlab.LabBot{Flags: botlab.ParseFlags(labFlags)}, Label: name + "[" + labFlags + "]"}
	}
	panic("unknown policy " + name)
}

func main() {
	flag.Parse()
	v := game.VariantBitola
	if *variant == "croatia" {
		v = game.VariantCroatia
	}
	switch *exp {
	case "bench":
		bench()
	case "ddcheck":
		ddcheck()
	case "ddcheck2":
		ddcheck2()
	case "duplicate":
		duplicate(v)
	case "regret":
		regret(v)
	case "bidtable":
		bidtable(v)
	case "bidpos":
		bidpos(v, *csvOut)
	default:
		fmt.Println("unknown exp")
		os.Exit(1)
	}
}

func bench() {
	rng := rand.New(rand.NewPCG(*seed, 2))
	for trickStart := 1; trickStart <= 7; trickStart++ {
		var totalMs float64
		var totalNodes int64
		runs := 20
		for r := 0; r < runs; r++ {
			deal := botlab.RandomDeal(rng)
			var hands [4]uint32
			for s := 0; s < 4; s++ {
				hands[s] = botlab.MaskOf(deal.Deck[s*8 : s*8+8])
			}
			trump := rng.IntN(4)
			d := botlab.NewDD(trump)
			leader := 0
			capot := int8(0)
			for t := 1; t < trickStart; t++ {
				trick := []int{}
				for k := 0; k < 4; k++ {
					seat := (leader + k) % 4
					legal := d.Legal(hands[seat], trick)
					cs := botlab.CardsOf(legal)
					c := botlab.CardIdx(cs[rng.IntN(len(cs))])
					hands[seat] &^= 1 << uint(c)
					trick = append(trick, c)
				}
				w := (leader + d.Winner(trick)) % 4
				switch {
				case capot == 0:
					capot = int8(1 + w%2)
				case capot == 1 && w%2 == 1, capot == 2 && w%2 == 0:
					capot = 3
				}
				leader = w
			}
			t0 := time.Now()
			_ = d.Value(hands, leader, nil, capot)
			totalMs += float64(time.Since(t0).Microseconds()) / 1000
			totalNodes += d.Nodes
		}
		fmt.Printf("from trick %d: avg %.1f ms, avg nodes %d\n", trickStart, totalMs/float64(runs), totalNodes/int64(runs))
	}
}

func naive(d *botlab.DD, hands [4]uint32, leader int, trick []int, capot int8) int {
	if len(trick) == 0 && hands[0]|hands[1]|hands[2]|hands[3] == 0 {
		return 0
	}
	seat := (leader + len(trick)) % 4
	legal := d.Legal(hands[seat], trick)
	best := math.MinInt32
	if seat%2 == 1 {
		best = math.MaxInt32
	}
	for _, c := range botlab.CardsOf(legal) {
		ci := botlab.CardIdx(c)
		nh := hands
		nh[seat] &^= 1 << uint(ci)
		nt := append(append([]int{}, trick...), ci)
		var val int
		if len(nt) == 4 {
			w := (leader + d.Winner(nt)) % 4
			pts := 0
			for _, x := range nt {
				pts += d.Pts(x)
			}
			sign := 1
			if w%2 == 1 {
				sign = -1
			}
			nc := capot
			switch {
			case nc == 0:
				nc = int8(1 + w%2)
			case nc == 1 && w%2 == 1, nc == 2 && w%2 == 0:
				nc = 3
			}
			val = sign * pts
			if nh[0]|nh[1]|nh[2]|nh[3] == 0 {
				switch nc {
				case 1:
					val += 100
				case 2:
					val -= 100
				default:
					val += sign * 10
				}
			} else {
				val += naive(d, nh, w, nil, nc)
			}
		} else {
			val = naive(d, nh, leader, nt, capot)
		}
		if seat%2 == 0 && val > best || seat%2 == 1 && val < best {
			best = val
		}
	}
	return best
}

func ddcheck() {
	rng := rand.New(rand.NewPCG(*seed, 3))
	bad := 0
	for r := 0; r < *n; r++ {
		deck := game.NewDeck()
		rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
		k := 2 + rng.IntN(3)
		var hands [4]uint32
		for s := 0; s < 4; s++ {
			hands[s] = botlab.MaskOf(deck[s*k : s*k+k])
		}
		d := botlab.NewDD(rng.IntN(4))
		capot := int8(rng.IntN(4))
		leader := rng.IntN(4)
		a := d.Value(hands, leader, nil, capot)
		b := naive(d, hands, leader, nil, capot)
		if a != b {
			bad++
			fmt.Printf("MISMATCH k=%d dd=%d naive=%d\n", k, a, b)
		}
	}
	fmt.Printf("ddcheck: %d positions, %d mismatches\n", *n, bad)
}

type stat struct {
	n    int
	sum  float64
	sum2 float64
}

func (s *stat) add(x float64) { s.n++; s.sum += x; s.sum2 += x * x }
func (s *stat) merge(o *stat) { s.n += o.n; s.sum += o.sum; s.sum2 += o.sum2 }
func (s *stat) mean() float64 {
	if s.n == 0 {
		return 0
	}
	return s.sum / float64(s.n)
}
func (s *stat) se() float64 {
	if s.n < 2 {
		return 0
	}
	m := s.mean()
	return math.Sqrt(math.Max(0, (s.sum2/float64(s.n)-m*m)/float64(s.n-1)))
}

type dupResult struct {
	pair                                     stat
	aCalls, aCallWins, bCalls, bCallWins     int
	passed, played, aPts, bPts, aFails, bFails int
	aDecl, bDecl                             int
}

func (r *dupResult) merge(o *dupResult) {
	r.pair.merge(&o.pair)
	r.aCalls += o.aCalls
	r.aCallWins += o.aCallWins
	r.bCalls += o.bCalls
	r.bCallWins += o.bCallWins
	r.passed += o.passed
	r.played += o.played
	r.aPts += o.aPts
	r.bPts += o.bPts
}

func duplicate(v game.Variant) {
	t0 := time.Now()
	deals := make([]botlab.Deal, *n)
	rng := rand.New(rand.NewPCG(*seed, 4))
	for i := range deals {
		deals[i] = botlab.RandomDeal(rng)
	}
	results := make([]dupResult, *workers)
	var wg sync.WaitGroup
	next := make(chan int)
	go func() {
		for i := 0; i < *n; i++ {
			next <- i
		}
		close(next)
	}()
	var nameA, nameB string
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			A := makePolicy(*polA, rand.New(rand.NewPCG(*seed, uint64(100+w))), *flagsA)
			B := makePolicy(*polB, rand.New(rand.NewPCG(*seed, uint64(200+w))), *flagsB)
			if w == 0 {
				nameA, nameB = A.Name(), B.Name()
			}
			res := &results[w]
			for i := range next {
				deal := deals[i]
				total := 0.0
				for g := 0; g < 2; g++ {
					var pol [4]botlab.Policy
					for s := 0; s < 4; s++ {
						if (s+g)%2 == 0 {
							pol[s] = A
						} else {
							pol[s] = B
						}
					}
					gs := botlab.NewDealState(deal, v)
					out, err := botlab.PlayHand(gs, pol, nil)
					if err != nil {
						fmt.Println("error:", err)
						os.Exit(1)
					}
					if out.Status != botlab.HandScored {
						res.passed++
						continue
					}
					res.played++
					a, b := botlab.Award(out)
					aTeam := g % 2
					if aTeam == 0 {
						total += float64(a - b)
						res.aPts += a
						res.bPts += b
					} else {
						total += float64(b - a)
						res.aPts += b
						res.bPts += a
					}
					if out.Caller%2 == aTeam {
						res.aCalls++
						if !out.Score.FailedContract {
							res.aCallWins++
						}
					} else {
						res.bCalls++
						if !out.Score.FailedContract {
							res.bCallWins++
						}
					}
				}
				res.pair.add(total / 2)
			}
		}(w)
	}
	wg.Wait()
	var tot dupResult
	for w := range results {
		tot.merge(&results[w])
	}
	fmt.Printf("%s vs %s (%s, %d deals x2, %.0fs)\n", nameA, nameB, v, *n, time.Since(t0).Seconds())
	fmt.Printf("  A advantage per hand: %+.2f ± %.2f (95%% CI ±%.2f)\n", tot.pair.mean(), tot.pair.se(), 1.96*tot.pair.se())
	fmt.Printf("  points banked: A %d  B %d  (A share %.1f%%)\n", tot.aPts, tot.bPts, 100*float64(tot.aPts)/float64(tot.aPts+tot.bPts))
	fmt.Printf("  A called %d (made %.0f%%), B called %d (made %.0f%%), passed out %d / %d\n",
		tot.aCalls, pct(tot.aCallWins, tot.aCalls), tot.bCalls, pct(tot.bCallWins, tot.bCalls), tot.passed, tot.played+tot.passed)
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return 100 * float64(a) / float64(b)
}

type regretAcc struct {
	byCtx   map[string]*stat
	byTrick [9]stat
	all     stat
	perHand stat
	optimal int
}

func newRegretAcc() *regretAcc { return &regretAcc{byCtx: map[string]*stat{}} }

func (r *regretAcc) merge(o *regretAcc) {
	for k, s := range o.byCtx {
		if r.byCtx[k] == nil {
			r.byCtx[k] = &stat{}
		}
		r.byCtx[k].merge(s)
	}
	for t := range r.byTrick {
		r.byTrick[t].merge(&o.byTrick[t])
	}
	r.all.merge(&o.all)
	r.perHand.merge(&o.perHand)
	r.optimal += o.optimal
}

func regret(v game.Variant) {
	t0 := time.Now()
	deals := make([]botlab.Deal, *n)
	rng := rand.New(rand.NewPCG(*seed, 7))
	for i := range deals {
		deals[i] = botlab.RandomDeal(rng)
	}
	accs := make([]*regretAcc, *workers)
	var wg sync.WaitGroup
	next := make(chan int)
	go func() {
		for i := 0; i < *n; i++ {
			next <- i
		}
		close(next)
	}()
	var nameA string
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			A := makePolicy(*polA, rand.New(rand.NewPCG(*seed, uint64(300+w))), *flagsA)
			if w == 0 {
				nameA = A.Name()
			}
			acc := newRegretAcc()
			accs[w] = acc
			pol := [4]botlab.Policy{A, A, A, A}
			for i := range next {
				gs := botlab.NewDealState(deals[i], v)
				var d *botlab.DD
				handRegret := 0.0
				hook := func(gs *game.GameState, seat int, action game.Action, view botlab.View) {
					if len(view.LegalCards) <= 1 {
						return
					}
					if d == nil {
						d = botlab.NewDD(botlab.SuitIdx(*gs.TrumpSuit))
					}
					_, hands, leader, trick, capot := botlab.StateDD(gs)
					vals := d.RootValues(hands, leader, trick, capot)
					sign := 1
					if seat%2 == 1 {
						sign = -1
					}
					best := math.MinInt32
					for _, val := range vals {
						if sign*val > best {
							best = sign * val
						}
					}
					chosen := sign * vals[botlab.CardIdx(*action.Card)]
					r := float64(best - chosen)
					ctx := context(view)
					if acc.byCtx[ctx] == nil {
						acc.byCtx[ctx] = &stat{}
					}
					acc.byCtx[ctx].add(r)
					acc.byTrick[gs.TrickNumber].add(r)
					acc.all.add(r)
					handRegret += r
					if r == 0 {
						acc.optimal++
					}
				}
				out, err := botlab.PlayHand(gs, pol, hook)
				if err != nil {
					fmt.Println("error:", err)
					os.Exit(1)
				}
				if out.Status == botlab.HandScored {
					acc.perHand.add(handRegret)
				}
			}
		}(w)
	}
	wg.Wait()
	tot := newRegretAcc()
	for _, a := range accs {
		tot.merge(a)
	}
	fmt.Printf("regret of %s (%s, %d deals, %.0fs): mean %.2f pts/decision, %.1f pts/hand (all 4 seats), optimal %.1f%% of %d decisions\n",
		nameA, v, *n, time.Since(t0).Seconds(), tot.all.mean(), tot.perHand.mean(), pct(tot.optimal, tot.all.n), tot.all.n)
	ctxs := make([]string, 0, len(tot.byCtx))
	for c := range tot.byCtx {
		ctxs = append(ctxs, c)
	}
	sort.Strings(ctxs)
	fmt.Println("  by context (mean regret, n, share of total regret):")
	for _, c := range ctxs {
		s := tot.byCtx[c]
		fmt.Printf("    %-34s %6.2f  n=%6d  share=%5.1f%%\n", c, s.mean(), s.n, 100*s.sum/tot.all.sum)
	}
	fmt.Println("  by trick (mean regret):")
	for t := 1; t <= 8; t++ {
		fmt.Printf("    trick %d: %6.2f  n=%d\n", t, tot.byTrick[t].mean(), tot.byTrick[t].n)
	}
}

func context(v botlab.View) string {
	if len(v.CurrentTrick) == 0 {
		if v.TrumpCallerSeat != nil && *v.TrumpCallerSeat%2 == v.Seat%2 {
			return "lead/ourCall"
		}
		return "lead/theirCall"
	}
	trump := *v.TrumpSuit
	led := v.CurrentTrick[0].Card.Suit
	wseat := v.CurrentTrick[0].PlayerSeat
	wcard := v.CurrentTrick[0].Card
	for _, tc := range v.CurrentTrick[1:] {
		if beats(tc.Card, wcard, led, trump) {
			wseat, wcard = tc.PlayerSeat, tc.Card
		}
	}
	who := "opp"
	if wseat%2 == v.Seat%2 {
		who = "partner"
	}
	pos := fmt.Sprintf("pos%d", len(v.CurrentTrick)+1)
	hasLed := false
	for _, c := range v.Hand {
		if c.Suit == led {
			hasLed = true
		}
	}
	shape := "follow"
	if !hasLed {
		shape = "void"
	}
	return "follow/" + who + "Wins/" + pos + "/" + shape
}

func beats(c, w game.Card, led, trump game.Suit) bool {
	ct, wt := c.Suit == trump, w.Suit == trump
	switch {
	case ct && !wt:
		return true
	case ct && wt:
		return game.TrumpRankOrder[c.Rank] > game.TrumpRankOrder[w.Rank]
	case wt:
		return false
	}
	return c.Suit == led && game.NonTrumpRankOrder[c.Rank] > game.NonTrumpRankOrder[w.Rank]
}

// ddcheck2 compares memo-on and memo-off alpha-beta from mid-hand positions of real deals.
func ddcheck2() {
	rng := rand.New(rand.NewPCG(*seed, 12))
	bad := 0
	for r := 0; r < *n; r++ {
		deal := botlab.RandomDeal(rng)
		var hands [4]uint32
		for s := 0; s < 4; s++ {
			hands[s] = botlab.MaskOf(deal.Deck[s*8 : s*8+8])
		}
		trump := rng.IntN(4)
		d := botlab.NewDD(trump)
		leader := 0
		capot := int8(0)
		tricks := 2 + rng.IntN(2)
		for t := 0; t < tricks; t++ {
			trick := []int{}
			for k := 0; k < 4; k++ {
				seat := (leader + k) % 4
				cs := botlab.CardsOf(d.Legal(hands[seat], trick))
				c := botlab.CardIdx(cs[rng.IntN(len(cs))])
				hands[seat] &^= 1 << uint(c)
				trick = append(trick, c)
			}
			w := (leader + d.Winner(trick)) % 4
			switch {
			case capot == 0:
				capot = int8(1 + w%2)
			case capot == 1 && w%2 == 1, capot == 2 && w%2 == 0:
				capot = 3
			}
			leader = w
		}
		a := d.Value(hands, leader, nil, capot)
		plain := botlab.NewDD(trump)
		plain.NoMemo = true
		b := plain.Value(hands, leader, nil, capot)
		if a != b {
			bad++
			fmt.Printf("MISMATCH after %d tricks: memo=%d plain=%d\n", tricks, a, b)
		}
	}
	fmt.Printf("ddcheck2: %d mid-hand positions (2-3 tricks played), %d mismatches\n", *n, bad)
}
