// ── Seleção com IA (botão em Vagas, 0.4.0) ──
// A IA lê as vagas do Descobrir mais compatíveis com o perfil e coloca na pipeline as
// que valem a candidatura (tag "auto-gerado"); o worker gera os currículos. A pessoa
// só confere e envia. Quem vê o botão: admin, master e quem o admin liberar (/me).
// Limites (por clique e por dia) e a IA usada vêm do painel admin.

const _as = { allowed: false, open: false, status: null, poll: null, runId: null, notified: null };

function setAutoSelectAccess(allowed) {
  _as.allowed = !!allowed;
  document.getElementById('btn-auto-select')?.classList.toggle('hidden', !_as.allowed);
  if (!_as.allowed) closeAutoPanel();
}

function toggleAutoPanel() {
  if (_as.open) { closeAutoPanel(); return; }
  _as.open = true;
  const panel = document.getElementById('auto-panel');
  panel.classList.remove('hidden');
  document.getElementById('btn-auto-select')?.setAttribute('aria-expanded', 'true');
  panel.innerHTML = '<p class="auto-muted">Carregando…</p>';
  loadAutoStatus();
}

function closeAutoPanel() {
  _as.open = false;
  document.getElementById('auto-panel')?.classList.add('hidden');
  document.getElementById('btn-auto-select')?.setAttribute('aria-expanded', 'false');
}

async function loadAutoStatus() {
  try {
    const res = await api.get('/auto-select');
    if (!res.ok) throw new Error('Não foi possível carregar a seleção com IA.');
    _as.status = await apiData(res);
    // Rodada em andamento (aberta em outra aba, ou a página recarregou): acompanha.
    if (_as.status.run?.status === 'running') autoWatch(_as.status.run.id);
    renderAutoPanel();
  } catch (e) {
    const panel = document.getElementById('auto-panel');
    if (panel) panel.innerHTML = `<p class="auto-muted">Erro: ${escHtml(e.message)}</p>`;
  }
}

function renderAutoPanel() {
  const panel = document.getElementById('auto-panel');
  const s = _as.status;
  if (!panel || !_as.open || !s) return;
  if (!s.allowed) { panel.innerHTML = '<p class="auto-muted">A seleção com IA não está liberada para a sua conta.</p>'; return; }
  const run = s.run;
  const running = run?.status === 'running';
  const n = Math.min(s.per_run, s.remaining);
  let cta;
  if (!s.enabled) cta = '<p class="auto-note">A seleção com IA está desligada no momento.</p>';
  else if (running) cta = '';
  else if (n <= 0) cta = `<p class="auto-note">Você já usou as ${s.per_day} vagas de hoje. Amanhã libera de novo.</p>`;
  else cta = `<button type="button" class="btn btn-primary auto-go" onclick="startAutoSelect(this)"><svg class="ico"><use href="#i-spark"/></svg>Escolher até ${n} ${n === 1 ? 'vaga' : 'vagas'}</button>`;

  panel.innerHTML = `
    <div class="auto-head">
      <div>
        <h3><svg class="ico"><use href="#i-spark"/></svg>Seleção com IA</h3>
        <p>A IA lê as vagas do Descobrir mais compatíveis com o seu perfil, como um recrutador do seu lado, e coloca na pipeline as que valem a candidatura. Os currículos são gerados sozinhos; você confere e envia no site da empresa.</p>
      </div>
      <button type="button" class="auto-close" onclick="closeAutoPanel()" aria-label="Fechar">✕</button>
    </div>
    <div class="auto-bar">
      <div class="auto-quota" title="O limite conta vagas escolhidas, não cliques">
        <span class="auto-quota-track"><i style="width:${s.per_day ? Math.min(100, s.used_today * 100 / s.per_day) : 0}%"></i></span>
        Hoje: <b>${s.used_today}</b> de ${s.per_day} vagas · até ${s.per_run} por vez
      </div>
      ${cta}
    </div>
    ${run ? renderAutoRun(run) : ''}`;
}

