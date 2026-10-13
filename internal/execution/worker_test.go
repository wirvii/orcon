// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/platform/id"
)

func TestWorkerID(t *testing.T) {
	u := id.NewSequence().New()
	w := execution.WorkerID(u)
	if w.String() != u.String() || w.IsZero() || !(execution.WorkerID{}).IsZero() {
		t.Errorf("WorkerID %s", w)
	}
}
