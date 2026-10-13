// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/id"
)

func TestRowIdentities(t *testing.T) {
	u := id.NewSequence().New()
	text := u.String()
	if flow.FlowID(u).String() != text || flow.FlowVersionID(u).String() != text || flow.FlowSnapshotID(u).String() != text {
		t.Error("String must be the hyphenated form of the underlying UUID")
	}
	if flow.FlowID(u).IsZero() || flow.FlowVersionID(u).IsZero() || flow.FlowSnapshotID(u).IsZero() {
		t.Error("a set identity is not zero")
	}
	if !(flow.FlowID{}).IsZero() || !(flow.FlowVersionID{}).IsZero() || !(flow.FlowSnapshotID{}).IsZero() {
		t.Error("the zero identity is zero")
	}
}

func TestRecordIsDeleted(t *testing.T) {
	var r flow.Record
	if r.IsDeleted() {
		t.Error("a new record is not deleted")
	}
	r.DeletedAt = time.Unix(1, 0)
	if !r.IsDeleted() {
		t.Error("a record with deleted_at is deleted")
	}
}
