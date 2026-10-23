// ── Admin panel ──

let _adminErrorsAll  = [];
let _isMaster        = false;
let _adminErrorsPage = 0;
const ADMIN_PAGE_SIZE = 20;

// ── View switch (erros / usuários) ──
function switchAdminView(view, btn) {
  document.querySelectorAll('.sub-nav [data-admin]').forEach(b => b.classList.remove('active'));
  if (btn) btn.classList.add('active');
  document.getElementById('admin-errors').classList.toggle('hidden', view !== 'errors');
  document.getElementById('admin-users').classList.toggle('hidden',  view !== 'users');
  document.getElementById('admin-ai').classList.toggle('hidden',     view !== 'ai');
  if (view === 'errors') loadAdminErrors();
  if (view === 'users')  loadAdminUsers();
  if (view === 'ai')     loadAISettings();
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

  // Conteúdo vem de páginas de vagas e respostas de IA: tudo escapado antes de ir para o HTML.
  body.innerHTML = page.map((r, i) => {
    const badge  = errorTypeBadge(r.ErrorType);
    const url    = r.URL ? `<span class="link truncate" style="max-width:180px;display:inline-block">${aiEsc(r.URL)}</span>` : '—';
    const jobID  = r.JobID ? `<span class="mono">${aiEsc(r.JobID.slice(0, 8))}…</span>` : '—';
    const when   = new Date(r.CreatedAt).toLocaleString('pt-BR');
    const msg    = `<span class="truncate" style="max-width:300px;display:inline-block">${aiEsc(r.ErrorMessage)}</span>`;
    return `<tr class="err-row" data-i="${start + i}" onclick="toggleErrorDetail(this)" title="Clique para ver a mensagem completa">
      <td data-label="Tipo">${badge}<span class="err-chevron">▶</span></td>
      <td data-label="URL">${url}</td>
      <td data-label="Mensagem">${msg}</td>
      <td data-label="Job">${jobID}</td>
      <td style="color:var(--muted);font-size:12px" data-label="Usuário">${aiEsc(r.UserEmail ?? '—')}</td>
      <td style="color:var(--muted);font-size:12px;white-space:nowrap" data-label="Quando">${when}</td>
    </tr>`;
  }).join('');
}

// Abre/fecha, logo abaixo da linha, a mensagem completa do erro com URL e job.
function toggleErrorDetail(tr) {
  const next = tr.nextElementSibling;
  if (next && next.classList.contains('err-detail-row')) {
    next.remove();
    tr.classList.remove('open');
    return;
  }
  const r = _adminErrorsAll[+tr.dataset.i];
  if (!r) return;
  const detail = document.createElement('tr');
  detail.className = 'err-detail-row';
  const url = r.URL && /^https?:\/\//i.test(r.URL)
    ? `<a class="link" href="${aiEsc(r.URL)}" target="_blank" rel="noopener noreferrer">${aiEsc(r.URL)}</a>` : aiEsc(r.URL || '—');
  detail.innerHTML = `<td colspan="6"><div class="err-detail">
      <div class="err-meta">
        <span><b>Quando:</b> ${new Date(r.CreatedAt).toLocaleString('pt-BR')}</span>
        <span><b>Usuário:</b> ${aiEsc(r.UserEmail ?? '—')}</span>
        <span><b>Job:</b> <span class="mono">${aiEsc(r.JobID || '—')}</span></span>
      </div>
      <div class="err-meta"><span><b>URL:</b> ${url}</span></div>
      <pre>${aiEsc(r.ErrorMessage)}</pre>
    </div></td>`;
  tr.after(detail);
  tr.classList.add('open');
}

