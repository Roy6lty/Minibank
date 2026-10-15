package reconciliation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	db "github.com/Roy6lty/minibank/internal/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	accountBalanceMismatchType  = "ACCOUNT_BALANCE"
	transferEntriesMismatchType = "TRANSFER_ENTRIES"
)

type Job struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func NewJob(pool *pgxpool.Pool, logger *slog.Logger) *Job {
	if logger == nil {
		logger = slog.Default()
	}
	return &Job{pool: pool, logger: logger}
}

func (j *Job) Run(ctx context.Context) error {
	tx, err := j.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return fmt.Errorf("begin reconciliation transaction: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(rollbackCtx)
	}()

	queries := db.New(tx)
	accountMismatches, err := queries.FindAccountBalanceMismatches(ctx)
	if err != nil {
		return fmt.Errorf("find account balance mismatches: %w", err)
	}
	transferMismatches, err := queries.FindCompletedTransferMismatches(ctx)
	if err != nil {
		return fmt.Errorf("find transfer ledger mismatches: %w", err)
	}
	if len(accountMismatches) > 0 || len(transferMismatches) > 0 {
		j.logger.Error(
			"reconciliation mismatches detected",
			"account_mismatches", len(accountMismatches),
			"transfer_mismatches", len(transferMismatches),
		)
	}

	for _, mismatch := range accountMismatches {
		if err := queries.RecordReconciliationMismatch(ctx, db.RecordReconciliationMismatchParams{
			MismatchKey:    "account-balance:" + uuidFromPgUUID(mismatch.AccountID).String(),
			MismatchType:   accountBalanceMismatchType,
			AccountID:      mismatch.AccountID,
			ExpectedAmount: pgtype.Int8{Int64: mismatch.LedgerBalance, Valid: true},
			ActualAmount:   pgtype.Int8{Int64: mismatch.AccountBalance, Valid: true},
			Details: fmt.Sprintf(
				"account balance mismatch: stored=%d ledger=%d currency=%s",
				mismatch.AccountBalance,
				mismatch.LedgerBalance,
				mismatch.Currency,
			),
		}); err != nil {
			return fmt.Errorf("record account balance mismatch: %w", err)
		}
	}

	for _, mismatch := range transferMismatches {
		if err := queries.RecordReconciliationMismatch(ctx, db.RecordReconciliationMismatchParams{
			MismatchKey:    "transfer-entries:" + uuidFromPgUUID(mismatch.TransferID).String(),
			MismatchType:   transferEntriesMismatchType,
			TransferID:     mismatch.TransferID,
			ExpectedAmount: pgtype.Int8{Int64: mismatch.TransferAmount, Valid: true},
			ActualAmount:   pgtype.Int8{Int64: mismatch.DebitAmount, Valid: true},
			Details: fmt.Sprintf(
				"completed transfer ledger mismatch: entries=%d debit_count=%d credit_count=%d debit_amount=%d credit_amount=%d source_debit_count=%d destination_credit_count=%d currency_mismatch=%t",
				mismatch.EntryCount,
				mismatch.DebitCount,
				mismatch.CreditCount,
				mismatch.DebitAmount,
				mismatch.CreditAmount,
				mismatch.SourceDebitCount,
				mismatch.DestinationCreditCount,
				mismatch.CurrencyMismatchCount > 0,
			),
		}); err != nil {
			return fmt.Errorf("record transfer ledger mismatch: %w", err)
		}
	}

	if err := queries.ResolveStaleReconciliationMismatches(ctx); err != nil {
		return fmt.Errorf("resolve stale reconciliation mismatches: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit reconciliation results: %w", err)
	}
	committed = true
	return nil
}

func (j *Job) RunPeriodically(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		j.logger.Error("reconciliation job disabled: interval must be positive")
		return
	}

	run := func() {
		if err := j.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			j.logger.Error("reconciliation job failed", slog.Any("error", err))
		}
	}
	run()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			run()
		case <-ctx.Done():
			return
		}
	}
}

func uuidFromPgUUID(id pgtype.UUID) uuid.UUID {
	return uuid.UUID(id.Bytes)
}
