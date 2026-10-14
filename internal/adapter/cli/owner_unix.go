// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package cli

import (
	"os"
	"os/user"
	"strconv"
	"syscall"
)

// ownerOf returns the name of the user that owns the path, or its numeric id when
// the name is unknown, or "" when the owner cannot be found.
func ownerOf(path string) string {
	info, err := os.Lstat(path)
	if err != nil {
		return ""
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	id := strconv.FormatUint(uint64(st.Uid), 10)
	if u, err := user.LookupId(id); err == nil {
		return u.Username
	}
	return id
}
