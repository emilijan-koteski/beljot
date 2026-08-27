package match

import (
	"log/slog"
	"time"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/ws"
)

// Season Points (SP) awarded at match end (Story 13.1):
//
//	SP = 50 (completion)
//	   + 100 (if the seat's team won)
//	   + floor(teamGamePoints / 10)
//	   + 50 (if a Capot or an instant win occurred anywhere in the match)
//
// Every term is a named const so a retune is a one-place change, the same
// convention xpPerGamePointDivisor and honorHalfLifeDays follow.
const (
	// spCompletionBonus is the flat award for reaching the terminal end. It is
	// what makes SP a PARTICIPATION ladder rather than a pure win counter: a
	// player who finishes and loses still climbs, just slowly.
	spCompletionBonus = 50

	// spWinBonus goes to both seats of the winning team.
	spWinBonus = 100

	// spPerGamePointDivisor converts a team's accumulated match game points into
	// SP: floor(teamGamePoints / spPerGamePointDivisor). Same divisor as XP's,
	// deliberately — the two systems read the same number off the same match, and
	// a divergence here would be a coincidence waiting to confuse someone.
	spPerGamePointDivisor = 10

	// spSpectacularBonus is the flat bonus for a match in which a Capot or an
	// instant win occurred. See computeSPAwards for why it is MATCH-scoped rather
	// than awarded only to the team that earned it.
	spSpectacularBonus = 50
)

// spAwardMsg is a prepared per-human event:season_points_awarded broadcast. Built
// during awardSeasonPoints but SENT by the finalize path AFTER the
// event:honor_updated loop and BEFORE the trailing event:match_state, preserving
// the Story 8.5-1 ordering contract:
//
//	match_end | match_abandoned
//	  -> coin_settlement -> xp_awarded -> honor_updated
//	  -> season_points_awarded -> match_state
//
// Mirrors honorUpdateMsg, xpAwardMsg and coinSettlementMsg.
type spAwardMsg struct {
	userID uint
	msg    []byte
}

// spSeatPresent reports whether a human seat was at the table when the match
// reached its terminal end. It is the SP eligibility gate and, identically, the
// games_completed gate.
//
// It is the SAME RULE computeHonorEvents applies (honor_record.go), read off the
// same `connected` snapshot and deliberately expressed here rather than by
// calling into honor's bucketing — that function returns honor events, not a
// presence answer, and Story 13.1 must not change its behaviour.
//
// A natural end or an accepted surrender (abandonedSeat == -1) reached a real
// terminal state, so every human seat is present by construction. On the
// abandonment path the expired seat is absent by definition (that is why its
// timer fired, so it is checked independently of its flag), and so is any OTHER
// seat sitting inside its own overlapping reconnect window.
//
// The gate FAILS OPEN: `connected` false reliably means absent, but true does NOT
// reliably mean present (HandleDisconnect only maintains it for drops observed in
// four phases — see the note at reconnect.go). So it under-charges rather than
// over-charges, which is the correct direction for a competitive ladder.
func spSeatPresent(connected [4]bool, seat, abandonedSeat int) bool {
	if abandonedSeat < 0 {
		return true
	}
	return seat != abandonedSeat && connected[seat]
}

// computeSPAwards returns the per-seat Season Points delta for a finished match
// (Story 13.1 AC1). Pure + table-tested.
//
// Bot seats and empty seats always earn 0 (the exact guard from settlement.go,
// computeXPAwards and computeHonorEvents).
//
// ABSENCE FORFEITS PER-SEAT, NOT PER-TEAM (Story 13.1 D5). Every human seat
// PRESENT at the terminal end earns the full formula; every ABSENT seat earns 0.
// The abandoner's teammate, if present, STILL EARNS.
//
// This deliberately differs from the two systems this file is modelled on:
//
//	Coins (9.2)   forfeit whole abandoning TEAM
//	XP (9.5)      forfeit whole abandoning TEAM (PO override 2026-06-22)
//	Honor (9.7)   per-SEAT presence
//	SP (13.1)     per-SEAT presence — reuses honor's gate
//
// The epic AC says "abandoning players earn 0 SP" — plural, player-scoped — and
// SP is a competitive ladder: zeroing a present player's ranked progress because
// their partner's network dropped is harsher than the XP case and is not what the
// AC asks for. A team-wide rule here would need an explicit PO override the way
// XP's did.
//
// THE CAPOT / INSTANT-WIN BONUS IS MATCH-LEVEL (Story 13.1 D2). The formula
// scopes its other terms explicitly ("if team won", "team_game_points") and
// pointedly does not scope this one, so it is read literally: if a Capot or an
// instant win happened anywhere in the match, ALL FOUR human seats get +50 —
// winners and losers alike. That is consistent with the established progression
// philosophy ("XP is a participation reward, not zero-sum", xp_award.go), which
// awards the losing team too: a spectacular match rewards the table. It is +50
// ONCE, not +50 per Capot hand.
//
// winnerTeam is the value the FINALIZER already resolved — *finalState.WinnerTeam
// on the natural path (surrender and stop-at-target both route through it with
// the winner already set), or 1 - TeamForSeat(abandonedSeat) on the abandonment
// path. It is never re-derived from scores here.
//
// ON AN INSTANT WIN, TeamScores may be [0,0] and there may be no hand results at
// all: floor(0/10) == 0 is a LEGITIMATE term, not a bug. Winners get
// 50 + 100 + 0 + 50 = 200 and losers 50 + 0 + 0 + 50 = 100.
func computeSPAwards(
	playerIDs [4]uint,
	botSeats [4]bool,
	connected [4]bool,
	teamScores [2]int,
	winnerTeam int,
	capotOrInstantWin bool,
	abandonedSeat int,
) (deltas [4]int) {
	for seat := 0; seat < 4; seat++ {
		if botSeats[seat] || playerIDs[seat] == 0 {
			continue
		}
		if !spSeatPresent(connected, seat, abandonedSeat) {
			// Absent at the terminal end — 0 SP. The seat still counts a
			// games_played (see awardSeasonPoints), which is what makes
			// games_played - games_completed the in-season absence count.
			continue
		}

		team := game.TeamForSeat(seat)
		sp := spCompletionBonus
		if team == winnerTeam {
			sp += spWinBonus
		}
		// Clamped at 0: a team total cannot go negative today, but SP is a
		// monotonic accumulator with a DB CHECK (sp >= 0), so a negative term must
		// never be able to reach the write.
		if points := teamScores[team] / spPerGamePointDivisor; points > 0 {
			sp += points
		}
		if capotOrInstantWin {
			sp += spSpectacularBonus
		}
		deltas[seat] = sp
	}
	return deltas
}

