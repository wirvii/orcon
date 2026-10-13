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
	"slices"
	"strings"
	"time"
)

// The reload channel (SPEC-015 plan, amendment 2 section 2-3) is the same on
// every operating system: the command writes a request file in <dataDir>/run and
// every server that uses that data directory, which looks for it every few
// seconds, reloads its configuration and writes its own result file. There is no
// pid file and no signal, so a stale file can never make the command signal an
// unrelated program.
//
// Files in <dataDir>/run:
//
//	reload-request            the request, written by the command, removed by it when done
//	reload-result.<server>    the answer of one server to the latest request
//	server.<server>           the registration of a live server, rewritten on every poll
const (
	runDirName      = "run"
	requestFileName = "reload-request"
	resultPrefix    = "reload-result."
	serverPrefix    = "server."
	// The most that either side reads from a file of the channel.
	maxChannelFile = 64 << 10
	// A request older than this is left over from a command that died; servers ignore it.
	staleRequest = time.Minute
	// A server that has not refreshed its registration for this many polls is gone.
	staleServerPolls = 3
	// How many request ids a server remembers, so it handles each one once.
	rememberedRequests = 16
)

// Default timings of the reload channel.
const (
	// ReloadPollInterval is how often the server looks for a request.
	ReloadPollInterval = 2 * time.Second
	// ReloadWait is how long the command waits for the servers' results.
	ReloadWait = 10 * time.Second
	// How often the command looks for results.
	clientPollInterval = 100 * time.Millisecond
)

// ErrNoServer means no server picked up the reload request in time.
var ErrNoServer = errors.New("no running Orcon server picked up the reload request")

// ReloadRequest is the content of the request file.
type ReloadRequest struct {
	// RequestID is random, so a result can be matched to its request.
	RequestID string `json:"requestId"`
}

// ReloadResult is the answer of one server.
type ReloadResult struct {
	// RequestID is the id of the request this answers.
	RequestID string `json:"requestId" yaml:"requestId"`
	// ServerID identifies the server process that answered.
	ServerID string `json:"serverId" yaml:"serverId"`
	// AppliedKeys lists the live keys whose new value is now in effect.
	AppliedKeys []string `json:"appliedKeys" yaml:"appliedKeys"`
	// RestartRequiredKeys lists the keys that changed and need a restart; they keep their running value.
	RestartRequiredKeys []string `json:"restartRequiredKeys" yaml:"restartRequiredKeys"`
	// Errors lists why the new configuration was refused; the previous one stays in effect.
	Errors []string `json:"errors" yaml:"errors"`
}

// ReloadReport is what orcon config reload prints: one result per server that answered.
type ReloadReport struct {
	// RequestID is the id of the request.
	RequestID string `json:"requestId" yaml:"requestId"`
	// Servers holds the result of each server that answered, sorted by server id.
	Servers []ReloadResult `json:"servers" yaml:"servers"`
	// Unanswered lists the servers that were running when the request was made and did not answer in time.
	Unanswered []string `json:"unanswered" yaml:"unanswered"`
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

func randomID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

// liveServers lists the servers whose registration is recent enough.
func liveServers(dir string, fresh time.Duration) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var ids []string
	for _, e := range entries {
		id, ok := strings.CutPrefix(e.Name(), serverPrefix)
		if !ok {
			continue
		}
		info, err := e.Info()
		if err == nil && time.Since(info.ModTime()) <= fresh {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

// RequestReload asks every server that uses dataDir to reload its configuration
// and waits up to wait for all of them to answer. The servers are the ones that
// registered in the run directory recently. It returns [ErrNoServer] when no
// server is running or none answers; a server that answers late is listed in
// Unanswered. The request and the results are removed before it returns.
func RequestReload(ctx context.Context, dataDir string, wait, poll time.Duration) (*ReloadReport, error) {
	dir := RunDir(dataDir)
	expected := liveServers(dir, staleServerPolls*ReloadPollInterval)
	if len(expected) == 0 {
		return nil, noServerError(dataDir)
	}
	id, err := randomID()
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(ReloadRequest{RequestID: id})
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	request := filepath.Join(dir, requestFileName)
	if err := writeAtomic(request, body); err != nil {
		return nil, err
	}
	defer cleanup(dir, request, id)

	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	tick := time.NewTicker(poll)
	defer tick.Stop()
	for {
		results := collectResults(dir, id)
		if len(answered(results, expected)) == len(expected) {
			return reloadReport(id, results, expected), nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("wait for the reload results: %w", ctx.Err())
		case <-deadline.C:
			results = collectResults(dir, id)
			if len(results) == 0 {
				return nil, noServerError(dataDir)
			}
			return reloadReport(id, results, expected), nil
		case <-tick.C:
		}
	}
}

func noServerError(dataDir string) error {
	return fmt.Errorf("%w in %s; is it running with this data directory?", ErrNoServer, dataDir)
}

// cleanup removes the request and the results of request id; best effort, the command is finishing.
func cleanup(dir, request, id string) {
	_ = os.Remove(request)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), resultPrefix) {
			if r := readResult(filepath.Join(dir, e.Name()), id); r != nil {
				_ = os.Remove(filepath.Join(dir, e.Name()))
			}
		}
	}
}

