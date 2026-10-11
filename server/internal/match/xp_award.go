package match

import (
	"log/slog"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/ws"
)

const xpPerGamePointDivisor = 10

const abandonPartialXPFactor = 1.0

// xpAwardMsg is a prepared per-human event:xp_awarded broadcast, sent after
// event:coin_settlement and before event:match_state.
type xpAwardMsg struct {
	userID uint
	msg    []byte
}

// computeXPAwards returns each human seat's XP: its team's score over xpPerGamePointDivisor.
// On abandonment the abandoning team gets 0 and the other team's XP is scaled by abandonPartialXPFactor.
func computeXPAwards(playerIDs [4]uint, botSeats [4]bool, teamScores [2]int, abandonedSeat int) (deltas [4]int) {
	for seat := 0; seat < 4; seat++ {
		if botSeats[seat] || playerIDs[seat] == 0 {
			continue
		}
		team := game.TeamForSeat(seat)
		base := teamScores[team] / xpPerGamePointDivisor

		if abandonedSeat >= 0 {
			if game.TeamForSeat(seat) == game.TeamForSeat(abandonedSeat) {
				continue
			}
			deltas[seat] = int(float64(base) * abandonPartialXPFactor)
			continue
		}

		deltas[seat] = base
	}
	return deltas
}

// awardXP applies lifetime XP for a finished match and prepares the per-human
// event:xp_awarded messages. Failures are logged and skip the events.
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
