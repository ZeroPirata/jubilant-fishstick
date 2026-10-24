// ── Ajuda: guia em slides por tópico ──
// Conteúdo curto por slide; a navegação é linear (o "Próximo" do último slide de um
// tópico abre o primeiro do seguinte). Teclado ← →, deslizar no celular.

const HELP_TOPICS = [
  {
    id: 'inicio', title: 'Começando', icon: 'i-brand',
    slides: [
      {
        title: 'O que o JobPipeline faz',
        body: '<p>Você cola o link de uma vaga. O sistema lê a vaga, compara com o seu perfil e monta um <b>currículo sob medida</b> (e a carta de apresentação, se quiser), pronto em PDF.</p>',
        visual: `<div class="hv-flow">
          ${['Vaga', 'Leitura', 'Match', 'IA', 'PDF'].map((s, i) => `<div class="hv-step" style="--i:${i}"><span>0${i + 1}</span>${s}</div>`).join('<i></i>')}
        </div>`,
      },
      {
        title: 'Tudo começa no seu perfil',
        body: `<p>A IA <b>não inventa nada</b>: ela escolhe e adapta o que está no seu perfil para cada vaga. Perfil completo, currículo melhor.</p>
          <ul><li>Experiências com conquistas concretas</li><li>Habilidades com o nível certo</li><li>Projetos e formação</li></ul>`,
        visual: `<div class="hv-stack">
          <div class="hv-card"><b>Experiências</b><span class="hv-bar" style="--w:90%"></span><span class="hv-bar" style="--w:70%"></span></div>
          <div class="hv-card"><b>Habilidades</b><span class="hv-chips"><em>Go</em><em>SQL</em><em>Docker</em></span></div>
          <div class="hv-card"><b>Projetos</b><span class="hv-bar" style="--w:60%"></span></div>
        </div>`,
        cta: { label: 'Importar meu currículo', action: 'impGoToImport()' },
      },
    ],
  },
  {
    id: 'perfil', title: 'Perfil', icon: 'i-user',
    slides: [
      {
        title: 'Importe em vez de digitar',
        body: `<p>Envie o currículo que você já tem (<b>PDF, DOCX ou TXT</b>) ou o PDF do LinkedIn (no seu perfil: <b>Mais → Salvar como PDF</b>).</p>
          <ul><li>A IA lê e separa experiências, formação e habilidades</li><li>Você confere, ajusta os níveis e escolhe o que salvar</li><li>O que já está no perfil vem desmarcado, sem duplicar</li></ul>`,
        visual: `<div class="hv-drop"><span>⇪</span><b>curriculo.pdf</b><small>lendo…</small><i class="hv-scan"></i></div>`,
        cta: { label: 'Importar agora', action: 'impGoToImport()' },
      },
      {
        title: 'Experiências que convencem',
        body: `<p>As <b>conquistas</b> viram os tópicos do currículo. Números fazem diferença.</p>
          <ul><li>Até <b>4 conquistas</b> por experiência vão para a IA: coloque as melhores primeiro</li>
          <li><b>Tecnologias</b> da experiência dizem onde você usou cada uma</li>
          <li><b>Tags</b> ajudam a decidir se a experiência entra no currículo de cada vaga</li></ul>`,
        visual: `<div class="hv-compare">
          <div class="hv-bad"><span>✕</span>Melhorei a performance do sistema</div>
          <div class="hv-good"><span>✓</span>Reduzi a latência da API em 40% com cache em Redis</div>
        </div>`,
      },
      {
        title: 'Habilidades e níveis',
        body: '<p>O nível de cada habilidade orienta a IA e entra no <b>score de qualidade</b> das vagas. Seja honesto: o currículo precisa se sustentar na entrevista.</p>',
        visual: `<div class="hv-levels">
          <div><em class="badge badge-basic">Básico</em>já usou, trabalha com ajuda</div>
          <div><em class="badge badge-intermediate">Intermediário</em>trabalha sozinho no dia a dia</div>
          <div><em class="badge badge-advanced">Avançado</em>resolve problemas complexos</div>
          <div><em class="badge badge-expert">Expert</em>referência, ensina e decide arquitetura</div>
        </div>`,
      },
      {
        title: 'Seus dados de contato',
        body: `<p>O cabeçalho do currículo usa a <b>Info Básica</b> e os <b>Links</b> do perfil.</p>
          <ul><li><b>Email de contato</b>: o que aparece no PDF. Em branco, usa o email de login</li>
          <li>LinkedIn, GitHub e portfólio viram links clicáveis no PDF</li></ul>`,
        visual: `<div class="hv-resume-head"><b>Seu Nome</b><span>contato@email.com · (11) 99999-9999</span><span class="hv-links"><u>LinkedIn</u><u>GitHub</u><u>Portfólio</u></span></div>`,
        cta: { label: 'Abrir Info Básica', action: "helpGoProfile('info')" },
      },
    ],
  },
  {
    id: 'vagas', title: 'Vagas', icon: 'i-jobs',
    slides: [
      {
        title: 'Cadastre uma vaga',
        body: `<p>Cole a URL da vaga (LinkedIn, Jobright e outros sites) e escolha:</p>
          <ul><li><b>Idioma</b> do currículo: PT-BR ou EN</li><li><b>O que gerar</b>: currículo + carta, só currículo ou só carta</li><li>Tem várias vagas? Use o <b>lote de links</b></li></ul>`,
        visual: `<div class="hv-form"><span class="hv-input">https://linkedin.com/jobs/…</span><span class="hv-row"><span class="hv-input">PT-BR</span><span class="hv-input">Currículo + Carta</span></span><span class="hv-btn">Cadastrar vaga</span></div>`,
        cta: { label: 'Cadastrar uma vaga', action: "navigateTo('nova-vaga')" },
      },
      {
        title: 'Quando o site não deixa ler a vaga',
        body: `<p>Alguns sites exigem login ou bloqueiam robôs, e a leitura automática falha.</p>
          <ul><li>Preencha <b>empresa</b>, <b>título</b> e cole a <b>descrição</b> da vaga ao cadastrar</li><li>Esses campos são opcionais quando o link funciona</li></ul>`,
        visual: `<div class="hv-form"><span class="hv-row"><span class="hv-input">Acme Corp</span><span class="hv-input">Dev Backend Pleno</span></span><span class="hv-input hv-area">Requisitos: Go, PostgreSQL, Docker…</span></div>`,
      },
      {
        title: 'Score de qualidade',
        body: '<p>Mostra quanto do que a vaga pede está nas suas <b>Habilidades</b>. Não impede a geração: serve para você priorizar onde se candidatar.</p>',
        visual: `<div class="hv-score">
          <div><em class="badge badge-low">Baixa</em><span class="hv-meter"><i style="--w:22%"></i></span>menos de 30%</div>
          <div><em class="badge badge-mid">Média</em><span class="hv-meter"><i style="--w:50%"></i></span>de 30% a 69%</div>
          <div><em class="badge badge-high">Alta</em><span class="hv-meter"><i style="--w:85%"></i></span>70% ou mais</div>
        </div>`,
      },
      {
        title: 'Acompanhe em tempo real',
        body: '<p>A lista de vagas se atualiza sozinha enquanto cada vaga avança. O indicador <b>tempo real conectado</b>, na barra lateral, mostra que as atualizações estão chegando.</p>',
        visual: `<div class="hv-status">
          ${['Pendente', 'Coletando', 'Analisando', 'Concluído'].map((s, i) => `<span class="hv-pill" style="--i:${i}">${s}</span>`).join('<i>›</i>')}
        </div>`,
      },
    ],
  },
  {
    id: 'curriculos', title: 'Currículos', icon: 'i-chart',
    slides: [
      {
        title: 'Quatro modelos de PDF',
        body: `<p>Escolha o visual na lista de cada currículo. Trocar de modelo <b>só reformata</b>: não chama a IA de novo, e a sua escolha fica lembrada.</p>
          <ul><li><b>Primeiro emprego</b>: destaca formação e projetos para quem está começando</li></ul>`,
        visual: `<div class="hv-templates">
          ${[['Clássico', ''], ['Executivo', 'hv-t-exec'], ['Compacto', 'hv-t-comp'], ['Primeiro emprego', 'hv-t-first']].map(([n, c]) => `<div class="hv-tpl ${c}"><span></span><span></span><span></span><b>${n}</b></div>`).join('')}
        </div>`,
      },
      {
        title: 'Avalie o que foi gerado',
        body: `<p>Sua avaliação ensina o sistema a escrever do seu jeito:</p>
          <ul><li><b>Excelente</b>: o currículo vira exemplo para os próximos</li><li><b>Bom</b> ou <b>Regular</b> com comentário: os comentários mais recentes entram nas próximas gerações</li></ul>`,
        visual: `<div class="hv-rating"><span>Fraco</span><span>Regular</span><span>Bom</span><span class="on">Excelente</span><em>"Tom ótimo, manter os números no topo"</em></div>`,
      },
      {
        title: 'Pronto para começar',
        body: `<ul><li>Complete o perfil antes de cadastrar vagas (importar é o caminho mais rápido)</li>
          <li>Prefira conquistas com números</li><li>Confira o email de contato</li><li>Avalie os currículos bons como Excelente</li></ul>`,
        visual: '<div class="hv-done"><span>✓</span></div>',
        cta: { label: 'Ir para as vagas', action: "navigateTo('vagas')" },
      },
    ],
  },
];

