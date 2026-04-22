# jubilant-fishstick

Plataforma end-to-end para geração automatizada de documentos profissionais personalizados, construída do zero com foco em performance e escalabilidade. Arquitetura orientada a eventos com worker concorrente em Go, pipeline assíncrono de múltiplas etapas, cache distribuído com Redis, geração de PDF via renderização HTML/CSS server-side e integração com APIs externas. Todo o ciclo — desde a coleta de dados até a entrega do documento final — é orquestrado sem intervenção manual.

---

## Como funciona

```
Usuário adiciona uma vaga (URL ou dados manuais)
         │
         ▼
  Job criado → status: pending
         │
         ▼ (Worker a cada 30s)
  ┌──────────────────────────────────┐
  │  Scraping básico (HTML/API)      │  Título, empresa, descrição bruta
  │  Scraping NL (LLM)               │  Stack, requisitos, descrição limpa
  │  Matching com perfil do usuário  │  Experiências, habilidades, projetos
  │  Cálculo de qualidade da vaga    │  % de compatibilidade com filtros
  └──────────────────────────────────┘
         │
         ├─ Qualidade baixa → encerra (sem gerar documento)
         │
         ▼
  LLM gera currículo + carta personalizada
         │
         ▼
  Placeholders substituídos pelos dados reais do usuário
         │
         ▼
  Currículo salvo → status: completed
         │
         ▼
  Usuário solicita PDF → WeasyPrint renderiza e entrega o arquivo
```

O worker aprende com feedbacks anteriores: avaliações de currículos gerados (poor / fair / good / excellent) são injetadas nos próximos prompts para que a qualidade melhore progressivamente.

---

## Stack

| Camada | Tecnologia |
|---|---|
| Backend | Go 1.26+ |
| Banco de dados | PostgreSQL (pgx v5) |
| Cache | Redis |
| LLM | Claude (Anthropic) · Gemini · Ollama |
| PDF | WeasyPrint (Python, processo persistente) |
| Auth | JWT (HS256) + Argon2id |
| Query builder | SQLC (type-safe) |
| Logging | Uber Zap |

---

## Pré-requisitos

- Docker + Docker Compose
- VS Code com a extensão Dev Containers

---

## Rodando localmente

O ambiente de desenvolvimento roda inteiramente dentro de um Dev Container.

```bash
# Abra o projeto no VS Code e aceite "Reopen in Container"
# ou via linha de comando:
devcontainer open .
```

O container sobe automaticamente com Go, PostgreSQL, Redis e todas as dependências instaladas via `.devcontainer/install-tools.sh`.

### Variáveis de ambiente

Crie um arquivo `.env` na raiz com base nas variáveis abaixo:

```env
# Servidor
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
CONTEXT_TIMEOUT=60s

# PostgreSQL
POSTGRES_HOST=db
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=hackton
POSTGRES_SSL_MODE=disable
POSTGRES_MAX_CONNECTIONS=10

# Redis
REDIS_ADDR=cache:6379
REDIS_PASSWORD=

# LLM principal (geração de currículo)
API_PROVIDER=claude          # claude | gemini | ollama
API_KEY_AI=sk-ant-...
API_URL_AI=https://api.anthropic.com/v1/messages
API_MODEL_AI=claude-sonnet-4-5

# LLM de scraping (opcional — ativa enriquecimento de vagas via IA)
SCRAPE_AI_ACTIVATE=false
SCRAPE_AI_PROVIDER=claude
SCRAPE_AI_KEY=sk-ant-...
SCRAPE_AI_MODEL=claude-haiku-4-5-20251001

# Autenticação
JWT_SECRET=troque-isso
JWT_EXPIRATION=24h

# Hashing (Argon2id)
HASH_ARGON2_PEPPER=troque-isso
HASH_ARGON2_MEMORY=65536
HASH_ARGON2_ITERATIONS=3
HASH_ARGON2_PARALLELISM=2
HASH_ARGON2_SALT_LEN=16
HASH_ARGON2_KEY_LEN=32

# Misc
DEBUG=false
PROJECT_NAME=hackton-treino
VERSION=0.1.0
```

### Banco de dados

```bash
# Aplicar schema
docker compose -f .devcontainer/docker-compose.yml exec db \
  psql -U postgres -d hackton -f /workspace/repository/schema.sql

# Popular com dados de seed (opcional)
docker compose -f .devcontainer/docker-compose.yml exec db \
  psql -U postgres -d hackton -f /workspace/.vscode/seed.sql
```

### Rodando a aplicação

```bash
# Dentro do container:
go run ./cmd/server
```

