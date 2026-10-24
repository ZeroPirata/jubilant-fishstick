// ── Importar currículo ──
// Envio do arquivo → a IA lê em segundo plano → tela "Dados compreendidos pelo seu
// currículo" → a pessoa confere/ajusta → só então salva no perfil.

const IMP_KEY = 'jp_import';
const IMP_GIVE_UP_MS = 9 * 60 * 1000; // > tempo máximo da leitura no servidor (7 min + folga)
const IMP_LEVELS = [
  ['basic', 'Básico'], ['intermediate', 'Intermediário'], ['advanced', 'Avançado'], ['expert', 'Expert'],
];
const IMP_LEVEL_LABEL = Object.fromEntries(IMP_LEVELS);

// phase: idle | processing | ready (rascunho esperando conferência) | error — alimenta o
// indicador da barra superior, visível quando a pessoa está em outra tela.
const _imp = { id: null, draft: null, current: null, poll: null, clock: null, started: 0, phase: 'idle' };

function impShow(stage) {
  for (const s of ['upload', 'processing', 'review']) {
    document.getElementById('imp-' + s).classList.toggle('hidden', s !== stage);
  }
}

function impError(msg, attempts) {
  const el = document.getElementById('imp-error');
  document.getElementById('imp-error-msg').textContent = msg || '';
  document.getElementById('imp-error-attempts').innerHTML = impAttemptsHTML(attempts || []);
  el.classList.toggle('hidden', !msg);
}

// Tentativas da leitura, sem o nome dos modelos (isso fica no painel admin):
// "1ª tentativa · serviço ocupado", "2ª tentativa · lendo agora · 1:12".
const IMP_ATTEMPT_LABELS = {
  ok: 'leu o currículo', rate_limit: 'serviço ocupado', timeout: 'demorou demais',
};
const impSecs = s => s < 60 ? `${Math.round(s)}s` : `${Math.floor(s / 60)}:${String(Math.round(s % 60)).padStart(2, '0')}`;

function impAttemptsHTML(attempts, trying, tryingSince) {
  const rows = attempts.map((a, i) => `
    <li class="${a.status === 'ok' ? 'is-ok' : 'is-fail'}"><span class="imp-att-icon">${a.status === 'ok' ? '✓' : '✕'}</span>
      <b>${i + 1}ª tentativa</b><span>${IMP_ATTEMPT_LABELS[a.status] || 'não conseguiu ler'} · ${impSecs(a.seconds)}</span></li>`);
  if (trying) {
    const since = tryingSince ? (Date.now() - new Date(tryingSince)) / 1000 : 0;
    rows.push(`<li class="is-now"><span class="imp-att-icon"></span><b>${attempts.length + 1}ª tentativa</b><span>lendo agora${since > 0 ? ' · ' + impSecs(since) : ''}</span></li>`);
  }
  return rows.join('');
}

function impSaved() {
  try { return JSON.parse(sessionStorage.getItem(IMP_KEY) || 'null'); } catch (_) { return null; }
}

function impForget() {
  sessionStorage.removeItem(IMP_KEY);
  clearTimeout(_imp.poll);
  clearInterval(_imp.clock);
  Object.assign(_imp, { id: null, draft: null, poll: null, clock: null, phase: 'idle' });
  impSyncPill();
}

// Ao entrar no app: se havia uma leitura em andamento (página recarregada), volta a
// acompanhar em segundo plano, sem precisar abrir a aba de importação.
function impResume() {
  const saved = impSaved();
  if (!saved?.id || _imp.id) return;
  _imp.id = saved.id;
  impStartWaiting(saved.name, saved.started);
  impPoll();
}

// ── Indicador fora da aba + aviso quando termina ──

const impViewingImport = () => _activeTab === 'perfil' && _activePerfil === 'import';

function impSyncPill() {
  const pill = document.getElementById('imp-pill');
  if (!pill) return;
  // Erro visto = resolvido. Rascunho pronto continua sinalizado até salvar ou descartar.
  if (impViewingImport() && _imp.phase === 'error') _imp.phase = 'idle';
  const nav = document.querySelector('nav button[data-tab="perfil"]');
  nav?.classList.toggle('nav-busy', _imp.phase === 'processing');
  nav?.classList.toggle('nav-ready', _imp.phase === 'ready');

  const show = !impViewingImport() && ['processing', 'ready', 'error'].includes(_imp.phase);
  pill.classList.toggle('hidden', !show);
  if (!show) return;
  pill.className = 'imp-pill is-' + _imp.phase;
  if (_imp.phase === 'processing') {
    const s = Math.max(0, Math.floor((Date.now() - _imp.started) / 1000));
    pill.innerHTML = `<span class="imp-pill-scan" aria-hidden="true"></span>⇪ lendo currículo · <b>${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}</b>`;
    pill.title = 'Clique para acompanhar';
  } else if (_imp.phase === 'ready') {
    pill.innerHTML = '✓ currículo pronto para conferir <b>→</b>';
    pill.title = 'Abrir a conferência';
  } else {
    pill.innerHTML = '! a leitura do currículo falhou <b>→</b>';
    pill.title = 'Ver o motivo';
  }
}

