-- name: CreateUser :one
INSERT INTO users (tenant_id, email, password_hash, role)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: ListUsersByTenant :many
SELECT * FROM users
WHERE tenant_id = $1
ORDER BY created_at DESC;

