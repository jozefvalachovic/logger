package audit

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"hash"
	"sync"
)

// HashChain provides tamper detection through cryptographic hash chaining
type HashChain struct {
	mu           sync.RWMutex
	algorithm    string
	previousHash string
	signingKey   []byte
	sequence     int64
}

// NewHashChain creates a new HashChain with the specified configuration
func NewHashChain(cfg HashChainConfig) *HashChain {
	algorithm := cfg.Algorithm
	if algorithm == "" {
		algorithm = "sha256"
	}
	return &HashChain{
		algorithm:    algorithm,
		previousHash: "",
		signingKey:   cfg.SigningKey,
		sequence:     0,
	}
}

// Chain adds an entry to the hash chain and returns the hash
func (h *HashChain) Chain(entry *AuditEntry) string {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.sequence++
	entry.Sequence = h.sequence
	entry.PreviousHash = h.previousHash

	content := h.hashContent(entry)
	var hashStr string

	if len(h.signingKey) > 0 {
		hashStr = h.hmacHash(content)
	} else {
		hashStr = h.plainHash(content)
	}

	entry.Hash = hashStr
	h.previousHash = hashStr

	return hashStr
}

// Verify checks if an entry's hash is valid
func (h *HashChain) Verify(entry *AuditEntry) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	content := h.hashContent(entry)
	var expectedHash string

	if len(h.signingKey) > 0 {
		expectedHash = h.hmacHash(content)
	} else {
		expectedHash = h.plainHash(content)
	}

	return entry.Hash == expectedHash
}

// VerifyChain verifies a sequence of entries forms a valid chain
func (h *HashChain) VerifyChain(entries []AuditEntry) error {
	if len(entries) == 0 {
		return nil
	}

	for i := 1; i < len(entries); i++ {
		if entries[i].PreviousHash != entries[i-1].Hash {
			return ErrHashChainBroken
		}
		if !h.Verify(&entries[i]) {
			return ErrHashChainBroken
		}
	}
	return nil
}

// GetSequence returns the current sequence number
func (h *HashChain) GetSequence() int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sequence
}

// SetState restores the hash chain state (used for recovery)
func (h *HashChain) SetState(previousHash string, sequence int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.previousHash = previousHash
	h.sequence = sequence
}

func (h *HashChain) hashContent(entry *AuditEntry) []byte {
	type hashableEntry struct {
		ID           string     `json:"id"`
		Timestamp    int64      `json:"timestamp"`
		Event        AuditEvent `json:"event"`
		Sequence     int64      `json:"sequence"`
		PreviousHash string     `json:"previous_hash"`
	}

	he := hashableEntry{
		ID:           entry.ID,
		Timestamp:    entry.Timestamp.UnixNano(),
		Event:        entry.Event,
		Sequence:     entry.Sequence,
		PreviousHash: entry.PreviousHash,
	}

	data, _ := json.Marshal(he)
	return data
}

func (h *HashChain) plainHash(data []byte) string {
	var hasher hash.Hash
	switch h.algorithm {
	case "sha512":
		hasher = sha512.New()
	default:
		hasher = sha256.New()
	}
	hasher.Write(data)
	return hex.EncodeToString(hasher.Sum(nil))
}

func (h *HashChain) hmacHash(data []byte) string {
	var hasher hash.Hash
	switch h.algorithm {
	case "sha512":
		hasher = hmac.New(sha512.New, h.signingKey)
	default:
		hasher = hmac.New(sha256.New, h.signingKey)
	}
	hasher.Write(data)
	return hex.EncodeToString(hasher.Sum(nil))
}
