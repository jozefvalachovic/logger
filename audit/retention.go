package audit

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// RetentionManager handles audit log retention and archival
type RetentionManager struct {
	mu       sync.Mutex
	cfg      *RetentionConfig
	basePath string
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// NewRetentionManager creates a new retention manager
func NewRetentionManager(cfg *RetentionConfig, basePath string) *RetentionManager {
	if cfg == nil {
		return nil
	}

	rm := &RetentionManager{
		cfg:      cfg,
		basePath: basePath,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}

	return rm
}

// Start begins the retention cleanup routine
func (r *RetentionManager) Start() {
	if r == nil {
		return
	}

	interval := r.cfg.CleanupInterval
	if interval == 0 {
		interval = time.Hour
	}

	go func() {
		defer close(r.doneCh)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		r.cleanup()

		for {
			select {
			case <-ticker.C:
				r.cleanup()
			case <-r.stopCh:
				return
			}
		}
	}()
}

// Stop stops the retention manager
func (r *RetentionManager) Stop() {
	if r == nil {
		return
	}

	close(r.stopCh)
	<-r.doneCh
}

// Cleanup performs a manual cleanup run
func (r *RetentionManager) Cleanup() error {
	if r == nil {
		return nil
	}
	return r.cleanup()
}

func (r *RetentionManager) cleanup() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cfg.LegalHold {
		return nil
	}

	files, err := r.findLogFiles()
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return nil
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime().Before(files[j].ModTime())
	})

	var totalSize int64
	for _, f := range files {
		totalSize += f.Size()
	}

	now := time.Now()
	cutoff := now.Add(-r.cfg.MaxAge)

	for _, f := range files {
		shouldArchive := false
		reason := ""

		if r.cfg.MaxAge > 0 && f.ModTime().Before(cutoff) {
			shouldArchive = true
			reason = "age"
		}

		if r.cfg.MaxSize > 0 && totalSize > r.cfg.MaxSize {
			shouldArchive = true
			reason = "size"
		}

		if shouldArchive {
			path := filepath.Join(r.basePath, f.Name())

			if r.cfg.ArchivePath != "" {
				if err := r.archiveFile(path, reason); err != nil {
					continue
				}
			}

			if r.cfg.DeleteAfterArchive || r.cfg.ArchivePath == "" {
				os.Remove(path)
			}

			totalSize -= f.Size()
		}
	}

	return nil
}

func (r *RetentionManager) findLogFiles() ([]os.FileInfo, error) {
	entries, err := os.ReadDir(r.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []os.FileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".log") && !strings.HasSuffix(name, ".jsonl") {
			continue
		}

		if strings.HasSuffix(name, ".gz") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, info)
	}

	return files, nil
}

func (r *RetentionManager) archiveFile(sourcePath, reason string) error {
	if err := os.MkdirAll(r.cfg.ArchivePath, 0750); err != nil {
		return err
	}

	filename := filepath.Base(sourcePath)
	timestamp := time.Now().Format("20060102-150405")
	archiveName := fmt.Sprintf("%s.%s.%s", filename, timestamp, reason)

	if r.cfg.CompressArchive {
		archiveName += ".gz"
	}

	destPath := filepath.Join(r.cfg.ArchivePath, archiveName)

	if r.cfg.CompressArchive {
		return r.compressFile(sourcePath, destPath)
	}

	return r.copyFile(sourcePath, destPath)
}

func (r *RetentionManager) compressFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	gzWriter := gzip.NewWriter(dstFile)
	defer gzWriter.Close()

	_, err = io.Copy(gzWriter, srcFile)
	return err
}

func (r *RetentionManager) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
