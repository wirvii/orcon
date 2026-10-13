// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

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

// Bounds of the number of abandoned expression evaluations a process allows
// to keep running (ADR-0018 section 3, SPEC-001 amendment 10). An evaluation
// that outlives its deadline is abandoned and finishes in the background; the
// memory they can hold is this number times the memory cap of a runtime.
const (
	// SandboxMaxAbandonedDefault is the default limit.
	SandboxMaxAbandonedDefault = 8
	// SandboxMaxAbandonedMin and SandboxMaxAbandonedMax bound the setting.
	SandboxMaxAbandonedMin = 1
	SandboxMaxAbandonedMax = 64
)

// ValidSandboxMaxAbandoned reports whether n is an allowed limit of abandoned
// evaluations still running.
func ValidSandboxMaxAbandoned(n int) bool {
	return n >= SandboxMaxAbandonedMin && n <= SandboxMaxAbandonedMax
}

// Valid reports whether r is one of the known roles.
func (r Role) Valid() bool {
	switch r {
	case RoleStandalone, RoleAPI, RoleScheduler, RoleWorker:
		return true
	default:
		return false
	}
}
