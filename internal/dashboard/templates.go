package dashboard

// stylesSrc is the shared style block, included by both pages via {{template "styles"}}.
const stylesSrc = `{{define "styles"}}<style>
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
  .badge{font-size:10px;font-weight:700;letter-spacing:.08em;padding:2px 7px;border-radius:5px;background:var(--down-bg);color:var(--down)}
  .sub{color:var(--faint);letter-spacing:.14em;text-transform:uppercase;font-size:10px;font-weight:600}
  .meta{margin-left:auto;display:flex;gap:20px;flex-wrap:wrap}
  .meta .v{font-family:var(--mono);font-size:13px}
  .btn{border:1px solid var(--border);background:var(--panel2);color:var(--muted);border-radius:8px;padding:6px 11px;font-size:12px;cursor:pointer;text-decoration:none;display:inline-block}
  .btn:hover{color:var(--ink);border-color:var(--accent)} .btn:focus-visible{outline:2px solid var(--accent);outline-offset:2px}
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
  .flag{display:grid;grid-template-columns:auto 1fr auto;align-items:center;gap:12px;padding:10px 6px;border-top:1px solid var(--border)}
  .flag:first-child{border-top:0}
  .sev{font-size:10px;font-weight:700;letter-spacing:.05em;padding:3px 8px;border-radius:6px}
  .sev.SUSPECT{background:var(--down-bg);color:var(--down)} .sev.WATCH{background:var(--warn-bg);color:var(--warn)}
  .flag .name{font-family:var(--mono);font-weight:700} .flag .why{color:var(--muted);font-size:12.5px}
  .flag .metric{font-family:var(--mono);text-align:right}
  .note{color:var(--faint);font-size:11.5px;padding:10px 6px 2px;font-style:italic;text-align:center}
  input,select{padding:7px 9px;border:1px solid var(--border);border-radius:7px;background:var(--panel2);color:var(--ink);font-size:13px;font-family:var(--sans)}
  input:focus,select:focus{outline:2px solid var(--accent);outline-offset:1px}
  .mini{font-size:11px;padding:3px 7px;border:1px solid var(--border);border-radius:6px;background:var(--panel2);color:var(--muted);cursor:pointer;font-family:var(--sans)}
  .mini:hover{color:var(--ink);border-color:var(--accent)} .mini.danger:hover{color:var(--down);border-color:var(--down)}
  .off{font-size:10px;font-weight:700;padding:1px 6px;border-radius:5px;background:var(--down-bg);color:var(--down)}
  @media (max-width:820px){.kpis{grid-template-columns:1fr 1fr}.grid{grid-template-columns:1fr}.strip{grid-template-columns:1fr 1fr}.meta{width:100%;margin-left:0}}
</style>{{end}}`

const themeScript = `<script>document.getElementById('tgl').addEventListener('click',function(){var r=document.documentElement,d=matchMedia('(prefers-color-scheme:dark)').matches,c=r.getAttribute('data-theme')||(d?'dark':'light');r.setAttribute('data-theme',c==='dark'?'light':'dark');});</script>`

