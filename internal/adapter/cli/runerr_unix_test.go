// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package cli_test

import (
	"os"
	"os/user"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/cli"
)

func TestReloadReportsAnUnreadableRunDirectoryAndItsOwner(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read any directory")
	}
	dir := t.TempDir()
	if err := cli.EnsureRunDir(dir); err != nil {
		t.Fatal(err)
	}
	run := cli.RunDir(dir)
	if err := os.Chmod(run, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(run, 0o700) }) // so the temporary directory can be removed
	me, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}

	_, err = cli.RequestReload(t.Context(), dir, time.Second, 10*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "permission denied") || !strings.Contains(err.Error(), "owned by "+me.Username) {
		t.Fatalf("RequestReload = %v, want a permission error that names the owner %s", err, me.Username)
	}
	r := exec(t, cli.Env{Environ: []string{"HOME=/h", "ORCON_DATA_DIR=" + dir}}, "config", "reload")
	if r.code != cli.ExitError || !strings.Contains(r.err.String(), "owned by "+me.Username) {
		t.Fatalf("exit %d stderr %q; want exit 1 naming the owner", r.code, r.err.String())
	}
}