function renderAutoRun(run) {
  if (run.status === 'running') {
    return `
      <div class="auto-run auto-running">
        <span class="disc-spinner" aria-hidden="true"></span>
        <div><b>Lendo ${run.candidates} ${run.candidates === 1 ? 'vaga' : 'vagas'} com a IA…</b>
        <p>Pode levar alguns minutos. Pode sair desta tela: as escolhidas aparecem na lista sozinhas.</p></div>
      </div>`;
  }
  const when = run.finished_at || run.started_at;
  const chosen = run.picks.filter(p => p.selected);
  const others = run.picks.filter(p => !p.selected);
  let head;
  if (run.status === 'error') head = `<p class="auto-result auto-err">${escHtml(run.error || 'A seleção falhou.')}</p>`;
  else if (run.status === 'empty' && !run.candidates) head = `<p class="auto-result">Nenhuma vaga nova para analisar agora. A IA só lê vagas que ainda não estão na sua pipeline e que ela não leu nos últimos 14 dias. Novas vagas chegam a cada varredura do Descobrir; confira também as skills no Perfil e o local e nível no Descobrir.</p>`;
  else if (run.status === 'empty') head = `<p class="auto-result">A IA leu ${run.analyzed} ${run.analyzed === 1 ? 'vaga' : 'vagas'} e nenhuma passou na triagem. Veja os motivos abaixo.</p>`;
  else head = `<p class="auto-result auto-ok">${chosen.length} ${chosen.length === 1 ? 'vaga escolhida' : 'vagas escolhidas'} de ${run.analyzed} lidas. Os currículos estão sendo gerados.</p>`;
  return `
    <div class="auto-run">
      <div class="auto-run-when">Última seleção: ${escHtml(fmtDate(when))}</div>
      ${head}
      ${chosen.length ? `<ul class="auto-picks">${chosen.map(autoPickItem).join('')}</ul>` : ''}
      ${others.length ? `
        <details class="auto-others"${chosen.length ? '' : ' open'}>
          <summary>${others.length} ${others.length === 1 ? 'ficou' : 'ficaram'} de fora</summary>
          <ul class="auto-picks">${others.map(autoPickItem).join('')}</ul>
        </details>` : ''}
    </div>`;
}

function autoScoreClass(score) {
  if (score == null) return '';
  return score >= 80 ? 'q-high' : score >= 60 ? 'q-mid' : 'q-low';
}

function autoPickItem(p) {
  const state = !p.selected ? '' : !p.job_id
    ? '<span class="auto-state">removida da pipeline</span>'
    : p.job_status === 'completed'
      ? `<button type="button" class="btn-link" onclick="openJobDetail('${p.job_id}', { focus: 'resumes' })">currículo pronto →</button>`
      : p.job_status === 'error'
        ? `<button type="button" class="btn-link" onclick="openJobDetail('${p.job_id}')">deu erro, ver vaga →</button>`
        : '<span class="auto-state">gerando currículo…</span>';
  return `
    <li class="auto-pick${p.selected ? ' chosen' : ''}">
      <span class="jl-pct ${autoScoreClass(p.score)}" title="Nota da IA (0 a 100)">${p.score == null ? '—' : p.score}</span>
      <div class="auto-pick-main">
        <div class="auto-pick-title">${escHtml(p.title || 'Vaga')}${p.company ? ` <span>· ${escHtml(p.company)}</span>` : ''}</div>
        ${p.reason ? `<p>${escHtml(p.reason)}</p>` : ''}
        <div class="auto-pick-meta">${p.coverage_pct}% da stack ${state}</div>
      </div>
    </li>`;
}

async function startAutoSelect(btn) {
  btn.disabled = true;
  try {
    const res = await api.post('/auto-select/runs', {});
    const body = await res.json().catch(() => ({}));
    if (!res.ok) { toast(body.error || 'Não foi possível iniciar a seleção.', 'error'); loadAutoStatus(); return; }
    const run = body.data;
    _as.status.run = run;
    if (run.status === 'running') autoWatch(run.id);
    renderAutoPanel();
  } catch (e) {
    btn.disabled = false;
    toast(e.message, 'error');
  }
}

// Acompanha a rodada até terminar. Avisa uma vez, mesmo com o painel fechado.
function autoWatch(id) {
  if (_as.runId === id && _as.poll) return;
  clearInterval(_as.poll);
  _as.runId = id;
  _as.poll = setInterval(async () => {
    try {
      const res = await api.get(`/auto-select/runs/${id}`);
      if (!res.ok) return;
      const run = await apiData(res);
      if (run.status === 'running') return;
      clearInterval(_as.poll);
      _as.poll = null;
      if (_as.notified !== id) {
        _as.notified = id;
        if (run.status === 'done') toast(`Seleção com IA: ${run.selected} ${run.selected === 1 ? 'vaga entrou' : 'vagas entraram'} na pipeline.`);
        else if (run.status === 'empty') toast('Seleção com IA: nenhuma vaga passou na triagem desta vez.');
        else toast(run.error || 'A seleção com IA falhou.', 'error');
      }
      if (_activeTab === 'vagas') loadJobs();
      if (_as.open) loadAutoStatus();
    } catch (_) {}
  }, 3000);
}

