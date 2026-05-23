package rclone

import (
	"context"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/stretchr/testify/assert"
)

func TestDummyFs(t *testing.T) {
	d := DummyFs{}
	assert.Equal(t, "dummy", d.Name())
	assert.Equal(t, "/", d.Root())
	assert.Equal(t, "", d.String())
	assert.Equal(t, time.Second, d.Precision())
	assert.Equal(t, hash.Set(hash.None), d.Hashes())
	assert.NotNil(t, d.Features())
}

func TestSourceFile(t *testing.T) {
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	sf := NewSourceFile("remote-name", "path/to/file.jpg", now, 4096)

	assert.Equal(t, "path/to/file.jpg", sf.String())
	assert.Equal(t, "path/to/file.jpg", sf.Remote())
	assert.Equal(t, now, sf.ModTime(context.Background()))
	assert.Equal(t, int64(4096), sf.Size())
	assert.True(t, sf.Storable())

	// Hash should return empty without error
	h, err := sf.Hash(context.Background(), hash.MD5)
	assert.NoError(t, err)
	assert.Equal(t, "", h)

	// Fs is a DummyFs
	got := sf.Fs()
	assert.Equal(t, "dummy", got.Name())
}

// Compile-time interface assertions (also validated in production code,
// but kept here so tests catch interface drift in this package).
func TestInterfaceSatisfaction(t *testing.T) {
	var _ fs.Info = (*DummyFs)(nil)
	var _ fs.ObjectInfo = (*SourceFile)(nil)
}
