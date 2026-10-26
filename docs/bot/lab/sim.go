package botlab

import (
	"fmt"
	"math/rand/v2"
	"slices"

	"github.com/emilijan/beljot/server/internal/bot"
	"github.com/emilijan/beljot/server/internal/game"
)

// Policy decides for one seat from the redacted view only.
type Policy interface {
	Name() string
	Decide(v View) game.Action
}

// StateAware is implemented by lab-only policies that read the full state (perfect information).
type StateAware interface {
	DecideState(gs *game.GameState, seat int, v View) game.Action
}

// CheatBot plays the double-dummy optimum of the actual deal: an upper bound, never fair.
type CheatBot struct{ Fallback Policy }

func (CheatBot) Name() string             { return "cheat" }
func (c CheatBot) Decide(v View) game.Action { return c.Fallback.Decide(v) }
func (c CheatBot) DecideState(gs *game.GameState, seat int, v View) game.Action {
	if gs.Phase != game.PhasePlaying || gs.PendingBelotSeat != nil || gs.AwaitingDeclaration || len(v.LegalCards) <= 1 {
		return c.Fallback.Decide(v)
	}
	d, hands, leader, trick, capot := StateDD(gs)
	vals := d.RootValues(hands, leader, trick, capot)
	sign := 1
	if seat%2 == 1 {
		sign = -1
	}
	best := -1
	bestVal := -1 << 30
	for _, lc := range v.LegalCards {
		idx := CardIdx(lc)
		if val, ok := vals[idx]; ok && sign*val > bestVal {
			bestVal = sign * val
			best = idx
		}
	}
	card := IdxCard(best)
	return game.Action{Type: game.ActionPlayCard, PlayerSeat: seat, Card: &card}
}

// CurrentBot wraps the production bot.
type CurrentBot struct{}

func (CurrentBot) Name() string { return "current" }
func (CurrentBot) Decide(v View) game.Action {
	return bot.Decide(bot.View{
		Seat: v.Seat, Hand: v.Hand, LegalCards: v.LegalCards, Phase: v.Phase, BiddingRound: v.BiddingRound,
		TrumpCandidate: v.TrumpCandidate, TrumpSuit: v.TrumpSuit, TrumpCallerSeat: v.TrumpCallerSeat,
		DealerSeat: v.DealerSeat, MustPickTrump: v.MustPickTrump, CurrentTrick: v.CurrentTrick, LeadSuit: v.LeadSuit,
		ActivePlayerSeat: v.ActivePlayerSeat, AwaitingDeclaration: v.AwaitingDeclaration, HasDeclarations: v.HasDeclarations,
		PendingBelot: v.PendingBelot, PartnerProposedSurrender: v.PartnerProposedSurrender, TeamScores: v.TeamScores,
		HandPoints: v.HandPoints, TricksWon: v.TricksWon, PlayedCards: v.PlayedCards, KnownVoids: v.KnownVoidsWithTrump, KnownCards: v.KnownCards,
		HasTrumpCandidate: v.HasTrumpCandidate, DeclaredCards: v.DeclaredCards, Clockwise: v.Clockwise, TrickNumber: v.TrickNumber,
		DeclPoints: v.DeclPoints, MatchTarget: v.MatchTarget, StopAtTarget: v.StopAtTarget, DeclarationsTransfer: v.DeclarationsTransfer,
	})
}

// LabBot runs the lab copy of the bot (with experiment flags).
type LabBot struct{ Flags Flags }

func (l LabBot) Name() string { return "lab[" + l.Flags.Text + "]" }
func (l LabBot) Decide(v View) game.Action {
	v.F = l.Flags
	if l.Flags.On("memory4") {
		v.KnownVoids = v.KnownVoidsWithTrump
	}
	return Decide(v)
}

// RandomBot plays uniformly among legal options; bids like the baseline test.
type RandomBot struct{ Rng *rand.Rand }

