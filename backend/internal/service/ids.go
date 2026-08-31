package service

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func newUUID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate uuid: %w", err)
	}

	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}

func newShortCode() (string, error) {
	value := make([]byte, 9)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate short code: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
