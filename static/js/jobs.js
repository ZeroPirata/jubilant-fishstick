// ── Jobs + Resumes + PDF + Feedback ──

const statusLabels = {
  pending:        'Pendente',
  processing:     'Processando',
  scraping_basic: 'Coletando',
  scraping_nl:    'Analisando',
  completed:      'Concluído',
  error:          'Erro',
};

const qualityLabels = { low: 'Baixa', mid: 'Média', high: 'Alta' };

// A qualidade não muda o status: vaga concluída com compatibilidade baixa gerou o
// currículo normalmente (o usuário decide se vale se candidatar).
function badge(status) {
  const label = statusLabels[status] || status;
  return `<span class="badge badge-${status}">${label}</span>`;
}

function qualityBadge(jobId, q) {
  if (!q) return '<span style="color:var(--muted)">—</span>';
  const label = qualityLabels[q] || q;
  return `<button class="badge badge-${q} badge-quality-btn" id="quality-btn-${jobId}" onclick="openJobDetail('${jobId}', { focus: 'fit' })" title="Ver a compatibilidade na tela da vaga">${label}<span class="quality-chevron">›</span></button>`;
}

// ── Jobs pagination state ──
const _jobsPg = { offset: 0, size: 20 };

function renderJobsPagination(total, cursor, size) {
  const from = total === 0 ? 0 : cursor + 1;
  const to   = Math.min(cursor + size, total);
  const info = document.getElementById('jobs-page-info');
  const prev = document.getElementById('jobs-prev');
  const next = document.getElementById('jobs-next');
  if (info) info.textContent = total === 0 ? '—' : `${from}–${to} de ${total}`;
  if (prev) prev.disabled = cursor === 0;
  if (next) next.disabled = cursor + size >= total;
}

function jobsPrevPage() {
  if (_jobsPg.offset === 0) return;
  _jobsPg.offset = Math.max(0, _jobsPg.offset - _jobsPg.size);
  loadJobs();
}

function jobsNextPage() {
  _jobsPg.offset += _jobsPg.size;
  loadJobs();
}

function changeJobsSize(val) {
  _jobsPg.size   = parseInt(val, 10);
  _jobsPg.offset = 0;
  loadJobs();
}

function filterJobsByStatus() {
  _jobsPg.offset = 0;
  loadJobs();
}

function filterJobsByQuality() {
  _jobsPg.offset = 0;
  loadJobs();
}

// Cache job stack/requirements by ID (populated in loadJobs)
const _jobMeta = new Map();

// ── SSE ──
let _evtSource = null;

function connectSSE() {
  const token = getToken();
  if (!token) return;
  if (_evtSource) return;
  _evtSource = new EventSource(`${API_BASE}/jobs/events?token=${encodeURIComponent(token)}`);
  _evtSource.onopen = () => setSSEStatus(true);
  _evtSource.onmessage = e => {
    try { applyJobEvent(JSON.parse(e.data)); } catch (_) {}
  };
  _evtSource.onerror = () => {
    setSSEStatus(false);
    _evtSource.close();
    _evtSource = null;
    setTimeout(connectSSE, 5000);
  };
}

// Indicador da sidebar: reflete a conexão real de atualizações em tempo real.
function setSSEStatus(online) {
  const box = document.getElementById('sse-status');
  if (!box) return;
  box.classList.toggle('offline', !online);
  document.getElementById('sse-status-text').textContent = online ? 'tempo real conectado' : 'reconectando…';
}

function disconnectSSE() {
  if (_evtSource) {
    _evtSource.close();
    _evtSource = null;
  }
}

function applyJobEvent(ev) {
  // Vagas mandadas pelo Descobrir mostram o andamento lá também; a tela da vaga aberta
  // se atualiza sozinha.
  if (typeof discOnJobEvent === 'function') discOnJobEvent(ev);
  if (typeof jdOnJobEvent === 'function') jdOnJobEvent(ev);
  const row = document.getElementById(`job-row-${ev.id}`);
  if (!row) { loadJobs(); return; }

  // Quality + gap (atualiza meta antes do badge para que badge() leia correto)
  if (ev.quality || ev.fit || ev.analysis) {
    const meta = _jobMeta.get(ev.id) || {};
    _jobMeta.set(ev.id, {
      ...meta,
      ...(ev.quality  ? { quality: ev.quality }   : {}),
      ...(ev.fit      ? { fit: ev.fit }           : {}),
      ...(ev.analysis ? { analysis: ev.analysis } : {}),
    });
    if (ev.quality) row.cells[4].innerHTML = qualityBadge(ev.id, ev.quality);
  }

  // Status (usa quality já atualizado acima)
  row.cells[3].innerHTML = badge(ev.status);

  // Company / title (preenchidos após scrape)
  if (ev.company_name) row.cells[1].textContent = ev.company_name;
  if (ev.job_title)    row.cells[2].querySelector('.job-title-link')?.replaceChildren(ev.job_title);

  // Botões de ação
  row.cells[7].dataset.status = ev.status;
  row.cells[7].innerHTML = jobActions(ev.id, ev.status);
}

