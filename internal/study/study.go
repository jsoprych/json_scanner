// Package study holds saved screens as data: a SQL WHERE clause (plus ranking and
// cap) over the snapshot table, owned by a user. Pre-DSL, SQL *is* the expression
// language — no parser to write. Studies live in a JSONL file (one per line) and
// are owned by the Global user until real accounts exist.
package study

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"cetus-marketdata-scanner/internal/user"
)

// Visibility is who may see a study (the ugo axis, read-only: no execute bit).
type Visibility string

const (
	VisPrivate Visibility = "private" // owner only
	VisGroup   Visibility = "group"   // owner + members of Group
	VisPublic  Visibility = "public"  // everyone (subject to tier)
)

// Study is a saved screen.
type Study struct {
	Owner      string     `json:"owner"`
	Visibility Visibility `json:"visibility,omitempty"` // default: public if global-owned, else private
	Group      string     `json:"group,omitempty"`      // group id when Visibility == group
	Tier       user.Tier  `json:"tier,omitempty"`       // min tier to access (default free)
	Key        string     `json:"key"`
	Title      string     `json:"title"`
	Emoji      string     `json:"emoji,omitempty"`
	Where      string     `json:"where"`              // SQL WHERE body over the snapshot table
	OrderBy    string     `json:"order_by,omitempty"` // SQL ORDER BY body (ranking)
	Limit      int        `json:"limit,omitempty"`
}

// LoadJSONL reads studies (one JSON object per line; blank lines and #-comments
// skipped). A missing owner defaults to the Global user.
func LoadJSONL(r io.Reader) ([]Study, error) {
	var out []Study
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	n := 0
	for sc.Scan() {
		n++
		t := strings.TrimSpace(sc.Text())
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		var s Study
		if err := json.Unmarshal([]byte(t), &s); err != nil {
			return nil, fmt.Errorf("studies line %d: %w", n, err)
		}
		if s.Owner == "" {
			s.Owner = user.GlobalID
		}
		if s.Tier == "" {
			s.Tier = user.TierFree
		}
		if s.Visibility == "" { // legacy rows: global studies are public, user studies private
			if s.Owner == user.GlobalID {
				s.Visibility = VisPublic
			} else {
				s.Visibility = VisPrivate
			}
		}
		out = append(out, s)
	}
	return out, sc.Err()
}

// LoadFile loads studies from a JSONL path.
func LoadFile(path string) ([]Study, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LoadJSONL(f)
}

// Accessible returns the studies a user may run. Three independent axes:
//   - visibility (private | group | public): who may see it
//   - tier (free | pro): the subscription entitlement gate
//   - role (admin): sees everything
//
// You always see your own studies (no tier gate on those). For others' studies you
// must be able to see them (visibility) AND have the tier.
func Accessible(studies []Study, u user.User) []Study {
	var out []Study
	for _, s := range studies {
		switch {
		case u.IsAdmin(): // admin sees everything
		case s.Owner == u.ID: // own studies, unconditionally
		case !visibleTo(s, u): // hidden by visibility/group
			continue
		case s.Tier.Rank() > u.Tier.Rank(): // tier gate (others' studies only)
			continue
		}
		out = append(out, s)
	}
	return out
}

// visibleTo reports whether a study (not owned by u) is visible to u by its
// visibility scope.
func visibleTo(s Study, u user.User) bool {
	switch s.Visibility {
	case VisPublic:
		return true
	case VisGroup:
		return u.InGroup(s.Group)
	case VisPrivate:
		return false
	default: // unset: global-owned is public, everything else private
		return s.Owner == user.GlobalID
	}
}

// ValidateClause rejects SQL fragments (WHERE / ORDER BY) with dangerous constructs,
// for UNTRUSTED (non-admin) authors. A denylist over an already-narrow surface — a
// read-only SELECT over the ephemeral, single-table, no-sensitive-data snapshot —
// not a full parser. Admins bypass it.
func ValidateClause(clause string) error {
	if strings.Contains(clause, ";") {
		return fmt.Errorf("';' is not allowed")
	}
	lower := strings.ToLower(clause)
	for _, bad := range []string{"--", "/*", "*/", "attach", "pragma", "load_extension", "union", "select"} {
		if strings.Contains(lower, bad) {
			return fmt.Errorf("%q is not allowed here", bad)
		}
	}
	return nil
}
