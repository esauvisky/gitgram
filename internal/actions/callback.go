// Package actions is the v2 seam for write actions triggered from Telegram
// inline buttons: the callback_data codec, the Capabilities check render uses
// to decide which buttons to draw, and the Dispatcher the callback-query
// route calls. v1 ships None and Unsupported only; nothing here depends on
// other packages.
package actions

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Version is the callback format version written by Encode.
const Version uint8 = 1

// MaxLen is Telegram's callback_data limit in bytes.
const MaxLen = 64

// Kind is the object a callback targets.
type Kind byte

// Callback kinds.
const (
	KindPipeline Kind = 'p'
	KindJob      Kind = 'j'
)

// Action names what the button does.
type Action string

// Actions foreseen for v2. Capabilities decides which are offered.
const (
	// ActionCancel asks to stop a pipeline; the card then offers
	// ActionCancelYes (stop it) and ActionCancelNo (keep it running).
	ActionCancel    Action = "cancel"
	ActionCancelYes Action = "cancelyes"
	ActionCancelNo  Action = "cancelno"
	ActionRetry     Action = "retry"
	// ActionPlay starts a manual job (KindJob, ObjectID is the job id).
	ActionPlay    Action = "play"
	ActionApprove Action = "approve"
	ActionMerge   Action = "merge"
	ActionRefresh Action = "refresh"
)

// Callback is the decoded callback_data of an inline button. The wire format
// is "ver:kind:projectID:objectID:action", e.g. "1:p:1234:987654:cancel".
type Callback struct {
	Ver       uint8
	Kind      Kind
	ProjectID int64
	ObjectID  int64
	Action    Action
}

// ErrInvalid is wrapped by Decode errors.
var ErrInvalid = errors.New("actions: invalid callback data")

// ErrTooLong is returned by Encode when the result exceeds MaxLen bytes.
var ErrTooLong = errors.New("actions: callback data exceeds 64 bytes")

// Encode serialises the callback. Ver 0 is written as Version.
func (c Callback) Encode() (string, error) {
	ver := c.Ver
	if ver == 0 {
		ver = Version
	}
	if c.Kind == 0 || c.Kind == ':' {
		return "", fmt.Errorf("%w: kind %q", ErrInvalid, c.Kind)
	}
	if c.Action == "" || strings.ContainsRune(string(c.Action), ':') {
		return "", fmt.Errorf("%w: action %q", ErrInvalid, c.Action)
	}
	s := strconv.FormatUint(uint64(ver), 10) + ":" + string(c.Kind) + ":" +
		strconv.FormatInt(c.ProjectID, 10) + ":" + strconv.FormatInt(c.ObjectID, 10) + ":" + string(c.Action)
	if len(s) > MaxLen {
		return "", fmt.Errorf("%w: %d", ErrTooLong, len(s))
	}
	return s, nil
}

// Decode parses callback_data produced by Encode. Errors wrap ErrInvalid.
func Decode(s string) (Callback, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 5 {
		return Callback{}, fmt.Errorf("%w: %q", ErrInvalid, s)
	}
	ver, err := strconv.ParseUint(parts[0], 10, 8)
	if err != nil {
		return Callback{}, fmt.Errorf("%w: version: %v", ErrInvalid, err)
	}
	if len(parts[1]) != 1 {
		return Callback{}, fmt.Errorf("%w: kind %q", ErrInvalid, parts[1])
	}
	projectID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return Callback{}, fmt.Errorf("%w: project id: %v", ErrInvalid, err)
	}
	objectID, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return Callback{}, fmt.Errorf("%w: object id: %v", ErrInvalid, err)
	}
	if parts[4] == "" {
		return Callback{}, fmt.Errorf("%w: empty action", ErrInvalid)
	}
	c := Callback{
		Ver: uint8(ver), Kind: Kind(parts[1][0]), ProjectID: projectID, ObjectID: objectID,
		Action: Action(parts[4]),
	}
	return c, nil
}
