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
	beloteBonusPoints    = 20
	capotBonusPoints     = 100
	allCards             = uint32(1<<search.NCards - 1)
)

// SearchResult is the card the search chose with the deals behind it, its lead over the runner-up and that lead's
// standard error, or for a pending Belote whether the search answered it with a skip.
type SearchResult struct {
	Card       game.Card
	Samples    int
	Margin     float64
	SE         float64
	Deferred   bool
	BeloteSkip bool
}

// SearchApplies reports whether the Monte Carlo search should pick this card: from the given trick on and only where
// the schedule has samples, on the seat's own turn with a real choice, and near a stop-at-target end only when it races.
func SearchApplies(v View, fromTrick int) bool {
	return SearchAppliesWith(v, fromTrick, DefaultSearchTuning())
}

// SearchAppliesWith is SearchApplies under a search tuning.
func SearchAppliesWith(v View, fromTrick int, t SearchTuning) bool {
	if fromTrick <= 0 || v.Phase != game.PhasePlaying || v.TrumpSuit == nil || v.ActivePlayerSeat != v.Seat {
		return false
	}
	if v.PendingBelot || v.AwaitingDeclaration || v.PartnerProposedSurrender {
		return false
	}
	if len(v.LegalCards) <= 1 || v.TrickNumber < fromTrick || samplesFor(v.TrickNumber, t) == 0 {
		return false
	}
	return t.RaceObjective || !withinStopAtTargetReach(v)
}

func withinStopAtTargetReach(v View) bool {
	if !v.StopAtTarget {
		return false
	}
	reach := remainingCardPoints(v) + lastTrickBonusPoints
	for team := range 2 {
		if v.MatchTarget-runningTotal(v, team) <= reach {
			return true
		}
	}
	return false
}

// runningTotal is what the team would bank if the match stopped now, the bot's own certain Belote included.
func runningTotal(v View, team int) int {
	return bankedTotal(v, team) + ownBelotePoints(v, team)
}

// bankedTotal is the team's running total from what the engine has already credited.
func bankedTotal(v View, team int) int {
	return v.TeamScores[team] + v.HandPoints[team] + v.DeclPoints[team]
}

// raceFor is the race the search plays, else nil: a stop-at-target race when the tuning races and a team is within
// reach, or a deciding-hand race on a table that ends only at a hand's end when the tuning decides and a team could.
func raceFor(v View, t SearchTuning) *search.Race {
	deciding := !v.StopAtTarget && t.DecidingHand && couldDecideThisHand(v)
	if !deciding && (!t.RaceObjective || !withinStopAtTargetReach(v)) {
		return nil
	}
	race := &search.Race{
		Target:       v.MatchTarget,
		Banked:       [2]int{bankedTotal(v, game.TeamA), bankedTotal(v, game.TeamB)},
		Scores:       v.TeamScores,
		Caller:       game.TeamForSeat(*v.TrumpCallerSeat),
		DecidingHand: deciding,
	}
	if ownBeloteCertain(v) {
		race.BeloteSeat, race.BeloteCards = v.Seat, search.MaskOf(belotePair(*v.TrumpSuit))
	}
	return race
}

// couldDecideThisHand is a loose upper bound: some team's match score plus every card point of the hand, a Capot, all
// declarations and a Belote yet to come reaches the target.
func couldDecideThisHand(v View) bool {
	most := v.HandPoints[game.TeamA] + v.HandPoints[game.TeamB] + remainingCardPoints(v) + capotBonusPoints +
		v.DeclPoints[game.TeamA] + v.DeclPoints[game.TeamB] + beloteBonusPoints
	return max(v.TeamScores[game.TeamA], v.TeamScores[game.TeamB])+most >= v.MatchTarget
}

// SearchCard runs the search for the seat's card, reporting false when it reached no decision and the rules should play.
// It is deterministic for a given view.
func SearchCard(v View, deadline time.Time) (SearchResult, bool) {
	return SearchCardWith(v, deadline, DefaultSearchTuning())
}

