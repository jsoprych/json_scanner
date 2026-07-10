// Package user is the identity layer that owns saved studies (and soon watchlists,
// alerts, profiles). Users have a subscription Tier (what they can access) and a
// Role (what they can do). Seeded from a JSONL registry for now; this migrates into
// the scanner's own DB when real accounts arrive — ownership/tier/role are wired
// through the model from day one so nothing has to be reshaped later.
package user

import (
	"bufio"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

// pwEnc encodes salt/derived-key in password hashes (compact, unpadded).
var pwEnc = base64.RawStdEncoding

const pbkdf2Iter = 600000 // OWASP-recommended for PBKDF2-HMAC-SHA256

// Tier is a subscription level. Higher tiers can access more studies.
type Tier string

const (
	TierFree Tier = "free"
	TierPro  Tier = "pro"
)

// Rank orders tiers (higher = more access). Unknown/empty ranks as free.
func (t Tier) Rank() int {
	if t == TierPro {
		return 1
	}
	return 0
}

// Role is an access role, orthogonal to subscription tier.
type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// User owns studies and other saved entities, at a subscription tier and role.
type User struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Tier       Tier   `json:"tier"`
	Role       Role   `json:"role"`
	Groups     []string `json:"groups,omitempty"`      // group memberships (for group-visible studies)
	PassHash   string   `json:"pass_hash,omitempty"`   // salted PBKDF2: pbkdf2_sha256$iter$salt$dk
	PassSHA256 string   `json:"pass_sha256,omitempty"` // legacy unsalted sha256 (read-only compat)
	Disabled   bool     `json:"disabled,omitempty"`    // disabled users can't sign in
}

// IsAdmin reports the admin role — an admin sees every study regardless of owner or
// tier.
func (u User) IsAdmin() bool { return u.Role == RoleAdmin }

// InGroup reports membership in a group.
func (u User) InGroup(g string) bool {
	for _, x := range u.Groups {
		if x == g {
			return true
		}
	}
	return false
}

// SetPassword stores a salted PBKDF2-HMAC-SHA256 hash of pw and clears any legacy
// hash. Salt is random per set, so identical passwords hash differently.
func (u *User) SetPassword(pw string) {
	salt := make([]byte, 16)
	rand.Read(salt)
	dk, _ := pbkdf2.Key(sha256.New, pw, salt, pbkdf2Iter, 32)
	u.PassHash = fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", pbkdf2Iter, pwEnc.EncodeToString(salt), pwEnc.EncodeToString(dk))
	u.PassSHA256 = ""
}

// CheckPassword reports whether pw matches, in constant time. Prefers the PBKDF2
// hash; falls back to the legacy unsalted sha256 for un-migrated accounts. A
// disabled user, or one with no password, never matches.
func (u User) CheckPassword(pw string) bool {
	if u.Disabled {
		return false
	}
	if u.PassHash != "" {
		return checkPBKDF2(u.PassHash, pw)
	}
	if u.PassSHA256 != "" { // legacy
		want, err := hex.DecodeString(u.PassSHA256)
		if err != nil {
			return false
		}
		sum := sha256.Sum256([]byte(pw))
		return subtle.ConstantTimeCompare(sum[:], want) == 1
	}
	return false
}

