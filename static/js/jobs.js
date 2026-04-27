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

function badge(status, quality) {
  const label = statusLabels[status] || status;
  const cls   = (status === 'completed' && quality === 'low') ? 'badge-error' : `badge-${status}`;
  return `<span class="badge ${cls}">${label}</span>`;
}

function qualityBadge(jobId, q) {
  if (!q) return '<span style="color:var(--muted)">—</span>';
  const label = qualityLabels[q] || q;
  return `<button class="badge badge-${q} badge-quality-btn" id="quality-btn-${jobId}" onclick="toggleQualityDetails('${jobId}')" title="Ver detalhes da compatibilidade">${label}<span class="quality-chevron">▾</span></button>`;
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
  _evtSource.onmessage = e => {
    try { applyJobEvent(JSON.parse(e.data)); } catch (_) {}
  };
  _evtSource.onerror = () => {
    _evtSource.close();
    _evtSource = null;
    setTimeout(connectSSE, 5000);
  };
}

function disconnectSSE() {
  if (_evtSource) {
    _evtSource.close();
    _evtSource = null;
  }
}

function applyJobEvent(ev) {
  const row = document.getElementById(`job-row-${ev.id}`);
  if (!row) { loadJobs(); return; }

  // Quality (atualiza meta antes do badge para que badge() leia correto)
  if (ev.quality) {
    const meta = _jobMeta.get(ev.id) || {};
    _jobMeta.set(ev.id, { ...meta, quality: ev.quality });
    row.cells[4].innerHTML = qualityBadge(ev.id, ev.quality);
  }

  // Status (usa quality já atualizado acima)
  const q = (_jobMeta.get(ev.id) || {}).quality;
  row.cells[3].innerHTML = badge(ev.status, q);

  // Company / title (preenchidos após scrape)
  if (ev.company_name) row.cells[1].textContent = ev.company_name;
  if (ev.job_title)    row.cells[2].textContent = ev.job_title;

  // Botões de ação
  const isDone = ev.status === 'completed';
  const isEnd  = isDone || ev.status === 'error';
  row.cells[7].querySelector('div').innerHTML = `
    ${isDone ? `<button class="btn-expand btn-sm" onclick="toggleResumes('${ev.id}', this)">Currículos</button>` : ''}
    ${isEnd  ? `<button class="btn btn-ghost btn-sm" onclick="retryJob('${ev.id}', this)">Refazer</button>` : ''}
    <button class="btn-delete" onclick="deleteJob('${ev.id}', this)">✕</button>
  `;
}

