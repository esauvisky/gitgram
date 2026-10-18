-- GitLab sends no webhook for published packages or container images, so
-- the registry watcher polls and keeps what it has already seen here: one
-- JSON blob per GitLab group.
CREATE TABLE registry_watch (
  scope TEXT PRIMARY KEY,
  state TEXT NOT NULL,
  updated_at INTEGER NOT NULL);
