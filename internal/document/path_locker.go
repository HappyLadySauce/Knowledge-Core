package document

import (
	"sort"
	"sync"
)

type pathLocker struct {
	mu    sync.Mutex
	locks map[string]*refLock
}

type refLock struct {
	mu   sync.Mutex
	refs int
}

type heldPathLock struct {
	key  string
	lock *refLock
}

func newPathLocker() *pathLocker {
	return &pathLocker{locks: make(map[string]*refLock)}
}

// lock serializes file operations for the same relative content paths.
// lock 串行化同一相对内容路径上的文件操作。
func (l *pathLocker) lock(paths ...string) func() {
	keys := uniqueSortedPaths(paths)
	held := make([]heldPathLock, 0, len(keys))
	for _, key := range keys {
		current := l.acquire(key)
		current.mu.Lock()
		held = append(held, heldPathLock{key: key, lock: current})
	}
	return func() {
		for i := len(held) - 1; i >= 0; i-- {
			held[i].lock.mu.Unlock()
			l.release(held[i].key, held[i].lock)
		}
	}
}

func (l *pathLocker) acquire(key string) *refLock {
	l.mu.Lock()
	defer l.mu.Unlock()
	current := l.locks[key]
	if current == nil {
		current = &refLock{}
		l.locks[key] = current
	}
	current.refs++
	return current
}

func (l *pathLocker) release(key string, current *refLock) {
	l.mu.Lock()
	defer l.mu.Unlock()
	current.refs--
	if current.refs == 0 {
		delete(l.locks, key)
	}
}

func uniqueSortedPaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	keys := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		keys = append(keys, path)
	}
	sort.Strings(keys)
	return keys
}
