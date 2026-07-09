// Package features provides metadata for scanner indicators/features.
package features

// Feature describes an available indicator/field in the snapshot.
type Feature struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ShortDesc   string `json:"short_desc"`
	LongDesc    string `json:"long_desc"`
	Category    string `json:"category"`
	DataType    string `json:"data_type"` // "numeric", "boolean", "price", "volume"
	Sortable    bool   `json:"sortable"`
	WikiURL     string `json:"wiki_url,omitempty"`
}

// Category groups related features.
const (
	CategoryTrend         = "trend"
	CategoryMomentum      = "momentum"
	CategoryVolatility    = "volatility"
	CategoryPriceStruct   = "price_structure"
	CategoryReturns       = "returns"
	CategoryVolume        = "volume"
	CategoryPrice         = "price"
)

// Registry returns all available features.
func Registry() []Feature {
	return allFeatures
}

// ByID returns a feature by ID, or nil if not found.
func ByID(id string) *Feature {
	for i := range allFeatures {
		if allFeatures[i].ID == id {
			return &allFeatures[i]
		}
	}
	return nil
}

// ByCategory returns features filtered by category.
func ByCategory(category string) []Feature {
	var out []Feature
	for _, f := range allFeatures {
		if f.Category == category {
			out = append(out, f)
		}
	}
	return out
}

// Categories returns all unique categories.
func Categories() []string {
	return []string{
		CategoryPrice,
		CategoryTrend,
		CategoryMomentum,
		CategoryVolatility,
		CategoryPriceStruct,
		CategoryReturns,
		CategoryVolume,
	}
}
