// ── Country code → ECharts world map name mapping ─────────────────────────────
const COUNTRY_NAMES = {
  AF:'Afghanistan',AG:'Antigua and Barb.',AL:'Albania',AM:'Armenia',AO:'Angola',
  AR:'Argentina',AT:'Austria',AU:'Australia',AZ:'Azerbaijan',BA:'Bosnia and Herz.',
  BB:'Barbados',BD:'Bangladesh',BE:'Belgium',BF:'Burkina Faso',BG:'Bulgaria',
  BH:'Bahrain',BI:'Burundi',BJ:'Benin',BN:'Brunei',BO:'Bolivia',BR:'Brazil',
  BS:'Bahamas',BT:'Bhutan',BW:'Botswana',BY:'Belarus',BZ:'Belize',
  CA:'Canada',CD:'Dem. Rep. Congo',CF:'Central African Rep.',CG:'Congo',
  CH:'Switzerland',CI:"Côte d'Ivoire",CL:'Chile',CM:'Cameroon',CN:'China',
  CO:'Colombia',CR:'Costa Rica',CU:'Cuba',CV:'Cape Verde',CY:'Cyprus',
  CZ:'Czech Rep.',DE:'Germany',DJ:'Djibouti',DK:'Denmark',DM:'Dominica',
  DO:'Dominican Rep.',DZ:'Algeria',EC:'Ecuador',EE:'Estonia',EG:'Egypt',
  ER:'Eritrea',ES:'Spain',ET:'Ethiopia',FI:'Finland',FJ:'Fiji',
  FR:'France',GA:'Gabon',GB:'United Kingdom',GE:'Georgia',GH:'Ghana',
  GM:'Gambia',GN:'Guinea',GQ:'Eq. Guinea',GR:'Greece',GT:'Guatemala',
  GW:'Guinea-Bissau',GY:'Guyana',HN:'Honduras',HR:'Croatia',HT:'Haiti',
  HU:'Hungary',ID:'Indonesia',IE:'Ireland',IL:'Israel',IN:'India',
  IQ:'Iraq',IR:'Iran',IS:'Iceland',IT:'Italy',JM:'Jamaica',JO:'Jordan',
  JP:'Japan',KE:'Kenya',KG:'Kyrgyzstan',KH:'Cambodia',KI:'Kiribati',
  KM:'Comoros',KP:'Dem. Rep. Korea',KR:'Korea',KW:'Kuwait',KZ:'Kazakhstan',
  LA:'Lao PDR',LB:'Lebanon',LI:'Liechtenstein',LK:'Sri Lanka',LR:'Liberia',
  LS:'Lesotho',LT:'Lithuania',LU:'Luxembourg',LV:'Latvia',LY:'Libya',
  MA:'Morocco',MD:'Moldova',ME:'Montenegro',MG:'Madagascar',MK:'Macedonia',
  ML:'Mali',MM:'Myanmar',MN:'Mongolia',MR:'Mauritania',MT:'Malta',
  MU:'Mauritius',MV:'Maldives',MW:'Malawi',MX:'Mexico',MY:'Malaysia',
  MZ:'Mozambique',NA:'Namibia',NE:'Niger',NG:'Nigeria',NI:'Nicaragua',
  NL:'Netherlands',NO:'Norway',NP:'Nepal',NR:'Nauru',NZ:'New Zealand',
  OM:'Oman',PA:'Panama',PE:'Peru',PG:'Papua New Guinea',PH:'Philippines',
  PK:'Pakistan',PL:'Poland',PS:'Palestine',PT:'Portugal',PW:'Palau',
  PY:'Paraguay',QA:'Qatar',RO:'Romania',RS:'Serbia',RU:'Russia',RW:'Rwanda',
  SA:'Saudi Arabia',SB:'Solomon Is.',SC:'Seychelles',SD:'Sudan',SE:'Sweden',
  SG:'Singapore',SH:'Saint Helena',SI:'Slovenia',SK:'Slovakia',SL:'Sierra Leone',
  SM:'San Marino',SN:'Senegal',SO:'Somalia',SR:'Suriname',SS:'S. Sudan',
  ST:'São Tomé and Principe',SV:'El Salvador',SY:'Syria',SZ:'Swaziland',
  TD:'Chad',TG:'Togo',TH:'Thailand',TJ:'Tajikistan',TL:'Timor-Leste',
  TM:'Turkmenistan',TN:'Tunisia',TO:'Tonga',TR:'Turkey',TT:'Trinidad and Tobago',
  TW:'Taiwan',TZ:'Tanzania',UA:'Ukraine',UG:'Uganda',US:'United States',
  UY:'Uruguay',UZ:'Uzbekistan',VA:'Vatican',VC:'St. Vin. and Gren.',
  VE:'Venezuela',VN:'Vietnam',VU:'Vanuatu',WS:'Samoa',YE:'Yemen',
  ZA:'South Africa',ZM:'Zambia',ZW:'Zimbabwe',
};

