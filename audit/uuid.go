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

	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%s", hostname[:min(8, len(hostname))], hex.EncodeToString(b))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// generateUUID generates a unique ID for audit entries
func generateUUID() string {
	now := time.Now()
	counter := atomic.AddUint64(&uuidCounter, 1)

	b := make([]byte, 8)
	_, _ = rand.Read(b)

	return fmt.Sprintf("%d-%s-%d-%s",
		now.UnixNano(),
		machineID,
		counter,
		hex.EncodeToString(b[:4]),
	)
}
