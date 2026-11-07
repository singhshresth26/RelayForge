-- name: CreateDelivery :one
INSERT INTO deliveries (event_id, endpoint_id, status, max_attempts)
VALUES ($1, $2, 'PENDING', $3)
ON CONFLICT (event_id, endpoint_id) DO NOTHING
RETURNING *;

-- name: GetDeliveryByID :one
SELECT * FROM deliveries
WHERE id = $1
LIMIT 1;

-- name: ListDeliveriesByApp :many
SELECT d.*, e.url as endpoint_url, ev.event_type
FROM deliveries d
JOIN endpoints e ON d.endpoint_id = e.id
JOIN events ev ON d.event_id = ev.id
WHERE e.app_id = $1
ORDER BY d.created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListDeliveriesByEvent :many
SELECT d.*, e.url as endpoint_url
FROM deliveries d
JOIN endpoints e ON d.endpoint_id = e.id
WHERE d.event_id = $1
ORDER BY d.created_at ASC;

-- name: UpdateDeliveryStatus :one
UPDATE deliveries
SET status = $2, current_attempt = $3, next_retry_at = $4, delivered_at = $5, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CreateDeliveryAttempt :one
INSERT INTO delivery_attempts (
    delivery_id, attempt_number, status_code, latency_ms, 
    request_headers, response_headers, response_body, error_message
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListAttemptsByDeliveryID :many
SELECT * FROM delivery_attempts
WHERE delivery_id = $1
ORDER BY attempt_number ASC;

