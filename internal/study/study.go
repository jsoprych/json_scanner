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

// Study is a saved screen.
type Study struct {
	Owner   string    `json:"owner"`
	Tier    user.Tier `json:"tier,omitempty"` // min tier to access (default free)
	Key     string    `json:"key"`
	Title   string    `json:"title"`
	Emoji   string    `json:"emoji,omitempty"`
	Where   string    `json:"where"`              // SQL WHERE body over the snapshot table
	OrderBy string    `json:"order_by,omitempty"` // SQL ORDER BY body (ranking)
	Limit   int       `json:"limit,omitempty"`
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

// Accessible returns the studies a user may run: owned by them or by Global, and at
// a tier the user's subscription unlocks. This is the monetization gate — free
// users see free studies; pro studies require pro.
func Accessible(studies []Study, u user.User) []Study {
	var out []Study
	for _, s := range studies {
		if s.Owner != user.GlobalID && s.Owner != u.ID {
			continue
		}
		if s.Tier.Rank() > u.Tier.Rank() {
			continue
		}
		out = append(out, s)
	}
	return out
}
