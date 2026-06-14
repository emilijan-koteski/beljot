package bot

import "github.com/emilijan/beljot/server/internal/game"

// Decide maps a redacted View to the bot's next action. Pure and
// deterministic — humanized randomness lives in the match-layer think delay,
// never in card choice, so every branch here is table-testable. The returned
// action is an ordinary game.Action validated by the same ApplyAction path
// as human actions.
func Decide(v View) game.Action {
	// Surrender is team-internal: the partner proposed, the bot accepts.
	// Takes priority over any other decision at this seat.
	if v.PartnerProposedSurrender {
		return game.Action{Type: game.ActionSurrenderAccept, PlayerSeat: v.Seat}
	}

	if v.Phase == game.PhaseBidding {
		return decideBid(v)
	}

	// Belote/Rebelote: +20 is unconditional value — always announce.
	if v.PendingBelot {
		return game.Action{Type: game.ActionAnnounceBelot, PlayerSeat: v.Seat}
	}

	// Declarations: the engine auto-detects melds from the hand
	// (declarations.go) — there is nothing to choose, and declaring whenever
	// possible maximizes points.
	if v.AwaitingDeclaration {
		return game.Action{Type: game.ActionDeclare, PlayerSeat: v.Seat}
	}

	card := chooseCard(v)
	return game.Action{Type: game.ActionPlayCard, PlayerSeat: v.Seat, Card: &card}
}

// --- Bidding ---

// wantsTrump is the hand-strength evaluator gate: call the suit as trump when
// holding ≥4 cards of it, or 3 that include the Jack or the 9+Ace pair.
// Bidding happens on the 5-card stage-1 hand.
func wantsTrump(hand []game.Card, suit game.Suit) bool {
	var count int
	var hasJack, hasNine, hasAce bool
	for _, c := range hand {
		if c.Suit != suit {
			continue
		}
		count++
		switch c.Rank {
		case game.RankJack:
			hasJack = true
		case game.Rank9:
			hasNine = true
		case game.RankAce:
			hasAce = true
		}
	}
	if count >= 4 {
		return true
	}
	return count == 3 && (hasJack || (hasNine && hasAce))
}

// trumpSuitScore ranks qualifying suits in round 2: trump-order card points
// plus a per-card length bonus. Constants live here, in one place, for
// tuning.
const trumpLengthBonus = 10

func trumpSuitScore(hand []game.Card, suit game.Suit) int {
	score := 0
	for _, c := range hand {
		if c.Suit == suit {
			score += game.TrumpCardPoints[c.Rank] + trumpLengthBonus
		}
	}
	return score
}

func decideBid(v View) game.Action {
	if v.BiddingRound == 1 {
		// Round 1: the candidate's suit is the only option — pick or pass.
		// A round-1 pick carries no Suit (the engine locks the candidate's).
		if v.TrumpCandidate != nil && wantsTrump(v.Hand, v.TrumpCandidate.Suit) {
			return game.Action{Type: game.ActionPickTrump, PlayerSeat: v.Seat}
		}
		return game.Action{Type: game.ActionPassTrump, PlayerSeat: v.Seat}
	}

	// Round 2: evaluate the non-candidate suits with the same evaluator and
	// pick the best one that clears the threshold, else pass. The candidate's
	// suit is locked out by the engine (already spent in round 1).
	var best *game.Suit
	bestScore := -1
	for _, suit := range game.AllSuits {
		if v.TrumpCandidate != nil && suit == v.TrumpCandidate.Suit {
			continue
		}
		if !wantsTrump(v.Hand, suit) {
			continue
		}
		if score := trumpSuitScore(v.Hand, suit); score > bestScore {
			s := suit
			best = &s
			bestScore = score
		}
	}
	if best != nil {
		return game.Action{Type: game.ActionPickTrump, PlayerSeat: v.Seat, Suit: best}
	}
	return game.Action{Type: game.ActionPassTrump, PlayerSeat: v.Seat}
}

// --- Card play ---

func chooseCard(v View) game.Card {
	legal := v.LegalCards
	if len(legal) == 1 {
		return legal[0]
	}
	if len(v.CurrentTrick) == 0 {
		return chooseLead(v, legal)
	}
	return chooseFollow(v, legal)
}

// chooseLead picks the card to open a trick: draw trumps with J/9 while our
// team called trump and trumps remain unseen, otherwise bank a side-suit
// boss (Ace first), otherwise lead the strongest side card.
func chooseLead(v View, legal []game.Card) game.Card {
	if v.TrumpSuit == nil {
		return legal[0] // defensive: unreachable in PhasePlaying
	}
	trump := *v.TrumpSuit

	if myTeamCalledTrump(v) && trumpsRemainUnseen(v, trump) {
		if c := highestOfSuit(legal, trump, game.TrumpRankOrder); c != nil {
			return *c
		}
	}

	// Side-suit boss (no unseen card of its suit beats it): cash the points,
	// preferring the highest-value boss (Aces first).
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
		return *boss
	}

	// No boss: lead the strongest side card.
	var best *game.Card
	bestOrder := -1
	for i := range legal {
		c := legal[i]
		if c.Suit == trump {
			continue
		}
		if game.NonTrumpRankOrder[c.Rank] > bestOrder {
			bestOrder = game.NonTrumpRankOrder[c.Rank]
			best = &legal[i]
		}
	}
	if best != nil {
		return *best
	}

	// Only trumps left.
	if c := highestOfSuit(legal, trump, game.TrumpRankOrder); c != nil {
		return *c
	}
	return legal[0]
}

