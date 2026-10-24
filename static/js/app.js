// ── App controller: auth, tabs, toast ──

function showAuth() {
  document.getElementById('auth-screen').classList.remove('hidden');
  document.getElementById('app').classList.add('hidden');
}

function showApp() {
  document.getElementById('auth-screen').classList.add('hidden');
  document.getElementById('app').classList.remove('hidden');
  checkAdminStatus();
  navigateTo('vagas');
  impResume();
  connectSSE();
}

// ── Confirm dialog ──
function confirmDialog(msg) {
  return new Promise(resolve => {
    document.getElementById('confirm-msg').textContent = msg;
    const modal  = document.getElementById('confirm-modal');
    const okBtn  = document.getElementById('confirm-ok');
    const noBtn  = document.getElementById('confirm-no');
    modal.classList.remove('hidden');

    function cleanup(result) {
      modal.classList.add('hidden');
      okBtn.removeEventListener('click', onOk);
      noBtn.removeEventListener('click', onNo);
      resolve(result);
    }
    function onOk() { cleanup(true);  }
    function onNo() { cleanup(false); }

    okBtn.addEventListener('click', onOk);
    noBtn.addEventListener('click', onNo);
  });
}

// ── Toast ──
let _toastTimer;
function toast(msg, tipo = 'success') {
  const el = document.getElementById('toast');
  el.textContent = msg;
  el.className = `toast ${tipo}`;
  el.onclick = null;
  clearTimeout(_toastTimer);
  _toastTimer = setTimeout(() => el.classList.add('hidden'), 3500);
}

// ── Tabs ──
let _activeTab = '';
function navigateTo(id) {
  document.querySelectorAll('section[data-tab]').forEach(s => s.classList.remove('active'));
  document.querySelectorAll('.sidebar button[data-tab]').forEach(b => b.classList.remove('active'));

  const section = document.getElementById('tab-' + id);
  // A Ajuda fica fora do menu (rodapé da barra lateral e ícone no topo do celular).
  document.querySelectorAll(`.sidebar button[data-tab="${id}"]`).forEach(b => b.classList.add('active'));
  const btn = document.querySelector(`nav button[data-tab="${id}"], .sidebar-foot button[data-tab="${id}"]`);
  if (section) section.classList.add('active');
  const crumb = document.getElementById('crumb-current');
  if (crumb && btn) crumb.textContent = (btn.querySelector('span') || btn).textContent.trim().toLowerCase();
  if (btn?.closest('nav') && window.matchMedia('(max-width: 860px)').matches) btn.scrollIntoView({ inline: 'nearest', block: 'nearest' });

  if (_activeTab === 'metricas') stopMetricsRefresh();
  _activeTab = id;
  if (id === 'vagas')    loadJobs();
  if (id === 'perfil')   loadProfile();
  if (id === 'filtros')  loadFilters();
  if (id === 'metricas') startMetricsRefresh();
  if (id === 'admin')    loadAdminPanel();
  if (id === 'ajuda')    loadHelp();
  impSyncPill();
}

// ── Auth: login / register ──
let _authMode = 'login';

function switchAuthMode(mode) {
  _authMode = mode;
  document.querySelectorAll('.auth-tabs button').forEach(b => b.classList.remove('active'));
  document.querySelector(`.auth-tabs button[data-mode="${mode}"]`).classList.add('active');
  document.getElementById('auth-login').classList.toggle('hidden',    mode !== 'login');
  document.getElementById('auth-register').classList.toggle('hidden', mode !== 'register');
}

async function doLogin() {
  const email    = document.getElementById('login-email').value.trim();
  const password = document.getElementById('login-password').value;
  if (!email || !password) { toast('Preencha email e senha.', 'error'); return; }

  try {
    const res = await api.post('/auth/login', { email, password });
    if (!res.ok) {
      const j = await res.json().catch(() => ({}));
      toast(j.error || 'Credenciais inválidas.', 'error');
      return;
    }
    const d = await res.json();
    setToken(d.data?.token ?? d.token);
    showApp();
  } catch (e) {
    toast(e.message || 'Erro de conexão.', 'error');
  }
}

