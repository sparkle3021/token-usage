package collector

import (
	"log"
	"os"
	"strings"
	"sync"
)

// FileFingerprint returns a fingerprint string (mtime:size) for a file.
func FileFingerprint(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return formatFingerprint(info.ModTime().UnixMilli(), info.Size())
}

func formatFingerprint(mtimeMs int64, size int64) string {
	return strings.Join([]string{
		int64ToStr(mtimeMs),
		int64ToStr(size),
	}, ":")
}

func int64ToStr(n int64) string {
	if n == 0 {
		return "0"
	}
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// PersistHandler is the optional persistence backend for ParseCache.
type PersistHandler interface {
	LoadParseCache(source, filePath string) (fingerprint string, lastOffset int64, ok bool)
	SaveParseCache(source, filePath, fingerprint string, lastOffset int64) error
	DeleteParseCacheBySource(source string) error
}

// BatchPersistHandler 可选：支持批量持久化的后端。ParseCache 检测到实现此接口
// 时用单事务批量提交，替代逐条 SaveParseCache。
type BatchPersistHandler interface {
	PersistHandler
	SaveParseCacheBatch(entries []PersistEntry) error
}

// BatchLoadHandler 可选：支持批量加载的后端。ParseCache 检测到实现此接口时
// 单次查询加载整个 source 的指纹，替代逐条 LoadParseCache（文件数千时
// 逐条 SQL 是大头开销）。
type BatchLoadHandler interface {
	LoadParseCacheBatch(source string) ([]PersistEntry, error)
}

// PersistEntry 批量持久化的单条指纹。
type PersistEntry struct {
	Source     string
	FilePath   string
	Fingerprint string
	LastOffset int64
}

type ParseState int

const (
	StateCached      ParseState = iota // fingerprint matches, records in memory
	StateIncremental                   // file grew (size > last_offset), incremental parse
	StateParse                         // no cache or file shrank, full parse
)

// ParseCache is a simple file-fingerprint based parse cache.
// Fingerprints are stored in memory immediately but only persisted
// to the backend on explicit PersistPending() call. This ensures
// the cache never marks files as "done" before the corresponding
// data has been committed to the database.
type ParseCache struct {
	mu        sync.Mutex
	version   int
	store     map[string]*cacheEntry // path -> entry
	persister PersistHandler
	source    string
	newPaths  int    // new entries since last reset
	pending   []string // paths whose fingerprints await persistence
}

type cacheEntry struct {
	Fingerprint string
	Records     interface{}
	LastOffset  int64
}

func NewParseCache(version int) *ParseCache {
	return &ParseCache{
		version: version,
		store:   make(map[string]*cacheEntry),
	}
}

// SetPersister attaches a persistence backend. Must be called before use.
func (c *ParseCache) SetPersister(p PersistHandler, source string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.persister = p
	c.source = source
}

// LoadFromDB pre-populates the cache from the persistence backend.
// Only fingerprints are stored; Records must be re-parsed on first access.
// 后端支持 BatchLoadHandler 时单次查询加载整个 source，否则逐条回退。
func (c *ParseCache) LoadFromDB(source string, paths []string) int {
	if c.persister == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	loaded := 0

	if bp, ok := c.persister.(BatchLoadHandler); ok {
		entries, err := bp.LoadParseCacheBatch(source)
		if err == nil {
			for _, e := range entries {
				if _, exists := c.store[e.FilePath]; !exists {
					c.store[e.FilePath] = &cacheEntry{Fingerprint: e.Fingerprint, LastOffset: e.LastOffset}
					loaded++
				}
			}
			if loaded > 0 {
				log.Printf("[cache] LoadFromDB batch source=%s loaded=%d paths=%d", source, loaded, len(paths))
			}
			return loaded
		}
		log.Printf("[cache] LoadFromDB batch error source=%s err=%v, fallback per-path", source, err)
	}

	for _, path := range paths {
		fp, offset, ok := c.persister.LoadParseCache(source, path)
		if ok {
			if _, exists := c.store[path]; !exists {
				c.store[path] = &cacheEntry{Fingerprint: fp, LastOffset: offset}
				loaded++
			}
		}
	}
	if loaded > 0 {
		log.Printf("[cache] LoadFromDB source=%s loaded=%d paths=%d", source, loaded, len(paths))
	}
	return loaded
}

// AllCached returns true when every path in the set has a matching fingerprint cached.
func (c *ParseCache) AllCached(paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, path := range paths {
		fp := FileFingerprint(path)
		if fp == "" {
			return false
		}
		entry, ok := c.store[path]
		if !ok || entry.Fingerprint != fp {
			return false
		}
	}
	return true
}

// FileUnchanged returns true when the file's current fingerprint matches the
// cached entry, regardless of whether parsed records are held in memory.
// This is the file-level incremental gate: collectors can skip re-reading,
// re-parsing and re-aggregating files that have not changed since last commit.
func (c *ParseCache) FileUnchanged(filePath string) bool {
	fp := FileFingerprint(filePath)
	if fp == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.store[filePath]
	if !ok {
		return false
	}
	return entry.Fingerprint == fp
}

// NewCount returns the number of newly cached entries since the last reset.
func (c *ParseCache) NewCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.newPaths
}

