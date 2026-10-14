// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// ReloadReport says what a reload did.
type ReloadReport struct {
	// Applied lists the live keys whose value changed and now has effect.
	Applied []string
	// Pending lists the keys that changed but need a restart; the running value is kept.
	Pending []string
	// Issues are the errors and warnings of the new configuration.
	Issues []Issue
}

// Reloader holds the configuration of a running process and rereads it on
// request (R-CONF-09). Only keys marked live change; the others keep their value
// until the process restarts, and the report lists them.
//
// A Reloader is safe for concurrent use. The load function should re-run
// [Schema.Load] with the same flags and environment as the start: only the
// configuration file and the secret files can have changed.
type Reloader struct {
	load    func() *Result
	current atomic.Pointer[Config]

	mu   sync.Mutex // serializes reloads and guards subs
	subs []func(old, current *Config, applied []string)
}

// NewReloader returns a Reloader whose current configuration is initial.
func NewReloader(initial *Config, load func() *Result) *Reloader {
	r := &Reloader{load: load}
	r.current.Store(initial)
	return r
}

// Current returns the configuration in effect.
func (r *Reloader) Current() *Config { return r.current.Load() }

// Subscribe registers fn to be called after a reload that applied at least one
// key, with the previous and the new configuration and the applied key paths.
// Subscribers run on the goroutine that called Reload, one after another.
func (r *Reloader) Subscribe(fn func(old, current *Config, applied []string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subs = append(r.subs, fn)
}

// Reload rereads the configuration. When the new configuration has errors it
// changes nothing and returns them as the error, so a bad edit never takes down
// a running process.
func (r *Reloader) Reload() (*ReloadReport, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	res := r.load()
	report := &ReloadReport{Issues: res.Issues}
	if err := res.Err(); err != nil {
		return report, fmt.Errorf("reload kept the running configuration: %w", err)
	}
	old := r.current.Load()
	merged := &Config{
		schema:        res.Config.schema,
		entries:       make(map[string]*Entry, len(res.Config.entries)),
		file:          res.Config.file,
		inlineSecrets: res.Config.inlineSecrets,
		fileMode:      res.Config.fileMode,
		platform:      res.Config.platform,
	}
	for _, k := range merged.schema.keys {
		next, prev := res.Config.entries[k.Path], old.entries[k.Path]
		switch {
		case prev == nil || prev.Value.equal(next.Value):
			merged.entries[k.Path] = next
		case k.Reload == ReloadLive:
			merged.entries[k.Path] = next
			report.Applied = append(report.Applied, k.Path)
		default:
			merged.entries[k.Path] = prev
			report.Pending = append(report.Pending, k.Path)
		}
	}
	r.current.Store(merged)
	if len(report.Applied) > 0 {
		for _, fn := range r.subs {
			fn(old, merged, report.Applied)
		}
	}
	return report, nil
}
