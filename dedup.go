package logger

import (
	"fmt"
	"hash/fnv"
	"maps"
	"sync"
	"sync/atomic"
	"time"
)

type dedupManager struct {
	mu      sync.Mutex
	entries map[uint64]*dedupEntry
	window  time.Duration
	stopCh  chan struct{}
}

type dedupEntry struct {
	count     int
	level     LogLevel
	msg       string // message of the first line, for the "(repeated N more times)" summary
	firstSeen time.Time
}

// dedupMgr holds the active dedup manager (nil when EnableDedup is false).
// Accessed atomically so the log path can read it without locking.
var dedupMgr atomic.Pointer[dedupManager]

func startDedup(window time.Duration) {
	if old := dedupMgr.Load(); old != nil {
		old.Stop()
	}
	dm := &dedupManager{
		entries: make(map[uint64]*dedupEntry),
		window:  window,
		stopCh:  make(chan struct{}),
	}
	dedupMgr.Store(dm)
	go dm.cleanup()
}

func stopDedup() {
	if old := dedupMgr.Swap(nil); old != nil {
		old.Stop()
	}
}

// dedupKey identifies a log line by its level, message and every key-value, in
// the order given. Key-values may be of any type, including uncomparable ones
// (slices, maps), so their formatted form is hashed rather than used as a map
// key. Each value's type is included so that 1 and "1" stay distinct, and a NUL
// separator keeps ("ab", "c") and ("a", "bc") apart. The key never leaves the
// dedup manager, so unredacted values are never printed.
func dedupKey(level LogLevel, msg string, keyValues []any) uint64 {
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%d\x00%s\x00", level, msg)
	for _, kv := range keyValues {
		_, _ = fmt.Fprintf(h, "%T=%v\x00", kv, kv)
	}
	return h.Sum64()
}

// ShouldLog reports whether a line should be written. A line is a duplicate,
// and is suppressed, only when its level, message and key-values all match a
// line already written within the window. Callers must not pass Audit lines:
// logInternal writes those unconditionally.
func (d *dedupManager) ShouldLog(level LogLevel, msg string, keyValues []any) bool {
	key := dedupKey(level, msg, keyValues)
	d.mu.Lock()
	defer d.mu.Unlock()
	if e, ok := d.entries[key]; ok {
		e.count++
		return false
	}
	d.entries[key] = &dedupEntry{
		count:     1,
		level:     level,
		msg:       msg,
		firstSeen: time.Now(),
	}
	return true
}

func (d *dedupManager) Flush() {
	d.mu.Lock()
	expired := make(map[uint64]*dedupEntry, len(d.entries))
	maps.Copy(expired, d.entries)
	d.entries = make(map[uint64]*dedupEntry)
	d.mu.Unlock()
	for _, e := range expired {
		e.logSummary()
	}
}

func (d *dedupManager) cleanup() {
	ticker := time.NewTicker(d.window)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			d.flushExpired()
		case <-d.stopCh:
			return
		}
	}
}

func (d *dedupManager) flushExpired() {
	d.mu.Lock()
	now := time.Now()
	var expired []*dedupEntry
	for k, v := range d.entries {
		if now.Sub(v.firstSeen) >= d.window {
			expired = append(expired, v)
			delete(d.entries, k)
		}
	}
	d.mu.Unlock()
	for _, e := range expired {
		e.logSummary()
	}
}

// logSummary writes the "(repeated N more times)" line for suppressed repeats,
// at the first line's level and with its message but no fields.
func (e *dedupEntry) logSummary() {
	if e.count > 1 {
		logInternalSync(e.level, fmt.Sprintf("%s (repeated %d more times)", e.msg, e.count-1), 0)
	}
}

func (d *dedupManager) Stop() {
	select {
	case <-d.stopCh:
	default:
		close(d.stopCh)
	}
}
