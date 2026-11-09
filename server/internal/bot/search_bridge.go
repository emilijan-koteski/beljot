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

// chooseSearch runs the chooser, sampling only deals that fit the auction when the tuning asks, and once more without
// that filter when no such deal was found.
func chooseSearch(v View, samples int, deadline time.Time, t SearchTuning, class func(hands [4]uint32) int) (search.Result, bool) {
	opts := search.Options{Samples: samples, SeedSalt: t.SeedSalt, EarlyStopAfter: searchEarlyStopAfter, EarlyStopSE: searchEarlyStopSE, SampleClass: class}
	in, pos, settle := informationFrom(v), positionFrom(v, t), settlementFrom(v)
	filtered := filteredOptions(v, opts, t)
	if filtered.SampleFilter != nil {
		if res, ok := search.Choose(in, pos, v.LegalCards, settle, filtered, deadline); ok {
			return res, true
		}
	}
	return search.Choose(in, pos, v.LegalCards, settle, opts, deadline)
}

// evaluateSearch is search.Evaluate's mean under the same auction filter and fallback as chooseSearch.
func evaluateSearch(v View, in search.Information, pos search.Position, s search.Settlement, opts search.Options,
	deadline time.Time, t SearchTuning) (float64, bool) {
	filtered := filteredOptions(v, opts, t)
	if filtered.SampleFilter != nil {
		if mean, _, _, ok := search.Evaluate(in, pos, s, filtered, deadline); ok {
			return mean, true
		}
	}
	mean, _, _, ok := search.Evaluate(in, pos, s, opts, deadline)
	return mean, ok
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
	withBelote, ok := evaluateSearch(v, in, positionFrom(announced, t), settlementFrom(announced), opts, deadline, t)
	if !ok {
		return true, false
	}
	withoutBelote, ok := evaluateSearch(v, in, positionFrom(v, t), settlementFrom(v), opts, deadline, t)
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

// cardsHeldBy counts from the bot's own hand, which is one card short once the bot has played to the trick.
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
