-- name: QuerySelectAllStackAliases :many
SELECT alias_from, alias_to FROM stack_aliases;

-- name: QuerySelectAllStackAliasesWithID :many
SELECT id, alias_from, alias_to FROM stack_aliases
ORDER BY alias_from;

-- name: QueryInsertStackAlias :one
INSERT INTO stack_aliases(alias_from, alias_to)
VALUES (@alias_from, @alias_to)
RETURNING *;

-- name: QueryDeleteStackAlias :exec
DELETE FROM stack_aliases
WHERE id = @id;
