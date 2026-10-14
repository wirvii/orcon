// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"flag"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/platform/config"
)

// reloadFixture returns a Reloader over a file the test can change.
func reloadFixture(t *testing.T, initial string) (*config.Reloader, fakeFS) {
	t.Helper()
	files := fakeFS{cfgPath: {data: initial, mode: 0o600}}
	schema := config.Default()
	loadAgain := func() *config.Result {
		fset := flag.NewFlagSet("test", flag.ContinueOnError)
		fset.SetOutput(io.Discard)
		fv := schema.BindFlags(fset)
		if err := schema.ParseFlags(fset, []string{"--config", cfgPath}); err != nil {
			t.Fatal(err)
		}
		return schema.Load(config.Input{Flags: fv, Environ: baseEnv(), FS: files, Decode: fakeDecode, GOOS: "linux"})
	}
	first := loadAgain()
	if err := first.Err(); err != nil {
		t.Fatal(err)
	}
	return config.NewReloader(first.Config, loadAgain), files
}

func TestReloadAppliesLiveKeysAndKeepsRestartKeys(t *testing.T) {
	t.Parallel()

	r, files := reloadFixture(t, "log.level: info\nhttp.address: 127.0.0.1:1000\nupdateCheck.enabled: false")
	var seen []string
	r.Subscribe(func(_, current *config.Config, applied []string) {
		seen = append(seen, applied...)
		if current.Value("log.level").Text() != "debug" {
			t.Errorf("a subscriber saw log.level = %q, want debug", current.Value("log.level").Text())
		}
	})
	files[cfgPath] = fakeFile{data: "log.level: debug\nhttp.address: 127.0.0.1:2000\nupdateCheck.enabled: true", mode: 0o600}

	report, err := r.Reload()
	if err != nil {
		t.Fatalf("Reload = %v", err)
	}
	slices.Sort(report.Applied)
	if want := []string{"log.level", "updateCheck.enabled"}; !slices.Equal(report.Applied, want) {
		t.Errorf("Applied = %v, want %v", report.Applied, want)
	}
	if want := []string{"http.address"}; !slices.Equal(report.Pending, want) {
		t.Errorf("Pending = %v, want %v (restart key)", report.Pending, want)
	}
	cur := r.Current()
	if got := cur.Value("log.level").Text(); got != "debug" {
		t.Errorf("log.level = %q, want debug", got)
	}
	if got := cur.Value("http.address").Text(); got != "127.0.0.1:1000" {
		t.Errorf("http.address = %q, want the running value 127.0.0.1:1000", got)
	}
	slices.Sort(seen)
	if want := []string{"log.level", "updateCheck.enabled"}; !slices.Equal(seen, want) {
		t.Errorf("subscriber saw %v, want %v", seen, want)
	}
}

func TestReloadWithErrorsKeepsTheRunningConfiguration(t *testing.T) {
	t.Parallel()

	r, files := reloadFixture(t, "log.level: info")
	files[cfgPath] = fakeFile{data: "log.level: shout\nbogus: 1", mode: 0o600}
	report, err := r.Reload()
	if err == nil || !strings.Contains(err.Error(), "kept the running configuration") {
		t.Fatalf("Reload = %v, want an error that says the configuration was kept", err)
	}
	if len(report.Issues) == 0 {
		t.Error("the report must carry the issues")
	}
	if got := r.Current().Value("log.level").Text(); got != "info" {
		t.Errorf("log.level = %q, want info", got)
	}
}

func TestReloadWithoutChangesCallsNoSubscriber(t *testing.T) {
	t.Parallel()

	r, _ := reloadFixture(t, "log.level: info")
	r.Subscribe(func(_, _ *config.Config, _ []string) { t.Error("nothing changed, nobody should be called") })
	report, err := r.Reload()
	if err != nil || len(report.Applied) != 0 || len(report.Pending) != 0 {
		t.Fatalf("Reload = %+v, %v; want an empty report", report, err)
	}
}

func TestReloadIsSafeUnderConcurrency(t *testing.T) {
	t.Parallel()

	r, _ := reloadFixture(t, "log.level: info")
	done := make(chan struct{})
	for range 4 {
		go func() {
			defer func() { done <- struct{}{} }()
			for range 20 {
				if _, err := r.Reload(); err != nil {
					t.Error(err)
				}
				_ = r.Current().Value("log.level")
			}
		}()
	}
	for range 4 {
		<-done
	}
}
