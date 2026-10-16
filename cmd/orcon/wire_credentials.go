// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/wirvii/orcon/internal/connector/declarative"
	httpdispatch "github.com/wirvii/orcon/internal/connector/dispatch"
	"github.com/wirvii/orcon/internal/credential"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/platform/id"
)

// ErrCredentialFields means a credential holds no field set orcon.http@1 knows how to apply.
var ErrCredentialFields = errors.New("the credential holds no token, username and password, or apiKey field")

// resolveCredentials is the credential function of the HTTP node over the credential service. It runs on
// every activation, so a removed share, a disabled credential or a host the credential is not bound to stops
// the next request (ADR-0043 sections 3 and 4). Nothing of a secret is in the errors it returns.
func resolveCredentials(r credential.Resolver, scopes store.Scopes) func(context.Context, httpdispatch.CredentialRequest) (httpdispatch.Credential, error) {
	return func(ctx context.Context, req httpdispatch.CredentialRequest) (httpdispatch.Credential, error) {
		cid, err := id.Parse(req.ID)
		if err != nil {
			return httpdispatch.Credential{}, fmt.Errorf("credential %q is not a credential identifier: %w", req.ID, credential.ErrNotFound)
		}
		scope, err := scopes.ScopeOf(req.Org)
		if err != nil {
			return httpdispatch.Credential{}, fmt.Errorf("credential scope: %w", err)
		}
		res, err := r.Resolve(ctx, scope, req.Project, cid)
		if err != nil {
			return httpdispatch.Credential{}, err
		}
		if err := res.CheckHost(req.Host); err != nil {
			return httpdispatch.Credential{}, err
		}
		return credentialOf(res)
	}
}

// credentialOf picks the authentication scheme from the fields the credential holds: username and password
// give BASIC, apiKey gives API_KEY in the header named by the public field headerName (X-API-Key by default),
// and token gives BEARER. The fields travel to the interpreter, which keeps them out of items, logs and results.
func credentialOf(res credential.Resolved) (httpdispatch.Credential, error) {
	fields := map[string]string{}
	for k, v := range res.Public {
		fields[k] = v
	}
	for _, name := range []string{"token", "username", "password", "apiKey"} {
		if v, ok := res.Secret(name); ok {
			fields[name] = v
		}
	}
	_, hasUser := fields["username"]
	_, hasPass := fields["password"]
	switch {
	case hasUser && hasPass:
		return httpdispatch.Credential{Auth: declarative.Auth{Type: declarative.AuthBasic}, Fields: fields}, nil
	case fields["apiKey"] != "":
		name := fields["headerName"]
		if name == "" {
			name = "X-API-Key"
		}
		return httpdispatch.Credential{Auth: declarative.Auth{Type: declarative.AuthAPIKey, In: "HEADER", Name: name, Field: "apiKey"}, Fields: fields}, nil
	case fields["token"] != "":
		return httpdispatch.Credential{Auth: declarative.Auth{Type: declarative.AuthBearer, Field: "token"}, Fields: fields}, nil
	}
	return httpdispatch.Credential{}, ErrCredentialFields
}
