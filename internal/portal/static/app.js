const state = {
  user: null,
  view: 'dashboard',
  apiBaseUrl: '',
  revealedKey: '',
  adminOrderStatus: 'pending',
};

const userViews = [
  { id: 'dashboard', label: '仪表盘', subtitle: '账户、调用与费用概览' },
  { id: 'keys', label: 'API 密钥', subtitle: '创建和查看你的调用凭据' },
  { id: 'usage', label: '使用记录', subtitle: '查看 Token、模型和费用明细' },
  { id: 'billing', label: '充值账单', subtitle: '提交充值申请并查看账务流水' },
  { id: 'integrations', label: '接入信息', subtitle: '复制 Base URL 并接入客户端' },
];

const adminViews = [
  { id: 'adminDashboard', label: '运营概览', subtitle: '充值、订单和客户运营状态' },
  { id: 'adminRecharge', label: '充值审核', subtitle: '确认或取消用户提交的充值申请' },
  { id: 'adminGuide', label: '配置边界', subtitle: 'Portal 与技术后台的操作分工' },
];

const viewMeta = [...userViews, ...adminViews].reduce((out, item) => {
  out[item.id] = item;
  return out;
}, {});

const statusMap = {
  active: '启用',
  disabled: '禁用',
  pending: '待确认',
  processing: '处理中',
  confirmed: '已确认',
  cancelled: '已取消',
  recharge: '充值',
  admin: '管理员',
  user: '用户',
};

const statusClassMap = {
  active: 'success',
  confirmed: 'success',
  pending: 'warning',
  processing: 'info',
  cancelled: 'danger',
  disabled: 'muted',
};

const $ = (id) => document.getElementById(id);

async function api(path, options = {}) {
  const res = await fetch(path, {
    ...options,
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || '请求失败');
  return data;
}

function fmt(value, digits = 0) {
  if (value === null || value === undefined || Number.isNaN(Number(value))) return '-';
  return Number(value).toLocaleString('zh-CN', { maximumFractionDigits: digits });
}

function money(value) {
  return `$${fmt(value, 4)}`;
}

function time(value) {
  return value ? new Date(value).toLocaleString('zh-CN') : '-';
}

function esc(value) {
  return String(value ?? '').replace(/[&<>"']/g, (char) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;',
  }[char]));
}

function raw(html) {
  return { html };
}

function roleLabel(role) {
  return statusMap[role] || role || '-';
}

function statusPill(status) {
  const label = statusMap[status] || status || '-';
  const kind = statusClassMap[status] || 'muted';
  return raw(`<span class="pill ${kind}"><span></span>${esc(label)}</span>`);
}

function setNotice(text, target = 'notice') {
  $(target).textContent = text || '';
}

function isAdmin() {
  return state.user?.role === 'admin';
}

function allowedViews() {
  return isAdmin() ? adminViews : userViews;
}

function normalizeView() {
  const views = allowedViews();
  if (!views.some((item) => item.id === state.view)) {
    state.view = views[0].id;
  }
}

function renderNav() {
  const items = allowedViews();
  $('navRoot').innerHTML = items.map((item) => `
    <button class="nav-item ${state.view === item.id ? 'active' : ''}" data-view="${esc(item.id)}" type="button">
      <span class="nav-icon">${esc(item.label.slice(0, 1))}</span>
      <span>${esc(item.label)}</span>
    </button>
  `).join('');
  document.querySelectorAll('[data-view]').forEach((button) => {
    button.onclick = () => {
      state.view = button.dataset.view;
      state.revealedKey = '';
      render();
    };
  });
}

function renderHeader() {
  const meta = viewMeta[state.view] || allowedViews()[0];
  $('viewTitle').textContent = meta.label;
  $('viewSubtitle').textContent = meta.subtitle;
  $('userBadge').innerHTML = `
    <span class="avatar">${esc((state.user.email || '?').slice(0, 1).toUpperCase())}</span>
    <span>
      <strong>${esc(state.user.email)}</strong>
      <small>${esc(roleLabel(state.user.role))}</small>
    </span>
  `;
}

function showLoading() {
  $('viewRoot').innerHTML = `
    <section class="toolbar shimmer"></section>
    <section class="table-shell shimmer"></section>
  `;
}

