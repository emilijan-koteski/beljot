package bot

import (
	"math/bits"

	"github.com/emilijan/beljot/server/internal/bot/search"
	"github.com/emilijan/beljot/server/internal/game"
)

const (
	bitolaBidHandSize   = 5
	croatianBidHandSize = 6
	forcedPickPasses    = 3
)

// bidCheck is one other seat's auction action as the table bidder would have taken it: the suits it could call, the
// table inputs, how many of its cards it held at the time, and whether it passed.
type bidCheck struct {
	seat         int
	passed       bool
	suits        []game.Suit
	hasCandidate bool
	pos          int
	handSize     int
}

// fits reports whether the bid-time hand, candidate included, matches the table for one of the check's suits.
func (c *bidCheck) fits(hand uint32) bool {
	for _, suit := range c.suits {
		if profileMatchesBidTable(profileOfMask(hand, suit), c.hasCandidate, c.pos) {
			return true
		}
	}
	return false
}

// explained reports whether some k cards of pool, added to chosen, form a bid-time hand that would have taken the
// recorded action: a match for a call, no match for a pass.
func (c *bidCheck) explained(pool uint32, k int, chosen uint32) bool {
	if k == 0 {
		return c.fits(chosen) != c.passed
	}
	if bits.OnesCount32(pool) < k {
		return false
	}
	low := pool & -pool
	return c.explained(pool&^low, k-1, chosen|low) || c.explained(pool&^low, k, chosen)
}

// profileOfMask is profileSuit for a hand given as a card mask.
func profileOfMask(hand uint32, suit game.Suit) suitProfile {
	idx := search.SuitIdx(suit)
	own := hand & search.SuitMask(idx)
	return suitProfile{
		count:    bits.OnesCount32(own),
		jack:     own&cardBit(game.Card{Rank: game.RankJack, Suit: suit}) != 0,
		nine:     own&cardBit(game.Card{Rank: game.Rank9, Suit: suit}) != 0,
		ace:      own&cardBit(game.Card{Rank: game.RankAce, Suit: suit}) != 0,
		sideAces: bits.OnesCount32(hand & acesMask &^ search.SuitMask(idx)),
	}
}

func cardBit(c game.Card) uint32 {
	return 1 << uint(search.CardIdx(c))
}

var acesMask = search.MaskOf([]game.Card{
	{Rank: game.RankAce, Suit: game.SuitSpades}, {Rank: game.RankAce, Suit: game.SuitHearts},
	{Rank: game.RankAce, Suit: game.SuitDiamonds}, {Rank: game.RankAce, Suit: game.SuitClubs},
})

// auctionCandidate is the Bitola candidate the auction recorded, nil when no record carries one.
func auctionCandidate(bids []BidRecord) *game.Card {
	for _, rec := range bids {
		if rec.Candidate != nil {
			return rec.Candidate
		}
	}
	return nil
}

// bidChecks turns every other seat's auction action into a check, leaving out Croatian's forced dealer pick.
func bidChecks(v View, candidate *game.Card) []bidCheck {
	var checks []bidCheck
	for _, rec := range v.Bids {
		if rec.Seat == v.Seat || rec.Seat < 0 || rec.Seat > 3 {
			continue
		}
		check := bidCheck{seat: rec.Seat, passed: rec.Passed, hasCandidate: v.HasTrumpCandidate, pos: rec.PassesBefore,
			handSize: croatianBidHandSize}
		if v.HasTrumpCandidate {
			check.handSize = bitolaBidHandSize
		}
		switch {
		case !rec.Passed && rec.Suit == nil:
			continue
		case !rec.Passed && !v.HasTrumpCandidate && rec.PassesBefore == forcedPickPasses:
			continue
		case !rec.Passed:
			check.suits = []game.Suit{*rec.Suit}
		case v.HasTrumpCandidate && rec.Round == 1:
			check.suits = []game.Suit{candidate.Suit}
		default:
			check.suits = callableSuits(v.HasTrumpCandidate, candidate)
		}
		checks = append(checks, check)
	}
	return checks
}

// callableSuits is every suit but the candidate's in a Bitola second round, all four in Croatian.
func callableSuits(hasCandidate bool, candidate *game.Card) []game.Suit {
	var suits []game.Suit
	for _, suit := range game.AllSuits {
		if !hasCandidate || suit != candidate.Suit {
			suits = append(suits, suit)
		}
	}
	return suits
}

// bidConsistent rejects a sampled deal in which another seat's auction action contradicts the bid table, trying every
// bid-time hand its eight cards allow. It is nil without an auction, or for a Bitola auction with no recorded candidate.
func bidConsistent(v View) func(hands [4]uint32) bool {
	if len(v.Bids) == 0 {
		return nil
	}
	candidate := auctionCandidate(v.Bids)
	if v.HasTrumpCandidate && candidate == nil {
		return nil
	}
	checks := bidChecks(v, candidate)
	if len(checks) == 0 {
		return nil
	}
	var seen [4]uint32
	for seat := range seen {
		seen[seat] = search.MaskOf(v.PlayedBy[seat])
	}
	for _, tc := range v.CurrentTrick {
		seen[tc.PlayerSeat] |= cardBit(tc.Card)
	}
	var shown uint32
	if v.HasTrumpCandidate {
		shown = cardBit(*candidate)
	}
	return func(hands [4]uint32) bool {
		for i := range checks {
			c := &checks[i]
			pool := (hands[c.seat] | seen[c.seat]) &^ shown
			if !c.explained(pool, min(c.handSize, bits.OnesCount32(pool)), shown) {
				return false
			}
		}
		return true
	}
}

// filteredOptions adds the auction filter to the options when the tuning asks for it and the auction says anything.
func filteredOptions(v View, opts search.Options, t SearchTuning) search.Options {
	if t.BidConsistency {
		opts.SampleFilter = bidConsistent(v)
	}
	return opts
}
