package botlab

import (
	"fmt"
	"slices"

	"github.com/emilijan/beljot/server/internal/game"
)

// EventTally counts table events for one policy over many hands.
type EventTally struct {
	Decisions            int
	BlindTens            int
	ForcedBlindTens      int
	HighKeptThenRuffed   int
	CapotsMade           int
	CapotsConceded       int
	CallerBeloteThenFell int
}

// Merge adds another tally into this one.
func (e *EventTally) Merge(o *EventTally) {
	e.Decisions += o.Decisions
	e.BlindTens += o.BlindTens
	e.ForcedBlindTens += o.ForcedBlindTens
	e.HighKeptThenRuffed += o.HighKeptThenRuffed
	e.CapotsMade += o.CapotsMade
	e.CapotsConceded += o.CapotsConceded
	e.CallerBeloteThenFell += o.CallerBeloteThenFell
}

// PerThousand scales an event count to events per 1000 decisions.
func (e EventTally) PerThousand(count int) float64 {
	if e.Decisions == 0 {
		return 0
	}
	return 1000 * float64(count) / float64(e.Decisions)
}

// Line formats the tally as one printed line for the named side.
func (e EventTally) Line(side string) string {
	return fmt.Sprintf("events %s: decisions %d, blind tens %d (%.1f per 1000; forced %d, free %d), high kept then ruffed %d (%.1f per 1000), capots made/conceded %d/%d, caller belote then fell %d",
		side, e.Decisions, e.BlindTens, e.PerThousand(e.BlindTens), e.ForcedBlindTens, e.BlindTens-e.ForcedBlindTens, e.HighKeptThenRuffed, e.PerThousand(e.HighKeptThenRuffed),
		e.CapotsMade, e.CapotsConceded, e.CallerBeloteThenFell)
}

type keptHigh struct {
	seat int
	card game.Card
}

// EventRecorder counts table events for the seats of one policy through the play hook.
type EventRecorder struct {
	name     string
	seats    [4]bool
	team     int
	tally    EventTally
	pending  []keptHigh
	inFlight []keptHigh
	trick    []game.TrickCard
	belote   [2]bool
}

// NewEventRecorder returns a recorder labelled with the policy name.
func NewEventRecorder(policyName string) *EventRecorder {
	return &EventRecorder{name: policyName}
}

// SetSeats names the seats this recorder counts for the next hand.
func (r *EventRecorder) SetSeats(seats [4]bool) {
	r.seats = seats
	for s, own := range seats {
		if own {
			r.team = s % 2
			return
		}
	}
}

// Tally returns the counts so far.
func (r *EventRecorder) Tally() EventTally { return r.tally }

// Hook returns the play hook that feeds the recorder; it ignores a call without a trump suit or a card.
func (r *EventRecorder) Hook() PlayHook {
	return func(gs *game.GameState, seat int, action game.Action, v View) {
		if gs.TrumpSuit == nil || action.Card == nil {
			return
		}
		if len(gs.CurrentTrick) == 0 {
			r.closeTrick(*gs.TrumpSuit)
		}
		for team := range gs.BelotPoints {
			if gs.BelotPoints[team] > 0 {
				r.belote[team] = true
			}
		}
		card := *action.Card
		r.trick = append(slices.Clone(gs.CurrentTrick), game.TrickCard{Card: card, PlayerSeat: seat})
		r.releaseKeptHigh(seat, card)
		if !r.seats[seat] {
			return
		}
		r.tally.Decisions++
		if isBlindTen(gs, seat, card, v) {
			r.tally.BlindTens++
			if len(v.LegalCards) == 1 {
				r.tally.ForcedBlindTens++
			}
		}
		r.keepHighCards(gs, seat, card, v)
	}
}

// FinishHand resolves the last trick, counts the hand-level events and clears the per-hand state.
func (r *EventRecorder) FinishHand(out HandOutcome, gs *game.GameState) {
	defer r.resetHand()
	if out.Status != HandScored {
		return
	}
	r.closeTrick(out.Trump)
	if out.Score.Capot && out.Score.CapotTeam != nil {
		if *out.Score.CapotTeam == r.team {
			r.tally.CapotsMade++
		} else {
			r.tally.CapotsConceded++
		}
	}
	if out.Caller%2 == r.team && r.belote[r.team] && gs.Rules.DeclarationsTransfer && out.Score.FailedContract {
		r.tally.CallerBeloteThenFell++
	}
}

