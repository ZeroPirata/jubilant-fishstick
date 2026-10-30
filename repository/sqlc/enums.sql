-- Só para o sqlc: NÃO é migration e não roda no banco.
--
-- Na 00001_initial_schema.sql os enums são criados dentro de blocos DO $$ ... $$
-- (para a migration ser idempotente), e o sqlc não lê PL/pgSQL. Sem este arquivo o
-- `sqlc generate` some com os tipos e passa a gerar string no lugar deles.
--
-- Ao criar ou alterar um enum numa migration, replique aqui.
CREATE TYPE skill_level AS ENUM ('basic', 'intermediate', 'advanced', 'expert');
CREATE TYPE feedback_status AS ENUM ('poor', 'fair', 'good', 'excellent');
CREATE TYPE job_quality AS ENUM ('low', 'mid', 'high');
CREATE TYPE job_status AS ENUM ('pending', 'error', 'processing', 'scraping_basic', 'scraping_nl', 'completed');
CREATE TYPE security_event_type AS ENUM ('login_failed', 'rate_limited', 'pdf_generated');
