package dashboard

// The UI is a small class-driven design system: semantic component classes
// (.appbar, .card, .tab, .kpi, .table, .study-form …) styled through CSS custom
// properties (design tokens). Theming = swapping token values on the root via
// [data-theme]; components never hard-code colors, so a new theme is a token block,
// not a template change. Rendered as a PWA app-shell (installable, standalone).

// headMeta is the shared <head> content: PWA hooks + a no-flash theme bootstrap.
const headMeta = `<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<meta name="theme-color" content="#0b0f16" media="(prefers-color-scheme: dark)">
<meta name="theme-color" content="#ffffff" media="(prefers-color-scheme: light)">
<meta name="apple-mobile-web-app-capable" content="yes">
<meta name="apple-mobile-web-app-title" content="Cetus">
<link rel="manifest" href="/manifest.webmanifest">
<link rel="icon" href="/icon.svg" type="image/svg+xml">
<link rel="apple-touch-icon" href="/icon.svg">
<script>(function(){var t=localStorage.getItem('cetus-theme');if(t)document.documentElement.setAttribute('data-theme',t);})();</script>`

// stylesSrc is the token system + component classes.
const stylesSrc = `{{define "styles"}}<style>
:root{
  --bg:#eef1f5; --surface:#ffffff; --surface-2:#f2f5f9; --surface-3:#e9edf3;
  --border:#d3dae3; --border-strong:#c0c9d4;
  --text:#141a23; --text-2:#43505f; --text-3:#66717f;         /* readable, not gray-on-white */
  --accent:#2563eb; --accent-2:#1d4ed8; --accent-soft:#2563eb14;
  --up:#0a8f4e; --down:#d23b3b; --warn:#b45309;
  --up-soft:#0a8f4e18; --down-soft:#d23b3b18; --warn-soft:#b4530918;
  --shadow:0 1px 2px #10203712,0 6px 20px #10203710;
  --radius:12px; --radius-sm:8px;
  --mono:ui-monospace,"SF Mono",Menlo,Consolas,monospace;
  --sans:system-ui,-apple-system,"Segoe UI",Roboto,Helvetica,Arial,sans-serif;
}
@media (prefers-color-scheme:dark){:root{
  --bg:#0b0f16; --surface:#141b25; --surface-2:#1b2431; --surface-3:#212c3b;
  --border:#26313f; --border-strong:#33404f;
  --text:#e9eef5; --text-2:#b4c0ce; --text-3:#8695a6;
  --accent:#4f9dff; --accent-2:#6cb0ff; --accent-soft:#4f9dff1e;
  --up:#35d07f; --down:#ff6b6b; --warn:#f0b429;
  --up-soft:#35d07f1e; --down-soft:#ff6b6b1e; --warn-soft:#f0b4291e;
  --shadow:none;
}}
:root[data-theme="light"]{ --bg:#eef1f5; --surface:#fff; --surface-2:#f2f5f9; --surface-3:#e9edf3; --border:#d3dae3; --border-strong:#c0c9d4; --text:#141a23; --text-2:#43505f; --text-3:#66717f; --accent:#2563eb; --accent-2:#1d4ed8; --accent-soft:#2563eb14; --up:#0a8f4e; --down:#d23b3b; --warn:#b45309; --up-soft:#0a8f4e18; --down-soft:#d23b3b18; --warn-soft:#b4530918; --shadow:0 1px 2px #10203712,0 6px 20px #10203710; }
:root[data-theme="dark"]{ --bg:#0b0f16; --surface:#141b25; --surface-2:#1b2431; --surface-3:#212c3b; --border:#26313f; --border-strong:#33404f; --text:#e9eef5; --text-2:#b4c0ce; --text-3:#8695a6; --accent:#4f9dff; --accent-2:#6cb0ff; --accent-soft:#4f9dff1e; --up:#35d07f; --down:#ff6b6b; --warn:#f0b429; --up-soft:#35d07f1e; --down-soft:#ff6b6b1e; --warn-soft:#f0b4291e; --shadow:none; }

*{box-sizing:border-box}
html,body{margin:0}
body{background:var(--bg);color:var(--text);font-family:var(--sans);font-size:14px;line-height:1.5;-webkit-font-smoothing:antialiased}
a{color:inherit;text-decoration:none}
.mono{font-family:var(--mono);font-variant-numeric:tabular-nums}
.u-up{color:var(--up)} .u-down{color:var(--down)} .u-warn{color:var(--warn)} .u-muted{color:var(--text-3)}

/* app shell */
.wrap{max-width:1200px;margin:0 auto;padding:16px 18px 64px}
.appbar{position:sticky;top:0;z-index:20;display:flex;align-items:center;gap:16px;flex-wrap:wrap;
  padding:10px 18px;background:var(--surface);border-bottom:1px solid var(--border);backdrop-filter:saturate(1.2) blur(6px)}
.appbar__brand{display:flex;align-items:center;gap:8px;font-weight:800;letter-spacing:-.01em}
.appbar__logo{width:22px;height:22px;display:grid;place-items:center;background:var(--accent);color:#fff;border-radius:7px;font-size:13px}
.appbar__brand small{color:var(--text-3);font-weight:600}
.pulse{width:8px;height:8px;border-radius:50%;background:var(--up);animation:pulse 2.4s infinite}
@keyframes pulse{0%{box-shadow:0 0 0 0 var(--up-soft)}70%{box-shadow:0 0 0 7px transparent}100%{box-shadow:0 0 0 0 transparent}}
@media (prefers-reduced-motion:reduce){.pulse{animation:none}}
.appbar__nav{display:flex;gap:4px}
.navlink{padding:6px 12px;border-radius:8px;color:var(--text-2);font-weight:600;font-size:13px}
.navlink:hover{background:var(--surface-2);color:var(--text)}
.navlink.is-active{background:var(--accent-soft);color:var(--accent)}
.appbar__stats{display:flex;gap:8px;margin-left:auto;flex-wrap:wrap}
.stat-chip{display:flex;flex-direction:column;padding:3px 10px;border-radius:8px;background:var(--surface-2);line-height:1.25}
.stat-chip i{font-style:normal;font-size:9.5px;letter-spacing:.08em;text-transform:uppercase;color:var(--text-3);font-weight:700}
.stat-chip b{font-family:var(--mono);font-size:13px;font-variant-numeric:tabular-nums}
.appbar__user{display:flex;align-items:center;gap:8px}
.user-badge{display:flex;align-items:center;gap:9px;padding:3px 10px 3px 3px;border:1px solid var(--border);border-radius:999px}
.user-badge__ava{width:28px;height:28px;border-radius:50%;display:grid;place-items:center;background:var(--accent);color:#fff;font-weight:800;font-size:13px}
.user-badge__meta{display:flex;flex-direction:column;line-height:1.15}
.user-badge__meta b{font-size:13px} .user-badge__meta i{font-style:normal;font-size:10.5px;color:var(--text-3);text-transform:capitalize}
.icon-btn{width:32px;height:32px;display:grid;place-items:center;border:1px solid var(--border);background:var(--surface-2);color:var(--text-2);border-radius:8px;cursor:pointer;font-size:14px}
.icon-btn:hover{color:var(--text);border-color:var(--accent)}
.btn{border:1px solid var(--border);background:var(--surface-2);color:var(--text-2);border-radius:8px;padding:6px 12px;font-size:12.5px;font-weight:600;cursor:pointer;font-family:var(--sans)}
.btn:hover{color:var(--text);border-color:var(--accent)}
.btn--primary{background:var(--accent);border-color:var(--accent);color:#fff}
.btn--primary:hover{background:var(--accent-2)}
.btn:focus-visible,.icon-btn:focus-visible,.navlink:focus-visible{outline:2px solid var(--accent);outline-offset:2px}

/* tabs */
.tabs{display:flex;gap:2px;border-bottom:1px solid var(--border);margin:8px 0 18px}
.tab{appearance:none;background:none;border:0;border-bottom:2px solid transparent;padding:9px 14px;margin-bottom:-1px;
  color:var(--text-3);font-weight:600;font-size:13.5px;cursor:pointer;font-family:var(--sans)}
.tab:hover{color:var(--text)}
.tab.is-active{color:var(--accent);border-bottom-color:var(--accent)}
.pane{display:none} .pane.is-active{display:block;animation:fade .18s ease}
@keyframes fade{from{opacity:0;transform:translateY(3px)}to{opacity:1;transform:none}}

/* kpis */
.kpi-grid{display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin-bottom:16px}
.kpi{background:var(--surface);border:1px solid var(--border);border-radius:var(--radius);padding:14px 16px;box-shadow:var(--shadow)}
.kpi__label{font-size:10.5px;letter-spacing:.07em;text-transform:uppercase;color:var(--text-3);font-weight:700}
.kpi__value{font-family:var(--mono);font-size:26px;font-weight:700;line-height:1.1;margin:6px 0 2px;font-variant-numeric:tabular-nums}
.kpi__value small{font-size:15px}
.kpi__foot{font-size:12px;color:var(--text-2)}
.delta{font-family:var(--mono);font-weight:700;font-size:12.5px}
.delta.is-up{color:var(--up)} .delta.is-down{color:var(--down)}
.meter{height:6px;border-radius:4px;background:var(--surface-3);overflow:hidden;margin-top:10px;display:flex}
.meter i{display:block;height:100%}

/* strip (warehouse) */
.strip{display:grid;grid-template-columns:repeat(4,1fr);gap:1px;background:var(--border);border:1px solid var(--border);border-radius:var(--radius);overflow:hidden;margin-bottom:16px}
.strip__cell{background:var(--surface);padding:12px 16px}
.strip__cell .kpi__value{font-size:19px}

/* cards */
.grid{display:grid;grid-template-columns:1fr 1fr;gap:16px}
.card{background:var(--surface);border:1px solid var(--border);border-radius:var(--radius);box-shadow:var(--shadow);overflow:hidden}
.card--wide{grid-column:1 / -1}
.card__head{display:flex;align-items:center;gap:9px;padding:12px 16px;border-bottom:1px solid var(--border)}
.card__head h3{margin:0;font-size:13.5px;font-weight:700}
.card__count{margin-left:auto;font-family:var(--mono);font-size:12px;color:var(--text-3)}
.card__body{padding:6px 8px 10px}
.empty{color:var(--text-3);font-style:italic;font-size:13px;padding:10px}

/* tables */
.table{width:100%;border-collapse:collapse;font-family:var(--mono);font-size:12.5px;font-variant-numeric:tabular-nums}
.table th,.table td{text-align:right;padding:6px 10px;white-space:nowrap}
.table th:first-child,.table td:first-child{text-align:left}
.table th{color:var(--text-3);font-weight:700;font-size:10px;letter-spacing:.05em;text-transform:uppercase;font-family:var(--sans)}
.table tbody tr{border-top:1px solid var(--border)} .table tbody tr:hover{background:var(--surface-2)}
.table .sym{font-weight:700}
.table--left td,.table--left th{text-align:left}

/* badges / flags */
.badge{font-size:10px;font-weight:800;letter-spacing:.05em;padding:2px 7px;border-radius:6px;background:var(--surface-3);color:var(--text-2)}
.badge--admin{background:var(--down-soft);color:var(--down)}
.badge--off{background:var(--down-soft);color:var(--down)}
.badge--tier{background:var(--accent-soft);color:var(--accent);text-transform:capitalize}
.flag{display:grid;grid-template-columns:auto 1fr auto;align-items:center;gap:12px;padding:10px 8px;border-top:1px solid var(--border)}
.flag:first-child{border-top:0}
.sev{font-size:10px;font-weight:800;letter-spacing:.05em;padding:3px 9px;border-radius:6px}
.sev.is-suspect{background:var(--down-soft);color:var(--down)}
.sev.is-watch{background:var(--warn-soft);color:var(--warn)}
.flag__name{font-family:var(--mono);font-weight:700}
.flag__why{color:var(--text-2);font-size:12.5px}
.flag__metric{font-family:var(--mono);text-align:right;font-size:12px}

/* forms */
.mini{font-size:11px;padding:3px 8px;border:1px solid var(--border);border-radius:6px;background:var(--surface-2);color:var(--text-2);cursor:pointer;font-family:var(--sans)}
.mini:hover{color:var(--text);border-color:var(--accent)} .mini--danger:hover{color:var(--down);border-color:var(--down)}
.input,.select,.textarea{width:100%;padding:8px 10px;border:1px solid var(--border);border-radius:var(--radius-sm);background:var(--surface-2);color:var(--text);font-size:13px;font-family:var(--sans)}
.textarea{min-height:64px;font-family:var(--mono);resize:vertical}
.input:focus,.select:focus,.textarea:focus{outline:2px solid var(--accent);outline-offset:1px;border-color:var(--accent)}
.study-form{display:grid;grid-template-columns:repeat(4,1fr);gap:10px;padding:14px 8px 6px;border-top:1px solid var(--border)}
.field{display:flex;flex-direction:column;gap:4px}
.field label{font-size:10px;letter-spacing:.05em;text-transform:uppercase;color:var(--text-3);font-weight:700}
.col-2{grid-column:span 2} .col-all{grid-column:1 / -1}
.btn-row{grid-column:1 / -1;display:flex;gap:10px;align-items:center;flex-wrap:wrap}
.result{font-family:var(--mono);font-size:12px;color:var(--text-2)}
.hint{font-size:11.5px;color:var(--text-3)}
.user-form{display:flex;gap:8px;flex-wrap:wrap;align-items:center;padding:12px 8px 4px;border-top:1px solid var(--border)}
.user-form .input,.user-form .select{width:auto}
.input--sm{width:90px;padding:5px 8px;font-family:var(--mono)}
.inline{display:inline}
.clip{max-width:240px;overflow:hidden;text-overflow:ellipsis}
.io-row{display:flex;gap:10px;align-items:flex-start;flex-wrap:wrap;padding:8px;border-top:1px solid var(--border);margin-top:6px}
.io-import{flex:1;min-width:220px}
.io-import summary{list-style:none;display:inline-block} .io-import summary::-webkit-details-marker{display:none}
.io-form{display:flex;gap:8px;margin-top:8px;flex-wrap:wrap}
.io-form .textarea{flex:1;min-width:240px;min-height:70px}
.foot-note{color:var(--text-3);font-size:11.5px;text-align:center;padding:14px 0 2px}

@media (max-width:860px){
  .kpi-grid{grid-template-columns:1fr 1fr} .grid{grid-template-columns:1fr}
  .strip{grid-template-columns:1fr 1fr} .study-form{grid-template-columns:1fr 1fr}
  .appbar__stats{width:100%;margin-left:0;order:3}
}
</style>{{end}}`

