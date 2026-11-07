-- name: CreateOutboxEvent :one
INSERT INTO outbox_events (event_id, app_id, event_type, payload, status)
VALUES ($1, $2, $3, $4, 'PENDING')
RETURNING *;

-- name: FetchPendingOutboxBatch :many
SELECT * FROM outbox_events
WHERE status = 'PENDING'
ORDER BY created_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxPublished :exec
UPDATE outbox_events
SET status = 'PUBLISHED', published_at = NOW()
WHERE id = ANY($1::bigint[]);

-- name: MarkOutboxFailed :exec
UPDATE outbox_events
SET status = 'FAILED', retry_count = retry_count + 1, last_error = $2
WHERE id = $1;

