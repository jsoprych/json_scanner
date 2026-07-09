package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"cetus-marketdata-scanner/internal/features"
	"cetus-marketdata-scanner/internal/snapshot"
	"cetus-marketdata-scanner/internal/telemetry"
)

func TestHealthEndpoint(t *testing.T) {
	snap, err := snapshot.Open(":memory:")
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
	snap, err := snapshot.Open(":memory:")
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
	snap, err := snapshot.Open(":memory:")
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
