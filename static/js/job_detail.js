// ── Tela da vaga ──
// Tudo de uma vaga num lugar: cabeçalho e ações, compatibilidade (análise da IA e
// stack × perfil), currículos e cover letters gerados, e a descrição (resumo da IA,
// texto completo lido do site e o que o usuário colou). Abre ao clicar na vaga em
// Vagas, em "Currículos"/qualidade e em "Ver currículo" no Descobrir.

// tab: painel aberto (fit | keywords | resumes | application | about). Como na Ajuda, um por vez em vez de
// uma página longa; continua o mesmo quando a tela recarrega sozinha (SSE).
// app/stage/done: candidatura da vaga (applications.js), para atualizar só a aba dela.
const _jd = { id: null, from: 'vagas', seq: 0, reloadTimer: null, language: '', tab: 'fit', app: null, stage: '', done: false };
const _jdTabs = ['fit', 'keywords', 'resumes', 'application', 'about'];

const _jdModes = { full: 'Currículo e cover letter', resume_only: 'Só currículo', cover_only: 'Só cover letter' };
const _jdSourceLabel = { processamento: 'lida do site no processamento', catalogo: 'do catálogo do Descobrir' };

function openJobDetail(id, { from = 'vagas', focus = '' } = {}) {
  _jd.id = id;
  _jd.from = from;
  _jd.tab = _jdTabs.includes(focus) ? focus : 'fit';
  navigateTo('vaga');
  setRoute('#/vaga/' + id);
  // A tela não tem item próprio no menu: fica marcada a aba de onde veio.
  document.querySelectorAll(`.sidebar button[data-tab="${from}"]`).forEach(b => b.classList.add('active'));
  const crumb = document.getElementById('crumb-current');
  if (crumb) crumb.textContent = `${from} / vaga`;
  window.scrollTo({ top: 0 });
  loadJobDetail();
}

function closeJobDetail() {
  navigateTo(_jd.from || 'vagas');
}

async function loadJobDetail() {
  const id = _jd.id;
  const seq = ++_jd.seq;
  const root = document.getElementById('jd-root');
  if (!id || !root) return;
  if (!root.dataset.id || root.dataset.id !== id) {
    root.innerHTML = '<p class="jd-empty">Carregando…</p>';
  }
  root.dataset.id = id;
  try {
    const [jobRes, resRes, appRes] = await Promise.all([
      api.get(`/jobs/${id}`),
      api.get(`/jobs/${id}/resumes?cursor=0&size=20`),
      api.get(`/jobs/${id}/application`),
    ]);
    if (seq !== _jd.seq) return;
    if (jobRes.status === 404) { root.innerHTML = '<p class="jd-empty">Esta vaga não existe mais.</p>'; return; }
    if (!jobRes.ok) throw new Error('Erro ao carregar a vaga.');
    const job = await apiData(jobRes);
    if (!job || !job.id) throw new Error('Resposta inesperada ao carregar a vaga.');
    const resumes = resRes.ok ? ((await apiList(resRes)).data || []) : [];
    const app = appRes.ok ? await apiData(appRes) : { stage: job.application_stage || '', stage_at: job.application_stage_at, events: [] };
    if (seq !== _jd.seq) return;
    _jd.language = job.language;
    _jd.app = app;
    _jd.stage = app.stage || '';
    _jd.done = job.status === 'completed';
    root.innerHTML = renderJobDetail(job, resumes, app);
  } catch (e) {
    if (seq === _jd.seq) root.innerHTML = `<p class="jd-empty">Erro: ${escHtml(e.message)}</p>`;
  }
}

// SSE: a vaga aberta muda de status (Refazer, processamento terminando) → recarrega.
// Vários eventos chegam em sequência; espera eles assentarem.
function jdOnJobEvent(ev) {
  if (_activeTab !== 'vaga' || ev.id !== _jd.id) return;
  clearTimeout(_jd.reloadTimer);
  _jd.reloadTimer = setTimeout(() => loadJobDetail(), 400);
}