async function doRegister() {
  const email    = document.getElementById('reg-email').value.trim();
  const password = document.getElementById('reg-password').value;
  if (!email || !password) { toast('Preencha email e senha.', 'error'); return; }
  if (password.length < 8) { toast('Senha deve ter ao menos 8 caracteres.', 'error'); return; }

  try {
    const res = await api.post('/auth/register', { email, password });
    if (!res.ok) {
      const j = await res.json().catch(() => ({}));
      toast(j.error || 'Erro ao criar conta.', 'error');
      return;
    }
    toast('Conta criada! Faça login.');
    switchAuthMode('login');
    document.getElementById('login-email').value = email;
  } catch (e) {
    toast(e.message || 'Erro de conexão.', 'error');
  }
}

function doLogout() {
  disconnectSSE();
  clearToken();
  localStorage.removeItem('jp_is_admin');
  if (typeof clearSkillsCache === 'function') clearSkillsCache();
  showAuth();
}

// ── Init ──
window.addEventListener('DOMContentLoaded', () => {
  // Auth mode switch
  document.querySelectorAll('.auth-tabs button').forEach(btn => {
    btn.addEventListener('click', () => switchAuthMode(btn.dataset.mode));
  });

  // Nav tabs
  document.querySelectorAll('.sidebar button[data-tab]').forEach(btn => {
    btn.addEventListener('click', () => navigateTo(btn.dataset.tab));
  });

  // Enter key on auth forms
  document.getElementById('login-password').addEventListener('keydown', e => {
    if (e.key === 'Enter') doLogin();
  });
  document.getElementById('reg-password').addEventListener('keydown', e => {
    if (e.key === 'Enter') doRegister();
  });

  // Relógio da barra superior
  const clock = document.getElementById('topbar-clock');
  const tick = () => { if (clock) clock.textContent = new Date().toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' }); };
  tick(); setInterval(tick, 30_000);

  if (getToken()) {
    showApp();
  } else {
    showAuth();
  }
});

// ── Atualização automática (métricas, importações) ──
// Cada tela registra o que recarregar; o intervalo para quando a aba do navegador está em
// segundo plano e recarrega na hora ao voltar. O selo "ao vivo" pausa/retoma com um clique.
const _live = {};

function liveStart(key, fn, ms) {
  liveStop(key);
  const paused = _live[key]?.paused ?? false;
  _live[key] = { fn, paused, timer: setInterval(() => { if (!document.hidden && !_live[key].paused) fn(); }, ms) };
  liveBadge(key);
}

function liveStop(key) {
  if (_live[key]) clearInterval(_live[key].timer);
}

function liveToggle(key) {
  const l = _live[key];
  if (!l) return;
  l.paused = !l.paused;
  if (!l.paused) l.fn();
  liveBadge(key);
}

// Chamado depois de cada carga bem-sucedida: mostra a hora da última atualização.
function liveMark(key) {
  if (_live[key]) _live[key].at = new Date();
  liveBadge(key);
}

function liveBadge(key) {
  const el = document.getElementById(`${key}-live`);
  const l = _live[key];
  if (!el || !l) return;
  const at = l.at ? l.at.toLocaleTimeString('pt-BR') : '—';
  el.classList.toggle('paused', l.paused);
  el.innerHTML = l.paused ? `<i></i>pausado · ${at}` : `<i></i>ao vivo · ${at}`;
  el.title = l.paused ? 'Atualização automática pausada. Clique para retomar.' : 'Atualiza sozinho a cada poucos segundos. Clique para pausar.';
}

document.addEventListener('visibilitychange', () => {
  if (document.hidden) return;
  for (const l of Object.values(_live)) if (l.timer && !l.paused) l.fn();
});
