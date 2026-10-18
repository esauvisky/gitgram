package cards

import (
	"slices"
	"time"

	"github.com/esauvisky/gitgram/internal/event"
)

// PackageState is the state behind one package card: one card per
// published version (for images, one digest), edited as more tags land
// on it.
type PackageState struct {
	SchemaVer int
	Project   event.Project
	Publisher event.User
	Type      string
	Name      string
	Image     string
	URL       string
	// Versions are the image's tags in arrival order, without duplicates;
	// the one version of any other package.
	Versions    []string
	LastEventAt time.Time
}

// PackageKey returns the object_state key of a package version.
func PackageKey(projectID, versionID int64) Key {
	return Key{Kind: KindPackage, ProjectID: projectID, ObjectID: versionID}
}

// ReducePackage folds one publication into s.
func ReducePackage(s *PackageState, ev *event.Package) (changed bool) {
	if s.SchemaVer == 0 {
		s.SchemaVer = SchemaVer
	}
	before := snapshot(s)
	s.Project = ev.Project
	s.Publisher = ev.User
	s.Type = ev.Type
	s.Name = ev.Name
	s.Image = ev.Image
	s.URL = ev.URL
	if !slices.Contains(s.Versions, ev.Version) {
		s.Versions = append(s.Versions, ev.Version)
	}
	if ev.Received.After(s.LastEventAt) {
		s.LastEventAt = ev.Received
	}
	return differs(before, s)
}
