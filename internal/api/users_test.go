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

func setupUserTest(t *testing.T) (*Handler, *user.Store, string) {
	snap, err := snapshot.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { snap.Close() })

	tmpFile := t.TempDir() + "/users.jsonl"
	userStore, err := user.OpenStore(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	// Create admin user
	admin := user.User{
		ID:   "admin",
		Name: "Admin User",
		Tier: user.TierPro,
		Role: user.RoleAdmin,
	}
	admin.SetPassword("adminpass")
	if err := userStore.Create(admin); err != nil {
		t.Fatal(err)
	}

	// Create regular user
	regular := user.User{
		ID:   "regular",
		Name: "Regular User",
		Tier: user.TierFree,
		Role: user.RoleUser,
	}
	regular.SetPassword("regularpass")
	if err := userStore.Create(regular); err != nil {
		t.Fatal(err)
	}

	secret := []byte("test-secret-key")
	signer := authjwt.NewHMACSigner(secret, "test", 24*time.Hour)
	verifier := authjwt.NewHMAC(secret, "sub", "test", "")

	log := telemetry.New(io.Discard)
	h := NewHandlerFull(snap, nil, nil, userStore, nil, nil, nil, nil, nil, nil, nil, nil, signer, verifier, nil, log)

	// Login as admin to get token
	loginBody := bytes.NewBufferString(`{"user": "admin", "password": "adminpass"}`)
	loginReq := httptest.NewRequest("POST", "/api/v1/auth/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	h.Login(loginW, loginReq)

	var loginResp LoginResponse
	if err := json.Unmarshal(loginW.Body.Bytes(), &loginResp); err != nil {
		t.Fatal(err)
	}

	return h, userStore, loginResp.Token
}

func TestListUsers(t *testing.T) {
	h, _, adminToken := setupUserTest(t)

	t.Run("admin can list users", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/users", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w := httptest.NewRecorder()

		h.ListUsers(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var users []user.User
		if err := json.Unmarshal(w.Body.Bytes(), &users); err != nil {
			t.Fatal(err)
		}

		if len(users) != 2 {
			t.Errorf("expected 2 users, got %d", len(users))
		}
	})

	t.Run("unauthenticated request fails", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/users", nil)
		w := httptest.NewRecorder()

		h.ListUsers(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})
}

func TestCreateUser(t *testing.T) {
	h, userStore, adminToken := setupUserTest(t)

	t.Run("admin can create user", func(t *testing.T) {
		body := bytes.NewBufferString(`{
			"id": "newuser",
			"name": "New User",
			"password": "newpass123",
			"tier": "free",
			"role": "user"
		}`)
		req := httptest.NewRequest("POST", "/api/v1/users", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.CreateUser(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d", w.Code)
		}

		var u user.User
		if err := json.Unmarshal(w.Body.Bytes(), &u); err != nil {
			t.Fatal(err)
		}

		if u.ID != "newuser" {
			t.Errorf("expected ID 'newuser', got %q", u.ID)
		}
		if u.Name != "New User" {
			t.Errorf("expected name 'New User', got %q", u.Name)
		}

		// Verify user was created
		created, exists := userStore.Find("newuser")
		if !exists {
			t.Error("user was not created in store")
		}
		if !created.CheckPassword("newpass123") {
			t.Error("password was not set correctly")
		}
	})

	t.Run("missing fields fails", func(t *testing.T) {
		body := bytes.NewBufferString(`{"id": "test"}`)
		req := httptest.NewRequest("POST", "/api/v1/users", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.CreateUser(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestUpdateUser(t *testing.T) {
	h, userStore, adminToken := setupUserTest(t)

	t.Run("admin can update user", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name": "Updated Name", "tier": "pro"}`)
		req := httptest.NewRequest("PUT", "/api/v1/users/regular", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "regular")
		w := httptest.NewRecorder()

		h.UpdateUser(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var u user.User
		if err := json.Unmarshal(w.Body.Bytes(), &u); err != nil {
			t.Fatal(err)
		}

		if u.Name != "Updated Name" {
			t.Errorf("expected name 'Updated Name', got %q", u.Name)
		}
		if u.Tier != user.TierPro {
			t.Errorf("expected tier 'pro', got %q", u.Tier)
		}

		// Verify in store
		updated, _ := userStore.Find("regular")
		if updated.Name != "Updated Name" {
			t.Error("name not updated in store")
		}
		if updated.Tier != user.TierPro {
			t.Error("tier not updated in store")
		}
	})

	t.Run("regular user can update own name", func(t *testing.T) {
		// Login as regular user
		loginBody := bytes.NewBufferString(`{"user": "regular", "password": "regularpass"}`)
		loginReq := httptest.NewRequest("POST", "/api/v1/auth/login", loginBody)
		loginReq.Header.Set("Content-Type", "application/json")
		loginW := httptest.NewRecorder()
		h.Login(loginW, loginReq)

		var loginResp LoginResponse
		if err := json.Unmarshal(loginW.Body.Bytes(), &loginResp); err != nil {
			t.Fatal(err)
		}

		body := bytes.NewBufferString(`{"name": "My New Name"}`)
		req := httptest.NewRequest("PUT", "/api/v1/users/regular", body)
		req.Header.Set("Authorization", "Bearer "+loginResp.Token)
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "regular")
		w := httptest.NewRecorder()

		h.UpdateUser(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("regular user cannot change tier", func(t *testing.T) {
		// Login as regular user
		loginBody := bytes.NewBufferString(`{"user": "regular", "password": "regularpass"}`)
		loginReq := httptest.NewRequest("POST", "/api/v1/auth/login", loginBody)
		loginReq.Header.Set("Content-Type", "application/json")
		loginW := httptest.NewRecorder()
		h.Login(loginW, loginReq)

		var loginResp LoginResponse
		if err := json.Unmarshal(loginW.Body.Bytes(), &loginResp); err != nil {
			t.Fatal(err)
		}

		body := bytes.NewBufferString(`{"tier": "pro"}`)
		req := httptest.NewRequest("PUT", "/api/v1/users/regular", body)
		req.Header.Set("Authorization", "Bearer "+loginResp.Token)
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "regular")
		w := httptest.NewRecorder()

		h.UpdateUser(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})
}

func TestDeleteUser(t *testing.T) {
	h, userStore, adminToken := setupUserTest(t)

	t.Run("admin can delete user", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/v1/users/regular", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.SetPathValue("id", "regular")
		w := httptest.NewRecorder()

		h.DeleteUser(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("expected status 204, got %d", w.Code)
		}

		// Verify user was deleted
		_, exists := userStore.Find("regular")
		if exists {
			t.Error("user was not deleted from store")
		}
	})
}
