-- +goose Up
-- +goose StatementBegin
-- Falha com unique violation se existirem contas que só diferem por maiúsculas;
-- nesse caso resolva os duplicados manualmente antes de aplicar.
UPDATE user_accounts SET email = lower(trim(email)) WHERE email <> lower(trim(email));

ALTER TABLE user_accounts
    ADD CONSTRAINT chk_user_accounts_email_lower CHECK (email = lower(email));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE user_accounts DROP CONSTRAINT IF EXISTS chk_user_accounts_email_lower;
-- +goose StatementEnd
