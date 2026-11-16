# Changelog

O que cada versão do site mudou, da mais nova para a mais antiga. Toda versão publicada ganha uma entrada aqui antes de subir.

## 0.5.0 (16/11/2026)
Buscas no LinkedIn.

- Novo bloco **Buscas no LinkedIn** no Descobrir: salve buscas com os filtros que quiser (palavras-chave, local, data de publicação, modelo, nível, tipo de vaga, candidatura simplificada, menos de 10 candidaturas, na minha rede e ordem) e abra no LinkedIn com um clique, já logado no seu navegador.
- Nenhum filtro vem marcado numa busca nova.
- Cole uma busca que você já usa no LinkedIn e os filtros dela são lidos. É assim que a cidade entra, porque o código do local só existe na URL do LinkedIn.
- O JobPipeline só monta o link: ele não acessa o LinkedIn.

## 0.4.10 (16/11/2026)
Novidades: o histórico de versões no painel admin.

- Nova aba **Novidades** no painel admin, só para administradores e masters: o que mudou em cada versão, da mais nova para a mais antiga. As três últimas aparecem abertas e as outras ficam em "Versões anteriores".
- O selo da versão na barra lateral (master) leva direto às Novidades.
- O texto vem do `CHANGELOG.md` do repositório, que agora traz o histórico desde a 0.2.0. O deploy não sobe uma versão sem entrada nele.

## 0.4.9 (04/11/2026)
Novo endereço do site.

- O site passa a ser `jobpipeline.gsbnunes.com.br`. Quem acessar o endereço antigo (`gerador-curriculo.gsbnunes.com.br`) é levado para o novo, na mesma página.

## 0.4.8 (04/11/2026)
Seleção com IA mais justa com o que falta na vaga.

- O que a vaga pede e você não tem agora tem peso: requisito central pesa mais que secundário. Dois centrais faltando (ou um central e dois secundários, ou quatro secundários) tiram a vaga; um central limita a nota a 65; cada secundário tira 5 pontos.
- O que a IA diz que falta é conferido contra as suas habilidades, com o mesmo cálculo da % da stack. Se você tem em nível intermediário ou acima, deixa de contar como falta.
- "Kafka ou similar" é atendido por algo equivalente, e só conta como falta o que está nos requisitos, não nas responsabilidades.
- Quando a mesma vaga aparece em mais de um site, a IA lê a descrição mais completa.

## 0.4.7 (04/11/2026)
Seleção com IA respeita vagas afirmativas e o nível.

- Vagas afirmativas (exclusivas para mulheres, pessoas negras, PcD, trans/LGBT) não são escolhidas. Textos padrão de diversidade, que não restringem a vaga, não contam.
- Vaga de nível vizinho ao seu só entra com nota 80+ e sem nada obrigatório faltando. A seleção pode escolher menos vagas do que o pedido.

## 0.4.6 (04/11/2026)
Seleção com IA: a IA extrai os fatos, o sistema decide.

- A IA agora devolve anos pedidos, modelo de trabalho e o que falta, e o código aplica as regras: anos acima de 1,5× a sua carreira, modelo fora do local escolhido ou dois obrigatórios faltando tiram a vaga; um faltando limita a nota a 65.
- Antes a IA podia aprovar uma vaga apontando ela mesma que era presencial ou que pedia uma stack que você não tem.
- Vagas repetidas com o nome da empresa escrito diferente são lidas uma vez só.
- Cargos não técnicos (vendas, RH) nem chegam a ir para a IA.

## 0.4.5 (04/11/2026)
Seleção com IA mais precisa no nível.

- Sem nível salvo no Descobrir, o nível vem do seu tempo de carreira: primeiro o nível certo, os vizinhos completam; estágio, especialista e gestão ficam de fora.
- A IA passa a apontar quando algo é eliminatório, e a vaga não entra.
- Habilidade básica não conta como domínio, e anos pedidos muito acima dos seus eliminam a vaga.

## 0.4.4 (04/11/2026)
Filtros novos na aba Vagas.

- Filtrar por % da stack, por etapa da candidatura e por origem (Seleção com IA ou manual), e ordenar pela %.
- Os filtros ficam lembrados no navegador, o filtro ativo aparece destacado e há um botão Limpar filtros.

## 0.4.3 (04/11/2026)
Motivo da Seleção com IA mais claro.

- Vaga aprovada mostra o que faz valer a pena e o risco; vaga recusada mostra o que a eliminou.
- A tela da vaga mostra a nota da IA (0–100) separada da % da stack, com explicação da diferença.
- Ajuda atualizada com o download em .zip e a diferença entre as duas notas.

## 0.4.2 (04/11/2026)
Currículo e carta num .zip pronto para enviar.

- Botão "Baixar tudo (.zip)": `empresa_vaga.zip` com `nome_sobrenome.pdf` e `cover_letter.pdf`.
- O currículo baixado sozinho também já sai com o seu nome.

## 0.4.1 (04/11/2026)
Correções no deploy e no login.

- A verificação depois do deploy passou a usar `/healthz` (confere banco e Redis). Antes ela fazia login com uma conta inexistente e gerava erro e evento de segurança falsos.
- Falha de banco no login responde erro 500 em vez de "senha errada".

## 0.4.0 (04/11/2026)
Seleção com IA.