function renderJobDetail(j, resumes, app) {
  const url = /^https?:\/\//i.test(j.url || '') ? j.url : '';
  const done = j.status === 'completed';
  const running = !done && j.status !== 'error';
  const meta = [
    `Cadastrada em ${fmtDate(j.created_at)}`,
    j.mode && _jdModes[j.mode] ? _jdModes[j.mode] : '',
    j.language ? `Idioma: ${j.language === 'en' ? 'inglês' : 'português'}` : '',
  ].filter(Boolean).map(escHtml).join(' · ');

  const header = `
    <div class="jd-header">
      <div class="jd-heading">
        <h2 class="jd-title">${escHtml(j.job_title || 'Vaga sem título')}</h2>
        <div class="jd-company">${escHtml(j.company_name || 'Empresa não identificada')}</div>
        <div class="jd-badges">
          ${badge(j.status)}
          ${j.quality ? `<span class="badge badge-${j.quality}">Compatibilidade ${escHtml((qualityLabels[j.quality] || j.quality).toLowerCase())}</span>` : ''}
          ${j.fit && j.fit.items.length ? `<span class="jd-pct">${j.fit.coverage_pct}% da stack</span>` : ''}
          <span id="jd-app-chip">${appStageChip(app?.stage)}</span>
        </div>
        <div class="jd-meta">${meta}</div>
      </div>
      <div class="jd-actions" id="jd-actions">
        <span id="jd-app-quick" class="jd-app-quick">${jdAppQuick(done, app?.stage)}</span>
        ${url ? `<a class="btn btn-ghost btn-sm" href="${escHtml(url)}" target="_blank" rel="noopener noreferrer">↗ Vaga original</a>` : ''}
        ${!running ? `<button class="btn btn-ghost btn-sm" onclick="jdRetryMenu()">Refazer</button>` : ''}
        <button class="btn btn-ghost btn-sm jd-delete" onclick="jdDelete()">Excluir</button>
      </div>
    </div>
    ${running ? `<div class="jd-running"><span class="disc-spinner" aria-hidden="true"></span>${escHtml(statusLabels[j.status] || 'Processando')}… a página se atualiza sozinha quando terminar.</div>` : ''}`;

  const fitSub = j.fit && j.fit.items.length ? `${j.fit.coverage_pct}% da stack` : (j.quality ? (qualityLabels[j.quality] || j.quality) : 'análise e stack');
  const tabs = [
    ['fit', 'Compatibilidade', fitSub, 'i-chart'],
    ['keywords', 'Palavras-chave', jdKeywordsSub(j.keywords), 'i-search'],
    ['resumes', 'Currículos e cover letters', resumes.length ? `${resumes.length} ${resumes.length === 1 ? 'gerado' : 'gerados'}` : (running ? 'gerando…' : 'nenhum ainda'), 'i-jobs'],
    ['application', 'Candidatura', jdAppSub(app), 'i-board'],
    ['about', 'Sobre a vaga', 'descrição, stack e requisitos', 'i-search'],
  ];
  const nav = `
    <div class="jd-tabs" role="tablist" aria-label="Seções da vaga">
      ${tabs.map(([id, title, sub, icon]) => `
        <button role="tab" id="jd-tab-${id}" class="help-topic ${_jd.tab === id ? 'active' : ''}" aria-selected="${_jd.tab === id}" aria-controls="jd-${id}" onclick="jdTab('${id}')">
          <svg class="ico"><use href="#${icon}"/></svg>
          <span class="help-topic-text"><b>${escHtml(title)}</b><small>${escHtml(sub)}</small></span>
        </button>`).join('')}
    </div>`;

  const fit = `
    <div class="jd-card" id="jd-fit" role="tabpanel"${_jd.tab === 'fit' ? '' : ' hidden'}>
      <h3 class="jd-card-title">Compatibilidade</h3>
      ${renderFitDetails({ stack: j.stacks || [], reqs: j.requirements || [], quality: j.quality, fit: j.fit, analysis: j.analysis })}
    </div>`;

  const keywords = `
    <div class="jd-card" id="jd-keywords" role="tabpanel"${_jd.tab === 'keywords' ? '' : ' hidden'}>
      <h3 class="jd-card-title">Palavras-chave</h3>
      ${renderKeywords(j.keywords)}
    </div>`;

  const resumesCard = `
    <div class="jd-card" id="jd-resumes" role="tabpanel"${_jd.tab === 'resumes' ? '' : ' hidden'}>
      <h3 class="jd-card-title">Currículos e cover letters</h3>
      ${resumes.length
        ? jdResumeToolbar(resumes.length) + renderResumes(j.id, resumes, { selectable: true, quality: j.quality })
        : `<p class="jd-empty-inline">${running ? 'Sendo gerado — aparece aqui quando ficar pronto.' : 'Nenhum currículo gerado ainda. Use “Refazer” para gerar.'}</p>`}
    </div>`;

  const application = `
    <div class="jd-card" id="jd-application" role="tabpanel"${_jd.tab === 'application' ? '' : ' hidden'}>
      <h3 class="jd-card-title">Candidatura</h3>
      <div id="jd-application-body">${appPanel(j.id, app, done)}</div>
    </div>`;

  const about = `
    <div class="jd-card" id="jd-about" role="tabpanel"${_jd.tab === 'about' ? '' : ' hidden'}>
      <h3 class="jd-card-title">Sobre a vaga</h3>
      ${j.description ? `
        <div class="jd-block">
          <div class="quality-group-label">Resumo da vaga (IA)</div>
          <p class="jd-text">${escHtml(j.description)}</p>
        </div>` : ''}
      ${(j.stacks || []).length ? `
        <div class="jd-block">
          <div class="quality-group-label">Stack extraída</div>
          <div class="jd-chips">${j.stacks.map(t => `<span class="fit-tag fit-neutral">${escHtml(t)}</span>`).join('')}</div>
        </div>` : ''}
      ${(j.requirements || []).length ? `
        <div class="jd-block">
          <div class="quality-group-label">Requisitos</div>
          <ul class="fit-list">${j.requirements.map(r => `<li>${escHtml(r)}</li>`).join('')}</ul>
        </div>` : ''}
      ${j.full_description ? `
        <div class="jd-block">
          <div class="quality-group-label">Descrição completa <span class="jd-source">— ${escHtml(_jdSourceLabel[j.full_description_from] || '')}</span></div>
          <div class="jd-full collapsed" id="jd-full">${escHtml(j.full_description)}</div>
          <button class="btn btn-ghost btn-sm" onclick="jdToggleFull(this)">Mostrar tudo</button>
        </div>` : `
        <div class="jd-block"><p class="jd-empty-inline">A descrição completa não foi guardada nesta vaga (processada antes disso). Ela passa a aparecer depois de um “Refazer”${url ? ', ou abra a vaga original' : ''}.</p></div>`}
      ${j.source_description ? `
        <div class="jd-block">
          <div class="quality-group-label">Descrição que você colou</div>
          <div class="jd-full">${escHtml(j.source_description)}</div>
        </div>` : ''}
    </div>`;

  return header + `<div class="jd-body">${nav}<div class="jd-stage">${fit + keywords + resumesCard + application + about}</div></div>`;
}

