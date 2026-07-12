// Package rates defines the canonical rate limit and usage tracking structures.
// Used by roles, throttle, admin panel, and API — single source of truth.
package rates

// Profile is the master rate/limit structure used across the system.
// Defined once here, consumed by roles, throttle, admin, and API handlers.
type Profile struct {
	APICallsPerMinute int `json:"api_calls_per_minute"`
	APICallsPerHour   int `json:"api_calls_per_hour"`
	APICallsPerDay    int `json:"api_calls_per_day"`
	MaxStudies        int `json:"max_studies"`
	MaxSavedResults   int `json:"max_saved_results"`
	MaxGroups         int `json:"max_groups"`
	MaxGroupMembers   int `json:"max_group_members"`
	ReplayDays        int `json:"replay_days"`
	MaxSymbolsPerScan int `json:"max_symbols_per_scan"`
	ExportMaxResults  int `json:"export_max_results"`
}

// Default returns sensible defaults for a new user.
func Default() Profile {
	return Profile{
		APICallsPerMinute: 60,
		APICallsPerHour:   1000,
		APICallsPerDay:    10000,
		MaxStudies:        3,
		MaxSavedResults:   100,
		MaxGroups:         5,
		MaxGroupMembers:   50,
		ReplayDays:        7,
		MaxSymbolsPerScan: 1000,
		ExportMaxResults:  1000,
	}
}

// Usage tracks daily consumption counters.
type Usage struct {
	APICalls   int `json:"api_calls"`
	Studies    int `json:"studies"`
	Results    int `json:"results"`
	Replays    int `json:"replays"`
}
