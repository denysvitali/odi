package ocrclient_test

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"os"
	"testing"

	"github.com/denysvitali/odi/pkg/ocrclient"
	"github.com/denysvitali/odi/pkg/ocrclient/caroundtripper"
	"github.com/stretchr/testify/require"
)

// This test sends synthetic pixels only and requires explicit live-test consent.
func TestLiveOCRHealthAndProcess(t *testing.T) {
	if os.Getenv("E2E_TEST") != "true" || os.Getenv("OCR_API_ADDR") == "" {
		t.Skip("requires E2E_TEST=true and OCR_API_ADDR")
	}
	c, err := ocrclient.New(os.Getenv("OCR_API_ADDR"))
	require.NoError(t, err)
	if caPath := os.Getenv("OCR_API_CA_PATH"); caPath != "" {
		transport, err := caroundtripper.New(caPath)
		require.NoError(t, err)
		require.NoError(t, c.SetHTTPTransport(transport))
	}
	ctx, cancel := context.WithTimeout(context.Background(), ocrclient.DefaultTimeout)
	defer cancel()
	healthy, err := c.HealthzContext(ctx)
	require.NoError(t, err)
	require.True(t, healthy)
	var input bytes.Buffer
	require.NoError(t, jpeg.Encode(&input, image.NewRGBA(image.Rect(0, 0, 32, 32)), nil))
	result, err := c.Process(ctx, &input)
	require.NoError(t, err)
	require.NotNil(t, result)
}
