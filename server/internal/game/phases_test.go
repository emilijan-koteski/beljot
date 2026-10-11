package game

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var phaseConstRe = regexp.MustCompile(`(?m)^\s*Phase[A-Za-z]+\s+Phase\s*=\s*"([a-z_]+)"`)

// TestAllPhasesCoversEveryConstant scans types.go for Phase constants and checks the hand-maintained
// AllPhases list contains every one.
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
