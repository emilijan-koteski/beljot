package bot

import (
	"slices"

	"github.com/emilijan/beljot/server/internal/game"
)

// Decide maps a redacted View to the bot's next action, deterministically so every branch is table-
// testable. Non-playing phases must return before chooseCard, since LegalCards is empty there.
func Decide(v View) game.Action {
	if v.PartnerProposedSurrender {
		return game.Action{Type: game.ActionSurrenderAccept, PlayerSeat: v.Seat}
	}

	if v.Phase == game.PhaseBidding {
		return decideBid(v)
	}

	if v.Phase == game.PhaseDeclaring {
		return decideDeclaration(v)
	}

	if v.PendingBelot {
		return game.Action{Type: game.ActionAnnounceBelot, PlayerSeat: v.Seat}
	}

	if v.AwaitingDeclaration {
		return game.Action{Type: game.ActionDeclare, PlayerSeat: v.Seat}
	}

	card := chooseCard(v)
	return game.Action{Type: game.ActionPlayCard, PlayerSeat: v.Seat, Card: &card}
}

// decideDeclaration declares only when the seat holds a meld, otherwise skips. The engine rejects a
// declare from a meldless hand, and the bot timer would re-arm forever.
func decideDeclaration(v View) game.Action {
	if v.HasDeclarations {
		return game.Action{Type: game.ActionDeclare, PlayerSeat: v.Seat}
	}
	return game.Action{Type: game.ActionSkipDeclare, PlayerSeat: v.Seat}
}

func chooseCard(v View) game.Card {
	legal := v.LegalCards
	if len(legal) == 1 {
		return legal[0]
	}
	if v.TrumpSuit != nil {
		if c := retainLastTrickWinner(v, legal, *v.TrumpSuit); c != nil {
			return *c
		}
	}
	if len(v.CurrentTrick) == 0 {
		return chooseLead(v, legal)
	}
	return chooseFollow(v, legal)
}

// retainLastTrickWinner, holding the master trump with two cards left, returns the card to spend
// now so a guaranteed winner remains for trick 8 (+10). Returns nil to defer to the normal
// heuristics.
func retainLastTrickWinner(v View, legal []game.Card, trump game.Suit) *game.Card {
	if len(v.Hand) != 2 {
		return nil
	}
	master := highestOfSuit(v.Hand, trump, game.TrumpRankOrder)
	if master == nil || !holdsMasterTrump(v, trump) {
		return nil
	}
	var spare *game.Card
	for i := range v.Hand {
		if v.Hand[i] != *master {
			spare = &v.Hand[i]
			break
		}
	}
	if spare == nil {
		return nil
	}
	partner := (v.Seat + 2) % 4
	for _, pc := range knownHeldBy(v, partner) {
		if pc.Suit == trump && game.TrumpRankOrder[pc.Rank] > game.TrumpRankOrder[master.Rank] {
			return nil
		}
	}
	if isUncuttableBoss(v, *spare, trump) && slices.Contains(legal, *master) {
		return master
	}
	if !slices.Contains(legal, *spare) {
		return nil
	}
	return spare
}

// isUncuttableBoss reports whether c is a non-trump suit boss that no opponent can ruff.
func isUncuttableBoss(v View, c game.Card, trump game.Suit) bool {
	return c.Suit != trump && isSuitBoss(c, v) && opponentsOutOfTrump(v, trump)
}

// opponentsOutOfTrump reports whether neither opponent could still play a trump. Unlike
// trumpsRemainUnseen, it does not count the partner's unknown trumps as outstanding.
func opponentsOutOfTrump(v View, trump game.Suit) bool {
	for _, seat := range []int{(v.Seat + 1) % 4, (v.Seat + 3) % 4} {
		if opponentMayHoldTrump(v, seat, trump) {
			return false
		}
	}
	return true
}

