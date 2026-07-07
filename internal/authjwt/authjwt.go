// Package authjwt is a minimal, dependency-free JWT verifier for proxy auth mode:
// it checks the signature of a token issued by the edge gateway (e.g. caddy-security)
// and returns the user-identity claim. Supports HS256/384/512 (HMAC shared secret)
// and RS256/384/512 (RSA public key).
//
// Security: the verifier is bound to ONE key type and only accepts algs matching it
// — an HMAC verifier rejects RS* tokens and vice-versa, and "none" is never
// accepted. This defeats the classic alg-confusion / alg=none attacks where the
// token's own header would otherwise choose the algorithm.
package authjwt

import (
	"crypto"
	"crypto/hmac"
	"crypto/rsa"
	_ "crypto/sha256" // register SHA-256 for crypto.Hash.New
	_ "crypto/sha512" // register SHA-384/512
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"
)

type keyType int

const (
	keyHMAC keyType = iota
	keyRSA
)

// Verifier verifies JWTs against a single configured key.
type Verifier struct {
	kt        keyType
	hmacKey   []byte
	rsaPub    *rsa.PublicKey
	userClaim string
	issuer    string
	audience  string
	leeway    time.Duration
}

func claimOr(c string) string {
	if c == "" {
		return "sub"
	}
	return c
}

// NewHMAC builds an HS* verifier from a shared secret.
func NewHMAC(secret []byte, userClaim, issuer, audience string) *Verifier {
	return &Verifier{kt: keyHMAC, hmacKey: secret, userClaim: claimOr(userClaim), issuer: issuer, audience: audience, leeway: 60 * time.Second}
}

// NewRSA builds an RS* verifier from an RSA public key.
func NewRSA(pub *rsa.PublicKey, userClaim, issuer, audience string) *Verifier {
	return &Verifier{kt: keyRSA, rsaPub: pub, userClaim: claimOr(userClaim), issuer: issuer, audience: audience, leeway: 60 * time.Second}
}

// LoadRSAPublicKeyPEM reads an RSA public key from a PEM file (PKIX, PKCS1, or an
// X.509 certificate).
func LoadRSAPublicKeyPEM(path string) (*rsa.PublicKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	blk, _ := pem.Decode(b)
	if blk == nil {
		return nil, errors.New("no PEM block in key file")
	}
	if pub, err := x509.ParsePKIXPublicKey(blk.Bytes); err == nil {
		if rp, ok := pub.(*rsa.PublicKey); ok {
			return rp, nil
		}
		return nil, errors.New("PEM is not an RSA public key")
	}
	if rp, err := x509.ParsePKCS1PublicKey(blk.Bytes); err == nil {
		return rp, nil
	}
	if cert, err := x509.ParseCertificate(blk.Bytes); err == nil {
		if rp, ok := cert.PublicKey.(*rsa.PublicKey); ok {
			return rp, nil
		}
	}
	return nil, errors.New("unsupported public key PEM")
}

var b64 = base64.RawURLEncoding

// Verify checks the token's signature and standard claims and returns the user id.
func (v *Verifier) Verify(token string) (string, error) {
	p0, p1, p2, ok := split3(token)
	if !ok {
		return "", errors.New("malformed jwt")
	}
	alg, _, err := decodeHeader(p0)
	if err != nil {
		return "", err
	}
	h, err := v.hashForAlg(alg) // rejects mismatched / "none"
	if err != nil {
		return "", err
	}
	sig, err := b64.DecodeString(p2)
	if err != nil {
		return "", fmt.Errorf("signature b64: %w", err)
	}
	if err := v.verifySig(h, []byte(p0+"."+p1), sig); err != nil {
		return "", err
	}
	claims, err := decodeClaims(p1)
	if err != nil {
		return "", err
	}
	return checkClaims(claims, v.userClaim, v.issuer, v.audience, v.leeway)
}

// decodeHeader returns the alg and kid from a JWT header segment.
func decodeHeader(part string) (alg, kid string, err error) {
	hb, err := b64.DecodeString(part)
	if err != nil {
		return "", "", fmt.Errorf("header b64: %w", err)
	}
	var h struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(hb, &h); err != nil {
		return "", "", fmt.Errorf("header json: %w", err)
	}
	return h.Alg, h.Kid, nil
}

