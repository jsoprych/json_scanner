package authjwt

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"time"
)

// Signer issues JWTs. Supports HS256/384/512 (HMAC) and RS256/384/512 (RSA).
type Signer struct {
	kt      keyType
	hmacKey []byte
	rsaKey  *rsa.PrivateKey
	alg     string
	issuer  string
	ttl     time.Duration
}

// NewHMACSigner builds an HS* signer from a shared secret.
func NewHMACSigner(secret []byte, issuer string, ttl time.Duration) *Signer {
	return &Signer{kt: keyHMAC, hmacKey: secret, alg: "HS256", issuer: issuer, ttl: ttl}
}

// NewRSASigner builds an RS* signer from an RSA private key.
func NewRSASigner(key *rsa.PrivateKey, issuer string, ttl time.Duration) *Signer {
	return &Signer{kt: keyRSA, rsaKey: key, alg: "RS256", issuer: issuer, ttl: ttl}
}

// TTL returns the token lifetime.
func (s *Signer) TTL() time.Duration { return s.ttl }

// LoadRSAPrivateKeyPEM reads an RSA private key from a PEM file (PKCS1 or PKCS8).
func LoadRSAPrivateKeyPEM(path string) (*rsa.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	blk, _ := pem.Decode(b)
	if blk == nil {
		return nil, errors.New("no PEM block in key file")
	}
	if key, err := x509.ParsePKCS1PrivateKey(blk.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(blk.Bytes); err == nil {
		if rp, ok := key.(*rsa.PrivateKey); ok {
			return rp, nil
		}
		return nil, errors.New("PEM is not an RSA private key")
	}
	return nil, errors.New("unsupported private key PEM")
}

// Claims holds the JWT payload.
type Claims struct {
	Sub      string `json:"sub"`               // subject (user id)
	Iss      string `json:"iss,omitempty"`     // issuer
	Aud      string `json:"aud,omitempty"`     // audience
	Exp      int64  `json:"exp"`               // expiration (Unix seconds)
	Iat      int64  `json:"iat"`               // issued at (Unix seconds)
	Nbf      int64  `json:"nbf,omitempty"`     // not before (Unix seconds)
	Tier     string `json:"tier,omitempty"`    // user tier (free/pro)
	Role     string `json:"role,omitempty"`    // user role (user/admin)
}

// Sign creates a JWT from the claims.
func (s *Signer) Sign(claims Claims) (string, error) {
	if claims.Sub == "" {
		return "", errors.New("sub claim required")
	}
	if claims.Iss == "" {
		claims.Iss = s.issuer
	}
	now := time.Now()
	if claims.Iat == 0 {
		claims.Iat = now.Unix()
	}
	if claims.Exp == 0 {
		claims.Exp = now.Add(s.ttl).Unix()
	}

	header := map[string]string{"alg": s.alg, "typ": "JWT"}
	hb, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	cb, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	h := b64.EncodeToString(hb)
	c := b64.EncodeToString(cb)
	signingInput := h + "." + c

	sig, err := s.sign([]byte(signingInput))
	if err != nil {
		return "", err
	}

	return signingInput + "." + b64.EncodeToString(sig), nil
}

func (s *Signer) sign(data []byte) ([]byte, error) {
	switch s.kt {
	case keyHMAC:
		mac := hmac.New(sha256.New, s.hmacKey)
		mac.Write(data)
		return mac.Sum(nil), nil
	case keyRSA:
		h := sha256.New()
		h.Write(data)
		return rsa.SignPKCS1v15(rand.Reader, s.rsaKey, crypto.SHA256, h.Sum(nil))
	}
	return nil, errors.New("no signing key configured")
}
