package securebox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

type Box struct {
	aead cipher.AEAD
}

type envelope struct {
	Version    int    `json:"version"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

func New(encodedKey string) (*Box, error) {
	key, err := base64.RawStdEncoding.DecodeString(encodedKey)
	if err != nil {
		key, err = base64.StdEncoding.DecodeString(encodedKey)
	}
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("CPANEL_ENCRYPTION_KEY must be a base64-encoded 32-byte key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return &Box{aead: aead}, nil
}

func (b *Box) Seal(value json.RawMessage) (json.RawMessage, error) {
	if !json.Valid(value) {
		return nil, fmt.Errorf("sensitive setting is not valid JSON")
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := b.aead.Seal(nil, nonce, value, nil)
	encoded, err := json.Marshal(envelope{
		Version:    1,
		Nonce:      base64.RawStdEncoding.EncodeToString(nonce),
		Ciphertext: base64.RawStdEncoding.EncodeToString(ciphertext),
	})
	if err != nil {
		return nil, fmt.Errorf("encode sealed value: %w", err)
	}
	return encoded, nil
}

func (b *Box) Open(value json.RawMessage) (json.RawMessage, error) {
	var wrapped envelope
	if err := json.Unmarshal(value, &wrapped); err != nil || wrapped.Version != 1 {
		return nil, fmt.Errorf("invalid encrypted setting envelope")
	}
	nonce, err := base64.RawStdEncoding.DecodeString(wrapped.Nonce)
	if err != nil || len(nonce) != b.aead.NonceSize() {
		return nil, fmt.Errorf("invalid encrypted setting nonce")
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(wrapped.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("invalid encrypted setting ciphertext")
	}
	plain, err := b.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt setting: %w", err)
	}
	return plain, nil
}
