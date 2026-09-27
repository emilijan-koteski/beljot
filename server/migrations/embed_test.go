package migrations

import (
	"strings"
	"testing"
)

func TestEveryUpMigrationHasADown(t *testing.T) {
	entries, err := FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		names[e.Name()] = true
	}
	if len(names) == 0 {
		t.Fatal("no migrations embedded")
	}
	for name := range names {
		if !strings.HasSuffix(name, ".sql") {
			t.Errorf("non-sql file embedded: %s", name)
		}
		if strings.HasSuffix(name, ".up.sql") {
			down := strings.TrimSuffix(name, ".up.sql") + ".down.sql"
			if !names[down] {
				t.Errorf("%s has no matching down migration", name)
			}
		}
	}
}