// ── Page state (pagination + search) ─────────────────────────────────────────

const pageState = {
  machines:  { offset: 0, limit: 50, search: '' },
  decisions: { offset: 0, limit: 50, search: '' },
  signals:   { offset: 0, limit: 50, search: '' },
};

function debounce(fn, delay) {
  let timer;
  return (...args) => { clearTimeout(timer); timer = setTimeout(() => fn(...args), delay); };
}

function renderPagination(containerId, state, itemCount, reloadFn) {
  const el = document.getElementById(containerId);
  if (!el) return;
  const hasPrev = state.offset > 0;
  const hasNext = itemCount === state.limit;
  if (!hasPrev && !hasNext) { el.innerHTML = ''; return; }
  const page = Math.floor(state.offset / state.limit) + 1;
  el.innerHTML = `
    <button class="btn btn-secondary btn-sm" data-dir="prev" ${hasPrev ? '' : 'disabled'}>← Prev</button>
    <span class="page-info">Page ${page}</span>
    <button class="btn btn-secondary btn-sm" data-dir="next" ${hasNext ? '' : 'disabled'}>Next →</button>
  `;
  el.querySelector('[data-dir="prev"]')?.addEventListener('click', () => {
    state.offset = Math.max(0, state.offset - state.limit);
    reloadFn();
  });
  el.querySelector('[data-dir="next"]')?.addEventListener('click', () => {
    state.offset += state.limit;
    reloadFn();
  });
}

// ── Auth ──────────────────────────────────────────────────────────────────────

let authHeader = null;
let oidcEnabled = false;

