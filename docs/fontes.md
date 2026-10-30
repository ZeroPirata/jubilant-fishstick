# Fontes de vagas do Descobrir

Regra: só entram fontes com API ou feed **público e documentado**, lidos sem chave de
outra pessoa, sem disfarçar o cliente (sem User-Agent falso, sem contornar bloqueio
anti-robô) e com link para a vaga original. Fonte cujos termos proíbem acesso
automatizado ou agregação fica de fora. LinkedIn fica de fora.

Comportamento comum a todas:
- Ritmo baixo: pausa entre pedidos ao mesmo site (`boardDelay`, 300 ms entre empresas)
  e respeito ao `429`/`Retry-After` (`getWithRetry`).
- Só o necessário: título, empresa, local, link, data e descrição, para filtrar e
  mostrar a vaga. O usuário se candidata no site de origem.
- Vagas publicadas há mais de 45 dias não entram (`MaxJobAge`); as que somem das
  buscas saem do catálogo em 7 dias.

## ATS (quadros de vagas por empresa)

Não têm busca geral: cada empresa tem o seu quadro, cadastrado no painel (Admin →
Descoberta → Empresas). O painel confere a empresa em todos os ATS pelo nome ou pelo
link.

Verificação dos termos (2026-10-30): as páginas de documentação de Ashby, Workable,
SmartRecruiters e Recruitee descrevem o endpoint público como recurso para a empresa
exibir as próprias vagas, sem chave. Nenhuma página lida proíbe ou autoriza
explicitamente o uso por terceiros, nem fala de limite de pedidos ou atribuição; os
termos de API específicos não foram encontrados (o link de termos do Workable
devolveu 404). É a mesma situação de Greenhouse e Lever, já em uso. Por isso o uso é
conservador: só empresas cadastradas, link para a vaga original, sem credenciais,
ritmo baixo. Se algum fornecedor publicar restrição, a fonte é desligada no painel e
removida.

| Fonte | Endpoint | Descrição | Observações |
|---|---|---|---|
| Greenhouse | `boards-api.greenhouse.io/v1/boards/<token>/jobs?content=true` | sim | Job Board API pública. |
| Lever | `api.lever.co/v0/postings/<token>?mode=json` | sim | Postings API pública. |
| Ashby | `api.ashbyhq.com/posting-api/job-board/<token>` | sim | [Docs](https://developers.ashbyhq.com/docs/public-job-posting-api). Vaga com `isListed=false` é ignorada (a documentação diz que ela só deve ser acessível pelo link direto). |
| Workable | `apply.workable.com/api/v1/widget/accounts/<conta>?details=true` | sim | [Docs](https://workable.readme.io/reference/jobs-1) documentam `www.workable.com/api/accounts/<conta>`, que redireciona para este. |
| SmartRecruiters | `api.smartrecruiters.com/v1/companies/<id>/postings?country=br` + detalhe por vaga | só no detalhe | [Docs](https://developers.smartrecruiters.com/docs/posting-api). Só os endpoints sem autenticação. A lista não traz descrição: até 200 vagas por empresa, 150 ms entre detalhes. Identificador inexistente responde 200 vazio, então "achou" = tem vagas. |
| Recruitee | `<empresa>.recruitee.com/api/offers/` | sim | [Docs](https://docs.recruitee.com/reference/offers). O token vira subdomínio: só `[a-z0-9-]`. |

Empresas cadastradas pelas migrations: 00016/00017 (Greenhouse, Lever) e 00025
(Nubank no Ashby, Experian no SmartRecruiters). Workable e Recruitee começam vazios:
nenhuma empresa brasileira com vagas foi achada na conferência.

## Busca por termo e listas

| Fonte | Tipo | Observações |
|---|---|---|
| Gupy | busca por termo | API pública do portal de vagas. |
| ProgramaThor | lista | |
| GitHub (repositórios de vagas) | lista | Issues de repositórios públicos de vagas, pela API do GitHub. |
| Himalayas | lista | API pública de vagas remotas; só as abertas ao Brasil. |

## Avaliadas e fora

| Fonte | Motivo |
|---|---|
| LinkedIn | Termos proíbem acesso automatizado; exigiria contornar bloqueio. |
| Remotive | Termos da API restringem o uso por agregadores. |
| Adzuna, Jooble | Cadastro da API exige cartão de crédito. |

## Próximas candidatas (não verificadas)

RemoteOK, We Work Remotely (RSS), Arbeitnow, Jobicy, HN "Who is hiring" (Algolia).
Cada uma só entra depois de ler os termos e registrar aqui.
