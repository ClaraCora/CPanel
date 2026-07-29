package httpapi

import (
	"bytes"
	"crypto/ecdh"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestNewRealityCredentialsProducesMatchingX25519Pair(t *testing.T) {
	credentials, err := newRealityCredentials()
	if err != nil {
		t.Fatal(err)
	}
	privateBytes, err := base64.RawURLEncoding.DecodeString(credentials.PrivateKey)
	if err != nil || len(privateBytes) != 32 {
		t.Fatalf("invalid private key: length=%d err=%v", len(privateBytes), err)
	}
	publicBytes, err := base64.RawURLEncoding.DecodeString(credentials.PublicKey)
	if err != nil || len(publicBytes) != 32 {
		t.Fatalf("invalid public key: length=%d err=%v", len(publicBytes), err)
	}
	privateKey, err := ecdh.X25519().NewPrivateKey(privateBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(privateKey.PublicKey().Bytes(), publicBytes) {
		t.Fatal("public key does not match private key")
	}
	shortID, err := hex.DecodeString(credentials.ShortID)
	if err != nil || len(shortID) != 8 {
		t.Fatalf("invalid short id: length=%d err=%v", len(shortID), err)
	}
}
