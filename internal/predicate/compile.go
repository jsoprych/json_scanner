package predicate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// CompilerVersion is folded into the study hash so a change in how predicates
// compile invalidates cached results without any per-study migration.
const CompilerVersion = "v1"

// MaxRules is a structural safety cap; per-tier limits (e.g. 8 for free beta) are
// smaller and enforced by the caller before Compile.
const MaxRules = 16

// MaxNameLen bounds the study name.
const MaxNameLen = 120

// Rule is exactly one comparison predicate, expressed as three opaque IDs.
type Rule struct {
	LeftID     FeatureID  `json:"leftId"`
	OperatorID OperatorID `json:"operatorId"`
	RightID    OperandID  `json:"rightId"`
}

// Sort names an approved sortable feature and a direction.
type Sort struct {
	FeatureID FeatureID `json:"featureId"`
	Direction string    `json:"direction"` // "asc" | "desc"
}

// Definition is the entire client-supplied study — IDs only, never SQL.
type Definition struct {
	Version    int         `json:"version"`
	Name       string      `json:"name"`
	UniverseID string      `json:"universeId"`
	Rules      []Rule      `json:"rules"`
	Connectors []Connector `json:"connectors"`
	Sort       Sort        `json:"sort"`
}

// Compiled is the mechanical output: a bare WHERE expression and ORDER BY clause
// (both drop straight into snapshot.Run / study.Study), the full canonical SELECT
// for inspection, and a content hash for result caching.
type Compiled struct {
	Where     string `json:"where"`   // bare predicate expr, no leading "WHERE "
	OrderBy   string `json:"orderBy"` // bare, no leading "ORDER BY "
	SelectSQL string `json:"selectSql"`
	Hash      string `json:"hash"`
}

// Compile validates a Definition against the server registry and emits the SQL.
// Every returned token traces to a registry entry; no request string is ever
// concatenated into SQL. limit is chosen by the server (tier), never the client.
func Compile(def Definition, limit int) (Compiled, error) {
	if def.Version != 1 {
		return Compiled{}, fmt.Errorf("unsupported study version %d", def.Version)
	}
	if len(def.Name) > MaxNameLen {
		return Compiled{}, fmt.Errorf("study name too long (%d > %d)", len(def.Name), MaxNameLen)
	}
	n := len(def.Rules)
	if n == 0 {
		return Compiled{}, fmt.Errorf("study needs at least one rule")
	}
	if n > MaxRules {
		return Compiled{}, fmt.Errorf("too many rules (%d > %d)", n, MaxRules)
	}
	if len(def.Connectors) != n-1 {
		return Compiled{}, fmt.Errorf("connectors must equal rules-1 (have %d, want %d)", len(def.Connectors), n-1)
	}

	predicates := make([]string, n)
	for i, r := range def.Rules {
		p, err := compilePredicate(r)
		if err != nil {
			return Compiled{}, fmt.Errorf("rule %d: %w", i+1, err)
		}
		predicates[i] = p
	}

	// Strict top-to-bottom evaluation with explicit parentheses, so mixed AND/OR
	// is unambiguous regardless of SQL's native precedence (design §6).
	expr := predicates[0]
	for i := 1; i < n; i++ {
		conn, err := compileConnector(def.Connectors[i-1])
		if err != nil {
			return Compiled{}, err
		}
		expr = "(" + expr + " " + conn + " " + predicates[i] + ")"
	}

	order, err := compileSort(def.Sort)
	if err != nil {
		return Compiled{}, err
	}

	sel := fmt.Sprintf("SELECT symbol FROM snapshot WHERE %s ORDER BY %s LIMIT %d", expr, order, limit)
	return Compiled{Where: expr, OrderBy: order, SelectSQL: sel, Hash: hash(def.UniverseID, sel)}, nil
}

// compilePredicate turns one validated rule into one SQL predicate. Scalar
// operators emit a single comparison; cross operators emit a self-parenthesized
// two-clause template (value on the correct side this bar, wrong side last bar).
func compilePredicate(r Rule) (string, error) {
	f, ok := featureByID[r.LeftID]
	if !ok {
		return "", fmt.Errorf("unknown feature %q", r.LeftID)
	}
	op, ok := operatorByID[r.OperatorID]
	if !ok {
		return "", fmt.Errorf("unknown operator %q", r.OperatorID)
	}
	o, ok := operandByID[r.RightID]
	if !ok {
		return "", fmt.Errorf("unknown operand %q", r.RightID)
	}
	if !allowed(f.ID, op.ID, o.ID) {
		return "", fmt.Errorf("incompatible: %s %s %s", f.ID, op.ID, o.ID)
	}

	if op.Cross || op.Down {
		if f.PrevExpr == "" || o.PrevExpr == "" {
			return "", fmt.Errorf("cross needs prior-bar values for %s and %s", f.ID, o.ID)
		}
		now, prev := ">", "<=" // up-cross: below-or-equal last bar, above now
		if op.Down {
			now, prev = "<", ">="
		}
		return fmt.Sprintf("(%s %s %s AND %s %s %s)",
			f.SQLExpr, now, o.SQLExpr, f.PrevExpr, prev, o.PrevExpr), nil
	}
	return f.SQLExpr + " " + op.SQL + " " + o.SQLExpr, nil
}

func compileConnector(c Connector) (string, error) {
	switch c {
	case And:
		return "AND", nil
	case Or:
		return "OR", nil
	default:
		return "", fmt.Errorf("unknown connector %q", c)
	}
}

func compileSort(s Sort) (string, error) {
	f, ok := featureByID[s.FeatureID]
	if !ok || !f.Sortable {
		return "", fmt.Errorf("feature %q is not an approved sort field", s.FeatureID)
	}
	switch strings.ToLower(s.Direction) {
	case "asc":
		return f.SQLExpr + " ASC", nil
	case "desc":
		return f.SQLExpr + " DESC", nil
	default:
		return "", fmt.Errorf("sort direction must be asc or desc, got %q", s.Direction)
	}
}

// hash is the Phase-1 result-cache key input: compiler version + universe + the
// exact generated SQL. No canonicalization — A AND B and B AND A may hash
// differently, which is acceptable for the MVP (design §12).
func hash(universe, sql string) string {
	sum := sha256.Sum256([]byte("compiler:" + CompilerVersion + "\nuniverse:" + universe + "\n" + sql))
	return hex.EncodeToString(sum[:])
}
