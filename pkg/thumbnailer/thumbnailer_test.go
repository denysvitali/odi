package thumbnailer

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Fill with a non-uniform pattern so JPEG quantization is realistic
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}))
	return buf.Bytes()
}

func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 64, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func decodeThumb(t *testing.T, r io.Reader) (image.Image, string) {
	t.Helper()
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	img, format, err := image.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	return img, format
}

func TestGenerate_FromJPEG(t *testing.T) {
	src := makeJPEG(t, 1600, 2000)
	out, err := Generate(bytes.NewReader(src))
	require.NoError(t, err)
	require.NotNil(t, out)

	img, format, err := image.Decode(out.(*bytes.Buffer))
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
	// Height must be the target thumbnail height
	assert.Equal(t, thumbnailHeight, img.Bounds().Dy())
	// Width should be scaled proportionally (1600/2000 * 800 = 640)
	assert.Equal(t, 640, img.Bounds().Dx())
}

func TestGenerate_FromPNG(t *testing.T) {
	src := makePNG(t, 400, 800)
	out, err := Generate(bytes.NewReader(src))
	require.NoError(t, err)

	img, format := decodeThumb(t, out)
	// Output is always JPEG regardless of input format
	assert.Equal(t, "jpeg", format)
	assert.Equal(t, thumbnailHeight, img.Bounds().Dy())
	// 400/800 * 800 = 400
	assert.Equal(t, 400, img.Bounds().Dx())
}

func TestGenerate_SquareImage(t *testing.T) {
	src := makeJPEG(t, 1000, 1000)
	out, err := Generate(bytes.NewReader(src))
	require.NoError(t, err)
	img, _ := decodeThumb(t, out)
	assert.Equal(t, thumbnailHeight, img.Bounds().Dy())
	assert.Equal(t, thumbnailHeight, img.Bounds().Dx())
}

func TestGenerate_InvalidInput(t *testing.T) {
	_, err := Generate(strings.NewReader("not an image at all"))
	assert.Error(t, err)
}

func TestGenerate_EmptyInput(t *testing.T) {
	_, err := Generate(bytes.NewReader(nil))
	assert.Error(t, err)
}
