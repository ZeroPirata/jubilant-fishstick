// ── Profile ──

let _activePerfil = 'info';

function showPerfilTab(id, btn) {
  _activePerfil = id;
  document.querySelectorAll('.sub-section').forEach(s => s.classList.remove('active'));
  document.querySelectorAll('.sub-nav button').forEach(b => b.classList.remove('active'));
  document.getElementById('perf-' + id)?.classList.add('active');
  btn.classList.add('active');

  const loaders = {
    info:         loadProfileInfo,
    links:        loadProfileLinks,
    experiences:  loadExperiences,
    academic:     loadAcademic,
    skills:       loadSkills,
    projects:     loadProjects,
    certificates: loadCertificates,
  };
  loaders[id]?.();
}

function loadProfile() {
  const btn = document.querySelector(`.sub-nav button[data-perf="${_activePerfil}"]`);
  showPerfilTab(_activePerfil, btn || document.querySelector('.sub-nav button'));
}

// ── Helpers ──
const n2e = v => v ?? '';
const e2n = v => v.trim() || null;
const d2m = d => d ? String(d).substring(0, 7) : '';
const m2d = m => m ? m + '-01' : '';

// ── Pagination state ──
const _pg = {
  exp:  { offset: 0, size: 20 },
  ac:   { offset: 0, size: 20 },
  sk:   { offset: 0, size: 20 },
  proj: { offset: 0, size: 20 },
  cert: { offset: 0, size: 20 },
};
const _pgLoaders = {
  exp:  () => loadExperiences(),
  ac:   () => loadAcademic(),
  sk:   () => loadSkills(),
  proj: () => loadProjects(),
  cert: () => loadCertificates(),
};

function _pgQS(key) {
  const p = _pg[key];
  const search = (document.getElementById(key + '-search')?.value || '').trim();
  let qs = `?offset=${p.offset}&size=${p.size}`;
  if (search) qs += `&search=${encodeURIComponent(search)}`;
  return qs;
}

function renderPagination(key, total, cursor, size) {
  const from = total === 0 ? 0 : cursor + 1;
  const to   = Math.min(cursor + size, total);
  const info = document.getElementById(key + '-page-info');
  const prev = document.getElementById(key + '-prev');
  const next = document.getElementById(key + '-next');
  if (info) info.textContent = total === 0 ? '—' : `${from}–${to} de ${total}`;
  if (prev) prev.disabled = cursor === 0;
  if (next) next.disabled = cursor + size >= total;
}

function prevPage(key) {
  const p = _pg[key];
  if (p.offset === 0) return;
  p.offset = Math.max(0, p.offset - p.size);
  _pgLoaders[key]();
}

function nextPage(key) {
  _pg[key].offset += _pg[key].size;
  _pgLoaders[key]();
}

function changePageSize(key, val) {
  _pg[key].size   = parseInt(val, 10);
  _pg[key].offset = 0;
  _pgLoaders[key]();
}

