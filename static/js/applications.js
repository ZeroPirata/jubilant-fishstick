// ── Candidaturas: quadro e etapa de cada vaga ──
// Depois do currículo pronto: enviado → entrevista → proposta → resultado. Para dar
// pouco trabalho: "Currículo pronto" se preenche sozinho (vagas concluídas), o próximo
// passo é um clique (com Desfazer), a data é a de hoje (dá para mudar) e baixar o PDF
// pergunta se já enviou. Cada marcação entra no histórico da vaga.

const appStageInfo = {
  ready:     { label: 'Currículo pronto', cls: 'st-ready' },
  applied:   { label: 'Enviado', cls: 'st-applied' },
  interview: { label: 'Entrevista', cls: 'st-interview' },
  offer:     { label: 'Proposta', cls: 'st-offer' },
  hired:     { label: 'Contratado', cls: 'st-hired' },
  rejected:  { label: 'Não seguiu', cls: 'st-rejected' },
  withdrawn: { label: 'Desisti', cls: 'st-withdrawn' },
  note:      { label: 'Anotação', cls: 'st-note' },
};

// Próximo passo de um clique em cada etapa.
const appNext = {
  ready:     ['applied', 'Marquei como enviado'],
  applied:   ['interview', 'Me chamaram'],
  interview: ['offer', 'Recebi proposta'],
  offer:     ['hired', 'Fui contratado'],
};

const appColumns = [
  { id: 'ready', title: 'Currículo pronto', empty: 'Nenhum currículo esperando envio.' },
  { id: 'applied', title: 'Enviado', empty: 'Marque aqui as vagas para as quais você mandou o currículo.' },
  { id: 'interview', title: 'Entrevista', empty: 'Nenhuma em entrevista.' },
  { id: 'offer', title: 'Proposta', empty: 'Nenhuma proposta ainda.' },
  { id: 'closed', title: 'Encerradas', empty: 'Contratado, não seguiu ou desistência aparecem aqui.' },
];

// Enviado há tanto tempo sem resposta: o cartão sugere encerrar (não muda sozinho).
const APP_STALE_DAYS = 21;
// Quantos cartões cada coluna mostra antes do "Mostrar mais".
const APP_COL_LIMIT = 8;

const _board = { items: [], col: 'ready', expanded: {}, seq: 0, loaded: false };
const _appAsked = new Set(); // vagas em que o "já enviou?" do PDF já apareceu

const appClosed = s => s === 'hired' || s === 'rejected' || s === 'withdrawn';
const appKey = stage => stage || 'ready';
const appColumnOf = stage => appClosed(stage) ? 'closed' : appKey(stage);

function appDaysSince(iso) {
  if (!iso) return null;
  return Math.max(0, Math.floor((Date.now() - new Date(iso).getTime()) / 86400000));
}

function appAgo(iso) {
  const d = appDaysSince(iso);
  if (d === null) return '';
  if (d === 0) return 'hoje';
  if (d === 1) return 'ontem';
  return `há ${d} dias`;
}

function appToday() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

function appStageChip(stage) {
  if (!stage) return '';
  const s = appStageInfo[stage];
  return s ? `<span class="app-chip ${s.cls}">${escHtml(s.label)}</span>` : '';
}

// ── Gravar ──

async function appPost(jobId, body) {
  const res = await api.post(`/jobs/${jobId}/application/events`, body);
  if (!res.ok) {
    let msg = 'Não foi possível salvar.';
    try { msg = (await res.json()).error || msg; } catch (_) {}
    throw new Error(msg);
  }
  return apiData(res);
}

// Um clique: marca a etapa com a data de hoje e oferece Desfazer.
async function appQuick(jobId, stage, note = '', btn = null) {
  if (btn) btn.disabled = true;
  try {
    const data = await appPost(jobId, { stage, note });
    appAfterChange(jobId, data);
    appToastUndo(`${appStageInfo[stage]?.label || 'Etapa'}: marcado.`, jobId, data.event_id);
  } catch (e) {
    if (btn) btn.disabled = false;
    toast(e.message, 'error');
  }
}

async function appUndo(jobId, eventId) {
  try {
    const res = await api.delete(`/jobs/${jobId}/application/events/${eventId}`);
    if (!res.ok) throw new Error('Não foi possível desfazer.');
    appAfterChange(jobId, await apiData(res));
    toast('Desfeito.');
  } catch (e) { toast(e.message, 'error'); }
}

