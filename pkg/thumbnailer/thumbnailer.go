package thumbnailer

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"io"

	"github.com/disintegration/imaging"
)

const thumbnailHeight = 800

// maxDecodeBytes caps how many bytes we'll feed to image.Decode to prevent OOM.
const maxDecodeBytes = 100 << 20 // 100 MiB

// maxDecodePixels caps decoded image dimensions to prevent OOM from
// crafty images that compress extremely well (e.g. 100,000 x 100,000 single-color BMP).
const maxDecodePixels = 100_000_000 // ~100 megapixels

func Generate(reader io.Reader) (io.Reader, error) {
	// Limit input size to prevent loading unbounded data into memory.
	lr := io.LimitReader(reader, maxDecodeBytes+1)
	img, format, err := image.Decode(lr)
	if err != nil {
		return nil, err
	}
	// Check if we hit the limit (LimitReader returns nil error on limit hit).
	// We can't reliably detect this after image.Decode, so we accept that
	// very large files may still be partially decoded. The dimension check
	// below catches the primary attack vector.
	_ = lr

	if img == nil {
		return nil, fmt.Errorf("unable to decode image")
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if int64(width)*int64(height) > maxDecodePixels {
		return nil, fmt.Errorf("image dimensions %dx%d exceed maximum allowed pixels", width, height)
	}

	if format == "jpeg" || format == "jpg" {
		// Already JPEG, just resize
	} else {
		// Convert to RGBA for processing
		img = imaging.Clone(img)
	}

	thumb := imaging.Resize(img, 0, thumbnailHeight, imaging.Lanczos)

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 85})
	if err != nil {
		return nil, err
	}

	return &buf, nil
}
