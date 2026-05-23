package b2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v5"
	rcloneb2 "github.com/rclone/rclone/backend/b2"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/sirupsen/logrus"

	odicrypt "github.com/denysvitali/odi/pkg/crypt"
	"github.com/denysvitali/odi/pkg/models"
	"github.com/denysvitali/odi/pkg/storage/model"
	"github.com/denysvitali/odi/pkg/storage/rclone"
)

var log = logrus.StandardLogger().WithField("package", "storage/b2")
var _ model.Storer = (*B2)(nil)
var _ model.Retriever = (*B2)(nil)
var _ model.Deleter = (*B2)(nil)
var _ model.PageLister = (*B2)(nil)

const (
	defaultChunkSize = 5 * 1024 * 1024 // 5 MiB

	retryInitialInterval = 200 * time.Millisecond
	retryMaxInterval     = 5 * time.Second
	retryMaxElapsedTime  = 30 * time.Second
)

type B2 struct {
	b2FS       fs.Fs
	bucketName string
	crypt      *odicrypt.OdiCrypt
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	// Network-level and transient HTTP errors
	return strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "no such host") ||
		strings.Contains(err.Error(), "temporary") ||
		strings.Contains(err.Error(), "Too Many Requests")
}

func withRetry(ctx context.Context, op func() error) error {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = retryInitialInterval
	b.MaxInterval = retryMaxInterval
	_, err := backoff.Retry(ctx, func() (struct{}, error) {
		err := op()
		if err != nil && !isRetryable(err) {
			return struct{}{}, backoff.Permanent(err)
		}
		return struct{}{}, err
	}, backoff.WithBackOff(b), backoff.WithMaxElapsedTime(retryMaxElapsedTime), backoff.WithNotify(func(err error, d time.Duration) {
		log.WithError(err).Warnf("retrying B2 operation in %s", d)
	}))
	return err
}

func (b *B2) Store(ctx context.Context, page models.ScannedPage) (err error) {
	key := fileName(page.ScanID, page.SequenceID)

	if b.crypt != nil {
		page.Reader, err = b.crypt.Encrypt(page.Reader)
		if err != nil {
			return fmt.Errorf("encrypt page %s: %w", key, err)
		}
	}

	fileSize, err := page.Reader.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("seek end for page %s: %w", key, err)
	}
	_, err = page.Reader.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("seek start for page %s: %w", key, err)
	}

	return withRetry(ctx, func() error {
		obj, err := b.b2FS.Put(ctx, page.Reader, b.toStorageFile(page, fileSize), &fs.RangeOption{Start: 0, End: fileSize})
		if err != nil {
			return fmt.Errorf("put object %s to bucket %s: %w", key, b.bucketName, err)
		}
		log.Debugf("obj=%+v", obj)
		return nil
	})
}

func fileName(scanID string, sequenceNumber int) string {
	return fmt.Sprintf("%s/%d.jpg", scanID, sequenceNumber)
}

func thumbnailKey(scanID string, sequenceNumber int) string {
	return fmt.Sprintf("%s/%d_thumb.jpg", scanID, sequenceNumber)
}

func (b *B2) toStorageFile(page models.ScannedPage, fileSize int64) fs.ObjectInfo {
	return rclone.NewSourceFile(
		b.bucketName,
		fileName(page.ScanID, page.SequenceID),
		page.ScanTime,
		fileSize,
	)
}

func (b *B2) Retrieve(ctx context.Context, scanID string, sequenceId int) (*models.ScannedPage, error) {
	key := fileName(scanID, sequenceId)
	var obj fs.Object
	if err := withRetry(ctx, func() error {
		var err error
		obj, err = b.b2FS.NewObject(ctx, key)
		return err
	}); err != nil {
		if errors.Is(err, fs.ErrorObjectNotFound) {
			log.Debugf("object not found in bucket %s: %s", b.bucketName, key)
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("lookup object %s in bucket %s: %w", key, b.bucketName, err)
	}

	var reader io.ReadSeeker
	objReader, err := obj.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("open object %s from bucket %s: %w", key, b.bucketName, err)
	}
	defer objReader.Close()

	if b.crypt != nil {
		reader, err = b.crypt.Decrypt(objReader)
		if err != nil {
			return nil, fmt.Errorf("decrypt object %s: %w", key, err)
		}
	} else {
		buffer := bytes.NewBuffer(nil)
		_, err = io.Copy(buffer, objReader)
		if err != nil {
			return nil, fmt.Errorf("read object %s from bucket %s: %w", key, b.bucketName, err)
		}
		reader = bytes.NewReader(buffer.Bytes())
	}

	return &models.ScannedPage{
		Reader:     reader,
		ScanID:     scanID,
		SequenceID: sequenceId,
		ScanTime:   obj.ModTime(ctx),
	}, nil
}