// capotOccurred reports whether any scored hand of the match was a Capot. The
// buffered hand results are the source (each row carries Capot bool), so it is
// "+50 once for the match", not once per Capot hand.
//
// The caller must pass a COPY taken under the session lock — never
// session.handResults read unlocked. See the hoisted snapshot in handleMatchEnd
// and the under-lock snapshot in handleSeatReconnectTimeout.
func capotOccurred(hands []HandResult) bool {
	for _, hr := range hands {
		if hr.Capot {
			return true
		}
	}
	return false
}

// awardSeasonPoints accrues Season Points for a finished match and prepares the
// per-human event:season_points_awarded messages (Story 13.1). It is a no-op (no
// mutation, no messages) when no SPAwarder is wired or when every seat is a bot.
//
// abandonedSeat is -1 for a natural end or an accepted surrender, or the
// abandoning player's seat for an abandonment. connected is the per-seat presence
// snapshot the caller took under the session lock.
//
// `now` is THE FINALIZER'S OWN STAMP, threaded through rather than read here.
// That is the whole point of SPAwarder taking a time: the season a match lands in
// must be decided once, by the code that decided the match was over, not by
// whichever layer happens to call the clock last. Reading it here would have made
// the interface's contract a comment rather than a fact — the clock read would
// just have moved one level down. Contrast recordHonor, which reads the clock
// itself on purpose (honor's decay reference must be the instant of the write).
//
// EVERY human seat is submitted, INCLUDING absent ones with an SP delta of 0:
// games_played counts every human seat in the match while games_completed counts
// only the present ones (Story 13.1 D10), so skipping the zero-delta seats the
// way awardXP skips them would silently lose the absence record.
//
// Mirrors settleMatch's, awardXP's and recordHonor's best-effort degradation: an
// ApplySeasonPoints failure is logged and the events are skipped, but the caller
// still fires match_end / match_abandoned and match_state so clients are never
// stranded on the table.
func (m *Manager) awardSeasonPoints(
	roomID uint,
	playerIDs [4]uint,
	botSeats [4]bool,
	connected [4]bool,
	teamScores [2]int,
	winnerTeam int,
	capotOrInstantWin bool,
	abandonedSeat int,
	now time.Time,
) []spAwardMsg {
	if m.spAwarder == nil {
		return nil
	}

	deltas := computeSPAwards(playerIDs, botSeats, connected, teamScores, winnerTeam, capotOrInstantWin, abandonedSeat)

	awards := make(map[uint]SPAward, 4)
	for seat := 0; seat < 4; seat++ {
		if botSeats[seat] || playerIDs[seat] == 0 {
			continue
		}
		awards[playerIDs[seat]] = SPAward{
			SP:        deltas[seat],
			Completed: spSeatPresent(connected, seat, abandonedSeat),
		}
	}
	if len(awards) == 0 {
		return nil
	}

	snapshots, err := m.spAwarder.ApplySeasonPoints(awards, now)
	if err != nil {
		slog.Error("session: failed to award season points", "roomID", roomID, "error", err)
		return nil
	}

	var msgs []spAwardMsg
	for seat := 0; seat < 4; seat++ {
		uid := playerIDs[seat]
		if botSeats[seat] || uid == 0 {
			continue
		}
		snap, ok := snapshots[uid]
		if !ok {
			// Missing from the returned snapshots — skip rather than push a wrong
			// value (the same rule awardXP's newTotals and recordHonor's snapshots
			// lookups apply).
			continue
		}
		payload := ws.SeasonPointsAwardedPayload{
			SPEarned:    deltas[seat],
			NewSeasonSP: snap.SP,
			RankTier:    snap.RankTier,
			TieredUp:    snap.TieredUp,
			SeasonName:  snap.SeasonName,
		}
		msgs = append(msgs, spAwardMsg{userID: uid, msg: buildMessage(ws.EventSeasonPointsAwarded, payload)})
	}
	return msgs
}
