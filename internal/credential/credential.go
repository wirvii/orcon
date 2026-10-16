// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// Status is the state of a credential. The stored values are UPPER_SNAKE
// (ADR-0015 section 2, SPEC-004 K19).
type Status string

// The states of a credential (ADR-0043 section 1).
const (
	// StatusOK means the credential can be used, subject to its sharing and host binding.
	StatusOK Status = "OK"
	// StatusNeedsSecret means a value is missing, for example after an import (R-IMP-02).
	StatusNeedsSecret Status = "NEEDS_SECRET"
	// StatusNeedsReauth means an OAuth grant was refused and the owner must consent again.
	StatusNeedsReauth Status = "NEEDS_REAUTH"
	// StatusDisabled means the owner switched the credential off.
	StatusDisabled Status = "DISABLED"
)

// Format names the encoding of the documents a credential stores. Only JSON exists.
const Format = "JSON"

// Errors. Every message names what failed and never a value.
var (
	// ErrNotFound means no credential of that identity exists in the organization, or the
	// project neither owns it nor holds a share of it.
	ErrNotFound = errors.New("credential: not found")
	// ErrInvalid means the input breaks a rule: a name, a field, a host or a size.
	ErrInvalid = errors.New("credential: invalid input")
	// ErrConflict means the change clashes with a stored row, such as a name already taken or a share that exists.
	ErrConflict = errors.New("credential: conflict")
	// ErrNotOwner means the project asked for an owner-only change (replace, update, delete, share) and does not own
	// the credential (R-AZ-07).
	ErrNotOwner = errors.New("credential: only the owning project may change the credential")
	// ErrNotUsable means the project may not use the credential now: its share was removed, or the credential is not
	// in status OK. A dispatch fails with this error explicitly (E-CRED-2).
	ErrNotUsable = errors.New("credential: the project cannot use the credential")
	// ErrHostNotAllowed means the request would send the credential to a host outside its allowed hosts (R-CRED-03).
	ErrHostNotAllowed = errors.New("credential: the credential is not allowed for this host")
	// ErrDecrypt means a stored ciphertext does not open under its key and row. The cause is never a value.
	ErrDecrypt = errors.New("credential: a stored value cannot be decrypted")
)

// Record is a credential row as the store keeps it. It holds no secret value.
type Record struct {
	ID        id.UUID
	OrgID     identity.OrgID
	ProjectID identity.ProjectID
	Name      string
	// ConnectorReference names the connector type the credential serves (SPEC-003).
	ConnectorReference string
	Status             Status
	// AllowedHosts lists the hosts the credential may be sent to (R-CRED-03).
	AllowedHosts   []string
	CurrentVersion int
	CreatedBy      identity.PrincipalID
	CreatedAt      time.Time
	UpdatedAt      time.Time
	// SecretUpdatedAt and SecretUpdatedBy describe the current version: when its secret was set and by whom.
	SecretUpdatedAt time.Time
	SecretUpdatedBy identity.PrincipalID
}

// Version is one stored version of a credential. The secret fields are encrypted; the public fields are JSON
// text and are the only part a reader ever sees.
type Version struct {
	ID           id.UUID
	OrgID        identity.OrgID
	CredentialID id.UUID
	Number       int
	// PublicFields is the JSON object of the non-secret fields.
	PublicFields []byte
	// SecretCiphertext is the binary envelope of the JSON object of the secret fields.
	SecretCiphertext []byte
	// DataKeyWrapped is the data key that sealed SecretCiphertext, wrapped by the organization key version below.
	DataKeyWrapped []byte
	// OrgKeyVersion is the organization key version that wrapped DataKeyWrapped.
	OrgKeyVersion int
	CreatedBy     identity.PrincipalID
	CreatedAt     time.Time
}

// Share lets another project of the same organization use a credential, never read it (R-AZ-07).
type Share struct {
	ID           id.UUID
	OrgID        identity.OrgID
	CredentialID id.UUID
	ProjectID    identity.ProjectID
	GrantedBy    identity.PrincipalID
	CreatedAt    time.Time
}

// Input is what a person enters to create a credential or to replace its secret. Public and Secret hold the field
// values by field name; Secret values never leave the store in clear.
type Input struct {
	Name               string
	ConnectorReference string
	AllowedHosts       []string
	Public             map[string]string
	Secret             map[string]string
}

// Settings is the change of a credential's name or host list (the secret is replaced with [Input] instead).
type Settings struct {
	Name         string
	AllowedHosts []string
}

// SecretState is what a reader sees of the secret: whether one is set and when and by whom it was last written.
// It never carries the value (R-CRED-01).
type SecretState struct {
	Set       bool
	UpdatedAt time.Time
	UpdatedBy identity.PrincipalID
}

// Summary is a credential as a reader of its project sees it. Owned is true when the reader's project owns it.
type Summary struct {
	Record
	Owned  bool
	Secret SecretState
	// Public holds the non-secret fields of the current version. List does not fill it.
	Public map[string]string
}

// Resolved is a credential at dispatch: its current version with the secret fields decrypted in memory. It is built
// only by [Service.Resolve]. The secret values are reachable through [Resolved.Secret] and [Resolved.SecretValues],
// which the caller adds to the masking set of the execution (R-CRED-04).
type Resolved struct {
	ID      id.UUID
	Name    string
	Version int
	// Public holds the non-secret fields.
	Public map[string]string

	hosts  []string
	secret map[string]string
}

// Secret returns one secret field and whether it exists.
func (r Resolved) Secret(field string) (string, bool) {
	v, ok := r.secret[field]
	return v, ok
}

// SecretNames returns the names of the secret fields, sorted. A caller that needs every field of the credential reads
// each one through [Resolved.Secret]; the values are never listed with their names.
func (r Resolved) SecretNames() []string {
	out := make([]string, 0, len(r.secret))
	for name := range r.secret {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

// SecretValues returns every secret value, for the masking set of an execution. The caller must not store them.
func (r Resolved) SecretValues() []string {
	out := make([]string, 0, len(r.secret))
	for _, v := range r.secret {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// CheckHost returns [ErrHostNotAllowed] unless the credential may be sent to host (R-CRED-03). A host matches an
// entry exactly, or a "*.suffix" entry with exactly one extra label.
func (r Resolved) CheckHost(host string) error {
	if !hostAllowed(r.hosts, host) {
		return ErrHostNotAllowed
	}
	return nil
}

// AllowedHosts returns the hosts the credential may be sent to.
func (r Resolved) AllowedHosts() []string {
	out := make([]string, len(r.hosts))
	copy(out, r.hosts)
	return out
}

// String implements [fmt.Stringer] and hides the secret values from every format verb.
func (Resolved) String() string { return "credential.Resolved{[REDACTED]}" }

// GoString implements [fmt.GoStringer] and hides the secret values from %#v.
func (Resolved) GoString() string { return "credential.Resolved{[REDACTED]}" }

// LogValue implements [slog.LogValuer] and hides the secret values from every log line.
func (Resolved) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }
