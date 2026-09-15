-- db/query/customer.sql

-- name: CreateCustomer :one
INSERT INTO customers (
    first_name,
    last_name,
    email,
    phone
)
VALUES ($1, $2, $3, $4)
RETURNING *;


-- name: GetCustomer :one
SELECT *
FROM customers
WHERE id = $1;


-- name: GetCustomerByEmail :one
SELECT *
FROM customers
WHERE email = $1;


-- name: GetCustomers :many
SELECT *
FROM customers
ORDER BY id
LIMIT $1
OFFSET $2;



-- name: GetCustomersByCreatedAt :many
SELECT *
FROM customers
ORDER BY created_at DESC
LIMIT $1
OFFSET $2;


-- name: GetCustomersOrderedByFirstName :many
SELECT *
FROM customers
ORDER BY first_name ASC
LIMIT $1
OFFSET $2;

-- name: GetCustomersOrderedByLastName :many
SELECT *
FROM customers
ORDER BY last_name ASC
LIMIT $1
OFFSET $2;


-- name: GetCustomersByFirstOrLastName :many
SELECT *
FROM customers
WHERE first_name = $1
   OR last_name = $1
ORDER BY created_at DESC
LIMIT $2
OFFSET $3;

-- name: SearchCustomersByName :many
SELECT *
FROM customers
WHERE
    (sqlc.narg(first_name)::text IS NULL
        OR first_name = sqlc.narg(first_name))
AND
    (sqlc.narg(last_name)::text IS NULL
        OR last_name = sqlc.narg(last_name))
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count)
OFFSET sqlc.arg(offset_count);

-- name: UpdateCustomerByID :one
UPDATE customers
SET
    first_name = COALESCE(sqlc.narg(first_name), first_name),
    last_name  = COALESCE(sqlc.narg(last_name), last_name),
    email      = COALESCE(sqlc.narg(email), email),
    phone      = COALESCE(sqlc.narg(phone), phone),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
RETURNING *;
