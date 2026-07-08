# Elko Scanner Study Editor — MVP Design Specification

**Status:** Phase 1 MVP / Beta

**Audience:** AI coding agents, frontend developers, Go backend developers, reviewers

**Primary implementation target:** Framework-independent TypeScript Web Component backed by a Go/SQLite scanner service

---

# 1. First Principle

> **This is not a general rule engine, formula language, SQL editor, or DSL.**
> 
> The MVP Study Editor exists for one narrow purpose:
> 
> **Build a scanner SQL `WHERE` clause by concatenating a finite sequence of pre-approved comparison predicates over pre-existing scanner snapshot fields/features.**

The transformation must remain obvious and mechanical:

```text
USER-FRIENDLY EDITOR ROWS
        ↓
FINITE INTERNAL IDS
        ↓
SERVER VALIDATION
        ↓
SERVER-OWNED TOKEN LOOKUP
        ↓
PREDICATE 1 + AND/OR + PREDICATE 2 + ...
        ↓
SQL WHERE CLAUSE
```

Example editor:

```text
[ Close ▼ ]      [ Is Above ▼ ]       [ SMA(200) ▼ ]

              [ AND ▼ ]

[ RSI(14) ▼ ]    [ Is Below ▼ ]       [ 35 ▼ ]

              [ AND ▼ ]

[ Relative Volume(20) ▼ ] [ Is Above ▼ ] [ 2.0x ▼ ]
```

Required scanner transformation:

```sql
WHERE close > sma200
  AND rsi14 < 35
  AND rel_volume20 > 2.0
```

The MVP architecture should preserve this direct relationship.

---

# 2. Explicit Non-Goals

The Phase 1 MVP MUST NOT support:

- arbitrary SQL;
- arbitrary field names;
- arbitrary indicator names;
- arbitrary function calls;
- arbitrary text expressions;
- arbitrary numeric entry;
- user-defined variables;
- subqueries;
- joins;
- formulas;
- nested logical groups;
- parentheses entered by the user;
- custom indicator periods;
- custom code;
- a general expression AST intended to become a programming language.

Do not over-engineer this component into a general rules framework.

The MVP is intentionally:

```text
comparison predicate
AND/OR
comparison predicate
AND/OR
comparison predicate
```

---

# 3. Product Goal

Create a reusable, user-friendly scan study editor that:

1. makes invalid predicates impossible to construct through the UI;
2. sends only finite internal IDs to the server;
3. allows the server to validate the complete study independently;
4. allows the Go backend to generate the scanner `WHERE` clause by simple token lookup and concatenation;
5. produces deterministic machine-generated SQL suitable for hashing and result-cache reuse;
6. can be embedded anywhere as a reusable component.

Recommended public element:

```html
<elko-study-editor></elko-study-editor>
```

---

# 4. Core MVP Data Model

A study contains:

```ts
interface StudyDefinition {
  version: 1;
  name: string;
  universeId: UniverseId;
  rules: StudyRule[];
  connectors: LogicalConnector[];
  sort: SortDefinition;
}
```

A rule is exactly one comparison predicate:

```ts
interface StudyRule {
  leftId: FeatureId;
  operatorId: OperatorId;
  rightId: OperandId;
}
```

Connectors appear between adjacent rules:

```ts
type LogicalConnector = "and" | "or";
```

Invariant:

```text
connectors.length === max(rules.length - 1, 0)
```

Sort definition:

```ts
interface SortDefinition {
  featureId: SortableFeatureId;
  direction: "asc" | "desc";
}
```

Phase 1 result limit is server-controlled and is NOT supplied by the client.

---

# 5. Exact 1:1 Mapping to SQL WHERE

Each complete rule maps to one server-owned SQL predicate template.

Example internal rule:

```json
{
  "leftId": "close",
  "operatorId": "above",
  "rightId": "sma_200"
}
```

Server registries resolve:

```text
leftId      close      → close
operatorId  above      → >
rightId     sma_200    → sma200
```

Generated predicate:

```sql
close > sma200
```

Another rule:

