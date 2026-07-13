package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"cetus-marketdata-scanner/internal/features"
	"cetus-marketdata-scanner/internal/screen"
	"cetus-marketdata-scanner/internal/snapshot"
	"cetus-marketdata-scanner/internal/study"
	"cetus-marketdata-scanner/internal/telemetry"
)

func TestHealthEndpoint(t *testing.T) {
	snap, err := snapshot.OpenTest(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()

	log := telemetry.New(io.Discard)
	h := NewHandler(snap, log)

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()

	h.Health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}
}

func TestFeaturesEndpoint(t *testing.T) {
	snap, err := snapshot.OpenTest(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()

	log := telemetry.New(io.Discard)
	h := NewHandler(snap, log)

	t.Run("list all features", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/features", nil)
		w := httptest.NewRecorder()

		h.Features(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp FeaturesResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}

		if resp.Count == 0 {
			t.Error("expected features, got 0")
		}

		if resp.Count != len(resp.Features) {
			t.Errorf("count mismatch: %d != %d", resp.Count, len(resp.Features))
		}
	})

	t.Run("filter by category", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/features?category=trend", nil)
		w := httptest.NewRecorder()

		h.Features(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp FeaturesResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}

		if resp.Count == 0 {
			t.Error("expected trend features, got 0")
		}

		for _, f := range resp.Features {
			if f.Category != "trend" {
				t.Errorf("expected category 'trend', got %q", f.Category)
			}
		}
	})
}

func TestFeatureByIDEndpoint(t *testing.T) {
	snap, err := snapshot.OpenTest(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()

	log := telemetry.New(io.Discard)
	h := NewHandler(snap, log)

	t.Run("get existing feature", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/features/sma200", nil)
		req.SetPathValue("id", "sma200")
		w := httptest.NewRecorder()

		h.FeatureByID(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var feat features.Feature
		if err := json.Unmarshal(w.Body.Bytes(), &feat); err != nil {
			t.Fatal(err)
		}

		if feat.ID != "sma200" {
			t.Errorf("expected id 'sma200', got %q", feat.ID)
		}

		if feat.Name == "" {
			t.Error("expected name, got empty")
		}

		if feat.ShortDesc == "" {
			t.Error("expected short_desc, got empty")
		}
	})

	t.Run("get non-existent feature", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/features/nonexistent", nil)
		req.SetPathValue("id", "nonexistent")
		w := httptest.NewRecorder()

		h.FeatureByID(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

func TestScanEndpoint(t *testing.T) {
	snap, err := snapshot.OpenTest(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()

	// Load test data
	rows := []screen.SnapshotRow{
		{Symbol: "AAPL", Close: 150, DollarVol: 1e9, Ret3m: 0.1, RSI14: 60},
		{Symbol: "MSFT", Close: 300, DollarVol: 2e9, Ret3m: 0.2, RSI14: 70},
	}
	if err := snap.Load(rows, 1700000000); err != nil {
		t.Fatal(err)
	}

	log := telemetry.New(io.Discard)
	h := NewHandler(snap, log)

	t.Run("GET scan with where clause", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/scan?where=rsi14%20%3E%2065", nil)
		w := httptest.NewRecorder()

		h.Scan(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp ScanResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}

		if resp.Count != 1 {
			t.Errorf("expected 1 match, got %d", resp.Count)
		}
		if len(resp.Matches) > 0 && resp.Matches[0].Symbol != "MSFT" {
			t.Errorf("expected MSFT, got %s", resp.Matches[0].Symbol)
		}
	})

	t.Run("POST scan with JSON body", func(t *testing.T) {
		body := bytes.NewBufferString(`{"where": "close > 200", "order_by": "close DESC", "limit": 10}`)
		req := httptest.NewRequest("POST", "/api/v1/scan", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.Scan(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp ScanResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}

		if resp.Count != 1 {
			t.Errorf("expected 1 match, got %d", resp.Count)
		}
	})

	t.Run("scan without where clause", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/scan", nil)
		w := httptest.NewRecorder()

		h.Scan(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestStudiesEndpoints(t *testing.T) {
	snap, err := snapshot.OpenTest(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()

	// Create a temporary studies file
	tmpFile := t.TempDir() + "/studies.jsonl"
	studyStore, err := study.OpenStore(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	log := telemetry.New(io.Discard)
	h := NewHandlerFull(snap, studyStore, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, log)

	t.Run("create study", func(t *testing.T) {
		body := bytes.NewBufferString(`{"key": "test-study", "title": "Test Study", "where": "rsi14 > 70", "owner": "global"}`)
		req := httptest.NewRequest("POST", "/api/v1/studies", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.CreateStudy(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d", w.Code)
		}
	})

	t.Run("list studies", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/studies", nil)
		w := httptest.NewRecorder()

		h.ListStudies(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var studies []study.Study
		if err := json.Unmarshal(w.Body.Bytes(), &studies); err != nil {
			t.Fatal(err)
		}

		if len(studies) != 1 {
			t.Errorf("expected 1 study, got %d", len(studies))
		}
	})

	t.Run("get study", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/studies/test-study", nil)
		req.SetPathValue("id", "test-study")
		w := httptest.NewRecorder()

		h.GetStudy(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var st study.Study
		if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
			t.Fatal(err)
		}

		if st.Key != "test-study" {
			t.Errorf("expected key 'test-study', got %q", st.Key)
		}
	})

	t.Run("delete study", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/v1/studies/test-study", nil)
		req.SetPathValue("id", "test-study")
		w := httptest.NewRecorder()

		h.DeleteStudy(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("expected status 204, got %d", w.Code)
		}

		// Verify deleted
		req = httptest.NewRequest("GET", "/api/v1/studies", nil)
		w = httptest.NewRecorder()
		h.ListStudies(w, req)
		var studies []study.Study
		json.Unmarshal(w.Body.Bytes(), &studies)
		if len(studies) != 0 {
			t.Errorf("expected 0 studies after delete, got %d", len(studies))
		}
	})
}

func TestSnapshotsEndpoint(t *testing.T) {
	snap, err := snapshot.OpenTest(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()

	// Load test snapshots
	rows := []screen.SnapshotRow{{Symbol: "AAPL", Close: 150}}
	snap.LoadHistory(rows, 1700000000, 1700000000)
	snap.LoadHistory(rows, 1700086400, 1700086400)

	log := telemetry.New(io.Discard)
	h := NewHandler(snap, log)

	req := httptest.NewRequest("GET", "/api/v1/snapshots", nil)
	w := httptest.NewRecorder()

	h.Snapshots(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp SnapshotsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.Count != 2 {
		t.Errorf("expected 2 snapshots, got %d", resp.Count)
	}
}