function errorTypeBadge(type) {
  const map = {
    scraper_incompatible: ['chip-yellow', 'Site incompatível'],
    scraper_error:        ['chip-red',    'Erro scraper'],
    scraper_empty:        ['chip-yellow', 'Vaga sem descrição'],
    profile_incomplete:   ['chip-yellow', 'Perfil vazio'],
    llm_error:            ['chip-red',    'Erro LLM'],
    llm_rate_limit:       ['chip-yellow', 'Rate limit LLM'],
  };
  const [cls, label] = map[type] ?? ['', type];
  return `<span class="metric-chip ${cls}" style="white-space:nowrap">${aiEsc(label)}</span>`;
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
    const badge = u.IsMaster
      ? `<span class="metric-chip chip-yellow">Master</span>`
      : u.IsAdmin
        ? `<span class="metric-chip chip-green">Admin</span>`
        : `<span class="metric-chip" style="background:rgba(113,128,150,.1);color:var(--muted)">Usuário</span>`;
    const btnLabel  = u.IsAdmin ? 'Remover admin' : 'Tornar admin';
    const btnClass  = u.IsAdmin ? 'btn-danger' : 'btn-ghost';
    const when = new Date(u.CreatedAt).toLocaleDateString('pt-BR');
    // Master não perde o admin direto: precisa remover o master antes.
    const adminBtn = u.IsMaster ? '' : `<button class="btn ${btnClass}" style="font-size:12px;padding:4px 10px"
          onclick="toggleAdmin('${u.ID}', ${!u.IsAdmin})">${btnLabel}</button>`;
    const masterBtn = !_isMaster ? '' : `<button class="btn ${u.IsMaster ? 'btn-danger' : 'btn-ghost'}" style="font-size:12px;padding:4px 10px"
          onclick="toggleMaster('${u.ID}', ${!u.IsMaster})">${u.IsMaster ? 'Remover master' : 'Tornar master'}</button>`;
    return `<tr>
      <td data-label="Email">${aiEsc(u.Email)}</td>
      <td data-label="Papel">${badge}</td>
      <td style="color:var(--muted);font-size:12px" data-label="Cadastro">${when}</td>
      <td style="display:flex;gap:6px">${adminBtn}${masterBtn}</td>
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

async function toggleMaster(userID, newVal) {
  if (newVal && !confirm('Master pode ver e trocar as API keys de IA. Confirmar?')) return;
  try {
    const res = await api.patch(`/admin/users/${userID}/master`, { is_master: newVal });
    if (!res.ok) {
      const d = await res.json().catch(() => ({}));
      throw new Error(d.error || 'status ' + res.status);
    }
    toast(newVal ? 'Usuário promovido a master.' : 'Master removido.');
    loadAdminUsers();
  } catch (e) {
    toast('Erro: ' + e.message, 'error');
  }
}

// ── Configurações de IA (master) ──
const AI_PROVIDERS = {
  openai: { label: 'OpenAI-compatível (Groq, OpenRouter, NVIDIA...)', url: 'https://api.groq.com/openai/v1/chat/completions', model: 'llama-3.3-70b-versatile' },
  gemini: { label: 'Gemini', url: 'https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash-lite:generateContent', model: '(vai na URL)' },
  ollama: { label: 'Ollama (local)', url: 'http://localhost:11434/v1/chat/completions', model: 'phi4' },
};

let _aiChains = { main: [], scrape: [] };
let _aiEncryption = true;

function aiEsc(s) {
  return String(s ?? '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

async function loadAISettings() {
  try {
    const res = await api.get('/admin/ai-settings');
    if (!res.ok) throw new Error('status ' + res.status);
    const d = (await res.json()).data;
    _aiEncryption = d.encryption_enabled;
    for (const purpose of ['main', 'scrape']) {
      _aiChains[purpose] = (d[purpose] || []).map(p => ({ ...p, api_key: '', clear_key: false }));
      document.getElementById(`ai-env-${purpose}`).textContent =
        `Lista vazia = usa o .env (${d.env_fallback[purpose]}).`;
      renderAIChain(purpose);
    }
    renderAIStatus(d.worker_paused_secs);
  } catch (e) {
    document.getElementById('ai-status').innerHTML =
      `<span class="metric-chip chip-red">Erro ao carregar: ${aiEsc(e.message)}</span>`;
  }
}

function renderAIStatus(pausedSecs) {
  const parts = [];
  if (pausedSecs > 0) {
    const h = Math.floor(pausedSecs / 3600), m = Math.ceil((pausedSecs % 3600) / 60);
    parts.push(`<span class="metric-chip chip-red">Worker pausado por rate limit (faltam ${h}h${m}m)</span>
      <button class="btn btn-ghost btn-sm" onclick="aiResume()">Retomar agora</button>`);
  } else {
    parts.push(`<span class="metric-chip chip-green">Worker ativo</span>`);
  }
  if (!_aiEncryption) {
    parts.push(`<span class="metric-chip chip-yellow">SETTINGS_ENCRYPTION_KEY não configurada: não é possível salvar API keys</span>`);
  }
  document.getElementById('ai-status').innerHTML =
    `<div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap">${parts.join('')}</div>`;
}

