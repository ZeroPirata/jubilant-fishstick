-- SkillLevel defines the technical proficiency in a stack or tool:
-- * basic: Initial knowledge, requires constant supervision.
-- * intermediate: Handles routine tasks with autonomy.
-- * advanced: Solves complex problems and understands architecture.
-- * expert: Technical reference, capable of mentoring and system design.
CREATE TYPE skill_level AS ENUM ('basic', 'intermediate', 'advanced', 'expert');

-- FeedbackStatus defines the qualitative perception of an analysis or interview:
-- * poor: Does not meet minimum requirements or critical failure.
-- * fair: Partially meets requirements but has significant gaps.
-- * good: Meets requirements well and demonstrates competence.
-- * excellent: Exceeds expectations and demonstrates high potential.
CREATE TYPE feedback_status AS ENUM ('poor', 'fair', 'good', 'excellent');

-- QualityVaga define o nível de compatibilidade da vaga com o perfil do usuário:
-- * low: Fora do perfil (00% ~ 35%)
-- * mid: Vaga boa (36% ~ 79%)
-- * high: Vaga excelente (80% ~ 100%)
CREATE TYPE job_quality AS ENUM ('low', 'mid', 'high');

-- StatusVaga representa o estado atual da vaga na pipeline de processamento:
-- * pending: Vaga inicial, aguardando início.
-- * error: Falha em alguma etapa da pipeline.
-- * processing: Worker assumiu a tarefa.
-- * scraping_basic: Efetuando extração de dados brutos.
-- * scraping_nl: Processando linguagem natural para requisitos.
-- * completed: Processo finalizado com sucesso.
CREATE TYPE job_status AS ENUM ('pending', 'error', 'processing', 'scraping_basic', 'scraping_nl', 'completed');


/**
    1 usuario só tem uma conta com o e-mail cadastrado.
    1 usuario só tem um profile, que ele é sempre atualizado.
    1 usuario só tem um link que é sempre atualizado.
    1 usuario tem nenhuma ou várias experiencias.
    1 usuario tem nenhuma ou vários historico academico.
    1 usuario tem nenhuma ou várias habilidades.
    1 usuario tem nenhum ou vários projetos.
    1 usuario tem nenhum ou vários certificados
    Quando o usaurio clicar em deletar a conta, a conta vai ficar desabilitada, preenchendo o `deleted_at` e não
    deixando ele logar, até ele entrar em contato com a administração pra isso.
*/

