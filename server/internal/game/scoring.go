package game

// scoreHand applies the last-trick or Capot bonus, settles the contract, and adds the hand to the match scores.
// It records LastHandResult, then either ends the match or pauses at PhaseHandComplete.
func scoreHand(state *GameState) {
	if state.TrickWinnerSeat == nil {
		return
	}
	lastTrickSeat := *state.TrickWinnerSeat
	lastTrickTeam := TeamForSeat(lastTrickSeat)

	state.CurrentTrick = nil
	state.LeadSuit = nil

	rawTeamACardPoints := state.HandPoints[TeamA]
	rawTeamBCardPoints := state.HandPoints[TeamB]

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

	aDeclTotal := state.DeclarationPoints[TeamA] + state.BelotPoints[TeamA]
	bDeclTotal := state.DeclarationPoints[TeamB] + state.BelotPoints[TeamB]
	aTotal := state.HandPoints[TeamA] + aDeclTotal
	bTotal := state.HandPoints[TeamB] + bDeclTotal

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

	failedContract := contractingTotal <= opposingTotal
	allPoints := aTotal + bTotal
	var aAwarded, bAwarded int
	switch {
	case isCapot:
		state.TeamScores[*capotTeam] += allPoints
		if *capotTeam == TeamA {
			aAwarded = allPoints
		} else {
			bAwarded = allPoints
		}
	case failedContract:
		state.TeamScores[opposingTeam] += allPoints
		if opposingTeam == TeamA {
			aAwarded = allPoints
		} else {
			bAwarded = allPoints
		}
	default:
		state.TeamScores[TeamA] += aTotal
		state.TeamScores[TeamB] += bTotal
		aAwarded = aTotal
		bAwarded = bTotal
	}

	state.LastHandResult = &HandScore{
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

	target := MatchTarget(state.MatchMode)
	aOver := state.TeamScores[TeamA] >= target
	bOver := state.TeamScores[TeamB] >= target

	if aOver || bOver {
		winner := determineMatchWinner(state, aOver, bOver)
		state.WinnerTeam = &winner
		state.Phase = PhaseMatchEnd
		return
	}

	state.Phase = PhaseHandComplete
	state.HandCompleteReady = [4]bool{}
}

// teamRunningTotal returns what team would bank if the match stopped now: its match score plus the unfinished hand's points.
// It excludes the last-trick and Capot bonuses, which only a completed hand earns.
func teamRunningTotal(state *GameState, team int) int {
	return state.TeamScores[team] +
		state.HandPoints[team] +
		state.DeclarationPoints[team] +
		state.BelotPoints[team]
}

// stopAtTargetIfReached ends the match on the spot when StopAtTarget is on and a team's running total reaches the target.
// Callers invoke it right after each mid-hand award and must abort their flow when it returns true.
func stopAtTargetIfReached(state *GameState) bool {
	if !state.Rules.StopAtTarget {
		return false
	}
	if state.Phase == PhaseMatchEnd || state.Phase == PhaseHandScoring {
		return false
	}

	target := MatchTarget(state.MatchMode)
	aTotal := teamRunningTotal(state, TeamA)
	bTotal := teamRunningTotal(state, TeamB)
	aOver := aTotal >= target
	bOver := bTotal >= target
	if !aOver && !bOver {
		return false
	}

	state.TeamScores[TeamA] = aTotal
	state.TeamScores[TeamB] = bTotal

	winner := determineMatchWinner(state, aOver, bOver)
	state.WinnerTeam = &winner
	state.Phase = PhaseMatchEnd
	state.LastHandResult = nil
	state.StoppedAtTarget = true

	state.TurnExpiresAt = nil
	state.TurnTimeRemaining = 0
	state.AwaitingDeclaration = false
	state.PendingBelotSeat = nil
	state.SurrenderProposerSeat = nil

	return true
}

// startNewHand resets per-hand state, rotates the dealer, and deals a fresh deck per state.Rules.DealShape.
// LastHandResult is kept so the session manager can still broadcast it.
func startNewHand(state *GameState) {
	state.HandNumber++
	state.DealerSeat = (state.DealerSeat + 1) % 4

	state.TrumpSuit = nil
	state.TrumpCallerSeat = nil
	state.TrumpCandidate = nil
	state.BiddingRound = 1
	state.BiddingPassCount = 0

	state.TrickNumber = 0
	state.CurrentTrick = []TrickCard{}
	state.LeadSuit = nil
	state.TrickWinnerSeat = nil
	state.AwaitingDeclaration = false
	state.DeclarationsResolved = !state.Rules.DeclarationsEnabled
	state.DeclarationsContested = false
	state.HandCompleteReady = [4]bool{}
	state.StoppedAtTarget = false
	state.WonByInstantWin = false

	state.HandPoints = [2]int{0, 0}
	state.DeclarationPoints = [2]int{0, 0}
	state.BelotPoints = [2]int{0, 0}
	state.TricksWon = [2]int{0, 0}
	state.PendingBelotSeat = nil
	state.BelotAnnounced = false
	state.WinnerTeam = nil
	state.TurnExpiresAt = nil

	state.DisconnectedSeat = -1
	state.ReconnectExpiresAt = nil

	for i := range state.Players {
		state.Players[i].Hand = []Card{}
		state.Players[i].FaceDownCards = nil
		state.Players[i].Declarations = nil
		state.Players[i].DeclarationAnswered = false
	}

	deck := NewDeck()
	ShuffleDeck(deck)
	dealCards(state, deck)

	state.ActivePlayerSeat = (state.DealerSeat + 1) % 4

	if winnerTeam := checkInstantWin(state); winnerTeam != nil {
		state.WinnerTeam = winnerTeam
		state.Phase = PhaseMatchEnd
		state.WonByInstantWin = true
		return
	}

	state.Phase = PhaseDealing
}

// checkInstantWin returns the team of a player holding all 8 trump cards, or nil if none does.
// It uses the locked trump, falling back to the candidate's suit before a pick.
func checkInstantWin(state *GameState) *int {
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

// determineMatchWinner returns the winning team once at least one team has crossed the target.
// If both crossed, the higher score wins and a tie goes to the contracting team.
func determineMatchWinner(state *GameState, aOver, bOver bool) int {
	if aOver && bOver {
		if state.TeamScores[TeamA] > state.TeamScores[TeamB] {
			return TeamA
		}
		if state.TeamScores[TeamB] > state.TeamScores[TeamA] {
			return TeamB
		}
		return TeamForSeat(*state.TrumpCallerSeat)
	}
	if aOver {
		return TeamA
	}
	return TeamB
}

// MatchTarget returns the match's point target: 501 for mode "501", 1001 otherwise.
func MatchTarget(mode string) int {
	if mode == "501" {
		return 501
	}
	return 1001
}
