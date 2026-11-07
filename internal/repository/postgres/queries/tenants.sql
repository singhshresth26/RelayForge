-- name: CreateTenant :one
INSERT INTO tenants (name, slug, plan)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetTenantByID :one
SELECT * FROM tenants
WHERE id = $1 LIMIT 1;

-- name: GetTenantBySlug :one
SELECT * FROM tenants
WHERE slug = $1 LIMIT 1;

-- name: ListTenants :many
SELECT * FROM tenants
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

