// Package orchestrator 的子文件，文件指纹持久化适配层。
package orchestrator

import (
	"token-dashboard/internal/collector"
	"token-dashboard/internal/database"
)

// cachePersister adapts *database.Manager to collector.PersistHandler.
type cachePersister struct {
	db *database.Manager
}

var _ collector.PersistHandler = (*cachePersister)(nil)
var _ collector.BatchPersistHandler = (*cachePersister)(nil)
var _ collector.BatchLoadHandler = (*cachePersister)(nil)

func newCachePersister(db *database.Manager) *cachePersister {
	return &cachePersister{db: db}
}

func (p *cachePersister) LoadParseCache(source, filePath string) (fingerprint string, lastOffset int64, ok bool) {
	return p.db.GetParseCache(source, filePath)
}

func (p *cachePersister) SaveParseCache(source, filePath, fingerprint string, lastOffset int64) error {
	return p.db.UpsertParseCacheFingerprint(source, filePath, fingerprint, lastOffset)
}

// LoadParseCacheBatch 单次查询加载整个 source 的指纹（批量加载路径）。
func (p *cachePersister) LoadParseCacheBatch(source string) ([]collector.PersistEntry, error) {
	entries, err := p.db.ListParseCache(source)
	if err != nil {
		return nil, err
	}
	out := make([]collector.PersistEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, collector.PersistEntry{
			Source: e.Source, FilePath: e.FilePath, Fingerprint: e.Fingerprint, LastOffset: e.LastOffset,
		})
	}
	return out, nil
}

// SaveParseCacheBatch 单事务批量持久化指纹，替代逐条 upsert。
func (p *cachePersister) SaveParseCacheBatch(entries []collector.PersistEntry) error {
	dbEntries := make([]database.ParseCacheEntry, 0, len(entries))
	for _, e := range entries {
		dbEntries = append(dbEntries, database.ParseCacheEntry{
			Source: e.Source, FilePath: e.FilePath, Fingerprint: e.Fingerprint, LastOffset: e.LastOffset,
		})
	}
	return p.db.BulkUpsertParseCache(dbEntries)
}

func (p *cachePersister) DeleteParseCacheBySource(source string) error {
	return p.db.DeleteParseCacheBySource(source)
}
