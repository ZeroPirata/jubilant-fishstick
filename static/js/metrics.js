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
    const res = await fetch('/metrics');
    if (!res.ok) throw new Error('status ' + res.status);
    const text = await res.text();
    const m = parsePrometheus(text);
    renderMetrics(m);
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
  set('m-jobs-total',     total > 0 ? `${Math.round(completed / total * 100)}% taxa de sucesso` : '—');

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

function startMetricsRefresh() {
  loadMetrics();
  _metricsTimer = setInterval(loadMetrics, 30_000);
}

function stopMetricsRefresh() {
  clearInterval(_metricsTimer);
  _metricsTimer = null;
}
