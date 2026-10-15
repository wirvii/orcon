// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package credential is the credential store of Orcon (SPEC-004 SE7, ADR-0043) and the
// production resolver that a dispatch uses to obtain a credential (ADR-0043 section 3).
//
// A credential belongs to one project. Its secret fields are write-only: they go in
// through [Service.Create] and [Service.Replace] and come out only through
// [Service.Resolve], which decrypts them in memory for one dispatch. Every replacement
// adds a version, so the current version is the one a dispatch uses. A credential
// carries the hosts it may be sent to ([Resolved.CheckHost], R-CRED-03). A project
// other than the owner may use a credential through a share, which never allows reading
// its secrets (R-AZ-07); the share is read on every resolution.
//
// The package imports no adapter. Storage is the [Store] port and the key hierarchy is
// the [DataKeys] port, which the envelope keyring implements (ADR-0042).
package credential