// SearchCardWith is SearchCard under a search tuning.
func SearchCardWith(v View, deadline time.Time, t SearchTuning) (SearchResult, bool) {
	samples := samplesFor(v.TrickNumber, t)
	if samples == 0 || v.TrumpSuit == nil || v.TrumpCallerSeat == nil {
		return SearchResult{}, false
	}
	res, ok := chooseSearch(v, samples, deadline, t, trumpDrawClass(v, t))
	if !ok {
		return SearchResult{}, false
	}
	card, ok := chooseAdjusted(v, res, t)
	if !ok {
		return SearchResult{}, false
	}
	if deferred, ok := deferTrumpDraw(v, res, card, t); ok {
		return deferred, true
	}
	return SearchResult{Card: card, Samples: res.Samples, Margin: res.Margin, SE: res.SE}, true
}

func chooseSearch(v View, samples int, deadline time.Time, t SearchTuning, class func(hands [4]uint32) int) (search.Result, bool) {
	opts := search.Options{Samples: samples, SeedSalt: t.SeedSalt, EarlyStopAfter: searchEarlyStopAfter, EarlyStopSE: searchEarlyStopSE, SampleClass: class}
	return search.Choose(informationFrom(v), positionFrom(v, t), v.LegalCards, settlementFrom(v), opts, deadline)
}

// DecideWith skips a pending Belote the search answered with a skip and plays the searched card when it is still a legal
// play and nothing else is pending, otherwise it decides by the rules.
func DecideWith(v View, pre *SearchResult) game.Action {
	if pre != nil && pre.BeloteSkip && v.PendingBelot && !v.PartnerProposedSurrender {
		return game.Action{Type: game.ActionSkipBelot, PlayerSeat: v.Seat}
	}
	if pre == nil || !canPlaySearchedCard(v, pre.Card) {
		return Decide(v)
	}
	card := pre.Card
	return game.Action{Type: game.ActionPlayCard, PlayerSeat: v.Seat, Card: &card}
}

// BeloteSearchApplies reports whether the seat's pending Belote is worth a search: its team called trump, declarations
// transfer on a fall and the tuning has a skip margin.
func BeloteSearchApplies(v View, t SearchTuning) bool {
	if !v.PendingBelot || v.Phase != game.PhasePlaying || v.TrumpSuit == nil || !myTeamCalledTrump(v) {
		return false
	}
	return v.DeclarationsTransfer && t.BeloteSkipMargin > 0 && samplesFor(v.TrickNumber, t) > 0
}

// BeloteAnswer answers the seat's pending Belote under the default tuning; ok reports that the search decided it.
func BeloteAnswer(v View, deadline time.Time) (announce bool, ok bool) {
	return BeloteAnswerWith(v, deadline, DefaultSearchTuning())
}

// BeloteAnswerWith skips the caller's Belote only when the hand searched without it beats the hand searched with it by
// more than the skip margin. It announces when the search does not apply, the Belote decides the race or time runs out.
func BeloteAnswerWith(v View, deadline time.Time, t SearchTuning) (announce bool, ok bool) {
	if !BeloteSearchApplies(v, t) {
		return true, false
	}
	announced := withOwnBelote(v)
	if beloteDecidesTheRace(announced, t) {
		return true, false
	}
	opts := search.Options{Samples: samplesFor(v.TrickNumber, t), SeedSalt: t.SeedSalt}
	in := informationFrom(v)
	withBelote, _, _, ok := search.Evaluate(in, positionFrom(announced, t), settlementFrom(announced), opts, deadline)
	if !ok {
		return true, false
	}
	withoutBelote, _, _, ok := search.Evaluate(in, positionFrom(v, t), settlementFrom(v), opts, deadline)
	if !ok {
		return true, false
	}
	teamSign := float64(1 - 2*game.TeamForSeat(v.Seat))
	gain := teamSign * (withoutBelote - withBelote)
	return gain <= float64(t.BeloteSkipMargin), true
}

