package match

import (
	"log/slog"
	"time"

	"github.com/emilijan/beljot/server/internal/ws"
)

// honorUpdateMsg is a prepared per-human event:honor_updated broadcast, sent
// after the event:xp_awarded loop and before the trailing event:match_state.
type honorUpdateMsg struct {
	userID uint
	msg    []byte
}

// computeHonorEvents returns each human seat's honor bucket for a finished match.
// On an abandonment (abandonedSeat >= 0) every seat absent at match end is
// charged, so quitting second never beats quitting first.
func computeHonorEvents(playerIDs [4]uint, botSeats [4]bool, connected [4]bool, abandonedSeat int) map[uint]HonorEvent {
	events := make(map[uint]HonorEvent, 4)
	for seat := 0; seat < 4; seat++ {
		if botSeats[seat] || playerIDs[seat] == 0 {
			continue
		}
		absent := abandonedSeat >= 0 && (seat == abandonedSeat || !connected[seat])
		events[playerIDs[seat]] = HonorEvent{Abandoned: absent}
	}
	return events
}

// recordHonor persists the honor outcome of a finished match and returns the per-human
// event:honor_updated messages. A recorder failure is logged and yields no messages.
func (m *Manager) recordHonor(roomID uint, playerIDs [4]uint, botSeats [4]bool, connected [4]bool, abandonedSeat int) []honorUpdateMsg {
	if m.honorRecorder == nil {
		return nil
	}

	events := computeHonorEvents(playerIDs, botSeats, connected, abandonedSeat)
	if len(events) == 0 {
		return nil
	}

	snapshots, err := m.honorRecorder.ApplyHonorEvents(events, time.Now().UTC())
	if err != nil {
		slog.Error("session: failed to record honor", "roomID", roomID, "error", err)
		return nil
	}

	var msgs []honorUpdateMsg
	for seat := 0; seat < 4; seat++ {
		uid := playerIDs[seat]
		if botSeats[seat] || uid == 0 {
			continue
		}
		snap, ok := snapshots[uid]
		if !ok {
			continue
		}
		payload := ws.HonorUpdatedPayload{
			HonorScore:          snap.Score,
			HonorTier:           snap.Tier,
			HonorCompletedTotal: snap.CompletedTotal,
			HonorAbandonedTotal: snap.AbandonedTotal,
			IsNewPlayer:         snap.IsNewPlayer,
		}
		msgs = append(msgs, honorUpdateMsg{userID: uid, msg: buildMessage(ws.EventHonorUpdated, payload)})
	}
	return msgs
}