// jobActions: Currículos · Refazer · ✕ numa linha só, com altura igual. Usada na lista
// e na atualização ao vivo (SSE) para as duas nunca divergirem.
function jobActions(id, status) {
  const isDone = status === 'completed';
  const isEnd  = isDone || status === 'error';
  return `<div class="job-actions">
    ${isDone ? `<button class="btn-expand" onclick="openJobDetail('${id}', { focus: 'resumes' })">Currículos</button>` : ''}
    ${isEnd  ? `<button class="btn btn-ghost btn-sm" onclick="retryJob('${id}', this)">Refazer</button>` : ''}
    <button class="btn-delete" onclick="deleteJob('${id}', this)" title="Excluir vaga" aria-label="Excluir vaga">✕</button>
  </div>`;
}

// ── Quality details toggle ──
// Tudo vem calculado do servidor (jobfit): a tela só desenha. Assim a nota, as
// etiquetas e a cobertura nunca discordam.

const _fitLevelLabels = { expert: 'expert', advanced: 'avançado', intermediate: 'intermediário', basic: 'básico' };
const _fitRecommendation = {
  candidatar:               { label: 'Vale se candidatar', cls: 'high' },
  candidatar_com_ressalvas: { label: 'Candidatar com ressalvas', cls: 'mid' },
  baixa_aderencia:          { label: 'Baixa aderência', cls: 'low' },
};

function fitTag(it) {
  const level = _fitLevelLabels[it.level] || it.level || '';
  if (it.status === 'missing') {
    return `<span class="fit-tag fit-missing" title="Não encontrado no seu perfil">${escHtml(it.name)}</span>`;
  }
  const via = it.skill && it.skill.toLowerCase() !== it.name.toLowerCase() ? ` (via ${it.skill})` : '';
  const title = `Você tem${via}${level ? `, nível ${level}` : ''}`;
  return `<span class="fit-tag fit-${it.status}" title="${escHtml(title)}">${escHtml(it.name)}${level ? `<small>${escHtml(level)}</small>` : ''}</span>`;
}

function fitSummaryText(fit) {
  const total = fit.items.length;
  const parts = [`${fit.strong + fit.partial} de ${total} tecnologias no seu perfil`];
  if (fit.partial) parts.push(`${fit.partial} em nível básico/intermediário`);
  if (fit.missing) parts.push(`${fit.missing} faltando`);
  return parts.join(' · ');
}

