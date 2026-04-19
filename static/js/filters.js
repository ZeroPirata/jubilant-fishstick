// ── Keyword Filters ──

async function loadFilters() {
  const tbody = document.getElementById('filters-body');
  try {
    const res = await api.get('/filters');
    const d = await apiData(res);
    const rows = Array.isArray(d) ? d : [];
    if (!rows.length) { tbody.innerHTML = '<tr><td colspan="2" class="empty">Nenhum filtro configurado.</td></tr>'; return; }
    tbody.innerHTML = rows.map(f => `
      <tr>
        <td>${f.keyword}</td>
        <td><button class="btn-delete" onclick="deleteFilter('${f.id}',this)">✕</button></td>
      </tr>
    `).join('');
  } catch (_) { tbody.innerHTML = '<tr><td colspan="2" class="empty">Erro ao carregar.</td></tr>'; }
}

async function addFilter() {
  const keyword = document.getElementById('filter-keyword').value.trim();
  if (!keyword) { toast('Keyword é obrigatória.', 'error'); return; }
  try {
    const res = await api.post('/filters', { keyword });
    if (!res.ok) { toast('Erro ao adicionar filtro.', 'error'); return; }
    document.getElementById('filter-keyword').value = '';
    toast('Filtro adicionado!');
    if (typeof _filtersCache !== 'undefined') _filtersCache = null;
    loadFilters();
  } catch (e) { toast(e.message, 'error'); }
}

async function deleteFilter(id, btn) {
  if (!confirm('Deletar filtro?')) return;
  btn.disabled = true;
  try {
    const res = await api.delete(`/filters/${id}`);
    if (!res.ok) { toast('Erro ao deletar.', 'error'); btn.disabled = false; return; }
    btn.closest('tr').remove();
    if (typeof _filtersCache !== 'undefined') _filtersCache = null;
    toast('Filtro removido.');
  } catch (e) { toast(e.message, 'error'); btn.disabled = false; }
}
