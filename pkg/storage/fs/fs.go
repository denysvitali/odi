package fs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/denysvitali/odi/pkg/models"
	"github.com/denysvitali/odi/pkg/storage/model"
)

// validateScanID validates that the scanID only contains safe characters
// and does not contain path traversal sequences.
var validateScanID = model.ValidateScanID

var log = logrus.StandardLogger().WithField("package", "storage/fs")

var plaintextWarnOnce sync.Once

type Fs struct {
	dir string
}

func (fs *Fs) Retrieve(ctx context.Context, scanID string, sequenceNumber int) (*models.ScannedPage, error) {
	if err := validateScanID(scanID); err != nil {
		return nil, err
	}
	p := path.Join(fs.dir, scanID, fmt.Sprintf("%d.jpg", sequenceNumber))
	//nolint:gosec // path is validated by validateScanID before use.
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("open file %s: %w", p, err)
	}
	return &models.ScannedPage{
		ScanID:     scanID,
		SequenceID: sequenceNumber,
		Reader:     f,
	}, nil
}

func (fs *Fs) Store(ctx context.Context, page models.ScannedPage) error {
	if err := validateScanID(page.ScanID); err != nil {
		return err
	}
	dir := path.Join(fs.dir, page.ScanID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create directory %s for scan %s: %w", dir, page.ScanID, err)
	}

	finalPath := path.Join(dir, fmt.Sprintf("%d.jpg", page.SequenceID))
	tmpPath := finalPath + ".tmp." + uuid.New().String()

	//nolint:gosec // path is validated by validateScanID before use.
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create temp file %s for scan %s page %d: %w", tmpPath, page.ScanID, page.SequenceID, err)
	}

	written, copyErr := io.Copy(f, page.Reader)
	syncErr := f.Sync()
	closeErr := f.Close()

	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write page %d of scan %s to %s: %w", page.SequenceID, page.ScanID, tmpPath, copyErr)
	}
	if syncErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("sync page %d of scan %s to %s: %w", page.SequenceID, page.ScanID, tmpPath, syncErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file for page %d of scan %s: %w", page.SequenceID, page.ScanID, closeErr)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename %s -> %s for page %d of scan %s: %w", tmpPath, finalPath, page.SequenceID, page.ScanID, err)
	}

	// Sync the directory to ensure the rename is durable.
	//nolint:gosec // path is validated by validateScanID before use.
	dirFile, err := os.Open(dir)
	if err == nil {
		_ = dirFile.Sync()
		_ = dirFile.Close()
	}

	if _, err := page.Reader.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind reader for page %d of scan %s: %w", page.SequenceID, page.ScanID, err)
	}
	log.Debugf("Created file %s (%d bytes)", finalPath, written)
	return nil
}

func (fs *Fs) Delete(ctx context.Context, scanID string, sequenceNumber int) error {
	if err := validateScanID(scanID); err != nil {
		return err
	}
	p := path.Join(fs.dir, scanID, fmt.Sprintf("%d.jpg", sequenceNumber))
	if err := os.Remove(p); err != nil {
		if os.IsNotExist(err) {
			return model.ErrNotFound
		}
		return fmt.Errorf("remove file %s: %w", p, err)
	}
	return nil
}

var _ model.Storer = (*Fs)(nil)
var _ model.Retriever = (*Fs)(nil)
var _ model.Deleter = (*Fs)(nil)
var _ model.PageLister = (*Fs)(nil)

func (fs *Fs) ListPages(ctx context.Context) ([]models.ScannedPage, error) {
	scans, err := os.ReadDir(fs.dir)
	if err != nil {
		return nil, fmt.Errorf("read storage directory %s: %w", fs.dir, err)
	}

	var pages []models.ScannedPage
	for _, scan := range scans {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !scan.IsDir() {
			continue
		}
		scanID := scan.Name()
		if err := validateScanID(scanID); err != nil {
			log.Warnf("skipping invalid scan directory %q: %v", scanID, err)
			continue
		}
		files, err := os.ReadDir(path.Join(fs.dir, scanID))
		if err != nil {
			return nil, fmt.Errorf("read scan directory %s: %w", scanID, err)
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			name := file.Name()
			if path.Ext(name) != ".jpg" || strings.HasSuffix(name, "_thumb.jpg") {
				continue
			}
			seqID, err := strconv.Atoi(name[:len(name)-len(".jpg")])
			if err != nil || seqID <= 0 {
				continue
			}
			pages = append(pages, models.ScannedPage{ScanID: scanID, SequenceID: seqID})
		}
	}
	return pages, nil
}

func New(dir string) (*Fs, error) {
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0700)
		if err != nil {
			return nil, fmt.Errorf("unable to create storage directory: %w", err)
		}
	}

	plaintextWarnOnce.Do(func() {
		log.Warn("filesystem storage stores files in plaintext; use a FUSE-encrypted mount for at-rest encryption")
	})

	fs := &Fs{
		dir: dir,
	}
	return fs, nil
}
