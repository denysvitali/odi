package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// SharePayload is the signed, self-describing portion of a share token. It is
// base64url-encoded and accompanied by an HMAC-SHA256 signature so it cannot
// be forged or tampered with without the server secret.
type SharePayload struct {
	ScanID     string `json:"scanID"`
	SequenceID int    `json:"sequenceID"`
	ExpiresAt  int64  `json:"expiresAt"`
	MaxViews   int    `json:"maxViews"`
	Nonce      string `json:"nonce"`
}

// shareNonceBytes is the number of random bytes used for the per-token nonce.
const shareNonceBytes = 16

// signShare serialises p, generates a fresh random nonce, and returns a token
// of the form base64url(payload) + "." + base64url(HMAC-SHA256(secret,
// payloadBytes)). The nonce is generated per call with crypto/rand so two
// tokens for the same scan/sequence are never identical.
func signShare(secret []byte, p SharePayload) (string, error) {
	nonce := make([]byte, shareNonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate share nonce: %w", err)
	}
	p.Nonce = base64.RawURLEncoding.EncodeToString(nonce)

	payloadBytes, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("marshal share payload: %w", err)
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write(payloadBytes)
	sig := mac.Sum(nil)

	token := base64.RawURLEncoding.EncodeToString(payloadBytes) + "." +
		base64.RawURLEncoding.EncodeToString(sig)
	return token, nil
}

// verifyShare splits token into its payload and signature parts, recomputes
// the HMAC over the decoded payload bytes, and compares it in constant time
// (hmac.Equal). On success it returns the decoded SharePayload. It does NOT
// check expiry or view limits — callers must enforce those against the stored
// record.
func verifyShare(secret []byte, token string) (SharePayload, error) {
	var p SharePayload

	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return p, fmt.Errorf("malformed share token")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return p, fmt.Errorf("decode share payload: %w", err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return p, fmt.Errorf("decode share signature: %w", err)
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write(payloadBytes)
	expected := mac.Sum(nil)

	if !hmac.Equal(sig, expected) {
		return p, fmt.Errorf("invalid share signature")
	}

	if err := json.Unmarshal(payloadBytes, &p); err != nil {
		return p, fmt.Errorf("unmarshal share payload: %w", err)
	}
	return p, nil
}
