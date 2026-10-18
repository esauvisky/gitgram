package render

import (
	"strconv"
	"strings"

	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// Package renders a package card: the title saying what kind of package
// was published (`@ada published a Docker image in demo`), the package
// reference in bold linked to the version page, its tags or version as
// code chips, and the command that installs it as a copyable block.
func Package(s *cards.PackageState, o Options) Message {
	var b htmlfmt.Builder
	headline(&b, o.who(s.Publisher)+" published "+htmlfmt.Esc(packageNoun(s.Type)), "in", s.Project, "")
	b.Line("<b>" + htmlfmt.A(packageRef(s), s.URL) + "</b>")
	chips := make([]string, len(s.Versions))
	for i, v := range s.Versions {
		chips[i] = htmlfmt.Code(v)
	}
	b.Line(strings.Join(chips, " · "))
	if cmd := installCommand(s); cmd != "" {
		b.Line("")
		b.Line(htmlfmt.Pre(cmd, "sh"))
	}
	taglineFooter(&b, "package:"+strconv.FormatInt(s.Project.ID, 10)+":"+s.URL)
	return Message{HTML: b.Truncate(o.limit(), s.URL)}
}

// packageNoun names what was published, with its article.
func packageNoun(typ string) string {
	switch typ {
	case event.PackageContainer, event.PackageDocker:
		return "a Docker image"
	case event.PackageNPM:
		return "an npm package"
	case event.PackageMaven:
		return "a Maven package"
	case event.PackageRubyGems:
		return "a Ruby gem"
	case event.PackageNuGet:
		return "a NuGet package"
	}
	return "a " + typ + " package"
}

// packageRef is how the package's ecosystem names it: the image reference,
// the scoped npm name, the bare name otherwise.
func packageRef(s *cards.PackageState) string {
	switch {
	case s.Image != "":
		return s.Image
	case s.Type == event.PackageNPM:
		return "@" + strings.ToLower(s.Namespace) + "/" + s.Name
	}
	return s.Name
}

// installCommand fetches the first published tag or the version; empty for
// types without a one-line install (Maven).
func installCommand(s *cards.PackageState) string {
	v := s.Versions[0]
	switch s.Type {
	case event.PackageContainer, event.PackageDocker:
		return "docker pull " + s.Image + ":" + v
	case event.PackageNPM:
		return "npm install " + packageRef(s) + "@" + v
	case event.PackageRubyGems:
		return "gem install " + s.Name + " -v " + v
	case event.PackageNuGet:
		return "dotnet add package " + s.Name + " --version " + v
	}
	return ""
}
