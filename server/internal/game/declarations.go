package game

import (
	"sort"

	"github.com/emilijan/beljot/server/internal/apperr"
)

var sequencePoints = map[int]int{
	3: 20,
	4: 50,
}

var fourOfAKindPoints = map[Rank]int{
	RankJack:  200,
	Rank9:     150,
	RankAce:   100,
	RankTen:   100,
	RankKing:  100,
	RankQueen: 100,
}

// detectDeclarations returns every sequence and four-of-a-kind in hand with its point value.
// When overlap is false a card counts toward at most one declaration.
func detectDeclarations(hand []Card, overlap bool) []Declaration {
	var decls []Declaration

	bySuit := map[Suit][]Card{}
	for _, c := range hand {
		bySuit[c.Suit] = append(bySuit[c.Suit], c)
	}

	for _, suit := range AllSuits {
		cards := bySuit[suit]
		if len(cards) < 3 {
			continue
		}
		sort.Slice(cards, func(i, j int) bool {
			return NaturalRankOrder[cards[i].Rank] < NaturalRankOrder[cards[j].Rank]
		})

		seqStart := 0
		for i := 1; i <= len(cards); i++ {
			consecutive := i < len(cards) &&
				NaturalRankOrder[cards[i].Rank] == NaturalRankOrder[cards[i-1].Rank]+1
			if !consecutive {
				seqLen := i - seqStart
				if seqLen >= 3 {
					seqCards := make([]Card, seqLen)
					copy(seqCards, cards[seqStart:i])
					pts := 100
					if v, ok := sequencePoints[seqLen]; ok {
						pts = v
					}
					decls = append(decls, Declaration{
						Type:  DeclarationSequence,
						Cards: seqCards,
						Value: pts,
					})
				}
				seqStart = i
			}
		}
	}

	byRank := map[Rank][]Card{}
	for _, c := range hand {
		byRank[c.Rank] = append(byRank[c.Rank], c)
	}
	for _, rank := range AllRanks {
		cards := byRank[rank]
		if len(cards) == 4 {
			if pts, ok := fourOfAKindPoints[rank]; ok {
				foakCards := make([]Card, 4)
				copy(foakCards, cards)
				decls = append(decls, Declaration{
					Type:  DeclarationFourOfAKind,
					Cards: foakCards,
					Value: pts,
				})
			}
		}
	}

	if overlap {
		return decls
	}
	return dedupOneCardOneGroup(decls)
}

// dedupOneCardOneGroup keeps the highest-value declaration among those sharing a card, preferring four-of-a-kind on ties.
// Survivors keep their original order.
func dedupOneCardOneGroup(decls []Declaration) []Declaration {
	if len(decls) <= 1 {
		return decls
	}

	order := make([]int, len(decls))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := decls[order[i]], decls[order[j]]
		if a.Value != b.Value {
			return a.Value > b.Value
		}
		if a.Type != b.Type {
			return a.Type == DeclarationFourOfAKind
		}
		return false
	})

	used := map[Card]bool{}
	keep := make([]bool, len(decls))
	for _, idx := range order {
		d := decls[idx]
		conflict := false
		for _, c := range d.Cards {
			if used[c] {
				conflict = true
				break
			}
		}
		if conflict {
			continue
		}
		for _, c := range d.Cards {
			used[c] = true
		}
		keep[idx] = true
	}

	out := make([]Declaration, 0, len(decls))
	for i, d := range decls {
		if keep[i] {
			out = append(out, d)
		}
	}
	return out
}

// hasDeclarableCombinations reports whether the hand contains any valid declaration.
func hasDeclarableCombinations(hand []Card, overlap bool) bool {
	return len(detectDeclarations(hand, overlap)) > 0
}

// HasDeclarableCombinations reports whether the seat holds anything it could declare under the game's overlap rule.
func HasDeclarableCombinations(state *GameState, seat int) bool {
	if seat < 0 || seat > 3 {
		return false
	}
	return hasDeclarableCombinations(state.Players[seat].Hand, state.Rules.DeclarationOverlap)
}

