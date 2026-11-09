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
	contractingTeam := TeamForSeat(*state.TrumpCallerSeat)

	tally := HandTally{
		CardPoints:           state.HandPoints,
		DeclPoints:           [2]int{aDeclTotal, bDeclTotal},
		CallerTeam:           contractingTeam,
		Capot:                isCapot,
		DeclarationsTransfer: state.Rules.DeclarationsTransfer,
		TieRule:              state.Rules.TieRule,
	}
	if capotTeam != nil {
		tally.CapotTeam = *capotTeam
	}
	settled := SettleHand(tally, state.HangingPoints)
	state.HangingPoints = cloneHangingPoints(settled.Hanging)
	state.TeamScores[TeamA] += settled.Awarded[TeamA]
	state.TeamScores[TeamB] += settled.Awarded[TeamB]

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
		FailedContract:  settled.FailedContract,
		ContractingTeam: contractingTeam,
		TeamAHandTotal:  settled.Awarded[TeamA],
		TeamBHandTotal:  settled.Awarded[TeamB],
		HangingCreated:  settled.Hanging,
		HangingPaid:     settled.Paid,
	}

	target := MatchTarget(state.MatchMode)
	aOver := state.TeamScores[TeamA] >= target
	bOver := state.TeamScores[TeamB] >= target

	if aOver || bOver {
		winner := determineMatchWinner(state, aOver, bOver)
		state.WinnerTeam = &winner
		state.Phase = PhaseMatchEnd
		state.HangingPoints = nil
		return
	}

	state.Phase = PhaseHandComplete
	state.HandCompleteReady = [4]bool{}
}

// HandTally is a completed hand as settlement sees it. CardPoints include the last-trick or Capot bonus, DeclPoints
// include Belote, and CapotTeam counts only when Capot is set.
type HandTally struct {
	CardPoints           [2]int
	DeclPoints           [2]int
	CallerTeam           int
	Capot                bool
	CapotTeam            int
	DeclarationsTransfer bool
	TieRule              TieRule
}

// HandSettlement is what a hand banks: each team's award with any hanging points paid, whether the caller fell,
// the entry a Bitola tie leaves hanging, and the hanging points the hand paid out and to whom.
type HandSettlement struct {
	Awarded        [2]int
	FailedContract bool
	Hanging        *HangingPoints
	Paid           *HangingPoints
}

// SettleHand settles a completed hand against the points left hanging by a tie, if any. It is the single rule that
// scoreHand and the bot search both apply: a Capot, then a pass, then a Bitola tie, then a fall.
func SettleHand(t HandTally, hanging *HangingPoints) HandSettlement {
	caller, opponents := t.CallerTeam, 1-t.CallerTeam
	totals := [2]int{t.CardPoints[TeamA] + t.DeclPoints[TeamA], t.CardPoints[TeamB] + t.DeclPoints[TeamB]}

	var s HandSettlement
	taker := opponents
	switch {
	case t.Capot:
		s.Awarded[t.CapotTeam] = wholeHandAward(t, t.CapotTeam)
		taker = t.CapotTeam
	case totals[caller] > totals[opponents]:
		s.Awarded = totals
		taker = caller
	case totals[caller] == totals[opponents] && t.TieRule == TieRuleHangingPoints:
		s.Awarded[opponents] = totals[opponents]
		s.Hanging = &HangingPoints{Amount: totals[caller], Team: caller}
	default:
		s.Awarded[opponents] = wholeHandAward(t, opponents)
	}

	if t.TieRule == TieRuleHangingPoints {
		s.FailedContract = totals[caller] < totals[opponents]
	} else {
		s.FailedContract = totals[caller] <= totals[opponents]
	}

	if hanging != nil {
		s.Awarded[taker] += hanging.Amount
		s.Paid = &HangingPoints{Amount: hanging.Amount, Team: taker}
	}
	return s
}

// wholeHandAward returns what a team taking the whole hand (a fall or a Capot) banks: every card point, plus
// both sides' declarations and Belote when they transfer, or only its own when they do not.
func wholeHandAward(t HandTally, winner int) int {
	cardPoints := t.CardPoints[TeamA] + t.CardPoints[TeamB]
	if t.DeclarationsTransfer {
		return cardPoints + t.DeclPoints[TeamA] + t.DeclPoints[TeamB]
	}
	return cardPoints + t.DeclPoints[winner]
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
	state.HangingPoints = nil

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
	state.DealerSeat = state.Rules.NextSeat(state.DealerSeat)

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

	state.ActivePlayerSeat = state.Rules.NextSeat(state.DealerSeat)

	if winnerTeam := checkInstantWin(state); winnerTeam != nil {
		state.WinnerTeam = winnerTeam
		state.Phase = PhaseMatchEnd
		state.WonByInstantWin = true
		state.HangingPoints = nil
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

// MatchTarget returns the match's point target: 501 or 701 for those modes, 1001 otherwise.
func MatchTarget(mode string) int {
	switch mode {
	case "501":
		return 501
	case "701":
		return 701
	default:
		return 1001
	}
}
