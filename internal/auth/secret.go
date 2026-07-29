package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

func NewSecret(prefix string, bytes int) (plain string, hash []byte, err error) {
	if bytes < 32 {
		return "", nil, fmt.Errorf("secret entropy must be at least 32 bytes")
	}
	random := make([]byte, bytes)
	if _, err := rand.Read(random); err != nil {
		return "", nil, fmt.Errorf("generate secret: %w", err)
	}
	plain = prefix + base64.RawURLEncoding.EncodeToString(random)
	digest := sha256.Sum256([]byte(plain))
	return plain, digest[:], nil
}

func HashSecret(secret string) []byte {
	digest := sha256.Sum256([]byte(secret))
	return digest[:]
}

func Prefix(secret string, length int) string {
	if len(secret) <= length {
		return secret
	}
	return secret[:length]
}
