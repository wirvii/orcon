// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

func TestDraftSaveRefusesAnOversizedDocument(t *testing.T) {
	// The store is nil: a document over the cap is refused before anything is read or written.
	drafts := flow.NewDrafts(nil, nil, nil)
	doc := []byte(`{"pad":"` + strings.Repeat("x", flow.MaxDraftBytes) + `"}`)
	_, err := drafts.Save(t.Context(), orgscope.Org{}, flow.SaveInput{Document: doc})
	if !errors.Is(err, identity.ErrInvalidInput) {
		t.Fatalf("Save of an oversized document = %v, want ErrInvalidInput", err)
	}
}
