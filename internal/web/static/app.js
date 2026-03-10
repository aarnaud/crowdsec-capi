// ── Auth ──────────────────────────────────────────────────────────────────────

let authHeader = null;

function savedCreds() {
  const u = localStorage.getItem('admin_user');
  const p = localStorage.getItem('admin_pass');
  return (u && p) ? btoa(u + ':' + p) : null;
}

async function doLogin() {
  const u = document.getElementById('login-user').value.trim();
  const p = document.getElementById('login-pass').value;
  const hdr = 'Basic ' + btoa(u + ':' + p);
  try {
    const r = await fetch('/admin/stats', { headers: { Authorization: hdr } });
    if (!r.ok) throw new Error('bad credentials');
    localStorage.setItem('admin_user', u);
    localStorage.setItem('admin_pass', p);
    authHeader = hdr;
    document.getElementById('login-overlay').classList.remove('open');
    document.getElementById('header-user').textContent = u;
    loadPage('dashboard');
  } catch {
    document.getElementById('login-error').textContent = 'Invalid credentials.';
  }
}

document.getElementById('login-pass').addEventListener('keydown', e => {
  if (e.key === 'Enter') doLogin();
});

function logout() {
  localStorage.removeItem('admin_user');
  localStorage.removeItem('admin_pass');
  authHeader = null;
  document.getElementById('login-overlay').classList.add('open');
  document.getElementById('login-error').textContent = '';
}

// ── API ───────────────────────────────────────────────────────────────────────

async function api(method, path, body) {
  const opts = { method, headers: { Authorization: authHeader } };
  if (body !== undefined) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }
  const r = await fetch(path, opts);
  if (r.status === 204) return null;
  const text = await r.text();
  if (!r.ok) throw new Error(text || `HTTP ${r.status}`);
  return text ? JSON.parse(text) : null;
}

// ── Navigation ────────────────────────────────────────────────────────────────

const pages = ['dashboard', 'machines', 'decisions', 'allowlists', 'enrollment', 'upstream'];

function showPage(name) {
  pages.forEach(p => {
    document.getElementById('page-' + p).classList.toggle('active', p === name);
  });
  document.querySelectorAll('nav button').forEach((btn, i) => {
    btn.classList.toggle('active', pages[i] === name);
  });
  loadPage(name);
}

