package domain

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
)

var idEncoding = base32.NewEncoding("0123456789abcdefghjkmnpqrstvwxyz").WithPadding(base32.NoPadding)

func NewID(prefix string) (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return prefix + "_" + strings.ToLower(idEncoding.EncodeToString(random)), nil
}

func MustID(prefix string) string {
	id, err := NewID(prefix)
	if err != nil {
		panic(err)
	}
	return id
}

func NewUUID() (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate uuid: %w", err)
	}
	random[6] = (random[6] & 0x0f) | 0x40
	random[8] = (random[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		random[0:4], random[4:6], random[6:8], random[8:10], random[10:16]), nil
}
