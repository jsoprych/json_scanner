package features

import (
	"testing"
)

func TestRegistry(t *testing.T) {
	feats := Registry()

	if len(feats) == 0 {
		t.Fatal("expected features, got 0")
	}

	// Verify all features have required fields
	for _, f := range feats {
		if f.ID == "" {
			t.Error("feature missing ID")
		}
		if f.Name == "" {
			t.Errorf("feature %s missing Name", f.ID)
		}
		if f.ShortDesc == "" {
			t.Errorf("feature %s missing ShortDesc", f.ID)
		}
		if f.LongDesc == "" {
			t.Errorf("feature %s missing LongDesc", f.ID)
		}
		if f.Category == "" {
			t.Errorf("feature %s missing Category", f.ID)
		}
		if f.DataType == "" {
			t.Errorf("feature %s missing DataType", f.ID)
		}
	}
}

func TestByID(t *testing.T) {
	t.Run("existing feature", func(t *testing.T) {
		f := ByID("sma200")
		if f == nil {
			t.Fatal("expected sma200, got nil")
		}
		if f.ID != "sma200" {
			t.Errorf("expected ID 'sma200', got %q", f.ID)
		}
		if f.Name != "SMA 200" {
			t.Errorf("expected Name 'SMA 200', got %q", f.Name)
		}
	})

	t.Run("non-existent feature", func(t *testing.T) {
		f := ByID("nonexistent")
		if f != nil {
			t.Errorf("expected nil, got %+v", f)
		}
	})
}

func TestByCategory(t *testing.T) {
	t.Run("trend category", func(t *testing.T) {
		feats := ByCategory(CategoryTrend)
		if len(feats) == 0 {
			t.Fatal("expected trend features, got 0")
		}

		for _, f := range feats {
			if f.Category != CategoryTrend {
				t.Errorf("expected category %q, got %q", CategoryTrend, f.Category)
			}
		}
	})

	t.Run("momentum category", func(t *testing.T) {
		feats := ByCategory(CategoryMomentum)
		if len(feats) == 0 {
			t.Fatal("expected momentum features, got 0")
		}

		for _, f := range feats {
			if f.Category != CategoryMomentum {
				t.Errorf("expected category %q, got %q", CategoryMomentum, f.Category)
			}
		}
	})

	t.Run("empty category", func(t *testing.T) {
		feats := ByCategory("nonexistent")
		if len(feats) != 0 {
			t.Errorf("expected 0 features, got %d", len(feats))
		}
	})
}

func TestCategories(t *testing.T) {
	cats := Categories()

	if len(cats) == 0 {
		t.Fatal("expected categories, got 0")
	}

	// Verify all categories are unique
	seen := make(map[string]bool)
	for _, c := range cats {
		if seen[c] {
			t.Errorf("duplicate category: %s", c)
		}
		seen[c] = true
	}

	// Verify expected categories exist
	expected := []string{
		CategoryPrice,
		CategoryTrend,
		CategoryMomentum,
		CategoryVolatility,
		CategoryPriceStruct,
		CategoryReturns,
		CategoryVolume,
	}

	for _, exp := range expected {
		if !seen[exp] {
			t.Errorf("missing expected category: %s", exp)
		}
	}
}

func TestFeatureCount(t *testing.T) {
	feats := Registry()

	// We expect around 50 features (the expanded set)
	if len(feats) < 40 {
		t.Errorf("expected at least 40 features, got %d", len(feats))
	}

	// Verify we have features in each category
	for _, cat := range Categories() {
		catFeats := ByCategory(cat)
		if len(catFeats) == 0 {
			t.Errorf("category %s has no features", cat)
		}
	}
}
