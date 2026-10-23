// ── Metrics dashboard — faz parse do formato Prometheus texto ──

let _metricsTimer = null;

function parsePrometheus(text) {
  const m = {};
  for (const line of text.split('\n')) {
    if (line.startsWith('#') || !line.trim()) continue;
    const match = line.match(/^([a-z_:][a-z0-9_:]*?)(\{[^}]*\})?\s+([-\d.e+]+)/);
    if (!match) continue;
    m[match[1] + (match[2] || '')] = parseFloat(match[3]);
  }
  return m;
}

function get(m, key) { return m[key] ?? 0; }

// Soma todas as entradas cujo nome começa com o prefixo dado.
function sumByPrefix(m, prefix) {
  let total = 0;
  for (const [k, v] of Object.entries(m)) {
    if (k.startsWith(prefix)) total += v;
  }
  return total;
}

// Soma entradas de um counter com label status="Nxx" (ex: statusClass='2' → 2xx).
function sumHTTPByStatus(m, statusClass) {
  let total = 0;
  for (const [k, v] of Object.entries(m)) {
    if (!k.startsWith('hackton_http_requests_total{')) continue;
    const match = k.match(/status="(\d+)"/);
    if (match && match[1].startsWith(statusClass)) total += v;
  }
  return total;
}

function fmt(n, decimals = 1) {
  if (n === 0) return '0';
  return n.toFixed(decimals).replace(/\.0$/, '');
}

function fmtBytes(bytes) {
  if (bytes < 1024 * 1024) return fmt(bytes / 1024) + ' KB';
  return fmt(bytes / (1024 * 1024)) + ' MB';
}

function fmtDur(seconds) {
  if (seconds === 0) return '—';
  if (seconds < 1) return fmt(seconds * 1000) + ' ms';
  if (seconds < 60) return fmt(seconds) + ' s';
  return fmt(seconds / 60) + ' min';
}

function avgDuration(m, metric, status) {
  const count = get(m, `${metric}_count{status="${status}"}`);
  const sum   = get(m, `${metric}_sum{status="${status}"}`);
  return count > 0 ? sum / count : 0;
}

// Estado de uma taxa de sucesso (cores de status reservadas, sempre com rótulo).
function statusLevel(rate) { return rate >= .9 ? 'good' : rate >= .7 ? 'warning' : 'critical'; }
function statusChip(rate, label) {
  return `<span class="status-chip status-${statusLevel(rate)}">${fmt(rate * 100)}% ${label}</span>`;
}
function setHTML(id, html) {
  const el = document.getElementById(id);
  if (el) el.innerHTML = html;
}

function set(id, value) {
  const el = document.getElementById(id);
  if (el) el.textContent = value;
}

// Aplica cor ao elemento baseado em thresholds numéricos.
// yellow/red são os limites. Abaixo de yellow = cor padrão (sem classe).
function setThreshold(id, raw, yellow, red) {
  const el = document.getElementById(id);
  if (!el) return;
  el.classList.remove('metric-ok', 'metric-warn', 'metric-danger');
  if (raw >= red)    el.classList.add('metric-danger');
  else if (raw >= yellow) el.classList.add('metric-warn');
  else               el.classList.add('metric-ok');
}

async function triggerGC() {
  const btn = document.getElementById('btn-force-gc');
  if (btn) { btn.disabled = true; btn.textContent = 'Aguardando…'; }
  try {
    const token = localStorage.getItem('jp_token');
    const res = await fetch('/api/v1/debug/gc', {
      method: 'POST',
      headers: token ? { Authorization: 'Bearer ' + token } : {},
    });
    if (!res.ok) throw new Error('status ' + res.status);
    const d = await res.json();
    const freed = d.freed_bytes ?? 0;
    const msg = freed > 0
      ? `GC concluído — liberou ${(freed / 1024 / 1024).toFixed(1)} MB`
      : 'GC concluído — heap já estava limpo';
    set('metrics-error', '');
    if (btn) btn.textContent = msg;
    setTimeout(() => { if (btn) { btn.textContent = 'Forçar GC'; btn.disabled = false; } }, 3000);
    loadMetrics();
  } catch (e) {
    if (btn) { btn.textContent = 'Erro: ' + e.message; btn.disabled = false; }
  }
}

