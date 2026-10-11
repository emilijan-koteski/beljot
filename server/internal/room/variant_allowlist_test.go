package room

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidVariantsIsBothVariants asserts both variants are creatable and validVariants
// holds exactly those two, so a new engine variant never becomes creatable by accident.
func TestValidVariantsIsBothVariants(t *testing.T) {
	const guard = "room creation is the gate on a variant being PLAYABLE, not merely " +
		"present in the engine — do not add a variant here until every one of its " +
		"rules ships, and update this test as part of the story that does"

	assert.Equal(t, map[string]bool{"bitola": true, "croatia": true}, validVariants,
		"room creation must accept exactly the two finished variants: %s", guard)

	for variant := range validVariants {
		assert.Contains(t, []string{"bitola", "croatia"}, variant,
			"variant %q is creatable but should not be: %s", variant, guard)
	}
	assert.True(t, validVariants["bitola"], "Bitola must stay creatable: %s", guard)
	assert.True(t, validVariants["croatia"],
		"the Croatian variant is complete and must be creatable (Story 12.8): %s", guard)
}
