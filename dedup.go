package logger

import (
	"fmt"
	"maps"
	"sync"
	"sync/atomic"
	"time"
)

type dedupManager struct {
	mu      sync.Mutex
	entries map[string]*dedupEntry
	window  time.Duration
	stopCh  chan struct{}
}

type dedupEntry struct {
	count     int
	level     LogLevel
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
		entries: make(map[string]*dedupEntry),
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

func (d *dedupManager) ShouldLog(level LogLevel, msg string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if e, ok := d.entries[msg]; ok {
		e.count++
		return false
	}
	d.entries[msg] = &dedupEntry{
		count:     1,
		level:     level,
		firstSeen: time.Now(),
	}
	return true
}

func (d *dedupManager) Flush() {
	d.mu.Lock()
	expired := make(map[string]*dedupEntry, len(d.entries))
	maps.Copy(expired, d.entries)
	d.entries = make(map[string]*dedupEntry)
	d.mu.Unlock()
	for msg, e := range expired {
		if e.count > 1 {
			logInternalSync(e.level, fmt.Sprintf("%s (repeated %d more times)", msg, e.count-1), 0)
		}
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
	type expiredItem struct {
		msg   string
		entry *dedupEntry
	}
	var expired []expiredItem
	for k, v := range d.entries {
		if now.Sub(v.firstSeen) >= d.window {
			expired = append(expired, expiredItem{k, v})
			delete(d.entries, k)
		}
	}
	d.mu.Unlock()
	for _, e := range expired {
		if e.entry.count > 1 {
			logInternalSync(e.entry.level, fmt.Sprintf("%s (repeated %d more times)", e.msg, e.entry.count-1), 0)
		}
	}
}

func (d *dedupManager) Stop() {
	select {
	case <-d.stopCh:
	default:
		close(d.stopCh)
	}
}
