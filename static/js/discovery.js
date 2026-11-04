// ── Descobrir vagas ──
// A lista é o catálogo (/discovery) filtrado pelas skills avançadas/expert do usuário.
// As palavras-chave (/filters) trazem áreas que o catálogo base não cobre. Os dados das
// vagas vêm de sites de terceiros: tudo passa por escDisc antes de ir para o HTML.

const _discPg = { offset: 0, size: 20 };
// Lista das vagas dispensadas (para restaurar) em vez da lista normal.
let _discShowDismissed = false;
let _discPollTimer = null;
// Cada clique em filtro recarrega a lista; só a resposta do último pedido é desenhada.
let _discLoadSeq = 0;

const _discWorkplace = { remote: 'Remoto', hybrid: 'Híbrido', onsite: 'Presencial' };
const _discSourceNames = {
  gupy: 'Gupy', programathor: 'ProgramaThor', github: 'GitHub', greenhouse: 'Greenhouse',
  lever: 'Lever', himalayas: 'Himalayas', ashby: 'Ashby', workable: 'Workable',
  smartrecruiters: 'SmartRecruiters', recruitee: 'Recruitee', linkedin: 'LinkedIn', catho: 'Catho',
};

// Níveis na ordem da API; o filtro de nível é salvo nas preferências, como o local.
const _discLevelNames = {
  intern: 'Estágio', junior: 'Júnior', mid: 'Pleno', senior: 'Sênior', expert: 'Especialista', manager: 'Gestão',
};
let _discLevels = [];
let _discLevelUnknown = true;

// Filtros marcados e os nomes que cada chip representa (os chips passam só o índice,
// para nome de skill com aspas não quebrar o onclick).
const _discFilters = { sources: [], skills: [] };
const _discFacetNames = { sources: [], skills: [] };

