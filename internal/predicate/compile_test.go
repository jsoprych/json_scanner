package predicate

import (
	"strings"
	"testing"
)

// def is a small builder for test studies.
func def(sort Sort, conns []Connector, rules ...Rule) Definition {
	return Definition{Version: 1, Name: "t", UniverseID: "r3000", Rules: rules, Connectors: conns, Sort: sort}
}

func rule(l FeatureID, op OperatorID, r OperandID) Rule { return Rule{l, op, r} }

var sortRet = Sort{FeatureID: "ret_3m", Direction: "desc"}

// TestCompileGolden pins exact SQL for representative studies (design §22.3).
func TestCompileGolden(t *testing.T) {
	cases := []struct {
		name  string
		def   Definition
		where string
		order string
	}{
		{
			name:  "single scalar",
			def:   def(sortRet, nil, rule("close", "above", "o_sma_200")),
			where: "close > sma200",
			order: "ret_3m DESC",
		},
		{
			name:  "two scalars AND",
			def:   def(sortRet, []Connector{And}, rule("close", "above", "o_sma_200"), rule("rsi_14", "below", "rsi_35")),
			where: "(close > sma200 AND rsi14 < 35)",
			order: "ret_3m DESC",
		},
		{
			name: "mixed AND then OR — left-to-right parens",
			def: def(sortRet, []Connector{And, Or},
				rule("close", "above", "o_sma_200"),
				rule("rsi_14", "below", "rsi_35"),
				rule("ret_3m", "above", "pct_10")),
			where: "((close > sma200 AND rsi14 < 35) OR ret_3m > 0.1)",
			order: "ret_3m DESC",
		},
		{
			name:  "close crosses above SMA200 — two-clause template",
			def:   def(sortRet, nil, rule("close", "crossed_above", "o_sma_200")),
			where: "(close > sma200 AND prev_close <= prev_sma200)",
			order: "ret_3m DESC",
		},
		{
			name:  "golden cross SMA50 x SMA200",
			def:   def(Sort{FeatureID: "close", Direction: "asc"}, nil, rule("sma_50", "crossed_above", "o_sma_200")),
			where: "(sma50 > sma200 AND prev_sma50 <= prev_sma200)",
			order: "close ASC",
		},
		{
			name:  "RSI crosses above 30 — constant threshold cross",
			def:   def(sortRet, nil, rule("rsi_14", "crossed_above", "rsi_30")),
			where: "(rsi14 > 30 AND prev_rsi14 <= 30)",
			order: "ret_3m DESC",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Compile(tc.def, 26)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if got.Where != tc.where {
				t.Errorf("WHERE\n got: %s\nwant: %s", got.Where, tc.where)
			}
			if got.OrderBy != tc.order {
				t.Errorf("ORDER BY got %q want %q", got.OrderBy, tc.order)
			}
			if !strings.Contains(got.SelectSQL, "LIMIT 26") {
				t.Errorf("SelectSQL missing limit: %s", got.SelectSQL)
			}
			if len(got.Hash) != 64 {
				t.Errorf("hash not sha256 hex: %q", got.Hash)
			}
		})
	}
}

// TestHashDeterministicAndDistinct: same study → same hash; different order of
// the same rules → different hash (acceptable per §12), and limit affects hash.
func TestHashDeterministicAndDistinct(t *testing.T) {
	ab := def(sortRet, []Connector{And}, rule("close", "above", "o_sma_200"), rule("rsi_14", "below", "rsi_35"))
	ba := def(sortRet, []Connector{And}, rule("rsi_14", "below", "rsi_35"), rule("close", "above", "o_sma_200"))
	h1, _ := Compile(ab, 26)
	h1b, _ := Compile(ab, 26)
	h2, _ := Compile(ba, 26)
	h3, _ := Compile(ab, 51)
	if h1.Hash != h1b.Hash {
		t.Error("same study hashed differently")
	}
	if h1.Hash == h2.Hash {
		t.Error("A AND B and B AND A unexpectedly share a hash")
	}
	if h1.Hash == h3.Hash {
		t.Error("limit did not affect hash")
	}
}

