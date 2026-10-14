// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package connector

import (
	"fmt"
	"slices"

	"github.com/wirvii/orcon/internal/flow"
)

// checkManifest applies the rules that need the whole manifest: unique names,
// references between nodes, operations, credentials and permissions, and the
// safety contract of every effect operation. It runs after the shape is known
// to be right.
func (r *reader) checkManifest(m *Manifest) {
	r.checkKind(m)
	r.checkUnique(m)
	for i := range m.Nodes {
		r.checkNode(m, i)
	}
	r.checkCredentials(m)
	r.checkPermissions(m)
	r.checkChangelog(m)
}

func (r *reader) checkKind(m *Manifest) {
	switch m.Kind {
	case KindDeclarative, KindNative, KindWASM:
		if len(m.Nodes) == 0 {
			r.failf(RuleShape, "/nodes", "a %s connector declares at least one node", m.Kind)
		}
	case KindLibrary, KindRuntime:
		if len(m.Nodes) > 0 {
			r.failf(RuleShape, "/nodes", "a %s package adds no nodes", m.Kind)
		}
	}
}

// duplicates reports every repeated value of a list at the path of the later one.
func duplicates[T comparable](r *reader, items []T, path func(i int) string, what string, name func(T) string) {
	seen := map[T]bool{}
	for i, item := range items {
		if seen[item] {
			r.failf(RuleValue, path(i), "%s %q is declared twice", what, name(item))
		}
		seen[item] = true
	}
}

func (r *reader) checkUnique(m *Manifest) {
	names := make([]string, len(m.Nodes))
	types := make([]string, len(m.Nodes))
	for i := range m.Nodes {
		names[i], types[i] = m.Nodes[i].Name, m.Nodes[i].Type
	}
	ident := func(s string) string { return s }
	duplicates(r, names, func(i int) string { return fmt.Sprintf("/nodes/%d/name", i) }, "node name", ident)
	duplicates(r, types, func(i int) string { return fmt.Sprintf("/nodes/%d/type", i) }, "node type", ident)
	creds := make([]string, len(m.Credentials))
	for i := range m.Credentials {
		creds[i] = m.Credentials[i].ID
	}
	duplicates(r, creds, func(i int) string { return fmt.Sprintf("/credentials/%d/id", i) }, "credential id", ident)
	limits := make([]string, len(m.RateLimits))
	for i := range m.RateLimits {
		limits[i] = m.RateLimits[i].Name
	}
	duplicates(r, limits, func(i int) string { return fmt.Sprintf("/rateLimits/%d/name", i) }, "rate limit", ident)
}

func (r *reader) checkNode(m *Manifest, index int) {
	n := &m.Nodes[index]
	ops := make([]string, len(n.Operations))
	for i := range n.Operations {
		ops[i] = n.Operations[i].Name
	}
	base := fmt.Sprintf("/nodes/%d", index)
	duplicates(r, ops, func(i int) string { return fmt.Sprintf("%s/operations/%d/name", base, i) }, "operation", func(s string) string { return s })
	for i := range n.Operations {
		r.checkOperation(m, n, i, fmt.Sprintf("%s/operations/%d", base, i))
	}
}

func (r *reader) checkOperation(m *Manifest, n *Node, index int, path string) {
	op := &n.Operations[index]
	if op.RateLimit != "" && !slices.ContainsFunc(m.RateLimits, func(l RateLimit) bool { return l.Name == op.RateLimit }) {
		r.failf(RuleReference, path+"/rateLimit", "operation %q names rate limit %q, which the manifest does not declare", op.Name, op.RateLimit)
	}
	if op.Safety == nil {
		return
	}
	r.checkSafetyReferences(n, op, path+"/safety")
	for _, problem := range flow.CheckContract(n.Contract(op)) {
		r.failf(RuleSafety, path+"/safety", "operation %q: %s", op.Name, problem)
	}
}