// chooseLead picks the card to open a trick: draw trumps while our team called trump, trumps remain
// unseen, and we still hold the master trump; otherwise bank a side-suit boss (Ace first);
// otherwise lead the safest low card.
func chooseLead(v View, legal []game.Card) game.Card {
	if v.TrumpSuit == nil {
		return legal[0]
	}
	trump := *v.TrumpSuit

	if myTeamCalledTrump(v) && trumpsRemainUnseen(v, trump) && holdsMasterTrump(v, trump) &&
		!opponentsOutOfTrump(v, trump) {
		if c := highestOfSuit(legal, trump, game.TrumpRankOrder); c != nil {
			return *c
		}
	}

	if myTeamCalledTrump(v) && trumpsRemainUnseen(v, trump) && !holdsMasterTrump(v, trump) &&
		!opponentsOutOfTrump(v, trump) {
		partner := (v.Seat + 2) % 4
		botTop := highestOfSuit(legal, trump, game.TrumpRankOrder)
		if botTop != nil &&
			v.TrumpCallerSeat != nil && *v.TrumpCallerSeat == partner &&
			!partnerVoidInTrump(v, partner, trump) &&
			!opponentKnownHoldsTrumpAbove(v, *botTop, trump) &&
			!opponentHoldsMasterTrump(v, trump) {
			if c := partnerDrawTrump(legal, trump); c != nil {
				return *c
			}
		}
	}

	var boss *game.Card
	for i := range legal {
		c := legal[i]
		if c.Suit == trump || !isSuitBoss(c, v) {
			continue
		}
		if boss == nil || game.NonTrumpCardPoints[c.Rank] > game.NonTrumpCardPoints[boss.Rank] {
			boss = &legal[i]
		}
	}
	if boss != nil {
		if boss.Rank == game.RankAce {
			if ten := findRankOfSuit(legal, boss.Suit, game.RankTen); ten != nil {
				return *ten
			}
		}
		return *boss
	}

	if c := leadIntoPartnerVoid(v, legal, trump); c != nil {
		return *c
	}

	var nonTrump []game.Card
	for _, c := range legal {
		if c.Suit != trump {
			nonTrump = append(nonTrump, c)
		}
	}
	if len(nonTrump) > 0 {
		return lowestValue(nonTrump, trump)
	}

	if c := highestOfSuit(legal, trump, game.TrumpRankOrder); c != nil {
		if isTrumpMaster(v, *c, trump) {
			return *c
		}
		return lowestValue(legal, trump)
	}
	return legal[0]
}

// chooseFollow picks the card when the trick already has cards: smear points onto a partner whose
// win is safe, win as cheaply as possible over an opponent, otherwise discard the lowest value
// while preserving trump.
func chooseFollow(v View, legal []game.Card) game.Card {
	if v.TrumpSuit == nil {
		return legal[0]
	}
	trump := *v.TrumpSuit
	winnerSeat, _ := trickWinner(v.CurrentTrick, trump)

	if game.TeamForSeat(winnerSeat) == game.TeamForSeat(v.Seat) {
		if partnerWinIsSafe(v, trump) {
			if c := bestSmear(v, legal, trump); c != nil {
				return *c
			}
			return strongestPreservingBoss(v, legal, trump)
		}
		if shouldSmearOntoPartnerBoss(v, trump) {
			if c := bestSmear(v, legal, trump); c != nil {
				return *c
			}
		}
		duck := lowestValue(legal, trump)
		if highestPointNonOvertaking(v, legal, trump) == nil &&
			trickPoints(v, trump)+cardPoints(duck, trump) > 0 {
			if c := trumpEconomyTake(v, legal, trump); c != nil {
				return *c
			}
		}
		return duck
	}

	if partnerTakesTrick(v, trump) {
		if c := bestSmear(v, legal, trump); c != nil {
			return *c
		}
		return strongestPreservingBoss(v, legal, trump)
	}

	if len(v.CurrentTrick) == 3 {
		led := v.CurrentTrick[0].Card.Suit
		if led != trump {
			if c := highestPointsLedSuitWinner(v, legal, led, trump); c != nil {
				return *c
			}
		}
	}

	if cheap := cheapestWinning(v, legal, trump); cheap != nil {
		if trickPoints(v, trump)+cardPoints(*cheap, trump) > 0 {
			if c := trumpEconomyTake(v, legal, trump); c != nil {
				return *c
			}
		}
		if c := cheapestLedSecureWinner(v, legal, trump); c != nil {
			return *c
		}
		return *cheap
	}
	return lowestValue(legal, trump)
}

// partnerTakesTrick reports whether the partner, playing last, is guaranteed to take the trick with
// a known threat-proof trump. On a non-trump lead the partner must be provably void in the led
// suit.
func partnerTakesTrick(v View, trump game.Suit) bool {
	if len(v.CurrentTrick) == 0 {
		return false
	}
	partner := (v.Seat + 2) % 4
	if !slices.Contains(seatsYetToPlay(v), partner) {
		return false
	}
	led := v.CurrentTrick[0].Card.Suit
	if led != trump {
		ledIdx := SuitIndex(led)
		if ledIdx < 0 || !v.KnownVoids[partner][ledIdx] {
			return false
		}
	}
	_, winning := trickWinner(v.CurrentTrick, trump)
	for _, pc := range knownHeldBy(v, partner) {
		if pc.Suit != trump || !beatsCard(pc, winning, led, trump) {
			continue
		}
		safe := true
		for _, t := range threats(v) {
			if beatsCard(t, pc, led, trump) {
				safe = false
				break
			}
		}
		if safe {
			return true
		}
	}
	return false
}

