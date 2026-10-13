// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package cli holds the command-line commands of the orcon binary that are not
// the server itself: orcon config check, schema and reload, and the hidden orcon
// docs generate. Later cuts of SPEC-015 add health, setup, backup and store
// commands here.
//
// It is an adapter package. A command parses its arguments, calls the platform
// and area packages, prints, and returns an exit code (ADR 0058 section 4); it
// never calls os.Exit. Everything it touches outside the process (environment,
// files, YAML codec, connection probes, the local instance) arrives in [Env], so
// the composition root decides what is real and tests decide what is fake.
//
// Local commands never talk to the API and never open the database.
package cli