// withOwnBelote is the view after the seat announces: its team's declarations carry the Belote's 20.
func withOwnBelote(v View) View {
	v.DeclPoints[game.TeamForSeat(v.Seat)] += beloteBonusPoints
	return v
}

// beloteDecidesTheRace reports whether the announced Belote alone ends a stop-at-target match, whatever the tuning races.
func beloteDecidesTheRace(announced View, t SearchTuning) bool {
	t.RaceObjective = true
	race := raceFor(announced, t)
	if race == nil || race.DecidingHand {
		return false
	}
	_, decided := search.RaceDecided(*race, [2]int{})
	return decided
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
	mask |= cardsAboveBounds(v, seat)
	for c := range cardsProvablyAbsent(v, seat) {
		mask |= 1 << uint(search.CardIdx(c))
	}
	return mask
}

func cardsAboveBounds(v View, seat int) uint32 {
	if v.TrumpSuit == nil {
		return 0
	}
	var mask uint32
	for _, suit := range game.AllSuits {
		bound := v.RankBounds[seat][SuitIndex(suit)]
		if bound < 0 {
			continue
		}
		for _, rank := range game.AllRanks {
			c := game.Card{Rank: rank, Suit: suit}
			if strengthOf(c, *v.TrumpSuit) > bound {
				mask |= 1 << uint(search.CardIdx(c))
			}
		}
	}
	return mask
}

// cardsHeldBy counts from the seat's own hand, which is one card short once the seat has played to the trick.
func cardsHeldBy(v View, seat int) int {
	held := len(v.Hand)
	if playedToTrick(v, v.Seat) {
		held++
	}
	if playedToTrick(v, seat) {
		held--
	}
	return held
}

func playedToTrick(v View, seat int) bool {
	return slices.ContainsFunc(v.CurrentTrick, func(tc game.TrickCard) bool { return tc.PlayerSeat == seat })
}

func trickMask(v View) uint32 {
	var mask uint32
	for _, tc := range v.CurrentTrick {
		mask |= 1 << uint(search.CardIdx(tc.Card))
	}
	return mask
}

// positionFrom needs a trump suit and its caller. The trick lists its cards in play order, led by the first card's seat.
func positionFrom(v View, t SearchTuning) search.Position {
	pos := search.Position{
		Trump:     search.SuitIdx(*v.TrumpSuit),
		Clockwise: v.Clockwise,
		Leader:    v.Seat,
		Trick:     make([]int, len(v.CurrentTrick)),
		Capot:     search.CapotFlag(v.TricksWon),
		Race:      raceFor(v, t),
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
		DeclPoints:           declPointsWithOwnBelote(v),
		TricksWon:            v.TricksWon,
		CallerTeam:           game.TeamForSeat(*v.TrumpCallerSeat),
		DeclarationsTransfer: v.DeclarationsTransfer,
		RemainingCardPoints:  remainingCardPoints(v),
	}
}

func declPointsWithOwnBelote(v View) [2]int {
	points := v.DeclPoints
	for team := range points {
		points[team] += ownBelotePoints(v, team)
	}
	return points
}

func ownBelotePoints(v View, team int) int {
	if team != game.TeamForSeat(v.Seat) || !ownBeloteCertain(v) {
		return 0
	}
	return beloteBonusPoints
}

// ownBeloteCertain reports that the seat will score Belote: declarations are on and its hand still holds the trump King and Queen.
func ownBeloteCertain(v View) bool {
	if !v.DeclarationsEnabled || v.TrumpSuit == nil {
		return false
	}
	pair := belotePair(*v.TrumpSuit)
	return slices.Contains(v.Hand, pair[0]) && slices.Contains(v.Hand, pair[1])
}

func belotePair(trump game.Suit) []game.Card {
	return []game.Card{{Rank: game.RankKing, Suit: trump}, {Rank: game.RankQueen, Suit: trump}}
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
