package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const quotaReportHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>CPA Quota Report</title>
<style>
:root {
  color-scheme: light dark;
  --bg: #f7f8fa;
  --panel: #ffffff;
  --text: #16181d;
  --muted: #667085;
  --line: #d8dde6;
  --accent: #0f766e;
  --accent-2: #2563eb;
  --bad: #b42318;
  --good: #067647;
  --warn: #b54708;
  --soft: #f2f4f7;
  --bar-bg: #eaecf0;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #111318;
    --panel: #181b22;
    --text: #eef1f6;
    --muted: #9aa4b2;
    --line: #303846;
    --accent: #2dd4bf;
    --accent-2: #60a5fa;
    --bad: #f97066;
    --good: #32d583;
    --warn: #fdb022;
    --soft: #202632;
    --bar-bg: #303846;
  }
}
* { box-sizing: border-box; }
body {
  margin: 0;
  font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  background: var(--bg);
  color: var(--text);
}
main {
  width: min(1180px, calc(100% - 32px));
  margin: 24px auto 48px;
}
h1 {
  font-size: 24px;
  line-height: 1.2;
  margin: 0 0 18px;
  letter-spacing: 0;
}
.toolbar, .instances, .summary, .results {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 8px;
}
.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: end;
  padding: 14px;
  margin-bottom: 14px;
}
.field {
  display: grid;
  gap: 6px;
  min-width: 180px;
}
label {
  font-size: 12px;
  color: var(--muted);
}
input, select, button {
  font: inherit;
  border-radius: 6px;
}
input, select {
  height: 36px;
  border: 1px solid var(--line);
  background: transparent;
  color: var(--text);
  padding: 0 10px;
}
input[type="checkbox"] {
  width: 16px;
  height: 16px;
  padding: 0;
}
button {
  height: 36px;
  border: 1px solid var(--line);
  background: transparent;
  color: var(--text);
  padding: 0 12px;
  cursor: pointer;
}
button.primary {
  background: var(--accent);
  border-color: var(--accent);
  color: white;
}
button:disabled {
  opacity: .55;
  cursor: wait;
}
.instances {
  padding: 6px;
  margin-bottom: 14px;
  overflow-x: auto;
}
table {
  width: 100%;
  border-collapse: collapse;
}
th, td {
  border-bottom: 1px solid var(--line);
  padding: 9px 8px;
  text-align: left;
  vertical-align: middle;
  font-size: 13px;
}
th {
  color: var(--muted);
  font-weight: 600;
}
tr:last-child td {
  border-bottom: 0;
}
.instances input[type="text"], .instances input[type="url"], .instances input[type="password"] {
  width: 100%;
  min-width: 180px;
}
.summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 1px;
  margin-bottom: 14px;
  overflow: hidden;
}
.metric {
  padding: 14px;
  background: var(--panel);
}
.metric strong {
  display: block;
  font-size: 24px;
  line-height: 1;
  margin-bottom: 6px;
}
.metric span {
  color: var(--muted);
  font-size: 12px;
}
.results {
  padding: 6px;
  overflow-x: auto;
}
.status {
  display: inline-flex;
  align-items: center;
  min-height: 22px;
  padding: 0 8px;
  border-radius: 999px;
  border: 1px solid var(--line);
  white-space: nowrap;
}
.status.valid {
  color: var(--good);
}
.status.invalid {
  color: var(--bad);
}
.muted {
  color: var(--muted);
}
.quota-cell {
  min-width: 300px;
  line-height: 1.35;
  color: var(--text);
}
.subscription-cell {
  min-width: 150px;
  line-height: 1.35;
}
.subscription-value {
  display: block;
  font-weight: 700;
  color: var(--text);
  white-space: nowrap;
}
.subscription-value.expired {
  color: var(--bad);
}
.subscription-date {
  display: block;
  margin-top: 4px;
  color: var(--muted);
  font-size: 12px;
  white-space: nowrap;
}
.quota-list {
  display: grid;
  gap: 8px;
}
.quota-window {
  width: 100%;
  min-width: 260px;
  padding: 9px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--soft);
}
.quota-window-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
  min-height: 22px;
  margin-bottom: 8px;
}
.quota-window-title {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
  font-weight: 650;
}
.quota-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  min-width: 28px;
  height: 22px;
  border-radius: 6px;
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--muted);
  font-size: 11px;
  font-weight: 700;
}
.quota-window-label {
  min-width: 0;
  overflow-wrap: anywhere;
}
.quota-window-percent {
  white-space: nowrap;
  font-weight: 750;
}
.quota-window.tone-good .quota-window-percent {
  color: var(--good);
}
.quota-window.tone-good .quota-bar-fill {
  background: var(--good);
}
.quota-window.tone-warn .quota-window-percent {
  color: var(--warn);
}
.quota-window.tone-warn .quota-bar-fill {
  background: var(--warn);
}
.quota-window.tone-bad .quota-window-percent {
  color: var(--bad);
}
.quota-window.tone-bad .quota-bar-fill {
  background: var(--bad);
}
.quota-bar {
  width: 100%;
  height: 8px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--bar-bg);
}
.quota-bar-fill {
  height: 100%;
  min-width: 0;
  border-radius: inherit;
}
.quota-window-meta {
  margin-top: 7px;
  color: var(--muted);
  font-size: 12px;
  overflow-wrap: anywhere;
}
.message {
  min-height: 22px;
  margin: 10px 2px 0;
  color: var(--muted);
  font-size: 13px;
}
.inline {
  display: inline-flex;
  gap: 8px;
  align-items: center;
}
@media (max-width: 720px) {
  main { width: min(100% - 20px, 1180px); margin-top: 16px; }
  .toolbar { align-items: stretch; }
  .field { min-width: 100%; }
  .summary { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
</style>
</head>
<body>
<main>
  <h1>CPA Quota Report</h1>
  <section class="toolbar">
    <div class="field">
      <label for="provider">Provider</label>
      <select id="provider">
        <option value="codex">Codex</option>
        <option value="claude">Claude</option>
        <option value="gemini-cli">Gemini CLI</option>
        <option value="antigravity">Antigravity</option>
        <option value="kimi">Kimi</option>
        <option value="xai">xAI</option>
        <option value="all">All</option>
      </select>
    </div>
    <label class="inline"><input id="remember" type="checkbox"> Remember locally</label>
    <button id="defaults" type="button">Tunnel Defaults</button>
    <button id="add" type="button">Add Instance</button>
    <button id="query" class="primary" type="button">Query</button>
  </section>

  <section class="instances">
    <table>
      <thead>
        <tr>
          <th>Use</th>
          <th>Name</th>
          <th>Base URL</th>
          <th>Management Key</th>
          <th></th>
        </tr>
      </thead>
      <tbody id="instances"></tbody>
    </table>
  </section>

  <section class="summary">
    <div class="metric"><strong id="metricInstances">0</strong><span>instances</span></div>
    <div class="metric"><strong id="metricTotal">0</strong><span>accounts</span></div>
    <div class="metric"><strong id="metricValid">0</strong><span>valid</span></div>
    <div class="metric"><strong id="metricInvalid">0</strong><span>invalid</span></div>
  </section>

  <section class="results">
    <table>
      <thead>
        <tr>
          <th>Instance</th>
          <th>Account</th>
          <th>Status</th>
          <th>Plan</th>
          <th>Subscription</th>
          <th>Quota</th>
          <th>Runtime</th>
          <th>Reason</th>
        </tr>
      </thead>
      <tbody id="results"></tbody>
    </table>
  </section>
  <div id="message" class="message"></div>
</main>

<script>
const storageKey = 'cpa-quota-report-v1';
const defaultInstances = [
  { enabled: true, name: 'CPA1-pro20x', baseUrl: 'http://127.0.0.1:18317', key: '' },
  { enabled: true, name: 'CPA2-plus-free', baseUrl: 'http://127.0.0.1:18318', key: '' },
  { enabled: true, name: 'CPA3-plus', baseUrl: 'http://127.0.0.1:18319', key: '' }
];
const state = { instances: [] };

function el(id) { return document.getElementById(id); }

function loadState() {
  try {
    const saved = JSON.parse(localStorage.getItem(storageKey) || 'null');
    if (saved && Array.isArray(saved.instances)) {
      state.instances = saved.instances;
      el('remember').checked = !!saved.remember;
    }
  } catch (_) {}
  if (!state.instances.length) state.instances = defaultInstances.map(x => ({ ...x }));
}

function saveState() {
  if (!el('remember').checked) {
    localStorage.removeItem(storageKey);
    return;
  }
  localStorage.setItem(storageKey, JSON.stringify({
    remember: true,
    instances: state.instances
  }));
}

function renderInstances() {
  const tbody = el('instances');
  tbody.innerHTML = '';
  state.instances.forEach((inst, index) => {
    const tr = document.createElement('tr');
    tr.innerHTML = [
      '<td><input type="checkbox" ' + (inst.enabled ? 'checked' : '') + ' data-field="enabled"></td>',
      '<td><input type="text" value="' + escapeAttr(inst.name || '') + '" data-field="name"></td>',
      '<td><input type="url" value="' + escapeAttr(inst.baseUrl || '') + '" data-field="baseUrl"></td>',
      '<td><input type="password" value="' + escapeAttr(inst.key || '') + '" data-field="key"></td>',
      '<td><button type="button" data-remove="' + index + '">Remove</button></td>'
    ].join('');
    tr.querySelectorAll('input').forEach(input => {
      input.addEventListener('input', () => {
        const field = input.dataset.field;
        state.instances[index][field] = input.type === 'checkbox' ? input.checked : input.value.trim();
        saveState();
      });
      input.addEventListener('change', () => {
        const field = input.dataset.field;
        state.instances[index][field] = input.type === 'checkbox' ? input.checked : input.value.trim();
        saveState();
      });
    });
    tr.querySelector('button').addEventListener('click', () => {
      state.instances.splice(index, 1);
      saveState();
      renderInstances();
    });
    tbody.appendChild(tr);
  });
}

function escapeAttr(value) {
  return String(value).replace(/[&<>"']/g, ch => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;'
  }[ch]));
}

function setBusy(busy) {
  el('query').disabled = busy;
  el('query').textContent = busy ? 'Querying' : 'Query';
}

function setMessage(text) {
  el('message').textContent = text || '';
}

function resetMetrics() {
  el('metricInstances').textContent = '0';
  el('metricTotal').textContent = '0';
  el('metricValid').textContent = '0';
  el('metricInvalid').textContent = '0';
}

async function queryAll() {
  const provider = el('provider').value;
  const selected = state.instances.filter(x => x.enabled);
  if (!selected.length) {
    setMessage('No instances selected.');
    return;
  }
  const missing = selected.filter(x => !x.baseUrl || !x.key);
  if (missing.length) {
    setMessage('Each selected instance needs a base URL and management key.');
    return;
  }
  setBusy(true);
  setMessage('');
  resetMetrics();
  el('results').innerHTML = '';
  try {
    const reports = await Promise.all(selected.map(inst => fetchReport(inst, provider)));
    renderReports(reports);
  } finally {
    setBusy(false);
  }
}

async function fetchReport(inst, provider) {
  const base = inst.baseUrl.replace(/\/+$/, '');
  const url = base + '/v0/management/auth-quota-report?provider=' + encodeURIComponent(provider);
  try {
    const res = await fetch(url, {
      headers: { Authorization: 'Bearer ' + inst.key }
    });
    const text = await res.text();
    if (!res.ok) {
      return { instance: inst.name || base, error: 'HTTP ' + res.status + ': ' + text.slice(0, 180) };
    }
    return { instance: inst.name || base, report: JSON.parse(text) };
  } catch (err) {
    return { instance: inst.name || base, error: String(err && err.message || err) };
  }
}

function renderReports(reports) {
  const totals = { instances: reports.length, total: 0, valid: 0, invalid: 0 };
  const tbody = el('results');
  tbody.innerHTML = '';
  reports.forEach(item => {
    if (item.error) {
      appendResultRow({
        instance: item.instance,
        account: '',
        status: 'invalid',
        plan: '',
        subscription: null,
        quota: '',
        runtime: '',
        reason: item.error
      });
      totals.invalid++;
      return;
    }
    const summary = item.report.summary || {};
    totals.total += summary.total || 0;
    totals.valid += summary.valid || 0;
    totals.invalid += summary.invalid || 0;
    const accounts = item.report.accounts || [];
    if (!accounts.length) {
      appendResultRow({
        instance: item.instance,
        account: '',
        status: 'invalid',
        plan: '',
        subscription: null,
        quota: '',
        runtime: '',
        reason: 'No matching accounts'
      });
      return;
    }
    accounts.forEach(account => appendResultRow(formatAccountRow(item.instance, account)));
  });
  el('metricInstances').textContent = String(totals.instances);
  el('metricTotal').textContent = String(totals.total);
  el('metricValid').textContent = String(totals.valid);
  el('metricInvalid').textContent = String(totals.invalid);
  setMessage('Updated ' + new Date().toLocaleString() + '.');
}

function formatAccountRow(instance, account) {
  const quota = account.quota || {};
  const subscription = account.subscription || null;
  const runtime = account.runtime_quota || {};
  const plan = quota.plan_type || (subscription && subscription.plan_type) || '';
  const runtimeText = Object.keys(runtime).length
    ? Object.entries(runtime).map(([k, v]) => k + ': ' + formatValue(v)).join('; ')
    : 'clear';
  return {
    instance,
    account: account.display || account.auth_index || '',
    status: account.status || 'invalid',
    plan,
    subscription,
    quota,
    runtime: runtimeText,
    reason: account.reason || ''
  };
}

function formatSubscriptionCell(subscription) {
  if (!subscription || typeof subscription !== 'object') {
    return '<span class="muted">unknown</span>';
  }
  const remaining = subscription.remaining_label || '';
  const activeUntil = formatResetClock(subscription.active_until) || subscription.active_until || '';
  if (!remaining && !activeUntil) {
    return '<span class="muted">unknown</span>';
  }
  const expired = subscription.expired === true;
  const value = expired ? 'expired' : remaining || 'active';
  return [
    '<span class="subscription-value' + (expired ? ' expired' : '') + '">' + escapeCell(value) + '</span>',
    activeUntil ? '<span class="subscription-date">' + escapeCell(activeUntil) + '</span>' : ''
  ].join('');
}

function formatQuotaCell(quota) {
  if (!quota || typeof quota !== 'object') {
    return '<span class="muted">' + escapeCell(quota || '') + '</span>';
  }
  const windows = Array.isArray(quota.windows) ? quota.windows : [];
  if (windows.length) {
    return '<div class="quota-list">' + windows.map(formatQuotaWindow).join('') + '</div>';
  }
  if (quota.known) return '<span class="muted">no quota windows</span>';
  return '<span class="muted">' + escapeCell(quota.message || 'quota unavailable') + '</span>';
}

function formatQuotaWindow(window) {
  const label = formatWindowLabel(window.label || window.id || 'quota');
  const badge = quotaBadge(label);
  const percent = normalizePercent(window.remaining_percent);
  const tone = quotaTone(percent);
  const percentText = percent == null ? '--' : Math.round(percent) + '%';
  const width = percent == null ? 0 : percent;
  const meta = formatResetMeta(window);
  const fullReset = formatFullResetTime(window.reset_at);
  return [
    '<div class="quota-window ' + tone + '"' + (fullReset ? ' title="' + escapeAttr(fullReset) + '"' : '') + '>',
      '<div class="quota-window-head">',
        '<div class="quota-window-title">',
          '<span class="quota-badge">' + escapeCell(badge) + '</span>',
          '<span class="quota-window-label">' + escapeCell(label) + '</span>',
        '</div>',
        '<div class="quota-window-percent">' + escapeCell(percentText) + '</div>',
      '</div>',
      '<div class="quota-bar"><div class="quota-bar-fill" style="width: ' + width.toFixed(1) + '%"></div></div>',
      '<div class="quota-window-meta">' + escapeCell(meta) + '</div>',
    '</div>'
  ].join('');
}

function normalizePercent(value) {
  if (typeof value !== 'number' || !Number.isFinite(value)) return null;
  return Math.max(0, Math.min(100, value));
}

function quotaTone(percent) {
  if (percent == null) return 'tone-warn';
  if (percent >= 70) return 'tone-good';
  if (percent >= 30) return 'tone-warn';
  return 'tone-bad';
}

function quotaBadge(label) {
  const lower = String(label || '').toLowerCase();
  if (lower.includes('weekly')) return 'W';
  if (lower.includes('5h') || lower.includes('five')) return '5h';
  return 'Q';
}

function formatWindowLabel(label) {
  const text = String(label || 'quota').replace(/[-_]+/g, ' ').replace(/\s+/g, ' ').trim();
  if (!text) return 'Quota';
  return text.split(' ').map(part => {
    const lower = part.toLowerCase();
    if (lower === '5h') return '5h';
    if (lower === 'weekly') return 'Weekly';
    if (part.length <= 3 && part === part.toUpperCase()) return part;
    return lower.slice(0, 1).toUpperCase() + lower.slice(1);
  }).join(' ');
}

function formatResetMeta(window) {
  const until = formatDurationUntil(window.reset_at, window.reset_after_seconds);
  const reset = formatResetClock(window.reset_at) || window.reset_label || '';
  if (until && reset) return until + ' (' + reset + ')';
  if (until) return until;
  if (reset) return 'resets ' + reset;
  return 'reset unknown';
}

function formatDurationUntil(resetAt, resetAfterSeconds) {
  let seconds = null;
  const resetTime = parseResetTime(resetAt);
  if (resetTime) {
    seconds = Math.max(0, Math.round((resetTime.getTime() - Date.now()) / 1000));
  } else if (typeof resetAfterSeconds === 'number' && Number.isFinite(resetAfterSeconds)) {
    seconds = Math.max(0, Math.round(resetAfterSeconds));
  }
  if (seconds == null) return '';
  if (seconds <= 0) return 'ready';
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return days + 'd ' + hours + 'h';
  if (hours > 0) return hours + 'h ' + minutes + 'm';
  if (minutes > 0) return minutes + 'm';
  return '<1m';
}

function formatResetClock(resetAt) {
  const resetTime = parseResetTime(resetAt);
  if (!resetTime) return '';
  const month = String(resetTime.getMonth() + 1).padStart(2, '0');
  const day = String(resetTime.getDate()).padStart(2, '0');
  const hour = String(resetTime.getHours()).padStart(2, '0');
  const minute = String(resetTime.getMinutes()).padStart(2, '0');
  return month + '/' + day + ' ' + hour + ':' + minute;
}

function formatFullResetTime(resetAt) {
  const resetTime = parseResetTime(resetAt);
  return resetTime ? 'Reset at ' + resetTime.toLocaleString() : '';
}

function parseResetTime(resetAt) {
  if (!resetAt) return null;
  const date = new Date(resetAt);
  return Number.isNaN(date.getTime()) ? null : date;
}

function formatValue(value) {
  if (value == null) return '';
  if (typeof value === 'string') return value;
  return JSON.stringify(value);
}

function appendResultRow(row) {
  const tr = document.createElement('tr');
  const status = row.status === 'valid' ? 'valid' : 'invalid';
  tr.innerHTML = [
    '<td>' + escapeCell(row.instance) + '</td>',
    '<td>' + escapeCell(row.account) + '</td>',
    '<td><span class="status ' + status + '">' + escapeCell(status) + '</span></td>',
    '<td>' + escapeCell(row.plan || '') + '</td>',
    '<td class="subscription-cell">' + formatSubscriptionCell(row.subscription) + '</td>',
    '<td class="quota-cell">' + formatQuotaCell(row.quota) + '</td>',
    '<td class="muted">' + escapeCell(row.runtime || '') + '</td>',
    '<td class="muted">' + escapeCell(row.reason || '') + '</td>'
  ].join('');
  el('results').appendChild(tr);
}

function escapeCell(value) {
  return String(value || '').replace(/[&<>"']/g, ch => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;'
  }[ch]));
}

el('add').addEventListener('click', () => {
  state.instances.push({ enabled: true, name: '', baseUrl: '', key: '' });
  saveState();
  renderInstances();
});
el('defaults').addEventListener('click', () => {
  state.instances = defaultInstances.map(x => ({ ...x }));
  saveState();
  renderInstances();
});
el('remember').addEventListener('change', saveState);
el('query').addEventListener('click', queryAll);

loadState();
renderInstances();
</script>
</body>
</html>`

func (s *Server) serveQuotaReportPage(c *gin.Context) {
	if s == nil || !s.managementRoutesEnabled.Load() {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(quotaReportHTML))
}
