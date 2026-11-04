-- +goose Up
-- +goose StatementBegin
-- Quadro de candidaturas: em que etapa está cada vaga depois do currículo pronto.
-- application_stage NULL = ainda não enviado (a vaga concluída aparece em "Currículo
-- pronto"). A etapa atual e a data dela são recalculadas a partir do histórico
-- (job_application_events) a cada mudança, para datas retroativas darem certo.
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS application_stage TEXT;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS application_stage_at TIMESTAMPTZ;
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_application_stage_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_application_stage_check CHECK (
    application_stage IS NULL OR
    application_stage IN ('applied', 'interview', 'offer', 'hired', 'rejected', 'withdrawn')
);

-- Histórico: cada mudança de etapa ou anotação. 'ready' = voltou para "Currículo
-- pronto" (desfazer o envio); 'note' = só uma anotação, não muda a etapa. A mesma
-- etapa pode se repetir (2ª entrevista). happened_at é a data informada pelo usuário.
CREATE TABLE IF NOT EXISTS job_application_events(
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id      UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
    stage       TEXT NOT NULL CHECK (stage IN ('ready', 'applied', 'interview', 'offer', 'hired', 'rejected', 'withdrawn', 'note')),
    note        TEXT NOT NULL DEFAULT '' CHECK (char_length(note) <= 1000),
    happened_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_job_application_events_job ON job_application_events(job_id, happened_at DESC, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_user_application ON jobs(user_id) WHERE application_stage IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS job_application_events;
DROP INDEX IF EXISTS idx_jobs_user_application;
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_application_stage_check;
ALTER TABLE jobs DROP COLUMN IF EXISTS application_stage_at;
ALTER TABLE jobs DROP COLUMN IF EXISTS application_stage;
-- +goose StatementEnd