// resolveDeclarations picks the team holding the single strongest declaration and returns it with the sum of that team's declarations.
// It returns -1 and 0 when nobody declared.
func resolveDeclarations(players [4]PlayerState, trumpSuit Suit, trickLeaderSeat int, rules VariantRules) (winningTeam int, totalPoints int) {
	type teamBest struct {
		decl       *Declaration
		playerSeat int
	}

	var bestByTeam [2]*teamBest

	for seat := 0; seat < 4; seat++ {
		team := TeamForSeat(seat)
		for i := range players[seat].Declarations {
			d := &players[seat].Declarations[i]
			current := bestByTeam[team]
			if current == nil || declarationBeats(d, seat, current.decl, current.playerSeat, trumpSuit, trickLeaderSeat, rules) {
				bestByTeam[team] = &teamBest{decl: d, playerSeat: seat}
			}
		}
	}

	if bestByTeam[0] == nil && bestByTeam[1] == nil {
		return -1, 0
	}

	if bestByTeam[0] == nil {
		return 1, teamDeclarationTotal(players, 1)
	}
	if bestByTeam[1] == nil {
		return 0, teamDeclarationTotal(players, 0)
	}

	b0, b1 := bestByTeam[0], bestByTeam[1]
	if declarationBeats(b0.decl, b0.playerSeat, b1.decl, b1.playerSeat, trumpSuit, trickLeaderSeat, rules) {
		return 0, teamDeclarationTotal(players, 0)
	}
	return 1, teamDeclarationTotal(players, 1)
}

// declarationBeats returns true if declaration a (from seatA) beats
// declaration b (from seatB) using the full tiebreaker chain.
func declarationBeats(a *Declaration, seatA int, b *Declaration, seatB int, trumpSuit Suit, trickLeaderSeat int, rules VariantRules) bool {
	if a.Value != b.Value {
		return a.Value > b.Value
	}

	if a.Type != b.Type {
		return a.Type == DeclarationFourOfAKind
	}

	if a.Type == DeclarationSequence && b.Type == DeclarationSequence {
		topA := sequenceTopCard(a.Cards)
		topB := sequenceTopCard(b.Cards)
		orderA := NaturalRankOrder[topA]
		orderB := NaturalRankOrder[topB]
		if orderA != orderB {
			return orderA > orderB
		}

		suitA := a.Cards[0].Suit
		suitB := b.Cards[0].Suit
		if (suitA == trumpSuit) != (suitB == trumpSuit) {
			return suitA == trumpSuit
		}
	}

	return rules.PlayDistance(trickLeaderSeat, seatA) < rules.PlayDistance(trickLeaderSeat, seatB)
}

// sequenceTopCard returns the highest-ranked card in a sequence.
func sequenceTopCard(cards []Card) Rank {
	best := cards[0].Rank
	for _, c := range cards[1:] {
		if NaturalRankOrder[c.Rank] > NaturalRankOrder[best] {
			best = c.Rank
		}
	}
	return best
}

// teamDeclarationTotal sums all declaration point values for the given team.
func teamDeclarationTotal(players [4]PlayerState, team int) int {
	total := 0
	for seat := 0; seat < 4; seat++ {
		if TeamForSeat(seat) == team {
			for _, d := range players[seat].Declarations {
				total += d.Value
			}
		}
	}
	return total
}

// handleDeclaring dispatches declare and skip_declare during PhaseDeclaring and rejects everything else.
func handleDeclaring(state *GameState, action Action) (*GameState, error) {
	if state.Phase != PhaseDeclaring {
		return nil, apperr.ErrWrongPhase
	}

	switch action.Type {
	case ActionDeclare:
		return handleDeclare(state, action)
	case ActionSkipDeclare:
		return handleSkipDeclare(state, action)
	default:
		return nil, apperr.ErrWrongPhase
	}
}

// openDeclarationPhase enters PhaseDeclaring, where every seat answers at once so nobody learns who holds a meld.
// ActivePlayerSeat is pinned to the trick-1 leader and TrickNumber stays 0.
func openDeclarationPhase(state *GameState) {
	state.Phase = PhaseDeclaring
	state.TrickNumber = 0
	state.CurrentTrick = []TrickCard{}
	state.AwaitingDeclaration = false
	state.ActivePlayerSeat = state.Rules.NextSeat(state.DealerSeat)
	for i := range state.Players {
		state.Players[i].DeclarationAnswered = false
	}
}

// allDeclarationsAnswered reports whether every connected seat has answered the declaration phase.
func allDeclarationsAnswered(state *GameState) bool {
	for i := range state.Players {
		if state.Players[i].Connected && !state.Players[i].DeclarationAnswered {
			return false
		}
	}
	return true
}

// maybeCloseDeclarationPhase closes the declaration phase once every connected seat has answered.
func maybeCloseDeclarationPhase(state *GameState) {
	if !allDeclarationsAnswered(state) {
		return
	}
	closeDeclarationPhase(state)
}