- Botão na aba Vagas: a IA lê as vagas mais compatíveis do Descobrir, dá nota e motivo para cada uma e coloca as aprovadas na pipeline com a tag "auto-gerado".
- Limites por clique e por dia, acesso liberado por usuário e configuração no painel admin, com cadeia de IA própria.
- Nova seção nas Métricas com o que a seleção escolheu, enviou, descartou e quanto custou.

## 0.3.8 (04/11/2026)
Quadro de candidaturas.

- Nova aba Candidaturas com colunas por etapa: pronto, enviado, entrevista, proposta e encerradas.
- Histórico por vaga com data e anotação; avançar de etapa em um clique, com Desfazer.
- Ao baixar o PDF, o site pergunta se você já enviou. Candidaturas sem retorno há 21 dias ficam destacadas.

## 0.3.7 (04/11/2026)
Página "Sobre a vaga" e listas compactas.

- Cada vaga do Descobrir tem página própria, com compatibilidade, descrição, gerar currículo, dispensar e avaliação do alerta. O sino abre essa página.
- A compatibilidade no Descobrir e no sino é a mesma da aba Vagas quando a vaga já foi gerada.
- A aba Vagas troca a tabela de 8 colunas por uma lista compacta; na tela da vaga, os currículos viram itens clicáveis.

## 0.3.6 (03/11/2026)
Currículo e carta com voz única.

- Tópicos em 1ª pessoa do passado ("Desenvolvi", não "Desenvolveu") e o mesmo cargo, anos e nomes de tecnologia em todas as seções.
- O que o resumo destaca aparece nos tópicos, sem fato ou verbo repetido.
- Carta em 1ª pessoa, de 150 a 250 palavras, com uma prova tirada da sua experiência. As mesmas regras valem em inglês.

## 0.3.5 (03/11/2026)
Nível "Gestão" no Descobrir.

- Cargos de gestão (manager, diretor, head, coordenador…) ganham nível próprio e podem ser deixados de fora. Eram quase metade das vagas sem nível.

## 0.3.4 (03/11/2026)
Numeral romano define o nível.

- "Analista I", "Engineer IV" e parecidos passam a ter nível (I = júnior, IV = sênior).

## 0.3.3 (03/11/2026)
Correções no celular.

- A escolha de idioma e do que gerar cabe no cartão (antes sumiam opções).
- Tocar em Gerar currículo não fecha mais o sino.

## 0.3.2 (03/11/2026)
Celular mais enxuto.

- Cartões de Métricas e do admin mostram só o resumo no celular; um toque abre o resto.
- Situação do worker (ativo ou pausado por limite da IA) em Métricas → Sistema.
- Preferências e Filtros do Descobrir recolhíveis, com as explicações atrás de "?".

## 0.3.1 (03/11/2026)
Alertas sem ruído e dispensar vagas.

- Alertas só de vagas dos últimos 7 dias que pedem uma habilidade sua em nível avançado ou expert. Empresa com 3 ou mais vagas novas vira um grupo.
- 👍/👎 em cada alerta, com o resultado nas Métricas.
- Dispensar ou restaurar uma vaga do Descobrir só para você.
- A mesma vaga em vários sites aparece uma vez, com "também em: …".
- Correções no celular, e "Go High Level" (um CRM) não conta mais como Go.

## 0.3.0 (31/10/2026)
Palavras-chave e alertas de vagas novas.

- Aba Palavras-chave na tela da vaga: termos da vaga × último currículo × perfil, sem IA, para passar nos filtros de ATS.
- Sino com vagas novas do Descobrir de compatibilidade alta, sem repetir.
- Deploy mais leve: testes e imagem rodam no GitHub e o servidor só baixa e reinicia.

## 0.2.3 (31/10/2026)
Ajuda e README.

- A Ajuda ganha os tópicos "Descobrir" e "Por dentro" (o que acontece com cada vaga, o que a IA faz e não faz).
- README reescrito.

## 0.2.2 (31/10/2026)
Mais empresas no Descobrir.

- Empresas cadastradas no Workable e no Recruitee.

## 0.2.1 (31/10/2026)
Empresas por ATS no painel.

- O painel mostra quantas empresas cada ATS tem e avisa quando não há nenhuma.

## 0.2.0 (31/10/2026)
Versão do app.

- O site passa a ter versão (`version.json`), visível para o admin master. O deploy só sobe com versão nova.
- Correção nas abas do admin (Usuários, Importações e Configurações de IA ficavam fora do lugar).

## Antes da 0.2.0 (23–30/10/2026)
O que já existia quando o versionamento começou.

- Currículo e carta sob medida a partir do link da vaga, com a IA escrevendo o texto e os fatos vindo do perfil.
- Quatro modelos de PDF (Clássico, Executivo, Compacto e Primeiro emprego) e currículo em inglês com tradução conferida.
- Importar o perfil a partir de um currículo (PDF, DOCX ou LinkedIn), com idiomas no perfil.
- Painel de IA com vários provedores e troca automática quando um falha; métricas de IA, banco e usuários.
- Aba Descobrir com vagas da Gupy, ProgramaThor, GitHub, Himalayas e dos ATS (Greenhouse, Lever, Ashby, Workable, SmartRecruiters, Recruitee), ordenadas pela compatibilidade.
- Ajuda em slides e o visual de console de pipeline.
