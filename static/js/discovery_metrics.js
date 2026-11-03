// ── Métricas → Busca de vagas ──
// Agendador (próxima varredura), andamento da varredura em curso, situação de cada
// site e histórico. A configuração fica no painel admin (discovery_admin.js).
//
// O painel de métricas atualiza a cada 10s; com varredura rodando, esta aba atualiza
// a cada 2s para o andamento acompanhar de perto.

const DM_TRIGGERS = { schedule: 'Agendada', admin: 'Painel', user: 'Usuário' };
const DM_STATUS = {
  running: ['badge-processing', 'Rodando'],
  success: ['badge-completed', 'Sucesso'],
  partial: ['badge-mid', 'Parcial'],
  error:   ['badge-error', 'Erro'],
};
const DM_FAST_MS = 2000;

let _dm = { seq: 0, fastTimer: null };

function goToDiscoverySettings() {
  navigateTo('admin');
  switchAdminView('discovery', document.querySelector('[data-admin="discovery"]'));
}

function dmStatusBadge(status) {
  const [cls, label] = DM_STATUS[status] || ['badge-pending', status];
  return `<span class="badge ${cls}">${label}</span>`;
}

const dmValidTime = iso => iso && !iso.startsWith('0001');

function dmSecs(s) {
  s = Math.max(0, Math.round(s));
  if (s < 60) return `${s}s`;
  const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60);
  return h ? `${h} h ${m} min` : `${m} min ${s % 60}s`;
}

function dmUntil(iso) {
  if (!dmValidTime(iso)) return '—';
  const s = (new Date(iso).getTime() - Date.now()) / 1000;
  if (s <= 30) return 'em instantes';
  if (s >= 172800) return `em ${Math.floor(s / 86400)} dias`;
  return `em ${dmSecs(s).replace(/ \d+s$/, '')}`;
}

function dmDuration(startIso, endIso) {
  return endIso ? dmSecs((new Date(endIso) - new Date(startIso)) / 1000) : '—';
}

function dmIntervalLabel(min) {
  if (min % 1440 === 0) return `${min / 1440} dia${min > 1440 ? 's' : ''}`;
  if (min % 60 === 0)   return `${min / 60} h`;
  return `${min} min`;
}

async function loadDiscoveryMetrics() {
  const seq = ++_dm.seq;
  clearTimeout(_dm.fastTimer);
  try {
    const res = await api.get('/admin/metrics/discovery');
    if (seq !== _dm.seq) return;
    if (!res.ok) { document.getElementById('metrics-error').textContent = 'Erro ao carregar a busca de vagas.'; return; }
    const d = await apiData(res);
    if (seq !== _dm.seq) return;
    document.getElementById('metrics-error').textContent = '';
    liveMark('metrics');
    liveBadge('metrics');
    renderDiscoveryMetrics(d);

    const live = _live.metrics && !_live.metrics.paused;
    if (d.scheduler.running && live) {
      _dm.fastTimer = setTimeout(() => {
        if (_activeTab === 'metricas' && _metricsView === 'busca') loadDiscoveryMetrics();
      }, DM_FAST_MS);
    }
  } catch (e) {
    document.getElementById('metrics-error').textContent = e.message;
  }
}

function renderDiscoveryMetrics(d) {
  renderDmCards(d);
  renderDmProgress(d.scheduler);
  renderDmSources(d.sources);
  renderDmRuns(d.runs);
  renderDmFeedback(d.alert_feedback || []);
}

// Avaliações do sino (👍/👎) por faixa de compatibilidade: quanto do que o alerta
// mandou foi útil. Base para ajustar a regra dos alertas.
function renderDmFeedback(rows) {
  const box = document.getElementById('dm-feedback');
  if (!box) return;
  const names = { high: 'Alta', mid: 'Média', low: 'Baixa' };
  const shown = ['high', 'mid', 'low'].map(q => rows.find(r => r.quality === q)).filter(Boolean);
  if (!shown.length) {
    box.innerHTML = '<p class="metric-sub" style="margin:0">Nenhuma avaliação ainda. No sino, abra uma vaga e marque 👍 ou 👎.</p>';
    return;
  }
  box.innerHTML = shown.map(r => {
    const total = r.relevant + r.not_relevant;
    const pct = total ? Math.round(100 * r.relevant / total) : 0;
    return `<div class="dm-feedback-row">
      <b>${names[r.quality]}</b>
      <div class="dm-feedback-bar" title="${pct}% úteis"><span style="width:${pct}%"></span></div>
      <span>${pct}% úteis · ${r.relevant} 👍 · ${r.not_relevant} 👎</span>
    </div>`;
  }).join('');
}

