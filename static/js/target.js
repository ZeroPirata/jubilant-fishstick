// ── Alvo do Descobrir ──
// O que a pessoa procura numa frase: função, tecnologias, nível e modo de trabalho.
// Sem formulário: o sistema sugere pelo perfil e entende texto livre ("golang backend
// remoto pleno"). Vale para a lista, os avisos, a Seleção com IA e as buscas do
// LinkedIn. O servidor guarda nas preferências do Descobrir (internal/handler/discovery_target.go).

let _target = null;           // { roles, technologies, seniorities, workplace_types }
let _targetSuggestion = null;
let _targetRoleLabels = {};   // chave → nome ("backend" → "Back-end")
const _targetWorkplace = { remote: 'Remoto', hybrid: 'Híbrido', onsite: 'Presencial', abroad: 'Exterior' };

async function loadTarget() {
  try {
    const res = await api.get('/discovery/target');
    if (!res.ok) return;
    const d = await apiData(res);
    _target = d.target;
    _targetSuggestion = d.empty ? d.suggestion : null;
    _targetRoleLabels = Object.fromEntries((d.roles || []).map(r => [r.key, r.label]));
  } catch (_) { return; }
  renderTarget();
}

function targetIsEmpty(t) {
  return !t || !(t.roles.length || t.technologies.length || t.seniorities.length || t.workplace_types.length);
}

// targetParts: [tipo, valor, nome] de cada item do Alvo, na ordem da frase.
function targetParts(t) {
  return [
    ...t.roles.map(v => ['roles', v, _targetRoleLabels[v] || v]),
    ...t.technologies.map(v => ['technologies', v, v]),
    ...t.seniorities.map(v => ['seniorities', v, _discLevelNames[v] || v]),
    ...t.workplace_types.map(v => ['workplace_types', v, _targetWorkplace[v] || v]),
  ];
}

function targetSentence(t) {
  const group = (kind) => targetParts(t).filter(p => p[0] === kind).map(p => p[2]).join(', ');
  return ['roles', 'technologies', 'seniorities', 'workplace_types'].map(group).filter(Boolean).join(' · ');
}

function renderTarget() {
  const box = document.getElementById('target-chips');
  if (!box || !_target) return;
  const empty = targetIsEmpty(_target);
  box.innerHTML = empty
    ? '<span class="target-empty">Nenhum alvo ainda: a lista mostra tudo que cita as suas skills.</span>'
    : targetParts(_target).map(([kind, v, label]) => `
        <span class="target-chip target-chip-${kind}">${escDisc(label)}<button type="button" onclick="removeTargetItem('${kind}', '${escDisc(v)}')" aria-label="Tirar ${escDisc(label)} do alvo" title="Tirar do alvo">×</button></span>`).join('');
  document.getElementById('target-clear').classList.toggle('hidden', empty);

  const sug = document.getElementById('target-suggest');
  const sugText = _targetSuggestion && !targetIsEmpty(_targetSuggestion) ? targetSentence(_targetSuggestion) : '';
  sug.classList.toggle('hidden', !sugText);
  if (sugText) document.getElementById('target-suggest-text').textContent = sugText;
}

// Aplica um texto livre: o que ele cita substitui aquela parte do Alvo; o que não cita
// fica como estava ("golang backend" não apaga o nível já escolhido).
async function applyTargetText() {
  const input = document.getElementById('target-text');
  const note = document.getElementById('target-note');
  const text = input.value.trim();
  if (!text) return;
  try {
    const res = await api.post('/discovery/target/parse', { text });
    const body = await res.json();
    if (!res.ok) throw new Error(body.error || 'status ' + res.status);
    const p = body.data;
    const next = { ..._target };
    let understood = false;
    for (const kind of ['roles', 'technologies', 'seniorities', 'workplace_types']) {
      if (p[kind].length) { next[kind] = p[kind]; understood = true; }
    }
    note.textContent = p.unknown.length
      ? `Não entendi: ${p.unknown.join(', ')}. ${p.unknown.length > 1 ? 'Essas palavras ficaram' : 'Essa palavra ficou'} de fora.`
      : '';
    if (!understood) return;
    if (await saveTarget(next)) input.value = '';
  } catch (e) {
    toast(e.message, 'error');
  }
}

async function useTargetSuggestion() {
  if (_targetSuggestion) await saveTarget(_targetSuggestion);
}

async function removeTargetItem(kind, value) {
  const next = { ..._target, [kind]: _target[kind].filter(v => v !== value) };
  await saveTarget(next);
}

async function clearTarget() {
  if (!confirm('Limpar o alvo? A lista volta a mostrar tudo que cita as suas skills.')) return;
  await saveTarget({ roles: [], technologies: [], seniorities: [], workplace_types: [] });
}

async function saveTarget(t) {
  try {
    const res = await api.put('/discovery/target', t);
    const body = await res.json();
    if (!res.ok) throw new Error(body.error || 'status ' + res.status);
    _target = { roles: body.data.roles, technologies: body.data.technologies,
                seniorities: body.data.seniorities, workplace_types: body.data.workplace_types };
    _targetSuggestion = null;
    renderTarget();
    // A tela antiga de preferências usa as mesmas colunas: recarrega para ficar igual.
    _discPg.offset = 0;
    _discAllRoles = false;
    loadDiscoveryPreferences();
    loadDiscoveryJobs();
    return true;
  } catch (e) {
    toast(e.message, 'error');
    return false;
  }
}

// Gera as buscas do LinkedIn pelo Alvo e abre o bloco com elas.
async function generateLinkedInFromTarget() {
  try {
    const res = await api.post('/linkedin/searches/generate', {});
    const body = await res.json();
    if (!res.ok) throw new Error(body.error || 'status ' + res.status);
    _liSearches = body.data || [];
    renderLinkedInSearches();
    const box = document.getElementById('li-box');
    box.open = true;
    box.scrollIntoView({ behavior: 'smooth', block: 'start' });
    toast('Buscas do LinkedIn geradas pelo seu alvo.');
  } catch (e) {
    toast(e.message, 'error');
  }
}

// discTargetBadges: por que a vaga está no topo ("Back-end", "Go no título").
function discTargetBadges(j) {
  const out = [];
  if (j.role_fit === 'match' || j.role_fit === 'neighbor') {
    const names = (j.roles || []).map(r => _targetRoleLabels[r] || r).join(' / ');
    out.push(`<span class="target-badge${j.role_fit === 'neighbor' ? ' target-badge-near' : ''}" title="${j.role_fit === 'match' ? 'Função do seu alvo' : 'Função próxima da do seu alvo'}">${escDisc(names)}</span>`);
  }
  if ((j.title_techs || []).length) {
    out.push(`<span class="target-badge" title="Tecnologia do seu alvo no título da vaga">${escDisc(j.title_techs.join(', '))} no título</span>`);
  }
  return out.join('');
}
