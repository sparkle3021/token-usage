// Package database 的子文件，collection_runs 表的 CRUD 操作。
package database

import (
	"log"
	"time"

	"token-dashboard/internal/model"
)

func (m *Manager) RecordRun(device, source, status, message, command string) error {
	_, err := m.stmtRecordRun.Exec(device, source, status, nullIfEmpty(message), nullIfEmpty(command))
	if err != nil {
		log.Printf("[db] RecordRun error source=%s status=%s err=%v", source, status, err)
	} else {
		log.Printf("[db] RecordRun ok source=%s status=%s message=%s", source, status, truncateID(message))
	}
	return err
}

func (m *Manager) RecordRunWithMtime(device, source, status, message, command string, lastFileMtime int64) error {
	_, err := m.db.Exec(`
		INSERT INTO collection_runs(device, source, status, message, collected_at, command, last_file_mtime)
		VALUES (?, ?, ?, ?, datetime('now'), ?, ?)
	`, device, source, status, nullIfEmpty(message), nullIfEmpty(command), lastFileMtime)
	if err != nil {
		log.Printf("[db] RecordRunWithMtime error source=%s mtime=%d err=%v", source, lastFileMtime, err)
	} else {
		log.Printf("[db] RecordRunWithMtime ok source=%s mtime=%d message=%s", source, lastFileMtime, truncateID(message))
	}
	return err
}

func (m *Manager) GetLastMtime(device, source string) (int64, error) {
	var mtime int64
	err := m.db.QueryRow(`
		SELECT COALESCE(MAX(last_file_mtime), 0) FROM collection_runs
		WHERE device = ? AND source = ? AND last_file_mtime IS NOT NULL
	`, device, source).Scan(&mtime)
	if err != nil {
		return 0, err
	}
	return mtime, nil
}

func (m *Manager) pruneCollectionRuns(keep int) {
	if keep <= 0 {
		keep = 500
	}
	result, err := m.db.Exec(`
		DELETE FROM collection_runs
		WHERE id NOT IN (
			SELECT id FROM collection_runs ORDER BY id DESC LIMIT ?
		)
	`, keep)
	if err != nil {
		log.Printf("[db] pruneCollectionRuns error keep=%d err=%v", keep, err)
	} else if n, _ := result.RowsAffected(); n > 0 {
		log.Printf("[db] pruneCollectionRuns pruned=%d keep=%d", n, keep)
	}
}

func (m *Manager) QueryRuns(limit int) ([]model.CollectionRun, error) {
	start := time.Now()
	rows, err := m.db.Query(`
		SELECT id, device, source, status, message, collected_at
		FROM collection_runs
		ORDER BY id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.CollectionRun
	for rows.Next() {
		var r model.CollectionRun
		var collectedAt string
		if err := rows.Scan(&r.ID, &r.Device, &r.Source, &r.Status, &r.Message, &collectedAt); err != nil {
			return nil, err
		}
		r.CollectedAt = collectedAt
		results = append(results, r)
	}
	log.Printf("[db] QueryRuns rows=%d elapsed=%v", len(results), time.Since(start))
	return results, rows.Err()
}
