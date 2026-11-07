package bot_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/bot"
	"github.com/emilijan/beljot/server/internal/bot/search"
)

func TestChooseAdjustedAppliesTheTenUnderAceBonus(t *testing.T) {
	v := viewForTweakCase(tenUnderAceCases()[0])
	require.NotNil(t, bot.TenUnderAceCandidateForTest(v))
	resultWith := func(tenMean, otherMean float64) search.Result {
		return search.Result{Cards: map[int]search.CardStats{
			search.CardIdx(card("TS")): {Mean: tenMean},
			search.CardIdx(card("7D")): {Mean: otherMean},
		}}
	}
	tests := []struct {
		name  string
		res   search.Result
		bonus int
		want  string
	}{
		{"the bonus lifts the ten past a close alternative", resultWith(10, 12), 3, "TS"},
		{"without the bonus the alternative wins", resultWith(10, 12), 0, "7D"},
		{"the bonus does not rescue a clearly worse ten", resultWith(10, 14), 3, "7D"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := bot.ChooseAdjustedForTest(v, tt.res, bot.SearchTuning{TenUnderAceBonus: tt.bonus})
			require.True(t, ok)
			assert.Equal(t, tt.want, got.String())
		})
	}
}
