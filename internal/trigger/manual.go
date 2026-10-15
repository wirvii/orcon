// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package trigger

import (
	"context"
	"fmt"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// ManualSource says which content a manual run executes.
type ManualSource string

// Sources of a manual run.
const (
	// ManualDraft runs a snapshot of the draft (ADR-0007 section 6).
	ManualDraft ManualSource = "DRAFT"
	// ManualPublished runs the live published version.
	ManualPublished ManualSource = "PUBLISHED"
)

// ManualCommand asks for a manual run of a flow.
type ManualCommand struct {
	Org    orgscope.Org
	Flow   flow.FlowID
	Source ManualSource
	// Node is the start event to enter through. It may be empty when the flow
	// has exactly one start event.
	Node flow.NodeID
}

// PrepareManualRun resolves what a manual run executes: for the draft, a
// snapshot of it, created now and shared by content (the draft must pass
// validation); for the published version, the live version. The run is in
// mode MANUAL, with no deduplication and no cap (R-INT-01), and works whether
// or not the flow is active.
func (s *Service) PrepareManualRun(ctx context.Context, cmd ManualCommand) (ManualRun, error) {
	rec, err := s.flows.GetFlow(ctx, cmd.Org, cmd.Flow)
	if err != nil {
		return ManualRun{}, fmt.Errorf("prepare a manual run of flow %s: %w", cmd.Flow, err)
	}
	run := ManualRun{Org: cmd.Org, FlowID: cmd.Flow, Mode: ModeManual}
	var content jsonvalue.Value
	switch cmd.Source {
	case ManualDraft:
		if content, err = jsonvalue.Parse(rec.Draft); err != nil {
			return ManualRun{}, fmt.Errorf("prepare a manual run of flow %s: the stored draft is not JSON: %w", cmd.Flow, err)
		}
	case ManualPublished:
		if rec.ActiveVersionID.IsZero() {
			return ManualRun{}, fmt.Errorf("prepare a manual run of flow %s: %w", cmd.Flow, ErrNotPublished)
		}
		v, verr := s.flows.GetVersion(ctx, cmd.Org, rec.ActiveVersionID)
		if verr != nil {
			return ManualRun{}, fmt.Errorf("prepare a manual run of flow %s: %w", cmd.Flow, verr)
		}
		if content, err = jsonvalue.Parse(v.Document); err != nil {
			return ManualRun{}, fmt.Errorf("prepare a manual run of flow %s: the stored version is not JSON: %w", cmd.Flow, err)
		}
		run.Version = v.ID
		run.Revision = flow.VersionRevision(int(v.Number))
	default:
		return ManualRun{}, fmt.Errorf("prepare a manual run of flow %s: unknown source %q", cmd.Flow, cmd.Source)
	}
	doc, err := flow.Read(content)
	if err != nil {
		return ManualRun{}, fmt.Errorf("prepare a manual run of flow %s: %w", cmd.Flow, err)
	}
	if run.Node, err = startNode(&doc, cmd.Node); err != nil {
		return ManualRun{}, fmt.Errorf("prepare a manual run of flow %s: %w", cmd.Flow, err)
	}
	if cmd.Source == ManualDraft {
		if err := s.snapshotDraft(ctx, &doc, cmd.Org, cmd.Flow, &run); err != nil {
			return ManualRun{}, fmt.Errorf("prepare a manual run of flow %s: %w", cmd.Flow, err)
		}
	}
	return run, nil
}

// snapshotDraft validates the draft and freezes it as a snapshot.
func (s *Service) snapshotDraft(ctx context.Context, doc *flow.Document, org orgscope.Org, flowID flow.FlowID, run *ManualRun) error {
	var report flow.Report
	if s.nodeTypes != nil {
		report = flow.ValidateWithTypes(ctx, doc, s.nodeTypes)
	} else {
		report = flow.Validate(doc)
	}
	if err := report.Err(); err != nil {
		return err
	}
	normalized := doc.Value()
	hash, err := flow.ContentHashOfValue(normalized)
	if err != nil {
		return err
	}
	raw, err := jsonvalue.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("encode the snapshot: %w", err)
	}
	rec, err := s.flows.CreateSnapshot(ctx, org, flow.SnapshotRecord{
		ID: flow.FlowSnapshotID(s.ids.New()), OrgID: identity.OrgID(org.UUID()), FlowID: flowID, ContentHash: hash.String(),
		Document: raw, CreatedAt: s.clock.Now().UTC(),
	})
	if err != nil {
		return err
	}
	run.Snapshot = rec.ID
	run.Revision = flow.SnapshotRevision(hash)
	return nil
}

// startNode picks the start event a manual run enters through.
func startNode(doc *flow.Document, want flow.NodeID) (flow.NodeID, error) {
	var starts []flow.NodeID
	for i := range doc.Nodes {
		if doc.Nodes[i].Element == flow.ElementStartEvent {
			starts = append(starts, doc.Nodes[i].ID)
		}
	}
	if want != "" {
		for _, id := range starts {
			if id == want {
				return id, nil
			}
		}
		return "", fmt.Errorf("node %q is not a start event: %w", want, ErrNoStartNode)
	}
	if len(starts) != 1 {
		return "", fmt.Errorf("the flow has %d start events and no node was chosen: %w", len(starts), ErrNoStartNode)
	}
	return starts[0], nil
}

// RunManually prepares a manual run and starts it with the given input through
// the [Starter] of the service, returning the identity of the execution.
func (s *Service) RunManually(ctx context.Context, cmd ManualCommand, input []byte) (string, error) {
	if s.starter == nil {
		return "", &ConfigError{Field: "Starter"}
	}
	run, err := s.PrepareManualRun(ctx, cmd)
	if err != nil {
		return "", err
	}
	executionID, err := s.starter.StartManual(ctx, run, input)
	if err != nil {
		return "", fmt.Errorf("start a manual run of flow %s: %w", cmd.Flow, err)
	}
	return executionID, nil
}
