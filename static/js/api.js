// API client com suporte a JWT
const API_BASE = 'http://localhost:8080/api/v1';

const getToken = () => localStorage.getItem('jp_token');
const setToken = t => localStorage.setItem('jp_token', t);
const clearToken = () => localStorage.removeItem('jp_token');

async function apiFetch(method, path, body) {
  const headers = { 'Content-Type': 'application/json' };
  const token = getToken();
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const opts = { method, headers };
  if (body !== undefined) opts.body = JSON.stringify(body);

  const res = await fetch(API_BASE + path, opts);

  if (res.status === 401) {
    clearToken();
    if (typeof showAuth === 'function') showAuth();
    throw new Error('Sessão expirada. Faça login novamente.');
  }

  return res;
}

const api = {
  get:    (path)       => apiFetch('GET',    path),
  post:   (path, body) => apiFetch('POST',   path, body),
  put:    (path, body) => apiFetch('PUT',    path, body),
  delete: (path)       => apiFetch('DELETE', path),
};

// Helpers de resposta
async function apiData(res) {
  const json = await res.json();
  return json.data;
}

async function apiList(res) {
  const json = await res.json();
  return { data: json.data ?? [], meta: json.meta ?? {} };
}

// Helpers para tipos pgx que podem chegar como string ou null
const pgStr  = v => (v && typeof v === 'string') ? v : null;
const fmtDate = iso => {
  if (!iso) return '—';
  if (iso.length === 10) return iso; // já é YYYY-MM-DD
  return new Date(iso).toLocaleString('pt-BR', { day:'2-digit', month:'2-digit', year:'numeric', hour:'2-digit', minute:'2-digit' });
};
const splitCsv = s => s.split(',').map(x => x.trim()).filter(Boolean);
