// ── Painel admin: configuração da busca de vagas ──
// Liga/desliga, intervalo, sites, empresas dos ATS e termos do catálogo, mais o
// "executar agora" com os sites escolhidos.
// O andamento e o histórico ficam em Métricas → Busca de vagas (discovery_metrics.js).

const DADM_PRESETS = [60, 360, 720, 1440];

const DADM_KINDS = { search: 'busca por termo', list: 'lista completa' };

// boards: empresas por ATS sendo editadas; checks: resultado da conferência por
// "site/token" (undefined = não conferida, 'checking' = em andamento).
let _dadm = { dirty: false, sources: [], boards: {}, checks: {}, found: [] };
const dadmBoardKey = (source, token) => `${source}/${token}`;

const dadmSourceName = n => (typeof _discSourceNames !== 'undefined' && _discSourceNames[n]) || n;

function goToDiscoveryMetrics() {
  navigateTo('metricas');
  switchMetricsView('busca', document.querySelector('[data-mview="busca"]'));
}

async function loadDiscoveryAdmin() {
  try {
    const res = await api.get('/admin/discovery/settings');
    if (!res.ok) { toast('Erro ao carregar a configuração da busca.', 'error'); return; }
    const d = await apiData(res);
    renderDadmForm(d);
  } catch (e) { toast(e.message, 'error'); }
}

function renderDadmForm(d) {
  const st = d.settings;
  document.getElementById('dadm-enabled').checked = st.enabled;

  const preset = document.getElementById('dadm-interval-preset');
  const custom = !DADM_PRESETS.includes(st.interval_minutes);
  preset.value = custom ? 'custom' : String(st.interval_minutes);
  document.getElementById('dadm-custom').classList.toggle('hidden', !custom);
  if (custom) {
    const unit = st.interval_minutes % 1440 === 0 ? 1440 : st.interval_minutes % 60 === 0 ? 60 : 1;
    document.getElementById('dadm-custom-unit').value = String(unit);
    document.getElementById('dadm-custom-value').value = st.interval_minutes / unit;
  }

  _dadm.sources = d.sources;
  const kind = s => `<span class="dadm-kind">${DADM_KINDS[s.kind] || escDisc(s.kind)}</span>`;
  document.getElementById('dadm-sources').innerHTML = d.sources.map(s => `
    <label class="field-check"><span class="ai-switch"><input type="checkbox" data-source="${escDisc(s.name)}" ${s.enabled ? 'checked' : ''} onchange="dadmDirty()" /><span></span></span> ${escDisc(dadmSourceName(s.name))} ${kind(s)}</label>`
  ).join('') || '<span class="card-note" style="margin:0">Nenhum site registrado.</span>';

  // "Executar agora" começa com os sites ligados marcados.
  document.getElementById('dadm-run-sources').innerHTML = d.sources.map(s => `
    <label class="field-check"><input type="checkbox" data-run-source="${escDisc(s.name)}" ${s.enabled ? 'checked' : ''} /> ${escDisc(dadmSourceName(s.name))} ${kind(s)}</label>`
  ).join('');

  const boardSources = d.sources.filter(s => s.uses_boards);
  document.getElementById('dadm-boards-card').classList.toggle('hidden', !boardSources.length);
  _dadm.boards = {};
  for (const s of boardSources) _dadm.boards[s.name] = ((st.boards && st.boards[s.name]) || []).map(b => ({ ...b }));
  _dadm.found = [];
  _dadm.foundQuery = '';
  renderDadmBoards();
  renderDadmFound();

  document.getElementById('dadm-terms').value = st.catalog_terms.join('\n');
  document.getElementById('dadm-updated').textContent = d.updated_at ? `Última alteração: ${fmtDate(d.updated_at)}` : '';
  dadmTermsCount();
  dadmSetDirty(false);
}

