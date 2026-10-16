// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Bounds of the input. They are checked before any encryption and name only the field, never its value.
const (
	maxNameLength          = 100
	maxConnectorLength     = 255
	maxFieldNameLength     = 64
	maxFields              = 64
	maxPublicValueBytes    = 4096
	maxSecretValueBytes    = 65536
	maxSecretDocumentBytes = 1 << 20
)

// validateName checks a credential name: 1 to 100 characters without control characters.
func validateName(name string) error {
	n := strings.TrimSpace(name)
	if n == "" || utf8.RuneCountInString(n) > maxNameLength {
		return fmt.Errorf("%w: the name must be 1 to %d characters", ErrInvalid, maxNameLength)
	}
	for _, r := range n {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("%w: the name contains a control character", ErrInvalid)
		}
	}
	return nil
}

// validateField checks a field name: letters, digits, underscore, dot and hyphen, 1 to 64 characters.
func validateField(name string) error {
	if name == "" || len(name) > maxFieldNameLength {
		return fmt.Errorf("%w: a field name must be 1 to %d characters", ErrInvalid, maxFieldNameLength)
	}
	for i := range len(name) {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_', c == '.', c == '-':
		default:
			return fmt.Errorf("%w: field %q has a character a field name may not hold", ErrInvalid, name)
		}
	}
	return nil
}

// validateFields checks the field maps of an input: the field count, the field names and the value sizes. A secret
// and a public field may not share a name.
func validateFields(public, secret map[string]string) error {
	if len(public)+len(secret) == 0 {
		return fmt.Errorf("%w: a credential needs at least one field", ErrInvalid)
	}
	if len(public)+len(secret) > maxFields {
		return fmt.Errorf("%w: at most %d fields", ErrInvalid, maxFields)
	}
	for name, v := range public {
		if err := validateField(name); err != nil {
			return err
		}
		if _, dup := secret[name]; dup {
			return fmt.Errorf("%w: field %q is both public and secret", ErrInvalid, name)
		}
		if len(v) > maxPublicValueBytes {
			return fmt.Errorf("%w: public field %q is longer than %d bytes", ErrInvalid, name, maxPublicValueBytes)
		}
	}
	for name, v := range secret {
		if err := validateField(name); err != nil {
			return err
		}
		if len(v) > maxSecretValueBytes {
			return fmt.Errorf("%w: secret field %q is longer than %d bytes", ErrInvalid, name, maxSecretValueBytes)
		}
	}
	return nil
}

// validateConnector checks the connector reference of a credential.
func validateConnector(ref string) error {
	if ref == "" || len(ref) > maxConnectorLength {
		return fmt.Errorf("%w: the connector reference must be 1 to %d characters", ErrInvalid, maxConnectorLength)
	}
	return nil
}