func myTeamCalledTrump(v View) bool {
	return v.TrumpCallerSeat != nil &&
		game.TeamForSeat(*v.TrumpCallerSeat) == game.TeamForSeat(v.Seat)
}

// trumpsRemainUnseen reports whether any trump could still be in an opponent's hand. It scans
// threats, so the partner's declared trumps do not count.
func trumpsRemainUnseen(v View, trump game.Suit) bool {
	for _, c := range threats(v) {
		if c.Suit == trump {
			return true
		}
	}
	return false
}

// unseenCards returns the cards in unknown hands: the deck minus played cards, the bot's hand, the
// current trick, and cards placed by the declaration reveal.
func unseenCards(v View) []game.Card {
	known := make(map[game.Card]bool, 32)
	for _, c := range v.PlayedCards {
		known[c] = true
	}
	for _, c := range v.Hand {
		known[c] = true
	}
	for _, tc := range v.CurrentTrick {
		known[tc.Card] = true
	}
	for seat := range 4 {
		for _, c := range knownHeldBy(v, seat) {
			known[c] = true
		}
	}
	out := make([]game.Card, 0, 32-len(known))
	for _, c := range game.NewDeck() {
		if !known[c] {
			out = append(out, c)
		}
	}
	return out
}

// knownHeldBy returns the publicly known cards (revealed or located) seat still holds, excluding
// any already played.
func knownHeldBy(v View, seat int) []game.Card {
	if seat < 0 || seat > 3 || len(v.KnownCards[seat]) == 0 {
		return nil
	}
	gone := make(map[game.Card]bool, len(v.PlayedCards)+len(v.CurrentTrick))
	for _, c := range v.PlayedCards {
		gone[c] = true
	}
	for _, tc := range v.CurrentTrick {
		gone[tc.Card] = true
	}
	out := make([]game.Card, 0, len(v.KnownCards[seat]))
	for _, c := range v.KnownCards[seat] {
		if !gone[c] {
			out = append(out, c)
		}
	}
	return out
}

// threats returns the cards an opponent could still play: unseen cards plus cards an opponent is
// known to hold. A partner's known card is never a threat.
func threats(v View) []game.Card {
	out := unseenCards(v)
	for seat := range 4 {
		if game.TeamForSeat(seat) == game.TeamForSeat(v.Seat) {
			continue
		}
		out = append(out, knownHeldBy(v, seat)...)
	}
	return out
}

// holdsMasterTrump reports whether no trump an opponent could play outranks the bot's best trump.
// Returns false when the bot holds no trump.
func holdsMasterTrump(v View, trump game.Suit) bool {
	top := highestOfSuit(v.Hand, trump, game.TrumpRankOrder)
	if top == nil {
		return false
	}
	for _, u := range threats(v) {
		if u.Suit == trump && game.TrumpRankOrder[u.Rank] > game.TrumpRankOrder[top.Rank] {
			return false
		}
	}
	return true
}

// partnerVoidInTrump reports whether the partner is known void in trump.
func partnerVoidInTrump(v View, partner int, trump game.Suit) bool {
	idx := SuitIndex(trump)
	return idx >= 0 && partner >= 0 && partner < 4 && v.KnownVoids[partner][idx]
}

// opponentKnownHoldsTrumpAbove reports whether an opponent is known from the reveal to hold a trump
// above ref.
func opponentKnownHoldsTrumpAbove(v View, ref game.Card, trump game.Suit) bool {
	for seat := range 4 {
		if game.TeamForSeat(seat) == game.TeamForSeat(v.Seat) {
			continue
		}
		for _, c := range knownHeldBy(v, seat) {
			if c.Suit == trump && game.TrumpRankOrder[c.Rank] > game.TrumpRankOrder[ref.Rank] {
				return true
			}
		}
	}
	return false
}

// cardsProvablyAbsent returns the cards just outside each run in a seat's raw reveal, in every suit.
// A declared run is maximal, so those cards cannot be in that seat's hand.
func cardsProvablyAbsent(v View, seat int) map[game.Card]bool {
	absent := map[game.Card]bool{}
	if seat < 0 || seat > 3 {
		return absent
	}
	for _, suit := range game.AllSuits {
		var present [8]bool
		for _, c := range v.DeclaredCards[seat] {
			if c.Suit == suit {
				present[game.NaturalRankOrder[c.Rank]] = true
			}
		}
		markRunNeighboursAbsent(absent, suit, present)
	}
	return absent
}

// markRunNeighboursAbsent adds the rank below and above every run of three or more present ranks.
func markRunNeighboursAbsent(absent map[game.Card]bool, suit game.Suit, present [8]bool) {
	for i := 0; i < 8; {
		if !present[i] {
			i++
			continue
		}
		j := i
		for j+1 < 8 && present[j+1] {
			j++
		}
		if j-i+1 >= 3 {
			if i-1 >= 0 {
				absent[game.Card{Suit: suit, Rank: game.NaturalRankSequence[i-1]}] = true
			}
			if j+1 < 8 {
				absent[game.Card{Suit: suit, Rank: game.NaturalRankSequence[j+1]}] = true
			}
		}
		i = j + 1
	}
}