function escDisc(str) {
  return String(str ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function safeHttpUrl(url) {
  return /^https?:\/\//i.test(url || '') ? url : '';
}

// Preferências e filtros ficam recolhidos no celular (abertos no computador), com um
// resumo do que está escolhido na linha do título.
let _discFoldInit = false;
function initDiscFolds() {
  if (_discFoldInit) return;
  _discFoldInit = true;
  const wide = window.matchMedia('(min-width: 861px)').matches;
  ['disc-prefs', 'disc-filters-box'].forEach(id => { const d = document.getElementById(id); if (d) d.open = wide; });
}

function updateDiscFoldSummaries() {
  const modes = [...document.querySelectorAll('input[name="disc-mode"]:checked')]
    .map(cb => cb.closest('label').textContent.trim());
  const cities = splitCsv(document.getElementById('disc-cities')?.value || '');
  const areas = document.querySelectorAll('#disc-keywords .disc-keyword').length;
  const parts = [
    modes.length ? modes.join(', ') : 'qualquer local',
    cities.length ? `${cities.length} cidade${cities.length > 1 ? 's' : ''}` : '',
    _discTechs.length ? `${_discTechs.length} tecnologia${_discTechs.length > 1 ? 's' : ''}` : 'skills do perfil',
    areas ? `${areas} área${areas > 1 ? 's' : ''}` : '',
  ].filter(Boolean);
  const p = document.getElementById('disc-prefs-sum');
  if (p) p.textContent = parts.join(' · ');
  const n = _discLevels.length + _discFilters.sources.length + _discFilters.skills.length;
  const f = document.getElementById('disc-filters-sum');
  if (f) f.textContent = n ? `${n} ativo${n > 1 ? 's' : ''}` : 'nível, plataforma, skills';
}

function loadDiscovery() {
  initDiscFolds();
  loadDiscoveryTechCatalog().then(loadDiscoveryPreferences);
  loadDiscoveryKeywords();
  loadDiscoveryJobs();
  pollDiscoveryStatus();
}

// ── Local de interesse e tecnologias (salvos juntos em /discovery/preferences) ──

// Catálogo de tecnologias (vem do servidor) e as escolhidas pelo usuário.
let _discTechCatalog = [];
let _discTechs = [];

async function loadDiscoveryTechCatalog() {
  if (_discTechCatalog.length) return;
  try {
    const res = await api.get('/discovery/technologies');
    if (res.ok) _discTechCatalog = (await apiData(res)) || [];
  } catch (_) { /* sem catálogo: o card fica vazio */ }
}

async function loadDiscoveryPreferences() {
  try {
    const res = await api.get('/discovery/preferences');
    if (!res.ok) return;
    const p = await apiData(res);
    document.querySelectorAll('input[name="disc-mode"]').forEach(cb => {
      cb.checked = (p.workplace_types || []).includes(cb.value);
    });
    document.getElementById('disc-cities').value = (p.cities || []).join(', ');
    _discTechs = p.technologies || [];
    _discLevels = p.seniorities || [];
    _discLevelUnknown = p.seniority_unknown !== false;
  } catch (_) { /* sem preferências: mostra tudo */ }
  renderDiscTechs();
}

async function saveDiscoveryPreferences() {
  const workplace_types = [...document.querySelectorAll('input[name="disc-mode"]:checked')].map(cb => cb.value);
  const cities = splitCsv(document.getElementById('disc-cities').value);
  try {
    const res = await api.put('/discovery/preferences', {
      workplace_types, cities, technologies: _discTechs,
      seniorities: _discLevels, seniority_unknown: _discLevelUnknown,
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      toast(body.error || 'Erro ao salvar as preferências.', 'error');
      return false;
    }
    _discPg.offset = 0;
    loadDiscoveryJobs();
    return true;
  } catch (e) { toast(e.message, 'error'); return false; }
}

function renderDiscTechs() {
  const box = document.getElementById('disc-techs');
  box.innerHTML = _discTechs.length
    ? _discTechs.map((t, i) => `
      <span class="disc-keyword disc-tech">${escDisc(t)}
        <button class="disc-keyword-x" aria-label="Remover ${escDisc(t)}" title="Remover" onclick="removeDiscoveryTech(${i})">✕</button>
      </span>`).join('')
    : '<span class="card-note" style="margin:0">Nenhuma tecnologia escolhida: a lista usa as skills do seu perfil.</span>';

  const groups = [];
  for (const t of _discTechCatalog) {
    let g = groups.find(g => g.name === t.group);
    if (!g) groups.push(g = { name: t.group, items: [] });
    g.items.push(t);
  }
  document.getElementById('disc-tech-select').innerHTML = '<option value="">Escolha uma tecnologia…</option>' +
    groups.map(g => `<optgroup label="${escDisc(g.name)}">${g.items.map(t => {
      const has = _discTechs.includes(t.name);
      return `<option value="${escDisc(t.name)}"${has ? ' disabled' : ''}>${escDisc(t.name)}${has ? ' ✓' : ''}</option>`;
    }).join('')}</optgroup>`).join('');
  updateDiscFoldSummaries();
}

async function addDiscoveryTech(name) {
  const sel = document.getElementById('disc-tech-select');
  name = name || sel.value;
  if (!name) { toast('Escolha uma tecnologia na lista.', 'error'); return; }
  if (_discTechs.includes(name)) return;
  _discTechs.push(name);
  renderDiscTechs();
  if (await saveDiscoveryPreferences()) {
    toast(`${name} adicionada. A lista já mostra as vagas que citam ela.`);
  } else {
    _discTechs = _discTechs.filter(t => t !== name);
    renderDiscTechs();
  }
}

async function removeDiscoveryTech(i) {
  const removed = _discTechs.splice(i, 1);
  renderDiscTechs();
  if (!await saveDiscoveryPreferences()) {
    _discTechs.splice(i, 0, ...removed);
    renderDiscTechs();
  }
}

// Termo digitado em "Outra área" que é uma tecnologia do catálogo ("golang" → Go).
function discTechForTerm(term) {
  term = term.trim().toLowerCase();
  return _discTechCatalog.find(t => t.name.toLowerCase() === term || t.terms.includes(term));
}

// ── Filtros de nível, plataforma e skill ──

async function toggleDiscLevel(level) {
  const before = [..._discLevels];
  const at = _discLevels.indexOf(level);
  if (at >= 0) _discLevels.splice(at, 1); else _discLevels.push(level);
  if (!await saveDiscoveryPreferences()) _discLevels = before;
}

async function toggleDiscLevelUnknown(cb) {
  _discLevelUnknown = cb.checked;
  if (!await saveDiscoveryPreferences()) _discLevelUnknown = !cb.checked;
}

function renderDiscLevels(levels) {
  const unknown = levels.find(f => f.name === 'unknown');
  document.getElementById('disc-level-chips').innerHTML = levels
    .filter(f => _discLevelNames[f.name])
    .map(f => `
      <button class="disc-chip ${_discLevels.includes(f.name) ? 'on' : ''}" onclick="toggleDiscLevel('${f.name}')">${_discLevelNames[f.name]}<span>${f.count}</span></button>`).join('');
  const cb = document.getElementById('disc-level-unknown');
  cb.checked = _discLevelUnknown;
  // Sem nível marcado o filtro não age: a opção só faz sentido com algum marcado.
  cb.closest('label').classList.toggle('hidden', !_discLevels.length);
  document.getElementById('disc-level-unknown-count').textContent = unknown ? `(${unknown.count})` : '';
}

function toggleDiscFilter(kind, index) {
  const name = _discFacetNames[kind][index];
  if (name === undefined) return;
  const list = _discFilters[kind];
  const at = list.indexOf(name);
  if (at >= 0) list.splice(at, 1); else list.push(name);
  _discPg.offset = 0;
  loadDiscoveryJobs();
}

function renderDiscFacets(facets) {
  const render = (kind, items, label) => {
    // Filtro marcado que sumiu das contagens continua visível, com 0, para dar para desmarcar.
    const names = items.map(f => f.name);
    _discFilters[kind].forEach(n => { if (!names.includes(n)) items.push({ name: n, count: 0 }); });
    _discFacetNames[kind] = items.map(f => f.name);
    return items.map((f, i) => `
      <button class="disc-chip ${_discFilters[kind].includes(f.name) ? 'on' : ''}" onclick="toggleDiscFilter('${kind}', ${i})">${escDisc(label(f.name))}<span>${f.count}</span></button>`).join('')
      || '<span class="card-note" style="margin:0">—</span>';
  };
  document.getElementById('disc-source-chips').innerHTML = render('sources', facets.sources || [], n => _discSourceNames[n] || n);
  document.getElementById('disc-skill-chips').innerHTML  = render('skills',  facets.skills  || [], n => n);
}

// ── Palavras-chave ("Outras áreas") ──
// O usuário escolhe a área numa lista em vez de digitar: cada área vira o termo que
// os sites entendem melhor. "Outra" libera o campo de texto para o que não está na lista.

const DISC_AREAS = [
  ['Tecnologia', [
    ['Desenvolvimento back-end', 'desenvolvedor backend'], ['Desenvolvimento front-end', 'desenvolvedor frontend'],
    ['Full stack', 'full stack'], ['Mobile', 'desenvolvedor mobile'], ['Análise de dados', 'analista de dados'],
    ['Engenharia de dados', 'engenheiro de dados'], ['Ciência de dados', 'cientista de dados'],
    ['Inteligência artificial / ML', 'machine learning'], ['DevOps / SRE', 'devops'], ['Cloud', 'cloud'],
    ['Segurança da informação', 'segurança da informação'], ['QA / Testes', 'qa'], ['Suporte técnico', 'suporte técnico'],
    ['Infraestrutura / Redes', 'analista de infraestrutura'], ['Produto (PM / PO)', 'product manager'], ['UX / UI design', 'ux designer'],
  ]],
  ['Administrativo e negócios', [
    ['Administração', 'assistente administrativo'], ['Recursos humanos', 'recursos humanos'],
    ['Recrutamento e seleção', 'recrutamento e seleção'], ['Departamento pessoal', 'departamento pessoal'],
    ['Financeiro', 'analista financeiro'], ['Contabilidade', 'contabilidade'], ['Fiscal / Tributário', 'analista fiscal'],
    ['Controladoria', 'controladoria'], ['Jurídico', 'advogado'], ['Compras', 'compras'], ['Logística', 'logística'],
    ['Gestão de projetos', 'gerente de projetos'], ['Processos', 'analista de processos'],
  ]],
  ['Comercial e marketing', [
    ['Vendas', 'vendas'], ['Pré-vendas / SDR', 'sdr'], ['Customer success', 'customer success'],
    ['Atendimento ao cliente', 'atendimento ao cliente'], ['Marketing', 'marketing'], ['Marketing digital', 'marketing digital'],
    ['Comunicação', 'comunicação'], ['Design gráfico', 'designer gráfico'],
  ]],
  ['Engenharia e indústria', [
    ['Engenharia civil', 'engenheiro civil'], ['Engenharia de produção', 'engenheiro de produção'],
    ['Engenharia mecânica', 'engenheiro mecânico'], ['Engenharia elétrica', 'engenheiro eletricista'],
    ['Manutenção', 'técnico de manutenção'], ['Segurança do trabalho', 'segurança do trabalho'],
  ]],
  ['Saúde e educação', [
    ['Enfermagem', 'enfermeiro'], ['Farmácia', 'farmacêutico'], ['Nutrição', 'nutricionista'],
    ['Psicologia', 'psicólogo'], ['Educação / Ensino', 'professor'],
  ]],
];
const DISC_AREA_OTHER = '__outra__';
// termo → nome da área, para os chips mostrarem "Recursos humanos" e não o termo cru.
const _discAreaLabel = Object.fromEntries(DISC_AREAS.flatMap(([, list]) => list.map(([label, term]) => [term, label])));

function renderDiscAreaSelect(taken) {
  const sel = document.getElementById('disc-area-select');
  const keep = sel.value;
  sel.innerHTML = '<option value="">Escolha uma área…</option>' + DISC_AREAS.map(([group, list]) => `
    <optgroup label="${escDisc(group)}">${list.map(([label, term]) => {
      const has = taken.has(term);
      return `<option value="${escDisc(term)}"${has ? ' disabled' : ''}>${escDisc(label)}${has ? ' ✓' : ''}</option>`;
    }).join('')}</optgroup>`).join('') +
    `<option value="${DISC_AREA_OTHER}">Outra (digitar)…</option>`;
  if (keep && !taken.has(keep)) sel.value = keep;
  discAreaChanged();
}

function discAreaChanged() {
  const other = document.getElementById('disc-area-select').value === DISC_AREA_OTHER;
  const input = document.getElementById('disc-keyword-input');
  input.classList.toggle('hidden', !other);
  if (other) input.focus();
}

async function loadDiscoveryKeywords() {
  const box = document.getElementById('disc-keywords');
  try {
    const res = await api.get('/filters?size=100');
    const rows = res.ok ? ((await apiData(res)) || []) : [];
    renderDiscAreaSelect(new Set(rows.map(f => f.keyword.toLowerCase())));
    if (!rows.length) {
      box.innerHTML = '<span class="card-note" style="margin:0">Nenhuma área extra ainda. Escolha uma na lista abaixo.</span>';
      return;
    }
    box.innerHTML = rows.map(f => {
      const label = _discAreaLabel[f.keyword.toLowerCase()] || f.keyword;
      return `
      <span class="disc-keyword" title="Buscado como “${escDisc(f.keyword)}”">${escDisc(label)}
        <button class="disc-keyword-x" aria-label="Remover ${escDisc(label)}" title="Remover" onclick="deleteDiscoveryKeyword('${escDisc(f.id)}', this)">✕</button>
      </span>`;
    }).join('');
    updateDiscFoldSummaries();
  } catch (_) {
    box.innerHTML = '<span class="card-note" style="margin:0">Erro ao carregar as áreas.</span>';
  }
}

async function addDiscoveryKeyword() {
  const sel = document.getElementById('disc-area-select');
  const input = document.getElementById('disc-keyword-input');
  const keyword = sel.value === DISC_AREA_OTHER ? input.value.trim() : sel.value;
  if (!keyword) { toast(sel.value === DISC_AREA_OTHER ? 'Digite a área.' : 'Escolha uma área na lista.', 'error'); return; }
  // Tecnologia digitada como área só alimentaria a busca, sem filtrar a lista: vai
  // para "Tecnologias de interesse", que faz as duas coisas.
  const asTech = sel.value === DISC_AREA_OTHER && discTechForTerm(keyword);
  if (asTech) {
    input.value = '';
    sel.value = '';
    discAreaChanged();
    if (_discTechs.includes(asTech.name)) { toast(`“${keyword}” é ${asTech.name}, que já está em Tecnologias de interesse.`); return; }
    toast(`“${keyword}” é uma tecnologia: adicionei ${asTech.name} em Tecnologias de interesse.`);
    addDiscoveryTech(asTech.name);
    return;
  }
  try {
    const res = await api.post('/filters', { keyword });
    if (res.status === 409) { toast('Essa área já está na sua lista.', 'error'); return; }
    if (!res.ok) { toast('Erro ao adicionar a área.', 'error'); return; }
    input.value = '';
    sel.value = '';
    toast('Área adicionada. Buscando vagas nas fontes…');
    loadDiscoveryKeywords();
    refreshDiscovery();
  } catch (e) { toast(e.message, 'error'); }
}

async function deleteDiscoveryKeyword(id, btn) {
  if (btn) btn.disabled = true;
  try {
    const res = await api.delete(`/filters/${id}`);
    if (!res.ok) { toast('Erro ao remover.', 'error'); return; }
    loadDiscoveryKeywords();
    _discPg.offset = 0;
    loadDiscoveryJobs();
  } catch (e) { toast(e.message, 'error'); }
}

// ── Busca ──

async function refreshDiscovery() {
  const btn = document.getElementById('btn-disc-refresh');
  btn.disabled = true;
  try {
    const res = await api.post('/discovery/refresh');
    if (res.status === 202) {
      setDiscoveryStatus({ running: true });
      pollDiscoveryStatus();
      return;
    }
    const body = await res.json().catch(() => ({}));
    toast(body.error || 'Erro ao iniciar a busca.', 'error');
    btn.disabled = false;
  } catch (e) {
    toast(e.message, 'error');
    btn.disabled = false;
  }
}

async function pollDiscoveryStatus() {
  clearTimeout(_discPollTimer);
  try {
    const res = await api.get('/discovery/status');
    if (!res.ok) return;
    const st = await apiData(res);
    const wasRunning = document.getElementById('btn-disc-refresh').dataset.running === '1';
    setDiscoveryStatus(st);
    if (st.running) {
      _discPollTimer = setTimeout(pollDiscoveryStatus, 3000);
    } else if (wasRunning) {
      toast('Busca concluída!');
      _discPg.offset = 0;
      loadDiscoveryJobs();
    }
  } catch (_) { /* status é só informativo */ }
}

function setDiscoveryStatus(st) {
  const btn = document.getElementById('btn-disc-refresh');
  const el  = document.getElementById('disc-status');
  btn.dataset.running = st.running ? '1' : '';
  btn.disabled = !!st.running;
  btn.textContent = st.running ? 'Buscando…' : 'Buscar novas vagas';
  const last = st.last_refresh && !st.last_refresh.startsWith('0001') ? st.last_refresh : null;
  el.textContent = st.running ? 'buscando nas fontes…' : (last ? `última busca: ${fmtDate(last)}` : '');
}

// ── Lista ──

function renderDiscoveryPagination(total, cursor, size) {
  const from = total === 0 ? 0 : cursor + 1;
  const to   = Math.min(cursor + size, total);
  document.getElementById('disc-page-info').textContent = total === 0 ? '—' : `${from}–${to} de ${total}`;
  document.getElementById('disc-prev').disabled = cursor === 0;
  document.getElementById('disc-next').disabled = cursor + size >= total;
}

function discoveryPrevPage() {
  if (_discPg.offset === 0) return;
  _discPg.offset = Math.max(0, _discPg.offset - _discPg.size);
  loadDiscoveryJobs();
}

function discoveryNextPage() {
  _discPg.offset += _discPg.size;
  loadDiscoveryJobs();
}

function changeDiscoverySize(val) {
  _discPg.size   = parseInt(val, 10);
  _discPg.offset = 0;
  loadDiscoveryJobs();
}

async function loadDiscoveryJobs() {
  const seq = ++_discLoadSeq;
  const tbody = document.getElementById('disc-body');
  tbody.innerHTML = '<p class="jl-empty">Carregando...</p>';
  try {
    const qs = new URLSearchParams({ offset: _discPg.offset, size: _discPg.size });
    _discFilters.sources.forEach(s => qs.append('source', s));
    _discFilters.skills.forEach(s => qs.append('skill', s));
    if (_discShowDismissed) qs.set('dismissed', '1');
    const res = await api.get(`/discovery/jobs?${qs}`);
    if (seq !== _discLoadSeq) return;
    if (!res.ok) { tbody.innerHTML = '<p class=\"jl-empty\">Erro ao carregar vagas.</p>'; return; }
    const json = await res.json();
    if (seq !== _discLoadSeq) return;
    const jobs = json.data ?? [];
    const meta = json.meta ?? {};
    document.getElementById('disc-skills-note').classList.toggle('hidden', json.only_strong_skills !== false);
    renderDiscFacets(json.facets || {});
    renderDiscLevels((json.facets || {}).levels || []);
    renderDiscoveryPagination(meta.total ?? 0, meta.cursor ?? _discPg.offset, meta.size ?? _discPg.size);
    renderDiscDismissedToggle(json.dismissed_count ?? 0);
    updateDiscFoldSummaries();

    if (!jobs.length && _discShowDismissed) {
      tbody.innerHTML = '<p class=\"jl-empty\">Nenhuma vaga dispensada.</p>';
      return;
    }
    if (!jobs.length) {
      const filtered = _discFilters.sources.length || _discFilters.skills.length || _discLevels.length;
      tbody.innerHTML = filtered
        ? '<p class=\"jl-empty\">Nenhuma vaga com esses filtros.</p>'
        : '<p class=\"jl-empty\">Nenhuma vaga do catálogo cita suas skills no seu local de interesse. Confira o Perfil, o local ou adicione uma área acima.</p>';
      return;
    }
    tbody.innerHTML = jobs.map(renderDiscoveryRow).join('');
  } catch (e) {
    tbody.innerHTML = `<p class="jl-empty">Erro: ${escDisc(e.message)}</p>`;
  }
}

// Linha compacta: compatibilidade, título e nível, empresa · site · local · data,
// a stack em chips e as ações. A descrição e o detalhe da compatibilidade ficam na
// página "Sobre a vaga" (Ver mais / olhinho, ou clique no título).
function renderDiscoveryRow(j) {
  const url = safeHttpUrl(j.url);
  const local = [j.international ? 'Exterior' : '', _discWorkplace[j.workplace_type] || '', j.location].filter(Boolean).join(' · ');
  // Skills do perfil primeiro; depois as tecnologias de interesse que não são skill.
  const techs = (j.matched_techs || []).filter(t => !j.matched_skills.some(s => s.toLowerCase() === t.toLowerCase()));
  // Depois, o resto da stack que a vaga cita, apagado: "C#, Java, Go ou Rust" com Go no perfil.
  const stack = j.stack || [];
  const strong = [...j.matched_skills.map(s => [s, '']), ...techs.map(t => [t, ' match-tag-tech'])];
  const room = Math.max(0, 6 - strong.length);
  const shown = stack.slice(0, room);
  const rest = stack.length - shown.length;
  const skills = strong.map(([t, cls]) => `<span class="match-tag${cls}">${escDisc(t)}</span>`).join('') +
    shown.map(t => `<span class="match-tag match-tag-other" title="Também citada na vaga">${escDisc(t)}</span>`).join('') +
    (rest > 0 ? `<span class="match-tag match-tag-other" title="${escDisc(stack.slice(room).join(', '))}">+${rest}</span>` : '');
  // Nível tirado do título (ou do campo de nível da fonte): é uma estimativa.
  const levels = (j.seniority || []).map(l => _discLevelNames[l]).filter(Boolean);
  const level = levels.length
    ? `<span class="disc-level" title="Nível estimado pelo título da vaga">${escDisc(levels.join(' / '))}</span>`
    : '';
  const fit = j.fit && (j.fit.items || []).length ? j.fit : null;
  const pct = fit
    ? `<span class="jl-pct q-${escDisc(fit.quality)}" title="${escDisc(discFitTitle(fit))}">${fit.coverage_pct}%</span>`
    : '<span class="jl-pct" title="Nenhuma tecnologia conhecida no texto da vaga">—</span>';
  const action = j.pipeline_job_id
    ? discPipeline(j.pipeline_job_id, j.pipeline_status)
    : discGenerateButton(url, j.language);
  const dismiss = j.dismissed
    ? `<button type="button" class="btn btn-ghost btn-sm" onclick="restoreDiscJob('${escDisc(j.id)}', this)">Restaurar</button>`
    : `<button type="button" class="btn-delete disc-dismiss" onclick="dismissDiscJob('${escDisc(j.id)}', this)" title="Dispensar: some da lista e dos alertas só para você (dá para restaurar em Dispensadas)" aria-label="Dispensar vaga">✕</button>`;
  const sub = [j.company, _discSourceNames[j.source] || j.source, local, j.published_at ? fmtDay(j.published_at) : ''].filter(Boolean).map(escDisc).join(' · ');

  return `
    <div class="jl-row disc-row" data-id="${escDisc(j.id)}">
      ${pct}
      <div class="jl-main">
        <div class="jl-title"><a class="disc-title" href="#/descobrir/${escDisc(j.id)}" onclick="event.preventDefault(); openDiscJob('${escDisc(j.id)}')">${escDisc(j.title)}</a>${level}</div>
        <div class="jl-sub">${sub}</div>
        ${skills ? `<div class="disc-skills">${skills}</div>` : ''}
      </div>
      <div class="jl-actions disc-actions">
        ${action}
        <button type="button" class="btn btn-ghost btn-sm disc-more" onclick="openDiscJob('${escDisc(j.id)}')" title="Ver mais: compatibilidade e descrição da vaga" aria-label="Ver mais">
          <svg class="ico"><use href="#i-eye"/></svg><span>Ver mais</span>
        </button>
        ${url ? `<a class="btn btn-ghost btn-sm disc-open" href="${escDisc(url)}" target="_blank" rel="noopener noreferrer" title="Abrir a vaga no site" aria-label="Abrir a vaga no site">↗</a>` : ''}
        ${dismiss}
      </div>
    </div>`;
}

// Dica do % na lista: quantas tecnologias você cobre e o que falta.
function discFitTitle(fit) {
  const missing = fit.items.filter(i => i.status === 'missing').map(i => i.name);
  return `Cobertura da stack: você cobre ${fit.strong + fit.partial} de ${fit.items.length}` +
    (missing.length ? ` · falta: ${missing.join(', ')}` : ' · cobre toda a stack');
}

// A mesma vaga (empresa e título iguais) em outros sites: a lista mostra uma só, com
// os links das outras.
function discAlsoOn(links) {
  const ok = (links || []).filter(l => safeHttpUrl(l.url));
  if (!ok.length) return '';
  return `<div class="disc-also">também em: ${ok.map(l =>
    `<a class="link" href="${escDisc(safeHttpUrl(l.url))}" target="_blank" rel="noopener noreferrer">${escDisc(_discSourceNames[l.source] || l.source)}</a>`).join(', ')}</div>`;
}

function renderDiscDismissedToggle(n) {
  const b = document.getElementById('disc-dismissed-toggle');
  if (!b) return;
  b.classList.toggle('hidden', !n && !_discShowDismissed);
  b.textContent = _discShowDismissed ? '← Voltar à lista' : `Dispensadas (${n})`;
}

function toggleDiscDismissed() {
  _discShowDismissed = !_discShowDismissed;
  _discPg.offset = 0;
  loadDiscoveryJobs();
}

// Dispensar some com a vaga da lista e dos alertas só para você; ela continua no
// catálogo e volta em "Dispensadas".
async function dismissDiscJob(id, btn) {
  btn.disabled = true;
  try {
    const res = await api.post(`/discovery/jobs/${encodeURIComponent(id)}/dismiss`, {});
    if (!res.ok) { btn.disabled = false; toast('Não foi possível dispensar.', 'error'); return; }
    const row = btn.closest('.jl-row');
    if (row) row.remove();
    const b = document.getElementById('disc-dismissed-toggle');
    const n = (parseInt((b?.textContent.match(/\d+/) || ['0'])[0], 10) || 0) + 1;
    renderDiscDismissedToggle(n);
    toast('Vaga dispensada. Ela fica em "Dispensadas".');
    if (typeof loadAlerts === 'function') loadAlerts();
  } catch (e) {
    btn.disabled = false;
    toast(e.message || 'Erro de conexão.', 'error');
  }
}

async function restoreDiscJob(id, btn) {
  btn.disabled = true;
  try {
    const res = await api.delete(`/discovery/jobs/${encodeURIComponent(id)}/dismiss`);
    if (!res.ok) { btn.disabled = false; toast('Não foi possível restaurar.', 'error'); return; }
    toast('Vaga restaurada.');
    loadDiscoveryJobs();
  } catch (e) {
    btn.disabled = false;
    toast(e.message || 'Erro de conexão.', 'error');
  }
}

// "Gerar currículo" abre a escolha de idioma (sugerido pelo texto da vaga) e do que
// gerar; a vaga vai para a pipeline normal. O worker lê a página da vaga; quando o site
// não tem leitor (GitHub, Himalayas) ou bloqueia, usa a descrição já gravada no catálogo.
function discGenerateButton(url, lang) {
  return `<button class="btn btn-primary btn-sm" onclick="openDiscGenerate(this)" data-url="${escDisc(url)}" data-lang="${lang === 'en' ? 'en' : 'pt-br'}">Gerar currículo</button>`;
}

function openDiscGenerate(btn) {
  const url = btn.dataset.url;
  if (!url) return;
  const wrap = document.createElement('span');
  wrap.className = 'disc-gen';
  wrap.dataset.url = url;
  wrap.dataset.lang = btn.dataset.lang;
  wrap.innerHTML = genChooser({ lang: btn.dataset.lang, onGo: 'addDiscoveredJob', onCancel: 'cancelDiscGenerate(this)' });
  btn.replaceWith(wrap);
}

function cancelDiscGenerate(btn) {
  const wrap = btn.closest('.disc-gen');
  wrap.outerHTML = discGenerateButton(wrap.dataset.url, wrap.dataset.lang);
}

async function addDiscoveredJob(mode, btn) {
  const wrap = btn.closest('.disc-gen');
  const url = wrap?.dataset.url;
  if (!url) return;
  const language = genLanguage(btn);
  const controls = wrap.querySelectorAll('button, select');
  controls.forEach(b => b.disabled = true);
  try {
    const res = await api.post('/jobs', { url, mode, language });
    if (res.status === 409) {
      toast('Esta vaga já está na sua lista de Vagas.', 'error');
      if (_activeTab === 'sobre') loadDiscJob(); else loadDiscoveryJobs(); // trazem o andamento dela
      return;
    }
    if (!res.ok) { toast('Erro ao cadastrar vaga.', 'error'); controls.forEach(b => b.disabled = false); return; }
    const { id } = (await apiData(res)) || {};
    toast(`Vaga enviada (${language === 'en' ? 'inglês' : 'português'})! Acompanhe o andamento aqui mesmo.`);
    wrap.outerHTML = discPipeline(id, 'pending');
  } catch (e) {
    toast(e.message, 'error');
    controls.forEach(b => b.disabled = false);
  }
}

// ── Andamento na pipeline ──
// Depois de "Gerar currículo", a linha acompanha a vaga em Vagas: fila, processamento
// e, ao concluir, "Ver currículo" abre a tela da vaga. O SSE das vagas
// (jobs.js) chama discOnJobEvent a cada mudança de status.

const _discInProgress = { pending: 'Na fila', processing: 'Processando', scraping_basic: 'Lendo a vaga', scraping_nl: 'Analisando a vaga' };

function discPipelineInner(jobId, status) {
  if (status === 'completed') {
    return `<button class="btn btn-sm disc-done" onclick="openJobDetail('${jobId}', { from: 'descobrir', focus: 'resumes' })">✓ Ver currículo</button>`;
  }
  if (status === 'error') {
    return `<button class="btn btn-sm disc-failed" onclick="navigateTo('vagas')" title="Veja o motivo e use Refazer em Vagas">Erro · ver em Vagas</button>`;
  }
  return `<span class="disc-progress"><span class="disc-spinner" aria-hidden="true"></span>${_discInProgress[status] || 'Gerando'}…</span>`;
}

function discPipeline(jobId, status) {
  if (!/^[0-9a-f-]{36}$/i.test(jobId || '')) return '<span class="badge badge-completed">Em Vagas</span>';
  return `<span class="disc-pipeline" data-job-id="${jobId}">${discPipelineInner(jobId, status)}</span>`;
}

function discOnJobEvent(ev) {
  document.querySelectorAll(`.disc-pipeline[data-job-id="${ev.id}"]`).forEach(el => {
    el.innerHTML = discPipelineInner(ev.id, ev.status);
  });
}
