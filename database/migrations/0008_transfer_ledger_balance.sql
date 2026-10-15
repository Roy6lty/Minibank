-- +goose Up

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_completed_transfer_ledger_balanced()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    entry_count BIGINT;
    debit_count BIGINT;
    credit_count BIGINT;
    debit_amount BIGINT;
    credit_amount BIGINT;
    source_debit_count BIGINT;
    destination_credit_count BIGINT;
    currency_mismatch_count BIGINT;
BEGIN
    IF NEW.status <> 'COMPLETED' THEN
        RETURN NULL;
    END IF;

    SELECT
        COUNT(le.id),
        COUNT(*) FILTER (WHERE le.entry_type = 'DEBIT'),
        COUNT(*) FILTER (WHERE le.entry_type = 'CREDIT'),
        COALESCE(SUM(le.amount) FILTER (WHERE le.entry_type = 'DEBIT'), 0)::bigint,
        COALESCE(SUM(le.amount) FILTER (WHERE le.entry_type = 'CREDIT'), 0)::bigint,
        COUNT(*) FILTER (
            WHERE le.entry_type = 'DEBIT'
              AND le.account_id = NEW.source_account_id
        ),
        COUNT(*) FILTER (
            WHERE le.entry_type = 'CREDIT'
              AND le.account_id = NEW.destination_account_id
        ),
        COUNT(*) FILTER (WHERE le.currency <> NEW.currency)
    INTO
        entry_count,
        debit_count,
        credit_count,
        debit_amount,
        credit_amount,
        source_debit_count,
        destination_credit_count,
        currency_mismatch_count
    FROM ledger_entries AS le
    WHERE le.transfer_id = NEW.id;

    IF entry_count <> 2
       OR debit_count <> 1
       OR credit_count <> 1
       OR debit_amount <> NEW.amount
       OR credit_amount <> NEW.amount
       OR source_debit_count <> 1
       OR destination_credit_count <> 1
       OR currency_mismatch_count <> 0 THEN
        RAISE EXCEPTION 'completed transfer % has an unbalanced ledger', NEW.id
            USING ERRCODE = '23514';
    END IF;

    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER completed_transfer_ledger_balanced
AFTER INSERT OR UPDATE ON transfers
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION assert_completed_transfer_ledger_balanced();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION reject_ledger_entry_for_completed_transfer()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    transfer_status VARCHAR(20);
BEGIN
    SELECT status
    INTO transfer_status
    FROM transfers
    WHERE id = NEW.transfer_id;

    IF transfer_status = 'COMPLETED' THEN
        RAISE EXCEPTION 'cannot add a ledger entry to completed transfer %', NEW.transfer_id
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER ledger_entries_no_post_completion
BEFORE INSERT ON ledger_entries
FOR EACH ROW
EXECUTE FUNCTION reject_ledger_entry_for_completed_transfer();

-- +goose Down

DROP TRIGGER ledger_entries_no_post_completion ON ledger_entries;
DROP TRIGGER completed_transfer_ledger_balanced ON transfers;
DROP FUNCTION reject_ledger_entry_for_completed_transfer();
DROP FUNCTION assert_completed_transfer_ledger_balanced();