// opponentHoldsMasterTrump reports whether the top outstanding trump is provably an opponent's.
// Valid only at an empty trick, since cards in the current trick are not counted as gone.
func opponentHoldsMasterTrump(v View, trump game.Suit) bool {
	gone := make(map[game.Card]bool, len(v.PlayedCards)+len(v.Hand))
	for _, c := range v.PlayedCards {
		gone[c] = true
	}
	for _, c := range v.Hand {
		gone[c] = true
	}
	var top *game.Card
	bestOrder := -1
	for _, r := range game.NaturalRankSequence {
		c := game.Card{Suit: trump, Rank: r}
		if gone[c] {
			continue
		}
		if game.TrumpRankOrder[r] > bestOrder {
			bestOrder = game.TrumpRankOrder[r]
			cc := c
			top = &cc
		}
	}
	if top == nil {
		return false
	}
	partner := (v.Seat + 2) % 4
	for _, pc := range knownHeldBy(v, partner) {
		if pc == *top {
			return false
		}
	}
	for seat := 0; seat < 4; seat++ {
		if game.TeamForSeat(seat) == game.TeamForSeat(v.Seat) {
			continue
		}
		for _, oc := range knownHeldBy(v, seat) {
			if oc == *top {
				return true
			}
		}
	}
	return cardsProvablyAbsent(v, partner)[*top]
}

// partnerDrawTrump picks the trump to lead for the partner: the weakest honor (Q, K, T, A), else
// the lowest non-9 trump. Returns nil when only the 9 is left, keeping it back as a near-master.
func partnerDrawTrump(legal []game.Card, trump game.Suit) *game.Card {
	for _, r := range []game.Rank{game.RankQueen, game.RankKing, game.RankTen, game.RankAce} {
		for i := range legal {
			if legal[i].Suit == trump && legal[i].Rank == r {
				return &legal[i]
			}
		}
	}
	var low *game.Card
	for i := range legal {
		c := legal[i]
		if c.Suit != trump || c.Rank == game.Rank9 {
			continue
		}
		if low == nil || game.TrumpRankOrder[c.Rank] < game.TrumpRankOrder[low.Rank] {
			low = &legal[i]
		}
	}
	return low
}

// leadIntoPartnerVoid returns the cheapest card of a side suit the partner is void in, so the
// partner can ruff. Returns nil when no such suit is held or the partner is void in trump.
func leadIntoPartnerVoid(v View, legal []game.Card, trump game.Suit) *game.Card {
	partner := (v.Seat + 2) % 4
	if partnerVoidInTrump(v, partner, trump) {
		return nil
	}
	var best *game.Card
	for _, s := range game.AllSuits {
		if s == trump {
			continue
		}
		idx := SuitIndex(s)
		if idx < 0 || !v.KnownVoids[partner][idx] {
			continue
		}
		suitCards := cardsOfSuit(legal, s)
		if len(suitCards) == 0 {
			continue
		}
		low := lowestValue(suitCards, trump)
		if best == nil || cardPoints(low, trump) < cardPoints(*best, trump) {
			lc := low
			best = &lc
		}
	}
	return best
}

// findRankOfSuit returns a pointer to the card of the given suit and rank in cards, or nil when it
// is not present.
func findRankOfSuit(cards []game.Card, suit game.Suit, rank game.Rank) *game.Card {
	for i := range cards {
		if cards[i].Suit == suit && cards[i].Rank == rank {
			return &cards[i]
		}
	}
	return nil
}

// cardsOfSuit returns the cards of the given suit.
func cardsOfSuit(cards []game.Card, s game.Suit) []game.Card {
	var out []game.Card
	for _, c := range cards {
		if c.Suit == s {
			out = append(out, c)
		}
	}
	return out
}

// isSuitBoss reports whether no card of c's suit that an opponent could play outranks c.
func isSuitBoss(c game.Card, v View) bool {
	for _, u := range threats(v) {
		if u.Suit == c.Suit && game.NonTrumpRankOrder[u.Rank] > game.NonTrumpRankOrder[c.Rank] {
			return false
		}
	}
	return true
}

// trickWinner returns the seat and card currently winning the trick.
func trickWinner(trick []game.TrickCard, trump game.Suit) (int, game.Card) {
	led := trick[0].Card.Suit
	bestSeat := trick[0].PlayerSeat
	bestCard := trick[0].Card
	for _, tc := range trick[1:] {
		if beatsCard(tc.Card, bestCard, led, trump) {
			bestSeat = tc.PlayerSeat
			bestCard = tc.Card
		}
	}
	return bestSeat, bestCard
}