// Tag nas listas e na tela da vaga.
function autoTag(j) {
  if (!j?.auto_selected) return '';
  const tip = ['Escolhida pela seleção com IA', j.auto_score != null ? `nota ${j.auto_score}` : '', j.auto_reason || ''].filter(Boolean).join(' · ');
  return `<span class="auto-tag" title="${escHtml(tip)}"><svg class="ico"><use href="#i-spark"/></svg>auto-gerado</span>`;
}

// ── Painel admin: configuração ──

const _asAdmin = { saved: '' };

async function loadAutoSelectAdmin() {
  const box = document.getElementById('asadm-form');
  try {
    const res = await api.get('/admin/auto-select/settings');
    if (!res.ok) throw new Error('status ' + res.status);
    const s = await apiData(res);
    document.getElementById('asadm-enabled').checked = s.enabled;
    for (const k of ['per_run', 'per_day', 'candidates', 'min_score']) document.getElementById('asadm-' + k).value = s[k];
    document.getElementById('asadm-min_quality').value = s.min_quality;
    document.getElementById('asadm-mode').value = s.mode;
    document.getElementById('asadm-updated').textContent = s.updated_at ? `Última alteração: ${fmtDate(s.updated_at)}` : '';
    _asAdmin.saved = JSON.stringify(asAdminPayload());
    asAdminDirty();
  } catch (e) {
    box?.insertAdjacentHTML('afterbegin', `<p class="metric-chip chip-red">Erro ao carregar: ${escHtml(e.message)}</p>`);
  }
}

function asAdminPayload() {
  const num = id => parseInt(document.getElementById('asadm-' + id).value, 10) || 0;
  return {
    enabled: document.getElementById('asadm-enabled').checked,
    per_run: num('per_run'), per_day: num('per_day'), candidates: num('candidates'), min_score: num('min_score'),
    min_quality: document.getElementById('asadm-min_quality').value,
    mode: document.getElementById('asadm-mode').value,
  };
}

function asAdminDirty() {
  const dirty = JSON.stringify(asAdminPayload()) !== _asAdmin.saved;
  document.getElementById('asadm-save').disabled = !dirty;
  document.getElementById('asadm-dirty').textContent = dirty ? 'alterações não salvas' : '';
}

async function asAdminSave() {
  const btn = document.getElementById('asadm-save');
  btn.disabled = true;
  try {
    const res = await api.put('/admin/auto-select/settings', asAdminPayload());
    const body = await res.json().catch(() => ({}));
    if (!res.ok) { toast(body.error || 'Erro ao salvar.', 'error'); asAdminDirty(); return; }
    toast('Seleção com IA: configuração salva.');
    loadAutoSelectAdmin();
  } catch (e) { toast(e.message, 'error'); asAdminDirty(); }
}

async function toggleAutoSelectAccess(userID, allowed) {
  try {
    const res = await api.patch(`/admin/users/${userID}/auto-select`, { allowed });
    if (!res.ok) throw new Error('status ' + res.status);
    toast(allowed ? 'Seleção com IA liberada.' : 'Seleção com IA removida.');
    loadAdminUsers();
  } catch (e) { toast('Erro: ' + e.message, 'error'); }
}

// ── Métricas → Seleção com IA ──

const AS_RUN_STATUS = {
  running: ['chip-yellow', 'Em andamento'], done: ['chip-green', 'Concluída'],
  empty: ['chip-muted', 'Nenhuma escolhida'], error: ['chip-red', 'Erro'],
};

async function loadAutoSelectMetrics() {
  const range = document.getElementById('as-range')?.value || '7d';
  try {
    const res = await api.get(`/admin/metrics/auto-select?range=${range}`);
    if (!res.ok) throw new Error('status ' + res.status);
    renderAutoSelectMetrics(await apiData(res));
    set('metrics-error', '');
  } catch (e) {
    document.getElementById('as-cards').innerHTML = `<p class="empty">Erro ao carregar: ${escHtml(e.message)}</p>`;
  }
}

function asPct(a, b) { return b ? `${Math.round(a * 100 / b)}%` : '—'; }

