-- telegram.chat_id may list several chats. card_messages keeps the card in
-- the first (primary) chat; card_mirrors holds the same card's message in
-- every other chat. sent_chats records, on send and reply rows, the chats
-- already delivered so a retry never posts twice.
CREATE TABLE card_mirrors (
  kind TEXT NOT NULL, project_id INTEGER NOT NULL, object_id INTEGER NOT NULL,
  chat_id INTEGER NOT NULL,
  thread_id INTEGER, message_id INTEGER, rendered_hash TEXT,
  status TEXT NOT NULL DEFAULT 'live',
  created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
  PRIMARY KEY (kind, project_id, object_id, chat_id),
  FOREIGN KEY (kind, project_id, object_id) REFERENCES object_state ON DELETE CASCADE);
ALTER TABLE outbox ADD COLUMN sent_chats TEXT NOT NULL DEFAULT '';