const HELP_KEY = 'jp_help_pos';
const _help = { t: 0, s: 0, bound: false };
const _helpFlat = HELP_TOPICS.flatMap((t, ti) => t.slides.map((_, si) => [ti, si]));

function helpGoProfile(sub) {
  _activePerfil = sub;
  navigateTo('perfil');
}

function loadHelp() {
  if (!_help.bound) {
    _help.bound = true;
    try { Object.assign(_help, JSON.parse(localStorage.getItem(HELP_KEY) || '{}')); } catch (_) {}
    helpBindInput();
  }
  // Posição inválida ou guia já concluído: recomeça do início.
  const last = _helpFlat[_helpFlat.length - 1];
  if (!HELP_TOPICS[_help.t]?.slides[_help.s] || (_help.t === last[0] && _help.s === last[1])) Object.assign(_help, { t: 0, s: 0 });
  helpRenderTopics();
  helpRender(0);
}

function helpRenderTopics() {
  document.getElementById('help-topics').innerHTML = HELP_TOPICS.map((t, i) => `
    <button role="tab" class="help-topic ${i === _help.t ? 'active' : ''}" aria-selected="${i === _help.t}" onclick="helpGo(${i}, 0)">
      <svg class="ico"><use href="#${t.icon}"/></svg>
      <span class="help-topic-text"><b>${t.title}</b><small>${t.slides.length} ${t.slides.length === 1 ? 'passo' : 'passos'}</small></span>
      <span class="help-topic-ticks">${t.slides.map((_, si) => `<i class="${i < _help.t || (i === _help.t && si <= _help.s) ? 'on' : ''}"></i>`).join('')}</span>
    </button>`).join('');
}

