// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package env

import (
	"bytes"
	"encoding/base64"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/credential/envelope"
)

func TestEnvProvider(t *testing.T) {
	t.Parallel()
	raw := bytes.Repeat([]byte{0x42}, envelope.KeySize)
	encoded := base64.StdEncoding.EncodeToString(raw)
	getenv := func(v string) func(string) string {
		return func(name string) string {
			if name != VarName {
				t.Errorf("getenv(%q), want only %q", name, VarName)
			}
			return v
		}
	}

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	p, err := New(t.Context(), getenv(encoded), logger)
	if err != nil {
		t.Fatalf("New = %v, want nil", err)
	}
	if p.Provider() != "env" || p.Reference() != "env:ORCON_MASTER_KEY" {
		t.Errorf("provider = %q, reference = %q", p.Provider(), p.Reference())
	}
	if !strings.Contains(logs.String(), "level=WARN") || strings.Contains(logs.String(), encoded) {
		t.Errorf("startup log = %q, want a warning without the key", logs.String())
	}
	want, _ := envelope.NewKey(raw)
	if got, _ := p.Fingerprint(t.Context()); got != envelope.Fingerprint(want) {
		t.Errorf("Fingerprint = %q, want %q", got, envelope.Fingerprint(want))
	}

	for name, value := range map[string]string{
		"unset":       "",
		"not base64":  "%%%%",
		"too short":   base64.StdEncoding.EncodeToString(raw[:16]),
		"raw 32 text": string(bytes.Repeat([]byte("k"), envelope.KeySize)) + "!",
	} {
		if _, err := New(t.Context(), getenv(value), nil); !errors.Is(err, envelope.ErrMasterKeyInvalid) {
			t.Errorf("New(%s) = %v, want ErrMasterKeyInvalid", name, err)
		}
	}
}