function loadPage(name) {
  if (name === 'dashboard')  loadDashboard();
  if (name === 'machines')   loadMachines();
  if (name === 'decisions')  loadDecisions();
  if (name === 'allowlists') loadAllowlists();
  if (name === 'enrollment') loadEnrollmentKeys();
  if (name === 'upstream')   loadUpstream();
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

let mapInstance = null;
let originChart = null;
let typeChart   = null;
let worldMapRegistered = false;

async function loadDashboard() {
  try {
    const [stats, upstream] = await Promise.all([
      api('GET', '/admin/stats'),
      api('GET', '/admin/upstream').catch(() => null),
    ]);

    // Stat cards
    document.getElementById('s-machines').textContent  = stats.machines.total;
    document.getElementById('s-machines-sub').textContent =
      `${stats.machines.validated} active · ${stats.machines.pending} pending · ${stats.machines.blocked} blocked`;
    document.getElementById('s-decisions').textContent = stats.decisions.total;
    const byOrigin = stats.decisions.by_origin || {};
    document.getElementById('s-decisions-sub').textContent =
      Object.entries(byOrigin).map(([k,v]) => `${v} ${k}`).join(' · ') || 'none';
    document.getElementById('s-signals').textContent   = stats.signals_last_24h;
    document.getElementById('s-upstream').textContent  = upstream ? (upstream.decision_count || 0) : '—';

    // Donut: by origin
    renderDonut('chart-origin', originChart, Object.entries(byOrigin).map(([name, value]) => ({ name, value })), [
      '#2f7ff5','#e63946','#2a9d5c','#f4a261','#9b59b6',
    ], chart => { originChart = chart; });

    // Donut: by type
    const byType = stats.decisions.by_type || {};
    renderDonut('chart-type', typeChart, Object.entries(byType).map(([name, value]) => ({ name, value })), [
      '#e63946','#f4a261','#2a9d5c',
    ], chart => { typeChart = chart; });

    // World map
    await renderWorldMap(stats.signals_by_country || []);

  } catch (e) {
    console.error('Dashboard error:', e);
  }
}

function renderDonut(elId, existing, data, colors, setter) {
  if (existing) existing.dispose();
  const el = document.getElementById(elId);
  const chart = echarts.init(el);
  setter(chart);
  chart.setOption({
    color: colors,
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    legend: { orient: 'vertical', right: 10, top: 'center', textStyle: { fontSize: 12 } },
    series: [{
      type: 'pie',
      radius: ['45%', '70%'],
      center: ['38%', '50%'],
      avoidLabelOverlap: true,
      label: { show: false },
      emphasis: { label: { show: true, fontSize: 14, fontWeight: 'bold' } },
      data: data.length ? data : [{ name: 'none', value: 0 }],
    }],
  });
}

async function renderWorldMap(countryData) {
  if (!worldMapRegistered) {
    try {
      const resp = await fetch('world.json');
      const worldJson = await resp.json();
      echarts.registerMap('world', worldJson);
      worldMapRegistered = true;
    } catch (e) {
      document.getElementById('worldmap').innerHTML =
        '<p style="color:#aaa;text-align:center;padding:40px">World map unavailable (requires CDN access)</p>';
      return;
    }
  }

  if (mapInstance) mapInstance.dispose();
  const el = document.getElementById('worldmap');
  mapInstance = echarts.init(el);

  const maxCount = countryData.reduce((m, d) => Math.max(m, d.count), 1);
  const mapData  = countryData.map(d => ({ name: d.country, value: d.count }));

  mapInstance.setOption({
    backgroundColor: '#fff',
    tooltip: {
      trigger: 'item',
      formatter: p => p.value ? `${p.name}: ${p.value.toLocaleString()} signals` : p.name,
    },
    visualMap: {
      min: 0,
      max: maxCount,
      left: 'left',
      bottom: 20,
      text: ['High', 'Low'],
      inRange: { color: ['#fde8ea', '#e63946'] },
      calculable: true,
      textStyle: { fontSize: 11 },
    },
    series: [{
      type: 'map',
      map: 'world',
      roam: true,
      emphasis: {
        label: { show: true, fontSize: 11 },
        itemStyle: { areaColor: '#c0392b' },
      },
      itemStyle: {
        areaColor: '#f0f2f5',
        borderColor: '#ccc',
        borderWidth: 0.5,
      },
      data: mapData,
    }],
  });

  window.addEventListener('resize', () => mapInstance && mapInstance.resize());
}

// ── Machines ──────────────────────────────────────────────────────────────────

async function loadMachines() {
  const tbody = document.getElementById('machines-body');
  try {
    const machines = await api('GET', '/admin/machines') || [];
    document.getElementById('machines-count').textContent = machines.length + ' machine(s)';
    if (!machines.length) {
      tbody.innerHTML = '<tr><td colspan="6" class="empty">No machines registered yet.</td></tr>';
      return;
    }
    tbody.innerHTML = machines.map(m => {
      const statusClass = { validated: 'badge-green', pending: 'badge-yellow', blocked: 'badge-red' }[m.status] || 'badge-gray';
      const tags = (m.tags || []).map(t => `<span class="tag">${esc(t)}</span>`).join('');
      const name = m.name ? `<strong>${esc(m.name)}</strong> ` : '';
      return `<tr>
        <td class="mono">${esc(m.machine_id)}</td>
        <td><span class="badge ${statusClass}">${m.status}</span></td>
        <td>${name}${tags}</td>
        <td class="mono text-muted">${m.ip_address || ''}</td>
        <td class="text-muted">${m.last_seen_at ? relTime(m.last_seen_at) : 'never'}</td>
        <td><div class="actions">
          ${m.status !== 'blocked'
            ? `<button class="btn btn-warn btn-sm" onclick="blockMachine('${esc(m.machine_id)}')">Block</button>`
            : `<button class="btn btn-secondary btn-sm" onclick="unblockMachine('${esc(m.machine_id)}')">Unblock</button>`}
          <button class="btn btn-danger btn-sm" onclick="deleteMachine('${esc(m.machine_id)}')">Delete</button>
        </div></td>
      </tr>`;
    }).join('');
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="6" class="empty">Error: ${esc(e.message)}</td></tr>`;
  }
}

async function blockMachine(id) {
  if (!confirm(`Block machine ${id}?`)) return;
  try { await api('PUT', `/admin/machines/${encodeURIComponent(id)}/block`); loadMachines(); }
  catch (e) { alert('Error: ' + e.message); }
}

async function unblockMachine(id) {
  if (!confirm(`Unblock machine ${id}?`)) return;
  try { await api('PUT', `/admin/machines/${encodeURIComponent(id)}/unblock`); loadMachines(); }
  catch (e) { alert('Error: ' + e.message); }
}

async function deleteMachine(id) {
  if (!confirm(`Delete machine ${id}? This cannot be undone.`)) return;
  try { await api('DELETE', `/admin/machines/${encodeURIComponent(id)}`); loadMachines(); }
  catch (e) { alert('Error: ' + e.message); }
}

// ── Decisions ─────────────────────────────────────────────────────────────────

async function loadDecisions() {
  const tbody = document.getElementById('decisions-body');
  try {
    const decisions = await api('GET', '/admin/decisions') || [];
    if (!decisions.length) {
      tbody.innerHTML = '<tr><td colspan="7" class="empty">No active decisions.</td></tr>';
      return;
    }
    tbody.innerHTML = decisions.map(d => {
      const originClass = { manual: 'badge-blue', 'local-signal': 'badge-green', 'upstream-capi': 'badge-gray' }[d.origin] || 'badge-gray';
      const typeClass   = d.type === 'ban' ? 'badge-red' : 'badge-yellow';
      return `<tr>
        <td><strong>${esc(d.value)}</strong></td>
        <td>${esc(d.scope)}</td>
        <td><span class="badge ${typeClass}">${esc(d.type)}</span></td>
        <td><span class="badge ${originClass}">${esc(d.origin)}</span></td>
        <td class="text-muted">${d.scenario ? esc(d.scenario) : '—'}</td>
        <td class="text-muted">${d.expires_at ? relTime(d.expires_at) : '—'}</td>
        <td><button class="btn btn-danger btn-sm" onclick="deleteDecision('${esc(d.uuid)}')">Delete</button></td>
      </tr>`;
    }).join('');
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="7" class="empty">Error: ${esc(e.message)}</td></tr>`;
  }
}

function openAddDecision() {
  document.getElementById('dec-value').value = '';
  document.getElementById('dec-scenario').value = '';
  document.getElementById('dec-duration').value = '24h';
  openModal('modal-decision');
}

async function submitDecision() {
  const body = {
    type:     document.getElementById('dec-type').value,
    scope:    document.getElementById('dec-scope').value,
    value:    document.getElementById('dec-value').value.trim(),
    duration: document.getElementById('dec-duration').value.trim() || '24h',
    scenario: document.getElementById('dec-scenario').value.trim(),
  };
  if (!body.value) { alert('Value is required'); return; }
  try { await api('POST', '/admin/decisions', body); closeModal('modal-decision'); loadDecisions(); }
  catch (e) { alert('Error: ' + e.message); }
}

async function deleteDecision(uuid) {
  if (!confirm('Delete this decision?')) return;
  try { await api('DELETE', `/admin/decisions/${uuid}`); loadDecisions(); }
  catch (e) { alert('Error: ' + e.message); }
}

// ── Allowlists ────────────────────────────────────────────────────────────────

let selectedAllowlistID = null;

async function loadAllowlists() {
  const tbody = document.getElementById('allowlists-body');
  try {
    const lists = await api('GET', '/admin/allowlists') || [];
    if (!lists.length) {
      tbody.innerHTML = '<tr><td colspan="4" class="empty">No allowlists yet.</td></tr>';
      document.getElementById('allowlist-entries-card').style.display = 'none';
      return;
    }
    tbody.innerHTML = lists.map(l => `<tr>
      <td><strong>${esc(l.name)}</strong>${l.managed ? ' <span class="badge badge-blue" title="Managed by allowlists file">managed</span>' : ''}</td>
      <td>${l.label ? esc(l.label) : '<span class="text-muted">—</span>'}</td>
      <td class="text-muted">${l.description ? esc(l.description) : '—'}</td>
      <td><div class="actions">
        <button class="btn btn-secondary btn-sm" onclick="selectAllowlist(${l.id}, '${esc(l.name)}')">Entries</button>
        ${l.managed
          ? '<span class="text-muted" title="Remove from allowlists file to delete" style="font-size:11px">file-managed</span>'
          : `<button class="btn btn-danger btn-sm" onclick="deleteAllowlist(${l.id})">Delete</button>`}
      </div></td>
    </tr>`).join('');
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="4" class="empty">Error: ${esc(e.message)}</td></tr>`;
  }
}

function openAddAllowlist() {
  document.getElementById('al-name').value = '';
  document.getElementById('al-label').value = '';
  document.getElementById('al-desc').value = '';
  openModal('modal-allowlist');
}

async function submitAllowlist() {
  const body = { name: document.getElementById('al-name').value.trim(), label: document.getElementById('al-label').value.trim(), description: document.getElementById('al-desc').value.trim() };
  if (!body.name) { alert('Name is required'); return; }
  try { await api('POST', '/admin/allowlists', body); closeModal('modal-allowlist'); loadAllowlists(); }
  catch (e) { alert('Error: ' + e.message); }
}

async function deleteAllowlist(id) {
  if (!confirm('Delete this allowlist and all its entries?')) return;
  try { await api('DELETE', `/admin/allowlists/${id}`); document.getElementById('allowlist-entries-card').style.display = 'none'; loadAllowlists(); }
  catch (e) { alert('Error: ' + e.message); }
}

function selectAllowlist(id, name) {
  selectedAllowlistID = id;
  document.getElementById('allowlist-entries-name').textContent = name;
  document.getElementById('allowlist-entries-card').style.display = 'block';
  document.getElementById('allowlist-entries-body').innerHTML =
    '<tr><td colspan="3" class="empty text-muted">Use "Add Entry" to add IPs/ranges to this allowlist.</td></tr>';
}

function openAddEntry() {
  if (!selectedAllowlistID) return;
  document.getElementById('entry-value').value = '';
  document.getElementById('entry-comment').value = '';
  openModal('modal-entry');
}

async function submitEntry() {
  const body = { scope: document.getElementById('entry-scope').value, value: document.getElementById('entry-value').value.trim(), comment: document.getElementById('entry-comment').value.trim() };
  if (!body.value) { alert('Value is required'); return; }
  try { await api('POST', `/admin/allowlists/${selectedAllowlistID}/entries`, body); closeModal('modal-entry'); alert('Entry added.'); }
  catch (e) { alert('Error: ' + e.message); }
}

// ── Enrollment Keys ───────────────────────────────────────────────────────────

async function loadEnrollmentKeys() {
  const tbody = document.getElementById('enrollment-body');
  try {
    const keys = await api('GET', '/admin/enrollment-keys') || [];
    if (!keys.length) { tbody.innerHTML = '<tr><td colspan="6" class="empty">No enrollment keys.</td></tr>'; return; }
    tbody.innerHTML = keys.map(k => {
      const uses    = k.max_uses ? `${k.use_count} / ${k.max_uses}` : `${k.use_count} / ∞`;
      const expired = k.expires_at && new Date(k.expires_at) < new Date();
      const tags    = (k.tags || []).map(t => `<span class="tag">${esc(t)}</span>`).join('');
      return `<tr>
        <td class="mono" style="font-size:11px">${esc(k.key)}</td>
        <td>${k.description ? esc(k.description) : '<span class="text-muted">—</span>'}</td>
        <td>${tags || '<span class="text-muted">—</span>'}</td>
        <td class="text-muted">${uses}</td>
        <td class="text-muted">${k.expires_at ? (expired ? '<span class="badge badge-red">expired</span>' : relTime(k.expires_at)) : '∞'}</td>
        <td><button class="btn btn-danger btn-sm" onclick="deleteKey(${k.id})">Revoke</button></td>
      </tr>`;
    }).join('');
  } catch (e) { tbody.innerHTML = `<tr><td colspan="6" class="empty">Error: ${esc(e.message)}</td></tr>`; }
}

function openAddKey() {
  document.getElementById('key-desc').value = '';
  document.getElementById('key-tags').value = '';
  document.getElementById('key-max').value  = '';
  openModal('modal-key');
}

async function submitKey() {
  const tagsRaw = document.getElementById('key-tags').value.trim();
  const maxRaw  = document.getElementById('key-max').value.trim();
  const body = { description: document.getElementById('key-desc').value.trim(), tags: tagsRaw ? tagsRaw.split(',').map(t => t.trim()).filter(Boolean) : [], max_uses: maxRaw ? parseInt(maxRaw, 10) : null };
  try {
    const result = await api('POST', '/admin/enrollment-keys', body);
    closeModal('modal-key'); loadEnrollmentKeys();
    if (result && result.key) prompt('Enrollment key (copy it now — not shown again):', result.key);
  } catch (e) { alert('Error: ' + e.message); }
}

async function deleteKey(id) {
  if (!confirm('Revoke this enrollment key?')) return;
  try { await api('DELETE', `/admin/enrollment-keys/${id}`); loadEnrollmentKeys(); }
  catch (e) { alert('Error: ' + e.message); }
}

// ── Upstream ──────────────────────────────────────────────────────────────────

async function loadUpstream() {
  const el = document.getElementById('upstream-content');
  try {
    const s = await api('GET', '/admin/upstream');
    el.innerHTML = `
      <table style="width:auto">
        <tr><th style="padding-right:32px">Last sync</th><td>${s.last_sync_at ? new Date(s.last_sync_at).toLocaleString() : '<span class="text-muted">never</span>'}</td></tr>
        <tr><th>Machine ID</th><td class="mono">${s.machine_id || '<span class="text-muted">not configured</span>'}</td></tr>
        <tr><th>Upstream decisions</th><td>${s.decision_count ?? 0}</td></tr>
      </table>`;
  } catch (e) { el.textContent = 'Error: ' + e.message; }
}

// ── Modal helpers ─────────────────────────────────────────────────────────────

function openModal(id)  { document.getElementById(id).classList.add('open'); }
function closeModal(id) { document.getElementById(id).classList.remove('open'); }

document.querySelectorAll('.modal-backdrop').forEach(m => {
  m.addEventListener('click', e => { if (e.target === m) m.classList.remove('open'); });
});

// ── Utils ─────────────────────────────────────────────────────────────────────

function esc(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

function relTime(iso) {
  const d    = new Date(iso);
  const diff = d - Date.now();
  const abs  = Math.abs(diff);
  const past = diff < 0;
  if (abs < 60000)    return past ? 'just now' : 'in a moment';
  if (abs < 3600000)  return (past ? '' : 'in ') + Math.round(abs/60000)   + 'm'  + (past ? ' ago' : '');
  if (abs < 86400000) return (past ? '' : 'in ') + Math.round(abs/3600000) + 'h'  + (past ? ' ago' : '');
  return d.toLocaleDateString();
}

// ── Boot ──────────────────────────────────────────────────────────────────────

const saved = savedCreds();
if (saved) {
  authHeader = 'Basic ' + saved;
  document.getElementById('login-overlay').classList.remove('open');
  document.getElementById('header-user').textContent = localStorage.getItem('admin_user');
  loadPage('dashboard');
}
