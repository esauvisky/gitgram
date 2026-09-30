-- push_messages remembers which Telegram message announced which branch, so
-- a reaction on a push card can run a pipeline on that branch.
CREATE TABLE push_messages (
  message_id INTEGER PRIMARY KEY, project_id INTEGER NOT NULL, ref TEXT NOT NULL, created_at INTEGER NOT NULL);
