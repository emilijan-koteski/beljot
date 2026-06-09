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

	// Step 3: Calculate total hand score per team
	aTotal := state.HandPoints[TeamA] + state.DeclarationPoints[TeamA]
	bTotal := state.HandPoints[TeamB] + state.DeclarationPoints[TeamB]

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
		TeamACardPoints: rawTeamACardPoints,
		TeamBCardPoints: rawTeamBCardPoints,
		TeamADeclPoints: state.DeclarationPoints[TeamA],
		TeamBDeclPoints: state.DeclarationPoints[TeamB],
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

// startNewHand resets all per-hand state, rotates the dealer, shuffles and deals
// a fresh deck, and transitions to PhaseBidding for the next hand.
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
	state.DeclarationsResolved = false
	state.HandCompleteReady = [4]bool{}

	// Reset per-hand scoring
	state.HandPoints = [2]int{0, 0}
	state.DeclarationPoints = [2]int{0, 0}
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

	// Clear player hands and declarations
	for i := range state.Players {
		state.Players[i].Hand = []Card{}
		state.Players[i].Declarations = nil
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
