package search

import (
	"encoding/binary"
	"hash/fnv"
	"math/rand/v2"
)

const (
	seats             = 4
	rejectionDraws    = 200
	constructiveFills = 200
)

// Information is everything one seat knows about the hands it cannot see.
// Known are proven cards, Need the cards still to deal, CannotHold the cards a seat provably lacks.
type Information struct {
	Seat       int
	Own        uint32
	Unseen     uint32
	Known      [4]uint32
	Need       [4]int
	CannotHold [4]uint32
}

// Seed hashes the information fields in a fixed order, so equal information always yields the same seed.
func (in Information) Seed() uint64 {
	buf := []byte{byte(in.Seat)}
	buf = binary.LittleEndian.AppendUint32(buf, in.Own)
	buf = binary.LittleEndian.AppendUint32(buf, in.Unseen)
	for _, known := range in.Known {
		buf = binary.LittleEndian.AppendUint32(buf, known)
	}
	for _, need := range in.Need {
		buf = append(buf, byte(need))
	}
	for _, cannotHold := range in.CannotHold {
		buf = binary.LittleEndian.AppendUint32(buf, cannotHold)
	}
	hash := fnv.New64a()
	_, _ = hash.Write(buf)
	return hash.Sum64()
}

// Sample draws one deal of the unseen cards that respects every hard constraint, or false when none fits.
// It rejects random deals first, then builds one card by card.
func Sample(rng *rand.Rand, in Information) ([4]uint32, bool) {
	cards := singleCardMasks(in.Unseen)
	for range rejectionDraws {
		shuffle(rng, cards)
		if hands, ok := dealByRejection(cards, in); ok {
			return hands, true
		}
	}
	for range constructiveFills {
		shuffle(rng, cards)
		if hands, ok := dealConstructively(rng, cards, in); ok {
			return hands, true
		}
	}
	return [4]uint32{}, false
}

func singleCardMasks(m uint32) []uint32 {
	cards := make([]uint32, 0, NCards)
	for ; m != 0; m &= m - 1 {
		cards = append(cards, m&-m)
	}
	return cards
}

func shuffle(rng *rand.Rand, cards []uint32) {
	rng.Shuffle(len(cards), func(i, j int) { cards[i], cards[j] = cards[j], cards[i] })
}

func startingHands(in Information) [4]uint32 {
	var hands [4]uint32
	for s := range seats {
		if s == in.Seat {
			hands[s] = in.Own
		} else {
			hands[s] = in.Known[s]
		}
	}
	return hands
}

func dealByRejection(cards []uint32, in Information) ([4]uint32, bool) {
	hands := startingHands(in)
	next := 0
	for s := range seats {
		if s == in.Seat {
			continue
		}
		for range in.Need[s] {
			if next == len(cards) || cards[next]&in.CannotHold[s] != 0 {
				return [4]uint32{}, false
			}
			hands[s] |= cards[next]
			next++
		}
	}
	return hands, next == len(cards)
}

func dealConstructively(rng *rand.Rand, cards []uint32, in Information) ([4]uint32, bool) {
	hands := startingHands(in)
	need := in.Need
	for _, card := range cards {
		var open [seats]int
		count := 0
		for s := range seats {
			if s != in.Seat && need[s] > 0 && card&in.CannotHold[s] == 0 {
				open[count] = s
				count++
			}
		}
		if count == 0 {
			return [4]uint32{}, false
		}
		s := open[rng.IntN(count)]
		hands[s] |= card
		need[s]--
	}
	return hands, allDealt(need, in.Seat)
}

func allDealt(need [4]int, seat int) bool {
	for s, n := range need {
		if s != seat && n != 0 {
			return false
		}
	}
	return true
}
