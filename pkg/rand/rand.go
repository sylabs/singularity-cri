package rand

import (
	"crypto/rand"
	"fmt"
)

// ID returns unique random id of passed length generated with crypto/rand.
func ID(len int) string {
	buf := make([]byte, len/2+1)
	rand.Read(buf)
	return fmt.Sprintf("%x", buf)[:len]
}
