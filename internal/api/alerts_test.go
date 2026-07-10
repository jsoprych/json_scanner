package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cetus-marketdata-scanner/internal/alert"
	"cetus-marketdata-scanner/internal/authjwt"
	"cetus-marketdata-scanner/internal/screen"
	"cetus-marketdata-scanner/internal/snapshot"
	"cetus-marketdata-scanner/internal/study"
	"cetus-marketdata-scanner/internal/telemetry"
	"cetus-marketdata-scanner/internal/user"
)

func setupAlertTest(t *testing.T) (*Handler, *user.Store, *study.Store, *snapshot.DB, string) {
	snap, err := snapshot.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { snap.Close() })

	// Load two snapshots
	rows1 := []screen.SnapshotRow{
		{Symbol: "AAPL", Close: 150, DollarVol: 1e9, RSI14: 30},
		{Symbol: "MSFT", Close: 300, DollarVol: 2e9, RSI14: 25},
	}
	rows2 := []screen.SnapshotRow{
		{Symbol: "AAPL", Close: 155, DollarVol: 1.1e9, RSI14: 25},
		{Symbol: "GOOG", Close: 2800, DollarVol: 3e9, RSI14: 20},
	}

	if err := snap.LoadHistory(rows1, 1700000000, 1700000000); err != nil {
		t.Fatal(err)
	}
	if err := snap.LoadHistory(rows2, 1700086400, 1700086400); err != nil {
		t.Fatal(err)
	}

	// Create user store
	tmpUserFile := t.TempDir() + "/users.jsonl"
	userStore, err := user.OpenStore(tmpUserFile)
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

	// Create study store
	tmpStudyFile := t.TempDir() + "/studies.jsonl"
	studyStore, err := study.OpenStore(tmpStudyFile)
	if err != nil {
		t.Fatal(err)
	}

	testStudy := study.Study{
		Key:   "oversold",
		Title: "Oversold Stocks",
		Where: "rsi14 < 35",
		Owner: "testuser",
	}
	if err := studyStore.Upsert(testStudy); err != nil {
		t.Fatal(err)
	}

	// Create detector
	detector := alert.NewDetector(snap)

	// Create JWT signer and verifier
	secret := []byte("test-secret")
	signer := authjwt.NewHMACSigner(secret, "test", 24*time.Hour)
	verifier := authjwt.NewHMAC(secret, "sub", "test", "")

	log := telemetry.New(io.Discard)
	h := NewHandlerFull(snap, studyStore, nil, userStore, nil, detector, nil, signer, verifier, log)

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

	return h, userStore, studyStore, snap, loginResp.Token
}

func TestAlertsEndpoint(t *testing.T) {
	h, _, _, _, token := setupAlertTest(t)

	t.Run("get alerts for study", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/studies/oversold/alerts?date=1700086400&prev_date=1700000000", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.SetPathValue("id", "oversold")
		w := httptest.NewRecorder()

		h.Alerts(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}

		entries := resp["entries"].([]interface{})
		exits := resp["exits"].([]interface{})

		if len(entries) != 1 {
			t.Errorf("expected 1 entry, got %d", len(entries))
		}
		if len(exits) != 1 {
			t.Errorf("expected 1 exit, got %d", len(exits))
		}
	})

	t.Run("missing date parameters", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/studies/oversold/alerts", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.SetPathValue("id", "oversold")
		w := httptest.NewRecorder()

		h.Alerts(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("non-existent study", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/studies/nonexistent/alerts?date=1700086400&prev_date=1700000000", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.SetPathValue("id", "nonexistent")
		w := httptest.NewRecorder()

		h.Alerts(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

func TestEntriesEndpoint(t *testing.T) {
	h, _, _, _, token := setupAlertTest(t)

	t.Run("get entries for study", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/studies/oversold/entries?date=1700086400&prev_date=1700000000", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.SetPathValue("id", "oversold")
		w := httptest.NewRecorder()

		h.Entries(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}

		entries := resp["entries"].([]interface{})
		if len(entries) != 1 {
			t.Errorf("expected 1 entry, got %d", len(entries))
		}

		entry := entries[0].(map[string]interface{})
		if entry["symbol"] != "GOOG" {
			t.Errorf("expected GOOG entry, got %v", entry["symbol"])
		}
	})
}

func TestExitsEndpoint(t *testing.T) {
	h, _, _, _, token := setupAlertTest(t)

	t.Run("get exits for study", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/studies/oversold/exits?date=1700086400&prev_date=1700000000", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.SetPathValue("id", "oversold")
		w := httptest.NewRecorder()

		h.Exits(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}

		exits := resp["exits"].([]interface{})
		if len(exits) != 1 {
			t.Errorf("expected 1 exit, got %d", len(exits))
		}

		exit := exits[0].(map[string]interface{})
		if exit["symbol"] != "MSFT" {
			t.Errorf("expected MSFT exit, got %v", exit["symbol"])
		}
	})
}
