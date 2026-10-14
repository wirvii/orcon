// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package envelope

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"

	"github.com/wirvii/orcon/internal/identity"
)

// KeyLevel is the level of a key in the hierarchy. The values are the ones
// the key tables store (enumerated columns are UPPER_SNAKE, ADR-0015 section 2).
type KeyLevel string

// The levels of the hierarchy.
const (
	// LevelMaster is the master key, which only has a canary: the key itself
	// is never stored.
	LevelMaster KeyLevel = "MASTER"
	// LevelInstance is the instance key (IK) for instance-level secrets.
	LevelInstance KeyLevel = "INSTANCE"
	// LevelOrg is an organization key (OK).
	LevelOrg KeyLevel = "ORG"
)

func (l KeyLevel) code() (byte, bool) {
	switch l {
	case LevelOrg:
		return 1, true
	case LevelInstance:
		return 2, true
	case LevelMaster:
		return 3, true
	default:
		return 0, false
	}
}

func levelFromCode(c byte) (KeyLevel, bool) {
	switch c {
	case 1:
		return LevelOrg, true
	case 2:
		return LevelInstance, true
	case 3:
		return LevelMaster, true
	default:
		return "", false
	}
}

// KeyID names one version of one key: the level, the organization (only for
// [LevelOrg]) and the version number, which starts at 1.
type KeyID struct {
	Level   KeyLevel
	OrgID   identity.OrgID
	Version int
}

// String returns a form for messages, for example "ORG:<org id>:v2".
func (k KeyID) String() string {
	if k.Level == LevelOrg {
		return "ORG:" + k.OrgID.String() + ":v" + strconv.Itoa(k.Version)
	}
	return string(k.Level) + ":v" + strconv.Itoa(k.Version)
}

// Validate checks the shape the level requires: an organization key names its
// organization, the other levels name none, and the version is positive.
func (k KeyID) Validate() error {
	if _, ok := k.Level.code(); !ok {
		return fmt.Errorf("%w: unknown level %q", ErrInvalidKeyID, string(k.Level))
	}
	if k.Version < 1 || int64(k.Version) > math.MaxUint32 {
		return fmt.Errorf("%w: version %d out of range", ErrInvalidKeyID, k.Version)
	}
	if k.Level == LevelOrg && k.OrgID.IsZero() {
		return fmt.Errorf("%w: organization key without organization", ErrInvalidKeyID)
	}
	if k.Level != LevelOrg && !k.OrgID.IsZero() {
		return fmt.Errorf("%w: %s key with an organization", ErrInvalidKeyID, string(k.Level))
	}
	return nil
}

// Scope returns the scope the key belongs to.
func (k KeyID) Scope() Scope { return Scope{Level: k.Level, OrgID: k.OrgID} }

// keyIDBinarySize is the fixed size of the binary form: level, organization,
// version.
const keyIDBinarySize = 1 + 16 + 4

// appendBinary appends the binary form. The key must have passed Validate.
func (k KeyID) appendBinary(dst []byte) []byte {
	code, _ := k.Level.code()
	dst = append(dst, code)
	dst = append(dst, k.OrgID[:]...)
	return binary.BigEndian.AppendUint32(dst, uint32(k.Version)) //nolint:gosec // G115: Validate bounds the version to uint32
}

func keyIDFromBinary(b []byte) (KeyID, error) {
	if len(b) != keyIDBinarySize {
		return KeyID{}, fmt.Errorf("%w: key identifier of %d bytes", ErrMalformed, len(b))
	}
	level, ok := levelFromCode(b[0])
	if !ok {
		return KeyID{}, fmt.Errorf("%w: unknown level code %d", ErrMalformed, b[0])
	}
	var org identity.OrgID
	copy(org[:], b[1:17])
	k := KeyID{Level: level, OrgID: org, Version: int(binary.BigEndian.Uint32(b[17:]))}
	if err := k.Validate(); err != nil {
		return KeyID{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	return k, nil
}

// Scope selects the key a new data key is wrapped with: an organization or
// the instance. The master level has no data keys.
type Scope struct {
	Level KeyLevel
	OrgID identity.OrgID
}

// InstanceScope is the scope of instance-level secrets.
func InstanceScope() Scope { return Scope{Level: LevelInstance} }

// OrgScope is the scope of one organization's secrets.
func OrgScope(org identity.OrgID) Scope { return Scope{Level: LevelOrg, OrgID: org} }

// Validate checks that the scope is the instance or an organization.
func (s Scope) Validate() error {
	if s.Level != LevelInstance && s.Level != LevelOrg {
		return fmt.Errorf("%w: scope level %q holds no data keys", ErrInvalidKeyID, string(s.Level))
	}
	return KeyID{Level: s.Level, OrgID: s.OrgID, Version: 1}.Validate()
}
