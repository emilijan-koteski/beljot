package dbmigrate

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// Runs only where a disposable Postgres is available (CI sets the variable;
// see .github/workflows/ci.yml). CI migrates that database with the
// golang-migrate CLI before the tests run, so the first Up already finds a
// schema_migrations table written by the old tool and must accept it as a
// no-op: exactly the restored-production case. Against an empty database the
// first Up applies everything and the second is the no-op.
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
