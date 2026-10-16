// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package whatsapp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// eventsOf reads one fixture of the webhook and returns its events, canonical,
// in order.
func eventsOf(t *testing.T, file string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", filepath.Clean("/"+file)))
	if err != nil {
		t.Fatalf("reading %s: %v", file, err)
	}
	body, err := jsonvalue.Parse(data)
	if err != nil {
		t.Fatalf("%s is not JSON: %v", file, err)
	}
	events, err := loadDefinition(t).Triggers["whatsappTrigger"].Decode.Events(body)
	if err != nil {
		t.Fatalf("Events(%s) failed: %v", file, err)
	}
	out := make([]string, 0, len(events))
	for _, e := range events {
		text, err := jsonvalue.Canonical(e)
		if err != nil {
			t.Fatalf("event is not encodable: %v", err)
		}
		out = append(out, string(text))
	}
	return out
}

// TestTextMessageBecomesOneNormalizedEvent: a text message keeps its sender,
// the contact name, the timestamp and the body, and the phone number that
// received it.
func TestTextMessageBecomesOneNormalizedEvent(t *testing.T) {
	got := eventsOf(t, "webhook-text-message.json")
	want := []string{canonical(t, `{"contactName":"Sheena Nelson","from":"16505551234",`+
		`"id":"wamid.HBgLMTY1MDM4Nzg4ODAVAgASGBQzQTU2RjM1NjdGRjE5NEZDODg2RQA=","phoneNumberId":"106540352242922",`+
		`"text":"Does it come in another color?","timestamp":"1504902988","type":"text"}`)}
	assertEvents(t, got, want)
}

// TestMediaMessageCarriesTheMediaId: an image has no text body; its media id
// is read from the type-specific object and the event has no text key.
func TestMediaMessageCarriesTheMediaId(t *testing.T) {
	got := eventsOf(t, "webhook-image-message.json")
	want := []string{canonical(t, `{"contactName":"Sheena Nelson","from":"16505551234",`+
		`"id":"wamid.HBgLMTY1MDM4Nzg4ODAVAgASGBQzQTU2RjM1NjdGRjE5NEZDODg2RQA=","mediaId":"1234567890123456",`+
		`"phoneNumberId":"106540352242922","timestamp":"1504902990","type":"image"}`)}
	assertEvents(t, got, want)
}

// TestEachMessageOfOneNotificationIsAnEvent: a notification with two messages
// yields two events in the order the provider sent them; an interactive reply
// takes its text from the button title.
func TestEachMessageOfOneNotificationIsAnEvent(t *testing.T) {
	got := eventsOf(t, "webhook-two-messages.json")
	want := []string{
		canonical(t, `{"contactName":"Sheena Nelson","from":"16505551234","id":"wamid.A1","phoneNumberId":"106540352242922","text":"Hola","timestamp":"1504903001","type":"text"}`),
		canonical(t, `{"contactName":"Sheena Nelson","from":"16505551234","id":"wamid.B2","phoneNumberId":"106540352242922","text":"Sí, quiero","timestamp":"1504903002","type":"interactive"}`),
	}
	assertEvents(t, got, want)
}

// TestStatusUpdatesYieldNoEvents: a delivery receipt has statuses and no
// messages, so it produces no event and the ingress acknowledges it.
func TestStatusUpdatesYieldNoEvents(t *testing.T) {
	assertEvents(t, eventsOf(t, "webhook-status-update.json"), nil)
}

func assertEvents(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event %d is %s, want %s", i, got[i], want[i])
		}
	}
}
