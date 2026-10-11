-- +goose Up

UPDATE accounts
SET currency = 'NGN'
WHERE BTRIM(currency) = '';

ALTER TABLE accounts
    ALTER COLUMN currency DROP DEFAULT;

ALTER TABLE accounts
    ADD CONSTRAINT valid_account_currency
    CHECK (currency IN ('NGN', 'USD', 'GBP', 'EUR'));

-- +goose Down

ALTER TABLE accounts
    DROP CONSTRAINT valid_account_currency;

ALTER TABLE accounts
    ALTER COLUMN currency SET DEFAULT 'NGN';
