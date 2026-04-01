CREATE TYPE sensor_status AS ENUM ('NORMAL', 'ALERTA', 'EMERGENCIA');

CREATE TABLE IF NOT EXISTS sensores (
    id BIGSERIAL PRIMARY KEY,
    nome VARCHAR(255) NOT NULL,
    loc VARCHAR(255) NOT NULL,
    status sensor_status NOT NULL DEFAULT 'NORMAL'
);

CREATE TABLE IF NOT EXISTS historico (
    id BIGSERIAL PRIMARY KEY,
    sensor_id BIGINT NOT NULL,
    value FLOAT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT fk_sensor_hist FOREIGN KEY (sensor_id) REFERENCES sensores(id)
);

CREATE INDEX idx_historico_sensor_id ON historico(sensor_id);

CREATE TABLE IF NOT EXISTS alerta (
    id BIGSERIAL PRIMARY KEY,
    sensor_id BIGINT NOT NULL,
    status sensor_status NOT NULL,
    alert_time TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_sensor_alerta FOREIGN KEY (sensor_id) REFERENCES sensores(id)
);



-- Desafio Curriculo
-- TIPOS
CREATE TYPE nivel_habilidade AS ENUM ('basico', 'intermediario', 'avancado', 'expert');
CREATE TYPE status_vaga AS ENUM ('pendente', 'processando', 'gerado', 'revisado', 'enviado', 'fora_do_perfil', 'improcessavel');
CREATE TYPE status_feedback AS ENUM ('medio', 'bom', 'excelente');

-- BANCO PESSOAL
CREATE TABLE IF NOT EXISTS informacoes_basicas (
    id          BIGSERIAL PRIMARY KEY,
    nome        VARCHAR(255) NOT NULL,
    email       VARCHAR(255) NOT NULL UNIQUE,
    telefone    VARCHAR(20),
    linkedin    VARCHAR(255),
    github      VARCHAR(255),
    portfolio   VARCHAR(255),
    resumo      TEXT,
    criado_em   TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS experiencias (
    id          BIGSERIAL PRIMARY KEY,
    empresa     VARCHAR(255) NOT NULL,
    cargo       VARCHAR(255) NOT NULL,
    descricao   TEXT,
    atual       BOOLEAN NOT NULL DEFAULT FALSE,
    data_inicio DATE NOT NULL,
    data_fim    DATE,
    stack       TEXT[],
    conquistas  TEXT[],
    tags        TEXT[]
);

CREATE TABLE IF NOT EXISTS formacao (
    id           BIGSERIAL PRIMARY KEY,
    instituicao  VARCHAR(255) NOT NULL,
    curso        VARCHAR(255) NOT NULL,
    data_inicio  DATE NOT NULL,
    data_fim     DATE,
    descricao    TEXT
);

CREATE TABLE IF NOT EXISTS habilidades (
    id     BIGSERIAL PRIMARY KEY,
    nome   VARCHAR(255) NOT NULL,
    nivel  nivel_habilidade NOT NULL,
    tags   TEXT[]
);

CREATE TABLE IF NOT EXISTS projetos (
    id          BIGSERIAL PRIMARY KEY,
    nome        VARCHAR(255) NOT NULL,
    descricao   TEXT,
    link        VARCHAR(255),
    tags        TEXT[],
    data_inicio  DATE NOT NULL,
    data_fim     DATE,
    facultativo    BOOLEAN NOT NULL DEFAULT FALSE
);

-- PIPELINE DE VAGAS
CREATE TABLE IF NOT EXISTS vagas (
    id           BIGSERIAL PRIMARY KEY,
    url          TEXT NOT NULL UNIQUE,
    empresa      VARCHAR(255),
    titulo       VARCHAR(255),
    descricao    TEXT,
    stack        TEXT[],
    requisitos   TEXT[],
    status       status_vaga NOT NULL DEFAULT 'pendente',
    criado_em    TIMESTAMP NOT NULL DEFAULT NOW(),
    idioma       TEXT
);

CREATE TABLE IF NOT EXISTS curriculos_gerados (
    id                BIGSERIAL PRIMARY KEY,
    vaga_id           BIGINT NOT NULL REFERENCES vagas(id),
    conteudo_json     JSONB NOT NULL,
    resume_path       TEXT,
    cover_letter_path TEXT,
    criado_em         TIMESTAMP NOT NULL DEFAULT NOW(),
    resume_path       TEXT
    cover_letter_path TEXT
);

-- MEMÓRIA DE FEEDBACK
CREATE TABLE IF NOT EXISTS feedback (
    id             BIGSERIAL PRIMARY KEY,
    curriculo_id   BIGINT NOT NULL REFERENCES curriculos_gerados(id),
    vaga_id        BIGINT NOT NULL REFERENCES vagas(id),
    status         status_feedback NOT NULL,
    comentario     TEXT,
    criado_em      TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS certificacoes (
    id          BIGSERIAL PRIMARY KEY,
    nome        VARCHAR(255) NOT NULL,
    emissor     VARCHAR(255) NOT NULL,
    emitido_em  DATE,
    codigo      VARCHAR(255),
    link        VARCHAR(255),
    tags        TEXT[]
);

CREATE TABLE filtros (
    id      BIGSERIAL PRIMARY KEY,
    keyword VARCHAR(100) NOT NULL UNIQUE
);


-- ÍNDICES PARA O MATCHING
CREATE INDEX IF NOT EXISTS idx_experiencias_tags ON experiencias   USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_habilidades_tags  ON habilidades    USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_projetos_tags     ON projetos       USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_certificacoes_tags ON certificacoes USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_vagas_stack       ON vagas          USING GIN(stack);
CREATE INDEX IF NOT EXISTS idx_feedback_status   ON feedback(status);