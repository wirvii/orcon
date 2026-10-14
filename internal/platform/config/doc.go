// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package config declares, resolves and validates the configuration of an Orcon
// process. It is a platform package (architecture-layers.md AL-1): it imports no
// area and no adapter.
//
// Every server key is declared once, in this package (key.go and the keys_*.go
// files). From that one schema come the command-line flags, the environment
// variable names, the YAML members, the checks of orcon config check, the JSON
// Schema of the file and the generated reference (R-CONF-02). Precedence is
// per key: flag, then environment variable, then the YAML file, then the
// default (R-CONF-01).
//
// The YAML syntax itself is not parsed here: the composition root passes a
// [FileDecoder] from internal/adapter/codec/configyaml (SPEC-008 AL-7).
//
// Also in this package are the process role type and the bounds of the expression
// sandbox settings, which predate the schema.
package config
