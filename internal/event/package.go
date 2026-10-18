package event

// Package types: GitHub's registry_package.package_type lowercased and
// GitLab's package_type, which share their names. Docker is GitHub's legacy
// docker.pkg.github.com registry; GHCR and GitLab images are Container.
const (
	PackageContainer = "container"
	PackageDocker    = "docker"
	PackageNPM       = "npm"
	PackageMaven     = "maven"
	PackageRubyGems  = "rubygems"
	PackageNuGet     = "nuget"
	PackagePyPI      = "pypi"
	PackageGo        = "golang"
	PackageHelm      = "helm"
	PackageTerraform = "terraform_module"
	PackageGeneric   = "generic"
	PackageConan     = "conan"
	PackageComposer  = "composer"
	PackageDebian    = "debian"
)

// Package is one version published to (or, for images, retagged in) a
// package registry: a GitHub registry_package delivery, or a version the
// GitLab registry watcher found.
type Package struct {
	Meta
	Project Project
	// User is who published the version.
	User User

	// Type is one of the Package* constants, or whatever new type GitHub
	// sends.
	Type string
	// Name is the package name as its ecosystem spells it, scope included
	// for npm ("@acme/ui").
	Name string
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