// renderFitDetails: compatibilidade (análise da IA, stack × perfil, requisitos) de uma
// vaga. Usada na tela da vaga (job_detail.js).
function renderFitDetails({ stack = [], reqs = [], quality = null, fit = null, analysis = null } = {}) {
  const qualityLbl = qualityLabels[quality] || '—';

  const summary = quality
    ? `Compatibilidade técnica:
       <span class="badge badge-${quality}" style="padding:1px 6px;font-size:10px">${qualityLbl}</span>
       <span style="color:var(--muted);font-size:11px">— pela stack da vaga e o nível das suas habilidades. O currículo é gerado em qualquer nível.</span>`
    : '<span style="color:var(--muted)">Qualidade ainda não calculada — aguarde o processamento.</span>';

  let analysisSection = '';
  if (analysis) {
    const rec = _fitRecommendation[analysis.recomendacao];
    const list = (title, items, cls) => items && items.length ? `
      <div class="fit-analysis-col">
        <div class="quality-group-label" style="margin-bottom:4px">${title}</div>
        <ul class="fit-list ${cls}">${items.map(i => `<li>${escHtml(i)}</li>`).join('')}</ul>
      </div>` : '';
    analysisSection = `
      <div class="fit-analysis">
        <div class="fit-analysis-head">
          <span class="quality-group-label">Análise da IA</span>
          ${rec ? `<span class="badge badge-${rec.cls}" style="padding:1px 6px;font-size:10px">${rec.label}</span>` : ''}
        </div>
        ${analysis.resumo ? `<p class="fit-analysis-text">${escHtml(analysis.resumo)}</p>` : ''}
        <div class="fit-analysis-cols">
          ${list('Pontos fortes', analysis.pontos_fortes, 'fit-list-good')}
          ${list('Lacunas', analysis.lacunas, 'fit-list-gap')}
        </div>
      </div>`;
  } else if (quality) {
    analysisSection = '<div class="fit-analysis-empty">Sem análise da IA para esta vaga (processada antes da análise existir). Use “Refazer” para gerar.</div>';
  }

  let stackSection = '';
  if (fit && fit.items.length) {
    const pct = fit.coverage_pct;
    stackSection = `
      <div style="margin-bottom:12px">
        <div class="quality-group-label" style="margin-bottom:6px">Stack da vaga × seu perfil</div>
        <div style="display:flex;align-items:center;gap:10px;margin-bottom:6px">
          <div style="background:var(--border);border-radius:4px;height:8px;flex:1;max-width:320px;overflow:hidden">
            <div style="background:${pct >= 70 ? 'var(--green)' : pct >= 30 ? 'var(--yellow)' : 'var(--red)'};height:100%;width:${pct}%;transition:width .4s"></div>
          </div>
          <div style="font-size:12px;font-weight:600;color:var(--text)">${pct}%</div>
          <div style="font-size:11px;color:var(--muted)">${escHtml(fitSummaryText(fit))}</div>
        </div>
        <div style="display:flex;flex-wrap:wrap;gap:4px">${fit.items.map(fitTag).join('')}</div>
        <div class="fit-legend">
          <span><i class="fit-dot fit-strong"></i>avançado/expert</span>
          <span><i class="fit-dot fit-partial"></i>básico/intermediário (conta parcialmente)</span>
          <span><i class="fit-dot fit-missing"></i>não está no perfil</span>
        </div>
      </div>`;
  } else if (stack.length) {
    stackSection = `
      <div style="margin-bottom:12px">
        <div class="quality-group-label" style="margin-bottom:4px">Stack da vaga</div>
        <div style="display:flex;flex-wrap:wrap;gap:4px">${stack.map(t => `<span class="fit-tag fit-missing">${escHtml(t)}</span>`).join('')}</div>
        <div style="font-size:11px;color:var(--muted);margin-top:6px">Cadastre habilidades no perfil para ver a compatibilidade.</div>
      </div>`;
  }

  const reqSection = reqs.length ? `
    <div class="fit-reqs">
      <div class="quality-group-label" style="margin-bottom:4px">⚠ Outros requisitos — confira você mesmo</div>
      <div style="font-size:11px;color:var(--muted);margin-bottom:4px">A porcentagem considera só a stack; ${analysis ? 'a análise da IA acima já leva estes requisitos em conta.' : 'estes requisitos não entram nela.'}</div>
      ${reqs.map(r => `<div class="quality-req">• ${escHtml(r)}</div>`).join('')}
    </div>` : '';

  const hasAnyData = analysisSection || stackSection || reqSection;

  return `
    <div class="quality-summary" style="margin-bottom:10px">${summary}</div>
    ${analysisSection}
    ${stackSection}
    ${reqSection}
    ${!hasAnyData ? '<span style="color:var(--muted)">Nenhum dado de stack extraído desta vaga.</span>' : ''}`;
}