// beatsCard reports whether candidate beats the current winner, given the led and trump suits.
// Mirrors the engine's trick-resolution ordering.
func beatsCard(candidate, winner game.Card, led, trump game.Suit) bool {
	cTrump := candidate.Suit == trump
	wTrump := winner.Suit == trump
	switch {
	case cTrump && !wTrump:
		return true
	case cTrump && wTrump:
		return game.TrumpRankOrder[candidate.Rank] > game.TrumpRankOrder[winner.Rank]
	case wTrump:
		return false
	default:
		return candidate.Suit == led &&
			game.NonTrumpRankOrder[candidate.Rank] > game.NonTrumpRankOrder[winner.Rank]
	}
}

// seatsYetToPlay lists the seats that act after this bot in the current trick.
func seatsYetToPlay(v View) []int {
	leader := v.CurrentTrick[0].PlayerSeat
	out := make([]int, 0, 3)
	for pos := len(v.CurrentTrick) + 1; pos < 4; pos++ {
		out = append(out, (leader+pos)%4)
	}
	return out
}

// partnerWinIsSafe reports whether no opponent can still beat the partner's winning card. Higher
// led-suit cards are ignored when every opponent left to play is void in the led suit.
func partnerWinIsSafe(v View, trump game.Suit) bool {
	if len(v.CurrentTrick) == 3 {
		return true
	}
	_, winning := trickWinner(v.CurrentTrick, trump)
	led := v.CurrentTrick[0].Card.Suit

	higherLedUnseen := false
	trumpThreatUnseen := false
	for _, u := range threats(v) {
		if !beatsCard(u, winning, led, trump) {
			continue
		}
		if u.Suit == trump {
			trumpThreatUnseen = true
		} else {
			higherLedUnseen = true
		}
	}
	if trumpThreatUnseen {
		return false
	}
	if !higherLedUnseen {
		return true
	}
	ledIdx := SuitIndex(led)
	for _, seat := range seatsYetToPlay(v) {
		if game.TeamForSeat(seat) == game.TeamForSeat(v.Seat) {
			continue
		}
		if ledIdx < 0 || !v.KnownVoids[seat][ledIdx] {
			return false
		}
	}
	return true
}

// opponentMayHoldTrump reports whether the opponent at seat could still play a trump and therefore
// ruff.
func opponentMayHoldTrump(v View, seat int, trump game.Suit) bool {
	for _, c := range knownHeldBy(v, seat) {
		if c.Suit == trump {
			return true
		}
	}
	if tIdx := SuitIndex(trump); tIdx >= 0 && v.KnownVoids[seat][tIdx] {
		return false
	}
	for _, c := range unseenCards(v) {
		if c.Suit == trump {
			return true
		}
	}
	return false
}

// shouldSmearOntoPartnerBoss reports whether the partner's winning card tops a non-trump led suit
// and no opponent left to play is known able to ruff it.
func shouldSmearOntoPartnerBoss(v View, trump game.Suit) bool {
	if len(v.CurrentTrick) == 0 {
		return false
	}
	led := v.CurrentTrick[0].Card.Suit
	if led == trump {
		return false
	}
	_, winning := trickWinner(v.CurrentTrick, trump)
	if winning.Suit != led {
		return false
	}
	for _, u := range threats(v) {
		if u.Suit == led && beatsCard(u, winning, led, trump) {
			return false
		}
	}
	ledIdx := SuitIndex(led)
	for _, seat := range seatsYetToPlay(v) {
		if game.TeamForSeat(seat) == game.TeamForSeat(v.Seat) {
			continue
		}
		if ledIdx >= 0 && v.KnownVoids[seat][ledIdx] && opponentMayHoldTrump(v, seat, trump) {
			return false
		}
	}
	return true
}

func cardPoints(c game.Card, trump game.Suit) int {
	if c.Suit == trump {
		return game.TrumpCardPoints[c.Rank]
	}
	return game.NonTrumpCardPoints[c.Rank]
}

// wouldOvertake reports whether playing c makes this seat the trick's new winner.
func wouldOvertake(c game.Card, trick []game.TrickCard, trump game.Suit) bool {
	_, winning := trickWinner(trick, trump)
	return beatsCard(c, winning, trick[0].Card.Suit, trump)
}

// highestPointNonOvertaking returns the highest-point legal card that leaves the current winner in
// place, or nil when every legal card overtakes.
func highestPointNonOvertaking(v View, legal []game.Card, trump game.Suit) *game.Card {
	var best *game.Card
	bestPts := -1
	for i := range legal {
		c := legal[i]
		if wouldOvertake(c, v.CurrentTrick, trump) {
			continue
		}
		if pts := cardPoints(c, trump); pts > bestPts {
			bestPts = pts
			best = &legal[i]
		}
	}
	return best
}

