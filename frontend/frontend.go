package frontend

import (
	"html/template"
	"log"
	"net/http"
)

var indexTmpl = template.Must(template.New("index").Parse(`<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <title>Minecraft Online Players</title>
  <meta name="viewport" content="width=device-width,initial-scale=1" />
  <style>
    :root{
      --bg1:#0f1720; /* dark blue */
      --bg2:#081018;
      --panel:#0b2a16; /* dark green panel */
      --panel2:#13391f;
      --accent:#96c93d; /* green accent */
      --accent-dark:#6aa02a;
      --muted:#c7d0c2;
      --tile:#082216;
      --offline:#2b2b2b;
      --glass: rgba(255,255,255,0.03);
    }
    html,body{height:100%;margin:0;font-family: 'Segoe UI', Roboto, Arial, sans-serif;background:linear-gradient(180deg,var(--bg1),var(--bg2));color:var(--muted)}
    .wrap{max-width:960px;margin:28px auto;padding:20px;background:linear-gradient(180deg,var(--panel),var(--panel2));border:6px solid #090a0b;box-shadow:0 10px 30px rgba(0,0,0,0.6);border-radius:6px}
    header{display:flex;align-items:center;gap:16px}
    .logo{width:72px;height:72px;background:linear-gradient(90deg,#fff 0%,#dcdcdc 100%);border:4px solid #000;box-shadow:inset 0 -6px 0 rgba(0,0,0,0.2);display:flex;align-items:center;justify-content:center;font-weight:700;color:#000;font-size:20px}
    h1{margin:0;font-family:monospace;font-size:28px;letter-spacing:1px;text-transform:uppercase;color:var(--accent)}
    .controls{margin-left:auto;display:flex;gap:8px}
    button.btn{background:linear-gradient(180deg,var(--accent),var(--accent-dark));color:#07210a;border:2px solid #032004;padding:8px 12px;border-radius:4px;cursor:pointer;font-weight:700}
    button.btn.ghost{background:transparent;border:2px solid rgba(255,255,255,0.06);color:var(--muted);font-weight:600}

    .section{margin-top:20px}
    .players-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(180px,1fr));gap:12px}
    .tile{background:linear-gradient(180deg,var(--tile),#062017);padding:12px;border:3px solid rgba(0,0,0,0.7);border-radius:6px;box-shadow:inset 0 -8px 0 rgba(0,0,0,0.12)}
    .player-name{font-family:monospace;font-weight:700;color:var(--accent);}    
    .player-status{margin-top:6px;color:#bfe7a7;font-weight:700}

    .offline-tile{background:linear-gradient(180deg,#222,#171717);border:3px solid rgba(255,255,255,0.03);color:#ddd}
    .small{font-size:13px;color:#a7b3a3}

    .collapsible{display:block;margin-top:12px;background:transparent;border:none;color:var(--accent);text-decoration:underline;cursor:pointer;padding:0}

    .lastseen-wrap{display:block;margin-top:12px;padding:12px;border-left:4px solid rgba(255,255,255,0.02);}    
    .lastseen-list{display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:10px}
    .lastseen-item{padding:10px;border-radius:6px;background:linear-gradient(180deg,#101010,#0b0b0b);border:2px solid rgba(255,255,255,0.03)}

    footer{margin-top:18px;color:#7f8a77;font-size:13px}

    @media (max-width:520px){.players-grid{grid-template-columns:repeat(2,1fr)}.logo{width:56px;height:56px}}
  </style>
</head>
<body>
  <div class="wrap">
    <header>
      <div class="logo">MC</div>
      <div>
        <h1>Minecraft players</h1>
      </div>
      <div class="controls">
        <button id="refresh" class="btn">Refresh</button>
      </div>
    </header>

    <section class="section">
      <h2 style="margin:8px 0 12px 0;color:var(--muted)">Online players</h2>
      <div id="online" class="players-grid">
        <div class="tile">Loading...</div>
      </div>
    </section>

    <section class="section">
      <div id="lastseen" class="lastseen-wrap">
        <h3 style="margin-top:0;color:var(--muted)">Offline players (last seen)</h3>
        <div id="offline-list" class="lastseen-list">Loading...</div>
      </div>
    </section>
  </div>

  <script>
    function timeAgo(iso){
      if (!iso) return 'unknown';
      const d = new Date(iso);
      if (isNaN(d)) return iso;
      const s = Math.floor((Date.now() - d.getTime())/1000);
      if (s < 10) return 'just now';
      if (s < 60) return s + 's ago';
      const m = Math.floor(s/60);
      if (m < 60) return m + 'm ago';
      const h = Math.floor(m/60);
      if (h < 24) return h + 'h ago';
      const days = Math.floor(h/24);
      return days + 'd ago';
    }

    let onlinePlayers = [];

    async function fetchOnline(){
      try {
        const [onlineRes, lastSeenRes] = await Promise.all([fetch('/online-players'), fetch('/last-seen')]);

        if (!onlineRes.ok) {
          renderOnlineError();
        } else {
          const players = await onlineRes.json();
          onlinePlayers = players || [];
          renderOnline();
        }

        if (!lastSeenRes.ok) {
          document.getElementById('offline-list').textContent = 'Failed to load last-seen data';
        } else {
          const data = await lastSeenRes.json();
          renderOffline(data);
        }
      } catch (e) {
        renderOnlineError();
        document.getElementById('offline-list').textContent = 'Failed to load last-seen data';
      }
    }

    function renderOnline(){
      const c = document.getElementById('online');
      c.innerHTML = '';
      if (!onlinePlayers || onlinePlayers.length === 0){
        const t = document.createElement('div');
        t.className = 'tile offline-tile';
        t.textContent = 'No players online';
        c.appendChild(t);
        return;
      }
      onlinePlayers.forEach(p => {
        const t = document.createElement('div');
        t.className = 'tile';
        const name = document.createElement('div');
        name.className = 'player-name';
        name.textContent = p;
        const status = document.createElement('div');
        status.className = 'player-status';
        status.textContent = 'Online';
        t.appendChild(name);
        t.appendChild(status);
        c.appendChild(t);
      });
    }

    function renderOnlineError(){
      const c = document.getElementById('online');
      c.innerHTML = '';
      const t = document.createElement('div');
      t.className = 'tile offline-tile';
      t.textContent = 'Failed to load online players';
      c.appendChild(t);
    }

    function renderOffline(data){
      const container = document.getElementById('offline-list');
      container.innerHTML = '';
      const onlineSet = new Set(onlinePlayers);
      let items = [];
      if (Array.isArray(data)){
        items = data;
      } else if (data && typeof data === 'object'){
        const keys = Object.keys(data).sort((a,b)=>a.localeCompare(b));
        items = keys.map(k => ({name: k, last_seen: data[k]}));
      }

      let any=false;
      items.forEach(it => {
        const name = it.name || it[0] || '';
        const last = it.last_seen || it[1] || it.lastSeen || '';
        if (!name) return;
        if (onlineSet.has(name)) return;
        any=true;
        const item = document.createElement('div');
        item.className = 'lastseen-item';
        const n = document.createElement('div');
        n.className = 'player-name';
        n.textContent = name;
        const when = document.createElement('div');
        when.className = 'small';
        when.textContent = 'last seen: '+ timeAgo(last) + ' (' + last + ')';
        item.appendChild(n);
        item.appendChild(when);
        container.appendChild(item);
      });
      if (!any){
        container.textContent = 'No offline players.';
      }
    }

    document.getElementById('refresh').addEventListener('click', async ()=>{
      document.getElementById('offline-list').textContent = 'Loading...';
      await fetchOnline();
    });

    fetchOnline();
    setInterval(fetchOnline,60000);
  </script>
</body>
</html>
`))

func IndexHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := indexTmpl.Execute(w, nil); err != nil {
			log.Println("Failed to render index template:", err)
		}
	}
}