// ── VAGAS ──
async function loadJobs() {
  const tbody = document.getElementById('jobs-body');
  const statusFilter  = document.getElementById('jobs-status-filter').value;
  const qualityFilter = document.getElementById('jobs-quality-filter').value;
  let qs = `?offset=${_jobsPg.offset}&size=${_jobsPg.size}`;
  if (statusFilter)  qs += `&status=${statusFilter}`;
  if (qualityFilter) qs += `&quality=${qualityFilter}`;
  tbody.innerHTML = `<tr><td colspan="8" class="empty">Carregando...</td></tr>`;

  try {
    const res = await api.get('/jobs' + qs);
    if (!res.ok) { tbody.innerHTML = `<tr><td colspan="8" class="empty">Erro ao carregar vagas.</td></tr>`; return; }
    const { data: jobs, meta } = await apiList(res);

    renderJobsPagination(meta.total ?? 0, meta.cursor ?? _jobsPg.offset, meta.size ?? _jobsPg.size);

    if (!jobs.length) {
      tbody.innerHTML = `<tr><td colspan="8" class="empty">Nenhuma vaga encontrada.</td></tr>`;
      return;
    }

    jobs.forEach(j => {
      _jobMeta.set(j.id, {
        stack: j.stacks || [], reqs: j.requirements || [], quality: j.quality,
        fit: j.fit || null, analysis: j.analysis || null,
      });
    });

    tbody.innerHTML = jobs.map(j => {
      const { quality } = _jobMeta.get(j.id);
      return `
      <tr id="job-row-${j.id}" class="job-row-link" onclick="jobRowClick(event, '${j.id}')" title="Abrir a vaga">
        <td style="color:var(--muted);font-size:11px" class="truncate" title="${j.id}" data-label="ID">${j.id.slice(0,8)}…</td>
        <td data-label="Empresa">${j.company_name ? escHtml(j.company_name) : '<span style="color:var(--muted)">—</span>'}</td>
        <td class="truncate" title="${escHtml(j.job_title || '')}" data-label="Título"><span class="job-title-link">${j.job_title ? escHtml(j.job_title) : '<span style="color:var(--muted)">—</span>'}</span></td>
        <td data-label="Status">${badge(j.status)}</td>
        <td data-label="Qualidade">${qualityBadge(j.id, quality)}</td>
        <td style="color:var(--muted)" data-label="Cadastrada">${fmtDate(j.created_at)}</td>
        <td data-label="URL">
          ${j.url ? `<a class="link" href="${j.url}" target="_blank" rel="noopener">↗ abrir</a>` : '—'}
        </td>
        <td class="job-actions-cell" data-status="${j.status}" data-lang="${j.language === 'en' ? 'en' : 'pt-br'}">${jobActions(j.id, j.status)}</td>
      </tr>
    `; }).join('');
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="8" class="empty">Erro: ${e.message}</td></tr>`;
  }
}

// Clique na linha abre a tela da vaga; botões, links e seletores fazem o próprio trabalho.
function jobRowClick(ev, id) {
  if (ev.target.closest('button, a, select, input, textarea, label')) return;
  openJobDetail(id);
}

async function deleteJob(id, btn) {
  if (!await confirmDialog('Deletar esta vaga e todos os currículos gerados?')) return;
  btn.disabled = true;
  try {
    const res = await api.delete(`/jobs/${id}`);
    if (!res.ok) { toast('Erro ao deletar vaga.', 'error'); btn.disabled = false; return; }
    document.getElementById(`job-row-${id}`)?.remove();
    const resumeRow = document.getElementById(`resumes-row-${id}`);
    resumeRow?.remove();
    toast('Vaga deletada.');
  } catch (e) { toast(e.message, 'error'); btn.disabled = false; }
}

// O Refazer troca as ações pela escolha do que gerar; "Cancelar" volta aos botões
// sem recarregar a lista. O ✕ de excluir sai enquanto isso, para não ficar ao lado
// do cancelar.
// ── Como gerar ──
// Idioma e o que gerar, escolhidos a cada geração: no Refazer (Vagas e tela da vaga) e
// no "Gerar currículo" do Descobrir. Vaga em inglês pede currículo em inglês, mas quem
// decide é o usuário. onGo é o nome da função chamada como onGo(mode, botão, ...args);
// o idioma ela lê com genLanguage(botão).
const _genModes = [['full', 'Tudo', 'Currículo e cover letter'], ['resume_only', 'Currículo', 'Só o currículo'], ['cover_only', 'Cover', 'Só a cover letter']];

// retry: mostra "Nova versão da IA". Marcada (padrão), a IA escreve de novo; sem
// marcar, se vaga, perfil, idioma e modo não mudaram, a última resposta é reaproveitada
// sem gastar cota.
function genChooser({ label = 'Gerar:', lang = 'pt-br', onGo, args = [], onCancel, retry = false }) {
  const extra = args.map(a => `, '${a}'`).join('');
  const langOpt = (v, name) => `<option value="${v}"${lang === v ? ' selected' : ''}>${name}</option>`;
  return `<div class="job-actions gen-chooser">
    <span class="job-actions-label">${label}</span>
    <select class="gen-lang" aria-label="Idioma do currículo" title="Idioma do currículo e da cover letter">
      ${langOpt('pt-br', 'PT-BR')}${langOpt('en', 'EN')}
    </select>
    ${retry ? `<label class="gen-fresh" title="Desmarcada: se nada mudou (vaga, perfil, idioma e modo), reaproveita a última resposta da IA sem gastar cota"><input type="checkbox" class="gen-fresh-check" checked> Nova versão da IA</label>` : ''}
    ${_genModes.map(([m, name, tip]) => `<button class="btn btn-ghost btn-sm" title="${tip}" onclick="${onGo}('${m}', this${extra})">${name}</button>`).join('')}
    <button class="btn btn-ghost btn-sm job-actions-cancel" onclick="${onCancel}">Cancelar</button>
  </div>`;
}

function genLanguage(btn) {
  return btn.closest('.gen-chooser')?.querySelector('.gen-lang')?.value || 'pt-br';
}

// genFresh: "Nova versão da IA" marcada (true quando o seletor não tem a opção).
function genFresh(btn) {
  const c = btn.closest('.gen-chooser')?.querySelector('.gen-fresh-check');
  return c ? c.checked : true;
}

function retryJob(id, btn) {
  const td = btn.closest('td');
  td.innerHTML = genChooser({ lang: td.dataset.lang, onGo: 'doRetryWithMode', args: [id], onCancel: `cancelRetry('${id}', this)`, retry: true });
}

function cancelRetry(id, btn) {
  const td = btn.closest('td');
  td.innerHTML = jobActions(id, td.dataset.status);
}

async function doRetryWithMode(mode, btn, id) {
  const language = genLanguage(btn), fresh = genFresh(btn);
  const div = btn.closest('div');
  div.querySelectorAll('button, select, input').forEach(b => b.disabled = true);
  try {
    const res = await api.put(`/jobs/${id}/retry`, { mode, language, fresh });
    if (!res.ok) { toast('Erro ao reprocessar.', 'error'); loadJobs(); return; }
    toast(`Vaga enviada para reprocessamento (${language === 'en' ? 'inglês' : 'português'}).`);
    loadJobs();
  } catch (e) { toast(e.message, 'error'); loadJobs(); }
}

// Modelos de PDF (iguais a TEMPLATES em scripts/gerar_pdf.py). Trocar de modelo
// só reformata: não chama a IA de novo.
const PDF_TEMPLATES = [
  ['classico',         'Clássico'],
  ['executivo',        'Executivo'],
  ['compacto',         'Compacto (1 página)'],
  ['primeiro_emprego', 'Primeiro emprego'],
];

function preferredTemplate() {
  try { return localStorage.getItem('jp_pdf_template') || 'classico'; } catch (_) { return 'classico'; }
}

function templateSelect(resumeId) {
  const pref = preferredTemplate();
  const opts = PDF_TEMPLATES.map(([v, l]) => `<option value="${v}" ${v === pref ? 'selected' : ''}>${l}</option>`).join('');
  return `<select id="tpl-${resumeId}" class="tpl-select" aria-label="Modelo do PDF" title="Modelo do PDF">${opts}</select>`;
}

// selectable: checkbox em cada card para apagar vários (tela da vaga).
function renderResumes(jobId, resumes, { selectable = false } = {}) {
  if (!resumes.length) return '<p style="color:var(--muted);font-size:13px">Nenhum currículo gerado ainda.</p>';
  return resumes.map((r, i) => {
    const key = `r-${jobId}-${i}`;
    let content = {};
    try { content = JSON.parse(r.content_json || '{}'); } catch (_) {}
    const hasPdf = !!r.resume_pdf_path;

    return `
      <div class="resume-card" id="rc-${r.id}">
        <div class="resume-card-header">
          ${selectable ? `<input type="checkbox" class="resume-check" value="${r.id}" onchange="jdSelChanged()" aria-label="Selecionar este currículo">` : ''}
          <div class="resume-card-title">
            <h4>${escHtml(r.job?.company_name || 'Empresa')} — ${escHtml(r.job?.job_title || 'Vaga')}</h4>
            <div class="meta">Gerado em ${fmtDate(r.created_at)}${r.job?.quality ? ` · qualidade: ${r.job.quality}` : ''}</div>
          </div>
          <div class="resume-card-actions">
            ${content.version >= 2 ? templateSelect(r.id) : ''}
            <button id="pdf-btn-${r.id}" class="btn-pdf" onclick="generatePDF('${jobId}','${r.id}',this)">
              ${hasPdf ? '↺ Regerar PDF' : '⬇ Gerar PDF'}
            </button>
            <button class="btn btn-ghost btn-sm" onclick="openFeedback('${jobId}','${r.id}')">Avaliar</button>
            <button class="btn-delete" onclick="deleteResume('${jobId}','${r.id}',this)">✕</button>
          </div>
        </div>

        ${hasPdf ? pdfBar(r.resume_pdf_path, r.cover_letter_path, r.id) : `<div id="pdf-bar-${r.id}"></div>`}

        <div class="resume-tabs">
          <button class="active" onclick="switchResumeTab(this,'${key}-cv')">Currículo</button>
          <button onclick="switchResumeTab(this,'${key}-cl')">Cover Letter</button>
        </div>
        <div id="${key}-cv" class="resume-pane active">
          <pre class="resume-text">${escHtml(content.curriculo || '')}</pre>
        </div>
        <div id="${key}-cl" class="resume-pane">
          <pre class="resume-text">${escHtml(content.cover_letter || '')}</pre>
        </div>
      </div>
    `;
  }).join('');
}

function pdfBar(resumePath, coverPath, resumeId) {
  return `
    <div id="pdf-bar-${resumeId}" class="pdf-bar">
      <span>PDFs:</span>
      ${resumePath ? `<button class="btn-pdf-download" onclick="downloadPdf('${resumePath}','curriculo.pdf')">📄 Currículo</button>` : ''}
      ${coverPath  ? `<button class="btn-pdf-download" onclick="downloadPdf('${coverPath}','cover_letter.pdf')">📄 Cover Letter</button>` : ''}
    </div>
  `;
}

async function downloadPdf(path, filename) {
  const token = getToken();
  let res;
  try {
    res = await fetch('/' + path, {
      headers: token ? { 'Authorization': `Bearer ${token}` } : {}
    });
  } catch (e) {
    toast('Erro de rede: ' + e.message, 'error');
    return;
  }
  if (res.status === 401) { toast('Sessão expirada — faça login novamente.', 'error'); return; }
  if (res.status === 403) { toast('Acesso negado ao arquivo.', 'error'); return; }
  if (res.status === 404) { toast('Arquivo não encontrado no servidor.', 'error'); return; }
  if (!res.ok)            { toast('Erro ao baixar o arquivo (HTTP ' + res.status + ').', 'error'); return; }
  const blob = await res.blob();
  if (blob.size === 0)    { toast('O arquivo gerado está vazio — tente regenerar.', 'error'); return; }
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  setTimeout(() => URL.revokeObjectURL(url), 2000);
}

function switchResumeTab(btn, targetId) {
  const card = btn.closest('.resume-card');
  card.querySelectorAll('.resume-tabs button').forEach(b => b.classList.remove('active'));
  card.querySelectorAll('.resume-pane').forEach(p => p.classList.remove('active'));
  btn.classList.add('active');
  document.getElementById(targetId)?.classList.add('active');
}

async function generatePDF(jobId, resumeId, btn) {
  btn.disabled = true;
  btn.textContent = '⏳ Gerando...';

  const select = document.getElementById(`tpl-${resumeId}`);
  const template = select ? select.value : 'classico';
  if (select) { try { localStorage.setItem('jp_pdf_template', template); } catch (_) {} }

  try {
    const res = await api.post(`/jobs/${jobId}/resumes/${resumeId}/pdf`, { template });
    if (!res.ok) {
      const txt = await res.text();
      toast('Erro ao gerar PDF: ' + txt, 'error');
      btn.disabled = false;
      btn.textContent = '⬇ Gerar PDF';
      return;
    }

    const data = await res.json();
    btn.textContent = '↺ Regerar PDF';
    btn.disabled = false;

    const barEl = document.getElementById(`pdf-bar-${resumeId}`);
    if (barEl) {
      barEl.outerHTML = pdfBar(data.resume_path, data.cover_letter_path, resumeId);
    }
    toast('PDFs gerados com sucesso!');
  } catch (e) {
    toast(e.message, 'error');
    btn.disabled = false;
    btn.textContent = '⬇ Gerar PDF';
  }
}

async function deleteResume(jobId, resumeId, btn) {
  if (!await confirmDialog('Deletar este currículo?')) return;
  btn.disabled = true;
  try {
    const res = await api.delete(`/jobs/${jobId}/resumes/${resumeId}`);
    if (!res.ok) { toast('Erro ao deletar currículo.', 'error'); btn.disabled = false; return; }
    document.getElementById(`rc-${resumeId}`)?.remove();
    toast('Currículo deletado.');
    // Na tela da vaga, recarrega para o contador "Currículos (N)" e o estado vazio.
    if (_activeTab === 'vaga' && _jd.id === jobId) loadJobDetail();
  } catch (e) { toast(e.message, 'error'); btn.disabled = false; }
}

// ── Aba de currículos (vista global: itera jobs completados) ──
async function loadResumesTab() {
  const container = document.getElementById('curriculos-container');
  container.innerHTML = '<p class="empty">Carregando vagas...</p>';

  try {
    const res = await api.get('/jobs?offset=0&size=200');
    const { data: jobs } = await apiList(res);
    const completed = jobs.filter(j => j.status === 'completed');

    if (!completed.length) {
      container.innerHTML = '<p class="empty">Nenhuma vaga com currículo gerado ainda.</p>';
      return;
    }

    container.innerHTML = '<p class="empty">Carregando currículos...</p>';

    const results = await Promise.all(
      completed.map(j =>
        api.get(`/jobs/${j.id}/resumes?cursor=0&size=10`)
          .then(r => apiList(r))
          .then(({ data }) => ({ job: j, resumes: data }))
          .catch(() => ({ job: j, resumes: [] }))
      )
    );

    const withResumes = results.filter(r => r.resumes.length > 0);
    if (!withResumes.length) {
      container.innerHTML = '<p class="empty">Nenhum currículo encontrado.</p>';
      return;
    }

    container.innerHTML = withResumes.map(({ job, resumes }) =>
      renderResumes(job.id, resumes)
    ).join('<hr style="border:none;border-top:1px solid var(--border);margin:20px 0">');
  } catch (e) {
    container.innerHTML = `<p class="empty">Erro: ${e.message}</p>`;
  }
}

// ── Nova Vaga ──
async function createJob() {
  const url         = document.getElementById('new-job-url').value.trim();
  const companyName = document.getElementById('new-job-company').value.trim() || undefined;
  const jobTitle    = document.getElementById('new-job-title').value.trim()   || undefined;
  const description = document.getElementById('new-job-desc').value.trim()    || undefined;
  const language    = document.getElementById('new-job-lang').value           || undefined;
  const mode        = document.getElementById('new-job-mode').value           || 'full';

  if (!url) { toast('URL da vaga é obrigatória.', 'error'); return; }
  if (description && !companyName) { toast('Preencha o nome da empresa ao informar a descrição.', 'error'); return; }
  if (description && !jobTitle)    { toast('Preencha o título da vaga ao informar a descrição.', 'error'); return; }

  const body = { url, mode };
  if (companyName)  body.company_name = companyName;
  if (jobTitle)     body.job_title    = jobTitle;
  if (description)  body.description  = description;
  if (language)     body.language     = language;

  try {
    const btn = document.getElementById('btn-create-job');
    btn.disabled = true;
    const res = await api.post('/jobs', body);
    btn.disabled = false;

    if (res.status === 409) { toast('Esta URL já foi cadastrada.', 'error'); return; }
    if (!res.ok) { toast('Erro ao cadastrar vaga.', 'error'); return; }

    const data = await res.json();
    const warnings = data.warnings || [];

    ['new-job-url','new-job-company','new-job-title','new-job-desc'].forEach(id => {
      document.getElementById(id).value = '';
    });

    if (warnings.length > 0) {
      const list = document.getElementById('profile-warnings-list');
      list.innerHTML = warnings.map(w => `<li>${escHtml(w)}</li>`).join('');
      document.getElementById('profile-warnings-banner').classList.remove('hidden');
      toast('Vaga cadastrada! Confira os avisos sobre o perfil.');
    } else {
      document.getElementById('profile-warnings-banner').classList.add('hidden');
      toast('Vaga cadastrada! O worker irá processá-la em breve.');
      navigateTo('vagas');
    }
  } catch (e) {
    toast(e.message, 'error');
  }
}

// ── Feedback modal ──
let _feedbackJobId = '', _feedbackResumeId = '', _feedbackRating = '';

function openFeedback(jobId, resumeId) {
  _feedbackJobId    = jobId;
  _feedbackResumeId = resumeId;
  _feedbackRating   = '';
  document.getElementById('feedback-comment').value = '';
  document.querySelectorAll('.rating-btn').forEach(b =>
    b.className = 'rating-btn'
  );
  document.getElementById('feedback-modal').classList.remove('hidden');
}

function closeFeedback() {
  document.getElementById('feedback-modal').classList.add('hidden');
}

function selectRating(val, btn) {
  _feedbackRating = val;
  document.querySelectorAll('.rating-btn').forEach(b => b.className = 'rating-btn');
  btn.classList.add(`sel-${val}`);
}

async function submitFeedback() {
  if (!_feedbackRating) { toast('Selecione uma avaliação.', 'error'); return; }
  const comments = document.getElementById('feedback-comment').value.trim();
  if (!comments) { toast('Comentário é obrigatório.', 'error'); return; }

  try {
    const res = await api.post(
      `/jobs/${_feedbackJobId}/resumes/${_feedbackResumeId}/feedback`,
      { status: _feedbackRating, comments }
    );
    if (!res.ok) { toast('Erro ao salvar avaliação.', 'error'); return; }
    closeFeedback();
    toast('Avaliação salva!');
  } catch (e) { toast(e.message, 'error'); }
}

function escHtml(str) {
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

// ── Lote de Links ──
const _bulkUrls = []; // { url: string, language: string, mode: string }[]

const _bulkModeLabels = { full: 'Tudo', resume_only: 'Currículo', cover_only: 'Cover' };

function _bulkRefresh() {
  const list  = document.getElementById('bulk-url-list');
  const empty = document.getElementById('bulk-url-empty');
  const btn   = document.getElementById('btn-bulk-submit');

  empty.style.display = _bulkUrls.length ? 'none' : 'block';
  btn.disabled        = _bulkUrls.length === 0;
  btn.textContent     = `Enviar ${_bulkUrls.length} vaga${_bulkUrls.length !== 1 ? 's' : ''}`;

  list.innerHTML = _bulkUrls.map((item, i) => `
    <div style="display:flex;align-items:center;gap:8px;background:var(--surface);border:1px solid var(--border);border-radius:6px;padding:7px 10px">
      <span style="font-size:11px;padding:2px 6px;border-radius:4px;background:var(--border);color:var(--muted);white-space:nowrap;flex-shrink:0">${escHtml(item.language.toUpperCase())}</span>
      <span style="font-size:11px;padding:2px 6px;border-radius:4px;background:var(--border);color:var(--muted);white-space:nowrap;flex-shrink:0">${escHtml(_bulkModeLabels[item.mode] || item.mode)}</span>
      <span style="flex:1;font-size:13px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--text)" title="${escHtml(item.url)}">${escHtml(item.url)}</span>
      <button onclick="removeBulkUrl(${i})" style="flex-shrink:0;background:none;border:none;cursor:pointer;color:var(--muted);font-size:15px;line-height:1;padding:0 2px" title="Remover">✕</button>
    </div>
  `).join('');
}

function addBulkUrl() {
  const input    = document.getElementById('bulk-url-input');
  const language = document.getElementById('bulk-lang').value;
  const mode     = document.getElementById('bulk-mode').value || 'full';
  const url      = input.value.trim();
  if (!url) return;
  if (!url.startsWith('http')) { toast('URL inválida.', 'error'); return; }
  if (_bulkUrls.some(item => item.url === url)) { toast('URL já adicionada.', 'error'); return; }
  _bulkUrls.push({ url, language, mode });
  input.value = '';
  input.focus();
  _bulkRefresh();
}

function removeBulkUrl(index) {
  _bulkUrls.splice(index, 1);
  _bulkRefresh();
}

async function submitBulkUrls() {
  if (!_bulkUrls.length) return;

  const btn = document.getElementById('btn-bulk-submit');
  btn.disabled    = true;
  btn.textContent = 'Enviando...';

  const results = await Promise.allSettled(
    _bulkUrls.map(({ url, language, mode }) => api.post('/jobs', { url, language, mode }))
  );

  let ok = 0, fail = 0;
  results.forEach(r => {
    if (r.status === 'fulfilled' && r.value.ok) ok++;
    else fail++;
  });

  _bulkUrls.length = 0;
  _bulkRefresh();

  if (fail === 0) {
    toast(`${ok} vaga${ok !== 1 ? 's' : ''} cadastrada${ok !== 1 ? 's' : ''}!`);
    navigateTo('vagas');
  } else {
    toast(`${ok} cadastrada${ok !== 1 ? 's' : ''}, ${fail} com erro.`, 'error');
  }
}