// cheapestWinning returns the lowest-point (then weakest) legal card that takes the trick, or nil
// when none can.
func cheapestWinning(v View, legal []game.Card, trump game.Suit) *game.Card {
	var best *game.Card
	bestPts := -1
	bestStrength := -1
	for i := range legal {
		c := legal[i]
		if !wouldOvertake(c, v.CurrentTrick, trump) {
			continue
		}
		pts := cardPoints(c, trump)
		strength := cardStrengthOf(c, trump)
		if best == nil || pts < bestPts || (pts == bestPts && strength < bestStrength) {
			best = &legal[i]
			bestPts = pts
			bestStrength = strength
		}
	}
	return best
}

// cheapestLedSecureWinner returns the lowest-point (then weakest) legal led-suit card that takes
// the trick and that no led-suit threat outranks, or nil. A ruff beats any led-suit card, so
// unseen trumps are no argument for a higher one.
func cheapestLedSecureWinner(v View, legal []game.Card, trump game.Suit) *game.Card {
	led := v.CurrentTrick[0].Card.Suit
	if led == trump {
		return nil
	}
	ledThreats := threats(v)
	var best *game.Card
	bestPts := -1
	bestStrength := -1
	for i := range legal {
		c := legal[i]
		if c.Suit != led || !wouldOvertake(c, v.CurrentTrick, trump) {
			continue
		}
		outranked := slices.ContainsFunc(ledThreats, func(t game.Card) bool {
			return t.Suit == led && game.NonTrumpRankOrder[t.Rank] > game.NonTrumpRankOrder[c.Rank]
		})
		if outranked {
			continue
		}
		pts := cardPoints(c, trump)
		strength := cardStrengthOf(c, trump)
		if best == nil || pts < bestPts || (pts == bestPts && strength < bestStrength) {
			best = &legal[i]
			bestPts = pts
			bestStrength = strength
		}
	}
	return best
}

// securelyWins reports whether c overtakes the current winner and no opponent left to play can beat
// it. Per seat it skips suits the seat is void in and, when the seat must follow, its off-suit
// cards.
func securelyWins(v View, c game.Card, trump game.Suit) bool {
	if len(v.CurrentTrick) == 0 {
		return false
	}
	if !wouldOvertake(c, v.CurrentTrick, trump) {
		return false
	}
	if len(v.CurrentTrick) == 3 {
		return true
	}
	led := v.CurrentTrick[0].Card.Suit
	unseen := unseenCards(v)
	for _, seat := range seatsYetToPlay(v) {
		if game.TeamForSeat(seat) == game.TeamForSeat(v.Seat) {
			continue
		}
		mustFollow := seatKnownHoldsSuit(v, seat, led)
		for _, t := range knownHeldBy(v, seat) {
			if mustFollow && t.Suit != led {
				continue
			}
			if beatsCard(t, c, led, trump) {
				return false
			}
		}
		for _, t := range unseen {
			if mustFollow && t.Suit != led {
				continue
			}
			if idx := SuitIndex(t.Suit); idx >= 0 && v.KnownVoids[seat][idx] {
				continue
			}
			if beatsCard(t, c, led, trump) {
				return false
			}
		}
	}
	return true
}

// seatKnownHoldsSuit reports whether the reveal proves seat still holds a card of the given suit —
// and is therefore forced to follow while that suit is led.
func seatKnownHoldsSuit(v View, seat int, suit game.Suit) bool {
	for _, c := range knownHeldBy(v, seat) {
		if c.Suit == suit {
			return true
		}
	}
	return false
}

// cheapestSecureWinning returns the lowest-point (then weakest) legal card that securelyWins, or
// nil.
func cheapestSecureWinning(v View, legal []game.Card, trump game.Suit) *game.Card {
	var best *game.Card
	bestPts := -1
	bestStrength := -1
	for i := range legal {
		c := legal[i]
		if !securelyWins(v, c, trump) {
			continue
		}
		pts := cardPoints(c, trump)
		strength := cardStrengthOf(c, trump)
		if best == nil || pts < bestPts || (pts == bestPts && strength < bestStrength) {
			best = &legal[i]
			bestPts = pts
			bestStrength = strength
		}
	}
	return best
}

// isControlTrump reports whether c is one of the two "control" trumps — the Jack or the 9 — kept
// back rather than spent merely to secure a low-value trick.
func isControlTrump(c game.Card, trump game.Suit) bool {
	return c.Suit == trump && (c.Rank == game.RankJack || c.Rank == game.Rank9)
}

// controlTrumpValue is the trump point value of a control trump (J=20, 9=14): the minimum trick pot
// worth spending it to guarantee a take.
func controlTrumpValue(c game.Card) int {
	return game.TrumpCardPoints[c.Rank]
}