async function doLogin() {
  const u = document.getElementById('login-user').value.trim();
  const p = document.getElementById('login-pass').value;
  const hdr = 'Basic ' + btoa(u + ':' + p);
  try {
    const r = await fetch('/admin/stats', { headers: { Authorization: hdr, Accept: 'application/json' }, credentials: 'include' });
    if (!r.ok) throw new Error('bad credentials');
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
  authHeader = null;
  if (oidcEnabled) {
    window.location.href = '/auth/logout';
  } else {
    document.getElementById('login-overlay').classList.add('open');
    document.getElementById('login-error').textContent = '';
  }
}

// ── API ───────────────────────────────────────────────────────────────────────

async function api(method, path, body) {
  const opts = {
    method,
    credentials: 'include',
    headers: { 'Accept': 'application/json' },
  };
  if (authHeader) opts.headers['Authorization'] = authHeader;
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

const pages = ['dashboard', 'machines', 'decisions', 'allowlists', 'enrollment', 'signals', 'upstream'];

function showPage(name) {
  pages.forEach(p => {
    document.getElementById('page-' + p).classList.toggle('active', p === name);
  });
  document.querySelectorAll('nav button').forEach((btn, i) => {
    btn.classList.toggle('active', pages[i] === name);
  });
  if (pageState[name]) pageState[name].offset = 0;
  loadPage(name);
}

function loadPage(name) {
  if (name === 'dashboard')  loadDashboard();
  if (name === 'machines')   loadMachines();
  if (name === 'decisions')  loadDecisions();
  if (name === 'allowlists') loadAllowlists();
  if (name === 'enrollment') loadEnrollmentKeys();
  if (name === 'signals')    loadSignals();
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
  const mapData  = countryData
    .filter(d => COUNTRY_NAMES[d.country.toUpperCase()])
    .map(d => ({ name: COUNTRY_NAMES[d.country.toUpperCase()], value: d.count }));

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
  const state = pageState.machines;
  const params = new URLSearchParams({ limit: state.limit, offset: state.offset });
  if (state.search) params.set('search', state.search);
  try {
    const machines = await api('GET', '/admin/machines?' + params) || [];
    const from = state.offset + 1;
    const to   = state.offset + machines.length;
    document.getElementById('machines-count').textContent =
      machines.length ? `${from}–${to}` : '0';
    if (!machines.length) {
      tbody.innerHTML = '<tr><td colspan="6" class="empty">No machines found.</td></tr>';
      renderPagination('machines-pagination', state, 0, loadMachines);
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
            ? `<button class="btn btn-warn btn-sm" data-action="block" data-id="${esc(m.machine_id)}">Block</button>`
            : `<button class="btn btn-secondary btn-sm" data-action="unblock" data-id="${esc(m.machine_id)}">Unblock</button>`}
          <button class="btn btn-danger btn-sm" data-action="delete-machine" data-id="${esc(m.machine_id)}">Delete</button>
        </div></td>
      </tr>`;
    }).join('');
    renderPagination('machines-pagination', state, machines.length, loadMachines);
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
  const state = pageState.decisions;
  const params = new URLSearchParams({ limit: state.limit, offset: state.offset });
  if (state.search) params.set('search', state.search);
  try {
    const decisions = await api('GET', '/admin/decisions?' + params) || [];
    if (!decisions.length) {
      tbody.innerHTML = '<tr><td colspan="7" class="empty">No decisions found.</td></tr>';
      renderPagination('decisions-pagination', state, 0, loadDecisions);
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
        <td><button class="btn btn-danger btn-sm" data-action="delete-decision" data-uuid="${esc(d.uuid)}">Delete</button></td>
      </tr>`;
    }).join('');
    renderPagination('decisions-pagination', state, decisions.length, loadDecisions);
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="7" class="empty">Error: ${esc(e.message)}</td></tr>`;
  }
}

async function loadSignals() {
  const tbody = document.getElementById('signals-body');
  const state = pageState.signals;
  const params = new URLSearchParams({ limit: state.limit, offset: state.offset });
  if (state.search) params.set('search', state.search);
  try {
    const signals = await api('GET', '/admin/signals?' + params) || [];
    if (!signals.length) {
      tbody.innerHTML = '<tr><td colspan="8" class="empty">No signals found.</td></tr>';
      renderPagination('signals-pagination', state, 0, loadSignals);
      return;
    }
    tbody.innerHTML = signals.map(s => {
      const ip = s.source_ip || s.source_range || '—';
      return `<tr>
        <td class="text-muted">${new Date(s.created_at).toLocaleString()}</td>
        <td class="mono">${esc(s.machine_id)}</td>
        <td>${esc(s.scenario)}</td>
        <td><strong>${esc(ip)}</strong></td>
        <td class="text-muted">${s.source_country ? esc(s.source_country) : '—'}</td>
        <td class="text-muted">${s.source_as_number != null ? s.source_as_number : '—'}</td>
        <td class="text-muted">${s.source_as_name ? esc(s.source_as_name) : '—'}</td>
        <td class="text-muted">${s.alert_count}</td>
      </tr>`;
    }).join('');
    renderPagination('signals-pagination', state, signals.length, loadSignals);
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="8" class="empty">Error: ${esc(e.message)}</td></tr>`;
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
        <button class="btn btn-secondary btn-sm" data-action="select-allowlist" data-id="${l.id}" data-name="${esc(l.name)}">Entries</button>
        ${l.managed
          ? '<span class="text-muted" title="Remove from allowlists file to delete" style="font-size:11px">file-managed</span>'
          : `<button class="btn btn-danger btn-sm" data-action="delete-allowlist" data-id="${l.id}">Delete</button>`}
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

async function selectAllowlist(id, name) {
  selectedAllowlistID = id;
  document.getElementById('allowlist-entries-name').textContent = name;
  document.getElementById('allowlist-entries-card').style.display = 'block';
  const tbody = document.getElementById('allowlist-entries-body');
  tbody.innerHTML = '<tr><td colspan="3" class="empty text-muted">Loading…</td></tr>';
  try {
    const entries = await api('GET', `/admin/allowlists/${id}/entries`) || [];
    if (!entries.length) {
      tbody.innerHTML = '<tr><td colspan="3" class="empty text-muted">No entries yet. Use "+ Add Entry" to add IPs/ranges.</td></tr>';
      return;
    }
    tbody.innerHTML = entries.map(e => `<tr>
      <td>${esc(e.scope)}</td>
      <td class="mono">${esc(e.value)}</td>
      <td class="text-muted">${e.comment ? esc(e.comment) : '—'}</td>
      <td><div class="actions">
        <button class="btn btn-secondary btn-sm" data-action="edit-entry" data-entry-id="${e.id}" data-scope="${esc(e.scope)}" data-value="${esc(e.value)}" data-comment="${e.comment ? esc(e.comment) : ''}">Edit</button>
        <button class="btn btn-danger btn-sm" data-action="delete-entry" data-entry-id="${e.id}">Delete</button>
      </div></td>
    </tr>`).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="3" class="empty">Error: ${esc(err.message)}</td></tr>`;
  }
}

function openAddEntry() {
  if (!selectedAllowlistID) return;
  document.getElementById('entry-value').value = '';
  document.getElementById('entry-comment').value = '';
  openModal('modal-entry');
}

let editingEntryID = null;

function openEditEntry(entryID, scope, value, comment) {
  editingEntryID = entryID;
  document.getElementById('edit-entry-scope').value = scope;
  document.getElementById('edit-entry-value').value = value;
  document.getElementById('edit-entry-comment').value = comment;
  openModal('modal-edit-entry');
}

async function submitEditEntry() {
  if (!editingEntryID || !selectedAllowlistID) return;
  const body = {
    scope:   document.getElementById('edit-entry-scope').value,
    value:   document.getElementById('edit-entry-value').value.trim(),
    comment: document.getElementById('edit-entry-comment').value.trim(),
  };
  if (!body.value) { alert('Value is required'); return; }
  try {
    await api('PUT', `/admin/allowlists/${selectedAllowlistID}/entries/${editingEntryID}`, body);
    closeModal('modal-edit-entry');
    await selectAllowlist(selectedAllowlistID, document.getElementById('allowlist-entries-name').textContent);
  } catch (e) { alert('Error: ' + e.message); }
}

async function deleteEntry(entryID) {
  if (!confirm('Delete this entry?')) return;
  try {
    await api('DELETE', `/admin/allowlists/${selectedAllowlistID}/entries/${entryID}`);
    await selectAllowlist(selectedAllowlistID, document.getElementById('allowlist-entries-name').textContent);
  } catch (e) { alert('Error: ' + e.message); }
}

async function submitEntry() {
  const body = { scope: document.getElementById('entry-scope').value, value: document.getElementById('entry-value').value.trim(), comment: document.getElementById('entry-comment').value.trim() };
  if (!body.value) { alert('Value is required'); return; }
  try {
    await api('POST', `/admin/allowlists/${selectedAllowlistID}/entries`, body);
    closeModal('modal-entry');
    await selectAllowlist(selectedAllowlistID, document.getElementById('allowlist-entries-name').textContent);
  }
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
        <td><button class="btn btn-danger btn-sm" data-action="delete-key" data-id="${k.id}">Revoke</button></td>
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

// Close modal on backdrop click or [data-close] buttons
document.querySelectorAll('.modal-backdrop').forEach(m => {
  m.addEventListener('click', e => { if (e.target === m) m.classList.remove('open'); });
});
document.addEventListener('click', e => {
  const btn = e.target.closest('[data-close]');
  if (btn) closeModal(btn.dataset.close);
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

// ── Static button wiring ──────────────────────────────────────────────────────

document.getElementById('login-btn').addEventListener('click', doLogin);
document.getElementById('logout-btn').addEventListener('click', logout);
document.getElementById('refresh-dashboard-btn').addEventListener('click', loadDashboard);
document.getElementById('add-decision-btn').addEventListener('click', openAddDecision);
document.getElementById('add-allowlist-btn').addEventListener('click', openAddAllowlist);
document.getElementById('add-entry-btn').addEventListener('click', openAddEntry);
document.getElementById('add-key-btn').addEventListener('click', openAddKey);
document.getElementById('submit-decision-btn').addEventListener('click', submitDecision);
document.getElementById('submit-allowlist-btn').addEventListener('click', submitAllowlist);
document.getElementById('submit-entry-btn').addEventListener('click', submitEntry);
document.getElementById('submit-edit-entry-btn').addEventListener('click', submitEditEntry);
document.getElementById('submit-key-btn').addEventListener('click', submitKey);

// Search inputs (debounced)
document.getElementById('machines-search').addEventListener('input', debounce(e => {
  pageState.machines.search = e.target.value.trim();
  pageState.machines.offset = 0;
  loadMachines();
}, 300));

document.getElementById('decisions-search').addEventListener('input', debounce(e => {
  pageState.decisions.search = e.target.value.trim();
  pageState.decisions.offset = 0;
  loadDecisions();
}, 300));

document.getElementById('signals-search').addEventListener('input', debounce(e => {
  pageState.signals.search = e.target.value.trim();
  pageState.signals.offset = 0;
  loadSignals();
}, 300));

// Nav buttons (data-page attribute)
document.querySelectorAll('nav button[data-page]').forEach(btn => {
  btn.addEventListener('click', () => showPage(btn.dataset.page));
});

// Event delegation for dynamically generated table rows
document.getElementById('machines-body').addEventListener('click', e => {
  const btn = e.target.closest('button[data-action]');
  if (!btn) return;
  const id = btn.dataset.id;
  if (btn.dataset.action === 'block')          blockMachine(id);
  else if (btn.dataset.action === 'unblock')   unblockMachine(id);
  else if (btn.dataset.action === 'delete-machine') deleteMachine(id);
});

document.getElementById('decisions-body').addEventListener('click', e => {
  const btn = e.target.closest('button[data-action="delete-decision"]');
  if (btn) deleteDecision(btn.dataset.uuid);
});

document.getElementById('allowlists-body').addEventListener('click', e => {
  const btn = e.target.closest('button[data-action]');
  if (!btn) return;
  if (btn.dataset.action === 'select-allowlist') selectAllowlist(Number(btn.dataset.id), btn.dataset.name);
  else if (btn.dataset.action === 'delete-allowlist') deleteAllowlist(Number(btn.dataset.id));
});

document.getElementById('enrollment-body').addEventListener('click', e => {
  const btn = e.target.closest('button[data-action="delete-key"]');
  if (btn) deleteKey(Number(btn.dataset.id));
});

document.getElementById('allowlist-entries-body').addEventListener('click', e => {
  const btn = e.target.closest('button[data-action]');
  if (!btn) return;
  const entryID = Number(btn.dataset.entryId);
  if (btn.dataset.action === 'edit-entry')
    openEditEntry(entryID, btn.dataset.scope, btn.dataset.value, btn.dataset.comment);
  else if (btn.dataset.action === 'delete-entry')
    deleteEntry(entryID);
});

// ── Boot ──────────────────────────────────────────────────────────────────────

async function boot() {
  // Check auth config
  try {
    const cfg = await fetch('/auth/config').then(r => r.json());
    oidcEnabled = cfg.oidc_enabled === true;
  } catch { oidcEnabled = false; }

  if (oidcEnabled) {
    // Show SSO button, hide basic auth form
    document.getElementById('login-basic').style.display = 'none';
    document.getElementById('login-oidc').style.display  = 'block';

    // Check session cookie via dedicated endpoint (never triggers browser Basic Auth dialog)
    try {
      const r = await fetch('/auth/session', { credentials: 'include' });
      const data = await r.json();
      if (!data.authenticated) throw new Error('not authenticated');
      document.getElementById('login-overlay').classList.remove('open');
      document.getElementById('header-user').textContent = data.name || data.email || 'SSO';
      loadPage('dashboard');
    } catch {
      document.getElementById('login-overlay').classList.add('open');
    }
    return;
  }

  // Basic auth mode: require credentials on every page load (no localStorage persistence)
  document.getElementById('login-overlay').classList.add('open');
}

boot();