// ── Candidatura (applications.js) ──
function jdAppSub(app) {
  const st = app?.stage;
  return st ? `${appStageInfo[st]?.label || st} ${appAgo(app.stage_at)}` : 'não enviado';
}

// Atalho no cabeçalho: currículo pronto e ainda não marcado como enviado.
function jdAppQuick(done, stage) {
  return done && !stage
    ? `<button type="button" class="btn btn-ghost btn-sm" onclick="appQuick(_jd.id, 'applied', '', this)" title="Marque quando mandar o currículo para a empresa: a vaga vai para Enviado no quadro de Candidaturas">✓ Marquei como enviado</button>`
    : '';
}

// Depois de marcar uma etapa: atualiza só o que depende dela (o resto da tela fica).
function jdApplicationChanged(app) {
  if (!app) return;
  _jd.app = app;
  _jd.stage = app.stage || '';
  const set = (id, html) => { const el = document.getElementById(id); if (el) el.innerHTML = html; };
  set('jd-application-body', appPanel(_jd.id, app, _jd.done));
  set('jd-app-chip', appStageChip(app.stage));
  set('jd-app-quick', jdAppQuick(_jd.done, app.stage));
  const sub = document.querySelector('#jd-tab-application small');
  if (sub) sub.textContent = jdAppSub(app);
}

