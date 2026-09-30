-- generation is bumped every time a coalesced card enqueue hits the existing
-- row, so the sender only deletes a row it fully rendered.
ALTER TABLE outbox ADD COLUMN generation INTEGER NOT NULL DEFAULT 0;