CREATE TABLE IF NOT EXISTS user_accounts(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL, 
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS user_profiles(
    user_id UUID PRIMARY KEY REFERENCES user_accounts(id) ON DELETE CASCADE,
    full_name TEXT NOT NULL,
    phone TEXT,
    about TEXT,
    contact_email TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS user_links(
    user_id UUID PRIMARY KEY REFERENCES user_accounts(id) ON DELETE CASCADE,
    linkedin_url TEXT,
    github_url TEXT,
    portfolio_url TEXT,
    other_links JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS user_experiences(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
    company_name TEXT NOT NULL,
    job_role TEXT NOT NULL,
    description TEXT,
    is_current_job BOOLEAN NOT NULL DEFAULT FALSE,
    start_date DATE NOT NULL,
    end_date DATE,
    tech_stack TEXT[], 
    achievements TEXT[],
    tags TEXT[],
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS user_academic_histories(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
    institution_name TEXT NOT NULL,
    course_name TEXT NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS user_skills(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
    skill_name TEXT NOT NULL,
    proficiency_level skill_level NOT NULL,
    tags TEXT[],
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS user_projects(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
    project_name TEXT NOT NULL,
    description TEXT NOT NULL,
    project_url TEXT,
    tags TEXT[],
    start_date DATE NOT NULL,
    end_date DATE,
    is_academic BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS user_certificates(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
    certificate_name TEXT NOT NULL,
    issuing_organization TEXT NOT NULL,
    issue_date DATE,
    credential_url TEXT,
    tags TEXT[],
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

/*
    O usaurio vai inserir nenhuma ou várias vagas, onde só ele vai poder ver.
    vai ser gerado nenhum ou vários curriculos pro usuario que inseriu a vaga.
    O feedback vai ser dado por apenas 1 e exclusivamente 1 usuario para vários curriculos feitos.
    O filtro vai ser de um unico usuario para suas vagas.
    
    stack_aliases é global.
*/

CREATE TABLE IF NOT EXISTS stack_aliases (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alias_from  TEXT NOT NULL UNIQUE, -- ex: 'Golang'
    alias_to    TEXT NOT NULL,        -- ex: 'Go'
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
    external_url    TEXT NOT NULL,
    company_name    TEXT,
    job_title       TEXT,
    description     TEXT,
    tech_stack      TEXT[],
    requirements    TEXT[],
    status          job_status NOT NULL DEFAULT 'pending',
    quality         job_quality,
    language        TEXT DEFAULT 'pt-br',
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ,
    deleted_at      TIMESTAMPTZ,
    
    UNIQUE(user_id, external_url) 
);

CREATE TABLE IF NOT EXISTS generated_resumes (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id            UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    user_id           UUID NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
    content_json      JSONB NOT NULL,
    resume_pdf_path   TEXT,
    cover_letter_path TEXT,
    created_at        TIMESTAMPTZ DEFAULT now(),
    updated_at        TIMESTAMPTZ,
    deleted_at        TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS resumes_feedback (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resume_id       UUID NOT NULL REFERENCES generated_resumes(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
    status          feedback_status NOT NULL,
    comments        TEXT,
    created_at      TIMESTAMPTZ DEFAULT now(),
    
    UNIQUE(resume_id)
);

CREATE TABLE IF NOT EXISTS user_job_filters (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
    keyword         TEXT NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ,
    deleted_at      TIMESTAMPTZ,
    
    UNIQUE(user_id, keyword)
);


-- Chaves Estrangeiras
CREATE INDEX IF NOT EXISTS idx_user_profiles_user_id ON user_profiles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_links_user_id ON user_links(user_id);
CREATE INDEX IF NOT EXISTS idx_user_experiences_user_id ON user_experiences(user_id);
CREATE INDEX IF NOT EXISTS idx_user_academic_histories_user_id ON user_academic_histories(user_id);
CREATE INDEX IF NOT EXISTS idx_user_skills_user_id ON user_skills(user_id);
CREATE INDEX IF NOT EXISTS idx_user_projects_user_id ON user_projects(user_id);
CREATE INDEX IF NOT EXISTS idx_user_certificates_user_id ON user_certificates(user_id);

-- Relacionamentos da Pipeline
CREATE INDEX IF NOT EXISTS idx_jobs_user_id ON jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_generated_resumes_job_id ON generated_resumes(job_id);
CREATE INDEX IF NOT EXISTS idx_generated_resumes_user_id ON generated_resumes(user_id);

-- Busca em Arrays (Tags e Stacks)
CREATE INDEX IF NOT EXISTS idx_user_experiences_stack_gin ON user_experiences USING GIN(tech_stack);
CREATE INDEX IF NOT EXISTS idx_user_experiences_tags_gin ON user_experiences USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_user_skills_tags_gin ON user_skills USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_user_projects_tags_gin ON user_projects USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_jobs_tech_stack_gin ON jobs USING GIN(tech_stack);

-- Login rápido: Busca apenas usuários ativos por e-mail
CREATE UNIQUE INDEX IF NOT EXISTS idx_active_users_email 
ON user_accounts(email) 
WHERE deleted_at IS NULL;

-- Pipeline: Busca apenas jobs que ainda precisam ser processados
CREATE INDEX IF NOT EXISTS idx_jobs_pending_status 
ON jobs(status) 
WHERE status = 'pending' AND deleted_at IS NULL;

ALTER TABLE user_skills
ADD CONSTRAINT user_skills_user_id_skill_name_key
UNIQUE (user_id, skill_name);

-- Eventos de segurança: login falho, rate limit, PDF gerado.
CREATE TYPE security_event_type AS ENUM ('login_failed', 'rate_limited', 'pdf_generated');

CREATE TABLE IF NOT EXISTS security_events (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type security_event_type NOT NULL,
    ip         TEXT,
    user_id    UUID REFERENCES user_accounts(id) ON DELETE SET NULL,
    metadata   JSONB,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_security_events_created_at ON security_events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_security_events_event_type ON security_events(event_type);
CREATE INDEX IF NOT EXISTS idx_security_events_ip        ON security_events(ip);