// ── Palavras-chave (ATS) ──
// Sistemas de triagem procuram no currículo os termos da vaga. O relatório (calculado
// no servidor, sem IA) compara a stack e as tecnologias da descrição com o último
// currículo gerado e com o perfil.
function jdKeywordsSub(k) {
  if (!k) return 'vaga × currículo';
  const total = k.in_resume.length + k.missing_in_resume.length + k.not_in_profile.length;
  if (!total) return 'nenhum termo';
  return k.has_resume ? `${k.in_resume.length} de ${total} no currículo` : `${total} ${total === 1 ? 'termo' : 'termos'} da vaga`;
}

function renderKeywords(k) {
  if (!k) return '<p class="jd-empty-inline">Cadastre suas skills no Perfil para ver o relatório.</p>';
  const total = k.in_resume.length + k.missing_in_resume.length + k.not_in_profile.length;
  if (!total) return '<p class="jd-empty-inline">A vaga ainda não tem stack nem descrição para comparar.</p>';
  const chips = (list, cls, fmt = t => escHtml(t)) =>
    `<div class="jd-chips">${list.map(t => `<span class="fit-tag ${cls}">${fmt(t)}</span>`).join('')}</div>`;
  const block = (title, hint, body) => `
    <div class="jd-block">
      <div class="quality-group-label">${title}</div>
      <p class="kw-hint">${hint}</p>
      ${body}
    </div>`;
  const out = [];
  out.push(`<p class="kw-intro">Muitas empresas filtram currículos por palavras-chave antes de alguém ler. Estes são os termos desta vaga (stack e tecnologias citadas na descrição), comparados com ${k.has_resume ? 'o último currículo gerado' : 'o seu perfil'}.</p>`);
  if (!k.has_resume) {
    out.push('<p class="jd-empty-inline">Ainda não há currículo gerado para esta vaga: a coluna "no currículo" aparece depois de gerar.</p>');
  } else if (k.in_resume.length) {
    out.push(block(`No currículo <span class="kw-count">${k.in_resume.length}</span>`, 'A vaga pede e o currículo cita.', chips(k.in_resume, 'fit-strong')));
  }
  if (k.missing_in_resume.length) {
    out.push(block(`No perfil, fora do currículo <span class="kw-count">${k.missing_in_resume.length}</span>`,
      k.has_resume
        ? 'Você tem, mas o currículo não cita. Confira se a experiência descreve o uso (o gerador só cita o que o perfil comprova) e use “Refazer” com “Nova versão da IA”.'
        : 'Você tem no perfil: o currículo gerado pode citar.',
      chips(k.missing_in_resume, 'fit-partial', g => escHtml(g.name) + (g.skill && g.skill.toLowerCase() !== g.name.toLowerCase() ? ` <small>${escHtml(g.skill)}</small>` : ''))));
  }
  if (k.not_in_profile.length) {
    out.push(block(`Fora do perfil <span class="kw-count">${k.not_in_profile.length}</span>`,
      'A vaga pede e o perfil não tem. Não entra no currículo: só adicione ao Perfil o que você realmente usou.',
      chips(k.not_in_profile, 'fit-missing')));
  }
  return out.join('');
}

function jdTab(id) {
  if (!_jdTabs.includes(id)) return;
  _jd.tab = id;
  for (const t of _jdTabs) {
    document.getElementById(`jd-${t}`)?.toggleAttribute('hidden', t !== id);
    const btn = document.getElementById(`jd-tab-${t}`);
    if (btn) { btn.classList.toggle('active', t === id); btn.setAttribute('aria-selected', t === id); }
  }
}

// ← → trocam de seção, como na Ajuda (fora de campos de texto).
document.addEventListener('keydown', ev => {
  if (_activeTab !== 'vaga' || ev.altKey || ev.ctrlKey || ev.metaKey) return;
  if (ev.target.closest?.('input, textarea, select, [contenteditable]')) return;
  const step = ev.key === 'ArrowRight' ? 1 : ev.key === 'ArrowLeft' ? -1 : 0;
  if (!step) return;
  const i = _jdTabs.indexOf(_jd.tab) + step;
  if (i >= 0 && i < _jdTabs.length) jdTab(_jdTabs[i]);
});

