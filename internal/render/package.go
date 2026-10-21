package render

import (
	"strconv"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// Package renders a package card: the title saying what kind of package
// was published, linked to its page (`A new Docker image was pushed to
// demo`, `by @ada` when the registry names the publisher), then one code
// line per tag or version with the reference it is fetched by
// (`ghcr.io/acme/demo:v1.4.0`).
func Package(s *cards.PackageState, o Options) Message {
	var b htmlfmt.Builder
	verb := "published"
	if s.Image != "" {
		verb = "pushed"
	}
	lead := "A new " + htmlfmt.A(packageNoun(s.Type), s.URL) + " was " + verb
	if !s.Publisher.IsZero() {
		lead += " by " + o.who(s.Publisher)
	}
	headline(&b, lead, "to", s.Project, "")
	for _, v := range s.Versions {
		b.Line(htmlfmt.Code(versionRef(s, v)))
	}
	taglineFooter(&b, "package:"+strconv.FormatInt(s.Project.ID, 10)+":"+s.URL)
	return Message{HTML: b.Truncate(o.limit(), s.URL)}
}

// packageNoun names what was published.
func packageNoun(typ string) string {
	switch typ {
	case event.PackageContainer, event.PackageDocker:
		return "Docker image"
	case event.PackageNPM:
		return "npm package"
	case event.PackageMaven:
		return "Maven package"
	case event.PackageRubyGems:
		return "Ruby gem"
	case event.PackageNuGet:
		return "NuGet package"
	case event.PackagePyPI:
		return "PyPI package"
	case event.PackageGo:
		return "Go module"
	case event.PackageHelm:
		return "Helm chart"
	case event.PackageTerraform:
		return "Terraform module"
	case event.PackageGeneric:
		return "generic package"
	case event.PackageConan:
		return "Conan package"
	case event.PackageComposer:
		return "Composer package"
	case event.PackageDebian:
		return "Debian package"
	}
	return "" + typ + " package"
}

// versionRef is one tag or version as its ecosystem references it:
// `image:tag`, `name@version` for npm and Go, `name==version` for PyPI,
// `name version` otherwise.
func versionRef(s *cards.PackageState, v string) string {
	switch s.Type {
	case event.PackageContainer, event.PackageDocker:
		return s.Image + ":" + v
	case event.PackageNPM, event.PackageGo:
		return s.Name + "@" + v
	case event.PackagePyPI:
		return s.Name + "==" + v
	}
	return s.Name + " " + v
}
