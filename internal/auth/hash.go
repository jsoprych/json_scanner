// Package auth provides reusable password hashing, session management, and
// account lockout. Zero dependencies on cetus-marketdata-scanner internals.
// Drop this package into any Go project that needs login/auth support.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

var b64 = base64.RawStdEncoding
const pbkdf2Iter = 600000 // OWASP recommended

// HashPassword returns a PBKDF2-HMAC-SHA256 hash of pw with a random salt.
// Format: pbkdf2_sha256$iter$salt$dk
func HashPassword(pw string) string {
	salt := make([]byte, 16)
	rand.Read(salt)
	dk, _ := pbkdf2.Key(sha256.New, pw, salt, pbkdf2Iter, 32)
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", pbkdf2Iter, b64.EncodeToString(salt), b64.EncodeToString(dk))
}

// CheckPassword reports whether pw matches the stored hash (constant time).
// Supports PBKDF2 and legacy SHA256 formats.
func CheckPassword(hash, pw string) bool {
	if hash == "" {
		return false
	}
	if strings.HasPrefix(hash, "pbkdf2_sha256$") {
		return checkPBKDF2(hash, pw)
	}
	// Legacy SHA256
	want, err := hex.DecodeString(hash)
	if err != nil {
		return false
	}
	sum := sha256.Sum256([]byte(pw))
	return subtle.ConstantTimeCompare(sum[:], want) == 1
}

func checkPBKDF2(stored, pw string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1 {
		return false
	}
	salt, err := b64.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := b64.DecodeString(parts[3])
	if err != nil {
		return false
	}
	dk, err := pbkdf2.Key(sha256.New, pw, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(dk, want) == 1
}
