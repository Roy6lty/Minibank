-- name: CreateTransfer :one
INSERT INTO transfers (
    reference,
    source_account_id,
    destination_account_id,
    amount,
    currency,
    description
)
VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;


-- name: GetTransfer :one
SELECT *
FROM transfers
WHERE id = $1
LIMIT 1;


-- name: GetTransferForPosting :one
SELECT *
FROM transfers
WHERE id = $1
FOR UPDATE;


-- name: GetTransferByReference :one
SELECT *
FROM transfers
WHERE reference = $1
LIMIT 1;


-- name: ListTransfers :many
SELECT *
FROM transfers
ORDER BY created_at DESC
LIMIT $1
OFFSET $2;


-- name: ListTransfersByAccount :many
SELECT *
FROM transfers
WHERE source_account_id = $1
   OR destination_account_id = $1
ORDER BY created_at DESC
LIMIT $2
OFFSET $3;


-- name: ListTransfersBetweenAccounts :many
SELECT *
FROM transfers
WHERE source_account_id = $1
  AND destination_account_id = $2
ORDER BY created_at DESC
LIMIT $3
OFFSET $4;


-- name: ListOutgoingTransfers :many
SELECT *
FROM transfers
WHERE source_account_id = $1
ORDER BY created_at DESC
LIMIT $2
OFFSET $3;


-- name: ListIncomingTransfers :many
SELECT *
FROM transfers
WHERE destination_account_id = $1
ORDER BY created_at DESC
LIMIT $2
OFFSET $3;

-- name: ListIncomingAndOutgoingTransfers :many
SELECT *
FROM transfers
WHERE destination_account_id = $1
   OR source_account_id = $1
ORDER BY created_at DESC
LIMIT $2
OFFSET $3;

-- name: UpdateTransferStatus :one
UPDATE transfers
SET
    status = sqlc.arg(new_status),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND status = sqlc.arg(current_status)
RETURNING *;
