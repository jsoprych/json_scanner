package user

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestPassword(t *testing.T) {
	var u User
	u.SetPassword("hunter2")
	if !strings.HasPrefix(u.PassHash, "pbkdf2_sha256$") {
		t.Fatalf("unexpected hash format: %s", u.PassHash)
	}
	if u.PassSHA256 != "" {
		t.Error("legacy hash should be cleared on set")
	}
	if !u.CheckPassword("hunter2") {
		t.Error("correct password should verify")
	}
	if u.CheckPassword("wrong") {
		t.Error("wrong password must not verify")
	}
	// salted: same password, different hash.
	var u2 User
	u2.SetPassword("hunter2")
	if u2.PassHash == u.PassHash {
		t.Error("salt should make identical passwords hash differently")
	}
	// disabled never matches.
	u.Disabled = true
	if u.CheckPassword("hunter2") {
		t.Error("disabled user must not verify")
	}
	// legacy unsalted sha256 still verifies (back-compat).
	sum := sha256.Sum256([]byte("legacy"))
	if !(User{PassSHA256: hex.EncodeToString(sum[:])}).CheckPassword("legacy") {
		t.Error("legacy sha256 should still verify")
	}
}

func TestRegistryLoad(t *testing.T) {
	data := `
# seed
{"id":"admin","name":"Admin","tier":"pro","role":"admin"}
{"id":"alice","name":"Alice","tier":"pro"}
{"id":"bob","name":"Bob"}
`
	reg, err := LoadJSONL(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.All()) != 3 {
		t.Fatalf("users = %d", len(reg.All()))
	}

	admin, ok := reg.Find("admin")
	if !ok || !admin.IsAdmin() {
		t.Errorf("admin = %+v ok=%v", admin, ok)
	}
	// defaults: missing role → user, missing tier → free.
	bob, _ := reg.Find("bob")
	if bob.Role != RoleUser || bob.Tier != TierFree {
		t.Errorf("bob defaults = %+v", bob)
	}
	alice, _ := reg.Find("alice")
	if alice.Tier != TierPro || alice.Role != RoleUser {
		t.Errorf("alice = %+v", alice)
	}
	if _, ok := reg.Find("nobody"); ok {
		t.Error("unexpected user 'nobody'")
	}
}

func TestInGroup(t *testing.T) {
	u := User{Groups: []string{"desk-a", "research"}}
	if !u.InGroup("desk-a") || !u.InGroup("research") {
		t.Error("expected membership")
	}
	if u.InGroup("nope") || (User{}).InGroup("desk-a") {
		t.Error("unexpected membership")
	}
}

func TestMissingIDErrors(t *testing.T) {
	if _, err := LoadJSONL(strings.NewReader(`{"name":"NoID"}`)); err == nil {
		t.Error("expected error for missing id")
	}
}
