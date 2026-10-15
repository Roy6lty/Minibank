# MiniBank

MiniBank is a Go REST API for customer accounts, transfers, ledger entries, transactions, and authentication.

The API uses PostgreSQL for durable data and Redis for rate limiting and failed-login protection.

## Status

Implemented:

- JWT access and refresh tokens
- Customer, account, transfer, ledger, and transaction modules
- Ownership and permission checks
- PostgreSQL transactions for financial mutations
- Redis-backed rate limiting and login-attempt protection
- Vault-backed secret loading in Docker Compose
- Strict JSON request validation
- 1 MiB maximum JSON request body size
- Structured JSON logging with sensitive-data redaction
- Request IDs through the `X-Request-ID` header
- Append-only PostgreSQL audit events for business and security activity
- Configurable database and HTTP timeouts
- Graceful HTTP server shutdown

Still pending:

- PostgreSQL and Redis high-availability topology
- Health and readiness endpoints
- A bundled migration command
- CI configuration
- PostgreSQL/Redis integration and failure tests
- Deposits, withdrawals, fees, interest, and reversals

## Architecture

```text
cmd/
  main.go                  Application entrypoint and HTTP server setup
config/
  settings.go              Environment loading and validation
database/
  migrations/              Goose-formatted PostgreSQL migrations
  query/                   SQL queries used by sqlc
  repository/              Repository and transaction boundaries
internal/
	audit/                  Audit event model, recorder, and request context
	httpx/                   JSON decoding, size limits, and validation
  logging/                 Structured logging and redaction
  middleware/              Recovery and request logging
  ratelimit/               Redis-backed rate limiting
  sqlc/                    Generated database access code
modules/
  auth/                    Authentication and authorization
  customer/                Customer operations
  account/                 Account operations
  transfers/               Transfer operations
  ledger/                  Ledger queries
  transactions/            Transaction operations
```

## Requirements

- Go `1.27.0` or newer
- PostgreSQL
- Redis
- HashiCorp Vault when `VAULT_ENABLED=true`
- Goose, or another migration tool that supports Goose-formatted migrations

## Configuration

Create a local environment file:

```bash
cp .env.example .env
```

Update at least these values in `.env`:

```dotenv
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=change-me
DB_NAME=minibank
DB_SSLMODE=disable

REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

ACCESS_SECRET=replace-with-a-long-random-secret
REFRESH_SECRET=replace-with-a-different-long-random-secret

VAULT_ENABLED=false
VAULT_ADDR=http://localhost:8200
VAULT_TOKEN=dev-root-token
VAULT_SECRET_PATH=minibank
```

The remaining settings control database pool limits, connection/query timeouts, Redis timeouts, HTTP timeouts, and shutdown behavior. See `.env.example` for defaults.

Do not commit `.env` or real credentials.

## Database Setup

The repository contains Goose-formatted migrations, but it does not currently include a migration runner.

Install Goose separately if needed:

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

Apply the migrations:

```bash
export DATABASE_URL='postgres://postgres:change-me@localhost:5432/minibank?sslmode=disable'
goose -dir database/migrations postgres "$DATABASE_URL" up
```

The SQL queries and generated code are configured through `sqlc.yaml`. After changing files under `database/query`, regenerate the code with:

```bash
sqlc generate
```

## Running Locally

Start PostgreSQL and Redis, apply the migrations, then run the API:

```bash
go mod download
go run ./cmd
```

## Running with Docker Compose

Copy the example environment file and replace the development secrets before using this outside local development:

```bash
cp .env.example .env
docker compose up --build
```

Compose starts PostgreSQL, Redis, and a development Vault server. It seeds the Vault KV secret at `secret/minibank`, applies the Goose migrations, and then starts the API at `http://localhost:8080`. The API loads database, Redis, and token secrets from Vault during startup.

Stop the services with:

```bash
docker compose down
```

To remove the persisted PostgreSQL and Redis volumes as well:

```bash
docker compose down -v
```

The server listens on:

