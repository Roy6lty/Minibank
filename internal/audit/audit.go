package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	db "github.com/Roy6lty/minibank/internal/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Outcome string

const (
	OutcomeSuccess Outcome = "SUCCESS"
	OutcomeFailure Outcome = "FAILURE"
)

const (
	EventTransferCreated          = "TRANSFER_CREATED"
	EventTransferStatusUpdated    = "TRANSFER_STATUS_UPDATED"
	EventTransactionCreated       = "TRANSACTION_CREATED"
	EventTransactionStatusUpdated = "TRANSACTION_STATUS_UPDATED"
	EventAuthRegister             = "AUTH_REGISTER"
	EventAuthLogin                = "AUTH_LOGIN"
	EventAuthRefresh              = "AUTH_REFRESH"
	EventAuthLogout               = "AUTH_LOGOUT"
	EventAuthenticationFailed     = "AUTHENTICATION_FAILED"
	EventAuthorizationDenied      = "AUTHORIZATION_DENIED"

	ResourceTransfer    = "transfer"
	ResourceTransaction = "transaction"
	ResourceUser        = "user"
	ResourceSession     = "session"
)

type Event struct {
	EventType    string
	Outcome      Outcome
	ActorUserID  uuid.UUID
	RequestID    string
	ResourceType string
	ResourceID   uuid.UUID
	Details      json.RawMessage
}

type EventRepository interface {
	Create(context.Context, db.CreateAuditEventParams) (db.AuditEvent, error)
}

type Recorder interface {
	Record(context.Context, Event) error
}

type recorder struct {
	repository EventRepository
}

func NewRecorder(repository EventRepository) Recorder {
	if repository == nil {
		return nil
	}
	return &recorder{repository: repository}
}

func (e Event) Validate() error {
	if strings.TrimSpace(e.EventType) == "" {
		return fmt.Errorf("audit event type is required")
	}
	if e.Outcome != OutcomeSuccess && e.Outcome != OutcomeFailure {
		return fmt.Errorf("invalid audit event outcome")
	}
	if len(e.Details) > 0 && !json.Valid(e.Details) {
		return fmt.Errorf("audit event details must be valid JSON")
	}
	return nil
}

func (r *recorder) Record(ctx context.Context, event Event) error {
	if err := event.Validate(); err != nil {
		return err
	}

	details := event.Details
	if len(details) == 0 {
		details = json.RawMessage(`{}`)
	}

	_, err := r.repository.Create(ctx, db.CreateAuditEventParams{
		EventType:    event.EventType,
		Outcome:      string(event.Outcome),
		ActorUserID:  nullableUUID(event.ActorUserID),
		RequestID:    nullableText(event.RequestID),
		ResourceType: nullableText(event.ResourceType),
		ResourceID:   nullableUUID(event.ResourceID),
		Details:      details,
	})
	return err
}

func StatusDetails(status string) json.RawMessage {
	details, _ := json.Marshal(struct {
		Status string `json:"status"`
	}{
		Status: status,
	})
	return details
}

func PermissionDetails(permission string) json.RawMessage {
	details, _ := json.Marshal(struct {
		Permission string `json:"permission"`
	}{
		Permission: permission,
	})
	return details
}

func nullableUUID(value uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: value, Valid: value != uuid.Nil}
}

func nullableText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

type contextKey struct{}

type requestIDContextKey struct{ contextKey }
type actorUserIDContextKey struct{ contextKey }

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

func RequestID(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

func WithActorUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, actorUserIDContextKey{}, userID)
}

func ActorUserID(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(actorUserIDContextKey{}).(uuid.UUID)
	return userID, ok && userID != uuid.Nil
}