async function loadMetrics() {
  try {
    // Exige admin (antes era o /metrics público).
    const res = await api.get('/admin/metrics/raw');
    if (!res.ok) throw new Error('status ' + res.status);
    const text = await res.text();
    const m = parsePrometheus(text);
    renderMetrics(m);
    renderDBMetrics(m);
    set('metrics-error', '');
  } catch (e) {
    set('metrics-error', 'Erro ao carregar métricas: ' + e.message);
  }
}

// ── Per-route breakdown ──

function approxPercentile(buckets, p) {
  if (!buckets.length) return 0;
  const total = buckets[buckets.length - 1].count;
  if (total === 0) return 0;
  const target = p * total;
  for (let i = 0; i < buckets.length; i++) {
    if (buckets[i].count >= target) {
      if (i === 0) return buckets[i].le === Infinity ? 0 : buckets[i].le;
      const prevLe = buckets[i - 1].le;
      const curLe  = buckets[i].le === Infinity ? prevLe * 2 : buckets[i].le;
      const span   = buckets[i].count - buckets[i - 1].count;
      const frac   = span > 0 ? (target - buckets[i - 1].count) / span : 0;
      return prevLe + frac * (curLe - prevLe);
    }
  }
  return 0;
}

function buildRouteTable(m) {
  const routes = {};

  function ensure(key, method, path) {
    if (!routes[key]) routes[key] = { method, path, total: 0, s2xx: 0, s4xx: 0, s5xx: 0, durSum: 0, durCount: 0, buckets: [] };
  }

  for (const [k, v] of Object.entries(m)) {
    if (k.startsWith('hackton_http_requests_total{')) {
      const method = k.match(/method="([^"]+)"/)?.[1];
      const path   = k.match(/path="([^"]+)"/)?.[1];
      const status = k.match(/status="([^"]+)"/)?.[1];
      if (!method || !path || !status) continue;
      const key = `${method} ${path}`;
      ensure(key, method, path);
      routes[key].total += v;
      if      (status.startsWith('2')) routes[key].s2xx += v;
      else if (status.startsWith('4')) routes[key].s4xx += v;
      else if (status.startsWith('5')) routes[key].s5xx += v;
    } else if (k.startsWith('hackton_http_request_duration_seconds_sum{')) {
      const method = k.match(/method="([^"]+)"/)?.[1];
      const path   = k.match(/path="([^"]+)"/)?.[1];
      if (!method || !path) continue;
      const key = `${method} ${path}`;
      ensure(key, method, path);
      routes[key].durSum += v;
    } else if (k.startsWith('hackton_http_request_duration_seconds_count{')) {
      const method = k.match(/method="([^"]+)"/)?.[1];
      const path   = k.match(/path="([^"]+)"/)?.[1];
      if (!method || !path) continue;
      const key = `${method} ${path}`;
      ensure(key, method, path);
      routes[key].durCount += v;
    } else if (k.startsWith('hackton_http_request_duration_seconds_bucket{')) {
      const method = k.match(/method="([^"]+)"/)?.[1];
      const path   = k.match(/path="([^"]+)"/)?.[1];
      const le     = k.match(/le="([^"]+)"/)?.[1];
      if (!method || !path || !le) continue;
      const key = `${method} ${path}`;
      ensure(key, method, path);
      routes[key].buckets.push({ le: le === '+Inf' ? Infinity : parseFloat(le), count: v });
    }
  }

  for (const r of Object.values(routes)) {
    r.buckets.sort((a, b) => a.le - b.le);
    r.p95 = approxPercentile(r.buckets, 0.95);
  }

  return Object.values(routes).sort((a, b) => b.total - a.total);
}

function latencyColor(sec) {
  if (sec >= 0.5) return 'var(--red)';
  if (sec >= 0.1) return 'var(--yellow)';
  return 'var(--green)';
}

