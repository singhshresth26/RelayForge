-- name: CreateEvent :one
INSERT INTO events (app_id, idempotency_key, event_type, payload, metadata)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetEventByID :one
SELECT * FROM events
WHERE id = $1
LIMIT 1;

-- name: GetEventByIdempotencyKey :one
SELECT * FROM events
WHERE app_id = $1 AND idempotency_key = $2
LIMIT 1;

-- name: ListEventsByApp :many
SELECT * FROM events
WHERE app_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

