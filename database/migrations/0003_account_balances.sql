ALTER TABLE accounts
    ADD COLUMN balance BIGINT NOT NULL DEFAULT 0;

ALTER TABLE accounts
    ADD CONSTRAINT non_negative_account_balance CHECK (balance >= 0);

ALTER TABLE ledger_entries
    ADD CONSTRAINT unique_transfer_account_entry
    UNIQUE (transfer_id, account_id, entry_type);
