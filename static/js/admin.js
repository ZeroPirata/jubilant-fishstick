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
  document.getElementById('admin-imports').classList.toggle('hidden', view !== 'imports');
  if (view === 'errors') loadAdminErrors();
  if (view === 'users')  loadAdminUsers();
  if (view === 'ai')     loadAISettings();
  if (view === 'imports') {
    loadAdminImports();
    liveStart('imports', () => {
      // Só recarrega com a aba visível; o intervalo continua barato quando não está.
      if (_activeTab === 'admin' && !document.getElementById('admin-imports').classList.contains('hidden')) loadAdminImports();
    }, 10_000);
  }
}

// ── Importações de currículo: quem enviou, quais modelos tentou, quanto levou e por que falhou ──
let _imports = [];
const IMPORT_STATUS = {
  processing:  ['chip-yellow', 'Em andamento'],
  done:        ['chip-green',  'Concluída'],
  error:       ['chip-red',    'Erro'],
  canceled:    ['chip-muted',  'Cancelada'],
  interrupted: ['chip-red',    'Interrompida'],
};

async function loadAdminImports() {
  try {
    const res = await api.get('/admin/imports?limit=200');
    if (!res.ok) throw new Error('status ' + res.status);
    _imports = (await res.json()).data || [];
    renderAdminImports();
    liveMark('imports');
  } catch (e) {
    document.getElementById('admin-imports-body').innerHTML =
      `<tr><td colspan="6" class="empty">Erro ao carregar: ${aiEsc(e.message)}</td></tr>`;
  }
}

const importSince = cur => Math.max(0, (Date.now() - new Date(cur.since)) / 1000);

const importSecs = it => {
  const end = it.finished_at ? new Date(it.finished_at) : (it.status === 'processing' ? new Date() : null);
  return end ? (end - new Date(it.started_at)) / 1000 : null;
};

function renderAdminImports() {
  const f = document.getElementById('imports-filter').value;
  const match = it => !f || (f === 'fail' ? ['error', 'interrupted'].includes(it.status) : it.status === f);
  const list = _imports.filter(match);

  // Resumo das últimas 24h (independe do filtro).
  const day = _imports.filter(it => Date.now() - new Date(it.started_at) < 864e5);
  const done = day.filter(it => it.status === 'done');
  const failed = day.filter(it => ['error', 'interrupted'].includes(it.status));
  const avg = done.length ? done.reduce((s, it) => s + importSecs(it), 0) / done.length : 0;
  document.getElementById('imports-stats').innerHTML = `
    <span class="metric-chip chip-muted">24h <b>${day.length}</b></span>
    <span class="metric-chip chip-green">concluídas <b>${done.length}</b></span>
    <span class="metric-chip chip-red">com erro <b>${failed.length}</b></span>
    ${done.length ? `<span class="metric-chip chip-muted">tempo médio <b>${fmtDur(avg)}</b></span>` : ''}`;

  const body = document.getElementById('admin-imports-body');
  const wasOpen = new Set([...body.querySelectorAll('tr.err-row.open')].map(tr => tr.dataset.id));
  body.innerHTML = list.length ? list.map(it => {
    const [cls, label] = IMPORT_STATUS[it.status] || ['chip-muted', it.status];
    const steps = (it.attempts || []).map(a => `<span class="imp-chain-step ${a.status === 'ok' ? 'ok' : a.status === 'canceled' ? 'skip' : 'fail'}" title="${aiEsc(AI_STATUS_LABELS[a.status] || a.status)}">${aiEsc(a.model)}</span>`);
    if (it.current) steps.push(`<span class="imp-chain-step now" title="tentando agora">${aiEsc(it.current.model)} · ${fmtDur(importSince(it.current))}</span>`);
    const chain = steps.join('<span class="imp-chain-arrow">→</span>') || '<span style="color:var(--muted)">—</span>';
    const secs = importSecs(it);
    return `<tr class="err-row" data-id="${aiEsc(it.id)}" onclick="toggleImportDetail(this)" title="Clique para ver as tentativas">
      <td data-label="Quando">${new Date(it.started_at).toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' })}</td>
      <td data-label="Usuário">${aiEsc(it.user_email || '—')}</td>
      <td data-label="Arquivo"><span><span class="mono">${aiEsc((it.file_kind || '?').toUpperCase())}</span> · ${fmtBytes(it.file_size)}</span></td>
      <td data-label="Resultado"><span class="metric-chip ${cls}">${label}</span>${it.applied_at ? ' <span class="metric-chip chip-green" title="A pessoa salvou no perfil">salva no perfil</span>' : ''}<span class="err-chevron">▶</span></td>
      <td data-label="Tentativas"><div class="imp-chain">${chain}</div></td>
      <td class="rt-num" data-label="Duração">${secs == null ? '—' : fmtDur(secs)}</td>
    </tr>`;
  }).join('') : '<tr><td colspan="6" class="empty">Nenhuma importação.</td></tr>';
  body.querySelectorAll('tr.err-row').forEach(tr => { if (wasOpen.has(tr.dataset.id)) toggleImportDetail(tr); });
}

