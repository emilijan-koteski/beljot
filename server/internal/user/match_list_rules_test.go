package user_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/auth"
	"github.com/emilijan/beljot/server/internal/match"
)

// TestListMatches_CarriesTheRulesTheMatchWasPlayedUnder checks history reads every rule flag from the
// match row, so a finished match can say which rules produced its score.
func TestListMatches_CarriesTheRulesTheMatchWasPlayedUnder(t *testing.T) {
	tests := []struct {
		name                 string
		declarationsEnabled  bool
		stopAtTarget         bool
		declarationsTransfer bool
	}{
		{"all at their defaults", true, false, true},
		{"declarations do not transfer", true, false, false},
		{"every rule changed", false, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, matchRepo, e := setupUserHandlerWithMatches()
			viewer := repo.addUser("alice", "alice@example.com", "en")
			viewerID := viewer.ID
			now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
			matchRepo.matches = []match.Match{{
				ID:                   1,
				RoomID:               1,
				Player1ID:            &viewerID,
				Player2IsBot:         true,
				Player3IsBot:         true,
				Player4IsBot:         true,
				HasBots:              true,
				TeamAScore:           1004,
				TeamBScore:           730,
				WinnerTeam:           0,
				Variant:              "bitola",
				MatchMode:            "1001",
				DeclarationsEnabled:  tt.declarationsEnabled,
				StopAtTarget:         tt.stopAtTarget,
				DeclarationsTransfer: tt.declarationsTransfer,
				StartedAt:            now.Add(-30 * time.Minute),
				CompletedAt:          now,
				Status:               "completed",
			}}

			token, err := auth.GenerateAccessToken(viewer.ID, testJWTSecret)
			require.NoError(t, err)

			rec := doListMatches(e, "1", "", token)
			require.Equal(t, http.StatusOK, rec.Code)

			resp := decodeMatchesResponse(t, rec.Body.Bytes())
			require.Len(t, resp.Items, 1)
			item := resp.Items[0]
			assert.Equal(t, tt.declarationsEnabled, item.DeclarationsEnabled)
			assert.Equal(t, tt.stopAtTarget, item.StopAtTarget)
			assert.Equal(t, tt.declarationsTransfer, item.DeclarationsTransfer)
			assert.Contains(t, rec.Body.String(), `"declarationsTransfer":`, "the key must be on the wire")
		})
	}
}