function renderAutoSelectMetrics(d) {
  const s = d.summary;
  const card = (label, val, sub, title = '') => `
    <div class="metric-card"${title ? ` title="${escHtml(title)}"` : ''}><div class="metric-label">${label}</div><div class="metric-val metric-val-sm">${val}</div><div class="metric-sub">${sub}</div></div>`;
  const autoRate = asPct(s.applied, s.generated);
  const manualRate = asPct(s.manual_applied, s.manual_generated);
  document.getElementById('as-cards').innerHTML = `
    <div class="metrics-grid4" style="margin:0 0 16px">
      ${card('Rodadas', s.runs, `${s.users} ${s.users === 1 ? 'pessoa' : 'pessoas'} · ${s.runs_error} com erro · ${s.runs_empty} sem escolha`)}
      ${card('Vagas lidas pela IA', s.analyzed, `${s.selected} escolhidas (${asPct(s.selected, s.analyzed)})`)}
      ${card('Nota média', s.selected ? Math.round(s.avg_score_selected) : '—', `escolhidas · ${s.analyzed - s.selected ? Math.round(s.avg_score_skipped) : '—'} nas que ficaram de fora`)}
      ${card('Currículos gerados', s.generated, `${s.gen_errors} com erro · ${s.discarded} descartadas pela pessoa`, 'Descartada = a pessoa apagou a vaga da pipeline')}
    </div>
    <div class="metrics-grid4" style="margin:0 0 16px">
      ${card('Enviadas', s.applied, `${autoRate} dos currículos gerados`, 'Marcadas como enviadas no quadro de Candidaturas')}
      ${card('Chegaram a entrevista', s.interviews, `${asPct(s.interviews, s.applied)} das enviadas`)}
      ${card('Comparação', `${autoRate} × ${manualRate}`, 'enviadas: IA × escolhidas pela pessoa', 'Das vagas com currículo gerado no período, quantas foram marcadas como enviadas')}
      ${card('Custo de IA (triagem)', fmtTokens(s.prompt_tokens + s.completion_tokens), `${s.ai_calls} chamadas · ${asPct(s.ai_ok, s.ai_calls)} ok · ${s.avg_duration_ms ? fmtDur(s.avg_duration_ms / 1000) : '—'} em média`, 'Tokens de entrada + saída. A geração dos currículos conta em IA → Currículo.')}
    </div>
    <p class="card-note" style="margin:-4px 0 16px">Configuração atual: ${d.settings.enabled ? 'ligada' : '<b>desligada</b>'} · ${d.settings.per_run} por clique · ${d.settings.per_day} por dia · lê ${d.settings.candidates} · nota mínima ${d.settings.min_score}
      <button class="btn-more" onclick="navigateTo('admin'); setTimeout(() => switchAdminView('autoselect', document.querySelector('[data-admin=autoselect]')), 50)">configurar →</button></p>`;

  document.getElementById('as-users').innerHTML = d.users.length ? d.users.map(u => `
    <tr>
      <td data-label="Pessoa">${escHtml(u.email)}</td>
      <td data-label="Rodadas">${u.runs}</td>
      <td data-label="Escolhidas">${u.selected}</td>
      <td data-label="Enviadas">${u.applied} <span class="auto-muted">${asPct(u.applied, u.selected)}</span></td>
      <td data-label="Descartadas">${u.discarded}</td>
    </tr>`).join('') : '<tr><td colspan="5" class="empty">Ninguém usou no período.</td></tr>';

  document.getElementById('as-runs').innerHTML = d.runs.length ? d.runs.map(r => {
    const [cls, label] = AS_RUN_STATUS[r.status] || ['chip-muted', r.status];
    const dur = r.finished_at ? fmtDur((new Date(r.finished_at) - new Date(r.started_at)) / 1000) : '—';
    return `
      <tr>
        <td data-label="Início">${escHtml(fmtDate(r.started_at))}</td>
        <td data-label="Pessoa">${escHtml(r.email)}</td>
        <td data-label="Status"><span class="metric-chip ${cls}" ${r.error ? `title="${escHtml(r.error)}"` : ''}>${label}</span></td>
        <td data-label="Lidas">${r.analyzed} de ${r.candidates}</td>
        <td data-label="Escolhidas">${r.selected} de ${r.requested}</td>
        <td data-label="Duração">${dur}</td>
      </tr>`;
  }).join('') : '<tr><td colspan="6" class="empty">Nenhuma rodada no período.</td></tr>';
}
