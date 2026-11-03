// ── Alertas de vagas novas (sininho) ──
// Vagas que entraram no Descobrir desde a última olhada, com compatibilidade a partir
// do limite escolhido (só alta, ou alta e média). O painel lista todas, para nenhuma
// ser marcada como vista sem ter sido mostrada. Abrir o painel ou o Descobrir marca
// como vistas: a mesma vaga não aparece de novo no contador. Atualiza ao entrar e a
// cada 10 minutos com a aba do navegador visível.
const _alerts = { data: null, timer: null, open: false, feedback: {} };
// Empresa com pelo menos isso de vagas novas vira um grupo recolhido no painel.
const ALERTS_GROUP_FROM = 3;
const ALERTS_EVERY = 10 * 60 * 1000;
const _alertLevels = { high: 'Só compatibilidade alta', mid: 'Alta e média', off: 'Desligado' };

function startAlerts() {
  document.querySelectorAll('.alerts-bell').forEach(b => b.classList.remove('hidden'));
  loadAlerts();
  clearInterval(_alerts.timer);
  _alerts.timer = setInterval(() => { if (!document.hidden && getToken()) loadAlerts(); }, ALERTS_EVERY);
}

function stopAlerts() {
  clearInterval(_alerts.timer);
  _alerts.data = null;
  closeAlerts();
  alertsBadge(0);
}

async function loadAlerts() {
  try {
    const res = await api.get('/discovery/alerts');
    if (!res.ok) return;
    _alerts.data = await apiData(res);
    alertsBadge(_alerts.data.enabled ? _alerts.data.count : 0);
    if (_alerts.open) renderAlerts();
    // Já está no Descobrir: as novas estão na tela.
    if (_activeTab === 'descobrir') markAlertsSeen();
  } catch (_) { /* sem rede: tenta de novo no próximo ciclo */ }
}

function alertsBadge(n) {
  document.querySelectorAll('.alerts-bell').forEach(b => {
    const c = b.querySelector('.alerts-count');
    c.textContent = n > 9 ? '9+' : String(n);
    c.classList.toggle('hidden', !n);
    b.classList.toggle('has-new', n > 0);
    b.setAttribute('aria-label', n ? `${n} ${n === 1 ? 'vaga nova' : 'vagas novas'}` : 'Vagas novas');
    b.title = n ? `${n} ${n === 1 ? 'vaga nova compatível' : 'vagas novas compatíveis'}` : 'Nenhuma vaga nova';
  });
}

// Marca como vistas no servidor (só quando há o que marcar). O painel continua
// mostrando a lista até fechar.
async function markAlertsSeen() {
  if (!_alerts.data || !_alerts.data.count) return;
  alertsBadge(0);
  try { await api.post('/discovery/alerts/seen', {}); } catch (_) {}
}

function toggleAlerts(ev) {
  ev?.stopPropagation();
  if (_alerts.open) { closeAlerts(); return; }
  _alerts.open = true;
  const panel = document.getElementById('alerts-panel');
  panel.classList.remove('hidden');
  renderAlerts();
  if (!_alerts.data) loadAlerts();
  markAlertsSeen();
}

function closeAlerts() {
  _alerts.open = false;
  document.getElementById('alerts-panel')?.classList.add('hidden');
}

function renderAlerts() {
  const panel = document.getElementById('alerts-panel');
  const d = _alerts.data;
  if (!d) { panel.innerHTML = '<p class="alerts-empty">Carregando…</p>'; return; }
  const level = d.enabled ? d.min_quality : 'off';
  const since = new Date(d.since).toLocaleString('pt-BR', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });
  let body;
  if (!d.enabled) {
    body = '<p class="alerts-empty">Alertas desligados.</p>';
  } else if (!d.count) {
    body = `<p class="alerts-empty">Nenhuma vaga nova compatível desde ${escHtml(since)}. A busca do Descobrir roda sozinha a cada poucas horas.</p>`;
  } else {
    const more = d.count - d.jobs.length;
    body = `<p class="alerts-sub">${d.count} ${d.count === 1 ? 'vaga nova' : 'vagas novas'} desde ${escHtml(since)}, da mais compatível para a menos</p>
      <ul class="alerts-list">${alertRows(d.jobs)}</ul>
      ${more > 0 ? `<p class="alerts-sub">E mais ${more} no Descobrir.</p>` : ''}`;
  }
  panel.innerHTML = `
    <div class="alerts-head"><b>Vagas novas</b><button type="button" class="alerts-close" onclick="closeAlerts()" aria-label="Fechar">×</button></div>
    ${body}
    <div class="alerts-foot">
      <button type="button" class="btn btn-ghost btn-sm" onclick="closeAlerts(); navigateTo('descobrir')">Abrir o Descobrir</button>
      <label class="alerts-level">Avisar
        <select onchange="saveAlertLevel(this.value)">
          ${Object.entries(_alertLevels).map(([v, l]) => `<option value="${v}"${v === level ? ' selected' : ''}>${l}</option>`).join('')}
        </select>
      </label>
    </div>`;
}

