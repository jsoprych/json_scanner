// Package predicate is the server-owned registry + mechanical SQL compiler for
// the structured study editor (see docs/ELKO_SCANNER_STUDY_EDITOR_MVP_DESIGN.md).
//
// First principle: a study is a finite sequence of pre-approved comparison
// predicates over pre-existing snapshot columns. The browser only ever sends
// opaque IDs (feature / operator / operand). Every SQL token — column name,
// comparison operator, numeric literal — originates HERE, in compiled server
// code, never from a request string. That makes the compiler structurally
// injection-proof: an unknown ID is rejected before any SQL is built.
//
// The registry is grounded in the columns the snapshot actually materializes
// (internal/snapshot); adding a feature means adding a real column first.
package predicate

// Typed IDs keep the three axes from being accidentally interchanged.
type (
	FeatureID  string
	OperatorID string
	OperandID  string
)

// Connector joins two adjacent rules. Only these two values are legal.
type Connector string

const (
	And Connector = "and"
	Or  Connector = "or"
)

// Feature is a left-hand column a user compares against. SQLExpr is the snapshot
// column; PrevExpr is the prior-bar column used by cross operators ("" = the
// feature has no prev column, so crossings against it are impossible).
type Feature struct {
	ID       FeatureID
	Label    string
	Category string
	SQLExpr  string
	PrevExpr string
	Sortable bool
}

// Operator is a comparison. Scalar operators carry an SQL symbol (>, >=, …).
// Cross operators carry no symbol; the compiler emits a fixed two-clause
// template (value crosses the threshold this bar), so one editor row can expand
// to more than one SQL comparison while the user still constructs no SQL.
type Operator struct {
	ID    OperatorID
	Label string
	SQL   string // scalar comparison symbol; "" for cross operators
	Cross bool   // true → up-cross (asc), paired with Down below
	Down  bool   // true → down-cross
}

// Operand is a right-hand value: either another column (Column=true, has a
// PrevExpr so it can appear on the right of a cross) or a fixed numeric literal.
// For a literal, PrevExpr == SQLExpr (a constant does not change bar-to-bar), so
// "RSI crossed above 30" is well-defined.
type Operand struct {
	ID       OperandID
	Label    string
	Category string
	SQLExpr  string
	PrevExpr string
	Column   bool
}

// scalar comparison operators.
var (
	OpAbove     = Operator{ID: "above", Label: "Is Above", SQL: ">"}
	OpAtOrAbove = Operator{ID: "at_or_above", Label: "Is At or Above", SQL: ">="}
	OpBelow     = Operator{ID: "below", Label: "Is Below", SQL: "<"}
	OpAtOrBelow = Operator{ID: "at_or_below", Label: "Is At or Below", SQL: "<="}
	OpCrossUp   = Operator{ID: "crossed_above", Label: "Crossed Above", Cross: true}
	OpCrossDown = Operator{ID: "crossed_below", Label: "Crossed Below", Down: true}
)

// Operators is the full closed operator vocabulary.
var Operators = []Operator{OpAbove, OpAtOrAbove, OpBelow, OpAtOrBelow, OpCrossUp, OpCrossDown}

// scalarOps / crossOps name the common operator groups used in the compatibility
// matrix so a feature can opt into a whole comparison family at once.
var (
	scalarOps = []OperatorID{OpAbove.ID, OpAtOrAbove.ID, OpBelow.ID, OpAtOrBelow.ID}
	crossOps  = []OperatorID{OpCrossUp.ID, OpCrossDown.ID}
)

// --- Features (left side), grounded in real snapshot columns ---

var Features = []Feature{
	{ID: "close", Label: "Close", Category: "Price", SQLExpr: "close", PrevExpr: "prev_close", Sortable: true},
	{ID: "high", Label: "Day High", Category: "Price", SQLExpr: "high"},
	{ID: "low", Label: "Day Low", Category: "Price", SQLExpr: "low"},
	{ID: "sma_50", Label: "SMA(50)", Category: "Trend", SQLExpr: "sma50", PrevExpr: "prev_sma50", Sortable: true},
	{ID: "sma_200", Label: "SMA(200)", Category: "Trend", SQLExpr: "sma200", PrevExpr: "prev_sma200", Sortable: true},
	{ID: "rsi_14", Label: "RSI(14)", Category: "Momentum", SQLExpr: "rsi14", PrevExpr: "prev_rsi14", Sortable: true},
	{ID: "ret_3m", Label: "3-Month Return", Category: "Performance", SQLExpr: "ret_3m", Sortable: true},
	{ID: "dollar_vol", Label: "Dollar Volume", Category: "Liquidity", SQLExpr: "dollar_vol", Sortable: true},
	{ID: "high_52w", Label: "52-Week High", Category: "Highs & Lows", SQLExpr: "high_52w"},
	{ID: "low_52w", Label: "52-Week Low", Category: "Highs & Lows", SQLExpr: "low_52w"},
}

