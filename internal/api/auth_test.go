package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cetus-marketdata-scanner/internal/authjwt"
	"cetus-marketdata-scanner/internal/snapshot"
	"cetus-marketdata-scanner/internal/telemetry"
	"cetus-marketdata-scanner/internal/user"
)

func TestLoginEndpoint(t *testing.T) {
	snap, err := snapshot.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()

	// Create a user store with a test user
	tmpFile := t.TempDir() + "/users.jsonl"
	userStore, err := user.OpenStore(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	testUser := user.User{
		ID:   "testuser",
		Name: "Test User",
		Tier: user.TierFree,
		Role: user.RoleUser,
	}
	testUser.SetPassword("testpass123")
	if err := userStore.Create(testUser); err != nil {
		t.Fatal(err)
	}

	// Create a signer and verifier
	secret := []byte("test-secret-key-for-jwt-signing")
	signer := authjwt.NewHMACSigner(secret, "test-issuer", 24*time.Hour)
	verifier := authjwt.NewHMAC(secret, "sub", "test-issuer", "")

	log := telemetry.New(io.Discard)
	h := NewHandlerFull(snap, nil, nil, userStore, nil, nil, nil, nil, nil, nil, nil, nil, signer, verifier, nil, log)

	t.Run("successful login", func(t *testing.T) {
		body := bytes.NewBufferString(`{"user": "testuser", "password": "testpass123"}`)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.Login(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp LoginResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}

		if resp.Token == "" {
			t.Error("expected token, got empty")
		}
		if resp.User != "testuser" {
			t.Errorf("expected user 'testuser', got %q", resp.User)
		}
		if resp.Tier != "free" {
			t.Errorf("expected tier 'free', got %q", resp.Tier)
		}
		if resp.Role != "user" {
			t.Errorf("expected role 'user', got %q", resp.Role)
		}
	})

	t.Run("invalid password", func(t *testing.T) {
		body := bytes.NewBufferString(`{"user": "testuser", "password": "wrongpass"}`)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.Login(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})

	t.Run("non-existent user", func(t *testing.T) {
		body := bytes.NewBufferString(`{"user": "nonexistent", "password": "testpass123"}`)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.Login(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})
}

func TestMeEndpoint(t *testing.T) {
	snap, err := snapshot.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()

	// Create a user store with a test user
	tmpFile := t.TempDir() + "/users.jsonl"
	userStore, err := user.OpenStore(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	testUser := user.User{
		ID:   "testuser",
		Name: "Test User",
		Tier: user.TierPro,
		Role: user.RoleAdmin,
	}
	testUser.SetPassword("testpass123")
	if err := userStore.Create(testUser); err != nil {
		t.Fatal(err)
	}

	// Create a signer and verifier
	secret := []byte("test-secret-key-for-jwt-signing")
	signer := authjwt.NewHMACSigner(secret, "test-issuer", 24*time.Hour)
	verifier := authjwt.NewHMAC(secret, "sub", "test-issuer", "")

	log := telemetry.New(io.Discard)
	h := NewHandlerFull(snap, nil, nil, userStore, nil, nil, nil, nil, nil, nil, nil, nil, signer, verifier, nil, log)

	t.Run("authenticated request", func(t *testing.T) {
		// First, login to get a token
		loginBody := bytes.NewBufferString(`{"user": "testuser", "password": "testpass123"}`)
		loginReq := httptest.NewRequest("POST", "/api/v1/auth/login", loginBody)
		loginReq.Header.Set("Content-Type", "application/json")
		loginW := httptest.NewRecorder()
		h.Login(loginW, loginReq)

		var loginResp LoginResponse
		if err := json.Unmarshal(loginW.Body.Bytes(), &loginResp); err != nil {
			t.Fatal(err)
		}

		// Now call /me with the token
		req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+loginResp.Token)
		w := httptest.NewRecorder()

		h.Me(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var u user.User
		if err := json.Unmarshal(w.Body.Bytes(), &u); err != nil {
			t.Fatal(err)
		}

		if u.ID != "testuser" {
			t.Errorf("expected user ID 'testuser', got %q", u.ID)
		}
		if u.Tier != user.TierPro {
			t.Errorf("expected tier 'pro', got %q", u.Tier)
		}
		if u.Role != user.RoleAdmin {
			t.Errorf("expected role 'admin', got %q", u.Role)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		w := httptest.NewRecorder()

		h.Me(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		w := httptest.NewRecorder()

		h.Me(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})
}
