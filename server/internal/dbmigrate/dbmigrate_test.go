package dbmigrate

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// Runs only where a disposable Postgres is available (CI sets the variable;
// see .github/workflows/ci.yml). Applying the full set twice proves the second
// run is a no-op, which is exactly what the first production start must be
// against the restored database.
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
