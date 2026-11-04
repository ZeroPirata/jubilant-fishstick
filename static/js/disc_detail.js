// ── Sobre a vaga (Descobrir) ──
// Uma vaga do catálogo numa página própria (#/descobrir/<id>), aberta pelo sino e pelo
// "Ver mais" do Descobrir: compatibilidade, descrição e onde mais ela aparece, para
// decidir se vale gerar o currículo. Depois de gerar, a mesma página acompanha o
// andamento e leva para a tela da vaga (#/vaga/<id>).

const _sd = { id: null, from: 'descobrir', back: 'descobrir', seq: 0, job: null, vote: null };

function openDiscJob(id, { from = 'descobrir' } = {}) {
  if (!/^[0-9a-f-]{36}$/i.test(id || '')) return;
  _sd.id = id;
  _sd.from = from;
  _sd.vote = null;
  if (typeof closeAlerts === 'function' && _alerts?.open) closeAlerts();
  if (_activeTab !== 'sobre' && _activeTab !== 'vaga') _sd.back = _activeTab || 'descobrir';
  navigateTo('sobre');
  setRoute('#/descobrir/' + id);
  document.querySelectorAll('.sidebar button[data-tab="descobrir"]').forEach(b => b.classList.add('active'));
  const crumb = document.getElementById('crumb-current');
  if (crumb) crumb.textContent = 'descobrir / sobre a vaga';
  window.scrollTo({ top: 0 });
  loadDiscJob();
}

// Volta para a tela de onde veio (o sino abre de qualquer aba); link aberto direto
// volta para o Descobrir.
function closeDiscJob() {
  navigateTo(_sd.back || 'descobrir');
}

async function loadDiscJob() {
  const id = _sd.id;
  const seq = ++_sd.seq;
  const root = document.getElementById('sd-root');
  if (!id || !root) return;
  if (root.dataset.id !== id) root.innerHTML = '<p class="jd-empty">Carregando…</p>';
  root.dataset.id = id;
  try {
    const res = await api.get(`/discovery/jobs/${id}`);
    if (seq !== _sd.seq) return;
    if (res.status === 404) {
      root.innerHTML = '<p class="jd-empty">Esta vaga saiu do catálogo (expirou ou foi removida pelo site).</p>';
      return;
    }
    if (!res.ok) throw new Error('Erro ao carregar a vaga.');
    const job = await apiData(res);
    if (seq !== _sd.seq) return;
    _sd.job = job;
    root.innerHTML = renderDiscJob(job);
  } catch (e) {
    if (seq === _sd.seq) root.innerHTML = `<p class="jd-empty">Erro: ${escHtml(e.message)}</p>`;
  }
}