function dadmPresetChanged() {
  const custom = document.getElementById('dadm-interval-preset').value === 'custom';
  document.getElementById('dadm-custom').classList.toggle('hidden', !custom);
  if (custom && !document.getElementById('dadm-custom-value').value) {
    document.getElementById('dadm-custom-value').value = 12;
    document.getElementById('dadm-custom-unit').value = '60';
  }
  dadmDirty();
}

function dadmIntervalMinutes() {
  const preset = document.getElementById('dadm-interval-preset').value;
  if (preset !== 'custom') return parseInt(preset, 10);
  const value = parseFloat(document.getElementById('dadm-custom-value').value);
  const unit  = parseInt(document.getElementById('dadm-custom-unit').value, 10);
  return Math.round(value * unit);
}

function dadmDisabledSources() {
  return [...document.querySelectorAll('#dadm-sources input[data-source]')]
    .filter(cb => !cb.checked).map(cb => cb.dataset.source);
}

function dadmTerms() {
  return document.getElementById('dadm-terms').value.split('\n').map(t => t.trim()).filter(Boolean);
}

function dadmTermsCount() {
  const n = dadmTerms().length;
  document.getElementById('dadm-terms-count').textContent = `${n} termo${n !== 1 ? 's' : ''}`;
}

function dadmBoards() { return _dadm.boards; }

// ── Empresas nos ATS ──

function dadmBoardStatus(source, token) {
  const c = _dadm.checks[dadmBoardKey(source, token)];
  if (c === undefined) return '<span style="color:var(--muted)">não conferida</span>';
  if (c === 'checking') return '<span style="color:var(--muted)">conferindo…</span>';
  if (c.error) return `<span class="dadm-err" title="${escDisc(c.error)}">erro ao conferir</span>`;
  if (!c.found) return '<span class="badge badge-error">não encontrada</span> <span class="disc-status">token errado ou saiu do ATS</span>';
  if (!c.jobs_kept) return `<span class="badge badge-mid">0 servem</span> <span class="disc-status">de ${c.jobs} vagas, nenhuma no Brasil</span>`;
  return `<span class="badge badge-completed">${c.jobs_kept} servem</span> <span class="disc-status">de ${c.jobs} vagas</span>`;
}

function renderDadmBoards() {
  const rows = Object.entries(_dadm.boards).flatMap(([source, list]) => list.map((b, i) => ({ source, i, ...b })))
    .sort((a, b) => a.company.localeCompare(b.company, 'pt-BR'));
  const total = rows.length;
  const checked = rows.map(r => _dadm.checks[dadmBoardKey(r.source, r.token)]).filter(c => c && c !== 'checking');
  const bad = checked.filter(c => c.error || !c.found || !c.jobs_kept).length;
  document.getElementById('dadm-boards-count').textContent =
    `${total} empresa${total !== 1 ? 's' : ''}` + (checked.length ? ` · ${checked.length} conferida${checked.length !== 1 ? 's' : ''}${bad ? `, ${bad} com problema` : ''}` : '');

  const tbody = document.getElementById('dadm-boards');
  if (!rows.length) { tbody.innerHTML = '<tr><td colspan="5" class="empty">Nenhuma empresa. Procure uma acima.</td></tr>'; return; }
  tbody.innerHTML = rows.map(r => `
    <tr>
      <td data-label="Empresa"><input type="text" class="dadm-board-name" value="${escDisc(r.company)}" maxlength="80"
            aria-label="Nome da empresa" onchange="dadmRenameBoard('${escDisc(r.source)}', '${escDisc(r.token)}', this.value)" /></td>
      <td data-label="Site">${escDisc(dadmSourceName(r.source))}</td>
      <td data-label="Token"><code>${escDisc(r.token)}</code></td>
      <td data-label="Situação">${dadmBoardStatus(r.source, r.token)}</td>
      <td><button class="disc-keyword-x" title="Remover" aria-label="Remover ${escDisc(r.company)}"
            onclick="dadmRemoveBoard('${escDisc(r.source)}', '${escDisc(r.token)}')">✕</button></td>
    </tr>`).join('');
}

