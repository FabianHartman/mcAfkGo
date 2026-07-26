function timeAgo(iso) {
  if (!iso) return 'unknown';
  const d = new Date(iso);
  if (isNaN(d)) return iso;
  const s = Math.floor((Date.now() - d.getTime()) / 1000);
  if (s < 10) return 'just now';
  if (s < 60) return s + 's ago';
  const m = Math.floor(s / 60);
  if (m < 60) return m + 'm ago';
  const h = Math.floor(m / 60);
  if (h < 24) return h + 'h ago';
  const days = Math.floor(h / 24);
  return days + 'd ago';
}

let onlinePlayers = [];

async function fetchOnline() {
  try {
    const [onlineRes, lastSeenRes] = await Promise.all([
      fetch('/online-players'),
      fetch('/last-seen')
    ]);

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

function renderOnline() {
  const c = document.getElementById('online');
  c.innerHTML = '';
  if (!onlinePlayers || onlinePlayers.length === 0) {
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

function renderOnlineError() {
  const c = document.getElementById('online');
  c.innerHTML = '';
  const t = document.createElement('div');
  t.className = 'tile offline-tile';
  t.textContent = 'Failed to load online players';
  c.appendChild(t);
}

function renderOffline(data) {
  const container = document.getElementById('offline-list');
  container.innerHTML = '';
  const onlineSet = new Set(onlinePlayers);
  let items = [];
  if (Array.isArray(data)) {
    items = data;
  } else if (data && typeof data === 'object') {
    const keys = Object.keys(data).sort((a, b) => a.localeCompare(b));
    items = keys.map(k => ({ name: k, last_seen: data[k] }));
  }

  let any = false;
  items.forEach(it => {
    const name = it.name || it[0] || '';
    const last = it.last_seen || it[1] || it.lastSeen || '';
    if (!name) return;
    if (onlineSet.has(name)) return;
    any = true;
    const item = document.createElement('div');
    item.className = 'lastseen-item';
    const n = document.createElement('div');
    n.className = 'player-name';
    n.textContent = name;
    const when = document.createElement('div');
    when.className = 'small';
    when.textContent = 'last seen: ' + timeAgo(last) + ' (' + last + ')';
    item.appendChild(n);
    item.appendChild(when);
    container.appendChild(item);
  });
  if (!any) {
    container.textContent = 'No offline players.';
  }
}

document.getElementById('refresh').addEventListener('click', async () => {
  document.getElementById('offline-list').textContent = 'Loading...';
  await fetchOnline();
});

fetchOnline();
setInterval(fetchOnline, 60000);
