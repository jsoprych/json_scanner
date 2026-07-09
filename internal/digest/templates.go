package digest

// htmlSrc is a responsive, email-safe digest: a centered max-width container,
// system fonts, no external resources, columns kept few so it reflows cleanly on a
// phone. Renders equally as a standalone web page.
const htmlSrc = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>ChartGeometry — Market Close Scan — {{.DateLabel}}</title>
<style>
  :root { color-scheme: light dark; }
  body { margin:0; padding:0; background:#f4f5f7; color:#1a1f2b;
         font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif; }
  .wrap { max-width:640px; margin:0 auto; padding:16px; }
  .card { background:#ffffff; border-radius:12px; padding:20px; margin:0 0 16px;
          box-shadow:0 1px 3px rgba(0,0,0,.08); }
  h1 { font-size:18px; margin:0 0 2px; }
  .muted { color:#6b7280; font-size:13px; }
  .breadth-row { display:flex; flex-wrap:wrap; gap:8px 20px; margin:10px 0 4px; }
  .stat { font-size:14px; }
  .stat b { font-size:16px; }
  .up { color:#0a7d34; } .down { color:#b4232a; } .flat { color:#6b7280; }
  h2 { font-size:15px; margin:0 0 8px; }
  table { width:100%; border-collapse:collapse; font-size:13px; }
  th,td { text-align:right; padding:6px 8px; border-bottom:1px solid #eef0f3; white-space:nowrap; }
  th:first-child, td:first-child { text-align:left; }
  th { color:#6b7280; font-weight:600; font-size:12px; text-transform:uppercase; letter-spacing:.03em; }
  td.sym { font-weight:700; }
  .none { color:#9aa1ab; font-style:italic; font-size:13px; }
  .foot { text-align:center; color:#6b7280; font-size:12px; padding:8px 0 24px; }
  .cta { display:inline-block; margin-top:6px; padding:9px 16px; border-radius:8px;
         background:#1a1f2b; color:#fff !important; text-decoration:none; font-size:13px; }
  @media (prefers-color-scheme: dark) {
    body { background:#0f1115; color:#e6e8ec; }
    .card { background:#171a21; box-shadow:none; }
    .muted,.stat,th { color:#9aa1ab; }
    th,td { border-color:#242833; }
    .cta { background:#e6e8ec; color:#0f1115 !important; }
  }
</style>
</head>
<body>
<div class="wrap">

  <div class="card">
    <h1>📊 ChartGeometry · Market Close Scan</h1>
    <div class="muted">{{.DateLabel}} · {{.SymbolsScanned}} symbols scanned</div>
  </div>

  <div class="card">
    <h2>Market Breadth</h2>
    <div class="breadth-row">
      <span class="stat">Above 200-DMA <b>{{pp1 .Breadth.PctAbove200}}%</b></span>
      <span class="stat">Above 50-DMA <b>{{pp1 .Breadth.PctAbove50}}%</b></span>
    </div>
    <div class="breadth-row">
      <span class="stat">New 52-wk highs <b class="up">{{.Breadth.New52wHigh}}</b></span>
      <span class="stat">New lows <b class="down">{{.Breadth.New52wLow}}</b></span>
    </div>
    <div class="muted">→ {{mood .Breadth}}</div>
  </div>

  {{range .Sections}}
  <div class="card">
    <h2>{{.Emoji}} {{.Title}}</h2>
    {{if .Rows}}
    <table>
      <thead><tr><th>Symbol</th><th>Close</th><th>RSI</th><th>3-mo</th><th>$ Vol</th></tr></thead>
      <tbody>
      {{range .Rows}}
        <tr><td class="sym">{{.Symbol}}</td><td>{{f2 .Close}}</td><td>{{pp1 .RSI14}}</td><td>{{pct1 .Ret3m}}</td><td>{{money .DollarVol}}</td></tr>
      {{end}}
      </tbody>
    </table>
    {{else}}<div class="none">— none today —</div>{{end}}
  </div>
  {{end}}

  <div class="foot">
    Want alerts on your own screen? <br>
    <a class="cta" href="#">Get the daily digest →</a>
  </div>

</div>
</body>
</html>`

// textSrc is the plaintext fallback.
const textSrc = `CHARTGEOMETRY · MARKET CLOSE SCAN
{{.DateLabel}} · {{.SymbolsScanned}} symbols scanned

MARKET BREADTH
  Above 200-DMA: {{pp1 .Breadth.PctAbove200}}%
  Above 50-DMA:  {{pp1 .Breadth.PctAbove50}}%
  New 52-wk highs: {{.Breadth.New52wHigh}}   New lows: {{.Breadth.New52wLow}}
  -> {{mood .Breadth}}
{{range .Sections}}
{{.Emoji}} {{.Title}}
{{- if .Rows}}
{{- range .Rows}}
  {{printf "%-8s" .Symbol}} {{f2 .Close}}  RSI {{pp1 .RSI14}}  3mo {{pct1 .Ret3m}}  {{money .DollarVol}}
{{- end}}
{{- else}}
  -- none today --
{{- end}}
{{end}}
Want alerts on your own screen? Subscribe to the daily digest.
`