func (RandomBot) Name() string { return "random" }
func (r RandomBot) Decide(v View) game.Action {
	seat := v.Seat
	if v.Phase == game.PhaseBidding {
		if v.MustPickTrump {
			s := game.AllSuits[r.Rng.IntN(4)]
			return game.Action{Type: game.ActionPickTrump, PlayerSeat: seat, Suit: &s}
		}
		if v.BiddingRound == 1 && v.TrumpCandidate != nil {
			if r.Rng.IntN(2) == 0 {
				return game.Action{Type: game.ActionPickTrump, PlayerSeat: seat}
			}
			return game.Action{Type: game.ActionPassTrump, PlayerSeat: seat}
		}
		opts := make([]game.Suit, 0, 4)
		for _, s := range game.AllSuits {
			if v.TrumpCandidate != nil && s == v.TrumpCandidate.Suit {
				continue
			}
			opts = append(opts, s)
		}
		pick := r.Rng.IntN(len(opts) + 1)
		if pick == len(opts) {
			return game.Action{Type: game.ActionPassTrump, PlayerSeat: seat}
		}
		s := opts[pick]
		return game.Action{Type: game.ActionPickTrump, PlayerSeat: seat, Suit: &s}
	}
	if v.Phase == game.PhaseDeclaring {
		if v.HasDeclarations {
			return game.Action{Type: game.ActionDeclare, PlayerSeat: seat}
		}
		return game.Action{Type: game.ActionSkipDeclare, PlayerSeat: seat}
	}
	if v.PendingBelot {
		return game.Action{Type: game.ActionAnnounceBelot, PlayerSeat: seat}
	}
	if v.AwaitingDeclaration {
		return game.Action{Type: game.ActionDeclare, PlayerSeat: seat}
	}
	c := v.LegalCards[r.Rng.IntN(len(v.LegalCards))]
	return game.Action{Type: game.ActionPlayCard, PlayerSeat: seat, Card: &c}
}

// Deal is a reproducible shuffled deck plus dealer.
type Deal struct {
	Deck   []game.Card
	Dealer int
}

func RandomDeal(rng *rand.Rand) Deal {
	deck := game.NewDeck()
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	return Deal{Deck: deck, Dealer: rng.IntN(4)}
}

// NewDealState builds a fresh engine state for the deal under the variant, mirroring game.dealCards.
func NewDealState(d Deal, variant game.Variant) *game.GameState {
	gs := game.NewGame([4]uint{0, 0, 0, 0}, [4]string{"", "", "", ""}, [4]bool{true, true, true, true}, variant, "1001", 1, true, false, true, false)
	for i := range gs.Players {
		gs.Players[i].Hand = []game.Card{}
		gs.Players[i].FaceDownCards = nil
		gs.Players[i].FaceDownCount = 0
	}
	gs.Deck = nil
	gs.TrumpCandidate = nil
	gs.DealerSeat = d.Dealer
	gs.ActivePlayerSeat = (d.Dealer + 1) % 4
	deck := d.Deck
	idx := 0
	if gs.Rules.DealShape == game.DealShapeAllBeforeBidding {
		for batch := 0; batch < 2; batch++ {
			for i := 0; i < 4; i++ {
				seat := (d.Dealer + 1 + i) % 4
				gs.Players[seat].Hand = append(gs.Players[seat].Hand, slices.Clone(deck[idx:idx+3])...)
				idx += 3
			}
		}
		for i := 0; i < 4; i++ {
			seat := (d.Dealer + 1 + i) % 4
			gs.Players[seat].FaceDownCards = slices.Clone(deck[idx : idx+2])
			gs.Players[seat].FaceDownCount = 2
			idx += 2
		}
		return gs
	}
	for i := 0; i < 4; i++ {
		seat := (d.Dealer + 1 + i) % 4
		gs.Players[seat].Hand = append(gs.Players[seat].Hand, slices.Clone(deck[idx:idx+3])...)
		idx += 3
	}
	for i := 0; i < 4; i++ {
		seat := (d.Dealer + 1 + i) % 4
		gs.Players[seat].Hand = append(gs.Players[seat].Hand, slices.Clone(deck[idx:idx+2])...)
		idx += 2
	}
	cand := deck[idx]
	gs.TrumpCandidate = &cand
	idx++
	gs.Deck = slices.Clone(deck[idx:])
	return gs
}