// appbar is the consistent header — logged-in user front & centre, global stats,
// primary nav (active-aware via .Page), theme control.
const headerSrc = `{{define "appbar"}}<header class="appbar">
  <div class="appbar__brand"><span class="appbar__logo">📡</span>Cetus <small>Scanner</small> <span class="pulse" title="live"></span></div>
  <nav class="appbar__nav">
    <a class="navlink {{if eq .Page "index"}}is-active{{end}}" href="/">Dashboard</a>
    {{if .M.Acting.IsAdmin}}<a class="navlink {{if eq .Page "admin"}}is-active{{end}}" href="/admin">Admin</a>{{end}}
  </nav>
  <div class="appbar__stats">
    <span class="stat-chip"><i>Day</i><b>{{.M.Digest.DateLabel}}</b></span>
    <span class="stat-chip"><i>Above 200-DMA</i><b>{{num1 .M.Digest.Breadth.PctAbove200}}%</b></span>
    <span class="stat-chip"><i>Scanned</i><b>{{.M.Digest.SymbolsScanned}}</b></span>
  </div>
  <div class="appbar__user">
    <button class="icon-btn" id="themeBtn" title="Theme (auto / light / dark)" aria-label="Toggle theme">◐</button>
    <span class="user-badge">
      <span class="user-badge__ava">{{firstLetter .M.Acting.Name}}</span>
      <span class="user-badge__meta"><b>{{.M.Acting.Name}}</b><i>{{.M.Acting.Tier}}{{if .M.Acting.IsAdmin}} · admin{{end}}</i></span>
    </span>
    {{if .M.SessionAuth}}<a class="btn" href="/logout">Sign out</a>{{end}}
  </div>
</header>{{end}}`