function toggleImportDetail(tr) {
  const next = tr.nextElementSibling;
  if (next && next.classList.contains('err-detail-row')) { next.remove(); tr.classList.remove('open'); return; }
  const it = _imports.find(x => x.id === tr.dataset.id);
  if (!it) return;
  const r = it.result;
  const found = r ? `${r.experiences} experiências · ${r.academic} formações · ${r.skills} habilidades · ${r.projects} projetos · ${r.certificates} certificados · ${r.languages || 0} idiomas` : '—';
  const now = it.current ? `
    <div class="imp-att-row">
      <span class="imp-att-n">${(it.attempts || []).length + 1}º</span>
      <span class="mono">${aiEsc(it.current.model)}</span><span style="color:var(--muted)">${aiEsc(it.current.provider)}</span>
      <span class="metric-chip chip-yellow"><i class="imp-now-dot"></i>tentando agora</span>
      <span class="mono" style="color:var(--muted)">${fmtDur(importSince(it.current))}</span>
    </div>` : '';
  const attempts = (it.attempts || []).length || now ? (it.attempts || []).map((a, i) => `
    <div class="imp-att-row">
      <span class="imp-att-n">${i + 1}º</span>
      <span class="mono">${aiEsc(a.model)}</span><span style="color:var(--muted)">${aiEsc(a.provider)}</span>
      <span class="metric-chip ${a.status === 'ok' ? 'chip-green' : a.status === 'canceled' ? 'chip-muted' : 'chip-red'}">${aiEsc(AI_STATUS_LABELS[a.status] || a.status)}</span>
      <span class="mono" style="color:var(--muted)">${fmtDur(a.seconds)}</span>
      ${a.error ? `<pre>${aiEsc(a.error)}</pre>` : ''}
    </div>`).join('') + now
    : `<p style="color:var(--muted)">${it.status === 'processing' ? 'Preparando a leitura do arquivo…' : 'Nenhuma tentativa registrada (cancelada ou interrompida antes da IA responder).'}</p>`;
  const row = document.createElement('tr');
  row.className = 'err-detail-row';
  row.innerHTML = `<td colspan="6"><div class="err-detail">
      <div class="err-meta">
        <span><b>Início:</b> ${new Date(it.started_at).toLocaleString('pt-BR')}</span>
        ${it.finished_at ? `<span><b>Fim:</b> ${new Date(it.finished_at).toLocaleString('pt-BR')}</span>` : ''}
        ${it.applied_at ? `<span><b>Salva no perfil:</b> ${new Date(it.applied_at).toLocaleString('pt-BR')}</span>` : ''}
        ${it.status === 'processing' ? '' : `<span><b>A IA encontrou:</b> ${found}</span>`}
      </div>
      ${it.status === 'interrupted' ? '<p class="imp-detail-note">O servidor reiniciou no meio da leitura (deploy ou restart).</p>' : ''}
      ${it.error ? `<div><b class="imp-detail-label">Erro</b><pre>${aiEsc(it.error)}</pre></div>` : ''}
      <div><b class="imp-detail-label">Tentativas</b><div class="imp-att-list">${attempts}</div></div>
    </div></td>`;
  tr.after(row);
  tr.classList.add('open');
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
      `<tr><td colspan="5" class="empty">Erro ao carregar: ${e.message}</td></tr>`;
  }
}

