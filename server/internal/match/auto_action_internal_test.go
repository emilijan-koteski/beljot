package match

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/ws"
)

// perUserRecorder keeps the auto-action type each user received, whichever hub primitive delivered it.
type perUserRecorder struct {
	got map[uint][]ws.AutoActionType
}

func (r *perUserRecorder) record(userID uint, msg []byte) {
	var env struct {
		Type    string               `json:"type"`
		Payload ws.AutoActionPayload `json:"payload"`
	}
	if err := json.Unmarshal(msg, &env); err != nil || env.Type != ws.EventAutoAction {
		return
	}
	if r.got == nil {
		r.got = map[uint][]ws.AutoActionType{}
	}
	r.got[userID] = append(r.got[userID], env.Payload.Type)
}

func (r *perUserRecorder) BroadcastToUsers(userIDs []uint, msg []byte) {
	for _, id := range userIDs {
		r.record(id, msg)
	}
}

func (r *perUserRecorder) SendToUser(userID uint, msg []byte) { r.record(userID, msg) }

func (r *perUserRecorder) SendFrames(frames []ws.UserFrame) {
	for _, f := range frames {
		r.record(f.UserID, f.Msg)
	}
}

func TestSendAutoAction_HidesPrivateSkipsFromOtherSeats(t *testing.T) {
	const timedOutSeat = 1
	playerIDs := [4]uint{10, 20, 0, 40}

	tests := []struct {
		name       string
		autoType   ws.AutoActionType
		othersType ws.AutoActionType
	}{
		{"skip_belot reaches the others as a plain timeout", ws.AutoActionSkipBelot, ws.AutoActionTimeout},
		{"skip_declare reaches the others as a plain timeout", ws.AutoActionSkipDeclare, ws.AutoActionTimeout},
		{"pass_trump is public bidding", ws.AutoActionPassTrump, ws.AutoActionPassTrump},
		{"pick_trump is public bidding", ws.AutoActionPickTrump, ws.AutoActionPickTrump},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := &perUserRecorder{}
			m := &Manager{hub: rec}

			m.sendAutoAction(playerIDs, timedOutSeat, tc.autoType)

			require.Len(t, rec.got, 3, "every human seat hears once and the bot seat never does")
			assert.Equal(t, []ws.AutoActionType{tc.autoType}, rec.got[20], "the timed-out player learns what was done")
			assert.Equal(t, []ws.AutoActionType{tc.othersType}, rec.got[10])
			assert.Equal(t, []ws.AutoActionType{tc.othersType}, rec.got[40])
		})
	}
}
