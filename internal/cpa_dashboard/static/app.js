const state = {
  view: 'overview',
  filters: { cpa_source: '', model: '', api_key_id: '', range: '7' },
  rmbFactor: '0.2',
};

const titles = {
  overview: ['总览', 'CPA 额度与 Sub2API 用量'],
  efficiency: ['额度效率', '按 CPA 实例归一化周额度效率'],
  accounts: ['账号健康', '按邮箱聚合的 CPA 账号状态'],
  usage: ['用量趋势', 'Sub2API token 与费用趋势'],
  cleanup: ['清理候选', '只展示候选，不执行清理'],
};

const fieldInfo = {
  cpa: 'CPA 实例来源，例如 cpa1、cpa2、cpa3。',
  weeklyConsumption: '同一统计窗口内，按账号邮箱汇总周额度剩余百分比的下降值；刷新导致的上涨不计入消耗。',
  requestCount: '同一有效统计窗口内，Sub2API usage_logs 记录的请求数量。',
  inputTokens: '同一有效统计窗口内的输入 token，页面统一按百万 token 显示为 M。',
  outputTokens: '同一有效统计窗口内的输出 token，页面统一按百万 token 显示为 M。',
  cacheReadTokens: '同一有效统计窗口内的缓存读取 token，页面统一按百万 token 显示为 M。',
  totalTokens: '输入、输出、缓存写入、缓存读取 token 的合计，页面统一按百万 token 显示为 M。',
  totalCost: '同一有效统计窗口内 Sub2API 记录的 total_cost，页面四舍五入到美元整数。',
  per100Tokens: '100%额度总token = 当前窗口总token / 周额度消耗百分比 * 100。',
  per100Cost: '100%额度费用 = 当前窗口费用 / 周额度消耗百分比 * 100。',
  monthlyTokens: '月估token = 100%额度总token * 4.3，按百万 token 显示为 M。',
  monthlyCost: '月估额度费用 = 100%额度费用 * 4.3，页面四舍五入到美元整数。',
  estimatedActualRMB: '预计实际额度RMB = 月估额度费用 * 用户输入参数。默认参数 0.2，只在浏览器前端计算，不写入数据库。',
  sample: '周额度消耗低于 10% 时显示“注意”，表示归一化估算可能波动较大。',
};

const $ = (id) => document.getElementById(id);

async function api(path, options = {}) {
  const res = await fetch(path, {
    ...options,
    headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
  });
  if (res.status === 401 && path !== '/api/session') {
    showLogin();
    throw new Error('authentication required');
  }
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || 'request failed');
  return data;
}

function rangeParams() {
  const days = Number(state.filters.range || '7');
  const end = new Date();
  const start = new Date(end);
  if (days === 1) start.setHours(0, 0, 0, 0);
  else start.setDate(start.getDate() - days);
  const params = new URLSearchParams({
    start: start.toISOString(),
    end: end.toISOString(),
  });
  if (state.filters.cpa_source) params.set('cpa_source', state.filters.cpa_source);
  if (state.filters.model) params.set('model', state.filters.model);
  if (state.filters.api_key_id) params.set('api_key_id', state.filters.api_key_id);
  return params;
}

function fmtNumber(value, digits = 0) {
  if (value === null || value === undefined || Number.isNaN(Number(value))) return '-';
  return Number(value).toLocaleString(undefined, { maximumFractionDigits: digits });
}

function fmtPercent(value) {
  return value === null || value === undefined ? '-' : `${fmtNumber(value, 2)}%`;
}

function fmtTokenM(value) {
  if (value === null || value === undefined || Number.isNaN(Number(value))) return '-';
  return `${fmtNumber(Number(value) / 1000000, 2)}M`;
}

function fmtMoney(value) {
  return value === null || value === undefined ? '-' : `$${fmtNumber(value, 0)}`;
}

function fmtRMB(value) {
  return value === null || value === undefined || Number.isNaN(Number(value)) ? '-' : `¥${fmtNumber(value, 0)}`;
}

function fmtTime(value) {
  if (!value) return '-';
  return new Date(value).toLocaleString();
}

function esc(value) {
  if (value === null || value === undefined) return '';
  return String(value).replace(/[&<>"']/g, (char) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;',
  }[char]));
}