const _searchTimers = {};
function debounceSearch(key) {
  clearTimeout(_searchTimers[key]);
  _searchTimers[key] = setTimeout(() => {
    _pg[key].offset = 0;
    _pgLoaders[key]();
  }, 350);
}
function escStr(s) {
  return String(s ?? '')
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

// ── Info Básica ──
async function loadProfileInfo() {
  try {
    const res = await api.get('/users/me');
    if (!res.ok) return;
    const d = await apiData(res);
    document.getElementById('info-nome').value          = n2e(d.full_name);
    document.getElementById('info-email').value         = n2e(d.email);
    document.getElementById('info-contact-email').value = n2e(d.contact_email);
    document.getElementById('info-telefone').value      = n2e(d.phone);
    document.getElementById('info-resumo').value        = n2e(d.about);
  } catch (_) {}
}

async function saveProfileInfo() {
  const full_name = document.getElementById('info-nome').value.trim();
  if (!full_name) { toast('Nome é obrigatório.', 'error'); return; }

  const phone         = e2n(document.getElementById('info-telefone').value);
  const about         = e2n(document.getElementById('info-resumo').value);
  const contact_email = e2n(document.getElementById('info-contact-email').value);

  try {
    const res = await api.put('/users/me/profile', { full_name, phone, about, contact_email });
    if (!res.ok) { toast('Erro ao salvar perfil.', 'error'); return; }
    toast('Perfil salvo!');
  } catch (e) { toast(e.message, 'error'); }
}

// ── Links ──
async function loadProfileLinks() {
  try {
    const res = await api.get('/users/me');
    if (!res.ok) return;
    const d = await apiData(res);
    document.getElementById('link-linkedin').value  = n2e(d.linkedin_url);
    document.getElementById('link-github').value    = n2e(d.github_url);
    document.getElementById('link-portfolio').value = n2e(d.portfolio_url);
  } catch (_) {}
}

async function saveLinks() {
  const body = {
    linkedin_url:  e2n(document.getElementById('link-linkedin').value),
    github_url:    e2n(document.getElementById('link-github').value),
    portfolio_url: e2n(document.getElementById('link-portfolio').value),
  };
  try {
    const res = await api.put('/users/me/links', body);
    if (!res.ok) { toast('Erro ao salvar links.', 'error'); return; }
    toast('Links salvos!');
  } catch (e) { toast(e.message, 'error'); }
}

// ── Experiências ──
let _expData = {}, _editingExpId = null;

async function loadExperiences() {
  const tbody = document.getElementById('exp-body');
  try {
    const res = await api.get('/users/me/experiences' + _pgQS('exp'));
    const { data: rows, meta } = await apiList(res);
    _expData = {};
    renderPagination('exp', meta.total ?? 0, meta.cursor ?? _pg.exp.offset, meta.size ?? _pg.exp.size);
    if (!rows.length) { tbody.innerHTML = '<tr><td colspan="7" class="empty">Nenhuma experiência.</td></tr>'; return; }
    rows.forEach(e => { _expData[e.id] = e; });
    tbody.innerHTML = rows.map(e => `
      <tr>
        <td>${escStr(e.company_name)}</td>
        <td>${escStr(e.job_role)}</td>
        <td style="color:var(--muted)">${d2m(e.start_date)} → ${e.is_current_job ? 'atual' : d2m(e.end_date)}</td>
        <td style="color:var(--muted)">${(e.tech_stack||[]).join(', ') || '—'}</td>
        <td style="color:var(--muted)">${(e.tags||[]).join(', ') || '—'}</td>
        <td>
          <button class="btn-edit" onclick="editExperience('${e.id}')">✎</button>
          <button class="btn-delete" onclick="deleteExperience('${e.id}',this)">✕</button>
        </td>
      </tr>
    `).join('');
  } catch (_) { tbody.innerHTML = '<tr><td colspan="7" class="empty">Erro ao carregar.</td></tr>'; }
}

function editExperience(id) {
  const e = _expData[id];
  if (!e) return;
  _editingExpId = id;
  document.getElementById('exp-empresa').value    = e.company_name || '';
  document.getElementById('exp-cargo').value      = e.job_role || '';
  document.getElementById('exp-descricao').value  = n2e(e.description);
  document.getElementById('exp-inicio').value     = d2m(e.start_date);
  document.getElementById('exp-fim').value        = d2m(e.end_date);
  document.getElementById('exp-atual').checked    = !!e.is_current_job;
  document.getElementById('exp-stack').value      = (e.tech_stack||[]).join(', ');
  document.getElementById('exp-conquistas').value = (e.achievements||[]).join(', ');
  document.getElementById('exp-tags').value       = (e.tags||[]).join(', ');
  document.getElementById('exp-submit-btn').textContent = 'Atualizar';
  document.getElementById('exp-cancel').classList.remove('hidden');
  document.getElementById('exp-empresa').scrollIntoView({ behavior: 'smooth' });
}

function cancelEditExperience() {
  _editingExpId = null;
  document.getElementById('exp-submit-btn').textContent = 'Adicionar';
  document.getElementById('exp-cancel').classList.add('hidden');
  ['exp-empresa','exp-cargo','exp-descricao','exp-inicio','exp-fim','exp-stack','exp-conquistas','exp-tags']
    .forEach(id => document.getElementById(id).value = '');
  document.getElementById('exp-atual').checked = false;
}

async function addExperience() {
  const body = {
    company_name:   document.getElementById('exp-empresa').value.trim(),
    job_role:       document.getElementById('exp-cargo').value.trim(),
    description:    document.getElementById('exp-descricao').value.trim(),
    is_current_job: document.getElementById('exp-atual').checked,
    start_date:     m2d(document.getElementById('exp-inicio').value.trim()),
    end_date:       m2d(document.getElementById('exp-fim').value.trim()),
    tech_stack:     splitCsv(document.getElementById('exp-stack').value),
    achievements:   splitCsv(document.getElementById('exp-conquistas').value),
    tags:           splitCsv(document.getElementById('exp-tags').value),
  };
  if (!body.company_name || !body.job_role || !body.start_date) {
    toast('Empresa, cargo e data início são obrigatórios.', 'error'); return;
  }
  try {
    const res = _editingExpId
      ? await api.put(`/users/me/experiences/${_editingExpId}`, body)
      : await api.post('/users/me/experiences', body);
    if (!res.ok) { toast('Erro ao salvar experiência.', 'error'); return; }
    cancelEditExperience();
    toast(_editingExpId ? 'Experiência atualizada!' : 'Experiência adicionada!');
    loadExperiences();
  } catch (e) { toast(e.message, 'error'); }
}

async function deleteExperience(id, btn) {
  if (!await confirmDialog('Deletar experiência?')) return;
  btn.disabled = true;
  try {
    const res = await api.delete(`/users/me/experiences/${id}`);
    if (!res.ok) { toast('Erro ao deletar.', 'error'); btn.disabled = false; return; }
    btn.closest('tr').remove();
    delete _expData[id];
    toast('Experiência removida.');
  } catch (e) { toast(e.message, 'error'); btn.disabled = false; }
}

// ── Formação Acadêmica ──
let _acData = {}, _editingAcId = null;

async function loadAcademic() {
  const tbody = document.getElementById('academic-body');
  try {
    const res = await api.get('/users/me/academic' + _pgQS('ac'));
    const { data: rows, meta } = await apiList(res);
    _acData = {};
    renderPagination('ac', meta.total ?? 0, meta.cursor ?? _pg.ac.offset, meta.size ?? _pg.ac.size);
    if (!rows.length) { tbody.innerHTML = '<tr><td colspan="4" class="empty">Nenhuma formação.</td></tr>'; return; }
    rows.forEach(a => { _acData[a.id] = a; });
    tbody.innerHTML = rows.map(a => `
      <tr>
        <td>${escStr(a.institution_name)}</td>
        <td>${escStr(a.course_name)}</td>
        <td style="color:var(--muted)">${d2m(a.start_date)} → ${d2m(a.end_date)}</td>
        <td>
          <button class="btn-edit" onclick="editAcademic('${a.id}')">✎</button>
          <button class="btn-delete" onclick="deleteAcademic('${a.id}',this)">✕</button>
        </td>
      </tr>
    `).join('');
  } catch (_) { tbody.innerHTML = '<tr><td colspan="4" class="empty">Erro ao carregar.</td></tr>'; }
}

function editAcademic(id) {
  const a = _acData[id];
  if (!a) return;
  _editingAcId = id;
  document.getElementById('ac-instituicao').value = a.institution_name || '';
  document.getElementById('ac-curso').value       = a.course_name || '';
  document.getElementById('ac-inicio').value      = d2m(a.start_date);
  document.getElementById('ac-fim').value         = d2m(a.end_date);
  document.getElementById('ac-submit-btn').textContent = 'Atualizar';
  document.getElementById('ac-cancel').classList.remove('hidden');
  document.getElementById('ac-instituicao').scrollIntoView({ behavior: 'smooth' });
}

function cancelEditAcademic() {
  _editingAcId = null;
  document.getElementById('ac-submit-btn').textContent = 'Adicionar';
  document.getElementById('ac-cancel').classList.add('hidden');
  ['ac-instituicao','ac-curso','ac-inicio','ac-fim'].forEach(id => document.getElementById(id).value = '');
}

async function addAcademic() {
  const body = {
    institution_name: document.getElementById('ac-instituicao').value.trim(),
    course_name:      document.getElementById('ac-curso').value.trim(),
    start_date:       m2d(document.getElementById('ac-inicio').value.trim()),
    end_date:         m2d(document.getElementById('ac-fim').value.trim()),
  };
  if (!body.institution_name || !body.course_name || !body.start_date) {
    toast('Instituição, curso e data início são obrigatórios.', 'error'); return;
  }
  try {
    const res = _editingAcId
      ? await api.put(`/users/me/academic/${_editingAcId}`, body)
      : await api.post('/users/me/academic', body);
    if (!res.ok) { toast('Erro ao salvar formação.', 'error'); return; }
    cancelEditAcademic();
    toast(_editingAcId ? 'Formação atualizada!' : 'Formação adicionada!');
    loadAcademic();
  } catch (e) { toast(e.message, 'error'); }
}

async function deleteAcademic(id, btn) {
  if (!await confirmDialog('Deletar formação?')) return;
  btn.disabled = true;
  try {
    const res = await api.delete(`/users/me/academic/${id}`);
    if (!res.ok) { toast('Erro ao deletar.', 'error'); btn.disabled = false; return; }
    btn.closest('tr').remove();
    delete _acData[id];
    toast('Formação removida.');
  } catch (e) { toast(e.message, 'error'); btn.disabled = false; }
}

// ── Habilidades ──
let _skData = {}, _editingSkId = null;

async function loadSkills() {
  const tbody = document.getElementById('skills-body');
  try {
    const res = await api.get('/users/me/skills' + _pgQS('sk'));
    const { data: rows, meta } = await apiList(res);
    _skData = {};
    renderPagination('sk', meta.total ?? 0, meta.cursor ?? _pg.sk.offset, meta.size ?? _pg.sk.size);
    if (!rows.length) { tbody.innerHTML = '<tr><td colspan="5" class="empty">Nenhuma habilidade.</td></tr>'; return; }
    rows.forEach(s => { _skData[s.id] = s; });
    tbody.innerHTML = rows.map(s => `
      <tr>
        <td>${escStr(s.skill_name)}</td>
        <td><span class="badge badge-${s.proficiency_level}">${s.proficiency_level}</span></td>
        <td style="color:var(--muted)">${(s.tags||[]).join(', ') || '—'}</td>
        <td>
          <button class="btn-edit" onclick="editSkill('${s.id}')">✎</button>
          <button class="btn-delete" onclick="deleteSkill('${s.id}',this)">✕</button>
        </td>
      </tr>
    `).join('');
  } catch (_) { tbody.innerHTML = '<tr><td colspan="5" class="empty">Erro ao carregar.</td></tr>'; }
}

function editSkill(id) {
  const s = _skData[id];
  if (!s) return;
  _editingSkId = id;
  document.getElementById('sk-nome').value  = s.skill_name || '';
  document.getElementById('sk-nivel').value = s.proficiency_level || 'advanced';
  document.getElementById('sk-tags').value  = (s.tags||[]).join(', ');
  document.getElementById('sk-submit-btn').textContent = 'Atualizar';
  document.getElementById('sk-cancel').classList.remove('hidden');
  document.getElementById('sk-nome').scrollIntoView({ behavior: 'smooth' });
}

function cancelEditSkill() {
  _editingSkId = null;
  document.getElementById('sk-submit-btn').textContent = 'Adicionar';
  document.getElementById('sk-cancel').classList.add('hidden');
  document.getElementById('sk-nome').value = '';
  document.getElementById('sk-tags').value = '';
  document.getElementById('sk-nivel').value = 'advanced';
}

async function addSkill() {
  const body = {
    skill_name:        document.getElementById('sk-nome').value.trim(),
    proficiency_level: document.getElementById('sk-nivel').value,
    tags:              splitCsv(document.getElementById('sk-tags').value),
  };
  if (!body.skill_name) { toast('Nome é obrigatório.', 'error'); return; }
  try {
    const res = _editingSkId
      ? await api.put(`/users/me/skills/${_editingSkId}`, body)
      : await api.post('/users/me/skills', body);
    if (!res.ok) { toast('Erro ao salvar habilidade.', 'error'); return; }
    cancelEditSkill();
    if (typeof clearSkillsCache === 'function') clearSkillsCache();
    toast(_editingSkId ? 'Habilidade atualizada!' : 'Habilidade adicionada!');
    loadSkills();
  } catch (e) { toast(e.message, 'error'); }
}

async function deleteSkill(id, btn) {
  if (!await confirmDialog('Deletar habilidade?')) return;
  btn.disabled = true;
  try {
    const res = await api.delete(`/users/me/skills/${id}`);
    if (!res.ok) { toast('Erro ao deletar.', 'error'); btn.disabled = false; return; }
    btn.closest('tr').remove();
    delete _skData[id];
    if (typeof clearSkillsCache === 'function') clearSkillsCache();
    toast('Habilidade removida.');
  } catch (e) { toast(e.message, 'error'); btn.disabled = false; }
}

// ── Projetos ──
let _projData = {}, _editingProjId = null;

async function loadProjects() {
  const tbody = document.getElementById('proj-body');
  try {
    const res = await api.get('/users/me/projects' + _pgQS('proj'));
    const { data: rows, meta } = await apiList(res);
    _projData = {};
    renderPagination('proj', meta.total ?? 0, meta.cursor ?? _pg.proj.offset, meta.size ?? _pg.proj.size);
    if (!rows.length) { tbody.innerHTML = '<tr><td colspan="6" class="empty">Nenhum projeto.</td></tr>'; return; }
    rows.forEach(p => { _projData[p.id] = p; });
    tbody.innerHTML = rows.map(p => `
      <tr>
        <td>${escStr(p.project_name)}</td>
        <td>${p.project_url ? `<a class="link" href="${escStr(p.project_url)}" target="_blank">↗</a>` : '—'}</td>
        <td style="color:var(--muted)">${d2m(p.start_date)} → ${d2m(p.end_date)}</td>
        <td style="color:var(--muted)">${(p.tags||[]).join(', ') || '—'}</td>
        <td>
          <button class="btn-edit" onclick="editProject('${p.id}')">✎</button>
          <button class="btn-delete" onclick="deleteProject('${p.id}',this)">✕</button>
        </td>
      </tr>
    `).join('');
  } catch (_) { tbody.innerHTML = '<tr><td colspan="6" class="empty">Erro ao carregar.</td></tr>'; }
}

function editProject(id) {
  const p = _projData[id];
  if (!p) return;
  _editingProjId = id;
  document.getElementById('proj-nome').value     = p.project_name || '';
  document.getElementById('proj-desc').value     = p.description || '';
  document.getElementById('proj-link').value     = n2e(p.project_url);
  document.getElementById('proj-tags').value     = (p.tags||[]).join(', ');
  document.getElementById('proj-inicio').value   = d2m(p.start_date);
  document.getElementById('proj-fim').value      = d2m(p.end_date);
  document.getElementById('proj-academico').checked = !!p.is_academic;
  document.getElementById('proj-submit-btn').textContent = 'Atualizar';
  document.getElementById('proj-cancel').classList.remove('hidden');
  document.getElementById('proj-nome').scrollIntoView({ behavior: 'smooth' });
}

function cancelEditProject() {
  _editingProjId = null;
  document.getElementById('proj-submit-btn').textContent = 'Adicionar';
  document.getElementById('proj-cancel').classList.add('hidden');
  ['proj-nome','proj-desc','proj-link','proj-tags','proj-inicio','proj-fim'].forEach(id => document.getElementById(id).value = '');
  document.getElementById('proj-academico').checked = false;
}

async function addProject() {
  const body = {
    project_name: document.getElementById('proj-nome').value.trim(),
    description:  document.getElementById('proj-desc').value.trim(),
    project_url:  document.getElementById('proj-link').value.trim(),
    tags:         splitCsv(document.getElementById('proj-tags').value),
    start_date:   m2d(document.getElementById('proj-inicio').value.trim()),
    end_date:     m2d(document.getElementById('proj-fim').value.trim()),
    is_academic:  document.getElementById('proj-academico').checked,
  };
  if (!body.project_name || !body.description || !body.start_date) {
    toast('Nome, descrição e data início são obrigatórios.', 'error'); return;
  }
  try {
    const res = _editingProjId
      ? await api.put(`/users/me/projects/${_editingProjId}`, body)
      : await api.post('/users/me/projects', body);
    if (!res.ok) { toast('Erro ao salvar projeto.', 'error'); return; }
    cancelEditProject();
    toast(_editingProjId ? 'Projeto atualizado!' : 'Projeto adicionado!');
    loadProjects();
  } catch (e) { toast(e.message, 'error'); }
}

async function deleteProject(id, btn) {
  if (!await confirmDialog('Deletar projeto?')) return;
  btn.disabled = true;
  try {
    const res = await api.delete(`/users/me/projects/${id}`);
    if (!res.ok) { toast('Erro ao deletar.', 'error'); btn.disabled = false; return; }
    btn.closest('tr').remove();
    delete _projData[id];
    toast('Projeto removido.');
  } catch (e) { toast(e.message, 'error'); btn.disabled = false; }
}

// ── Certificados ──
let _certData = {}, _editingCertId = null;

async function loadCertificates() {
  const tbody = document.getElementById('cert-body');
  try {
    const res = await api.get('/users/me/certificates' + _pgQS('cert'));
    const { data: rows, meta } = await apiList(res);
    _certData = {};
    renderPagination('cert', meta.total ?? 0, meta.cursor ?? _pg.cert.offset, meta.size ?? _pg.cert.size);
    if (!rows.length) { tbody.innerHTML = '<tr><td colspan="6" class="empty">Nenhum certificado.</td></tr>'; return; }
    rows.forEach(c => { _certData[c.id] = c; });
    tbody.innerHTML = rows.map(c => `
      <tr>
        <td>${escStr(c.certificate_name)}</td>
        <td style="color:var(--muted)">${escStr(c.issuing_organization)}</td>
        <td style="color:var(--muted)">${d2m(c.issue_date)}</td>
        <td>${c.credential_url ? `<a class="link" href="${escStr(c.credential_url)}" target="_blank">↗</a>` : '—'}</td>
        <td>
          <button class="btn-edit" onclick="editCertificate('${c.id}')">✎</button>
          <button class="btn-delete" onclick="deleteCertificate('${c.id}',this)">✕</button>
        </td>
      </tr>
    `).join('');
  } catch (_) { tbody.innerHTML = '<tr><td colspan="6" class="empty">Erro ao carregar.</td></tr>'; }
}

function editCertificate(id) {
  const c = _certData[id];
  if (!c) return;
  _editingCertId = id;
  document.getElementById('cert-nome').value    = c.certificate_name || '';
  document.getElementById('cert-emissor').value = c.issuing_organization || '';
  document.getElementById('cert-data').value    = d2m(c.issue_date);
  document.getElementById('cert-link').value    = n2e(c.credential_url);
  document.getElementById('cert-tags').value    = (c.tags||[]).join(', ');
  document.getElementById('cert-submit-btn').textContent = 'Atualizar';
  document.getElementById('cert-cancel').classList.remove('hidden');
  document.getElementById('cert-nome').scrollIntoView({ behavior: 'smooth' });
}

function cancelEditCertificate() {
  _editingCertId = null;
  document.getElementById('cert-submit-btn').textContent = 'Adicionar';
  document.getElementById('cert-cancel').classList.add('hidden');
  ['cert-nome','cert-emissor','cert-data','cert-link','cert-tags'].forEach(id => document.getElementById(id).value = '');
}

async function addCertificate() {
  const body = {
    certificate_name:     document.getElementById('cert-nome').value.trim(),
    issuing_organization: document.getElementById('cert-emissor').value.trim(),
    issue_date:           m2d(document.getElementById('cert-data').value.trim()),
    credential_url:       document.getElementById('cert-link').value.trim(),
    tags:                 splitCsv(document.getElementById('cert-tags').value),
  };
  if (!body.certificate_name || !body.issuing_organization || !body.issue_date) {
    toast('Nome, emissor e data são obrigatórios.', 'error'); return;
  }
  try {
    const res = _editingCertId
      ? await api.put(`/users/me/certificates/${_editingCertId}`, body)
      : await api.post('/users/me/certificates', body);
    if (!res.ok) { toast('Erro ao salvar certificado.', 'error'); return; }
    cancelEditCertificate();
    toast(_editingCertId ? 'Certificado atualizado!' : 'Certificado adicionado!');
    loadCertificates();
  } catch (e) { toast(e.message, 'error'); }
}

async function deleteCertificate(id, btn) {
  if (!await confirmDialog('Deletar certificado?')) return;
  btn.disabled = true;
  try {
    const res = await api.delete(`/users/me/certificates/${id}`);
    if (!res.ok) { toast('Erro ao deletar.', 'error'); btn.disabled = false; return; }
    btn.closest('tr').remove();
    delete _certData[id];
    toast('Certificado removido.');
  } catch (e) { toast(e.message, 'error'); btn.disabled = false; }
}