// indexSrc — the user dashboard: Signals + My Studies tabs.
const indexSrc = `{{define "index"}}<!doctype html><html lang="en"><head>` + headMeta + `
<title>Cetus Scanner — Dashboard</title>{{template "styles"}}</head><body>
{{template "appbar" (hdr . "index")}}
<main class="wrap">
  <div class="tabs" role="tablist">
    <button class="tab is-active" data-tab="signals">Signals</button>
    <button class="tab" data-tab="mystudies">My Studies</button>
  </div>

  <section class="pane is-active" data-pane="signals">
    <div class="kpi-grid">
      <div class="kpi"><div class="kpi__label">Above 200-DMA</div><div class="kpi__value">{{num1 .Digest.Breadth.PctAbove200}}<small>%</small></div></div>
      <div class="kpi"><div class="kpi__label">Above 50-DMA</div><div class="kpi__value">{{num1 .Digest.Breadth.PctAbove50}}<small>%</small></div></div>
      <div class="kpi"><div class="kpi__label">New 52-wk highs</div><div class="kpi__value u-up">{{.Digest.Breadth.New52wHigh}}</div><div class="kpi__foot">new lows <b class="u-down">{{.Digest.Breadth.New52wLow}}</b></div></div>
      <div class="kpi"><div class="kpi__label">Universe scanned</div><div class="kpi__value">{{.Digest.SymbolsScanned}}</div><div class="kpi__foot">for {{.Acting.Tier}} tier</div></div>
    </div>
    <div class="grid">
      {{range .Digest.Sections}}
      <div class="card">
        <div class="card__head"><span>{{.Emoji}}</span><h3>{{.Title}}</h3><span class="card__count">{{len .Rows}}</span></div>
        <div class="card__body">{{if .Rows}}<table class="table">
          <thead><tr><th>Sym</th><th>Close</th><th>RSI</th><th>3-mo</th><th>$ Vol</th></tr></thead>
          <tbody>{{range .Rows}}<tr><td class="sym">{{.Symbol}}</td><td>{{num2 .Close}}</td><td>{{num1 .RSI14}}</td><td class="{{if gt0 .Ret3m}}u-up{{else if lt0 .Ret3m}}u-down{{end}}">{{retpct .Ret3m}}</td><td>{{money .DollarVol}}</td></tr>{{end}}</tbody>
        </table>{{else}}<div class="empty">— none —</div>{{end}}</div>
      </div>
      {{end}}
    </div>
  </section>

  <section class="pane" data-pane="mystudies">
    <div class="card card--wide">
      <div class="card__head"><span>🧪</span><h3>My Studies</h3><span class="card__count">{{len .MyStudies}}{{if gt .StudyQuota 0}} / {{.StudyQuota}}{{else}} / ∞{{end}}</span></div>
      <div class="card__body">
        {{if .MyStudies}}<table class="table">
          <thead><tr><th>Key</th><th>Title</th><th>Vis</th><th>Group</th><th>WHERE</th><th></th></tr></thead>
          <tbody>{{range .MyStudies}}<tr>
            <td class="sym">{{.Key}}</td><td>{{.Emoji}} {{.Title}}</td><td>{{.Visibility}}</td><td>{{.Group}}</td>
            <td class="mono clip">{{.Where}}</td>
            <td><button class="mini" type="button" onclick="editStudy('{{.Key}}')">edit</button><button class="mini" type="button" onclick="cloneStudy('{{.Key}}')">clone</button>
              <form method="post" action="/studies" class="inline"><input type="hidden" name="action" value="delete"><input type="hidden" name="key" value="{{.Key}}"><button class="mini mini--danger" onclick="return confirm('Delete {{.Key}}?')">del</button></form></td>
          </tr>{{end}}</tbody>
        </table>{{else}}<div class="empty">— no studies yet — create one below —</div>{{end}}
        <form method="post" action="/studies" id="studyForm" class="study-form">
          <div class="field"><label>Key</label><input class="input" name="key" id="s_key" placeholder="unique-key" required></div>
          <div class="field"><label>Title</label><input class="input" name="title" id="s_title" placeholder="My Breakout"></div>
          <div class="field"><label>Emoji</label><input class="input" name="emoji" id="s_emoji" placeholder="🔥"></div>
          <div class="field"><label>Visibility</label><select class="select" name="visibility" id="s_vis"><option value="private">private</option><option value="group">group (shared)</option></select></div>
          <div class="field"><label>Group</label><input class="input" name="group" id="s_group" placeholder="desk-a"></div>
          <div class="field col-2"><label>Order by</label><input class="input" name="order_by" id="s_order" placeholder="dollar_vol DESC"></div>
          <div class="field"><label>Limit</label><input class="input" type="number" name="limit" id="s_limit" placeholder="10"></div>
          <div class="field col-all"><label>WHERE (SQL over the snapshot)</label><textarea class="textarea" name="where" id="s_where" placeholder="close > sma200 AND rsi14 between 55 and 70"></textarea></div>
          <div class="btn-row">
            <button class="btn btn--primary" type="submit">Save</button>
            <button class="btn" type="button" onclick="testStudy()">Test WHERE</button>
            <button class="btn" type="button" onclick="clearStudy()">New</button>
            <span class="result" id="s_result"></span>
            {{if and (gt .StudyQuota 0) (ge (len .MyStudies) .StudyQuota)}}<span class="u-warn hint">Limit reached ({{.StudyQuota}}) — upgrade for more.</span>{{end}}
          </div>
        </form>
        {{template "studyio" .}}
      </div>
    </div>
  </section>

  <p class="foot-note">Live · generated {{.Digest.GeneratedAt.Format "2006-01-02 15:04 UTC"}}</p>
</main>
<script>var STUDYDATA={{studiesJSON .MyStudies}};</script>` + appScript + `</body></html>{{end}}`