// ── Seleção de currículos ──
// Marca vários e apaga de uma vez. A seleção vive nos checkboxes; recarregar a tela
// (SSE, exclusão) começa sem nada marcado.
function jdResumeToolbar(total) {
  return `
    <div class="jd-select-bar" id="jd-select-bar">
      <label class="jd-select-all"><input type="checkbox" onchange="jdSelectAll(this.checked)"> Selecionar todos (${total})</label>
      <button class="btn btn-ghost btn-sm jd-delete" id="jd-delete-selected" onclick="jdDeleteSelected()" disabled>Apagar selecionados</button>
    </div>`;
}

function jdSelectedResumes() {
  return [...document.querySelectorAll('#jd-resumes .resume-check:checked')].map(c => c.value);
}

function jdSelectAll(on) {
  document.querySelectorAll('#jd-resumes .resume-check').forEach(c => { c.checked = on; });
  jdSelChanged();
}

function jdSelChanged() {
  const all = document.querySelectorAll('#jd-resumes .resume-check');
  const n = jdSelectedResumes().length;
  document.querySelectorAll('#jd-resumes .resume-card').forEach(card => {
    card.classList.toggle('selected', !!card.querySelector('.resume-check:checked'));
  });
  const btn = document.getElementById('jd-delete-selected');
  if (btn) { btn.disabled = n === 0; btn.textContent = n ? `Apagar selecionados (${n})` : 'Apagar selecionados'; }
  const allBox = document.querySelector('#jd-select-bar input[type=checkbox]');
  if (allBox) { allBox.checked = n > 0 && n === all.length; allBox.indeterminate = n > 0 && n < all.length; }
}

async function jdDeleteSelected() {
  const ids = jdSelectedResumes();
  if (!ids.length) return;
  const msg = ids.length === 1 ? 'Deletar o currículo selecionado?' : `Deletar os ${ids.length} currículos selecionados?`;
  if (!await confirmDialog(msg)) return;
  const jobId = _jd.id;
  document.querySelectorAll('#jd-select-bar button, #jd-select-bar input').forEach(b => b.disabled = true);
  const results = await Promise.allSettled(ids.map(id => api.delete(`/jobs/${jobId}/resumes/${id}`)));
  const failed = results.filter(r => r.status === 'rejected' || !r.value.ok).length;
  const ok = ids.length - failed;
  if (failed) toast(`${ok} apagado(s), ${failed} com erro.`, 'error');
  else toast(ok === 1 ? 'Currículo deletado.' : `${ok} currículos deletados.`);
  loadJobDetail();
}

function jdToggleFull(btn) {
  const el = document.getElementById('jd-full');
  const collapsed = el.classList.toggle('collapsed');
  btn.textContent = collapsed ? 'Mostrar tudo' : 'Mostrar menos';
}

function jdRetryMenu() {
  const lang = _jd.language === 'en' ? 'en' : 'pt-br';
  document.getElementById('jd-actions').innerHTML =
    genChooser({ label: 'Gerar de novo:', lang, onGo: 'jdRetry', onCancel: 'loadJobDetail()', retry: true });
}

async function jdRetry(mode, btn) {
  const language = genLanguage(btn), fresh = genFresh(btn);
  btn.closest('.jd-actions').querySelectorAll('button, select, input').forEach(b => b.disabled = true);
  try {
    const res = await api.put(`/jobs/${_jd.id}/retry`, { mode, language, fresh });
    if (!res.ok) { toast('Erro ao reprocessar.', 'error'); loadJobDetail(); return; }
    toast(`Vaga enviada para reprocessamento (${language === 'en' ? 'inglês' : 'português'}).`);
  } catch (e) { toast(e.message, 'error'); }
  loadJobDetail();
}

async function jdDelete() {
  if (!await confirmDialog('Deletar esta vaga e todos os currículos gerados?')) return;
  try {
    const res = await api.delete(`/jobs/${_jd.id}`);
    if (!res.ok) { toast('Erro ao deletar vaga.', 'error'); return; }
    toast('Vaga deletada.');
    _jd.id = null;
    closeJobDetail();
  } catch (e) { toast(e.message, 'error'); }
}
