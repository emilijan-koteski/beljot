package room

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidVariantsIsBothVariants is the enforcement behind "Croatian is
// creatable, and nothing else is".
//
// It replaces the Bitola-only tripwire that guarded Epic 12 while the Croatian
// rules were still landing story by story. Story 12.8 — the enablement story
// that tripwire named as its owner — opened the gate, so the assertion inverts:
// both variants must be creatable, and the map must still be exactly those two.
//
// validVariants stays a hand-maintained duplicate of the engine's variant set,
// so the second half of that sentence is the part that keeps mattering: adding a
// variant constant in internal/game does not — and must not — make it creatable
// here. A half-built variant exposed in room creation lets players start a game
// that cannot be finished, which is exactly what this test exists to prevent.
func TestValidVariantsIsBothVariants(t *testing.T) {
	const guard = "room creation is the gate on a variant being PLAYABLE, not merely " +
		"present in the engine — do not add a variant here until every one of its " +
		"rules ships, and update this test as part of the story that does"

	assert.Equal(t, map[string]bool{"bitola": true, "croatia": true}, validVariants,
		"room creation must accept exactly the two finished variants: %s", guard)

	// Spelled out separately so a failure names the offending value rather than
	// dumping a map diff.
	for variant := range validVariants {
		assert.Contains(t, []string{"bitola", "croatia"}, variant,
			"variant %q is creatable but should not be: %s", variant, guard)
	}
	assert.True(t, validVariants["bitola"], "Bitola must stay creatable: %s", guard)
	assert.True(t, validVariants["croatia"],
		"the Croatian variant is complete and must be creatable (Story 12.8): %s", guard)
}