// ── Quality details toggle ──
function toggleQualityDetails(jobId) {
  const existing = document.getElementById(`quality-row-${jobId}`);
  const btn = document.getElementById(`quality-btn-${jobId}`);
  if (existing) { existing.remove(); btn?.classList.remove('expanded'); return; }
  btn?.classList.add('expanded');

  const { stack = [], reqs = [], quality = null } = _jobMeta.get(jobId) || {};

  const qualityLabelsMap = { low: 'Baixa', mid: 'Média', high: 'Alta' };
  const qualityCls       = quality || 'mid';
  const qualityLbl       = qualityLabelsMap[quality] || '—';

  const stackSection = stack.length ? `
    <div style="margin-bottom:${reqs.length ? 10 : 0}px">
      <div class="quality-group-label" style="margin-bottom:4px">Stack tecnológico detectado</div>
      <div style="display:flex;flex-wrap:wrap;gap:4px">
        ${stack.map(t => `<span class="miss-tag">${escHtml(t)}</span>`).join('')}
      </div>
    </div>` : '';

  const reqSection = reqs.length ? `
    <div>
      <div class="quality-group-label" style="margin-bottom:4px">Requisitos (não contam no score)</div>
      <div style="display:flex;flex-direction:column;gap:3px">
        ${reqs.map(r => `<div class="quality-req" style="color:var(--muted)">${escHtml(r)}</div>`).join('')}
      </div>
    </div>` : '';

  const tr = document.createElement('tr');
  tr.id = `quality-row-${jobId}`;
  tr.className = 'quality-details-row';
  tr.innerHTML = `<td colspan="8"><div class="quality-details-panel">
    <div class="quality-summary" style="margin-bottom:10px">
      ${quality
        ? `Compatibilidade calculada pelo sistema:
           <span class="badge badge-${qualityCls}" style="padding:1px 6px;font-size:10px">${qualityLbl}</span>
           &nbsp;<span style="color:var(--muted);font-size:11px">baseada nas suas habilidades cadastradas</span>`
        : '<span style="color:var(--muted)">Qualidade ainda não calculada — aguarde o processamento.</span>'
      }
    </div>
    ${stackSection}
    ${reqSection}
    ${!stackSection && !reqSection ? '<span style="color:var(--muted)">Nenhum dado de stack extraído desta vaga.</span>' : ''}
  </div></td>`;

  document.getElementById(`job-row-${jobId}`)?.insertAdjacentElement('afterend', tr);
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
      _jobMeta.set(j.id, { stack: j.stacks || [], reqs: j.requirements || [], quality: j.quality });
    });

    tbody.innerHTML = jobs.map(j => {
      const { quality } = _jobMeta.get(j.id);
      return `
      <tr id="job-row-${j.id}">
        <td style="color:var(--muted);font-size:11px" class="truncate" title="${j.id}">${j.id.slice(0,8)}…</td>
        <td>${j.company_name || '<span style="color:var(--muted)">—</span>'}</td>
        <td class="truncate" title="${j.job_title||''}">${j.job_title || '<span style="color:var(--muted)">—</span>'}</td>
        <td>${badge(j.status, j.quality)}</td>
        <td>${qualityBadge(j.id, quality)}</td>
        <td style="color:var(--muted)">${fmtDate(j.created_at)}</td>
        <td>
          ${j.url ? `<a class="link" href="${j.url}" target="_blank" rel="noopener">↗ abrir</a>` : '—'}
        </td>
        <td>
          <div style="display:flex;gap:6px;align-items:center;flex-wrap:wrap">
            ${j.status === 'completed'
              ? `<button class="btn-expand btn-sm" onclick="toggleResumes('${j.id}', this)">Currículos</button>`
              : ''
            }
            ${j.status === 'error' || j.status === 'completed'
              ? `<button class="btn btn-ghost btn-sm" onclick="retryJob('${j.id}', this)">Refazer</button>`
              : ''
            }
            <button class="btn-delete" onclick="deleteJob('${j.id}', this)">✕</button>
          </div>
        </td>
      </tr>
    `; }).join('');
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="8" class="empty">Erro: ${e.message}</td></tr>`;
  }
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

async function retryJob(id, btn) {
  btn.disabled = true;
  try {
    const res = await api.put(`/jobs/${id}/retry`);
    if (!res.ok) { toast('Erro ao reprocessar.', 'error'); btn.disabled = false; return; }
    toast('Vaga enviada para reprocessamento.');
    loadJobs();
  } catch (e) { toast(e.message, 'error'); btn.disabled = false; }
}

// ── Toggle resumes panel per job row ──
async function toggleResumes(jobId, btn) {
  const existingRow = document.getElementById(`resumes-row-${jobId}`);
  if (existingRow) {
    existingRow.remove();
    btn.textContent = 'Currículos';
    return;
  }

  btn.textContent = '⏳ Carregando...';
  btn.disabled = true;

  try {
    const res = await api.get(`/jobs/${jobId}/resumes?cursor=0&size=20`);
    const { data: resumes } = await apiList(res);

    const jobRow = document.getElementById(`job-row-${jobId}`);
    const tr = document.createElement('tr');
    tr.id = `resumes-row-${jobId}`;
    tr.className = 'job-resumes-row';
    tr.innerHTML = `<td colspan="8"><div class="job-resumes-panel">
      ${renderResumes(jobId, resumes)}
    </div></td>`;
    jobRow.insertAdjacentElement('afterend', tr);

    btn.textContent = '▲ Fechar';
    btn.disabled = false;
  } catch (e) {
    toast(e.message, 'error');
    btn.textContent = 'Currículos';
    btn.disabled = false;
  }
}

function renderResumes(jobId, resumes) {
  if (!resumes.length) return '<p style="color:var(--muted);font-size:13px">Nenhum currículo gerado ainda.</p>';
  return resumes.map((r, i) => {
    const key = `r-${jobId}-${i}`;
    let content = {};
    try { content = JSON.parse(r.content_json || '{}'); } catch (_) {}
    const hasPdf = !!r.resume_pdf_path;

    return `
      <div class="resume-card" id="rc-${r.id}">
        <div class="resume-card-header">
          <div>
            <h4>${r.job?.company_name || 'Empresa'} — ${r.job?.job_title || 'Vaga'}</h4>
            <div class="meta">Gerado em ${fmtDate(r.created_at)}${r.job?.quality ? ` · qualidade: ${r.job.quality}` : ''}</div>
          </div>
          <div class="resume-card-actions">
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

  try {
    const res = await api.post(`/jobs/${jobId}/resumes/${resumeId}/pdf`);
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
  const url = document.getElementById('new-job-url').value.trim();
  if (!url) { toast('URL da vaga é obrigatória.', 'error'); return; }

  const companyName  = document.getElementById('new-job-company').value.trim()   || undefined;
  const jobTitle     = document.getElementById('new-job-title').value.trim()      || undefined;
  const description  = document.getElementById('new-job-desc').value.trim()       || undefined;
  const stackRaw     = document.getElementById('new-job-stack').value.trim();
  const language     = document.getElementById('new-job-lang').value             || undefined;

  const body = { url };
  if (companyName)     body.company_name = companyName;
  if (jobTitle)        body.job_title    = jobTitle;
  if (description)     body.description  = description;
  if (language)        body.language     = language;
  if (stackRaw)        body.stacks       = splitCsv(stackRaw);

  try {
    const btn = document.getElementById('btn-create-job');
    btn.disabled = true;
    const res = await api.post('/jobs', body);
    btn.disabled = false;

    if (res.status === 409) { toast('Esta URL já foi cadastrada.', 'error'); return; }
    if (!res.ok) { toast('Erro ao cadastrar vaga.', 'error'); return; }

    ['new-job-url','new-job-company','new-job-title','new-job-desc','new-job-stack'].forEach(id => {
      document.getElementById(id).value = '';
    });
    toast('Vaga cadastrada! O worker irá processá-la em breve.');
    navigateTo('vagas');
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
const _bulkUrls = []; // { url: string, language: string }[]

function _bulkRefresh() {
  const list  = document.getElementById('bulk-url-list');
  const empty = document.getElementById('bulk-url-empty');
  const btn   = document.getElementById('btn-bulk-submit');

  empty.style.display = _bulkUrls.length ? 'none' : 'block';
  btn.disabled        = _bulkUrls.length === 0;
  btn.textContent     = `Enviar ${_bulkUrls.length} vaga${_bulkUrls.length !== 1 ? 's' : ''}`;

  list.innerHTML = _bulkUrls.map((item, i) => `
    <div style="display:flex;align-items:center;gap:8px;background:var(--surface);border:1px solid var(--border);border-radius:6px;padding:7px 10px">
      <span style="font-size:11px;padding:2px 8px;border-radius:4px;background:var(--border);color:var(--muted);white-space:nowrap;flex-shrink:0">${escHtml(item.language.toUpperCase())}</span>
      <span style="flex:1;font-size:13px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--text)" title="${escHtml(item.url)}">${escHtml(item.url)}</span>
      <button onclick="removeBulkUrl(${i})" style="flex-shrink:0;background:none;border:none;cursor:pointer;color:var(--muted);font-size:15px;line-height:1;padding:0 2px" title="Remover">✕</button>
    </div>
  `).join('');
}

function addBulkUrl() {
  const input    = document.getElementById('bulk-url-input');
  const language = document.getElementById('bulk-lang').value;
  const url      = input.value.trim();
  if (!url) return;
  if (!url.startsWith('http')) { toast('URL inválida.', 'error'); return; }
  if (_bulkUrls.some(item => item.url === url)) { toast('URL já adicionada.', 'error'); return; }
  _bulkUrls.push({ url, language });
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
    _bulkUrls.map(({ url, language }) => api.post('/jobs', { url, language }))
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
