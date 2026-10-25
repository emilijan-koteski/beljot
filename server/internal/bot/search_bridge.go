package bot

import (
	"math/bits"
	"slices"
	"time"

	"github.com/emilijan/beljot/server/internal/bot/search"
	"github.com/emilijan/beljot/server/internal/game"
)

const (
	searchEarlyStopAfter = 8
	searchEarlyStopSE    = 1.0
	lastTrickBonusPoints = 10
	allCards             = uint32(1<<search.NCards - 1)
)

// SearchResult is the card the search chose with the deals behind it, its lead over the runner-up and that lead's
// standard error.
type SearchResult struct {
	Card    game.Card
	Samples int
	Margin  float64
	SE      float64
}

// SearchApplies reports whether the Monte Carlo search should pick this card: from the given trick on, in the playing
// phase with a real choice, and not when a team could still end the match inside this hand.
func SearchApplies(v View, fromTrick int) bool {
	if fromTrick <= 0 || v.Phase != game.PhasePlaying || v.TrumpSuit == nil {
		return false
	}
	if v.PendingBelot || v.AwaitingDeclaration || len(v.LegalCards) <= 1 || v.TrickNumber < fromTrick {
		return false
	}
	return !withinStopAtTargetReach(v)
}

func withinStopAtTargetReach(v View) bool {
	if !v.StopAtTarget {
		return false
	}
	reach := remainingCardPoints(v) + lastTrickBonusPoints
	for team := range 2 {
		banked := v.TeamScores[team] + v.HandPoints[team] + v.DeclPoints[team]
		if v.MatchTarget-banked <= reach {
			return true
		}
	}
	return false
}

// SearchCard runs the search for the seat's card, reporting false when it reached no decision and the rules should play.
// It is deterministic for a given view.
func SearchCard(v View, deadline time.Time) (SearchResult, bool) {
	samples := search.SamplesForTrick(v.TrickNumber)
	if samples == 0 || v.TrumpSuit == nil || v.TrumpCallerSeat == nil {
		return SearchResult{}, false
	}
	opts := search.Options{Samples: samples, EarlyStopAfter: searchEarlyStopAfter, EarlyStopSE: searchEarlyStopSE}
	res, ok := search.Choose(informationFrom(v), positionFrom(v), v.LegalCards, settlementFrom(v), opts, deadline)
	if !ok {
		return SearchResult{}, false
	}
	return SearchResult{Card: res.Card, Samples: res.Samples, Margin: res.Margin, SE: res.SE}, true
}

// DecideWith plays the searched card when it is still a legal play and nothing else is pending, otherwise it decides by
// the rules.
func DecideWith(v View, pre *SearchResult) game.Action {
	if pre == nil || !canPlaySearchedCard(v, pre.Card) {
		return Decide(v)
	}
	card := pre.Card
	return game.Action{Type: game.ActionPlayCard, PlayerSeat: v.Seat, Card: &card}
}

func canPlaySearchedCard(v View, card game.Card) bool {
	if v.Phase != game.PhasePlaying || v.PendingBelot || v.AwaitingDeclaration || v.PartnerProposedSurrender {
		return false
	}
	return slices.Contains(v.LegalCards, card)
}

// informationFrom turns the view into what the sampler needs: the seat's hand, the cards it cannot place, those it can
// place with a seat, and how many cards each other seat still holds.
func informationFrom(v View) search.Information {
	own := search.MaskOf(v.Hand)
	in := search.Information{Seat: v.Seat, Own: own}
	var located uint32
	for seat := range 4 {
		if seat == v.Seat {
			continue
		}
		in.Known[seat] = search.MaskOf(knownHeldBy(v, seat)) &^ own
		in.CannotHold[seat] = cannotHold(v, seat) &^ in.Known[seat]
		in.Need[seat] = max(0, cardsHeldBy(v, seat)-bits.OnesCount32(in.Known[seat]))
		located |= in.Known[seat]
	}
	gone := search.MaskOf(v.PlayedCards) | trickMask(v)
	in.Unseen = allCards &^ own &^ gone &^ located
	return in
}

func cannotHold(v View, seat int) uint32 {
	var mask uint32
	for suit := range 4 {
		if v.KnownVoids[seat][suit] {
			mask |= search.SuitMask(suit)
		}
	}
	for c := range cardsProvablyAbsent(v, seat) {
		mask |= 1 << uint(search.CardIdx(c))
	}
	return mask
}

func cardsHeldBy(v View, seat int) int {
	if slices.ContainsFunc(v.CurrentTrick, func(tc game.TrickCard) bool { return tc.PlayerSeat == seat }) {
		return len(v.Hand) - 1
	}
	return len(v.Hand)
}

func trickMask(v View) uint32 {
	var mask uint32
	for _, tc := range v.CurrentTrick {
		mask |= 1 << uint(search.CardIdx(tc.Card))
	}
	return mask
}

// positionFrom needs a trump suit. The trick lists its cards in play order, led by the first card's seat.
func positionFrom(v View) search.Position {
	pos := search.Position{
		Trump:     search.SuitIdx(*v.TrumpSuit),
		Clockwise: v.Clockwise,
		Leader:    v.Seat,
		Trick:     make([]int, len(v.CurrentTrick)),
		Capot:     search.CapotFlag(v.TricksWon),
	}
	if len(v.CurrentTrick) > 0 {
		pos.Leader = v.CurrentTrick[0].PlayerSeat
	}
	for i, tc := range v.CurrentTrick {
		pos.Trick[i] = search.CardIdx(tc.Card)
	}
	return pos
}

// settlementFrom needs a trump suit and its caller, and counts as remaining every card the solver will score.
func settlementFrom(v View) search.Settlement {
	return search.Settlement{
		HandPoints:           v.HandPoints,
		DeclPoints:           v.DeclPoints,
		TricksWon:            v.TricksWon,
		CallerTeam:           game.TeamForSeat(*v.TrumpCallerSeat),
		DeclarationsTransfer: v.DeclarationsTransfer,
		RemainingCardPoints:  remainingCardPoints(v),
	}
}

// remainingCardPoints sums the trump-aware points of the seat's hand, the unseen and located cards and the current
// trick. It needs a trump suit.
func remainingCardPoints(v View) int {
	in := informationFrom(v)
	cards := in.Own | in.Unseen | in.Known[0] | in.Known[1] | in.Known[2] | in.Known[3] | trickMask(v)
	trump := search.SuitIdx(*v.TrumpSuit)
	total := 0
	for ; cards != 0; cards &= cards - 1 {
		total += search.Points(bits.TrailingZeros32(cards), trump)
	}
	return total
}
