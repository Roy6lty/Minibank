-- +goose Up

CREATE TABLE audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    event_type VARCHAR(60) NOT NULL,
    outcome VARCHAR(20) NOT NULL,

    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    request_id VARCHAR(128),

    resource_type VARCHAR(40),
    resource_id UUID,

    details JSONB NOT NULL DEFAULT '{}'::jsonb,

    CONSTRAINT non_empty_audit_event_type
        CHECK (BTRIM(event_type) <> ''),

    CONSTRAINT valid_audit_event_outcome
        CHECK (outcome IN ('SUCCESS', 'FAILURE'))
);

CREATE INDEX idx_audit_events_occurred_at
    ON audit_events(occurred_at DESC);

CREATE INDEX idx_audit_events_event_type
    ON audit_events(event_type);

CREATE INDEX idx_audit_events_actor_user_id
    ON audit_events(actor_user_id);

CREATE INDEX idx_audit_events_resource
    ON audit_events(resource_type, resource_id);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION prevent_audit_event_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'audit events are immutable';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER audit_events_no_update
BEFORE UPDATE ON audit_events
FOR EACH ROW
EXECUTE FUNCTION prevent_audit_event_mutation();

CREATE TRIGGER audit_events_no_delete
BEFORE DELETE ON audit_events
FOR EACH ROW
EXECUTE FUNCTION prevent_audit_event_mutation();

-- +goose Down

DROP TRIGGER audit_events_no_delete ON audit_events;
DROP TRIGGER audit_events_no_update ON audit_events;
DROP FUNCTION prevent_audit_event_mutation();
DROP TABLE audit_events;