async function refreshMe() {
  try {
    const data = await api('/api/me');
    state.user = data.user;
    state.apiBaseUrl = data.sub2api_base_url;
    normalizeView();
    $('authPanel').classList.add('hidden');
    $('appPanel').classList.remove('hidden');
    renderNav();
    await render();
  } catch {
    state.user = null;
    $('authPanel').classList.remove('hidden');
    $('appPanel').classList.add('hidden');
  }
}

async function render() {
  if (!state.user) return;
  normalizeView();
  setNotice('');
  renderNav();
  renderHeader();
  showLoading();
  try {
    if (state.view === 'keys') return await renderKeys();
    if (state.view === 'usage') return await renderUsage();
    if (state.view === 'billing') return await renderBilling();
    if (state.view === 'integrations') return renderIntegrations();
    if (state.view === 'adminDashboard') return await renderAdminDashboard();
    if (state.view === 'adminRecharge') return await renderAdminRecharge();
    if (state.view === 'adminGuide') return renderAdminGuide();
    return await renderDashboard();
  } catch (err) {
    $('viewRoot').innerHTML = emptyState('加载失败', err.message || '请刷新后重试');
  }
}

async function renderDashboard() {
  const [summary, keys] = await Promise.all([
    api('/api/usage/summary'),
    api('/api/api-keys'),
  ]);
  $('viewRoot').innerHTML = `
    <section class="metric-grid">
      ${metricCard('API Key', fmt(keys.items.length), '已创建密钥数量')}
      ${metricCard('请求数', fmt(summary.summary.request_count), '最近 7 天默认统计')}
      ${metricCard('Token', fmt(summary.summary.total_tokens), '输入、输出与缓存合计')}
      ${metricCard('费用', money(summary.summary.total_cost), '按账单记录汇总')}
    </section>
    <section class="layout-two">
      <div class="panel">
        <div class="section-title">
          <span>快速接入</span>
          <strong>API Base URL</strong>
        </div>
        <div class="code-row">
          <code>${esc(state.apiBaseUrl)}</code>
          <button class="icon-button" data-copy="${esc(state.apiBaseUrl)}" type="button">复制</button>
        </div>
      </div>
      <div class="panel">
        <div class="section-title">
          <span>下一步</span>
          <strong>创建密钥后即可调用</strong>
        </div>
        <div class="action-row">
          <button class="primary" data-jump="keys" type="button">创建 API Key</button>
          <button class="secondary" data-jump="billing" type="button">提交充值</button>
        </div>
      </div>
    </section>
    ${renderRecentKeys(keys.items)}
  `;
  bindCopyButtons();
  bindJumpButtons();
}

function renderRecentKeys(items) {
  if (!items.length) {
    return emptyState('还没有 API Key', '创建第一个密钥后，这里会显示密钥预览和状态。', 'keys');
  }
  return `
    <section class="table-shell">
      <div class="table-header">
        <h3>最近的 API Key</h3>
      </div>
      ${table(['名称', '预览', '状态', '创建时间'], items.slice(0, 5).map((item) => [
        item.name,
        item.key_preview,
        statusPill(item.status),
        time(item.created_at),
      ]))}
    </section>
  `;
}

async function renderKeys() {
  const data = await api('/api/api-keys');
  $('viewRoot').innerHTML = `
    <section class="toolbar">
      <label class="field compact">
        <span>密钥名称</span>
        <input id="keyName" placeholder="例如 production-key" />
      </label>
      <button id="createKey" class="primary" type="button">创建 API Key</button>
    </section>
    ${state.revealedKey ? `
      <section class="panel accent-panel">
        <div class="section-title">
          <span>请立即保存</span>
          <strong>完整密钥只展示一次</strong>
        </div>
        <div class="code-row">
          <code>${esc(state.revealedKey)}</code>
          <button class="icon-button" data-copy="${esc(state.revealedKey)}" type="button">复制</button>
        </div>
      </section>
    ` : ''}
    <section class="table-shell">
      <div class="table-header">
        <h3>API Key</h3>
      </div>
      ${data.items.length ? table(['名称', '预览', '状态', '创建时间'], data.items.map((item) => [
        item.name,
        item.key_preview,
        statusPill(item.status),
        time(item.created_at),
      ])) : emptyState('暂无 API Key', '创建密钥后即可开始调用 API。')}
    </section>
  `;
  $('createKey').onclick = async () => {
    try {
      const created = await api('/api/api-keys', {
        method: 'POST',
        body: JSON.stringify({ name: $('keyName').value || 'Default key' }),
      });
      state.revealedKey = created.key;
      await renderKeys();
      setNotice('API Key 已创建，请立即复制保存。');
    } catch (err) {
      setNotice(err.message);
    }
  };
  bindCopyButtons();
}