// ResetNew resets the new entry counter.
func (c *ParseCache) ResetNew() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.newPaths = 0
}

// Clear empties the entire cache but retains the persister binding.
func (c *ParseCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store = make(map[string]*cacheEntry)
	c.newPaths = 0
}

// Get returns cached records if the file is unchanged.
func (c *ParseCache) GetWithOffset(filePath string) (interface{}, int64, ParseState) {
	fp := FileFingerprint(filePath)
	if fp == "" {
		return nil, 0, StateParse
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.store[filePath]
	if !ok {
		return nil, 0, StateParse
	}

	if entry.Fingerprint == fp && entry.Records != nil {
		return entry.Records, 0, StateCached
	}

	if entry.LastOffset > 0 {
		fi, err := os.Stat(filePath)
		if err == nil && fi.Size() > entry.LastOffset {
			return nil, entry.LastOffset, StateIncremental
		}
	}

	return nil, 0, StateParse
}

// Set stores parsed records for a file. The fingerprint is held in memory
// but NOT persisted yet — call PersistPending() to write all pending
// fingerprints to the backend. This prevents the cache from skipping
// re-parses after a crash during the engine's write phase.
func (c *ParseCache) SetWithOffset(filePath string, records interface{}, fileSize int64) {
	fp := FileFingerprint(filePath)
	if fp == "" {
		return
	}

	c.mu.Lock()
	_, existed := c.store[filePath]
	c.store[filePath] = &cacheEntry{
		Fingerprint: fp,
		Records:     records,
		LastOffset:  fileSize,
	}
	if !existed {
		c.newPaths++
	}
	c.pending = append(c.pending, filePath)
	c.mu.Unlock()
}

// PersistPending persists all pending fingerprints to the backend.
// Should be called after data has been successfully written to the database.
// 若后端实现 BatchPersistHandler，则单事务批量提交；否则逐条回退。
func (c *ParseCache) PersistPending() error {
	c.mu.Lock()
	pending := c.pending
	c.pending = nil
	persistData := make([]struct {
		path   string
		offset int64
	}, 0, len(pending))
	for _, path := range pending {
		if entry, ok := c.store[path]; ok {
			persistData = append(persistData, struct {
				path   string
				offset int64
			}{path, entry.LastOffset})
		}
	}
	persister, source := c.persister, c.source
	c.mu.Unlock()

	if persister == nil || len(persistData) == 0 {
		return nil
	}

	// 批量路径：一次事务提交所有指纹，替代逐条 upsert。
	if bp, ok := persister.(BatchPersistHandler); ok {
		entries := make([]PersistEntry, 0, len(persistData))
		for _, d := range persistData {
			fp := FileFingerprint(d.path)
			if fp == "" {
				continue
			}
			entries = append(entries, PersistEntry{
				Source: source, FilePath: d.path, Fingerprint: fp, LastOffset: d.offset,
			})
		}
		if err := bp.SaveParseCacheBatch(entries); err != nil {
			log.Printf("[cache] PersistPending batch error source=%s count=%d err=%v", source, len(entries), err)
			return err
		}
		log.Printf("[cache] PersistPending batch ok source=%s count=%d", source, len(entries))
		return nil
	}

	var lastErr error
	for _, d := range persistData {
		fp := FileFingerprint(d.path)
		if fp == "" {
			continue
		}
		if err := persister.SaveParseCache(source, d.path, fp, d.offset); err != nil {
			log.Printf("[cache] PersistPending error source=%s path=%s err=%v", source, d.path, err)
			lastErr = err
		}
	}
	if lastErr == nil {
		log.Printf("[cache] PersistPending ok source=%s count=%d", source, len(persistData))
	}
	return lastErr
}

// DiscardPending clears all pending fingerprints without persisting them.
// Use after a failed write to ensure files will be re-parsed on next run.
func (c *ParseCache) DiscardPending() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, path := range c.pending {
		delete(c.store, path)
	}
	c.pending = nil
}