function renderRouteTable(m) {
  const rows = buildRouteTable(m);
  const body = document.getElementById('m-routes-body');
  const card = document.getElementById('m-routes-card');
  if (!body || !card) return;
  if (rows.length === 0) { card.style.display = 'none'; return; }
  card.style.display = '';

  const dim = `<span style="color:var(--muted)">0</span>`;
  body.innerHTML = rows.map(r => {
    const avgRaw = r.durCount > 0 ? r.durSum / r.durCount : 0;
    const avgStr = avgRaw > 0 ? fmtDur(avgRaw) : '—';
    const p95Str = r.p95  > 0 ? fmtDur(r.p95)  : '—';
    const lc     = avgRaw > 0 ? latencyColor(avgRaw) : 'var(--muted)';
    const p95c   = r.p95  > 0 ? latencyColor(r.p95)  : 'var(--muted)';
    return `<tr>
      <td><span class="method-badge method-${r.method.toLowerCase()}">${r.method}</span></td>
      <td class="route-path">${r.path}</td>
      <td class="rt-num">${r.total}</td>
      <td class="rt-num" style="color:${lc}">${avgStr}</td>
      <td class="rt-num" style="color:${p95c}">${p95Str}</td>
      <td class="rt-num" style="color:var(--green)">${r.s2xx || dim}</td>
      <td class="rt-num" style="color:${r.s4xx > 0 ? 'var(--yellow)' : ''}">${r.s4xx > 0 ? r.s4xx : dim}</td>
      <td class="rt-num" style="color:${r.s5xx > 0 ? 'var(--red)' : ''}">${r.s5xx > 0 ? r.s5xx : dim}</td>
    </tr>`;
  }).join('');
}

function renderMetrics(m) {
  // Jobs
  const completed = get(m, 'hackton_jobs_processed_total{status="completed"}');
  const errored   = get(m, 'hackton_jobs_processed_total{status="error"}');
  const recovered = get(m, 'hackton_jobs_processed_total{status="recovered"}');
  const total     = completed + errored;

  set('m-jobs-completed', completed);
  set('m-jobs-error',     errored);
  set('m-jobs-recovered', recovered);
  setHTML('m-jobs-total', total > 0 ? statusChip(completed / total, 'de sucesso') : '—');

  // Worker
  set('m-goroutines-active', get(m, 'hackton_worker_active_goroutines'));

  // LLM
  const llmOkCount  = get(m, 'hackton_llm_duration_seconds_count{status="ok"}');
  const llmErrCount = get(m, 'hackton_llm_duration_seconds_count{status="error"}');
  const llmRlCount  = get(m, 'hackton_llm_duration_seconds_count{status="rate_limit"}');
  const llmTotal    = llmOkCount + llmErrCount + llmRlCount;
  const llmAvg      = avgDuration(m, 'hackton_llm_duration_seconds', 'ok');

  set('m-llm-total',     llmTotal || '—');
  set('m-llm-avg',       fmtDur(llmAvg));
  set('m-llm-ok',        llmOkCount);
  set('m-llm-err',       llmErrCount);
  set('m-llm-rl',        llmRlCount);

  // Scraper
  const scrOkCount  = get(m, 'hackton_scraper_duration_seconds_count{status="ok"}');
  const scrErrCount = get(m, 'hackton_scraper_duration_seconds_count{status="error"}');
  const scrAvg      = avgDuration(m, 'hackton_scraper_duration_seconds', 'ok');

  set('m-scraper-total', (scrOkCount + scrErrCount) || '—');
  set('m-scraper-avg',   fmtDur(scrAvg));
  set('m-scraper-ok',    scrOkCount);
  set('m-scraper-err',   scrErrCount);

  // Go runtime
  const goRoutines = get(m, 'go_goroutines');
  const goHeap     = get(m, 'go_memstats_heap_alloc_bytes');
  const goThreads  = get(m, 'go_threads');
  set('m-go-goroutines', goRoutines);
  set('m-go-heap',       fmtBytes(goHeap));
  set('m-go-gc',         get(m, 'go_gc_duration_seconds_count'));
  set('m-go-threads',    goThreads);
  setThreshold('m-go-goroutines', goRoutines, 100, 500);
  setThreshold('m-go-heap',       goHeap,     100 * 1024 * 1024, 500 * 1024 * 1024);
  setThreshold('m-go-threads',    goThreads,  50,  100);

  // HTTP API
  const httpTotal  = sumByPrefix(m, 'hackton_http_requests_total{');
  const http2xx    = sumHTTPByStatus(m, '2');
  const http4xx    = sumHTTPByStatus(m, '4');
  const http5xx    = sumHTTPByStatus(m, '5');
  const httpDurSum   = sumByPrefix(m, 'hackton_http_request_duration_seconds_sum{');
  const httpDurCount = sumByPrefix(m, 'hackton_http_request_duration_seconds_count{');
  const httpAvg      = httpDurCount > 0 ? httpDurSum / httpDurCount : 0;
  const httpSuccessRate = httpTotal > 0 ? fmt(http2xx / httpTotal * 100) + '%' : '—';

  set('m-http-total',        httpTotal || '—');
  set('m-http-avg',          fmtDur(httpAvg));
  set('m-http-success-rate', httpSuccessRate);
  set('m-http-2xx',          http2xx);
  set('m-http-4xx',          http4xx);
  set('m-http-5xx',          http5xx);

  // Timestamp
  set('metrics-updated', 'Atualizado ' + new Date().toLocaleTimeString('pt-BR'));
  renderRouteTable(m);
}

