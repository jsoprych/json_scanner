package authjwt

import (
	"crypto"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// JWKS verifies RS* tokens against a rotating JSON Web Key Set (e.g. Cloudflare
// Access's certs endpoint). It selects the signing key by the token's `kid`,
// caches the set, and re-fetches (rate-limited) when it sees an unknown kid — so
// key rotation doesn't break verification. RSA-only by design (JWKS providers here
// sign with RS256); other algs are rejected.
type JWKS struct {
	url        string
	userClaim  string
	issuer     string
	audience   string
	leeway     time.Duration
	client     *http.Client
	minRefresh time.Duration

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

// NewJWKS builds a verifier that fetches keys from url.
func NewJWKS(url, userClaim, issuer, audience string) *JWKS {
	return &JWKS{
		url: url, userClaim: claimOr(userClaim), issuer: issuer, audience: audience,
		leeway:     60 * time.Second,
		client:     &http.Client{Timeout: 10 * time.Second},
		minRefresh: 60 * time.Second,
		keys:       map[string]*rsa.PublicKey{},
	}
}

// Verify checks the token against the JWKS and returns the identity claim.
func (j *JWKS) Verify(token string) (string, error) {
	p0, p1, p2, ok := split3(token)
	if !ok {
		return "", errors.New("malformed jwt")
	}
	alg, kid, err := decodeHeader(p0)
	if err != nil {
		return "", err
	}
	h, err := rsHash(alg) // RSA-only; rejects HS*/none
	if err != nil {
		return "", err
	}
	key, err := j.keyByKID(kid)
	if err != nil {
		return "", err
	}
	sig, err := b64.DecodeString(p2)
	if err != nil {
		return "", fmt.Errorf("signature b64: %w", err)
	}
	hh := h.New()
	hh.Write([]byte(p0 + "." + p1))
	if err := rsa.VerifyPKCS1v15(key, h, hh.Sum(nil), sig); err != nil {
		return "", errors.New("signature mismatch")
	}
	claims, err := decodeClaims(p1)
	if err != nil {
		return "", err
	}
	return checkClaims(claims, j.userClaim, j.issuer, j.audience, j.leeway)
}

func rsHash(alg string) (crypto.Hash, error) {
	switch alg {
	case "RS256":
		return crypto.SHA256, nil
	case "RS384":
		return crypto.SHA384, nil
	case "RS512":
		return crypto.SHA512, nil
	}
	return 0, fmt.Errorf("alg %q not allowed (JWKS is RSA-only)", alg)
}

func (j *JWKS) keyByKID(kid string) (*rsa.PublicKey, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if k, ok := j.keys[kid]; ok {
		return k, nil
	}
	// Unknown kid: refetch, but rate-limited so a bad kid can't hammer the endpoint.
	if time.Since(j.fetchedAt) < j.minRefresh {
		return nil, fmt.Errorf("unknown key id %q", kid)
	}
	if err := j.refresh(); err != nil {
		return nil, err
	}
	if k, ok := j.keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("unknown key id %q", kid)
}

type jwkKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (j *JWKS) refresh() error {
	resp, err := j.client.Get(j.url)
	if err != nil {
		return fmt.Errorf("jwks fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks status %d", resp.StatusCode)
	}
	var set struct {
		Keys []jwkKey `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return fmt.Errorf("jwks json: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey)
	for _, k := range set.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		if pub, err := rsaFromNE(k.N, k.E); err == nil {
			keys[k.Kid] = pub
		}
	}
	if len(keys) == 0 {
		return errors.New("jwks: no usable RSA keys")
	}
	j.keys = keys
	j.fetchedAt = time.Now()
	return nil
}

func rsaFromNE(nB64, eB64 string) (*rsa.PublicKey, error) {
	dec := func(s string) ([]byte, error) {
		return base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "=")) // tolerate padded keys
	}
	nb, err := dec(nB64)
	if err != nil {
		return nil, err
	}
	eb, err := dec(eB64)
	if err != nil {
		return nil, err
	}
	e := new(big.Int).SetBytes(eb)
	if !e.IsInt64() {
		return nil, errors.New("bad exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: int(e.Int64())}, nil
}
