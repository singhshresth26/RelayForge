-- name: AddEndpointSubscription :exec
INSERT INTO endpoint_subscriptions (endpoint_id, event_type)
VALUES ($1, $2)
ON CONFLICT (endpoint_id, event_type) DO NOTHING;

-- name: RemoveEndpointSubscription :exec
DELETE FROM endpoint_subscriptions
WHERE endpoint_id = $1 AND event_type = $2;

-- name: ClearEndpointSubscriptions :exec
DELETE FROM endpoint_subscriptions
WHERE endpoint_id = $1;

-- name: ListSubscriptionsByEndpoint :many
SELECT event_type, created_at
FROM endpoint_subscriptions
WHERE endpoint_id = $1
ORDER BY event_type ASC;

