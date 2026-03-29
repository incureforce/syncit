package serverdb

import (
	"database/sql"
	"embed"
	"fmt"
	"strings"
)

//go:embed migrations/*.sql
var migrationSQL embed.FS

const currentUserVersion = 1

func migrate(db *sql.DB) error {
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return err
	}
	if v == currentUserVersion {
		return nil
	}
	if v == 0 {
		return applyMigrationFile(db, "001_initial.sql")
	}
	return fmt.Errorf("unsupported schema user_version %d (expected %d for current schema; recreate or use a fresh database file)", v, currentUserVersion)
}

func splitStatements(sql string) []string {
	sql = strings.ReplaceAll(sql, "\r\n", "\n")
	var out []string
	for _, part := range strings.Split(sql, ";") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		out = append(out, p+";")
	}
	return out
}