function renderDmCards(d) {
  const sch = d.scheduler;
  const set = (id, v) => { document.getElementById(id).innerHTML = v; };

  set('dm-state', sch.running ? 'Varrendo agora' : (d.enabled ? 'Ativa' : 'Desativada'));
  const users = sch.user_runs ? ` · ${sch.user_runs} busca${sch.user_runs > 1 ? 's' : ''} de usuário rodando` : '';
  set('dm-state-sub', (d.enabled ? `a cada ${dmIntervalLabel(d.interval_minutes)}` : 'nenhuma busca nos sites') + users);

  if (!d.enabled) {
    set('dm-next', '—');
    set('dm-next-sub', 'busca desativada');
  } else if (sch.running) {
    set('dm-next', 'após esta');
    set('dm-next-sub', `${dmIntervalLabel(d.interval_minutes)} depois do início desta`);
  } else {
    set('dm-next', escDisc(dmUntil(sch.next_run_at)));
    set('dm-next-sub', dmValidTime(sch.next_run_at) ? fmtDate(sch.next_run_at) : '');
  }

  const last = d.runs.find(r => r.trigger !== 'user' && !r.partial && r.status !== 'running');
  if (last) {
    set('dm-last', dmStatusBadge(last.status));
    set('dm-last-sub', `${relativeTime(last.started_at)} · ${dmDuration(last.started_at, last.finished_at)} · ${last.jobs_new} novas de ${last.jobs_saved} lidas`);
  } else {
    set('dm-last', '—');
    set('dm-last-sub', 'nenhuma varredura completa ainda');
  }

  const active = d.sources.reduce((n, s) => n + s.jobs_active, 0);
  const total  = d.sources.reduce((n, s) => n + s.jobs_total, 0);
  set('dm-catalog', `${active}`);
  set('dm-catalog-sub', `vagas ativas · ${total} no banco`);
}

// Andamento: os sites rodam em paralelo, então cada um tem sua barra. Site de busca
// conta termos; site de lista conta páginas ou empresas (total só depois da 1ª página).
// A estimativa geral é a do site que vai terminar por último.
function renderDmProgress(sch) {
  const box = document.getElementById('dm-progress');
  const p = sch.progress;
  if (!p) { box.classList.add('hidden'); box.innerHTML = ''; return; }

  const now = Date.now();
  const elapsed = (now - new Date(p.started_at)) / 1000;
  let etaMax = 0, etaKnown = true;
  const rows = p.sources.map(sp => {
    const done = !!sp.ended_at;
    const pct = done ? 100 : sp.total ? Math.min(99, Math.round(sp.done / sp.total * 100)) : 0;
    let eta = '';
    if (done) {
      eta = `terminou em ${dmSecs((new Date(sp.ended_at) - new Date(sp.started_at)) / 1000)}`;
    } else if (sp.done > 0 && sp.total > sp.done) {
      const left = (now - new Date(sp.started_at)) / 1000 / sp.done * (sp.total - sp.done);
      etaMax = Math.max(etaMax, left);
      eta = `~${dmSecs(left)}`;
    } else {
      etaKnown = false;
      eta = 'estimando…';
    }
    const unit = sp.kind === 'list' ? '' : ' termos';
    const count = sp.total ? `${sp.done}/${sp.total}${unit}` : (sp.done ? `${sp.done} lidas` : '');
    const now_ = done ? '' : sp.current
      ? (sp.kind === 'search' ? `“${escDisc(sp.current)}”${sp.page ? ` · pág. ${sp.page}` : ''}` : escDisc(sp.current))
      : 'preparando…';
    return `
      <div class="dm-src-row${done ? ' is-done' : ''}">
        <div class="dm-src-name"><b>${escDisc(dadmSourceName(sp.source))}</b> <span class="dadm-kind">${DADM_KINDS[sp.kind] || ''}</span></div>
        <div class="dm-bar"><i style="width:${pct}%"></i></div>
        <div class="dm-src-info">
          <span>${count}</span>
          <span>${sp.jobs_saved} vagas${sp.jobs_new ? ` · <b>${sp.jobs_new} novas</b>` : ''}</span>
          ${sp.failed ? `<span style="color:var(--red)">${sp.failed} falha${sp.failed > 1 ? 's' : ''}</span>` : ''}
          <span class="dm-src-now">${now_}</span>
          <span>${eta}</span>
        </div>
      </div>`;
  }).join('');

  const running = p.sources.filter(sp => !sp.ended_at).length;
  const etaText = !running ? 'gravando…' : etaKnown ? `~${dmSecs(etaMax)} restantes` : 'estimando…';
  box.classList.remove('hidden');
  box.innerHTML = `
    <div class="dm-progress-head">
      <div class="metric-card-title" style="margin:0">Varredura em andamento · ${DM_TRIGGERS[p.trigger] || escDisc(p.trigger)}${p.partial ? ' · <span class="badge badge-mid">parcial</span>' : ''}</div>
      <span class="disc-status">${p.sources.length - running} de ${p.sources.length} sites prontos · ${etaText}</span>
    </div>
    <div class="dm-facts">
      <span>Tempo <b>${dmSecs(elapsed)}</b></span>
      <span>Vagas gravadas <b>${p.jobs_saved}</b></span>
      <span>Novas <b>${p.jobs_new}</b></span>
      <span>Termos <b>${p.keywords}</b></span>
      <span>Falhas <b style="color:${p.failed ? 'var(--red)' : 'inherit'}">${p.failed}</b></span>
    </div>
    <div class="dm-src-list">${rows}</div>
    ${p.last_error ? `<div class="dadm-err" style="margin-top:8px;max-width:none">Último erro: ${escDisc(p.last_error)}</div>` : ''}`;
}

