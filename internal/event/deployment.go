package event

import "time"

// Deployment statuses.
const (
	DeploymentRunning  = "running"
	DeploymentSuccess  = "success"
	DeploymentFailed   = "failed"
	DeploymentCanceled = "canceled"
	DeploymentBlocked  = "blocked"
)

// Deployment is a Deployment Hook.
type Deployment struct {
	Meta
	Project Project
	// User is the deployer.
	User User

	// ID is deployment_id.
	ID int64
	// Status is one of the Deployment* constants.
	Status          string
	StatusChangedAt time.Time
	// DeployableID and DeployableURL point at the job that performed the
	// deployment.
	DeployableID  int64
	DeployableURL string
	// Environment is the environment name; EnvironmentURL is its external
	// URL when configured; EnvironmentTier is production, staging, ...
	Environment     string
	EnvironmentURL  string
	EnvironmentTier string
	// Ref is the branch or tag name (not a full ref).
	Ref    string
	Commit Commit
}

// EventKind implements Event.
func (*Deployment) EventKind() Kind { return KindDeployment }

// Proj implements Event.
func (d *Deployment) Proj() Project { return d.Project }

// Actor implements Event.
func (d *Deployment) Actor() User { return d.User }
