package authjwt

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"
)

func sign(alg string, key any, claims map[string]any) string {
	hb, _ := json.Marshal(map[string]string{"alg": alg, "typ": "JWT"})
	pb, _ := json.Marshal(claims)
	si := b64.EncodeToString(hb) + "." + b64.EncodeToString(pb)
	var sig []byte
	switch alg {
	case "HS256":
		m := hmac.New(sha256.New, key.([]byte))
		m.Write([]byte(si))
		sig = m.Sum(nil)
	case "RS256":
		h := sha256.Sum256([]byte(si))
		sig, _ = rsa.SignPKCS1v15(rand.Reader, key.(*rsa.PrivateKey), crypto.SHA256, h[:])
	}
	return si + "." + b64.EncodeToString(sig)
}

func baseClaims() map[string]any {
	return map[string]any{
		"sub": "alice", "iss": "caddy", "aud": "scanner",
		"exp": float64(time.Now().Add(time.Hour).Unix()),
	}
}

func TestHMACVerify(t *testing.T) {
	secret := []byte("s3cret")
	v := NewHMAC(secret, "sub", "caddy", "scanner")

	id, err := v.Verify(sign("HS256", secret, baseClaims()))
	if err != nil || id != "alice" {
		t.Fatalf("valid HS256: id=%q err=%v", id, err)
	}

	// wrong secret → reject
	if _, err := v.Verify(sign("HS256", []byte("wrong"), baseClaims())); err == nil {
		t.Error("expected signature mismatch for wrong secret")
	}
	// expired → reject
	exp := baseClaims()
	exp["exp"] = float64(time.Now().Add(-time.Hour).Unix())
	if _, err := v.Verify(sign("HS256", secret, exp)); err == nil {
		t.Error("expected expired rejection")
	}
	// issuer mismatch → reject
	bad := baseClaims()
	bad["iss"] = "evil"
	if _, err := v.Verify(sign("HS256", secret, bad)); err == nil {
		t.Error("expected issuer mismatch")
	}
	// missing identity claim → reject
	noSub := baseClaims()
	delete(noSub, "sub")
	if _, err := v.Verify(sign("HS256", secret, noSub)); err == nil {
		t.Error("expected missing-claim rejection")
	}
}

func TestRSAVerifyAndAlgConfusion(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rv := NewRSA(&priv.PublicKey, "sub", "", "")
	id, err := rv.Verify(sign("RS256", priv, baseClaims()))
	if err != nil || id != "alice" {
		t.Fatalf("valid RS256: id=%q err=%v", id, err)
	}

	// ALG-CONFUSION: an attacker forges an HS256 token using the RSA *public* key
	// bytes as the HMAC secret. A naive verifier that trusts the token's alg would
	// accept it. Our RSA verifier must reject any HS* alg outright.
	pubBytes := priv.PublicKey.N.Bytes()
	forged := sign("HS256", pubBytes, baseClaims())
	if _, err := rv.Verify(forged); err == nil {
		t.Error("alg-confusion: RSA verifier must reject an HS256 token")
	}

	// An HMAC verifier must likewise reject an RS256 token.
	hv := NewHMAC([]byte("k"), "sub", "", "")
	if _, err := hv.Verify(sign("RS256", priv, baseClaims())); err == nil {
		t.Error("HMAC verifier must reject an RS256 token")
	}
}

func TestMalformed(t *testing.T) {
	v := NewHMAC([]byte("k"), "sub", "", "")
	for _, tok := range []string{"", "abc", "a.b", "a.b.c.d"} {
		if _, err := v.Verify(tok); err == nil {
			t.Errorf("expected error for %q", tok)
		}
	}
}
