package server

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignVerifyShareRoundTrip(t *testing.T) {
	secret := []byte("super-secret-api-token")
	p := SharePayload{
		ScanID:     "abc-123",
		SequenceID: 4,
		ExpiresAt:  1893456000,
		MaxViews:   5,
	}

	token, err := signShare(secret, p)
	require.NoError(t, err)
	require.Contains(t, token, ".")

	got, err := verifyShare(secret, token)
	require.NoError(t, err)
	assert.Equal(t, p.ScanID, got.ScanID)
	assert.Equal(t, p.SequenceID, got.SequenceID)
	assert.Equal(t, p.ExpiresAt, got.ExpiresAt)
	assert.Equal(t, p.MaxViews, got.MaxViews)
	// A fresh nonce must have been injected during signing.
	assert.NotEmpty(t, got.Nonce, "signShare must populate a nonce")
}

func TestSignShareNonceIsUnique(t *testing.T) {
	secret := []byte("secret")
	p := SharePayload{ScanID: "s", SequenceID: 1}

	t1, err := signShare(secret, p)
	require.NoError(t, err)
	t2, err := signShare(secret, p)
	require.NoError(t, err)

	assert.NotEqual(t, t1, t2, "two tokens for the same payload must differ (per-call nonce)")
}

func TestVerifyShareRejectsWrongSecret(t *testing.T) {
	token, err := signShare([]byte("real-secret"), SharePayload{ScanID: "s", SequenceID: 1})
	require.NoError(t, err)

	_, err = verifyShare([]byte("attacker-secret"), token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid share signature")
}

func TestVerifyShareRejectsTamperedPayload(t *testing.T) {
	secret := []byte("secret")
	token, err := signShare(secret, SharePayload{ScanID: "s", SequenceID: 1, MaxViews: 1})
	require.NoError(t, err)

	parts := strings.SplitN(token, ".", 2)
	require.Len(t, parts, 2)

	// Forge a payload that grants unlimited views, keeping the original signature.
	forged := base64.RawURLEncoding.EncodeToString([]byte(`{"scanID":"s","sequenceID":1,"maxViews":9999}`))
	tampered := forged + "." + parts[1]

	_, err = verifyShare(secret, tampered)
	require.Error(t, err, "tampering with the payload must fail HMAC verification")
}

func TestVerifyShareRejectsMalformedTokens(t *testing.T) {
	secret := []byte("secret")
	cases := []string{
		"",
		"no-dot-separator",
		".",
		"only-left.",
		".only-right",
		"!!!notbase64!!!.also-bad",
	}
	for _, tok := range cases {
		_, err := verifyShare(secret, tok)
		assert.Error(t, err, "token %q must be rejected", tok)
	}
}
