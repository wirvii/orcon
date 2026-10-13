// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package trigger

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func sortedKeys[V any](m map[string]V) []string { return slices.Sorted(maps.Keys(m)) }

// Activate switches every automatic trigger of the flow on, on the live
// version, without creating a version (R-ACT-02). A flow that is already
// active is left as it is. A flow with no published version fails with
// [ErrNotPublished].
func (s *Service) Activate(ctx context.Context, org identity.OrgID, flowID flow.FlowID) error {
	return s.setActivation(ctx, org, flowID, flow.ActivationActive)
}

// Deactivate switches every automatic trigger of the flow off: addresses
// answer as inactive and no automatic trigger accepts events. Executions in
// progress go on (ADR-0024 section 5). A flow that is already inactive is left
// as it is.
func (s *Service) Deactivate(ctx context.Context, org identity.OrgID, flowID flow.FlowID) error {
	return s.setActivation(ctx, org, flowID, flow.ActivationInactive)
}

func (s *Service) setActivation(ctx context.Context, org identity.OrgID, flowID flow.FlowID, target flow.ActivationState) error {
	err := s.store.Atomic(ctx, org, func(ctx context.Context, tx Tx) error {
		state, err := tx.LockFlowState(ctx, flowID)
		if err != nil {
			return err
		}
		if state.IsDeleted() {
			return fmt.Errorf("flow %s: %w", flowID, flow.ErrNotFound)
		}
		if state.ActivationState == target {
			return nil
		}
		if target == flow.ActivationActive && state.ActiveVersionID.IsZero() {
			return fmt.Errorf("flow %s: %w", flowID, ErrNotPublished)
		}
		now := s.clock.Now().UTC()
		if err := tx.SetActivationState(ctx, flowID, target, now); err != nil {
			return err
		}
		status, event := StatusInactive, EventFlowDeactivated
		if target == flow.ActivationActive {
			status, event = StatusActive, EventFlowActivated
		}
		if err := tx.SetTriggerStatuses(ctx, flowID, status, now); err != nil {
			return err
		}
		if err := s.appendEvent(ctx, tx, org, flowID, "", event, now, map[string]jsonvalue.Value{
			"flowId": jsonvalue.String(flowID.String()),
		}); err != nil {
			return err
		}
		return tx.NotifyRoutesChanged(ctx, flowID)
	})
	if err != nil {
		return fmt.Errorf("set activation of flow %s to %s: %w", flowID, target, err)
	}
	return nil
}

// Delete soft-deletes the flow. In the same transaction its automatic
// triggers are switched off exactly as by a deactivation: addresses answer as
// inactive, and new events are refused with the reason INACTIVE. Executions in
// progress go on with their version (R-ACT-09, ADR-0024 section 8).
func (s *Service) Delete(ctx context.Context, org identity.OrgID, flowID flow.FlowID, by identity.PrincipalID) error {
	err := s.store.Atomic(ctx, org, func(ctx context.Context, tx Tx) error {
		state, err := tx.LockFlowState(ctx, flowID)
		if err != nil {
			return err
		}
		if state.IsDeleted() {
			return fmt.Errorf("flow %s: %w", flowID, flow.ErrNotFound)
		}
		now := s.clock.Now().UTC()
		if err := tx.SoftDeleteFlow(ctx, flowID, by, now); err != nil {
			return err
		}
		if err := tx.SetActivationState(ctx, flowID, flow.ActivationInactive, now); err != nil {
			return err
		}
		if err := tx.SetTriggerStatuses(ctx, flowID, StatusInactive, now); err != nil {
			return err
		}
		if state.ActivationState == flow.ActivationActive {
			if err := s.appendEvent(ctx, tx, org, flowID, "", EventFlowDeactivated, now, map[string]jsonvalue.Value{
				"flowId": jsonvalue.String(flowID.String()),
				"reason": jsonvalue.String("FLOW_DELETED"),
			}); err != nil {
				return err
			}
		}
		return tx.NotifyRoutesChanged(ctx, flowID)
	})
	if err != nil {
		return fmt.Errorf("delete flow %s: %w", flowID, err)
	}
	return nil
}

// Restore brings back a flow deleted less than 30 days ago. The flow comes
// back inactive and so do its triggers: a person activates it explicitly
// (R-ACT-09). A name already taken in the project fails with flow.ErrConflict
// and nothing changes.
func (s *Service) Restore(ctx context.Context, org identity.OrgID, flowID flow.FlowID) error {
	err := s.store.Atomic(ctx, org, func(ctx context.Context, tx Tx) error {
		state, err := tx.LockFlowState(ctx, flowID)
		if err != nil {
			return err
		}
		if !state.IsDeleted() {
			return fmt.Errorf("flow %s: %w", flowID, ErrNotDeleted)
		}
		now := s.clock.Now().UTC()
		if now.Sub(state.DeletedAt) > RecoveryWindow {
			return fmt.Errorf("flow %s was deleted at %s: %w", flowID, state.DeletedAt.Format(time.RFC3339), ErrRecoveryExpired)
		}
		return tx.RestoreFlow(ctx, flowID, now)
	})
	if err != nil {
		return fmt.Errorf("restore flow %s: %w", flowID, err)
	}
	return nil
}

