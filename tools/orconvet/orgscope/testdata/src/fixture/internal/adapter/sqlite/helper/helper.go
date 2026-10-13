// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package helper builds statement text in another package of the layer.
package helper

import "strings"

// Select builds text at run time.
func Select(table string) string { return "SELECT id FROM " + table }

// Trim returns a constant, but a call is never a constant expression.
func Trim() string { return strings.TrimSpace("SELECT 1 ") }
