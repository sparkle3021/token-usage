// Package database 的子文件，parse_cache 表的 CRUD 操作。
package database

import (
	"fmt"
	"log"
	"time"
)

func (m *Manager) UpsertParseCache(source, filePath, fingerprint string, records []byte) error {
	_, err := m.db.Exec(`
		INSERT INTO parse_cache(source, file_path, fingerprint, records, updated_at)
		VALUES (?, ?, ?, ?, datetime('now','localtime'))
		ON CONFLICT(source, file_path) DO UPDATE SET
			fingerprint = excluded.fingerprint,
			records = excluded.records,
			updated_at = datetime('now','localtime')
	`, source, filePath, fingerprint, records)
	return err
}

func (m *Manager) UpsertParseCacheFingerprint(source, filePath, fingerprint string, lastOffset int64) error {
	_, err := m.db.Exec(`
		INSERT INTO parse_cache(source, file_path, fingerprint, records, last_parsed_offset, updated_at)
		VALUES (?, ?, ?, NULL, ?, datetime('now','localtime'))
		ON CONFLICT(source, file_path) DO UPDATE SET
			fingerprint = excluded.fingerprint,
			last_parsed_offset = excluded.last_parsed_offset,
			updated_at = datetime('now','localtime')
	`, source, filePath, fingerprint, lastOffset)
	return err
}

// ParseCacheEntry 待批量持久化的指纹条目。
type ParseCacheEntry struct {
	Source     string
	FilePath   string
	Fingerprint string
	LastOffset int64
}

// BulkUpsertParseCache 单事务批量 upsert parse_cache 指纹，替代逐条 upsert。
func (m *Manager) BulkUpsertParseCache(entries []ParseCacheEntry) error {
	if len(entries) == 0 {
		return nil
	}
	start := time.Now()
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for i := range entries {
		e := entries[i]
		if _, err := tx.Exec(`
			INSERT INTO parse_cache(source, file_path, fingerprint, records, last_parsed_offset, updated_at)
			VALUES (?, ?, ?, NULL, ?, datetime('now','localtime'))
			ON CONFLICT(source, file_path) DO UPDATE SET
				fingerprint = excluded.fingerprint,
				last_parsed_offset = excluded.last_parsed_offset,
				updated_at = datetime('now','localtime')
		`, e.Source, e.FilePath, e.Fingerprint, e.LastOffset); err != nil {
			return fmt.Errorf("bulk upsert parse_cache row %d: %w", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	log.Printf("[perf] db BulkUpsertParseCache rows=%d elapsed=%v", len(entries), time.Since(start))
	return nil
}

func (m *Manager) GetParseCache(source, filePath string) (fingerprint string, lastOffset int64, ok bool) {
	err := m.db.QueryRow(`
		SELECT fingerprint, last_parsed_offset FROM parse_cache
		WHERE source = ? AND file_path = ?
	`, source, filePath).Scan(&fingerprint, &lastOffset)
	if err != nil {
		return "", 0, false
	}
	return fingerprint, lastOffset, true
}

func (m *Manager) GetParseCacheWithRecord(source, filePath string) (fingerprint string, records []byte, ok bool) {
	err := m.db.QueryRow(`
		SELECT fingerprint, records FROM parse_cache
		WHERE source = ? AND file_path = ?
	`, source, filePath).Scan(&fingerprint, &records)
	if err != nil {
		return "", nil, false
	}
	return fingerprint, records, true
}

func (m *Manager) DeleteParseCacheBySource(source string) error {
	_, err := m.db.Exec(`DELETE FROM parse_cache WHERE source = ?`, source)
	return err
}

func (m *Manager) PruneParseCache(maxAgeDays, maxRows int) {
	if maxAgeDays <= 0 {
		maxAgeDays = 30
	}
	if maxRows <= 0 {
		maxRows = 1000
	}
	m.db.Exec(`DELETE FROM parse_cache WHERE updated_at < datetime('now', ?)`, fmt.Sprintf("-%d days", maxAgeDays))
	m.db.Exec(`DELETE FROM parse_cache WHERE rowid NOT IN (SELECT rowid FROM parse_cache ORDER BY updated_at DESC LIMIT ?)`, maxRows)
}
