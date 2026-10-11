package dbmigrate

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUpIsIdempotent checks a second Up is a no-op, including on a database the
// golang-migrate CLI already migrated (as CI does). Needs BELJOT_TEST_DB_URL.
func TestUpIsIdempotent(t *testing.T) {
	url := os.Getenv("BELJOT_TEST_DB_URL")
	if url == "" {
		t.Skip("BELJOT_TEST_DB_URL not set")
	}
	first, err := Up(url)
	require.NoError(t, err)
	require.NotZero(t, first)

	second, err := Up(url)
	require.NoError(t, err)
	require.Equal(t, first, second)
}

func TestUpReturnsErrorWhenDatabaseUnreachable(t *testing.T) {
	_, err := Up("postgres://nobody:nothing@127.0.0.1:1/nowhere?sslmode=disable&connect_timeout=1")
	require.Error(t, err)
}
