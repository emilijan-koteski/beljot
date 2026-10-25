package room_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuickPlay_RoomCreatedSendsTheFullLobbySnapshot(t *testing.T) {
	e, _, broadcaster := setupTestWithBroadcast()

	require.Equal(t, http.StatusOK, doQuickPlay(e, validToken(5)).Code)

	var created []byte
	for _, c := range broadcaster.allCalls {
		if msgTypeOf(t, c.msg) == "system:room_created" {
			created = c.msg
			break
		}
	}
	require.NotNil(t, created, "QuickPlay must broadcast system:room_created for a synthesized room")

	var payload map[string]any
	require.NoError(t, json.Unmarshal(payloadOf(t, created), &payload))

	for _, key := range []string{"ownerUsername", "ownerAvatarUrl", "isPrivate", "minHonor", "allowNewPlayers", "createdAt", "updatedAt"} {
		assert.Contains(t, payload, key)
	}
	assert.Equal(t, true, payload["isQuickPlay"])
	players, ok := payload["players"].([]any)
	require.True(t, ok, "players must be a list, got %T", payload["players"])
	require.Len(t, players, 1, "the creator is the only player in a new Quick Play room")
	creator, ok := players[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(5), creator["userId"])
}
