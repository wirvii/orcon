// Package config holds process-level configuration types shared by the
// composition root. It is a platform package: it imports no area or adapter.
package config

// Role selects which part of Orcon a process runs.
type Role string

// The roles a process can run. A new role needs an Accepted ADR.
const (
	// RoleStandalone runs every component in one process.
	RoleStandalone Role = "standalone"
	// RoleAPI serves the HTTP API and the web UI.
	RoleAPI Role = "api"
	// RoleScheduler runs trigger scheduling.
	RoleScheduler Role = "scheduler"
	// RoleWorker executes flow steps.
	RoleWorker Role = "worker"
)

// Valid reports whether r is one of the known roles.
func (r Role) Valid() bool {
	switch r {
	case RoleStandalone, RoleAPI, RoleScheduler, RoleWorker:
		return true
	default:
		return false
	}
}
