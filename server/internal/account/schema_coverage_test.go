package account

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var keptUserReferences = []string{
	"matches.abandoned_by",
	"matches.player1_id",
	"matches.player2_id",
	"matches.player3_id",
	"matches.player4_id",
	"matches.surrendered_by",
	"rooms.owner_id",
}

func schemaDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("BELJOT_TEST_DB_URL")
	if dsn == "" {
		t.Skip("BELJOT_TEST_DB_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func scrubbedColumns() []string {
	matches := regexp.MustCompile(`(\w+) = `).FindAllStringSubmatch(scrubUser, -1)
	cols := make([]string, 0, len(matches))
	for _, m := range matches {
		if m[1] != "id" {
			cols = append(cols, m[1])
		}
	}
	return cols
}

func TestScrubUser_CoversEveryUsersColumn(t *testing.T) {
	db := schemaDB(t)
	var columns []string
	require.NoError(t, db.Raw(`SELECT column_name FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'users' AND column_name <> 'id'`).Scan(&columns).Error)
	require.NotEmpty(t, columns)

	scrubbed := scrubbedColumns()
	for _, col := range columns {
		assert.Contains(t, scrubbed, col, "users.%s is not reset by the erase; add it to scrubUser", col)
	}
}

func TestErase_HandlesEveryForeignKeyToUsers(t *testing.T) {
	db := schemaDB(t)
	var refs []string
	require.NoError(t, db.Raw(`SELECT c.conrelid::regclass::text || '.' || a.attname
		FROM pg_constraint c
		JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY (c.conkey)
		WHERE c.contype = 'f' AND c.confrelid = 'users'::regclass`).Scan(&refs).Error)
	require.NotEmpty(t, refs)

	handled := strings.Join(satelliteDeletes, "\n") + "\n" + renameOwnedRooms
	for _, ref := range refs {
		table, _, _ := strings.Cut(ref, ".")
		if slices.Contains(keptUserReferences, ref) {
			continue
		}
		assert.Contains(t, handled, "FROM "+table+" ", "%s references users but the erase neither deletes nor keeps it", ref)
	}
}