func collectResults(dir, id string) []ReloadResult {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []ReloadResult
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), resultPrefix) {
			continue
		}
		if r := readResult(filepath.Join(dir, e.Name()), id); r != nil {
			out = append(out, *r)
		}
	}
	slices.SortFunc(out, func(a, b ReloadResult) int { return strings.Compare(a.ServerID, b.ServerID) })
	return out
}

func answered(results []ReloadResult, expected []string) []string {
	var got []string
	for _, r := range results {
		if slices.Contains(expected, r.ServerID) {
			got = append(got, r.ServerID)
		}
	}
	return got
}

func reloadReport(id string, results []ReloadResult, expected []string) *ReloadReport {
	rep := &ReloadReport{RequestID: id, Servers: results, Unanswered: []string{}}
	got := answered(results, expected)
	for _, e := range expected {
		if !slices.Contains(got, e) {
			rep.Unanswered = append(rep.Unanswered, e)
		}
	}
	return rep
}

// readResult returns the result for request id, or nil when it is not there or answers another request.
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

// WatchReloadRequests registers this server in <dataDir>/run and, every poll
// interval, looks for a reload request. It handles each request id once: it
// calls handle and writes the returned result in its own result file, so the
// command can read one answer per server. Every server sharing the data
// directory answers. It runs on the calling goroutine until ctx ends; the caller
// owns it and waits for it to return. It removes its registration on the way out.
func WatchReloadRequests(ctx context.Context, dataDir string, poll time.Duration, handle func(requestID string) ReloadResult, logger *slog.Logger) {
	dir := RunDir(dataDir)
	serverID, err := randomID()
	if err != nil {
		logger.ErrorContext(ctx, "orcon config reload cannot reach this server", "error", err)
		return
	}
	serverID = serverID[:12]
	registration := filepath.Join(dir, serverPrefix+serverID)
	defer func() { _ = os.Remove(registration) }() // best effort: a stale registration expires by itself
	var handled []string
	register := func() {
		if err := writeAtomic(registration, []byte(serverID+"\n")); err != nil {
			logger.WarnContext(ctx, "cannot register for orcon config reload", "error", err)
		}
	}
	register()
	tick := time.NewTicker(poll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			register()
			handled = serveRequest(ctx, dir, serverID, handled, handle, logger)
		}
	}
}

// serveRequest handles the pending request if this server has not handled its id yet.
func serveRequest(ctx context.Context, dir, serverID string, handled []string, handle func(string) ReloadResult, logger *slog.Logger) []string {
	request := filepath.Join(dir, requestFileName)
	info, err := os.Stat(request)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logger.WarnContext(ctx, "cannot look at the reload request", "error", err)
		}
		return handled
	}
	if time.Since(info.ModTime()) > staleRequest {
		return handled
	}
	data, err := readBounded(request)
	if err != nil {
		return handled // the command may have removed it meanwhile
	}
	var req ReloadRequest
	if err := json.Unmarshal(data, &req); err != nil || req.RequestID == "" {
		logger.WarnContext(ctx, "ignored a malformed reload request")
		return handled
	}
	if slices.Contains(handled, req.RequestID) {
		return handled
	}
	handled = append(handled, req.RequestID)
	if len(handled) > rememberedRequests {
		handled = handled[1:]
	}
	res := handle(req.RequestID)
	res.RequestID, res.ServerID = req.RequestID, serverID
	body, err := json.Marshal(res)
	if err != nil {
		logger.ErrorContext(ctx, "cannot encode the reload result", "error", err)
		return handled
	}
	if err := writeAtomic(filepath.Join(dir, resultPrefix+serverID), body); err != nil {
		logger.ErrorContext(ctx, "cannot write the reload result", "error", err)
	}
	return handled
}
