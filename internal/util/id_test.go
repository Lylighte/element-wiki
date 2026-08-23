package util

import (
	"strings"
	"sync"
	"testing"
)

func TestNewIDFormatAndUniqueness(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := NewID()
		if len(id) != 26 {
			t.Fatalf("ULID 长度应 26, got %q", id)
		}
		if strings.ToLower(id) != id && strings.ToUpper(id) != id {
			t.Fatalf("大小写混用: %q", id)
		}
		if seen[id] {
			t.Fatalf("ID 重复: %q", id)
		}
		seen[id] = true
	}
}

func TestNewIDMonotonicUnderConcurrency(t *testing.T) {
	const n = 200
	ids := make([]string, n)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mu.Lock()
			ids[i] = NewID()
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	for i := 1; i < n; i++ {
		if ids[i] == ids[i-1] {
			t.Fatalf("并发下产生重复 ID: %s", ids[i])
		}
	}
}
