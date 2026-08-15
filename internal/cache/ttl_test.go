package cache

import (
	"testing"
	"time"
)

func TestTTLSetGet(t *testing.T) {
	c := New[string](time.Minute)
	c.Set("a", "1")
	if v, ok := c.Get("a"); !ok || v != "1" {
		t.Fatalf("Get(a) = %q, %v; want %q, true", v, ok, "1")
	}
	if _, ok := c.Get("missing"); ok {
		t.Fatal("Get(missing) should be false")
	}
}

func TestTTLExpiry(t *testing.T) {
	c := New[int](30 * time.Millisecond)
	c.Set("k", 42)
	if _, ok := c.Get("k"); !ok {
		t.Fatal("entry should exist before expiry")
	}
	time.Sleep(60 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Fatal("entry should have expired")
	}
}

func TestTTLOverwrite(t *testing.T) {
	c := New[string](time.Minute)
	c.Set("k", "old")
	c.Set("k", "new")
	if v, _ := c.Get("k"); v != "new" {
		t.Fatalf("Get(k) = %q, want %q", v, "new")
	}
}

func TestTTLZeroTTL(t *testing.T) {
	// Zero TTL disables expiry entirely.
	c := New[string](0)
	c.Set("k", "v")
	time.Sleep(5 * time.Millisecond)
	if v, ok := c.Get("k"); !ok || v != "v" {
		t.Fatalf("zero-TTL entry lost: %q %v", v, ok)
	}
}

func TestTTLClear(t *testing.T) {
	c := New[string](time.Minute)
	c.Set("a", "1")
	c.Set("b", "2")
	c.Clear()
	if c.Len() != 0 {
		t.Fatalf("Len after Clear = %d, want 0", c.Len())
	}
}

func TestTTLSweepOnSet(t *testing.T) {
	// Expired entries must be swept when a new Set runs, so Len never
	// reports stale keys.
	c := New[int](30 * time.Millisecond)
	c.Set("expired", 1)
	time.Sleep(60 * time.Millisecond)
	c.Set("fresh", 2)
	if got := c.Len(); got != 1 {
		t.Fatalf("Len after sweep = %d, want 1 (only the fresh entry)", got)
	}
	if _, ok := c.Get("expired"); ok {
		t.Fatal("expired entry should be gone after sweep")
	}
}

func TestTTLManyKeys(t *testing.T) {
	// Ensure the water-mark compaction path is exercised without error.
	c := New[string](time.Minute)
	for i := 0; i < 5000; i++ {
		c.Set(string(rune(i%26))+string(rune('a'+i/26)), "v")
	}
	if c.Len() != 5000 {
		t.Fatalf("Len = %d, want 5000", c.Len())
	}
	if _, ok := c.Get(string(rune(0)) + "a"); !ok {
		t.Fatal("expected first key to still be present")
	}
}

func TestTTLConcurrent(t *testing.T) {
	c := New[int](time.Minute)
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func(id int) {
			for j := 0; j < 200; j++ {
				key := string(rune('a' + id))
				c.Set(key, j)
				_, _ = c.Get(key)
			}
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 8; i++ {
		<-done
	}
	if c.Len() == 0 {
		t.Fatal("cache should contain entries after concurrent access")
	}
}
