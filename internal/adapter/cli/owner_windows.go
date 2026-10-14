// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package cli

// ownerOf returns "": Windows access control lists have no single owner id to show.
func ownerOf(string) string { return "" }
