// Package user is the identity layer that owns saved studies (and soon watchlists,
// alerts, profiles). Users have a subscription Tier (what they can access) and a
// Role (what they can do). Seeded from a JSONL registry for now; this migrates into
// the scanner's own DB when real accounts arrive — ownership/tier/role are wired
// through the model from day one so nothing has to be reshaped later.
package user

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

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
	ID   string `json:"id"`
	Name string `json:"name"`
	Tier Tier   `json:"tier"`
	Role Role   `json:"role"`
}

// IsAdmin reports the admin role — an admin sees every study regardless of owner or
// tier.
func (u User) IsAdmin() bool { return u.Role == RoleAdmin }

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