// closeDeclarationPhase resolves the declaration contest and opens trick 1 unconditionally.
func closeDeclarationPhase(state *GameState) {
	resolveDeclarationsForHand(state)

	for i := range state.Players {
		state.Players[i].DeclarationAnswered = false
	}
	state.AwaitingDeclaration = false
	state.Phase = PhasePlaying
	state.ActivePlayerSeat = state.Rules.NextSeat(state.DealerSeat)
	state.TrickNumber = 1
	state.CurrentTrick = []TrickCard{}

	if stopAtTargetIfReached(state) {
		state.TrickNumber = 0
	}
}

// ForceCloseDeclarationPhase resolves the declaration contest and opens trick 1, treating unanswered seats as skipped.
// The session manager calls it when the phase's fixed window elapses.
func ForceCloseDeclarationPhase(state *GameState) (*GameState, error) {
	if state.Phase != PhaseDeclaring {
		return nil, apperr.ErrWrongPhase
	}
	newState := cloneGameState(state)
	closeDeclarationPhase(newState)
	RefreshDerivedFlags(newState)
	return newState, nil
}

// handleDeclare detects and stores the player's declarations at trick 1 or during PhaseDeclaring.
func handleDeclare(state *GameState, action Action) (*GameState, error) {
	if err := checkDeclarationAnswerable(state, action.PlayerSeat); err != nil {
		return nil, err
	}

	hand := state.Players[action.PlayerSeat].Hand
	decls := detectDeclarations(hand, state.Rules.DeclarationOverlap)
	if len(decls) == 0 {
		return nil, apperr.ErrDeclarationNotAvailable
	}

	newState := cloneGameState(state)

	for i := range decls {
		decls[i].PlayerSeat = action.PlayerSeat
	}
	newState.Players[action.PlayerSeat].Declarations = decls
	newState.AwaitingDeclaration = false

	if newState.Phase == PhaseDeclaring {
		newState.Players[action.PlayerSeat].DeclarationAnswered = true
		maybeCloseDeclarationPhase(newState)
	}

	return newState, nil
}

// handleSkipDeclare records a skip_declare at trick 1 or during PhaseDeclaring.
func handleSkipDeclare(state *GameState, action Action) (*GameState, error) {
	if err := checkDeclarationAnswerable(state, action.PlayerSeat); err != nil {
		return nil, err
	}

	newState := cloneGameState(state)
	newState.AwaitingDeclaration = false

	if newState.Phase == PhaseDeclaring {
		newState.Players[action.PlayerSeat].DeclarationAnswered = true
		maybeCloseDeclarationPhase(newState)
	}

	return newState, nil
}

// checkDeclarationAnswerable reports whether seat may declare or skip now.
// In PhaseDeclaring any unanswered seat may answer; at trick 1 only the prompted active seat may.
func checkDeclarationAnswerable(state *GameState, seat int) error {
	if seat < 0 || seat > 3 {
		return apperr.ErrNotYourTurn
	}

	if state.Phase == PhaseDeclaring {
		if state.Players[seat].DeclarationAnswered {
			return apperr.ErrWrongPhase
		}
		return nil
	}

	if state.TrickNumber != 1 {
		return apperr.ErrWrongPhase
	}
	if !state.AwaitingDeclaration {
		return apperr.ErrWrongPhase
	}
	if seat != state.ActivePlayerSeat {
		return apperr.ErrNotYourTurn
	}
	return nil
}

// hasBelot returns true if the hand contains both K and Q of the given trump suit.
func hasBelot(hand []Card, trumpSuit Suit) bool {
	hasKing := false
	hasQueen := false
	for _, c := range hand {
		if c.Suit == trumpSuit {
			if c.Rank == RankKing {
				hasKing = true
			}
			if c.Rank == RankQueen {
				hasQueen = true
			}
		}
	}
	return hasKing && hasQueen
}

// shouldPromptBelot returns true if the played card triggers a Belot announcement prompt.
// The hand parameter must be the player's hand BEFORE the card was removed.
func shouldPromptBelot(state *GameState, playedCard Card, handBeforePlay []Card) bool {
	if !state.Rules.DeclarationsEnabled {
		return false
	}
	if state.TrumpSuit == nil || state.BelotAnnounced {
		return false
	}
	trumpSuit := *state.TrumpSuit

	if playedCard.Suit != trumpSuit {
		return false
	}
	if playedCard.Rank != RankKing && playedCard.Rank != RankQueen {
		return false
	}

	return hasBelot(handBeforePlay, trumpSuit)
}