// ThumbnailExists checks if a thumbnail exists for the given scan and sequence
func (b *B2) ThumbnailExists(ctx context.Context, scanID string, sequenceId int) (bool, error) {
	key := thumbnailKey(scanID, sequenceId)
	_, err := b.b2FS.NewObject(ctx, key)
	if err != nil {
		if errors.Is(err, fs.ErrorObjectNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// StoreThumbnail stores a thumbnail image for the given scan and sequence
func (b *B2) StoreThumbnail(ctx context.Context, scanID string, sequenceId int, reader io.Reader) error {
	key := thumbnailKey(scanID, sequenceId)

	// Read into buffer since we need file size and reader may not be seekable
	var buf bytes.Buffer
	_, err := io.Copy(&buf, reader)
	if err != nil {
		return fmt.Errorf("read thumbnail %s into buffer: %w", key, err)
	}
	fileSize := int64(buf.Len())

	var readerToStore io.Reader = bytes.NewReader(buf.Bytes())
	if b.crypt != nil {
		// Encrypt from a fresh reader so the entire buffer is consumed.
		encryptedReader, err := b.crypt.Encrypt(bytes.NewReader(buf.Bytes()))
		if err != nil {
			return fmt.Errorf("encrypt thumbnail %s: %w", key, err)
		}
		encryptedSize, err := encryptedReader.Seek(0, io.SeekEnd)
		if err != nil {
			return fmt.Errorf("seek encrypted thumbnail %s: %w", key, err)
		}
		_, err = encryptedReader.Seek(0, io.SeekStart)
		if err != nil {
			return fmt.Errorf("rewind encrypted thumbnail %s: %w", key, err)
		}
		readerToStore = encryptedReader
		fileSize = encryptedSize
	}

	obj, err := b.b2FS.Put(ctx, readerToStore, b.toThumbnailStorageFile(scanID, sequenceId, fileSize), &fs.RangeOption{Start: 0, End: fileSize})
	if err != nil {
		return fmt.Errorf("put thumbnail %s to bucket %s: %w", key, b.bucketName, err)
	}
	log.Debugf("thumbnail obj=%+v", obj)
	return nil
}

func (b *B2) toThumbnailStorageFile(scanID string, sequenceId int, fileSize int64) fs.ObjectInfo {
	return rclone.NewSourceFile(
		b.bucketName,
		thumbnailKey(scanID, sequenceId),
		time.Now(),
		fileSize,
	)
}

// RetrieveThumbnail retrieves a thumbnail image
func (b *B2) RetrieveThumbnail(ctx context.Context, scanID string, sequenceId int) (*models.ThumbnailPage, error) {
	key := thumbnailKey(scanID, sequenceId)
	obj, err := b.b2FS.NewObject(ctx, key)
	if err != nil {
		if errors.Is(err, fs.ErrorObjectNotFound) {
			log.Debugf("thumbnail not found in bucket %s: %s", b.bucketName, key)
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("lookup thumbnail %s in bucket %s: %w", key, b.bucketName, err)
	}

	var reader io.ReadSeeker
	objReader, err := obj.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("open thumbnail %s from bucket %s: %w", key, b.bucketName, err)
	}
	defer objReader.Close()

	if b.crypt != nil {
		reader, err = b.crypt.Decrypt(objReader)
		if err != nil {
			return nil, fmt.Errorf("decrypt thumbnail %s: %w", key, err)
		}
	} else {
		buffer := bytes.NewBuffer(nil)
		_, err = io.Copy(buffer, objReader)
		if err != nil {
			return nil, fmt.Errorf("read thumbnail %s from bucket %s: %w", key, b.bucketName, err)
		}
		reader = bytes.NewReader(buffer.Bytes())
	}

	return &models.ThumbnailPage{
		Reader:     reader,
		ScanID:     scanID,
		SequenceID: sequenceId,
	}, nil
}

// ListFiles returns a list of files for a given scan
func (b *B2) ListFiles(scanID string) ([]models.ScannedPage, error) {
	ctx := context.Background()
	objects, err := b.b2FS.List(ctx, scanID)
	if err != nil {
		return nil, fmt.Errorf("list files for scan %s in bucket %s: %w", scanID, b.bucketName, err)
	}

	var files []models.ScannedPage
	for _, obj := range objects {
		files = append(files, objToScannedPage(obj))
	}
	return files, nil
}

func (b *B2) Delete(ctx context.Context, scanID string, sequenceId int) error {
	key := fileName(scanID, sequenceId)
	var obj fs.Object
	if err := withRetry(ctx, func() error {
		var err error
		obj, err = b.b2FS.NewObject(ctx, key)
		return err
	}); err != nil {
		if errors.Is(err, fs.ErrorObjectNotFound) {
			return model.ErrNotFound
		}
		return fmt.Errorf("lookup object %s in bucket %s: %w", key, b.bucketName, err)
	}
	if err := withRetry(ctx, func() error {
		return obj.Remove(ctx)
	}); err != nil {
		return fmt.Errorf("remove object %s from bucket %s: %w", key, b.bucketName, err)
	}
	return nil
}

func objToScannedPage(obj fs.DirEntry) models.ScannedPage {
	s := models.ScannedPage{}
	fileName := path.Base(obj.Remote())
	scanID := path.Dir(obj.Remote())
	s.ScanID = scanID
	fileName = strings.TrimSuffix(fileName, ".jpg")
	seqId, err := strconv.ParseInt(fileName, 10, 64)
	if err != nil {
		log.Warnf("failed to parse sequence ID from %q: %v", fileName, err)
	}
	s.SequenceID = int(seqId)
	return s
}

func scannedPageFromRemote(remote string) (models.ScannedPage, bool) {
	if !strings.HasSuffix(remote, ".jpg") || strings.HasSuffix(remote, "_thumb.jpg") {
		return models.ScannedPage{}, false
	}
	scanID := path.Dir(remote)
	if scanID == "." || scanID == "/" || scanID == "" {
		return models.ScannedPage{}, false
	}
	name := strings.TrimSuffix(path.Base(remote), ".jpg")
	seqID, err := strconv.Atoi(name)
	if err != nil || seqID <= 0 {
		return models.ScannedPage{}, false
	}
	return models.ScannedPage{ScanID: scanID, SequenceID: seqID}, true
}

func (b *B2) ListPages(ctx context.Context) ([]models.ScannedPage, error) {
	roots, err := b.b2FS.List(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("list root in bucket %s: %w", b.bucketName, err)
	}

	var pages []models.ScannedPage
	for _, entry := range roots {
		if page, ok := scannedPageFromRemote(entry.Remote()); ok {
			pages = append(pages, page)
			continue
		}
		if strings.Contains(path.Base(entry.Remote()), ".") {
			continue
		}

		entries, err := b.b2FS.List(ctx, entry.Remote())
		if err != nil {
			return nil, fmt.Errorf("list scan %s in bucket %s: %w", entry.Remote(), b.bucketName, err)
		}
		for _, obj := range entries {
			if page, ok := scannedPageFromRemote(obj.Remote()); ok {
				pages = append(pages, page)
			}
		}
	}
	return pages, nil
}

type Config struct {
	Account    string
	Key        string
	BucketName string

	// Encryption specific
	Passphrase string
}

var nonceWarnOnce sync.Once

func New(config Config) (*B2, error) {
	if config.Account == "" {
		return nil, fmt.Errorf("account is required")
	}
	if config.Key == "" {
		return nil, fmt.Errorf("key is required")
	}
	if config.BucketName == "" {
		return nil, fmt.Errorf("bucket name is required")
	}

	if len(config.Passphrase) == 0 {
		log.Warnf("no passphrase provided for bucket %s, encryption will be disabled", config.BucketName)
	}

	b2FS, err := rcloneb2.NewFs(context.Background(),
		"b2",
		config.BucketName+"/",
		configmap.Simple{
			"account":    config.Account,
			"key":        config.Key,
			"chunk_size": fmt.Sprintf("%d", defaultChunkSize),
		},
	)

	if err != nil {
		return nil, fmt.Errorf("create B2 filesystem for bucket %s: %w", config.BucketName, err)
	}

	b := &B2{
		bucketName: config.BucketName,
		b2FS:       b2FS,
	}

	if len(config.Passphrase) != 0 {
		// Get key from passphrase
		b.crypt, err = odicrypt.New(config.Passphrase)
		if err != nil {
			return nil, fmt.Errorf("derive encryption key from passphrase for bucket %s: %w", config.BucketName, err)
		}
		nonceWarnOnce.Do(func() {
			log.Warn("AES-GCM random-nonce mode: do not encrypt more than ~64 GB total under a single passphrase without key rotation")
		})
	}

	return b, nil
}
