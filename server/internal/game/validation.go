package game

func legalCards(state *GameState, seat int) []Card {
	hand := state.Players[seat].Hand

	if len(state.CurrentTrick) == 0 {
		return hand
	}

	if state.TrumpSuit == nil || state.LeadSuit == nil {
		return hand
	}
	trumpSuit := *state.TrumpSuit
	ledSuit := *state.LeadSuit

	suitCards := filterBySuit(hand, ledSuit)

	if len(suitCards) > 0 {
		ledSuitCanWin := ledSuit == trumpSuit || highestTrumpInTrick(state.CurrentTrick, trumpSuit) == nil
		if ledSuitCanWin {
			if higher := applyMustOverplayLedSuit(suitCards, state.CurrentTrick, ledSuit, trumpSuit); len(higher) > 0 {
				return higher
			}
		}
		return suitCards
	}

	trumpCards := filterBySuit(hand, trumpSuit)
	if len(trumpCards) > 0 {
		if overTrumps := applyOverTrump(trumpCards, state.CurrentTrick, trumpSuit); len(overTrumps) > 0 {
			return overTrumps
		}
		return trumpCards
	}

	return hand
}

func LegalCards(state *GameState, seat int) []Card {
	return legalCards(state, seat)
}

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
		return nil
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
			bestSeat = tc.PlayerSeat
			bestIsTrump = true
			bestOrder = order
		} else if isTrump && bestIsTrump && order > bestOrder {
			bestSeat = tc.PlayerSeat
			bestOrder = order
		} else if !isTrump && !bestIsTrump && isLed && order > bestOrder {
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
