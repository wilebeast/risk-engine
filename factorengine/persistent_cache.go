package factorengine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type PersistedProgramEntry struct {
	Key         string               `json:"key"`
	Fingerprint string               `json:"fingerprint"`
	CreatedAt   time.Time            `json:"created_at"`
	ExpiresAt   time.Time            `json:"expires_at"`
	Compiled    compiledExprSnapshot `json:"compiled"`
}

type PersistentProgramStore interface {
	Load(key string) (*PersistedProgramEntry, bool, error)
	Save(entry PersistedProgramEntry) error
	Delete(key string) error
}

type FileProgramStore struct {
	dir string
	mu  sync.Mutex
}

func NewFileProgramStore(dir string) *FileProgramStore {
	return &FileProgramStore{dir: dir}
}

func (s *FileProgramStore) Load(key string) (*PersistedProgramEntry, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.pathForKey(key))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var entry PersistedProgramEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil, false, err
	}
	return &entry, true, nil
}

func (s *FileProgramStore) Save(entry PersistedProgramEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return os.WriteFile(s.pathForKey(entry.Key), raw, 0o644)
}

func (s *FileProgramStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := os.Remove(s.pathForKey(key))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *FileProgramStore) pathForKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:])+".json")
}

type PersistentProgramCache struct {
	store         PersistentProgramStore
	local         ProgramCache
	ttl           time.Duration
	now           func() time.Time
	localExpiry   map[string]time.Time
	mu            sync.Mutex
	invalidations atomic.Uint64
}

func NewPersistentProgramCache(store PersistentProgramStore, local ProgramCache, ttl time.Duration) *PersistentProgramCache {
	if local == nil {
		local = NewLRUProgramCache(1024)
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &PersistentProgramCache{
		store:       store,
		local:       local,
		ttl:         ttl,
		now:         time.Now,
		localExpiry: make(map[string]time.Time),
	}
}

func (c *PersistentProgramCache) Get(key string) (CompiledExpr, bool) {
	if expr, ok := c.local.Get(key); ok {
		if !c.localEntryExpired(key) {
			return expr, true
		}
		c.deleteLocal(key)
	}
	if c.store == nil {
		return nil, false
	}

	entry, ok, err := c.store.Load(key)
	if err != nil || !ok {
		return nil, false
	}
	if c.isExpired(entry) {
		_ = c.store.Delete(key)
		return nil, false
	}

	expr, err := restoreCompiledExpr(entry.Compiled)
	if err != nil {
		_ = c.store.Delete(key)
		return nil, false
	}
	c.local.Set(key, expr)
	c.setLocalExpiry(key, entry.ExpiresAt)
	return expr, true
}

func (c *PersistentProgramCache) Set(key string, expr CompiledExpr) {
	c.local.Set(key, expr)
	now := c.now()
	expiresAt := now.Add(c.ttl)
	c.setLocalExpiry(key, expiresAt)
	if c.store == nil {
		return
	}
	snapshot, err := snapshotCompiledExpr(expr)
	if err != nil {
		return
	}
	_ = c.store.Save(PersistedProgramEntry{
		Key:         key,
		Fingerprint: expr.Fingerprint(),
		CreatedAt:   now,
		ExpiresAt:   expiresAt,
		Compiled:    snapshot,
	})
}

func (c *PersistentProgramCache) Invalidate(key string) error {
	c.invalidations.Add(1)
	c.deleteLocal(key)
	if c.store != nil {
		if err := c.store.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

func (c *PersistentProgramCache) isExpired(entry *PersistedProgramEntry) bool {
	return !entry.ExpiresAt.IsZero() && !entry.ExpiresAt.After(c.now())
}

func (c *PersistentProgramCache) localEntryExpired(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	expiresAt, ok := c.localExpiry[key]
	if !ok {
		return false
	}
	return !expiresAt.IsZero() && !expiresAt.After(c.now())
}

func (c *PersistentProgramCache) setLocalExpiry(key string, expiresAt time.Time) {
	c.mu.Lock()
	c.localExpiry[key] = expiresAt
	c.mu.Unlock()
}

func (c *PersistentProgramCache) deleteLocal(key string) {
	if invalidating, ok := c.local.(InvalidatingProgramCache); ok {
		invalidating.Delete(key)
	}
	c.mu.Lock()
	delete(c.localExpiry, key)
	c.mu.Unlock()
}

func (c *PersistentProgramCache) Invalidations() uint64 {
	return c.invalidations.Load()
}