// knownSafeRuff reports whether trump c, ruffing a non-trump lead, beats every over-ruff pinned to
// an opponent left to play. Unseen trumps only count for a seat known void in the led suit.
func knownSafeRuff(v View, c game.Card, trump game.Suit) bool {
	led := v.CurrentTrick[0].Card.Suit
	ledIdx := SuitIndex(led)
	for _, seat := range seatsYetToPlay(v) {
		if game.TeamForSeat(seat) == game.TeamForSeat(v.Seat) {
			continue
		}
		for _, t := range knownHeldBy(v, seat) {
			if beatsCard(t, c, led, trump) {
				return false
			}
		}
		voidInLed := ledIdx >= 0 && v.KnownVoids[seat][ledIdx]
		if voidInLed && opponentMayHoldTrump(v, seat, trump) {
			for _, t := range unseenCards(v) {
				if t.Suit == trump && beatsCard(t, c, led, trump) {
					return false
				}
			}
		}
	}
	return true
}

// highestExpendableKnownSafeRuff returns the highest-point non-control trump that overtakes and is
// a knownSafeRuff, or nil.
func highestExpendableKnownSafeRuff(v View, legal []game.Card, trump game.Suit) *game.Card {
	var best *game.Card
	bestPts := -1
	for i := range legal {
		c := legal[i]
		if c.Suit != trump || isControlTrump(c, trump) {
			continue
		}
		if !wouldOvertake(c, v.CurrentTrick, trump) {
			continue
		}
		if !knownSafeRuff(v, c, trump) {
			continue
		}
		if pts := game.TrumpCardPoints[c.Rank]; pts > bestPts {
			bestPts = pts
			best = &legal[i]
		}
	}
	return best
}

// trumpEconomyTake returns a card that takes the trick while preserving the control trumps (J, 9),
// or nil. A J/9 is spent only on a trump lead with points on the table, or when the pot is worth
// its value.
func trumpEconomyTake(v View, legal []game.Card, trump game.Suit) *game.Card {
	secure := cheapestSecureWinning(v, legal, trump)
	if secure != nil {
		if !isControlTrump(*secure, trump) {
			return secure
		}
		if v.CurrentTrick[0].Card.Suit == trump && trickPoints(v, trump) > 0 {
			return secure
		}
	}
	if v.CurrentTrick[0].Card.Suit != trump {
		if preserve := highestExpendableKnownSafeRuff(v, legal, trump); preserve != nil {
			if secure == nil || trickPoints(v, trump) < controlTrumpValue(*secure) {
				return preserve
			}
		}
	}
	if secure != nil && trickPoints(v, trump) >= controlTrumpValue(*secure) {
		return secure
	}
	return nil
}

// trickPoints sums the card points already lying in the current trick.
func trickPoints(v View, trump game.Suit) int {
	pts := 0
	for _, tc := range v.CurrentTrick {
		pts += cardPoints(tc.Card, trump)
	}
	return pts
}

// bestSmear returns the highest-point non-overtaking card, skipping an unbacked boss worth
// guarding. Falls back to highestPointNonOvertaking when only such bosses remain, and returns nil
// when every card overtakes.
func bestSmear(v View, legal []game.Card, trump game.Suit) *game.Card {
	if len(v.CurrentTrick) == 0 {
		return nil
	}
	var best *game.Card
	bestPts := -1
	bestStrength := -1
	hasCandidate := false
	for i := range legal {
		c := legal[i]
		if wouldOvertake(c, v.CurrentTrick, trump) {
			continue
		}
		hasCandidate = true
		if cardIsBoss(v, c, trump) && !heldSameSuitBoss(v, c, trump) &&
			bossWorthGuarding(v, c, trump) {
			continue
		}
		pts := cardPoints(c, trump)
		strength := cardStrengthOf(c, trump)
		if pts > bestPts || (pts == bestPts && strength < bestStrength) {
			best = &legal[i]
			bestPts = pts
			bestStrength = strength
		}
	}
	if best == nil && hasCandidate {
		return highestPointNonOvertaking(v, legal, trump)
	}
	return best
}

// heldSameSuitBoss reports whether the hand holds another card of c's suit that is itself a boss —
// the backup that makes c safe to spend without giving up control of the suit.
func heldSameSuitBoss(v View, c game.Card, trump game.Suit) bool {
	for _, d := range v.Hand {
		if d != c && d.Suit == c.Suit && cardIsBoss(v, d, trump) {
			return true
		}
	}
	return false
}

