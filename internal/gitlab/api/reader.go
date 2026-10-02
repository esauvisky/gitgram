package api

import "context"

// Reader is the read-only surface the engine and reconciler need. It exists
// so v2 can add a Writer (retry, cancel, approve, merge) with per-user tokens
// without touching the v1 consumers.
type Reader interface {
	// Compare is GET /projects/:id/repository/compare. With straight=true it
	// compares from→to directly instead of via the merge base; a non-empty
	// Commits for from=after,to=before means the branch history was rewritten.
	Compare(ctx context.Context, projectID int64, from, to string, straight bool) (*Compare, error)
	// Pipeline is GET /projects/:id/pipelines/:pipeline_id.
	Pipeline(ctx context.Context, projectID, id int64) (*Pipeline, error)
	// PipelineJobs pages through GET .../pipelines/:pipeline_id/jobs;
	// includeRetried adds retried runs (include_retried=true).
	PipelineJobs(ctx context.Context, projectID, id int64, includeRetried bool) ([]Job, error)
	// JobTrace is GET /projects/:id/jobs/:job_id/trace: the job's raw log.
	JobTrace(ctx context.Context, projectID, jobID int64) (string, error)
	// GroupProjects lists every non-archived project under group (path or id),
	// subgroups included, excluding projects merely shared with the group.
	GroupProjects(ctx context.Context, group string) ([]Project, error)
}

var _ Reader = (*Client)(nil)

// User is a GitLab user as returned by the REST API.
type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	WebURL    string `json:"web_url"`
}
