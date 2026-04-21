package contentdigest

import (
	"crypto/sha256"
	"encoding/hex"
)

const Prefix = "sha256:"

func Sum(data []byte) string {
	sum := sha256.Sum256(data)
	return Prefix + hex.EncodeToString(sum[:])
}
