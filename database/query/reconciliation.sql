-- name: FindAccountBalanceMismatches :many
SELECT
    a.id AS account_id,
    a.currency,
    a.balance AS account_balance,
    COALESCE(
        SUM(
            CASE
                WHEN le.entry_type = 'CREDIT' THEN le.amount
                ELSE -le.amount
            END
        ),
        0
    )::bigint AS ledger_balance
FROM accounts AS a
LEFT JOIN ledger_entries AS le ON le.account_id = a.id
GROUP BY a.id, a.currency, a.balance
HAVING a.balance <> COALESCE(
    SUM(
        CASE
            WHEN le.entry_type = 'CREDIT' THEN le.amount
            ELSE -le.amount
        END
    ),
    0
)::bigint
ORDER BY a.id;

-- name: FindCompletedTransferMismatches :many
SELECT
    t.id AS transfer_id,
    t.amount AS transfer_amount,
    t.currency,
    COUNT(le.id)::bigint AS entry_count,
    COUNT(*) FILTER (WHERE le.entry_type = 'DEBIT')::bigint AS debit_count,
    COUNT(*) FILTER (WHERE le.entry_type = 'CREDIT')::bigint AS credit_count,
    COALESCE(SUM(le.amount) FILTER (WHERE le.entry_type = 'DEBIT'), 0)::bigint AS debit_amount,
    COALESCE(SUM(le.amount) FILTER (WHERE le.entry_type = 'CREDIT'), 0)::bigint AS credit_amount,
    COUNT(*) FILTER (
        WHERE le.entry_type = 'DEBIT'
          AND le.account_id = t.source_account_id
    )::bigint AS source_debit_count,
    COUNT(*) FILTER (
        WHERE le.entry_type = 'CREDIT'
          AND le.account_id = t.destination_account_id
    )::bigint AS destination_credit_count,
    COUNT(*) FILTER (WHERE le.currency <> t.currency)::bigint AS currency_mismatch_count
FROM transfers AS t
LEFT JOIN ledger_entries AS le ON le.transfer_id = t.id
WHERE t.status = 'COMPLETED'
GROUP BY t.id, t.amount, t.currency, t.source_account_id, t.destination_account_id
HAVING COUNT(le.id) <> 2
    OR COUNT(*) FILTER (WHERE le.entry_type = 'DEBIT') <> 1
    OR COUNT(*) FILTER (WHERE le.entry_type = 'CREDIT') <> 1
    OR COALESCE(SUM(le.amount) FILTER (WHERE le.entry_type = 'DEBIT'), 0) <> t.amount
    OR COALESCE(SUM(le.amount) FILTER (WHERE le.entry_type = 'CREDIT'), 0) <> t.amount
    OR COUNT(*) FILTER (
        WHERE le.entry_type = 'DEBIT'
          AND le.account_id = t.source_account_id
    ) <> 1
    OR COUNT(*) FILTER (
        WHERE le.entry_type = 'CREDIT'
          AND le.account_id = t.destination_account_id
    ) <> 1
    OR COUNT(*) FILTER (WHERE le.currency <> t.currency) <> 0
ORDER BY t.id;

-- name: RecordReconciliationMismatch :exec
INSERT INTO reconciliation_mismatches (
    mismatch_key,
    mismatch_type,
    account_id,
    transfer_id,
    expected_amount,
    actual_amount,
    details
)
VALUES (
    sqlc.arg(mismatch_key),
    sqlc.arg(mismatch_type),
    sqlc.narg(account_id),
    sqlc.narg(transfer_id),
    sqlc.narg(expected_amount),
    sqlc.narg(actual_amount),
    sqlc.arg(details)
)
ON CONFLICT (mismatch_key) DO UPDATE SET
    mismatch_type = EXCLUDED.mismatch_type,
    account_id = EXCLUDED.account_id,
    transfer_id = EXCLUDED.transfer_id,
    expected_amount = EXCLUDED.expected_amount,
    actual_amount = EXCLUDED.actual_amount,
    details = EXCLUDED.details,
    last_detected_at = NOW(),
    resolved_at = NULL;

-- name: ResolveStaleReconciliationMismatches :exec
UPDATE reconciliation_mismatches
SET resolved_at = NOW()
WHERE resolved_at IS NULL
  AND last_detected_at < NOW();

-- name: ListOpenReconciliationMismatches :many
SELECT
    id,
    mismatch_key,
    mismatch_type,
    account_id,
    transfer_id,
    expected_amount,
    actual_amount,
    details,
    first_detected_at,
    last_detected_at,
    resolved_at
FROM reconciliation_mismatches
WHERE resolved_at IS NULL
ORDER BY last_detected_at DESC;
