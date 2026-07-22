package qrtoken

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

const qrTokenBytes = 32 

func GenerateQRToken() (string, error) {
	buf := make([]byte, qrTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("crypto rand: %w", err)
	}
	return hex.EncodeToString(buf), nil
}