// indexSrc is the user-facing dashboard: breadth + the acting user's signal studies.
const indexSrc = `{{define "index"}}<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Cetus Scanner — Dashboard</title>{{template "styles"}}</head>
<body><div class="shell">
  <div class="top">
    <div class="brand"><span>📡</span> Cetus&nbsp;Scanner <span class="led" title="live"></span></div>
    <div class="meta">
      <div><span class="lbl">Viewing as</span><br><span class="v">{{.Acting.Name}} · {{.Acting.Tier}}</span></div>
      <div><span class="lbl">Trading day</span><br><span class="v">{{.Digest.DateLabel}}</span></div>
    </div>
    {{if .Acting.IsAdmin}}<a class="btn" href="/admin">Admin →</a>{{end}}
    <button class="btn" id="tgl" aria-label="Toggle theme">◐ Theme</button>
    {{if .SessionAuth}}<a class="btn" href="/logout">Sign out</a>{{end}}
  </div>

  <div class="kpis">
    <div class="tile"><span class="lbl">Above 200-DMA</span>
      <div class="big">{{num1 .Digest.Breadth.PctAbove200}}<span style="font-size:16px">%</span></div>
      {{$d := .Digest.Breadth.DeltaAbove200}}<div class="foot"><span class="delta {{if gt0 $d}}up{{else if lt0 $d}}down{{end}}">{{num1 $d}} pp</span> vs prior</div></div>
    <div class="tile"><span class="lbl">Above 50-DMA</span>
      <div class="big">{{num1 .Digest.Breadth.PctAbove50}}<span style="font-size:16px">%</span></div>
      {{$e := .Digest.Breadth.DeltaAbove50}}<div class="foot"><span class="delta {{if gt0 $e}}up{{else if lt0 $e}}down{{end}}">{{num1 $e}} pp</span> vs prior</div></div>
    <div class="tile"><span class="lbl">New 52-wk highs</span><div class="big up">{{.Digest.Breadth.New52wHigh}}</div><div class="foot">new lows <b class="down">{{.Digest.Breadth.New52wLow}}</b></div></div>
    <div class="tile"><span class="lbl">Universe scanned</span><div class="big">{{.Digest.SymbolsScanned}}</div><div class="foot">studies for {{.Acting.Tier}} tier</div></div>
  </div>

  <div class="grid">
    {{range .Digest.Sections}}
    <div class="panel">
      <div class="ph"><span>{{.Emoji}}</span><h2>{{.Title}}</h2><span class="cnt">{{len .Rows}}</span></div>
      <div class="pb">{{if .Rows}}<table>
        <thead><tr><th>Sym</th><th>Close</th><th>RSI</th><th>3-mo</th><th>$ Vol</th></tr></thead>
        <tbody>{{range .Rows}}<tr><td class="sym">{{.Symbol}}</td><td>{{num2 .Close}}</td><td>{{num1 .RSI14}}</td><td class="{{if gt0 .Ret3m}}up{{else if lt0 .Ret3m}}down{{end}}">{{retpct .Ret3m}}</td><td>{{money .DollarVol}}</td></tr>{{end}}</tbody>
      </table>{{else}}<div class="none">— none —</div>{{end}}</div>
    </div>
    {{end}}
  </div>

  <div class="panel full" style="margin-top:16px">
    <div class="ph"><span>🧪</span><h2>My Studies</h2><span class="cnt">{{len .MyStudies}}{{if gt .StudyQuota 0}} / {{.StudyQuota}}{{else}} / ∞{{end}}</span></div>
    <div class="pb">
      {{if .MyStudies}}<table>
        <thead><tr><th>Key</th><th>Title</th><th>Vis</th><th>Group</th><th>WHERE</th><th></th></tr></thead>
        <tbody>{{range .MyStudies}}<tr>
          <td class="sym">{{.Key}}</td><td>{{.Emoji}} {{.Title}}</td><td>{{.Visibility}}</td><td>{{.Group}}</td>
          <td class="mono" style="max-width:240px;overflow:hidden;text-overflow:ellipsis">{{.Where}}</td>
          <td style="white-space:nowrap">
            <button class="mini" type="button" onclick="editMine('{{.Key}}')">edit</button>
            <form method="post" action="/studies" style="display:inline"><input type="hidden" name="action" value="delete"><input type="hidden" name="key" value="{{.Key}}"><button class="mini danger" onclick="return confirm('Delete {{.Key}}?')">del</button></form>
          </td>
        </tr>{{end}}</tbody>
      </table>{{else}}<div class="none">— no studies yet —</div>{{end}}
      <form method="post" action="/studies" id="myForm" style="border-top:1px solid var(--border);padding:12px 6px;display:grid;grid-template-columns:repeat(4,1fr);gap:8px">
        <input type="hidden" name="action" value="save">
        <input name="key" id="m_key" placeholder="key (unique)" required>
        <input name="title" id="m_title" placeholder="title">
        <input name="emoji" id="m_emoji" placeholder="emoji">
        <select name="visibility" id="m_vis"><option value="private">private</option><option value="group">group (shared)</option></select>
        <input name="group" id="m_group" placeholder="group (if group)">
        <input name="order_by" id="m_order" placeholder="order_by  e.g. dollar_vol DESC" style="grid-column:span 2">
        <input name="limit" id="m_limit" type="number" placeholder="limit">
        <textarea name="where" id="m_where" placeholder="WHERE  e.g. close > sma200 AND rsi14 between 55 and 70" style="grid-column:1/-1;min-height:52px;font-family:var(--mono);padding:7px 9px;border:1px solid var(--border);border-radius:7px;background:var(--panel2);color:var(--ink)"></textarea>
        <div style="grid-column:1/-1;display:flex;gap:8px;align-items:center;flex-wrap:wrap">
          <button class="btn" type="submit">Save</button>
          <button class="btn" type="button" onclick="testMine()">Test WHERE</button>
          <button class="btn" type="button" onclick="clearMine()">New</button>
          <span id="m_result" class="mono" style="font-size:12px;color:var(--muted)"></span>
          {{if and (gt .StudyQuota 0) (ge (len .MyStudies) .StudyQuota)}}<span class="warn" style="font-size:12px">Limit reached ({{.StudyQuota}}) — upgrade for more.</span>{{end}}
        </div>
      </form>
    </div>
  </div>

  <p class="note">Live · <span class="mono">scanner serve</span> · generated {{.Digest.GeneratedAt.Format "2006-01-02 15:04 UTC"}}</p>
  <script>
    var MINE = {{studiesJSON .MyStudies}};
    function _g(x){return document.getElementById(x);}
    function editMine(k){var s=MINE.find(function(x){return x.key===k;});if(!s)return;
      _g('m_key').value=s.key;_g('m_title').value=s.title||'';_g('m_emoji').value=s.emoji||'';
      _g('m_vis').value=(s.visibility==='group'?'group':'private');_g('m_group').value=s.group||'';
      _g('m_limit').value=s.limit||'';_g('m_order').value=s.order_by||'';_g('m_where').value=s.where||'';
      _g('m_result').textContent='editing '+s.key;_g('myForm').scrollIntoView();}
    function clearMine(){_g('myForm').reset();_g('m_result').textContent='';}
    function testMine(){var b=new URLSearchParams({where:_g('m_where').value,order_by:_g('m_order').value,limit:_g('m_limit').value||'20'});
      _g('m_result').textContent='testing…';
      fetch('/studies/test',{method:'POST',body:b}).then(function(r){return r.json();}).then(function(d){
        _g('m_result').textContent=d.error?('✗ '+d.error):('✓ '+d.count+' match: '+((d.sample||[]).join(', ')||'—'));
      }).catch(function(e){_g('m_result').textContent='✗ '+e;});}
  </script>
</div>` + themeScript + `</body></html>{{end}}`

