package thumbnailer

import (
	"bytes"
	"image"
	"image/jpeg"
	"io"

	"github.com/disintegration/imaging"
)

const thumbnailHeight = 800

func Generate(reader io.Reader) (io.Reader, error) {
	img, format, err := image.Decode(reader)
	if err != nil {
		return nil, err
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
