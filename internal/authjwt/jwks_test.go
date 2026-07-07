package authjwt

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func signKID(kid string, priv *rsa.PrivateKey, claims map[string]any) string {
	hb, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": kid})
	pb, _ := json.Marshal(claims)
	si := b64.EncodeToString(hb) + "." + b64.EncodeToString(pb)
	h := sha256.Sum256([]byte(si))
	sig, _ := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, h[:])
	return si + "." + b64.EncodeToString(sig)
}

func TestJWKS(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	kid := "key-1"
	n := base64.RawURLEncoding.EncodeToString(priv.PublicKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(priv.PublicKey.E)).Bytes())
	body := fmt.Sprintf(`{"keys":[{"kty":"RSA","use":"sig","alg":"RS256","kid":%q,"n":%q,"e":%q}]}`, kid, n, e)

	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.Write([]byte(body))
	}))
	defer srv.Close()

	v := NewJWKS(srv.URL, "email", "", "")

	// valid token → identity from the email claim; keys fetched once.
	tok := signKID(kid, priv, map[string]any{"email": "a@b.com", "exp": float64(time.Now().Add(time.Hour).Unix())})
	id, err := v.Verify(tok)
	if err != nil || id != "a@b.com" {
		t.Fatalf("valid: id=%q err=%v", id, err)
	}
	if hits != 1 {
		t.Errorf("expected 1 JWKS fetch, got %d", hits)
	}

	// second valid token uses the cached key — no refetch.
	if _, err := v.Verify(signKID(kid, priv, map[string]any{"email": "c@d.com", "exp": float64(time.Now().Add(time.Hour).Unix())})); err != nil {
		t.Fatalf("cached verify: %v", err)
	}
	if hits != 1 {
		t.Errorf("cache miss: fetched %d times", hits)
	}

	// token signed by a DIFFERENT key (same kid) → signature mismatch.
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	if _, err := v.Verify(signKID(kid, other, map[string]any{"email": "x@y.com", "exp": float64(time.Now().Add(time.Hour).Unix())})); err == nil {
		t.Error("expected signature mismatch for wrong key")
	}
}