// adminSrc is the operator console: ingestion coverage, warehouse, data-quality
// watch, and the user registry.
const adminSrc = `{{define "admin"}}<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Cetus Scanner — Admin</title>{{template "styles"}}</head>
<body><div class="shell">
  <div class="top">
    <div class="brand"><span>📡</span> Cetus&nbsp;Scanner <span class="badge">ADMIN</span> <span class="led"></span></div>
    <div class="meta">
      <div><span class="lbl">Trading day</span><br><span class="v">{{.Digest.DateLabel}}</span></div>
      <div><span class="lbl">Scan time</span><br><span class="v">{{ms .ScanMillis}}</span></div>
    </div>
    <a class="btn" href="/">← Dashboard</a>
    <button class="btn" id="tgl" aria-label="Toggle theme">◐ Theme</button>
    {{if .SessionAuth}}<a class="btn" href="/logout">Sign out</a>{{end}}
  </div>

  <div class="kpis">
    <div class="tile"><span class="lbl">Universe · has data</span>
      <div class="big">{{.Stats.Count "SUCCESS"}}</div>
      <div class="foot">of {{.Stats.Total}} tracked · <b class="mono">{{num1 (.Stats.Pct "SUCCESS")}}%</b> ingested</div>
      <div class="bar" title="SUCCESS / IN_FLIGHT / PENDING / EMPTY">
        <i style="width:{{num1 (.Stats.Pct "SUCCESS")}}%;background:var(--up)"></i>
        <i style="width:{{num1 (.Stats.Pct "IN_FLIGHT")}}%;background:var(--accent)"></i>
        <i style="width:{{num1 (.Stats.Pct "PENDING")}}%;background:var(--panel2)"></i>
        <i style="width:{{num1 (.Stats.Pct "EMPTY")}}%;background:var(--warn)"></i></div></div>
    <div class="tile"><span class="lbl">Data-quality flags</span><div class="big">{{len .Flags}}</div>
      <div class="foot"><span class="down">{{.Suspect}} suspect</span> · <span class="warn">{{.Watch}} watch</span></div></div>
    <div class="tile"><span class="lbl">Above 200-DMA</span><div class="big">{{num1 .Digest.Breadth.PctAbove200}}<span style="font-size:16px">%</span></div>
      {{$d := .Digest.Breadth.DeltaAbove200}}<div class="foot"><span class="delta {{if gt0 $d}}up{{else if lt0 $d}}down{{end}}">{{num1 $d}} pp</span> vs prior</div></div>
    <div class="tile"><span class="lbl">Users</span><div class="big">{{len .Users}}</div><div class="foot">registry</div></div>
  </div>

  <div class="strip">
    <div class="c"><span class="lbl">Registry</span><div class="big">{{humanInt (int64 .Stats.RegistrySymbols)}}</div></div>
    <div class="c"><span class="lbl">EOD bars</span><div class="big">{{humanInt .Stats.EODBars}}</div></div>
    <div class="c"><span class="lbl">Split ledger</span><div class="big">{{.Stats.Splits}}</div></div>
    <div class="c"><span class="lbl">Warehouse</span><div class="big">{{dbSize .DBSizeBytes}}</div></div>
  </div>

  <div class="grid">
    <div class="panel full">
      <div class="ph"><span>🛡️</span><h2>Data-Quality Watch</h2><span class="cnt">Sentinel Tier-0 · {{len .Flags}} flagged</span></div>
      <div class="pb" style="padding:4px 12px 12px">
        {{range .Flags}}<div class="flag">
          <span class="sev {{upper .Severity}}">{{upper .Severity}}</span>
          <div><span class="name">{{.Symbol}}</span> &nbsp;<span class="why">{{.Reason}}</span></div>
          <div class="metric">3mo <span class="up">{{retpct .Ret3m}}</span> · {{money .DollarVol}} · {{ratio .Ratio200}}</div>
        </div>{{else}}<div class="none">— no anomalies —</div>{{end}}
        <div class="note" style="text-align:left">Tier-0 deterministic flags. Canonical fixes belong upstream in the pipeline (fix once); this panel only surfaces.</div>
      </div>
    </div>

    <div class="panel full">
      <div class="ph"><span>👥</span><h2>Users</h2><span class="cnt">{{len .Users}}</span></div>
      <div class="pb"><table>
        <thead><tr><th>ID</th><th>Name</th><th>Tier</th><th>Role</th><th>Groups</th><th>Manage</th></tr></thead>
        <tbody>{{range .Users}}<tr>
          <td class="sym">{{.ID}}{{if .Disabled}} <span class="off">off</span>{{end}}</td>
          <td>{{.Name}}</td><td>{{.Tier}}</td><td>{{.Role}}</td>
          <td><form method="post" action="/admin/users" style="display:inline;white-space:nowrap">
            <input type="hidden" name="action" value="set-groups"><input type="hidden" name="id" value="{{.ID}}">
            <input name="groups" value="{{join .Groups}}" placeholder="a, b" style="width:84px">
            <button class="mini" type="submit">set</button>
          </form></td>
          <td><form method="post" action="/admin/users" style="display:inline;white-space:nowrap">
            <input type="hidden" name="id" value="{{.ID}}">
            {{if .Disabled}}<button class="mini" name="action" value="enable">Enable</button>{{else}}<button class="mini" name="action" value="disable">Disable</button>{{end}}
            <button class="mini" name="action" value="{{if eq .Tier "pro"}}set-free{{else}}set-pro{{end}}">{{if eq .Tier "pro"}}→free{{else}}→pro{{end}}</button>
            <button class="mini" name="action" value="{{if .IsAdmin}}set-user{{else}}set-admin{{end}}">{{if .IsAdmin}}→user{{else}}→admin{{end}}</button>
            <button class="mini danger" name="action" value="delete" onclick="return confirm('Delete {{.ID}}?')">del</button>
          </form></td>
        </tr>{{end}}</tbody>
      </table>
      <form method="post" action="/admin/users" style="display:flex;gap:8px;flex-wrap:wrap;align-items:center;padding:12px 8px 6px;border-top:1px solid var(--border)">
        <input type="hidden" name="action" value="create">
        <input name="id" placeholder="id" style="width:90px" required>
        <input name="name" placeholder="name" style="width:120px">
        <input name="password" type="password" placeholder="password" style="width:120px">
        <input name="groups" placeholder="groups (csv)" style="width:110px">
        <select name="tier"><option value="free">free</option><option value="pro">pro</option></select>
        <select name="role"><option value="user">user</option><option value="admin">admin</option></select>
        <button class="btn" type="submit">Create user</button>
      </form></div>
    </div>

    <div class="panel full">
      <div class="ph"><span>🧪</span><h2>Studies</h2><span class="cnt">{{len .Studies}}</span></div>
      <div class="pb">
        <table>
          <thead><tr><th>Key</th><th>Title</th><th>Owner</th><th>Vis</th><th>Tier</th><th>Group</th><th>WHERE</th><th></th></tr></thead>
          <tbody>{{range .Studies}}<tr>
            <td class="sym">{{.Key}}</td><td>{{.Emoji}} {{.Title}}</td><td>{{.Owner}}</td><td>{{.Visibility}}</td><td>{{.Tier}}</td><td>{{.Group}}</td>
            <td class="mono" style="max-width:240px;overflow:hidden;text-overflow:ellipsis">{{.Where}}</td>
            <td style="white-space:nowrap">
              <button class="mini" type="button" onclick="editStudy('{{.Key}}')">edit</button>
              <form method="post" action="/studies" style="display:inline"><input type="hidden" name="action" value="delete"><input type="hidden" name="key" value="{{.Key}}"><button class="mini danger" onclick="return confirm('Delete {{.Key}}?')">del</button></form>
            </td>
          </tr>{{end}}</tbody>
        </table>
        <form method="post" action="/studies" id="studyForm" style="border-top:1px solid var(--border);padding:12px 6px;display:grid;grid-template-columns:repeat(4,1fr);gap:8px">
          <input type="hidden" name="action" value="save">
          <input name="key" id="s_key" placeholder="key (unique)" required>
          <input name="title" id="s_title" placeholder="title">
          <input name="emoji" id="s_emoji" placeholder="emoji">
          <input name="owner" id="s_owner" placeholder="owner (blank=global)">
          <select name="visibility" id="s_vis"><option value="public">public</option><option value="group">group</option><option value="private">private</option></select>
          <input name="group" id="s_group" placeholder="group">
          <select name="tier" id="s_tier"><option value="free">free</option><option value="pro">pro</option></select>
          <input name="limit" id="s_limit" type="number" placeholder="limit">
          <input name="order_by" id="s_order" placeholder="order_by  e.g. dollar_vol DESC" style="grid-column:1/-1">
          <textarea name="where" id="s_where" placeholder="WHERE  e.g. close > sma200 AND rsi14 between 55 and 70" style="grid-column:1/-1;min-height:52px;font-family:var(--mono);padding:7px 9px;border:1px solid var(--border);border-radius:7px;background:var(--panel2);color:var(--ink)"></textarea>
          <div style="grid-column:1/-1;display:flex;gap:8px;align-items:center;flex-wrap:wrap">
            <button class="btn" type="submit">Save study</button>
            <button class="btn" type="button" onclick="testStudy()">Test WHERE</button>
            <button class="btn" type="button" onclick="clearStudy()">New</button>
            <span id="s_result" class="mono" style="font-size:12px;color:var(--muted)"></span>
          </div>
        </form>
      </div>
    </div>
  </div>
  <p class="note">Admin · <span class="mono">scanner serve</span> · generated {{.Digest.GeneratedAt.Format "2006-01-02 15:04 UTC"}}</p>
  <script>
    var STUDIES = {{studiesJSON .Studies}};
    function _g(x){return document.getElementById(x);}
    function editStudy(k){var s=STUDIES.find(function(x){return x.key===k;});if(!s)return;
      _g('s_key').value=s.key;_g('s_title').value=s.title||'';_g('s_emoji').value=s.emoji||'';_g('s_owner').value=s.owner||'';
      _g('s_vis').value=s.visibility||'public';_g('s_group').value=s.group||'';_g('s_tier').value=s.tier||'free';
      _g('s_limit').value=s.limit||'';_g('s_order').value=s.order_by||'';_g('s_where').value=s.where||'';
      _g('s_result').textContent='editing '+s.key;_g('studyForm').scrollIntoView();}
    function clearStudy(){_g('studyForm').reset();_g('s_result').textContent='';}
    function testStudy(){var b=new URLSearchParams({where:_g('s_where').value,order_by:_g('s_order').value,limit:_g('s_limit').value||'20'});
      _g('s_result').textContent='testing…';
      fetch('/studies/test',{method:'POST',body:b}).then(function(r){return r.json();}).then(function(d){
        _g('s_result').textContent=d.error?('✗ '+d.error):('✓ '+d.count+' match: '+((d.sample||[]).join(', ')||'—'));
      }).catch(function(e){_g('s_result').textContent='✗ '+e;});}
  </script>
</div>` + themeScript + `</body></html>{{end}}`

// loginSrc is the sign-in page.
const loginSrc = `{{define "login"}}<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Cetus Scanner — Sign in</title>{{template "styles"}}</head>
<body><div class="shell" style="max-width:380px">
  <div class="panel" style="margin-top:12vh">
    <div class="ph"><span>📡</span><h2>Cetus Scanner — Sign in</h2></div>
    <div class="pb" style="padding:18px">
      {{if .Error}}<div style="color:var(--down);font-size:13px;margin-bottom:12px">{{.Error}}</div>{{end}}
      <form method="post" action="/login">
        <label class="lbl">User</label><br>
        <input name="user" autofocus autocomplete="username" style="width:100%;margin:5px 0 12px"><br>
        <label class="lbl">Password</label><br>
        <input name="password" type="password" autocomplete="current-password" style="width:100%;margin:5px 0 16px"><br>
        <button class="btn" type="submit" style="width:100%;padding:9px">Sign in</button>
      </form>
      <div class="note" style="text-align:left;margin-top:14px">Users: {{range .Users}}<b>{{.ID}}</b> · {{end}}<span style="color:var(--faint)">(dev: password = id)</span></div>
    </div>
  </div>
</div></body></html>{{end}}`
