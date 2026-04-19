-- name: QuerySelectFiltersForUser :many
SELECT keyword FROM user_job_filters
WHERE user_id = @user_id;

-- name: QuerySelectFiltersForUserWithID :many
SELECT id, keyword, user_id, COUNT(*) OVER() AS total_count FROM user_job_filters
WHERE user_id = @user_id
ORDER BY keyword
LIMIT @size OFFSET @cursor;

-- name: QueryInsertFilter :one
INSERT INTO user_job_filters(user_id, keyword)
VALUES (@user_id, @keyword)
RETURNING *;

-- name: QueryDeleteFilter :exec
DELETE FROM user_job_filters
WHERE id = @id AND user_id = @user_id;
