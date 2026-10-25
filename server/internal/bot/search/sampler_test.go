package search_test

import (
	"fmt"
	"math/bits"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/bot/search"
)

const seatCount = 4

func rngFor(in search.Information) *rand.Rand {
	return rand.New(rand.NewPCG(in.Seed(), 0))
}

func randomInformation(rng *rand.Rand) search.Information {
	deck := rng.Perm(search.NCards)
	var remaining [seatCount]uint32
	for i, card := range deck {
		remaining[i/8] |= 1 << uint(card)
	}
	in := search.Information{Seat: rng.IntN(seatCount)}
	for s := range seatCount {
		for m := remaining[s]; m != 0; m &= m - 1 {
			if rng.IntN(100) < 25 {
				remaining[s] &^= m & -m
			}
		}
	}
	in.Own = remaining[in.Seat]
	for s := range seatCount {
		if s == in.Seat {
			continue
		}
		for m := remaining[s]; m != 0; m &= m - 1 {
			if rng.IntN(100) < 20 {
				in.Known[s] |= m & -m
			}
		}
		in.Need[s] = bits.OnesCount32(remaining[s] &^ in.Known[s])
		in.Unseen |= remaining[s] &^ in.Known[s]
		for suit := range seatCount {
			if remaining[s]&search.SuitMask(suit) == 0 && rng.IntN(2) == 0 {
				in.CannotHold[s] |= search.SuitMask(suit)
			}
		}
		for card := range search.NCards {
			if remaining[s]&(1<<uint(card)) == 0 && rng.IntN(100) < 10 {
				in.CannotHold[s] |= 1 << uint(card)
			}
		}
	}
	return in
}

func requireConsistent(t *testing.T, in search.Information, hands [4]uint32) {
	t.Helper()
	require.Equal(t, in.Own, hands[in.Seat])
	all := in.Own | in.Unseen
	var union uint32
	for s := range seatCount {
		require.Zero(t, union&hands[s], "hands overlap")
		union |= hands[s]
		if s == in.Seat {
			continue
		}
		all |= in.Known[s]
		require.Equal(t, in.Known[s], hands[s]&in.Known[s], "seat %d lost a known card", s)
		require.Equal(t, bits.OnesCount32(in.Known[s])+in.Need[s], bits.OnesCount32(hands[s]), "seat %d hand size", s)
		require.Zero(t, hands[s]&in.CannotHold[s], "seat %d holds a forbidden card", s)
	}
	require.Equal(t, all, union)
}

func TestSampleRespectsEveryConstraint(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for range 20 {
		in := randomInformation(rng)
		sampler := rngFor(in)
		for range 50 {
			hands, ok := search.Sample(sampler, in)
			require.True(t, ok)
			requireConsistent(t, in, hands)
		}
	}
}

func TestSampleIsDeterministicForEqualInformation(t *testing.T) {
	first := randomInformation(rand.New(rand.NewPCG(3, 4)))
	second := randomInformation(rand.New(rand.NewPCG(3, 4)))
	require.Equal(t, first.Seed(), second.Seed())
	a, b := rngFor(first), rngFor(second)
	for range 10 {
		handsA, okA := search.Sample(a, first)
		handsB, okB := search.Sample(b, second)
		require.True(t, okA)
		require.True(t, okB)
		require.Equal(t, handsA, handsB)
	}
}

func TestSeedChangesWithInformation(t *testing.T) {
	base := randomInformation(rand.New(rand.NewPCG(5, 6)))
	flips := map[string]func(*search.Information){
		"seat":   func(in *search.Information) { in.Seat ^= 1 },
		"own":    func(in *search.Information) { in.Own ^= 1 },
		"unseen": func(in *search.Information) { in.Unseen ^= 1 },
	}
	for s := range seatCount {
		flips[fmt.Sprintf("known%d", s)] = func(in *search.Information) { in.Known[s] ^= 1 }
		flips[fmt.Sprintf("need%d", s)] = func(in *search.Information) { in.Need[s]++ }
		flips[fmt.Sprintf("cannotHold%d", s)] = func(in *search.Information) { in.CannotHold[s] ^= 1 }
	}
	for name, flip := range flips {
		t.Run(name, func(t *testing.T) {
			changed := base
			flip(&changed)
			assert.NotEqual(t, base.Seed(), changed.Seed())
		})
	}
}

func TestSampleFallsBackToConstructiveFill(t *testing.T) {
	in := search.Information{
		Seat:   0,
		Own:    search.SuitMask(3),
		Unseen: search.SuitMask(0) | search.SuitMask(1) | search.SuitMask(2),
		Need:   [4]int{0, 8, 8, 8},
		CannotHold: [4]uint32{
			1: search.SuitMask(0),
			2: search.SuitMask(1),
			3: search.SuitMask(2),
		},
	}
	sampler := rngFor(in)
	for range 100 {
		hands, ok := search.Sample(sampler, in)
		require.True(t, ok)
		requireConsistent(t, in, hands)
	}
}
