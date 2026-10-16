// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/config"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/trigger"
)

// TestWhatsAppTriggerCredentialIsChecked: the WhatsApp trigger verifies every inbound request with the
// stored credential its credential parameter names, so a missing or unusable one is refused when the flow
// is published and when it is activated, never on the first request (rule T-V3).
func TestWhatsAppTriggerCredentialIsChecked(t *testing.T) {
	for _, kind := range e2eKinds() {
		t.Run(string(kind), func(t *testing.T) {
			env := storetest.NewEngineEnv(t, kind, storetest.EngineOptions{})
			st, ok := env.Store.(engineStore)
			if !ok {
				t.Fatalf("the %s store does not implement the engine store", kind)
			}
			svc, owner := credentialServiceOf(t, env)
			good := storeCredential(t, svc, env, owner, "WhatsApp", "whatsapp", "graph.facebook.com", map[string]string{
				"token": "t", "appSecret": testAppSecret, "verifyToken": testVerifyToken,
			})
			noSecret := storeCredential(t, svc, env, owner, "Sends only", "whatsapp", "graph.facebook.com", map[string]string{"token": "t"})

			model := storeModelCredential(t, svc, env, owner)

			eng, err := newEngine(st, engineOptions{
				role: config.RoleStandalone, logger: slog.New(slog.DiscardHandler), version: "test", resolver: svc,
			})
			if err != nil {
				t.Fatalf("new engine: %v", err)
			}
			org := storetest.ScopeOf(env.Org)

			// The trigger check, one case at a time.
			check := connectorTriggerCredentials{resolver: svc}
			params := func(credential string) jsonvalue.Value {
				var members []jsonvalue.Member
				if credential != "<absent>" {
					members = append(members, jsonvalue.Member{Key: "credential", Value: jsonvalue.String(credential)})
				}
				v, err := jsonvalue.Object(members...)
				if err != nil {
					t.Fatalf("build the parameters: %v", err)
				}
				return v
			}
			cases := []struct {
				name, credential, want string
			}{
				{"no credential parameter", "<absent>", "names none"},
				{"an empty credential parameter", "", "names none"},
				{"not an identifier", "my-credential", "not a credential identifier"},
				{"an unknown credential", env.IDs.New().String(), "not found or not usable"},
				{"a credential without the app secret", noSecret, "no appSecret"},
				{"a valid credential", good, ""},
			}
			for _, c := range cases {
				got, err := check.Missing(t.Context(), org, env.Project, whatsappTriggerType, params(c.credential))
				if err != nil {
					t.Fatalf("%s: %v", c.name, err)
				}
				switch {
				case c.want == "" && len(got) != 0:
					t.Errorf("%s: reported %v", c.name, got)
				case c.want != "" && (len(got) != 1 || !strings.Contains(got[0], c.want)):
					t.Errorf("%s: reported %v, want one mentioning %q", c.name, got, c.want)
				}
			}
			if got, _ := (connectorTriggerCredentials{}).Missing(t.Context(), org, env.Project, whatsappTriggerType, params(good)); len(got) != 1 {
				t.Errorf("without a credential store the credential must be reported missing, got %v", got)
			}
			if got, err := check.Missing(t.Context(), org, env.Project, WebhookNodeType, params("<absent>")); err != nil || len(got) != 0 {
				t.Errorf("the native webhook needs no credential, got %v, %v", got, err)
			}

			// Publication refuses a flow whose trigger names a credential the project cannot use.
			assertTV3 := func(what string, err error) {
				t.Helper()
				var ve *flow.ValidationError
				if !errors.As(err, &ve) || !ve.Report.Has(trigger.CodeTV3, flow.SeverityError) {
					t.Fatalf("%s: want a %s validation error, got %v", what, trigger.CodeTV3, err)
				}
			}
			unknown := env.IDs.New().String()
			org, bad, by := storeWhatsAppFlow(t, env, fillOf(unknown, model))
			_, err = eng.trigger.Publish(t.Context(), trigger.PublishCommand{Org: org, Flow: bad, By: by})
			assertTV3("publish with an unknown credential", err)

			// A flow published with a valid credential cannot be activated once the credential is gone.
			org, ok2, by := storeWhatsAppFlow(t, env, fillOf(good, model))
			if _, err := eng.trigger.Publish(t.Context(), trigger.PublishCommand{Org: org, Flow: ok2, By: by}); err != nil {
				t.Fatalf("publish with a valid credential: %v", err)
			}
			gone, err := id.Parse(good)
			if err != nil {
				t.Fatalf("parse the credential id: %v", err)
			}
			if err := svc.Delete(t.Context(), org, env.Project, gone, time.Time{}, owner); err != nil {
				t.Fatalf("delete the credential: %v", err)
			}
			assertTV3("activate without the credential", eng.trigger.Activate(t.Context(), org, ok2))
		})
	}
}
