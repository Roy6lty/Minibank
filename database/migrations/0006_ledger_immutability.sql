-- +goose Up

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION prevent_ledger_entry_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'ledger entries are immutable';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER ledger_entries_no_update
BEFORE UPDATE ON ledger_entries
FOR EACH ROW
EXECUTE FUNCTION prevent_ledger_entry_mutation();

CREATE TRIGGER ledger_entries_no_delete
BEFORE DELETE ON ledger_entries
FOR EACH ROW
EXECUTE FUNCTION prevent_ledger_entry_mutation();

-- +goose Down

DROP TRIGGER ledger_entries_no_delete ON ledger_entries;
DROP TRIGGER ledger_entries_no_update ON ledger_entries;
DROP FUNCTION prevent_ledger_entry_mutation();
