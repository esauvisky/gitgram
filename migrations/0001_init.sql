CREATE TABLE webhook_deliveries (key TEXT PRIMARY KEY, event_kind TEXT NOT NULL, received_at INTEGER NOT NULL);
CREATE INDEX ix_deliveries_received ON webhook_deliveries(received_at);   -- pruned > 7d

CREATE TABLE object_state (
  kind TEXT NOT NULL, project_id INTEGER NOT NULL, object_id INTEGER NOT NULL,  -- pipeline: id; mr/issue: iid
  state_json BLOB NOT NULL, schema_ver INTEGER NOT NULL DEFAULT 1,
  final INTEGER NOT NULL DEFAULT 0, last_event_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
  PRIMARY KEY (kind, project_id, object_id));
CREATE INDEX ix_object_state_open ON object_state(kind, final, updated_at);

CREATE TABLE card_messages (
  kind TEXT NOT NULL, project_id INTEGER NOT NULL, object_id INTEGER NOT NULL,
  thread_id INTEGER, message_id INTEGER, rendered_hash TEXT,
  status TEXT NOT NULL DEFAULT 'live',            -- live | deleted | uneditable
  created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
  PRIMARY KEY (kind, project_id, object_id),
  FOREIGN KEY (kind, project_id, object_id) REFERENCES object_state ON DELETE CASCADE);
CREATE INDEX ix_card_messages_msg ON card_messages(message_id);

CREATE TABLE object_links (                       -- mr <-> head_pipeline, parent <-> child pipeline
  project_id INTEGER, kind TEXT, object_id INTEGER, rel TEXT,
  to_project_id INTEGER, to_kind TEXT, to_object_id INTEGER,
  PRIMARY KEY (project_id, kind, object_id, rel, to_project_id, to_kind, to_object_id));

CREATE TABLE outbox (
  id INTEGER PRIMARY KEY AUTOINCREMENT, thread_id INTEGER,
  op TEXT NOT NULL,                               -- send | reply | card
  card_kind TEXT, card_project_id INTEGER, card_object_id INTEGER,   -- card: target; reply: anchor
  payload BLOB,                                   -- send/reply: render.Message JSON; card: NULL
  not_before INTEGER NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, last_error TEXT, created_at INTEGER NOT NULL);
CREATE UNIQUE INDEX ux_outbox_card ON outbox(card_kind, card_project_id, card_object_id) WHERE op = 'card';
