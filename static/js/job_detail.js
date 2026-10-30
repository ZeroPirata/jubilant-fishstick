// ── Tela da vaga ──
// Tudo de uma vaga num lugar: cabeçalho e ações, compatibilidade (análise da IA e
// stack × perfil), currículos e cover letters gerados, e a descrição (resumo da IA,
// texto completo lido do site e o que o usuário colou). Abre ao clicar na vaga em
// Vagas, em "Currículos"/qualidade e em "Ver currículo" no Descobrir.

// tab: painel aberto (fit | resumes | about). Como na Ajuda, um por vez em vez de
// uma página longa; continua o mesmo quando a tela recarrega sozinha (SSE).
const _jd = { id: null, from: 'vagas', seq: 0, reloadTimer: null, language: '', tab: 'fit' };
const _jdTabs = ['fit', 'resumes', 'about'];

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
    const [jobRes, resRes] = await Promise.all([
      api.get(`/jobs/${id}`),
      api.get(`/jobs/${id}/resumes?cursor=0&size=20`),
    ]);
    if (seq !== _jd.seq) return;
    if (jobRes.status === 404) { root.innerHTML = '<p class="jd-empty">Esta vaga não existe mais.</p>'; return; }
    if (!jobRes.ok) throw new Error('Erro ao carregar a vaga.');
    const job = await apiData(jobRes);
    if (!job || !job.id) throw new Error('Resposta inesperada ao carregar a vaga.');
    const resumes = resRes.ok ? ((await apiList(resRes)).data || []) : [];
    if (seq !== _jd.seq) return;
    _jd.language = job.language;
    root.innerHTML = renderJobDetail(job, resumes);
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

function renderJobDetail(j, resumes) {
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
        </div>
        <div class="jd-meta">${meta}</div>
      </div>
      <div class="jd-actions" id="jd-actions">
        ${url ? `<a class="btn btn-ghost btn-sm" href="${escHtml(url)}" target="_blank" rel="noopener noreferrer">↗ Vaga original</a>` : ''}
        ${!running ? `<button class="btn btn-ghost btn-sm" onclick="jdRetryMenu()">Refazer</button>` : ''}
        <button class="btn btn-ghost btn-sm jd-delete" onclick="jdDelete()">Excluir</button>
      </div>
    </div>
    ${running ? `<div class="jd-running"><span class="disc-spinner" aria-hidden="true"></span>${escHtml(statusLabels[j.status] || 'Processando')}… a página se atualiza sozinha quando terminar.</div>` : ''}`;

  const fitSub = j.fit && j.fit.items.length ? `${j.fit.coverage_pct}% da stack` : (j.quality ? (qualityLabels[j.quality] || j.quality) : 'análise e stack');
  const tabs = [
    ['fit', 'Compatibilidade', fitSub, 'i-chart'],
    ['resumes', 'Currículos e cover letters', resumes.length ? `${resumes.length} ${resumes.length === 1 ? 'gerado' : 'gerados'}` : (running ? 'gerando…' : 'nenhum ainda'), 'i-jobs'],
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

  const resumesCard = `
    <div class="jd-card" id="jd-resumes" role="tabpanel"${_jd.tab === 'resumes' ? '' : ' hidden'}>
      <h3 class="jd-card-title">Currículos e cover letters</h3>
      ${resumes.length
        ? jdResumeToolbar(resumes.length) + renderResumes(j.id, resumes, { selectable: true })
        : `<p class="jd-empty-inline">${running ? 'Sendo gerado — aparece aqui quando ficar pronto.' : 'Nenhum currículo gerado ainda. Use “Refazer” para gerar.'}</p>`}
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

  return header + `<div class="jd-body">${nav}<div class="jd-stage">${fit + resumesCard + about}</div></div>`;
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
