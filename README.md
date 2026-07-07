# cetus-marketdata-scanner

Ultra-lean **market-data scanner** in Go. It reads the split-adjusted EOD bars
produced by the **[cetus-marketdata-pipeline](https://github.com/jsoprych/cetus-marketdata-pipeline)**
warehouse (read-only) and turns them into signals, a daily digest, and a live
dashboard.

**Ethos:** pure Go, zero CGO, single static binary, minimal dependencies
(`modernc.org/sqlite` — same driver as the pipeline), structured JSON logging.

```
cetus-marketdata-pipeline  ──(SQLite: adjusted_bars)──►  cetus-marketdata-scanner
       (ingestion — upstream)                                 (scan — this repo)
```

## Modes

| Command | What it does |
|---------|--------------|
| `scanner` / `scanner scan` | Per-symbol **JSONL signal stream** (volume / price / gap breakouts) on stdout |
| `scanner digest` | **Daily post-close digest** — whole-universe snapshot → market breadth + preset studies (52-wk highs, golden cross, oversold bounce, momentum leaders), rendered `html` / `text` / `json` |
| `scanner serve` | The digest as a **live HTML dashboard** over HTTP (cached, `?refresh=1` to force) |
| `scanner anomalies` | **Data-quality pass** (Sentinel Tier-0): flags extreme-move × thin-liquidity × price/200-DMA outliers as text/JSONL — the deterministic seam the future cross-source + LLM tiers extend |

Any mode scans the universe chosen by `SCANNER_UNIVERSE` (`all` · `exchange:NASDAQ`
· `list:sp500` · `file:tickers.txt`).

> **Phase 1 (MVP).** The `digest`/`serve` path is built on a small fixed set of
> indicators (SMA 50/200, RSI(14), 52-wk high/low, 3-mo return, $-volume) computed
> right-aligned with a 1-bar setback. See [`docs/PHASE1_MVP.md`](docs/PHASE1_MVP.md)
> and [`docs/INDICATORS.md`](docs/INDICATORS.md).

## The read contract

The warehouse schema is owned upstream. **Read the data dictionary before touching
queries:**

- Local (siblings on disk): [`../cetus-marketdata-pipeline/docs/DATA_DICTIONARY.md`](../cetus-marketdata-pipeline/docs/DATA_DICTIONARY.md)
- GitHub (private): `jsoprych/cetus-marketdata-pipeline` → `docs/DATA_DICTIONARY.md`

Key points this scanner already honors: open the DB **read-only**, read the
**`adjusted_bars`** view (split-adjusted OHLCV — no client math), scan the
`SUCCESS` universe from `symbol_pipeline_state`, and treat free-tier IEX **volume
as a fraction** of consolidated (prefer price-based signals for now).

## Quickstart

Requires Go 1.25+ and a cetus warehouse DB (the pipeline's `cetus.db`).

```bash
make build

# 1) JSONL signal stream (stdout = signals, stderr = logs):
bin/scanner > signals.jsonl
SCANNER_MAX_SYMBOLS=200 bin/scanner | jq -c 'select(.type=="price_breakout")'

# 2) Daily digest — the free-tier report:
bin/scanner digest                                   # HTML to stdout
SCANNER_DIGEST_FORMAT=text bin/scanner digest        # plaintext, terminal-friendly
SCANNER_DIGEST_FORMAT=html SCANNER_DIGEST_OUT=digest.html bin/scanner digest

# 3) Live dashboard — test it in a browser:
bin/scanner serve                                    # http://localhost:8080
```

The warehouse path resolves as **`SCANNER_DB_PATH` → `CETUS_DB` → default**, so one
shared `CETUS_DB` env points every cetus consumer at the central store while any app
can override.

### Test the dashboard from a remote/headless box

```bash
# on the server:
bin/scanner serve                 # binds :8080
# from your laptop, forward the port over SSH, then open http://localhost:8080
ssh -L 8080:localhost:8080 you@server
```

Signal-stream example (JSONL):

```json
{"symbol":"NVDA","type":"volume_breakout","date":1751414400,"value":3.2}
{"symbol":"AMD","type":"gap_up","date":1751414400,"value":0.08}
```

## Configuration

Env-only (stdlib, no flags framework yet).

**Warehouse path** (shared convention — first non-empty wins):

| Variable | Default | Purpose |
|----------|---------|---------|
| `SCANNER_DB_PATH` | — | Per-app override of the warehouse path |
| `CETUS_DB` | — | Shared warehouse path for all cetus consumers |
| *(fallback)* | `../cetus-marketdata-pipeline/cetus.db` | Sibling default (flips to `../CETUS/cetus.db` post-migration) |

**Signal scan** (`scanner` / `scan`):

| Variable | Default | Purpose |
|----------|---------|---------|
| `SCANNER_SINCE_DAYS` | `120` | Recent history loaded per symbol |
| `SCANNER_LOOKBACK` | `20` | Trailing bars forming the scan baseline |
| `SCANNER_VOLUME_MULT` | `2.0` | Volume-breakout threshold (× trailing avg) |
| `SCANNER_GAP_PCT` | `0.05` | Gap threshold (fraction vs prior close) |
| `SCANNER_MAX_SYMBOLS` | `0` | Cap the scanned universe (0 = no cap) |
| `SCANNER_UNIVERSE` | `all` | Scope: `all` \| `exchange:X` \| `list:NAME` \| `file:PATH` |
| `SCANNER_ANOMALY_FORMAT` | `text` | `anomalies` output: `text` \| `jsonl` |

**Digest & dashboard** (`digest` / `serve`):

| Variable | Default | Purpose |
|----------|---------|---------|
| `SCANNER_DIGEST_LOOKBACK_DAYS` | `400` | History per symbol (covers the 252-bar 52-wk window) |
| `SCANNER_MIN_DOLLAR_VOL` | `1e6` | Liquidity floor for the digest universe |
| `SCANNER_DIGEST_TOP_N` | `8` | Rows per section |
| `SCANNER_DIGEST_MOMENTUM_N` | `10` | Rows in the momentum leaderboard |
| `SCANNER_DIGEST_FORMAT` | `html` | `html` \| `text` \| `json` |
| `SCANNER_DIGEST_OUT` | *(stdout)* | Output file path |
| `SCANNER_DIGEST_WORKERS` | `0` | Scan parallelism (0 = NumCPU) |
| `SCANNER_SERVE_ADDR` | `:8080` | Dashboard listen address |
| `SCANNER_SERVE_TTL_SECS` | `600` | Dashboard render cache TTL |

## Layout

```
cmd/scanner/          entrypoint: dispatch scan | digest | serve
internal/model/       neutral types (Bar, Signal)
internal/store/       READ-ONLY reader of the cetus DB (adjusted_bars, universe)
internal/scanner/     pure Scan(symbol, bars, cfg) []Signal  (JSONL signals)
internal/indicators/  pure indicator funcs (SMA, RSI, rolling high/low, return)
internal/screen/      SnapshotRow build, preset studies, market breadth (pure)
internal/scan/        concurrent whole-universe snapshot (worker pool, BarLoader iface)
internal/sentinel/    data-quality Tier-0 flags (deterministic; AI tiers extend it)
internal/digest/      Digest assembly + html/text/json renderers
internal/config/      env-first configuration (SCANNER_*, CETUS_DB)
internal/telemetry/   slog JSON logger (stderr)
docs/                 PHASE1_MVP · INDICATORS · SCANNER_DESIGN · AGENTS
```

## Development

```bash
make test   # go test ./...
make vet    # go vet ./...
```

The `scanner.Scan` function is **pure** (no I/O) — it's the seam a future
back-test/strategy engine calls bar-by-bar, and it's unit-tested with synthetic
bars (no DB needed).