---

## Build de produção

```bash
docker compose -f .devcontainer/docker-compose.prod.yml build
docker compose -f .devcontainer/docker-compose.prod.yml up -d
```

O Dockerfile de produção usa multi-stage build: compila o binário Go em `golang:alpine` e copia apenas o necessário para uma imagem `alpine` com WeasyPrint instalado.

---

## API

Todos os endpoints protegidos exigem o header:
```
Authorization: Bearer <token>
```

### Auth

| Método | Endpoint | Descrição |
|---|---|---|
| POST | `/api/v1/auth/register` | Cria conta |
| POST | `/api/v1/auth/login` | Autentica e retorna JWT |

### Perfil do usuário

| Método | Endpoint | Descrição |
|---|---|---|
| GET / PUT | `/api/v1/users/me/profile` | Perfil pessoal |
| PUT | `/api/v1/users/me/links` | Links (LinkedIn, GitHub, portfólio) |
| GET / POST | `/api/v1/users/me/experiences` | Experiências profissionais |
| PUT / DELETE | `/api/v1/users/me/experiences/{id}` | Editar / remover experiência |
| GET / POST | `/api/v1/users/me/skills` | Habilidades |
| PUT / DELETE | `/api/v1/users/me/skills/{id}` | Editar / remover habilidade |
| GET / POST | `/api/v1/users/me/projects` | Projetos |
| PUT / DELETE | `/api/v1/users/me/projects/{id}` | Editar / remover projeto |
| GET / POST | `/api/v1/users/me/academic` | Formação acadêmica |
| PUT / DELETE | `/api/v1/users/me/academic/{id}` | Editar / remover formação |
| GET / POST | `/api/v1/users/me/certificates` | Certificados |
| PUT / DELETE | `/api/v1/users/me/certificates/{id}` | Editar / remover certificado |

### Vagas e currículos

| Método | Endpoint | Descrição |
|---|---|---|
| GET / POST | `/api/v1/jobs` | Listar / adicionar vaga |
| PUT / DELETE | `/api/v1/jobs/{id}` | Atualizar / remover vaga |
| PUT | `/api/v1/jobs/{id}/retry` | Reprocessar vaga com erro |
| GET | `/api/v1/jobs/{id}/resumes` | Listar currículos gerados |
| GET | `/api/v1/jobs/{id}/resumes/{resume_id}` | Ver currículo |
| DELETE | `/api/v1/jobs/{id}/resumes/{resume_id}` | Remover currículo |
| POST | `/api/v1/jobs/{id}/resumes/{resume_id}/pdf` | Gerar PDF |
| POST | `/api/v1/jobs/{id}/resumes/{resume_id}/feedback` | Enviar feedback |

### Filtros de vagas

| Método | Endpoint | Descrição |
|---|---|---|
| GET / POST | `/api/v1/filters` | Listar / adicionar keyword de filtro |
| DELETE | `/api/v1/filters/{id}` | Remover filtro |

Os filtros são keywords técnicas (ex: `golang`, `grpc`, `postgresql`) que o worker usa para calcular a compatibilidade entre a vaga e o perfil do usuário. Vagas com menos de 30% de match são descartadas automaticamente.

---

## Scrapers suportados

| Site | Formato de URL |
|---|---|
| LinkedIn | `linkedin.com/jobs/view/{id}` ou `?currentJobId={id}` |
| Amazon Jobs | `amazon.jobs/...` |
| GeekHunter | `geekh­unter.com.br/...` |
| JobRight | `jobright.ai/jobs/info/{id}` |

---

## Estrutura do projeto

```
.
├── cmd/server/         # Entrypoint
├── config/             # Configuração via env
├── database/           # Conexão PostgreSQL e Redis
├── internal/
│   ├── db/             # Código gerado pelo SQLC (type-safe)
│   ├── handler/        # Handlers HTTP
│   ├── middleware/     # Auth, CORS, logging, timeout, panic recovery
│   ├── repository/     # Acesso a dados (users, jobs, cache, etc)
│   ├── routes/         # Definição das rotas
│   ├── scraper/        # Scrapers por plataforma
│   ├── security/       # JWT e Argon2id
│   ├── services/       # Integração com LLMs
│   └── worker/         # Pipeline de processamento assíncrono
│       └── prompts/    # System prompts (pt-BR e en)
├── repository/
│   ├── schema.sql      # Schema do banco
│   └── query/          # Queries SQL (input do SQLC)
└── scripts/
    └── gerar_pdf.py    # Servidor WeasyPrint (processo persistente)
```