// handleAnnounceBelot awards 20 points to the announcing player's team.
// The target check is deferred while a trick-1 declaration contest is still open, so declared melds are scored first.
func handleAnnounceBelot(state *GameState, action Action) (*GameState, error) {
	if state.PendingBelotSeat == nil || *state.PendingBelotSeat != action.PlayerSeat {
		return nil, apperr.ErrBelotNotAvailable
	}

	newState := cloneGameState(state)
	team := TeamForSeat(action.PlayerSeat)
	newState.BelotPoints[team] += 20
	newState.BelotAnnounced = true
	newState.PendingBelotSeat = nil

	deferForOpenTrick1Contest := newState.TrickNumber == 1 && !newState.DeclarationsResolved
	if !deferForOpenTrick1Contest && stopAtTargetIfReached(newState) {
		return newState, nil
	}

	finishCardPlay(newState)

	return newState, nil
}

// handleSkipBelot processes a skip_belot action.
func handleSkipBelot(state *GameState, action Action) (*GameState, error) {
	if state.PendingBelotSeat == nil || *state.PendingBelotSeat != action.PlayerSeat {
		return nil, apperr.ErrBelotNotAvailable
	}

	newState := cloneGameState(state)
	newState.PendingBelotSeat = nil

	finishCardPlay(newState)

	return newState, nil
}

// finishCardPlay completes the deferred post-card-play flow after Belot resolution.
// Advances the active player and resolves the trick if 4 cards have been played.
func finishCardPlay(state *GameState) {
	lastCard := state.CurrentTrick[len(state.CurrentTrick)-1]
	state.ActivePlayerSeat = state.Rules.NextSeat(lastCard.PlayerSeat)

	if state.TrickNumber == 1 && len(state.CurrentTrick) < 4 {
		checkDeclarationPrompt(state)
	}

	if len(state.CurrentTrick) == 4 {
		resolveTrickWithDeclarations(state)
	}
}

// resolveTrickWithDeclarations resolves a trick and, if it's trick 1,
// also resolves declarations.
func resolveTrickWithDeclarations(state *GameState) {
	resolveTrick(state)

	if state.TrickNumber == 2 && !state.DeclarationsResolved {
		resolveDeclarationsForHand(state)
	}
	if state.Phase == PhaseHandScoring && !state.DeclarationsResolved {
		resolveDeclarationsForHand(state)
	}

	if stopAtTargetIfReached(state) {
		return
	}

	if state.Phase == PhaseHandScoring {
		scoreHand(state)
	}
}

// bothTeamsDeclared reports whether each team put at least one meld on the table.
// It must be read before resolveDeclarationsForHand clears the losing team's declarations.
func bothTeamsDeclared(state *GameState) bool {
	var teamA, teamB bool
	for seat := 0; seat < 4; seat++ {
		if len(state.Players[seat].Declarations) == 0 {
			continue
		}
		if TeamForSeat(seat) == TeamA {
			teamA = true
		} else {
			teamB = true
		}
	}
	return teamA && teamB
}

// resolveDeclarationsForHand resolves declarations after trick 1 and awards points.
func resolveDeclarationsForHand(state *GameState) {
	state.DeclarationsContested = bothTeamsDeclared(state)

	trickLeaderSeat := state.Rules.NextSeat(state.DealerSeat)

	winningTeam, totalPoints := resolveDeclarations(state.Players, *state.TrumpSuit, trickLeaderSeat, state.Rules)
	if winningTeam >= 0 {
		state.DeclarationPoints[winningTeam] = totalPoints
		losingTeam := 1 - winningTeam
		for seat := 0; seat < 4; seat++ {
			if TeamForSeat(seat) == losingTeam {
				state.Players[seat].Declarations = nil
			}
		}
	}
	state.DeclarationsResolved = true
}

// checkDeclarationPrompt sets AwaitingDeclaration if it is trick 1 and the active player has declarable combinations.
func checkDeclarationPrompt(state *GameState) {
	if state.TrickNumber != 1 || state.DeclarationsResolved {
		return
	}
	seat := state.ActivePlayerSeat
	if len(state.Players[seat].Declarations) > 0 {
		return
	}
	if hasDeclarableCombinations(state.Players[seat].Hand, state.Rules.DeclarationOverlap) {
		state.AwaitingDeclaration = true
	}
}