function infoIcon(text) {
  if (!text) return '';
  return `<span class="info-icon" title="${esc(text)}" aria-label="${esc(text)}" tabindex="0">i</span>`;
}

function labelWithInfo(label, info) {
  return `<span class="field-label">${esc(label)}${infoIcon(info)}</span>`;
}

function header(label, info) {
  return { label, info };
}

function setNotice(text) {
  const node = $('notice');
  if (!text) {
    node.classList.add('hidden');
    node.textContent = '';
    return;
  }
  node.textContent = text;
  node.classList.remove('hidden');
}

function reportNotice(data) {
  return [data.alignment_notice, data.attribution_warning, data.total?.sample_warning].filter(Boolean).join('\n');
}

function fmtWindow(start, end) {
  if (!start || !end) return '-';
  return `${fmtTime(start)} - ${fmtTime(end)}`;
}

function metric(label, value, info = '') {
  return `<div class="metric"><div class="label">${labelWithInfo(label, info)}</div><div class="value">${value}</div></div>`;
}

function parseRMBFactor() {
  const raw = String(state.rmbFactor).trim();
  if (raw === '') return Number.NaN;
  const value = Number(raw);
  return Number.isFinite(value) ? value : Number.NaN;
}

function computeEstimatedActualRMB(monthlyCost) {
  const cost = Number(monthlyCost);
  const factor = parseRMBFactor();
  if (!Number.isFinite(cost) || !Number.isFinite(factor)) return Number.NaN;
  return cost * factor;
}

function estimatedActualRMBMetric(monthlyCost) {
  return `<div class="metric rmb-metric">
    <div class="label">${labelWithInfo('预计实际额度RMB', fieldInfo.estimatedActualRMB)}</div>
    <div class="rmb-control">
      <span>参数</span>
      <input id="rmbFactorInput" type="number" min="0" step="0.01" inputmode="decimal" value="${esc(state.rmbFactor)}" />
    </div>
    <div class="value" id="estimatedRMBValue">${fmtRMB(computeEstimatedActualRMB(monthlyCost))}</div>
  </div>`;
}

function bindEstimatedActualRMB(monthlyCost) {
  const input = $('rmbFactorInput');
  const value = $('estimatedRMBValue');
  if (!input || !value) return;
  input.addEventListener('input', () => {
    state.rmbFactor = input.value;
    value.textContent = fmtRMB(computeEstimatedActualRMB(monthlyCost));
  });
}

function table(headers, rows) {
  return `<div class="panel"><table><thead><tr>${headers.map((h) => {
    if (typeof h === 'string') return `<th>${esc(h)}</th>`;
    return `<th>${labelWithInfo(h.label, h.info)}</th>`;
  }).join('')}</tr></thead><tbody>${rows.join('')}</tbody></table></div>`;
}

function renderEfficiencyRows(rows) {
  return table(
    [
      header('CPA', fieldInfo.cpa),
      header('周额度消耗', fieldInfo.weeklyConsumption),
      header('请求', fieldInfo.requestCount),
      header('输入', fieldInfo.inputTokens),
      header('输出', fieldInfo.outputTokens),
      header('缓存读', fieldInfo.cacheReadTokens),
      header('费用', fieldInfo.totalCost),
      header('100%额度总token', fieldInfo.per100Tokens),
      header('100%额度费用', fieldInfo.per100Cost),
      header('月估额度费用', fieldInfo.monthlyCost),
      header('月估token', fieldInfo.monthlyTokens),
      header('样本', fieldInfo.sample),
    ],
    rows.map((r) => `<tr>
      <td>${esc(r.cpa_source)}</td>
      <td>${fmtPercent(r.weekly_consumption_percent)}</td>
      <td>${fmtNumber(r.request_count)}</td>
      <td>${fmtTokenM(r.input_tokens)}</td>
      <td>${fmtTokenM(r.output_tokens)}</td>
      <td>${fmtTokenM(r.cache_read_tokens)}</td>
      <td>${fmtMoney(r.total_cost)}</td>
      <td>${fmtTokenM(r.per_100_percent?.total_tokens)}</td>
      <td>${fmtMoney(r.per_100_percent?.total_cost)}</td>
      <td>${fmtMoney(r.monthly_estimate?.total_cost)}</td>
      <td>${fmtTokenM(r.monthly_estimate?.total_tokens)}</td>
      <td class="${r.sample_warning ? 'warn' : 'ok'}">${r.sample_warning ? '注意' : '稳定'}</td>
    </tr>`)
  );
}