function impGoToImport() {
  _activePerfil = 'import';
  if (_activeTab === 'perfil') showPerfilTab('import', document.querySelector('.sub-nav button[data-perf="import"]'));
  else navigateTo('perfil');
}

// O navegador só deixa tocar som depois de um gesto da pessoa: o envio do arquivo destrava.
let _impAudio = null;
function impUnlockAudio() {
  try {
    _impAudio ??= new (window.AudioContext || window.webkitAudioContext)();
    if (_impAudio.state === 'suspended') _impAudio.resume();
  } catch (_) {}
}

function impBeep(ok) {
  try {
    impUnlockAudio();
    const ctx = _impAudio, t = ctx.currentTime;
    // Dois tons curtos e baixos (subindo = pronto, descendo = falhou).
    (ok ? [660, 880] : [520, 390]).forEach((f, i) => {
      const o = ctx.createOscillator(), g = ctx.createGain();
      o.type = 'sine'; o.frequency.value = f;
      g.gain.setValueAtTime(0.0001, t + i * 0.16);
      g.gain.exponentialRampToValueAtTime(0.08, t + i * 0.16 + 0.02);
      g.gain.exponentialRampToValueAtTime(0.0001, t + i * 0.16 + 0.14);
      o.connect(g).connect(ctx.destination);
      o.start(t + i * 0.16); o.stop(t + i * 0.16 + 0.15);
    });
  } catch (_) {}
}

const _impTitle = document.title;
function impFlashTitle(text) {
  if (!document.hidden) return;
  document.title = `● ${text} · ${_impTitle}`;
  document.addEventListener('visibilitychange', function back() {
    if (document.hidden) return;
    document.title = _impTitle;
    document.removeEventListener('visibilitychange', back);
  });
}

// Avisa só quem não está olhando a importação (quem está vê a tela mudar sozinha).
function impNotify(ok, msg) {
  impSyncPill();
  if (impViewingImport() && !document.hidden) return;
  impBeep(ok);
  impFlashTitle(ok ? 'Currículo pronto' : 'Falha ao ler currículo');
  toast(msg, ok ? 'success' : 'error');
  const el = document.getElementById('toast');
  el.classList.add('toast-action');
  el.onclick = () => { el.classList.add('hidden'); impGoToImport(); };
  clearTimeout(_toastTimer);
  _toastTimer = setTimeout(() => { el.classList.add('hidden'); el.classList.remove('toast-action'); el.onclick = null; }, 9000);
}

// Ao abrir a aba: retoma uma leitura em andamento (ou um rascunho ainda não salvo).
function loadImport() {
  impBindDrop();
  if (_imp.draft) { impShow('review'); impSyncPill(); return; }
  const saved = impSaved();
  if (saved?.id) {
    _imp.id = saved.id;
    impStartWaiting(saved.name, saved.started);
    impPoll();
    return;
  }
  impShow('upload');
}

function impBindDrop() {
  const drop = document.getElementById('imp-drop');
  if (drop.dataset.bound) return;
  drop.dataset.bound = '1';
  ['dragenter', 'dragover'].forEach(ev => drop.addEventListener(ev, e => { e.preventDefault(); drop.classList.add('over'); }));
  ['dragleave', 'drop'].forEach(ev => drop.addEventListener(ev, e => { e.preventDefault(); drop.classList.remove('over'); }));
  drop.addEventListener('drop', e => impUpload(e.dataTransfer.files[0]));
}

