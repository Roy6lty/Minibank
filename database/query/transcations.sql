-- db/query/transaction.sql

-- name: CreateTransaction :one
INSERT INTO transactions (
    reference,
    transaction_type,
    status,
    amount,
    currency,
    description
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;


-- name: GetTransaction :one
SELECT *
FROM transactions
WHERE id = $1
LIMIT 1;


-- name: GetTransactionByReference :one
SELECT *
FROM transactions
WHERE reference = $1
LIMIT 1;


-- name: GetTransactions :many
SELECT *
FROM transactions
ORDER BY created_at DESC
LIMIT $1
OFFSET $2;


-- name: GetTransactionsByType :many
SELECT *
FROM transactions
WHERE transaction_type = $1
ORDER BY created_at DESC
LIMIT $2
OFFSET $3;


-- name: GetTransactionsByStatus :many
SELECT *
FROM transactions
WHERE status = $1
ORDER BY created_at DESC
LIMIT $2
OFFSET $3;


-- name: UpdateTransactionStatus :one
UPDATE transactions
SET
    status = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;