// checkPBKDF2 verifies a "pbkdf2_sha256$iter$salt$dk" hash.
func checkPBKDF2(stored, pw string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1 {
		return false
	}
	salt, err := pwEnc.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := pwEnc.DecodeString(parts[3])
	if err != nil {
		return false
	}
	dk, err := pbkdf2.Key(sha256.New, pw, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(dk, want) == 1
}

// GlobalID is the owner id for shared, system-owned entities.
const GlobalID = "global"

// Global is the built-in system superuser (admin, top tier) — the fallback owner
// and acting user before/without a real account.
func Global() User { return User{ID: GlobalID, Name: "Global", Tier: TierPro, Role: RoleAdmin} }

// Registry is a set of users loaded from a JSONL seed (soon: the scanner's own DB).
type Registry struct {
	all  []User
	byID map[string]User
}

// LoadJSONL reads users (one JSON object per line; blank and #-comment lines
// skipped). Missing tier defaults to free, missing role to user.
func LoadJSONL(r io.Reader) (*Registry, error) {
	reg := &Registry{byID: map[string]User{}}
	sc := bufio.NewScanner(r)
	n := 0
	for sc.Scan() {
		n++
		t := strings.TrimSpace(sc.Text())
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		var u User
		if err := json.Unmarshal([]byte(t), &u); err != nil {
			return nil, fmt.Errorf("users line %d: %w", n, err)
		}
		if u.ID == "" {
			return nil, fmt.Errorf("users line %d: missing id", n)
		}
		if u.Tier == "" {
			u.Tier = TierFree
		}
		if u.Role == "" {
			u.Role = RoleUser
		}
		reg.all = append(reg.all, u)
		reg.byID[u.ID] = u
	}
	return reg, sc.Err()
}

// LoadFile loads a users registry from a JSONL path.
func LoadFile(path string) (*Registry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LoadJSONL(f)
}

// Find returns the user with the given id.
func (r *Registry) Find(id string) (User, bool) {
	u, ok := r.byID[id]
	return u, ok
}

// All returns every user, in file order.
func (r *Registry) All() []User { return r.all }

// Store is a mutable, JSONL-file-backed user registry, safe for concurrent use.
// The admin dashboard creates/updates/deletes through it; every mutation atomically
// rewrites the file. (Migrates into the scanner's own DB with real accounts.)
type Store struct {
	path string
	mu   sync.Mutex
	all  []User
	byID map[string]User
}

// OpenStore loads a user store from path (a missing file yields an empty store).
func OpenStore(path string) (*Store, error) {
	s := &Store{path: path, byID: map[string]User{}}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	defer f.Close()
	reg, err := LoadJSONL(f)
	if err != nil {
		return nil, err
	}
	s.all = reg.all
	for _, u := range s.all {
		s.byID[u.ID] = u
	}
	return s, nil
}

// All returns a copy of every user.
func (s *Store) All() []User {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]User(nil), s.all...)
}

// Find returns the user with the given id.
func (s *Store) Find(id string) (User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.byID[id]
	return u, ok
}

// Create adds a new user (error if the id exists).
func (s *Store) Create(u User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(u.ID) == "" {
		return fmt.Errorf("id required")
	}
	if _, ok := s.byID[u.ID]; ok {
		return fmt.Errorf("user %q already exists", u.ID)
	}
	if u.Tier == "" {
		u.Tier = TierFree
	}
	if u.Role == "" {
		u.Role = RoleUser
	}
	s.all = append(s.all, u)
	s.byID[u.ID] = u
	return s.save()
}

// mutate applies fn to the stored user and persists.
func (s *Store) mutate(id string, fn func(*User)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.all {
		if s.all[i].ID == id {
			fn(&s.all[i])
			s.byID[id] = s.all[i]
			return s.save()
		}
	}
	return fmt.Errorf("user %q not found", id)
}

// SetDisabled disables/enables a user.
func (s *Store) SetDisabled(id string, d bool) error { return s.mutate(id, func(u *User) { u.Disabled = d }) }

// SetName changes a user's display name.
func (s *Store) SetName(id, name string) error { return s.mutate(id, func(u *User) { u.Name = name }) }

// SetTier changes a user's subscription tier.
func (s *Store) SetTier(id string, t Tier) error { return s.mutate(id, func(u *User) { u.Tier = t }) }

// SetRole changes a user's role.
func (s *Store) SetRole(id string, r Role) error { return s.mutate(id, func(u *User) { u.Role = r }) }

// SetPassword resets a user's password.
func (s *Store) SetPassword(id, pw string) error { return s.mutate(id, func(u *User) { u.SetPassword(pw) }) }

// SetGroups replaces a user's group memberships.
func (s *Store) SetGroups(id string, groups []string) error {
	return s.mutate(id, func(u *User) { u.Groups = groups })
}

// Delete removes a user.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[id]; !ok {
		return fmt.Errorf("user %q not found", id)
	}
	delete(s.byID, id)
	out := s.all[:0]
	for _, u := range s.all {
		if u.ID != id {
			out = append(out, u)
		}
	}
	s.all = out
	return s.save()
}

// save atomically rewrites the JSONL file. Caller holds the lock.
func (s *Store) save() error {
	tmp := s.path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	fmt.Fprintln(w, "# Users — one JSON per line. tier: free|pro · role: user|admin. Managed by the admin dashboard.")
	enc := json.NewEncoder(w)
	for _, u := range s.all {
		if err := enc.Encode(u); err != nil {
			f.Close()
			return err
		}
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
