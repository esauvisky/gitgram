package event

// Package types (registry_package.package_type, lowercased). Docker is the
// legacy docker.pkg.github.com registry; GHCR images are Container.
const (
	PackageContainer = "container"
	PackageDocker    = "docker"
	PackageNPM       = "npm"
	PackageMaven     = "maven"
	PackageRubyGems  = "rubygems"
	PackageNuGet     = "nuget"
)

// Package is one version published to (or, for images, retagged in) a
// package registry: a GitHub registry_package delivery.
type Package struct {
	Meta
	Project Project
	// User is who published the version.
	User User

	// Type is one of the Package* constants, or whatever new type GitHub
	// sends.
	Type string
	// Name is the package name, e.g. "gitgram"; Namespace is its owner.
	Name      string
	Namespace string
	// VersionID identifies the version (one image digest) in the registry.
	VersionID int64
	// Version is the tag this delivery put on an image, or the version of
	// any other package.
	Version string
	// Image is the image reference without tag, e.g.
	// "ghcr.io/esauvisky/gitgram"; empty for other types.
	Image string
	// URL is the version's page.
	URL string
}

// IsImage reports whether the package is a container image.
func (p *Package) IsImage() bool { return p.Type == PackageContainer || p.Type == PackageDocker }

// EventKind implements Event.
func (*Package) EventKind() Kind { return KindPackage }

// Proj implements Event.
func (p *Package) Proj() Project { return p.Project }

// Actor implements Event.
func (p *Package) Actor() User { return p.User }
