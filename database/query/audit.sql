-- name: CreateAuditEvent :one
INSERT INTO audit_events (
    event_type,
    outcome,
    actor_user_id,
    request_id,
    resource_type,
    resource_id,
    details
)
VALUES (
    sqlc.arg(event_type),
    sqlc.arg(outcome),
    sqlc.narg(actor_user_id),
    sqlc.narg(request_id),
    sqlc.narg(resource_type),
    sqlc.narg(resource_id),
    sqlc.arg(details)
)
RETURNING *;

-- name: ListAuditEventsByResource :many
SELECT *
FROM audit_events
WHERE resource_type = sqlc.arg(resource_type)
  AND resource_id = sqlc.arg(resource_id)
ORDER BY occurred_at DESC
LIMIT sqlc.arg(limit_count)
OFFSET sqlc.arg(offset_count);

-- name: ListRecentAuditEvents :many
SELECT *
FROM audit_events
ORDER BY occurred_at DESC
LIMIT sqlc.arg(limit_count)
OFFSET sqlc.arg(offset_count);
