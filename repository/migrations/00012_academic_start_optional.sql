-- +goose Up
-- +goose StatementBegin
-- Muitos currículos trazem só a conclusão do curso ("Julho/2026"): o início passa a ser opcional.
ALTER TABLE user_academic_histories ALTER COLUMN start_date DROP NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE user_academic_histories SET start_date = COALESCE(end_date, created_at::date) WHERE start_date IS NULL;
ALTER TABLE user_academic_histories ALTER COLUMN start_date SET NOT NULL;
-- +goose StatementEnd
