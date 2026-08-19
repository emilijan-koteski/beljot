package room

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidVariantsIsBitolaOnly is the enforcement behind the "Croatian is still
// not selectable" acceptance criterion.
//
// validVariants is a hand-maintained duplicate of the engine's variant set, so
// adding a variant constant in internal/game does not — and must not — make it
// creatable here. The Croatian variant's rules land across several stories, and
// exposing the room option before the last of them ships would let players start
// a half-built game.
//
// This test is the tripwire, not an obstacle: when Croatian enablement lands,
// updating this table IS part of that story.
func TestValidVariantsIsBitolaOnly(t *testing.T) {
	const unblockedBy = "the Croatian variant enablement story (Epic 12, Story 12.8) " +
		"is the only place this allowlist should grow — if you are not implementing it, " +
		"do not add a variant here"

	assert.Equal(t, map[string]bool{"bitola": true}, validVariants,
		"room creation must accept exactly one variant: %s", unblockedBy)

	// Spelled out separately so a failure names the offending value rather than
	// dumping a map diff.
	for variant := range validVariants {
		assert.Equal(t, "bitola", variant,
			"variant %q is creatable but should not be: %s", variant, unblockedBy)
	}
	assert.False(t, validVariants["croatia"],
		"the Croatian variant must stay unselectable: %s", unblockedBy)
}
