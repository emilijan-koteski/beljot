package game

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// phaseConstRe matches a `PhaseX Phase = "value"` declaration in types.go.
var phaseConstRe = regexp.MustCompile(`(?m)^\s*Phase[A-Za-z]+\s+Phase\s*=\s*"([a-z_]+)"`)

// TestAllPhasesCoversEveryConstant is the guard that keeps AllPhases honest.
//
// AllPhases is hand-maintained (Go cannot enumerate the constants of a string
// type at runtime), and everything downstream trusts it: the ws contract test
// writes it to a golden, and the client derives its Phase union from that
// golden. A phase constant added to types.go but forgotten in AllPhases would
// travel the wire with no client union member and no test failing anywhere —
// exactly the drift this chain exists to prevent. So instead of trusting the
// list, scan the source for the declarations and compare.
func TestAllPhasesCoversEveryConstant(t *testing.T) {
	src, err := os.ReadFile("types.go")
	require.NoError(t, err, "types.go must be readable — this test scans it for Phase constants")

	matches := phaseConstRe.FindAllStringSubmatch(string(src), -1)
	require.NotEmpty(t, matches, "found no Phase constants in types.go — has the declaration style changed?")

	declared := make(map[string]bool, len(matches))
	for _, m := range matches {
		declared[m[1]] = true
	}

	listed := make(map[string]bool, len(AllPhases()))
	for _, p := range AllPhases() {
		listed[string(p)] = true
	}

	for value := range declared {
		assert.Truef(t, listed[value],
			"phase %q is declared in types.go but missing from AllPhases() — add it there, "+
				"regenerate the ws golden (UPDATE_GOLDENS=1), and add the matching client union member",
			value)
	}
	for value := range listed {
		assert.Truef(t, declared[value],
			"AllPhases() lists %q but no such constant is declared in types.go", value)
	}
	assert.Len(t, AllPhases(), len(declared), "AllPhases() must have exactly one entry per constant (no duplicates)")
}