// Toast clicável (como o da importação): "… · Desfazer".
function appToastUndo(msg, jobId, eventId) {
  toast(eventId ? `${msg} Toque aqui para desfazer.` : msg);
  if (!eventId) return;
  const el = document.getElementById('toast');
  el.classList.add('toast-action');
  el.onclick = () => { el.classList.add('hidden'); el.onclick = null; appUndo(jobId, eventId); };
  clearTimeout(_toastTimer);
  _toastTimer = setTimeout(() => { el.classList.add('hidden'); el.classList.remove('toast-action'); el.onclick = null; }, 7000);
}

// Depois de gravar: atualiza a tela aberta (quadro, tela da vaga) e a lista de Vagas.
function appAfterChange(jobId, data) {
  const it = _board.items.find(x => x.id === jobId);
  if (it && data) {
    it.stage = data.stage || '';
    it.stage_at = data.stage_at || null;
    const sent = (data.events || []).filter(e => e.stage === 'applied').map(e => e.happened_at).sort();
    it.applied_at = sent[0] || null;
    it.reached_interview = (data.events || []).some(e => ['interview', 'offer', 'hired'].includes(e.stage));
    it.last_note = (data.events || []).find(e => e.note)?.note || '';
  }
  if (_activeTab === 'candidaturas') renderBoard();
  if (_activeTab === 'vaga' && _jd.id === jobId && typeof jdApplicationChanged === 'function') jdApplicationChanged(data);
  if (_activeTab === 'vagas' && typeof loadJobs === 'function') loadJobs();
}

// ── Editor: qualquer etapa, data e anotação ──

function appEditor(jobId, stage, ctx) {
  const cur = stage || '';
  const next = appNext[appKey(cur)]?.[0] || 'note';
  // A etapa atual não aparece de novo, exceto Entrevista (2ª, 3ª rodada...).
  const opts = ['applied', 'interview', 'offer', 'hired', 'rejected', 'withdrawn', 'note'].filter(s => s !== cur || s === 'interview');
  if (cur) opts.push('ready');
  const label = s => s === 'ready' ? 'Voltar para "Currículo pronto"' : appStageInfo[s].label;
  return `
    <div class="app-editor" data-job="${jobId}" data-ctx="${ctx}">
      <div class="app-opts" role="radiogroup" aria-label="Etapa">
        ${opts.map(s => `
          <label class="app-opt ${appStageInfo[s].cls}">
            <input type="radio" name="app-stage-${ctx}-${jobId}" value="${s}" ${s === next ? 'checked' : ''}>
            <span>${escHtml(label(s))}</span>
          </label>`).join('')}
      </div>
      <div class="app-editor-row">
        <label class="app-date">Data <input type="date" value="${appToday()}" max="${appToday()}"></label>
        <textarea rows="2" maxlength="1000" placeholder="Anotação (opcional)" title="Ex.: com quem falou, próxima etapa, faixa salarial"></textarea>
      </div>
      <div class="app-editor-actions">
        <button type="button" class="btn btn-primary btn-sm" onclick="appSaveEditor(this)">Salvar</button>
        <button type="button" class="btn btn-ghost btn-sm" onclick="appCloseEditor(this)">Cancelar</button>
      </div>
    </div>`;
}

async function appSaveEditor(btn) {
  const ed = btn.closest('.app-editor');
  const jobId = ed.dataset.job;
  const stage = ed.querySelector('input[type=radio]:checked')?.value;
  const date = ed.querySelector('input[type=date]').value;
  const note = ed.querySelector('textarea').value.trim();
  if (!stage) return;
  if (stage === 'note' && !note) { toast('Escreva a anotação.', 'error'); return; }
  ed.querySelectorAll('button, input, textarea').forEach(b => b.disabled = true);
  try {
    const data = await appPost(jobId, { stage, note, date: date === appToday() ? '' : date });
    appAfterChange(jobId, data);
    toast(stage === 'note' ? 'Anotação salva.' : `${appStageInfo[stage]?.label || 'Etapa'}: salvo.`);
  } catch (e) {
    ed.querySelectorAll('button, input, textarea').forEach(b => b.disabled = false);
    toast(e.message, 'error');
  }
}

function appCloseEditor(btn) {
  const ed = btn.closest('.app-editor');
  const holder = ed.parentElement;
  ed.remove();
  holder?.closest('.board-card')?.classList.remove('editing');
}

// ── Quadro ──

