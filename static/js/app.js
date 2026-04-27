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
  clearTimeout(_toastTimer);
  _toastTimer = setTimeout(() => el.classList.add('hidden'), 3500);
}

// ── Tabs ──
let _activeTab = '';
function navigateTo(id) {
  document.querySelectorAll('section[data-tab]').forEach(s => s.classList.remove('active'));
  document.querySelectorAll('nav button[data-tab]').forEach(b => b.classList.remove('active'));

  const section = document.getElementById('tab-' + id);
  const btn     = document.querySelector(`nav button[data-tab="${id}"]`);
  if (section) section.classList.add('active');
  if (btn)     btn.classList.add('active');

  if (_activeTab === 'metricas') stopMetricsRefresh();
  _activeTab = id;
  if (id === 'vagas')    loadJobs();
  if (id === 'perfil')   loadProfile();
  if (id === 'filtros')  loadFilters();
  if (id === 'metricas') startMetricsRefresh();
  if (id === 'admin')    loadAdminPanel();
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
  document.querySelectorAll('nav button[data-tab]').forEach(btn => {
    btn.addEventListener('click', () => navigateTo(btn.dataset.tab));
  });

  // Enter key on auth forms
  document.getElementById('login-password').addEventListener('keydown', e => {
    if (e.key === 'Enter') doLogin();
  });
  document.getElementById('reg-password').addEventListener('keydown', e => {
    if (e.key === 'Enter') doRegister();
  });

  if (getToken()) {
    showApp();
  } else {
    showAuth();
  }
});
