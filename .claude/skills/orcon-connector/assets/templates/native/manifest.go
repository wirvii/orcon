// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package acmequeue

import (
	"embed"

	"github.com/wirvii/orcon/packages/go/connector/sdk"
)

// Files holds the package files that are not Go code: authoring record,
// catalogs, icon, documentation and recorded tests.
//
//go:embed authoring.json i18n icons docs tests
var Files embed.FS

// manifest is the orcon/connector@1 manifest of the connector. Enumerated
// values are UPPER_SNAKE (NM-13); names and identifiers keep their form.
var manifest = sdk.Manifest{
	Format:      "orcon/connector@1",
	ID:          "orcon.acme-queue",
	Version:     "1.0.0",
	Kind:        sdk.KindNative,
	DisplayName: "Acme Queue",
	Description: "Send and read messages in Acme Queue.",
	License:     "Apache-2.0",
	Compat:      sdk.Compat{EngineAPI: "^1.0.0"},
	Nodes: []sdk.Node{{
		Name:        "acmeQueue",
		Type:        "orcon.acme-queue.acmeQueue@1",
		DisplayName: "Acme Queue",
		Group:       sdk.GroupAction,
		Parameters: []sdk.Parameter{
			{Name: "queue", Type: sdk.ParamString, Required: true, Description: "Name of the queue."},
			{Name: "body", Type: sdk.ParamJSON, Required: true, DisplayWhen: map[string][]any{"operation": {"message.send"}}, Description: "Body of the message."},
			{Name: "returnAll", Type: sdk.ParamBoolean, Default: false, DisplayWhen: map[string][]any{"operation": {"message.getMany"}}, Description: "Whether to return every waiting message instead of the first ones up to the limit."},
			{Name: "limit", Type: sdk.ParamNumber, Default: 50, Minimum: 1, DisplayWhen: map[string][]any{"operation": {"message.getMany"}, "returnAll": {false}}, Description: "Maximum number of messages to return."},
		},
		Operations: []sdk.Operation{
			{Name: "message.getMany", DisplayName: "Get many messages", Purity: sdk.PurityRead, ItemMode: sdk.ItemModePerItem},
			{
				Name: "message.send", DisplayName: "Send a message", Purity: sdk.PurityEffect, ItemMode: sdk.ItemModePerItem,
				Safety: &sdk.Safety{
					Effect: sdk.EffectWrite, Level: sdk.LevelUnsafe,
					Idempotency: &sdk.Idempotency{Mode: sdk.IdempotencyNone},
					OnUncertain: sdk.OnUncertainSuspend, RetrySafe: false,
					ManualCheck: &sdk.ManualCheck{
						Description: "Open the queue {{ $parameter.queue }} in the Acme Queue console and look for a message sent around {{ $intent.sentAt }}.",
						URL:         "https://console.acme.example/queues/{{ $parameter.queue }}",
					},
					ProofLevel: sdk.ProofDeclared,
				},
			},
		},
		Outputs: []string{"main"},
	}},
	Credentials: []sdk.CredentialType{{
		ID:          "password",
		DisplayName: "Acme Queue user",
		Fields: []sdk.CredentialField{
			{Name: "username", Type: sdk.ParamString, Required: true},
			{Name: "password", Type: sdk.ParamString, Sensitive: true, Required: true},
		},
		Scheme: sdk.Scheme{Type: sdk.SchemeNone},
		Test:   "message.getMany",
	}},
	Permissions: sdk.Permissions{
		Network:             sdk.Network{Domains: []string{"queue.acme.example"}, Ports: []int{5671}},
		Credentials:         []string{"password"},
		CredentialRawAccess: true,
	},
	Changelog: []sdk.ChangelogEntry{{Version: "1.0.0", Date: "2026-10-13", Notes: []string{"changelog.1.0.0.first"}}},
}
