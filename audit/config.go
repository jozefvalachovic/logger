package audit

import (
	"io"
	"os"
	"time"
)

// Config holds all configuration for enterprise audit logging
type Config struct {
	EnableStructured bool
	Output           io.Writer
	HashChain        HashChainConfig
	WAL              WALConfig
	Tracing          TracingConfig
	Service          *ServiceContext
	Sinks            []Sink
	RateLimit        *RateLimitConfig
	Retention        *RetentionConfig
	Store            Store
	Compliance       ComplianceStandard
	SampleRate       float64
	BufferSize       int
	FlushInterval    time.Duration
	WriteTimeout     time.Duration
	DeadLetterPath   string
	MaxRetries       int
	RetryBackoff     time.Duration
}

// HashChainConfig configures tamper detection
type HashChainConfig struct {
	Enabled          bool
	Algorithm        string
	SigningKey       []byte
	EnableSignatures bool
	PrivateKey       []byte
}

// WALConfig configures write-ahead logging
type WALConfig struct {
	Enabled            bool
	Path               string
	SyncOnWrite        bool
	MaxSize            int64
	CheckpointInterval time.Duration
}

// TracingConfig configures distributed tracing
type TracingConfig struct {
	Enabled           bool
	PropagationFormat string
	TraceIDHeader     string
	SpanIDHeader      string
	ParentIDHeader    string
}

// ServiceContext holds service identification for auto-enrichment
type ServiceContext struct {
	Name        string
	Version     string
	Environment string
	Region      string
	Instance    string
	Namespace   string
	Metadata    map[string]string
}

// RateLimitConfig configures rate limiting for audit writes
type RateLimitConfig struct {
	EventsPerSecond int
	BurstSize       int
	DropWhenLimited bool
	QueueSize       int
}

// RetentionConfig configures audit log retention
type RetentionConfig struct {
	MaxAge             time.Duration
	MaxSize            int64
	ArchivePath        string
	CompressArchive    bool
	DeleteAfterArchive bool
	LegalHold          bool
	CleanupInterval    time.Duration
}

// ComplianceStandard represents a compliance framework
type ComplianceStandard string

const (
	ComplianceNone    ComplianceStandard = ""
	ComplianceSOC2    ComplianceStandard = "soc2"
	ComplianceHIPAA   ComplianceStandard = "hipaa"
	CompliancePCIDSS  ComplianceStandard = "pci-dss"
	ComplianceGDPR    ComplianceStandard = "gdpr"
	ComplianceFedRAMP ComplianceStandard = "fedramp"
)

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() Config {
	return Config{
		EnableStructured: false,
		Output:           os.Stdout,
		HashChain: HashChainConfig{
			Enabled:   false,
			Algorithm: "sha256",
		},
		WAL: WALConfig{
			Enabled:            false,
			SyncOnWrite:        false,
			MaxSize:            100 << 20,
			CheckpointInterval: time.Minute,
		},
		Tracing: TracingConfig{
			Enabled:           false,
			PropagationFormat: "w3c",
			TraceIDHeader:     "traceparent",
		},
		SampleRate:    1.0,
		BufferSize:    1000,
		FlushInterval: time.Second,
		WriteTimeout:  10 * time.Second,
		MaxRetries:    3,
		RetryBackoff:  100 * time.Millisecond,
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.WAL.Enabled && c.WAL.Path == "" {
		return ErrWALPathRequired
	}
	if c.SampleRate < 0.0 || c.SampleRate > 1.0 {
		return ErrInvalidSampleRate
	}
	if c.RateLimit != nil && c.RateLimit.EventsPerSecond <= 0 {
		return ErrInvalidRateLimit
	}
	return nil
}

// WithCompliance applies compliance-specific defaults
func (c *Config) WithCompliance(standard ComplianceStandard) *Config {
	switch standard {
	case ComplianceSOC2:
		c.applySOC2Defaults()
	case ComplianceHIPAA:
		c.applyHIPAADefaults()
	case CompliancePCIDSS:
		c.applyPCIDSSDefaults()
	case ComplianceGDPR:
		c.applyGDPRDefaults()
	case ComplianceFedRAMP:
		c.applyFedRAMPDefaults()
	}
	c.Compliance = standard
	return c
}

func (c *Config) applySOC2Defaults() {
	c.EnableStructured = true
	c.HashChain.Enabled = true
	c.WAL.Enabled = true
	c.WAL.SyncOnWrite = true
	c.SampleRate = 1.0
	if c.Retention == nil {
		c.Retention = &RetentionConfig{
			MaxAge:             365 * 24 * time.Hour,
			CompressArchive:    true,
			DeleteAfterArchive: false,
		}
	}
}

func (c *Config) applyHIPAADefaults() {
	c.EnableStructured = true
	c.HashChain.Enabled = true
	c.HashChain.EnableSignatures = true
	c.WAL.Enabled = true
	c.WAL.SyncOnWrite = true
	c.SampleRate = 1.0
	if c.Retention == nil {
		c.Retention = &RetentionConfig{
			MaxAge:             6 * 365 * 24 * time.Hour,
			CompressArchive:    true,
			DeleteAfterArchive: false,
			LegalHold:          false,
		}
	}
}

func (c *Config) applyPCIDSSDefaults() {
	c.EnableStructured = true
	c.HashChain.Enabled = true
	c.WAL.Enabled = true
	c.WAL.SyncOnWrite = true
	c.SampleRate = 1.0
	if c.Retention == nil {
		c.Retention = &RetentionConfig{
			MaxAge:             365 * 24 * time.Hour,
			CompressArchive:    true,
			DeleteAfterArchive: false,
		}
	}
}

func (c *Config) applyGDPRDefaults() {
	c.EnableStructured = true
	c.HashChain.Enabled = true
	c.SampleRate = 1.0
	if c.Retention == nil {
		c.Retention = &RetentionConfig{
			MaxAge:             90 * 24 * time.Hour,
			CompressArchive:    true,
			DeleteAfterArchive: true,
		}
	}
}

func (c *Config) applyFedRAMPDefaults() {
	c.EnableStructured = true
	c.HashChain.Enabled = true
	c.HashChain.EnableSignatures = true
	c.WAL.Enabled = true
	c.WAL.SyncOnWrite = true
	c.SampleRate = 1.0
	if c.Retention == nil {
		c.Retention = &RetentionConfig{
			MaxAge:             3 * 365 * 24 * time.Hour,
			CompressArchive:    true,
			DeleteAfterArchive: false,
		}
	}
}

// NewServiceContextFromEnv creates ServiceContext from environment variables
func NewServiceContextFromEnv() *ServiceContext {
	return &ServiceContext{
		Name:        getEnvOrDefault("SERVICE_NAME", getEnvOrDefault("K8S_SERVICE_NAME", "unknown")),
		Version:     getEnvOrDefault("SERVICE_VERSION", getEnvOrDefault("APP_VERSION", "unknown")),
		Environment: getEnvOrDefault("ENVIRONMENT", getEnvOrDefault("ENV", "development")),
		Region:      getEnvOrDefault("REGION", getEnvOrDefault("AWS_REGION", "")),
		Instance:    getEnvOrDefault("INSTANCE_ID", getEnvOrDefault("K8S_POD_NAME", getEnvOrDefault("HOSTNAME", ""))),
		Namespace:   getEnvOrDefault("K8S_NAMESPACE", ""),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
