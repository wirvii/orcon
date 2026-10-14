// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package trigger is the business area of triggers: what starts an execution
// of a flow, and the lifecycle that turns a flow on and off (ADR-0020,
// ADR-0024). It holds domain rules and the ports they consume; the HTTP
// ingress, the storage and the connector code live in adapters.
//
// # Registry and lifecycle
//
// Publishing a flow validates its draft, creates the next immutable version
// and rebuilds the registry of its triggers and web addresses in one
// transaction ([Service.Publish]). Activating and deactivating switch every
// automatic trigger of the flow on and off without creating a version
// ([Service.Activate], [Service.Deactivate]); rolling back republishes an
// earlier version as the next one ([Service.Rollback]); deleting a flow
// switches its triggers off in the same transaction as the soft delete
// ([Service.Delete]). A trigger keeps its row, its addresses and its state
// across versions as long as its configuration hash is unchanged (ADR-0024
// section 3).
//
// Publication adds the trigger rules T-V1..T-V9 (ADR-0024 section 2) to the
// validation of the flow area; [Service.Validate] runs them without
// publishing.
//
// # Acceptance
//
// [Service.Accept] is the read the ingress acceptance transaction of later
// cuts repeats inside its own transaction: it names the version an event is
// pinned to, or refuses it because the flow or the trigger is inactive,
// deleted or unknown.
//
// # Manual runs
//
// [Service.PrepareManualRun] resolves the revision a manual run executes: a
// snapshot of the draft or the published version, in mode MANUAL, with no
// deduplication and no cap (ADR-0020 section 8). Starting the execution is
// the engine's port; the durable engine arrives with the execution storage.
package trigger