async function renderUsage() {
  const [summary, records] = await Promise.all([
    api('/api/usage/summary'),
    api('/api/usage/records?limit=100'),
  ]);
  $('viewRoot').innerHTML = `
    <section class="metric-grid">
      ${metricCard('请求数', fmt(summary.summary.request_count), '默认最近 7 天')}
      ${metricCard('输入 Token', fmt(summary.summary.input_tokens), 'Prompt 与上下文')}
      ${metricCard('输出 Token', fmt(summary.summary.output_tokens), '模型返回内容')}
      ${metricCard('费用', money(summary.summary.total_cost), '账单侧汇总')}
    </section>
    <section class="table-shell">
      <div class="table-header">
        <h3>使用明细</h3>
      </div>
      ${records.items.length ? table(['时间', 'API Key', '模型', '输入', '输出', '缓存读取', '费用'], records.items.map((item) => [
        time(item.created_at),
        item.api_key_name,
        item.model || item.requested_model || '-',
        fmt(item.input_tokens),
        fmt(item.output_tokens),
        fmt(item.cache_read_tokens),
        money(item.total_cost),
      ])) : emptyState('暂无使用记录', '用户调用 API 后，这里会出现请求、Token 和费用明细。')}
    </section>
  `;
}

async function renderBilling() {
  const [orders, ledger] = await Promise.all([
    api('/api/recharge-orders'),
    api('/api/billing/ledger'),
  ]);
  $('viewRoot').innerHTML = `
    <section class="toolbar billing-form">
      <label class="field compact">
        <span>充值金额</span>
        <input id="amount" type="number" min="0" step="0.01" placeholder="例如 50" />
      </label>
      <label class="field grow">
        <span>备注</span>
        <input id="note" placeholder="转账凭证、订单号或其他说明" />
      </label>
      <button id="createOrder" class="primary" type="button">提交充值申请</button>
    </section>
    <section class="table-shell">
      <div class="table-header">
        <h3>充值申请</h3>
      </div>
      ${orders.items.length ? table(['金额', '状态', '备注', '提交时间'], orders.items.map((item) => [
        `${fmt(item.amount, 2)} ${item.currency}`,
        statusPill(item.status),
        item.note || '-',
        time(item.created_at),
      ])) : emptyState('暂无充值申请', '提交后管理员会在运营台确认。')}
    </section>
    <section class="table-shell">
      <div class="table-header">
        <h3>账务流水</h3>
      </div>
      ${ledger.items.length ? table(['类型', '金额', '余额变动后', '备注', '时间'], ledger.items.map((item) => [
        statusMap[item.type] || item.type,
        `${fmt(item.amount, 2)} ${item.currency}`,
        item.sub2api_balance_after === null || item.sub2api_balance_after === undefined ? '-' : money(item.sub2api_balance_after),
        item.note || '-',
        time(item.created_at),
      ])) : emptyState('暂无账务流水', '充值确认后会生成不可变账务记录。')}
    </section>
  `;
  $('createOrder').onclick = async () => {
    try {
      await api('/api/recharge-orders', {
        method: 'POST',
        body: JSON.stringify({ amount: Number($('amount').value), currency: 'USD', note: $('note').value }),
      });
      await renderBilling();
      setNotice('充值申请已提交，等待管理员确认。');
    } catch (err) {
      setNotice(err.message);
    }
  };
}

