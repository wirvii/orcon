// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package connector

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Manifest is an orcon/connector@1 manifest (ADR-0029 section 4): the one
// document that describes a connector of any kind to the editor, the importer,
// the catalog and the validator of publication.
//
// [Read] accepts only documents that follow the schema, so a Manifest that came
// out of Read is well formed. [Manifest.Value] returns the document it was read
// from, which is what is stored and hashed.
type Manifest struct {
	Format      string
	ID          string
	Version     string
	Kind        Kind
	DisplayName string
	Description string
	Publisher   Publisher
	// License is an SPDX license expression.
	License string
	// Source is nil when the manifest does not say where it was built from.
	Source      *Source
	Compat      Compat
	Nodes       []Node
	Credentials []Credential
	Permissions Permissions
	RateLimits  []RateLimit
	// N8n is reserved for the converter and kept opaque here.
	N8n jsonvalue.Value
	// AI is reserved for the AI design and kept opaque here.
	AI        jsonvalue.Value
	Changelog []ChangelogEntry

	doc jsonvalue.Value
}

// Publisher says who ships the package.
type Publisher struct {
	Name    string
	URL     string
	Contact string
}

// Source says where the package was built from.
type Source struct {
	Repository  string
	Commit      string
	BuildRecipe string
}

// Compat says which engine and runtime interface the connector was built for.
type Compat struct {
	// EngineAPI is a SemVer range of the engine API, such as ">=1.0.0 <2.0.0".
	EngineAPI string
	// WasmABI is the ABI version of a WASM connector; zero for other kinds.
	WasmABI int
}

// Node is a node type a connector adds to the editor.
type Node struct {
	// Name is the node name inside the connector, such as "order".
	Name string
	// Type is "<connector id>.<name>@<major>", the type flows use.
	Type        string
	DisplayName string
	Group       Group
	// Operations are the operations of an ACTION node.
	Operations []Operation
	// Trigger is the declaration of a TRIGGER node.
	Trigger *Trigger
	// Parameters is the parameter schema of the node, kept opaque here; the
	// declarative interpreter reads it (ADR-0030 section 3).
	Parameters jsonvalue.Value
	// Outputs are the named output ports; empty means the single "main" port.
	Outputs []string
	// SensitiveFields are JSON pointers into inputs and outputs that never
	// reach logs or history.
	SensitiveFields []string
	// N8nTypes are the n8n node types this node replaces.
	N8nTypes []N8nType
}

// N8nType maps an n8n node type and its type versions to a node.
type N8nType struct {
	Name         string
	TypeVersions []float64
	// ParameterMap is the conversion of parameters, kept opaque here.
	ParameterMap jsonvalue.Value
}

// Operation is one thing a node does.
type Operation struct {
	// Name is "<resource>.<verb>", such as "contact.create".
	Name        string
	DisplayName string
	Purity      flow.Purity
	ItemMode    ItemMode
	// Safety is the external-operation contract; required exactly when Purity
	// is EFFECT.
	Safety *Safety
	// RateLimit names an entry of the manifest's rateLimits.
	RateLimit string
	// Retry holds classification overrides, kept opaque here.
	Retry jsonvalue.Value
}

// Safety is the machine-readable external-operation contract of an operation
// (ADR-0029 section 5). The rules that bind its fields together are
// [flow.CheckContract], so the manifest and the flow validation cannot
// disagree.
type Safety struct {
	Effect       flow.EffectKind
	Level        flow.SafetyLevel
	Idempotency  *IdempotencyContract
	Verification *VerificationContract
	OnUncertain  flow.OnUncertain
	RetrySafe    bool
	Compensation *Compensation
	Batch        *Batch
	ManualCheck  *ManualCheck
	ProofLevel   ProofLevel
}

// IdempotencyContract describes how repeated requests are deduplicated.
type IdempotencyContract struct {
	Mode flow.IdempotencyMode
	// Location is where the key travels; required with a key mode.
	Location *flow.KeyPlace
	// TTL is the ISO 8601 duration a key stays valid at the provider.
	TTL   string
	Scope KeyScope
}

// VerificationContract names the read operation that looks an outcome up.
type VerificationContract struct {
	Operation string
	// Match is the expression that recognizes the outcome.
	Match string
	// NotFoundMeansFailed says whether a missing record proves the failure.
	NotFoundMeansFailed bool
	// Consistency is "STRONG" or an ISO 8601 delay before the record is visible.
	Consistency string
}

// Compensation names the operation that undoes this one.
type Compensation struct {
	Operation    string
	Precondition string
	Verify       string
}

// Batch describes a bulk operation.
type Batch struct {
	MaxItems       int
	PerItemResults bool
}

// ManualCheck tells a person how to check in the provider whether a write
// happened.
type ManualCheck struct {
	// Description is a template.
	Description string
	// URL is an optional template.
	URL string
}

// Trigger is the declaration of a trigger node (ADR-0020 section 3).
type Trigger struct {
	Family          TriggerFamily
	Delivery        Delivery
	DedupeKeySource DedupeSource
	// MaxPayload is the largest accepted event payload in bytes; zero means
	// the instance default.
	MaxPayload int64
}

// Credential is a credential type a connector needs.
type Credential struct {
	ID          string
	DisplayName string
	Fields      []CredentialField
	Scheme      Scheme
	// Test names the operation that tests a credential.
	Test string
	// N8nTypes are the n8n credential types this one replaces.
	N8nTypes []string
}

