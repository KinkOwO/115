package catalog

import (
	"sync"
	"testing"
)

func TestSizedCacheEvictsOnlyOldEntriesAndPreservesHeldValues(t *testing.T) {
	var c SizedCache[int, []byte]
	held := []byte("held")
	c.Put(1, held, 4, 8, 2)
	c.Put(2, []byte("cold"), 4, 8, 2)
	c.Get(1)
	c.Put(3, []byte("next"), 4, 8, 2)
	if _, ok := c.Get(1); !ok {
		t.Fatal("hot entry lost")
	}
	if _, ok := c.Get(2); ok {
		t.Fatal("old entry retained")
	}
	c.Put(4, make([]byte, 9), 9, 8, 2)
	if _, ok := c.Get(4); ok {
		t.Fatal("oversized entry cached")
	}
	if n, count := c.Usage(); n > 8 || count > 2 {
		t.Fatal(n, count)
	}
	c.Clear()
	if string(held) != "held" {
		t.Fatal("held value mutated")
	}
}
func TestSizedCacheConcurrentReadsStayBounded(t *testing.T) {
	var c SizedCache[int, int]
	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Go(func() {
			for i := range 1000 {
				key := worker*1000 + i
				c.Put(key, key, 1, 64, 32)
				c.Get(key)
			}
		})
	}
	wg.Wait()
	if n, count := c.Usage(); n > 64 || count > 32 {
		t.Fatal(n, count)
	}
}
