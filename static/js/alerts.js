// ── Alertas de vagas novas (sininho) ──
// Vagas que entraram no Descobrir desde a última olhada, com compatibilidade a partir
// do limite escolhido (só alta, ou alta e média). O painel lista todas, para nenhuma
// ser marcada como vista sem ter sido mostrada. Abrir o painel ou o Descobrir marca
// como vistas: a mesma vaga não aparece de novo no contador. Atualiza ao entrar e a
// cada 10 minutos com a aba do navegador visível.
const _alerts = { data: null, timer: null, open: false };
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
      <ul class="alerts-list">${d.jobs.map(alertItem).join('')}</ul>
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

function alertItem(j) {
  const url = /^https?:\/\//i.test(j.url || '') ? j.url : '';
  const pct = j.fit && j.fit.items.length ? `<span class="alerts-pct q-${escHtml(j.fit.quality)}">${j.fit.coverage_pct}%</span>` : '';
  const source = (typeof _discSourceNames !== 'undefined' && _discSourceNames[j.source]) || j.source;
  return `<li>
    ${pct}
    <div class="alerts-job">
      ${url ? `<a href="${escHtml(url)}" target="_blank" rel="noopener noreferrer">${escHtml(j.title)}</a>` : `<span>${escHtml(j.title)}</span>`}
      <small>${[j.company, source].filter(Boolean).map(escHtml).join(' · ')}</small>
    </div>
  </li>`;
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
  if (_alerts.open && !ev.target.closest('#alerts-panel, .alerts-bell')) closeAlerts();
});
document.addEventListener('keydown', ev => { if (ev.key === 'Escape' && _alerts.open) closeAlerts(); });