// bossWorthGuarding reports whether an unprotected boss can still win a future trick. A trump
// always can; a non-trump cannot in a ruffable endgame or in a suit an opponent can ruff.
func bossWorthGuarding(v View, c game.Card, trump game.Suit) bool {
	if c.Suit == trump {
		return true
	}
	if len(v.Hand) == 2 && !opponentsOutOfTrump(v, trump) {
		return false
	}
	idx := SuitIndex(c.Suit)
	for _, seat := range []int{(v.Seat + 1) % 4, (v.Seat + 3) % 4} {
		if idx >= 0 && v.KnownVoids[seat][idx] && opponentMayHoldTrump(v, seat, trump) {
			return false
		}
	}
	return true
}

// strongestByTrickPower returns the card with the greatest trick-taking power. Callers pass a
// single-suit slice, where beatsCard is a total order.
func strongestByTrickPower(cards []game.Card, led, trump game.Suit) game.Card {
	best := cards[0]
	for _, c := range cards[1:] {
		if beatsCard(c, best, led, trump) {
			best = c
		}
	}
	return best
}

// isTrumpMaster reports whether c is a trump that no trump an OPPONENT could play (threats)
// outranks.
func isTrumpMaster(v View, c game.Card, trump game.Suit) bool {
	if c.Suit != trump {
		return false
	}
	for _, u := range threats(v) {
		if u.Suit == trump && game.TrumpRankOrder[u.Rank] > game.TrumpRankOrder[c.Rank] {
			return false
		}
	}
	return true
}

// cardIsBoss reports whether c is the uncontested top of its suit from the OPPONENTS' side: the
// trump master, or a non-trump suit boss.
func cardIsBoss(v View, c game.Card, trump game.Suit) bool {
	if c.Suit == trump {
		return isTrumpMaster(v, c, trump)
	}
	return isSuitBoss(c, v)
}

// strongestPreservingBoss returns the strongest legal card, or the second strongest when the
// strongest is a boss worth keeping. A non-trump Ace backed by its Ten is spent, since the Ten
// inherits control.
func strongestPreservingBoss(v View, legal []game.Card, trump game.Suit) game.Card {
	led := v.CurrentTrick[0].Card.Suit
	strongest := strongestByTrickPower(legal, led, trump)
	if len(legal) < 2 || !cardIsBoss(v, strongest, trump) {
		return strongest
	}
	if strongest.Suit != trump && strongest.Rank == game.RankAce &&
		findRankOfSuit(legal, strongest.Suit, game.RankTen) != nil {
		return strongest
	}
	rest := make([]game.Card, 0, len(legal)-1)
	removed := false
	for _, c := range legal {
		if !removed && c == strongest {
			removed = true
			continue
		}
		rest = append(rest, c)
	}
	return strongestByTrickPower(rest, led, trump)
}

// highestPointsLedSuitWinner returns the highest-point led-suit card that overtakes the winner, or
// nil. The caller guarantees led != trump.
func highestPointsLedSuitWinner(v View, legal []game.Card, led, trump game.Suit) *game.Card {
	var best *game.Card
	bestPts := -1
	bestStrength := -1
	for i := range legal {
		c := legal[i]
		if c.Suit != led || !wouldOvertake(c, v.CurrentTrick, trump) {
			continue
		}
		pts := game.NonTrumpCardPoints[c.Rank]
		strength := game.NonTrumpRankOrder[c.Rank]
		if best == nil || pts > bestPts || (pts == bestPts && strength > bestStrength) {
			best = &legal[i]
			bestPts = pts
			bestStrength = strength
		}
	}
	return best
}

// lowestValue returns the lowest-point legal card, preferring non-trump on ties (preserve trump),
// then the weakest rank.
func lowestValue(legal []game.Card, trump game.Suit) game.Card {
	best := legal[0]
	for _, c := range legal[1:] {
		bp, cp := cardPoints(best, trump), cardPoints(c, trump)
		switch {
		case cp < bp:
			best = c
		case cp == bp:
			bestIsTrump := best.Suit == trump
			cIsTrump := c.Suit == trump
			if bestIsTrump && !cIsTrump {
				best = c
			} else if bestIsTrump == cIsTrump && cardStrengthOf(c, trump) < cardStrengthOf(best, trump) {
				best = c
			}
		}
	}
	return best
}

// highestOfSuit returns the strongest card of the given suit per the provided rank order, or nil
// when none is held.
func highestOfSuit(cards []game.Card, suit game.Suit, order map[game.Rank]int) *game.Card {
	var best *game.Card
	bestOrder := -1
	for i := range cards {
		c := cards[i]
		if c.Suit != suit {
			continue
		}
		if order[c.Rank] > bestOrder {
			bestOrder = order[c.Rank]
			best = &cards[i]
		}
	}
	return best
}

func cardStrengthOf(c game.Card, trump game.Suit) int {
	if c.Suit == trump {
		return game.TrumpRankOrder[c.Rank]
	}
	return game.NonTrumpRankOrder[c.Rank]
}