// adminSrc — operator console: Overview / Data Quality / Users / Studies tabs.
const adminSrc = `{{define "admin"}}<!doctype html><html lang="en"><head>` + headMeta + `
<title>Cetus Scanner — Admin</title>{{template "styles"}}</head><body>
{{template "appbar" (hdr . "admin")}}
<main class="wrap">
  <div class="tabs" role="tablist">
    <button class="tab is-active" data-tab="overview">Overview</button>
    <button class="tab" data-tab="quality">Data Quality <span class="badge">{{len .Flags}}</span></button>
    <button class="tab" data-tab="users">Users <span class="badge">{{len .Users}}</span></button>
    <button class="tab" data-tab="studies">Studies <span class="badge">{{len .Studies}}</span></button>
  </div>

  <section class="pane is-active" data-pane="overview">
    <div class="kpi-grid">
      <div class="kpi"><div class="kpi__label">Universe · has data</div><div class="kpi__value">{{.Stats.Count "SUCCESS"}}</div>
        <div class="kpi__foot">of {{.Stats.Total}} · <b class="mono">{{num1 (.Stats.Pct "SUCCESS")}}%</b> ingested</div>
        <div class="meter" title="SUCCESS / IN_FLIGHT / PENDING / EMPTY">
          <i style="width:{{num1 (.Stats.Pct "SUCCESS")}}%;background:var(--up)"></i><i style="width:{{num1 (.Stats.Pct "IN_FLIGHT")}}%;background:var(--accent)"></i><i style="width:{{num1 (.Stats.Pct "PENDING")}}%;background:var(--surface-3)"></i><i style="width:{{num1 (.Stats.Pct "EMPTY")}}%;background:var(--warn)"></i></div></div>
      <div class="kpi"><div class="kpi__label">Data-quality flags</div><div class="kpi__value">{{len .Flags}}</div><div class="kpi__foot"><span class="u-down">{{.Suspect}} suspect</span> · <span class="u-warn">{{.Watch}} watch</span></div></div>
      <div class="kpi"><div class="kpi__label">Above 200-DMA</div><div class="kpi__value">{{num1 .Digest.Breadth.PctAbove200}}<small>%</small></div></div>
      <div class="kpi"><div class="kpi__label">Scan time</div><div class="kpi__value">{{ms .ScanMillis}}</div><div class="kpi__foot">{{len .Users}} users</div></div>
    </div>
    <div class="strip">
      <div class="strip__cell"><div class="kpi__label">Registry</div><div class="kpi__value">{{humanInt (int64 .Stats.RegistrySymbols)}}</div></div>
      <div class="strip__cell"><div class="kpi__label">EOD bars</div><div class="kpi__value">{{humanInt .Stats.EODBars}}</div></div>
      <div class="strip__cell"><div class="kpi__label">Split ledger</div><div class="kpi__value">{{.Stats.Splits}}</div></div>
      <div class="strip__cell"><div class="kpi__label">Warehouse</div><div class="kpi__value">{{dbSize .DBSizeBytes}}</div></div>
    </div>
  </section>

  <section class="pane" data-pane="quality">
    <div class="card card--wide">
      <div class="card__head"><span>🛡️</span><h3>Data-Quality Watch</h3><span class="card__count">Sentinel Tier-0 · {{len .Flags}} flagged</span></div>
      <div class="card__body">
        {{range .Flags}}<div class="flag"><span class="sev is-{{.Severity}}">{{upper .Severity}}</span>
          <div><span class="flag__name">{{.Symbol}}</span> &nbsp;<span class="flag__why">{{.Reason}}</span></div>
          <div class="flag__metric">3mo <span class="u-up">{{retpct .Ret3m}}</span> · {{money .DollarVol}} · {{ratio .Ratio200}}</div></div>
        {{else}}<div class="empty">— no anomalies —</div>{{end}}
        <p class="hint" style="padding:10px 8px 0">Tier-0 deterministic flags. Canonical fixes belong upstream in the pipeline; this only surfaces.</p>
      </div>
    </div>
  </section>

  <section class="pane" data-pane="users">
    <div class="card card--wide">
      <div class="card__head"><span>👥</span><h3>Users</h3><span class="card__count">{{len .Users}}</span></div>
      <div class="card__body">
        <table class="table">
          <thead><tr><th>ID</th><th>Name</th><th>Tier</th><th>Role</th><th>Groups</th><th>Manage</th></tr></thead>
          <tbody>{{range .Users}}<tr>
            <td class="sym">{{.ID}}{{if .Disabled}} <span class="badge badge--off">off</span>{{end}}</td><td>{{.Name}}</td><td>{{.Tier}}</td><td>{{.Role}}</td>
            <td><form method="post" action="/admin/users" class="inline"><input type="hidden" name="action" value="set-groups"><input type="hidden" name="id" value="{{.ID}}"><input class="input input--sm" name="groups" value="{{join .Groups}}" placeholder="a, b"><button class="mini">set</button></form></td>
            <td><form method="post" action="/admin/users" class="inline">
              <input type="hidden" name="id" value="{{.ID}}">
              {{if .Disabled}}<button class="mini" name="action" value="enable">Enable</button>{{else}}<button class="mini" name="action" value="disable">Disable</button>{{end}}
              <button class="mini" name="action" value="{{if eq .Tier "pro"}}set-free{{else}}set-pro{{end}}">{{if eq .Tier "pro"}}→free{{else}}→pro{{end}}</button>
              <button class="mini" name="action" value="{{if .IsAdmin}}set-user{{else}}set-admin{{end}}">{{if .IsAdmin}}→user{{else}}→admin{{end}}</button>
              <button class="mini mini--danger" name="action" value="delete" onclick="return confirm('Delete {{.ID}}?')">del</button>
            </form></td>
          </tr>{{end}}</tbody>
        </table>
        <form method="post" action="/admin/users" class="user-form">
          <input type="hidden" name="action" value="create">
          <input class="input" name="id" placeholder="id" required><input class="input" name="name" placeholder="name">
          <input class="input" name="password" type="password" placeholder="password"><input class="input" name="groups" placeholder="groups (csv)">
          <select class="select" name="tier"><option value="free">free</option><option value="pro">pro</option></select>
          <select class="select" name="role"><option value="user">user</option><option value="admin">admin</option></select>
          <button class="btn btn--primary" type="submit">Create user</button>
        </form>
      </div>
    </div>
  </section>

  <section class="pane" data-pane="studies">
    <div class="card card--wide">
      <div class="card__head"><span>🧪</span><h3>Studies</h3><span class="card__count">{{len .Studies}}</span></div>
      <div class="card__body">
        <table class="table"><thead><tr><th>Key</th><th>Title</th><th>Owner</th><th>Vis</th><th>Tier</th><th>Group</th><th>WHERE</th><th></th></tr></thead>
          <tbody>{{range .Studies}}<tr>
            <td class="sym">{{.Key}}</td><td>{{.Emoji}} {{.Title}}</td><td>{{.Owner}}</td><td>{{.Visibility}}</td><td>{{.Tier}}</td><td>{{.Group}}</td>
            <td class="mono clip">{{.Where}}</td>
            <td><button class="mini" type="button" onclick="editStudy('{{.Key}}')">edit</button><button class="mini" type="button" onclick="cloneStudy('{{.Key}}')">clone</button>
              <form method="post" action="/studies" class="inline"><input type="hidden" name="action" value="delete"><input type="hidden" name="key" value="{{.Key}}"><button class="mini mini--danger" onclick="return confirm('Delete {{.Key}}?')">del</button></form></td>
          </tr>{{end}}</tbody>
        </table>
        <form method="post" action="/studies" id="studyForm" class="study-form">
          <div class="field"><label>Key</label><input class="input" name="key" id="s_key" placeholder="unique-key" required></div>
          <div class="field"><label>Title</label><input class="input" name="title" id="s_title"></div>
          <div class="field"><label>Emoji</label><input class="input" name="emoji" id="s_emoji"></div>
          <div class="field"><label>Owner</label><input class="input" name="owner" id="s_owner" placeholder="global"></div>
          <div class="field"><label>Visibility</label><select class="select" name="visibility" id="s_vis"><option value="public">public</option><option value="group">group</option><option value="private">private</option></select></div>
          <div class="field"><label>Group</label><input class="input" name="group" id="s_group"></div>
          <div class="field"><label>Tier</label><select class="select" name="tier" id="s_tier"><option value="free">free</option><option value="pro">pro</option></select></div>
          <div class="field"><label>Limit</label><input class="input" type="number" name="limit" id="s_limit"></div>
          <div class="field col-all"><label>Order by</label><input class="input" name="order_by" id="s_order" placeholder="dollar_vol DESC"></div>
          <div class="field col-all"><label>WHERE</label><textarea class="textarea" name="where" id="s_where" placeholder="close > sma200 AND rsi14 < 45"></textarea></div>
          <div class="btn-row"><button class="btn btn--primary" type="submit">Save study</button><button class="btn" type="button" onclick="testStudy()">Test WHERE</button><button class="btn" type="button" onclick="clearStudy()">New</button><span class="result" id="s_result"></span></div>
        </form>
        {{template "studyio" .}}
      </div>
    </div>
  </section>

  <p class="foot-note">Admin · generated {{.Digest.GeneratedAt.Format "2006-01-02 15:04 UTC"}}</p>
</main>
<script>var STUDYDATA={{studiesJSON .Studies}};</script>` + appScript + `</body></html>{{end}}`

