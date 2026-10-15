package audit

import (
	"context"
	"encoding/json"
	"testing"

	db "github.com/Roy6lty/minibank/internal/sqlc"
	"github.com/google/uuid"
)

type repositoryStub struct {
	params db.CreateAuditEventParams
	calls  int
}

func (r *repositoryStub) Create(_ context.Context, params db.CreateAuditEventParams) (db.AuditEvent, error) {
	r.params = params
	r.calls++
	return db.AuditEvent{}, nil
}

func TestRecordMapsEventAndContext(t *testing.T) {
	actorID := uuid.New()
	repo := &repositoryStub{}
	ctx := WithRequestID(WithActorUserID(context.Background(), actorID), "request-123")

	recorder := NewRecorder(repo)
	err := recorder.Record(ctx, Event{
		EventType:    EventAuthLogin,
		Outcome:      OutcomeSuccess,
		ActorUserID:  actorID,
		RequestID:    RequestID(ctx),
		ResourceType: ResourceUser,
		ResourceID:   actorID,
	})
	if err != nil {
		t.Fatalf("Record returned an error: %v", err)
	}

	if repo.calls != 1 {
		t.Fatalf("expected one repository call, got %d", repo.calls)
	}
	if repo.params.EventType != EventAuthLogin || repo.params.Outcome != string(OutcomeSuccess) {
		t.Fatalf("unexpected event fields: %+v", repo.params)
	}
	if !repo.params.ActorUserID.Valid || repo.params.ActorUserID.Bytes != actorID {
		t.Fatal("actor user ID was not mapped")
	}
	if !repo.params.RequestID.Valid || repo.params.RequestID.String != "request-123" {
		t.Fatal("request ID was not mapped")
	}
	if string(repo.params.Details) != "{}" {
		t.Fatalf("expected default details object, got %s", repo.params.Details)
	}
}

func TestRecordRejectsInvalidDetails(t *testing.T) {
	repo := &repositoryStub{}
	recorder := NewRecorder(repo)
	err := recorder.Record(context.Background(), Event{
		EventType: EventAuthLogin,
		Outcome:   OutcomeSuccess,
		Details:   json.RawMessage("not-json"),
	})
	if err == nil {
		t.Fatal("expected invalid details error")
	}
	if repo.calls != 0 {
		t.Fatal("invalid events must not reach the repository")
	}
}

func TestStatusDetails(t *testing.T) {
	details := StatusDetails("COMPLETED")
	if string(details) != `{"status":"COMPLETED"}` {
		t.Fatalf("unexpected status details: %s", details)
	}
}