function renderAdminUsers(users) {
  const body = document.getElementById('admin-users-body');
  if (!users.length) {
    body.innerHTML = `<tr><td colspan="5" class="empty">Nenhum usuário.</td></tr>`;
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
      <td style="color:var(--muted);font-size:12px" data-label="Último acesso" title="${u.LastSeen ? new Date(u.LastSeen).toLocaleString('pt-BR') : ''}">${relativeTime(u.LastSeen)}</td>
      <td style="color:var(--muted);font-size:12px" data-label="Cadastro">${when}</td>
      <td style="display:flex;gap:6px">${adminBtn}${masterBtn}</td>
    </tr>`;
  }).join('');
}

// "agora", "há 12 min", "há 3 h", "há 5 dias" (ou "—" sem registro).
function relativeTime(iso) {
  if (!iso) return '—';
  const s = (Date.now() - new Date(iso).getTime()) / 1000;
  if (s < 60) return 'agora';
  if (s < 3600) return `há ${Math.floor(s / 60)} min`;
  if (s < 86400) return `há ${Math.floor(s / 3600)} h`;
  return `há ${Math.floor(s / 86400)} dia${s >= 172800 ? 's' : ''}`;
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

// Uma lista só de IAs; cada uma pode estar na cadeia principal, na de scrape ou nas duas
// (no banco são linhas por cadeia, agrupadas aqui por provider + URL + modelo).
// Filtrar por cadeia só muda o que aparece: a ordem de cada cadeia é editada na sua aba.
const AI_CHAINS = {
  main:   { label: 'IA Principal', note: 'Geração do currículo.' },
  scrape: { label: 'IA Scrape',    note: 'Extração da vaga e do currículo importado.' },
};
let _ai = { items: [], order: { main: [], scrape: [] }, view: 'all', open: null, saved: '', env: {}, seq: 0 };
let _aiEncryption = true;

function aiEsc(s) {
  return String(s ?? '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

const aiItem = uid => _ai.items.find(it => it.uid === uid);
// No Gemini o modelo vai na URL (…/models/<modelo>:generateContent).
const aiModelName = it => it.model || (it.url.match(/models\/([^:/?]+)/) || [])[1] || '(sem modelo)';
const aiHost = url => { try { return new URL(url).host; } catch { return url || 'sem URL'; } };

async function loadAISettings() {
  try {
    const res = await api.get('/admin/ai-settings');
    if (!res.ok) throw new Error('status ' + res.status);
    const d = (await res.json()).data;
    _aiEncryption = d.encryption_enabled;
    _ai.env = d.env_fallback || {};
    _ai.items = []; _ai.order = { main: [], scrape: [] }; _ai.open = null;
    for (const purpose of ['main', 'scrape']) {
      for (const row of d[purpose] || []) {
        const key = `${row.provider}|${row.url}|${row.model}`;
        let it = _ai.items.find(x => x.key === key && !x.ids[purpose]);
        if (!it) {
          it = { uid: 'ai' + (++_ai.seq), key, ids: {}, enabled: {}, provider: row.provider, url: row.url, model: row.model,
                 has_key: false, key_hint: '', api_key: '', clear_key: false };
          _ai.items.push(it);
        }
        it.ids[purpose] = row.id;
        it.enabled[purpose] = row.enabled;
        if (row.has_key && !it.has_key) { it.has_key = true; it.key_hint = row.key_hint; }
        _ai.order[purpose].push(it.uid);
      }
    }
    _ai.saved = JSON.stringify(aiPayload());
    renderAI();
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

// O que vai para o PUT: as duas cadeias, na ordem de cada uma.
function aiPayload() {
  const out = {};
  for (const purpose of ['main', 'scrape']) {
    out[purpose] = _ai.order[purpose].map(uid => {
      const it = aiItem(uid);
      const id = it.ids[purpose] || '';
      const keyFrom = !id && it.has_key && !it.api_key && !it.clear_key ? (it.ids.main || it.ids.scrape || '') : '';
      return { id, provider: it.provider, url: it.url.trim(), model: it.model.trim(), api_key: it.api_key.trim(),
               clear_key: it.clear_key, enabled: !!it.enabled[purpose], key_from: keyFrom };
    });
  }
  return out;
}

function aiSyncDirty() {
  const dirty = JSON.stringify(aiPayload()) !== _ai.saved;
  for (const id of ['ai-save', 'ai-discard', 'ai-dirty-note']) document.getElementById(id).classList.toggle('hidden', !dirty);
  return dirty;
}

function renderAI() {
  const v = _ai.view;
  const n = { all: _ai.items.length, main: _ai.order.main.length, scrape: _ai.order.scrape.length };
  document.getElementById('ai-views').innerHTML = [['all', 'Todas as IAs'], ['main', AI_CHAINS.main.label], ['scrape', AI_CHAINS.scrape.label]]
    .map(([k, label]) => `<button class="${k === v ? 'active' : ''}" onclick="aiView('${k}')">${label} <span class="ai-count">${n[k]}</span></button>`).join('');

  const note = document.getElementById('ai-view-note');
  if (v === 'all') {
    note.textContent = 'Todas as IAs cadastradas. Em "Editar" você escolhe em quais cadeias cada uma entra; a ordem de cada cadeia fica na aba dela.';
  } else {
    note.textContent = `${AI_CHAINS[v].note} Tenta na ordem; só passa para a próxima se a anterior falhar. Lista vazia = usa o .env (${_ai.env[v] || '—'}).`;
  }

  const uids = v === 'all' ? _ai.items.map(it => it.uid) : _ai.order[v];
  document.getElementById('ai-list').innerHTML = uids.length
    ? uids.map((uid, i) => aiRowHTML(aiItem(uid), i, uids.length)).join('')
    : `<p class="empty">${v === 'all' ? 'Nenhuma IA cadastrada.' : 'Nenhuma IA nesta cadeia. Adicione uma nova ou inclua uma existente pela aba "Todas as IAs".'}</p>`;
  aiSyncDirty();
}

function aiKeyLabel(it) {
  if (it.clear_key) return 'key será removida';
  if (it.api_key) return 'key nova';
  if (it.has_key) return `key ${it.key_hint}`;
  return 'sem key';
}

function aiSummaryHTML(it, i) {
  const v = _ai.view;
  let chips;
  if (v === 'all') {
    chips = ['main', 'scrape'].filter(p => _ai.order[p].includes(it.uid)).map(p => {
      const pos = _ai.order[p].indexOf(it.uid) + 1;
      return `<span class="ai-chip ai-chip-${p}${it.enabled[p] ? '' : ' off'}" title="${it.enabled[p] ? '' : 'desativada nesta cadeia'}">${AI_CHAINS[p].label.replace('IA ', '')} ${pos}º${it.enabled[p] ? '' : ' · off'}</span>`;
    }).join('') || '<span class="ai-chip ai-chip-none">fora das cadeias</span>';
  } else {
    chips = '';
  }
  return `${v !== 'all' ? `<span class="ai-pos">${i + 1}º</span>` : ''}
    <span class="ai-prov ai-prov-${aiEsc(it.provider)}">${aiEsc(it.provider)}</span>
    <span class="ai-name"><b>${aiEsc(aiModelName(it))}</b><small>${aiEsc(aiHost(it.url))} · ${aiEsc(aiKeyLabel(it))}</small></span>
    <span class="ai-chips">${chips}</span>`;
}

function aiRowHTML(it, i, total) {
  const v = _ai.view, open = _ai.open === it.uid;
  const off = v !== 'all' && !it.enabled[v];
  const controls = v === 'all' ? '' : `
    <label class="ai-switch" title="${off ? 'Desativada: fica na lista mas não é usada' : 'Ativa'}">
      <input type="checkbox" ${off ? '' : 'checked'} onchange="aiToggle('${it.uid}', this.checked)"><span></span></label>
    <button class="btn-page" title="Subir" ${i === 0 ? 'disabled' : ''} onclick="aiMove('${it.uid}',-1)">↑</button>
    <button class="btn-page" title="Descer" ${i === total - 1 ? 'disabled' : ''} onclick="aiMove('${it.uid}',1)">↓</button>`;
  return `<div class="ai-row${open ? ' open' : ''}${off ? ' off' : ''}" id="ai-row-${it.uid}">
    <div class="ai-row-head">
      <div class="ai-sum">${aiSummaryHTML(it, i)}</div>
      <div class="ai-row-actions">${controls}
        <button class="btn btn-ghost btn-sm" onclick="aiEdit('${it.uid}')">${open ? 'Fechar' : 'Editar'}</button>
      </div>
    </div>
    ${open ? aiFormHTML(it) : ''}
  </div>`;
}

function aiFormHTML(it) {
  const preset = AI_PROVIDERS[it.provider] || AI_PROVIDERS.openai;
  const opts = Object.entries(AI_PROVIDERS).map(([k, p]) =>
    `<option value="${k}" ${k === it.provider ? 'selected' : ''}>${aiEsc(p.label)}</option>`).join('');
  const on = f => `oninput="aiSet('${it.uid}','${f}',this.value)"`;
  const member = ['main', 'scrape'].map(p => `
    <label class="ai-member ${_ai.order[p].includes(it.uid) ? 'on' : ''}">
      <input type="checkbox" ${_ai.order[p].includes(it.uid) ? 'checked' : ''} onchange="aiMember('${it.uid}','${p}',this.checked)">
      <span><b>${AI_CHAINS[p].label}</b><small>${AI_CHAINS[p].note}</small></span></label>`).join('');
  const nowhere = !_ai.order.main.includes(it.uid) && !_ai.order.scrape.includes(it.uid);
  return `<div class="ai-form">
    <div class="field"><label>Usar em</label><div class="ai-members">${member}</div>
      ${nowhere ? '<p class="ai-warn">Fora das duas cadeias: esta IA será apagada ao salvar.</p>' : ''}</div>
    <div class="field-row">
      <div class="field"><label>Provider</label><select onchange="aiSet('${it.uid}','provider',this.value);renderAI()">${opts}</select></div>
      <div class="field"><label>Modelo</label><input type="text" value="${aiEsc(it.model)}" placeholder="${aiEsc(preset.model)}" ${on('model')} /></div>
    </div>
    <div class="field"><label>URL</label><input type="url" value="${aiEsc(it.url)}" placeholder="${aiEsc(preset.url)}" ${on('url')} /></div>
    <div class="field" style="margin-bottom:0"><label>API key <span style="text-transform:none;letter-spacing:0">(${aiEsc(aiKeyLabel(it))}${it.ids.main && it.ids.scrape ? ', vale para as duas cadeias' : ''})</span></label>
      <div style="display:flex;gap:8px">
        <input type="password" autocomplete="new-password" value="${aiEsc(it.api_key)}"
          placeholder="${it.has_key ? 'deixe vazio para manter a atual' : 'opcional para Ollama'}" ${on('api_key')} onchange="renderAI()" />
        ${it.has_key && !it.clear_key ? `<button class="btn btn-ghost btn-sm" onclick="aiSet('${it.uid}','clear_key',true);aiSet('${it.uid}','api_key','');renderAI()">Remover key</button>` : ''}
      </div>
    </div>
    <div class="ai-form-foot">
      <button class="btn btn-danger btn-sm" onclick="aiRemove('${it.uid}')">Remover IA</button>
      <button class="btn btn-ghost btn-sm" onclick="aiEdit('${it.uid}')">Fechar</button>
    </div>
  </div>`;
}

function aiView(v) { _ai.view = v; renderAI(); }
function aiEdit(uid) { _ai.open = _ai.open === uid ? null : uid; renderAI(); }

// Digitação não redesenha o formulário (perderia o foco): só o resumo da linha e o botão salvar.
function aiSet(uid, field, value) {
  const it = aiItem(uid);
  it[field] = value;
  if (field === 'api_key' && value) it.clear_key = false;
  const row = document.querySelector(`#ai-row-${uid} .ai-sum`);
  const list = _ai.view === 'all' ? _ai.items.map(x => x.uid) : _ai.order[_ai.view];
  if (row) row.innerHTML = aiSummaryHTML(it, list.indexOf(uid));
  aiSyncDirty();
}

function aiToggle(uid, on) { aiItem(uid).enabled[_ai.view] = on; renderAI(); }

function aiMember(uid, purpose, on) {
  const order = _ai.order[purpose];
  if (on && !order.includes(uid)) { order.push(uid); aiItem(uid).enabled[purpose] = true; }
  if (!on) order.splice(order.indexOf(uid), 1);
  renderAI();
}

function aiMove(uid, dir) {
  const order = _ai.order[_ai.view], i = order.indexOf(uid);
  [order[i], order[i + dir]] = [order[i + dir], order[i]];
  renderAI();
}

function aiAdd() {
  const it = { uid: 'ai' + (++_ai.seq), key: '', ids: {}, enabled: {}, provider: 'openai', url: '', model: '',
               has_key: false, key_hint: '', api_key: '', clear_key: false };
  _ai.items.push(it);
  // Na aba de uma cadeia, a nova IA já entra nela; em "Todas" a pessoa escolhe no formulário.
  if (_ai.view !== 'all') { _ai.order[_ai.view].push(it.uid); it.enabled[_ai.view] = true; }
  _ai.open = it.uid;
  renderAI();
  document.querySelector(`#ai-row-${it.uid} input[type=text]`)?.focus();
}

async function aiRemove(uid) {
  const it = aiItem(uid);
  const where = ['main', 'scrape'].filter(p => _ai.order[p].includes(uid)).map(p => AI_CHAINS[p].label).join(' e ');
  if (where && !await confirmDialog(`Remover ${aiModelName(it)} (está em ${where})? Só vale depois de salvar.`)) return;
  for (const p of ['main', 'scrape']) _ai.order[p] = _ai.order[p].filter(x => x !== uid);
  _ai.items = _ai.items.filter(x => x.uid !== uid);
  _ai.open = null;
  renderAI();
}

async function aiDiscard() { await loadAISettings(); toast('Alterações descartadas.'); }

async function aiSave() {
  try {
    const res = await api.put('/admin/ai-settings', aiPayload());
    const d = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(d.error || 'status ' + res.status);
    toast('Configuração salva. Vale a partir do próximo job.');
    await loadAISettings();
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
    if (d.data?.profile_empty) impWelcome();
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
