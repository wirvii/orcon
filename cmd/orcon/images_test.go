// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dockerCommand reads the ENV and CMD lines of a Dockerfile: the environment the image sets
// and the arguments after the entrypoint.
func dockerCommand(t *testing.T, path string) (env map[string]string, cmd []string) {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // G304: a Dockerfile of this repository
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	env = map[string]string{}
	for line := range strings.SplitSeq(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "ENV "):
			if name, value, ok := strings.Cut(strings.TrimPrefix(line, "ENV "), "="); ok {
				env[name] = value
			}
		case strings.HasPrefix(line, "CMD "):
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "CMD ")), &cmd); err != nil {
				t.Fatalf("%s: CMD is not a JSON array: %v", path, err)
			}
		}
	}
	if len(cmd) == 0 {
		t.Fatalf("%s has no CMD", path)
	}
	return env, cmd
}

// TestImageCommandsParse proves that the command of each image is one orcon accepts and that
// the listen address is the one the platform sends traffic to. It runs orcon config check
// with the flags of the CMD and the ENV of the image, which resolves them like orcon serve does.
func TestImageCommandsParse(t *testing.T) {
	images := []struct {
		dockerfile string
		address    string
	}{
		{filepath.Join("..", "..", "build", "release", "Dockerfile"), ":8080"},
		{filepath.Join("..", "..", "build", "preview", "Dockerfile"), "0.0.0.0:8080"}, // Cloud Run needs every interface
	}
	for _, img := range images {
		t.Run(filepath.Base(filepath.Dir(img.dockerfile)), func(t *testing.T) {
			env, cmd := dockerCommand(t, img.dockerfile)
			if cmd[0] != "serve" {
				t.Fatalf("CMD = %q, want it to start with serve", cmd)
			}
			for name, value := range env {
				t.Setenv(name, value)
			}
			if env["ORCON_DATA_DIR"] == "" {
				t.Setenv("HOME", t.TempDir()) // the base image sets HOME for the nonroot user
			}
			var stdout, stderr bytes.Buffer
			args := append([]string{"config", "check", "-o", "json"}, cmd[1:]...)
			if code := run(t.Context(), args, &stdout, &stderr); code != exitOK {
				t.Fatalf("config check %q exit = %d; stdout %s stderr %s", cmd[1:], code, stdout.String(), stderr.String())
			}
			var rep struct {
				Entries []struct {
					Key, Source string
					Value       any
				} `json:"entries"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
				t.Fatal(err)
			}
			for _, e := range rep.Entries {
				if e.Key == "http.address" && e.Value != img.address {
					t.Errorf("http.address = %v, want %s", e.Value, img.address)
				}
				if e.Key == "role" && e.Value != "standalone" {
					t.Errorf("role = %v, want standalone", e.Value)
				}
			}
		})
	}
}
