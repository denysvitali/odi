package ocrclient

import (
	"bytes"
	"context"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/denysvitali/odi/pkg/ocrclient/caroundtripper"
	"github.com/stretchr/testify/require"
)

func TestGuardedDialUsesValidatedNumericAddress(t *testing.T) {
	var calls int
	resolve := func(ctx context.Context, host string) ([]net.IPAddr, error) {
		require.Equal(t, "ocr.example", host)
		return []net.IPAddr{{IP: net.ParseIP("203.0.113.8")}}, nil
	}
	expected := errors.New("synthetic dial result")
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		calls++
		require.Equal(t, "203.0.113.8:443", address)
		return nil, expected
	}
	_, err := guardedDialContext(false, resolve, dial)(context.Background(), "tcp", "ocr.example:443")
	require.ErrorIs(t, err, expected)
	require.Equal(t, 1, calls)
}

func TestGuardedDialRejectsUnapprovedAddresses(t *testing.T) {
	for _, tt := range []struct {
		name     string
		ips      []net.IPAddr
		allow    bool
		wantDial bool
	}{
		{"loopback", []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, false, false},
		{"mixed DNS", []net.IPAddr{{IP: net.ParseIP("203.0.113.8")}, {IP: net.ParseIP("192.168.1.2")}}, false, false},
		{"explicit LAN opt in", []net.IPAddr{{IP: net.ParseIP("192.168.1.2")}}, true, true},
		{"empty DNS", nil, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			resolve := func(context.Context, string) ([]net.IPAddr, error) { return tt.ips, nil }
			dial := func(context.Context, string, string) (net.Conn, error) {
				called = true
				return nil, errors.New("synthetic")
			}
			_, err := guardedDialContext(tt.allow, resolve, dial)(context.Background(), "tcp", "ocr.example:443")
			require.Error(t, err)
			require.Equal(t, tt.wantDial, called)
		})
	}
}

func TestOCRNeverFollowsRedirects(t *testing.T) {
	t.Setenv(EnvAllowPrivateTargets, "true")
	t.Setenv("ODI_"+EnvAllowPrivateTargets, "true")
	t.Setenv("ODI_"+EnvAllowedHosts, "")
	var received atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received.Add(1) }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	c, err := New(source.URL, WithMaxRetries(0))
	require.NoError(t, err)
	_, err = c.Process(context.Background(), bytes.NewReader([]byte("synthetic-image")))
	require.Error(t, err)
	require.Zero(t, received.Load())
}

func TestOCRCustomCAPreservesDestinationPolicy(t *testing.T) {
	t.Setenv(EnvAllowPrivateTargets, "true")
	t.Setenv("ODI_"+EnvAllowPrivateTargets, "true")
	t.Setenv("ODI_"+EnvAllowedHosts, "")
	var received atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received.Add(1); _, _ = io.WriteString(w, `{}`) }))
	defer srv.Close()
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}), 0600))
	ca, err := caroundtripper.New(caPath)
	require.NoError(t, err)
	c, err := New(srv.URL)
	require.NoError(t, err)
	require.NoError(t, c.SetHTTPTransport(ca))
	healthy, err := c.HealthzContext(context.Background())
	require.NoError(t, err)
	require.True(t, healthy)
	require.EqualValues(t, 1, received.Load())
	// Reconfigure a guarded client with the same TLS roots. Private dialing must
	// remain denied even though the original custom CA transport trusts the peer.
	c.allowPrivateTargets = false
	require.NoError(t, c.SetHTTPTransport(ca))
	_, err = c.HealthzContext(context.Background())
	require.Error(t, err)
	require.EqualValues(t, 1, received.Load())
}

func TestOCRHealthCancellation(t *testing.T) {
	t.Setenv(EnvAllowPrivateTargets, "true")
	t.Setenv("ODI_"+EnvAllowPrivateTargets, "true")
	t.Setenv("ODI_"+EnvAllowedHosts, "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("canceled request reached server") }))
	defer srv.Close()
	c, err := New(srv.URL)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = c.HealthzContext(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func TestOCRPrivateHostAllowlistAndEnvPrecedence(t *testing.T) {
	t.Setenv(EnvAllowPrivateTargets, "true")
	t.Setenv("ODI_"+EnvAllowPrivateTargets, "true")
	t.Setenv("ODI_"+EnvAllowedHosts, "")
	t.Setenv("ODI_"+EnvAllowedHosts, "127.0.0.1, ocr.internal")
	_, err := New("http://127.0.0.1:8080")
	require.NoError(t, err)
	_, err = New("http://192.168.1.2:8080")
	require.EqualError(t, err, "OCR target is outside the configured host allowlist")
	t.Setenv("ODI_"+EnvAllowPrivateTargets, "false")
	_, err = New("http://127.0.0.1:8080")
	require.Error(t, err)
}