let _metricsView = 'geral';

function switchMetricsView(view, btn) {
  _metricsView = view;
  document.querySelectorAll('[data-mview]').forEach(b => b.classList.toggle('active', b === btn));
  for (const v of ['geral', 'sistema', 'ia', 'banco']) {
    document.getElementById(`metrics-view-${v}`).classList.toggle('hidden', v !== view);
  }
  refreshMetricsView();
}

function refreshMetricsView() {
  if (_metricsView === 'geral') loadGeneralMetrics();
  else if (_metricsView === 'ia') loadAIMetrics();
  else loadMetrics();
}

function startMetricsRefresh() {
  refreshMetricsView();
  _metricsTimer = setInterval(refreshMetricsView, 30_000);
}

function stopMetricsRefresh() {
  clearInterval(_metricsTimer);
  _metricsTimer = null;
}

// ── Banco: consultas (tracer do pgx) e pool de conexões ──

function renderDBMetrics(m) {
  const acquired = get(m, 'hackton_db_pool_acquired_conns');
  set('db-acquired', acquired);
  set('db-conns', `${get(m, 'hackton_db_pool_total_conns')} abertas · ${get(m, 'hackton_db_pool_idle_conns')} ociosas · máx. ${get(m, 'hackton_db_pool_max_conns')}`);
  const waits = get(m, 'hackton_db_pool_empty_acquire_total');
  set('db-waits', waits);
  set('db-wait-time', waits > 0 ? `${fmtDur(get(m, 'hackton_db_pool_acquire_seconds_total'))} aguardando no total` : 'nenhuma: o pool deu conta');

  const rows = dbQueryRows(m);
  const total = rows.reduce((a, r) => a + r.count, 0);
  const errors = rows.reduce((a, r) => a + r.errors, 0);
  const sum = rows.reduce((a, r) => a + r.sum, 0);
  set('db-queries', total);
  set('db-errors', errors > 0 ? `${errors} com erro` : 'nenhum erro');
  set('db-avg', total > 0 ? fmtDur(sum / total) : '—');
  renderDBQueryTable(rows);
}

