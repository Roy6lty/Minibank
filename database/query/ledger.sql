-- name: CreateLedgerEntry :one
INSERT INTO ledger_entries (
    transfer_id,
    account_id,
    entry_type,
    amount,
    currency
)
VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;


-- name: GetLedgerEntry :one
SELECT *
FROM ledger_entries
WHERE id = $1
LIMIT 1;


-- name: ListLedgerEntriesByTransfer :many
SELECT *
FROM ledger_entries
WHERE transfer_id = $1
ORDER BY created_at ASC;


-- name: ListLedgerEntriesByAccount :many
SELECT *
FROM ledger_entries
WHERE account_id = $1
ORDER BY created_at DESC
LIMIT $2
OFFSET $3;