// chooseFollow picks the card when the trick already has cards: smear points
// onto a partner whose win is safe, win as cheaply as possible over an
// opponent, otherwise discard the lowest value while preserving trump.
func chooseFollow(v View, legal []game.Card) game.Card {
	if v.TrumpSuit == nil {
		return legal[0] // defensive: unreachable in PhasePlaying
	}
	trump := *v.TrumpSuit
	winnerSeat, _ := trickWinner(v.CurrentTrick, trump)

	if game.TeamForSeat(winnerSeat) == game.TeamForSeat(v.Seat) {
		if partnerWinIsSafe(v, trump) {
			// Smear: the highest-point card that does not overtake the
			// partner's already-won trick.
			if c := highestPointNonOvertaking(v, legal, trump); c != nil {
				return *c
			}
			// No non-overtaking card — every legal play would overtake the
			// partner. The trick stays ours either way; spend the cheapest.
			return lowestValue(legal, trump)
		}
		// Partner's card is still contestable — keep our points home. While the
		// led suit can still win, the overplay rule auto-promotes us when we
		// hold the boss.
		return lowestValue(legal, trump)
	}

	// Opponent currently wins: take the trick as cheaply as possible.
	if c := cheapestWinning(v, legal, trump); c != nil {
		return *c
	}
	// Cannot win: discard the lowest-value card, preserving trump.
	return lowestValue(legal, trump)
}

// --- Helpers (all pure) ---

func myTeamCalledTrump(v View) bool {
	return v.TrumpCallerSeat != nil &&
		game.TeamForSeat(*v.TrumpCallerSeat) == game.TeamForSeat(v.Seat)
}

// trumpsRemainUnseen reports whether any trump cards are still in unknown
// hands (not played, not held by this bot). Derived from unseenCards so the
// PlayedCards/CurrentTrick overlap dedupes — memory records a card at play
// time, so mid-trick it appears in BOTH sources, and a naive count would
// declare trumps exhausted early.
func trumpsRemainUnseen(v View, trump game.Suit) bool {
	for _, c := range unseenCards(v) {
		if c.Suit == trump {
			return true
		}
	}
	return false
}

// unseenCards returns the cards in unknown hands: the full deck minus this
// hand's played cards, the bot's own hand, and the current trick.
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
	out := make([]game.Card, 0, 32-len(known))
	for _, c := range game.NewDeck() {
		if !known[c] {
			out = append(out, c)
		}
	}
	return out
}

// isSuitBoss reports whether no unseen card of the same (non-trump) suit
// outranks c.
func isSuitBoss(c game.Card, v View) bool {
	for _, u := range unseenCards(v) {
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

// beatsCard reports whether candidate beats the current winner, given the
// led and trump suits. Mirrors the engine's trick-resolution ordering.
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
		// Only the led suit can win a trump-less trick.
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

// partnerWinIsSafe reports whether the partner's current trick win can no
// longer be contested: the bot closes the trick, or no unseen card can
// legally beat the partner's card. When only higher led-suit cards survive,
// known voids (the holders must cut, but no trump beats the partner) settle
// it.
func partnerWinIsSafe(v View, trump game.Suit) bool {
	if len(v.CurrentTrick) == 3 {
		return true
	}
	_, winning := trickWinner(v.CurrentTrick, trump)
	led := v.CurrentTrick[0].Card.Suit

	higherLedUnseen := false
	trumpThreatUnseen := false
	for _, u := range unseenCards(v) {
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
	// Higher led-suit cards are still out, but a seat known void in the led
	// suit cannot hold them; if every remaining opponent is void, the
	// partner's card stands (no trump threatens it — checked above).
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

func cardPoints(c game.Card, trump game.Suit) int {
	if c.Suit == trump {
		return game.TrumpCardPoints[c.Rank]
	}
	return game.NonTrumpCardPoints[c.Rank]
}

// wouldOvertake reports whether playing c makes this seat the trick's new
// winner.
func wouldOvertake(c game.Card, trick []game.TrickCard, trump game.Suit) bool {
	_, winning := trickWinner(trick, trump)
	return beatsCard(c, winning, trick[0].Card.Suit, trump)
}

// highestPointNonOvertaking returns the highest-point legal card that leaves
// the current winner in place, or nil when every legal card overtakes.
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

// cheapestWinning returns the lowest-point (then weakest) legal card that
// takes the trick, or nil when none can.
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

// lowestValue returns the lowest-point legal card, preferring non-trump on
// ties (preserve trump), then the weakest rank.
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

// highestOfSuit returns the strongest card of the given suit per the
// provided rank order, or nil when none is held.
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
