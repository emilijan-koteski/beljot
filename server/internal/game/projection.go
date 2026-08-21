package game

// ProjectForSeat returns what ONE seat is allowed to see of a game state: the
// per-recipient mask applied to every match_state frame at the serialization
// boundary (Story 12.10). It is the whole card-privacy policy in one pure
// function — the same seat-scoped boundary buildBotView draws for bots,
// computed exactly where the recipient is known.
//
// Masked, per recipient:
//
//   - Every OTHER seat's Hand is emptied; HandCount is set from the REAL hand
//     length on all four seats so opponents still render the right number of
//     card backs. (Deck and FaceDownCards never serialize at all — json:"-".)
//   - Every OTHER seat's Declarations are emptied while the contest is
//     unresolved: declaration cards ARE in-hand cards. Once DeclarationsResolved
//     is true the engine has already nil'd the losing team's melds, so what
//     remains is the public reveal and passes through.
//   - PendingBelotSeat is nil'd unless it names the recipient: broadcasting it
//     tells everyone a player holds the second trump royal before they choose
//     to announce — the exact secret the announce/decline choice protects. The
//     client only ever compares the field to its own seat, so null-for-others
//     is compatible.
//
// The recipient's own information stays intact. The input is NEVER mutated —
// the state is deep-cloned first (Go slices share backing arrays, so masking
// in place would reach back into the live state). Variant-blind by
// construction (D-VAR-1): nothing here reads Variant or Rules.
//
// seat is the recipient's seat (0-3). There is deliberately no spectator or
// omniscient variant: an out-of-range seat simply matches no player, which
// fails safe — everything is masked.
func ProjectForSeat(gs *GameState, seat int) *GameState {
	p := cloneGameState(gs)

	for i := range p.Players {
		p.Players[i].HandCount = len(p.Players[i].Hand)
		if i == seat {
			continue
		}
		// Non-nil empties, not nil: the wire contract serializes these as []
		// (the client's strict schema parses arrays, and Go marshals nil
		// slices as null).
		p.Players[i].Hand = []Card{}
		if !p.DeclarationsResolved {
			p.Players[i].Declarations = []Declaration{}
		}
	}

	if p.PendingBelotSeat != nil && *p.PendingBelotSeat != seat {
		p.PendingBelotSeat = nil
	}

	return p
}
