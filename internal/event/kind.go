package event

// Kind names an event class. Values match GitLab's object_kind where one
// exists ("build" is normalized to "job").
type Kind string

// Event kinds.
const (
	KindPush         Kind = "push"
	KindTagPush      Kind = "tag_push"
	KindPipeline     Kind = "pipeline"
	KindJob          Kind = "job"
	KindMergeRequest Kind = "merge_request"
	KindNote         Kind = "note"
	KindIssue        Kind = "issue"
	KindRelease      Kind = "release"
	KindDeployment   Kind = "deployment"
)

// Kinds lists every kind in a stable order.
var Kinds = []Kind{
	KindPush, KindTagPush, KindPipeline, KindJob, KindMergeRequest,
	KindNote, KindIssue, KindRelease, KindDeployment,
}

// String returns the kind as a string.
func (k Kind) String() string { return string(k) }
