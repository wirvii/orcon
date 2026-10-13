// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !race

package wazero_test

import "time"

// The default time limit of one expression (ADR-0018 section 3).
const defaultTestTimeout = 100 * time.Millisecond