// decodeClaims decodes a JWT payload segment.
func decodeClaims(part string) (map[string]any, error) {
	pb, err := b64.DecodeString(part)
	if err != nil {
		return nil, fmt.Errorf("payload b64: %w", err)
	}
	var c map[string]any
	if err := json.Unmarshal(pb, &c); err != nil {
		return nil, fmt.Errorf("claims json: %w", err)
	}
	return c, nil
}

// checkClaims validates exp/nbf/iss/aud and returns the identity claim. Shared by
// all verifiers.
func checkClaims(claims map[string]any, userClaim, issuer, audience string, leeway time.Duration) (string, error) {
	now := time.Now()
	exp, ok := numClaim(claims, "exp")
	if !ok {
		return "", errors.New("missing exp claim")
	}
	if now.After(time.Unix(exp, 0).Add(leeway)) {
		return "", errors.New("token expired")
	}
	if nbf, ok := numClaim(claims, "nbf"); ok && now.Add(leeway).Before(time.Unix(nbf, 0)) {
		return "", errors.New("token not yet valid")
	}
	if issuer != "" {
		if iss, _ := claims["iss"].(string); iss != issuer {
			return "", errors.New("issuer mismatch")
		}
	}
	if audience != "" && !audMatch(claims["aud"], audience) {
		return "", errors.New("audience mismatch")
	}
	id, _ := claims[userClaim].(string)
	if id == "" {
		return "", fmt.Errorf("identity claim %q missing or empty", userClaim)
	}
	return id, nil
}

func split3(s string) (a, b, c string, ok bool) {
	i := indexByte(s, '.')
	if i < 0 {
		return
	}
	j := indexByte(s[i+1:], '.')
	if j < 0 {
		return
	}
	j += i + 1
	if indexByte(s[j+1:], '.') >= 0 {
		return
	}
	return s[:i], s[i+1 : j], s[j+1:], true
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// hashForAlg maps a JWT alg to a hash, enforcing that it matches this verifier's key
// type — the alg-confusion guard.
func (v *Verifier) hashForAlg(alg string) (crypto.Hash, error) {
	switch v.kt {
	case keyHMAC:
		switch alg {
		case "HS256":
			return crypto.SHA256, nil
		case "HS384":
			return crypto.SHA384, nil
		case "HS512":
			return crypto.SHA512, nil
		}
		return 0, fmt.Errorf("alg %q not allowed for an HMAC verifier", alg)
	case keyRSA:
		switch alg {
		case "RS256":
			return crypto.SHA256, nil
		case "RS384":
			return crypto.SHA384, nil
		case "RS512":
			return crypto.SHA512, nil
		}
		return 0, fmt.Errorf("alg %q not allowed for an RSA verifier", alg)
	}
	return 0, errors.New("no verification key configured")
}

func (v *Verifier) verifySig(h crypto.Hash, signingInput, sig []byte) error {
	switch v.kt {
	case keyHMAC:
		mac := hmac.New(h.New, v.hmacKey)
		mac.Write(signingInput)
		if !hmac.Equal(mac.Sum(nil), sig) {
			return errors.New("signature mismatch")
		}
		return nil
	case keyRSA:
		hh := h.New()
		hh.Write(signingInput)
		if err := rsa.VerifyPKCS1v15(v.rsaPub, h, hh.Sum(nil), sig); err != nil {
			return errors.New("signature mismatch")
		}
		return nil
	}
	return errors.New("no verification key configured")
}

func audMatch(aud any, want string) bool {
	switch a := aud.(type) {
	case string:
		return a == want
	case []any:
		for _, x := range a {
			if s, ok := x.(string); ok && s == want {
				return true
			}
		}
	}
	return false
}

func numClaim(c map[string]any, k string) (int64, bool) {
	switch v := c[k].(type) {
	case float64:
		return int64(v), true
	case json.Number:
		n, err := v.Int64()
		return n, err == nil
	}
	return 0, false
}
