// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"context"
	"time"

	"github.com/wirvii/orcon/internal/platform/id"
)

// WorkerID identifies a process of the engine in the worker registry. The
// storage adapters convert it to and from [id.UUID]; no other package does.
type WorkerID id.UUID

// String returns the lowercase hyphenated form.
func (i WorkerID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i WorkerID) IsZero() bool { return id.UUID(i).IsZero() }

// WorkerRecord is a row of worker: an informational registry of processes.
type WorkerRecord struct {
	ID          WorkerID
	Role        string
	Host        string
	Version     string
	StartedAt   time.Time
	HeartbeatAt time.Time
}

// Worker roles, the spelling of the --role flag in UPPER_SNAKE.
const (
	WorkerStandalone = "STANDALONE"
	WorkerAPI        = "API"
	WorkerScheduler  = "SCHEDULER"
	WorkerWorker     = "WORKER"
)

// WorkerStore persists the worker registry.
type WorkerStore interface {
	// RegisterWorker inserts the worker or refreshes its heartbeat and version.
	//
	//orcon:scope instance worker is an instance table, not owned by an organization
	RegisterWorker(ctx context.Context, w WorkerRecord) error
}
