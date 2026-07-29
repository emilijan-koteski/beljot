package match

import (
	"log/slog"
	"time"

	"github.com/emilijan/beljot/server/internal/ws"
)

// honorUpdateMsg is a prepared per-human event:honor_updated broadcast. Built
// during recordHonor but SENT by the finalize path AFTER the event:xp_awarded
// loop and BEFORE the trailing event:match_state, preserving the Story 8.5-1
// ordering contract. Mirrors xpAwardMsg in xp_award.go and coinSettlementMsg in
// settlement.go.
type honorUpdateMsg struct {
	userID uint
	msg    []byte
}

// computeHonorEvents returns the per-player honor bucket for a finished match
// (Story 9.7). Pure + table-tested. There are exactly two buckets — completed
// and abandoned (Story 9.7 D1) — and every human seat that reaches a terminal
// state gets exactly one event.
//
// abandonedSeat is -1 for a natural end (win/loss) and for an accepted
// surrender, in which case every human seat scores `completed`. For an
// abandonment it is the seat whose reconnect window expired.
//
// Bot and empty seats never accrue honor (the exact guard from settlement.go
// and computeXPAwards).
//
// CONCURRENT DOUBLE-DISCONNECT — the rule is PRESENCE, not whose timer fired
// (PO decision 2026-07-29, review pass 2; supersedes the pass-1 rule).
// handleConcurrentDisconnectLocked opens a fresh full window plus its own timer
// for every subsequent drop, so two or more seats sitting inside overlapping
// reconnect windows is a first-class state. On the abandonment path EVERY human
// seat that was not at the table when the match ended is charged `abandoned` —
// not merely the one whose timer happened to expire first.
//
// Two earlier rules were tried and rejected:
//
//   - Crediting the other absent seats `completed` (as originally shipped) RAISED
//     the honor of a player who walked out and never came back, which made
//     quitting SECOND strictly better than quitting first — a gameable bypass of
//     the signal Story 9.8 gates room access on.
//   - Writing nothing for them (review pass 1) removed the reward but replaced it
//     with a worse hole: because no row is written, honor_completed_total +
//     honor_abandoned_total never increments, so a repeat second-quitter stayed
//     pinned at the 80 prior with isNewPlayer=true forever — and 9.8's join gate
//     reads both off the auth envelope.
//
// Charging on absence is representable, monotonic, and leaves no ordering
// incentive. Note this does NOT create a new abandonment trigger:
// spec-abandonment-per-player-results.md freezes what ENDS a match and what lands
// in matches.abandoned_by, and neither changes here — only which honor bucket a
// seat falls into.
//
// The presence gate applies ONLY on the abandonment path (abandonedSeat >= 0). A
// natural end or an accepted surrender reached a real terminal state, so every
// human seat is credited regardless of what the presence snapshot says.
//
// Caveat worth knowing: `connected` fails OPEN. HandleDisconnect only maintains
// Players[].Connected for drops observed in four phases, so a socket reaped
// during a transient phase leaves the seat reading connected and it will be
// credited. That is a pre-existing disconnect-tracking gap, recorded in
// deferred-work.md, and it errs lenient.
func computeHonorEvents(playerIDs [4]uint, botSeats [4]bool, connected [4]bool, abandonedSeat int) map[uint]HonorEvent {
	events := make(map[uint]HonorEvent, 4)
	for seat := 0; seat < 4; seat++ {
		if botSeats[seat] || playerIDs[seat] == 0 {
			continue
		}
		// The expired seat is charged even if its Connected flag were somehow
		// stale, so seat == abandonedSeat is checked independently of presence.
		absent := abandonedSeat >= 0 && (seat == abandonedSeat || !connected[seat])
		events[playerIDs[seat]] = HonorEvent{Abandoned: absent}
	}
	return events
}

// recordHonor persists the honor outcome of a finished match and prepares the
// per-human event:honor_updated messages (Story 9.7). It is a no-op (no
// mutation, no messages) when no HonorRecorder is wired or when every seat is a
// bot.
//
// Mirrors settleMatch's and awardXP's best-effort degradation philosophy: an
// ApplyHonorEvents failure is logged and the events are skipped, but the caller
// still fires match_end/match_abandoned and match_state so clients are never
// stranded on the table.
//
// The clock is read HERE rather than passed in because honor's decay reference
// stamp must be the moment of the write, and both finalizers already stamp
// their own CompletedAt independently.
//
// connected is the per-seat presence snapshot taken by the caller under the
// session lock; see computeHonorEvents for what it gates.
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
			// Missing from the returned snapshots — skip rather than push a
			// wrong value (same rule as awardXP's newTotals lookup).
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