async function impUpload(file) {
  if (!file) return;
  impError('');
  if (file.size > 5 * 1024 * 1024) { impError('O arquivo tem mais de 5 MB. Exporte o currículo sem imagens ou em outro formato.'); return; }

  impUnlockAudio();
  impStartWaiting(file.name, Date.now());
  let res, json = {};
  try {
    res = await fetch(API_BASE + '/users/me/import', {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${getToken()}`, 'Content-Type': file.type || 'application/octet-stream' },
      body: file,
    });
    json = await res.json().catch(() => ({}));
  } catch (_) {
    impStopWaiting();
    impError('Não consegui enviar o arquivo. Confira sua conexão e tente de novo.');
    return;
  }
  if (res.status === 401) { clearToken(); showAuth?.(); return; }
  if (!res.ok) {
    impStopWaiting();
    impError(json.error || 'Não consegui ler esse arquivo.');
    return;
  }
  _imp.id = json.data.id;
  sessionStorage.setItem(IMP_KEY, JSON.stringify({ id: _imp.id, name: file.name, started: _imp.started }));
  impPoll();
}

function impStartWaiting(name, started) {
  _imp.started = started || Date.now();
  document.getElementById('imp-attempts').innerHTML = '';
  _imp.phase = 'processing';
  document.getElementById('imp-file-name').textContent = name || 'currículo';
  impShow('processing');
  clearInterval(_imp.clock);
  const tick = () => {
    const s = Math.max(0, Math.floor((Date.now() - _imp.started) / 1000));
    document.getElementById('imp-elapsed').textContent = `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
    document.getElementById('imp-working-msg').textContent = s < 20
      ? 'Extraindo experiências, formação e habilidades…'
      : s < 70 ? 'Organizando cargos, datas e tecnologias…' : 'O modelo principal está ocupado; usando o próximo da fila…';
    impSyncPill();
  };
  tick();
  _imp.clock = setInterval(tick, 1000);
}

function impStopWaiting() {
  clearInterval(_imp.clock);
  _imp.phase = 'idle';
  impShow('upload');
  impSyncPill();
}

async function impPoll() {
  clearTimeout(_imp.poll);
  let res;
  try { res = await api.get(`/users/me/import/${_imp.id}`); }
  catch (_) { _imp.poll = setTimeout(impPoll, 4000); return; }

  if (res.status === 404) {
    impForget(); impStopWaiting();
    impError('A leitura expirou. Envie o arquivo de novo.');
    return;
  }
  if (!res.ok) { _imp.poll = setTimeout(impPoll, 4000); return; }

  const st = await apiData(res);
  if (st.status === 'processing') {
    document.getElementById('imp-attempts').innerHTML = impAttemptsHTML(st.attempts || [], st.trying, st.trying_since);
    // Trava do navegador: o servidor sempre encerra antes disso; se não encerrou, algo morreu.
    if (Date.now() - _imp.started > IMP_GIVE_UP_MS) {
      api.delete(`/users/me/import/${_imp.id}`).catch(() => {});
      impForget(); impShow('upload');
      impError('A leitura não respondeu a tempo e foi cancelada. Envie o arquivo de novo.', st.attempts);
      return;
    }
    _imp.poll = setTimeout(impPoll, 2500);
    return;
  }

  clearInterval(_imp.clock);
  if (st.status === 'error') {
    impForget(); impShow('upload');
    impError(st.error || 'Não consegui entender o currículo.', st.attempts);
    _imp.phase = 'error';
    impNotify(false, 'Não consegui ler o currículo. Clique para ver o motivo.');
    return;
  }
  _imp.draft = st.draft;
  await impRenderReview();
  _imp.phase = 'ready';
  impNotify(true, 'Currículo lido! Clique aqui para conferir os dados.');
}

// ── Boas-vindas: perfil vazio abre direto na importação ──
// Calculado no servidor a cada login (profile_empty no /me), não um flag de "usuário novo":
// quem preencheu à mão deixa de ver; quem apagou tudo volta a ver. "Pular" fica neste navegador.
const IMP_SKIP_KEY = 'jp_import_skip';

function impWelcome() {
  const skipped = (() => { try { return localStorage.getItem(IMP_SKIP_KEY) === '1'; } catch (_) { return false; } })();
  if (skipped) return;
  document.getElementById('imp-welcome').classList.remove('hidden');
  document.getElementById('imp-skip').classList.remove('hidden');
  document.querySelectorAll('.imp-default-head').forEach(el => el.classList.add('hidden'));
  // Só redireciona se a pessoa ainda está na tela inicial (o /me responde depois do primeiro render).
  if (_activeTab !== 'vagas') return;
  _activePerfil = 'import';
  navigateTo('perfil');
}

function impEndWelcome() {
  document.getElementById('imp-welcome').classList.add('hidden');
  document.getElementById('imp-skip').classList.add('hidden');
  document.querySelectorAll('.imp-default-head').forEach(el => el.classList.remove('hidden'));
}

function impSkipWelcome() {
  try { localStorage.setItem(IMP_SKIP_KEY, '1'); } catch (_) {}
  impEndWelcome();
  showPerfilTab('info', document.querySelector('.sub-nav button[data-perf="info"]'));
}

// ── Tela de conferência ──

const impMonth = m => {
  if (!m) return '';
  const [y, mm] = m.split('-');
  return `${mm}/${y}`;
};

const impPeriod = (start, end, current) =>
  `${impMonth(start) || '?'} – ${current ? 'atual' : (impMonth(end) || '?')}`;

function impUsage(s) {
  const parts = [];
  if (s.explicit) parts.push('nível escrito no currículo');
  if (s.months > 0) {
    const y = Math.round(s.months / 6) / 2;
    const dur = s.months < 12 ? `${s.months} ${s.months === 1 ? 'mês' : 'meses'}` : `~${String(y).replace('.', ',')} ${y === 1 ? 'ano' : 'anos'}`;
    parts.push(`${dur} em ${(s.used_in || []).join(', ')}`);
  } else if (!s.explicit) {
    parts.push('listada no currículo, sem uso nas experiências');
  }
  return parts.join(' · ');
}

async function impRenderReview() {
  const d = _imp.draft;
  try {
    const res = await api.get('/users/me');
    _imp.current = res.ok ? await apiData(res) : {};
  } catch (_) { _imp.current = {}; }
  const cur = _imp.current || {};

  const profileRows = [
    ['full_name', 'Nome completo', d.profile.full_name, pgStr(cur.full_name)],
    ['contact_email', 'Email de contato', d.profile.contact_email, pgStr(cur.contact_email)],
    ['phone', 'Telefone', d.profile.phone, pgStr(cur.phone)],
    ['about', 'Resumo / About', d.profile.about, pgStr(cur.about), true],
  ];
  const linkRows = [
    ['linkedin_url', 'LinkedIn', d.links.linkedin_url, pgStr(cur.linkedin_url)],
    ['github_url', 'GitHub', d.links.github_url, pgStr(cur.github_url)],
    ['portfolio_url', 'Portfolio', d.links.portfolio_url, pgStr(cur.portfolio_url)],
  ];

  const el = document.getElementById('imp-review');
  el.innerHTML = `
    <div class="imp-review-head">
      <div>
        <div class="card-eyebrow">${d.source === 'linkedin' ? 'PDF do LinkedIn' : 'Currículo'} · ainda não salvo</div>
        <h3 class="imp-title">Dados compreendidos pelo seu currículo</h3>
        <p class="imp-lead">Confira o que a IA entendeu. Ajuste o que estiver errado, escolha o seu nível em cada habilidade e desmarque o que não quiser no perfil. Itens que você já tem no perfil vêm desmarcados.</p>
      </div>
    </div>

    ${impFieldsBlock('profile', 'Info básica', profileRows)}
    ${impFieldsBlock('links', 'Links', linkRows)}
    ${impSkillsBlock(d.skills)}
    ${impLanguagesBlock(d.languages || [])}
    ${impListBlock('experiences', 'Experiências', d.experiences, impExperienceItem)}
    ${impListBlock('academic', 'Formação', d.academic, impAcademicItem)}
    ${impListBlock('projects', 'Projetos', d.projects, impProjectItem)}
    ${impListBlock('certificates', 'Certificados', d.certificates, impCertificateItem)}

    <div class="imp-footer">
      <span id="imp-summary" class="imp-summary"></span>
      <div class="form-actions">
        <button class="btn btn-ghost" onclick="impDiscard()">Descartar</button>
        <button class="btn btn-primary" id="imp-save" onclick="impApply()">Salvar no perfil</button>
      </div>
    </div>`;

  if (!el.dataset.bound) {
    el.dataset.bound = '1';
    el.addEventListener('input', impUpdateSummary);
    el.addEventListener('change', impUpdateSummary);
  }
  impShow('review');
  impUpdateSummary();
}

function impFieldsBlock(sec, title, rows) {
  const visible = rows.filter(r => r[2]);
  if (!visible.length) return '';
  return `
    <div class="form-card imp-block" data-sec="${sec}">
      <div class="imp-block-head"><h4>${title}</h4></div>
      <div class="imp-fields">
        ${visible.map(([f, label, found, current, long]) => {
          const same = current && current.trim() === found.trim();
          const checked = !current; // substituir o que a pessoa já escreveu é opt-in
          const input = long
            ? `<textarea data-f="${f}" rows="4">${escStr(found)}</textarea>`
            : `<input type="text" data-f="${f}" value="${escStr(found)}" />`;
          return `
          <div class="imp-field ${same ? 'is-same' : ''}" data-row="${f}">
            <label class="imp-check"><input type="checkbox" ${checked ? 'checked' : ''} ${same ? 'disabled' : ''} /><span>${label}</span></label>
            <div class="imp-field-body">
              ${input}
              <div class="imp-field-cur">${same ? 'igual ao que já está no perfil'
                : current ? `no perfil hoje: <span>${escStr(current.length > 90 ? current.slice(0, 90) + '…' : current)}</span> · marque para substituir`
                : 'vazio no perfil hoje'}</div>
            </div>
          </div>`;
        }).join('')}
      </div>
    </div>`;
}

function impSkillsBlock(skills) {
  return `
    <div class="form-card imp-block imp-skills" data-sec="skills">
      <div class="imp-block-head">
        <h4>Habilidades <span class="imp-count" data-count="skills"></span></h4>
        <div class="imp-bulk">
          <button class="btn-more" onclick="impBulk('skills', true)">marcar todas</button>
          <button class="btn-more" onclick="impBulk('skills', false)">desmarcar todas</button>
        </div>
      </div>
      <p class="card-note">O nível vem sugerido pelo tempo de uso nas suas experiências (ou pelo que está escrito no currículo). Clique no nível certo para você.</p>
      <div class="imp-skill-list" id="imp-skill-list">
        ${skills.map(impSkillRow).join('')}
      </div>
      <div class="imp-skill-add">
        <input type="text" id="imp-skill-new" placeholder="Faltou alguma? Digite e pressione Enter" onkeydown="if(event.key==='Enter'){event.preventDefault();impAddSkill()}" />
        <button class="btn btn-ghost btn-sm" onclick="impAddSkill()">+ Adicionar</button>
      </div>
    </div>`;
}

function impSkillRow(s) {
  const exists = !!s.existing_id;
  const level = exists && !s.explicit ? s.existing_level : s.proficiency_level;
  const hint = impUsage(s);
  return `
    <div class="imp-skill" data-item data-level="${level}" data-exists="${exists ? 1 : ''}" data-orig="${exists ? s.existing_level : ''}">
      <input type="checkbox" class="imp-skill-check" ${exists ? '' : 'checked'} aria-label="Incluir ${escStr(s.skill_name)}" />
      <div class="imp-skill-main">
        <input type="text" class="imp-skill-name" data-f="skill_name" value="${escStr(s.skill_name)}" aria-label="Nome da habilidade" />
        <div class="imp-skill-hint">${escStr(hint)}${exists ? ` <span class="imp-tag">no perfil: ${IMP_LEVEL_LABEL[s.existing_level] || s.existing_level}</span>` : ''}</div>
      </div>
      <div class="imp-levels" role="radiogroup" aria-label="Nível">
        ${IMP_LEVELS.map(([v, l]) => `<button type="button" role="radio" aria-checked="${v === level}" class="lvl-${v} ${v === level ? 'on' : ''}" onclick="impSetLevel(this,'${v}')">${l}</button>`).join('')}
      </div>
    </div>`;
}

// Idiomas: mesma linha das habilidades (marcar, nome, nível em botões), com os 5 níveis padrão.
function impLanguagesBlock(langs) {
  return `
    <div class="form-card imp-block imp-skills" data-sec="languages">
      <div class="imp-block-head"><h4>Idiomas <span class="imp-count" data-count="languages"></span></h4></div>
      <p class="card-note">${langs.length ? 'Confira o nível de cada idioma.' : 'Nenhum idioma encontrado no currículo. Adicione os que você fala: eles aparecem no currículo gerado.'}</p>
      <div class="imp-skill-list" id="imp-lang-list">${langs.map(impLanguageRow).join('')}</div>
      <div class="imp-skill-add">
        <select id="imp-lang-new" aria-label="Idioma para adicionar">
          ${LANGUAGES.map(([c, n]) => `<option value="${c}">${n}</option>`).join('')}
        </select>
        <button class="btn btn-ghost btn-sm" onclick="impAddLanguage()">+ Adicionar idioma</button>
      </div>
    </div>`;
}

function impLanguageRow(l) {
  const exists = !!l.existing_level;
  const level = l.proficiency;
  return `
    <div class="imp-skill imp-lang" data-item data-level="${level}" data-exists="${exists ? 1 : ''}" data-orig="${exists ? l.existing_level : ''}" data-lang="${escStr(l.language)}">
      <input type="checkbox" class="imp-skill-check" ${exists && l.existing_level === level ? '' : 'checked'} aria-label="Incluir ${escStr(languageName(l.language))}" />
      <div class="imp-skill-main">
        <div class="imp-lang-name">${escStr(languageName(l.language))}</div>
        <div class="imp-skill-hint">${l.added ? 'adicionado por você' : 'encontrado no currículo'}${exists ? ` <span class="imp-tag">no perfil: ${languageLevel(l.existing_level)}</span>` : ''}</div>
      </div>
      <div class="imp-levels imp-levels-5" role="radiogroup" aria-label="Nível">
        ${LANGUAGE_LEVELS.map(([v, t]) => `<button type="button" role="radio" aria-checked="${v === level}" class="lvl-${v} ${v === level ? 'on' : ''}" onclick="impSetLevel(this,'${v}')">${t}</button>`).join('')}
      </div>
    </div>`;
}

function impAddLanguage() {
  const code = document.getElementById('imp-lang-new').value;
  const list = document.getElementById('imp-lang-list');
  if (list.querySelector(`[data-lang="${code}"]`)) { toast(`${languageName(code)} já está na lista.`, 'error'); return; }
  list.insertAdjacentHTML('beforeend', impLanguageRow({ language: code, proficiency: 'intermediate', added: true }));
  list.closest('.imp-block').querySelector('.card-note').textContent = 'Confira o nível de cada idioma.';
  impUpdateSummary();
}

function impSetLevel(btn, level) {
  const row = btn.closest('.imp-skill');
  row.dataset.level = level;
  row.querySelectorAll('.imp-levels button').forEach(b => {
    const on = b === btn;
    b.classList.toggle('on', on);
    b.setAttribute('aria-checked', on);
  });
  // Mudar o nível de uma habilidade que já está no perfil = querer atualizar.
  const check = row.querySelector('.imp-skill-check');
  if (row.dataset.exists) check.checked = level !== row.dataset.orig;
  else check.checked = true;
  impUpdateSummary();
}

function impAddSkill() {
  const input = document.getElementById('imp-skill-new');
  const name = input.value.trim();
  if (!name) return;
  const list = document.getElementById('imp-skill-list');
  list.insertAdjacentHTML('beforeend', impSkillRow({ skill_name: name, proficiency_level: 'intermediate', months: 0, explicit: false })
    .replace('listada no currículo, sem uso nas experiências', 'adicionada por você'));
  input.value = '';
  impUpdateSummary();
}

function impBulk(sec, on) {
  document.querySelectorAll(`#imp-review [data-sec="${sec}"] [data-item] > input[type=checkbox], #imp-review [data-sec="${sec}"] [data-item] .imp-item-head input[type=checkbox]`)
    .forEach(c => { c.checked = on; });
  impUpdateSummary();
}

function impListBlock(sec, title, items, render) {
  if (!items?.length) return '';
  return `
    <div class="form-card imp-block" data-sec="${sec}">
      <div class="imp-block-head">
        <h4>${title} <span class="imp-count" data-count="${sec}"></span></h4>
        <div class="imp-bulk">
          <button class="btn-more" onclick="impBulk('${sec}', true)">marcar todos</button>
          <button class="btn-more" onclick="impBulk('${sec}', false)">desmarcar todos</button>
        </div>
      </div>
      <div class="imp-items">${items.map(render).join('')}</div>
    </div>`;
}

// Cartão de item: cabeçalho com resumo + corpo com os campos (aberto quando falta algo).
function impItem({ title, meta, exists, missing, body }) {
  return `
    <div class="imp-item ${missing ? 'open' : ''}" data-item>
      <div class="imp-item-head">
        <input type="checkbox" ${exists ? '' : 'checked'} aria-label="Incluir" />
        <button type="button" class="imp-item-title" onclick="this.closest('.imp-item').classList.toggle('open')">
          <span class="imp-item-name">${title}</span>
          <span class="imp-item-meta">${meta}</span>
        </button>
        ${exists ? '<span class="imp-tag">já no perfil</span>' : ''}
        ${missing ? `<span class="imp-tag imp-tag-warn">${missing}</span>` : ''}
        <button type="button" class="imp-item-toggle" aria-label="Editar" onclick="this.closest('.imp-item').classList.toggle('open')">editar ▾</button>
      </div>
      <div class="imp-item-body">${body}</div>
    </div>`;
}

const impInput = (f, label, value, opts = {}) => `
  <div class="field">
    <label>${label}${opts.req ? ' *' : ''}</label>
    <input type="${opts.type || 'text'}" data-f="${f}" ${opts.kind ? `data-kind="${opts.kind}"` : ''} ${opts.req ? 'data-req' : ''} value="${escStr(value || '')}" ${opts.ph ? `placeholder="${opts.ph}"` : ''} />
  </div>`;

const impCheck = (f, label, checked) => `
  <div class="field-check"><input type="checkbox" data-f="${f}" ${checked ? 'checked' : ''} /><label>${label}</label></div>`;

function impExperienceItem(e) {
  return impItem({
    title: `${escStr(e.job_role)} <span class="imp-at">em</span> ${escStr(e.company_name)}`,
    meta: impPeriod(e.start_date, e.end_date, e.is_current_job) + (e.tech_stack?.length ? ' · ' + escStr(e.tech_stack.slice(0, 5).join(', ')) : ''),
    exists: e.exists,
    missing: !e.start_date ? 'falta a data de início' : '',
    body: `
      <div class="field-row">${impInput('company_name', 'Empresa', e.company_name, { req: 1 })}${impInput('job_role', 'Cargo', e.job_role, { req: 1 })}</div>
      <div class="field-row">${impInput('start_date', 'Início', e.start_date, { type: 'month', req: 1 })}${impInput('end_date', 'Fim', e.end_date, { type: 'month' })}</div>
      ${impCheck('is_current_job', 'Emprego atual', e.is_current_job)}
      <div class="field"><label>Conquistas <span class="hint">(uma por linha)</span></label><textarea data-f="achievements" data-kind="lines" rows="4">${escStr((e.achievements || []).join('\n'))}</textarea></div>
      ${impInput('tech_stack', 'Tecnologias <span class="hint">(vírgula)</span>', (e.tech_stack || []).join(', '), { kind: 'csv' })}
      <div class="field"><label>Descrição</label><textarea data-f="description" rows="2">${escStr(e.description || '')}</textarea></div>`,
  });
}

function impAcademicItem(a) {
  return impItem({
    title: `${escStr(a.course_name)} <span class="imp-at">·</span> ${escStr(a.institution_name)}`,
    meta: !a.start_date && a.end_date ? `conclusão ${impMonth(a.end_date)}` : impPeriod(a.start_date, a.end_date, false).replace('– ?', '– em andamento'),
    exists: a.exists,
    missing: '',
    body: `
      ${impInput('institution_name', 'Instituição', a.institution_name, { req: 1 })}
      ${impInput('course_name', 'Curso', a.course_name, { req: 1 })}
      <div class="field-row">${impInput('start_date', 'Início (opcional)', a.start_date, { type: 'month' })}${impInput('end_date', 'Conclusão', a.end_date, { type: 'month' })}</div>`,
  });
}

function impProjectItem(p) {
  return impItem({
    title: escStr(p.project_name),
    meta: (p.start_date ? impPeriod(p.start_date, p.end_date, false) : 'sem data') + (p.is_academic ? ' · acadêmico' : ''),
    exists: p.exists,
    missing: !p.start_date ? 'falta a data de início' : '',
    body: `
      ${impInput('project_name', 'Nome', p.project_name, { req: 1 })}
      <div class="field"><label>Descrição</label><textarea data-f="description" rows="3">${escStr(p.description || '')}</textarea></div>
      ${impInput('project_url', 'URL', p.project_url, { ph: 'https://' })}
      <div class="field-row">${impInput('start_date', 'Início', p.start_date, { type: 'month', req: 1 })}${impInput('end_date', 'Fim', p.end_date, { type: 'month' })}</div>
      ${impCheck('is_academic', 'Projeto acadêmico', p.is_academic)}
      ${impInput('tags', 'Tags <span class="hint">(vírgula)</span>', (p.tags || []).join(', '), { kind: 'csv' })}`,
  });
}

function impCertificateItem(c) {
  return impItem({
    title: escStr(c.certificate_name),
    meta: [c.issuing_organization, impMonth(c.issue_date)].filter(Boolean).map(escStr).join(' · ') || 'sem emissor nem data',
    exists: c.exists,
    missing: '',
    body: `
      ${impInput('certificate_name', 'Nome', c.certificate_name, { req: 1 })}
      ${impInput('issuing_organization', 'Emissor', c.issuing_organization)}
      <div class="field-row">${impInput('issue_date', 'Emissão', c.issue_date, { type: 'month' })}${impInput('credential_url', 'URL da credencial', c.credential_url, { ph: 'https://' })}</div>`,
  });
}

// ── Coleta e envio ──

function impReadItem(el) {
  const o = {};
  el.querySelectorAll('[data-f]').forEach(inp => {
    const f = inp.dataset.f;
    if (inp.type === 'checkbox') o[f] = inp.checked;
    else if (inp.dataset.kind === 'lines') o[f] = inp.value.split('\n').map(x => x.trim()).filter(Boolean);
    else if (inp.dataset.kind === 'csv') o[f] = splitCsv(inp.value);
    else o[f] = inp.value.trim();
  });
  return o;
}

const impIsChecked = el => el.querySelector('.imp-item-head > input[type=checkbox], .imp-skill-check')?.checked;

function impCollect() {
  const root = document.getElementById('imp-review');
  const req = { import_id: _imp.id, experiences: [], academic: [], skills: [], projects: [], certificates: [], languages: [] };

  for (const sec of ['profile', 'links']) {
    const obj = {};
    root.querySelectorAll(`[data-sec="${sec}"] [data-row]`).forEach(row => {
      if (row.querySelector('input[type=checkbox]').checked) obj[row.dataset.row] = row.querySelector('[data-f]').value.trim();
    });
    if (Object.keys(obj).length) req[sec] = obj;
  }
  for (const sec of ['experiences', 'academic', 'projects', 'certificates']) {
    root.querySelectorAll(`[data-sec="${sec}"] [data-item]`).forEach(el => {
      if (impIsChecked(el)) req[sec].push(impReadItem(el));
    });
  }
  root.querySelectorAll('[data-sec="skills"] [data-item]').forEach(el => {
    if (!impIsChecked(el)) return;
    req.skills.push({ skill_name: el.querySelector('[data-f=skill_name]').value.trim(), proficiency_level: el.dataset.level });
  });
  root.querySelectorAll('[data-sec="languages"] [data-item]').forEach(el => {
    if (impIsChecked(el)) req.languages.push({ language: el.dataset.lang, proficiency: el.dataset.level });
  });
  return req;
}

function impUpdateSummary() {
  const root = document.getElementById('imp-review');
  if (!root || root.classList.contains('hidden')) return;
  const parts = [];
  const names = { experiences: ['experiência', 'experiências'], academic: ['formação', 'formações'], skills: ['habilidade', 'habilidades'], projects: ['projeto', 'projetos'], certificates: ['certificado', 'certificados'], languages: ['idioma', 'idiomas'] };
  for (const sec of Object.keys(names)) {
    const all = root.querySelectorAll(`[data-sec="${sec}"] [data-item]`);
    const on = [...all].filter(impIsChecked).length;
    const badge = root.querySelector(`[data-count="${sec}"]`);
    if (badge) badge.textContent = `${on}/${all.length}`;
    if (on) parts.push(`${on} ${names[sec][on === 1 ? 0 : 1]}`);
  }
  const fields = root.querySelectorAll('[data-row] input[type=checkbox]:checked').length;
  if (fields) parts.unshift(`${fields} ${fields === 1 ? 'dado' : 'dados'} do perfil`);
  document.getElementById('imp-summary').textContent = parts.length ? 'Vai salvar: ' + parts.join(' · ') : 'Nada marcado para salvar';
  document.getElementById('imp-save').disabled = !parts.length;
}

// Confere os obrigatórios antes de enviar e abre o item com problema.
function impValidate() {
  const root = document.getElementById('imp-review');
  root.querySelectorAll('.imp-invalid').forEach(x => x.classList.remove('imp-invalid'));
  let first = null;
  root.querySelectorAll('.imp-item[data-item]').forEach(item => {
    if (!impIsChecked(item)) return;
    item.querySelectorAll('[data-req]').forEach(inp => {
      if (!inp.value.trim()) {
        inp.classList.add('imp-invalid');
        item.classList.add('open');
        first ??= inp;
      }
    });
  });
  const nameRow = root.querySelector('[data-row="full_name"]');
  if (nameRow?.querySelector('input[type=checkbox]').checked && !nameRow.querySelector('[data-f]').value.trim()) {
    first ??= nameRow.querySelector('[data-f]');
    first.classList.add('imp-invalid');
  }
  if (first) {
    first.scrollIntoView({ behavior: 'smooth', block: 'center' });
    first.focus({ preventScroll: true });
    toast('Preencha os campos marcados antes de salvar.', 'error');
    return false;
  }
  return true;
}

async function impApply() {
  if (!impValidate()) return;
  const btn = document.getElementById('imp-save');
  btn.disabled = true;
  btn.textContent = 'Salvando…';
  try {
    const res = await api.post('/users/me/import/apply', impCollect());
    const json = await res.json().catch(() => ({}));
    if (!res.ok) { toast(json.error || 'Erro ao salvar.', 'error'); return; }
    const s = json.data;
    const done = [
      s.profile && 'info básica', s.links && 'links',
      s.experiences && `${s.experiences} experiência(s)`, s.academic && `${s.academic} formação(ões)`,
      s.skills_added && `${s.skills_added} habilidade(s) nova(s)`, s.skills_updated && `${s.skills_updated} nível(is) atualizado(s)`,
      s.projects && `${s.projects} projeto(s)`, s.certificates && `${s.certificates} certificado(s)`,
      s.languages && `${s.languages} idioma(s)`,
    ].filter(Boolean);
    impForget();
    impEndWelcome();
    impShow('upload');
    toast('Perfil atualizado: ' + (done.join(', ') || 'nada mudou'));
    showPerfilTab('info', document.querySelector('.sub-nav button[data-perf="info"]'));
  } catch (e) {
    toast(e.message, 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = 'Salvar no perfil';
  }
}

async function impCancel() {
  const id = _imp.id;
  impForget();
  impShow('upload');
  impError('');
  if (id) api.delete(`/users/me/import/${id}`).catch(() => {});
  toast('Leitura cancelada.');
}

async function impDiscard() {
  if (!await confirmDialog('Descartar os dados lidos do currículo? Nada foi salvo no perfil.')) return;
  // Apaga também no servidor: o rascunho tem dados pessoais e não precisa esperar expirar.
  if (_imp.id) api.delete(`/users/me/import/${_imp.id}`).catch(() => {});
  impForget();
  impShow('upload');
}
