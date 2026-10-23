package config

import (
	"sync"
	"testing"
)

func TestEntries_ConcurrentSetAndRead(t *testing.T) {
	entry := asString("test.concurrent", "default")
	t.Cleanup(func() { _ = entry.Set("default", false) })

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = entry.Set("changed", false)
		}()
		go func() {
			defer wg.Done()
			_ = entry.Value()
			_ = WebHost.Value()
		}()
	}
	wg.Wait()

	if entry.Value() != "changed" {
		t.Fatalf("expected the last written value, got %q", entry.Value())
	}
}
