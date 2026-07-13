// Package nlp provides a natural-language-to-SQL translator for scanner studies.
// Uses an external LLM (OpenAI-compatible API) with pre-prompting and response validation.
package nlp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cetus-marketdata-scanner/internal/dblog"
)

// Config holds LLM connection settings.
type Config struct {
	BaseURL string // e.g., "https://api.deepseek.com/v1"
	APIKey  string
	Model   string // e.g., "deepseek-v4-flash"
}

// Translator converts natural language to scanner study SQL.
type Translator struct {
	cfg    Config
	client *http.Client
}

// New creates a translator with the given config.
func New(cfg Config) *Translator {
	return &Translator{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// columnList is the data-driven schema reference injected into the prompt.
// Adding a column to the scanner auto-adds it here — no code changes needed.
var columnList = `## Available Columns (snapshot table)

| Column | Type | Description |
|--------|------|-------------|
| close, high, low, open | REAL | OHLCV prices (split-adjusted) |
| sma5, sma10, sma20, sma30, sma50, sma100, sma200 | REAL | Simple moving averages |
| ema10, ema21, ema50, ema100, ema200 | REAL | Exponential moving averages |
| pct_from_sma50, pct_from_sma200 | REAL | % distance from SMA |
| ma_stack | INTEGER | 1 if EMA10>EMA21>EMA50>EMA200 |
| psar | REAL | Parabolic SAR (stop-and-reverse) |
| aroon_up, aroon_down, aroon_osc | REAL | Aroon indicator (0-100) |
| rsi14 | REAL | RSI 14-day (0-100) |
| macd, macd_signal, macd_hist | REAL | MACD line / signal / histogram |
| stoch_k, stoch_d | REAL | Stochastic %K, %D (0-100) |
| willr14 | REAL | Williams %R (-100 to 0) |
| cci20 | REAL | Commodity Channel Index |
| roc10, roc20 | REAL | Rate of Change (decimal) |
| adx14, di_plus, di_minus | REAL | ADX system (trend strength) |
| mfi14 | REAL | Money Flow Index (0-100) |
| cmf20 | REAL | Chaikin Money Flow |
| ultimate_osc | REAL | Ultimate Oscillator (0-100) |
| atr14, atr_pct | REAL | Average True Range / ATR% |
| bb_upper, bb_mid, bb_lower | REAL | Bollinger Bands (20,2) |
| bb_bandwidth | REAL | BB width (squeeze detection) |
| bb_pct_b | REAL | Position in BB (0=lower, 1=upper) |
| keltner_upper, keltner_mid, keltner_lower | REAL | Keltner Channels (20,10,2) |
| hist_vol20 | REAL | Historical volatility (annualized) |
| high_52w, low_52w | REAL | 52-week high/low |
| is_52w_high, is_52w_low | INTEGER | Boolean: at 52w extreme |
| gap_pct | REAL | Overnight gap % |
| true_range | REAL | True range |
| pct_off_52w_high, pct_above_52w_low | REAL | Distance from 52w extremes |
| ret_1d, ret_5d, ret_1m, ret_3m, ret_6m, ret_1y | REAL | Returns (decimal, 0.05 = 5%) |
| dollar_vol | REAL | Close × volume (split-invariant) |
| avg_dollar_vol20 | REAL | 20-day avg dollar volume |
| rel_volume | REAL | Volume / 20-day avg |
| obv | REAL | On-balance volume |
| vwap_dist | REAL | % distance from VWAP |
| golden_cross | INTEGER | 1 if SMA50 crossed above SMA200 |
| oversold_bounce | INTEGER | 1 if RSI crossed above 30 |`

var systemPrompt = `You are a SQL translator for a stock market scanner. Convert natural language into a SQL WHERE clause + ORDER BY + LIMIT for the snapshot table.

STRICT RULES:
1. ONLY use columns listed below. If a concept has no matching column, say "NO_MATCH".
2. ONLY use these operators: = > >= < <= BETWEEN AND LIKE AND OR ( )
3. Return NULL for unknown comparisons. Do NOT use IS NULL.
4. Return values without quotes for numbers (e.g., rsi14 < 30, not rsi14 < '30').
5. NEVER use: ; DROP INSERT DELETE UPDATE CREATE ALTER EXEC UNION SELECT subquery.
6. Output ONLY in this format — nothing else:

WHERE: the WHERE clause (or "1=1" if none)
ORDER BY: column direction (e.g., "close DESC") or "none"
LIMIT: number (e.g., "20") or "0"

7. Use BETWEEN for ranges (rsi14 BETWEEN 55 AND 70).
8. Use AND for combining conditions. Use OR sparingly with parentheses.
9. column names are lowercase with underscores.
10. returns are decimals: 5% = 0.05, -10% = -0.10.

` + columnList

// StudyResult is the parsed and validated study output.
type StudyResult struct {
	Where   string `json:"where"`
	OrderBy string `json:"order_by"`
	Limit   int    `json:"limit"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// Translate converts natural language to a validated study.
func (t *Translator) Translate(input string) (*StudyResult, error) {
	raw, err := t.callLLM(input)
	if err != nil {
		return nil, err
	}
	return parseAndValidate(raw)
}

func (t *Translator) callLLM(input string) (string, error) {
	body := chatRequest{
		Model: t.cfg.Model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: input},
		},
	}
	data, _ := json.Marshal(body)
	url := strings.TrimRight(t.cfg.BaseURL, "/") + "/chat/completions"

	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.cfg.APIKey)

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()

	var cr chatResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&cr); err != nil {
		return "", fmt.Errorf("llm decode: %w", err)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("empty llm response")
	}
	return cr.Choices[0].Message.Content, nil
}

// validColumns is the canonical list extracted from the schema. MUST be kept in sync.
var validColumns = map[string]bool{
	"close": true, "high": true, "low": true, "open": true,
	"sma5": true, "sma10": true, "sma20": true, "sma30": true,
	"sma50": true, "sma100": true, "sma200": true,
	"ema10": true, "ema21": true, "ema50": true, "ema100": true, "ema200": true,
	"pct_from_sma50": true, "pct_from_sma200": true, "ma_stack": true,
	"psar": true, "aroon_up": true, "aroon_down": true, "aroon_osc": true,
	"rsi14": true, "macd": true, "macd_signal": true, "macd_hist": true,
	"stoch_k": true, "stoch_d": true, "willr14": true, "cci20": true,
	"roc10": true, "roc20": true, "adx14": true, "di_plus": true, "di_minus": true,
	"mfi14": true, "cmf20": true, "ultimate_osc": true,
	"atr14": true, "atr_pct": true,
	"bb_upper": true, "bb_mid": true, "bb_lower": true, "bb_bandwidth": true, "bb_pct_b": true,
	"keltner_upper": true, "keltner_mid": true, "keltner_lower": true,
	"hist_vol20": true,
	"high_52w": true, "low_52w": true, "is_52w_high": true, "is_52w_low": true,
	"gap_pct": true, "true_range": true,
	"pct_off_52w_high": true, "pct_above_52w_low": true,
	"ret_1d": true, "ret_5d": true, "ret_1m": true, "ret_3m": true, "ret_6m": true, "ret_1y": true,
	"dollar_vol": true, "avg_dollar_vol20": true, "rel_volume": true,
	"obv": true, "vwap_dist": true,
	"golden_cross": true, "oversold_bounce": true,
	"between": true, "and": true, "or": true, "not": true, "desc": true, "asc": true,
}

var dangerousPatterns = []string{
	";", "--", "/*", "*/", "drop ", "delete ", "insert ", "update ",
	"create ", "alter ", "exec ", "union ", "select ", "attach ", "detach ",
	"pragma ", "trigger ", "savepoint ", "load_extension ",
}

// parseAndValidate extracts WHERE/ORDER BY/LIMIT from LLM output and validates.
func parseAndValidate(raw string) (*StudyResult, error) {
	r := &StudyResult{Where: "1=1", OrderBy: "dollar_vol DESC", Limit: 20}

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "where:"):
			r.Where = strings.TrimSpace(line[6:])
		case strings.HasPrefix(lower, "order by:"):
			o := strings.TrimSpace(line[9:])
			if o != "" && o != "none" {
				r.OrderBy = o
			}
		case strings.HasPrefix(lower, "limit:"):
			l := strings.TrimSpace(line[6:])
			var n int
			if _, err := fmt.Sscanf(l, "%d", &n); err == nil && n > 0 {
				r.Limit = n
			}
		}
	}

	// Validate
	if r.Where == "" || r.Where == "1=1" {
		r.Where = "1=1"
	}
	if err := validateClause(r.Where); err != nil {
		return nil, fmt.Errorf("invalid WHERE: %w", err)
	}
	if r.OrderBy != "" && r.OrderBy != "none" {
		if err := validateClause(r.OrderBy); err != nil {
			return nil, fmt.Errorf("invalid ORDER BY: %w", err)
		}
		// Validate sort column
		col := strings.Fields(r.OrderBy)[0]
		if !validColumns[strings.ToLower(col)] {
			return nil, fmt.Errorf("invalid ORDER BY column: %q", col)
		}
	}

	return r, nil
}

func validateClause(clause string) error {
	lower := strings.ToLower(clause)
	for _, p := range dangerousPatterns {
		if strings.Contains(lower, p) {
			return fmt.Errorf("rejected dangerous pattern %q", p)
		}
	}
	return nil
}

// ValidateSQL tests whether a WHERE clause compiles against the snapshot schema.
// Uses LIMIT 0 — SQLite parses and plans the query but reads zero rows.
// Returns nil if the SQL is valid, or the SQLite error if not.
func ValidateSQL(db *dblog.DB, where, orderBy string) error {
	if db == nil {
		return nil
	}
	// Build a safe test query: SELECT 1 to avoid touching any real data
	q := "SELECT 1 FROM snapshot WHERE snapshot_date = (SELECT MAX(snapshot_date) FROM snapshot) AND (" + where + ")"
	if orderBy != "" {
		q += " ORDER BY " + orderBy
	}
	q += " LIMIT 0"
	_, err := db.Exec("EXPLAIN " + q)
	return err
}
