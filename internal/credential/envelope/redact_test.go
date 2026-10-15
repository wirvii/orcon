// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package envelope

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// TestKeyMaterialNeverPrintsInStructs formats every struct that holds key
// material, by value and by pointer, with the fmt verbs that reflect into
// private fields and with both slog handlers, and checks that no form of the
// key bytes appears.
func TestKeyMaterialNeverPrintsInStructs(t *testing.T) {
	t.Parallel()
	raw := bytes.Repeat([]byte{0xA7}, KeySize)
	key, err := NewKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	mk := NewLocalMasterKey("file", "/k", key)
	ring := NewKeyring(mk, newFakeStore())
	// UTC times carry no pointer to the lazily loaded local zone. fmt walks
	// time.Time.loc by reflection while another test's logging initializes
	// the local zone, which is a data race on that zone.
	expires := time.Unix(0, 0).UTC()
	ring.keys[KeyID{Level: LevelInstance, Version: 1}] = cachedKey{key: key.clone(), expires: expires}

	values := map[string]any{
		"Key":              key,
		"LocalMasterKey":   *mk,
		"*LocalMasterKey":  mk,
		"cachedKey":        cachedKey{key: key, expires: expires},
		"DataKey":          DataKey{Key: key},
		"*Keyring":         ring,
		"struct with Key":  struct{ K Key }{key},
		"private Key":      struct{ k Key }{key},
		"slice of Keys":    []Key{key},
		"map of Keys":      map[string]Key{"a": key},
		"pointer to Key":   &key,
		"private pointers": struct{ k *Key }{&key},
	}
	forbidden := []string{
		string(raw), hex.EncodeToString(raw), strings.ToUpper(hex.EncodeToString(raw)),
		"167 167", "0xa7, 0xa7", "a7a7", "A7A7",
	}
	for name, v := range values {
		outputs := map[string]string{
			"%v":  fmt.Sprintf("%v", v),
			"%+v": fmt.Sprintf("%+v", v),
			"%#v": fmt.Sprintf("%#v", v),
			"%s":  fmt.Sprintf("%s", v),
			"%x":  fmt.Sprintf("%x", v),
			"%d":  fmt.Sprintf("%d", v),
		}
		var text, jsonOut bytes.Buffer
		slog.New(slog.NewTextHandler(&text, nil)).Info("m", slog.Any("v", v))
		slog.New(slog.NewJSONHandler(&jsonOut, nil)).Info("m", slog.Any("v", v))
		outputs["slog text"], outputs["slog json"] = text.String(), jsonOut.String()
		for how, out := range outputs {
			for _, bad := range forbidden {
				if strings.Contains(out, bad) {
					t.Errorf("%s printed with %s contains key bytes (%q): %.200s", name, how, bad, out)
				}
			}
		}
	}
}
