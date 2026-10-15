-- +goose Up

ALTER TABLE transactions
    ADD COLUMN transfer_id UUID REFERENCES transfers(id);

UPDATE transactions AS transaction_record
SET transfer_id = transfer_record.id
FROM transfers AS transfer_record
WHERE transaction_record.transaction_type = 'TRANSFER'
  AND transaction_record.reference = transfer_record.reference;

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM transactions
        WHERE transaction_type = 'TRANSFER'
          AND transfer_id IS NULL
    ) THEN
        RAISE EXCEPTION 'existing TRANSFER transaction has no matching transfer';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM transfers AS transfer_record
        JOIN transactions AS transaction_record
          ON transaction_record.reference = transfer_record.reference
        WHERE transaction_record.transaction_type <> 'TRANSFER'
    ) THEN
        RAISE EXCEPTION 'transfer reference is already used by a non-transfer transaction';
    END IF;
END;
$$;
-- +goose StatementEnd

INSERT INTO transactions (
    transfer_id,
    reference,
    transaction_type,
    status,
    amount,
    currency,
    description
)
SELECT
    transfer_record.id,
    transfer_record.reference,
    'TRANSFER',
    transfer_record.status,
    transfer_record.amount,
    transfer_record.currency,
    transfer_record.description
FROM transfers AS transfer_record
WHERE NOT EXISTS (
    SELECT 1
    FROM transactions AS transaction_record
    WHERE transaction_record.transfer_id = transfer_record.id
);

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM transfers AS transfer_record
        JOIN transactions AS transaction_record
          ON transaction_record.transfer_id = transfer_record.id
        WHERE transaction_record.reference <> transfer_record.reference
           OR transaction_record.transaction_type <> 'TRANSFER'
           OR transaction_record.status <> transfer_record.status
           OR transaction_record.amount <> transfer_record.amount
           OR transaction_record.currency <> transfer_record.currency
    ) THEN
        RAISE EXCEPTION 'existing transfer transaction does not match its transfer';
    END IF;
END;
$$;
-- +goose StatementEnd

ALTER TABLE transactions
    ADD CONSTRAINT transfer_transaction_link
    CHECK (
        (transaction_type = 'TRANSFER' AND transfer_id IS NOT NULL)
        OR
        (transaction_type <> 'TRANSFER' AND transfer_id IS NULL)
    );

CREATE UNIQUE INDEX idx_transactions_transfer_id
    ON transactions(transfer_id)
    WHERE transfer_id IS NOT NULL;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_transfer_transaction_consistent()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    target_transfer_id UUID;
    transfer_reference VARCHAR(100);
    transfer_status VARCHAR(20);
    transfer_amount BIGINT;
    transfer_currency CHAR(3);
    transaction_reference VARCHAR(100);
    transaction_type VARCHAR(30);
    transaction_status VARCHAR(20);
    transaction_amount BIGINT;
    transaction_currency CHAR(3);
BEGIN
    IF TG_TABLE_NAME = 'transfers' THEN
        target_transfer_id := NEW.id;
    ELSIF TG_OP = 'INSERT' THEN
        target_transfer_id := NEW.transfer_id;
    ELSE
        IF OLD.transfer_id IS NOT NULL
           AND NEW.transfer_id IS DISTINCT FROM OLD.transfer_id THEN
            RAISE EXCEPTION 'transfer transaction link cannot be changed'
                USING ERRCODE = '23514';
        END IF;
        target_transfer_id := COALESCE(NEW.transfer_id, OLD.transfer_id);
    END IF;

    IF target_transfer_id IS NULL THEN
        RETURN NULL;
    END IF;

    SELECT reference, status, amount, currency
    INTO transfer_reference, transfer_status, transfer_amount, transfer_currency
    FROM transfers
    WHERE id = target_transfer_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'transfer % does not exist', target_transfer_id
            USING ERRCODE = '23503';
    END IF;

    SELECT reference, transaction_type, status, amount, currency
    INTO transaction_reference, transaction_type, transaction_status,
        transaction_amount, transaction_currency
    FROM transactions
    WHERE transfer_id = target_transfer_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'transfer % must have one transaction record', target_transfer_id
            USING ERRCODE = '23514';
    END IF;

    IF transaction_reference <> transfer_reference
       OR transaction_type <> 'TRANSFER'
       OR transaction_status <> transfer_status
       OR transaction_amount <> transfer_amount
       OR transaction_currency <> transfer_currency THEN
        RAISE EXCEPTION 'transaction record does not match transfer %', target_transfer_id
            USING ERRCODE = '23514';
    END IF;

    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER transfer_transaction_consistent
AFTER INSERT OR UPDATE ON transfers
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION assert_transfer_transaction_consistent();

CREATE CONSTRAINT TRIGGER transaction_transfer_consistent
AFTER INSERT OR UPDATE OR DELETE ON transactions
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION assert_transfer_transaction_consistent();

-- +goose Down

DROP TRIGGER transaction_transfer_consistent ON transactions;
DROP TRIGGER transfer_transaction_consistent ON transfers;
DROP FUNCTION assert_transfer_transaction_consistent();
DROP INDEX idx_transactions_transfer_id;
ALTER TABLE transactions DROP CONSTRAINT transfer_transaction_link;
ALTER TABLE transactions DROP COLUMN transfer_id;