async function renderOverview() {
  const data = await api('/api/overview');
  setNotice([data.seven_day_alignment_notice, data.today_alignment_notice].filter(Boolean).join('\n'));
  const root = $('viewRoot');
  root.innerHTML = `
    <section class="grid">
      ${metric('当前有效账号', fmtNumber(data.current_success_accounts))}
      ${metric('当前异常账号', fmtNumber(data.current_error_accounts))}
      ${metric('今日请求', fmtNumber(data.today_usage.request_count))}
      ${metric('今日费用', fmtMoney(data.today_usage.total_cost))}
      ${metric('7天周额度消耗', fmtPercent(data.seven_day_efficiency_total.weekly_consumption_percent))}
      ${metric('100%额度总token', fmtTokenM(data.seven_day_efficiency_total.per_100_percent?.total_tokens))}
      ${metric('100%额度费用', fmtMoney(data.seven_day_efficiency_total.per_100_percent?.total_cost))}
      ${metric('最近采集', fmtTime(data.latest_collection_at))}
    </section>
    ${renderAccountTable(data.current_accounts)}
  `;
}

async function renderEfficiency() {
  const data = await api(`/api/quota-efficiency?${rangeParams().toString()}`);
  setNotice(reportNotice(data));
  $('viewRoot').innerHTML = `
    <section class="grid">
      ${metric('周额度消耗', fmtPercent(data.total.weekly_consumption_percent), fieldInfo.weeklyConsumption)}
      ${metric('总token', fmtTokenM(data.total.total_tokens), fieldInfo.totalTokens)}
      ${metric('总费用', fmtMoney(data.total.total_cost), fieldInfo.totalCost)}
      ${metric('100%额度费用', fmtMoney(data.total.per_100_percent?.total_cost), fieldInfo.per100Cost)}
      ${metric('月估token', fmtTokenM(data.total.monthly_estimate?.total_tokens), fieldInfo.monthlyTokens)}
      ${metric('月估额度费用', fmtMoney(data.total.monthly_estimate?.total_cost), fieldInfo.monthlyCost)}
      ${estimatedActualRMBMetric(data.total.monthly_estimate?.total_cost)}
    </section>
    ${renderEfficiencyRows(data.rows)}
  `;
  bindEstimatedActualRMB(data.total.monthly_estimate?.total_cost);
}

function renderAccountTable(rows) {
  return table(
    ['CPA', '账号邮箱', 'auth文件', '套餐', '状态', '5小时', '周额度', '周刷新', '采集时间'],
    rows.map((r) => `<tr>
      <td>${esc(r.cpa_source)}</td>
      <td>${esc(r.account_email) || '-'}</td>
      <td>${esc(r.auth_file) || '-'}</td>
      <td>${esc(r.account_plan) || '-'}</td>
      <td class="${r.status === 'success' && !r.data_stale ? 'ok' : 'bad'}">${esc(r.status)}</td>
      <td>${fmtPercent(r.five_hour_remaining_percent)}</td>
      <td>${fmtPercent(r.weekly_remaining_percent)}</td>
      <td>${fmtTime(r.weekly_reset_at)}</td>
      <td>${fmtTime(r.collected_at)}</td>
    </tr>`)
  );
}

async function renderAccounts() {
  const params = new URLSearchParams();
  if (state.filters.cpa_source) params.set('cpa_source', state.filters.cpa_source);
  const data = await api(`/api/cpa-accounts?${params.toString()}`);
  setNotice('');
  $('viewRoot').innerHTML = renderAccountTable(data.rows);
}

async function renderUsage() {
  const data = await api(`/api/usage?${rangeParams().toString()}`);
  setNotice('');
  $('viewRoot').innerHTML = table(
    ['时间', 'CPA', '请求', '输入', '输出', '缓存读', '费用', '实际费用'],
    data.buckets.map((b) => `<tr>
      <td>${fmtTime(b.bucket_start)}</td>
      <td>${esc(b.cpa_source)}</td>
      <td>${fmtNumber(b.request_count)}</td>
      <td>${fmtTokenM(b.input_tokens)}</td>
      <td>${fmtTokenM(b.output_tokens)}</td>
      <td>${fmtTokenM(b.cache_read_tokens)}</td>
      <td>${fmtMoney(b.total_cost)}</td>
      <td>${fmtMoney(b.actual_cost)}</td>
    </tr>`)
  );
}

