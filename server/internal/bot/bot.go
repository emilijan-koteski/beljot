package bot

import (
	"slices"

	"github.com/emilijan/beljot/server/internal/game"
)

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
// holding ≥4 cards of it; or exactly 3 that include the Jack (if a 7 or 8 is
// among them a side Ace is required, otherwise unconditional); or exactly 3 with
// the 9+Ace pair plus a side Ace; or exactly 2 that are the Jack AND the 9 plus
// a side Ace (the two strongest trumps, worth a call when backed by an outside
// winner). A side Ace is any Ace in another suit — it need not be backed by a
// second card of its suit. Used by both bidding rounds. Callers pass the
// hand-plus-candidate the picker would actually hold (see decideBid).
func wantsTrump(hand []game.Card, suit game.Suit) bool {
	var count int
	var hasJack, hasNine, hasAce, has7, has8 bool
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
		case game.Rank7:
			has7 = true
		case game.Rank8:
			has8 = true
		}
	}
	if count >= 4 {
		return true
	}
	if count == 3 {
		if hasJack {
			// 3 trumps including the Jack: a take when the other two are both
			// 9-or-higher. A 7 or 8 among them makes the hand weaker, but a side
			// Ace (an outside winner) still makes it worth calling.
			return (!has7 && !has8) || hasSideAce(hand, suit)
		}
		// 3 trumps without the Jack: the 9+Ace pair with a side Ace.
		return hasNine && hasAce && hasSideAce(hand, suit)
	}
	// Exactly 2 trumps: a call only when they are the two strongest trumps (the
	// Jack and the 9) and an outside Ace backs the hand.
	if count == 2 {
		return hasJack && hasNine && hasSideAce(hand, suit)
	}
	return false
}