// Na ordem do ranking; a empresa com ALERTS_GROUP_FROM vagas ou mais vira um grupo no
// lugar da mais compatível dela ("Empresa X: 15 vagas"), que abre a lista.
function alertRows(jobs) {
  const key = j => (j.company || '').trim().toLowerCase();
  const byCompany = {};
  jobs.forEach(j => { if (key(j)) (byCompany[key(j)] ||= []).push(j); });
  const done = new Set();
  return jobs.map(j => {
    const k = key(j);
    const group = k ? byCompany[k] : null;
    if (!group || group.length < ALERTS_GROUP_FROM) return alertItem(j);
    if (done.has(k)) return '';
    done.add(k);
    return alertGroup(group);
  }).join('');
}

function alertGroup(jobs) {
  const best = jobs[0];
  const pct = best.fit && best.fit.items.length ? `<span class="alerts-pct q-${escHtml(best.fit.quality)}">${best.fit.coverage_pct}%</span>` : '<span class="alerts-pct"></span>';
  return `<li class="alerts-group">
    <button type="button" class="alerts-row" onclick="toggleAlertGroup(this)" aria-expanded="false">
      ${pct}
      <span class="alerts-job">
        <span class="alerts-title">${escHtml(best.company)}: ${jobs.length} vagas</span>
        <small>a melhor: ${escHtml(best.title)}</small>
      </span>
      <span class="alerts-chevron" aria-hidden="true">›</span>
    </button>
    <ul class="alerts-sublist" hidden>${jobs.map(alertItem).join('')}</ul>
  </li>`;
}

function toggleAlertGroup(btn) {
  const li = btn.closest('.alerts-group');
  const list = li.querySelector('.alerts-sublist');
  list.hidden = !list.hidden;
  btn.setAttribute('aria-expanded', !list.hidden);
  li.classList.toggle('open', !list.hidden);
}

function alertJob(id) {
  return (_alerts.data?.jobs || []).find(j => j.id === id);
}

// Cada vaga abre ali mesmo: compatibilidade, trecho da descrição, stack e as ações do
// Descobrir (Gerar currículo, andamento na pipeline, vaga original), mais avaliar o
// alerta e dispensar a vaga.
function alertItem(j) {
  const pct = j.fit && j.fit.items.length ? `<span class="alerts-pct q-${escHtml(j.fit.quality)}">${j.fit.coverage_pct}%</span>` : '<span class="alerts-pct"></span>';
  const source = (typeof _discSourceNames !== 'undefined' && _discSourceNames[j.source]) || j.source;
  return `<li class="alerts-item" data-id="${escHtml(j.id)}">
    <button type="button" class="alerts-row" onclick="toggleAlertItem(this)" aria-expanded="false">
      ${pct}
      <span class="alerts-job">
        <span class="alerts-title">${escHtml(j.title)}</span>
        <small>${[j.company, source].filter(Boolean).map(escHtml).join(' · ')}</small>
      </span>
      <span class="alerts-chevron" aria-hidden="true">›</span>
    </button>
    <div class="alerts-detail" hidden></div>
  </li>`;
}

function toggleAlertItem(btn) {
  const li = btn.closest('.alerts-item');
  const detail = li.querySelector('.alerts-detail');
  const open = detail.hidden;
  detail.hidden = !open;
  btn.setAttribute('aria-expanded', open);
  li.classList.toggle('open', open);
  if (open && !detail.innerHTML) detail.innerHTML = alertDetail(alertJob(li.dataset.id));
}

