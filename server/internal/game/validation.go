package game

// legalCards returns the set of cards that the player at the given seat is
// legally allowed to play, given the current game state. Implements Bitola
// variant card play validation:
//
//  1. Follow suit if possible. The overplay (iber) obligation — play a
//     led-suit card strictly higher than the highest led-suit card on the
//     table — applies ONLY while the led suit can still win the trick: when
//     the led suit is trump, or no trump has cut the trick yet. Once a trump
//     cuts a non-trump-led trick the led suit can no longer win, so any
//     same-suit card is legal.
//  2. If void in led suit and the player holds at least one trump → must
//     cut. Over-trump the highest trump on the table if possible; otherwise
//     any trump is legal. Non-trumps are illegal in this branch — Bitola has
//     no partner-winning exemption.
//  3. If void in led suit and the player holds no trump → any card is legal.
func legalCards(state *GameState, seat int) []Card {
	hand := state.Players[seat].Hand

	// Leading: any card is legal
	if len(state.CurrentTrick) == 0 {
		return hand
	}

	if state.TrumpSuit == nil || state.LeadSuit == nil {
		return hand // defensive: treat as leading if pointers unset
	}
	trumpSuit := *state.TrumpSuit
	ledSuit := *state.LeadSuit

	// Collect cards of the led suit
	suitCards := filterBySuit(hand, ledSuit)

	if len(suitCards) > 0 {
		// Must follow suit. The overplay (iber) obligation applies only while
		// the led suit can still win the trick: when the led suit is trump, or
		// no trump has cut the trick yet. Once a trump cuts a non-trump-led
		// trick the led suit can no longer take it, so any same-suit card is
		// legal (the trump is already winning).
		ledSuitCanWin := ledSuit == trumpSuit || highestTrumpInTrick(state.CurrentTrick, trumpSuit) == nil
		if ledSuitCanWin {
			if higher := applyMustOverplayLedSuit(suitCards, state.CurrentTrick, ledSuit, trumpSuit); len(higher) > 0 {
				return higher
			}
		}
		return suitCards
	}

	// Void in led suit — must cut with trump if any trump is held, regardless
	// of who is currently winning the trick (Bitola has no partner exemption).
	trumpCards := filterBySuit(hand, trumpSuit)
	if len(trumpCards) > 0 {
		if overTrumps := applyOverTrump(trumpCards, state.CurrentTrick, trumpSuit); len(overTrumps) > 0 {
			return overTrumps
		}
		return trumpCards
	}

	// Void in led suit and no trump held — any card is legal.
	return hand
}

// LegalCards exposes the legal-play set for a seat (Story 10.3 — consumed by
// the bot decision layer and its simulation baseline). Thin wrapper over the
// unexported legalCards; zero rule changes, and engine behavior remains
// tested through ApplyAction.
func LegalCards(state *GameState, seat int) []Card {
	return legalCards(state, seat)
}

// applyMustOverplayLedSuit returns the led-suit cards in hand that are strictly
// higher than the highest led-suit card currently in the trick. Uses
// TrumpRankOrder when the led suit is trump, NonTrumpRankOrder otherwise.
// Returns nil if no overplay exists or the trick contains no led-suit card
// (the latter shouldn't happen when called with a non-empty trick, since the
// first card defines the led suit).
func applyMustOverplayLedSuit(suitCards []Card, trick []TrickCard, ledSuit, trumpSuit Suit) []Card {
	rankOrder := NonTrumpRankOrder
	if ledSuit == trumpSuit {
		rankOrder = TrumpRankOrder
	}
	bestOrder := -1
	for _, tc := range trick {
		if tc.Card.Suit == ledSuit {
			if order := rankOrder[tc.Card.Rank]; order > bestOrder {
				bestOrder = order
			}
		}
	}
	if bestOrder < 0 {
		return nil
	}
	var higher []Card
	for _, c := range suitCards {
		if rankOrder[c.Rank] > bestOrder {
			higher = append(higher, c)
		}
	}
	return higher
}

// isCardLegal checks whether a specific card is a legal play for the given seat.
func isCardLegal(state *GameState, seat int, card Card) bool {
	for _, c := range legalCards(state, seat) {
		if c.Rank == card.Rank && c.Suit == card.Suit {
			return true
		}
	}
	return false
}

// applyOverTrump filters trump cards to only those that beat the highest trump
// already in the trick. Returns nil if no over-trumps are available.
func applyOverTrump(trumpCards []Card, trick []TrickCard, trumpSuit Suit) []Card {
	highest := highestTrumpInTrick(trick, trumpSuit)
	if highest == nil {
		return nil // no trump in trick — no over-trump obligation
	}

	highestOrder := TrumpRankOrder[*highest]
	var overTrumps []Card
	for _, c := range trumpCards {
		if TrumpRankOrder[c.Rank] > highestOrder {
			overTrumps = append(overTrumps, c)
		}
	}
	return overTrumps
}

// highestTrumpInTrick returns the rank of the highest trump card in the trick,
// or nil if no trump has been played.
func highestTrumpInTrick(trick []TrickCard, trumpSuit Suit) *Rank {
	var best *Rank
	bestOrder := -1
	for _, tc := range trick {
		if tc.Card.Suit == trumpSuit {
			order := TrumpRankOrder[tc.Card.Rank]
			if order > bestOrder {
				bestOrder = order
				r := tc.Card.Rank
				best = &r
			}
		}
	}
	return best
}

// currentTrickWinnerSeat determines which player is currently winning among
// the cards played so far in the trick (1-3 cards).
func currentTrickWinnerSeat(trick []TrickCard, trumpSuit Suit) int {
	if len(trick) == 0 {
		return -1
	}

	ledSuit := trick[0].Card.Suit
	bestSeat := trick[0].PlayerSeat
	bestIsTrump := trick[0].Card.Suit == trumpSuit
	bestOrder := cardStrength(trick[0].Card, trumpSuit)

	for _, tc := range trick[1:] {
		isTrump := tc.Card.Suit == trumpSuit
		isLed := tc.Card.Suit == ledSuit
		order := cardStrength(tc.Card, trumpSuit)

		if isTrump && !bestIsTrump {
			// Trump beats non-trump
			bestSeat = tc.PlayerSeat
			bestIsTrump = true
			bestOrder = order
		} else if isTrump && bestIsTrump && order > bestOrder {
			// Higher trump beats lower trump
			bestSeat = tc.PlayerSeat
			bestOrder = order
		} else if !isTrump && !bestIsTrump && isLed && order > bestOrder {
			// Higher card of led suit beats lower (only led suit can win)
			bestSeat = tc.PlayerSeat
			bestOrder = order
		}
	}

	return bestSeat
}

// cardStrength returns the ranking strength of a card. Trump cards use
// TrumpRankOrder, non-trump use NonTrumpRankOrder.
func cardStrength(card Card, trumpSuit Suit) int {
	if card.Suit == trumpSuit {
		return TrumpRankOrder[card.Rank]
	}
	return NonTrumpRankOrder[card.Rank]
}

// filterBySuit returns all cards in hand that match the given suit.
func filterBySuit(hand []Card, suit Suit) []Card {
	var result []Card
	for _, c := range hand {
		if c.Suit == suit {
			result = append(result, c)
		}
	}
	return result
}
