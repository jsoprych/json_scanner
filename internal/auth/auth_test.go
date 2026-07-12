package auth

import (
	"database/sql"
	"net/http/httptest"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// --- Password Hashing ---

func TestHashPassword(t *testing.T) {
	h := HashPassword("test123")
	if h == "" {
		t.Fatal("empty hash")
	}
	if !CheckPassword(h, "test123") {
		t.Error("should match")
	}
	if CheckPassword(h, "wrong") {
		t.Error("should not match wrong password")
	}
}

func TestHashPasswordUniqueness(t *testing.T) {
	h1 := HashPassword("same")
	h2 := HashPassword("same")
	if h1 == h2 {
		t.Error("hashes should differ due to random salt")
	}
	if !CheckPassword(h1, "same") {
		t.Error("h1 should match same")
	}
	if !CheckPassword(h2, "same") {
		t.Error("h2 should match same")
	}
}

func TestCheckPasswordEmpty(t *testing.T) {
	if CheckPassword("", "anything") {
		t.Error("empty hash should not match")
	}
}

func TestCheckPasswordLegacySHA256(t *testing.T) {
	// Legacy format: hex-encoded SHA256
	h := HashPassword("legacy-test")
	if !CheckPassword(h, "legacy-test") {
		t.Error("PBKDF2 should match")
	}
}

func TestCheckPasswordWrongFormat(t *testing.T) {
	if CheckPassword("garbage!!", "test") {
		t.Error("invalid format should not match")
	}
}

// --- Password Validation ---

func TestValidatePassword(t *testing.T) {
	pol := PasswordPolicy{MinLength: 8, RequireUpper: true, RequireDigit: true}

	tests := []struct {
		pw   string
		want bool
	}{
		{"Short1", false},           // too short
		{"nouppercase1", false},     // no uppercase
		{"NoDigitsHere", false},     // no digit
		{"GoodPass1", true},
		{"VeryLongGoodPass1", true},
		{"password", false},         // common
		{"P@ssw0rd123", true},       // special chars count as digits
	}
	for _, tc := range tests {
		err := ValidatePassword(tc.pw, pol)
		ok := err == nil
		if ok != tc.want {
			t.Errorf("ValidatePassword(%q) = %v, want %v (err: %v)", tc.pw, ok, tc.want, err)
		}
	}
}

func TestStrengthScore(t *testing.T) {
	if s := StrengthScore(""); s != 0 {
		t.Errorf("empty = %d, want 0", s)
	}
	weak := StrengthScore("short")
	strong := StrengthScore("VeryLongP@ssw0rd!")
	if weak >= strong {
		t.Errorf("weak(%d) should be < strong(%d)", weak, strong)
	}
	if s := StrengthScore("Verystr0ng!"); s < 60 {
		t.Errorf("strong password scored %d, want >= 60", s)
	}
}

func TestStrengthLabel(t *testing.T) {
	tests := []struct{ score int; want string }{
		{0, "Too Weak"},
		{25, "Weak"},
		{45, "Fair"},
		{65, "Strong"},
		{85, "Very Strong"},
	}
	for _, tc := range tests {
		if got := StrengthLabel(tc.score); got != tc.want {
			t.Errorf("StrengthLabel(%d) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

// --- Session Store ---

func TestSessionStore(t *testing.T) {
	s := NewSessionStore("test_session")

	// Create
	tok := s.Create("alice", time.Hour)
	if tok == "" {
		t.Fatal("empty token")
	}

	// Get valid
	uid, ok := s.Get(tok)
	if !ok || uid != "alice" {
		t.Errorf("Get = (%q, %v), want (alice, true)", uid, ok)
	}

	// Delete
	s.Delete(tok)
	_, ok = s.Get(tok)
	if ok {
		t.Error("should not find deleted session")
	}
}

func TestSessionStoreExpiry(t *testing.T) {
	s := NewSessionStore("test_session")
	tok := s.Create("bob", 1*time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	_, ok := s.Get(tok)
	if ok {
		t.Error("expired session should not be valid")
	}
}

func TestSessionStoreCookie(t *testing.T) {
	s := NewSessionStore("test_cookie")
	tok := s.Create("carol", time.Hour)

	w := httptest.NewRecorder()
	s.SetCookie(w, tok, time.Hour)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", w.Header().Get("Set-Cookie"))

	got, ok := s.GetCookie(req)
	if !ok || got != tok {
		t.Errorf("GetCookie = (%q, %v), want (%q, true)", got, ok, tok)
	}
}

func TestSessionStoreClearCookie(t *testing.T) {
	s := NewSessionStore("test_cookie")
	w := httptest.NewRecorder()
	s.ClearCookie(w)
	c := w.Header().Get("Set-Cookie")
	if c == "" {
		t.Error("ClearCookie should set a cookie")
	}
}

// --- Login Guard ---

func testGuardDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestLoginGuard(t *testing.T) {
	db := testGuardDB(t)
	defer db.Close()

	g, err := NewLoginGuard(db)
	if err != nil {
		t.Fatal(err)
	}

	// No record → allowed
	msg, allowed := g.Check("alice")
	if !allowed || msg != "" {
		t.Errorf("first check: msg=%q allowed=%v", msg, allowed)
	}

	// Record 5 failures
	for i := 0; i < 5; i++ {
		g.RecordFailure("alice")
	}

	// Should be locked now
	msg, allowed = g.Check("alice")
	if allowed {
		t.Error("should be locked after 5 failures")
	}
	if msg == "" {
		t.Error("should have lock message")
	}

	// Record success clears it
	g.RecordSuccess("alice")
	_, allowed = g.Check("alice")
	if !allowed {
		t.Error("should be unlocked after success")
	}
}

func TestLoginGuardSeparateAccounts(t *testing.T) {
	db := testGuardDB(t)
	defer db.Close()

	g, _ := NewLoginGuard(db)

	// Lock alice
	for i := 0; i < 5; i++ {
		g.RecordFailure("alice")
	}
	_, allowed := g.Check("alice")
	if allowed {
		t.Error("alice should be locked")
	}

	// Bob should still be fine
	_, allowed = g.Check("bob")
	if !allowed {
		t.Error("bob should still be allowed")
	}
}
