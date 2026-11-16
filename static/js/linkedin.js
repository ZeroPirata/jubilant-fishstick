// ── Buscas no LinkedIn (aba Descobrir) ──
// O usuário salva buscas com os filtros que quiser e abre o link no próprio navegador,
// logado. O servidor só monta a URL (internal/linkedinsearch): não acessa o LinkedIn.
// Nenhum filtro vem marcado numa busca nova.

const LI_OPTIONS = {
  posted_within: [['', 'Qualquer data'], ['r86400', 'Últimas 24 h'], ['r604800', 'Última semana'], ['r2592000', 'Último mês']],
  workplace:     [['1', 'Presencial'], ['2', 'Remoto'], ['3', 'Híbrido']],
  experience:    [['1', 'Estágio'], ['2', 'Assistente'], ['3', 'Júnior'], ['4', 'Pleno-sênior'], ['5', 'Diretor'], ['6', 'Executivo']],
  job_types:     [['F', 'Tempo integral'], ['P', 'Meio período'], ['C', 'Contrato'], ['T', 'Temporário'], ['I', 'Estágio'], ['V', 'Voluntário'], ['O', 'Outro']],
  flags:         [['easy_apply', 'Candidatura simplificada'], ['few_applicants', 'Menos de 10 candidaturas'], ['in_network', 'Na minha rede']],
  sort:          [['', 'Padrão do LinkedIn'], ['DD', 'Mais recentes'], ['R', 'Mais relevantes']],
};
const LI_GEO_BRASIL = '106057199';

let _liSearches = [];
let _liEdit = null;        // busca em edição (null = formulário fechado)
let _liPreviewTimer = null;

function liEmpty() {
  return { id: '', name: '', keywords: '', geo_id: '', geo_label: '', posted_within: '', workplace: [], experience: [],
           job_types: [], easy_apply: false, few_applicants: false, in_network: false, sort: '' };
}

async function loadLinkedInSearches() {
  const box = document.getElementById('li-list');
  try {
    const res = await api.get('/linkedin/searches');
    if (!res.ok) throw new Error('status ' + res.status);
    _liSearches = (await apiData(res)) || [];
  } catch (e) {
    box.innerHTML = `<p class="li-empty">Erro ao carregar as buscas: ${escHtml(e.message)}</p>`;
    return;
  }
  renderLinkedInSearches();
}

// liSummary: os filtros da busca em uma linha ("go · Brasil · Remoto, Híbrido · semana").
function liSummary(s) {
  const label = (kind, v) => (LI_OPTIONS[kind].find(o => o[0] === v) || [, v])[1];
  const parts = [
    s.keywords ? `“${s.keywords}”` : 'sem palavras-chave',
    s.geo_id ? (s.geo_label || `local ${s.geo_id}`) : '',
    s.posted_within ? label('posted_within', s.posted_within) : '',
    s.workplace.map(v => label('workplace', v)).join(', '),
    s.experience.map(v => label('experience', v)).join(', '),
    s.job_types.map(v => label('job_types', v)).join(', '),
    ...LI_OPTIONS.flags.filter(([k]) => s[k]).map(([, l]) => l),
    s.sort ? label('sort', s.sort) : '',
  ];
  return parts.filter(Boolean).join(' · ');
}

function renderLinkedInSearches() {
  const box = document.getElementById('li-list');
  const sum = document.getElementById('li-sum');
  if (sum) sum.textContent = _liSearches.length ? `${_liSearches.length} salva${_liSearches.length > 1 ? 's' : ''}` : 'monte o link da busca com os seus filtros';
  if (!_liSearches.length) {
    box.innerHTML = '<p class="li-empty">Nenhuma busca salva ainda.</p>';
    return;
  }
  box.innerHTML = _liSearches.map((s, i) => `
    <div class="li-item">
      <div class="li-item-main">
        <div class="li-item-name">${escHtml(s.name)}</div>
        <div class="li-item-sum">${escHtml(liSummary(s))}</div>
      </div>
      <div class="li-item-actions">
        <a class="btn btn-primary btn-sm" href="${escHtml(s.url)}" target="_blank" rel="noopener noreferrer">Abrir no LinkedIn ↗</a>
        <button class="btn btn-ghost btn-sm" onclick="liEdit(${i})">Editar</button>
        <button class="btn btn-ghost btn-sm" onclick="liDelete(${i})">Excluir</button>
      </div>
    </div>`).join('');
}

function liNew() {
  _liEdit = liEmpty();
  liOpenForm();
}

function liEdit(i) {
  const s = _liSearches[i];
  if (!s) return;
  _liEdit = JSON.parse(JSON.stringify(s));
  liOpenForm();
}

function liOpenForm() {
  const form = document.getElementById('li-form');
  form.classList.remove('hidden');
  document.getElementById('li-form-title').textContent = _liEdit.id ? 'Editar busca' : 'Nova busca';
  document.getElementById('li-paste').value = '';
  document.getElementById('li-name').value = _liEdit.name;
  document.getElementById('li-keywords').value = _liEdit.keywords;
  document.getElementById('li-geo-label').value = _liEdit.geo_label;
  renderLinkedInForm();
  form.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
  document.getElementById('li-name').focus();
}

function liCloseForm() {
  _liEdit = null;
  document.getElementById('li-form').classList.add('hidden');
}