// --- Operands (right side): columns first, then finite literal presets ---

// column operands (comparing one feature against another).
var operandColumns = []Operand{
	{ID: "o_close", Label: "Close", Category: "Price", SQLExpr: "close", PrevExpr: "prev_close", Column: true},
	{ID: "o_sma_50", Label: "SMA(50)", Category: "Trend", SQLExpr: "sma50", PrevExpr: "prev_sma50", Column: true},
	{ID: "o_sma_200", Label: "SMA(200)", Category: "Trend", SQLExpr: "sma200", PrevExpr: "prev_sma200", Column: true},
	{ID: "o_high_52w", Label: "52-Week High", Category: "Highs & Lows", SQLExpr: "high_52w", PrevExpr: "high_52w", Column: true},
	{ID: "o_low_52w", Label: "52-Week Low", Category: "Highs & Lows", SQLExpr: "low_52w", PrevExpr: "low_52w", Column: true},
}

// preset numeric operands: id → label → literal SQL. Kept finite so the right
// side is a dropdown, never free-form entry.
type preset struct {
	id, label, sql, category string
}

var (
	rsiLevels = []preset{
		{"rsi_20", "20", "20", "RSI Level"}, {"rsi_25", "25", "25", "RSI Level"},
		{"rsi_30", "30", "30", "RSI Level"}, {"rsi_35", "35", "35", "RSI Level"},
		{"rsi_40", "40", "40", "RSI Level"}, {"rsi_50", "50", "50", "RSI Level"},
		{"rsi_60", "60", "60", "RSI Level"}, {"rsi_65", "65", "65", "RSI Level"},
		{"rsi_70", "70", "70", "RSI Level"}, {"rsi_75", "75", "75", "RSI Level"},
		{"rsi_80", "80", "80", "RSI Level"},
	}
	priceLevels = []preset{
		{"price_1", "$1", "1", "Price"}, {"price_2", "$2", "2", "Price"},
		{"price_5", "$5", "5", "Price"}, {"price_10", "$10", "10", "Price"},
		{"price_20", "$20", "20", "Price"}, {"price_50", "$50", "50", "Price"},
		{"price_100", "$100", "100", "Price"}, {"price_200", "$200", "200", "Price"},
		{"price_500", "$500", "500", "Price"},
	}
	dollarVolLevels = []preset{
		{"dv_100k", "$100K", "100000", "Dollar Volume"}, {"dv_500k", "$500K", "500000", "Dollar Volume"},
		{"dv_1m", "$1M", "1000000", "Dollar Volume"}, {"dv_5m", "$5M", "5000000", "Dollar Volume"},
		{"dv_10m", "$10M", "10000000", "Dollar Volume"}, {"dv_25m", "$25M", "25000000", "Dollar Volume"},
		{"dv_50m", "$50M", "50000000", "Dollar Volume"}, {"dv_100m", "$100M", "100000000", "Dollar Volume"},
		{"dv_250m", "$250M", "250000000", "Dollar Volume"}, {"dv_500m", "$500M", "500000000", "Dollar Volume"},
		{"dv_1b", "$1B", "1000000000", "Dollar Volume"},
	}
	// return levels are fractions: ret_3m of 0.10 == +10%.
	pctLevels = []preset{
		{"pct_neg50", "-50%", "-0.5", "Return"}, {"pct_neg25", "-25%", "-0.25", "Return"},
		{"pct_neg20", "-20%", "-0.2", "Return"}, {"pct_neg10", "-10%", "-0.1", "Return"},
		{"pct_neg5", "-5%", "-0.05", "Return"}, {"pct_0", "0%", "0", "Return"},
		{"pct_5", "+5%", "0.05", "Return"}, {"pct_10", "+10%", "0.1", "Return"},
		{"pct_20", "+20%", "0.2", "Return"}, {"pct_25", "+25%", "0.25", "Return"},
		{"pct_50", "+50%", "0.5", "Return"}, {"pct_100", "+100%", "1.0", "Return"},
	}
)

