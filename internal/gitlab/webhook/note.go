package webhook

import (
	"github.com/esauvisky/gitgram/internal/event"
)

type rawNote struct {
	User             rawUser    `json:"user"`
	Project          rawProject `json:"project"`
	ObjectAttributes struct {
		ID           int64  `json:"id"`
		Note         string `json:"note"`
		NoteableType string `json:"noteable_type"`
		URL          string `json:"url"`
		DiscussionID string `json:"discussion_id"`
		Type         string `json:"type"`
		System       bool   `json:"system"`
		Action       string `json:"action"`
		CommitID     string `json:"commit_id"`
		CreatedAt    ts     `json:"created_at"`
		UpdatedAt    ts     `json:"updated_at"`
		Position     *struct {
			OldPath string `json:"old_path"`
			NewPath string `json:"new_path"`
			OldLine *int   `json:"old_line"`
			NewLine *int   `json:"new_line"`
		} `json:"position"`
	} `json:"object_attributes"`
	MergeRequest *rawMRRef    `json:"merge_request"`
	Issue        *rawIssueRef `json:"issue"`
	Commit       *struct {
		ID string `json:"id"`
	} `json:"commit"`
}

func parseNote(body []byte, meta event.Meta) (event.Event, error) {
	var raw rawNote
	if err := decode(body, kindNote, &raw); err != nil {
		return nil, err
	}
	oa := raw.ObjectAttributes
	project := raw.Project.event()

	n := &event.Note{
		Meta:         meta,
		Project:      project,
		User:         raw.User.event(),
		ID:           oa.ID,
		NoteableType: oa.NoteableType,
		Body:         oa.Note,
		URL:          oa.URL,
		DiscussionID: oa.DiscussionID,
		IsDiff:       oa.Type == "DiffNote",
		System:       oa.System,
		Action:       oa.Action,
		CreatedAt:    oa.CreatedAt.Time,
		UpdatedAt:    oa.UpdatedAt.Time,
	}
	if n.Action == "" {
		n.Action = event.NoteActionCreate
	}
	if pos := oa.Position; pos != nil {
		n.FilePath = pos.NewPath
		if n.FilePath == "" {
			n.FilePath = pos.OldPath
		}
		switch {
		case pos.NewLine != nil:
			n.Line = *pos.NewLine
		case pos.OldLine != nil:
			n.Line = *pos.OldLine
		}
	}
	switch oa.NoteableType {
	case event.NoteableMergeRequest:
		n.MR = raw.MergeRequest.event(project)
	case event.NoteableIssue:
		n.Issue = raw.Issue.event(project)
	case event.NoteableCommit:
		n.CommitSHA = oa.CommitID
		if n.CommitSHA == "" && raw.Commit != nil {
			n.CommitSHA = raw.Commit.ID
		}
	}
	return n, nil
}
