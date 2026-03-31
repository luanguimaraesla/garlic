package crypto

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// HashSHA256 calculates the SHA-256 hash of the given string and returns it as a hex string.
func HashSHA256(input string) string {
	hasher := sha256.New()
	hasher.Write([]byte(input))
	return hex.EncodeToString(hasher.Sum(nil))
}

func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