function helpRender(dir) {
  const topic = HELP_TOPICS[_help.t];
  const slide = topic.slides[_help.s];
  const pos = _helpFlat.findIndex(([t, s]) => t === _help.t && s === _help.s);

  const el = document.getElementById('help-slide');
  el.className = 'help-slide' + (dir > 0 ? ' from-right' : dir < 0 ? ' from-left' : '');
  el.innerHTML = `
    <div class="help-visual">${slide.visual}</div>
    <div class="help-copy">
      <div class="card-eyebrow">${topic.title} · ${_help.s + 1} de ${topic.slides.length}</div>
      <h3>${slide.title}</h3>
      ${slide.body}
      ${slide.cta ? `<button class="btn btn-ghost help-cta" onclick="${slide.cta.action}">${slide.cta.label} →</button>` : ''}
    </div>`;

  document.getElementById('help-dots').innerHTML = topic.slides.map((_, i) =>
    `<button class="${i === _help.s ? 'on' : ''}" aria-label="Passo ${i + 1}" onclick="helpGo(${_help.t}, ${i})"></button>`).join('');
  document.getElementById('help-count').textContent = `${pos + 1} / ${_helpFlat.length}`;
  document.getElementById('help-progress-bar').style.width = `${((pos + 1) / _helpFlat.length) * 100}%`;
  document.getElementById('help-prev').disabled = pos === 0;
  const next = document.getElementById('help-next');
  const last = pos === _helpFlat.length - 1;
  next.disabled = last;
  next.textContent = last ? 'Fim' : (_help.s === topic.slides.length - 1 ? `${HELP_TOPICS[_help.t + 1].title} ›` : 'Próximo ›');
  try { localStorage.setItem(HELP_KEY, JSON.stringify({ t: _help.t, s: _help.s })); } catch (_) {}
}

function helpGo(t, s) {
  const from = _helpFlat.findIndex(([a, b]) => a === _help.t && b === _help.s);
  const to = _helpFlat.findIndex(([a, b]) => a === t && b === s);
  Object.assign(_help, { t, s });
  helpRenderTopics();
  helpRender(Math.sign(to - from));
}

function helpStep(delta) {
  const pos = _helpFlat.findIndex(([t, s]) => t === _help.t && s === _help.s) + delta;
  if (pos < 0 || pos >= _helpFlat.length) return;
  helpGo(..._helpFlat[pos]);
}

function helpBindInput() {
  document.addEventListener('keydown', e => {
    if (_activeTab !== 'ajuda' || e.target.closest('input, textarea, select')) return;
    if (e.key === 'ArrowRight') helpStep(1);
    if (e.key === 'ArrowLeft') helpStep(-1);
  });
  // Deslizar no celular: arrasto horizontal de 50px ou mais troca o slide.
  const stage = document.querySelector('.help-stage');
  let x0 = null, y0 = null;
  stage.addEventListener('touchstart', e => { x0 = e.touches[0].clientX; y0 = e.touches[0].clientY; }, { passive: true });
  stage.addEventListener('touchend', e => {
    if (x0 === null) return;
    const dx = e.changedTouches[0].clientX - x0, dy = e.changedTouches[0].clientY - y0;
    if (Math.abs(dx) > 50 && Math.abs(dx) > Math.abs(dy) * 1.5) helpStep(dx < 0 ? 1 : -1);
    x0 = null;
  }, { passive: true });
}