function dadmHasBoard(source, token) {
  return (_dadm.boards[source] || []).some(b => b.token === token);
}

function dadmRenameBoard(source, token, name) {
  const b = (_dadm.boards[source] || []).find(b => b.token === token);
  if (!b) return;
  b.company = name.trim() || token;
  dadmDirty();
}

function dadmRemoveBoard(source, token) {
  _dadm.boards[source] = (_dadm.boards[source] || []).filter(b => b.token !== token);
  renderDadmBoards();
  renderDadmFound();
  dadmDirty();
}

async function dadmCheckBoard(source, token) {
  const key = dadmBoardKey(source, token);
  _dadm.checks[key] = 'checking';
  renderDadmBoards();
  try {
    const res = await api.post('/admin/discovery/boards/check', { source, token });
    const d = res.ok ? await apiData(res) : null;
    _dadm.checks[key] = d && d.results[0] ? d.results[0] : { error: 'falha ao conferir' };
  } catch (e) {
    _dadm.checks[key] = { error: e.message };
  }
  renderDadmBoards();
}

// Confere todas, 4 por vez, para não abrir dezenas de pedidos de uma vez.
async function dadmCheckAllBoards() {
  const btn = document.getElementById('dadm-board-checkall');
  btn.disabled = true;
  const queue = Object.entries(_dadm.boards).flatMap(([source, list]) => list.map(b => [source, b.token]));
  const worker = async () => { while (queue.length) await dadmCheckBoard(...queue.shift()); };
  try { await Promise.all([worker(), worker(), worker(), worker()]); }
  finally { btn.disabled = false; }
}

async function dadmFindBoards() {
  const input = document.getElementById('dadm-board-query');
  const query = input.value.trim();
  if (!query) { toast('Digite o nome da empresa ou o link do quadro de vagas.', 'error'); return; }
  const btn = document.getElementById('dadm-board-find');
  btn.disabled = true;
  _dadm.found = 'searching';
  renderDadmFound();
  try {
    const res = await api.post('/admin/discovery/boards/check', { query });
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      toast(err.error || 'Erro ao procurar.', 'error');
      _dadm.found = [];
    } else {
      const d = await apiData(res);
      // Nome sugerido: o que foi digitado; com link, o que o ATS informa (às vezes
      // estranho, como "Stone - Linkedin": dá para editar antes de adicionar).
      const typed = /[/.]/.test(query) ? '' : query;
      _dadm.found = d.results.map(r => ({ ...r, suggested: typed || r.company || r.token }));
      _dadm.foundQuery = query;
      for (const r of d.results) _dadm.checks[dadmBoardKey(r.source, r.token)] = r;
    }
  } catch (e) {
    toast(e.message, 'error');
    _dadm.found = [];
  } finally {
    btn.disabled = false;
  }
  renderDadmFound();
}

function renderDadmFound() {
  const box = document.getElementById('dadm-board-found');
  if (_dadm.found === 'searching') { box.innerHTML = '<p class="disc-status">Procurando no Greenhouse e na Lever…</p>'; return; }
  if (!_dadm.foundQuery) { box.innerHTML = ''; return; }
  if (!_dadm.found.length) {
    box.innerHTML = `<p class="card-note dadm-found-empty">Não achei “${escDisc(_dadm.foundQuery)}” no Greenhouse nem na Lever. A empresa pode usar outro sistema de vagas, ou o identificador é diferente do nome: abra a página de vagas dela e, se o link for de <code>greenhouse.io</code> ou <code>lever.co</code>, cole o link aqui.</p>`;
    return;
  }
  box.innerHTML = `<div class="dadm-found">${_dadm.found.map((r, i) => {
    const added = dadmHasBoard(r.source, r.token);
    return `
      <div class="dadm-found-row">
        <div><b>${escDisc(dadmSourceName(r.source))}</b> · <code>${escDisc(r.token)}</code>${r.company ? ` · ${escDisc(r.company)}` : ''}
          <div class="disc-status">${r.jobs_kept} de ${r.jobs} vagas servem${r.jobs_kept ? '' : ' (nenhuma no Brasil ou remota para cá)'}</div></div>
        ${added ? '<span class="badge badge-completed">já cadastrada</span>' : `
          <input type="text" id="dadm-found-name-${i}" value="${escDisc(r.suggested)}" maxlength="80" aria-label="Nome a exibir" />
          <button class="btn btn-ghost btn-sm" onclick="dadmAddFound(${i})">Adicionar</button>`}
      </div>`;
  }).join('')}</div>`;
}

