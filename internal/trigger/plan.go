// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package trigger

import (
	"fmt"
	"slices"
	"strings"

	"github.com/wirvii/orcon/internal/flow"
)

// Spec is the desired registry entry of one trigger node of a version: what
// publication derives from the document and compares with the stored entry.
type Spec struct {
	NodeID flow.NodeID
	Family flow.TriggerFamily
	Type   string
	// ConfigurationHash is the hash of the trigger block, node type and
	// parameters (ADR-0020 section 2).
	ConfigurationHash string
	// Addresses are the production addresses of the node.
	Addresses []AddressSpec
}

// AddressSpec is the desired production address of a trigger.
type AddressSpec struct {
	Family          flow.AddressFamily
	Pattern         string
	StaticPrefix    string
	RandomSuffix    string
	ResponseProfile flow.ResponseProfile
	// Methods are sorted and without repeats; never empty.
	Methods []string
}

// Prefixes of the production paths (ADR-0022 section 1).
const (
	n8nPrefix    = "/webhook/"
	nativePrefix = "/hooks/"
)

// BuildSpecs derives the registry entries of a document, one per node that
// carries a trigger block, in the order of the nodes. The project segment of
// native addresses comes from projectSlug.
func BuildSpecs(doc *flow.Document, projectSlug string) ([]Spec, error) {
	var specs []Spec
	for i := range doc.Nodes {
		n := &doc.Nodes[i]
		if n.Trigger == nil {
			continue
		}
		hash, err := n.TriggerConfigurationHash()
		if err != nil {
			return nil, fmt.Errorf("build trigger spec of node %q: %w", n.ID, err)
		}
		spec := Spec{NodeID: n.ID, Family: n.Trigger.Family, Type: n.Type, ConfigurationHash: hash}
		for _, a := range n.Trigger.Addresses {
			spec.Addresses = append(spec.Addresses, addressSpec(a, projectSlug))
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

func addressSpec(a flow.TriggerAddress, projectSlug string) AddressSpec {
	pattern := ProductionPattern(a, projectSlug)
	profile := a.ResponseProfile
	if profile == "" {
		profile = flow.ResponseProfile(a.Family)
	}
	methods := a.Methods
	if len(methods) == 0 {
		methods = []string{"POST"}
	}
	methods = slices.Clone(methods)
	slices.Sort(methods)
	return AddressSpec{
		Family:          a.Family,
		Pattern:         pattern,
		StaticPrefix:    StaticPrefix(pattern),
		RandomSuffix:    a.Suffix,
		ResponseProfile: profile,
		Methods:         slices.Compact(methods),
	}
}

// ProductionPattern returns the production path of an address (ADR-0022
// section 1). An n8n address with route parameters and a webhook id uses the
// form with the webhook id that n8n shows; a native address carries the
// project and the optional random suffix.
func ProductionPattern(a flow.TriggerAddress, projectSlug string) string {
	if a.Family == flow.AddressN8N {
		if a.WebhookID != "" && hasParameter(a.Path) {
			return n8nPrefix + a.WebhookID + "/" + a.Path
		}
		return n8nPrefix + a.Path
	}
	p := nativePrefix + projectSlug + "/" + a.Path
	if a.Suffix != "" {
		p += "/" + a.Suffix
	}
	return p
}

func hasParameter(path string) bool {
	for _, s := range strings.Split(path, "/") {
		if strings.HasPrefix(s, ":") {
			return true
		}
	}
	return false
}

// StaticPrefix returns the part of a pattern before its first parameter
// segment, without a trailing slash. A pattern without parameters is its own
// prefix.
func StaticPrefix(pattern string) string {
	segments := strings.Split(pattern, "/")
	for i, s := range segments {
		if strings.HasPrefix(s, ":") {
			return strings.Join(segments[:i], "/")
		}
	}
	return pattern
}

// Overlaps reports whether two different patterns can match the same path: the
// same number of segments where every pair is equal or has a parameter on one
// side (ADR-0022 section 3, a warning). Equal patterns do not overlap; they
// collide.
func Overlaps(a, b string) bool {
	if a == b {
		return false
	}
	sa, sb := strings.Split(a, "/"), strings.Split(b, "/")
	if len(sa) != len(sb) {
		return false
	}
	for i := range sa {
		if sa[i] == sb[i] || strings.HasPrefix(sa[i], ":") || strings.HasPrefix(sb[i], ":") {
			continue
		}
		return false
	}
	return true
}

// SpecsByNode indexes specs by node id.
func SpecsByNode(specs []Spec) map[flow.NodeID]Spec {
	out := make(map[flow.NodeID]Spec, len(specs))
	for _, s := range specs {
		out[s.NodeID] = s
	}
	return out
}