// BuildView mirrors match.buildBotView.
func BuildView(gs *game.GameState, seat int, mem *Memory) View {
	v := View{
		Seat:                seat,
		Hand:                gs.Players[seat].Hand,
		Phase:               gs.Phase,
		BiddingRound:        gs.BiddingRound,
		TrumpCandidate:      gs.TrumpCandidate,
		TrumpSuit:           gs.TrumpSuit,
		TrumpCallerSeat:     gs.TrumpCallerSeat,
		DealerSeat:          gs.DealerSeat,
		CurrentTrick:        gs.CurrentTrick,
		LeadSuit:            gs.LeadSuit,
		ActivePlayerSeat:    gs.ActivePlayerSeat,
		AwaitingDeclaration: gs.AwaitingDeclaration && gs.ActivePlayerSeat == seat,
		HasDeclarations:     gs.Phase == game.PhaseDeclaring && game.HasDeclarableCombinations(gs, seat),
		PendingBelot:        gs.PendingBelotSeat != nil && *gs.PendingBelotSeat == seat,
		TeamScores:          gs.TeamScores,
		HandPoints:          gs.HandPoints,
		TricksWon:           gs.TricksWon,

		HasTrumpCandidate:    gs.Rules.HasTrumpCandidate,
		Clockwise:            gs.Rules.Clockwise,
		TrickNumber:          gs.TrickNumber,
		MatchTarget:          game.MatchTarget(gs.MatchMode),
		StopAtTarget:         gs.Rules.StopAtTarget,
		DeclarationsTransfer: gs.Rules.DeclarationsTransfer,
	}
	if gs.SurrenderProposerSeat != nil && (*gs.SurrenderProposerSeat+2)%4 == seat {
		v.PartnerProposedSurrender = true
	}
	if gs.Phase == game.PhasePlaying {
		v.LegalCards = game.LegalCards(gs, seat)
	}
	v.MustPickTrump = game.MustPickTrump(gs, seat)
	if mem != nil {
		v.PlayedCards = mem.PlayedCards()
		v.KnownVoids = mem.KnownVoids()
		v.KnownVoidsWithTrump = mem.KnownVoidsWithTrump()
		v.KnownCards = mem.KnownCards()
		v.DeclaredCards = mem.DeclaredCards()
	}
	v.DeclPoints = [2]int{gs.DeclarationPoints[0] + gs.BelotPoints[0], gs.DeclarationPoints[1] + gs.BelotPoints[1]}
	v.BidRound = gs.BiddingRound
	if gs.TrumpCallerSeat != nil {
		caller := *gs.TrumpCallerSeat
		first := (gs.DealerSeat + 1) % 4
		hasCandidate := gs.Rules.HasTrumpCandidate
		for k := 0; k < 4; k++ {
			s := (first + k) % 4
			if s == caller {
				break
			}
			if hasCandidate && gs.BiddingRound == 1 {
				v.Passed[s].Candidate = true
			} else {
				v.Passed[s].FreeSuits = true
			}
		}
		if hasCandidate && gs.BiddingRound == 2 {
			for s := 0; s < 4; s++ {
				v.Passed[s].Candidate = true
			}
		}
		v.ForcedCall = gs.Rules.AllPassOutcome == game.AllPassDealerMustPick && caller == gs.DealerSeat && gs.BiddingPassCount >= 3
	}
	return v
}

type HandStatus int

const (
	HandScored HandStatus = iota
	HandPassedOut
	HandInstantWin
)

// PlayHook is called before every card play with the full state (for measurement only).
type PlayHook func(gs *game.GameState, seat int, action game.Action, v View)

type HandOutcome struct {
	Status HandStatus
	Score  game.HandScore
	Caller int
	Trump  game.Suit
}

