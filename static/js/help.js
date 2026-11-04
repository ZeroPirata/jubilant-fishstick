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
        title: 'Compatibilidade',
        body: `<p>Mostra quanto da stack da vaga está nas suas <b>Habilidades</b>, pesando o nível: expert e avançado contam inteiro, intermediário 75%, básico 40%.</p>
          <ul><li>É a mesma conta na aba Vagas, na tela da vaga e no Descobrir</li><li>Não impede a geração: serve para você priorizar onde se candidatar</li></ul>`,
        visual: `<div class="hv-score">
          <div><em class="badge badge-low">Baixa</em><span class="hv-meter"><i style="--w:22%"></i></span>menos de 30%</div>
          <div><em class="badge badge-mid">Média</em><span class="hv-meter"><i style="--w:50%"></i></span>de 30% a 69%</div>
          <div><em class="badge badge-high">Alta</em><span class="hv-meter"><i style="--w:85%"></i></span>70% ou mais</div>
        </div>`,
      },
      {
        title: 'A tela da vaga',
        body: `<p>Clique numa vaga para ver tudo dela em quatro abas (use ← → para trocar):</p>
          <ul><li><b>Compatibilidade</b>: análise da IA (pontos fortes, lacunas, recomendação) e stack × perfil</li>
          <li><b>Palavras-chave</b>: os termos da vaga que o currículo cita, os que você tem no perfil mas ficaram de fora e os que o perfil não tem (sem IA)</li>
          <li><b>Currículos e cover letters</b>: PDFs, modelos, avaliação e apagar vários de uma vez</li>
          <li><b>Sobre a vaga</b>: resumo, descrição completa e de onde ela veio</li></ul>
          <p>O endereço (<code>#/vaga/…</code>) pode ser salvo ou aberto em outra aba; o F5 volta para a mesma tela.</p>`,
        visual: `<div class="hv-status">
          ${['Compatibilidade', 'Palavras-chave', 'Currículos', 'Sobre a vaga'].map((s, i) => `<span class="hv-pill" style="--i:${i}">${s}</span>`).join('<i>·</i>')}
        </div>`,
      },
      {
        title: 'Acompanhe em tempo real',
        body: '<p>A lista de vagas se atualiza sozinha enquanto cada vaga avança. O indicador <b>tempo real conectado</b>, na barra lateral, mostra que as atualizações estão chegando.</p>',
        visual: `<div class="hv-status">
          ${['Pendente', 'Coletando', 'Analisando', 'Concluído'].map((s, i) => `<span class="hv-pill" style="--i:${i}">${s}</span>`).join('<i>›</i>')}
        </div>`,
      },
      {
        title: 'Filtre a sua lista',
        body: `<p>Acima da lista de vagas:</p>
          <ul><li><b>% da stack</b>: só as vagas em que você cobre pelo menos essa parte das tecnologias (calculado com as suas habilidades de agora)</li>
          <li><b>Candidatura</b>: as mesmas etapas do quadro, de “pronto, não enviado” a “encerrada”</li>
          <li><b>Origem</b>: as escolhidas pela Seleção com IA ou as que você cadastrou</li>
          <li><b>Ordenar por maior %</b>: as mais compatíveis primeiro</li></ul>
          <p>Os filtros ficam lembrados neste navegador; o filtro em uso fica destacado e <b>Limpar filtros</b> volta tudo.</p>`,
        visual: `<div class="hv-status">
          ${['% da stack', 'Candidatura', 'Origem', 'Ordenar'].map((s, i) => `<span class="hv-pill" style="--i:${i}">${s}</span>`).join('<i>·</i>')}
        </div>`,
      },
    ],
  },
  {
    id: 'descobrir', title: 'Descobrir', icon: 'i-search',
    slides: [
      {
        title: 'Vagas que vêm até você',
        body: `<p>O Descobrir mantém um <b>catálogo próprio</b> de vagas de tecnologia, lido de fontes públicas:</p>
          <ul><li>Gupy, ProgramaThor, repositórios de vagas do GitHub e Himalayas (remotas)</li>
          <li>Quadros de vagas de empresas nos ATS: Greenhouse, Lever, Ashby, Workable, SmartRecruiters e Recruitee</li>
          <li>Atualiza sozinho a cada 12 horas; o botão de busca procura na hora com os seus termos</li></ul>`,
        visual: `<div class="hv-stack">
          <div class="hv-card"><b>Sites de vagas</b><span class="hv-chips"><em>Gupy</em><em>GitHub</em><em>Himalayas</em></span></div>
          <div class="hv-card"><b>Empresas (ATS)</b><span class="hv-chips"><em>Greenhouse</em><em>Lever</em><em>Ashby</em></span></div>
          <div class="hv-card"><b>Catálogo</b><span class="hv-bar" style="--w:85%"></span></div>
        </div>`,
        cta: { label: 'Abrir o Descobrir', action: "navigateTo('descobrir')" },
      },
      {
        title: 'Ordenadas pelo que você sabe',
        body: `<p>Cada vaga mostra a <b>compatibilidade</b> com o seu perfil: quanto da stack você cobre (pesando o nível), quantas tecnologias são suas e o que falta.</p>
          <ul><li>A lista vem das mais compatíveis para as menos</li><li>O que falta é um bom roteiro de estudo</li>
          <li>Antes de gerar, é uma estimativa pelas tecnologias citadas no texto; depois de gerar, vale a leitura completa da vaga pela IA (o mesmo número da aba Vagas)</li></ul>`,
        visual: `<div class="hv-stack">
          <div class="disc-fit"><span class="disc-fit-pct disc-fit-high">82% da stack</span><span>você cobre 5 de 6</span> · <span class="disc-fit-gaps">falta: Kafka</span></div>
          <div class="disc-fit"><span class="disc-fit-pct disc-fit-mid">55% da stack</span><span>você cobre 3 de 5</span> · <span class="disc-fit-gaps">falta: AWS, Terraform</span></div>
          <div class="disc-fit"><span class="disc-fit-pct disc-fit-low">20% da stack</span><span>você cobre 1 de 5</span></div>
        </div>`,
      },
      {
        title: 'Filtre do seu jeito',
        body: `<p>Sem nada escolhido, a lista usa as <b>habilidades do seu perfil</b>. Escolha tecnologias para ver vagas fora dele.</p>
          <ul><li>"golang" e "go" contam como Go; "go-to" não conta</li>
          <li>Habilidades fora da lista de tecnologias ("APIs REST") são procuradas no texto da vaga</li>
          <li>Filtre por site, nível (júnior, pleno, sênior), local, modo de trabalho e vagas do exterior</li></ul>`,
        visual: `<div class="hv-form"><span class="hv-row"><span class="hv-input">Go</span><span class="hv-input">PostgreSQL</span><span class="hv-input">+</span></span><span class="hv-row"><span class="hv-input">Pleno</span><span class="hv-input">Remoto</span></span></div>`,
      },
      {
        title: 'Da vaga ao currículo',
        body: `<p><b>Ver mais</b> (o olhinho no celular) abre a página da vaga: compatibilidade e descrição completa, para decidir antes de gerar. <b>Gerar currículo</b> manda a vaga para a sua lista. Você escolhe o idioma (o da vaga vem sugerido) e o que gerar.</p>
          <ul><li>Quando fica pronto, o botão vira <b>Ver currículo</b></li>
          <li>Se o site da vaga bloquear a leitura, o sistema usa a descrição guardada no catálogo</li></ul>`,
        visual: `<div class="hv-status">
          ${['Gerar currículo', 'Na fila', 'Ver currículo'].map((s, i) => `<span class="hv-pill" style="--i:${i}">${s}</span>`).join('<i>›</i>')}
        </div>`,
      },
      {
        title: 'Avisos de vagas novas',
        body: `<p>O <b>sininho</b> no topo conta as vagas que entraram no catálogo desde a sua última olhada e combinam com você.</p>
          <ul><li>Escolha avisar só das de compatibilidade <b>alta</b> (padrão) ou também das <b>médias</b>, ou desligar</li>
          <li>Valem o local e o nível que você salvou no Descobrir</li>
          <li>Tocar numa vaga do sininho abre a página dela, com 👍/👎 para dizer se o aviso foi útil</li>
          <li>Abrir o sininho ou o Descobrir marca como vistas: a mesma vaga não é avisada de novo</li></ul>`,
        visual: `<div class="hv-status">
          ${['varredura', 'vaga nova', 'compatível?', 'sininho'].map((s, i) => `<span class="hv-pill" style="--i:${i}">${s}</span>`).join('<i>›</i>')}
        </div>`,
      },
      {
        title: 'Catálogo sempre atual',
        body: `<ul><li>Vaga que some das buscas sai da lista em <b>36 horas</b> e é apagada em <b>7 dias</b></li>
          <li>Vaga publicada há mais de <b>45 dias</b> não entra: muitos sites deixam a vaga no ar depois de preenchida</li>
          <li>Vagas repetidas (mesmo título e empresa) aparecem uma vez só</li></ul>`,
        visual: `<div class="hv-status">
          ${['nova', 'ativa', 'sumiu 36h', 'apagada 7d'].map((s, i) => `<span class="hv-pill" style="--i:${i}">${s}</span>`).join('<i>›</i>')}
        </div>`,
      },
    ],
  },
  {
    id: 'candidaturas', title: 'Candidaturas', icon: 'i-board',
    slides: [
      {
        title: 'Em que etapa está cada currículo',
        body: `<p>A aba <b>Candidaturas</b> é um quadro: <b>Currículo pronto → Enviado → Entrevista → Proposta → Encerradas</b>.</p>
          <ul><li>Currículo gerado entra sozinho em "Currículo pronto"</li>
          <li>Depois de mandar para a empresa, um clique em <b>Marquei como enviado</b> (ou arraste o cartão, no computador)</li>
          <li>Errou? O aviso tem <b>Desfazer</b>, e o histórico da vaga deixa apagar qualquer item</li></ul>`,
        visual: `<div class="hv-flow">
          ${['Pronto', 'Enviado', 'Entrevista', 'Proposta', 'Fim'].map((s, i) => `<div class="hv-step" style="--i:${i}"><span>0${i + 1}</span>${s}</div>`).join('<i></i>')}
        </div>`,
        cta: { label: 'Abrir Candidaturas', action: "navigateTo('candidaturas')" },
      },
      {
        title: 'Pouca anotação manual',
        body: `<p>O quadro faz o máximo sozinho:</p>
          <ul><li>Ao baixar o PDF, ele pergunta se você já enviou</li>
          <li>A data é a de hoje; dá para escolher outra se você esqueceu de marcar</li>
          <li>Anotação é opcional (com quem falou, próxima etapa, salário)</li>
          <li>Enviado há ${APP_STALE_DAYS}+ dias sem resposta fica destacado, com <b>Encerrar</b> num clique</li></ul>
          <p>No topo: quantas você enviou e quantas chegaram a entrevista.</p>`,
      },
    ],
  },
  {
    id: 'selecao', title: 'Seleção com IA', icon: 'i-spark',
    slides: [
      {
        title: 'A IA escolhe as vagas por você',
        body: `<p>Em <b>Vagas</b>, o botão <b>Seleção com IA</b> (quando liberado para a sua conta):</p>
          <ul><li>pega as vagas do Descobrir mais compatíveis com o seu perfil, com o local e o nível que você salvou (sem nível salvo, usa o seu tempo de experiência: estágio e vagas muito acima ficam de fora)</li>
          <li>a IA lê cada uma como um recrutador do seu lado: senioridade pedida, idioma, requisitos eliminatórios, tipo de função</li>
          <li>as regras de corte são fixas, não dependem do humor da IA: 2+ requisitos obrigatórios que você não tem, anos pedidos bem acima do seu tempo de carreira ou vaga híbrida/presencial quando você quer remoto tiram a vaga, mesmo com nota boa</li>
          <li>as que valem a candidatura entram na pipeline com a tag <b>auto-gerado</b> e o currículo é gerado sozinho</li></ul>
          <p>Você só confere e envia no site da empresa: a candidatura em si não dá para fazer por você.</p>`,
        visual: `<div class="hv-flow">
          ${['Descobrir', 'Triagem IA', 'Pipeline', 'Currículo', 'Você envia'].map((s, i) => `<div class="hv-step" style="--i:${i}"><span>0${i + 1}</span>${s}</div>`).join('<i></i>')}
        </div>`,
      },
      {
        title: 'Transparente e com limite',
        body: `<ul><li>Cada vaga lida ganha uma <b>nota da IA</b> (0 a 100) e um <b>motivo</b>, inclusive as que ficaram de fora</li>
          <li>Na tela da vaga, a aba Compatibilidade mostra por que a IA escolheu e, quando há, o principal <b>risco</b></li>
          <li>A nota da IA não é a <b>% da stack</b>: a nota julga a vaga inteira (senioridade, anos pedidos, requisitos, área); a % só compara as tecnologias com as suas habilidades</li>
          <li>Há um limite por clique e por dia; a mesma vaga não é lida de novo por 14 dias</li>
          <li>Não gostou de uma escolha? Exclua a vaga: isso também ajuda a medir a qualidade da seleção</li></ul>`,
      },
    ],
  },
  {
    id: 'bastidores', title: 'Por dentro', icon: 'i-shield',
    slides: [
      {
        title: 'As etapas de cada vaga',
        body: `<ol class="help-steps">
          <li><b>Leitura</b>: o texto do site da vaga; se o site bloquear, a descrição que você colou ou a do catálogo</li>
          <li><b>Seleção</b>: as experiências, projetos e habilidades do seu perfil que combinam com a vaga</li>
          <li><b>Compatibilidade</b>: stack da vaga × suas habilidades</li>
          <li><b>Escrita</b>: a IA escreve resumo, tópicos, carta e a análise da vaga</li>
          <li><b>Montagem e conferência</b>: o sistema junta tudo, confere e gera o PDF</li></ol>`,
        visual: `<div class="hv-flow">
          ${['Leitura', 'Seleção', 'Fit', 'IA', 'Conferência', 'PDF'].map((s, i) => `<div class="hv-step" style="--i:${i}"><span>0${i + 1}</span>${s}</div>`).join('<i></i>')}
        </div>`,
      },
      {
        title: 'A IA escreve, os fatos são seus',
        body: `<p>A IA só recebe o que está no seu perfil e devolve <b>texto</b>. Empresa, cargo, datas, formação e contato são colocados pelo sistema a partir do seu cadastro: a IA não consegue mudá-los.</p>
          <p>Por isso o perfil é tão importante: nada entra no currículo que não esteja nele.</p>`,
        visual: `<div class="hv-compare">
          <div class="hv-good"><span>IA</span>resumo, tópicos das experiências, carta</div>
          <div class="hv-good"><span>Você</span>empresas, cargos, datas, formação, contato</div>
        </div>`,
      },
      {
        title: 'Conferências antes de entregar',
        body: `<ul><li>Resumo que cita tecnologia que você não tem (ou anos de experiência em primeiro emprego) volta para a IA corrigir; o que continuar errado é cortado</li>
          <li>Ponto forte da análise que cita tecnologia fora do seu perfil é removido</li>
          <li>Em inglês, cargos e habilidades são traduzidos, mas tradução que sobe o nível ("Júnior" → "Senior") ou acrescenta coisas é descartada: fica o original</li></ul>`,
        visual: `<div class="hv-compare">
          <div class="hv-bad"><span>✕</span>"Experiência sólida com Kubernetes" (fora do perfil)</div>
          <div class="hv-good"><span>✓</span>"Experiência com Docker e AWS em produção"</div>
        </div>`,
      },
      {
        title: 'Quem decide é você',
        body: `<p>A compatibilidade e a análise da IA <b>só informam</b>: o currículo é gerado em qualquer nível.</p>
          <ul><li>A análise traz pontos fortes, lacunas e uma recomendação</li>
          <li>Vaga com compatibilidade baixa pode valer a pena; a decisão é sua</li></ul>`,
        visual: `<div class="hv-score">
          <div><em class="badge badge-high">Vale se candidatar</em>bom encaixe</div>
          <div><em class="badge badge-mid">Com ressalvas</em>vale tentar, sabendo das lacunas</div>
          <div><em class="badge badge-low">Baixa aderência</em>lacunas grandes</div>
        </div>`,
      },
      {
        title: 'Refazer sem gastar à toa',
        body: `<p>No <b>Refazer</b> você escolhe idioma, o que gerar e se quer uma <b>Nova versão da IA</b>.</p>
          <ul><li>Desmarcada: se nada mudou (vaga, perfil, avaliações, idioma e modo), a última resposta da IA é reaproveitada na hora, sem gastar cota</li>
          <li>Trocar o modelo do PDF nunca chama a IA</li></ul>`,
        visual: `<div class="hv-form"><span class="hv-row"><span class="hv-input">EN</span><span class="hv-input">☑ Nova versão da IA</span></span><span class="hv-btn">Currículo</span></div>`,
      },
      {
        title: 'Vagas buscadas com respeito',
        body: `<ul><li>Só APIs e quadros de vagas <b>públicos</b>, lidos sem login de ninguém e sem burlar bloqueio de sites</li>
          <li>O LinkedIn não é raspado: os termos dele proíbem</li>
          <li>O link leva sempre à vaga original: a candidatura é feita por você no site da empresa</li></ul>`,
        visual: '<div class="hv-done"><span>✓</span></div>',
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
        title: 'Baixe pronto para enviar',
        body: `<p>Sem renomear arquivo antes de mandar:</p>
          <ul><li><b>Baixar tudo (.zip)</b>: currículo e cover letter juntos, num .zip com o nome da empresa e da vaga</li>
          <li>O currículo sai com o <b>seu nome</b> (nome e sobrenome do perfil); a carta, como <code>cover_letter.pdf</code></li>
          <li>Os botões avulsos continuam lá, para portais que pedem só o currículo</li></ul>
          <p>Nomes em minúsculas, sem acento e curtos.</p>`,
        visual: `<div class="hv-compare">
          <div class="hv-bad"><span>✕</span>curriculo.pdf · cover_letter.pdf</div>
          <div class="hv-good"><span>✓</span>nubank_desenvolvedor_back_end.zip<br>└ gabriel_nunes.pdf<br>└ cover_letter.pdf</div>
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