function renderIntegrations() {
  $('viewRoot').innerHTML = `
    <section class="layout-two">
      <div class="panel">
        <div class="section-title">
          <span>请求地址</span>
          <strong>API Base URL</strong>
        </div>
        <div class="code-row">
          <code>${esc(state.apiBaseUrl)}</code>
          <button class="icon-button" data-copy="${esc(state.apiBaseUrl)}" type="button">复制</button>
        </div>
      </div>
      <div class="panel">
        <div class="section-title">
          <span>鉴权方式</span>
          <strong>Authorization Header</strong>
        </div>
        <div class="code-row">
          <code>Authorization: Bearer &lt;API_KEY&gt;</code>
          <button class="icon-button" data-copy="Authorization: Bearer <API_KEY>" type="button">复制</button>
        </div>
      </div>
    </section>
    <section class="panel">
      <div class="section-title">
        <span>OpenAI SDK 兼容示例</span>
        <strong>Node.js</strong>
      </div>
      <pre><code>import OpenAI from "openai";

const client = new OpenAI({
  apiKey: process.env.API_KEY,
  baseURL: "${esc(state.apiBaseUrl)}"
});</code></pre>
    </section>
  `;
  bindCopyButtons();
}

async function renderAdminDashboard() {
  const orders = await api('/api/admin/recharge-orders');
  const pending = orders.items.filter((item) => item.status === 'pending' || item.status === 'processing');
  const confirmed = orders.items.filter((item) => item.status === 'confirmed');
  const confirmedAmount = confirmed.reduce((sum, item) => sum + Number(item.amount || 0), 0);
  $('viewRoot').innerHTML = `
    <section class="metric-grid">
      ${metricCard('待处理充值', fmt(pending.length), '需要管理员确认')}
      ${metricCard('已确认订单', fmt(confirmed.length), '历史确认数量')}
      ${metricCard('已确认金额', money(confirmedAmount), '按订单金额汇总')}
      ${metricCard('订单总数', fmt(orders.items.length), '全部充值申请')}
    </section>
    <section class="table-shell">
      <div class="table-header">
        <h3>最近充值申请</h3>
        <button class="secondary" data-jump="adminRecharge" type="button">进入审核</button>
      </div>
      ${orders.items.length ? table(['用户', '金额', '状态', '备注', '提交时间'], orders.items.slice(0, 8).map((item) => [
        item.user_email,
        `${fmt(item.amount, 2)} ${item.currency}`,
        statusPill(item.status),
        item.note || '-',
        time(item.created_at),
      ])) : emptyState('暂无充值申请', '用户提交充值申请后会显示在这里。')}
    </section>
  `;
  bindJumpButtons();
}

async function renderAdminRecharge() {
  const query = state.adminOrderStatus === 'all' ? '' : `?status=${encodeURIComponent(state.adminOrderStatus)}`;
  const data = await api(`/api/admin/recharge-orders${query}`);
  $('viewRoot').innerHTML = `
    <section class="toolbar">
      ${adminFilterButton('pending', '待确认')}
      ${adminFilterButton('confirmed', '已确认')}
      ${adminFilterButton('cancelled', '已取消')}
      ${adminFilterButton('all', '全部')}
    </section>
    <section class="table-shell">
      <div class="table-header">
        <h3>充值审核</h3>
      </div>
      ${data.items.length ? table(['用户', '金额', '状态', '备注', '提交时间', '操作'], data.items.map((item) => [
        item.user_email,
        `${fmt(item.amount, 2)} ${item.currency}`,
        statusPill(item.status),
        item.note || '-',
        time(item.created_at),
        adminOrderActions(item),
      ])) : emptyState('没有匹配的充值申请', '切换筛选条件查看其他状态。')}
    </section>
  `;
  document.querySelectorAll('[data-admin-filter]').forEach((button) => {
    button.onclick = () => {
      state.adminOrderStatus = button.dataset.adminFilter;
      renderAdminRecharge();
    };
  });
  document.querySelectorAll('[data-confirm]').forEach((button) => {
    button.onclick = async () => {
      try {
        await api(`/api/admin/recharge-orders/${button.dataset.confirm}/confirm`, { method: 'POST', body: '{}' });
        await renderAdminRecharge();
        setNotice('充值已确认，余额已同步。');
      } catch (err) {
        setNotice(err.message);
      }
    };
  });
  document.querySelectorAll('[data-cancel]').forEach((button) => {
    button.onclick = async () => {
      try {
        await api(`/api/admin/recharge-orders/${button.dataset.cancel}/cancel`, { method: 'POST', body: '{}' });
        await renderAdminRecharge();
        setNotice('充值申请已取消。');
      } catch (err) {
        setNotice(err.message);
      }
    };
  });
}

