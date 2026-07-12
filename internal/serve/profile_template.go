package serve

const profileHTML = `<!doctype html><html><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>Profile — Cetus Scanner</title>
<style>
:root{--bg:#f8fafc;--card:#fff;--br:#e2e8f0;--tx:#0f172a;--mu:#64748b;--pr:#3b82f6;--er:#ef4444;--ok:#10b981;--rd:8px}
*{margin:0;padding:0;box-sizing:border-box}
body{font:16px system-ui,sans-serif;background:var(--bg);color:var(--tx);display:flex;justify-content:center;padding-top:40px}
.c{background:var(--card);border:1px solid var(--br);border-radius:var(--rd);padding:24px;width:100%%;max-width:400px}
h2{margin-bottom:16px}.fh{margin-bottom:12px}.fh label{display:block;font-size:13px;color:var(--mu);margin-bottom:4px}
.fh input{width:100%%;padding:8px 12px;border:1px solid var(--br);border-radius:var(--rd);font-size:15px}
.fh input:focus{outline:none;border-color:var(--pr)}
.btn{width:100%%;padding:10px;border:none;border-radius:var(--rd);font-size:15px;cursor:pointer;font-weight:600;background:var(--pr);color:#fff}
.err{background:var(--er);color:#fff;padding:10px;border-radius:var(--rd);margin-bottom:12px;font-size:14px}
.ok{background:var(--ok);color:#fff;padding:10px;border-radius:var(--rd);margin-bottom:12px;font-size:14px}
.lnk{text-align:center;margin-top:12px;font-size:14px}.lnk a{color:var(--pr);text-decoration:none}
.st{height:4px;background:#e2e8f0;border-radius:2px;margin-bottom:8px;overflow:hidden}
.stb{height:100%%;transition:width .2s,background .2s}
</style></head><body><div class="c"><h2>%s — Change Password</h2>%s%s
<form method="post">
<div class="fh"><label>Current Password</label><input name="current_password" type="password" required></div>
<div class="fh"><label>New Password</label><input name="new_password" type="password" id="np" oninput="up()" required></div>
<div class="st"><div id="sb" class="stb" style="width:0%%"></div></div>
<div class="fh"><label>Confirm</label><input name="confirm_password" type="password" required></div>
<button class="btn">Change Password</button>
</form><div class="lnk"><a href="/">← Dashboard</a></div></div>
<script>
function up(){var p=document.getElementById('np').value,s=0;if(p){s=p.length*4;if(/[A-Z]/.test(p))s+=10;if(/[a-z]/.test(p))s+=10;if(/[0-9]/.test(p))s+=15;if(/[^A-Za-z0-9]/.test(p))s+=20;if(p.length>=12)s+=10;s=Math.min(s,100)}var b=document.getElementById('sb');b.style.width=s+'%%';b.style.background=s>=80?'#10b981':s>=60?'#3b82f6':s>=40?'#f59e0b':'#ef4444'}
</script></body></html>`
