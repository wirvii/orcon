// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// The reload channel (SPEC-015 plan, amendment 2 section 2-3) is the same on
// every operating system: the command writes a request file in <dataDir>/run and
// the server, which looks for it every few seconds, reloads its configuration and
// writes a result file. There is no pid file and no signal, so a stale file can
// never make the command signal an unrelated program.
const (
	runDirName      = "run"
	requestFileName = "reload-request"
	claimedFileName = "reload-request.claimed"
	resultFileName  = "reload-result"
	// The most that either side reads from a file of the channel.
	maxChannelFile = 64 << 10
)

// Default timings of the reload channel.
const (
	// ReloadPollInterval is how often the server looks for a request.
	ReloadPollInterval = 2 * time.Second
	// ReloadWait is how long the command waits for the server's result.
	ReloadWait = 10 * time.Second
	// How often the command looks for the result.
	clientPollInterval = 100 * time.Millisecond
)

// ErrNoServer means no server picked up the reload request in time.
var ErrNoServer = errors.New("no running Orcon server picked up the reload request")

// ReloadRequest is the content of the request file.
type ReloadRequest struct {
	// RequestID is random, so a result can be matched to its request.
	RequestID string `json:"requestId"`
}

// ReloadResult is the content of the result file and the output of orcon config reload.
type ReloadResult struct {
	// RequestID is the id of the request this answers.
	RequestID string `json:"requestId" yaml:"requestId"`
	// AppliedKeys lists the live keys whose new value is now in effect.
	AppliedKeys []string `json:"appliedKeys" yaml:"appliedKeys"`
	// RestartRequiredKeys lists the keys that changed and need a restart; they keep their running value.
	RestartRequiredKeys []string `json:"restartRequiredKeys" yaml:"restartRequiredKeys"`
	// Errors lists why the new configuration was refused; the previous one stays in effect.
	Errors []string `json:"errors" yaml:"errors"`
}

// RunDir returns the directory of the reload channel inside a data directory.
func RunDir(dataDir string) string { return filepath.Join(dataDir, runDirName) }

// EnsureRunDir creates <dataDir>/run, readable by the owner only. The server
// calls it at start.
func EnsureRunDir(dataDir string) error {
	dir := RunDir(dataDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil { //nolint:gosec // G302: a directory needs the execute bit; 0700 is owner-only
		return fmt.Errorf("restrict %s: %w", dir, err)
	}
	return nil
}

// writeAtomic writes data to path through a temporary file and a rename, so a
// reader never sees half a file.
func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("write %s: %w", name, err)
	}
	if err := os.Chmod(name, 0o600); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("restrict %s: %w", name, err)
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("rename to %s: %w", path, err)
	}
	return nil
}

func readBounded(path string) ([]byte, error) {
	f, err := os.Open(path) //nolint:gosec // G304: the path is inside the run directory of the operator's data directory
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }() // read-only file
	data, err := io.ReadAll(io.LimitReader(f, maxChannelFile))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

// RequestReload asks the servers that use dataDir to reload their configuration
// and waits up to wait for the answer. It returns [ErrNoServer] when nobody
// answers; it then removes its request.
func RequestReload(ctx context.Context, dataDir string, wait, poll time.Duration) (*ReloadResult, error) {
	dir := RunDir(dataDir)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil, noServerError(dataDir)
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, fmt.Errorf("generate request id: %w", err)
	}
	id := hex.EncodeToString(raw[:])
	body, err := json.Marshal(ReloadRequest{RequestID: id})
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	request := filepath.Join(dir, requestFileName)
	if err := writeAtomic(request, body); err != nil {
		return nil, err
	}
	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	tick := time.NewTicker(poll)
	defer tick.Stop()
	for {
		if res := readResult(filepath.Join(dir, resultFileName), id); res != nil {
			return res, nil
		}
		select {
		case <-ctx.Done():
			_ = os.Remove(request) // best effort: the command is going away
			return nil, fmt.Errorf("wait for the reload result: %w", ctx.Err())
		case <-deadline.C:
			_ = os.Remove(request) // best effort: nobody picked it up
			return nil, noServerError(dataDir)
		case <-tick.C:
		}
	}
}

func noServerError(dataDir string) error {
	return fmt.Errorf("%w in %s; is it running with this data directory?", ErrNoServer, dataDir)
}

// readResult returns the result for request id, or nil when it is not there yet.
func readResult(path, id string) *ReloadResult {
	data, err := readBounded(path)
	if err != nil {
		return nil
	}
	var res ReloadResult
	if json.Unmarshal(data, &res) != nil || res.RequestID != id {
		return nil
	}
	return &res
}

// WatchReloadRequests looks for a reload request in <dataDir>/run every poll
// interval until ctx ends. For each request it calls handle with the request id
// and writes the returned result for the command to read. It runs on the calling
// goroutine; the caller owns it and waits for it to return.
//
// When several servers share a data directory, the one whose poll finds the
// request first claims it (by renaming it), so only that one answers.
func WatchReloadRequests(ctx context.Context, dataDir string, poll time.Duration, handle func(requestID string) ReloadResult, logger *slog.Logger) {
	dir := RunDir(dataDir)
	tick := time.NewTicker(poll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			serveRequest(ctx, dir, handle, logger)
		}
	}
}

func serveRequest(ctx context.Context, dir string, handle func(string) ReloadResult, logger *slog.Logger) {
	request, claimed := filepath.Join(dir, requestFileName), filepath.Join(dir, claimedFileName)
	if err := os.Rename(request, claimed); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logger.WarnContext(ctx, "cannot claim the reload request", "error", err)
		}
		return
	}
	defer func() { _ = os.Remove(claimed) }() // the request is consumed whatever happens
	data, err := readBounded(claimed)
	if err != nil {
		logger.WarnContext(ctx, "cannot read the reload request", "error", err)
		return
	}
	var req ReloadRequest
	if err := json.Unmarshal(data, &req); err != nil || req.RequestID == "" {
		logger.WarnContext(ctx, "ignored a malformed reload request")
		return
	}
	res := handle(req.RequestID)
	res.RequestID = req.RequestID
	body, err := json.Marshal(res)
	if err != nil {
		logger.ErrorContext(ctx, "cannot encode the reload result", "error", err)
		return
	}
	if err := writeAtomic(filepath.Join(dir, resultFileName), body); err != nil {
		logger.ErrorContext(ctx, "cannot write the reload result", "error", err)
	}
}