```json
{
  "leftId": "rsi_14",
  "operatorId": "below",
  "rightId": "rsi_level_35"
}
```

Server registries resolve:

```text
rsi_14       → rsi14
below        → <
rsi_level_35 → 35
```

Generated predicate:

```sql
rsi14 < 35
```

The complete study:

```text
P1 AND P2
```

becomes:

```sql
WHERE close > sma200
  AND rsi14 < 35
```

No user-controlled string is inserted into SQL.

---

# 6. Mixed AND / OR Semantics

The MVP MAY allow `AND` and `OR` between rows.

Because raw SQL gives `AND` higher precedence than `OR`, the compiler MUST NOT simply emit an ambiguous unparenthesized expression when connector types are mixed.

The editor uses strict top-to-bottom evaluation.

Example editor:

```text
P1
AND
P2
OR
P3
```

MVP meaning:

```text
((P1 AND P2) OR P3)
```

Generated SQL:

```sql
WHERE ((predicate_1 AND predicate_2) OR predicate_3)
```

Compiler algorithm:

```text
expr = predicate[0]

for i from 1 to predicateCount - 1:
    expr = "(" + expr + " " + connector[i-1] + " " + predicate[i] + ")"
```

This keeps the transformation:

- deterministic;
- easy to inspect;
- easy to hash;
- easy for users to understand visually;
- free of user-entered parentheses.

No nested groups are allowed in Phase 1.

---

# 7. Browser UI Behavior

The editor MUST behave like a constrained predicate builder.

## 7.1 Rule row

Each row contains exactly three controls:

```text
[ LEFT FEATURE ▼ ] [ COMPARISON ▼ ] [ RIGHT OPERAND ▼ ]
```

Example:

```text
[ Close ▼ ] [ Is Above ▼ ] [ SMA(200) ▼ ]
```

The UI never exposes SQL identifiers.

## 7.2 Connector row

Between complete predicates:

```text
[ AND ▼ ]
```

Options:

```text
AND
OR
```

## 7.3 Add rule

```text
[ + ADD RULE ]
```

Adding a rule appends:

```text
connector + empty predicate row
```

## 7.4 Remove rule

Each row may expose:

```text
[ × ]
```

Removing a row also repairs the connector array.

## 7.5 Rule completion flow

The controls are sequentially constrained:

```text
choose LEFT
    ↓
load only valid OPERATORS
    ↓
choose OPERATOR
    ↓
load only compatible RIGHT OPERANDS
```

A rule is not valid until all three IDs are present.

---

# 8. UI Constraint Model

The browser receives a harmless scanner catalog from the server.

Recommended endpoint:

```text
GET /api/scanner/catalog
```

The catalog contains labels and legal relationships only.

Example:

```json
{
  "features": [
    {
      "id": "close",
      "label": "Close",
      "category": "Price",
      "operatorIds": [
        "above",
        "at_or_above",
        "below",
        "at_or_below",
        "crossed_above",
        "crossed_below"
      ]
    }
  ],
  "operators": [
    {
      "id": "above",
      "label": "Is Above"
    }
  ],
  "operands": [
    {
      "id": "sma_200",
      "label": "SMA(200)",
      "category": "Trend"
    }
  ],
  "compatibility": {
    "close": {
      "above": ["open", "prev_close", "sma_20", "sma_50", "sma_100", "sma_200"]
    }
  }
}
```

The browser uses this catalog to make invalid combinations unavailable.

The browser catalog MUST NOT contain:

- SQL table names;
- SQL column names;
- SQL fragments;
- SQL expressions;
- database schema details.

---

# 9. Server-Owned Registry

The Go server is the authoritative source of truth.

The registry drives:

1. catalog generation for the browser;
2. server validation;
3. SQL compilation.

Conceptual Go types:

```go
type FeatureID string
type OperatorID string
type OperandID string

type FeatureDefinition struct {
    ID        FeatureID
    Label     string
    Category  string
    SQLExpr   string
    Sortable  bool
}

type OperatorDefinition struct {
    ID    OperatorID
    Label string
    SQL   string
}

type OperandDefinition struct {
    ID      OperandID
    Label   string
    SQLExpr string
}
```

