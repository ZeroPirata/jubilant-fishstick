-- name: QueryInsertSecurityEvent :exec
INSERT INTO security_events (event_type, ip, user_id, metadata)
VALUES ($1, $2, $3, $4);

-- name: QueryListSecurityEvents :many
SELECT id, event_type, ip, user_id, metadata, created_at
FROM security_events
WHERE created_at >= $1
ORDER BY created_at DESC
LIMIT 100;
