-- name: CreateAPIKey :one
INSERT INTO api_keys (tenant_id, name, key_prefix, key_hash, scopes, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAPIKeyByHash :one
SELECT * FROM api_keys
WHERE key_hash = $1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())
LIMIT 1;

-- name: ListAPIKeysByTenant :many
SELECT * FROM api_keys
WHERE tenant_id = $1 AND revoked_at IS NULL
ORDER BY created_at DESC;

-- name: RevokeAPIKey :one
UPDATE api_keys
SET revoked_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND revoked_at IS NULL
RETURNING *;

-- name: UpdateAPIKeyLastUsed :exec
UPDATE api_keys
SET last_used_at = NOW()
WHERE id = $1;