The server also owns compatibility relationships:

```go
type CompatibilityKey struct {
    Left     FeatureID
    Operator OperatorID
}

var AllowedRightOperands map[CompatibilityKey]map[OperandID]struct{}
```

A study is valid only when:

```text
left feature exists
AND
operator exists
AND
right operand exists
AND
(right operand is allowed for left feature + operator)
```

---

# 10. SQL Compiler

The compiler must be deliberately boring.

Recommended API:

```go
type ValidatedRule struct {
    Left     FeatureID
    Operator OperatorID
    Right    OperandID
}

type ValidatedStudy struct {
    Rules      []ValidatedRule
    Connectors []LogicalConnector
    Sort       ValidatedSort
}

type CompiledStudy struct {
    WhereSQL string
    OrderSQL string
    FullSQL  string
    Hash     string
}

func CompileStudy(study ValidatedStudy) (CompiledStudy, error)
```

Pseudo-code:

```go
func compilePredicate(rule ValidatedRule) string {
    left := featureRegistry[rule.Left].SQLExpr
    op := operatorRegistry[rule.Operator].SQL
    right := operandRegistry[rule.Right].SQLExpr

    return left + " " + op + " " + right
}
```

Compile study:

```go
predicates := make([]string, len(study.Rules))

for i, rule := range study.Rules {
    predicates[i] = compilePredicate(rule)
}

expr := predicates[0]

for i := 1; i < len(predicates); i++ {
    connector := compileConnector(study.Connectors[i-1])
    expr = "(" + expr + " " + connector + " " + predicates[i] + ")"
}

whereSQL := "WHERE " + expr
```

Example output:

```sql
WHERE ((close > sma200 AND rsi14 < 35) OR rel_volume20 > 2.0)
```

The sort clause is compiled separately from approved sortable fields:

```sql
ORDER BY ret_63d DESC
```

The server appends the tier-controlled limit:

```sql
LIMIT 26
```

For a Free tier displaying 25 results, `26` allows the server to set:

```text
has_more = true
```

without a second `COUNT(*)` query.

---

# 11. SQL Injection Security Model

The MVP should be structurally resistant to SQL injection.

## 11.1 Client cannot send SQL

The study request contains only known IDs.

## 11.2 Server assumes the client is hostile

A curl request can bypass the UI, so the server validates every ID again.

## 11.3 SQL tokens come only from server-owned registries

Every SQL identifier, operator, literal, and connector originates from compiled server code.

## 11.4 No raw string concatenation from request values

Forbidden pattern:

```go
sql := "WHERE " + request.Left + " " + request.Operator + " " + request.Right
```

Required pattern:

```go
left, ok := featureRegistry[request.LeftID]
if !ok {
    return ErrInvalidFeature
}
```

## 11.5 Closed vocabulary

Unknown IDs are rejected.

Examples:

```text
leftId = "close; DROP TABLE users" → rejected
operatorId = "OR 1=1 --"          → rejected
rightId = "evil"                  → rejected
```

SQLite never receives those strings.

## 11.6 Defense in depth

Scanner execution connections should use:

```sql
PRAGMA query_only = ON;
```

The scanner snapshot connection should not have access to mutable user tables.

---

# 12. Exact Phase 1 Hashing

Do not add canonicalization logic in Phase 1.

Hash the exact deterministic machine-generated query specification.

Recommended input:

```text
compiler_version
+
universe_id
+
full generated SQL
```

Example:

```text
compiler:v1
universe:r3000
SELECT symbol_id FROM snapshot WHERE (close > sma200 AND rsi14 < 35) ORDER BY ret_63d DESC LIMIT 26
```

Then:

```text
study_hash = SHA-256(hash_input)
```

Phase 1 consequences:

```text
A AND B
```

and:

```text
B AND A
```

may produce different hashes.

That is acceptable for the MVP.

Do not add rule sorting or logical equivalence analysis until real usage proves it valuable.

---

# 13. Daily Result Cache Key