func (r *EventRecorder) resetHand() {
	r.pending = r.pending[:0]
	r.inFlight = r.inFlight[:0]
	r.trick = nil
	r.belote = [2]bool{}
}

func (r *EventRecorder) closeTrick(trump game.Suit) {
	if len(r.trick) == 4 && r.trick[0].Card.Suit != trump {
		winnerSeat := trickWinnerSeat(r.trick, trump)
		if winnerCard := trickWinnerCard(r.trick, trump); winnerCard.Suit == trump {
			for _, k := range r.inFlight {
				if winnerSeat%2 != k.seat%2 {
					r.tally.HighKeptThenRuffed++
				}
			}
		}
	}
	r.inFlight = r.inFlight[:0]
	r.trick = nil
}

func (r *EventRecorder) releaseKeptHigh(seat int, card game.Card) {
	kept := r.pending[:0]
	for _, k := range r.pending {
		if k.seat == seat && k.card == card {
			r.inFlight = append(r.inFlight, k)
			continue
		}
		kept = append(kept, k)
	}
	r.pending = kept
}

func (r *EventRecorder) keepHighCards(gs *game.GameState, seat int, card game.Card, v View) {
	trump := *gs.TrumpSuit
	if len(gs.CurrentTrick) != 3 || gs.CurrentTrick[0].Card.Suit == trump || card.Suit != gs.CurrentTrick[0].Card.Suit {
		return
	}
	for _, tc := range gs.CurrentTrick {
		if tc.Card.Suit == trump {
			return
		}
	}
	if trickWinnerSeat(r.trick, trump) != seat {
		return
	}
	for _, held := range v.Hand {
		isHigh := held.Rank == game.RankAce || held.Rank == game.RankTen
		if held.Suit != card.Suit || !isHigh || game.NonTrumpRankOrder[held.Rank] <= game.NonTrumpRankOrder[card.Rank] {
			continue
		}
		k := keptHigh{seat: seat, card: held}
		if !slices.Contains(r.pending, k) {
			r.pending = append(r.pending, k)
		}
	}
}

func isBlindTen(gs *game.GameState, seat int, card game.Card, v View) bool {
	trump := *gs.TrumpSuit
	if card.Rank != game.RankTen || card.Suit == trump {
		return false
	}
	if len(gs.CurrentTrick) > 0 && trickWinnerSeat(gs.CurrentTrick, trump)%2 == seat%2 {
		return false
	}
	if len(gs.CurrentTrick) > 0 && gs.CurrentTrick[0].Card.Suit == card.Suit && !trumpCut(gs.CurrentTrick, trump) {
		return false
	}
	ace := game.Card{Rank: game.RankAce, Suit: card.Suit}
	partner := (seat + 2) % 4
	return !slices.Contains(v.PlayedCards, ace) && !slices.Contains(v.Hand, ace) && !slices.Contains(v.KnownCards[partner], ace)
}

// trumpCut reports whether a trump lies on a trick whose led suit is not trump.
func trumpCut(trick []game.TrickCard, trump game.Suit) bool {
	if trick[0].Card.Suit == trump {
		return false
	}
	for _, tc := range trick {
		if tc.Card.Suit == trump {
			return true
		}
	}
	return false
}

func trickWinnerSeat(trick []game.TrickCard, trump game.Suit) int {
	best := trick[0]
	for _, tc := range trick[1:] {
		if beats(tc.Card, best.Card, trick[0].Card.Suit, trump) {
			best = tc
		}
	}
	return best.PlayerSeat
}

func trickWinnerCard(trick []game.TrickCard, trump game.Suit) game.Card {
	seat := trickWinnerSeat(trick, trump)
	for _, tc := range trick {
		if tc.PlayerSeat == seat {
			return tc.Card
		}
	}
	return trick[0].Card
}

func beats(c, best game.Card, led, trump game.Suit) bool {
	switch {
	case c.Suit == trump && best.Suit != trump:
		return true
	case c.Suit == trump && best.Suit == trump:
		return game.TrumpRankOrder[c.Rank] > game.TrumpRankOrder[best.Rank]
	case c.Suit == led && best.Suit == led:
		return game.NonTrumpRankOrder[c.Rank] > game.NonTrumpRankOrder[best.Rank]
	}
	return false
}
