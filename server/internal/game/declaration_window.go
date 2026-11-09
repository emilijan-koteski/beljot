package game

// declarationWindowOpen reports whether the turn-window timing still takes a declaration in the current trick:
// declarations are on, the hand's contest has not resolved, and the trick is at most DeclarationLastTrick.
func declarationWindowOpen(state *GameState) bool {
	return state.Rules.DeclarationsEnabled &&
		state.Rules.DeclarationTiming == DeclarationTimingTurnWindow &&
		!state.DeclarationsResolved &&
		state.TrickNumber >= 1 &&
		state.TrickNumber <= state.Rules.DeclarationLastTrick
}

// checkDeclarationPrompt prompts the active seat before its card while the declaration window is open and its
// current hand holds a meld. Call it at every turn start during play.
func checkDeclarationPrompt(state *GameState) {
	if state.Phase != PhasePlaying || !declarationWindowOpen(state) {
		return
	}
	seat := state.ActivePlayerSeat
	if hasDeclarableCombinations(state.Players[seat].Hand, state.Rules.DeclarationOverlap) {
		state.AwaitingDeclaration = true
	}
}

// openDeclarationCycle moves a declaration at a turn prompt into the simultaneous declaring phase, with the declarer
// already answered. The trick and ActivePlayerSeat stay, so play resumes at the declarer's card.
func openDeclarationCycle(state *GameState, declarer int) {
	state.Phase = PhaseDeclaring
	state.AwaitingDeclaration = false
	for i := range state.Players {
		state.Players[i].DeclarationAnswered = i == declarer
	}
	maybeCloseDeclarationPhase(state)
}

// closeDeclarationCycle resolves the contest and returns to the same trick and seat with no new prompt, which the
// resolution rules out for the rest of the hand. A team reaching the target ends the match here.
func closeDeclarationCycle(state *GameState) {
	resolveDeclarationsForHand(state)

	for i := range state.Players {
		state.Players[i].DeclarationAnswered = false
	}
	state.AwaitingDeclaration = false
	state.Phase = PhasePlaying

	stopAtTargetIfReached(state)
}
