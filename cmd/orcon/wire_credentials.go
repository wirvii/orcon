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
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/identity"
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
			if errors.Is(err, credential.ErrHostNotAllowed) {
				return httpdispatch.Credential{}, &execution.NodeError{Code: httpdispatch.CodeCredentialHostNotAllowed, Class: execution.ClassPermanent, Err: err}
			}
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
		return httpdispatch.Credential{Auth: declarative.Auth{Type: declarative.AuthBasic}, Fields: fields, Mask: res.SecretValues()}, nil
	case fields["apiKey"] != "":
		name := fields["headerName"]
		if name == "" {
			name = "X-API-Key"
		}
		return httpdispatch.Credential{Auth: declarative.Auth{Type: declarative.AuthAPIKey, In: "HEADER", Name: name, Field: "apiKey"}, Fields: fields, Mask: res.SecretValues()}, nil
	case fields["token"] != "":
		return httpdispatch.Credential{Auth: declarative.Auth{Type: declarative.AuthBearer, Field: "token"}, Fields: fields, Mask: res.SecretValues()}, nil
	}
	return httpdispatch.Credential{}, ErrCredentialFields
}

// inboundFields returns every field of a stored credential, public and secret, for the checks that send no request
// with it: the signature of an inbound webhook and the subscription handshake of its provider. It runs on every
// request, for the project of the route, so a removed share or a disabled credential stops the next request
// (ADR-0043 section 3). No host is checked because nothing is sent. The values never reach a log or an error.
type inboundFields func(ctx context.Context, org identity.OrgID, project identity.ProjectID, credentialID string) (map[string]string, error)

// resolveInbound is the inbound credential function over the credential service.
func resolveInbound(r credential.Resolver, scopes store.Scopes) inboundFields {
	return func(ctx context.Context, org identity.OrgID, project identity.ProjectID, credentialID string) (map[string]string, error) {
		cid, err := id.Parse(credentialID)
		if err != nil {
			return nil, fmt.Errorf("the credential named by the node is not a credential identifier: %w", credential.ErrNotFound)
		}
		scope, err := scopes.ScopeOf(org)
		if err != nil {
			return nil, fmt.Errorf("credential scope: %w", err)
		}
		res, err := r.Resolve(ctx, scope, project, cid)
		if err != nil {
			return nil, err
		}
		fields := make(map[string]string, len(res.Public))
		for k, v := range res.Public {
			fields[k] = v
		}
		for _, name := range res.SecretNames() {
			if v, ok := res.Secret(name); ok {
				fields[name] = v
			}
		}
		return fields, nil
	}
}

// refuseInbound is the inbound credential function of a process that has no credential store.
func refuseInbound(context.Context, identity.OrgID, identity.ProjectID, string) (map[string]string, error) {
	return nil, ErrNoCredentialStore
}
