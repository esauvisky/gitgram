package store

import (
	"context"
	"fmt"
)

// AddLink records a directed link; adding the same link twice is a no-op.
func (q queries) AddLink(ctx context.Context, l Link) error {
	_, err := q.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO object_links
		   (project_id, kind, object_id, rel, to_project_id, to_kind, to_object_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		l.From.ProjectID, l.From.Kind, l.From.ObjectID, l.Rel, l.To.ProjectID, l.To.Kind, l.To.ObjectID)
	if err != nil {
		return fmt.Errorf("add link %v -%s-> %v: %w", l.From, l.Rel, l.To, err)
	}
	return nil
}

// LinksFrom returns every link with the given rel leaving from.
func (q queries) LinksFrom(ctx context.Context, from Key, rel string) ([]Link, error) {
	links, err := q.queryLinks(ctx,
		`SELECT project_id, kind, object_id, rel, to_project_id, to_kind, to_object_id FROM object_links
		 WHERE project_id = ? AND kind = ? AND object_id = ? AND rel = ?
		 ORDER BY to_kind, to_project_id, to_object_id`,
		from.ProjectID, from.Kind, from.ObjectID, rel)
	if err != nil {
		return nil, fmt.Errorf("links from %v (%s): %w", from, rel, err)
	}
	return links, nil
}

// LinksTo returns every link with the given rel pointing at to.
func (q queries) LinksTo(ctx context.Context, to Key, rel string) ([]Link, error) {
	links, err := q.queryLinks(ctx,
		`SELECT project_id, kind, object_id, rel, to_project_id, to_kind, to_object_id FROM object_links
		 WHERE to_project_id = ? AND to_kind = ? AND to_object_id = ? AND rel = ?
		 ORDER BY kind, project_id, object_id`,
		to.ProjectID, to.Kind, to.ObjectID, rel)
	if err != nil {
		return nil, fmt.Errorf("links to %v (%s): %w", to, rel, err)
	}
	return links, nil
}

func (q queries) queryLinks(ctx context.Context, query string, args ...any) ([]Link, error) {
	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Link
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.From.ProjectID, &l.From.Kind, &l.From.ObjectID, &l.Rel,
			&l.To.ProjectID, &l.To.Kind, &l.To.ObjectID); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
