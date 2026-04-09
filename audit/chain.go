package audit

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"sync"
)

// HashChain provides tamper detection through cryptographic hash chaining
type HashChain struct {
	mu               sync.RWMutex
	algorithm        string
	previousHash     string
	signingKey       []byte
	sequence         int64
	enableSignatures bool
	privateKey       ed25519.PrivateKey
}

// NewHashChain creates a new HashChain with the specified configuration
func NewHashChain(cfg HashChainConfig) *HashChain {
	algorithm := cfg.Algorithm
	if algorithm == "" {
		algorithm = "sha256"
	}
	hc := &HashChain{
		algorithm:        algorithm,
		previousHash:     "",
		signingKey:       cfg.SigningKey,
		sequence:         0,
		enableSignatures: cfg.EnableSignatures,
	}
	if cfg.EnableSignatures && len(cfg.PrivateKey) >= ed25519.PrivateKeySize {
		hc.privateKey = ed25519.PrivateKey(cfg.PrivateKey)
	}
	return hc
}

// Chain adds an entry to the hash chain and returns the hash
func (h *HashChain) Chain(entry *AuditEntry) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.sequence++
	entry.Sequence = h.sequence
	entry.PreviousHash = h.previousHash

	content, err := h.hashContent(entry)
	if err != nil {
		return "", err
	}
	var hashStr string

	if len(h.signingKey) > 0 {
		hashStr = h.hmacHash(content)
	} else {
		hashStr = h.plainHash(content)
	}

	entry.Hash = hashStr
	h.previousHash = hashStr

	// Sign the hash with Ed25519 if signatures are enabled
	if h.enableSignatures && len(h.privateKey) > 0 {
		sig := ed25519.Sign(h.privateKey, []byte(hashStr))
		entry.Signature = hex.EncodeToString(sig)
	}

	return hashStr, nil
}

// Verify checks if an entry's hash is valid
func (h *HashChain) Verify(entry *AuditEntry) (bool, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	content, err := h.hashContent(entry)
	if err != nil {
		return false, err
	}
	var expectedHash string

	if len(h.signingKey) > 0 {
		expectedHash = h.hmacHash(content)
	} else {
		expectedHash = h.plainHash(content)
	}

	return subtle.ConstantTimeCompare([]byte(entry.Hash), []byte(expectedHash)) == 1, nil
}

// VerifyChain verifies a sequence of entries forms a valid chain
func (h *HashChain) VerifyChain(entries []AuditEntry) error {
	if len(entries) == 0 {
		return nil
	}

	for i := 1; i < len(entries); i++ {
		if subtle.ConstantTimeCompare([]byte(entries[i].PreviousHash), []byte(entries[i-1].Hash)) != 1 {
			return ErrHashChainBroken
		}
		valid, err := h.Verify(&entries[i])
		if err != nil {
			return fmt.Errorf("audit: hash verification failed: %w", err)
		}
		if !valid {
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

func (h *HashChain) hashContent(entry *AuditEntry) ([]byte, error) {
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

	data, err := json.Marshal(he)
	if err != nil {
		return nil, fmt.Errorf("audit: failed to marshal hash content: %w", err)
	}
	return data, nil
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

// VerifySignature verifies the Ed25519 signature on an audit entry.
// Returns false if the entry has no signature or no private key is configured.
func (h *HashChain) VerifySignature(entry *AuditEntry) (bool, error) {
	if entry.Signature == "" {
		return false, nil
	}
	if len(h.privateKey) == 0 {
		return false, fmt.Errorf("audit: no private key for signature verification")
	}
	pubKey := h.privateKey.Public().(ed25519.PublicKey)
	sig, err := hex.DecodeString(entry.Signature)
	if err != nil {
		return false, fmt.Errorf("audit: invalid signature encoding: %w", err)
	}
	return ed25519.Verify(pubKey, []byte(entry.Hash), sig), nil
}

// VerifySignatureWithKey verifies a signature using the provided public key.
func VerifySignatureWithKey(entry *AuditEntry, publicKey ed25519.PublicKey) (bool, error) {
	if entry.Signature == "" {
		return false, nil
	}
	sig, err := hex.DecodeString(entry.Signature)
	if err != nil {
		return false, fmt.Errorf("audit: invalid signature encoding: %w", err)
	}
	return ed25519.Verify(publicKey, []byte(entry.Hash), sig), nil
}

// Close zeroes all sensitive key material held by the HashChain.
// After Close, the chain must not be used for signing or HMAC operations.
func (h *HashChain) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range h.signingKey {
		h.signingKey[i] = 0
	}
	h.signingKey = nil
	for i := range h.privateKey {
		h.privateKey[i] = 0
	}
	h.privateKey = nil
}