async function loadBoard() {
  const root = document.getElementById('board-root');
  if (!root) return;
  const seq = ++_board.seq;
  if (!_board.loaded) root.innerHTML = '<p class="jl-empty">Carregando…</p>';
  try {
    const res = await api.get('/applications');
    if (seq !== _board.seq) return;
    if (!res.ok) throw new Error('Erro ao carregar as candidaturas.');
    _board.items = (await apiData(res)) || [];
    _board.loaded = true;
    renderBoard();
  } catch (e) {
    if (seq === _board.seq) root.innerHTML = `<p class="jl-empty">Erro: ${escHtml(e.message)}</p>`;
  }
}

function boardSortKey(it) {
  return new Date(it.stage ? it.stage_at : (it.resume_at || it.created_at)).getTime() || 0;
}

function renderBoard() {
  const root = document.getElementById('board-root');
  if (!root) return;
  const items = _board.items;
  if (!items.length) {
    root.innerHTML = `
      <div class="board-empty">
        <p><b>Nenhuma candidatura ainda.</b></p>
        <p>Quando um currículo fica pronto, a vaga aparece aqui em "Currículo pronto". Depois de mandar para a empresa, marque "Enviado" e acompanhe as próximas etapas.</p>
        <button class="btn btn-primary btn-sm" onclick="navigateTo('descobrir')">Procurar vagas no Descobrir</button>
      </div>`;
    return;
  }

  const groups = Object.fromEntries(appColumns.map(c => [c.id, []]));
  for (const it of items) groups[appColumnOf(it.stage)].push(it);
  for (const k in groups) groups[k].sort((a, b) => boardSortKey(b) - boardSortKey(a));

  const sent = items.filter(it => it.applied_at);
  const answered = sent.filter(it => it.reached_interview).length;
  const active = items.filter(it => ['applied', 'interview', 'offer'].includes(it.stage)).length;
  const stale = items.filter(appIsStale).length;
  const stats = [
    ['Enviadas', sent.length, 'Currículos que você marcou como enviados'],
    ['Em andamento', active, 'Enviado, entrevista ou proposta'],
    ['Chegaram a entrevista', sent.length ? `${answered} · ${Math.round(answered * 100 / sent.length)}%` : '—', 'Das enviadas, quantas chamaram para entrevista (mesmo que depois não tenham seguido)'],
    ['Sem retorno', stale, `Enviadas há ${APP_STALE_DAYS}+ dias sem resposta`],
  ];

  if (!groups[_board.col]) _board.col = 'ready';
  root.innerHTML = `
    <div class="board-stats">
      ${stats.map(([k, v, t]) => `<div class="board-stat" title="${escHtml(t)}"><b>${escHtml(String(v))}</b><span>${escHtml(k)}</span></div>`).join('')}
    </div>
    <div class="board-switch" role="tablist" aria-label="Etapas">
      ${appColumns.map(c => `<button role="tab" class="${_board.col === c.id ? 'active' : ''}" aria-selected="${_board.col === c.id}" onclick="boardShowCol('${c.id}')">${escHtml(c.title)} <span>${groups[c.id].length}</span></button>`).join('')}
    </div>
    <div class="board">
      ${appColumns.map(c => boardColumn(c, groups[c.id])).join('')}
    </div>`;
}

function boardColumn(c, list) {
  const all = !!_board.expanded[c.id];
  const shown = all ? list : list.slice(0, APP_COL_LIMIT);
  const more = list.length - shown.length;
  return `
    <section class="board-col col-${c.id}${_board.col === c.id ? ' active' : ''}" data-col="${c.id}"
      ondragover="boardDragOver(event)" ondragleave="boardDragLeave(event)" ondrop="boardDrop(event, '${c.id}')">
      <header class="board-col-head"><span>${escHtml(c.title)}</span><span class="board-count">${list.length}</span></header>
      <div class="board-cards">
        ${shown.length ? shown.map(boardCard).join('') : `<p class="board-col-empty">${escHtml(c.empty)}</p>`}
        ${more > 0 ? `<button class="btn btn-ghost btn-sm board-more" onclick="boardExpand('${c.id}', true)">Mostrar mais ${more}</button>` : ''}
        ${all && list.length > APP_COL_LIMIT ? `<button class="btn btn-ghost btn-sm board-more" onclick="boardExpand('${c.id}', false)">Mostrar menos</button>` : ''}
      </div>
    </section>`;
}

function appIsStale(it) {
  return it.stage === 'applied' && appDaysSince(it.stage_at) >= APP_STALE_DAYS;
}