// Purge deletes for good a flow whose recovery window has passed, with its
// versions, snapshots and trigger rows. A flow inside the window fails with
// [ErrRecoveryWindow]. Retention calls it (ADR-0077 section 4).
func (s *Service) Purge(ctx context.Context, org identity.OrgID, flowID flow.FlowID) error {
	err := s.store.Atomic(ctx, org, func(ctx context.Context, tx Tx) error {
		state, err := tx.LockFlowState(ctx, flowID)
		if err != nil {
			return err
		}
		if !state.IsDeleted() {
			return fmt.Errorf("flow %s: %w", flowID, ErrNotDeleted)
		}
		if s.clock.Now().UTC().Sub(state.DeletedAt) <= RecoveryWindow {
			return fmt.Errorf("flow %s deleted at %s: %w", flowID, state.DeletedAt.Format(time.RFC3339), ErrRecoveryWindow)
		}
		if err := tx.PurgeFlow(ctx, flowID); err != nil {
			return err
		}
		return tx.NotifyRoutesChanged(ctx, flowID)
	})
	if err != nil {
		return fmt.Errorf("purge flow %s: %w", flowID, err)
	}
	return nil
}

// FlowStatus is the lifecycle state of a flow as the editor shows it.
type FlowStatus struct {
	// ActivationState says whether the automatic triggers accept events.
	ActivationState flow.ActivationState
	// ActiveVersion is the number of the live version; zero while nothing is
	// published.
	ActiveVersion int64
	// HasUnpublishedChanges is true when the content hash of the draft differs
	// from the content hash of the live version (R-ACT-01), or when nothing is
	// published yet.
	HasUnpublishedChanges bool
	// Triggers is the registry of the flow, removed entries included.
	Triggers []Trigger
}

// Status reports the activation state, the live version, whether the draft
// has unpublished changes, and the triggers of the flow.
func (s *Service) Status(ctx context.Context, org identity.OrgID, flowID flow.FlowID) (FlowStatus, error) {
	rec, err := s.flows.GetFlow(ctx, org, flowID)
	if err != nil {
		return FlowStatus{}, fmt.Errorf("status of flow %s: %w", flowID, err)
	}
	out := FlowStatus{ActivationState: rec.ActivationState, HasUnpublishedChanges: true}
	if out.ActivationState == "" {
		out.ActivationState = flow.ActivationInactive
	}
	if !rec.ActiveVersionID.IsZero() {
		v, err := s.flows.GetVersion(ctx, org, rec.ActiveVersionID)
		if err != nil {
			return FlowStatus{}, fmt.Errorf("status of flow %s: read the live version: %w", flowID, err)
		}
		out.ActiveVersion = v.Number
		out.HasUnpublishedChanges = draftDiffers(rec.Draft, v.ContentHash)
	}
	err = s.store.Read(ctx, org, func(ctx context.Context, tx Tx) error {
		var rerr error
		out.Triggers, rerr = tx.ListTriggers(ctx, flowID)
		return rerr
	})
	if err != nil {
		return FlowStatus{}, fmt.Errorf("status of flow %s: %w", flowID, err)
	}
	return out, nil
}

// draftDiffers reports whether the draft, normalized the way publication
// normalizes it, has a different content hash than the live version. A draft
// that does not read as a document differs by definition.
func draftDiffers(draft []byte, liveHash string) bool {
	value, err := jsonvalue.Parse(draft)
	if err != nil {
		return true
	}
	doc, err := flow.Read(value)
	if err != nil {
		return true
	}
	h, err := flow.ContentHash(&doc)
	if err != nil {
		return true
	}
	return h.String() != liveHash
}

// Accept reads what an ingress acceptance needs about the trigger of a node:
// the version the event is pinned to. It returns the [Acceptance] and, when
// the flow is inactive or deleted or the node left the live version, the
// error [ErrInactive] together with it, so a caller can report the reason
// INACTIVE (R-OBS-01). A flow or node that does not exist fails with
// flow.ErrNotFound.
//
// The ingress acceptance transaction of cut T2 repeats this read inside its own
// transaction through [Tx.ResolveAcceptance]; calling it here is for callers
// that only need the answer.
func (s *Service) Accept(ctx context.Context, org identity.OrgID, flowID flow.FlowID, node flow.NodeID) (Acceptance, error) {
	var acc Acceptance
	err := s.store.Read(ctx, org, func(ctx context.Context, tx Tx) error {
		var rerr error
		acc, rerr = tx.ResolveAcceptance(ctx, flowID, node)
		return rerr
	})
	if err != nil {
		return Acceptance{}, fmt.Errorf("accept an event for node %q of flow %s: %w", node, flowID, err)
	}
	if !acc.Accepting {
		return acc, fmt.Errorf("accept an event for node %q of flow %s: %w", node, flowID, ErrInactive)
	}
	return acc, nil
}
