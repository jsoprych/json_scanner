package dashboard

// dashSrc is the admin ops console: warehouse/ingestion state + market breadth +
// signal sections + the Sentinel data-quality watch. Self-contained, responsive,
// light/dark. Served live by `scanner serve`.
const dashSrc = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Cetus Scanner · Ops Console — {{.Digest.DateLabel}}</title>
<style>
  :root{
    --ground:#eef1f5; --panel:#fff; --panel2:#f6f8fb; --border:#d9dee6;
    --ink:#16202b; --muted:#5a6b7b; --faint:#8a99a8; --accent:#0891b2;
    --up:#0f9d58; --down:#d93025; --warn:#b7791f;
    --up-bg:#0f9d5818; --down-bg:#d9302518; --warn-bg:#b7791f18;
    --shadow:0 1px 2px rgba(16,32,43,.06),0 4px 16px rgba(16,32,43,.06);
    --mono:ui-monospace,"SF Mono",Menlo,Consolas,monospace;
    --sans:system-ui,-apple-system,"Segoe UI",Roboto,Helvetica,Arial,sans-serif;
  }
  @media (prefers-color-scheme:dark){:root{
    --ground:#0a0e13; --panel:#121820; --panel2:#0e141b; --border:#1e2732;
    --ink:#dbe4ee; --muted:#7d8b9a; --faint:#566472; --accent:#22d3ee;
    --up:#35c76a; --down:#ff5c57; --warn:#f0b429;
    --up-bg:#35c76a1c; --down-bg:#ff5c571c; --warn-bg:#f0b4291c; --shadow:none;}}
  :root[data-theme="dark"]{
    --ground:#0a0e13; --panel:#121820; --panel2:#0e141b; --border:#1e2732;
    --ink:#dbe4ee; --muted:#7d8b9a; --faint:#566472; --accent:#22d3ee;
    --up:#35c76a; --down:#ff5c57; --warn:#f0b429;
    --up-bg:#35c76a1c; --down-bg:#ff5c571c; --warn-bg:#f0b4291c; --shadow:none;}
  :root[data-theme="light"]{
    --ground:#eef1f5; --panel:#fff; --panel2:#f6f8fb; --border:#d9dee6;
    --ink:#16202b; --muted:#5a6b7b; --faint:#8a99a8; --accent:#0891b2;
    --up:#0f9d58; --down:#d93025; --warn:#b7791f;
    --up-bg:#0f9d5818; --down-bg:#d9302518; --warn-bg:#b7791f18;
    --shadow:0 1px 2px rgba(16,32,43,.06),0 4px 16px rgba(16,32,43,.06);}
  *{box-sizing:border-box}
  body{margin:0;background:var(--ground);color:var(--ink);font-family:var(--sans);font-size:14px;line-height:1.5}
  .shell{max-width:1180px;margin:0 auto;padding:18px 20px 56px}
  .mono{font-family:var(--mono);font-variant-numeric:tabular-nums}
  .lbl{font-size:10.5px;letter-spacing:.09em;text-transform:uppercase;color:var(--muted);font-weight:600}
  .up{color:var(--up)} .down{color:var(--down)} .warn{color:var(--warn)}
  .top{display:flex;align-items:center;gap:14px;flex-wrap:wrap;padding:12px 16px;background:var(--panel);border:1px solid var(--border);border-radius:12px;box-shadow:var(--shadow);margin-bottom:16px}
  .brand{display:flex;align-items:center;gap:9px;font-weight:700}
  .led{width:8px;height:8px;border-radius:50%;background:var(--up);animation:pulse 2.4s infinite}
  @keyframes pulse{0%{box-shadow:0 0 0 0 var(--up-bg)}70%{box-shadow:0 0 0 7px transparent}100%{box-shadow:0 0 0 0 transparent}}
  @media (prefers-reduced-motion:reduce){.led{animation:none}}
  .sub{color:var(--faint);letter-spacing:.14em;text-transform:uppercase;font-size:10px;font-weight:600}
  .meta{margin-left:auto;display:flex;gap:20px;flex-wrap:wrap}
  .meta .v{font-family:var(--mono);font-size:13px}
  .tgl{border:1px solid var(--border);background:var(--panel2);color:var(--muted);border-radius:8px;padding:6px 11px;font-size:12px;cursor:pointer}
  .tgl:hover{color:var(--ink);border-color:var(--accent)} .tgl:focus-visible{outline:2px solid var(--accent);outline-offset:2px}
  .kpis{display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin-bottom:16px}
  .tile{background:var(--panel);border:1px solid var(--border);border-radius:12px;padding:14px 15px;box-shadow:var(--shadow)}
  .tile .big{font-family:var(--mono);font-size:27px;font-weight:600;line-height:1.1;margin:7px 0 2px}
  .tile .foot{font-size:12px;color:var(--muted)}
  .delta{font-family:var(--mono);font-weight:600;font-size:12.5px}
  .bar{height:5px;border-radius:3px;background:var(--panel2);overflow:hidden;margin-top:9px;display:flex}
  .bar i{display:block;height:100%}
  .strip{display:grid;grid-template-columns:repeat(4,1fr);gap:1px;background:var(--border);border:1px solid var(--border);border-radius:12px;overflow:hidden;margin-bottom:16px}
  .strip .c{background:var(--panel);padding:12px 15px}
  .strip .big{font-family:var(--mono);font-size:19px;font-weight:600}
  .grid{display:grid;grid-template-columns:1fr 1fr;gap:16px}
  .panel{background:var(--panel);border:1px solid var(--border);border-radius:12px;box-shadow:var(--shadow);overflow:hidden}
  .ph{display:flex;align-items:center;gap:9px;padding:12px 15px;border-bottom:1px solid var(--border)}
  .ph h2{margin:0;font-size:13px;font-weight:700} .ph .cnt{margin-left:auto;font-family:var(--mono);font-size:12px;color:var(--faint)}
  .pb{padding:6px 6px 8px}
  table{width:100%;border-collapse:collapse;font-family:var(--mono);font-size:12.5px;font-variant-numeric:tabular-nums}
  th,td{text-align:right;padding:6px 9px;white-space:nowrap} th:first-child,td:first-child{text-align:left}
  th{color:var(--faint);font-weight:600;font-size:10px;letter-spacing:.06em;text-transform:uppercase;font-family:var(--sans)}
  tbody tr{border-top:1px solid var(--border)} tbody tr:hover{background:var(--panel2)}
  .sym{font-weight:700} .none{color:var(--faint);font-style:italic;font-size:13px;padding:8px}
  .full{grid-column:1 / -1}
  .breadth-row{display:flex;flex-wrap:wrap;gap:8px 20px;margin:10px 0 4px} .stat b{font-size:16px;font-family:var(--mono)}
  .flag{display:grid;grid-template-columns:auto 1fr auto;align-items:center;gap:12px;padding:10px 6px;border-top:1px solid var(--border)}
  .flag:first-child{border-top:0}
  .sev{font-size:10px;font-weight:700;letter-spacing:.05em;padding:3px 8px;border-radius:6px}
  .sev.SUSPECT{background:var(--down-bg);color:var(--down)} .sev.WATCH{background:var(--warn-bg);color:var(--warn)}
  .flag .name{font-family:var(--mono);font-weight:700} .flag .why{color:var(--muted);font-size:12.5px}
  .flag .metric{font-family:var(--mono);text-align:right}
  .note{color:var(--faint);font-size:11.5px;padding:10px 6px 2px;font-style:italic}
  @media (max-width:820px){.kpis{grid-template-columns:1fr 1fr}.grid{grid-template-columns:1fr}.strip{grid-template-columns:1fr 1fr}.meta{width:100%;margin-left:0}}
</style>
</head>
<body>
<div class="shell">

  <div class="top">
    <div class="brand"><span>📡</span> Cetus&nbsp;Scanner <span class="led" title="live"></span>
      <span class="sub">Ops Console</span></div>
    <div class="meta">
      <div><span class="lbl">Trading day</span><br><span class="v">{{.Digest.DateLabel}}</span></div>
      <div><span class="lbl">Eligible</span><br><span class="v">{{.Digest.SymbolsScanned}}</span></div>
      <div><span class="lbl">Scan time</span><br><span class="v">{{ms .ScanMillis}}</span></div>
    </div>
    <button class="tgl" id="tgl" aria-label="Toggle theme">◐ Theme</button>
  </div>

  <div class="kpis">
    <div class="tile">
      <span class="lbl">Universe · has data</span>
      <div class="big">{{.Stats.Count "SUCCESS"}}</div>
      <div class="foot">of {{.Stats.Total}} tracked · <b class="mono">{{num1 (.Stats.Pct "SUCCESS")}}%</b> ingested</div>
      <div class="bar" title="SUCCESS / IN_FLIGHT / PENDING / EMPTY">
        <i style="width:{{num1 (.Stats.Pct "SUCCESS")}}%;background:var(--up)"></i>
        <i style="width:{{num1 (.Stats.Pct "IN_FLIGHT")}}%;background:var(--accent)"></i>
        <i style="width:{{num1 (.Stats.Pct "PENDING")}}%;background:var(--panel2)"></i>
        <i style="width:{{num1 (.Stats.Pct "EMPTY")}}%;background:var(--warn)"></i>
      </div>
    </div>
    <div class="tile">
      <span class="lbl">Data-quality flags</span>
      <div class="big">{{len .Flags}}</div>
      <div class="foot"><span class="down">{{.Suspect}} suspect</span> · <span class="warn">{{.Watch}} watch</span></div>
    </div>
    <div class="tile">
      <span class="lbl">Breadth · above 200-DMA</span>
      <div class="big">{{num1 .Digest.Breadth.PctAbove200}}<span style="font-size:16px">%</span></div>
      {{$d := .Digest.Breadth.DeltaAbove200}}
      <div class="foot"><span class="delta {{if gt0 $d}}up{{else if lt0 $d}}down{{end}}">{{num1 $d}} pp</span> vs prior</div>
    </div>
    <div class="tile">
      <span class="lbl">Breadth · above 50-DMA</span>
      <div class="big">{{num1 .Digest.Breadth.PctAbove50}}<span style="font-size:16px">%</span></div>
      {{$e := .Digest.Breadth.DeltaAbove50}}
      <div class="foot"><span class="delta {{if gt0 $e}}up{{else if lt0 $e}}down{{end}}">{{num1 $e}} pp</span> · {{.Digest.Breadth.New52wHigh}} hi / {{.Digest.Breadth.New52wLow}} lo</div>
    </div>
  </div>

  <div class="strip">
    <div class="c"><span class="lbl">Registry</span><div class="big">{{humanInt (int64 .Stats.RegistrySymbols)}}</div></div>
    <div class="c"><span class="lbl">EOD bars</span><div class="big">{{humanInt .Stats.EODBars}}</div></div>
    <div class="c"><span class="lbl">Split ledger</span><div class="big">{{.Stats.Splits}}</div></div>
    <div class="c"><span class="lbl">Warehouse</span><div class="big">{{dbSize .DBSizeBytes}}</div></div>
  </div>

  <div class="grid">
    {{range .Digest.Sections}}
    <div class="panel">
      <div class="ph"><span>{{.Emoji}}</span><h2>{{.Title}}</h2><span class="cnt">{{len .Rows}}</span></div>
      <div class="pb">
        {{if .Rows}}
        <table>
          <thead><tr><th>Sym</th><th>Close</th><th>RSI</th><th>3-mo</th><th>$ Vol</th></tr></thead>
          <tbody>
          {{range .Rows}}<tr><td class="sym">{{.Symbol}}</td><td>{{num2 .Close}}</td><td>{{num1 .RSI14}}</td><td class="{{if gt0 .Ret3m}}up{{else if lt0 .Ret3m}}down{{end}}">{{retpct .Ret3m}}</td><td>{{money .DollarVol}}</td></tr>{{end}}
          </tbody>
        </table>
        {{else}}<div class="none">— none —</div>{{end}}
      </div>
    </div>
    {{end}}

    <div class="panel full">
      <div class="ph"><span>🛡️</span><h2>Data-Quality Watch</h2><span class="cnt">Sentinel Tier-0 · {{len .Flags}} flagged</span></div>
      <div class="pb" style="padding:4px 12px 12px">
        {{range .Flags}}
        <div class="flag">
          <span class="sev {{upper .Severity}}">{{upper .Severity}}</span>
          <div><span class="name">{{.Symbol}}</span> &nbsp;<span class="why">{{.Reason}}</span></div>
          <div class="metric">3mo <span class="up">{{retpct .Ret3m}}</span> · {{money .DollarVol}} · {{ratio .Ratio200}}</div>
        </div>
        {{else}}<div class="none">— no anomalies —</div>{{end}}
        <div class="note">Tier-0 deterministic flags. Canonical fixes belong upstream in the pipeline (fix once); this panel only surfaces.</div>
      </div>
    </div>
  </div>

  <p class="note" style="text-align:center">Live · <span class="mono">scanner serve</span> over the cetus warehouse · generated {{.Digest.GeneratedAt.Format "2006-01-02 15:04 UTC"}}</p>
</div>
<script>
  document.getElementById('tgl').addEventListener('click',function(){
    var r=document.documentElement,dark=matchMedia('(prefers-color-scheme:dark)').matches;
    var cur=r.getAttribute('data-theme')||(dark?'dark':'light');
    r.setAttribute('data-theme',cur==='dark'?'light':'dark');
  });
</script>
</body>
</html>`
