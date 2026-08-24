package game

// scoreHand calculates the final hand score after all 8 tricks are resolved.
// It applies last-trick bonus (or Capot bonus), checks for failed contracts,
// updates match scores, and either starts a new hand or ends the match.
// Populates LastHandResult with the full scoring breakdown for client broadcast.
// Mutates an already-cloned state (called from within resolveTrickWithDeclarations).
func scoreHand(state *GameState) {
	// Step 1: Determine last-trick winner's seat and team
	if state.TrickWinnerSeat == nil {
		return // defensive: should never happen in normal flow
	}
	lastTrickSeat := *state.TrickWinnerSeat
	lastTrickTeam := TeamForSeat(lastTrickSeat)

	// The hand is over — clear the just-resolved trick from the served state so
	// the hand-complete / match-end snapshot carries no current trick. On the
	// 8th trick resolveTrick returns early (PhaseHandScoring) and SKIPS its
	// "set up next trick" reset, so without this the authoritative match_state
	// still holds the four last-trick cards; after the client's collect sweep
	// clears its snapshot it would fall back to that live trick and flash the
	// four cards back at table center. TrickWinnerSeat is intentionally KEPT —
	// the final-hand event:trick_resolved resolves its winner from it.
	state.CurrentTrick = nil
	state.LeadSuit = nil

	// Capture raw card points BEFORE bonus application
	rawTeamACardPoints := state.HandPoints[TeamA]
	rawTeamBCardPoints := state.HandPoints[TeamB]

	// Step 2: Apply Capot bonus (+100) or last-trick bonus (+10)
	isCapot := false
	var capotTeam *int
	capotBonus := 0
	lastTrickBonus := 0

	if state.TricksWon[TeamA] == 8 {
		state.HandPoints[TeamA] += 100
		isCapot = true
		t := TeamA
		capotTeam = &t
		capotBonus = 100
	} else if state.TricksWon[TeamB] == 8 {
		state.HandPoints[TeamB] += 100
		isCapot = true
		t := TeamB
		capotTeam = &t
		capotBonus = 100
	} else {
		state.HandPoints[lastTrickTeam] += 10
		lastTrickBonus = 10
	}

	// Step 3: Calculate total hand score per team. Belote (BelotPoints) counts
	// as a declaration: it joins each team's total and its declaration figure.
	aDeclTotal := state.DeclarationPoints[TeamA] + state.BelotPoints[TeamA]
	bDeclTotal := state.DeclarationPoints[TeamB] + state.BelotPoints[TeamB]
	aTotal := state.HandPoints[TeamA] + aDeclTotal
	bTotal := state.HandPoints[TeamB] + bDeclTotal

	// Step 4: Failed contract check
	contractingTeam := TeamForSeat(*state.TrumpCallerSeat)
	opposingTeam := 1 - contractingTeam

	var contractingTotal, opposingTotal int
	if contractingTeam == TeamA {
		contractingTotal = aTotal
		opposingTotal = bTotal
	} else {
		contractingTotal = bTotal
		opposingTotal = aTotal
	}

	// Step 5: Award points — failed contract or normal scoring.
	// The trump-calling team must score STRICTLY MORE than the opponents to
	// succeed. An equal total (a tie, e.g. 81:81 of the 162 base points) is a
	// failed hand for the caller — they don't clear half the points in play.
	//
	// NOTE: this "tie -> all points to opponents" behavior is the CROATIAN-variant
	// rule, currently applied to ALL variants as an interim stand-in. The Bitola
	// variant must eventually use HANGING POINTS (carry-over) on a tie instead —
	// points held over, nobody scores, carried to the next decisive hand. That
	// needs cross-hand state and is deferred to Epic 12 (see deferred-work.md).
	failedContract := contractingTotal <= opposingTotal
	allPoints := aTotal + bTotal
	var aAwarded, bAwarded int
	switch {
	case isCapot:
		// Capot: the team that won all 8 tricks takes EVERY point in the hand —
		// card points, the +100 bonus, and both teams' declarations. The side
		// that won no trick scores nothing, forfeiting even declarations it had
		// won in the declaration contest; a team that takes no trick cannot bank
		// points. (When the non-contracting team makes Capot the contract has
		// also failed, but the destination is identical: everything to the Capot
		// team. The FailedContract flag below still reflects that correctly.)
		state.TeamScores[*capotTeam] += allPoints
		if *capotTeam == TeamA {
			aAwarded = allPoints
		} else {
			bAwarded = allPoints
		}
	case failedContract:
		// Failed contract: contracting team gets 0, opponent gets ALL points
		state.TeamScores[opposingTeam] += allPoints
		if opposingTeam == TeamA {
			aAwarded = allPoints
		} else {
			bAwarded = allPoints
		}
	default:
		// Normal scoring: each team keeps their own points
		state.TeamScores[TeamA] += aTotal
		state.TeamScores[TeamB] += bTotal
		aAwarded = aTotal
		bAwarded = bTotal
	}

	// Step 6: Populate LastHandResult for broadcast
	state.LastHandResult = &HandScore{
		// Stamped so consumers can tell this result from a stale one carried
		// forward by startNewHand. See HandScore.HandNumber.
		HandNumber:      state.HandNumber,
		TeamACardPoints: rawTeamACardPoints,
		TeamBCardPoints: rawTeamBCardPoints,
		TeamADeclPoints: aDeclTotal,
		TeamBDeclPoints: bDeclTotal,
		LastTrickTeam:   lastTrickTeam,
		LastTrickSeat:   lastTrickSeat,
		LastTrickBonus:  lastTrickBonus,
		Capot:           isCapot,
		CapotTeam:       capotTeam,
		CapotBonus:      capotBonus,
		FailedContract:  failedContract,
		ContractingTeam: contractingTeam,
		TeamAHandTotal:  aAwarded,
		TeamBHandTotal:  bAwarded,
	}

	// Step 7: Check match-end condition with tiebreaker logic
	target := matchTarget(state.MatchMode)
	aOver := state.TeamScores[TeamA] >= target
	bOver := state.TeamScores[TeamB] >= target

	if aOver || bOver {
		winner := determineMatchWinner(state, aOver, bOver)
		state.WinnerTeam = &winner
		state.Phase = PhaseMatchEnd
		return
	}

	// Step 8: Hold for the hand-complete pause. The next hand is NOT dealt here —
	// the session manager waits for players to acknowledge (action:continue ->
	// startNewHand) or for the auto-continue timeout. This keeps the next hand's
	// cards, turn, and trump prompt off-screen until the score is seen.
	state.Phase = PhaseHandComplete
	state.HandCompleteReady = [4]bool{}
}

