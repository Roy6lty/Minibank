-- +goose Up

CREATE TABLE reconciliation_mismatches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mismatch_key TEXT NOT NULL UNIQUE,
    mismatch_type VARCHAR(40) NOT NULL,
    account_id UUID REFERENCES accounts(id),
    transfer_id UUID REFERENCES transfers(id),
    expected_amount BIGINT,
    actual_amount BIGINT,
    details TEXT NOT NULL,
    first_detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,

    CONSTRAINT valid_reconciliation_mismatch_type
        CHECK (mismatch_type IN ('ACCOUNT_BALANCE', 'TRANSFER_ENTRIES')),

    CONSTRAINT reconciliation_mismatch_reference
        CHECK (
            (mismatch_type = 'ACCOUNT_BALANCE' AND account_id IS NOT NULL AND transfer_id IS NULL)
            OR
            (mismatch_type = 'TRANSFER_ENTRIES' AND account_id IS NULL AND transfer_id IS NOT NULL)
        )
);

CREATE INDEX idx_reconciliation_mismatches_open
    ON reconciliation_mismatches(last_detected_at DESC)
    WHERE resolved_at IS NULL;

-- +goose Down

DROP TABLE reconciliation_mismatches;
