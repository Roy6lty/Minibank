-- +goose Up

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE customers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    phone TEXT UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id),
    account_number VARCHAR(20) NOT NULL UNIQUE,
    account_type VARCHAR(20) NOT NULL,
    currency CHAR(3) NOT NULL DEFAULT 'NGN',
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT valid_account_status
        CHECK (status IN ('ACTIVE', 'FROZEN', 'CLOSED')),

    CONSTRAINT valid_account_type
        CHECK (account_type IN ('SAVINGS', 'CURRENT'))
);

CREATE TABLE transfers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reference VARCHAR(100) NOT NULL UNIQUE,

    source_account_id UUID NOT NULL REFERENCES accounts(id),
    destination_account_id UUID NOT NULL REFERENCES accounts(id),

    amount BIGINT NOT NULL,
    currency CHAR(3) NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    description TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT positive_transfer_amount
        CHECK (amount > 0),

    CONSTRAINT different_accounts
        CHECK (source_account_id <> destination_account_id),

    CONSTRAINT valid_transfer_status
        CHECK (
            status IN (
                'PENDING',
                'PROCESSING',
                'COMPLETED',
                'FAILED',
                'REVERSED'
            )
        )
);

CREATE TABLE ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    transfer_id UUID NOT NULL REFERENCES transfers(id),
    account_id UUID NOT NULL REFERENCES accounts(id),

    entry_type VARCHAR(10) NOT NULL,
    amount BIGINT NOT NULL,
    currency CHAR(3) NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT positive_ledger_amount
        CHECK (amount > 0),

    CONSTRAINT valid_entry_type
        CHECK (entry_type IN ('DEBIT', 'CREDIT'))
);

CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    reference VARCHAR(100) NOT NULL UNIQUE,

    transaction_type VARCHAR(30) NOT NULL,

    amount BIGINT NOT NULL,
    currency CHAR(3) NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',

    description TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT positive_transaction_amount
        CHECK (amount > 0),

    CONSTRAINT valid_transaction_type
        CHECK (
            transaction_type IN (
                'TRANSFER',
                'DEPOSIT',
                'WITHDRAWAL',
                'FEE',
                'INTEREST',
                'REVERSAL'
            )
        ),

    CONSTRAINT valid_transaction_status
        CHECK (
            status IN (
                'PENDING',
                'PROCESSING',
                'COMPLETED',
                'FAILED',
                'REVERSED'
            )
        )
);

CREATE INDEX idx_transactions_created_at
    ON transactions(created_at);

CREATE INDEX idx_transactions_type
    ON transactions(transaction_type);

CREATE INDEX idx_transactions_status
    ON transactions(status);

CREATE INDEX idx_accounts_customer_id
    ON accounts(customer_id);

CREATE INDEX idx_transfers_source_account_id
    ON transfers(source_account_id);

CREATE INDEX idx_transfers_destination_account_id
    ON transfers(destination_account_id);

CREATE INDEX idx_transfers_created_at
    ON transfers(created_at);

CREATE INDEX idx_ledger_entries_transfer_id
    ON ledger_entries(transfer_id);

CREATE INDEX idx_ledger_entries_account_id
    ON ledger_entries(account_id);

CREATE INDEX idx_ledger_entries_created_at
    ON ledger_entries(created_at);

-- +goose Down

DROP TABLE ledger_entries;
DROP TABLE transactions;
DROP TABLE transfers;
DROP TABLE accounts;
DROP TABLE customers;
DROP EXTENSION IF EXISTS pgcrypto;
