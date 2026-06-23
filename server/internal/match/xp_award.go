package match

import (
	"log/slog"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/ws"
)

// xpPerGamePointDivisor converts a team's accumulated match game points into XP:
// xpEarned = floor(teamGamePoints / xpPerGamePointDivisor). Placeholder constant
// (Story 9.5 Design Decision D3), kept named so a future tuning pass is one line.
const xpPerGamePointDivisor = 10

// abandonPartialXPFactor scales the NON-abandoning team's XP on an abandonment.
// It defaults to 1.0 ("points-so-far" — exactly the normal-end formula; partial-
// ness emerges from the lower point total at the early end). The abandoning team
// always forfeits to 0 regardless of this factor (Story 9.5 Design Decision D4).
// A future discount on the non-abandoning team's XP is a one-number change here.
const abandonPartialXPFactor = 1.0

// xpAwardMsg is a prepared per-human event:xp_awarded broadcast. Built during
// awardXP but SENT by the finalize path AFTER the event:coin_settlement loop and
// BEFORE the trailing event:match_state, preserving the Story 8.5-1 ordering
// contract (match_end/abandoned → coin_settlement → xp_awarded → match_state).
// Mirrors coinSettlementMsg in settlement.go.
type xpAwardMsg struct {
	userID uint
	msg    []byte
}

// computeXPAwards returns the per-seat lifetime-XP delta for a finished match
// (Story 9.5). Pure + table-tested. Bot seats and empty seats always earn 0
// (the exact guard from settlement.go).
//
// Normal end (abandonedSeat == -1): each human seat earns
// floor(teamScores[team] / xpPerGamePointDivisor) — both teammates get the same
// amount and the LOSING team still earns (XP is a participation reward, not
// zero-sum).
//
// Abandonment (abandonedSeat >= 0): the WHOLE abandoning team forfeits all XP
// (PO override 2026-06-22 — mirrors coin settlement's team-based forfeit), and
// the non-abandoning team earns the normal-end amount scaled by
// abandonPartialXPFactor. At the default factor 1.0 this is one code path:
// compute the normal-end XP for every seat, then zero the abandoning team's two
// seats. (A normal LOSS still earns XP — abandonment is the punishment.)
func computeXPAwards(playerIDs [4]uint, botSeats [4]bool, teamScores [2]int, abandonedSeat int) (deltas [4]int) {
	for seat := 0; seat < 4; seat++ {
		if botSeats[seat] || playerIDs[seat] == 0 {
			continue
		}
		team := game.TeamForSeat(seat)
		base := teamScores[team] / xpPerGamePointDivisor

		if abandonedSeat >= 0 {
			if game.TeamForSeat(seat) == game.TeamForSeat(abandonedSeat) {
				// Abandoning team forfeits all XP.
				continue
			}
			// Non-abandoning team: points-so-far, scaled by the partial factor.
			deltas[seat] = int(float64(base) * abandonPartialXPFactor)
			continue
		}

		deltas[seat] = base
	}
	return deltas
}

// awardXP applies lifetime XP for a finished match and prepares the per-human
// event:xp_awarded messages (Story 9.5). abandonedSeat is -1 for a normal end,
// or the abandoning player's seat for an abandonment (whose whole team then
// forfeits — see computeXPAwards). It is a no-op (no mutation, no messages)
// when no XPAwarder is wired or when every seat's delta is 0.
//
// Mirrors settleMatch's best-effort degradation philosophy: an ApplyXPAwards
// failure is logged and the events are skipped, but the caller still fires
// match_end/match_abandoned and match_state so clients are never stranded.
func (m *Manager) awardXP(roomID uint, playerIDs [4]uint, botSeats [4]bool, teamScores [2]int, abandonedSeat int) []xpAwardMsg {
	if m.xpAwarder == nil {
		return nil
	}

	deltas := computeXPAwards(playerIDs, botSeats, teamScores, abandonedSeat)

	awards := make(map[uint]int)
	for seat := 0; seat < 4; seat++ {
		if botSeats[seat] || playerIDs[seat] == 0 || deltas[seat] == 0 {
			continue
		}
		awards[playerIDs[seat]] = deltas[seat]
	}
	if len(awards) == 0 {
		return nil
	}

	newTotals, err := m.xpAwarder.ApplyXPAwards(awards)
	if err != nil {
		slog.Error("session: failed to apply XP awards", "roomID", roomID, "error", err)
		return nil
	}

	var msgs []xpAwardMsg
	for seat := 0; seat < 4; seat++ {
		uid := playerIDs[seat]
		if botSeats[seat] || uid == 0 || deltas[seat] == 0 {
			continue
		}
		newTotal, ok := newTotals[uid]
		if !ok {
			// Missing from the returned totals — skip rather than push a wrong value.
			continue
		}
		earned := deltas[seat]
		payload := ws.XPAwardedPayload{
			XPEarned:   earned,
			NewTotalXP: newTotal,
			NewLevel:   m.xpAwarder.LevelForXP(newTotal),
			LeveledUp:  m.xpAwarder.LevelForXP(newTotal) > m.xpAwarder.LevelForXP(newTotal-earned),
		}
		msgs = append(msgs, xpAwardMsg{userID: uid, msg: buildMessage(ws.EventXPAwarded, payload)})
	}
	return msgs
}