// Chips: kind de valor único (posted_within, sort) troca; os outros marcam e desmarcam.
function liChips(kind) {
  const single = kind === 'posted_within' || kind === 'sort';
  return LI_OPTIONS[kind].map(([v, label]) => {
    const on = kind === 'flags' ? _liEdit[v] : single ? _liEdit[kind] === v : _liEdit[kind].includes(v);
    return `<button type="button" class="disc-chip ${on ? 'on' : ''}" aria-pressed="${on}" onclick="liToggle('${kind}','${v}')">${escHtml(label)}</button>`;
  }).join('');
}

function renderLinkedInForm() {
  ['posted_within', 'workplace', 'experience', 'job_types', 'flags', 'sort'].forEach(kind => {
    document.getElementById('li-chips-' + kind).innerHTML = liChips(kind);
  });
  const geo = document.getElementById('li-geo-code');
  geo.textContent = _liEdit.geo_id ? `código ${_liEdit.geo_id}` : 'sem local (o LinkedIn usa o do seu perfil)';
  document.getElementById('li-geo-label').disabled = !_liEdit.geo_id;
  document.getElementById('li-geo-clear').classList.toggle('hidden', !_liEdit.geo_id);
  liSchedulePreview();
}

function liToggle(kind, v) {
  if (kind === 'flags') _liEdit[v] = !_liEdit[v];
  else if (kind === 'posted_within' || kind === 'sort') _liEdit[kind] = v;
  else {
    const list = _liEdit[kind];
    const at = list.indexOf(v);
    if (at >= 0) list.splice(at, 1); else list.push(v);
  }
  renderLinkedInForm();
}

function liInput() {
  _liEdit.name = document.getElementById('li-name').value;
  _liEdit.keywords = document.getElementById('li-keywords').value;
  _liEdit.geo_label = document.getElementById('li-geo-label').value;
  liSchedulePreview();
}

function liSetBrasil() {
  _liEdit.geo_id = LI_GEO_BRASIL;
  _liEdit.geo_label = 'Brasil';
  document.getElementById('li-geo-label').value = 'Brasil';
  renderLinkedInForm();
}

function liClearGeo() {
  _liEdit.geo_id = '';
  _liEdit.geo_label = '';
  document.getElementById('li-geo-label').value = '';
  renderLinkedInForm();
}

// Link ao vivo: o servidor monta (a mesma regra da busca salva).
function liSchedulePreview() {
  clearTimeout(_liPreviewTimer);
  _liPreviewTimer = setTimeout(liPreview, 250);
}

async function liPreview() {
  if (!_liEdit) return;
  const link = document.getElementById('li-preview');
  const err = document.getElementById('li-preview-error');
  try {
    const res = await api.post('/linkedin/searches/preview', _liEdit);
    const body = await res.json();
    if (!res.ok) throw new Error(body.error || 'status ' + res.status);
    link.href = body.data.url;
    link.textContent = body.data.url;
    err.textContent = '';
  } catch (e) {
    err.textContent = e.message;
  }
}

// Colar uma URL de busca do LinkedIn: preenche palavras-chave, local e filtros.
async function liReadURL() {
  const input = document.getElementById('li-paste');
  const url = input.value.trim();
  if (!url) return;
  try {
    const res = await api.post('/linkedin/searches/from-url', { url });
    const body = await res.json();
    if (!res.ok) throw new Error(body.error || 'status ' + res.status);
    const name = _liEdit.name;
    _liEdit = Object.assign(liEmpty(), body.data, { id: _liEdit.id, name });
    delete _liEdit.url;
    document.getElementById('li-keywords').value = _liEdit.keywords;
    document.getElementById('li-geo-label').value = _liEdit.geo_label;
    if (!name && _liEdit.keywords) {
      _liEdit.name = _liEdit.keywords.slice(0, 80);
      document.getElementById('li-name').value = _liEdit.name;
    }
    input.value = '';
    renderLinkedInForm();
    toast(_liEdit.geo_id && !_liEdit.geo_label ? 'Filtros lidos. Dê um nome ao local (ex: São José dos Campos).' : 'Filtros lidos da URL.');
    if (_liEdit.geo_id && !_liEdit.geo_label) document.getElementById('li-geo-label').focus();
  } catch (e) {
    toast(e.message, 'error');
  }
}

async function liSave() {
  liInput();
  const editing = _liEdit.id;
  try {
    const res = editing
      ? await api.put(`/linkedin/searches/${editing}`, _liEdit)
      : await api.post('/linkedin/searches', _liEdit);
    const body = await res.json();
    if (!res.ok) throw new Error(body.error || 'status ' + res.status);
    toast(editing ? 'Busca atualizada.' : 'Busca salva.');
    liCloseForm();
    loadLinkedInSearches();
  } catch (e) {
    toast(e.message, 'error');
  }
}

async function liDelete(i) {
  const s = _liSearches[i];
  if (!s || !confirm(`Excluir a busca "${s.name}"?`)) return;
  try {
    const res = await api.delete(`/linkedin/searches/${s.id}`);
    if (!res.ok && res.status !== 404) throw new Error('status ' + res.status);
    if (_liEdit && _liEdit.id === s.id) liCloseForm();
    loadLinkedInSearches();
  } catch (e) {
    toast('Erro ao excluir: ' + e.message, 'error');
  }
}