async function renderCleanup() {
  const params = new URLSearchParams();
  if (state.filters.cpa_source) params.set('cpa_source', state.filters.cpa_source);
  const data = await api(`/api/cleanup-candidates?${params.toString()}`);
  setNotice('第一版只展示候选，不会删除、移动或修改任何 auth 文件。');
  $('viewRoot').innerHTML = table(
    ['CPA', '账号邮箱', 'auth文件', '状态', '30天失败次数', '同邮箱有成功文件', '原因'],
    data.rows.map((r) => `<tr>
      <td>${esc(r.cpa_source)}</td>
      <td>${esc(r.account_email) || '-'}</td>
      <td>${esc(r.auth_file) || '-'}</td>
      <td class="bad">${esc(r.status)}</td>
      <td>${fmtNumber(r.failure_snapshots_last_30d)}</td>
      <td>${r.has_success_same_email ? '是' : '否'}</td>
      <td>${esc(r.reason)}</td>
    </tr>`)
  );
}

async function render() {
  const [title, subtitle] = titles[state.view];
  $('viewTitle').textContent = title;
  $('viewSubtitle').textContent = subtitle;
  document.querySelectorAll('.nav[data-view]').forEach((button) => {
    button.classList.toggle('active', button.dataset.view === state.view);
  });
  setNotice('');
  try {
    if (state.view === 'overview') await renderOverview();
    if (state.view === 'efficiency') await renderEfficiency();
    if (state.view === 'accounts') await renderAccounts();
    if (state.view === 'usage') await renderUsage();
    if (state.view === 'cleanup') await renderCleanup();
  } catch (error) {
    if (error.message !== 'authentication required') {
      setNotice(error.message);
    }
  }
}

async function loadFilters() {
  const data = await api('/api/filters');
  fillSelect($('cpaSelect'), data.cpa_sources, '全部');
  fillSelect($('modelSelect'), data.models, '全部');
  fillSelect($('apiKeySelect'), data.api_keys, '全部');
}

function fillSelect(select, items, emptyLabel) {
  select.innerHTML = `<option value="">${esc(emptyLabel)}</option>` + (items || []).map((item) => `<option value="${esc(item.id)}">${esc(item.label)}</option>`).join('');
}

function showLogin() {
  $('loginPanel').classList.remove('hidden');
  $('dashboardShell').classList.add('hidden');
}

function showDashboard() {
  $('loginPanel').classList.add('hidden');
  $('dashboardShell').classList.remove('hidden');
}

async function boot() {
  const session = await api('/api/session').catch(() => ({ authenticated: false }));
  if (!session.authenticated) {
    showLogin();
    return;
  }
  showDashboard();
  await loadFilters();
  await render();
}

$('loginForm').addEventListener('submit', async (event) => {
  event.preventDefault();
  $('loginError').textContent = '';
  try {
    await api('/api/session', { method: 'POST', body: JSON.stringify({ password: $('passwordInput').value }) });
    showDashboard();
    await loadFilters();
    await render();
  } catch (error) {
    $('loginError').textContent = error.message;
  }
});

$('logoutButton').addEventListener('click', async () => {
  await api('/api/session', { method: 'DELETE' }).catch(() => {});
  showLogin();
});

$('refreshButton').addEventListener('click', render);

document.querySelectorAll('.nav[data-view]').forEach((button) => {
  button.addEventListener('click', () => {
    state.view = button.dataset.view;
    render();
  });
});

$('rangeSelect').addEventListener('change', (event) => { state.filters.range = event.target.value; render(); });
$('cpaSelect').addEventListener('change', (event) => { state.filters.cpa_source = event.target.value; render(); });
$('modelSelect').addEventListener('change', (event) => { state.filters.model = event.target.value; render(); });
$('apiKeySelect').addEventListener('change', (event) => { state.filters.api_key_id = event.target.value; render(); });

boot();
