// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package redact holds the types that keep secret values out of logs, errors,
// JSON and every other text form (go-style.md GS-O5). It is a platform package:
// it imports no area and no adapter.
//
// A value wrapped in [Secret] prints as [Redacted] through every formatter and
// encoder; the plain text is available only through [Secret.Reveal], which the
// caller uses at the point it hands the value to the component that needs it.
package redact

import (
	"fmt"
	"log/slog"
)

// Redacted is what a secret prints as.
const Redacted = "[REDACTED]"

// Secret is a secret string held in memory. The zero Secret is empty. A Secret
// is immutable: copies share nothing that can change.
type Secret struct{ plain string }

// New wraps a plain value.
func New(plain string) Secret { return Secret{plain: plain} }

// Reveal returns the plain value.
func (s Secret) Reveal() string { return s.plain }

// IsSet reports whether the secret holds a value.
func (s Secret) IsSet() bool { return s.plain != "" }

// String returns [Redacted].
func (Secret) String() string { return Redacted }

// GoString returns [Redacted], so %#v does not leak the value.
func (Secret) GoString() string { return Redacted }

// Format writes [Redacted] for every verb, so no fmt verb can reach the value.
func (Secret) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(Redacted)) }

// MarshalText returns [Redacted], so text encoders do not leak the value.
func (Secret) MarshalText() ([]byte, error) { return []byte(Redacted), nil }

// MarshalJSON returns the JSON string "[REDACTED]".
func (Secret) MarshalJSON() ([]byte, error) { return []byte(`"` + Redacted + `"`), nil }

// LogValue returns [Redacted] for log/slog.
func (Secret) LogValue() slog.Value { return slog.StringValue(Redacted) }