The study hash identifies the study definition.

Daily results also depend on the current market snapshot.

Recommended cache key:

```text
run_cache_key = SHA-256(
    study_hash
    + snapshot_id
    + universe_version
)
```

Nightly flow:

```text
active study
    ↓
compute run_cache_key
    ↓
cache hit?
    ├─ yes → reuse result
    └─ no  → execute scanner query
                  ↓
              save result
```

---

# 14. Recommended MVP Feature Set

The Phase 1 editor should target a finite snapshot schema.

Recommended initial set: approximately 50 fields/features.

## 14.1 Price and current session

```text
open
high
low
close
prev_close
change_1d_pct
gap_pct
intraday_range_pct
close_in_day_range_pct
dist_52w_high_pct
```

## 14.2 Returns

```text
ret_5d
ret_20d
ret_63d
ret_126d
ret_252d
```

## 14.3 Trend

```text
sma_20
sma_50
sma_100
sma_200
ema_20
ema_50
ema_200
dist_sma_20_pct
dist_sma_50_pct
dist_sma_100_pct
dist_sma_200_pct
```

## 14.4 Momentum and trend strength

```text
rsi_14
macd_line
macd_signal
macd_histogram
adx_14
plus_di_14
minus_di_14
```

## 14.5 Highs, lows, and range position

```text
high_20d
high_63d
high_252d
low_20d
low_63d
low_252d
range_52w_position_pct
dist_52w_low_pct
```

## 14.6 Volatility and Bollinger Bands

```text
atr_14_pct
bb_upper_20_2
bb_middle_20_2
bb_lower_20_2
bb_bandwidth_pct
```

## 14.7 Liquidity and activity

```text
dollar_volume_today
avg_dollar_volume_20
median_dollar_volume_20
relative_volume_20
```

The exact set may be adjusted during implementation, but the architecture assumes a finite materialized scanner table.

---

# 15. Approved Operators

Recommended Phase 1 operator IDs:

```text
above
at_or_above
below
at_or_below
crossed_above
crossed_below
```

Server mapping:

```text
above          → >
at_or_above    → >=
below          → <
at_or_below    → <=
```

Cross operators require pre-existing previous-value snapshot columns or server-defined cross expressions.

Example:

```text
close crossed above sma_200
```

may compile to:

```sql
close > sma200
AND prev_close <= prev_sma200
```

Important:

A single visible editor row may map to a fixed server-owned predicate template containing more than one SQL comparison.

The 1:1 rule remains:

```text
one editor rule
    ↓
one server-owned predicate template
```

The user still does not construct SQL.

---

# 16. Approved Preset Operands

Phase 1 should prefer finite preset operands over arbitrary number entry.

Examples:

## RSI levels

```text
20
25
30
35
40
50
60
65
70
75
80
```

## Relative volume

```text
0.5x
0.75x
1.0x
1.25x
1.5x
2.0x
3.0x
5.0x
10.0x
```

## Dollar volume

```text
$100K
$500K
$1M
$5M
$10M
$25M
$50M
$100M
$250M
$500M
$1B
```

## Price

```text
$1
$2
$5
$10
$20
$50
$100
$200
$500
```

## Percentage levels

```text
-50%
-25%
-20%
-15%
-10%
-5%
0%
5%
10%
15%
20%
25%
50%
100%
```

These operands are catalog IDs, not arbitrary user strings.

Example:

```text
rsi_level_35 → 35
relvol_2x    → 2.0
adv_5m       → 5000000
```

---

# 17. Web Component Design

Recommended component:

```html
<elko-study-editor></elko-study-editor>
```

Do not extend a built-in element.

Use an autonomous custom element.

Recommended source structure:

```text
web/components/study-editor/
│
├── package.json
├── tsconfig.json
├── src/
│   ├── core/
│   │   ├── types.ts
│   │   ├── model.ts
│   │   ├── validate.ts
│   │   └── catalog.ts
│   │
│   ├── component/
│   │   ├── elko-study-editor.ts
│   │   ├── rule-row.ts
│   │   ├── feature-picker.ts
│   │   └── styles.css
│   │
│   └── index.ts
│
└── tests/
```

