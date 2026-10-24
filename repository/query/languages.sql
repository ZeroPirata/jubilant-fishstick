-- Idiomas do perfil (user_languages).

-- name: QuerySelectAllLanguages :many
SELECT * FROM user_languages
WHERE user_id = @user_id AND deleted_at IS NULL
ORDER BY array_position(ARRAY['native', 'fluent', 'advanced', 'intermediate', 'basic'], proficiency), language;

-- name: QueryInsertLanguage :one
INSERT INTO user_languages(user_id, language, proficiency)
VALUES (@user_id, @language, @proficiency)
RETURNING *;

-- name: QueryUpdateLanguage :exec
UPDATE user_languages
SET language = @language, proficiency = @proficiency, updated_at = now()
WHERE id = @id AND user_id = @user_id AND deleted_at IS NULL;

-- name: QueryDeleteLanguage :exec
UPDATE user_languages
SET deleted_at = now()
WHERE id = @id AND user_id = @user_id;