// checkSafetyReferences resolves the operations a safety contract names within
// its node. A missing one is reported here, with its path, so the generic
// contract rules do not have to guess.
func (r *reader) checkSafetyReferences(n *Node, op *Operation, path string) {
	s := op.Safety
	if s.Verification != nil {
		if _, ok := n.OperationByName(s.Verification.Operation); !ok {
			r.failf(RuleReference, path+"/verification/operation", "operation %q verifies with %q, which is not an operation of node %q", op.Name, s.Verification.Operation, n.Name)
		}
	}
	if c := s.Compensation; c != nil {
		if _, ok := n.OperationByName(c.Operation); !ok {
			r.failf(RuleReference, path+"/compensation/operation", "operation %q compensates with %q, which is not an operation of node %q", op.Name, c.Operation, n.Name)
		}
		if c.Verify != "" {
			if _, ok := n.OperationByName(c.Verify); !ok {
				r.failf(RuleReference, path+"/compensation/verify", "operation %q verifies its compensation with %q, which is not an operation of node %q", op.Name, c.Verify, n.Name)
			}
		}
	}
}

func (r *reader) checkCredentials(m *Manifest) {
	hasOperation := func(name string) bool {
		for i := range m.Nodes {
			if _, ok := m.Nodes[i].OperationByName(name); ok {
				return true
			}
		}
		return false
	}
	for i := range m.Credentials {
		c := &m.Credentials[i]
		base := fmt.Sprintf("/credentials/%d", i)
		if c.Test != "" && !hasOperation(c.Test) {
			r.failf(RuleReference, base+"/test", "credential %q is tested with %q, which is not an operation of any node", c.ID, c.Test)
		}
		if c.Scheme.Type == SchemeSessionToken && !hasOperation(c.Scheme.Login) {
			r.failf(RuleReference, base+"/scheme/login", "credential %q logs in with %q, which is not an operation of any node", c.ID, c.Scheme.Login)
		}
		fields := make([]string, len(c.Fields))
		for j := range c.Fields {
			fields[j] = c.Fields[j].Name
		}
		duplicates(r, fields, func(j int) string { return fmt.Sprintf("%s/fields/%d/name", base, j) }, "credential field", func(s string) string { return s })
	}
}

func (r *reader) checkPermissions(m *Manifest) {
	p := &m.Permissions
	for i, id := range p.Credentials {
		if !slices.ContainsFunc(m.Credentials, func(c Credential) bool { return c.ID == id }) {
			r.failf(RuleReference, fmt.Sprintf("/permissions/credentials/%d", i), "permission names credential %q, which the manifest does not declare", id)
		}
	}
	for i, field := range p.Network.FromCredential {
		if !credentialFieldExists(m, p.Credentials, field) {
			r.failf(RuleReference, fmt.Sprintf("/permissions/network/fromCredential/%d", i), "host field %q is not a field of a credential listed in permissions.credentials", field)
		}
	}
	if len(p.Network.FromCredential) > 0 && len(p.Credentials) == 0 {
		r.failf(RulePermission, "/permissions/network/fromCredential", "a host taken from a credential needs the credential in permissions.credentials")
	}
}

func credentialFieldExists(m *Manifest, listed []string, field string) bool {
	for i := range m.Credentials {
		c := &m.Credentials[i]
		if !slices.Contains(listed, c.ID) {
			continue
		}
		if slices.ContainsFunc(c.Fields, func(f CredentialField) bool { return f.Name == field }) {
			return true
		}
	}
	return false
}

func (r *reader) checkChangelog(m *Manifest) {
	versions := make([]string, len(m.Changelog))
	for i := range m.Changelog {
		versions[i] = m.Changelog[i].Version
	}
	duplicates(r, versions, func(i int) string { return fmt.Sprintf("/changelog/%d/version", i) }, "changelog version", func(s string) string { return s })
	if !slices.Contains(versions, m.Version) {
		r.failf(RuleReference, "/changelog", "the changelog has no entry for version %s; every released version has one (MF-9)", m.Version)
	}
	for i := range m.Changelog {
		if len(m.Changelog[i].Notes) == 0 {
			r.failf(RuleShape, fmt.Sprintf("/changelog/%d/notes", i), "an entry lists at least one message key")
		}
	}
}
