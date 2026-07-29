package httpapi

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
)

type realityCredentials struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
	ShortID    string `json:"short_id"`
}

func newRealityCredentials() (realityCredentials, error) {
	privateKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return realityCredentials{}, fmt.Errorf("generate x25519 key: %w", err)
	}
	shortID := make([]byte, 8)
	if _, err := rand.Read(shortID); err != nil {
		return realityCredentials{}, fmt.Errorf("generate reality short id: %w", err)
	}
	return realityCredentials{
		PrivateKey: base64.RawURLEncoding.EncodeToString(privateKey.Bytes()),
		PublicKey:  base64.RawURLEncoding.EncodeToString(privateKey.PublicKey().Bytes()),
		ShortID:    hex.EncodeToString(shortID),
	}, nil
}

func (s *Server) handleGenerateRealityCredentials(w http.ResponseWriter, r *http.Request) {
	credentials, err := newRealityCredentials()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "CREDENTIAL_GENERATION_FAILED", "Reality 密钥生成失败，请重试", nil)
		return
	}
	writeData(w, r, http.StatusCreated, credentials)
}
