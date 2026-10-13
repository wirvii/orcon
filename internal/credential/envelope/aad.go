// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package envelope

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/identity"
)

const aadPrefix = "orcon:v1"

// AAD is the additional authenticated data of a ciphertext: the row it
// belongs to. It is encoded as orcon:v1|org_id|object_type|object_id|version|field
// (ADR-0042 section 1), so a ciphertext moved to another organization, object,
// version or field fails authentication.
//
// OrgID is the zero value only for instance-level secrets. ObjectType,
// ObjectID and Field are required and must not contain the separator.
type AAD struct {
	OrgID      identity.OrgID
	ObjectType string
	ObjectID   string
	Version    int
	Field      string
}

// Bytes returns the encoding, or [ErrInvalidAAD].
func (a AAD) Bytes() ([]byte, error) {
	if a.Version < 0 {
		return nil, fmt.Errorf("%w: negative version", ErrInvalidAAD)
	}
	for _, f := range [...]struct{ name, value string }{
		{"object type", a.ObjectType},
		{"object id", a.ObjectID},
		{"field", a.Field},
	} {
		if err := checkAADText(f.name, f.value); err != nil {
			return nil, err
		}
	}
	org := ""
	if !a.OrgID.IsZero() {
		org = a.OrgID.String()
	}
	return []byte(strings.Join([]string{
		aadPrefix, org, a.ObjectType, a.ObjectID, strconv.Itoa(a.Version), a.Field,
	}, "|")), nil
}

// checkAADText rejects what would make two different rows encode alike: an
// empty value, the separator and control characters.
func checkAADText(name, value string) error {
	if value == "" {
		return fmt.Errorf("%w: empty %s", ErrInvalidAAD, name)
	}
	for _, r := range value {
		if r == '|' || r < 0x20 || r == 0x7f {
			return fmt.Errorf("%w: %s contains a reserved character", ErrInvalidAAD, name)
		}
	}
	return nil
}

// The purposes that keyAAD distinguishes.
const (
	purposeKeyWrap = "key"
	purposeDEKWrap = "dek"
	purposeCanary  = "canary"
)

// canaryPlaintext is the fixed constant a canary encrypts.
const canaryPlaintext = "orcon-canary-v1"

// keyAAD binds a wrapped key or a canary to its purpose and its key identity,
// so the wrapped form of one key never opens as another key or as a canary.
func keyAAD(purpose string, id KeyID) []byte {
	org := ""
	if id.Level == LevelOrg {
		org = id.OrgID.String()
	}
	return []byte(strings.Join([]string{
		aadPrefix, purpose, string(id.Level), org, strconv.Itoa(id.Version),
	}, "|"))
}