// teamRunningTotal returns what team would have banked if the match stopped
// RIGHT NOW: its match score plus everything it has accumulated so far in the
// current, unfinished hand.
//
// The three accumulators are exactly the three things that move mid-hand — card
// points from resolved tricks, the declaration contest's award, and Belote's +20
// — which is why the three stopAtTarget checkpoints sit immediately after each
// of their write sites.
//
// Deliberately NOT the same arithmetic as scoreHand: no last-trick +10 and no
// Capot +100, because the hand never completed and neither bonus was earned; and
// no failed-hand transfer, because the taker's "strictly more" test only makes
// sense on a finished hand. scoreHand:56-59 computes a superficially similar
// total but does so AFTER bonus application, so it is not reusable here.
func teamRunningTotal(state *GameState, team int) int {
	return state.TeamScores[team] +
		state.HandPoints[team] +
		state.DeclarationPoints[team] +
		state.BelotPoints[team]
}

// stopAtTargetIfReached is the whole "dosta" (enough) rule, in one place. When
// the room plays with StopAtTarget and either team's RUNNING total has reached
// the match target, it ends the match on the spot and reports true; otherwise it
// leaves the state untouched and reports false.
//
// THIS COMMENT IS THE CANONICAL STATEMENT OF THE RULE. types.go's
// VariantRules.StopAtTarget, declarations.go's three call sites, room/model.go's
// Room.StopAtTarget and migration 000022 all point here rather than restating it,
// so there is one place to correct if it ever changes.
//
// EXACTLY THREE CHECKPOINTS, because there are exactly three places mid-hand
// points are awarded, each with one write site in this package: a trick resolving
// (playing.go, resolveTrick), the declaration contest resolving
// (declarations.go, resolveDeclarationsForHand) and a Belote announcement
// (declarations.go, handleAnnounceBelot). Callers hook this helper immediately
// after each award, in the same ApplyAction call, and must abort the rest of
// their flow when it returns true — a Belote stop in particular must NOT go on to
// resolve the trick that was in progress.
//
// ONE DEFERRAL, and only one: handleAnnounceBelot skips this check while
// TrickNumber == 1 && !DeclarationsResolved. Under Bitola timing the trick-1 meld
// contest is still open at that moment, and its award lands only when the trick
// resolves, so stopping on the +20 would bank a running total with every declared
// meld missing. See that call site for the full reasoning and for the rejected
// alternative.
//
// It is a no-op in three cases, all cheap and all deliberate:
//
//   - StopAtTarget is off (every existing room), so OFF is byte-identical to the
//     behaviour before this rule existed.
//
//   - the state is already PhaseMatchEnd or PhaseHandScoring — a completed hand
//     belongs to scoreHand, which applies the bonuses and does its own target
//     check. Trick 8 is not a checkpoint.
//
//     PhaseHandComplete is deliberately NOT in that set, and its absence is not
//     an oversight: no checkpoint can be reached from it. It is the post-scoreHand
//     pause, where no trick resolves, no contest runs and no Belote is announced —
//     the next thing that happens is action:continue or the auto-advance, both of
//     which go through startNewHand. Adding it would be dead code that implied a
//     path exists.
//
//   - nobody has reached the target yet.
//
// At the stop it:
//
//   - commits each team's running total into TeamScores and LEAVES HandPoints,
//     DeclarationPoints and BelotPoints POPULATED. That is the same state shape
//     scoreHand already produces at a normal match end: it too banks a hand's
//     points into TeamScores without clearing the accumulators they came from.
//     An earlier draft of this rule zeroed them, to stop the scoreboard's "+N
//     this hand" bar double-counting banked points — but that bar already behaves
//     exactly this way at every normal match end, so the zeroing bought nothing
//     and broke the declaration reveal:
//     broadcastDeclarationsResolvedIfTransition derives the reveal's winnerTeam
//     from DeclarationPoints[team] > 0, so a declaration-driven stop shipped a
//     reveal with winnerTeam null and no winner for the panel to anchor to.
//
//   - resolves WinnerTeam through the existing determineMatchWinner, which
//     dereferences TrumpCallerSeat — always non-nil at all three checkpoints,
//     since none of them can be reached before a seat has taken trump.
//
//     Note what determineMatchWinner is NOT asked to do: there is no failed-hand
//     transfer here. scoreHand's "the taker must score strictly more" test needs
//     a finished hand, so a crossing by the taker's OPPONENTS simply wins for
//     them and the taker keeps whatever it had banked — nothing moves between
//     teams at a stop.
//
//   - nils LastHandResult. LOAD-BEARING, not tidiness: the match layer's
//     handJustScored and bufferHandResultIfScored both gate on it being non-nil
//     plus a transition into match_end, and startNewHand deliberately never
//     clears it. A hand-3 stop still holding hand 2's result would emit a false
//     event:hand_scored and write hand 2's numbers into the final hand_results
//     row. The aborted hand deliberately gets NO hand_results row at all.
//
//   - clears the timer and prompt fields, so the final snapshot carries no live
//     turn, meld prompt, Belote prompt or surrender proposal.
//
// It deliberately does NOT touch ActivePlayerSeat or the trick state.
// trickResolvedWinnerSeat reads ActivePlayerSeat as its trick-1..7 winner
// fallback, so overwriting it would broadcast the wrong winner on the final
// trick; and an abandoned trick with cards still face up is exactly where play
// stopped, which is the truthful thing to show.
func stopAtTargetIfReached(state *GameState) bool {
	if !state.Rules.StopAtTarget {
		return false
	}
	if state.Phase == PhaseMatchEnd || state.Phase == PhaseHandScoring {
		return false
	}

	target := matchTarget(state.MatchMode)
	aTotal := teamRunningTotal(state, TeamA)
	bTotal := teamRunningTotal(state, TeamB)
	aOver := aTotal >= target
	bOver := bTotal >= target
	if !aOver && !bOver {
		return false
	}

	// Bank the running totals. The accumulators they were built from are left
	// alone — see the doc comment: clearing DeclarationPoints blinds the
	// declaration reveal's winner derivation, and scoreHand does not clear them
	// at a normal match end either.
	state.TeamScores[TeamA] = aTotal
	state.TeamScores[TeamB] = bTotal

	// Both teams over at one checkpoint IS reachable, and the trick-1 deferral is
	// what makes it so. Away from trick 1 only one team can gain at a single
	// checkpoint, so only one can cross. But the deferral banks a Belote +20 and
	// then waits for the trick-1 resolution, where TWO awards land in the same
	// call — the trick's card points and the declaration contest — and they can go
	// to opposite teams. So the announcing team can cross on its +20 while the
	// other crosses on the contest, both first observed here.
	//
	// determineMatchWinner settles it the same way the end-of-hand path already
	// settles a hand in which both teams cross: higher score, then the taker.
	// Deliberately NOT "whoever crossed first chronologically" — that would need
	// the crossing recorded on the state, and it would make a dosta stop resolve a
	// double crossing differently from every other match end.
	winner := determineMatchWinner(state, aOver, bOver)
	state.WinnerTeam = &winner
	state.Phase = PhaseMatchEnd

	// No HandScore is fabricated for the aborted hand — no synthetic last-trick
	// team, Capot flag or failed-hand verdict. See the doc comment above for why
	// nil here is load-bearing.
	state.LastHandResult = nil

	// Records that THIS match end was a stop, so the match layer can say so
	// rather than infer it. Without this the only signal is "StopAtTarget is on
	// and LastHandResult is nil", which is also true of a surrender and of an
	// instant-win, and the client ends up telling the player nothing at all: a
	// match that stops mid-hand otherwise reads as an unexplained score jump with
	// cards still in hand. Server-only (json:"-") — the client learns it from
	// event:match_end's outcome reason, not from the state snapshot.
	state.StoppedAtTarget = true

	state.TurnExpiresAt = nil
	state.TurnTimeRemaining = 0
	state.AwaitingDeclaration = false
	state.PendingBelotSeat = nil
	state.SurrenderProposerSeat = nil

	return true
}