The browser component must not know SQL exists.

---

# 18. Component Public API

Recommended properties:

```ts
interface ElkoStudyEditor extends HTMLElement {
  catalog: ScannerCatalog;
  value: StudyDefinition | null;
  readOnly: boolean;
  maxRules: number;
}
```

Recommended events:

```text
study-change
study-validity-change
study-save
study-run
```

Example:

```ts
editor.addEventListener("study-save", (event) => {
  const study = event.detail.study;
  saveStudy(study);
});
```

The component should not own authentication, routing, persistence, or SQL compilation.

---

# 19. Editor Layout

Recommended desktop layout:

```text
┌────────────────────────────────────────────────────────────────────┐
│ CREATE SCAN STUDY                                                  │
│                                                                    │
│ Study Name                                                         │
│ [ Oversold Above Long-Term Trend_______________________________ ]  │
│                                                                    │
│ Universe                                                           │
│ [ Russell 3000 ▼ ]                                                 │
│                                                                    │
│ ─────────────────── RULES ───────────────────────────────────────  │
│                                                                    │
│  1   [ Close ▼ ]   [ Is Above ▼ ]       [ SMA(200) ▼ ]      [×]   │
│                                                                    │
│                      [ AND ▼ ]                                     │
│                                                                    │
│  2   [ RSI(14) ▼ ] [ Is Below ▼ ]       [ 35 ▼ ]            [×]   │
│                                                                    │
│                      [ AND ▼ ]                                     │
│                                                                    │
│  3   [ Relative Volume(20) ▼ ] [ Is Above ▼ ] [ 2.0x ▼ ]   [×]   │
│                                                                    │
│                         [ + ADD RULE ]                              │
│                                                                    │
│ ─────────────────── SORT RESULTS ────────────────────────────────  │
│                                                                    │
│ Sort by [ 3-Month Return ▼ ] [ Highest First ▼ ]                   │
│                                                                    │
│ FREE BETA: Top 25 results                                          │
│                                                                    │
│ ────────────────────────────────────────────────────────────────  │
│                                                                    │
│ Find stocks where:                                                 │
│                                                                    │
│ • Close is above SMA(200)                                          │
│ • AND RSI(14) is below 35                                          │
│ • AND Relative Volume(20) is above 2.0x                            │
│                                                                    │
│                  [ SAVE & RUN STUDY ]                              │
└────────────────────────────────────────────────────────────────────┘
```

The natural-language preview is generated from the same finite catalog labels.

It is display-only and never parsed back into rules.

---

# 20. Feature Picker UX

Do not use one flat 50-item dropdown.

Use a searchable finite picker grouped by category:

```text
Search features...

PRICE & SESSION
    Close
    Open
    Gap %
    Intraday Range %

PERFORMANCE
    1-Week Return
    1-Month Return
    3-Month Return

TREND
    SMA(20)
    SMA(50)
    SMA(200)
    EMA(20)

MOMENTUM
    RSI(14)
    MACD Line
    MACD Signal
    ADX(14)

HIGHS & LOWS
    20-Day High
    52-Week High

VOLATILITY
    ATR(14) %
    Bollinger Upper Band

LIQUIDITY
    Average Dollar Volume(20)
    Relative Volume(20)
```

Search filters only known catalog labels.

It is not a free-form expression input.

---

# 21. Request Validation

Recommended server checks:

```text
1. authentication required to save a custom study
2. strict Content-Type
3. maximum request body size
4. strict JSON decoding
5. unknown JSON fields rejected
6. supported schema version required
7. maximum study-name length
8. maximum rule count enforced by tier
9. connectors count must equal rules count - 1
10. every feature ID must exist
11. every operator ID must exist
12. every operand ID must exist
13. every combination must exist in compatibility matrix
14. every sort feature must be approved and sortable
15. sort direction must be asc or desc
16. universe ID must be approved for user tier
17. result limit is chosen by server, never client
```

Recommended Free Beta limits:

