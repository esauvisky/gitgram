package webhook

import (
	"fmt"

	"github.com/esauvisky/gitgram/internal/event"
)

type rawMergeRequest struct {
	User             rawUser    `json:"user"`
	Project          rawProject `json:"project"`
	ObjectAttributes struct {
		ID                  int64  `json:"id"`
		IID                 int64  `json:"iid"`
		Title               string `json:"title"`
		Description         string `json:"description"`
		URL                 string `json:"url"`
		State               string `json:"state"`
		Action              string `json:"action"`
		Draft               bool   `json:"draft"`
		WorkInProgress      bool   `json:"work_in_progress"`
		SourceBranch        string `json:"source_branch"`
		TargetBranch        string `json:"target_branch"`
		AuthorID            int64  `json:"author_id"`
		DetailedMergeStatus string `json:"detailed_merge_status"`
		HeadPipelineID      *int64 `json:"head_pipeline_id"`
		MergedAt            ts     `json:"merged_at"`
	} `json:"object_attributes"`
}

func parseMergeRequest(body []byte, meta event.Meta) (event.Event, error) {
	var raw rawMergeRequest
	if err := decode(body, kindMergeRequest, &raw); err != nil {
		return nil, err
	}
	oa := raw.ObjectAttributes
	user := raw.User.event()
	project := raw.Project.event()
	mr := &event.MergeRequest{
		Meta:                meta,
		Project:             project,
		User:                user,
		Action:              oa.Action,
		ID:                  oa.ID,
		IID:                 oa.IID,
		Title:               oa.Title,
		Description:         oa.Description,
		URL:                 oa.URL,
		State:               oa.State,
		Draft:               oa.Draft || oa.WorkInProgress,
		SourceBranch:        oa.SourceBranch,
		TargetBranch:        oa.TargetBranch,
		DetailedMergeStatus: oa.DetailedMergeStatus,
		HeadPipelineID:      oa.HeadPipelineID,
		MergedAt:            oa.MergedAt.ptr(),
	}
	// Payloads only carry author_id; the acting user is the author when the
	// ids match. An id-only user would clobber the author already in state.
	if oa.AuthorID != 0 && user.ID == oa.AuthorID {
		mr.Author = user
	}
	if mr.URL == "" {
		mr.URL = fmt.Sprintf("%s/-/merge_requests/%d", project.WebURL, oa.IID)
	}
	return mr, nil
}

// rawMRRef is the merge_request{} sibling object in pipeline payloads.
type rawMRRef struct {
	IID   int64  `json:"iid"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

func (m *rawMRRef) event(project event.Project) *event.MRRef {
	if m == nil || m.IID == 0 {
		return nil
	}
	url := m.URL
	if url == "" {
		url = fmt.Sprintf("%s/-/merge_requests/%d", project.WebURL, m.IID)
	}
	return &event.MRRef{IID: m.IID, Title: m.Title, URL: url}
}
