// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package envelope

import (
	"context"
	"errors"
	"fmt"

	"github.com/wirvii/orcon/internal/identity"
)

// ensureBatch is how many organizations without a key one query returns.
const ensureBatch = 500

// EnsureOrgKeys gives version 1 of the organization key to every organization that has none
// (SPEC-004 R-KEY-05, SE3b). An organization is created together with its key, so one without
// a key was created before that held; without it, its audit chain cannot be sealed and its
// credentials cannot be stored. The caller runs it after [Verify], so every key it writes is
// wrapped by the verified master key.
//
// The created callback, which may be nil, is called with each organization that received a key.
// A failure on one organization does not stop the others: the failures are returned joined.
// Callers that start together are safe: the second writer of the same key gets [ErrKeyExists]
// and the organization is counted as repaired by the first.
func (k *Keyring) EnsureOrgKeys(ctx context.Context, gaps identity.OrgKeyGapStore, created func(identity.OrgID)) error {
	seen := make(map[identity.OrgID]struct{})
	var failures []error
	for {
		orgs, err := gaps.ListOrgsWithoutActiveKey(ctx, ensureBatch)
		if err != nil {
			return errors.Join(append(failures, fmt.Errorf("list organizations without a key: %w", err))...)
		}
		fresh := 0
		for _, org := range orgs {
			if _, done := seen[org]; done {
				continue
			}
			seen[org] = struct{}{}
			fresh++
			_, err := k.CreateKey(ctx, OrgScope(org))
			switch {
			case err == nil:
				if created != nil {
					created(org)
				}
			case errors.Is(err, ErrKeyExists):
				// another process gave it a key first
			default:
				failures = append(failures, fmt.Errorf("organization %s: %w", org, err))
			}
		}
		if fresh == 0 {
			return errors.Join(failures...)
		}
	}
}