// loginSrc — sign-in.
const loginSrc = `{{define "login"}}<!doctype html><html lang="en"><head>` + headMeta + `
<title>Cetus Scanner — Sign in</title>{{template "styles"}}</head><body>
<main class="wrap" style="max-width:400px">
  <div class="card" style="margin-top:12vh">
    <div class="card__head"><span class="appbar__logo">📡</span><h3>Cetus Scanner — Sign in</h3></div>
    <div class="card__body" style="padding:18px">
      {{if .Error}}<div class="u-down" style="font-size:13px;margin-bottom:12px">{{.Error}}</div>{{end}}
      <form method="post" action="/login">
        <div class="field" style="margin-bottom:12px"><label>User</label><input class="input" name="user" autofocus autocomplete="username"></div>
        <div class="field" style="margin-bottom:16px"><label>Password</label><input class="input" name="password" type="password" autocomplete="current-password"></div>
        <button class="btn btn--primary" type="submit" style="width:100%;padding:10px">Sign in</button>
      </form>
      <p class="hint" style="margin-top:14px">Users: {{range .Users}}<b>{{.ID}}</b> · {{end}}<span class="u-muted">(dev: password = id)</span></p>
    </div>
  </div>
</main></body></html>{{end}}`

// studyioSrc — the shared Export / Import toolbar for study panes.
const studyioSrc = `{{define "studyio"}}<div class="io-row">
  <a class="btn" href="/studies/export" download>⭳ Export JSONL</a>
  <details class="io-import"><summary class="btn">⭱ Import…</summary>
    <form method="post" action="/studies/import" class="io-form">
      <textarea class="textarea" name="jsonl" placeholder="paste studies JSONL — one JSON object per line (e.g. from Export)"></textarea>
      <button class="btn btn--primary" type="submit">Import</button>
    </form>
  </details>
</div>{{end}}`

