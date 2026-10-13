// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package acme is the official declarative connector for Acme, the example
// service of the connector templates. Copy this folder to
// internal/adapter/connectors/<name>/ and rename the service.
package acme

import "embed"

// Files holds the whole connector package: manifest, declarative body,
// authoring record, catalogs, icon, documentation and recorded tests.
// cmd/orcon registers it like a native connector (ADR-0079 §1).
//
//go:embed orcon-package.json connector.json authoring.json i18n icons docs tests
var Files embed.FS
