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
	"cetus-marketdata-scanner/internal/study"
	"cetus-marketdata-scanner/internal/telemetry"
	"cetus-marketdata-scanner/internal/user"
)

func setupSubscriptionTest(t *testing.T) (*Handler, *user.Store, *study.Store, *study.SubscriptionStore, string) {
	snap, err := snapshot.OpenTest(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { snap.Close() })

	// Create user store
	tmpUserFile := t.TempDir() + "/users.jsonl"
	userStore, err := user.OpenStore(tmpUserFile)
	if err != nil {
		t.Fatal(err)
	}

	// Create test user
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

	// Create study store
	tmpStudyFile := t.TempDir() + "/studies.jsonl"
	studyStore, err := study.OpenStore(tmpStudyFile)
	if err != nil {
		t.Fatal(err)
	}

	// Create test study
	testStudy := study.Study{
		Key:   "test-study",
		Title: "Test Study",
		Where: "rsi14 < 30",
		Owner: "testuser",
	}
	if err := studyStore.Upsert(testStudy); err != nil {
		t.Fatal(err)
	}

	// Create subscription store
	tmpSubFile := t.TempDir() + "/subscriptions.jsonl"
	subStore, err := study.OpenSubscriptionStore(tmpSubFile)
	if err != nil {
		t.Fatal(err)
	}

	// Create JWT signer and verifier
	secret := []byte("test-secret")
	signer := authjwt.NewHMACSigner(secret, "test", 24*time.Hour)
	verifier := authjwt.NewHMAC(secret, "sub", "test", "")

	log := telemetry.New(io.Discard)
	h := NewHandlerFull(snap, studyStore, nil, userStore, subStore, nil, nil, nil, nil, nil, nil, nil, signer, verifier, nil, log)

	// Login to get token
	loginBody := bytes.NewBufferString(`{"user": "testuser", "password": "testpass123"}`)
	loginReq := httptest.NewRequest("POST", "/api/v1/auth/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	h.Login(loginW, loginReq)

	var loginResp LoginResponse
	if err := json.Unmarshal(loginW.Body.Bytes(), &loginResp); err != nil {
		t.Fatal(err)
	}

	return h, userStore, studyStore, subStore, loginResp.Token
}

func TestSubscribe(t *testing.T) {
	h, _, _, subStore, token := setupSubscriptionTest(t)

	t.Run("subscribe to study", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/studies/test-study/subscribe", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.SetPathValue("id", "test-study")
		w := httptest.NewRecorder()

		h.Subscribe(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}

		if resp["status"] != "subscribed" {
			t.Errorf("expected status 'subscribed', got %q", resp["status"])
		}

		// Verify subscription was created
		if !subStore.IsSubscribed("testuser", "test-study") {
			t.Error("subscription was not created in store")
		}
	})

	t.Run("subscribe to non-existent study", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/studies/nonexistent/subscribe", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.SetPathValue("id", "nonexistent")
		w := httptest.NewRecorder()

		h.Subscribe(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

func TestUnsubscribe(t *testing.T) {
	h, _, _, subStore, token := setupSubscriptionTest(t)

	// First subscribe
	subStore.Subscribe("testuser", "test-study")

	t.Run("unsubscribe from study", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/v1/studies/test-study/subscribe", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.SetPathValue("id", "test-study")
		w := httptest.NewRecorder()

		h.Unsubscribe(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}

		if resp["status"] != "unsubscribed" {
			t.Errorf("expected status 'unsubscribed', got %q", resp["status"])
		}

		// Verify subscription was removed
		if subStore.IsSubscribed("testuser", "test-study") {
			t.Error("subscription was not removed from store")
		}
	})
}

func TestIsSubscribed(t *testing.T) {
	h, _, _, subStore, token := setupSubscriptionTest(t)

	t.Run("check subscription when not subscribed", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/studies/test-study/subscribed", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.SetPathValue("id", "test-study")
		w := httptest.NewRecorder()

		h.IsSubscribed(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}

		if resp["is_subscribed"].(bool) != false {
			t.Error("expected is_subscribed to be false")
		}
	})

	t.Run("check subscription when subscribed", func(t *testing.T) {
		subStore.Subscribe("testuser", "test-study")

		req := httptest.NewRequest("GET", "/api/v1/studies/test-study/subscribed", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.SetPathValue("id", "test-study")
		w := httptest.NewRecorder()

		h.IsSubscribed(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}

		if resp["is_subscribed"].(bool) != true {
			t.Error("expected is_subscribed to be true")
		}
	})
}

func TestSubscriptions(t *testing.T) {
	h, _, studyStore, subStore, token := setupSubscriptionTest(t)

	// Create another study
	studyStore.Upsert(study.Study{
		Key:   "test-study-2",
		Title: "Test Study 2",
		Where: "rsi14 > 70",
		Owner: "testuser",
	})

	// Subscribe to both studies
	subStore.Subscribe("testuser", "test-study")
	subStore.Subscribe("testuser", "test-study-2")

	t.Run("get user subscriptions", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/subscriptions", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		h.Subscriptions(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var studies []study.Study
		if err := json.Unmarshal(w.Body.Bytes(), &studies); err != nil {
			t.Fatal(err)
		}

		if len(studies) != 2 {
			t.Errorf("expected 2 subscriptions, got %d", len(studies))
		}
	})
}

func TestStudySubscribers(t *testing.T) {
	h, userStore, _, subStore, token := setupSubscriptionTest(t)

	// Create admin user
	admin := user.User{
		ID:   "admin",
		Name: "Admin User",
		Tier: user.TierPro,
		Role: user.RoleAdmin,
	}
	admin.SetPassword("adminpass")
	userStore.Create(admin)

	// Subscribe multiple users
	subStore.Subscribe("testuser", "test-study")

	// Login as admin
	loginBody := bytes.NewBufferString(`{"user": "admin", "password": "adminpass"}`)
	loginReq := httptest.NewRequest("POST", "/api/v1/auth/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	h.Login(loginW, loginReq)

	var loginResp LoginResponse
	json.Unmarshal(loginW.Body.Bytes(), &loginResp)

	t.Run("admin can view subscribers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/studies/test-study/subscribers", nil)
		req.Header.Set("Authorization", "Bearer "+loginResp.Token)
		req.SetPathValue("id", "test-study")
		w := httptest.NewRecorder()

		h.StudySubscribers(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}

		subscribers := resp["subscribers"].([]interface{})
		if len(subscribers) != 1 {
			t.Errorf("expected 1 subscriber, got %d", len(subscribers))
		}
	})

	t.Run("non-admin cannot view subscribers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/studies/test-study/subscribers", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.SetPathValue("id", "test-study")
		w := httptest.NewRecorder()

		h.StudySubscribers(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})
}