```text
http://localhost:8080
```

There is currently no built-in health or readiness endpoint. A successful process startup means the initial Redis and PostgreSQL connection checks passed.

## API

All routes are prefixed with `/api/v1`.

### Authentication

| Method | Path | Authentication |
| --- | --- | --- |
| `POST` | `/auth/register` | Public |
| `POST` | `/auth/login` | Public, rate limited |
| `POST` | `/auth/refresh` | Public |
| `POST` | `/auth/logout` | Public |

Protected endpoints require:

```http
Authorization: Bearer <access_token>
```

### Customers

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/customers/` | Create a customer |
| `GET` | `/customers/search` | Search customers by name |
| `GET` | `/customers/{customerID}` | Retrieve a customer |
| `PATCH` | `/customers/{customerID}` | Update a customer |

### Accounts

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/accounts/` | Create an account |
| `GET` | `/accounts/search` | Search accounts |
| `GET` | `/accounts/{accountNumber}` | Retrieve an account |
| `PATCH` | `/accounts/{accountNumber}` | Update account status |

### Transfers

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/transfers/` | Create a transfer |
| `GET` | `/transfers/` | List transfers |
| `GET` | `/transfers/{transferID}` | Retrieve a transfer |
| `GET` | `/transfers/reference/{referenceID}` | Retrieve by reference |
| `PATCH` | `/transfers/{transferID}` | Update transfer status |

### Ledger

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/ledger/accounts/{accountID}/balance` | Retrieve an account balance |
| `GET` | `/ledger/accounts/{accountID}/entries` | List account entries |
| `GET` | `/ledger/transfers/{transferID}/entries` | List transfer entries |
| `GET` | `/ledger/entries/{entryID}` | Retrieve a ledger entry |

### Transactions

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/transactions/` | Create a transaction |
| `GET` | `/transactions/{transactionID}` | Retrieve a transaction |
| `PATCH` | `/transactions/{transactionID}` | Update transaction status |

## Example Request

Register a customer:

```bash
curl -i http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{
    "first_name": "Ada",
    "last_name": "Lovelace",
    "email": "ada@example.com",
    "phone": "+2348000000000",
    "password": "correct-horse-battery-staple"
  }'
```

JSON request bodies are limited to 1 MiB, reject unknown fields, and are checked against endpoint-specific validation rules. Invalid bodies return `400 Bad Request`; oversized bodies return `413 Payload Too Large`.

## Logging

The API configures `log/slog` with a JSON handler writing to stderr. Request logs include:

- Request ID
- HTTP method
- Matched route template
- Status code
- Request duration

Request bodies, query values, authorization headers, cookies, raw paths, and client IP addresses are not logged. Known credentials, tokens, secrets, PII, account identifiers, and sensitive error text are redacted as `[REDACTED]`.

Clients may provide a safe `X-Request-ID` value. If omitted or invalid, the server generates one and returns it in the response header.

## Audit Trail

Audit events are stored in the append-only `audit_events` table. Financial mutations write transfer and transaction lifecycle events in the same PostgreSQL transaction as the business change. Authentication and authorization events include request IDs and actor IDs when known.

Audit details do not contain passwords, tokens, authorization headers, raw login identifiers, or other unnecessary sensitive values. Database triggers reject updates and deletes from the audit table.

## Development Checks

Run the full test suite:

```bash
go test ./...
```

Run the race detector for the full project:

```bash
go test -race ./...
```

Run static analysis:

```bash
go vet ./...
```

Format changed Go files with:

```bash
go fmt ./...
```

## Security Notes

- Redis-backed rate limits fail closed when Redis is unavailable.
- High-risk account and transfer mutations require an active session and rate limits.
- Customer resource access is ownership-checked unless the principal is an administrator.
- Use `DB_SSLMODE=require`, `verify-ca`, or `verify-full` outside local development.
- Use separate, randomly generated access and refresh secrets.
- PostgreSQL and Redis currently use single configured endpoints; application-level HA failover is not configured.
