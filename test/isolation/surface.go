// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package isolation

import (
	"reflect"
	"sort"
)

// StoreSurface is the surface of the store methods: every method of [Stores], the
// ports the adapters implement. A method added to a port without a case fails
// TestRegistryComplete; a port added to the adapters without a place in [Stores]
// fails TestSurfaceIsEverythingTheAdaptersImplement.
type StoreSurface struct{}

// Routes returns nothing: no HTTP route handles organization data yet. The first
// cut that adds one replaces this with the routes of the OpenAPI document.
func (StoreSurface) Routes() []string { return nil }

// StoreMethods returns the names of the methods of [Stores], sorted.
func (StoreSurface) StoreMethods() []string {
	t := reflect.TypeOf((*Stores)(nil)).Elem()
	names := make([]string, 0, t.NumMethod())
	for i := range t.NumMethod() {
		names = append(names, t.Method(i).Name)
	}
	sort.Strings(names)
	return names
}

// UncoveredStoreMethods are the methods that the SQLite and PostgreSQL stores implement
// and that have no isolation case yet. They belong to cuts that reached main before this
// battery: the execution journal and engine (SPEC-001 C2b and C4a), the connector
// packages and grants (SPEC-003 CN1), the trigger transactions (SPEC-002 T1), and the
// connector references of flows. Each still takes the organization as an identity
// (the trigger transactions take a scope), so none of them is checked by the
// orgscope analyzer. The list is a known gap, not an exemption: TestSurfaceIsEverythingTheAdaptersImplement
// fails when a method is added to the adapters or removed from them without this list changing, and a
// case for one of them moves it into [StoreSurface]. The cases are owed by BL-100.
var UncoveredStoreMethods = []string{
	"Atomic", "ClaimOutbox", "ClaimTokens", "FindGrant", "FireDueTimers",
	"GetExecution", "GetExternalOperation", "GetJoinInstance", "GetPackage", "GetPackageByID", "GetTimer",
	"GetToken", "InsertGrant", "InsertPackage", "ListAttentionRequests",
	"ListDraftConnectorReferences", "ListExecutionEvents", "ListExecutions", "ListExternalOperations",
	"ListPackageVersions", "ListPackages", "ListVersionConnectorReferences", "Read", "ReleaseExpiredLeases",
	"ReleaseWorkerLeases", "RenewLeases", "ResolveWaitSecret",
	// Added by the durable engine (SPEC-001 C4a: payloads, snapshots, lost leases, token
	// attempts and nondeterminism reads), by its last-chance tokens, and by the outbox
	// routing. They take the organization as well (ScopeOf, the scope itself); their cases
	// are owed by BL-100 like the rest of this list.
	"ClaimLastChanceToken", "GetPayload", "LastChanceWaiting", "ListAttempts", "ListTokenNondeterminism",
	"LoadSnapshot", "ReleaseLostLeases", "ReleaseTokenLease", "RouteEvents", "ScopeOf",
	// Added by the shared rate buckets of the connector dispatch (SPEC-003 CN3-dispatch, ADR-0014
	// section 2.4). They take the organization; their cases are owed by BL-100 as well.
	"BlockBucket", "TakeBucketPermits",
	// RunnableCounts is the instance-wide count the fair claim planner reads (SPEC-001 C11): it
	// takes no organization, so it is not an isolation subject, and the list keeps the surface exact.
	"RunnableCounts",
	// ResolveRoute maps an unauthenticated webhook path to the organization that owns it (SPEC-002
	// T2): it runs before any organization is known, so it has no organization parameter. Its
	// cases are owed by BL-100 with the rest of this list.
	"ResolveRoute",
	// Added by the read side of the executions API (the node runs and stored outputs of an execution,
	// and the audit entry of a read of data). They take the scope of the organization like the other
	// execution reads; storetest checks that another organization lists nothing (NodeReads,
	// OrgAuditAppend), and their cases here are owed by BL-100 with the rest of this list.
	"AppendOrgEntry", "ListNodeExecutions", "ListNodeOutputs",
	// The retention of editor data (SPEC-014 ED2, [editor.Retention]) removes rows of every organization
	// by design: it is the engine's cleanup, not a request of a person, so it has no organization case.
	"PurgeEditorPresence", "PurgeEditorTestData",
}

// NoSurface is the surface of a tree without routes or store methods that handle
// organization data. It is kept for tests that need an empty surface.
type NoSurface struct{}

// Routes returns nothing.
func (NoSurface) Routes() []string { return nil }

// StoreMethods returns nothing.
func (NoSurface) StoreMethods() []string { return nil }

// DefaultSurface is what TestRegistryComplete compares [BuildRegistry] with.
var DefaultSurface Surface = StoreSurface{}