function renderAIChain(purpose) {
  const el = document.getElementById(`ai-chain-${purpose}`);
  const chain = _aiChains[purpose];
  if (!chain.length) {
    el.innerHTML = `<p class="empty" style="margin-bottom:14px">Nenhum provider no painel.</p>`;
    return;
  }
  el.innerHTML = chain.map((p, i) => {
    const preset = AI_PROVIDERS[p.provider] || AI_PROVIDERS.openai;
    const opts = Object.entries(AI_PROVIDERS).map(([k, v]) =>
      `<option value="${k}" ${k === p.provider ? 'selected' : ''}>${aiEsc(v.label)}</option>`).join('');
    let keyHint = 'sem key';
    if (p.clear_key) keyHint = 'key será removida ao salvar';
    else if (p.api_key) keyHint = 'key nova será salva';
    else if (p.has_key) keyHint = `key salva: ${aiEsc(p.key_hint)}`;
    const on = (field) => `oninput="aiSet('${purpose}',${i},'${field}',this.value)"`;
    return `<div style="border:1px solid var(--border);border-radius:8px;padding:12px;margin-bottom:12px;${p.enabled ? '' : 'opacity:.55'}">
      <div style="display:flex;align-items:center;gap:8px;margin-bottom:10px">
        <strong style="font-size:13px">${i + 1}º</strong>
        <label class="field-check" style="margin:0"><input type="checkbox" ${p.enabled ? 'checked' : ''}
          onchange="aiSet('${purpose}',${i},'enabled',this.checked);renderAIChain('${purpose}')" /><label>Ativo</label></label>
        <span style="margin-left:auto;display:flex;gap:4px">
          <button class="btn-page" title="Subir" ${i === 0 ? 'disabled' : ''} onclick="aiMove('${purpose}',${i},-1)">↑</button>
          <button class="btn-page" title="Descer" ${i === chain.length - 1 ? 'disabled' : ''} onclick="aiMove('${purpose}',${i},1)">↓</button>
          <button class="btn btn-danger btn-sm" onclick="aiRemove('${purpose}',${i})">Remover</button>
        </span>
      </div>
      <div class="field-row">
        <div class="field"><label>Provider</label>
          <select onchange="aiSet('${purpose}',${i},'provider',this.value);renderAIChain('${purpose}')">${opts}</select></div>
        <div class="field"><label>Modelo</label>
          <input type="text" value="${aiEsc(p.model)}" placeholder="${aiEsc(preset.model)}" ${on('model')} /></div>
      </div>
      <div class="field"><label>URL</label>
        <input type="url" value="${aiEsc(p.url)}" placeholder="${aiEsc(preset.url)}" ${on('url')} /></div>
      <div class="field" style="margin-bottom:0"><label>API key <span style="text-transform:none;letter-spacing:0">(${keyHint})</span></label>
        <div style="display:flex;gap:8px">
          <input type="password" autocomplete="new-password" value="${aiEsc(p.api_key)}"
            placeholder="${p.has_key ? 'deixe vazio para manter a atual' : 'opcional para Ollama'}"
            oninput="aiSet('${purpose}',${i},'api_key',this.value)" onchange="renderAIChain('${purpose}')" />
          ${p.has_key && !p.clear_key ? `<button class="btn btn-ghost btn-sm" onclick="aiSet('${purpose}',${i},'clear_key',true);aiSet('${purpose}',${i},'api_key','');renderAIChain('${purpose}')">Remover key</button>` : ''}
        </div>
      </div>
    </div>`;
  }).join('');
}

function aiSet(purpose, i, field, value) {
  _aiChains[purpose][i][field] = value;
  if (field === 'api_key' && value) _aiChains[purpose][i].clear_key = false;
}

function aiAdd(purpose) {
  _aiChains[purpose].push({ id: '', provider: 'openai', url: '', model: '', enabled: true, has_key: false, api_key: '', clear_key: false });
  renderAIChain(purpose);
}

function aiRemove(purpose, i) {
  _aiChains[purpose].splice(i, 1);
  renderAIChain(purpose);
}

function aiMove(purpose, i, dir) {
  const chain = _aiChains[purpose];
  [chain[i], chain[i + dir]] = [chain[i + dir], chain[i]];
  renderAIChain(purpose);
}

async function aiSave(purpose) {
  const providers = _aiChains[purpose].map(p => ({
    id: p.id, provider: p.provider, url: p.url.trim(), model: p.model.trim(),
    api_key: p.api_key.trim(), clear_key: p.clear_key, enabled: p.enabled,
  }));
  try {
    const res = await api.put(`/admin/ai-settings/${purpose}`, { providers });
    const d = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(d.error || 'status ' + res.status);
    toast('Configuração salva. Vale a partir do próximo job.');
    loadAISettings();
  } catch (e) {
    toast('Erro: ' + e.message, 'error');
  }
}

async function aiResume() {
  try {
    const res = await api.post('/admin/ai-settings/resume', {});
    if (!res.ok) throw new Error('status ' + res.status);
    toast('Worker retomado.');
    loadAISettings();
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
    _isMaster = !!d.data?.is_master;
    localStorage.setItem('jp_is_admin', isAdmin ? '1' : '0');
    applyAdminVisibility(isAdmin);
    document.querySelectorAll('.master-only').forEach(el => el.classList.toggle('hidden', !_isMaster));
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