function boardCard(it) {
  const key = appKey(it.stage);
  const next = appNext[key];
  const when = key === 'ready'
    ? (it.resume_at ? `currículo gerado ${appAgo(it.resume_at)}` : '')
    : appClosed(it.stage) ? `encerrada ${appAgo(it.stage_at)}` : `${appStageInfo[key].label.toLowerCase()} ${appAgo(it.stage_at)}`;
  const q = it.quality ? `<span class="badge badge-${it.quality}">${escHtml(qualityLabels[it.quality] || it.quality)}</span>` : '';
  return `
    <article class="board-card${appIsStale(it) ? ' stale' : ''}" draggable="true" data-id="${it.id}"
      ondragstart="boardDragStart(event, '${it.id}')" ondragend="boardDragEnd(event)">
      <a class="board-title" href="#/vaga/${it.id}" onclick="event.preventDefault(); openJobDetail('${it.id}', { from: 'candidaturas', focus: 'application' })">${escHtml(it.job_title || 'Vaga sem título')}</a>
      <div class="board-company">${escHtml(it.company_name || 'Empresa não identificada')}</div>
      <div class="board-meta">
        ${appClosed(it.stage) ? appStageChip(it.stage) : ''}
        ${q}
        ${when ? `<span class="board-when">${escHtml(when)}</span>` : ''}
      </div>
      ${it.last_note ? `<p class="board-note" title="${escHtml(it.last_note)}">${escHtml(it.last_note)}</p>` : ''}
      ${appIsStale(it) ? `
        <div class="board-stale">Sem retorno ${escHtml(appAgo(it.stage_at))}.
          <button type="button" class="btn-link" onclick="appQuick('${it.id}', 'rejected', 'Sem retorno da empresa', this)">Encerrar</button>
        </div>` : ''}
      <div class="board-actions">
        ${next ? `<button type="button" class="btn btn-ghost btn-sm board-next" onclick="appQuick('${it.id}', '${next[0]}', '', this)">${escHtml(next[1])}</button>` : ''}
        <button type="button" class="btn btn-ghost btn-sm board-edit" onclick="boardEdit('${it.id}', this)" title="Outra etapa, data ou anotação" aria-label="Outra etapa, data ou anotação">⋯</button>
      </div>
    </article>`;
}

function boardEdit(id, btn) {
  const card = btn.closest('.board-card');
  const open = card.querySelector('.app-editor');
  if (open) { open.remove(); card.classList.remove('editing'); return; }
  const it = _board.items.find(x => x.id === id);
  card.classList.add('editing');
  card.insertAdjacentHTML('beforeend', appEditor(id, it?.stage || '', 'board'));
}

function boardShowCol(id) {
  _board.col = id;
  renderBoard();
}

function boardExpand(id, on) {
  _board.expanded[id] = on;
  renderBoard();
}

// ── Arrastar (desktop) ──
// Soltar numa coluna marca a etapa dela; em "Encerradas" abre a escolha do resultado.

function boardDragStart(ev, id) {
  if (ev.target.closest?.('.app-editor')) { ev.preventDefault(); return; }
  ev.dataTransfer.setData('text/plain', id);
  ev.dataTransfer.effectAllowed = 'move';
  ev.currentTarget.classList.add('dragging');
}

function boardDragEnd(ev) {
  ev.currentTarget.classList.remove('dragging');
  document.querySelectorAll('.board-col.drop').forEach(c => c.classList.remove('drop'));
}

function boardDragOver(ev) {
  ev.preventDefault();
  ev.currentTarget.classList.add('drop');
}

function boardDragLeave(ev) {
  if (!ev.currentTarget.contains(ev.relatedTarget)) ev.currentTarget.classList.remove('drop');
}

function boardDrop(ev, col) {
  ev.preventDefault();
  ev.currentTarget.classList.remove('drop');
  const id = ev.dataTransfer.getData('text/plain');
  const it = _board.items.find(x => x.id === id);
  if (!it || appColumnOf(it.stage) === col) return;
  if (col === 'closed') {
    const card = document.querySelector(`.board-card[data-id="${id}"]`);
    if (!card || card.querySelector('.app-editor')) return;
    card.classList.add('editing');
    card.insertAdjacentHTML('beforeend', appEditor(id, it.stage, 'board'));
    const r = card.querySelector('input[value="rejected"]');
    if (r) r.checked = true;
    return;
  }
  appQuick(id, col === 'ready' ? 'ready' : col);
}

// ── Tela da vaga: aba "Candidatura" ──

