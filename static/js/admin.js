// ── Admin panel ──

let _adminErrorsAll  = [];
let _adminErrorsPage = 0;
const ADMIN_PAGE_SIZE = 20;

// ── View switch (erros / usuários) ──
function switchAdminView(view, btn) {
  document.querySelectorAll('.sub-nav [data-admin]').forEach(b => b.classList.remove('active'));
  if (btn) btn.classList.add('active');
  document.getElementById('admin-errors').classList.toggle('hidden', view !== 'errors');
  document.getElementById('admin-users').classList.toggle('hidden',  view !== 'users');
  if (view === 'errors') loadAdminErrors();
  if (view === 'users')  loadAdminUsers();
}

// ── Error log ──
async function loadAdminErrors() {
  try {
    const res = await api.get('/admin/errors?limit=500&offset=0');
    if (!res.ok) throw new Error('status ' + res.status);
    const d = await res.json();
    _adminErrorsAll  = d.data?.data || [];
    _adminErrorsPage = 0;
    renderAdminErrors();
  } catch (e) {
    document.getElementById('admin-errors-body').innerHTML =
      `<tr><td colspan="6" class="empty">Erro ao carregar: ${e.message}</td></tr>`;
  }
}

function filterAdminErrors() {
  _adminErrorsPage = 0;
  renderAdminErrors();
}

function renderAdminErrors() {
  const filter = document.getElementById('admin-error-filter').value;
  const rows   = (filter ? _adminErrorsAll.filter(r => r.ErrorType === filter) : _adminErrorsAll) || [];
  const total  = rows.length;
  const start  = _adminErrorsPage * ADMIN_PAGE_SIZE;
  const page   = rows.slice(start, start + ADMIN_PAGE_SIZE);

  document.getElementById('admin-errors-count').textContent =
    total ? `${total} erro${total !== 1 ? 's' : ''}` : 'Nenhum erro';

  const pageEl = document.getElementById('admin-err-page');
  const totalPages = Math.max(1, Math.ceil(total / ADMIN_PAGE_SIZE));
  pageEl.textContent = `${_adminErrorsPage + 1} / ${totalPages}`;
  document.getElementById('admin-err-prev').disabled = _adminErrorsPage === 0;
  document.getElementById('admin-err-next').disabled = start + ADMIN_PAGE_SIZE >= total;

  const body = document.getElementById('admin-errors-body');
  if (!page.length) {
    body.innerHTML = `<tr><td colspan="6" class="empty">Nenhum erro encontrado.</td></tr>`;
    return;
  }

  body.innerHTML = page.map(r => {
    const badge  = errorTypeBadge(r.ErrorType);
    const url    = r.URL    ? `<a href="${r.URL}" target="_blank" class="link truncate" style="max-width:180px;display:inline-block" title="${r.URL}">${r.URL}</a>` : '—';
    const jobID  = r.JobID  ? `<span class="mono truncate" style="max-width:100px;display:inline-block" title="${r.JobID}">${r.JobID.slice(0,8)}…</span>` : '—';
    const user   = r.UserEmail ?? '—';
    const when   = new Date(r.CreatedAt).toLocaleString('pt-BR');
    const msg    = `<span title="${r.ErrorMessage}" class="truncate" style="max-width:260px;display:inline-block">${r.ErrorMessage}</span>`;
    return `<tr>
      <td>${badge}</td>
      <td>${url}</td>
      <td>${msg}</td>
      <td>${jobID}</td>
      <td style="color:var(--muted);font-size:12px">${user}</td>
      <td style="color:var(--muted);font-size:12px;white-space:nowrap">${when}</td>
    </tr>`;
  }).join('');
}

function errorTypeBadge(type) {
  const map = {
    scraper_incompatible: ['chip-yellow', 'Site incompatível'],
    scraper_error:        ['chip-red',    'Erro scraper'],
    llm_error:            ['chip-red',    'Erro LLM'],
    llm_rate_limit:       ['chip-yellow', 'Rate limit LLM'],
  };
  const [cls, label] = map[type] ?? ['', type];
  return `<span class="metric-chip ${cls}" style="white-space:nowrap">${label}</span>`;
}

function adminErrorsPrev() {
  if (_adminErrorsPage > 0) { _adminErrorsPage--; renderAdminErrors(); }
}
function adminErrorsNext() {
  _adminErrorsPage++;
  renderAdminErrors();
}

// ── Users ──
async function loadAdminUsers() {
  try {
    const res = await api.get('/admin/users');
    if (!res.ok) throw new Error('status ' + res.status);
    const d = await res.json();
    renderAdminUsers(d.data?.data || []);
  } catch (e) {
    document.getElementById('admin-users-body').innerHTML =
      `<tr><td colspan="4" class="empty">Erro ao carregar: ${e.message}</td></tr>`;
  }
}

function renderAdminUsers(users) {
  const body = document.getElementById('admin-users-body');
  if (!users.length) {
    body.innerHTML = `<tr><td colspan="4" class="empty">Nenhum usuário.</td></tr>`;
    return;
  }
  body.innerHTML = users.map(u => {
    const badge = u.IsAdmin
      ? `<span class="metric-chip chip-green">Admin</span>`
      : `<span class="metric-chip" style="background:rgba(113,128,150,.1);color:var(--muted)">Usuário</span>`;
    const btnLabel  = u.IsAdmin ? 'Remover admin' : 'Tornar admin';
    const btnClass  = u.IsAdmin ? 'btn-danger' : 'btn-ghost';
    const when = new Date(u.CreatedAt).toLocaleDateString('pt-BR');
    return `<tr>
      <td>${u.Email}</td>
      <td>${badge}</td>
      <td style="color:var(--muted);font-size:12px">${when}</td>
      <td><button class="btn ${btnClass}" style="font-size:12px;padding:4px 10px"
          onclick="toggleAdmin('${u.ID}', ${!u.IsAdmin})">${btnLabel}</button></td>
    </tr>`;
  }).join('');
}

async function toggleAdmin(userID, newVal) {
  try {
    const res = await api.patch(`/admin/users/${userID}`, { is_admin: newVal });
    if (!res.ok) throw new Error('status ' + res.status);
    toast(newVal ? 'Usuário promovido a admin.' : 'Admin removido.');
    loadAdminUsers();
  } catch (e) {
    toast('Erro: ' + e.message, 'error');
  }
}

// ── Bootstrap: check isAdmin on app load ──
async function checkAdminStatus() {
  try {
    const res = await api.get('/me');
    if (!res.ok) return;
    const d = await res.json();
    const isAdmin = !!d.data?.is_admin;
    localStorage.setItem('jp_is_admin', isAdmin ? '1' : '0');
    applyAdminVisibility(isAdmin);
  } catch (_) {}
}

function applyAdminVisibility(isAdmin) {
  document.querySelectorAll('.admin-only').forEach(el => {
    el.classList.toggle('hidden', !isAdmin);
  });
}

function loadAdminPanel() {
  switchAdminView('errors', document.querySelector('[data-admin="errors"]'));
}