// Consultas ao Postgres (tracer do pgx), ordenadas pelo tempo total gasto.
function dbQueryRows(m) {
  const queries = {};
  for (const [k, v] of Object.entries(m)) {
    const mm = k.match(/^hackton_db_query_duration_seconds_(bucket|sum|count)\{(.*)\}$/);
    if (!mm) continue;
    const labels = Object.fromEntries([...mm[2].matchAll(/(\w+)="([^"]*)"/g)].map(x => [x[1], x[2]]));
    const q = queries[labels.query] ??= { count: 0, sum: 0, errors: 0, buckets: {} };
    if (mm[1] === 'count') { q.count += v; if (labels.status === 'error') q.errors += v; }
    if (mm[1] === 'sum') q.sum += v;
    if (mm[1] === 'bucket') { const le = labels.le === '+Inf' ? Infinity : parseFloat(labels.le); q.buckets[le] = (q.buckets[le] || 0) + v; }
  }
  return Object.entries(queries).map(([name, q]) => ({
    name, ...q,
    p95: approxPercentile(Object.entries(q.buckets).map(([le, count]) => ({ le: Number(le), count })).sort((a, b) => a.le - b.le), 0.95),
  })).sort((a, b) => b.sum - a.sum);
}

function renderDBQueryTable(rows) {
  const body = document.getElementById('db-queries-body');
  body.innerHTML = rows.length ? rows.map(r => `<tr>
      <td data-label="Consulta"><code>${escHtml(r.name)}</code></td>
      <td class="rt-num" data-label="Execuções">${r.count}</td>
      <td class="rt-num" style="color:${latencyColor(r.sum / r.count)}" data-label="Média">${fmtDur(r.sum / r.count)}</td>
      <td class="rt-num" data-label="p95">${fmtDur(r.p95)}</td>
      <td class="rt-num" data-label="Tempo total">${fmtDur(r.sum)}</td>
      <td class="rt-num" data-label="Erros">${r.errors ? `<span class="metric-chip chip-red">${r.errors}</span>` : '0'}</td>
    </tr>`).join('') : '<tr><td colspan="6" class="empty">Nenhuma consulta registrada desde o último reinício.</td></tr>';
}

// ── IA: histórico das chamadas aos providers ──

const AI_STATUS_LABELS = {
  rate_limit: 'rate limit', timeout: 'timeout', truncated: 'cortada', empty: 'vazia',
  parse_error: 'JSON inválido', http_error: 'erro HTTP', network_error: 'rede', error: 'outro',
};
const AI_PURPOSE_LABELS = { main: 'Currículo', scrape: 'Scrape' };
const AI_BUCKET_MS = { '24h': 3600e3, '7d': 6 * 3600e3, '30d': 24 * 3600e3 };

function fmtMs(ms) { return ms ? fmtDur(ms / 1000) : '—'; }

function fmtTokens(n) {
  if (n >= 1e6) return fmt(n / 1e6) + ' mi';
  if (n >= 1e3) return fmt(n / 1e3) + ' mil';
  return String(Math.round(n));
}

async function loadAIMetrics() {
  const range = document.getElementById('ai-range').value;
  try {
    const res = await api.get('/admin/metrics/ai?range=' + range);
    if (!res.ok) throw new Error('status ' + res.status);
    const d = (await res.json()).data;
    renderAIMetrics(d, range);
    set('metrics-error', '');
    set('metrics-updated', 'Atualizado ' + new Date().toLocaleTimeString('pt-BR'));
  } catch (e) {
    set('metrics-error', 'Erro ao carregar métricas de IA: ' + e.message);
  }
}

function renderAIMetrics(d, range) {
  const s = d.summary;
  set('ai-calls', s.calls || '0');
  setHTML('ai-success', s.calls ? statusChip(s.success_rate, 'com sucesso') : 'nenhuma chamada no período');
  set('ai-avg', fmtMs(s.avg_ok_ms));
  set('ai-tokens', fmtTokens(s.prompt_tokens + s.completion_tokens));
  set('ai-tokens-split', `${fmtTokens(s.prompt_tokens)} entrada · ${fmtTokens(s.completion_tokens)} saída`);
  set('ai-fallback', s.fallback_saves);

  renderAITimeline(d.timeline, d.since, AI_BUCKET_MS[range]);

  const models = document.getElementById('ai-models-body');
  models.innerHTML = d.models.length ? d.models.map(m => {
    const rate = m.calls ? m.ok / m.calls : 0;
    const fails = Object.entries(m.by_status).filter(([k]) => k !== 'ok')
      .map(([k, n]) => `<span class="metric-chip chip-red" style="white-space:nowrap">${escHtml(AI_STATUS_LABELS[k] || k)} ${n}</span>`).join(' ');
    return `<tr title="${aiEsc(m.last_error || '')}">
      <td data-label="Finalidade">${AI_PURPOSE_LABELS[m.purpose] || escHtml(m.purpose)}</td>
      <td data-label="Modelo"><code>${escHtml(m.model || m.provider)}</code><div style="color:var(--muted);font-size:11px">${escHtml(m.provider)}</div></td>
      <td class="rt-num" data-label="Chamadas">${m.calls}</td>
      <td class="rt-num" data-label="Sucesso">${fmt(rate * 100)}%</td>
      <td class="rt-num" data-label="p50">${fmtMs(m.p50_ms)}</td>
      <td class="rt-num" data-label="p95">${fmtMs(m.p95_ms)}</td>
      <td class="rt-num" data-label="Tokens méd. (entrada / saída)">${fmtTokens(m.avg_prompt_tokens)} / ${fmtTokens(m.avg_output_tokens)}</td>
      <td data-label="Falhas por tipo">${fails || '<span class="metric-chip chip-muted">nenhuma</span>'}</td>
    </tr>`;
  }).join('') : '<tr><td colspan="8" class="empty">Nenhuma chamada no período.</td></tr>';

  const failures = document.getElementById('ai-failures-body');
  failures.innerHTML = d.failures.length ? d.failures.map(f => `<tr>
      <td style="white-space:nowrap" data-label="Quando">${new Date(f.at).toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' })}</td>
      <td data-label="Finalidade">${AI_PURPOSE_LABELS[f.purpose] || escHtml(f.purpose)}</td>
      <td data-label="Modelo"><code>${escHtml(f.model || f.provider)}</code></td>
      <td data-label="Tipo"><span class="metric-chip chip-red" style="white-space:nowrap">${escHtml(AI_STATUS_LABELS[f.status] || f.status)}</span></td>
      <td class="rt-num" data-label="Tempo">${fmtMs(f.ms)}</td>
      <td style="max-width:420px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" title="${aiEsc(f.error)}" data-label="Erro">${escHtml(f.error)}</td>
    </tr>`).join('') : '<tr><td colspan="6" class="empty">Nenhuma falha no período.</td></tr>';
}

// Barras empilhadas por intervalo: sucesso embaixo, falha (hachurada) em cima.
// Intervalos sem chamadas aparecem vazios, para o eixo do tempo ficar contínuo.
function renderAITimeline(timeline, since, bucketMs) {
  const byStart = new Map(timeline.map(b => [Math.floor(new Date(b.start).getTime() / bucketMs), b]));
  const first = Math.floor(new Date(since).getTime() / bucketMs);
  const last = Math.floor(Date.now() / bucketMs);
  const buckets = [];
  for (let i = first; i <= last; i++) {
    const b = byStart.get(i);
    buckets.push({ start: i * bucketMs, ok: b ? b.ok : 0, failed: b ? b.failed : 0 });
  }
  const max = Math.max(1, ...buckets.map(b => b.ok + b.failed));
  set('ai-ymax', max);

  const el = document.getElementById('ai-timeline');
  el.innerHTML = buckets.map((b, i) => {
    const pct = n => (n / max * 100).toFixed(2);
    const fail = b.failed ? `<div class="ai-seg-fail" style="height:${pct(b.failed)}%"></div>` : '';
    const ok = b.ok ? `<div class="ai-seg-ok" style="height:${pct(b.ok)}%"></div>` : '';
    return `<div class="ai-col" data-i="${i}">${fail}${ok}</div>`;
  }).join('');

  const label = ms => {
    const dt = new Date(ms);
    return bucketMs >= 24 * 3600e3
      ? dt.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit' })
      : dt.toLocaleString('pt-BR', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });
  };
  const mid = buckets[Math.floor(buckets.length / 2)];
  document.getElementById('ai-xaxis').innerHTML =
    `<span>${label(buckets[0].start)}</span><span>${label(mid.start)}</span><span>${label(buckets[buckets.length - 1].start)}</span>`;

  const tip = document.getElementById('ai-tooltip');
  el.onmousemove = e => {
    const col = e.target.closest('.ai-col');
    if (!col) { tip.classList.add('hidden'); return; }
    const b = buckets[+col.dataset.i];
    tip.innerHTML = `<div style="color:var(--muted);margin-bottom:2px">${label(b.start)}</div>
      <div><i class="swatch swatch-ok"></i> Sucesso <b>${b.ok}</b></div>
      <div><i class="swatch swatch-fail"></i> Falha <b>${b.failed}</b></div>`;
    tip.classList.remove('hidden');
    const x = Math.min(e.clientX + 12, window.innerWidth - tip.offsetWidth - 8);
    tip.style.left = x + 'px';
    tip.style.top = (e.clientY - tip.offsetHeight - 10) + 'px';
  };
  el.onmouseleave = () => tip.classList.add('hidden');
}

