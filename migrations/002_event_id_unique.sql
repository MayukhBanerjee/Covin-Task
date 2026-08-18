-- Add a UNIQUE constraint on events.event_id so the database itself enforces
-- idempotency.  The application-level EventExists check is a fast path; this
-- constraint is the hard guarantee that no two rows with the same event_id can
-- ever exist, even under concurrent inserts that race past the check.
ALTER TABLE events ADD CONSTRAINT events_event_id_unique UNIQUE (event_id);
