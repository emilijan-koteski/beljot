package game

import (
	"sort"

	"github.com/emilijan/beljot/server/internal/apperr"
)

// sequencePoints maps sequence length to point value.
var sequencePoints = map[int]int{
	3: 20,
	4: 50,
	// 5+ = 100 (handled in code)
}

// fourOfAKindPoints maps rank to point value for four-of-a-kind declarations.
// Only ranks with non-zero card points are declarable (no 4×7 or 4×8).
var fourOfAKindPoints = map[Rank]int{
	RankJack:  200,
	Rank9:     150,
	RankAce:   100,
	RankTen:   100,
	RankKing:  100,
	RankQueen: 100,
}

// detectDeclarations scans a player's hand for all valid declarations.
// Returns sequences and four-of-a-kind combinations with their point values.
// Longer sequences subsume shorter subsequences within them.
//
// overlap comes from VariantRules.DeclarationOverlap and must always be passed
// explicitly by the caller from the game state's resolved config — never
// defaulted, and never derived from the variant name. When it is true a single
// card may count toward more than one declaration, so the one-card-one-group
// dedup is skipped and every detected meld survives.
func detectDeclarations(hand []Card, overlap bool) []Declaration {
	var decls []Declaration

	// --- Sequences: consecutive ranks of the same suit ---
	// Group cards by suit
	bySuit := map[Suit][]Card{}
	for _, c := range hand {
		bySuit[c.Suit] = append(bySuit[c.Suit], c)
	}

	// AllSuits order, never the map's: Go randomizes map iteration per run, and
	// the order melds are appended in is the order they reach the wire, the
	// declaration-contract golden, and the client's reveal panel. Iterating
	// bySuit directly made TestVariantRulesAndDeclarationsContract fail roughly
	// one run in four on any hand holding two melds.
	for _, suit := range AllSuits {
		cards := bySuit[suit]
		if len(cards) < 3 {
			continue
		}
		// Sort by natural rank order
		sort.Slice(cards, func(i, j int) bool {
			return NaturalRankOrder[cards[i].Rank] < NaturalRankOrder[cards[j].Rank]
		})

		// Find maximal consecutive sequences
		seqStart := 0
		for i := 1; i <= len(cards); i++ {
			consecutive := i < len(cards) &&
				NaturalRankOrder[cards[i].Rank] == NaturalRankOrder[cards[i-1].Rank]+1
			if !consecutive {
				seqLen := i - seqStart
				if seqLen >= 3 {
					seqCards := make([]Card, seqLen)
					copy(seqCards, cards[seqStart:i])
					pts := 100 // 5+
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

	// --- Four-of-a-kind: player holds all 4 suits of the same rank ---
	byRank := map[Rank][]Card{}
	for _, c := range hand {
		byRank[c.Rank] = append(byRank[c.Rank], c)
	}
	// AllRanks order, for the same reason as AllSuits above: a hand with two
	// four-of-a-kinds must emit them in a fixed order.
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

	// A card may participate in several declarations under this config, so
	// every detected meld stands on its own.
	if overlap {
		return decls
	}
	return dedupOneCardOneGroup(decls)
}

// dedupOneCardOneGroup applies the one-card-one-group rule (the behaviour
// VariantRules.DeclarationOverlap=false selects). Among
// declarations that share at least one card, the highest-Value one is kept
// and the rest are dropped. Stable — original order is preserved among
// survivors.
//
// Equal-Value ties keep the four-of-a-kind, matching declarationBeats rule 2,
// so the survivor is the same meld the clash comparison would have preferred.
// Rule 2 alone settles every reachable tie here: overlap is only possible
// between a sequence and a four-of-a-kind (sequences are maximal per-suit
// runs, so two sequences never share a card; four-of-a-kinds are
// rank-disjoint), and the later chain steps need trump and seat, neither of
// which is known at detection time.
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

// hasDeclarableCombinations returns true if the hand contains any valid
// declarations. overlap has the same meaning as in detectDeclarations; the
// predicate is in fact invariant under it (dedup keeps at least one meld from
// any non-empty set), but the parameter is threaded through so the two
// functions cannot drift apart.
func hasDeclarableCombinations(hand []Card, overlap bool) bool {
	return len(detectDeclarations(hand, overlap)) > 0
}

// HasDeclarableCombinations reports whether the seat holds anything it could
// declare, under the overlap rule this game's config resolved. Exported for the
// bot-view builder: in the dedicated declaration phase every seat is asked, so a
// bot must know whether declare is legal for it before choosing between declare
// and skip — the engine rejects a declare from an empty hand, and a rejected bot
// action re-arms into a reschedule loop.
//
// Reads the rule off the state's own config, never the variant name (D-VAR-1),
// so the bot receives a consequence and never learns which variant it plays.
func HasDeclarableCombinations(state *GameState, seat int) bool {
	if seat < 0 || seat > 3 {
		return false
	}
	return hasDeclarableCombinations(state.Players[seat].Hand, state.Rules.DeclarationOverlap)
}

// resolveDeclarations compares all players' declarations after trick 1.
// Returns winning team index (0=team A, 1=team B) and total declaration points
// for the winning team. Returns -1 and 0 if no declarations exist.
//
// The winner is the team holding the SINGLE strongest declaration on the
// table — never the team with the larger meld sum. Once the winning team is
// chosen, the awarded total is the sum of that team's declarations only;
// the losing team scores 0 regardless of how many melds they held.
// Belot (K+Q of trump) is awarded separately via handleAnnounceBelot and
// does not flow through this function.
//
// Resolution rules (applied to each team's strongest meld via declarationBeats):
// 1. Pick each team's single strongest declaration
// 2. Higher point value wins; on tie, four-of-a-kind beats sequence
// 3. On tie among sequences: higher top card (natural rank order, J > 10) wins
// 4. On tie: trump-suit sequence beats non-trump
// 5. On tie: team whose declaring player is earlier in play order wins
// 6. Winning team scores the sum of ALL their declarations
func resolveDeclarations(players [4]PlayerState, trumpSuit Suit, trickLeaderSeat int) (winningTeam int, totalPoints int) {
	// Collect best declaration per team
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
			if current == nil || declarationBeats(d, seat, current.decl, current.playerSeat, trumpSuit, trickLeaderSeat) {
				bestByTeam[team] = &teamBest{decl: d, playerSeat: seat}
			}
		}
	}

	// No declarations at all
	if bestByTeam[0] == nil && bestByTeam[1] == nil {
		return -1, 0
	}

	// One team has declarations, the other doesn't
	if bestByTeam[0] == nil {
		return 1, teamDeclarationTotal(players, 1)
	}
	if bestByTeam[1] == nil {
		return 0, teamDeclarationTotal(players, 0)
	}

	// Both teams have declarations — compare best
	b0, b1 := bestByTeam[0], bestByTeam[1]
	if declarationBeats(b0.decl, b0.playerSeat, b1.decl, b1.playerSeat, trumpSuit, trickLeaderSeat) {
		return 0, teamDeclarationTotal(players, 0)
	}
	return 1, teamDeclarationTotal(players, 1)
}

// declarationBeats returns true if declaration a (from seatA) beats
// declaration b (from seatB) using the full tiebreaker chain.
func declarationBeats(a *Declaration, seatA int, b *Declaration, seatB int, trumpSuit Suit, trickLeaderSeat int) bool {
	// 1. Higher value wins
	if a.Value != b.Value {
		return a.Value > b.Value
	}

	// 2. Four-of-a-kind beats sequence at equal value
	if a.Type != b.Type {
		return a.Type == DeclarationFourOfAKind
	}

	// 3. For equal-value sequences: higher top card wins. Sequences rank by the
	// NATURAL declaration order (7<8<9<10<J<Q<K<A) — NOT the trick-taking order,
	// where Ten outranks Jack. In a meld, Jack outranks Ten.
	if a.Type == DeclarationSequence && b.Type == DeclarationSequence {
		topA := sequenceTopCard(a.Cards)
		topB := sequenceTopCard(b.Cards)
		orderA := NaturalRankOrder[topA]
		orderB := NaturalRankOrder[topB]
		if orderA != orderB {
			return orderA > orderB
		}

		// 4. Trump suit sequence wins
		suitA := a.Cards[0].Suit
		suitB := b.Cards[0].Suit
		if (suitA == trumpSuit) != (suitB == trumpSuit) {
			return suitA == trumpSuit
		}
	}

	// 5. Earlier in play order from trick leader wins
	distA := (seatA - trickLeaderSeat + 4) % 4
	distB := (seatB - trickLeaderSeat + 4) % 4
	return distA < distB
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

// handleDeclaring processes the actions valid inside PhaseDeclaring, the
// dedicated declaration phase Croatian runs between bidding and trick 1.
// declare and skip_declare are the only ones — play_card in particular is
// rejected, since no trick is open yet.
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

// openDeclarationPhase enters PhaseDeclaring from a resolved bid. Every seat is
// asked AT ONCE and answers independently; there is no cursor and no seat on the
// clock.
//
// That simultaneity is the whole point, not a convenience. The one-at-a-time
// cursor this replaced prompted only meld-holding seats, so ActivePlayerSeat
// during the phase named exactly the seats holding melds and the table learned
// who had one before they chose — which is the secret skipping is supposed to
// keep. Here every seat answers whatever it holds, so nothing on the wire
// separates a meld holder from anyone else until the reveal.
//
// ActivePlayerSeat is pinned to (DealerSeat+1)%4 — where it must land for trick
// 1 anyway — and stays there for the phase's whole duration. It is a positional
// constant, uncorrelated with anyone's cards, which keeps every path that
// assumes a 0-3 seat working without a sentinel. Clients suppress the
// active-seat highlight on the phase, since no one is on the clock.
//
// TrickNumber is deliberately 0: no trick is open, and Bitola's trick-1
// declaration path (checkDeclarationPrompt, resolveTrickWithDeclarations) is
// guarded on TrickNumber == 1, so it can never engage here.
//
// AwaitingDeclaration stays false throughout — it is Bitola's "this seat is
// being asked" flag, and there is no such seat here.
func openDeclarationPhase(state *GameState) {
	state.Phase = PhaseDeclaring
	state.TrickNumber = 0
	state.CurrentTrick = []TrickCard{}
	state.AwaitingDeclaration = false
	state.ActivePlayerSeat = (state.DealerSeat + 1) % 4
	for i := range state.Players {
		state.Players[i].DeclarationAnswered = false
	}
}

// allDeclarationsAnswered reports whether every CONNECTED seat has answered the
// dedicated declaration phase. Disconnected seats are excluded for the same
// reason allConnectedReady excludes them from the score-reveal pause: a dropped
// player must not hold the table hostage.
//
// On the ORDINARY disconnect path this exclusion is belt-and-braces rather than
// load-bearing: HandleDisconnect moves the whole table to PhaseDisconnected, so
// no answer reaches this gate until the seat is back. It earns its keep on the
// concurrent-disconnect path, where a second seat is marked Connected=false
// while the phase itself is restored and answers keep flowing.
func allDeclarationsAnswered(state *GameState) bool {
	for i := range state.Players {
		if state.Players[i].Connected && !state.Players[i].DeclarationAnswered {
			return false
		}
	}
	return true
}

// maybeCloseDeclarationPhase ends the phase once every connected seat has
// answered, and does nothing otherwise.
//
// The contest resolves through the SAME resolveDeclarationsForHand Bitola uses
// at trick 2 (DeclarationsResolved flips false→true here, so the match layer's
// fire-once reveal latch works unchanged) and the game moves straight to
// PhasePlaying at trick 1 — the reveal panel floats over live play exactly as it
// does in Bitola.
//
// Answer ORDER cannot affect the outcome: resolveDeclarations breaks ties on
// trickLeaderSeat, a positional fact, never on who spoke first.
func maybeCloseDeclarationPhase(state *GameState) {
	if !allDeclarationsAnswered(state) {
		return
	}
	closeDeclarationPhase(state)
}

// closeDeclarationPhase resolves the contest and opens trick 1 unconditionally.
// Split out from maybeCloseDeclarationPhase so the session manager's fixed
// window can force it through ForceCloseDeclarationPhase without re-testing a
// gate it has already decided to override.
func closeDeclarationPhase(state *GameState) {
	resolveDeclarationsForHand(state)

	for i := range state.Players {
		state.Players[i].DeclarationAnswered = false
	}
	state.AwaitingDeclaration = false
	state.Phase = PhasePlaying
	state.ActivePlayerSeat = (state.DealerSeat + 1) % 4
	state.TrickNumber = 1
	state.CurrentTrick = []TrickCard{}
}

// ForceCloseDeclarationPhase resolves the declaration contest and opens trick 1
// regardless of who has answered — every unanswered seat is treated as having
// skipped. The session manager calls it when the phase's fixed window elapses,
// so an absent or idle seat cannot stall the table.
//
// Mirrors ForceAdvanceHandComplete, including its bypass of the action
// dispatcher: this is a server-initiated advance, not a player action.
func ForceCloseDeclarationPhase(state *GameState) (*GameState, error) {
	if state.Phase != PhaseDeclaring {
		return nil, apperr.ErrWrongPhase
	}
	newState := cloneGameState(state)
	closeDeclarationPhase(newState)
	// Every bypass of the action dispatcher owes this: ApplyAction refreshes the
	// derived wire flags on its way out, and a direct Phase write does not.
	RefreshDerivedFlags(newState)
	return newState, nil
}

// handleDeclare processes a declare action — at trick 1 under
// DeclarationTimingDuringFirstTrick, or inside PhaseDeclaring under
// DeclarationTimingDedicatedPhase.
// Auto-detects all declarations from the player's hand and stores them.
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

	// Set player seat on each declaration
	for i := range decls {
		decls[i].PlayerSeat = action.PlayerSeat
	}
	newState.Players[action.PlayerSeat].Declarations = decls
	newState.AwaitingDeclaration = false

	// In the dedicated phase nothing else ends the phase — Bitola relies on the
	// same seat's following play_card, which does not exist here.
	if newState.Phase == PhaseDeclaring {
		newState.Players[action.PlayerSeat].DeclarationAnswered = true
		maybeCloseDeclarationPhase(newState)
	}

	return newState, nil
}

// handleSkipDeclare processes a skip_declare action — at trick 1 under
// DeclarationTimingDuringFirstTrick, or inside PhaseDeclaring under
// DeclarationTimingDedicatedPhase.
func handleSkipDeclare(state *GameState, action Action) (*GameState, error) {
	if err := checkDeclarationAnswerable(state, action.PlayerSeat); err != nil {
		return nil, err
	}

	newState := cloneGameState(state)
	newState.AwaitingDeclaration = false

	// See handleDeclare: the dedicated phase owns its own close.
	if newState.Phase == PhaseDeclaring {
		newState.Players[action.PlayerSeat].DeclarationAnswered = true
		maybeCloseDeclarationPhase(newState)
	}

	return newState, nil
}

// checkDeclarationAnswerable is the shared admission test for declare and
// skip_declare, and the ONE place the two declaration timings diverge on who
// may speak.
//
//   - Dedicated phase (Croatian): any of the four seats may answer, in any
//     order, at any time while the phase is open — that is what makes the phase
//     simultaneous. A seat that has already answered is rejected, so a stale
//     click or a reconnected client cannot overwrite its own skip with a
//     declare after seeing the table sit still.
//   - Trick 1 (Bitola): unchanged — the prompted seat is the active seat and
//     AwaitingDeclaration must be set.
//
// Selected by the phase already on state, never by the variant name (D-VAR-1).
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
	// A room playing without declarations has no Belote either — the +20 for
	// holding K+Q of trump is an announcement, and a table that plays "bez
	// zvanja" plays without it. Unlike the meld skip this needs its own check:
	// Belote is not a meld, so DeclarationsResolved does not gate it.
	if !state.Rules.DeclarationsEnabled {
		return false
	}
	if state.TrumpSuit == nil || state.BelotAnnounced {
		return false
	}
	trumpSuit := *state.TrumpSuit

	// Card must be K or Q of trump
	if playedCard.Suit != trumpSuit {
		return false
	}
	if playedCard.Rank != RankKing && playedCard.Rank != RankQueen {
		return false
	}

	// Player must have held both K and Q before playing
	return hasBelot(handBeforePlay, trumpSuit)
}

// handleAnnounceBelot processes an announce_belot action.
// Awards 20 points to the announcing player's team.
func handleAnnounceBelot(state *GameState, action Action) (*GameState, error) {
	if state.PendingBelotSeat == nil || *state.PendingBelotSeat != action.PlayerSeat {
		return nil, apperr.ErrBelotNotAvailable
	}

	newState := cloneGameState(state)
	team := TeamForSeat(action.PlayerSeat)
	// Belote is a declaration, not card points — tracked in BelotPoints so it
	// surfaces under declarations everywhere (it still counts toward the team's
	// hand total and transfers on a failed contract / capot like any point).
	newState.BelotPoints[team] += 20
	newState.BelotAnnounced = true
	newState.PendingBelotSeat = nil

	// Resume deferred turn flow
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

	// Resume deferred turn flow
	finishCardPlay(newState)

	return newState, nil
}

// finishCardPlay completes the deferred post-card-play flow after Belot resolution.
// Advances the active player and resolves the trick if 4 cards have been played.
func finishCardPlay(state *GameState) {
	// Advance active player (was deferred during Belot prompt)
	// The seat stored in the trick's last card is the player who just played.
	lastCard := state.CurrentTrick[len(state.CurrentTrick)-1]
	state.ActivePlayerSeat = (lastCard.PlayerSeat + 1) % 4

	// Check declaration prompt for next player at trick 1
	if state.TrickNumber == 1 && len(state.CurrentTrick) < 4 {
		checkDeclarationPrompt(state)
	}

	// Resolve trick if 4 cards have been played
	if len(state.CurrentTrick) == 4 {
		resolveTrickWithDeclarations(state)
	}
}

// resolveTrickWithDeclarations resolves a trick and, if it's trick 1,
// also resolves declarations.
func resolveTrickWithDeclarations(state *GameState) {
	resolveTrick(state)

	// After trick 1 resolves, resolve declarations
	if state.TrickNumber == 2 && !state.DeclarationsResolved {
		// We just incremented from trick 1 to trick 2 in resolveTrick
		resolveDeclarationsForHand(state)
	}
	// Handle the edge case where trick 1 was the 8th trick (impossible in standard
	// Belot since trickNumber starts at 1 and goes to 8, but for safety):
	if state.Phase == PhaseHandScoring && !state.DeclarationsResolved {
		resolveDeclarationsForHand(state)
	}

	// After all tricks complete, score the hand and start next hand (or end match).
	// PhaseHandScoring is only set by resolveTrick when TrickNumber == 8.
	if state.Phase == PhaseHandScoring {
		scoreHand(state)
	}
}

// bothTeamsDeclared reports whether each team put at least one meld on the
// table — the only condition under which the winner was decided by a
// comparison rather than by being the sole declarer.
//
// Must be read BEFORE resolveDeclarationsForHand clears the losing team's
// declarations, which is why its single caller is that function's first
// statement.
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
	// Record whether a comparison decided this contest while BOTH teams' melds
	// are still on the table. Everything below destroys the evidence: the
	// losing team's declarations are cleared a few lines down, and under
	// DeclarationTimingDedicatedPhase the seat whose answer triggered this call
	// had its melds stored moments ago in the SAME ApplyAction — so a caller
	// comparing the pre- and post-action states cannot see them at all.
	state.DeclarationsContested = bothTeamsDeclared(state)

	// Determine trick 1 leader: the player after the dealer
	trickLeaderSeat := (state.DealerSeat + 1) % 4

	winningTeam, totalPoints := resolveDeclarations(state.Players, *state.TrumpSuit, trickLeaderSeat)
	if winningTeam >= 0 {
		state.DeclarationPoints[winningTeam] = totalPoints
		// Clear losing team's declarations
		losingTeam := 1 - winningTeam
		for seat := 0; seat < 4; seat++ {
			if TeamForSeat(seat) == losingTeam {
				state.Players[seat].Declarations = nil
			}
		}
	}
	state.DeclarationsResolved = true
}

// checkDeclarationPrompt sets AwaitingDeclaration if the current active player
// has declarable combinations and it's trick 1.
//
// This is also the Bitola half of the declarations-off skip, and it needs no
// check of its own: a room with Rules.DeclarationsEnabled false starts every
// hand with DeclarationsResolved already true (NewGame and startNewHand seed
// it), so the guard below returns before a seat is ever prompted. Do not add a
// redundant config read here — the seeded flag is the single mechanism, and two
// mechanisms would be two things to keep in step.
func checkDeclarationPrompt(state *GameState) {
	if state.TrickNumber != 1 || state.DeclarationsResolved {
		return
	}
	seat := state.ActivePlayerSeat
	// Check if this player already has declarations stored (already declared)
	if len(state.Players[seat].Declarations) > 0 {
		return
	}
	if hasDeclarableCombinations(state.Players[seat].Hand, state.Rules.DeclarationOverlap) {
		state.AwaitingDeclaration = true
	}
}
