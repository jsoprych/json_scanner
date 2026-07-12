// Package auth provides reusable password validation and authentication utilities.
// Independent of HTTP handlers — can be used by any frontend (dashboard, API, CLI).
package auth

import (
	"fmt"
	"strings"
	"unicode"
)

// PasswordPolicy defines validation rules.
type PasswordPolicy struct {
	MinLength    int
	RequireUpper bool
	RequireDigit bool
}

// DefaultPolicy returns sensible defaults.
func DefaultPolicy() PasswordPolicy {
	return PasswordPolicy{
		MinLength:    8,
		RequireUpper: true,
		RequireDigit: true,
	}
}

// commons is a small blocklist of the most common passwords.
var commons = map[string]bool{
	"password": true, "password1": true, "password123": true,
	"12345678": true, "123456789": true, "qwerty123": true,
	"admin": true, "admin123": true, "administrator": true,
	"letmein": true, "welcome": true, "monkey": true,
	"abc123": true, "football": true, "iloveyou": true,
	"trustno1": true, "dragon": true, "master": true,
	"login": true, "princess": true, "qwerty": true,
	"solo": true, "starwars": true, "sunshine": true,
	"shadow": true, "passw0rd": true, "pass1234": true,
}

// ValidatePassword checks a password against the policy. Returns nil if valid.
func ValidatePassword(pw string, policy PasswordPolicy) error {
	if len(pw) < policy.MinLength {
		return fmt.Errorf("password must be at least %d characters", policy.MinLength)
	}
	if policy.RequireUpper && !hasUpper(pw) {
		return fmt.Errorf("password must contain at least one uppercase letter")
	}
	if policy.RequireDigit && !hasDigit(pw) && !hasSpecial(pw) {
		return fmt.Errorf("password must contain at least one digit or special character")
	}
	if commons[strings.ToLower(pw)] {
		return fmt.Errorf("password is too common — choose something unique")
	}
	return nil
}

// StrengthScore returns 0-100 indicating password strength. Used for UI meters.
func StrengthScore(pw string) int {
	if len(pw) == 0 {
		return 0
	}
	score := len(pw) * 4 // length
	if hasUpper(pw) {
		score += 10
	}
	if hasLower(pw) {
		score += 10
	}
	if hasDigit(pw) {
		score += 15
	}
	if hasSpecial(pw) {
		score += 20
	}
	if len(pw) >= 12 {
		score += 10
	}
	if !commons[strings.ToLower(pw)] {
		score += 10
	}
	if score > 100 {
		score = 100
	}
	return score
}

// StrengthLabel returns a human-readable label for the score.
func StrengthLabel(score int) string {
	switch {
	case score >= 80:
		return "Very Strong"
	case score >= 60:
		return "Strong"
	case score >= 40:
		return "Fair"
	case score >= 20:
		return "Weak"
	default:
		return "Too Weak"
	}
}

// StrengthClass returns a CSS class for the score meter.
func StrengthClass(score int) string {
	switch {
	case score >= 80:
		return "strong"
	case score >= 60:
		return "good"
	case score >= 40:
		return "fair"
	case score >= 20:
		return "weak"
	default:
		return "bad"
	}
}

func hasUpper(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func hasLower(s string) bool {
	for _, r := range s {
		if unicode.IsLower(r) {
			return true
		}
	}
	return false
}

func hasDigit(s string) bool {
	for _, r := range s {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func hasSpecial(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
