CREATE TABLE account_products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,

    interest_rate NUMERIC(8, 5) NOT NULL DEFAULT 0,
    minimum_balance NUMERIC(18, 2) NOT NULL DEFAULT 0,
    maintenance_fee NUMERIC(18, 2) NOT NULL DEFAULT 0,
    withdrawal_fee_rate NUMERIC(8, 5) NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