```text
3 saved custom studies
8 rules per study
Top 25 results
EOD execution
```

---

# 22. Required Tests

## 22.1 UI tests

Verify:

- selecting a feature restricts operator choices;
- selecting an operator restricts right operands;
- impossible feature/operator/operand combinations cannot be selected;
- adding a rule adds exactly one connector and one rule;
- removing a rule repairs connectors correctly;
- mixed AND/OR preview matches top-to-bottom semantics;
- serialized output contains IDs only.

## 22.2 Server validation tests

Reject:

```text
unknown feature IDs
unknown operator IDs
unknown operand IDs
invalid compatibility combinations
missing connectors
extra connectors
unknown JSON fields
oversized studies
unsupported universe IDs
unsupported sort fields
```

## 22.3 SQL compiler golden tests

Input:

```text
Close > SMA(200)
AND
RSI(14) < 35
```

Expected exact output:

```sql
WHERE (close > sma200 AND rsi14 < 35)
```

Input:

```text
Close > SMA(200)
AND
RSI(14) < 35
OR
Relative Volume(20) > 2.0x
```

Expected exact output:

```sql
WHERE ((close > sma200 AND rsi14 < 35) OR rel_volume20 > 2.0)
```

## 22.4 Security tests

Send malicious IDs such as:

```text
close; DROP TABLE users
OR 1=1 --
UNION SELECT
PRAGMA
ATTACH
\x00
invalid UTF-8
very long strings
```

Required behavior:

```text
request rejected before SQL compilation
```

## 22.5 Fuzzing invariants

For every arbitrary request payload:

```text
invalid IDs must never appear in generated SQL
```

For every successfully compiled study:

```text
every SQL token must be traceable to a server-owned registry entry
```

---

# 23. Implementation Order

## Step 1 — Freeze snapshot fields

Finalize the materialized scanner snapshot columns.

## Step 2 — Build Go registry

Create:

```text
feature registry
operator registry
operand registry
compatibility matrix
sortable feature registry
```

## Step 3 — Build compiler tests first

Implement exact input → exact SQL golden tests.

## Step 4 — Build catalog endpoint

```text
GET /api/scanner/catalog
```

## Step 5 — Build TypeScript model

Implement:

```text
StudyDefinition
StudyRule
connectors
validation helpers
catalog lookup
```

## Step 6 — Build `<elko-study-editor>`

Implement the reusable component.

## Step 7 — Save/run endpoint

```text
POST /api/studies
```

Flow:

```text
decode
validate
compile
hash
cache lookup
run if needed
save subscription
return result
```

## Step 8 — Nightly reuse

The same saved study definition and compiled query join the overnight batch system.

---

# 24. Acceptance Criteria

The MVP is complete when all of the following are true:

1. A user can construct a study only through finite dropdown choices.
2. The browser sends only schema-versioned IDs.
3. The server rejects any unregistered ID.
4. The server rejects any incompatible predicate combination.
5. The server compiles valid rules into a deterministic `WHERE` clause.
6. Mixed AND/OR rules compile with explicit left-to-right parentheses.
7. No user-supplied string can become part of SQL.
8. The exact compiled query can be hashed.
9. Identical exact machine-generated studies reuse cached results.
10. The component can be embedded as `<elko-study-editor>` without page-specific logic.

---

# 25. Final Architecture Summary

```text
<elko-study-editor>
        │
        │ finite IDs only
        ▼
StudyDefinition
        │
        ▼
POST /api/studies
        │
        ▼
strict decode
        │
        ▼
server registry validation
        │
        ▼
ValidatedStudy
        │
        ▼
mechanical SQL compilation
        │
        ▼
WHERE (predicate AND predicate OR ...)
        │
        ▼
exact query hash
        │
        ▼
cache lookup
        │
        ├── hit  → reuse result
        │
        └── miss → SQLite scan
```

The Phase 1 design should stay intentionally narrow:

> **A reusable, user-friendly editor for concatenating pre-approved scanner-table comparison predicates into an easy-to-generate SQL `WHERE` clause.**

That is the MVP.