// CredentialField is one value a person fills in.
type CredentialField struct {
	Name      string
	Type      FieldType
	Sensitive bool
	Required  bool
}

// Scheme says how the host applies a credential to a request (ADR-0030
// section 4). Only the members of its Type are set.
type Scheme struct {
	Type SchemeType
	// In and Name locate an API key (API_KEY).
	In     Placement
	Name   string
	Prefix string
	// Grant, AuthURL, TokenURL, Scopes and TokenPlacement describe OAUTH2.
	Grant          OAuthGrant
	AuthURL        string
	TokenURL       string
	Scopes         []string
	TokenPlacement string
	// Headers holds the header templates of CUSTOM_HEADERS, in document order.
	Headers []HeaderTemplate
	// Service and Region describe AWS_SIG_V4.
	Service string
	Region  string
	// Login, Extract, TTL and Placement describe SESSION_TOKEN.
	Login     string
	Extract   string
	TTL       string
	Placement string
}

// HeaderTemplate is one header of the CUSTOM_HEADERS scheme.
type HeaderTemplate struct {
	Name     string
	Template string
}

// Permissions are what the connector asks an administrator to approve.
type Permissions struct {
	Network Network
	// Credentials lists the credential types the connector may use.
	Credentials []string
	// CredentialRawAccess lets the connector read raw credential values; it
	// needs the administrator's approval.
	CredentialRawAccess bool
	Resources           Resources
}

// Network is the network the connector may reach (ADR-0029 section 8).
type Network struct {
	// Domains are exact hosts or one leading "*." label wildcard over a host.
	Domains []string
	Ports   []int
	// Schemes are "https" and "http"; empty means "https" only.
	Schemes []string
	// AllowPrivateNetworks lets the connector reach private addresses.
	AllowPrivateNetworks bool
	// FromCredential lists credential fields whose value is the host, for
	// self-hosted services.
	FromCredential []string
}

// Resources bounds what one invocation may use.
type Resources struct {
	MemoryMiB     int
	Timeout       string
	MaxConcurrent int
}

// RateLimit is a named provider rate limit (ADR-0030 section 7).
type RateLimit struct {
	Name string
	// Key is CREDENTIAL, HOST or an expression starting with "=".
	Key               string
	RequestsPerSecond float64
	Burst             int
	Concurrency       int
	RespectHeaders    *RespectHeaders
}

// RespectHeaders names the response headers that carry rate limit state.
type RespectHeaders struct {
	RetryAfter string
	ResetAt    *ResetAt
	Remaining  string
}

// ResetAt names the header holding the reset time and how it is written.
type ResetAt struct {
	Header string
	Format ResetFormat
}

// ChangelogEntry is what changed in one released version, as message keys of
// the package's i18n catalogs.
type ChangelogEntry struct {
	Version string
	// Date is a calendar date, YYYY-MM-DD.
	Date  string
	Notes []string
}

// Value returns the JSON document the manifest was read from.
func (m *Manifest) Value() jsonvalue.Value { return m.doc }

// PermissionsDigest returns "sha256:" and the hexadecimal SHA-256 of the JSON
// Canonicalization Scheme text of the permissions block. A grant records it, so
// a version that asks for different permissions needs a new approval
// (ADR-0033 section 7).
func (m *Manifest) PermissionsDigest() (string, error) {
	perms, ok := m.doc.Get("permissions")
	if !ok {
		perms = jsonvalue.Value{}
	}
	canonical, err := jsonvalue.Canonical(perms)
	if err != nil {
		return "", fmt.Errorf("canonicalize permissions of %s: %w", m.ID, err)
	}
	sum := sha256.Sum256(canonical)
	return DigestPrefix + hex.EncodeToString(sum[:]), nil
}

// NodeByType returns the node with the given node type.
func (m *Manifest) NodeByType(nodeType string) (*Node, bool) {
	for i := range m.Nodes {
		if m.Nodes[i].Type == nodeType {
			return &m.Nodes[i], true
		}
	}
	return nil, false
}

// OperationByName returns the operation of the node with the given name.
func (n *Node) OperationByName(name string) (*Operation, bool) {
	for i := range n.Operations {
		if n.Operations[i].Name == name {
			return &n.Operations[i], true
		}
	}
	return nil, false
}

// Contract returns the flow-level contract of an operation, with the purity of
// its verification operation resolved against the node. The result is what
// [flow.CheckContract] judges. It is nil when the operation has no safety
// contract.
func (n *Node) Contract(op *Operation) *flow.Contract {
	if op.Safety == nil {
		return nil
	}
	s := op.Safety
	c := &flow.Contract{
		Effect: s.Effect, Level: s.Level, OnUncertain: s.OnUncertain, RetrySafe: s.RetrySafe,
	}
	if s.Idempotency != nil {
		c.Idempotency = &flow.Idempotency{Mode: s.Idempotency.Mode, Location: s.Idempotency.Location, TTL: s.Idempotency.TTL}
	}
	if s.Verification != nil {
		v := &flow.Verification{Operation: s.Verification.Operation}
		if target, ok := n.OperationByName(s.Verification.Operation); ok {
			v.OperationPurity = target.Purity
		}
		c.Verification = v
	}
	return c
}
