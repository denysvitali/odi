package fs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"

	"github.com/sirupsen/logrus"

	"github.com/denysvitali/odi/pkg/models"
	"github.com/denysvitali/odi/pkg/storage/model"
)

// validScanIDRegex matches only alphanumeric characters, hyphens, and underscores
var validScanIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// validateScanID validates that the scanID only contains safe characters
// and does not contain path traversal sequences
func validateScanID(scanID string) error {
	if scanID == "" {
		return fmt.Errorf("scanID cannot be empty")
	}
	if !validScanIDRegex.MatchString(scanID) {
		return fmt.Errorf("scanID contains invalid characters: only alphanumeric characters, hyphens, and underscores are allowed")
	}
	return nil
}

var log = logrus.StandardLogger().WithField("package", "storage/fs")

type Fs struct {
	dir string
}

func (fs *Fs) Retrieve(ctx context.Context, scanID string, sequenceNumber int) (*models.ScannedPage, error) {
	if err := validateScanID(scanID); err != nil {
		return nil, err
	}
	p := path.Join(fs.dir, scanID, fmt.Sprintf("%d.jpg", sequenceNumber))
	f, err := os.Open(p)
	if err != nil {
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
	// Check if directory exists
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0700)
		if err != nil {
			return fmt.Errorf("create directory %s for scan %s: %w", dir, page.ScanID, err)
		}
	}

	p := path.Join(dir, fmt.Sprintf("%d.jpg", page.SequenceID))
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create file %s for scan %s page %d: %w", p, page.ScanID, page.SequenceID, err)
	}
	defer f.Close()
	if _, err := io.Copy(f, page.Reader); err != nil {
		return fmt.Errorf("write page %d of scan %s to %s: %w", page.SequenceID, page.ScanID, p, err)
	}
	if _, err := page.Reader.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind reader for page %d of scan %s: %w", page.SequenceID, page.ScanID, err)
	}
	log.Debugf("Created file %s", f.Name())
	return nil
}

var _ model.Storer = (*Fs)(nil)
var _ model.Retriever = (*Fs)(nil)

func New(dir string) (*Fs, error) {
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0700)
		if err != nil {
			return nil, fmt.Errorf("unable to create storage directory: %w", err)
		}
	}

	fs := &Fs{
		dir: dir,
	}
	return fs, nil
}