// PlayHand drives one deal to a scored hand with the given seat policies.
func PlayHand(gs *game.GameState, pol [4]Policy, hook PlayHook) (HandOutcome, error) {
	mem := NewMemory()
	for steps := 0; steps < 600; steps++ {
		switch gs.Phase {
		case game.PhaseDealing:
			gs.Phase = game.PhaseBidding
			game.RefreshDerivedFlags(gs)
		case game.PhaseBidding:
			seat := gs.ActivePlayerSeat
			v := BuildView(gs, seat, mem)
			action := pol[seat].Decide(v)
			if action.Type == game.ActionPassTrump && gs.Rules.AllPassOutcome == game.AllPassReshuffleAndRotate &&
				gs.BiddingRound == 2 && gs.BiddingPassCount == 3 {
				return HandOutcome{Status: HandPassedOut}, nil
			}
			cand := gs.TrumpCandidate
			next, err := game.ApplyAction(gs, action)
			if err != nil {
				return HandOutcome{}, fmt.Errorf("bidding seat %d %s: %w", seat, action.Type, err)
			}
			if action.Type == game.ActionPickTrump && cand != nil {
				mem.ObserveCandidate(seat, *cand)
			}
			gs = next
		case game.PhaseDeclaring:
			seat := -1
			for s := range gs.Players {
				if !gs.Players[s].DeclarationAnswered {
					seat = s
					break
				}
			}
			v := BuildView(gs, seat, mem)
			action := pol[seat].Decide(v)
			next, err := game.ApplyAction(gs, action)
			if err != nil {
				return HandOutcome{}, fmt.Errorf("declaring seat %d: %w", seat, err)
			}
			gs = next
			if gs.DeclarationsResolved {
				mem.ObserveDeclarations(gs.Players)
			}
		case game.PhasePlaying:
			seat := gs.ActivePlayerSeat
			if gs.PendingBelotSeat != nil {
				seat = *gs.PendingBelotSeat
			}
			v := BuildView(gs, seat, mem)
			var action game.Action
			if sa, ok := pol[seat].(StateAware); ok {
				action = sa.DecideState(gs, seat, v)
			} else {
				action = pol[seat].Decide(v)
			}
			if action.Type == game.ActionPlayCard && hook != nil {
				hook(gs, seat, action, v)
			}
			oldLead := gs.LeadSuit
			next, err := game.ApplyAction(gs, action)
			if err != nil {
				return HandOutcome{}, fmt.Errorf("playing seat %d %s %v: %w", seat, action.Type, action.Card, err)
			}
			if action.Type == game.ActionPlayCard {
				mem.ObservePlay(seat, *action.Card, oldLead)
				mem.ObserveDiscard(seat, *action.Card, oldLead, gs.TrumpSuit)
			}
			if action.Type == game.ActionAnnounceBelot && next.TrumpSuit != nil {
				mem.ObserveCandidate(seat, game.Card{Rank: game.RankKing, Suit: *next.TrumpSuit})
				mem.ObserveCandidate(seat, game.Card{Rank: game.RankQueen, Suit: *next.TrumpSuit})
			}
			gs = next
			if gs.DeclarationsResolved {
				mem.ObserveDeclarations(gs.Players)
			}
		case game.PhaseHandComplete:
			return HandOutcome{Status: HandScored, Score: *gs.LastHandResult, Caller: *gs.TrumpCallerSeat, Trump: *gs.TrumpSuit}, nil
		case game.PhaseMatchEnd:
			if gs.LastHandResult == nil {
				return HandOutcome{Status: HandInstantWin}, nil
			}
			return HandOutcome{Status: HandScored, Score: *gs.LastHandResult, Caller: *gs.TrumpCallerSeat, Trump: *gs.TrumpSuit}, nil
		default:
			return HandOutcome{}, fmt.Errorf("unexpected phase %s", gs.Phase)
		}
	}
	return HandOutcome{}, fmt.Errorf("hand did not terminate")
}

// Award returns (teamA, teamB) match points banked for the hand.
func Award(o HandOutcome) (int, int) {
	if o.Status != HandScored {
		return 0, 0
	}
	return o.Score.TeamAHandTotal, o.Score.TeamBHandTotal
}
