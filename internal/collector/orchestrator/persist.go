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

func newCachePersister(db *database.Manager) *cachePersister {
	return &cachePersister{db: db}
}

func (p *cachePersister) LoadParseCache(source, filePath string) (fingerprint string, ok bool) {
	fp, _, found := p.db.GetParseCache(source, filePath)
	return fp, found
}

func (p *cachePersister) SaveParseCache(source, filePath, fingerprint string) error {
	return p.db.UpsertParseCacheFingerprint(source, filePath, fingerprint)
}

func (p *cachePersister) DeleteParseCacheBySource(source string) error {
	return p.db.DeleteParseCacheBySource(source)
}
