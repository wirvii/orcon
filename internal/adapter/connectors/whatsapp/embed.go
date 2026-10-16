// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package whatsapp is the official declarative connector of the WhatsApp Cloud
// API of Meta (orcon.whatsapp). Its operations are described as data in
// connector.json and run by the declarative interpreter; this package holds
// only the package files, which the tests build and check.
package whatsapp

import "embed"

// Files holds the whole connector package: manifest, declarative body, authoring
// record, license and notice, catalogs and icon. The connection guides under
// docs/ are documentation and are not part of the package.
//
//go:embed orcon-package.json connector.json authoring.json LICENSE NOTICE i18n icons
var Files embed.FS
