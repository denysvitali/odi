package models

import (
	"fmt"
	"io"
	"time"
)

type ScannedPage struct {
	Reader        io.ReadSeeker
	ScanID        string
	SequenceID    int
	ScanTime      time.Time
	ContentDigest string
}

func (s ScannedPage) ID() string {
	return fmt.Sprintf("%s_%d", s.ScanID, s.SequenceID)
}

type ThumbnailPage struct {
	Reader     io.ReadSeeker
	ScanID     string
	SequenceID int
}
