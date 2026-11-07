-- name: CreateApplication :one
INSERT INTO applications (tenant_id, name, description, is_active)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetApplicationByID :one
SELECT * FROM applications
WHERE id = $1 AND tenant_id = $2
LIMIT 1;

-- name: ListApplicationsByTenant :many
SELECT * FROM applications
WHERE tenant_id = $1
ORDER BY created_at DESC;

-- name: UpdateApplication :one
UPDATE applications
SET name = $3, description = $4, is_active = $5, updated_at = NOW()
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: DeleteApplication :exec
DELETE FROM applications
WHERE id = $1 AND tenant_id = $2;

