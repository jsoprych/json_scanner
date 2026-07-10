package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	for _, k := range []string{"SCANNER_DB_PATH", "CETUS_DB", "SCANNER_STORE_DB"} {
		os.Unsetenv(k)
	}
	cfg := Load()
	if cfg.SinceDays != 120 {
		t.Errorf("SinceDays: got %d want 120", cfg.SinceDays)
	}
	if cfg.Lookback != 20 {
		t.Errorf("Lookback: got %d want 20", cfg.Lookback)
	}
	if cfg.VolumeMult != 2.0 {
		t.Errorf("VolumeMult: got %f want 2.0", cfg.VolumeMult)
	}
	if cfg.GapPct != 0.05 {
		t.Errorf("GapPct: got %f want 0.05", cfg.GapPct)
	}
	if cfg.ServeAddr != ":8080" {
		t.Errorf("ServeAddr: got %q want %q", cfg.ServeAddr, ":8080")
	}
	if cfg.AuthMode != "login" {
		t.Errorf("AuthMode: got %q want %q", cfg.AuthMode, "login")
	}
	if cfg.Universe != "index:r3000" {
		t.Errorf("Universe: got %q want %q", cfg.Universe, "index:r3000")
	}
}

func TestEnvOverride(t *testing.T) {
	os.Setenv("SCANNER_DB_PATH", "/tmp/test.db")
	defer os.Unsetenv("SCANNER_DB_PATH")

	cfg := Load()
	if cfg.DBPath != "/tmp/test.db" {
		t.Errorf("DBPath: got %q want %q", cfg.DBPath, "/tmp/test.db")
	}
}

func TestResolveDBPathPrecedence(t *testing.T) {
	os.Unsetenv("SCANNER_DB_PATH")
	os.Unsetenv("CETUS_DB")

	os.Setenv("CETUS_DB", "/tmp/cetus.db")
	defer os.Unsetenv("CETUS_DB")

	got := resolveDBPath()
	if got != "/tmp/cetus.db" {
		t.Errorf("resolveDBPath with CETUS_DB: got %q want %q", got, "/tmp/cetus.db")
	}

	os.Setenv("SCANNER_DB_PATH", "/tmp/override.db")
	defer os.Unsetenv("SCANNER_DB_PATH")

	got = resolveDBPath()
	if got != "/tmp/override.db" {
		t.Errorf("resolveDBPath with SCANNER_DB_PATH: got %q want %q", got, "/tmp/override.db")
	}
}

func TestResolveRelative(t *testing.T) {
	if got := resolveRelative("/abs/path.db"); got != "/abs/path.db" {
		t.Errorf("absolute path changed: got %q", got)
	}
	if got := resolveRelative(""); got != "" {
		t.Errorf("empty path changed: got %q", got)
	}
	if got := resolveRelative(":memory:"); got != ":memory:" {
		t.Errorf(":memory: changed: got %q", got)
	}

	exe, err := os.Executable()
	if err != nil {
		t.Skip("cannot determine executable")
	}
	got := resolveRelative("relative/path.db")
	want := filepath.Join(filepath.Dir(exe), "relative/path.db")
	if got != want {
		t.Errorf("resolveRelative: got %q want %q", got, want)
	}
}

func TestResolveStoreDB(t *testing.T) {
	os.Unsetenv("SCANNER_STORE_DB")

	exe, err := os.Executable()
	if err != nil {
		t.Skip("cannot determine executable")
	}
	got := resolveStoreDB()
	want := filepath.Join(filepath.Dir(exe), defaultScannerDB)
	if got != want {
		t.Errorf("resolveStoreDB default: got %q want %q", got, want)
	}

	os.Setenv("SCANNER_STORE_DB", "/tmp/store.db")
	defer os.Unsetenv("SCANNER_STORE_DB")
	if got := resolveStoreDB(); got != "/tmp/store.db" {
		t.Errorf("resolveStoreDB env: got %q want %q", got, "/tmp/store.db")
	}

	os.Setenv("SCANNER_STORE_DB", ":memory:")
	defer os.Unsetenv("SCANNER_STORE_DB")
	if got := resolveStoreDB(); got != ":memory:" {
		t.Errorf("resolveStoreDB :memory:: got %q want %q", got, ":memory:")
	}
}