// TestReject covers the validation matrix (design §21, §22.2).
func TestReject(t *testing.T) {
	cases := []struct {
		name string
		def  Definition
	}{
		{"bad version", Definition{Version: 2, Rules: []Rule{rule("close", "above", "o_sma_200")}, Sort: sortRet}},
		{"no rules", def(sortRet, nil)},
		{"extra connector", def(sortRet, []Connector{And}, rule("close", "above", "o_sma_200"))},
		{"missing connector", def(sortRet, nil, rule("close", "above", "o_sma_200"), rule("rsi_14", "below", "rsi_35"))},
		{"unknown feature", def(sortRet, nil, rule("evil", "above", "o_sma_200"))},
		{"unknown operator", def(sortRet, nil, rule("close", "haxor", "o_sma_200"))},
		{"unknown operand", def(sortRet, nil, rule("close", "above", "evil"))},
		{"incompatible combo", def(sortRet, nil, rule("rsi_14", "above", "o_sma_200"))},
		{"unknown connector", def(sortRet, []Connector{"nand"}, rule("close", "above", "o_sma_200"), rule("rsi_14", "below", "rsi_35"))},
		{"unsortable sort field", def(Sort{FeatureID: "high", Direction: "desc"}, nil, rule("close", "above", "o_sma_200"))},
		{"bad sort direction", def(Sort{FeatureID: "ret_3m", Direction: "sideways"}, nil, rule("close", "above", "o_sma_200"))},
		{"cross against non-cross operand", def(sortRet, nil, rule("ret_3m", "crossed_above", "pct_10"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Compile(tc.def, 26); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

// TestInjectionIDsNeverReachSQL: hostile IDs (design §22.4) are rejected before
// any SQL is produced — the compiler returns an error and an empty result.
func TestInjectionIDsNeverReachSQL(t *testing.T) {
	evil := []Rule{
		{LeftID: "close; DROP TABLE users", OperatorID: "above", RightID: "o_sma_200"},
		{LeftID: "close", OperatorID: "OR 1=1 --", RightID: "o_sma_200"},
		{LeftID: "close", OperatorID: "above", RightID: "UNION SELECT"},
		{LeftID: "close", OperatorID: "above", RightID: "') OR '1'='1"},
	}
	for _, r := range evil {
		c, err := Compile(def(sortRet, nil, r), 26)
		if err == nil {
			t.Fatalf("hostile rule compiled: %+v", r)
		}
		if c.Where != "" || c.SelectSQL != "" {
			t.Fatalf("hostile rule leaked SQL: %+v", c)
		}
	}
}

// TestEverySQLTokenTraceable: for any successfully compiled study, every token in
// the WHERE clause is traceable to a registry entry — no request string appears.
// (design §22.5 fuzzing invariant, checked over the whole compatibility matrix.)
func TestEverySQLTokenTraceable(t *testing.T) {
	// Collect the closed set of legal SQL tokens the compiler may emit.
	legal := map[string]bool{"(": true, ")": true, "AND": true, "OR": true,
		">": true, ">=": true, "<": true, "<=": true}
	for _, f := range Features {
		legal[f.SQLExpr] = true
		if f.PrevExpr != "" {
			legal[f.PrevExpr] = true
		}
	}
	for _, o := range Operands {
		legal[o.SQLExpr] = true
	}

	for f, ops := range Compatibility {
		for op, operands := range ops {
			for o := range operands {
				c, err := Compile(def(sortRet, nil, rule(f, op, o)), 26)
				if err != nil {
					t.Fatalf("legal combo failed to compile: %s %s %s: %v", f, op, o, err)
				}
				// Tokenize the WHERE clause by splitting on spaces after peeling parens.
				for _, tok := range strings.Fields(strings.NewReplacer("(", " ", ")", " ").Replace(c.Where)) {
					if !legal[tok] {
						t.Errorf("untraceable SQL token %q from %s %s %s", tok, f, op, o)
					}
				}
			}
		}
	}
}

// TestCatalogHasNoSQL: the browser catalog must expose zero snapshot column names
// or SQL fragments (design §8) — only IDs and labels.
func TestCatalogHasNoSQL(t *testing.T) {
	cat := BuildCatalog()
	// Every SQL column expression must be absent from any catalog label.
	var sqlTokens []string
	for _, f := range Features {
		sqlTokens = append(sqlTokens, f.SQLExpr)
	}
	for _, cf := range cat.Features {
		for _, tok := range sqlTokens {
			if cf.Label == tok {
				t.Errorf("feature label %q leaks SQL column name", cf.Label)
			}
		}
	}
	if len(cat.Features) == 0 || len(cat.Operators) == 0 || len(cat.Operands) == 0 {
		t.Fatal("catalog is empty")
	}
	// Compatibility in the catalog must only reference known IDs.
	for f, ops := range cat.Compatibility {
		if _, ok := featureByID[f]; !ok {
			t.Errorf("catalog references unknown feature %q", f)
		}
		for op, operands := range ops {
			if _, ok := operatorByID[op]; !ok {
				t.Errorf("catalog references unknown operator %q", op)
			}
			for _, o := range operands {
				if _, ok := operandByID[o]; !ok {
					t.Errorf("catalog references unknown operand %q", o)
				}
			}
		}
	}
}
