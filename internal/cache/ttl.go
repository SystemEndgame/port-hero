// Package cache provides a small generic TTL cache used to bound the
// memory footprint and staleness of per-repo / per-UID / per-container
// lookups performed by the inspector and ancestry packages.
package cache

import (
	"sync"
	"time"
)

type entry[V any] struct {
	value V
	exp   time.Time
}

// TTL is a thread-safe generic cache with per-key expiry. Expiry is checked
// lazily per key on Get (O(1)), and a full sweep runs only on Set and Len so
// steady-state list refreshes never pay an O(N) scan. Memory stays bounded
// because the map is compacted whenever it outgrows twice its previous
// high-water mark.
type TTL[V any] struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]entry[V]
	water   int
}

// New creates a TTL cache where entries expire ttl after being stored.
// A zero ttl disables expiry (entries live until compacted or cleared).
func New[V any](ttl time.Duration) *TTL[V] {
	return &TTL[V]{
		ttl:     ttl,
		entries: make(map[string]entry[V]),
	}
}

// Get returns the value for key if present and not expired. It checks only
// the requested key (O(1)); expired keys are dropped on the fly, so no full
// scan happens on the hot list-refresh path.
func (c *TTL[V]) Get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		var zero V
		return zero, false
	}
	if c.ttl > 0 && !e.exp.After(time.Now()) {
		delete(c.entries, key)
		var zero V
		return zero, false
	}
	return e.value, true
}

// Set stores value for key, replacing any existing entry, and opportunistically
// sweeps expired entries so stale keys never accumulate.
func (c *TTL[V]) Set(key string, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ttl > 0 {
		c.removeExpiredLocked()
	}
	c.entries[key] = entry[V]{value: value, exp: time.Now().Add(c.ttl)}
	// Opportunistic compaction: keep the map bounded as keys churn.
	if len(c.entries) > c.water*2 && c.water > 0 {
		compacted := make(map[string]entry[V], len(c.entries))
		for k, v := range c.entries {
			compacted[k] = v
		}
		c.entries = compacted
	}
	c.water = maxInt(c.water, len(c.entries))
}

// Len returns the current number of live entries, sweeping expired ones.
func (c *TTL[V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ttl > 0 {
		c.removeExpiredLocked()
	}
	return len(c.entries)
}

// Clear removes every entry.
func (c *TTL[V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]entry[V])
	c.water = 0
}

func (c *TTL[V]) removeExpiredLocked() {
	now := time.Now()
	for k, e := range c.entries {
		if !e.exp.After(now) {
			delete(c.entries, k)
		}
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
