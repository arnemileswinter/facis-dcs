-- The outbox events a consumer that must not miss one (webhook delivery,
-- automatic deployment) has handled (DCS-NFR-SF-01). Such a consumer reads
-- the outbox table: core NATS drops an event that no member of a queue group
-- receives, for example during a restart.
CREATE TABLE IF NOT EXISTS outbox_consumer_progress
(
    consumer        VARCHAR(64) NOT NULL,
    outbox_event_id BIGINT      NOT NULL,
    handled_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer, outbox_event_id)
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_type_created
    ON outbox_events (event_type, created_at);