function appStepper(stage) {
  const key = appKey(stage);
  const order = ['ready', 'applied', 'interview', 'offer', 'result'];
  const pos = appClosed(stage) ? 4 : order.indexOf(key);
  const label = s => s === 'result' ? (appClosed(stage) ? appStageInfo[stage].label : 'Resultado') : s === 'ready' ? 'Pronto' : appStageInfo[s].label;
  return `
    <ol class="app-steps">
      ${order.map((s, i) => `<li class="${i < pos ? 'done' : i === pos ? 'now' : ''}${s === 'result' && appClosed(stage) ? ' ' + appStageInfo[stage].cls : ''}">${escHtml(label(s))}</li>`).join('')}
    </ol>`;
}

function appPanel(jobId, app, done) {
  const stage = app?.stage || '';
  const key = appKey(stage);
  const next = appNext[key];
  const events = app?.events || [];
  const since = stage ? `${appStageInfo[key].label} ${appAgo(app.stage_at)}` : (done ? 'Currículo pronto, ainda não enviado' : 'Currículo ainda não gerado');
  const stale = stage === 'applied' && appDaysSince(app.stage_at) >= APP_STALE_DAYS;
  return `
    ${appStepper(stage)}
    <p class="app-now">Etapa atual: <b>${escHtml(since)}</b></p>
    ${stale ? `<p class="app-stale-note">Sem retorno ${escHtml(appAgo(app.stage_at))}. Se a empresa não respondeu, encerre para o quadro ficar limpo.
      <button type="button" class="btn-link" onclick="appQuick('${jobId}', 'rejected', 'Sem retorno da empresa', this)">Encerrar: sem retorno</button></p>` : ''}
    <div class="app-actions" id="jd-app-actions">
      ${next ? `<button type="button" class="btn btn-primary btn-sm" onclick="appQuick('${jobId}', '${next[0]}', '', this)">${escHtml(next[1])}</button>` : ''}
      <button type="button" class="btn btn-ghost btn-sm" onclick="jdAppEdit(this)">Outra etapa ou anotação</button>
    </div>
    <div id="jd-app-editor"></div>
    <div class="quality-group-label app-hist-title">Histórico</div>
    ${events.length ? `
      <ul class="app-timeline">
        ${events.map(e => `
          <li>
            <span class="app-tl-date">${escHtml(fmtDay(e.happened_at))}</span>
            <span class="app-tl-body">
              ${e.stage === 'ready' ? '<span class="app-chip st-ready">Voltou para currículo pronto</span>' : appStageChip(e.stage)}
              ${e.note ? `<span class="app-tl-note">${escHtml(e.note)}</span>` : ''}
            </span>
            <button type="button" class="app-tl-del" onclick="jdAppDelete('${e.id}', this)" title="Apagar este item" aria-label="Apagar este item">✕</button>
          </li>`).join('')}
      </ul>` : '<p class="jd-empty-inline">Nada marcado ainda. Quando mandar o currículo para a empresa, use "Marquei como enviado".</p>'}`;
}

function jdAppEdit(btn) {
  const holder = document.getElementById('jd-app-editor');
  if (!holder) return;
  if (holder.firstElementChild) { holder.innerHTML = ''; return; }
  holder.innerHTML = appEditor(_jd.id, _jd.stage || '', 'job');
}

async function jdAppDelete(eventId, btn) {
  if (!await confirmDialog('Apagar este item do histórico?')) return;
  btn.disabled = true;
  try {
    const res = await api.delete(`/jobs/${_jd.id}/application/events/${eventId}`);
    if (!res.ok) throw new Error('Não foi possível apagar.');
    appAfterChange(_jd.id, await apiData(res));
  } catch (e) { btn.disabled = false; toast(e.message, 'error'); }
}

// Depois de baixar o PDF na tela da vaga: se ainda não está marcado como enviado,
// pergunta uma vez (por vaga, nesta sessão).
function appAskSentAfterPdf() {
  if (_activeTab !== 'vaga' || !_jd.id || _jd.stage || _appAsked.has(_jd.id)) return;
  const jobId = _jd.id;
  _appAsked.add(jobId);
  setTimeout(() => {
    toast('Já enviou este currículo para a empresa? Toque aqui para marcar como enviado.');
    const el = document.getElementById('toast');
    el.classList.add('toast-action');
    el.onclick = () => { el.classList.add('hidden'); el.onclick = null; appQuick(jobId, 'applied'); };
    clearTimeout(_toastTimer);
    _toastTimer = setTimeout(() => { el.classList.add('hidden'); el.classList.remove('toast-action'); el.onclick = null; }, 10000);
  }, 600);
}