// ── Geral: o essencial de Sistema, IA e Banco numa tela só ──

async function loadGeneralMetrics() {
  try {
    const [rawRes, aiRes] = await Promise.all([api.get('/admin/metrics/raw'), api.get('/admin/metrics/ai?range=24h')]);
    if (!rawRes.ok) throw new Error('métricas: status ' + rawRes.status);
    if (!aiRes.ok) throw new Error('IA: status ' + aiRes.status);
    const m = parsePrometheus(await rawRes.text());
    const ai = (await aiRes.json()).data;
    renderGeneralMetrics(m, ai);
    set('metrics-error', '');
    set('metrics-updated', 'Atualizado ' + new Date().toLocaleTimeString('pt-BR'));
  } catch (e) {
    set('metrics-error', 'Erro ao carregar métricas: ' + e.message);
  }
}

function renderGeneralMetrics(m, ai) {
  // Jobs (desde o último reinício)
  const ok = get(m, 'hackton_jobs_processed_total{status="completed"}');
  const err = get(m, 'hackton_jobs_processed_total{status="error"}');
  set('g-jobs-ok', ok);
  setHTML('g-jobs-sub', ok + err > 0 ? statusChip(ok / (ok + err), 'de sucesso') : 'nenhum job desde o reinício');
  set('g-jobs-err', err);
  setThreshold('g-jobs-err', err, 1, 5);

  // IA (histórico de 24h no banco)
  const s = ai.summary;
  set('g-ai-calls', s.calls || '0');
  setHTML('g-ai-sub', s.calls ? statusChip(s.success_rate, 'com sucesso') : 'nenhuma chamada');
  set('g-ai-avg', fmtMs(s.avg_ok_ms));
  set('g-ai-fallback', s.fallback_saves ? `fallback salvou ${s.fallback_saves}×` : 'fallback não foi necessário');

  const models = document.getElementById('g-ai-models');
  const top = ai.models.slice().sort((a, b) => b.calls - a.calls).slice(0, 5);
  models.innerHTML = top.length ? top.map(x => {
    const rate = x.calls ? x.ok / x.calls : 0;
    return `<div class="mini-row" title="${aiEsc(x.last_error || '')}">
      <span class="name">${escHtml(x.model || x.provider)}<small>${AI_PURPOSE_LABELS[x.purpose] || escHtml(x.purpose)} · ${x.calls} chamadas</small>
        <span class="bar bar-${statusLevel(rate)}" aria-hidden="true"><i style="width:${(rate * 100).toFixed(0)}%"></i></span></span>
      <span class="status-chip status-${statusLevel(rate)}">${fmt(rate * 100)}%</span>
      <span class="num">${fmtMs(x.p50_ms)}</span>
    </div>`; }).join('') : '<div class="empty">Nenhuma chamada nas últimas 24h.</div>';

  // API
  const httpTotal = sumByPrefix(m, 'hackton_http_requests_total{');
  const http2xx = sumHTTPByStatus(m, '2');
  const http5xx = sumHTTPByStatus(m, '5');
  const durSum = sumByPrefix(m, 'hackton_http_request_duration_seconds_sum{');
  const durCount = sumByPrefix(m, 'hackton_http_request_duration_seconds_count{');
  set('g-http-avg', durCount ? fmtDur(durSum / durCount) : '—');
  setHTML('g-http-sub', httpTotal ? `${statusChip(http2xx / httpTotal, '2xx')} <span style="margin-left:4px">${httpTotal} req.</span>` : 'sem requisições');
  set('g-5xx', http5xx);
  setThreshold('g-5xx', http5xx, 1, 10);

  // Sistema
  set('g-goroutines', get(m, 'hackton_worker_active_goroutines'));
  set('g-heap', fmtBytes(get(m, 'go_memstats_heap_alloc_bytes')));

  // Banco
  set('g-db-acquired', get(m, 'hackton_db_pool_acquired_conns'));
  set('g-db-conns', `de ${get(m, 'hackton_db_pool_max_conns')} · ${get(m, 'hackton_db_pool_empty_acquire_total')} esperas`);
  const rows = dbQueryRows(m);
  const total = rows.reduce((a, r) => a + r.count, 0);
  const sum = rows.reduce((a, r) => a + r.sum, 0);
  set('g-db-avg', total ? fmtDur(sum / total) : '—');
  set('g-db-queries', `${total} consultas`);
  const maxSum = rows.length ? rows[0].sum : 0;
  document.getElementById('g-db-top').innerHTML = rows.slice(0, 3).map(r => `<div class="mini-row">
      <span class="name">${escHtml(r.name)}<span class="bar bar-dom" aria-hidden="true"><i style="width:${maxSum ? (r.sum / maxSum * 100).toFixed(0) : 0}%"></i></span></span>
      <span class="num">${r.count}×</span>
      <span class="num">${fmtDur(r.sum)}</span>
    </div>`).join('') || '<div class="empty">Nenhuma consulta desde o reinício.</div>';
}
