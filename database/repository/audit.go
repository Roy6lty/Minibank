package repository

import (
	"context"

	db "github.com/Roy6lty/minibank/internal/sqlc"
)

type AuditRepository interface {
	Create(
		ctx context.Context,
		arg db.CreateAuditEventParams,
	) (db.AuditEvent, error)

	GetByResource(
		ctx context.Context,
		arg db.ListAuditEventsByResourceParams,
	) ([]db.AuditEvent, error)

	ListRecent(
		ctx context.Context,
		arg db.ListRecentAuditEventsParams,
	) ([]db.AuditEvent, error)
}

type auditRepository struct {
	queries *db.Queries
}

func NewAuditRepository(queries *db.Queries) AuditRepository {
	return &auditRepository{queries: queries}
}

func (r *auditRepository) Create(
	ctx context.Context,
	arg db.CreateAuditEventParams,
) (db.AuditEvent, error) {
	return r.queries.CreateAuditEvent(ctx, arg)
}

func (r *auditRepository) GetByResource(
	ctx context.Context,
	arg db.ListAuditEventsByResourceParams,
) ([]db.AuditEvent, error) {
	return r.queries.ListAuditEventsByResource(ctx, arg)
}

func (r *auditRepository) ListRecent(
	ctx context.Context,
	arg db.ListRecentAuditEventsParams,
) ([]db.AuditEvent, error) {
	return r.queries.ListRecentAuditEvents(ctx, arg)
}
