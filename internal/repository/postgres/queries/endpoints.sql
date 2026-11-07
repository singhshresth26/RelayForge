-- name: CreateEndpoint :one
INSERT INTO endpoints (app_id, url, secret_encrypted, description, is_active, max_retries, timeout_ms, rate_limit_rps)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetEndpointByID :one
SELECT * FROM endpoints
WHERE id = $1
LIMIT 1;

-- name: GetEndpointWithApp :one
SELECT e.*, a.tenant_id 
FROM endpoints e
JOIN applications a ON e.app_id = a.id
WHERE e.id = $1 AND a.tenant_id = $2
LIMIT 1;

-- name: ListEndpointsByApp :many
SELECT * FROM endpoints
WHERE app_id = $1
ORDER BY created_at DESC;

-- name: ListActiveEndpointsForEvent :many
SELECT e.* 
FROM endpoints e
JOIN endpoint_subscriptions es ON e.id = es.endpoint_id
WHERE e.app_id = $1 
  AND e.is_active = TRUE 
  AND (es.event_type = $2 OR es.event_type = '*')
ORDER BY e.created_at ASC;

-- name: UpdateEndpoint :one
UPDATE endpoints
SET url = $2, description = $3, is_active = $4, max_retries = $5, timeout_ms = $6, rate_limit_rps = $7, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateEndpointSecret :one
UPDATE endpoints
SET secret_encrypted = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteEndpoint :exec
DELETE FROM endpoints
WHERE id = $1;

