package serverdb

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
)

const (
	settingMaxFileBytes = "settings_max_file_bytes"
	settingQuotaBytes   = "settings_quota_bytes"
	settingUsedBytes    = "settings_used_bytes"
)

// SeedDefaultSettings inserts default limit rows in metadata if missing.
func SeedDefaultSettings(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		INSERT OR IGNORE INTO metadata (name, data) VALUES
		(?, ?),
		(?, ?),
		(?, ?)
	`, settingMaxFileBytes, "10737418240", settingQuotaBytes, "107374182400", settingUsedBytes, "0")
	return err
}

func (s *ServerDB) intSetting(ctx context.Context, key string) (int64, error) {
	var v string
	err := s.sql.QueryRowContext(ctx, `SELECT data FROM metadata WHERE name = ?`, key).Scan(&v)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(v, 10, 64)
}

// Limits returns max bytes per upload and total quota.
func (s *ServerDB) Limits(ctx context.Context) (maxFile, quota, used int64, err error) {
	maxFile, err = s.intSetting(ctx, settingMaxFileBytes)
	if err != nil {
		return 0, 0, 0, err
	}
	quota, err = s.intSetting(ctx, settingQuotaBytes)
	if err != nil {
		return 0, 0, 0, err
	}
	used, err = s.intSetting(ctx, settingUsedBytes)
	if err != nil {
		return 0, 0, 0, err
	}
	return maxFile, quota, used, nil
}

// TryCommitBlobUsage increases used storage when a new blob was added; rolls back if over quota.
func (s *ServerDB) TryCommitBlobUsage(ctx context.Context, newBytes int64) error {
	if newBytes <= 0 {
		return nil
	}
	tx, err := s.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var usedStr, quotaStr string
	if err := tx.QueryRowContext(ctx, `SELECT data FROM metadata WHERE name = ?`, settingUsedBytes).Scan(&usedStr); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `SELECT data FROM metadata WHERE name = ?`, settingQuotaBytes).Scan(&quotaStr); err != nil {
		return err
	}
	used, _ := strconv.ParseInt(usedStr, 10, 64)
	quota, _ := strconv.ParseInt(quotaStr, 10, 64)
	if used+newBytes > quota {
		return ErrQuotaExceeded
	}
	newUsed := used + newBytes
	_, err = tx.ExecContext(ctx, `UPDATE metadata SET data = ? WHERE name = ?`, strconv.FormatInt(newUsed, 10), settingUsedBytes)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// ErrQuotaExceeded is returned when a write would exceed configured quota.
var ErrQuotaExceeded = errors.New("storage quota exceeded")