function alertDetail(j) {
  const url = /^https?:\/\//i.test(j.url || '') ? j.url : '';
  const local = [j.international ? 'Exterior' : '', (typeof _discWorkplace !== 'undefined' && _discWorkplace[j.workplace_type]) || '', j.location].filter(Boolean).join(' · ');
  const stack = [...(j.matched_skills || []), ...(j.stack || [])];
  const action = j.pipeline_job_id ? discPipeline(j.pipeline_job_id, j.pipeline_status) : (url ? discGenerateButton(url, j.language) : '');
  return `
    ${discFitLine(j.fit)}
    ${local ? `<div class="alerts-meta">${escHtml(local)}</div>` : ''}
    ${j.snippet ? `<p class="alerts-snippet">${escHtml(j.snippet)}</p>` : ''}
    ${stack.length ? `<div class="disc-skills">${stack.slice(0, 10).map(t => `<span class="match-tag">${escHtml(t)}</span>`).join('')}</div>` : ''}
    ${typeof discAlsoOn === 'function' ? discAlsoOn(j.also_on) : ''}
    <div class="disc-actions alerts-actions">
      ${action}
      ${url ? `<a class="link" href="${escHtml(url)}" target="_blank" rel="noopener noreferrer">↗ vaga original</a>` : ''}
    </div>
    <div class="alerts-feedback">
      <span>Este alerta foi útil?</span>
      ${alertFeedbackButtons(j.id)}
      <button type="button" class="btn btn-ghost btn-sm alerts-dismiss" onclick="dismissAlertJob(this)"
        title="Some do sino e do Descobrir só para você (dá para restaurar no Descobrir, em Dispensadas)">Dispensar</button>
    </div>`;
}

function alertFeedbackButtons(id) {
  const v = _alerts.feedback[id];
  return `<button type="button" class="alerts-vote${v === true ? ' on' : ''}" onclick="sendAlertFeedback(this, true)" aria-pressed="${v === true}" title="Útil">👍</button>
    <button type="button" class="alerts-vote${v === false ? ' on' : ''}" onclick="sendAlertFeedback(this, false)" aria-pressed="${v === false}" title="Não útil">👎</button>`;
}

// Avaliação do alerta, com a compatibilidade que a vaga tinha: aparece em Métricas →
// Busca de vagas, para ajustar a regra com dados.
async function sendAlertFeedback(btn, relevant) {
  const li = btn.closest('.alerts-item');
  const j = alertJob(li.dataset.id);
  if (!j || !j.fit) return;
  try {
    const res = await api.post('/discovery/alerts/feedback', {
      job_id: j.id, relevant, quality: j.fit.quality, coverage_pct: j.fit.coverage_pct,
    });
    if (!res.ok) { toast('Não foi possível salvar a avaliação.', 'error'); return; }
    _alerts.feedback[j.id] = relevant;
    btn.parentElement.querySelectorAll('.alerts-vote').forEach(b => {
      const on = b === btn;
      b.classList.toggle('on', on);
      b.setAttribute('aria-pressed', on);
    });
    toast(relevant ? 'Obrigado: alerta marcado como útil.' : 'Obrigado: alerta marcado como não útil.');
  } catch (e) {
    toast(e.message || 'Erro de conexão.', 'error');
  }
}

async function dismissAlertJob(btn) {
  const li = btn.closest('.alerts-item');
  const id = li.dataset.id;
  btn.disabled = true;
  try {
    const res = await api.post(`/discovery/jobs/${encodeURIComponent(id)}/dismiss`, {});
    if (!res.ok) { btn.disabled = false; toast('Não foi possível dispensar.', 'error'); return; }
    if (_alerts.data) {
      _alerts.data.jobs = _alerts.data.jobs.filter(j => j.id !== id);
      _alerts.data.count = Math.max(0, _alerts.data.count - 1);
    }
    renderAlerts();
    toast('Vaga dispensada. Ela fica no Descobrir, em "Dispensadas".');
    if (_activeTab === 'descobrir' && typeof loadDiscoveryJobs === 'function') loadDiscoveryJobs();
  } catch (e) {
    btn.disabled = false;
    toast(e.message || 'Erro de conexão.', 'error');
  }
}

async function saveAlertLevel(v) {
  const body = v === 'off' ? { enabled: false, min_quality: _alerts.data?.min_quality || 'high' } : { enabled: true, min_quality: v };
  try {
    const res = await api.put('/discovery/alerts', body);
    if (!res.ok) { toast('Não foi possível salvar.', 'error'); return; }
    toast(v === 'off' ? 'Alertas desligados.' : `Alertas: ${_alertLevels[v].toLowerCase()}.`);
    loadAlerts();
  } catch (e) {
    toast(e.message || 'Erro de conexão.', 'error');
  }
}

document.addEventListener('click', ev => {
  if (!_alerts.open) return;
  // "Ver currículo" leva para a tela da vaga: o painel fecha junto.
  if (ev.target.closest('#alerts-panel .disc-done')) { closeAlerts(); return; }
  if (!ev.target.closest('#alerts-panel, .alerts-bell')) closeAlerts();
});
document.addEventListener('keydown', ev => { if (ev.key === 'Escape' && _alerts.open) closeAlerts(); });