function renderAdminGuide() {
  $('viewRoot').innerHTML = `
    <section class="panel">
      <div class="section-title">
        <span>运营台</span>
        <strong>Portal 管理员负责客户侧操作</strong>
      </div>
      <ul class="clean-list">
        <li>确认充值申请，并让系统同步用户余额。</li>
        <li>查看用户提交的订单、金额、备注和处理状态。</li>
        <li>客户 API Key、用量和账单由 Portal 依据用户身份隔离展示。</li>
      </ul>
    </section>
    <section class="panel">
      <div class="section-title">
        <span>技术后台</span>
        <strong>渠道、模型、价格和分组仍在底层网关配置</strong>
      </div>
      <ul class="clean-list">
        <li>新增模型、渠道、账号池、价格规则时，先在技术后台配置。</li>
        <li>Portal 后续套餐只保存产品展示和底层能力映射。</li>
        <li>机器服务账号只给后端调用，不作为人工登录账号使用。</li>
      </ul>
    </section>
  `;
}

function adminFilterButton(value, label) {
  const active = state.adminOrderStatus === value ? 'active' : '';
  return `<button class="segmented ${active}" data-admin-filter="${esc(value)}" type="button">${esc(label)}</button>`;
}

function adminOrderActions(item) {
  if (item.status !== 'pending' && item.status !== 'processing') {
    return raw('<span class="muted-text">-</span>');
  }
  return raw(`
    <div class="row-actions">
      <button class="primary small" data-confirm="${esc(item.id)}" type="button">确认</button>
      <button class="ghost small danger-text" data-cancel="${esc(item.id)}" type="button">取消</button>
    </div>
  `);
}

function metricCard(label, value, note) {
  return `
    <div class="metric-card">
      <span>${esc(label)}</span>
      <strong>${esc(value)}</strong>
      <small>${esc(note)}</small>
    </div>
  `;
}

function table(headers, rows) {
  return `<table>
    <thead><tr>${headers.map((h) => `<th>${esc(h)}</th>`).join('')}</tr></thead>
    <tbody>${rows.map((row) => `<tr>${row.map(tableCell).join('')}</tr>`).join('')}</tbody>
  </table>`;
}

function tableCell(cell) {
  if (cell && typeof cell === 'object' && 'html' in cell) {
    return `<td>${cell.html}</td>`;
  }
  return `<td>${esc(cell)}</td>`;
}

function emptyState(title, detail, jumpView = '') {
  return `
    <div class="empty-state">
      <strong>${esc(title)}</strong>
      <span>${esc(detail)}</span>
      ${jumpView ? `<button class="secondary" data-jump="${esc(jumpView)}" type="button">去处理</button>` : ''}
    </div>
  `;
}

function bindJumpButtons() {
  document.querySelectorAll('[data-jump]').forEach((button) => {
    button.onclick = () => {
      state.view = button.dataset.jump;
      render();
    };
  });
}

function bindCopyButtons() {
  document.querySelectorAll('[data-copy]').forEach((button) => {
    button.onclick = async () => {
      const text = button.dataset.copy;
      await copyText(text);
      setNotice('已复制到剪贴板。');
    };
  });
}

async function copyText(text) {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return;
    } catch {
      // Fall back for plain HTTP deployments where Clipboard API can be restricted.
    }
  }
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.style.position = 'fixed';
  textarea.style.opacity = '0';
  document.body.appendChild(textarea);
  textarea.select();
  document.execCommand('copy');
  document.body.removeChild(textarea);
}

$('loginForm').onsubmit = async (event) => {
  event.preventDefault();
  try {
    setNotice('', 'authNotice');
    await api('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email: $('loginEmail').value, password: $('loginPassword').value }),
    });
    await refreshMe();
  } catch (err) {
    setNotice(err.message, 'authNotice');
  }
};

$('registerForm').onsubmit = async (event) => {
  event.preventDefault();
  try {
    setNotice('', 'authNotice');
    await api('/api/auth/register', {
      method: 'POST',
      body: JSON.stringify({ email: $('registerEmail').value, password: $('registerPassword').value }),
    });
    await refreshMe();
  } catch (err) {
    setNotice(err.message, 'authNotice');
  }
};

$('logoutButton').onclick = async () => {
  await api('/api/auth/logout', { method: 'POST', body: '{}' }).catch(() => {});
  state.user = null;
  state.view = 'dashboard';
  state.revealedKey = '';
  await refreshMe();
};

$('refreshButton').onclick = render;

refreshMe();