function dadmAddFound(i) {
  const r = _dadm.found[i];
  if (!r || dadmHasBoard(r.source, r.token)) return;
  const name = document.getElementById(`dadm-found-name-${i}`).value.trim() || r.token;
  (_dadm.boards[r.source] = _dadm.boards[r.source] || []).push({ token: r.token, company: name });
  renderDadmBoards();
  renderDadmFound();
  dadmDirty();
  toast(`${name} adicionada. Salve as alterações para valer na próxima varredura.`);
}

function dadmSetDirty(on) {
  _dadm.dirty = on;
  document.getElementById('dadm-save').disabled = !on;
  document.getElementById('dadm-dirty').textContent = on ? 'alterações não salvas' : '';
}

function dadmDirty() {
  dadmTermsCount();
  dadmSetDirty(true);
}

async function dadmSave() {
  const interval = dadmIntervalMinutes();
  if (!Number.isFinite(interval) || interval < 15 || interval > 10080) {
    toast('O intervalo precisa ficar entre 15 minutos e 7 dias.', 'error');
    return;
  }
  const body = {
    enabled: document.getElementById('dadm-enabled').checked,
    interval_minutes: interval,
    catalog_terms: dadmTerms(),
    disabled_sources: dadmDisabledSources(),
    boards: dadmBoards(),
  };
  const btn = document.getElementById('dadm-save');
  btn.disabled = true;
  try {
    const res = await api.put('/admin/discovery/settings', body);
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      toast(err.error || 'Erro ao salvar.', 'error');
      btn.disabled = false;
      return;
    }
    toast('Configuração salva. O agendador já está usando.');
    loadDiscoveryAdmin();
  } catch (e) {
    toast(e.message, 'error');
    btn.disabled = false;
  }
}

function dadmRunSelectAll(on) {
  document.querySelectorAll('#dadm-run-sources input[data-run-source]').forEach(cb => { cb.checked = on; });
}

// Todos os sites ligados + catálogo com palavras dos usuários = varredura completa
// (manda o corpo vazio, e ela conta para o agendamento). Qualquer outra escolha é parcial.
async function dadmRunNow() {
  const chosen = [...document.querySelectorAll('#dadm-run-sources input[data-run-source]:checked')].map(cb => cb.dataset.runSource);
  if (!chosen.length) { toast('Marque ao menos um site.', 'error'); return; }
  const catalogOnly = document.querySelector('input[name="dadm-run-terms"]:checked').value === 'catalog';
  const enabled = _dadm.sources.filter(s => s.enabled).map(s => s.name);
  const full = !catalogOnly && chosen.length === enabled.length && chosen.every(n => enabled.includes(n));

  if (_dadm.dirty && !await confirmDialog('Há alterações não salvas. A varredura vai usar a configuração salva. Continuar?')) return;
  const btn = document.getElementById('dadm-run');
  btn.disabled = true;
  try {
    const res = await api.post('/admin/discovery/run', full ? {} : { sources: chosen, catalog_only: catalogOnly });
    if (res.status === 409) { toast('Já existe uma varredura rodando.', 'error'); return; }
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      toast(err.error || 'Erro ao disparar a varredura.', 'error');
      return;
    }
    toast('Varredura iniciada. Abrindo o andamento…');
    goToDiscoveryMetrics();
  } catch (e) {
    toast(e.message, 'error');
  } finally {
    btn.disabled = false;
  }
}
