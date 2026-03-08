package audit

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

var (
	uuidCounter uint64
	machineID   string
)

func init() {
	machineID = getMachineID()
}

func getMachineID() string {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based entropy if crypto/rand fails
		ts := time.Now().UnixNano()
		for i := range b {
			b[i] = byte(ts >> (i * 8))
		}
	}
	return fmt.Sprintf("%s-%s", hostname[:min(8, len(hostname))], hex.EncodeToString(b))
}

// generateUUID generates a unique ID for audit entries
func generateUUID() string {
	now := time.Now()
	counter := atomic.AddUint64(&uuidCounter, 1)

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based entropy if crypto/rand fails
		ts := now.UnixNano()
		for i := range b {
			b[i] = byte(ts >> (i * 8))
		}
	}

	return fmt.Sprintf("%d-%s-%d-%s",
		now.UnixNano(),
		machineID,
		counter,
		hex.EncodeToString(b[:8]),
	)
}
