-- db/query/account.sql

-- name: CreateAccount :one
INSERT INTO accounts (
    customer_id,
    account_number,
    account_type,
    currency
)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetAccount :one
SELECT *
FROM accounts
WHERE id = $1;

-- name: LockAccount :one
SELECT *
FROM accounts
WHERE id = $1
FOR UPDATE;

-- name: AdjustAccountBalance :one
UPDATE accounts
SET balance = balance + $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: GetAccountByAccountNumber :one
SELECT *
FROM accounts
WHERE account_number = $1;


-- name: GetMultipleAccountsByAccountNumber :many
SELECT *
FROM accounts
WHERE account_number = ANY($1::text[]);

-- name: SearchAccounts :many
SELECT *
FROM accounts
WHERE
    (sqlc.narg(account_number)::text IS NULL
        OR account_number = sqlc.narg(account_number))
AND
    (sqlc.narg(account_type)::text IS NULL
        OR account_type = sqlc.narg(account_type))
AND
    (sqlc.narg(customer_id)::uuid IS NULL
        OR customer_id = sqlc.narg(customer_id))
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count)
OFFSET sqlc.arg(offset_count);


-- name: UpdateAccountByAccountNumber :one
UPDATE accounts
SET status = sqlc.arg(new_status),
    updated_at = NOW()
WHERE account_number = sqlc.arg(account_number)
  AND status = sqlc.arg(current_status)
RETURNING *;
