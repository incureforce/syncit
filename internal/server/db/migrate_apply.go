package serverdb

import (
	"database/sql"
	"fmt"
)

func applyMigrationFile(db *sql.DB, name string) error {
	data, err := migrationSQL.ReadFile("migrations/" + name)
	if err != nil {
		return err
	}
	for _, stmt := range splitStatements(string(data)) {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("server migrate %s: %w\nstmt: %s", name, err, stmt)
		}
	}
	return nil
}