// Operands is the full closed operand vocabulary (columns + all literal presets).
var Operands = buildOperands()

func buildOperands() []Operand {
	out := append([]Operand(nil), operandColumns...)
	for _, group := range [][]preset{rsiLevels, priceLevels, dollarVolLevels, pctLevels} {
		for _, p := range group {
			out = append(out, Operand{ID: OperandID(p.id), Label: p.label, Category: p.category, SQLExpr: p.sql, PrevExpr: p.sql})
		}
	}
	return out
}

// --- Compatibility matrix: feature → operator → allowed right operands ---
//
// This is the heart of "invalid predicates are impossible to construct". The UI
// greys out anything not listed here; the server re-checks it on every request.

// compat is a fluent builder for the matrix.
type compat map[FeatureID]map[OperatorID]map[OperandID]struct{}

func ids(ps ...[]preset) []OperandID {
	var out []OperandID
	for _, g := range ps {
		for _, p := range g {
			out = append(out, OperandID(p.id))
		}
	}
	return out
}

func (c compat) allow(f FeatureID, ops []OperatorID, operands ...OperandID) compat {
	if c[f] == nil {
		c[f] = map[OperatorID]map[OperandID]struct{}{}
	}
	for _, op := range ops {
		if c[f][op] == nil {
			c[f][op] = map[OperandID]struct{}{}
		}
		for _, o := range operands {
			c[f][op][o] = struct{}{}
		}
	}
	return c
}

// Compatibility is the authoritative legal-combination set.
var Compatibility = func() compat {
	c := compat{}
	priceOperands := append([]OperandID{"o_sma_50", "o_sma_200", "o_high_52w", "o_low_52w"}, ids(priceLevels)...)
	// close: compare / cross against trend lines, 52w extremes, and price levels.
	c.allow("close", scalarOps, priceOperands...)
	c.allow("close", crossOps, "o_sma_50", "o_sma_200", "o_high_52w", "o_low_52w")
	// day high / low against price levels and 52w extremes.
	c.allow("high", scalarOps, append([]OperandID{"o_high_52w"}, ids(priceLevels)...)...)
	c.allow("low", scalarOps, append([]OperandID{"o_low_52w"}, ids(priceLevels)...)...)
	// moving averages against each other (golden/death cross) and price levels.
	c.allow("sma_50", scalarOps, append([]OperandID{"o_sma_200", "o_close"}, ids(priceLevels)...)...)
	c.allow("sma_50", crossOps, "o_sma_200")
	c.allow("sma_200", scalarOps, append([]OperandID{"o_sma_50", "o_close"}, ids(priceLevels)...)...)
	// RSI against its levels, scalar and cross (constant threshold ⇒ cross valid).
	c.allow("rsi_14", scalarOps, ids(rsiLevels)...)
	c.allow("rsi_14", crossOps, ids(rsiLevels)...)
	// 3-month return against percentage levels.
	c.allow("ret_3m", scalarOps, ids(pctLevels)...)
	// dollar volume against liquidity thresholds.
	c.allow("dollar_vol", scalarOps, ids(dollarVolLevels)...)
	// close vs 52-week extremes already covered above; extremes rarely a left side.
	return c
}()

// --- lookup indices (built once) ---

var (
	featureByID  = index(Features, func(f Feature) FeatureID { return f.ID })
	operatorByID = index(Operators, func(o Operator) OperatorID { return o.ID })
	operandByID  = index(Operands, func(o Operand) OperandID { return o.ID })
)

func index[T any, K comparable](items []T, key func(T) K) map[K]T {
	m := make(map[K]T, len(items))
	for _, it := range items {
		m[key(it)] = it
	}
	return m
}

// allowed reports whether (feature, operator, operand) is a legal combination.
func allowed(f FeatureID, op OperatorID, o OperandID) bool {
	ops, ok := Compatibility[f]
	if !ok {
		return false
	}
	set, ok := ops[op]
	if !ok {
		return false
	}
	_, ok = set[o]
	return ok
}
