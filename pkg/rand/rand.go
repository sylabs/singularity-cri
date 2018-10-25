package rand

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateID returns unique random id of passed length generated with crypto/rand.
func GenerateID(len int) string {
	buf := make([]byte, len/2+1)
	rand.Read(buf)
	return hex.EncodeToString(buf[:len])
}
