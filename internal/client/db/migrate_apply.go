package clientdb

import (
	"database/sql"
	"fmt"
)

// applyMigration runs SQL from an embedded migration file and sets user_version via PRAGMA in the file.
func applyMigration(db *sql.DB, name string) error {
	data, err := migrationSQL.ReadFile("migrations/" + name)
	if err != nil {
		return err
	}
	for _, stmt := range splitStatements(string(data)) {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("client migrate %s: %w\nstmt: %s", name, err, stmt)
		}
	}
	return nil
}