function renderDiscJob(j) {
  const url = safeHttpUrl(j.url);
  const source = _discSourceNames[j.source] || j.source;
  const levels = (j.seniority || []).map(l => _discLevelNames[l]).filter(Boolean);
  const local = [j.international ? 'Exterior' : '', _discWorkplace[j.workplace_type] || '', j.location].filter(Boolean).join(' · ');
  const fit = j.fit && (j.fit.items || []).length ? j.fit : null;
  const meta = [
    local,
    j.published_at ? `Publicada em ${fmtDay(j.published_at)}` : '',
    j.language ? `Idioma: ${j.language === 'en' ? 'inglês' : 'português'}` : '',
  ].filter(Boolean).map(escHtml).join(' · ');

  const action = j.pipeline_job_id
    ? discPipeline(j.pipeline_job_id, j.pipeline_status)
    : (url ? discGenerateButton(url, j.language) : '');
  const dismiss = j.dismissed
    ? `<button type="button" class="btn btn-ghost btn-sm" onclick="sdDismiss(false, this)">Restaurar</button>`
    : `<button type="button" class="btn btn-ghost btn-sm disc-dismiss" onclick="sdDismiss(true, this)" title="Some do Descobrir e dos alertas só para você">Dispensar</button>`;

  const header = `
    <div class="jd-header">
      <div class="jd-heading">
        <h2 class="jd-title">${escHtml(j.title || 'Vaga sem título')}</h2>
        <div class="jd-company">${[j.company, source].filter(Boolean).map(escHtml).join(' · ')}</div>
        <div class="jd-badges">
          ${fit ? `<span class="badge badge-${escHtml(fit.quality)}">Compatibilidade ${escHtml((qualityLabels[fit.quality] || fit.quality).toLowerCase())}</span>
            <span class="jd-pct">${fit.coverage_pct}% da stack · você cobre ${fit.strong + fit.partial} de ${fit.items.length}</span>` : ''}
          ${levels.length ? `<span class="disc-level" title="Nível estimado pelo título da vaga">${escHtml(levels.join(' / '))}</span>` : ''}
          ${j.dismissed ? '<span class="badge">Dispensada</span>' : ''}
        </div>
        ${meta ? `<div class="jd-meta">${meta}</div>` : ''}
        ${discAlsoOn(j.also_on)}
      </div>
      <div class="jd-actions sd-actions">
        ${action}
        ${url ? `<a class="btn btn-ghost btn-sm" href="${escHtml(url)}" target="_blank" rel="noopener noreferrer">↗ Vaga original</a>` : ''}
        ${dismiss}
      </div>
    </div>
    ${_sd.from === 'alerts' && fit ? `
      <div class="sd-feedback">
        <span>Este alerta foi útil?</span>
        <button type="button" class="alerts-vote${_sd.vote === true ? ' on' : ''}" onclick="sdFeedback(true, this)" aria-pressed="${_sd.vote === true}" title="Útil">👍</button>
        <button type="button" class="alerts-vote${_sd.vote === false ? ' on' : ''}" onclick="sdFeedback(false, this)" aria-pressed="${_sd.vote === false}" title="Não útil">👎</button>
      </div>` : ''}`;

  const fromNote = j.fit_from === 'pipeline'
    ? 'Pela leitura completa da vaga feita na geração do currículo: o mesmo número da aba Vagas.'
    : 'Estimativa pelas tecnologias citadas no texto da vaga. Ao gerar o currículo, a IA lê a vaga inteira e o número é refeito (pode mudar).';
  const fitCard = `
    <div class="jd-card">
      <h3 class="jd-card-title">Compatibilidade</h3>
      ${fit
        ? renderFitDetails({ quality: fit.quality, fit, analysisHint: false }) + `<p class="sd-note">${escHtml(fromNote)}</p>`
        : '<p class="jd-empty-inline">Nenhuma tecnologia conhecida foi identificada no texto desta vaga. Leia a descrição abaixo para decidir.</p>'}
    </div>`;

  const desc = (j.description || '').trim();
  const long = desc.length > 1200;
  const aboutCard = `
    <div class="jd-card">
      <h3 class="jd-card-title">Sobre a vaga</h3>
      ${desc ? `
        <div class="jd-full${long ? ' collapsed' : ''}" id="sd-full">${escHtml(desc)}</div>
        ${long ? '<button class="btn btn-ghost btn-sm" onclick="sdToggleFull(this)">Mostrar tudo</button>' : ''}`
        : `<p class="jd-empty-inline">O site não mandou a descrição desta vaga.${url ? ' Abra a vaga original para ler.' : ''}</p>`}
    </div>`;

  return header + fitCard + aboutCard;
}

function sdToggleFull(btn) {
  const el = document.getElementById('sd-full');
  const collapsed = el.classList.toggle('collapsed');
  btn.textContent = collapsed ? 'Mostrar tudo' : 'Mostrar menos';
}

async function sdDismiss(dismiss, btn) {
  const id = _sd.id;
  btn.disabled = true;
  try {
    const path = `/discovery/jobs/${encodeURIComponent(id)}/dismiss`;
    const res = dismiss ? await api.post(path, {}) : await api.delete(path);
    if (!res.ok) { btn.disabled = false; toast(dismiss ? 'Não foi possível dispensar.' : 'Não foi possível restaurar.', 'error'); return; }
    toast(dismiss ? 'Vaga dispensada. Ela fica no Descobrir, em "Dispensadas".' : 'Vaga restaurada.');
    if (typeof loadAlerts === 'function') loadAlerts();
    loadDiscJob();
  } catch (e) {
    btn.disabled = false;
    toast(e.message || 'Erro de conexão.', 'error');
  }
}

// Avaliação do alerta (quando a página veio do sino), com a compatibilidade que a vaga
// tinha: aparece em Métricas → Busca de vagas.
async function sdFeedback(relevant, btn) {
  const j = _sd.job;
  if (!j || !j.fit) return;
  try {
    const res = await api.post('/discovery/alerts/feedback', {
      job_id: j.id, relevant, quality: j.fit.quality, coverage_pct: j.fit.coverage_pct,
    });
    if (!res.ok) { toast('Não foi possível salvar a avaliação.', 'error'); return; }
    _sd.vote = relevant;
    btn.parentElement.querySelectorAll('.alerts-vote').forEach(b => {
      const on = b === btn;
      b.classList.toggle('on', on);
      b.setAttribute('aria-pressed', on);
    });
    toast(relevant ? 'Obrigado: alerta marcado como útil.' : 'Obrigado: alerta marcado como não útil.');
  } catch (e) {
    toast(e.message || 'Erro de conexão.', 'error');
  }
}