// appScript — tab switching, theme cycling (persisted), study editor, PWA SW.
const appScript = `<script>
(function(){
  // tabs
  document.querySelectorAll('.tab').forEach(function(t){t.addEventListener('click',function(){
    var name=t.getAttribute('data-tab');
    document.querySelectorAll('.tab').forEach(function(x){x.classList.toggle('is-active',x===t);});
    document.querySelectorAll('.pane').forEach(function(p){p.classList.toggle('is-active',p.getAttribute('data-pane')===name);});
  });});
  // theme cycle auto→light→dark
  var order=['','light','dark'],glyph={'':'◐',light:'☀',dark:'☾'};
  function set(t){var r=document.documentElement;if(t){r.setAttribute('data-theme',t);localStorage.setItem('cetus-theme',t);}else{r.removeAttribute('data-theme');localStorage.removeItem('cetus-theme');}var b=document.getElementById('themeBtn');if(b)b.textContent=glyph[t];}
  set(localStorage.getItem('cetus-theme')||'');
  var tb=document.getElementById('themeBtn');if(tb)tb.addEventListener('click',function(){var c=localStorage.getItem('cetus-theme')||'';set(order[(order.indexOf(c)+1)%order.length]);});
})();
function _g(x){return document.getElementById(x);}
function editStudy(k){var s=(window.STUDYDATA||[]).find(function(x){return x.key===k;});if(!s)return;
  [['s_key','key'],['s_title','title'],['s_emoji','emoji'],['s_owner','owner'],['s_group','group'],['s_order','order_by'],['s_limit','limit'],['s_where','where']].forEach(function(p){var el=_g(p[0]);if(el)el.value=s[p[1]]||'';});
  var v=_g('s_vis');if(v)v.value=s.visibility||'private';var t=_g('s_tier');if(t)t.value=s.tier||'free';
  var r=_g('s_result');if(r)r.textContent='editing '+s.key;_g('studyForm').scrollIntoView();}
function clearStudy(){_g('studyForm').reset();var r=_g('s_result');if(r)r.textContent='';}
function cloneStudy(k){editStudy(k);var e=_g('s_key');if(e)e.value=(e.value||k)+'-copy';var r=_g('s_result');if(r)r.textContent='cloned '+k+' — change the key & Save';}
function testStudy(){var b=new URLSearchParams({where:_g('s_where').value,order_by:(_g('s_order')||{}).value||'',limit:(_g('s_limit')||{}).value||'20'});
  _g('s_result').textContent='testing…';
  fetch('/studies/test',{method:'POST',body:b}).then(function(r){return r.json();}).then(function(d){
    _g('s_result').textContent=d.error?('✗ '+d.error):('✓ '+d.count+' match: '+((d.sample||[]).join(', ')||'—'));
  }).catch(function(e){_g('s_result').textContent='✗ '+e;});}
if('serviceWorker' in navigator){navigator.serviceWorker.register('/sw.js').catch(function(){});}
</script>`
