// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

//go:build race

package wazero_test

import "time"

// The race detector slows the host side of every call into the sandbox by a
// large factor, so tests that are not about time limits get a long one.
const defaultTestTimeout = 60 * time.Second