// hasSideAce reports whether the hand holds an Ace in some non-trump suit. The
// Ace need not be backed by a second card of its suit — a lone side Ace still
// counts as an outside winner.
func hasSideAce(hand []game.Card, trump game.Suit) bool {
	for _, c := range hand {
		if c.Suit != trump && c.Rank == game.RankAce {
			return true
		}
	}
	return false
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
	// The picker ALWAYS receives the face-up trump candidate (engine appends it
	// in handlePickTrump, both rounds), so the bid must be evaluated on the hand
	// the bot would actually hold after picking: the 5 dealt cards PLUS the
	// candidate. In round 1 the candidate is a trump (its suit IS the trump
	// suit); in round 2 it is a guaranteed side card (its suit is locked out).
	bidHand := v.Hand
	if v.TrumpCandidate != nil {
		bidHand = append(slices.Clone(v.Hand), *v.TrumpCandidate)
	}

	if v.BiddingRound == 1 {
		// Round 1: the candidate's suit is the only option — pick or pass.
		// A round-1 pick carries no Suit (the engine locks the candidate's).
		if v.TrumpCandidate != nil && wantsTrump(bidHand, v.TrumpCandidate.Suit) {
			return game.Action{Type: game.ActionPickTrump, PlayerSeat: v.Seat}
		}
		return game.Action{Type: game.ActionPassTrump, PlayerSeat: v.Seat}
	}

	// Round 2: evaluate the non-candidate suits with the same evaluator and
	// pick the best one that clears the threshold, else pass. The candidate's
	// suit is locked out by the engine (already spent in round 1), but the
	// candidate card itself still lands in the picker's hand as a side card, so
	// it counts toward the side-Ace backup in wantsTrump.
	var best *game.Suit
	bestScore := -1
	for _, suit := range game.AllSuits {
		if v.TrumpCandidate != nil && suit == v.TrumpCandidate.Suit {
			continue
		}
		if !wantsTrump(bidHand, suit) {
			continue
		}
		if score := trumpSuitScore(bidHand, suit); score > bestScore {
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
	// Endgame retention: hold the master trump back for the forced last trick
	// (+10 "dix de der") instead of squandering it now. Returns the card to
	// spend this trick, or nil to defer to the normal lead/follow heuristics.
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

// retainLastTrickWinner implements last-trick retention ("dix de der", +10).
// At the second-to-last trick — exactly two cards in hand — the forced 8th
// trick (one legal card each) goes to whoever RETAINS the best card through
// trick 7. The bot must hold the master trump: it is a guaranteed trick-8
// winner under ANY lead (no card an opponent could play outranks it, and it
// wins whether led or forced in as a follow). It normally spends the OTHER
// card now and banks the master for the last trick.
//
// Generalization: when the other card is ITSELF a guaranteed trick-8 winner —
// an uncuttable non-trump boss (a suit boss no opponent can ruff, because the
// opponents are out of trump; see isUncuttableBoss) — the bot instead spends
// the MASTER TRUMP now and keeps the boss for trick 8. Leading the master wins
// trick 7 and hands the bot the lead into trick 8, where the uncuttable boss
// then wins. The outcome is identical (the team takes both tricks either way),
// but the trump is led first, keeping the side winner in reserve.
//
// Note the non-master case is deliberately NOT handled here: if the bot holds a
// lower trump plus an uncuttable boss, spending that trump first would only hand
// the lead (and the master trump) to the partner, so the current "cash the boss
// now" lead is already at least as good — the team keeps trick-8 control either
// way. Only the master-trump holder gains from re-ordering.
//
// Returns nil — deferring to the normal heuristics — unless every condition
// holds:
//   - exactly two cards remain (the endgame decision point);
//   - the bot's best trump is the master (holdsMasterTrump);
//   - the card it would spend is legal THIS trick. Follow-suit / over-trump
//     rules can force a card out; when they do, len(legal)==1 already plays it
//     above, and this guard means we never assume a card we cannot hold back;
//   - the partner is not already known (from the declaration reveal) to hold a
//     higher trump — if it does, the team controls trick 8 regardless, so we
//     don't fight the partner and let the normal heuristics play (e.g. draw).
func retainLastTrickWinner(v View, legal []game.Card, trump game.Suit) *game.Card {
	if len(v.Hand) != 2 {
		return nil
	}
	master := highestOfSuit(v.Hand, trump, game.TrumpRankOrder)
	if master == nil || !holdsMasterTrump(v, trump) {
		return nil
	}
	// The single other card is the spare we would normally spend now.
	var spare *game.Card
	for i := range v.Hand {
		if v.Hand[i] != *master {
			spare = &v.Hand[i]
			break
		}
	}
	if spare == nil {
		return nil // defensive: two identical cards are impossible
	}
	// Don't fight a partner who already secures the last trick: if the partner
	// is known to hold a trump above our master, the team takes trick 8 anyway.
	partner := (v.Seat + 2) % 4
	for _, pc := range knownHeldBy(v, partner) {
		if pc.Suit == trump && game.TrumpRankOrder[pc.Rank] > game.TrumpRankOrder[master.Rank] {
			return nil
		}
	}
	// Both cards guaranteed to win trick 8: spend the master trump first and keep
	// the uncuttable boss for the last trick (provided the master is legal now).
	if isUncuttableBoss(v, *spare, trump) && slices.Contains(legal, *master) {
		return master
	}
	// Otherwise keep the master and spend the spare — only when the spare is a
	// legal play this trick.
	if !slices.Contains(legal, *spare) {
		return nil
	}
	return spare
}

// isUncuttableBoss reports whether c is a non-trump card that is the boss of its
// suit AND cannot be ruffed — the opponents are out of trump, so no one can cut
// it. Such a card is a guaranteed trick-8 winner WHEN THE BOT LEADS trick 8.
func isUncuttableBoss(v View, c game.Card, trump game.Suit) bool {
	return c.Suit != trump && isSuitBoss(c, v) && opponentsOutOfTrump(v, trump)
}

// opponentsOutOfTrump reports whether NEITHER opponent could still play a trump
// (both known void in trump, or every trump is otherwise accounted for). When
// true the remaining trumps sit only with the bot and its partner, so there is
// nothing to draw and no opponent can ruff. Distinct from trumpsRemainUnseen,
// which counts the partner's UNKNOWN trumps as if they were an opponent's and so
// still reports trumps outstanding here.
func opponentsOutOfTrump(v View, trump game.Suit) bool {
	for _, seat := range []int{(v.Seat + 1) % 4, (v.Seat + 3) % 4} {
		if opponentMayHoldTrump(v, seat, trump) {
			return false
		}
	}
	return true
}

// chooseLead picks the card to open a trick: draw trumps while our team called
// trump, trumps remain unseen, and we still hold the master trump; otherwise
// bank a side-suit boss (Ace first); otherwise lead the safest low card.
func chooseLead(v View, legal []game.Card) game.Card {
	if v.TrumpSuit == nil {
		return legal[0] // defensive: unreachable in PhasePlaying
	}
	trump := *v.TrumpSuit

	// Draw trumps to strip the opponents — but only while we still hold the
	// master (top remaining) trump. Leading a high trump we cannot back with the
	// master just hands it to whoever holds the Jack/9 above it. Skipped once the
	// opponents are out of trump (opponentsOutOfTrump): with only teammates
	// holding trumps there is nothing to draw, and leading trump would just strip
	// the partner's control — lead a side suit instead and let the partner lead.
	if myTeamCalledTrump(v) && trumpsRemainUnseen(v, trump) && holdsMasterTrump(v, trump) &&
		!opponentsOutOfTrump(v, trump) {
		if c := highestOfSuit(legal, trump, game.TrumpRankOrder); c != nil {
			return *c
		}
	}

	// Draw trumps for the partner: when the PARTNER called trump, lead a trump to
	// strip the opponents even though WE do not hold the master — the partner is
	// assumed to hold the top trumps (J/9) and overtrumps to take the trick. We
	// sacrifice the weakest honor first (Q, K, T, A) and never lead the 9 (a
	// near-master kept as a winner, so the partner's Jack never gets stripped by
	// our own draw) — see partnerDrawTrump. Skipped when the partner is void in
	// trump (no overtrump to set up), when a known opponent holding already
	// outranks our best trump (the "partner has the top" assumption has been
	// disproven by the reveal), or when the opponents are already out of trump
	// (nothing left to draw — let the partner keep the lead).
	if myTeamCalledTrump(v) && trumpsRemainUnseen(v, trump) && !holdsMasterTrump(v, trump) &&
		!opponentsOutOfTrump(v, trump) {
		partner := (v.Seat + 2) % 4
		botTop := highestOfSuit(legal, trump, game.TrumpRankOrder)
		if botTop != nil &&
			v.TrumpCallerSeat != nil && *v.TrumpCallerSeat == partner &&
			!partnerVoidInTrump(v, partner, trump) &&
			!opponentKnownHoldsTrumpAbove(v, *botTop, trump) {
			if c := partnerDrawTrump(legal, trump); c != nil {
				return *c
			}
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
		// Ace+Ten exception: when the chosen boss is the Ace and we also hold that
		// suit's Ten, cash the Ten first. Holding the Ace makes the Ten a boss too,
		// so it still wins the trick — and we keep the Ace as the guaranteed master
		// of the suit. Applies ONLY to the Ace+Ten pair, not other touching honors.
		if boss.Rank == game.RankAce {
			if ten := findRankOfSuit(legal, boss.Suit, game.RankTen); ten != nil {
				return *ten
			}
		}
		return *boss
	}

	// Feed the partner a ruff: with no boss to cash, if the partner is known
	// void in a side suit (and not known void in trump, so it can still ruff),
	// lead the lowest card of that suit so the partner trumps the trick and wins.
	if c := leadIntoPartnerVoid(v, legal, trump); c != nil {
		return *c
	}

	// No boss: lead the safest card — the lowest-value non-trump (a 0-point
	// 7/8/9 when held). Avoids exposing a 10 or other high card into an unseen
	// Ace whenever a cheaper non-trump exists, and never burns a trump here.
	var nonTrump []game.Card
	for _, c := range legal {
		if c.Suit != trump {
			nonTrump = append(nonTrump, c)
		}
	}
	if len(nonTrump) > 0 {
		return lowestValue(nonTrump, trump)
	}

	// Only trumps left: lead the highest trump ONLY when it is the master (no
	// opponent can beat it) — a winner worth cashing. Otherwise lead the lowest
	// (weakest) trump; leading a high non-master trump just donates it to the
	// opponent's master, so keep the stronger trumps back.
	if c := highestOfSuit(legal, trump, game.TrumpRankOrder); c != nil {
		if isTrumpMaster(v, *c, trump) {
			return *c
		}
		return lowestValue(legal, trump)
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
			// partner. The trick stays ours either way, so capture it with our
			// strongest card, but keep a boss/master back: if the strongest legal
			// card is the boss of its suit, drop to the second strongest (still
			// wins, since every legal card overtakes) and preserve the boss for a
			// later trick.
			return strongestPreservingBoss(v, legal, trump)
		}
		// Risk-smear (Rule 8): the partner holds the uncatchable top of a
		// non-trump led suit (e.g. the Ace). The only thing that can beat it is a
		// ruff from the one opponent still to play, and that opponent is NOT known
		// void in the led suit — so the ruff is only a possibility. Take the
		// calculated risk and pile high points onto the trick. If every legal play
		// would overtake (a forced ruff with no card to spare), there is nothing to
		// smear and we fall through to the cheap ruff below.
		if shouldSmearOntoPartnerBoss(v, trump) {
			if c := highestPointNonOvertaking(v, legal, trump); c != nil {
				return *c
			}
		}
		// Partner's card is still genuinely contestable (or a known ruff looms) —
		// keep our points home. While the led suit can still win, the overplay rule
		// auto-promotes us when we hold the boss.
		return lowestValue(legal, trump)
	}

	// Opponent currently wins, but if our partner is yet to play and is KNOWN
	// (from the reveal) to take this trick, it is effectively ours — don't spend
	// a winner overtaking. Smear the highest-point non-overtaking card instead.
	if partnerTakesTrick(v, trump) {
		if c := highestPointNonOvertaking(v, legal, trump); c != nil {
			return *c
		}
		return lowestValue(legal, trump)
	}

	// Opponent currently wins. When we are LAST to play and can win by FOLLOWING
	// the led (non-trump) suit, bank the highest-point led-suit winner instead of
	// the cheapest: kept and led next trick that high card risks being ruffed,
	// but banked into this trick — which we are guaranteed to take as the last
	// player — it is safe. Ruff wins (void in the led suit) and trump-led tricks
	// fall through to cheapestWinning unchanged.
	if len(v.CurrentTrick) == 3 {
		led := v.CurrentTrick[0].Card.Suit
		if led != trump {
			if c := highestPointsLedSuitWinner(v, legal, led, trump); c != nil {
				return *c
			}
		}
	}

	// Opponent currently wins: take the trick as cheaply as possible.
	if c := cheapestWinning(v, legal, trump); c != nil {
		return *c
	}
	// Cannot win: discard the lowest-value card, preserving trump.
	return lowestValue(legal, trump)
}

// partnerTakesTrick reports whether the bot's partner is GUARANTEED to take the
// current (opponent-led) trick, so the bot must not spend a winner overtaking
// it. The bot's partner (seat+2) is yet to play only when the bot sits at the
// leader's left — and then the partner plays LAST, with exactly one opponent
// acting between them. The take is sound only when the partner can LEGALLY play
// the beater no matter what that opponent does: it must be provably void in the
// led suit (so it is forced to ruff, not to follow suit into a loss) AND hold a
// trump that beats the current winner and that no opponent-reachable card
// (threats) can beat. A higher led-suit card is NOT enough — an intervening
// opponent could ruff and force the suit-bound partner under.
func partnerTakesTrick(v View, trump game.Suit) bool {
	if len(v.CurrentTrick) == 0 {
		return false // defensive: chooseFollow only runs mid-trick
	}
	partner := (v.Seat + 2) % 4
	if !slices.Contains(seatsYetToPlay(v), partner) {
		return false
	}
	led := v.CurrentTrick[0].Card.Suit
	ledIdx := SuitIndex(led)
	if ledIdx < 0 || !v.KnownVoids[partner][ledIdx] {
		return false // partner might be forced to follow suit and lose
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

// --- Helpers (all pure) ---

func myTeamCalledTrump(v View) bool {
	return v.TrumpCallerSeat != nil &&
		game.TeamForSeat(*v.TrumpCallerSeat) == game.TeamForSeat(v.Seat)
}

// trumpsRemainUnseen reports whether any trump could still be in an OPPONENT's
// hand — an unknown hand, or a known opponent holding from the reveal. Scans
// threats (not raw unseen) so a partner's declared trumps do NOT count: once
// the opponents are provably void of trump, there is nothing to draw and the
// bot stops leading trumps at its own partner. Derived from threats so the
// PlayedCards/CurrentTrick overlap dedupes (memory records a card at play time,
// so mid-trick it appears in both sources).
func trumpsRemainUnseen(v View, trump game.Suit) bool {
	for _, c := range threats(v) {
		if c.Suit == trump {
			return true
		}
	}
	return false
}

// unseenCards returns the cards in UNKNOWN hands: the full deck minus this
// hand's played cards, the bot's own hand, the current trick, and any card the
// declaration reveal lets us place in a specific seat (no longer unknown).
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

// knownHeldBy returns the cards we know seat still holds: its publicly revealed
// declaration cards minus any already played this hand or sitting in the
// current trick. A declared card that has since been played is no longer a
// holding, so it drops out here.
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

// threats returns the cards an OPPONENT could still play: cards in unknown
// hands plus cards we KNOW an opponent holds (revealed declarations when the
// opponents won the contest). A partner's known card is never a threat — the
// bot does not fight it. With no reveal, threats == unseenCards, so every
// heuristic below behaves exactly as before.
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

// holdsMasterTrump reports whether the bot controls the top trump from the
// OPPONENTS' side: true when no trump an opponent could play (threats) outranks
// the bot's best trump; false when the bot holds no trump. Scanning threats
// (not raw unseen) relaxes Goal A's conservatism — once the partner has
// declared the high trumps, those cards leave the threat set, so the bot is
// recognized as controlling the suit and may keep drawing with its own best
// trump. Gates the draw-trumps lead.
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

// partnerVoidInTrump reports whether the partner is KNOWN void in the trump
// suit (inferred from a prior non-follow). Shared by the partner trump-draw
// (Rule 4) and the partner-void ruff feed (Rule 7).
func partnerVoidInTrump(v View, partner int, trump game.Suit) bool {
	idx := SuitIndex(trump)
	return idx >= 0 && partner >= 0 && partner < 4 && v.KnownVoids[partner][idx]
}

// opponentKnownHoldsTrumpAbove reports whether a card we KNOW an opponent holds
// (from the declaration reveal) is a trump that outranks ref. Used to abort the
// partner trump-draw when the reveal proves the top trumps are NOT with the
// partner.
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

// partnerDrawTrump picks the trump to lead when drawing for the partner
// (Rule 4): sacrifice the weakest honor first — Queen, then King, then Ten, then
// Ace — keeping the stronger trumps back. With no honor it leads the lowest
// trump that is NOT the 9; the 9 is a near-master held back as a winner, and the
// Jack is never a candidate here (holding it makes the bot the master, handled by
// the draw-with-master lead). Returns nil when the only trump available to lead
// would be the 9 — then the bot does not draw at all.
func partnerDrawTrump(legal []game.Card, trump game.Suit) *game.Card {
	for _, r := range []game.Rank{game.RankQueen, game.RankKing, game.RankTen, game.RankAce} {
		for i := range legal {
			if legal[i].Suit == trump && legal[i].Rank == r {
				return &legal[i]
			}
		}
	}
	// No honor — lead the lowest trump that is not the 9.
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

// leadIntoPartnerVoid returns the lowest-value card of a side suit the partner
// is known void in (so the partner can ruff and win the trick), or nil when no
// such lead applies. Requires the partner not to be known void in trump (else
// it cannot ruff). Skips the trump suit and any suit the bot cannot lead. Among
// several candidate void suits it picks the cheapest sacrifice (lowest card
// value), breaking ties by AllSuits order for determinism. Only reached after
// the boss step, so the bot holds no side boss to cash here.
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

// findRankOfSuit returns a pointer to the card of the given suit and rank in
// cards, or nil when it is not present.
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

// isSuitBoss reports whether no card an OPPONENT could play (threats) of the
// same (non-trump) suit outranks c. Scanning threats keeps a known opponent
// holding (e.g. an Ace the opponents declared) a real threat, so the bot never
// mistakes its King for a boss.
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
// longer be contested: the bot closes the trick, or no card an opponent could
// play (threats) can legally beat the partner's card. When only higher led-suit
// cards survive,
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

// opponentMayHoldTrump reports whether the given opponent seat could still play
// a trump, and therefore ruff. True when it is known to hold a trump (revealed
// and not yet played); false when it is known void in trump; otherwise true only
// if some trump is still unseen (could be in its hand). When every trump is
// accounted for, a seat void in the led suit cannot ruff.
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

// shouldSmearOntoPartnerBoss reports whether the partner's currently-winning
// card is the uncatchable top of a NON-TRUMP led suit (only a ruff could beat
// it) and no yet-to-play opponent can still ruff it. When true, the bot piles
// high points onto the trick, accepting the unknown ruff risk (Rule 8). Reached
// only from the partner-winning branch after partnerWinIsSafe is false, i.e.
// some unseen trump could ruff — but here that ruff is only a possibility.
//
// The only thing that keeps points home is a yet-to-play opponent that is known
// void in the led suit AND can still hold a trump (a near-certain ruff). If that
// opponent is also out of trump — known void in trump, or every trump already
// accounted for — it cannot ruff, so we still smear.
func shouldSmearOntoPartnerBoss(v View, trump game.Suit) bool {
	if len(v.CurrentTrick) == 0 {
		return false // defensive: chooseFollow only runs mid-trick
	}
	led := v.CurrentTrick[0].Card.Suit
	if led == trump {
		return false // non-trump scenario only
	}
	_, winning := trickWinner(v.CurrentTrick, trump)
	if winning.Suit != led {
		return false // partner won by ruff, not by following the led suit
	}
	// Boss: no card an opponent could play of the led suit beats the partner's.
	for _, u := range threats(v) {
		if u.Suit == led && beatsCard(u, winning, led, trump) {
			return false
		}
	}
	// Don't donate into a near-certain ruff: a yet-to-play opponent known void in
	// the led suit that can still hold a trump keeps our points home. A void
	// opponent that cannot possibly ruff (out of trump) does not block the smear.
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

// strongestByTrickPower returns the card with the greatest trick-taking power
// given the led and trump suits (any trump beats any non-trump; within a suit
// class, rank order decides). Callers pass a SINGLE-suit slice (follow-suit or
// ruff cards), where beatsCard is a total order and ties cannot occur.
func strongestByTrickPower(cards []game.Card, led, trump game.Suit) game.Card {
	best := cards[0]
	for _, c := range cards[1:] {
		if beatsCard(c, best, led, trump) {
			best = c
		}
	}
	return best
}

// isTrumpMaster reports whether c is a trump that no trump an OPPONENT could
// play (threats) outranks.
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

// cardIsBoss reports whether c is the uncontested top of its suit from the
// OPPONENTS' side: the trump master, or a non-trump suit boss.
func cardIsBoss(v View, c game.Card, trump game.Suit) bool {
	if c.Suit == trump {
		return isTrumpMaster(v, c, trump)
	}
	return isSuitBoss(c, v)
}

// strongestPreservingBoss returns the strongest legal card by trick power, but
// when that card is the boss/master of its suit it returns the SECOND strongest
// instead — preserving the boss for a later trick. Used when every legal card
// would overtake a partner who has safely won the trick: the team keeps the
// trick regardless, so we capture it with a high (but not top) card. With a
// single legal card there is nothing to preserve.
func strongestPreservingBoss(v View, legal []game.Card, trump game.Suit) game.Card {
	led := v.CurrentTrick[0].Card.Suit
	strongest := strongestByTrickPower(legal, led, trump)
	if len(legal) < 2 || !cardIsBoss(v, strongest, trump) {
		return strongest
	}
	// Ace+Ten exception: when the boss is a non-trump Ace and we also hold that
	// suit's Ten, smear the Ace onto this (already-won) trick instead of keeping
	// it. Spending the Ace surrenders no control — the Ten becomes the suit's new
	// boss — so we bank the higher points now and still keep a winner.
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

// highestPointsLedSuitWinner returns the highest-point card of the led suit that
// would overtake the current winner, or nil when none (void in the led suit, or
// no led-suit card outranks the winner — e.g. a trump already cut a non-trump
// trick). Ties broken by strength. The caller guarantees led != trump, so
// NonTrumpCardPoints is the correct value table.
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