// Resultado de um site numa varredura: termos ok/falhos (busca) ou listas/empresas (lista).
function dmSourceResult(s) {
  if (s.kind === 'list') {
    if (!s.keywords_failed) return 'lista ✓';
    return s.keywords_ok ? `${s.keywords_ok}✓ ${s.keywords_failed}✗` : 'lista ✗';
  }
  return `${s.keywords_ok}✓${s.keywords_failed ? ` ${s.keywords_failed}✗` : ''}`;
}

function renderDmSources(sources) {
  const tbody = document.getElementById('dm-sources');
  if (!sources.length) { tbody.innerHTML = '<tr><td colspan="6" class="empty">Nenhum site registrado.</td></tr>'; return; }

  tbody.innerHTML = sources.map(s => {
    const state = !s.registered ? '<span style="color:var(--muted)">removido do código</span>'
      : s.enabled ? '<span class="badge badge-completed">ligado</span>'
      : '<span class="badge badge-pending">desligado</span>';
    const last = !s.last_run_at ? '<span style="color:var(--muted)">nunca</span>'
      : s.keywords_failed === 0 ? `<span class="badge badge-completed">ok</span> ${relativeTime(s.last_run_at)}`
      : s.keywords_ok === 0     ? `<span class="badge badge-error">falhou</span> ${relativeTime(s.last_run_at)}`
      : `<span class="badge badge-mid">${s.keywords_failed} falha${s.keywords_failed > 1 ? 's' : ''}</span> ${relativeTime(s.last_run_at)}`;
    const newJobs = s.last_run_at ? `<div class="disc-status">${s.jobs_new} novas na última</div>` : '';
    // ATS sem empresa cadastrada não lê nada: a varredura sai "ok" com 0 vagas.
    const boards = s.boards == null ? ''
      : s.boards === 0 ? '<div class="disc-status dm-no-boards">nenhuma empresa cadastrada: adicione em Busca de vagas → Empresas nos ATS</div>'
      : `<div class="disc-status">${s.boards} empresa${s.boards > 1 ? 's' : ''}</div>`;
    return `
      <tr>
        <td data-label="Site"><b>${escDisc(dadmSourceName(s.name))}</b>${s.kind ? `<div class="dadm-kind">${DADM_KINDS[s.kind] || ''}</div>` : ''}</td>
        <td data-label="Situação">${state}${boards}</td>
        <td data-label="Última varredura">${last}</td>
        <td data-label="Vagas ativas">${s.jobs_active} <span style="color:var(--muted)">/ ${s.jobs_total}</span>${newJobs}</td>
        <td data-label="Último sucesso">${s.last_success_at ? relativeTime(s.last_success_at) : '—'}</td>
        <td data-label="Último erro">${s.last_error ? `<div class="dadm-err">${escDisc(s.last_error)}</div>` : '—'}</td>
      </tr>`;
  }).join('');
}

function renderDmRuns(runs) {
  const tbody = document.getElementById('dm-runs');
  if (!runs.length) { tbody.innerHTML = '<tr><td colspan="7" class="empty">Nenhuma varredura ainda.</td></tr>'; return; }

  tbody.innerHTML = runs.map(r => {
    const sources = r.sources.map(s => {
      const cls = s.keywords_failed === 0 ? 'match-tag' : 'dadm-err';
      const tip = [s.last_error, `${s.jobs_saved} vagas, ${s.jobs_new} novas, ${dmSecs(s.duration_ms / 1000)}`].filter(Boolean).join(' · ');
      return `<span class="${cls}" title="${escDisc(tip)}">${escDisc(dadmSourceName(s.source))} ${dmSourceResult(s)}</span>`;
    }).join(' ') || '<span style="color:var(--muted)">—</span>';
    return `
      <tr>
        <td data-label="Início">${fmtDate(r.started_at)}</td>
        <td data-label="Gatilho">${DM_TRIGGERS[r.trigger] || escDisc(r.trigger)}${r.partial ? ' <span class="badge badge-mid" title="Só parte dos sites ou só o catálogo: não conta para o agendamento">parcial</span>' : ''}</td>
        <td data-label="Status">${dmStatusBadge(r.status)}</td>
        <td data-label="Duração">${r.status === 'running' ? dmSecs((Date.now() - new Date(r.started_at)) / 1000) + '…' : dmDuration(r.started_at, r.finished_at)}</td>
        <td data-label="Termos">${r.keywords}</td>
        <td data-label="Vagas"><b>${r.jobs_new}</b> novas <span style="color:var(--muted)">· ${r.jobs_saved} lidas${r.jobs_removed ? ` · −${r.jobs_removed}` : ''}</span></td>
        <td data-label="Sites">${sources}</td>
      </tr>`;
  }).join('');
}