// startNewHand resets all per-hand state, rotates the dealer, shuffles and deals
// a fresh deck, and transitions to PhaseBidding for the next hand.
//
// The deal goes through dealCards, which reads state.Rules.DealShape — so hand 2
// onwards of a match is dealt exactly like hand 1 was, whatever the variant.
func startNewHand(state *GameState) {
	// Advance hand metadata
	state.HandNumber++
	state.DealerSeat = (state.DealerSeat + 1) % 4

	// Reset bidding state
	state.TrumpSuit = nil
	state.TrumpCallerSeat = nil
	state.TrumpCandidate = nil
	state.BiddingRound = 1
	state.BiddingPassCount = 0

	// Reset trick state
	state.TrickNumber = 0
	state.CurrentTrick = []TrickCard{}
	state.LeadSuit = nil
	state.TrickWinnerSeat = nil
	state.AwaitingDeclaration = false
	// Re-seeded from the rule config, not blindly reset to false: in a room
	// playing without declarations this flag starting true is what makes every
	// downstream guard skip the contest (see NewGame). Resetting it to false
	// unconditionally would skip declarations in hand 1 only and let them return
	// from hand 2 onwards.
	state.DeclarationsResolved = !state.Rules.DeclarationsEnabled
	state.DeclarationsContested = false
	state.HandCompleteReady = [4]bool{}
	// Belongs to the hand that just ended, not the one being dealt. Unreachable
	// in practice (a stop ends the match, so no next hand is dealt) but reset
	// here so the flag can never outlive its hand.
	state.StoppedAtTarget = false

	// Reset per-hand scoring
	state.HandPoints = [2]int{0, 0}
	state.DeclarationPoints = [2]int{0, 0}
	state.BelotPoints = [2]int{0, 0}
	state.TricksWon = [2]int{0, 0}
	state.PendingBelotSeat = nil
	state.BelotAnnounced = false
	state.WinnerTeam = nil
	state.TurnExpiresAt = nil
	// NOTE: LastHandResult is intentionally NOT cleared here — it must persist
	// in the state returned to the session manager for broadcast. It is overwritten
	// by the next scoreHand() call, so it never leaks across hands.

	// Reset disconnect fields (defensive — PhaseDisconnected blocks new hands,
	// but ensures clean state if flow changes)
	state.DisconnectedSeat = -1
	state.ReconnectExpiresAt = nil

	// Clear player hands, face-down cards, and declarations
	for i := range state.Players {
		state.Players[i].Hand = []Card{}
		state.Players[i].FaceDownCards = nil
		state.Players[i].Declarations = nil
		state.Players[i].DeclarationAnswered = false
	}

	// Generate fresh deck, shuffle, and deal
	deck := NewDeck()
	ShuffleDeck(deck)
	dealCards(state, deck)

	// Set active player and phase
	state.ActivePlayerSeat = (state.DealerSeat + 1) % 4

	// Check for instant-win (player holds all 8 trump cards)
	if winnerTeam := checkInstantWin(state); winnerTeam != nil {
		state.WinnerTeam = winnerTeam
		state.Phase = PhaseMatchEnd
		return
	}

	state.Phase = PhaseDealing
}

