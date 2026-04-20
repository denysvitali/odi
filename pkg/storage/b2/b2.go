package b2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	rcloneb2 "github.com/rclone/rclone/backend/b2"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	odicrypt "github.com/denysvitali/odi/pkg/crypt"
	"github.com/denysvitali/odi/pkg/models"
	"github.com/denysvitali/odi/pkg/storage/model"
	"github.com/denysvitali/odi/pkg/storage/rclone"
)

var log = logrus.StandardLogger().WithField("package", "storage/b2")
var _ model.Storer = (*B2)(nil)
var _ model.Retriever = (*B2)(nil)

type B2 struct {
	b2FS       fs.Fs
	bucketName string
	crypt      *odicrypt.OdiCrypt
}

func (b *B2) Store(ctx context.Context, page models.ScannedPage) (err error) {
	key := fileName(page.ScanID, page.SequenceID)

	if b.crypt != nil {
		// The nonce needs to be unique, but not secure.
		// It should not be reused for more than 64GB of data for the same key.
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

	obj, err := b.b2FS.Put(ctx, page.Reader, b.toStorageFile(page, fileSize), &fs.RangeOption{Start: 0, End: fileSize})
	if err != nil {
		return fmt.Errorf("put object %s to bucket %s: %w", key, b.bucketName, err)
	}
	log.Debugf("obj=%+v", obj)
	return nil
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
	obj, err := b.b2FS.NewObject(ctx, key)
	if err != nil {
		if errors.Is(err, fs.ErrorObjectNotFound) {
			log.Debugf("object not found in bucket %s: %s", b.bucketName, key)
			return nil, os.ErrNotExist
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

	var readerToStore io.Reader = &buf
	if b.crypt != nil {
		encryptedReader, err := b.crypt.Encrypt(&buf)
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
			return nil, os.ErrNotExist
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

type Config struct {
	Account    string
	Key        string
	BucketName string

	// Encryption specific
	Passphrase string
}

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
			"chunk_size": "5M",
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
	}

	return b, nil
}