// checkInstantWin checks if any player holds all 8 cards of the trump suit after
// dealing. Returns the winning team index, or nil if no instant-win.
func checkInstantWin(state *GameState) *int {
	// Prefer the locked trump (post-pick). Fall back to the candidate's suit
	// when no trump has been chosen yet — that branch covers stage-1 states
	// (where no hand can hold all 8 of any suit anyway) and direct fixture
	// states used by package-internal tests.
	var trumpSuit Suit
	switch {
	case state.TrumpSuit != nil:
		trumpSuit = *state.TrumpSuit
	case state.TrumpCandidate != nil:
		trumpSuit = state.TrumpCandidate.Suit
	default:
		return nil
	}
	for i := range state.Players {
		trumpCount := 0
		for _, card := range state.Players[i].Hand {
			if card.Suit == trumpSuit {
				trumpCount++
			}
		}
		if trumpCount == 8 {
			team := TeamForSeat(state.Players[i].Seat)
			return &team
		}
	}
	return nil
}

// determineMatchWinner resolves which team wins when at least one team has crossed
// the match target. Handles tiebreaker: if both teams crossed, higher score wins;
// if tied, the contracting team (trump picker) wins.
func determineMatchWinner(state *GameState, aOver, bOver bool) int {
	if aOver && bOver {
		// Both teams crossed — higher score wins
		if state.TeamScores[TeamA] > state.TeamScores[TeamB] {
			return TeamA
		}
		if state.TeamScores[TeamB] > state.TeamScores[TeamA] {
			return TeamB
		}
		// Tied scores — contracting team (trump picker) wins
		return TeamForSeat(*state.TrumpCallerSeat)
	}
	// Only one team crossed
	if aOver {
		return TeamA
	}
	return TeamB
}

// matchTarget returns the point threshold for match completion based on the match mode.
func matchTarget(mode string) int {
	if mode == "501" {
		return 501
	}
	return 1001
}
